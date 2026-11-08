package order

import (
	"path/filepath"
	"testing"

	"pos/internal/catalog"
	"pos/internal/stock"
	"pos/internal/store"
)

func newModeSvc(t *testing.T) (*Service, *stock.Service) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := NewService(db, catalog.NewService(db))
	st := stock.NewService(db)
	svc.SetStock(st)
	return svc, st
}

// 已制作退品：不回补库存，写损耗台账，记录成本。
func TestRefundWasteKeepsStockAndWritesLoss(t *testing.T) {
	svc, st := newModeSvc(t)
	if _, err := st.Set(1, 10, 2); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}
	// 给商品设置进价，才能验证成本快照
	old, _ := svc.products.Get(1)
	if _, err := svc.products.Update(old.ID, catalog.SaveInput{Name: old.Name, Price: 13, Cost: 4, Kind: "single"}); err != nil {
		t.Fatalf("设置成本失败: %v", err)
	}
	o, err := svc.Create(CreateInput{Cash: 5000, Items: []ItemInput{{ProductID: 1, Qty: 3}}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if _, err := svc.RefundWithMode(o.OrderNo, []RefundItem{{ProductID: "1", Qty: 1}}, "做多了", "店长", "waste_made"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	row, _ := st.Get(1)
	if row.Qty != 7 {
		t.Fatalf("已制作退品不应回补库存，应仍为 7，实际 %d", row.Qty)
	}
	var n, cost int
	_ = svc.db.SQL().QueryRow("SELECT COUNT(*), COALESCE(SUM(cost_minor),0) FROM t_pos_loss WHERE ref_id=?", o.OrderNo).Scan(&n, &cost)
	if n != 1 || cost <= 0 {
		t.Fatalf("应写 1 条损耗且成本>0，实际 n=%d cost=%d", n, cost)
	}
}

// 未制作取消：即使门店默认不回补，也要回补库存，且不写损耗。
func TestRefundCancelRestocksEvenIfPolicyOff(t *testing.T) {
	svc, st := newModeSvc(t)
	if _, err := st.Set(1, 10, 2); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}
	o, err := svc.Create(CreateInput{Cash: 5000, Items: []ItemInput{{ProductID: 1, Qty: 3}}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if _, err := svc.RefundWithMode(o.OrderNo, []RefundItem{{ProductID: "1", Qty: 1}}, "未做", "店长", "cancel_unmade"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	row, _ := st.Get(1)
	if row.Qty != 8 {
		t.Fatalf("未制作取消应回补到 8，实际 %d", row.Qty)
	}
	var n int
	_ = svc.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_loss WHERE ref_id=?", o.OrderNo).Scan(&n)
	if n != 0 {
		t.Fatalf("未制作取消不应写损耗，实际 %d", n)
	}
}
