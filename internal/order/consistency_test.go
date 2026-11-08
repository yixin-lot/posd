package order

import (
	"testing"

	"pos/internal/shift"
)

// 这组测试守的是「两条读取路径必须一致」：
// Create 返回的对象、Get 读出来的、List 里那条，字段必须相同。
// 之前 Get 少 SELECT 了 member_discount 与 shift_id，
// 导致退款按原价算、跨班次统计为 0 —— 而单看 Create 的返回值发现不了。
func TestOrderReadPathsAgree(t *testing.T) {
	svc, mem, db := newSvcWithMember(t)
	shifts := shift.NewService(db)

	if _, err := shifts.Open(100, "店长"); err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	m, _ := mem.Create("13800000901", "读路径")
	if _, err := mem.Topup(m.ID, 10000, "充值"); err != nil {
		t.Fatalf("充值失败: %v", err)
	}

	created, err := svc.Create(CreateInput{
		MemberID:          m.ID,
		MemberDiscountPct: 10, // 打 9 折
		MemberPointRate:   1,
		Payments:          []PaymentInput{{Method: "member", Amount: 2340, MemberID: m.ID}},
		Cash:              3000,
		Items:             []ItemInput{{ProductID: 1, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if err := svc.SetTable(created.OrderNo, "A1"); err != nil {
		t.Fatalf("写桌台失败: %v", err)
	}

	got, err := svc.Get(created.OrderNo)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	list, err := svc.List(10)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("List 应有数据")
	}
	fromList := list[0]

	// 关键字段：任何一条读路径丢了都要立刻红
	type check struct {
		name           string
		want, got, list int64
	}
	strChecks := []struct {
		name            string
		want, got, list string
	}{
		{"OrderNo", created.OrderNo, got.OrderNo, fromList.OrderNo},
		{"TableName", "A1", got.TableName, fromList.TableName},
		{"BizDate", created.BizDate, got.BizDate, fromList.BizDate},
	}
	for _, c := range strChecks {
		if c.want != c.got || c.want != c.list {
			t.Fatalf("%s 不一致: create=%q get=%q list=%q", c.name, c.want, c.got, c.list)
		}
	}

	numChecks := []check{
		{"MemberID", m.ID, got.MemberID, fromList.MemberID},
		{"MemberDiscount", created.MemberDiscount, got.MemberDiscount, fromList.MemberDiscount},
		{"PointsEarned", created.PointsEarned, got.PointsEarned, fromList.PointsEarned},
		{"Payable", created.Payable, got.Payable, fromList.Payable},
		{"ShiftID", created.ShiftID, got.ShiftID, fromList.ShiftID},
	}
	for _, c := range numChecks {
		if c.want == 0 {
			t.Fatalf("%s 在 Create 返回值里就是 0，测试数据没造对", c.name)
		}
		if c.want != c.got || c.want != c.list {
			t.Fatalf("%s 不一致: create=%d get=%d list=%d", c.name, c.want, c.got, c.list)
		}
	}
}

// 退款必须按「折后应收」退，不能按原价退。
func TestRefundUsesDiscountedPayable(t *testing.T) {
	svc, mem, db := newSvcWithMember(t)
	if _, err := shift.NewService(db).Open(100, "店长"); err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	m, _ := mem.Create("13800000902", "退款折扣")
	if _, err := mem.Topup(m.ID, 10000, "充值"); err != nil {
		t.Fatalf("充值失败: %v", err)
	}

	o, err := svc.Create(CreateInput{
		MemberID:          m.ID,
		MemberDiscountPct: 10,
		Payments:          []PaymentInput{{Method: "member", Amount: 2340, MemberID: m.ID}},
		Cash:              3000,
		Items:             []ItemInput{{ProductID: 1, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if o.Total != 2600 || o.Payable != 2340 {
		t.Fatalf("原价 2600 打 9 折应为 2340，实际 total=%d payable=%d", o.Total, o.Payable)
	}

	before, _ := mem.Get(m.ID)
	if _, err := svc.Refund(o.OrderNo, nil, "整单退", "店长"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	after, _ := mem.Get(m.ID)
	if after.Balance-before.Balance != 2340 {
		t.Fatalf("应退回折后 2340，实际退回 %d", after.Balance-before.Balance)
	}

	var refunded int64
	if err := db.SQL().QueryRow(
		"SELECT amount FROM t_pos_refund WHERE order_no=? AND kind='refund'", o.OrderNo).Scan(&refunded); err != nil {
		t.Fatalf("查退款记录失败: %v", err)
	}
	if refunded != 2340 {
		t.Fatalf("退款记录应为 2340，实际 %d", refunded)
	}
}

// 作废金额同样按折后应收。
func TestVoidUsesDiscountedPayable(t *testing.T) {
	svc, mem, db := newSvcWithMember(t)
	if _, err := shift.NewService(db).Open(100, "店长"); err != nil {
		t.Fatalf("开班失败: %v", err)
	}
	m, _ := mem.Create("13800000903", "作废折扣")
	o, err := svc.Create(CreateInput{
		MemberID:          m.ID,
		MemberDiscountPct: 10,
		Cash:              3000,
		Items:             []ItemInput{{ProductID: 1, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if _, err := svc.Void(o.OrderNo, "点错单", "店长"); err != nil {
		t.Fatalf("作废失败: %v", err)
	}
	var amt int64
	if err := db.SQL().QueryRow(
		"SELECT amount FROM t_pos_refund WHERE order_no=? AND kind='void'", o.OrderNo).Scan(&amt); err != nil {
		t.Fatalf("查作废记录失败: %v", err)
	}
	if amt != 2340 {
		t.Fatalf("作废金额应为折后 2340，实际 %d", amt)
	}
}
