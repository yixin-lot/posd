// Package report 提供日结与商品销量统计（只读）。
//
// 口径与班次一致：
//   - 销售额不含作废单（status='void'）；
//   - 净销售 = 销售 - 退款（退款含按件与整单，含作废产生的退款流水）；
//   - 商品销量按「售出数量 - 已退数量」计。
package report

import (
	"strconv"
	"errors"

	"pos/internal/store"
)

// ProductStat 是商品销量行。
type ProductStat struct {
	Name    string
	Qty     int
	Amount  int64
}

// MethodStat 是按支付方式的汇总。
type MethodStat struct {
	Method string
	Amount int64
}

// Daily 是某营业日的汇总（金额单位：分）。
type Daily struct {
	BizDate     string
	OrderCount  int
	GrossSales  int64
	RefundTotal int64
	NetSales    int64
	VoidCount   int
	VoidTotal   int64 // 作废金额（含作废产生的退款流水）
	AvgTicket   int64
	CashNet     int64
	ByMethod    []MethodStat
	// 会员维度
	MemberAmount  int64 // 会员支付的销售额
	MemberDiscountTotal int64 // 会员等级折扣合计（商家让利）
	MemberOrders  int   // 涉及会员支付的订单数
	PointsEarned  int64 // 当日发放积分
	PointsUsed    int64 // 当日核销积分
	TopProducts []ProductStat
	// 毛利：按商品当前进价估算（订单不存成本快照）——用于看趋势，不做账。
	GrossProfit int64
	// Hourly 是 0-23 点销售额（分），用于看时段高峰。
	Hourly [24]int64
}

// Service 查询统计。
type Service struct {
	db *store.DB
}

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// Daily 汇总某营业日（bizDate 形如 20261103）。
func (s *Service) Daily(bizDate string) (Daily, error) {
	d := Daily{BizDate: bizDate}

	if err := s.db.SQL().QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(payable_amount),0), COALESCE(SUM(cash_amount-change_amount),0)"+
			" FROM t_pos_order WHERE biz_date=? AND status NOT IN ('void','held','cancelled')", bizDate).
		Scan(&d.OrderCount, &d.GrossSales, &d.CashNet); err != nil {
		return Daily{}, err
	}
	if err := s.db.SQL().QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(refund_amount),0) FROM t_pos_order"+
			" WHERE biz_date=? AND status='void'", bizDate).
		Scan(&d.VoidCount, &d.VoidTotal); err != nil {
		return Daily{}, err
	}
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(r.amount),0) FROM t_pos_refund r JOIN t_pos_order o ON o.id=r.order_id"+
			" WHERE o.biz_date=? AND r.kind='refund'", bizDate).Scan(&d.RefundTotal); err != nil {
		return Daily{}, err
	}
	d.NetSales = d.GrossSales - d.RefundTotal
	if d.OrderCount > 0 {
		d.AvgTicket = d.NetSales / int64(d.OrderCount)
	}

	payRows, err := s.db.SQL().Query(
		"SELECT p.method, COALESCE(SUM(p.amount),0) FROM t_pos_payment p"+
			" JOIN t_pos_order o ON o.id=p.order_id"+
			" WHERE o.biz_date=? AND o.status<>'void' GROUP BY p.method ORDER BY p.method", bizDate)
	if err != nil {
		return Daily{}, err
	}
	for payRows.Next() {
		var m MethodStat
		if err := payRows.Scan(&m.Method, &m.Amount); err != nil {
			_ = payRows.Close()
			return Daily{}, err
		}
		d.ByMethod = append(d.ByMethod, m)
	}
	_ = payRows.Close()

	// 会员消费：会员余额或积分支付的金额（按订单去重，避免组合支付重复计数）
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(o.payable_amount),0), COUNT(DISTINCT o.id)"+
			" FROM t_pos_order o WHERE o.biz_date=? AND o.status NOT IN ('void','held','cancelled')"+
			" AND EXISTS (SELECT 1 FROM t_pos_payment p WHERE p.order_id=o.id AND p.method IN ('member','points'))",
		bizDate).Scan(&d.MemberAmount, &d.MemberOrders); err != nil {
		return Daily{}, err
	}
	// 会员等级折扣：单独列示，避免和"会员消费"混在一起
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(member_discount),0) FROM t_pos_order"+
			" WHERE biz_date=? AND status NOT IN ('void','held','cancelled')", bizDate).
		Scan(&d.MemberDiscountTotal); err != nil {
		return Daily{}, err
	}

	// 积分发放/核销：按流水统计（当天）。bizDate 是 20261103，这里转成 2026-11-03 再比。
	sqlDate := bizDate
	if len(bizDate) == 8 {
		sqlDate = bizDate[0:4] + "-" + bizDate[4:6] + "-" + bizDate[6:8]
	}
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(CASE WHEN kind='points_earn' THEN amount ELSE 0 END),0),"+
			" COALESCE(SUM(CASE WHEN kind='points_use' THEN amount ELSE 0 END),0)"+
			" FROM t_pos_member_txn WHERE substr(created_at,1,10)=?", sqlDate).
		Scan(&d.PointsEarned, &d.PointsUsed); err != nil {
		return Daily{}, err
	}

	rows, err := s.db.SQL().Query(
		"SELECT i.name, SUM(i.qty - i.refunded_qty), SUM(i.amount - i.price*i.refunded_qty)"+
			" FROM t_pos_order_item i JOIN t_pos_order o ON o.id=i.order_id"+
			" WHERE o.biz_date=? AND o.status<>'void'"+
			" GROUP BY i.product_id, i.name ORDER BY 3 DESC LIMIT 10", bizDate)
	if err != nil {
		return Daily{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var p ProductStat
		if err := rows.Scan(&p.Name, &p.Qty, &p.Amount); err != nil {
			return Daily{}, err
		}
		if p.Qty > 0 {
			d.TopProducts = append(d.TopProducts, p)
		}
	}
	if err := s.fillProfitAndHourly(&d, "o.biz_date=?", bizDate); err != nil {
		return Daily{}, err
	}
	return d, nil
}

