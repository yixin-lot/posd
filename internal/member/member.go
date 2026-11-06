// Package member 是会员基础能力：开卡、储值、余额扣减与流水。
//
// 金额一律以「分」存库；余额变动必须写流水，且流水与余额在同一事务里更新。
package member

import (
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"

	"pos/internal/store"
)

var (
	// ErrNotFound 会员不存在。
	ErrNotFound = errors.New("会员不存在")
	// ErrConflict 手机号已开卡。
	ErrConflict = errors.New("该手机号已开卡")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
	// ErrInsufficient 余额不足。
	ErrInsufficient = errors.New("余额不足")
	// ErrInactive 会员已停用。
	ErrInactive = errors.New("会员已停用")
)

// Member 是一个会员。
type Member struct {
	ID         int64
	Phone      string
	Name       string
	Balance    int64 // 分
	Points     int
	TotalSpent int64 // 累计消费（分），等级依据
	Active     bool
	PointsExpireAt string // 积分到期日（YYYY-MM-DD，空=永不过期）
	PointsNotifiedAt string // 上次提醒日期（空=未提醒）
}

// Txn 是一条余额流水。
type Txn struct {
	Kind         string
	Amount       int64
	BalanceAfter int64
	Note         string
	CreatedAt    string
}

// Service 管理会员。
type Service struct {
	db *store.DB
}

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// SetActive 停用/启用会员（停用后不能开卡消费，但历史流水保留）。
func (s *Service) SetActive(id int64, active bool) (Member, error) {
	if _, err := s.db.SQL().Exec("UPDATE t_pos_member SET active=? WHERE id=?", boolInt(active), id); err != nil {
		return Member{}, err
	}
	return s.Get(id)
}

// Delete 删除会员：有流水（充过值/消费过）只能停用。
func (s *Service) Delete(id int64) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	var n int
	if err := s.db.SQL().QueryRow(
		"SELECT COUNT(*) FROM t_pos_member_txn WHERE member_id=?", id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrConflict
	}
	_, err := s.db.SQL().Exec("DELETE FROM t_pos_member WHERE id=?", id)
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Search 按手机号/姓名模糊查（会员中心页用），limit 默认 50。
func (s *Service) Search(keyword string, limit int) ([]Member, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	kw := "%" + strings.TrimSpace(keyword) + "%"
	rows, err := s.db.SQL().Query(
		"SELECT id,phone,name,balance,points,total_spent,active,COALESCE(points_expire_at,''),COALESCE(points_notified_at,'') FROM t_pos_member"+
			" WHERE phone LIKE ? OR name LIKE ? ORDER BY id DESC LIMIT ?", kw, kw, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var mm Member
		if err := rows.Scan(&mm.ID, &mm.Phone, &mm.Name, &mm.Balance, &mm.Points, &mm.TotalSpent, &mm.Active, &mm.PointsExpireAt, &mm.PointsNotifiedAt); err != nil {
			return nil, err
		}
		out = append(out, mm)
	}
	return out, rows.Err()
}

// Create 开卡：手机号唯一。
func (s *Service) Create(phone, name string) (Member, error) {
	phone = strings.TrimSpace(phone)
	if !validPhone(phone) {
		return Member{}, errBad("手机号格式不正确（11 位，1 开头）")
	}
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_member(phone,name,balance,points,created_at)"+
			" VALUES(?,?,0,0,datetime('now','localtime'))", phone, strings.TrimSpace(name))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Member{}, ErrConflict
		}
		return Member{}, err
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

