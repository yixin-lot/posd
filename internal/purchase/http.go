package purchase

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Handler 暴露进货接口（写操作限店长）。
type Handler struct{ svc *Service }

// NewHandler 创建处理器。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 挂载 /pos/v1/purchases/*。
func (h *Handler) Register(mux *http.ServeMux, guard func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("GET /pos/v1/purchases", func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
			limit = n
		}
		list, err := h.svc.List(limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取进货单失败"})
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /pos/v1/purchases", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		var body struct {
			Supplier string `json:"supplier"`
			Note     string `json:"note"`
			Operator string `json:"operator"`
			Items    []struct {
				ProductID int64 `json:"productId"`
				Qty       int   `json:"qty"`
				UnitCost  int64 `json:"unitCost"` // 分
			} `json:"items"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		items := make([]InputItem, 0, len(body.Items))
		for _, it := range body.Items {
			items = append(items, InputItem{ProductID: it.ProductID, Qty: it.Qty, UnitCost: it.UnitCost})
		}
		o, err := h.svc.Create(body.Supplier, body.Note, body.Operator, items)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, o)
	})

	mux.HandleFunc("GET /pos/v1/purchases/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "进货单编号不合法"})
			return
		}
		o, err := h.svc.Get(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	})
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "进货单不存在"})
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
