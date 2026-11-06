package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pos/internal/catalog"
	"pos/internal/shift"
	"pos/internal/store"
)

func newService(t *testing.T) *Service {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(db, catalog.NewService(db))
}

func TestCreateComputesMoneyFromDatabase(t *testing.T) {
	svc := newService(t)
	o, err := svc.Create(CreateInput{
		Cash: 50,
		Items: []ItemInput{
			{ProductID: 1, Qty: 2}, // 珍珠奶茶 13.00
			{ProductID: 12, Qty: 1}, // 蛋挞 6.00
		},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if o.Total != 3200 {
		t.Fatalf("合计应为 3200 分，实际 %d", o.Total)
	}
	if o.Change != 1800 {
		t.Fatalf("找零应为 1800 分，实际 %d", o.Change)
	}
	if !strings.HasPrefix(o.OrderNo, "L") {
		t.Fatalf("单号格式不对: %s", o.OrderNo)
	}
	if len(o.Items) != 2 || o.Items[0].Name != "珍珠奶茶" {
		t.Fatalf("明细不符: %+v", o.Items)
	}
}

func TestIdempotentByKey(t *testing.T) {
	svc := newService(t)
	in := CreateInput{Cash: 20, IdempotencyKey: "k-1", Items: []ItemInput{{ProductID: 1, Qty: 1}}}
	first, err := svc.Create(in)
	if err != nil {
		t.Fatalf("首次下单失败: %v", err)
	}
	second, err := svc.Create(in)
	if err != nil {
		t.Fatalf("重复下单失败: %v", err)
	}
	if first.OrderNo != second.OrderNo {
		t.Fatalf("幂等失效：%s vs %s", first.OrderNo, second.OrderNo)
	}
}

// 幂等语义：首次 201，重复提交 200 且订单号相同（契约要求）。
func TestCreateHTTPStatusOnReplay(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	Register(mux, NewService(db, catalog.NewService(db)), Options{})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := `{"idempotencyKey":"idem-http-1","cash":50,"items":[{"productId":"1","qty":1}]}`
	post := func() (int, string) {
		resp, err := http.Post(ts.URL+"/pos/v1/orders", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("请求失败: %v", err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var o Order
		if err := json.Unmarshal(raw, &o); err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		return resp.StatusCode, o.OrderNo
	}

	code1, no1 := post()
	code2, no2 := post()
	if code1 != http.StatusCreated {
		t.Fatalf("首次应 201，实际 %d", code1)
	}
	if code2 != http.StatusOK {
		t.Fatalf("重复提交应 200，实际 %d", code2)
	}
	if no1 != no2 {
		t.Fatalf("重复提交订单号应相同：%s vs %s", no1, no2)
	}
}

// 回归：List 曾在 rows 未关闭时查明细，单连接池下自死锁。
func TestListWithItemsDoesNotDeadlock(t *testing.T) {
	svc := newService(t)
	for i := 0; i < 3; i++ {
		if _, err := svc.Create(CreateInput{Cash: 100, Items: []ItemInput{{ProductID: 1, Qty: 1}}}); err != nil {
			t.Fatalf("下单失败: %v", err)
		}
	}
	done := make(chan struct{})
	var (
		list []Order
		err  error
	)
	go func() {
		list, err = svc.List(10)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("List 超时：疑似单连接自死锁")
	}
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("应返回 3 单，实际 %d", len(list))
	}
	for _, o := range list {
		if len(o.Items) != 1 {
			t.Fatalf("订单 %s 明细缺失", o.OrderNo)
		}
	}
}

func TestRejectsInsufficientCash(t *testing.T) {
	svc := newService(t)
	_, err := svc.Create(CreateInput{Cash: 1, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "实收金额不足") {
		t.Fatalf("应拒绝实收不足，实际 %v", err)
	}
}

func TestRejectsInactiveProduct(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	products := catalog.NewService(db)
	if _, err := db.SQL().Exec("UPDATE t_pos_product SET active=0 WHERE id=1"); err != nil {
		t.Fatalf("下架失败: %v", err)
	}
	svc := NewService(db, products)
	_, err = svc.Create(CreateInput{Cash: 100, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if !strings.Contains(err.Error(), "已下架") {
		t.Fatalf("应拒绝下架商品，实际 %v", err)
	}
}
func TestRefundSetsStatusAndAmount(t *testing.T) {
	svc := newService(t)
	o, err := svc.Create(CreateInput{Cash: 20, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	refunded, err := svc.Refund(o.OrderNo, nil, "顾客不要了", "店长")
	if err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	if refunded.Status != "refunded" {
		t.Fatalf("状态应为 refunded，实际 %s", refunded.Status)
	}
	if refunded.Refunded != o.Payable {
		t.Fatalf("退款金额应为 %d，实际 %d", o.Payable, refunded.Refunded)
	}
	if refunded.RefundedAt == "" {
		t.Fatal("应记录退款时间")
	}
}

func TestRefundTwiceConflicts(t *testing.T) {
	svc := newService(t)
	o, _ := svc.Create(CreateInput{Cash: 20, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if _, err := svc.Refund(o.OrderNo, nil, "", "店长"); err != nil {
		t.Fatalf("首次退款失败: %v", err)
	}
	if _, err := svc.Refund(o.OrderNo, nil, "", "店长"); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复退款应返回 ErrConflict，实际 %v", err)
	}
}

func TestRefundUnknownOrder(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Refund("L-NOT-EXIST", nil, "", "店长"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在订单应返回 ErrNotFound，实际 %v", err)
	}
}
func TestPartialRefundByItem(t *testing.T) {
	svc := newService(t)
	o, err := svc.Create(CreateInput{
		Cash: 50,
		Items: []ItemInput{
			{ProductID: 1, Qty: 2},  // 13.00 x2
			{ProductID: 12, Qty: 1}, // 6.00
		},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}

	// 退 1 件珍珠奶茶：13 元，订单仍为已支付
	part, err := svc.Refund(o.OrderNo, []RefundItem{{ProductID: "1", Qty: 1}}, "退一杯", "店长")
	if err != nil {
		t.Fatalf("部分退款失败: %v", err)
	}
	if part.Status != "paid" {
		t.Fatalf("部分退款后仍应为 paid，实际 %s", part.Status)
	}
	if part.Refunded != 1300 {
		t.Fatalf("已退金额应为 1300，实际 %d", part.Refunded)
	}
	if part.Items[0].RefundedQty != 1 {
		t.Fatalf("明细已退数量应为 1，实际 %d", part.Items[0].RefundedQty)
	}

	// 超退：再退 2 件珍珠奶茶（只剩 1 件）应被拒
	if _, err := svc.Refund(o.OrderNo, []RefundItem{{ProductID: "1", Qty: 2}}, "", "店长"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("超退应返回 ErrBadRequest，实际 %v", err)
	}

	// 退剩余全部 → 订单转为已退款
	rest, err := svc.Refund(o.OrderNo, []RefundItem{
		{ProductID: "1", Qty: 1},
		{ProductID: "12", Qty: 1},
	}, "剩余全退", "店长")
	if err != nil {
		t.Fatalf("退剩余失败: %v", err)
	}
	if rest.Status != "refunded" {
		t.Fatalf("全部退完后状态应为 refunded，实际 %s", rest.Status)
	}
	if rest.Refunded != rest.Payable {
		t.Fatalf("已退金额应等于应收：%d vs %d", rest.Refunded, rest.Payable)
	}
}
func TestPriceOverrideRequiresPermissionAndAudits(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	svc := NewService(db, catalog.NewService(db))

	price := 10.0
	// 无权限：拒绝
	if _, err := svc.Create(CreateInput{Cash: 100, Items: []ItemInput{{ProductID: 1, Qty: 1, Price: &price}}}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("无权限改价应被拒，实际 %v", err)
	}

	// 有权限：改价成功 + 原价留存 + 审计落库
	o, err := svc.Create(CreateInput{
		Cash: 100, AllowPriceOverride: true, Operator: "店长",
		Items: []ItemInput{{ProductID: 1, Qty: 1, Price: &price}},
	})
	if err != nil {
		t.Fatalf("有权限改价失败: %v", err)
	}
	it := o.Items[0]
	if it.Price != 1000 {
		t.Fatalf("成交价应为 1000 分，实际 %d", it.Price)
	}
	if it.OriginalPrice != 1300 {
		t.Fatalf("原价应为 1300 分，实际 %d", it.OriginalPrice)
	}
	if !it.Overridden {
		t.Fatal("应标记为改价")
	}
	if o.Total != 1000 {
		t.Fatalf("合计应按改后价，实际 %d", o.Total)
	}

	var n int
	if err := db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_audit WHERE action='price_override' AND order_no=?", o.OrderNo).Scan(&n); err != nil {
		t.Fatalf("查审计失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("应写入 1 条改价审计，实际 %d", n)
	}
}

func TestPriceUnchangedNeedsNoPermission(t *testing.T) {
	svc := newService(t)
	same := 13.0
	o, err := svc.Create(CreateInput{Cash: 100, Items: []ItemInput{{ProductID: 1, Qty: 1, Price: &same}}})
	if err != nil {
		t.Fatalf("原价下单不应需要权限: %v", err)
	}
	if o.Items[0].Overridden {
		t.Fatal("价格未变不应标记改价")
	}
}
func TestVoidOrder(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	svc := NewService(db, catalog.NewService(db))

	o, err := svc.Create(CreateInput{Cash: 20, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	voided, err := svc.Void(o.OrderNo, "点错了", "店长")
	if err != nil {
		t.Fatalf("作废失败: %v", err)
	}
	if voided.Status != "void" {
		t.Fatalf("状态应为 void，实际 %s", voided.Status)
	}
	if voided.Refunded != o.Payable {
		t.Fatalf("作废应把钱记入退款：%d vs %d", voided.Refunded, o.Payable)
	}
	if voided.VoidReason != "点错了" || voided.VoidedAt == "" {
		t.Fatalf("作废原因/时间缺失: %+v", voided)
	}

	if _, err := svc.Void(o.OrderNo, "", "店长"); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复作废应冲突，实际 %v", err)
	}

	var n int
	if err := db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_audit WHERE action='void' AND order_no=?", o.OrderNo).Scan(&n); err != nil {
		t.Fatalf("查审计失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("应写 1 条作废审计，实际 %d", n)
	}
}

func TestVoidRejectedAfterRefund(t *testing.T) {
	svc := newService(t)
	o, _ := svc.Create(CreateInput{Cash: 50, Items: []ItemInput{{ProductID: 1, Qty: 2}}})
	if _, err := svc.Refund(o.OrderNo, []RefundItem{{ProductID: "1", Qty: 1}}, "退一杯", "店长"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	if _, err := svc.Void(o.OrderNo, "", "店长"); !errors.Is(err, ErrConflict) {
		t.Fatalf("退过款的订单不应允许作废，实际 %v", err)
	}
}

func TestScanPayRecordsMethod(t *testing.T) {
	svc := newService(t)
	o, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "wechat", Amount: 13}},
		Items:    []ItemInput{{ProductID: 1, Qty: 1}},
	})
	if err != nil {
		t.Fatalf("扫码记账下单失败: %v", err)
	}
	if len(o.Payments) != 1 || o.Payments[0].Method != "wechat" || o.Payments[0].Amount != 1300 {
		t.Fatalf("支付记录不符: %+v", o.Payments)
	}
	if o.Change != 0 {
		t.Fatalf("扫码不应有找零，实际 %d", o.Change)
	}
}

func TestPaymentsMustMatchPayable(t *testing.T) {
	svc := newService(t)
	_, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "alipay", Amount: 10}},
		Items:    []ItemInput{{ProductID: 1, Qty: 1}},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("金额不一致应被拒，实际 %v", err)
	}
	_, err = svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "bitcoin", Amount: 13}},
		Items:    []ItemInput{{ProductID: 1, Qty: 1}},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("未知支付方式应被拒，实际 %v", err)
	}
}

func TestMixedPayment(t *testing.T) {
	svc := newService(t)
	o, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "cash", Amount: 5}, {Method: "wechat", Amount: 8}},
		Cash:     10,
		Items:    []ItemInput{{ProductID: 1, Qty: 1}},
	})
	if err != nil {
		t.Fatalf("组合支付失败: %v", err)
	}
	if len(o.Payments) != 2 {
		t.Fatalf("应有两笔支付，实际 %d", len(o.Payments))
	}
	if o.Change != 500 {
		t.Fatalf("找零应为 5 元，实际 %d", o.Change)
	}
}

