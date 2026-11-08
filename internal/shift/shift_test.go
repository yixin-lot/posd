package shift

import (
	"path/filepath"
	"testing"

	"pos/internal/catalog"
	"pos/internal/member"
	"pos/internal/order"
	"pos/internal/store"
)

func TestOpenSellAndClose(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	shifts := NewService(db)
	orders := order.NewService(db, catalog.NewService(db))

	sh, err := shifts.Open(20000, "陈晓")
	if err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	if _, err := orders.Create(order.CreateInput{Cash: 2000, Items: []order.ItemInput{{ProductID: 1, Qty: 1}}}); err != nil {
		t.Fatalf("下单 1 失败: %v", err)
	}
	if _, err := orders.Create(order.CreateInput{Cash: 2000, Items: []order.ItemInput{{ProductID: 2, Qty: 1}}}); err != nil {
		t.Fatalf("下单 2 失败: %v", err)
	}

	closed, err := shifts.Close(sh.ID, 23100, "多 5 元")
	if err != nil {
		t.Fatalf("结班失败: %v", err)
	}
	// 期初 200 + 现金 25 = 225；实盘 231，差异 +6
	if closed.Expected != 22500 {
		t.Fatalf("现金应收应为 22500 分，实际 %d", closed.Expected)
	}
	if closed.Difference != 600 {
		t.Fatalf("差异应为 600 分，实际 %d", closed.Difference)
	}
	if closed.OrderCount != 2 {
		t.Fatalf("订单笔数应为 2，实际 %d", closed.OrderCount)
	}
	if len(closed.Payments) != 1 || closed.Payments[0].Amount != 2500 {
		t.Fatalf("现金收款应为 2500 分: %+v", closed.Payments)
	}
}

