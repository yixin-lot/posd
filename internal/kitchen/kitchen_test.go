package kitchen

import (
	"errors"
	"path/filepath"
	"testing"

	"pos/internal/store"
)

func newSvc(t *testing.T) *Service {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(db)
}

func TestCreateAndMatch(t *testing.T) {
	s := newSvc(t)
	st, err := s.Create("水吧", "POS-58", "奶茶,果茶")
	if err != nil {
		t.Fatalf("建档口失败: %v", err)
	}
	if !st.Match("奶茶") || !st.Match("果茶") {
		t.Fatal("奶茶/果茶应命中水吧")
	}
	if st.Match("甜品") {
		t.Fatal("甜品不该进水吧")
	}

	all, _ := s.Create("总厨", "POS-80", "")
	if !all.Match("甜品") || !all.Match("任意类目") {
		t.Fatal("空类目应全收")
	}

	if _, err := s.Create("水吧", "POS-58", ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("重名应冲突，实际 %v", err)
	}
	if _, err := s.Create("", "POS-58", ""); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("空名称应报错，实际 %v", err)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	s := newSvc(t)
	st, _ := s.Create("水吧", "POS-58", "奶茶")

	upd, err := s.Update(st.ID, "水吧2", "POS-59", "奶茶,咖啡", false)
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if upd.Name != "水吧2" || upd.Active || upd.Printer != "POS-59" {
		t.Fatalf("更新结果不符: %+v", upd)
	}
	if err := s.Delete(st.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := s.Get(st.ID); err == nil {
		t.Fatal("删除后应查不到")
	}
}

func TestNormalizeTrimsCategories(t *testing.T) {
	s := newSvc(t)
	st, _ := s.Create(" 水吧 ", " POS-58 ", " 奶茶 , 果茶 , ")
	if st.Name != "水吧" || st.Printer != "POS-58" || st.Categories != "奶茶,果茶" {
		t.Fatalf("规范化不符: %+v", st)
	}
}