// fillProfitAndHourly 补算毛利与时段分布。
// 订单不存成本快照，毛利用商品**当前进价**估算——看趋势可以，不能当账。
func (s *Service) fillProfitAndHourly(d *Daily, where string, arg any) error {
	rows, err := s.db.SQL().Query(
		"SELECT i.qty, i.amount, COALESCE(p.cost,0), substr(o.created_at,12,2)"+
			" FROM t_pos_order_item i JOIN t_pos_order o ON o.id=i.order_id"+
			" LEFT JOIN t_pos_product p ON p.id=i.product_id"+
			" WHERE "+where+" AND o.status NOT IN ('void','held','cancelled')", arg)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var qty, amount, cost int64
		var hour string
		if err := rows.Scan(&qty, &amount, &cost, &hour); err != nil {
			return err
		}
		d.GrossProfit += amount - cost*qty
		if h, err := strconv.Atoi(hour); err == nil && h >= 0 && h < 24 {
			d.Hourly[h] += amount
		}
	}
	return rows.Err()
}

func (s *Service) fillProfitAndHourlyMonth(d *Daily, month string) error {
	from := month[:4] + month[5:7]
	// biz_date 形如 20261103，按月取前 6 位
	rows, err := s.db.SQL().Query(
		"SELECT i.qty, i.amount, COALESCE(p.cost,0), substr(o.created_at,12,2)"+
			" FROM t_pos_order_item i JOIN t_pos_order o ON o.id=i.order_id"+
			" LEFT JOIN t_pos_product p ON p.id=i.product_id"+
			" WHERE substr(o.biz_date,1,6)=? AND o.status NOT IN ('void','held','cancelled')", from)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var qty, amount, cost int64
		var hour string
		if err := rows.Scan(&qty, &amount, &cost, &hour); err != nil {
			return err
		}
		d.GrossProfit += amount - cost*qty
		if h, err := strconv.Atoi(hour); err == nil && h >= 0 && h < 24 {
			d.Hourly[h] += amount
		}
	}
	return rows.Err()
}

