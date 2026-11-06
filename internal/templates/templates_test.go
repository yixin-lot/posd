package templates

import "testing"

func TestAllTemplatesLoad(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("应有 4 个业态，实际 %d", len(all))
	}
	for _, tp := range all {
		if tp.ID == "" || tp.Name == "" || len(tp.Products) == 0 {
			t.Fatalf("模板缺字段: %+v", tp)
		}
		for _, it := range tp.Products {
			if it.Price <= 0 || it.Name == "" {
				t.Fatalf("模板商品不合法: %+v", it)
			}
		}
	}
}

func TestLoadUnknown(t *testing.T) {
	if _, err := Load("nope"); err == nil {
		t.Fatal("未知业态应报错")
	}
}
