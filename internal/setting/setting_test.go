package setting

import (
	"path/filepath"
	"testing"

	"pos/internal/store"
)

func newService(t *testing.T) *Service {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(db)
}

func TestDefaults(t *testing.T) {
	s := newService(t)
	name, _, footer, err := s.Shop()
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if name != DefaultShopName || footer != DefaultShopFooter {
		t.Fatalf("默认值不符: %q %q", name, footer)
	}
}

func TestSetAndOverwrite(t *testing.T) {
	s := newService(t)
	if err := s.Set(KeyShopName, "楼下奶茶"); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if err := s.Set(KeyShopName, "楼下奶茶 2 店"); err != nil {
		t.Fatalf("覆盖失败: %v", err)
	}
	name, _, _, err := s.Shop()
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if name != "楼下奶茶 2 店" {
		t.Fatalf("覆盖后应取最新值，实际 %q", name)
	}
}

func TestAllReturnsMap(t *testing.T) {
	s := newService(t)
	_ = s.Set(KeyShopFooter, "欢迎再来")
	all, err := s.All()
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if all[KeyShopFooter] != "欢迎再来" {
		t.Fatalf("All 结果不符: %+v", all)
	}
}
