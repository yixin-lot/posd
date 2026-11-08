package escpos

import (
	"strings"
	"testing"
)

// 英语小票：标签英文、货币前缀、日期由 main 预处理（这里只验渲染）。
func TestRenderOrderEnglish(t *testing.T) {
	b := RenderOrder(Shop{Name: "Old Street Cafe", Phone: "555-0100"}, Order{
		Lang: "en-US", Currency: "$",
		OrderNo: "L20261106-0001", CreatedAt: "06 Nov 2026 21:30", Table: "A1",
		Items: []Item{{Name: "Milk Tea", Price: "4.50", Qty: 2, Amount: "9.00"}},
		Total: "9.00", Cash: "10.00", Change: "1.00",
		Discount: "0.00", MemberDiscount: "0.00", RoundOff: "0.00", Points: "0",
		Payments: []Pay{{Name: "Cash", Amount: "10.00"}},
		Footer:   "Thanks!",
	})
	out := decode(t, b)
	for _, want := range []string{"Order", "Table", "Time", "Total", "$9.00", "Cash", "$10.00", "Change", "$1.00", "06 Nov 2026 21:30"} {
		if !strings.Contains(out, want) {
			t.Fatalf("英语小票缺少 %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "合计") {
		t.Fatal("英语小票不应出现中文标签")
	}
}
