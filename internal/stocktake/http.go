package stocktake

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Handler 暴露盘点接口（写操作限店长）。
type Handler struct{ svc *Service }

// NewHandler 创建处理器。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 挂载 /pos/v1/stocktakes/*。
func (h *Handler) Register(mux *http.ServeMux, guard func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("GET /pos/v1/stocktakes", func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
			limit = n
		}
		list, err := h.svc.List(limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取盘点单失败"})
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /pos/v1/stocktakes", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		var body struct {
			Note     string `json:"note"`
			Operator string `json:"operator"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body)
		sh, err := h.svc.Create(body.Note, body.Operator)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, sh)
	})

	mux.HandleFunc("GET /pos/v1/stocktakes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "盘点单编号不合法"})
			return
		}
		sh, err := h.svc.Get(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sh)
	})

	mux.HandleFunc("PUT /pos/v1/stocktakes/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "盘点单编号不合法"})
			return
		}
		var body struct {
			Items []struct {
				ProductID int64 `json:"productId"`
				ActualQty int   `json:"actualQty"`
			} `json:"items"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		actual := make(map[int64]int, len(body.Items))
		for _, it := range body.Items {
			actual[it.ProductID] = it.ActualQty
		}
		sh, err := h.svc.SetActual(id, actual)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sh)
	})

	mux.HandleFunc("POST /pos/v1/stocktakes/{id}/close", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "盘点单编号不合法"})
			return
		}
		var body struct {
			Operator string `json:"operator"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body)
		sh, err := h.svc.Close(id, body.Operator)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sh)
	})
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "盘点单不存在"})
	case errors.Is(err, ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"message": err.Error()})
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
