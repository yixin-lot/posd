package catalog

import (
	"path/filepath"
	"testing"

	"pos/internal/store"
)

func TestBOMRollupCost(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	svc := NewService(db)

	a, _ := svc.Create(SaveInput{Name: "Tea", Price: 0.01, Cost: 2})
	b, _ := svc.Create(SaveInput{Name: "Milk", Price: 0.01, Cost: 3})
	parent, _ := svc.Create(SaveInput{Name: "Latte", Price: 12, Cost: 0})

	if err := svc.SetBOM(parent.ID, []BOMItem{
		{ComponentID: a.ID, QtyMilli: 2000}, // 2 份茶
		{ComponentID: b.ID, QtyMilli: 1000}, // 1 份奶
	}); err != nil {
		t.Fatalf("保存配方失败: %v", err)
	}
	cost, err := svc.RollupCost(parent.ID, 0)
	if err != nil {
		t.Fatalf("卷算失败: %v", err)
	}
	if cost != 700 {
		t.Fatalf("成本应为 700 分，实际 %d", cost)
	}
}

func TestBOMRejectsCycle(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	svc := NewService(db)
	a, _ := svc.Create(SaveInput{Name: "A", Price: 1})
	b, _ := svc.Create(SaveInput{Name: "B", Price: 1})
	if err := svc.SetBOM(a.ID, []BOMItem{{ComponentID: b.ID, QtyMilli: 1000}}); err != nil {
		t.Fatalf("A→B 失败: %v", err)
	}
	if err := svc.SetBOM(b.ID, []BOMItem{{ComponentID: a.ID, QtyMilli: 1000}}); err == nil {
		t.Fatal("B→A 应被环检测拒绝")
	}
}
