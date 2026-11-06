package member

import (
	"errors"
	"testing"
)

func TestTierBySpend(t *testing.T) {
	s := newSvc(t)
	m, _ := s.Create("13800000201", "等级测试")

	if tier := s.TierOf(m); tier.Name != "普通会员" || tier.Discount != 0 {
		t.Fatalf("新会员应为普通会员: %+v", tier)
	}

	// 累计消费 120000 分（1200 元）→ 银卡（阈值 1000 元）
	if _, err := s.db.SQL().Exec("UPDATE t_pos_member SET total_spent=120000 WHERE id=?", m.ID); err != nil {
		t.Fatalf("造累计消费失败: %v", err)
	}
	got, _ := s.Get(m.ID)
	tier := s.TierOf(got)
	if tier.Name != "银卡会员" || tier.Discount != 2 || tier.PointRate != 2 {
		t.Fatalf("应升到银卡（2%% 折扣、2 倍积分）: %+v", tier)
	}
}

func TestExpirePoints(t *testing.T) {
	s := newSvc(t)
	m, _ := s.Create("13800000202", "过期测试")
	if _, err := s.db.SQL().Exec("UPDATE t_pos_member SET points=500, points_expire_at='2026-01-01' WHERE id=?", m.ID); err != nil {
		t.Fatalf("造过期数据失败: %v", err)
	}

	n, err := s.ExpirePoints("2026-11-04")
	if err != nil {
		t.Fatalf("过期处理失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("应处理 1 人，实际 %d", n)
	}
	got, _ := s.Get(m.ID)
	if got.Points != 0 {
		t.Fatalf("过期后积分应为 0，实际 %d", got.Points)
	}
	list, _ := s.Txns(m.ID, 10)
	if len(list) == 0 || list[0].Kind != "points_expire" {
		t.Fatalf("应写 points_expire 流水: %+v", list)
	}
}

func TestInactiveMemberCannotSpend(t *testing.T) {
	s := newSvc(t)
	m, _ := s.Create("13800000501", "停用会员")
	if _, err := s.Topup(m.ID, 5000, "充值"); err != nil {
		t.Fatalf("充值失败: %v", err)
	}
	if _, err := s.SetActive(m.ID, false); err != nil {
		t.Fatalf("停用失败: %v", err)
	}
	if _, err := s.Consume(m.ID, 1300, "消费"); !errors.Is(err, ErrInactive) {
		t.Fatalf("停用会员消费应被拒，实际 %v", err)
	}
	// 余额不应被扣
	got, _ := s.Get(m.ID)
	if got.Balance != 5000 {
		t.Fatalf("余额不应变化，实际 %d", got.Balance)
	}
}

func TestPointsNotifiedMarker(t *testing.T) {
	s := newSvc(t)
	m, _ := s.Create("13800000601", "提醒测试")
	if _, err := s.db.SQL().Exec(
		"UPDATE t_pos_member SET points=500, points_expire_at=date('now','+3 days') WHERE id=?", m.ID); err != nil {
		t.Fatalf("造到期数据失败: %v", err)
	}

	got, _ := s.Get(m.ID)
	if got.PointsNotifiedAt != "" {
		t.Fatalf("初始不应有提醒标记，实际 %q", got.PointsNotifiedAt)
	}
	if err := s.MarkPointsNotified(m.ID); err != nil {
		t.Fatalf("标记失败: %v", err)
	}
	got, _ = s.Get(m.ID)
	if got.PointsNotifiedAt == "" {
		t.Fatal("标记后应记录日期")
	}

	// 新积分入账应清空标记（有效期变了，需要重新提醒）
	tx, err := s.db.SQL().Begin()
	if err != nil {
		t.Fatalf("开事务失败: %v", err)
	}
	if err := s.AddPointsTx(tx, m.ID, 100, 1, 0); err != nil {
		_ = tx.Rollback()
		t.Fatalf("加积分失败: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	got, _ = s.Get(m.ID)
	if got.PointsNotifiedAt != "" {
		t.Fatalf("新积分后应清空提醒标记，实际 %q", got.PointsNotifiedAt)
	}
}
