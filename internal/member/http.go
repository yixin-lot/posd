// Package member 的 HTTP 层。
package member

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Handler 暴露会员接口。
type Handler struct {
	svc *Service
	// bonusRules 由 posd 注入（读设置），为空表示不赠送。
	bonusRules func() []BonusRule
}

// SetBonusRuleSource 注入储值赠送规则来源。
func (h *Handler) SetBonusRuleSource(fn func() []BonusRule) { h.bonusRules = fn }

// NewHandler 创建处理器。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type memberJSON struct {
	ID      int64   `json:"id"`
	Phone   string  `json:"phone"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
	Points  int     `json:"points"`
	TotalSpent float64 `json:"totalSpent"`
	Level      string `json:"level"`
	Discount   int64  `json:"discountPct"`
	PointRate  int64  `json:"pointRate"`
	Active     bool   `json:"active"`
}

// Register 挂载 /pos/v1/members/*（全部需要登录，由上层中间件保证）。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /pos/v1/members", func(w http.ResponseWriter, r *http.Request) {
		phone := r.URL.Query().Get("phone")
		if phone == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请提供手机号"})
			return
		}
		m, found, err := h.svc.FindByPhone(phone)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "查询失败"})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "该手机号还没有开卡"})
			return
		}
		writeJSON(w, http.StatusOK, h.toJSON(m))
	})

	mux.HandleFunc("GET /pos/v1/members/list", func(w http.ResponseWriter, r *http.Request) {
		list, err := h.svc.Search(r.URL.Query().Get("keyword"), 50)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取会员失败"})
			return
		}
		out := make([]memberJSON, 0, len(list))
		for _, m := range list {
			out = append(out, h.toJSON(m))
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /pos/v1/members", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Phone string `json:"phone"`
			Name  string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		m, err := h.svc.Create(body.Phone, body.Name)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, h.toJSON(m))
	})

	mux.HandleFunc("POST /pos/v1/members/{id}/topup", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		var body struct {
			Amount float64 `json:"amount"`
			Note   string  `json:"note"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		cents := int64(body.Amount*100 + 0.5)
		var bonus int64
		if h.bonusRules != nil {
			bonus = BonusFor(h.bonusRules(), cents)
		}
		m, err := h.svc.TopupWithBonus(id, cents, bonus, body.Note)
		if err != nil {
			writeErr(w, err)
			return
		}
		// 返回带赠金金额的结构，前端才能提示"充 500 送 50"
		writeJSON(w, http.StatusOK, map[string]any{
			"member": h.toJSON(m),
			"bonus":  float64(bonus) / 100,
		})
	})

	mux.HandleFunc("POST /pos/v1/members/{id}/active", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		var body struct {
			Active bool `json:"active"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		m, err := h.svc.SetActive(id, body.Active)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, h.toJSON(m))
	})

	mux.HandleFunc("DELETE /pos/v1/members/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		if err := h.svc.Delete(id); err != nil {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /pos/v1/members/expiring", func(w http.ResponseWriter, r *http.Request) {
		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		list, err := h.svc.ExpiringSoon(days)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取失败"})
			return
		}
		out := make([]memberJSON, 0, len(list))
		for _, m := range list {
			out = append(out, h.toJSON(m))
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("GET /pos/v1/members/{id}/quote", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		m, err := h.svc.Get(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		if !m.Active {
			writeJSON(w, http.StatusConflict, map[string]string{"message": "会员已停用"})
			return
		}
		amount, _ := strconv.ParseFloat(r.URL.Query().Get("amount"), 64)
		cents := int64(amount*100 + 0.5)
		t := h.svc.TierOf(m)
		off := int64(0)
		if t.Discount > 0 && t.Discount < 100 {
			off = cents * t.Discount / 100
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"level": t.Name, "discountPct": t.Discount, "pointRate": t.PointRate,
			"payable": float64(cents-off) / 100, "discount": float64(off) / 100,
			"points": m.Points, "balance": float64(m.Balance) / 100,
		})
	})

	mux.HandleFunc("GET /pos/v1/members/{id}/txns", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		list, err := h.svc.Txns(id, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取流水失败"})
			return
		}
		out := make([]map[string]any, 0, len(list))
		for _, t := range list {
			out = append(out, map[string]any{
				"kind": t.Kind, "amount": float64(t.Amount) / 100,
				"balanceAfter": float64(t.BalanceAfter) / 100,
				"note": t.Note, "createdAt": t.CreatedAt,
			})
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func (h *Handler) toJSON(m Member) memberJSON {
	t := h.svc.TierOf(m)
	return memberJSON{ID: m.ID, Phone: m.Phone, Name: m.Name,
		Balance: float64(m.Balance) / 100, Points: m.Points,
		TotalSpent: float64(m.TotalSpent) / 100, Level: t.Name,
		Discount: t.Discount, PointRate: t.PointRate, Active: m.Active}
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "会员不存在"})
	case errors.Is(err, ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"message": "该手机号已开卡"})
	case errors.Is(err, ErrInactive):
		writeJSON(w, http.StatusConflict, map[string]string{"message": "会员已停用，不能消费或充值"})
	case errors.Is(err, ErrInsufficient):
		writeJSON(w, http.StatusConflict, map[string]string{"message": "余额不足"})
	case errors.Is(err, ErrBadRequest):
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
