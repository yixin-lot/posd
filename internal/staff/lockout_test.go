package staff

import (
	"errors"
	"testing"
	"time"
)

// fakeNow 让测试可控地推进时间。
func fakeNow(t *testing.T, svc *Service) *time.Time {
	t.Helper()
	base := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return base }
	return &base
}

// TestLockout_AfterFiveFails：5 次失败后第 6 次返回 423（*LockedError，带剩余时间），
// 且锁定期间即使 PIN 正确也被拒。
func TestLockout_AfterFiveFails(t *testing.T) {
	svc := newService(t)
	now := fakeNow(t, svc)

	for i := 0; i < 5; i++ {
		if _, _, err := svc.Login("admin", "0000"); err != ErrUnauthorized {
			t.Fatalf("第 %d 次失败应返回 ErrUnauthorized: %v", i+1, err)
		}
	}
	_, _, err := svc.Login("admin", "0000")
	var locked *LockedError
	if !errors.As(err, &locked) {
		t.Fatalf("第 6 次应返回 LockedError(423)，实际 %v", err)
	}
	if locked.Remaining <= 0 || locked.Remaining > 301 {
		t.Fatalf("剩余时间异常: %d 秒（应为 5 分钟附近）", locked.Remaining)
	}
	// 锁定期间正确 PIN 也 423
	if _, _, err := svc.Login("admin", "1234"); !errors.As(err, &locked) {
		t.Fatalf("锁定期间正确 PIN 也应 423: %v", err)
	}
	_ = now
}

// TestLockout_RecoversAfterWindow：锁定 5 分钟后解除，可正常登录。
func TestLockout_RecoversAfterWindow(t *testing.T) {
	svc := newService(t)
	now := fakeNow(t, svc)

	for i := 0; i < 5; i++ {
		_, _, _ = svc.Login("admin", "0000")
	}
	var locked *LockedError
	if _, _, err := svc.Login("admin", "0000"); !errors.As(err, &locked) {
		t.Fatalf("第 6 次应锁定: %v", err)
	}
	// 推进 6 分钟
	*now = now.Add(6 * time.Minute)
	token, _, err := svc.Login("admin", "1234")
	if err != nil {
		t.Fatalf("锁定解除后应可登录: %v", err)
	}
	if token == "" {
		t.Fatal("登录成功但 token 为空")
	}
}

// TestLockout_SuccessResets：成功登录清零失败计数，之后需再满 5 次才锁定。
func TestLockout_SuccessResets(t *testing.T) {
	svc := newService(t)
	_ = fakeNow(t, svc)

	// 3 次失败
	for i := 0; i < 3; i++ {
		_, _, _ = svc.Login("admin", "0000")
	}
	// 成功登录清零
	if _, _, err := svc.Login("admin", "1234"); err != nil {
		t.Fatalf("成功登录失败: %v", err)
	}
	// 清零后再失败 5 次：前 4 次应仍是 401
	for i := 0; i < 4; i++ {
		if _, _, err := svc.Login("admin", "0000"); err != ErrUnauthorized {
			t.Fatalf("清零后第 %d 次失败应 401 而非锁定: %v", i+1, err)
		}
	}
	// 第 5 次失败触发锁定
	if _, _, err := svc.Login("admin", "0000"); err != ErrUnauthorized {
		t.Fatalf("第 5 次失败应仍 401: %v", err)
	}
	// 第 6 次 423
	var locked *LockedError
	if _, _, err := svc.Login("admin", "0000"); !errors.As(err, &locked) {
		t.Fatalf("清零后再满 5 次失败应锁定: %v", err)
	}
}
