package order

import (
	"errors"
	"path/filepath"
	"testing"

	"pos/internal/catalog"
	"pos/internal/member"
	"pos/internal/store"
)

func newSvcWithMember(t *testing.T) (*Service, *member.Service, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mem := member.NewService(db)
	svc := NewService(db, catalog.NewService(db))
	svc.SetMembers(mem)
	return svc, mem, db
}

func TestMemberBalancePayment(t *testing.T) {
	svc, mem, db := newSvcWithMember(t)
	m, err := mem.Create("13800000077", "会员甲")
	if err != nil {
		t.Fatalf("开卡失败: %v", err)
	}
	if _, err := mem.Topup(m.ID, 5000, "充值"); err != nil {
		t.Fatalf("充值失败: %v", err)
	}

	o, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "member", Amount: 26, MemberID: m.ID}},
		Items:    []ItemInput{{ProductID: 1, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("余额支付失败: %v", err)
	}
	if o.Payable != 2600 {
		t.Fatalf("应收应为 2600，实际 %d", o.Payable)
	}

	got, _ := mem.Get(m.ID)
	if got.Balance != 2400 {
		t.Fatalf("余额应扣到 2400，实际 %d", got.Balance)
	}
	list, _ := mem.Txns(m.ID, 10)
	var sawConsume bool
	for _, tx := range list {
		if tx.Kind == "consume" && tx.Amount == 2600 {
			sawConsume = true
		}
	}
	if !sawConsume {
		t.Fatalf("应有 2600 的消费流水: %+v", list)
	}

	var method string
	var memberID int64
	if err := db.SQL().QueryRow(
		"SELECT method, member_id FROM t_pos_payment WHERE order_id=?", o.ID).
		Scan(&method, &memberID); err != nil {
		t.Fatalf("查支付记录失败: %v", err)
	}
	if method != "member" || memberID != m.ID {
		t.Fatalf("支付记录不符: %s %d", method, memberID)
	}
}

func TestMemberBalanceInsufficientRollsBack(t *testing.T) {
	svc, mem, _ := newSvcWithMember(t)
	m, _ := mem.Create("13800000078", "会员乙")

	if _, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "member", Amount: 26, MemberID: m.ID}},
		Items:    []ItemInput{{ProductID: 1, Qty: 2}},
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("余额不足应报业务冲突，实际 %v", err)
	}

	got, _ := mem.Get(m.ID)
	if got.Balance != 0 {
		t.Fatalf("失败订单不应扣余额，实际 %d", got.Balance)
	}
	orders, _ := svc.List(10)
	if len(orders) != 0 {
		t.Fatalf("失败订单不应落库，实际 %d 单", len(orders))
	}
}

func TestMemberPaymentRequiresMemberID(t *testing.T) {
	svc, mem, _ := newSvcWithMember(t)
	if _, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "member", Amount: 13}},
		Items:    []ItemInput{{ProductID: 1, Qty: 1}},
	}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("缺会员应报错，实际 %v", err)
	}
	_ = mem
}

func TestMemberRefundGoesBackToBalance(t *testing.T) {
	svc, mem, db := newSvcWithMember(t)
	m, _ := mem.Create("13800000088", "会员丙")
	_, _ = mem.Topup(m.ID, 5000, "充值")

	o, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "member", Amount: 26, MemberID: m.ID}},
		Items:    []ItemInput{{ProductID: 1, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("余额支付失败: %v", err)
	}
	if got, _ := mem.Get(m.ID); got.Balance != 2400 {
		t.Fatalf("支付后余额应为 2400，实际 %d", got.Balance)
	}

	// 部分退款：退一件（13 元）→ 回到余额
	if _, err := svc.Refund(o.OrderNo, []RefundItem{{ProductID: "1", Qty: 1}}, "退一件", "店长"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	if got, _ := mem.Get(m.ID); got.Balance != 3700 {
		t.Fatalf("部分退款后余额应为 3700，实际 %d", got.Balance)
	}

	var method string
	var memberID int64
	if err := db.SQL().QueryRow(
		"SELECT method, member_id FROM t_pos_refund WHERE order_no=? ORDER BY id DESC LIMIT 1", o.OrderNo).
		Scan(&method, &memberID); err != nil {
		t.Fatalf("查退款记录失败: %v", err)
	}
	if method != "member" || memberID != m.ID {
		t.Fatalf("退款记录应标记 member：%s %d", method, memberID)
	}

	// 整单退完：余额回到 5000
	if _, err := svc.Refund(o.OrderNo, nil, "全退", "店长"); err != nil {
		t.Fatalf("整单退款失败: %v", err)
	}
	if got, _ := mem.Get(m.ID); got.Balance != 5000 {
		t.Fatalf("全退后余额应回到 5000，实际 %d", got.Balance)
	}
}

func TestCashOrderRefundStaysCash(t *testing.T) {
	svc, mem, db := newSvcWithMember(t)
	o, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "cash", Amount: 13}},
		Cash:     20,
		Items:    []ItemInput{{ProductID: 1, Qty: 1}},
	})
	if err != nil {
		t.Fatalf("现金下单失败: %v", err)
	}
	if _, err := svc.Refund(o.OrderNo, nil, "退", "店长"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	var method string
	if err := db.SQL().QueryRow(
		"SELECT method FROM t_pos_refund WHERE order_no=? ORDER BY id DESC LIMIT 1", o.OrderNo).
		Scan(&method); err != nil {
		t.Fatalf("查退款记录失败: %v", err)
	}
	if method != "cash" {
		t.Fatalf("现金单退款应记 cash，实际 %s", method)
	}
	_ = mem
}

