package order

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

type itemJSON struct {
	ProductID   string  `json:"productId"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Qty         int     `json:"qty"`
	Amount      float64 `json:"amount"`
	RefundedQty int     `json:"refundedQty"`
	Category    string  `json:"category,omitempty"`
	// Components 是套餐组件快照（单品为空）
	Components []componentJSON `json:"components,omitempty"`
}

type componentJSON struct {
	ComponentID string `json:"componentId"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Qty         int    `json:"qty"`
}

type payJSON struct {
	Method string  `json:"method"`
	Amount float64 `json:"amount"`
}

type orderJSON struct {
	OrderNo     string     `json:"orderNo"`
	BizDate     string     `json:"bizDate"`
	Status      string     `json:"status"`
	Total       float64    `json:"total"`
	Discount    float64    `json:"discount"`
	Payable     float64    `json:"payable"`
	Cash        float64    `json:"cash"`
	Change      float64    `json:"change"`
	PrintStatus int        `json:"printStatus"`
	CreatedAt   string     `json:"createdAt"`
	Refunded    float64    `json:"refundAmount"`
	RefundedAt  string     `json:"refundedAt,omitempty"`
	VoidedAt    string     `json:"voidedAt,omitempty"`
	VoidReason  string     `json:"voidReason,omitempty"`
	HeldAt      string     `json:"heldAt,omitempty"`
	MemberDiscount float64 `json:"memberDiscount"`
	MemberID       int64   `json:"memberId"`
	PointsEarned   int64   `json:"pointsEarned"`
	RoundOff    float64    `json:"roundOff"`
	CrossDayFrom string    `json:"crossDayFrom,omitempty"`
	TableName    string    `json:"tableName,omitempty"`
	Payments    []payJSON  `json:"payments"`
	Items       []itemJSON `json:"items"`
}

// ToJSON 把订单转成对外 DTO（camelCase、元）。
// 供其它域（如桌台）复用，避免直接暴露内部结构导致字段风格不一致。
func ToJSON(o Order) any {
	return toJSON(o)
}

// Options 是 HTTP 层的权限钩子（由 posd 用 staff/setting 实现）。
// 为 nil 时一律拒绝——安全默认。
type Options struct {
	CanDiscount   func(*http.Request) bool
	CanRefund     func(*http.Request) bool
	CanChangePrice func(*http.Request) bool
	CanVoid        func(*http.Request) bool
	// RoundMode 返回门店抹零方式（none/jiao/yuan）。
	RoundMode func(*http.Request) string
	// MemberBenefit 返回会员等级的折扣百分比与积分倍率。
	MemberBenefit func(memberID int64) (discountPct int64, pointRate int64)
	// PointsEarn / PointsRedeem 返回积分规则（每元得分 / 多少分抵 1 元）。
	PointsEarn   func(*http.Request) int64
	PointsRedeem func(*http.Request) int64
	PointsExpireMonths func(*http.Request) int64
	// StockBlock 返回库存不足时是否拦截下单。
	StockBlock func(*http.Request) bool
	Operator      func(*http.Request) string
}

func (o Options) canDiscount(r *http.Request) bool {
	return o.CanDiscount != nil && o.CanDiscount(r)
}

func (o Options) canRefund(r *http.Request) bool {
	return o.CanRefund != nil && o.CanRefund(r)
}

func (o Options) canChangePrice(r *http.Request) bool {
	return o.CanChangePrice != nil && o.CanChangePrice(r)
}

func (o Options) pointsEarn(r *http.Request) int64 {
	if o.PointsEarn == nil {
		return 1
	}
	return o.PointsEarn(r)
}

func (o Options) pointsRedeem(r *http.Request) int64 {
	if o.PointsRedeem == nil {
		return 100
	}
	return o.PointsRedeem(r)
}

func (o Options) stockBlock(r *http.Request) bool {
	if o.StockBlock == nil {
		return false
	}
	return o.StockBlock(r)
}

func (o Options) pointsExpireMonths(r *http.Request) int64 {
	if o.PointsExpireMonths == nil {
		return 0
	}
	return o.PointsExpireMonths(r)
}

func (o Options) canVoid(r *http.Request) bool {
	return o.CanVoid != nil && o.CanVoid(r)
}

func (o Options) operator(r *http.Request) string {
	if o.Operator == nil {
		return ""
	}
	return o.Operator(r)
}

