package member

import (
	"path/filepath"
	"testing"

	"pos/internal/store"
)

func newBonusSvc(t *testing.T) *Service {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(db)
}

func TestParseBonusRules(t *testing.T) {
	rules := ParseBonusRules("1000:150, 500:50, garbage, 0:5")
	if len(rules) != 2 {
		t.Fatalf("应解析出 2 条合法规则，实际 %d: %+v", len(rules), rules)
	}
	if rules[0].Threshold != 50000 || rules[0].Bonus != 5000 {
		t.Fatalf("应按门槛升序，第一条为 500 元送 50，实际 %+v", rules[0])
	}
	if rules[1].Threshold != 100000 || rules[1].Bonus != 15000 {
		t.Fatalf("第二条为 1000 元送 150，实际 %+v", rules[1])
	}
}

func TestBonusForPicksHighestTier(t *testing.T) {
	rules := ParseBonusRules("500:50,1000:150")
	cases := []struct{ amount, want int64 }{
		{49999, 0},
		{50000, 5000},
		{99999, 5000},
		{100000, 15000},
		{200000, 15000}, // 不叠加
	}
	for _, c := range cases {
		if got := BonusFor(rules, c.amount); got != c.want {
			t.Fatalf("充 %d 分应送 %d，实际 %d", c.amount, c.want, got)
		}
	}
}

func TestTopupWithBonusWritesTwoTxns(t *testing.T) {
	s := newBonusSvc(t)
	m, _ := s.Create("13800000801", "储值测试")

	got, err := s.TopupWithBonus(m.ID, 50000, 5000, "前台充值")
	if err != nil {
		t.Fatalf("充值失败: %v", err)
	}
	if got.Balance != 55000 {
		t.Fatalf("余额应为 550（本金 500 + 赠 50），实际 %d", got.Balance)
	}
	list, _ := s.Txns(m.ID, 10)
	var topup, bonus int64
	for _, tx := range list {
		switch tx.Kind {
		case "topup":
			topup += tx.Amount
		case "bonus":
			bonus += tx.Amount
		}
	}
	if topup != 50000 || bonus != 5000 {
		t.Fatalf("流水应本金 50000 / 赠金 5000，实际 %d / %d", topup, bonus)
	}
}
