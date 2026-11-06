// Package kitchen 的 HTTP 层。
package kitchen

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Handler 暴露档口接口。
type Handler struct{ svc *Service }

// NewHandler 创建处理器。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 挂载 /pos/v1/kitchen/stations/*（写操作由 guard 限店长）。
func (h *Handler) Register(mux *http.ServeMux, guard func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("GET /pos/v1/kitchen/stations", func(w http.ResponseWriter, r *http.Request) {
		list, err := h.svc.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取档口失败"})
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /pos/v1/kitchen/stations", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		var body struct {
			Name       string `json:"name"`
			Printer    string `json:"printer"`
			Categories string `json:"categories"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		st, err := h.svc.Create(body.Name, body.Printer, body.Categories)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, st)
	})

	mux.HandleFunc("PUT /pos/v1/kitchen/stations/{id}", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "编号不合法"})
			return
		}
		var body struct {
			Name       string `json:"name"`
			Printer    string `json:"printer"`
			Categories string `json:"categories"`
			Active     bool   `json:"active"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		st, err := h.svc.Update(id, body.Name, body.Printer, body.Categories, body.Active)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	})

	mux.HandleFunc("DELETE /pos/v1/kitchen/stations/{id}", func(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "档口不存在"})
	case errors.Is(err, ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"message": "档口名重复"})
	case errors.Is(err, ErrBadPrinter):
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "打印机地址不合法（应形如 tcp://192.168.1.50:9100 或 Windows 队列名）"})
	case errors.Is(err, ErrBadRequest):
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "名称与打印机名不能为空"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