func TestMemberPointsAccrue(t *testing.T) {
	svc, mem, _ := newSvcWithMember(t)
	m, _ := mem.Create("13800000099", "会员丁")
	_, _ = mem.Topup(m.ID, 5000, "充值")

	if _, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "member", Amount: 26, MemberID: m.ID}},
		Items:    []ItemInput{{ProductID: 1, Qty: 2}},
	}); err != nil {
		t.Fatalf("余额支付失败: %v", err)
	}
	got, _ := mem.Get(m.ID)
	if got.Points != 26 {
		t.Fatalf("26 元应积 26 分，实际 %d", got.Points)
	}
}

func TestPointsPaymentAndRefund(t *testing.T) {
	svc, mem, db := newSvcWithMember(t)
	m, _ := mem.Create("13800000100", "会员戊")
	// 先充值再消费攒积分：26 元 → 26 分，不够抵扣；这里直接造 5000 分
	if _, err := db.SQL().Exec("UPDATE t_pos_member SET points=5000 WHERE id=?", m.ID); err != nil {
		t.Fatalf("造积分失败: %v", err)
	}

	o, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "points", Amount: 26, MemberID: m.ID}},
		Items:    []ItemInput{{ProductID: 1, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("积分支付失败: %v", err)
	}
	got, _ := mem.Get(m.ID)
	if got.Points != 2400 {
		t.Fatalf("5000-2600 应剩 2400 分，实际 %d", got.Points)
	}

	if _, err := svc.Refund(o.OrderNo, nil, "退", "店长"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	got, _ = mem.Get(m.ID)
	if got.Points != 5000 {
		t.Fatalf("退款后积分应回到 5000，实际 %d", got.Points)
	}
	var method string
	_ = db.SQL().QueryRow(
		"SELECT method FROM t_pos_refund WHERE order_no=? ORDER BY id DESC LIMIT 1", o.OrderNo).Scan(&method)
	if method != "points" {
		t.Fatalf("退款方式应为 points，实际 %s", method)
	}
}

func TestPointsInsufficient(t *testing.T) {
	svc, mem, _ := newSvcWithMember(t)
	m, _ := mem.Create("13800000101", "会员己")
	if _, err := svc.Create(CreateInput{
		Payments: []PaymentInput{{Method: "points", Amount: 26, MemberID: m.ID}},
		Items:    []ItemInput{{ProductID: 1, Qty: 2}},
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("积分不足应报业务冲突，实际 %v", err)
	}
}

func TestMemberTierDiscount(t *testing.T) {
	svc, mem, _ := newSvcWithMember(t)
	m, _ := mem.Create("13800000301", "银卡测试")

	o, err := svc.Create(CreateInput{
		MemberID:          m.ID,
		MemberDiscountPct: 2,
		MemberPointRate:   2,
		Payments:          []PaymentInput{{Method: "cash", Amount: 25.48}},
		Cash:              30,
		Items:             []ItemInput{{ProductID: 1, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("会员价下单失败: %v", err)
	}
	if o.MemberDiscount != 52 {
		t.Fatalf("26 元 2%% 折扣应为 52 分，实际 %d", o.MemberDiscount)
	}
	if o.Payable != 2548 {
		t.Fatalf("应收应为 2548，实际 %d", o.Payable)
	}
}

func TestHeldPayUsesSameMemberRules(t *testing.T) {
	svc, mem, _ := newSvcWithMember(t)
	m, _ := mem.Create("13800000950", "挂单会员")
	if _, err := mem.Topup(m.ID, 10000, "充值"); err != nil {
		t.Fatalf("充值失败: %v", err)
	}

	// 挂单：2 杯 = 26 元
	o, err := svc.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 2}}})
	if err != nil {
		t.Fatalf("挂单失败: %v", err)
	}

	// 收款时用「2 倍积分 + 2 个月有效期」的规则
	if _, err := svc.PayWith(o.OrderNo, []PaymentInput{{Method: "member", Amount: 26, MemberID: m.ID}}, 30,
		MemberRules{PointsPerYuan: 100, PointsEarnPerYuan: 1, PointRate: 2, ExpireMonths: 2}); err != nil {
		t.Fatalf("挂单收款失败: %v", err)
	}

	got, _ := mem.Get(m.ID)
	if got.Points != 52 {
		t.Fatalf("26 元 × 2 倍应得 52 分，实际 %d", got.Points)
	}
	if got.PointsExpireAt == "" {
		t.Fatal("设置了有效期就应写入到期日")
	}
	paid, _ := svc.Get(o.OrderNo)
	if paid.PointsEarned != 52 {
		t.Fatalf("订单应记录本单积分 52，实际 %d", paid.PointsEarned)
	}
}

func TestHeldPayPointsRedemptionUsesRate(t *testing.T) {
	svc, mem, db := newSvcWithMember(t)
	m, _ := mem.Create("13800000951", "挂单积分")
	if _, err := db.SQL().Exec("UPDATE t_pos_member SET points=10000 WHERE id=?", m.ID); err != nil {
		t.Fatalf("造积分失败: %v", err)
	}
	o, err := svc.Create(CreateInput{Hold: true, Items: []ItemInput{{ProductID: 1, Qty: 1}}})
	if err != nil {
		t.Fatalf("挂单失败: %v", err)
	}
	// 200 分抵 1 元：13 元应收 → 2600 分
	if _, err := svc.PayWith(o.OrderNo, []PaymentInput{{Method: "points", Amount: 13, MemberID: m.ID}}, 0,
		MemberRules{PointsPerYuan: 200}); err != nil {
		t.Fatalf("积分收款失败: %v", err)
	}
	got, _ := mem.Get(m.ID)
	if got.Points != 7400 {
		t.Fatalf("10000-2600 应剩 7400，实际 %d", got.Points)
	}
}
