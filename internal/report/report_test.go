package report

import (
	"path/filepath"
	"testing"
	"time"

	"pos/internal/catalog"
	"pos/internal/order"
	"pos/internal/store"
)

func TestDailyAggregates(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	orders := order.NewService(db, catalog.NewService(db))
	svc := NewService(db)

	// 两单正常 + 一单作废
	o1, _ := orders.Create(order.CreateInput{Cash: 50, Items: []order.ItemInput{{ProductID: 1, Qty: 2}}})
	o2, _ := orders.Create(order.CreateInput{Cash: 50, Items: []order.ItemInput{{ProductID: 12, Qty: 1}}})
	o3, _ := orders.Create(order.CreateInput{Cash: 50, Items: []order.ItemInput{{ProductID: 1, Qty: 1}}})
	if _, err := orders.Void(o3.OrderNo, "点错", "店长"); err != nil {
		t.Fatalf("作废失败: %v", err)
	}
	// 退 o1 一件
	if _, err := orders.Refund(o1.OrderNo, []order.RefundItem{{ProductID: "1", Qty: 1}}, "退", "店长"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}

	d, err := svc.Daily(o1.BizDate)
	if err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if d.OrderCount != 2 {
		t.Fatalf("有效订单应为 2，实际 %d", d.OrderCount)
	}
	if d.VoidCount != 1 {
		t.Fatalf("作废应为 1，实际 %d", d.VoidCount)
	}
	wantGross := o1.Payable + o2.Payable
	if d.GrossSales != wantGross {
		t.Fatalf("销售额应为 %d，实际 %d", wantGross, d.GrossSales)
	}
	if d.RefundTotal != 1300 {
		t.Fatalf("退款合计应为 1300，实际 %d", d.RefundTotal)
	}
	if d.NetSales != wantGross-1300 {
		t.Fatalf("净销售应为 %d，实际 %d", wantGross-1300, d.NetSales)
	}
	if len(d.TopProducts) == 0 {
		t.Fatal("应有商品销量")
	}
	// 珍珠奶茶：3 件卖出，1 件作废，1 件退 → 净 1 件
	if d.TopProducts[0].Name != "珍珠奶茶" || d.TopProducts[0].Qty != 1 {
		t.Fatalf("销量统计不符: %+v", d.TopProducts[0])
	}
}

func TestDailyMemberDiscountSeparate(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	s := NewService(db)
	today := time.Now().Format("20060102")

	// 一单：会员支付 26 元，等级折扣让利 52 分
	if _, err := db.SQL().Exec(
		"INSERT INTO t_pos_order(shop_id,order_no,status,total_amount,discount_amount,payable_amount,"+
			"cash_amount,change_amount,round_off,member_discount,member_id,print_status,biz_date,created_at)"+
			" VALUES('shop_local','L"+today+"-9001','paid',2600,0,2548,3000,452,0,52,1,0,'"+today+"',datetime('now','localtime'))"); err != nil {
		t.Fatalf("造订单失败: %v", err)
	}
	if _, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_payment(order_id,method,amount,member_id,created_at) VALUES(1,'member',2548,1,datetime('now','localtime'))"); err != nil {
		t.Fatalf("造支付失败: %v", err)
	}

	d, err := s.Daily(today)
	if err != nil {
		t.Fatalf("日结失败: %v", err)
	}
	if d.MemberAmount != 2548 {
		t.Fatalf("会员消费应为 2548，实际 %d", d.MemberAmount)
	}
	if d.MemberDiscountTotal != 52 {
		t.Fatalf("会员折扣应单独统计为 52，实际 %d", d.MemberDiscountTotal)
	}
	if d.NetSales != 2548 {
		t.Fatalf("净销售应为 2548，实际 %d", d.NetSales)
	}
}
