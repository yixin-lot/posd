// Package store 是单机版的 SQLite 存储层（ADR-002：本地即真源）。
//
// 迁移脚本与 Java 版一一对应（0001..0005），保证两边能打开同一个库文件。
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// DB 是存储句柄。
type DB struct {
	sql *sql.DB
}

// Open 打开（或创建）数据库，并执行未应用的迁移。
func Open(path string) (*DB, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: 打开 %s 失败: %w", path, err)
	}
	// SQLite 单写者：连接池固定为 1，避免 SQLITE_BUSY。
	raw.SetMaxOpenConns(1)
	db := &DB{sql: raw}
	if err := db.migrate(); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return db, nil
}

// Close 关闭数据库。
func (d *DB) Close() error { return d.sql.Close() }

// SQL 暴露底层句柄，供仓储层使用。
func (d *DB) SQL() *sql.DB { return d.sql }

func (d *DB) migrate() error {
	if _, err := d.sql.Exec(
		"CREATE TABLE IF NOT EXISTS schema_version (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)"); err != nil {
		return fmt.Errorf("store: 建版本表失败: %w", err)
	}

	applied := map[string]bool{}
	rows, err := d.sql.Query("SELECT version FROM schema_version")
	if err != nil {
		return fmt.Errorf("store: 读取版本失败: %w", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return err
		}
		applied[v] = true
	}
	_ = rows.Close()

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	// 必须按版本号**数值**排序：字典序会把 V10 排在 V2 前面，
	// 导致新迁移引用的表还没建出来（V10 依赖 V4 的 t_pos_product）。
	sort.Slice(names, func(i, j int) bool {
		return versionNumber(names[i]) < versionNumber(names[j])
	})

	for _, name := range names {
		// 版本号取 "__" 前的前缀（V1/V2…），与 Java 版 Migrator 完全一致，
		// 否则两边会互相认为对方没执行过迁移，导致重复建表/重复种子数据。
		version := strings.SplitN(name, "__", 2)[0]
		if applied[version] {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := d.sql.Exec(string(body)); err != nil {
			return fmt.Errorf("store: 执行迁移 %s 失败: %w", name, err)
		}
		if _, err := d.sql.Exec("INSERT INTO schema_version(version, applied_at) VALUES(?, datetime('now'))", version); err != nil {
			return err
		}
	}
	return nil
}

// ---- 迁移期最小查询：只用于验证库结构与 Java 版兼容，不做业务逻辑 ----

// CountProducts 返回商品数。
func (d *DB) CountProducts() (int, error) {
	return d.count("SELECT COUNT(*) FROM t_pos_product")
}

// CountOrders 返回订单数。
func (d *DB) CountOrders() (int, error) {
	return d.count("SELECT COUNT(*) FROM t_pos_order")
}

// TodayPaid 返回指定营业日（biz_date，YYYYMMDD）已收款订单的笔数与金额合计。
// 只统计 status='paid'，挂单/退款/作废不计入「今日已收」。
func (d *DB) TodayPaid(bizDate string) (int, float64, error) {
	var cnt int
	var sum sql.NullFloat64
	err := d.sql.QueryRow(
		"SELECT COUNT(*), SUM(payable_amount) FROM t_pos_order WHERE status='paid' AND biz_date=?",
		bizDate).Scan(&cnt, &sum)
	if err != nil {
		return 0, 0, err
	}
	if !sum.Valid {
		return cnt, 0, nil
	}
	return cnt, sum.Float64, nil
}

// CountShifts 返回班次数。
func (d *DB) CountShifts() (int, error) {
	return d.count("SELECT COUNT(*) FROM t_pos_shift")
}

// CountStaff 返回员工数。
func (d *DB) CountStaff() (int, error) {
	return d.count("SELECT COUNT(*) FROM t_pos_staff")
}

// SchemaVersion 返回已应用的最大版本号（按数值比较，V10 > V9）。
func (d *DB) SchemaVersion() (string, error) {
	var v sql.NullString
	if err := d.sql.QueryRow(
		"SELECT version FROM schema_version ORDER BY CAST(SUBSTR(version,2) AS INTEGER) DESC LIMIT 1").
		Scan(&v); err != nil {
		return "", err
	}
	return v.String, nil
}

// versionNumber 解析 "V12__x.sql" → 12。
func versionNumber(name string) int {
	version := strings.SplitN(name, "__", 2)[0]
	n, _ := strconv.Atoi(strings.TrimPrefix(version, "V"))
	return n
}

func (d *DB) count(query string) (int, error) {
	var n int
	if err := d.sql.QueryRow(query).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
