// Package backup 负责数据库快照：唯一真源是 data/pos.db，丢了就是全部账目丢失。
//
// 用 SQLite 的 VACUUM INTO 做在线快照：不需要停服务，也不会读到写了一半的页。
package backup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"pos/internal/store"
)

// Service 定时给数据库打快照并清理旧文件。
type Service struct {
	db   *store.DB
	dir  string
	keep int
	now  func() time.Time
	log  *slog.Logger
}

// New 创建备份服务（dir 默认是库文件同级的 backups/）。
func New(db *store.DB, dir string, keep int, log *slog.Logger) *Service {
	if keep <= 0 {
		keep = 7
	}
	return &Service{db: db, dir: dir, keep: keep, now: time.Now, log: log}
}

// RunOnce 立刻打一个快照，并清理超出保留数的旧快照。
func (s *Service) RunOnce() (string, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", err
	}
	name := "pos-" + s.now().Format("20060102-150405") + ".db"
	path := filepath.Join(s.dir, name)
	// VACUUM INTO 要求目标文件不存在
	_ = os.Remove(path)
	if _, err := s.db.SQL().Exec("VACUUM INTO ?", path); err != nil {
		return "", fmt.Errorf("快照失败: %w", err)
	}
	if err := s.prune(); err != nil {
		s.log.Warn("清理旧备份失败", "err", err)
	}
	return path, nil
}

// Start 每日打一次快照（启动时先打一次）。
func (s *Service) Start(ctx context.Context) {
	if _, err := s.RunOnce(); err != nil {
		s.log.Warn("启动备份失败", "err", err)
	} else {
		s.log.Info("已生成数据库快照", "dir", s.dir)
	}
	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if _, err := s.RunOnce(); err != nil {
					s.log.Warn("定时备份失败", "err", err)
				}
			}
		}
	}()
}

// prune 只保留最近 keep 个快照。
func (s *Service) prune() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	type f struct {
		path string
		at   time.Time
	}
	var files []f
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "pos-") || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, f{filepath.Join(s.dir, e.Name()), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.After(files[j].at) })
	for i := s.keep; i < len(files); i++ {
		_ = os.Remove(files[i].path)
	}
	return nil
}
