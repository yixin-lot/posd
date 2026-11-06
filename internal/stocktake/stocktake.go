// Package stocktake 是库存盘点：生成账面快照 → 录实盘 → 关单按差异调账。
//
// 为什么不直接改库存：盘点要留"盘前账面 / 实盘 / 差异"三列，
// 否则月底对不上账时无法复盘是谁、什么时候、差了多少。
package stocktake

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"pos/internal/store"
)

var (
	// ErrNotFound 盘点单不存在。
	ErrNotFound = errors.New("盘点单不存在")
	// ErrConflict 状态冲突（已关单/已关闭）。
	ErrConflict = errors.New("盘点单状态冲突")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
)

// Item 是一条盘点明细。
type Item struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	SystemQty int    `json:"systemQty"`
	ActualQty *int   `json:"actualQty"`
	Diff      int    `json:"diff"`
}

// Sheet 是一张盘点单。
type Sheet struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	Operator  string `json:"operator"`
	CreatedAt string `json:"createdAt"`
	ClosedAt  string `json:"closedAt,omitempty"`
	Items     []Item `json:"items"`
}

// Service 管理盘点单。
type Service struct {
	db *store.DB
}

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// Create 按当前库存生成一张盘点单（只包含建了库存行的商品）。
func (s *Service) Create(note, operator string) (Sheet, error) {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Sheet{}, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		"INSERT INTO t_pos_stocktake(status,note,operator,created_at)"+
			" VALUES('open',?,?,datetime('now','localtime'))",
		strings.TrimSpace(note), operator)
	if err != nil {
		return Sheet{}, err
	}
	id, _ := res.LastInsertId()
	if _, err := tx.Exec(
		"INSERT INTO t_pos_stocktake_item(stocktake_id,product_id,system_qty)"+
			" SELECT ?, product_id, qty FROM t_pos_stock", id); err != nil {
		return Sheet{}, err
	}
	if err := tx.Commit(); err != nil {
		return Sheet{}, err
	}
	return s.Get(id)
}

// List 返回盘点单列表（不含明细）。
func (s *Service) List(limit int) ([]Sheet, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.SQL().Query(
		"SELECT id,status,note,operator,created_at,COALESCE(closed_at,'')"+
			" FROM t_pos_stocktake ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Sheet{}
	for rows.Next() {
		var sh Sheet
		if err := rows.Scan(&sh.ID, &sh.Status, &sh.Note, &sh.Operator, &sh.CreatedAt, &sh.ClosedAt); err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// Get 读一张盘点单（含明细与名称）。
func (s *Service) Get(id int64) (Sheet, error) {
	var sh Sheet
	err := s.db.SQL().QueryRow(
		"SELECT id,status,note,operator,created_at,COALESCE(closed_at,'')"+
			" FROM t_pos_stocktake WHERE id=?", id).
		Scan(&sh.ID, &sh.Status, &sh.Note, &sh.Operator, &sh.CreatedAt, &sh.ClosedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Sheet{}, ErrNotFound
	}
	if err != nil {
		return Sheet{}, err
	}

	rows, err := s.db.SQL().Query(
		"SELECT i.product_id, COALESCE(p.name,''), i.system_qty, i.actual_qty"+
			" FROM t_pos_stocktake_item i LEFT JOIN t_pos_product p ON p.id=i.product_id"+
			" WHERE i.stocktake_id=? ORDER BY p.name, i.product_id", id)
	if err != nil {
		return Sheet{}, err
	}
	defer rows.Close()
	sh.Items = []Item{}
	for rows.Next() {
		var it Item
		var actual sql.NullInt64
		if err := rows.Scan(&it.ProductID, &it.Name, &it.SystemQty, &actual); err != nil {
			return Sheet{}, err
		}
		if actual.Valid {
			v := int(actual.Int64)
			it.ActualQty = &v
			it.Diff = v - it.SystemQty
		}
		sh.Items = append(sh.Items, it)
	}
	return sh, rows.Err()
}

// SetActual 录实盘数量（可分批提交）。
func (s *Service) SetActual(id int64, actual map[int64]int) (Sheet, error) {
	sh, err := s.Get(id)
	if err != nil {
		return Sheet{}, err
	}
	if sh.Status != "open" {
		return Sheet{}, fmt.Errorf("%w: 盘点单已关单", ErrConflict)
	}
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Sheet{}, err
	}
	defer func() { _ = tx.Rollback() }()
	for pid, qty := range actual {
		if qty < 0 {
			return Sheet{}, fmt.Errorf("%w: 实盘数量不能为负", ErrBadRequest)
		}
		res, err := tx.Exec(
			"UPDATE t_pos_stocktake_item SET actual_qty=? WHERE stocktake_id=? AND product_id=?",
			qty, id, pid)
		if err != nil {
			return Sheet{}, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return Sheet{}, fmt.Errorf("%w: 盘点单里没有该商品", ErrBadRequest)
		}
	}
	if err := tx.Commit(); err != nil {
		return Sheet{}, err
	}
	return s.Get(id)
}

// Close 关单：按差异调账（stock.qty = actual），写审计。
func (s *Service) Close(id int64, operator string) (Sheet, error) {
	sh, err := s.Get(id)
	if err != nil {
		return Sheet{}, err
	}
	if sh.Status != "open" {
		return Sheet{}, fmt.Errorf("%w: 盘点单已关单", ErrConflict)
	}

	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Sheet{}, err
	}
	defer func() { _ = tx.Rollback() }()

	changed := 0
	for _, it := range sh.Items {
		if it.ActualQty == nil || it.Diff == 0 {
			continue
		}
		if _, err := tx.Exec(
			"UPDATE t_pos_stock SET qty=?, updated_at=datetime('now','localtime') WHERE product_id=?",
			*it.ActualQty, it.ProductID); err != nil {
			return Sheet{}, err
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_audit(action,order_no,product_id,detail,operator,created_at)"+
				" VALUES('stocktake','',?,?,?,datetime('now','localtime'))",
			fmt.Sprint(it.ProductID),
			fmt.Sprintf("盘点单#%d %+d（账面 %d → 实盘 %d）", id, it.Diff, it.SystemQty, *it.ActualQty),
			operator); err != nil {
			return Sheet{}, err
		}
		changed++
	}
	if _, err := tx.Exec(
		"UPDATE t_pos_stocktake SET status='done', closed_at=datetime('now','localtime'), operator=?"+
			" WHERE id=?", operator, id); err != nil {
		return Sheet{}, err
	}
	if err := tx.Commit(); err != nil {
		return Sheet{}, err
	}
	out, err := s.Get(id)
	if err != nil {
		return Sheet{}, err
	}
	_ = changed
	return out, nil
}
