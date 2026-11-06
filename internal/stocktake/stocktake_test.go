package stocktake

import (
	"errors"
	"path/filepath"
	"testing"

	"pos/internal/stock"
	"pos/internal/store"
)

func newSvc(t *testing.T) (*Service, *stock.Service) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.SQL().Exec(
		"INSERT INTO t_pos_product(id,shop_id,name,category,price,active,created_at,updated_at) VALUES(101,1,'盘点奶茶','奶茶',1300,1,datetime('now'),datetime('now')),(102,1,'盘点蛋挞','甜点',600,1,datetime('now'),datetime('now'))"); err != nil {
		t.Fatalf("建商品失败: %v", err)
	}
	return NewService(db), stock.NewService(db)
}

func TestStocktakeFlowAppliesDiff(t *testing.T) {
	svc, st := newSvc(t)
	if _, err := st.Set(101, 10, 0); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}
	if _, err := st.Set(102, 5, 0); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}

	sh, err := svc.Create("月末盘点", "店长")
	if err != nil {
		t.Fatalf("生成盘点单失败: %v", err)
	}
	if len(sh.Items) != 2 {
		t.Fatalf("应有 2 条明细，实际 %d", len(sh.Items))
	}

	// 实盘：奶茶 8（盘亏 2），蛋挞 5（无差异）
	if _, err := svc.SetActual(sh.ID, map[int64]int{101: 8, 102: 5}); err != nil {
		t.Fatalf("录实盘失败: %v", err)
	}

	done, err := svc.Close(sh.ID, "店长")
	if err != nil {
		t.Fatalf("关单失败: %v", err)
	}
	if done.Status != "done" {
		t.Fatalf("关单后状态应为 done，实际 %s", done.Status)
	}

	row, _ := st.Get(101)
	if row.Qty != 8 {
		t.Fatalf("盘亏后奶茶应为 8，实际 %d", row.Qty)
	}
	row2, _ := st.Get(102)
	if row2.Qty != 5 {
		t.Fatalf("无差异的蛋挞应保持 5，实际 %d", row2.Qty)
	}

	// 审计留痕
	var n int
	if err := svc.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_audit WHERE action='stocktake'").Scan(&n); err != nil {
		t.Fatalf("查审计失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("应写 1 条盘点审计（差异项），实际 %d", n)
	}
}

func TestStocktakeClosedCannotReedit(t *testing.T) {
	svc, st := newSvc(t)
	if _, err := st.Set(101, 3, 0); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}
	sh, _ := svc.Create("", "店长")
	if _, err := svc.SetActual(sh.ID, map[int64]int{101: 3}); err != nil {
		t.Fatalf("录实盘失败: %v", err)
	}
	if _, err := svc.Close(sh.ID, "店长"); err != nil {
		t.Fatalf("关单失败: %v", err)
	}
	_, err := svc.SetActual(sh.ID, map[int64]int{101: 99})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("已关单不应再改，实际 %v", err)
	}
}