type createRequest struct {
	IdempotencyKey string  `json:"idempotencyKey"`
	Cash           float64 `json:"cash"`
	Discount       float64 `json:"discount"`
	Hold           bool    `json:"hold"`
	MemberID       int64   `json:"memberId"`
	Payments       []struct {
		Method   string  `json:"method"`
		Amount   float64 `json:"amount"`
		MemberID int64   `json:"memberId"`
	} `json:"payments"`
	Items          []struct {
		ProductID string   `json:"productId"`
		Qty       int      `json:"qty"`
		Price     *float64 `json:"price"` // 可选：单品改价
		// 套餐可选项：groupId → componentId
		Choices []struct {
			GroupID     int64 `json:"groupId"`
			ComponentID int64 `json:"componentId"`
		} `json:"choices"`
	} `json:"items"`
}

// Register 挂载订单接口。
func Register(mux *http.ServeMux, s *Service, opt Options) {
	mux.HandleFunc("POST /pos/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		// 整单优惠是「让利」，默认只有店长能做；开关见 setting.perm_discount_cashier。
		if req.Discount > 0 && !opt.canDiscount(r) {
			writeErr(w, http.StatusForbidden, "需要店长权限才能打折")
			return
		}
		// 改价同理：先按权限拦掉（403），避免落到领域层变成 400。
		for _, it := range req.Items {
			if it.Price != nil && !opt.canChangePrice(r) {
				writeErr(w, http.StatusForbidden, "需要店长权限才能改价")
				return
			}
		}
		pays := make([]PaymentInput, 0, len(req.Payments))
		for _, p := range req.Payments {
			pays = append(pays, PaymentInput{Method: p.Method, Amount: p.Amount, MemberID: p.MemberID})
		}
		roundMode := "none"
		if opt.RoundMode != nil {
			roundMode = opt.RoundMode(r)
		}
		pct, rate := int64(0), int64(1)
		if req.MemberID > 0 && opt.MemberBenefit != nil {
			pct, rate = opt.MemberBenefit(req.MemberID)
		}
		in := CreateInput{
			Hold:               req.Hold,
			MemberID:           req.MemberID,
			MemberDiscountPct:  pct,
			MemberPointRate:    rate,
			PointsEarnPerYuan:  opt.pointsEarn(r),
			PointsPerYuan:      opt.pointsRedeem(r),
			PointsExpireMonths: opt.pointsExpireMonths(r),
			StockBlockWhenShort: opt.stockBlock(r),
			RoundMode:          roundMode,
			Payments:           pays,
			IdempotencyKey:     req.IdempotencyKey,
			Cash:               req.Cash,
			Discount:           req.Discount,
			AllowPriceOverride: opt.canChangePrice(r),
			Operator:           opt.operator(r),
		}
		for _, it := range req.Items {
			pid, err := strconv.ParseInt(it.ProductID, 10, 64)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "商品编号不合法")
				return
			}
			choices := make([]Choice, 0, len(it.Choices))
			for _, ch := range it.Choices {
				choices = append(choices, Choice{GroupID: ch.GroupID, ComponentID: ch.ComponentID})
			}
			in.Items = append(in.Items, ItemInput{ProductID: pid, Qty: it.Qty, Price: it.Price, Choices: choices})
		}
		o, created, err := s.CreateWithStatus(in)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		// 幂等：新建才 201；重复提交返回已有订单，用 200（契约 §orders）。
		code := http.StatusCreated
		if !created {
			code = http.StatusOK
		}
		writeJSON(w, code, toJSON(o))
	})

	mux.HandleFunc("GET /pos/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		list, err := s.List(limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		today := bizDate(time.Now())
		out := make([]orderJSON, 0, len(list))
		for _, o := range list {
			j := toJSON(o)
			// 跨天未结的挂单：前端要标出来，否则会被遗忘
			if o.Status == "held" && o.BizDate != today && o.BizDate != "" {
				j.CrossDayFrom = o.BizDate
			}
			out = append(out, j)
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /pos/v1/orders/{orderNo}/refund", func(w http.ResponseWriter, r *http.Request) {
		if !opt.canRefund(r) {
			writeErr(w, http.StatusForbidden, "需要店长权限才能退款")
			return
		}
		var body struct {
			Reason string `json:"reason"`
			Items  []struct {
				ProductID string `json:"productId"`
				Qty       int    `json:"qty"`
			} `json:"items"`
		}
		// 空 body 视为整单退；但 body 存在却解析失败时必须报错，
		// 否则「JSON 写错」会被静默当成整单退款——那是资损。
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		items := make([]RefundItem, 0, len(body.Items))
		for _, it := range body.Items {
			items = append(items, RefundItem{ProductID: it.ProductID, Qty: it.Qty})
		}
		o, err := s.Refund(r.PathValue("orderNo"), items, body.Reason, opt.operator(r))
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toJSON(o))
	})

	mux.HandleFunc("POST /pos/v1/orders/{orderNo}/pay", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Cash     float64 `json:"cash"`
			Payments []struct {
				Method   string  `json:"method"`
				Amount   float64 `json:"amount"`
				MemberID int64   `json:"memberId"`
			} `json:"payments"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		pays := make([]PaymentInput, 0, len(body.Payments))
		for _, p := range body.Payments {
			pays = append(pays, PaymentInput{Method: p.Method, Amount: p.Amount, MemberID: p.MemberID})
		}
		// 挂单收款必须沿用与即时结算相同的会员规则
		rules := MemberRules{
			PointsPerYuan:     opt.pointsRedeem(r),
			PointsEarnPerYuan: opt.pointsEarn(r),
			ExpireMonths:      opt.pointsExpireMonths(r),
		}
		if len(pays) > 0 && pays[0].MemberID > 0 && opt.MemberBenefit != nil {
			_, rate := opt.MemberBenefit(pays[0].MemberID)
			rules.PointRate = rate
		}
		o, err := s.PayWith(r.PathValue("orderNo"), pays, body.Cash, rules)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toJSON(o))
	})

	mux.HandleFunc("POST /pos/v1/orders/{orderNo}/void", func(w http.ResponseWriter, r *http.Request) {
		if !opt.canVoid(r) {
			writeErr(w, http.StatusForbidden, "需要店长权限才能作废")
			return
		}
		var body struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		o, err := s.Void(r.PathValue("orderNo"), body.Reason, opt.operator(r))
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toJSON(o))
	})

	mux.HandleFunc("POST /pos/v1/orders/{orderNo}/reprint", func(w http.ResponseWriter, r *http.Request) {
		o, err := s.Reprint(r.PathValue("orderNo"))
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, toJSON(o))
	})

	mux.HandleFunc("GET /pos/v1/orders/{orderNo}", func(w http.ResponseWriter, r *http.Request) {
		o, err := s.Get(r.PathValue("orderNo"))
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toJSON(o))
	})
}

func toJSON(o Order) orderJSON {
	out := orderJSON{
		OrderNo: o.OrderNo, BizDate: o.BizDate, Status: o.Status,
		Total: yuan(o.Total), Discount: yuan(o.Discount), Payable: yuan(o.Payable),
		Cash: yuan(o.Cash), Change: yuan(o.Change),
		PrintStatus: o.PrintStatus, CreatedAt: o.CreatedAt,
		Refunded: yuan(o.Refunded), RefundedAt: o.RefundedAt,
		VoidedAt: o.VoidedAt, VoidReason: o.VoidReason, HeldAt: o.HeldAt, RoundOff: yuan(o.RoundOff),
		MemberDiscount: yuan(o.MemberDiscount), MemberID: o.MemberID, PointsEarned: o.PointsEarned,
		TableName: o.TableName,
	}
	for _, it := range o.Items {
		j := itemJSON{
			ProductID: it.ProductID, Name: it.Name, Price: yuan(it.Price), Qty: it.Qty,
			Amount: yuan(it.Amount), RefundedQty: it.RefundedQty, Category: it.Category,
		}
		for _, c := range it.Components {
			j.Components = append(j.Components, componentJSON{
				ComponentID: c.ComponentID, Name: c.Name, Category: c.Category, Qty: c.Qty,
			})
		}
		out.Items = append(out.Items, j)
	}
	out.Payments = toPayJSON(o.Payments)
	return out
}

func yuan(cents int64) float64 { return float64(cents) / 100 }

func writeServiceErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrBadRequest):
		writeErr(w, http.StatusBadRequest, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"code": http.StatusText(code), "message": message})
}

func toPayJSON(list []Payment) []payJSON {
	out := make([]payJSON, 0, len(list))
	for _, p := range list {
		out = append(out, payJSON{Method: p.Method, Amount: yuan(p.Amount)})
	}
	return out
}
