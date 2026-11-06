package member

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

func TestCreateAndDuplicate(t *testing.T) {
	s := newSvc(t)
	m, err := s.Create("13800000001", "张三")
	if err != nil {
		t.Fatalf("开卡失败: %v", err)
	}
	if m.Phone != "13800000001" || m.Balance != 0 {
		t.Fatalf("开卡结果不符: %+v", m)
	}
	if _, err := s.Create("13800000001", "李四"); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复手机号应冲突，实际 %v", err)
	}
}

func TestTopupAndConsume(t *testing.T) {
	s := newSvc(t)
	m, _ := s.Create("13800000002", "王五")

	m, err := s.Topup(m.ID, 5000, "现金充值")
	if err != nil {
		t.Fatalf("充值失败: %v", err)
	}
	if m.Balance != 5000 {
		t.Fatalf("余额应为 5000，实际 %d", m.Balance)
	}

	m, err = s.Consume(m.ID, 1300, "买奶茶")
	if err != nil {
		t.Fatalf("消费失败: %v", err)
	}
	if m.Balance != 3700 {
		t.Fatalf("余额应为 3700，实际 %d", m.Balance)
	}

	if _, err := s.Consume(m.ID, 99999, "超额"); !errors.Is(err, ErrInsufficient) {
		t.Fatalf("余额不足应报错，实际 %v", err)
	}

	list, err := s.Txns(m.ID, 10)
	if err != nil {
		t.Fatalf("读流水失败: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应有 2 条流水，实际 %d", len(list))
	}
	if list[0].Kind != "consume" || list[0].BalanceAfter != 3700 {
		t.Fatalf("流水不符: %+v", list[0])
	}
}

func TestFindByPhone(t *testing.T) {
	s := newSvc(t)
	if _, found, _ := s.FindByPhone("13900000000"); found {
		t.Fatal("未开卡不应找到")
	}
	m, _ := s.Create("13900000000", "赵六")
	got, found, err := s.FindByPhone("13900000000")
	if err != nil || !found || got.ID != m.ID {
		t.Fatalf("按手机号查询失败: %v %v %+v", err, found, got)
	}
}