func TestSecondOpenRejected(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	svc := NewService(db)
	if _, err := svc.Open(100, "A"); err != nil {
		t.Fatalf("首次开班失败: %v", err)
	}
	if _, err := svc.Open(100, "B"); err == nil {
		t.Fatal("已有未结班次时应拒绝")
	}
}
// 退款要从钱箱拿走现金：现金应收 = 期初 + 收款 - 退款。
func TestCloseAccountsRefund(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	shifts := NewService(db)
	orders := order.NewService(db, catalog.NewService(db))

	sh, err := shifts.Open(10000, "陈晓")
	if err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	o, err := orders.Create(order.CreateInput{Cash: 2000, Items: []order.ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if _, err := orders.Refund(o.OrderNo, nil, "退货", "陈晓"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}

	cur, _, err := shifts.Latest()
	if err != nil {
		t.Fatalf("查班次失败: %v", err)
	}
	if cur.RefundTotal != o.Payable {
		t.Fatalf("退款合计应为 %d，实际 %d", o.Payable, cur.RefundTotal)
	}

	closed, err := shifts.Close(sh.ID, 10000, "退了一单")
	if err != nil {
		t.Fatalf("结班失败: %v", err)
	}
	// 100 期初 + 13 收款 - 13 退款 = 100
	if closed.Expected != 10000 {
		t.Fatalf("现金应收应为 10000 分，实际 %d", closed.Expected)
	}
	if closed.Difference != 0 {
		t.Fatalf("差异应为 0，实际 %d", closed.Difference)
	}
}
// 作废单不计销售，但钱进过钱箱又退出去，现金应收不变。
func TestVoidExcludedFromShiftSales(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	shifts := NewService(db)
	orders := order.NewService(db, catalog.NewService(db))

	sh, err := shifts.Open(10000, "陈晓")
	if err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	o, _ := orders.Create(order.CreateInput{Cash: 2000, Items: []order.ItemInput{{ProductID: 1, Qty: 1}}})
	if _, err := orders.Void(o.OrderNo, "点错", "陈晓"); err != nil {
		t.Fatalf("作废失败: %v", err)
	}
	cur, _, _ := shifts.Latest()
	if cur.Total != 0 || cur.OrderCount != 0 {
		t.Fatalf("作废单不应计入销售：total=%d count=%d", cur.Total, cur.OrderCount)
	}
	closed, err := shifts.Close(sh.ID, 10000, "")
	if err != nil {
		t.Fatalf("结班失败: %v", err)
	}
	if closed.Expected != 10000 {
		t.Fatalf("现金应收应为 10000，实际 %d", closed.Expected)
	}
}

func TestShiftDineInCounts(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	shifts := NewService(db)
	orders := order.NewService(db, catalog.NewService(db))

	sh, err := shifts.Open(10000, "店长")
	if err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	// 外带单
	if _, err := orders.Create(order.CreateInput{Cash: 2000, Items: []order.ItemInput{{ProductID: 1, Qty: 1}}}); err != nil {
		t.Fatalf("外带下单失败: %v", err)
	}
	// 堂食单：挂单 + 写桌台 + 结账
	dine, err := orders.Create(order.CreateInput{Hold: true, Items: []order.ItemInput{{ProductID: 1, Qty: 2}}})
	if err != nil {
		t.Fatalf("开台挂单失败: %v", err)
	}
	if err := orders.SetTable(dine.OrderNo, "A1"); err != nil {
		t.Fatalf("写桌台失败: %v", err)
	}
	if _, err := orders.Pay(dine.OrderNo, []order.PaymentInput{{Method: "cash", Amount: 2600}}, 3000); err != nil {
		t.Fatalf("结账失败: %v", err)
	}

	cur, _, err := shifts.Latest()
	if err != nil {
		t.Fatalf("查班次失败: %v", err)
	}
	if cur.DineInCount != 1 || cur.TakeawayCnt != 1 {
		t.Fatalf("堂食/外带应为 1/1，实际 %d/%d", cur.DineInCount, cur.TakeawayCnt)
	}
	closed, err := shifts.Close(sh.ID, 13900, "")
	if err != nil {
		t.Fatalf("结班失败: %v", err)
	}
	if closed.Difference != 0 {
		t.Fatalf("差异应为 0，实际 %d", closed.Difference)
	}
}

func TestShiftPointsStats(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	shifts := NewService(db)
	members := member.NewService(db)
	orders := order.NewService(db, catalog.NewService(db))
	orders.SetMembers(members)

	m, _ := members.Create("13800000701", "积分班次")
	if _, err := members.Topup(m.ID, 10000, "充值"); err != nil {
		t.Fatalf("充值失败: %v", err)
	}
	if _, err := shifts.Open(10000, "店长"); err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	if _, err := orders.Create(order.CreateInput{
		MemberID: m.ID,
		Payments: []order.PaymentInput{{Method: "member", Amount: 2600, MemberID: m.ID}},
		Items:    []order.ItemInput{{ProductID: 1, Qty: 2}},
	}); err != nil {
		t.Fatalf("会员支付失败: %v", err)
	}

	sh, _, err := shifts.Latest()
	if err != nil {
		t.Fatalf("查班次失败: %v", err)
	}
	if sh.PointsEarned != 26 {
		t.Fatalf("本班应发放 26 积分，实际 %d", sh.PointsEarned)
	}
}

func TestShiftPointsByShiftID(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	shifts := NewService(db)
	members := member.NewService(db)
	orders := order.NewService(db, catalog.NewService(db))
	orders.SetMembers(members)

	m, _ := members.Create("13800000702", "跨窗积分")
	if _, err := members.Topup(m.ID, 10000, "充值"); err != nil {
		t.Fatalf("充值失败: %v", err)
	}
	if _, err := shifts.Open(10000, "店长"); err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	if _, err := orders.Create(order.CreateInput{
		MemberID: m.ID,
		Payments: []order.PaymentInput{{Method: "member", Amount: 2600, MemberID: m.ID}},
		Items:    []order.ItemInput{{ProductID: 1, Qty: 2}},
	}); err != nil {
		t.Fatalf("会员支付失败: %v", err)
	}
	sh, _, _ := shifts.Latest()

	// 把流水时间改到"班次开始之前"：按时间窗会算 0，按 shift_id 仍应算 26
	if _, err := db.SQL().Exec(
		"UPDATE t_pos_member_txn SET created_at='2000-01-01 00:00:00' WHERE member_id=?", m.ID); err != nil {
		t.Fatalf("改时间失败: %v", err)
	}
	got, err := shifts.byID(sh.ID)
	if err != nil {
		t.Fatalf("查班次失败: %v", err)
	}
	if got.PointsEarned != 26 {
		t.Fatalf("按 shift_id 应仍为 26，实际 %d", got.PointsEarned)
	}
}

func TestCrossRefundCounted(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	shifts := NewService(db)
	orders := order.NewService(db, catalog.NewService(db))

	// 一班：卖一单
	first, _ := shifts.Open(10000, "店长")
	o, err := orders.Create(order.CreateInput{Cash: 2000, Items: []order.ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if _, err := shifts.Close(first.ID, 11300, ""); err != nil {
		t.Fatalf("结班失败: %v", err)
	}

	// 二班：退上一班的单
	if _, err := shifts.Open(10000, "店长"); err != nil {
		t.Fatalf("二班开班失败: %v", err)
	}
	if _, err := orders.Refund(o.OrderNo, nil, "隔班退", "店长"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}

	cur, _, _ := shifts.Latest()
	got, err := shifts.byID(cur.ID)
	if err != nil {
		t.Fatalf("查班次失败: %v", err)
	}
	if got.CrossRefundCount != 1 {
		t.Fatalf("应识别 1 笔跨班次退款，实际 %d", got.CrossRefundCount)
	}
	if got.CrossRefundTotal != 1300 {
		t.Fatalf("跨班次退款金额应为 1300，实际 %d", got.CrossRefundTotal)
	}
}