// Monthly 汇总某月（month = 2026-11），复用 Daily 的字段口径。
func (s *Service) Monthly(month string) (Daily, error) {
	if len(month) != 7 {
		return Daily{}, errors.New("月份格式应为 YYYY-MM")
	}
	from := month + "-01"
	to := month + "-31"

	out := Daily{BizDate: month}
	if err := s.db.SQL().QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(payable_amount),0), COALESCE(SUM(cash_amount-change_amount),0)"+
			" FROM t_pos_order WHERE substr(biz_date,1,4)||'-'||substr(biz_date,5,2)=?"+
			" AND status NOT IN ('void','held','cancelled')", month).
		Scan(&out.OrderCount, &out.GrossSales, &out.CashNet); err != nil {
		return Daily{}, err
	}
	if err := s.db.SQL().QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(refund_amount),0) FROM t_pos_order"+
			" WHERE substr(biz_date,1,4)||'-'||substr(biz_date,5,2)=? AND status='void'", month).
		Scan(&out.VoidCount, &out.VoidTotal); err != nil {
		return Daily{}, err
	}
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(r.amount),0) FROM t_pos_refund r JOIN t_pos_order o ON o.id=r.order_id"+
			" WHERE substr(o.biz_date,1,4)||'-'||substr(o.biz_date,5,2)=? AND r.kind='refund'", month).
		Scan(&out.RefundTotal); err != nil {
		return Daily{}, err
	}
	out.NetSales = out.GrossSales - out.RefundTotal
	if out.OrderCount > 0 {
		out.AvgTicket = out.NetSales / int64(out.OrderCount)
	}
	_ = from
	_ = to
	if err := s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(member_discount),0) FROM t_pos_order"+
			" WHERE substr(biz_date,1,4)||'-'||substr(biz_date,5,2)=? AND status NOT IN ('void','held','cancelled')", month).
		Scan(&out.MemberDiscountTotal); err != nil {
		return Daily{}, err
	}

	rows, err := s.db.SQL().Query(
		"SELECT p.method, COALESCE(SUM(p.amount),0) FROM t_pos_payment p"+
			" JOIN t_pos_order o ON o.id=p.order_id"+
			" WHERE substr(o.biz_date,1,4)||'-'||substr(o.biz_date,5,2)=? AND o.status<>'void'"+
			" GROUP BY p.method ORDER BY p.method", month)
	if err != nil {
		return Daily{}, err
	}
	for rows.Next() {
		var m MethodStat
		if err := rows.Scan(&m.Method, &m.Amount); err != nil {
			_ = rows.Close()
			return Daily{}, err
		}
		out.ByMethod = append(out.ByMethod, m)
	}
	_ = rows.Close()

	top, err := s.db.SQL().Query(
		"SELECT i.name, SUM(i.qty - i.refunded_qty), SUM(i.amount - i.price*i.refunded_qty)"+
			" FROM t_pos_order_item i JOIN t_pos_order o ON o.id=i.order_id"+
			" WHERE substr(o.biz_date,1,4)||'-'||substr(o.biz_date,5,2)=? AND o.status<>'void'"+
			" GROUP BY i.product_id, i.name ORDER BY 3 DESC LIMIT 20", month)
	if err != nil {
		return Daily{}, err
	}
	defer top.Close()
	for top.Next() {
		var p ProductStat
		if err := top.Scan(&p.Name, &p.Qty, &p.Amount); err != nil {
			return Daily{}, err
		}
		if p.Qty > 0 {
			out.TopProducts = append(out.TopProducts, p)
		}
	}
	if err := s.fillProfitAndHourlyMonth(&out, month); err != nil {
		return Daily{}, err
	}
	return out, top.Err()
}