// Get 按 ID 查询。
func (s *Service) Get(id int64) (Member, error) {
	var m Member
	err := s.db.SQL().QueryRow(
		"SELECT id,phone,name,balance,points,total_spent,active,COALESCE(points_expire_at,''),COALESCE(points_notified_at,'') FROM t_pos_member WHERE id=?", id).
		Scan(&m.ID, &m.Phone, &m.Name, &m.Balance, &m.Points, &m.TotalSpent, &m.Active, &m.PointsExpireAt, &m.PointsNotifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return m, err
}

// FindByPhone 按手机号查（收银台常用）。
func (s *Service) FindByPhone(phone string) (Member, bool, error) {
	var m Member
	err := s.db.SQL().QueryRow(
		"SELECT id,phone,name,balance,points,total_spent,active,COALESCE(points_expire_at,''),COALESCE(points_notified_at,'') FROM t_pos_member WHERE phone=?",
		strings.TrimSpace(phone)).Scan(&m.ID, &m.Phone, &m.Name, &m.Balance, &m.Points, &m.TotalSpent, &m.Active, &m.PointsExpireAt, &m.PointsNotifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Member{}, false, nil
	}
	if err != nil {
		return Member{}, false, err
	}
	return m, true, nil
}

// Topup 充值（金额单位：分）。
func (s *Service) Topup(id, amount int64, note string) (Member, error) {
	if amount <= 0 {
		return Member{}, errBad("充值金额必须大于 0")
	}
	return s.change(id, "topup", amount, note)
}

// BonusRule 是一条储值赠送规则（单位：分）。
type BonusRule struct {
	Threshold int64
	Bonus     int64
}

// ParseBonusRules 解析 "500:50,1000:150"（元:元）。
func ParseBonusRules(raw string) []BonusRule {
	var out []BonusRule
	for _, part := range strings.Split(raw, ",") {
		fields := strings.Split(strings.TrimSpace(part), ":")
		if len(fields) != 2 {
			continue
		}
		th, err1 := strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
		bn, err2 := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
		if err1 != nil || err2 != nil || th <= 0 || bn <= 0 {
			continue
		}
		out = append(out, BonusRule{Threshold: int64(th*100 + 0.5), Bonus: int64(bn*100 + 0.5)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Threshold < out[j].Threshold })
	return out
}

// BonusFor 返回本次充值应赠送的金额（取满足条件的最高档，不叠加）。
func BonusFor(rules []BonusRule, amount int64) int64 {
	var bonus int64
	for _, r := range rules {
		if amount >= r.Threshold {
			bonus = r.Bonus
		}
	}
	return bonus
}

// TopupWithBonus 充值并赠送：本金与赠金分两条流水记（赠金单独统计，便于算营销成本）。
func (s *Service) TopupWithBonus(id, amount, bonus int64, note string) (Member, error) {
	if amount <= 0 {
		return Member{}, errBad("充值金额必须大于 0")
	}
	m, err := s.change(id, "topup", amount, note)
	if err != nil {
		return Member{}, err
	}
	if bonus <= 0 {
		return m, nil
	}
	return s.change(id, "bonus", bonus, "储值赠送")
}

// Consume 余额消费（金额单位：分）；余额不足返回 ErrInsufficient。
func (s *Service) Consume(id, amount int64, note string) (Member, error) {
	if amount <= 0 {
		return Member{}, errBad("消费金额必须大于 0")
	}
	return s.change(id, "consume", -amount, note)
}

// change 在一个事务里改余额并写流水。
func (s *Service) change(id int64, kind string, delta int64, note string) (Member, error) {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Member{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		balance int64
		active  int
	)
	if err := tx.QueryRow("SELECT balance, active FROM t_pos_member WHERE id=?", id).
		Scan(&balance, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Member{}, ErrNotFound
		}
		return Member{}, err
	}
	if active != 1 && delta < 0 {
		return Member{}, ErrInactive
	}
	next := balance + delta
	if next < 0 {
		return Member{}, ErrInsufficient
	}
	if _, err := tx.Exec("UPDATE t_pos_member SET balance=? WHERE id=?", next, id); err != nil {
		return Member{}, err
	}
	if _, err := tx.Exec(
		"INSERT INTO t_pos_member_txn(member_id,kind,amount,balance_after,note,created_at)"+
			" VALUES(?,?,?,?,?,datetime('now','localtime'))",
		id, kind, abs(delta), next, note); err != nil {
		return Member{}, err
	}
	if err := tx.Commit(); err != nil {
		return Member{}, err
	}
	return s.Get(id)
}

// ConsumeTx 在调用方事务内扣余额并写流水（订单余额支付用，保证与订单同一事务）。
func (s *Service) ConsumeTx(tx *sql.Tx, id, amount, shiftID int64, note string) error {
	if amount <= 0 {
		return errBad("消费金额必须大于 0")
	}
	var (
		balance int64
		active  int
	)
	if err := tx.QueryRow("SELECT balance, active FROM t_pos_member WHERE id=?", id).
		Scan(&balance, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if active != 1 {
		return ErrInactive
	}
	next := balance - amount
	if next < 0 {
		return ErrInsufficient
	}
	if _, err := tx.Exec("UPDATE t_pos_member SET balance=? WHERE id=?", next, id); err != nil {
		return err
	}
	_, err := tx.Exec(
		"INSERT INTO t_pos_member_txn(member_id,kind,amount,balance_after,note,created_at)"+
			" VALUES(?,'consume',?,?,?,datetime('now','localtime'))",
		id, amount, next, note)
	return err
}

// RefundTx 在调用方事务内把退款打回余额并写流水。
func (s *Service) RefundTx(tx *sql.Tx, id, amount, shiftID int64, note string) error {
	if amount <= 0 {
		return errBad("退款金额必须大于 0")
	}
	var balance int64
	if err := tx.QueryRow("SELECT balance FROM t_pos_member WHERE id=?", id).Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	next := balance + amount
	if _, err := tx.Exec("UPDATE t_pos_member SET balance=? WHERE id=?", next, id); err != nil {
		return err
	}
	_, err := tx.Exec(
		"INSERT INTO t_pos_member_txn(member_id,kind,amount,balance_after,note,shift_id,created_at)"+
			" VALUES(?,'refund',?,?,?,?,datetime('now','localtime'))",
		id, amount, next, note, shiftID)
	return err
}

// AddPointsTx 在调用方事务内累加积分（1 元 = 1 分，向下取整）。
func (s *Service) AddPointsTx(tx *sql.Tx, id, amountCents, expireMonths, shiftID int64) error {
	if amountCents <= 0 {
		return nil
	}
	points := amountCents / 100
	if points <= 0 {
		return nil
	}
	// 有效期由调用方（HTTP 层读设置）传入，0 = 永不过期。
	// 注意：这里必须在事务内避免再查库，SQLite 单连接会自锁。
	if expireMonths > 0 {
		if _, err := tx.Exec(
			"UPDATE t_pos_member SET points=points+?, points_notified_at=NULL,"+
				" points_expire_at=date('now', '+' || ? || ' months') WHERE id=?",
			points, expireMonths, id); err != nil {
			return err
		}
	} else if _, err := tx.Exec("UPDATE t_pos_member SET points=points+? WHERE id=?", points, id); err != nil {
		return err
	}
	var after int64
	_ = tx.QueryRow("SELECT points FROM t_pos_member WHERE id=?", id).Scan(&after)
	_, err := tx.Exec(
		"INSERT INTO t_pos_member_txn(member_id,kind,amount,balance_after,note,shift_id,created_at)"+
			" VALUES(?,'points_earn',?,?,?,?,datetime('now','localtime'))",
		id, points, after, "消费积分", shiftID)
	return err
}

// SpendPointsTx 在调用方事务内扣积分（100 积分 = 1 元，由调用方换算）。
func (s *Service) SpendPointsTx(tx *sql.Tx, id, points, shiftID int64, note string) error {
	if points <= 0 {
		return errBad("抵扣积分必须大于 0")
	}
	var (
		cur    int64
		active int
	)
	if err := tx.QueryRow("SELECT points, active FROM t_pos_member WHERE id=?", id).Scan(&cur, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if active != 1 {
		return ErrInactive
	}
	if cur < points {
		return ErrInsufficient
	}
	if _, err := tx.Exec("UPDATE t_pos_member SET points=points-? WHERE id=?", points, id); err != nil {
		return err
	}
	var after int64
	_ = tx.QueryRow("SELECT points FROM t_pos_member WHERE id=?", id).Scan(&after)
	_, err := tx.Exec(
		"INSERT INTO t_pos_member_txn(member_id,kind,amount,balance_after,note,shift_id,created_at)"+
			" VALUES(?,'points_use',?,?,?,?,datetime('now','localtime'))",
		id, points, after, note, shiftID)
	return err
}

// RefundPointsTx 在调用方事务内把积分退回。
func (s *Service) RefundPointsTx(tx *sql.Tx, id, points, shiftID int64, note string) error {
	if points <= 0 {
		return nil
	}
	if _, err := tx.Exec("UPDATE t_pos_member SET points=points+? WHERE id=?", points, id); err != nil {
		return err
	}
	var after int64
	_ = tx.QueryRow("SELECT points FROM t_pos_member WHERE id=?", id).Scan(&after)
	_, err := tx.Exec(
		"INSERT INTO t_pos_member_txn(member_id,kind,amount,balance_after,note,shift_id,created_at)"+
			" VALUES(?,'points_refund',?,?,?,?,datetime('now','localtime'))",
		id, points, after, note, shiftID)
	return err
}

// Txns 返回最近流水。
func (s *Service) Txns(id int64, limit int) ([]Txn, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.SQL().Query(
		"SELECT kind,amount,balance_after,note,created_at FROM t_pos_member_txn"+
			" WHERE member_id=? ORDER BY id DESC LIMIT ?", id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Txn
	for rows.Next() {
		var t Txn
		if err := rows.Scan(&t.Kind, &t.Amount, &t.BalanceAfter, &t.Note, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// validPhone 校验手机号（11 位、1 开头）；非大陆号可后续放开。
func validPhone(p string) bool {
	if len(p) != 11 || p[0] != '1' {
		return false
	}
	for _, c := range p {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func errBad(msg string) error {
	return &badRequest{msg}
}

type badRequest struct{ msg string }

func (e *badRequest) Error() string { return e.msg }
func (e *badRequest) Is(target error) bool { return target == ErrBadRequest }

// Tier 是一个会员等级（阈值单位：分；折扣为百分比；积分倍率为整数）。
type Tier struct {
	Threshold  int64
	Name       string
	Discount   int64
	PointRate  int64
}

// Tiers 从设置里解析等级表（解析失败时给一个默认等级，不影响收银）。
func (s *Service) Tiers() []Tier {
	raw := ""
	_ = s.db.SQL().QueryRow("SELECT v FROM t_pos_setting WHERE k='member_tiers'").Scan(&raw)
	if strings.TrimSpace(raw) == "" {
		raw = "0:普通会员:0:1"
	}
	var out []Tier
	for _, part := range strings.Split(raw, ",") {
		f := strings.Split(strings.TrimSpace(part), ":")
		if len(f) != 4 {
			continue
		}
		th, _ := strconv.ParseInt(strings.TrimSpace(f[0]), 10, 64)
		disc, _ := strconv.ParseInt(strings.TrimSpace(f[2]), 10, 64)
		rate, _ := strconv.ParseInt(strings.TrimSpace(f[3]), 10, 64)
		if rate <= 0 {
			rate = 1
		}
		out = append(out, Tier{Threshold: th * 100, Name: strings.TrimSpace(f[1]), Discount: disc, PointRate: rate})
	}
	if len(out) == 0 {
		out = []Tier{{Threshold: 0, Name: "普通会员", Discount: 0, PointRate: 1}}
	}
	return out
}

// TierOf 按累计消费匹配等级（取不超过累计消费的最大阈值）。
func (s *Service) TierOf(m Member) Tier {
	best := Tier{Name: "普通会员", PointRate: 1}
	for _, t := range s.Tiers() {
		if m.TotalSpent >= t.Threshold {
			best = t
		}
	}
	return best
}

// AddSpentTx 在调用方事务内累加累计消费（等级依据）。
func (s *Service) AddSpentTx(tx *sql.Tx, id, amount int64) error {
	_, err := tx.Exec("UPDATE t_pos_member SET total_spent=total_spent+? WHERE id=?", amount, id)
	return err
}

// MarkPointsNotified 记录"已在小票上提醒过"，避免同一顾客每单都被提醒。
func (s *Service) MarkPointsNotified(id int64) error {
	_, err := s.db.SQL().Exec(
		"UPDATE t_pos_member SET points_notified_at=date('now','localtime') WHERE id=?", id)
	return err
}

// ExpiringSoon 返回 days 天内积分就要过期的会员（到期提醒）。
func (s *Service) ExpiringSoon(days int) ([]Member, error) {
	if days <= 0 || days > 90 {
		days = 7
	}
	rows, err := s.db.SQL().Query(
		"SELECT id,phone,name,balance,points,total_spent,active,COALESCE(points_expire_at,''),COALESCE(points_notified_at,'') FROM t_pos_member"+
			" WHERE points>0 AND points_expire_at IS NOT NULL AND points_expire_at<>''"+
			" AND points_expire_at<=date('now','+' || ? || ' days') ORDER BY points_expire_at", days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var mm Member
		if err := rows.Scan(&mm.ID, &mm.Phone, &mm.Name, &mm.Balance, &mm.Points, &mm.TotalSpent, &mm.Active, &mm.PointsExpireAt, &mm.PointsNotifiedAt); err != nil {
			return nil, err
		}
		out = append(out, mm)
	}
	return out, rows.Err()
}

// ExpirePoints 把过期会员的积分清零并记流水；返回处理人数。
func (s *Service) ExpirePoints(now string) (int, error) {
	rows, err := s.db.SQL().Query(
		"SELECT id, points FROM t_pos_member WHERE points>0"+
			" AND points_expire_at IS NOT NULL AND points_expire_at<>'' AND points_expire_at<=?", now)
	if err != nil {
		return 0, err
	}
	type hit struct {
		id     int64
		points int64
	}
	var hits []hit
	for rows.Next() {
		var h hit
		if err := rows.Scan(&h.id, &h.points); err != nil {
			_ = rows.Close()
			return 0, err
		}
		hits = append(hits, h)
	}
	_ = rows.Close()

	for _, h := range hits {
		tx, err := s.db.SQL().Begin()
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec("UPDATE t_pos_member SET points=0, points_expire_at=NULL WHERE id=?", h.id); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_member_txn(member_id,kind,amount,balance_after,note,created_at)"+
				" VALUES(?,'points_expire',?,0,'积分过期',datetime('now','localtime'))", h.id, h.points); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
	}
	return len(hits), nil
}
