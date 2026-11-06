package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pos/internal/store"
)

func newDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRunOnceCreatesSnapshot(t *testing.T) {
	db := newDB(t)
	dir := filepath.Join(t.TempDir(), "backups")
	svc := New(db, dir, 3, slogDiscard())

	path, err := svc.RunOnce()
	if err != nil {
		t.Fatalf("备份失败: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("快照文件不存在: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("快照不应为空")
	}

	// 快照必须能独立打开，且商品数与源库一致
	clone, err := store.Open(path)
	if err != nil {
		t.Fatalf("快照打不开: %v", err)
	}
	defer clone.Close()
	src, _ := db.CountProducts()
	got, err := clone.CountProducts()
	if err != nil {
		t.Fatalf("读快照失败: %v", err)
	}
	if got != src {
		t.Fatalf("快照商品数 %d 与源库 %d 不一致", got, src)
	}
}

func TestPruneKeepsOnlyN(t *testing.T) {
	db := newDB(t)
	dir := filepath.Join(t.TempDir(), "backups")
	svc := New(db, dir, 2, slogDiscard())
	base := time.Now()
	svc.now = func() time.Time { return base }
	if _, err := svc.RunOnce(); err != nil {
		t.Fatalf("第一次备份失败: %v", err)
	}
	for i := 0; i < 3; i++ {
		base = base.Add(time.Second)
		svc.now = func() time.Time { return base }
		if _, err := svc.RunOnce(); err != nil {
			t.Fatalf("备份失败: %v", err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("应只保留 2 个快照，实际 %d", len(entries))
	}
}