func TestHoldThenPay(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	svc := NewService(db, catalog.NewService(db))

	printed := 0
	svc.OnCreated(func(Order) { printed++ })

	held, err := svc.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 2}}})
	if err != nil {
		t.Fatalf("挂单失败: %v", err)
	}
	if held.Status != "held" {
		t.Fatalf("挂单状态应为 held，实际 %s", held.Status)
	}
	if len(held.Payments) != 0 {
		t.Fatalf("挂单不应有支付记录: %+v", held.Payments)
	}
	if printed != 0 {
		t.Fatalf("挂单不应出票，实际打印 %d 次", printed)
	}

	paid, err := svc.Pay(held.OrderNo, []PaymentInput{{Method: "wechat", Amount: 26}}, 0)
	if err != nil {
		t.Fatalf("挂单收款失败: %v", err)
	}
	if paid.Status != "paid" {
		t.Fatalf("收款后状态应为 paid，实际 %s", paid.Status)
	}
	if len(paid.Payments) != 1 || paid.Payments[0].Method != "wechat" {
		t.Fatalf("支付记录不符: %+v", paid.Payments)
	}
	if printed != 1 {
		t.Fatalf("收款后应出票一次，实际 %d", printed)
	}

	if _, err := svc.Pay(held.OrderNo, nil, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复收款应冲突，实际 %v", err)
	}
}

