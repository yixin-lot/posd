// Package shift 是交接班（P4）：开班、结班、对账汇总。
package shift

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"pos/internal/store"
)

var (
	// ErrNotFound 班次不存在。
	ErrNotFound = errors.New("班次不存在")
	// ErrConflict 已有未结班次 / 已结班。
	ErrConflict = errors.New("班次状态冲突")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
)

// Payment 是收款方式小计。
type Payment struct {
	Method string
	Amount int64
}

// Shift 是班次。
type Shift struct {
	ID          int64
	ShiftNo     string
	Status      string
	StaffName   string
	OpeningCash int64
	Expected    int64
	Actual      int64
	Difference  int64
	OrderCount  int
	OpenedAt    string
	ClosedAt    string
	Note        string
	Total       int64
	CashNet     int64
	RefundTotal int64 // 本班次退款合计（现金退款会减少钱箱）
	VoidTotal   int64 // 本班次作废产生的退款（同样减少钱箱，但口径独立）
	DineInCount int   // 堂食单数（有桌台）
	TakeawayCnt int   // 外带单数（无桌台）
	MemberDiscountTotal int64 // 会员等级折扣合计（商家让利）
	PointsEarned int64 // 本班次发放积分
	CrossRefundCount int   // 本班次处理的、原单不属于本班次的退款笔数
	CrossRefundTotal int64 // 上述退款金额（分）
	PointsUsed   int64 // 本班次核销积分（积分抵扣）
	Payments    []Payment
}

// Service 提供交接班用例。
type Service struct {
	db  *store.DB
	now func() time.Time
	// onClosed 在结班后触发（打印交接单）；失败不影响结班。
	onClosed func(Shift)
}

// OnClosed 注册结班后置动作。
func (s *Service) OnClosed(fn func(Shift)) { s.onClosed = fn }

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db, now: time.Now} }

// Open 开班；已有未结班次时返回 ErrConflict。
func (s *Service) Open(openingCash float64, staffName string) (Shift, error) {
	if _, found, err := s.Latest(); err != nil {
		return Shift{}, err
	} else if found {
		var status string
		if err := s.db.SQL().QueryRow("SELECT status FROM t_pos_shift ORDER BY id DESC LIMIT 1").Scan(&status); err == nil && status == "open" {
			return Shift{}, fmt.Errorf("%w: 已有未结班次，请先结班", ErrConflict)
		}
	}

	now := s.now()
	bizDate := now.Format("20060102")
	if now.Hour() < 4 {
		bizDate = now.AddDate(0, 0, -1).Format("20060102")
	}
	var seq int
	if err := s.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_shift WHERE shift_no LIKE ?", "S"+bizDate+"-%").Scan(&seq); err != nil {
		return Shift{}, err
	}
	shiftNo := fmt.Sprintf("S%s-%03d", bizDate, seq+1)

	if openingCash < 0 {
		return Shift{}, fmt.Errorf("%w: 期初现金不能为负", ErrBadRequest)
	}
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_shift(shop_id,shift_no,staff_name,status,opening_cash,opened_at)"+
			" VALUES(1,?,?,'open',?,?)",
		shiftNo, staffName, int64(openingCash*100), now.Format("2006-01-02 15:04:05"))
	if err != nil {
		return Shift{}, err
	}
	id, _ := res.LastInsertId()
	return s.byID(id)
}

// Latest 返回最近一个班次（可能是已结班）。
func (s *Service) Latest() (Shift, bool, error) {
	var id int64
	err := s.db.SQL().QueryRow("SELECT id FROM t_pos_shift ORDER BY id DESC LIMIT 1").Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Shift{}, false, nil
	}
	if err != nil {
		return Shift{}, false, err
	}
	sh, err := s.byID(id)
	return sh, err == nil, err
}

// Close 结班：算现金应收、差异，写回。
func (s *Service) Close(id int64, actualCash float64, note string) (Shift, error) {
	sh, err := s.byID(id)
	if err != nil {
		return Shift{}, err
	}
	if sh.Status != "open" {
		return Shift{}, fmt.Errorf("%w: 该班次已结班", ErrConflict)
	}
	// 现金应收 = 期初 + 收款 - 退款（退款把现金拿出去了）。
	expected := sh.OpeningCash + sh.CashNet - sh.RefundTotal - sh.VoidTotal
	actual := int64(actualCash * 100)
	diff := actual - expected

	if _, err := s.db.SQL().Exec(
		"UPDATE t_pos_shift SET status='closed',expected_cash=?,actual_cash=?,difference=?,order_count=?,note=?,"+
			"closed_at=? WHERE id=?",
		expected, actual, diff, sh.OrderCount, note, s.now().Format("2006-01-02 15:04:05"), id); err != nil {
		return Shift{}, err
	}
	closed, err := s.byID(id)
	if err != nil {
		return Shift{}, err
	}
	if s.onClosed != nil {
		s.onClosed(closed)
	}
	return closed, nil
}

