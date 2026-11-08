// Package table 的 HTTP 层。
package table

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"pos/internal/order"
)

// Handler 需要订单服务：开台=建挂单，结账=收款后清台。
type Handler struct {
	svc    *Service
	orders *order.Service
}

// NewHandler 创建处理器。
func NewHandler(svc *Service, orders *order.Service) *Handler {
	return &Handler{svc: svc, orders: orders}
}

// Register 挂载 /pos/v1/tables/*。
func (h *Handler) Register(mux *http.ServeMux, guard func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("GET /pos/v1/tables", func(w http.ResponseWriter, r *http.Request) {
		list, err := h.svc.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取桌台失败"})
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /pos/v1/tables", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		t, err := h.svc.Create(body.Name)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, t)
	})

	mux.HandleFunc("POST /pos/v1/tables/{id}/open", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		var body struct {
			Guests int `json:"guests"`
			Items  []struct {
				ProductID string   `json:"productId"`
				Qty       int      `json:"qty"`
				PriceMinor *int64 `json:"priceMinor"`
			} `json:"items"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		in := order.CreateInput{Hold: true, Operator: "开台"}
		for _, it := range body.Items {
			var pid int64
			for _, c := range it.ProductID {
				pid = pid*10 + int64(c-'0')
			}
			in.Items = append(in.Items, order.ItemInput{ProductID: pid, Qty: it.Qty, Price: it.PriceMinor})
		}
		o, err := h.orders.Create(in)
		if err != nil {
			writeErr(w, err)
			return
		}
		t, err := h.svc.Bind(id, o.OrderNo, body.Guests)
		if err != nil {
			writeErr(w, err)
			return
		}
		// 订单上记桌台名，小票与后厨单才能打印桌号
		_ = h.orders.SetTable(o.OrderNo, t.Name)
		writeJSON(w, http.StatusOK, map[string]any{"table": t, "order": order.ToJSON(o)})
	})

	mux.HandleFunc("POST /pos/v1/tables/{id}/merge", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		var body struct {
			IntoTableID int64 `json:"intoTableId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		from, err := h.svc.Get(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		to, err := h.svc.Get(body.IntoTableID)
		if err != nil {
			writeErr(w, err)
			return
		}
		if from.Status != "occupied" || to.Status != "occupied" {
			writeJSON(w, http.StatusConflict, map[string]string{"message": "两张桌台都要有进行中的订单"})
			return
		}
		merged, err := h.orders.MergeInto(from.OrderNo, to.OrderNo)
		if err != nil {
			writeErr(w, err)
			return
		}
		freed, err := h.svc.Release(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"from": freed, "into": to, "order": order.ToJSON(merged)})
	})

	mux.HandleFunc("POST /pos/v1/tables/{id}/transfer", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		var body struct {
			ToTableID int64 `json:"toTableId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		from, to, err := h.svc.Transfer(id, body.ToTableID)
		if err != nil {
			writeErr(w, err)
			return
		}
		// 订单上的桌台名也要跟着改，小票/后厨单才对
		_ = h.orders.TransferTable(to.OrderNo, to.Name)
		writeJSON(w, http.StatusOK, map[string]any{"from": from, "to": to})
	})

	mux.HandleFunc("POST /pos/v1/tables/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		t, err := h.svc.Get(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		if t.Status != "occupied" || t.OrderNo == "" {
			writeJSON(w, http.StatusConflict, map[string]string{"message": "该桌台没有进行中的订单"})
			return
		}
		var body struct {
			Items []struct {
				ProductID string   `json:"productId"`
				Qty       int      `json:"qty"`
				PriceMinor *int64 `json:"priceMinor"`
			} `json:"items"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		in := make([]order.ItemInput, 0, len(body.Items))
		for _, it := range body.Items {
			var pid int64
			for _, c := range it.ProductID {
				pid = pid*10 + int64(c-'0')
			}
			in = append(in, order.ItemInput{ProductID: pid, Qty: it.Qty, Price: it.PriceMinor})
		}
		o, err := h.orders.AddItems(t.OrderNo, in)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"table": t, "order": o})
	})

	mux.HandleFunc("POST /pos/v1/tables/{id}/checkout", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		t, err := h.svc.Get(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		if t.Status != "occupied" || t.OrderNo == "" {
			writeJSON(w, http.StatusConflict, map[string]string{"message": "该桌台没有待结账的订单"})
			return
		}
		var body struct {
			CashMinor int64 `json:"cashMinor"`
			Payments []struct {
				Method   string  `json:"method"`
				AmountMinor int64 `json:"amountMinor"`
				MemberID int64   `json:"memberId"`
			} `json:"payments"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		pays := make([]order.PaymentInput, 0, len(body.Payments))
		for _, p := range body.Payments {
			pays = append(pays, order.PaymentInput{Method: p.Method, Amount: p.AmountMinor, MemberID: p.MemberID})
		}
		o, err := h.orders.Pay(t.OrderNo, pays, body.CashMinor)
		if err != nil {
			writeErr(w, err)
			return
		}
		freed, err := h.svc.Release(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"table": freed, "order": o})
	})

	mux.HandleFunc("POST /pos/v1/tables/{id}/release", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		t, err := h.svc.Release(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, t)
	})

	mux.HandleFunc("DELETE /pos/v1/tables/{id}", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
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
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "桌台不存在"})
	case errors.Is(err, ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"message": "桌台状态不允许该操作"})
	case errors.Is(err, ErrBadRequest):
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "参数不合法"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
