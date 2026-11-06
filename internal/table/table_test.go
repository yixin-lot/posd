package table

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

func TestCreateAndBind(t *testing.T) {
	s := newSvc(t)
	tb, err := s.Create("A1")
	if err != nil {
		t.Fatalf("建桌失败: %v", err)
	}
	if tb.Status != "free" {
		t.Fatalf("新桌应为空闲，实际 %s", tb.Status)
	}
	bound, err := s.Bind(tb.ID, "L20261103-0001", 2)
	if err != nil {
		t.Fatalf("开台失败: %v", err)
	}
	if bound.Status != "occupied" || bound.OrderNo == "" {
		t.Fatalf("开台结果不符: %+v", bound)
	}
	if _, err := s.Bind(tb.ID, "L20261103-0002", 2); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复开台应冲突，实际 %v", err)
	}
	if err := s.Delete(tb.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("占用中的桌台不可删，实际 %v", err)
	}
	freed, err := s.Release(tb.ID)
	if err != nil {
		t.Fatalf("清台失败: %v", err)
	}
	if freed.Status != "free" || freed.OrderNo != "" {
		t.Fatalf("清台后应空闲: %+v", freed)
	}
}

func TestDuplicateName(t *testing.T) {
	s := newSvc(t)
	if _, err := s.Create("B1"); err != nil {
		t.Fatalf("建桌失败: %v", err)
	}
	if _, err := s.Create("B1"); !errors.Is(err, ErrConflict) {
		t.Fatalf("重名应冲突，实际 %v", err)
	}
}
