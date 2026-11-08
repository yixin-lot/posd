package catalog

import (
	"path/filepath"
	"testing"

	"pos/internal/store"
)

// PUT 省略 kind/bundle 时必须保留套餐结构（线上事故：编辑一次套餐变回单品）。
func TestUpdateKeepsBundleWhenKindOmitted(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	svc := NewService(db)

	a, err := svc.Create(SaveInput{Name: "Tea", Price: 10})
	if err != nil {
		t.Fatalf("建组件 A 失败: %v", err)
	}
	b, err := svc.Create(SaveInput{Name: "Pearl", Price: 2})
	if err != nil {
		t.Fatalf("建组件 B 失败: %v", err)
	}
	bun, err := svc.Create(SaveInput{
		Kind: "bundle", Name: "Combo", Price: 11,
		Bundle: []BundleItem{{ComponentID: a.ID, Qty: 1}, {ComponentID: b.ID, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("建套餐失败: %v", err)
	}

	// 模拟旧前端：只提交名称/价格，不带 kind/bundle
	got, err := svc.Update(bun.ID, SaveInput{Name: "Combo Renamed", Price: 12})
	if err != nil {
		t.Fatalf("更新套餐失败: %v", err)
	}
	if got.Kind != "bundle" {
		t.Fatalf("省略 kind 后应保持 bundle，实际 %q", got.Kind)
	}
	if len(got.Bundle) != 2 {
		t.Fatalf("组件应保留 2 个，实际 %d", len(got.Bundle))
	}
}
