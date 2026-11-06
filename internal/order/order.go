// Package order 是订单域（P4）：算钱、幂等、挂班次、查询。
//
// 金额全程用「分」的 int64；HTTP 边界才转成元。前端传的价格一律忽略。
package order

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"pos/internal/catalog"
	"pos/internal/member"
	"pos/internal/stock"
	"pos/internal/store"
)

var (
	// ErrNotFound 订单不存在。
	ErrNotFound = errors.New("订单不存在")
	// ErrBadRequest 参数或业务校验失败。
	ErrBadRequest = errors.New("参数不合法")
)

// ItemInput 是下单行。Price 非空表示改价（单位：元），需权限。
// Choice 是套餐可选项的选择结果（group → 选中的组件）。
type Choice struct {
	GroupID     int64
	ComponentID int64
}

type ItemInput struct {
	// Choices 只对套餐有效：每个可选项组选一个组件；不传则取组内第一个。
	Choices []Choice
	ProductID int64
	Qty       int
	Price     *float64
}

// PaymentInput 是一笔支付（金额单位：元）。
//
// 记账式支付：微信/支付宝只记录方式与金额，不对账、不接通道（ADR-004）。
type PaymentInput struct {
	Method   string // cash / wechat / alipay / member
	Amount   float64
	MemberID int64 // method=member 时必填
}

// CreateInput 是下单入参（金额单位：元）。
type CreateInput struct {
	IdempotencyKey string
	Cash           float64
	Discount       float64
	Payments       []PaymentInput
	Items          []ItemInput
	// 积分规则（由 HTTP 层按设置传入）：每元得分、多少分抵 1 元。
	PointsEarnPerYuan   int64
	PointsPerYuan       int64
	// MemberID 选中的会员（可空）；MemberDiscountPct 由 HTTP 层按等级给出。
	MemberID          int64
	MemberDiscountPct int64
	MemberPointRate   int64
	PointsExpireMonths int64
	// StockBlockWhenShort 为 true 时，库存不足直接拒绝下单。
	StockBlockWhenShort bool
	// RoundMode 由 HTTP 层按门店设置传入：none/jiao/yuan。
	RoundMode string
	// Hold=true 表示挂单：先落库、不收款、不出票。
	Hold bool
	// AllowPriceOverride 由 HTTP 层按权限决定；false 时带改价直接拒绝。
	AllowPriceOverride bool
	Operator           string
}

// Item 是订单明细。
type Item struct {
	ProductID   string
	Name        string
	Price       int64 // 实际成交价（可能被改）
	OriginalPrice int64 // 目录价（改价时留存）
	Overridden  bool  // 是否改过价
	Qty         int
	Amount      int64
	RefundedQty int // 已退数量（部分退款用）
	// Category 是下单时的商品类目快照（后厨按它分档口，改类目不影响历史单）
	Category string
	// Components 是套餐的组件快照（单品为空）。后厨按组件分单；
	// 存快照而不是实时查，保证商品改价/下架不影响历史订单。
	Components []ItemComponent
}

