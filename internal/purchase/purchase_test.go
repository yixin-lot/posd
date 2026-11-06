package purchase

import (
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
	return NewService(db), stock.NewService(db)
}

func TestPurchaseAddsStockAndCost(t *testing.T) {
	svc, st := newSvc(t)
	// 商品 1、2 来自种子数据；先只给 1 建库存
	if _, err := st.Set(1, 2, 0); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}

	o, err := svc.Create("鲜奶供应商", "周一进货", "店长", []InputItem{
		{ProductID: 1, Qty: 10, UnitCost: 500},
		{ProductID: 2, Qty: 4, UnitCost: 300}, // 未跟踪：入库时应自动建行
	})
	if err != nil {
		t.Fatalf("进货失败: %v", err)
	}
	if o.TotalCost != 10*500+4*300 {
		t.Fatalf("总成本应为 %d，实际 %d", 10*500+4*300, o.TotalCost)
	}
	if len(o.Items) != 2 {
		t.Fatalf("应有 2 条明细，实际 %d", len(o.Items))
	}

	row, err := st.Get(1)
	if err != nil {
		t.Fatalf("读库存失败: %v", err)
	}
	if row.Qty != 12 {
		t.Fatalf("进货后应为 12，实际 %d", row.Qty)
	}
	row2, err := st.Get(2)
	if err != nil {
		t.Fatalf("未跟踪商品入库后应有库存行: %v", err)
	}
	if row2.Qty != 4 {
		t.Fatalf("新行应为 4，实际 %d", row2.Qty)
	}
}

func TestPurchaseRejectsBadItem(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.Create("", "", "", nil); err == nil {
		t.Fatal("空明细应报错")
	}
	if _, err := svc.Create("", "", "", []InputItem{{ProductID: 999999, Qty: 1, UnitCost: 1}}); err == nil {
		t.Fatal("不存在的商品应报错")
	}
}
