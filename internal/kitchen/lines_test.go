package kitchen

import (
	"testing"

	"pos/internal/order"
)

func TestLinesExpandsBundleComponents(t *testing.T) {
	items := []order.Item{
		{
			Name: "超值套餐", Qty: 2, Category: "套餐",
			Components: []order.ItemComponent{
				{Name: "珍珠奶茶", Category: "奶茶", Qty: 1},
				{Name: "蛋挞", Category: "甜点", Qty: 2},
			},
		},
	}
	// 水吧只收奶茶
	bar := Lines(items, func(c string) bool { return c == "奶茶" })
	if len(bar) != 1 || bar[0].Name != "珍珠奶茶（套餐）" || bar[0].Qty != 2 {
		t.Fatalf("水吧应收到 珍珠奶茶（套餐）x2（组件 1 × 套餐 2），实际 %+v", bar)
	}
	// 后厨收甜点：组件 2 × 套餐 2 = 4
	hot := Lines(items, func(c string) bool { return c == "甜点" })
	if len(hot) != 1 || hot[0].Name != "蛋挞（套餐）" || hot[0].Qty != 4 {
		t.Fatalf("后厨应收到 蛋挞（套餐）x4，实际 %+v", hot)
	}
	// 套餐本身的类目不该命中任何档口
	byBundle := Lines(items, func(c string) bool { return c == "套餐" })
	if len(byBundle) != 0 {
		t.Fatalf("套餐类目不该单独出单，实际 %+v", byBundle)
	}
}

func TestLinesSingleItemUsesSnapshotCategory(t *testing.T) {
	items := []order.Item{{Name: "珍珠奶茶", Qty: 3, Category: "奶茶"}}
	got := Lines(items, func(c string) bool { return c == "奶茶" })
	if len(got) != 1 || got[0].Qty != 3 {
		t.Fatalf("单品应直接按类目命中，实际 %+v", got)
	}
	if len(Lines(items, func(c string) bool { return c == "甜点" })) != 0 {
		t.Fatal("类目不匹配时不该出单")
	}
}