func TestHeldOrdersExcludedFromShiftSales(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	shifts := shift.NewService(db)
	orders := NewService(db, catalog.NewService(db))

	sh, err := shifts.Open(100, "店长")
	if err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	// 一单已付、一单挂单
	if _, err := orders.Create(CreateInput{Cash: 20, Items: []ItemInput{{ProductID: 1, Qty: 1}}}); err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if _, err := orders.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 3}}}); err != nil {
		t.Fatalf("挂单失败: %v", err)
	}
	cur, _, err := shifts.Latest()
	if err != nil {
		t.Fatalf("查班次失败: %v", err)
	}
	if cur.OrderCount != 1 || cur.Total != 1300 {
		t.Fatalf("挂单不应计入班次：count=%d total=%d", cur.OrderCount, cur.Total)
	}
	closed, err := shifts.Close(sh.ID, 113, "")
	if err != nil {
		t.Fatalf("结班失败: %v", err)
	}
	if closed.Expected != 11300 {
		t.Fatalf("现金应收应为 11300，实际 %d", closed.Expected)
	}
}

func TestRoundOff(t *testing.T) {
	svc := newService(t)
	cases := []struct {
		mode   string
		pay    int64
		round  int64
	}{
		{"none", 1387, 0},
		{"jiao", 1387, 7},   // 13.87 -> 13.80
		{"jiao", 1380, 0},   // 已是整角
		{"yuan", 1387, 87},  // 13.87 -> 13.00
		{"yuan", 1300, 0},
	}
	for _, c := range cases {
		if got := roundOffCents(c.pay, c.mode); got != c.round {
			t.Fatalf("%s 抹零 %d 分应为 %d，实际 %d", c.mode, c.pay, c.round, got)
		}
	}

	// 端到端：改价到 13.87 → 抹分后应收 13.80
	price := 13.87
	o, err := svc.Create(CreateInput{
		AllowPriceOverride: true, Operator: "店长", RoundMode: "jiao",
		Cash:  20,
		Items: []ItemInput{{ProductID: 1, Qty: 1, Price: &price}},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if o.Total != 1387 {
		t.Fatalf("合计应为 1387，实际 %d", o.Total)
	}
	if o.RoundOff != 7 {
		t.Fatalf("抹零应为 7 分，实际 %d", o.RoundOff)
	}
	if o.Payable != 1380 {
		t.Fatalf("应收应为 1380，实际 %d", o.Payable)
	}
	if o.Change != 620 {
		t.Fatalf("找零应为 620，实际 %d", o.Change)
	}
}

func TestAddItemsToHeldOrder(t *testing.T) {
	svc := newService(t)
	o, err := svc.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("挂单失败: %v", err)
	}
	if o.Payable != 1300 {
		t.Fatalf("初始应收应为 1300，实际 %d", o.Payable)
	}
	got, err := svc.AddItems(o.OrderNo, []ItemInput{{ProductID: 1, Qty: 2}})
	if err != nil {
		t.Fatalf("加菜失败: %v", err)
	}
	if got.Payable != 3900 {
		t.Fatalf("加菜后应收应为 3900，实际 %d", got.Payable)
	}
	if len(got.Items) != 2 {
		t.Fatalf("应有 2 行明细，实际 %d", len(got.Items))
	}
	// 已付款的单不能加菜
	paid, _ := svc.Create(CreateInput{Cash: 20, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if _, err := svc.AddItems(paid.OrderNo, []ItemInput{{ProductID: 1, Qty: 1}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("已付款单加菜应冲突，实际 %v", err)
	}
}

func TestCrossDayHeldPay(t *testing.T) {
	svc := newService(t)
	o, err := svc.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("挂单失败: %v", err)
	}
	today := o.BizDate
	// 把挂单改成"昨天"的
	if _, err := svc.db.SQL().Exec("UPDATE t_pos_order SET biz_date='20260101' WHERE order_no=?", o.OrderNo); err != nil {
		t.Fatalf("改日期失败: %v", err)
	}

	paid, err := svc.Pay(o.OrderNo, []PaymentInput{{Method: "cash", Amount: 13}}, 20)
	if err != nil {
		t.Fatalf("跨天结账失败: %v", err)
	}
	if paid.BizDate == "20260101" {
		t.Fatal("结账后营业日应改为今天")
	}
	if paid.CrossDayFrom != "20260101" {
		t.Fatalf("应记录原营业日，实际 %q", paid.CrossDayFrom)
	}
	var n int
	_ = svc.db.SQL().QueryRow(
		"SELECT COUNT(*) FROM t_pos_audit WHERE action='cross_day_pay' AND order_no=?", o.OrderNo).Scan(&n)
	if n != 1 {
		t.Fatalf("应写一条 cross_day_pay 审计，实际 %d", n)
	}
	if paid.BizDate != today {
		t.Logf("今天营业日变化（跨过 4 点）：%s -> %s", today, paid.BizDate)
	}
}

func TestAutoCancelHeld(t *testing.T) {
	svc := newService(t)
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.Local)

	// 31 分钟前挂的单 → 应自动取消
	old, err := svc.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 2}}})
	if err != nil {
		t.Fatalf("挂单失败: %v", err)
	}
	if _, err := svc.db.SQL().Exec(
		"UPDATE t_pos_order SET held_at=? WHERE order_no=?",
		now.Add(-31*time.Minute).Format("2006-01-02 15:04:05"), old.OrderNo); err != nil {
		t.Fatalf("拨老挂单时间失败: %v", err)
	}
	// 刚挂的单 → 不应取消
	fresh, err := svc.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("挂单失败: %v", err)
	}

	n, err := svc.AutoCancelHeld(now, 30*time.Minute)
	if err != nil {
		t.Fatalf("自动取消失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("应取消 1 张超时挂单，实际 %d", n)
	}
	got, err := svc.Get(old.OrderNo)
	if err != nil {
		t.Fatalf("读取取消订单失败: %v", err)
	}
	if got.Status != "cancelled" {
		t.Fatalf("超时挂单应变为 cancelled，实际 %s", got.Status)
	}
	if got.VoidReason != "挂单超时自动取消" || got.VoidOperator != "system" {
		t.Fatalf("取消原因/操作者不对: %q / %q", got.VoidReason, got.VoidOperator)
	}
	// 挂单从未收款：不得写退款流水（否则班次对账会把它当成钱进出）。
	var refunds int
	if err := svc.db.SQL().QueryRow(
		"SELECT COUNT(*) FROM t_pos_refund WHERE order_no=?", old.OrderNo).Scan(&refunds); err != nil {
		t.Fatalf("查退款失败: %v", err)
	}
	if refunds != 0 {
		t.Fatalf("自动取消不应写退款流水，实际 %d", refunds)
	}
	var audits int
	if err := svc.db.SQL().QueryRow(
		"SELECT COUNT(*) FROM t_pos_audit WHERE action='cancel' AND order_no=?", old.OrderNo).Scan(&audits); err != nil {
		t.Fatalf("查审计失败: %v", err)
	}
	if audits != 1 {
		t.Fatalf("应写 1 条 cancel 审计，实际 %d", audits)
	}
	// 刚挂的单保持 held
	freshGot, err := svc.Get(fresh.OrderNo)
	if err != nil {
		t.Fatalf("读取挂单失败: %v", err)
	}
	if freshGot.Status != "held" {
		t.Fatalf("未超时挂单应保持 held，实际 %s", freshGot.Status)
	}
	// 重复清扫幂等：二次调用不再取消
	n2, err := svc.AutoCancelHeld(now, 30*time.Minute)
	if err != nil || n2 != 0 {
		t.Fatalf("二次清扫应取消 0 张，实际 %d err=%v", n2, err)
	}
}

