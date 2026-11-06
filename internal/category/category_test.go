package category

import (
	"path/filepath"
	"testing"

	"pos/internal/store"
)

func newSvc(t *testing.T) (*Service, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(db), db
}

func TestSeededFromProducts(t *testing.T) {
	s, _ := newSvc(t)
	list, err := s.List()
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("应从现有商品回填类目")
	}
	for _, c := range list {
		if c.Count <= 0 {
			t.Fatalf("类目 %s 应有商品", c.Name)
		}
	}
}

func TestRenameCascadesProducts(t *testing.T) {
	s, db := newSvc(t)
	list, _ := s.List()
	old := list[0].Name
	if _, err := s.Rename(list[0].ID, "改名后类目", 1); err != nil {
		t.Fatalf("改名失败: %v", err)
	}
	var n int
	if err := db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_product WHERE category=?", old).Scan(&n); err != nil {
		t.Fatalf("查商品失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("旧类目名仍有 %d 个商品（未级联）", n)
	}
}

func TestDeleteBlockedWhenInUse(t *testing.T) {
	s, _ := newSvc(t)
	list, _ := s.List()
	if err := s.Delete(list[0].ID); err != ErrConflict {
		t.Fatalf("仍被商品引用时应拒绝删除，实际 %v", err)
	}
	if _, err := s.Create("空类目", 9); err != nil {
		t.Fatalf("新建失败: %v", err)
	}
	fresh, _ := s.List()
	for _, c := range fresh {
		if c.Name == "空类目" {
			if err := s.Delete(c.ID); err != nil {
				t.Fatalf("空类目应可删除: %v", err)
			}
		}
	}
}
