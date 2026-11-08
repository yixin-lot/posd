package order

import "testing"

func TestServeDeductsStockWhenOnServe(t *testing.T) {
	svc, st := newModeSvc(t)
	svc.SetDeductOnServe(func() bool { return true })
	if _, err := st.Set(1, 10, 2); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}
	o, err := svc.Create(CreateInput{Cash: 5000, Items: []ItemInput{{ProductID: 1, Qty: 2}}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if row, _ := st.Get(1); row.Qty != 10 {
		t.Fatalf("on_serve 下单不应扣料，应仍为 10，实际 %d", row.Qty)
	}
	if _, err := svc.Serve(o.OrderNo); err != nil {
		t.Fatalf("出餐失败: %v", err)
	}
	if row, _ := st.Get(1); row.Qty != 8 {
		t.Fatalf("出餐后应扣到 8，实际 %d", row.Qty)
	}
	var n int
	_ = svc.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_stock_move WHERE ref_id=? AND move_type='sale'", o.OrderNo).Scan(&n)
	if n != 1 {
		t.Fatalf("应写 1 条销售流水，实际 %d", n)
	}
}
