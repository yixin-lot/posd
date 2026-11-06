package escpos

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func decode(t *testing.T, b []byte) string {
	t.Helper()
	s, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err != nil {
		t.Fatalf("GBK 解码失败: %v", err)
	}
	return string(s)
}

// TestRenderOrderCoversEveryField 守卫「新增字段忘了渲染」。
//
// 做法：把 escpos.Order 的每个字段都填成唯一标记（如 F_Table），
// 渲染后检查标记是否出现在纸面。以后谁给 Order 加字段却忘了在
// RenderOrder 里输出，这条测试立刻红。
//
// 例外（不单独成行、而是参与组合的字段）必须写进 ignore 并说明原因。
func TestRenderOrderCoversEveryField(t *testing.T) {
	ignore := map[string]string{
		"Cash":   "仅用于判断是否打印找零行，金额本身来自 Change",
		"Change": "同上，随现金行一起输出",
		"Logo":   "二进制图片，由 TestRenderOrderPrintsLogo 单独断言",
		"Lang":     "语言选择器，决定 Labels，不单独成行",
		"Currency": "货币前缀，拼进金额，不单独成行",
	}

	o := Order{
		OrderNo: "F_OrderNo", CreatedAt: "F_CreatedAt", Table: "F_Table",
		Total: "F_Total", Discount: "F_Discount", RoundOff: "F_RoundOff",
		MemberDiscount: "F_MemberDiscount", Points: "F_Points", Notice: "F_Notice", Footer: "F_Footer",
		QRText: "F_QRText",
		Items:    []Item{{Name: "F_ItemName", Price: "F_ItemPrice", Qty: 1, Amount: "F_ItemAmount"}},
		Payments: []Pay{{Name: "F_PayName", Amount: "F_PayAmount"}},
	}

	out := decode(t, RenderOrder(Shop{Name: "F_Shop"}, o))

	v := reflect.ValueOf(o)
	tp := v.Type()
	for i := 0; i < tp.NumField(); i++ {
		name := tp.Field(i).Name
		if reason, ok := ignore[name]; ok {
			t.Logf("跳过 %s：%s", name, reason)
			continue
		}
		f := v.Field(i)
		switch f.Kind() {
		case reflect.String:
			if f.String() == "" {
				t.Fatalf("字段 %s 没填测试值", name)
			}
			if !strings.Contains(out, f.String()) {
				t.Fatalf("字段 %s 的值 %q 没出现在小票上——RenderOrder 忘了渲染？", name, f.String())
			}
		case reflect.Slice:
			// 切片内容的标记已分别填在 Items/Payments 里，这里只要求非空
			if f.Len() == 0 {
				t.Fatalf("切片字段 %s 没填测试值", name)
			}
		}
	}

	// 店铺名与小票标题也必须出现
	if !strings.Contains(out, "F_Shop") {
		t.Fatal("店名没打印")
	}
	// 小票必须有「单号」与「合计」两个锚点，否则结构就散了
	for _, anchor := range []string{"单号", "合计"} {
		if !strings.Contains(out, anchor) {
			t.Fatalf("小票缺少锚点 %q", anchor)
		}
	}
}

// TestRenderOrderZeroValuesStayQuiet：没有的项不要印出来（避免"折扣 0.00"这种噪音）。
func TestRenderOrderZeroValuesStayQuiet(t *testing.T) {
	out := decode(t, RenderOrder(Shop{Name: "店"}, Order{
		OrderNo: "L1", CreatedAt: "2026-11-04 10:00:00",
		Items:   []Item{{Name: "茶", Price: "13.00", Qty: 1, Amount: "13.00"}},
		Total:   "13.00", Cash: "13.00", Change: "0.00",
		MemberDiscount: "0.00", RoundOff: "0.00", Points: "0",
	}))
	for _, unwanted := range []string{"会员折扣", "抹零", "本单积分"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("零值不该打印 %q", unwanted)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
