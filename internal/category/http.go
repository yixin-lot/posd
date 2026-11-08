// Package category 的 HTTP 层。
package category

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Handler 暴露类目接口；guard 负责"是否店长"。
type Handler struct{ svc *Service }

// NewHandler 创建处理器。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 挂载 /pos/v1/categories/*。
func (h *Handler) Register(mux *http.ServeMux, guard func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("GET /pos/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		list, err := h.svc.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取类目失败"})
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /pos/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		var body struct {
			Name string `json:"name"`
			Sort int    `json:"sort"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		c, err := h.svc.Create(body.Name, body.Sort)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, c)
	})

	mux.HandleFunc("PUT /pos/v1/categories/{id}", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		var body struct {
			Name string `json:"name"`
			Sort int    `json:"sort"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		c, err := h.svc.Rename(id, body.Name, body.Sort)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	})

	mux.HandleFunc("DELETE /pos/v1/categories/{id}", func(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusNotFound, map[string]string{"message": err.Error()})
	case errors.Is(err, ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"message": "类目重名，或仍被商品使用"})
	case errors.Is(err, ErrBadRequest):
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "类目名不能为空且不超过 20 字"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
