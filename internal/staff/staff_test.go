package staff

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
	svc, err := NewService(db)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	return svc
}

func TestSeedsDefaultManager(t *testing.T) {
	svc := newService(t)
	n, err := svc.Count()
	if err != nil || n != 1 {
		t.Fatalf("应有 1 个默认店长，实际 %d err=%v", n, err)
	}
}

func TestLoginAndVerify(t *testing.T) {
	svc := newService(t)
	token, st, err := svc.Login("admin", "1234")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if st.Role != "manager" {
		t.Fatalf("角色错误: %s", st.Role)
	}
	if _, ok := svc.Verify(token); !ok {
		t.Fatal("令牌应有效")
	}
}

func TestRejectsWrongPin(t *testing.T) {
	svc := newService(t)
	if _, _, err := svc.Login("admin", "0000"); err != ErrUnauthorized {
		t.Fatalf("错误 PIN 应返回 ErrUnauthorized，实际 %v", err)
	}
}

func TestRejectsUnknownUser(t *testing.T) {
	svc := newService(t)
	if _, _, err := svc.Login("nobody", "1234"); err != ErrUnauthorized {
		t.Fatalf("未知账号应返回 ErrUnauthorized，实际 %v", err)
	}
}