func (s *Service) byID(id int64) (Shift, error) {
	var (
		sh         Shift
		expected   sql.NullInt64
		actual     sql.NullInt64
		difference sql.NullInt64
		orderCount sql.NullInt64
		closedAt   sql.NullString
	)
	err := s.db.SQL().QueryRow(
		"SELECT id,shift_no,status,staff_name,opening_cash,expected_cash,actual_cash,difference,order_count,"+
			"opened_at,closed_at,note FROM t_pos_shift WHERE id=?", id).
		Scan(&sh.ID, &sh.ShiftNo, &sh.Status, &sh.StaffName, &sh.OpeningCash, &expected, &actual, &difference,
			&orderCount, &sh.OpenedAt, &closedAt, &sh.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return Shift{}, ErrNotFound
	}
	if err != nil {
		return Shift{}, err
	}
	sh.Expected, sh.Actual, sh.Difference = expected.Int64, actual.Int64, difference.Int64
	sh.OrderCount = int(orderCount.Int64) // 开班时为 NULL；下面会用订单汇总覆盖
	sh.ClosedAt = closedAt.String

	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(payable_amount),0), COUNT(*) FROM t_pos_order"+
			" WHERE shift_id=? AND status NOT IN ('void','held','cancelled')", id).
		Scan(&sh.Total, &sh.OrderCount); err != nil {
		return Shift{}, err
	}
	// 堂食/外带分开统计（按订单是否绑了桌台）。
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(CASE WHEN table_name<>'' THEN 1 ELSE 0 END),0),"+
			" COALESCE(SUM(CASE WHEN table_name='' THEN 1 ELSE 0 END),0)"+
			" FROM t_pos_order WHERE shift_id=? AND status NOT IN ('void','held','cancelled')", id).
		Scan(&sh.DineInCount, &sh.TakeawayCnt); err != nil {
		return Shift{}, err
	}

	// 会员等级折扣：单独一行，避免和"会员消费"混在一起
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(member_discount),0) FROM t_pos_order"+
			" WHERE shift_id=? AND status NOT IN ('void','held','cancelled')", id).
		Scan(&sh.MemberDiscountTotal); err != nil {
		return Shift{}, err
	}

	// 积分发放/核销：优先按 shift_id（精确）；历史数据 shift_id=0 的退回时间窗。
	endAt := "9999-12-31 23:59:59"
	if sh.ClosedAt != "" {
		endAt = sh.ClosedAt
	}
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(CASE WHEN kind='points_earn' THEN amount ELSE 0 END),0),"+
			" COALESCE(SUM(CASE WHEN kind='points_use' THEN amount ELSE 0 END),0)"+
			" FROM t_pos_member_txn"+
			" WHERE shift_id=? OR (shift_id=0 AND created_at>=? AND created_at<=?)",
		id, sh.OpenedAt, endAt).Scan(&sh.PointsEarned, &sh.PointsUsed); err != nil {
		return Shift{}, err
	}

	// 跨班次/跨天退款：这笔退款记在本班次，但原单属于别的班次。
	if err := s.db.SQL().QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(amount),0) FROM t_pos_refund"+
			" WHERE created_at>=? AND created_at<=? AND shift_id<>? AND shift_id<>0",
		sh.OpenedAt, endAt, id).Scan(&sh.CrossRefundCount, &sh.CrossRefundTotal); err != nil {
		return Shift{}, err
	}

	// 退款与作废分开统计：钱都要从钱箱拿走，但纸面口径不同。
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(CASE WHEN r.kind='refund' THEN r.amount ELSE 0 END),0),"+
			" COALESCE(SUM(CASE WHEN r.kind='void' THEN r.amount ELSE 0 END),0)"+
			" FROM t_pos_refund r JOIN t_pos_order o ON o.id=r.order_id WHERE o.shift_id=?", id).
		Scan(&sh.RefundTotal, &sh.VoidTotal); err != nil {
		return Shift{}, err
	}
	rows, err := s.db.SQL().Query(
		"SELECT p.method, COALESCE(SUM(p.amount),0) FROM t_pos_payment p JOIN t_pos_order o ON o.id=p.order_id"+
			" WHERE o.shift_id=? GROUP BY p.method", id)
	if err != nil {
		return Shift{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.Method, &p.Amount); err != nil {
			return Shift{}, err
		}
		sh.Payments = append(sh.Payments, p)
		if p.Method == "cash" {
			sh.CashNet = p.Amount
		}
	}
	return sh, rows.Err()
}
