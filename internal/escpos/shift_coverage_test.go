package escpos

import (
	"reflect"
	"strings"
	"testing"

)

// TestRenderShiftCoversEveryField 守卫「交接单新增字段忘了渲染」。
// 与订单小票同款做法：每个字段填唯一标记，渲染后必须出现。
func TestRenderShiftCoversEveryField(t *testing.T) {
	// 下面这些字段不单独成行：要么参与组合输出，要么非文本。
	ignore := map[string]string{
		"Payments":   "收款构成按多行输出，单独用例覆盖",
		"Note":       "备注为空时不打印",
		"OpenedAt":   "与 ClosedAt 同行区间展示",
		"ClosedAt":   "同上",
		"ShiftNo":    "标题区已含单号",
		"StaffName":  "与单号同行",
		"Expected":   "与 Actual/Difference 组合成对账行",
		"Actual":     "同上",
		"Difference": "同上",
		"DineInCount": "0 时不打印",
		"TakeawayCnt": "同上",
		"CrossRefundCount": "0 时不打印",
		"PointsEarned": "0 时不打印",
		"PointsUsed":   "同上",
	}

	s := Shift{
		ShiftNo: "F_ShiftNo", StaffName: "F_Staff", OpenedAt: "F_Open", ClosedAt: "F_Close",
		OpeningCash: "F_Opening", OrderCount: 7, Total: "F_Total",
		RefundTotal: "F_Refund", VoidTotal: "F_Void",
		MemberDiscountTotal: "F_MemberDisc",
		Expected: "F_Expected", Actual: "F_Actual", Difference: "F_Diff", Note: "F_Note",
		DineInCount: 3, TakeawayCnt: 4, CrossRefundCount: 1, CrossRefundTotal: "F_Cross",
		PointsEarned: 12, PointsUsed: 5,
	}
	out := decode(t, RenderShift(s))

	v := reflect.ValueOf(s)
	tp := v.Type()
	for i := 0; i < tp.NumField(); i++ {
		name := tp.Field(i).Name
		if _, ok := ignore[name]; ok {
			continue
		}
		if v.Field(i).Kind() != reflect.String {
			continue
		}
		want := v.Field(i).String()
		if want != "" && !strings.Contains(out, want) {
			t.Fatalf("Shift.%s=%q 没出现在交接单上——RenderShift 漏了它", name, want)
		}
	}
	// 关键锚点
	for _, anchor := range []string{"交接班单", "订单笔数", "现金"} {
		if !strings.Contains(out, anchor) {
			t.Fatalf("交接单缺少锚点 %q", anchor)
		}
	}
}
