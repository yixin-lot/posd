package kitchen

import "pos/internal/order"

// KitchenLine 是后厨单上的一行。
type KitchenLine struct {
	Name string
	Qty  int
}

// Lines 把订单明细转成某个档口的后厨行。
//
// 关键规则：**套餐按组件分档口**——套餐本身没有类目意义，
// 汉堡要去热菜档口、可乐要去水吧。组件数量要乘套餐份数。
func Lines(items []order.Item, match func(category string) bool) []KitchenLine {
	var out []KitchenLine
	for _, it := range items {
		if len(it.Components) > 0 {
			for _, c := range it.Components {
				if !match(c.Category) {
					continue
				}
				out = append(out, KitchenLine{Name: c.Name + "（套餐）", Qty: c.Qty * it.Qty})
			}
			continue
		}
		// 单品：用商品自身类目匹配（类目在快照里没有，由调用方提供）
		if !match(it.Category) {
			continue
		}
		out = append(out, KitchenLine{Name: it.Name, Qty: it.Qty})
	}
	return out
}
