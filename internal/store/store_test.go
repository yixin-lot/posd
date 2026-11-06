package store

import (
	"strconv"
	"strings"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "pos.db"))
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer db.Close()

	products, err := db.CountProducts()
	if err != nil {
		t.Fatalf("查商品失败: %v", err)
	}
	if products != 12 {
		t.Fatalf("种子商品应为 12，实际 %d", products)
	}

	version, err := db.SchemaVersion()
	if err != nil {
		t.Fatalf("查版本失败: %v", err)
	}
	if version != latestVersion() {
		t.Fatalf("迁移版本应为 %s，实际 %q", latestVersion(), version)
	}
}

// 兼容性测试：Java 版生成的库文件，Go 版必须能直接打开且数据完整。
func TestOpensJavaCreatedDatabase(t *testing.T) {
	src := filepath.Join("testdata", "java")
	dst := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("读取 fixture 失败: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), body, 0o644); err != nil {
			t.Fatalf("写入 %s 失败: %v", e.Name(), err)
		}
	}

	db, err := Open(filepath.Join(dst, "pos.db"))
	if err != nil {
		t.Fatalf("打开 Java 库失败: %v", err)
	}
	defer db.Close()

	// 迁移必须全部跳过（版本号已对齐），种子不能被重复插入。
	// fixture 是在 Java 环境里日常用过的库（含后来新增的商品），所以只断言"没有翻倍"。
	products, _ := db.CountProducts()
	if products < 12 || products >= 24 {
		t.Fatalf("Java 库商品数异常：%d（12 个种子；若接近 24 说明种子被重复执行）", products)
	}
	// 打开 Java 库后会补齐它没有的迁移（当前最新 V11），版本随之前进是预期行为。
	version, _ := db.SchemaVersion()
	if version != latestVersion() {
		t.Fatalf("打开 Java 库后应补齐迁移到 %s，实际 %q", latestVersion(), version)
	}
	orders, err := db.CountOrders()
	if err != nil {
		t.Fatalf("查订单失败: %v", err)
	}
	if orders < 1 {
		t.Fatalf("Java 库应至少 1 单，实际 %d", orders)
	}
	shifts, _ := db.CountShifts()
	if shifts < 1 {
		t.Fatalf("Java 库应至少 1 个班次，实际 %d", shifts)
	}
}

// latestVersion 取 migrations 目录里的最大版本号（按数值），
// 这样新增迁移不用再改测试断言。
func latestVersion() string {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return ""
	}
	best := 0
	out := ""
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		n := versionNumber(e.Name())
		if n > best {
			best, out = n, "V"+strconv.Itoa(n)
		}
	}
	return out
}