// ItemComponent 是套餐里的一个组件。
type ItemComponent struct {
	ComponentID string `json:"componentId"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Qty         int    `json:"qty"`
}

// Payment 是一笔支付记录。
type Payment struct {
	Method   string
	Amount   int64
	MemberID int64
}

// Order 是订单。
type Order struct {
	ID          int64
	OrderNo     string
	BizDate     string
	Status      string
	Total       int64
	Discount    int64
	Payable     int64
	Cash        int64
	Change      int64
	PrintStatus int
	CreatedAt   string
	Refunded    int64  // 已退金额（分）
	RefundedAt  string // 退款时间，未退款为空
	HeldAt      string // 挂单时间（未付款）
	TableName   string // 桌台名（堂食，空=非堂食）
	RoundOff    int64  // 抹零金额（分，商家让利）
	MemberDiscount int64 // 会员等级折扣（分）
	PointsEarned   int64 // 本单发放积分
	MemberID       int64
	ShiftID        int64
	Operator       string // 收银员（下单时的登录员工姓名，订单查询页展示）
	CrossDayFrom   string // 跨天结账时的原营业日（仅本次响应用）
	VoidedAt    string // 作废时间
	VoidReason  string
	VoidOperator string
	Payments    []Payment
	Items       []Item
}

// Service 提供订单用例。
type Service struct {
	db       *store.DB
	products *catalog.Service
	stock    *stock.Service
	members  *member.Service
	now      func() time.Time
	// onCreated 在订单提交后触发（用于入打印队列）；失败不影响下单。
	onCreated func(Order)
	// onVoid 在作废后触发（打印作废小票）；失败不影响作废。
	onVoid func(Order)

	// onReprint 在重打时触发（重新入打印队列）。
	onReprint func(Order)
}

// SetStock 注入库存服务（没有库存模块时传 nil，全部跳过）。
func (s *Service) SetStock(st *stock.Service) { s.stock = st }

// SetMembers 注入会员服务（余额支付用）。
func (s *Service) SetMembers(m *member.Service) { s.members = m }

// SetTable 把桌台名写到订单上（开台时调用）。
func (s *Service) SetTable(orderNo, tableName string) error {
	_, err := s.db.SQL().Exec("UPDATE t_pos_order SET table_name=? WHERE order_no=?", tableName, orderNo)
	return err
}

// OnVoid 注册作废动作。
func (s *Service) OnVoid(fn func(Order)) { s.onVoid = fn }

// OnReprint 注册重打动作。
func (s *Service) OnReprint(fn func(Order)) { s.onReprint = fn }

// OnCreated 注册下单后置动作（可多次覆盖）。
func (s *Service) OnCreated(fn func(Order)) { s.onCreated = fn }

// NewService 创建订单服务。
func NewService(db *store.DB, products *catalog.Service) *Service {
	return &Service{db: db, products: products, now: time.Now}
}

const dayCutHour = 4

// Create 下单：服务端重算金额 → 落库 → 挂班次。
//
// 返回值 created=false 表示幂等命中（没有新建），HTTP 层据此决定 201 还是 200。
func (s *Service) Create(in CreateInput) (Order, error) {
	o, _, err := s.CreateWithStatus(in)
	return o, err
}

// CreateWithStatus 同 Create，额外告诉调用方这次是否真的新建了订单。
func (s *Service) CreateWithStatus(in CreateInput) (Order, bool, error) {
	if len(in.Items) == 0 {
		return Order{}, false, fmt.Errorf("%w: 购物车为空", ErrBadRequest)
	}
	if in.IdempotencyKey != "" {
		if existing, err := s.findByIdem(in.IdempotencyKey); err == nil {
			return existing, false, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return Order{}, false, err
		}
	}

	var (
		total int64
		items []Item
	)
	for _, it := range in.Items {
		if it.Qty <= 0 {
			return Order{}, false, fmt.Errorf("%w: 数量必须大于 0", ErrBadRequest)
		}
		p, err := s.products.Get(it.ProductID)
		if err != nil {
			return Order{}, false, fmt.Errorf("%w: 商品不存在或已下架", ErrBadRequest)
		}
		if !p.Active {
			return Order{}, false, fmt.Errorf("%w: 商品已下架：%s", ErrBadRequest, p.Name)
		}
		// 单品改价：只有拿到权限才允许，且必须留存原价（审计在事务里写）。
		price := p.Price
		overridden := false
		if it.Price != nil {
			want := yuanToCents(*it.Price)
			if want <= 0 {
				return Order{}, false, fmt.Errorf("%w: 改价必须大于 0", ErrBadRequest)
			}
			if want != p.Price {
				if !in.AllowPriceOverride {
					return Order{}, false, fmt.Errorf("%w: 需要店长权限才能改价", ErrBadRequest)
				}
				price = want
				overridden = true
			}
		}
		amount := price * int64(it.Qty)
		total += amount
		// 套餐：把组件（含类目）快照下来，后厨按组件分档口
		var comps []ItemComponent
		if p.Kind == "bundle" {
			chosen := map[int64]int64{}
			for _, ch := range it.Choices {
				chosen[ch.GroupID] = ch.ComponentID
			}
			// 每个选项组只能出一个组件：优先用顾客选的，否则取组内第一个
			pickedGroups := map[int64]bool{}
			for _, b := range p.Bundle {
				if b.GroupID > 0 {
					if pickedGroups[b.GroupID] {
						continue
					}
					if want, ok := chosen[b.GroupID]; ok && want != b.ComponentID {
						continue
					}
					pickedGroups[b.GroupID] = true
				}
				cp, err := s.products.Get(b.ComponentID)
				if err != nil {
					return Order{}, false, fmt.Errorf("%w: 套餐组件不存在", ErrBadRequest)
				}
				comps = append(comps, ItemComponent{
					ComponentID: fmt.Sprint(b.ComponentID), Name: cp.Name,
					Category: cp.Category, Qty: b.Qty,
				})
			}
		}
		items = append(items, Item{
			ProductID: fmt.Sprint(p.ID), Name: p.Name, Price: price, OriginalPrice: p.Price,
			Overridden: overridden, Qty: it.Qty, Amount: amount,
			Category: p.Category, Components: comps,
		})
	}

	discount := yuanToCents(in.Discount)
	if discount < 0 {
		return Order{}, false, fmt.Errorf("%w: 优惠不能为负", ErrBadRequest)
	}
	payable := total - discount
	if payable < 0 {
		return Order{}, false, fmt.Errorf("%w: 优惠不能大于合计", ErrBadRequest)
	}
	// 会员等级折扣：在整单优惠之后再打折（服务端按等级算，客户端传不了）。
	memberDiscount := int64(0)
	if in.MemberID > 0 && in.MemberDiscountPct > 0 && in.MemberDiscountPct < 100 {
		memberDiscount = payable * in.MemberDiscountPct / 100
		payable -= memberDiscount
	}

	// 抹零：向下取整，差额单独记账（商家让利，不从折扣里扣）。
	roundOff := roundOffCents(payable, in.RoundMode)
	payable -= roundOff
	// 挂单不收款：状态 held，支付明细为空，不出票。
	payments := in.Payments
	var cashApplied, cash, change int64
	if !in.Hold {
		var err error
		payments, cash, change, err = resolvePayments(in.Payments, payable, in.Cash)
		if err != nil {
			return Order{}, false, err
		}
		cashApplied = cash - change
		_ = cashApplied
	}

	now := s.now()
	bizDate := bizDate(now)
	createdAt := now.Format("2006-01-02 15:04:05")

	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Order{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	// 班次先查出来：会员流水要记 shift_id，不能再靠时间窗估。
	var shiftID sql.NullInt64
	if err := tx.QueryRow("SELECT id FROM t_pos_shift WHERE status='open' ORDER BY id DESC LIMIT 1").Scan(&shiftID); err != nil {
		shiftID = sql.NullInt64{}
	}

	var seq int
	if err := tx.QueryRow("SELECT COUNT(*) FROM t_pos_order WHERE biz_date=?", bizDate).Scan(&seq); err != nil {
		return Order{}, false, err
	}
	orderNo := fmt.Sprintf("L%s-%04d", bizDate, seq+1)

	pointsEarned := int64(0)
	orderStatus := "paid"
	if in.Hold {
		orderStatus = "held"
	}
	res, err := tx.Exec(
		"INSERT INTO t_pos_order(shop_id,order_no,biz_date,status,total_amount,discount_amount,payable_amount,"+
			"cash_amount,change_amount,idem_key,print_status,created_at,held_at,round_off,member_discount,member_id,points_earned,operator)"+
			" VALUES(1,?,?,?,?,?,?,?,?,?,0,?,?,?,?,?,?,?)",
		orderNo, bizDate, orderStatus, total, discount, payable, cash, change, nullable(in.IdempotencyKey),
		createdAt, heldAt(in.Hold, createdAt), roundOff, memberDiscount, in.MemberID, pointsEarned, in.Operator)
	if err != nil {
		return Order{}, false, err
	}
	orderID, _ := res.LastInsertId()
	for _, it := range items {
		if it.Overridden {
			if _, err := tx.Exec(
				"INSERT INTO t_pos_audit(action,order_no,product_id,detail,operator,created_at)"+
					" VALUES('price_override',?,?,?,?,?)",
				orderNo, it.ProductID, fmt.Sprintf("%d->%d", it.OriginalPrice, it.Price),
				in.Operator, createdAt); err != nil {
				return Order{}, false, err
			}
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_order_item(order_id,product_id,name,price,original_price,price_overridden,qty,amount,components)"+
				" VALUES(?,?,?,?,?,?,?,?,?)",
			orderID, it.ProductID, it.Name, it.Price, it.OriginalPrice, boolInt(it.Overridden),
			it.Qty, it.Amount, componentsJSON(it.Components)); err != nil {
			return Order{}, false, err
		}
	}
	// 支付记录存「抵扣金额」，找零只记在订单上（否则班次对账会把找零算进流水）。
	// 会员规则统一走 MemberRules（与挂单收款同一套），避免两条路径口径分叉
	rules := MemberRules{
		PointsPerYuan:     in.PointsPerYuan,
		PointsEarnPerYuan: in.PointsEarnPerYuan,
		PointRate:         in.MemberPointRate,
		ExpireMonths:      in.PointsExpireMonths,
	}
	for _, p := range payments {
		amt := yuanToCents(p.Amount)
		if p.Method == "points" || p.Method == "member" {
			earn, err := s.applyMemberTx(tx, p, orderNo, shiftID.Int64, rules)
			if err != nil {
				return Order{}, false, err
			}
			pointsEarned += earn
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_payment(order_id,method,amount,created_at,member_id) VALUES(?,?,?,?,?)",
			orderID, p.Method, amt, createdAt, p.MemberID); err != nil {
			return Order{}, false, err
		}
	}
	// 积分是在支付循环里累加的，插入时还是 0，必须回写，
	// 否则 Get/List 读到的本单积分永远是 0（小票重打就丢行）。
	if _, err := tx.Exec("UPDATE t_pos_order SET points_earned=? WHERE id=?", pointsEarned, orderID); err != nil {
		return Order{}, false, err
	}
	// 库存：下单即预占（挂单也占），套餐按组件展开。
	if s.stock != nil {
		want := map[int64]int{}
		for _, it := range items {
			if len(it.Components) > 0 {
				for _, c := range it.Components {
					pid, _ := strconv.ParseInt(c.ComponentID, 10, 64)
					want[pid] += c.Qty * it.Qty
				}
				continue
			}
			pid, _ := strconv.ParseInt(it.ProductID, 10, 64)
			want[pid] += it.Qty
		}
		short, err := s.stock.DeductTx(tx, want, in.StockBlockWhenShort)
		if err != nil {
			return Order{}, false, err
		}
		if len(short) > 0 {
			return Order{}, false, fmt.Errorf("%w: 库存不足 %s", ErrConflict, strings.Join(short, "、"))
		}
	}

	if shiftID.Valid {
		if _, err := tx.Exec("UPDATE t_pos_order SET shift_id=? WHERE id=?", shiftID.Int64, orderID); err != nil {
			return Order{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Order{}, false, err
	}

	status := "paid"
	if in.Hold {
		status = "held"
	}
	created := Order{
		ID: orderID, OrderNo: orderNo, BizDate: bizDate, Status: status,
		Total: total, Discount: discount, Payable: payable, Cash: cash, Change: change, RoundOff: roundOff,
		PrintStatus: 0, CreatedAt: createdAt, Items: items,
		Payments: toPayments(payments),
		MemberDiscount: memberDiscount, MemberID: in.MemberID, PointsEarned: pointsEarned,
		ShiftID: shiftID.Int64,
	}
	if !in.Hold && s.onCreated != nil {
		// 打印失败不回滚订单：纸可以补打，账不能丢。挂单不打印。
		s.onCreated(created)
	}
	return created, true, nil
}

// AddItems 给挂单追加商品（桌台加菜）：重算合计与应收后写回。
func (s *Service) AddItems(orderNo string, items []ItemInput) (Order, error) {
	o, err := s.Get(orderNo)
	if err != nil {
		return Order{}, err
	}
	if o.Status != "held" {
		return Order{}, fmt.Errorf("%w: 只有挂单可以加菜（当前 %s）", ErrConflict, o.Status)
	}

	total := int64(0)
	type newItem struct {
		productID string
		name      string
		price     int64
		qty       int
		amount    int64
	}
	var add []newItem
	for _, it := range items {
		if it.Qty <= 0 {
			return Order{}, fmt.Errorf("%w: 数量必须大于 0", ErrBadRequest)
		}
		p, err := s.products.Get(it.ProductID)
		if err != nil || !p.Active {
			return Order{}, fmt.Errorf("%w: 商品不存在或已下架", ErrBadRequest)
		}
		price := p.Price
		if it.Price != nil {
			price = yuanToCents(*it.Price)
		}
		add = append(add, newItem{
			productID: fmt.Sprint(p.ID), name: p.Name, price: price, qty: it.Qty,
			amount: price * int64(it.Qty),
		})
		total += price * int64(it.Qty)
	}
	for _, it := range o.Items {
		total += it.Amount
	}

	// 应收 = 合计 - 整单折扣 - 会员折扣 - 抹零（抹零按合计重新算）
	payable := total - o.Discount - o.MemberDiscount
	roundOff := roundOffCents(payable, "none")
	if o.RoundOff > 0 {
		roundOff = roundOffCents(payable, "jiao")
	}
	payable -= roundOff

	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback() }()
	for _, it := range add {
		if _, err := tx.Exec(
			"INSERT INTO t_pos_order_item(order_id,product_id,name,price,original_price,price_overridden,qty,amount)"+
				" VALUES(?,?,?,?,?,0,?,?)",
			o.ID, it.productID, it.name, it.price, it.price, it.qty, it.amount); err != nil {
			return Order{}, err
		}
	}
	if _, err := tx.Exec(
		"UPDATE t_pos_order SET total_amount=?, payable_amount=?, round_off=? WHERE id=?",
		total, payable, roundOff, o.ID); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return s.Get(orderNo)
}

// MergeInto 并台：把 from 挂单的明细并入 to 挂单，from 置为作废（挂单无收款，不产生退款）。
func (s *Service) MergeInto(fromOrderNo, toOrderNo string) (Order, error) {
	from, err := s.Get(fromOrderNo)
	if err != nil {
		return Order{}, err
	}
	to, err := s.Get(toOrderNo)
	if err != nil {
		return Order{}, err
	}
	if from.Status != "held" || to.Status != "held" {
		return Order{}, fmt.Errorf("%w: 只有两张挂单可以并台", ErrConflict)
	}
	if from.OrderNo == to.OrderNo {
		return Order{}, fmt.Errorf("%w: 不能并到自己", ErrBadRequest)
	}

	items := make([]ItemInput, 0, len(from.Items))
	for _, it := range from.Items {
		pid, _ := strconv.ParseInt(it.ProductID, 10, 64)
		price := float64(it.Price) / 100
		items = append(items, ItemInput{ProductID: pid, Qty: it.Qty, Price: &price})
	}
	merged, err := s.AddItems(to.OrderNo, items)
	if err != nil {
		return Order{}, err
	}
	// 源单作废（无支付，不写退款流水）
	if _, err := s.db.SQL().Exec(
		"UPDATE t_pos_order SET status='void', void_reason='并台并入 '+?, voided_at=datetime('now','localtime')"+
			" WHERE id=?", to.OrderNo, from.ID); err != nil {
		return Order{}, err
	}
	_, _ = s.db.SQL().Exec(
		"INSERT INTO t_pos_audit(action,order_no,product_id,detail,operator,created_at)"+
			" VALUES('table_merge',?,'',?,'',datetime('now','localtime'))",
		from.OrderNo, "并入 "+to.OrderNo)
	return merged, nil
}

// TransferTable 转台：把桌台名写到订单（桌台表的绑定由调用方处理）。
func (s *Service) TransferTable(orderNo, tableName string) error {
	if _, err := s.Get(orderNo); err != nil {
		return err
	}
	return s.SetTable(orderNo, tableName)
}


// stockMap 把订单明细展开成「商品ID → 数量」，套餐展开成组件。
// only 非空时只统计这些商品；qty 为退款数量（<=0 表示整行）。
func stockMap(items []Item, only map[string]int) map[int64]int {
	out := map[int64]int{}
	hasOnly := len(only) > 0
	for _, it := range items {
		qty := it.Qty
		if hasOnly {
			want, ok := only[it.ProductID]
			if !ok {
				continue
			}
			if want > 0 && want < qty {
				qty = want
			}
		}
		if len(it.Components) > 0 {
			for _, c := range it.Components {
				pid, _ := strconv.ParseInt(c.ComponentID, 10, 64)
				out[pid] += c.Qty * qty
			}
			continue
		}
		pid, _ := strconv.ParseInt(it.ProductID, 10, 64)
		out[pid] += qty
	}
	return out
}

// MemberRules 是会员结算规则（由 HTTP 层按设置传入，缺省走默认值）。
//
// 为什么集中成一个结构：Create（即时结算）与 Pay（挂单后结算）必须用**同一套**
// 规则，否则同一笔消费在两条路径上算出的积分/抵扣不同——那是账务分叉。
type MemberRules struct {
	PointsPerYuan     int64 // 多少积分抵 1 元（默认 100）
	PointsEarnPerYuan int64 // 每消费 1 元得几分（默认 1）
	PointRate         int64 // 等级倍率（默认 1）
	ExpireMonths      int64 // 积分有效期（月，0=永不过期）
}

func (r MemberRules) redeemRate() int64 {
	if r.PointsPerYuan <= 0 {
		return 100
	}
	return r.PointsPerYuan
}

func (r MemberRules) earnPerYuan() int64 {
	if r.PointsEarnPerYuan <= 0 {
		return 1
	}
	return r.PointsEarnPerYuan
}

func (r MemberRules) rate() int64 {
	if r.PointRate <= 0 {
		return 1
	}
	return r.PointRate
}

// applyMemberTx 处理一笔会员支付（积分抵扣 / 余额扣款 + 累积积分 + 累计消费），
// 返回本单因此发放的积分个数。
func (s *Service) applyMemberTx(tx *sql.Tx, p PaymentInput, orderNo string, shiftID int64, r MemberRules) (int64, error) {
	if s.members == nil {
		return 0, fmt.Errorf("%w: 未启用会员功能", ErrBadRequest)
	}
	amt := yuanToCents(p.Amount)
	switch p.Method {
	case "points":
		need := amt * r.redeemRate() / 100
		if err := s.members.SpendPointsTx(tx, p.MemberID, need, shiftID, "订单 "+orderNo); err != nil {
			return 0, memberErr(err)
		}
		return 0, nil
	case "member":
		if err := s.members.ConsumeTx(tx, p.MemberID, amt, shiftID, "订单 "+orderNo); err != nil {
			return 0, memberErr(err)
		}
		earn := amt / 100 * r.earnPerYuan() * r.rate()
		if earn <= 0 {
			earn = amt / 100
		}
		if err := s.members.AddPointsTx(tx, p.MemberID, earn*100, r.ExpireMonths, shiftID); err != nil {
			return 0, err
		}
		if err := s.members.AddSpentTx(tx, p.MemberID, amt); err != nil {
			return 0, err
		}
		return earn, nil
	}
	return 0, nil
}

// Pay 给挂单收款：只允许 held 状态，收款后转 paid 并出票。
func (s *Service) Pay(orderNo string, in []PaymentInput, cashIn float64) (Order, error) {
	// 兼容旧调用：不带规则时按默认（100 分抵 1 元、1 元 1 分）
	return s.PayWith(orderNo, in, cashIn, MemberRules{})
}

// PayWith 是带会员规则版本的挂单收款。
func (s *Service) PayWith(orderNo string, in []PaymentInput, cashIn float64, rules MemberRules) (Order, error) {
	o, err := s.Get(orderNo)
	if err != nil {
		return Order{}, err
	}
	if o.Status != "held" {
		return Order{}, fmt.Errorf("%w: 只有挂单可以收款（当前状态 %s）", ErrConflict, o.Status)
	}
	payments, cash, change, err := resolvePayments(in, o.Payable, cashIn)
	if err != nil {
		return Order{}, err
	}
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := s.now().Format("2006-01-02 15:04:05")
	pointsEarned := int64(0)
	for _, p := range payments {
		amt := yuanToCents(p.Amount)
		if p.Method == "points" || p.Method == "member" {
			earn, err := s.applyMemberTx(tx, p, orderNo, o.ShiftID, rules)
			if err != nil {
				return Order{}, err
			}
			pointsEarned += earn
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_payment(order_id,method,amount,created_at,member_id) VALUES(?,?,?,?,?)",
			o.ID, p.Method, amt, now, p.MemberID); err != nil {
			return Order{}, err
		}
	}
	// 跨天结账：挂单是昨天（或更早）的单，今天才收钱。
	// 记账口径：钱哪天收到算哪天，所以把 biz_date 与 shift_id 挪到今天，
	// 并在审计里留原日期，避免"昨天的销售今天才入账"对不上账。
	today := bizDate(s.now())
	crossDay := o.BizDate != today
	if crossDay {
		var shiftID sql.NullInt64
		if err := tx.QueryRow("SELECT id FROM t_pos_shift WHERE status='open' ORDER BY id DESC LIMIT 1").Scan(&shiftID); err != nil {
			shiftID = sql.NullInt64{}
		}
		if _, err := tx.Exec(
			"UPDATE t_pos_order SET status='paid', cash_amount=?, change_amount=?, biz_date=?, shift_id=?, points_earned=? WHERE id=?",
			cash, change, today, shiftID.Int64, pointsEarned, o.ID); err != nil {
			return Order{}, err
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_audit(action,order_no,product_id,detail,operator,created_at)"+
				" VALUES('cross_day_pay',?,'',?,?,datetime('now','localtime'))",
			orderNo, "原营业日 "+o.BizDate+" → "+today, ""); err != nil {
			return Order{}, err
		}
	} else if _, err := tx.Exec(
		"UPDATE t_pos_order SET status='paid', cash_amount=?, change_amount=?, points_earned=? WHERE id=?",
		cash, change, pointsEarned, o.ID); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	out, err := s.Get(orderNo)
	if err != nil {
		return Order{}, err
	}
	if crossDay {
		out.CrossDayFrom = o.BizDate
	}
	if s.onCreated != nil {
		s.onCreated(out)
	}
	return out, nil
}

// ErrConflict 状态冲突（如重复退款）。
var ErrConflict = errors.New("订单状态冲突")

// RefundItem 指定要退的明细行与数量。
type RefundItem struct {
	ProductID string
	Qty       int
}

// refundRoute 算出本次退款各方式的金额（原路退回：积分→积分，余额→余额，其余记现金）。
func (s *Service) refundRoute(o Order, amount int64) (memberID, toMember, toPoints int64) {
	var refundedMember, refundedPoints int64
	_ = s.db.SQL().QueryRow(
		"SELECT COALESCE(SUM(CASE WHEN method='member' THEN amount ELSE 0 END),0),"+
			" COALESCE(SUM(CASE WHEN method='points' THEN amount ELSE 0 END),0)"+
			" FROM t_pos_refund WHERE order_id=?", o.ID).
		Scan(&refundedMember, &refundedPoints)

	var memberPaid, pointsPaid int64
	for _, p := range o.Payments {
		if p.MemberID <= 0 {
			continue
		}
		memberID = p.MemberID
		switch p.Method {
		case "member":
			memberPaid += p.Amount
		case "points":
			pointsPaid += p.Amount
		}
	}

	// 先退积分（100 积分=1 元，金额与积分同值换算），再退余额。
	if left := pointsPaid - refundedPoints; left > 0 && memberID > 0 {
		toPoints = amount
		if toPoints > left {
			toPoints = left
		}
		amount -= toPoints
	}
	if left := memberPaid - refundedMember; left > 0 && memberID > 0 && amount > 0 {
		toMember = amount
		if toMember > left {
			toMember = left
		}
	}
	return memberID, toMember, toPoints
}

// Refund 退款：items 为空表示整单退剩余金额；否则按明细行退数量（部分退款）。
//
// 权限（谁是店长/是否允许收银员退款）由 HTTP 层判断，领域层只管状态与账。
func (s *Service) Refund(orderNo string, items []RefundItem, reason, operator string) (Order, error) {
	o, err := s.Get(orderNo)
	if err != nil {
		return Order{}, err
	}
	if o.Status == "refunded" {
		return Order{}, fmt.Errorf("%w: 该订单已全额退款", ErrConflict)
	}
	if o.Status != "paid" {
		return Order{}, fmt.Errorf("%w: 只有已支付订单可以退款", ErrConflict)
	}

	remaining := o.Payable - o.Refunded
	if remaining <= 0 {
		return Order{}, fmt.Errorf("%w: 没有可退金额", ErrConflict)
	}

	type line struct {
		productID string
		qty       int
		amount    int64
	}
	var (
		lines  []line
		amount int64
	)
	if len(items) == 0 {
		amount = remaining // 整单退剩余
	} else {
		byProduct := map[string]Item{}
		for _, it := range o.Items {
			byProduct[it.ProductID] = it
		}
		for _, req := range items {
			it, ok := byProduct[req.ProductID]
			if !ok {
				return Order{}, fmt.Errorf("%w: 订单里没有商品 %s", ErrBadRequest, req.ProductID)
			}
			left := it.Qty - it.RefundedQty
			if req.Qty <= 0 || req.Qty > left {
				return Order{}, fmt.Errorf("%w: %s 最多还能退 %d 件", ErrBadRequest, it.Name, left)
			}
			lines = append(lines, line{productID: req.ProductID, amount: it.Price * int64(req.Qty), qty: req.Qty})
			amount += it.Price * int64(req.Qty)
		}
		if amount <= 0 {
			return Order{}, fmt.Errorf("%w: 退款金额必须大于 0", ErrBadRequest)
		}
		if amount > remaining {
			return Order{}, fmt.Errorf("%w: 退款金额超过可退余额", ErrBadRequest)
		}
	}

	now := s.now().Format("2006-01-02 15:04:05")
	newRefunded := o.Refunded + amount
	status := "paid"
	if newRefunded >= o.Payable {
		status = "refunded"
	}

	// 必须在开事务前算：SQLite 只有一条连接，事务内再查会自锁。
	memberID, toMember, toPoints := s.refundRoute(o, amount)

	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback() }()

	method := "cash"
	if toPoints > 0 {
		method = "points"
	} else if toMember > 0 {
		method = "member"
	}
	if _, err := tx.Exec(
		"INSERT INTO t_pos_refund(order_id,order_no,amount,reason,operator,created_at,method,member_id,shift_id,order_biz_date)"+
			" VALUES(?,?,?,?,?,?,?,?,?,?)",
		o.ID, orderNo, amount, reason, operator, now, method, memberID, o.ShiftID, o.BizDate); err != nil {
		return Order{}, err
	}
	// 库存回补：按本次退款数量（套餐展开成组件）
	if s.stock != nil {
		only := map[string]int{}
		if len(items) > 0 {
			for _, it := range items {
				only[it.ProductID] += it.Qty
			}
		}
		if err := s.stock.RestoreTx(tx, stockMap(o.Items, only)); err != nil {
			return Order{}, err
		}
	}

	// 积分/余额支付的金额原路退回（与退款记录同一事务）。
	if toPoints > 0 {
		if s.members == nil {
			return Order{}, fmt.Errorf("%w: 未启用会员功能，无法退回积分", ErrBadRequest)
		}
		if err := s.members.RefundPointsTx(tx, memberID, toPoints, o.ShiftID, "退款 "+orderNo); err != nil {
			return Order{}, memberErr(err)
		}
	}
	if toMember > 0 {
		if s.members == nil {
			return Order{}, fmt.Errorf("%w: 未启用会员功能，无法退回余额", ErrBadRequest)
		}
		if err := s.members.RefundTx(tx, memberID, toMember, o.ShiftID, "退款 "+orderNo); err != nil {
			return Order{}, err
		}
	}
	if _, err := tx.Exec(
		"UPDATE t_pos_order SET status=?, refund_amount=refund_amount+?, refunded_at=? WHERE id=?",
		status, amount, now, o.ID); err != nil {
		return Order{}, err
	}
	for _, ln := range lines {
		if _, err := tx.Exec(
			"UPDATE t_pos_order_item SET refunded_qty=refunded_qty+? WHERE order_id=? AND product_id=?",
			ln.qty, o.ID, ln.productID); err != nil {
			return Order{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return s.Get(orderNo)
}

// Void 整单作废：点错/重复下单时用，销售不计入、钱原路退回。
//
// 与 Refund 的区别：
//   - Refund 是售后（商品可能已交付），销售仍计入净额；
//   - Void 是"这单不算"，作废后从销售额里剔除。
//
// 已退过款（部分或全部）的订单不允许作废——两种语义混用会算不清账。
func (s *Service) Void(orderNo, reason, operator string) (Order, error) {
	o, err := s.Get(orderNo)
	if err != nil {
		return Order{}, err
	}
	if o.Status == "void" {
		return Order{}, fmt.Errorf("%w: 该订单已作废", ErrConflict)
	}
	if o.Status != "paid" {
		return Order{}, fmt.Errorf("%w: 只有已支付订单可以作废", ErrConflict)
	}
	if o.Refunded > 0 {
		return Order{}, fmt.Errorf("%w: 已退款订单请用退款，不要作废", ErrConflict)
	}

	now := s.now().Format("2006-01-02 15:04:05")
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// 钱要回到钱箱账上：记一条退款流水（金额=应收），班次对账才能对上。
	if s.stock != nil {
		if err := s.stock.RestoreTx(tx, stockMap(o.Items, nil)); err != nil {
			return Order{}, err
		}
	}
	if _, err := tx.Exec(
		"INSERT INTO t_pos_refund(order_id,order_no,amount,reason,operator,created_at,kind,shift_id,order_biz_date)"+
			" VALUES(?,?,?,?,?,?,'void',?,?)",
		o.ID, orderNo, o.Payable, "作废："+reason, operator, now, o.ShiftID, o.BizDate); err != nil {
		return Order{}, err
	}
	if _, err := tx.Exec(
		"UPDATE t_pos_order SET status='void', refund_amount=?, refunded_at=?,"+
			" voided_at=?, void_reason=?, void_operator=? WHERE id=?",
		o.Payable, now, now, reason, operator, o.ID); err != nil {
		return Order{}, err
	}
	if _, err := tx.Exec(
		"INSERT INTO t_pos_audit(action,order_no,product_id,detail,operator,created_at)"+
			" VALUES('void',?,?,?,?,?)",
		orderNo, "", reason, operator, now); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	out, err := s.Get(orderNo)
	if err != nil {
		return Order{}, err
	}
	if s.onVoid != nil {
		s.onVoid(out)
	}
	return out, nil
}

// DefaultHeldExpire 是外带挂单的默认超时（分钟）。
// 桌台挂单（table_name 非空）**永不**自动取消：客人还在吃，单不能被系统撤掉。
const DefaultHeldExpire = 30 * time.Minute

// AutoCancelHeld 把超时未结算的挂单置为已取消（status='cancelled'）。
//
// 挂单从未收款：不写退款流水（钱从未进钱箱，写入会污染班次对账），
// 只改状态并记一条 audit(action='cancel')，返回取消的订单数。
// 行级条件 UPDATE 保证重复清扫或与并发结算竞态时不会误取消已结算的单。
func (s *Service) AutoCancelHeld(now time.Time, expire time.Duration) (int, error) {
	if expire <= 0 {
		return 0, nil
	}
	cutoff := now.Add(-expire).Format("2006-01-02 15:04:05")
	rows, err := s.db.SQL().Query(
		"SELECT id, order_no FROM t_pos_order WHERE status='held' AND held_at<>'' AND held_at < ?"+
			" AND COALESCE(table_name,'')=''", cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var targets []struct{ id int64; no string }
	for rows.Next() {
		var t struct{ id int64; no string }
		if err := rows.Scan(&t.id, &t.no); err != nil {
			return 0, err
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	nowStr := now.Format("2006-01-02 15:04:05")
	for _, t := range targets {
		tx, err := s.db.SQL().Begin()
		if err != nil {
			return 0, err
		}
		r, err := tx.Exec(
			"UPDATE t_pos_order SET status='cancelled', voided_at=?, void_reason=?, void_operator=?"+
				" WHERE id=? AND status='held'",
			nowStr, "挂单超时自动取消", "system", t.id)
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		if n, _ := r.RowsAffected(); n == 0 {
			_ = tx.Rollback()
			continue
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_audit(action,order_no,product_id,detail,operator,created_at)"+
				" VALUES('cancel',?,?,?,?,?)",
			t.no, "", "挂单超过 30 分钟未结算，自动取消", "system", nowStr); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
	}
	return len(targets), nil
}

// orderColumns 是订单读取的**唯一**列定义。
//
// 为什么必须是唯一：Get 和 List 曾经各写一份，改一处忘另一处，
// 结果 Get 少了 member_discount/shift_id，退款按原价算、跨班次统计为 0。
// 现在只允许在这里改。
const orderColumns = "order_no,biz_date,status,total_amount,discount_amount,payable_amount," +
	"cash_amount,change_amount,print_status,created_at,id,refund_amount," +
	"COALESCE(refunded_at,''),COALESCE(voided_at,''),COALESCE(void_reason,'')," +
	"COALESCE(void_operator,''),COALESCE(held_at,''),COALESCE(table_name,'')," +
	"round_off,member_discount,member_id,points_earned,COALESCE(shift_id,0),COALESCE(operator,'')"

// scanOrder 把一行订单扫描成 Order（列顺序与 orderColumns 一一对应）。
func scanOrder(sc interface{ Scan(dest ...any) error }) (Order, error) {
	var o Order
	err := sc.Scan(&o.OrderNo, &o.BizDate, &o.Status, &o.Total, &o.Discount, &o.Payable,
		&o.Cash, &o.Change, &o.PrintStatus, &o.CreatedAt, &o.ID, &o.Refunded, &o.RefundedAt,
		&o.VoidedAt, &o.VoidReason, &o.VoidOperator, &o.HeldAt, &o.TableName, &o.RoundOff,
		&o.MemberDiscount, &o.MemberID, &o.PointsEarned, &o.ShiftID, &o.Operator)
	return o, err
}

// List 返回最近订单。
func (s *Service) List(limit int) ([]Order, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.SQL().Query(
		"SELECT "+orderColumns+" FROM t_pos_order ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}

	// 先把结果集读完并关闭，再去查明细。
	// SQLite 连接池只有 1 个连接（store.Open 里 SetMaxOpenConns(1)），
	// 在 rows 未关闭时发第二条查询会永远等连接 —— 自死锁，并拖死整个进程。
	var out []Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	for i := range out {
		out[i].Items, _ = s.itemsOf(out[i].ID)
	}
	return out, nil
}

// Get 按单号查询。
func (s *Service) Get(orderNo string) (Order, error) {
	o, err := scanOrder(s.db.SQL().QueryRow(
		"SELECT "+orderColumns+" FROM t_pos_order WHERE order_no=?", orderNo))
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	o.Items, err = s.itemsOf(o.ID)
	if err != nil {
		return o, err
	}
	o.Payments, err = s.paymentsOf(o.ID)
	return o, err
}

// paymentsOf 读取订单的支付明细（组合支付会有多行）。
func (s *Service) paymentsOf(orderID int64) ([]Payment, error) {
	rows, err := s.db.SQL().Query(
		"SELECT method, amount, member_id FROM t_pos_payment WHERE order_id=? ORDER BY id", orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.Method, &p.Amount, &p.MemberID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Reprint 触发重打；订单不存在返回 ErrNotFound。
func (s *Service) Reprint(orderNo string) (Order, error) {
	o, err := s.Get(orderNo)
	if err != nil {
		return Order{}, err
	}
	if s.onReprint != nil {
		s.onReprint(o)
	}
	return o, nil
}

func (s *Service) itemsOf(orderID int64) ([]Item, error) {
	rows, err := s.db.SQL().Query(
		"SELECT product_id,name,price,original_price,price_overridden,qty,amount,refunded_qty,COALESCE(components,'')"+
			" FROM t_pos_order_item WHERE order_id=? ORDER BY id", orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		var overridden int
		var compsRaw string
		if err := rows.Scan(&it.ProductID, &it.Name, &it.Price, &it.OriginalPrice, &overridden,
			&it.Qty, &it.Amount, &it.RefundedQty, &compsRaw); err != nil {
			return nil, err
		}
		it.Overridden = overridden == 1
		if compsRaw != "" {
			_ = json.Unmarshal([]byte(compsRaw), &it.Components)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Service) findByIdem(key string) (Order, error) {
	var orderNo string
	if err := s.db.SQL().QueryRow("SELECT order_no FROM t_pos_order WHERE idem_key=?", key).Scan(&orderNo); err != nil {
		return Order{}, err
	}
	return s.Get(orderNo)
}

// BizDate 对外暴露营业日口径（凌晨 dayCutHour 点前算前一天），
// 供健康检查等外部读取「今日已收」时与订单落库口径保持一致。
func BizDate(t time.Time) string { return bizDate(t) }

func bizDate(t time.Time) string {
	d := t
	if t.Hour() < dayCutHour {
		d = t.AddDate(0, 0, -1)
	}
	return d.Format("20060102")
}

// memberErr 把会员域错误翻译成订单域错误（避免 500 漏到前端）。
func memberErr(err error) error {
	switch {
	case errors.Is(err, member.ErrInactive):
		return fmt.Errorf("%w: 会员已停用", ErrConflict)
	case errors.Is(err, member.ErrInsufficient):
		return fmt.Errorf("%w: 会员余额或积分不足", ErrConflict)
	case errors.Is(err, member.ErrNotFound):
		return fmt.Errorf("%w: 会员不存在", ErrBadRequest)
	default:
		return err
	}
}

// resolvePayments 校验支付明细并算出实收/找零（下单与挂单支付共用）。
func resolvePayments(in []PaymentInput, payable int64, cashIn float64) ([]PaymentInput, int64, int64, error) {
	payments := in
	if len(payments) == 0 {
		payments = []PaymentInput{{Method: "cash", Amount: float64(payable) / 100}}
	}
	cashApplied, paidSum := int64(0), int64(0)
	for _, p := range payments {
		switch p.Method {
		case "cash", "wechat", "alipay":
		case "member", "points":
			if p.MemberID <= 0 {
				return nil, 0, 0, fmt.Errorf("%w: 会员支付缺少会员", ErrBadRequest)
			}
		default:
			return nil, 0, 0, fmt.Errorf("%w: 不支持的支付方式：%s", ErrBadRequest, p.Method)
		}
		amt := yuanToCents(p.Amount)
		if amt <= 0 {
			return nil, 0, 0, fmt.Errorf("%w: 支付金额必须大于 0", ErrBadRequest)
		}
		if p.Method == "cash" {
			cashApplied += amt
		}
		paidSum += amt
	}
	if paidSum != payable {
		return nil, 0, 0, fmt.Errorf("%w: 支付合计 %d 分与应收 %d 分不一致", ErrBadRequest, paidSum, payable)
	}
	cash := yuanToCents(cashIn)
	if cash == 0 {
		cash = cashApplied
	}
	if cash < cashApplied {
		return nil, 0, 0, fmt.Errorf("%w: 实收金额不足", ErrBadRequest)
	}
	return payments, cash, cash - cashApplied, nil
}

// roundOffCents 按方式计算抹零金额（向下取整，永不进位）。
func roundOffCents(payable int64, mode string) int64 {
	switch mode {
	case "jiao": // 抹分：13.87 -> 13.80
		return payable % 10
	case "yuan": // 抹角：13.87 -> 13.00
		return payable % 100
	default:
		return 0
	}
}

func heldAt(hold bool, createdAt string) any {
	if hold {
		return createdAt
	}
	return nil
}

func toPayments(in []PaymentInput) []Payment {
	out := make([]Payment, 0, len(in))
	for _, p := range in {
		out = append(out, Payment{Method: p.Method, Amount: yuanToCents(p.Amount), MemberID: p.MemberID})
	}
	return out
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// componentsJSON 序列化套餐组件快照（空则存空串，便于 SQL 判空）。
func componentsJSON(c []ItemComponent) string {
	if len(c) == 0 {
		return ""
	}
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}

func yuanToCents(y float64) int64 { return int64(y*100 + copysign(0.5, y)) }

func copysign(v, sign float64) float64 {
	if sign < 0 {
		return -v
	}
	return v
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}