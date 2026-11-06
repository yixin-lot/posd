package escpos

import (
	"strings"
	"golang.org/x/text/encoding/simplifiedchinese"
	"bytes"
	"fmt"
	"os"
	"testing"
)

// 这两个用例是 ADR-005 P1 的判据：Go 版渲染必须与 Java 版逐字节相同。
// golden 文件由 service/src/test/java/com/pos/print/EscPosFixtureTest 生成。

func TestRenderOrderMatchesJava(t *testing.T) {
	want, err := os.ReadFile("testdata/receipt.bin")
	if err != nil {
		t.Fatalf("读取 golden 失败: %v", err)
	}

	got := RenderOrder(
		Shop{Name: "沪上·真茶铺", Phone: "", Footer: "谢谢光临，欢迎再来"},
		Order{
			OrderNo:   "L20261102-0001",
			CreatedAt: "2026-11-02 14:30:00",
			Items: []Item{
				{Name: "珍珠奶茶", Price: "13.00", Qty: 2, Amount: "26.00"},
				{Name: "蛋挞", Price: "6.00", Qty: 1, Amount: "6.00"},
			},
			Total:    "32.00",
			Discount: "0.00",
			Cash:     "50.00",
			Change:   "18.00",
		})

	assertSameBytes(t, want, got)
}

func TestRenderShiftMatchesJava(t *testing.T) {
	want, err := os.ReadFile("testdata/shift.bin")
	if err != nil {
		t.Fatalf("读取 golden 失败: %v", err)
	}

	got := RenderShift(Shift{
		ShiftNo:     "S20261102-001",
		StaffName:   "陈晓",
		OpenedAt:    "2026-11-02 07:00:00",
		ClosedAt:    "2026-11-02 14:30:00",
		OpeningCash: "200.00",
		Payments:    []Payment{{Method: "cash", Amount: "13.00"}},
		OrderCount:  1,
		Total:       "13.00",
		Expected:    "213.00",
		Actual:      "213.00",
		Difference:  "0.00",
		Note:        "对账无误",
	})

	assertSameBytes(t, want, got)
}

func assertSameBytes(t *testing.T, want, got []byte) {
	t.Helper()
	if bytes.Equal(want, got) {
		return
	}
	n := len(want)
	if len(got) < n {
		n = len(got)
	}
	for i := 0; i < n; i++ {
		if want[i] != got[i] {
			lo := max(0, i-16)
			hi := min(len(want), i+16)
			lo2 := max(0, i-16)
			hi2 := min(len(got), i+16)
			t.Fatalf("第 %d 字节起不一致\nJava: % x\nGo  : % x", i, want[lo:hi], got[lo2:hi2])
		}
	}
	t.Fatalf("长度不一致：Java %d 字节，Go %d 字节\ngot 结尾: %s", len(want), len(got), fmt.Sprintf("% x", got[n:]))
}