func TestAutoCancelHeldSkipsPaid(t *testing.T) {
	svc := newService(t)
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.Local)

	paid, err := svc.Create(CreateInput{Cash: 20, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	// 已支付订单即使 held_at 很久（历史异常数据）也不应被自动取消。
	if _, err := svc.db.SQL().Exec(
		"UPDATE t_pos_order SET held_at=? WHERE order_no=?",
		now.Add(-2*time.Hour).Format("2006-01-02 15:04:05"), paid.OrderNo); err != nil {
		t.Fatalf("拨 held_at 失败: %v", err)
	}
	n, err := svc.AutoCancelHeld(now, 30*time.Minute)
	if err != nil {
		t.Fatalf("自动取消失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("已支付订单不应被取消，实际 %d", n)
	}
	got, err := svc.Get(paid.OrderNo)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if got.Status != "paid" {
		t.Fatalf("已支付订单状态不应变化，实际 %s", got.Status)
	}
}

func TestAutoCancelSkipsTableHeld(t *testing.T) {
	svc := newService(t)
	o, err := svc.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("挂单失败: %v", err)
	}
	if err := svc.SetTable(o.OrderNo, "A1"); err != nil {
		t.Fatalf("写桌台失败: %v", err)
	}
	// 把挂单时间推到两小时前
	if _, err := svc.db.SQL().Exec(
		"UPDATE t_pos_order SET held_at=datetime('now','localtime','-2 hours') WHERE order_no=?", o.OrderNo); err != nil {
		t.Fatalf("改时间失败: %v", err)
	}

	n, err := svc.AutoCancelHeld(time.Now(), 30*time.Minute)
	if err != nil {
		t.Fatalf("清扫失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("桌台挂单不该被自动取消，实际取消 %d 单", n)
	}
	got, _ := svc.Get(o.OrderNo)
	if got.Status != "held" {
		t.Fatalf("桌台挂单应保持 held，实际 %s", got.Status)
	}
}

func TestAutoCancelOnlyTakeawayAndRespectsSetting(t *testing.T) {
	svc := newService(t)
	o, err := svc.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("挂单失败: %v", err)
	}
	if _, err := svc.db.SQL().Exec(
		"UPDATE t_pos_order SET held_at=datetime('now','localtime','-2 hours') WHERE order_no=?", o.OrderNo); err != nil {
		t.Fatalf("改时间失败: %v", err)
	}

	// 0 分钟 = 关闭自动取消
	if n, _ := svc.AutoCancelHeld(time.Now(), 0); n != 0 {
		t.Fatalf("关闭自动取消后不应取消任何单，实际 %d", n)
	}
	// 240 分钟 > 已挂 120 分钟 → 不该取消
	if n, _ := svc.AutoCancelHeld(time.Now(), 240*time.Minute); n != 0 {
		t.Fatalf("未到超时不应取消，实际 %d", n)
	}
	// 30 分钟 → 应取消
	n, err := svc.AutoCancelHeld(time.Now(), 30*time.Minute)
	if err != nil {
		t.Fatalf("清扫失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("外带挂单超时应取消 1 单，实际 %d", n)
	}
	got, _ := svc.Get(o.OrderNo)
	if got.Status != "cancelled" {
		t.Fatalf("应变为 cancelled，实际 %s", got.Status)
	}

	// 已取消的单不能结算
	if _, err := svc.Pay(o.OrderNo, []PaymentInput{{Method: "cash", Amount: 13}}, 20); err == nil {
		t.Fatal("已取消的挂单不该能收款")
	}
}

func TestBundleOrderSnapshotsComponents(t *testing.T) {
	svc := newService(t)
	drink, _ := svc.products.Create(catalog.SaveInput{Name: "珍珠奶茶", Category: "奶茶", Price: 13})
	cake, _ := svc.products.Create(catalog.SaveInput{Name: "蛋挞", Category: "甜点", Price: 6})
	bundle, err := svc.products.Create(catalog.SaveInput{
		Name: "超值套餐", Category: "套餐", Price: 20, Kind: "bundle",
		Bundle: []catalog.BundleItem{{ComponentID: drink.ID, Qty: 1}, {ComponentID: cake.ID, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("建套餐失败: %v", err)
	}
	if bundle.Kind != "bundle" || len(bundle.Bundle) != 2 {
		t.Fatalf("套餐应保存 2 个组件，实际 %+v", bundle)
	}

	o, err := svc.Create(CreateInput{Cash: 50, Items: []ItemInput{{ProductID: bundle.ID, Qty: 2}}})
	if err != nil {
		t.Fatalf("套餐下单失败: %v", err)
	}
	if o.Payable != 4000 {
		t.Fatalf("两套应收 4000 分（套餐价 20×2），实际 %d", o.Payable)
	}
	if len(o.Items) != 1 || len(o.Items[0].Components) != 2 {
		t.Fatalf("明细应有 1 行、2 个组件快照，实际 %+v", o.Items)
	}
	if o.Items[0].Components[1].Qty != 2 || o.Items[0].Components[1].Category != "甜点" {
		t.Fatalf("组件快照应含数量与类目，实际 %+v", o.Items[0].Components[1])
	}

	// 重新读订单也要能还原组件（重打小票/后厨补打依赖它）
	got, _ := svc.Get(o.OrderNo)
	if len(got.Items[0].Components) != 2 {
		t.Fatalf("Get 应还原组件快照，实际 %+v", got.Items[0].Components)
	}

	// 退整套：按明细退，金额等于套餐价×数量
	if _, err := svc.Refund(o.OrderNo, []RefundItem{{ProductID: fmt.Sprint(bundle.ID), Qty: 2}}, "退套餐", "店长"); err != nil {
		t.Fatalf("退套餐失败: %v", err)
	}
}

func TestBundleOptionChoice(t *testing.T) {
	svc := newService(t)
	tea, _ := svc.products.Create(catalog.SaveInput{Name: "珍珠奶茶", Category: "奶茶", Price: 13})
	fruit, _ := svc.products.Create(catalog.SaveInput{Name: "果茶", Category: "果茶", Price: 15})
	cake, _ := svc.products.Create(catalog.SaveInput{Name: "蛋挞", Category: "甜点", Price: 6})

	bundle, err := svc.products.Create(catalog.SaveInput{
		Name: "自选套餐", Category: "套餐", Price: 20, Kind: "bundle",
		Bundle: []catalog.BundleItem{
			{ComponentID: tea.ID, Qty: 1, GroupID: 1},
			{ComponentID: fruit.ID, Qty: 1, GroupID: 1},
			{ComponentID: cake.ID, Qty: 1},
		},
	})
	if err != nil {
		t.Fatalf("建套餐失败: %v", err)
	}

	// 指定选果茶
	o, err := svc.Create(CreateInput{
		Cash: 50,
		Items: []ItemInput{{
			ProductID: bundle.ID, Qty: 1,
			Choices: []Choice{{GroupID: 1, ComponentID: fruit.ID}},
		}},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	names := map[string]bool{}
	for _, c := range o.Items[0].Components {
		names[c.Name] = true
	}
	if !names["果茶"] || names["珍珠奶茶"] {
		t.Fatalf("应选果茶且不含珍珠奶茶，实际 %+v", o.Items[0].Components)
	}
	if !names["蛋挞"] {
		t.Fatal("固定组件应保留")
	}

	// 不指定：默认取组内第一个（珍珠奶茶）
	o2, err := svc.Create(CreateInput{Cash: 50, Items: []ItemInput{{ProductID: bundle.ID, Qty: 1}}})
	if err != nil {
		t.Fatalf("默认下单失败: %v", err)
	}
	names2 := map[string]bool{}
	for _, c := range o2.Items[0].Components {
		names2[c.Name] = true
	}
	if !names2["珍珠奶茶"] || names2["果茶"] {
		t.Fatalf("默认应取珍珠奶茶，实际 %+v", o2.Items[0].Components)
	}
}
