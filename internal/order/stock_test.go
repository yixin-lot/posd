package order

import (
	"errors"
	"path/filepath"
	"testing"

	"pos/internal/catalog"
	"pos/internal/stock"
	"pos/internal/store"
)

func newStockSvc(t *testing.T) (*Service, *stock.Service) {
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

func TestOrderDeductsStockAndRefundRestores(t *testing.T) {
	svc, st := newStockSvc(t)
	if _, err := st.Set(1, 10, 2); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}

	o, err := svc.Create(CreateInput{Cash: 50, Items: []ItemInput{{ProductID: 1, Qty: 3}}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	row, _ := st.Get(1)
	if row.Qty != 7 {
		t.Fatalf("卖 3 份应剩 7，实际 %d", row.Qty)
	}

	if _, err := svc.Refund(o.OrderNo, []RefundItem{{ProductID: "1", Qty: 1}}, "退一份", "店长"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	row, _ = st.Get(1)
	if row.Qty != 8 {
		t.Fatalf("退 1 份应回到 8，实际 %d", row.Qty)
	}
}

func TestOrderDeductsBundleComponents(t *testing.T) {
	svc, st := newStockSvc(t)
	drink, _ := svc.products.Create(catalog.SaveInput{Name: "奶茶", Category: "奶茶", Price: 13})
	cake, _ := svc.products.Create(catalog.SaveInput{Name: "蛋挞", Category: "甜点", Price: 6})
	bundle, err := svc.products.Create(catalog.SaveInput{
		Name: "套餐", Category: "套餐", Price: 20, Kind: "bundle",
		Bundle: []catalog.BundleItem{{ComponentID: drink.ID, Qty: 1}, {ComponentID: cake.ID, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("建套餐失败: %v", err)
	}
	if _, err := st.Set(drink.ID, 10, 0); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}
	if _, err := st.Set(cake.ID, 10, 0); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}

	if _, err := svc.Create(CreateInput{Cash: 50, Items: []ItemInput{{ProductID: bundle.ID, Qty: 2}}}); err != nil {
		t.Fatalf("套餐下单失败: %v", err)
	}
	d, _ := st.Get(drink.ID)
	c, _ := st.Get(cake.ID)
	if d.Qty != 8 {
		t.Fatalf("奶茶应扣 2 剩 8，实际 %d", d.Qty)
	}
	if c.Qty != 6 {
		t.Fatalf("蛋挞应扣 4 剩 6，实际 %d", c.Qty)
	}
}

func TestStockBlockWhenEnabled(t *testing.T) {
	svc, st := newStockSvc(t)
	if _, err := st.Set(1, 1, 0); err != nil {
		t.Fatalf("建库存失败: %v", err)
	}
	// 不拦截：允许卖成负库存
	if _, err := svc.Create(CreateInput{Cash: 50, Items: []ItemInput{{ProductID: 1, Qty: 2}}}); err != nil {
		t.Fatalf("默认应允许负库存，实际 %v", err)
	}
	row, _ := st.Get(1)
	if row.Qty != -1 {
		t.Fatalf("应卖成 -1，实际 %d", row.Qty)
	}

	// 拦截模式
	if _, err := st.Set(1, 1, 0); err != nil {
		t.Fatalf("重置库存失败: %v", err)
	}
	_, err := svc.Create(CreateInput{StockBlockWhenShort: true, Cash: 50, Items: []ItemInput{{ProductID: 1, Qty: 2}}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("库存不足应拦截，实际 %v", err)
	}
	row, _ = st.Get(1)
	if row.Qty != 1 {
		t.Fatalf("被拦截后库存不该变化，实际 %d", row.Qty)
	}
}
