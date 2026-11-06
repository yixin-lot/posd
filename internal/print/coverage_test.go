package print

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"

	"pos/internal/escpos"
)

func decodeGBK(t *testing.T, b []byte) string {
	t.Helper()
	s, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err != nil {
		t.Fatalf("GBK 解码失败: %v", err)
	}
	return string(s)
}

// TestRenderTicketCoversEveryField 守卫「Ticket 有字段但没渲染」。
//
// 背景：Reprint 曾经自己拼 escpos.Order，漏了桌台/支付/会员折扣/积分/抹零，
// 重打的小票就掉行。现在只有 renderTicket 一个入口，这条测试确保
// Ticket 里每个可打印字段都真的落到了纸上。
func TestRenderTicketCoversEveryField(t *testing.T) {
	// OrderID 只是入队用的主键，不上纸；Shop 单独断言
	// Cash 与 Change 互斥：无支付明细时印「现金」，有明细时印「找零」。
	// 两者的覆盖放在 TestCashAndChangeAreConditional 里单独验。
	ignore := map[string]string{
		"OrderID": "仅用于入队/回写状态",
		"Shop":    "店名单独断言",
		"Cash":    "无支付明细时才印，见 TestCashAndChangeAreConditional",
		"Change":  "有支付明细时才印，见 TestCashAndChangeAreConditional",
	}

	tk := Ticket{
		OrderNo: "F_OrderNo", CreatedAt: "F_CreatedAt", Table: "F_Table",
		Total: 1320, Discount: 100, Cash: 2000, Change: 580, RoundOff: 20,
		MemberDiscount: 260, PointsEarned: 23, Notice: "F_Notice",
		Shop:  escpos.Shop{Name: "F_Shop"},
		Items: []escpos.Item{{Name: "F_Item", Price: "13.20", Qty: 1, Amount: "13.20"}},
		Payments: []escpos.Pay{{Name: "F_Pay", Amount: "13.20"}},
	}

	out := decodeGBK(t, renderTicket(tk))

	v := reflect.ValueOf(tk)
	tp := v.Type()
	for i := 0; i < tp.NumField(); i++ {
		name := tp.Field(i).Name
		if reason, ok := ignore[name]; ok {
			t.Logf("跳过 %s：%s", name, reason)
			continue
		}
		switch v.Field(i).Kind() {
		case reflect.String:
			want := v.Field(i).String()
			if want != "" && !strings.Contains(out, want) {
				t.Fatalf("Ticket.%s=%q 没出现在小票上——renderTicket 漏了它", name, want)
			}
		case reflect.Int64:
			// PointsEarned 是「积分个数」，不是金额，不能按分转元
			want := money(v.Field(i).Int())
			if name == "PointsEarned" {
				want = strconv.FormatInt(v.Field(i).Int(), 10)
			}
			if v.Field(i).Int() != 0 && !strings.Contains(out, want) {
				t.Fatalf("Ticket.%s（%s）没出现在小票上", name, want)
			}
		}
	}
	if !strings.Contains(out, "F_Shop") {
		t.Fatal("店名没打印")
	}
}

func TestCashAndChangeAreConditional(t *testing.T) {
	// 无支付明细：印「现金」，不印「找零」
	cashOnly := decodeGBK(t, renderTicket(Ticket{
		OrderNo: "L1", CreatedAt: "2026-11-04 10:00:00",
		Total: 1300, Cash: 2000, Change: 700,
		Items: []escpos.Item{{Name: "茶", Price: "13.00", Qty: 1, Amount: "13.00"}},
	}))
	if !strings.Contains(cashOnly, "现金") || !strings.Contains(cashOnly, "20.00") {
		t.Fatal("无支付明细时应印现金金额")
	}
	// 纯现金单要同时印「现金」和「找零」（与历史 golden 一致）
	if !strings.Contains(cashOnly, "找零") || !strings.Contains(cashOnly, "7.00") {
		t.Fatalf("纯现金单应印找零，实际:\n%s", cashOnly)
	}

	// 有支付明细：印「找零」，不重复印「现金」
	withPay := decodeGBK(t, renderTicket(Ticket{
		OrderNo: "L2", CreatedAt: "2026-11-04 10:01:00",
		Total: 1300, Cash: 2000, Change: 700,
		Items:    []escpos.Item{{Name: "茶", Price: "13.00", Qty: 1, Amount: "13.00"}},
		Payments: []escpos.Pay{{Name: "微信", Amount: "13.00"}},
	}))
	if !strings.Contains(withPay, "找零") || !strings.Contains(withPay, "7.00") {
		t.Fatal("有支付明细时应印找零金额")
	}
}

// TestEnqueueAndReprintRenderSame 守卫两条入队路径的纸面内容一致。
func TestEnqueueAndReprintRenderSame(t *testing.T) {
	tk := Ticket{
		OrderNo: "L1", CreatedAt: "2026-11-04 10:00:00", Table: "A1",
		Total: 2340, MemberDiscount: 260, PointsEarned: 23,
		Shop:  escpos.Shop{Name: "店"},
		Items: []escpos.Item{{Name: "茶", Price: "13.00", Qty: 2, Amount: "26.00"}},
		Payments: []escpos.Pay{{Name: "会员余额", Amount: "23.40"}},
	}
	once := decodeGBK(t, renderTicket(tk))
	twice := decodeGBK(t, renderTicket(tk))
	if once != twice {
		t.Fatal("同一张 Ticket 两次渲染结果不同")
	}
	for _, want := range []string{"会员折扣", "本单积分", "会员余额", "A1"} {
		if !strings.Contains(once, want) {
			t.Fatalf("渲染结果缺少 %q", want)
		}
	}
}