func TestRenderVoidContainsKeyFields(t *testing.T) {
	b := RenderVoid(Shop{Name: "楼下奶茶"}, Void{
		OrderNo: "L20261103-0001", CreatedAt: "2026-11-03 07:30:00",
		Amount: "13.00", Reason: "点错单", Operator: "店长",
	})
	if len(b) == 0 {
		t.Fatal("作废小票不应为空")
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	s := string(decoded)
	for _, want := range []string{"作废单", "L20261103-0001", "13.00", "点错单", "店长", "不计销售"} {
		if !strings.Contains(s, want) {
			t.Fatalf("作废小票缺少 %q", want)
		}
	}
}

func TestRenderShiftWithRefund(t *testing.T) {
	b := RenderShift(Shift{
		ShiftNo: "S20261103-001", StaffName: "陈晓",
		OpeningCash: "100.00", Total: "39.00", OrderCount: 3,
		RefundTotal: "13.00",
		Expected:    "126.00", Actual: "126.00", Difference: "0.00",
	})
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	s := string(decoded)
	for _, want := range []string{"退款合计", "-13.00", "应收合计", "现金应收"} {
		if !strings.Contains(s, want) {
			t.Fatalf("交接单缺少 %q", want)
		}
	}
}

func TestRenderKitchenHasNoPrices(t *testing.T) {
	b := RenderKitchen("L20261103-0009", "A1", "水吧", "2026-11-03 21:00:00",
		[]KitchenItem{{Name: "珍珠奶茶", Qty: 2}, {Name: "蛋挞", Qty: 1}})
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	s := string(decoded)
	for _, want := range []string{"后厨单", "A1", "L20261103-0009", "珍珠奶茶  x2", "蛋挞  x1", "请按单出品"} {
		if !strings.Contains(s, want) {
			t.Fatalf("后厨单缺少 %q", want)
		}
	}
	if strings.Contains(s, "13.00") || strings.Contains(s, "合计") {
		t.Fatal("后厨单不应出现价格")
	}
}

func TestRenderOrderWithTable(t *testing.T) {
	b := RenderOrder(Shop{Name: "楼下奶茶"}, Order{
		OrderNo: "L20261103-0010", CreatedAt: "2026-11-03 22:00:00", Table: "A1",
		Items: []Item{{Name: "珍珠奶茶", Price: "13.00", Qty: 1, Amount: "13.00"}},
		Total: "13.00", Cash: "13.00", Change: "0.00",
	})
	decoded, _ := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	s := string(decoded)
	if !strings.Contains(s, "桌台") || !strings.Contains(s, "A1") {
		t.Fatal("堂食小票应打印桌台")
	}
}

func TestRenderOrderMemberLines(t *testing.T) {
	b := RenderOrder(Shop{Name: "楼下奶茶"}, Order{
		OrderNo: "L20261104-0001", CreatedAt: "2026-11-04 06:30:00",
		Items:   []Item{{Name: "珍珠奶茶", Price: "13.00", Qty: 2, Amount: "26.00"}},
		Total:   "26.00", MemberDiscount: "0.52", RoundOff: "0.08",
		Payments: []Pay{{Name: "会员余额", Amount: "25.40"}},
		Points:   "26",
	})
	decoded, _ := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	s := string(decoded)
	for _, want := range []string{"会员折扣", "-0.52", "本单积分", "+26", "抹零", "-0.08"} {
		if !strings.Contains(s, want) {
			t.Fatalf("小票缺少 %q", want)
		}
	}
}

func TestRenderOrderNotice(t *testing.T) {
	b := RenderOrder(Shop{Name: "楼下奶茶"}, Order{
		OrderNo: "L20261104-0020", CreatedAt: "2026-11-04 09:00:00",
		Items:   []Item{{Name: "珍珠奶茶", Price: "13.00", Qty: 1, Amount: "13.00"}},
		Total: "13.00", Cash: "13.00", Change: "0.00",
		Notice: "您有 500 积分将于 2026-11-10 到期，请尽快使用",
	})
	decoded, _ := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if !strings.Contains(string(decoded), "积分将于 2026-11-10 到期") {
		t.Fatal("小票应打印积分到期提醒")
	}
}

func TestRenderShiftMemberDiscount(t *testing.T) {
	b := RenderShift(Shift{
		ShiftNo: "S20261104-001", OpenedAt: "2026-11-04 08:00:00", ClosedAt: "2026-11-04 12:00:00",
		StaffName: "店长", OpeningCash: "100.00", OrderCount: 3, Total: "100.00",
		Expected: "200.00", Actual: "200.00", Difference: "0.00",
		RefundTotal: "0.00", VoidTotal: "0.00", MemberDiscountTotal: "2.60",
	})
	decoded, _ := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	s := string(decoded)
	if !strings.Contains(s, "会员折扣") || !strings.Contains(s, "-2.60") {
		t.Fatal("交接单应打印会员折扣")
	}
}
