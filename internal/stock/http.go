package stock

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Handler 暴露库存接口。
type Handler struct{ svc *Service }

// NewHandler 创建处理器。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 挂载 /pos/v1/stock/*（写操作由 guard 限店长）。
func (h *Handler) Register(mux *http.ServeMux, guard func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("GET /pos/v1/stock", func(w http.ResponseWriter, r *http.Request) {
		list, err := h.svc.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取库存失败"})
			return
		}
		if list == nil {
			list = []Row{}
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("GET /pos/v1/stock/alerts", func(w http.ResponseWriter, r *http.Request) {
		list, err := h.svc.Alerts()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取库存失败"})
			return
		}
		if list == nil {
			list = []Row{}
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("PUT /pos/v1/stock/{productId}", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		pid, err := strconv.ParseInt(r.PathValue("productId"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "商品编号不合法"})
			return
		}
		var body struct {
			Qty     int `json:"qty"`
			WarnQty int `json:"warnQty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		row, err := h.svc.Set(pid, body.Qty, body.WarnQty)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, row)
	})

	mux.HandleFunc("POST /pos/v1/stock/{productId}/adjust", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		pid, err := strconv.ParseInt(r.PathValue("productId"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "商品编号不合法"})
			return
		}
		var body struct {
			Delta  int    `json:"delta"`
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		row, err := h.svc.Adjust(pid, body.Delta, body.Reason)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, row)
	})
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "该商品没有库存记录"})
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
