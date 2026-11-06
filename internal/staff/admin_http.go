// 员工管理 HTTP 路由（设置页）。仅店长（manager）可访问。
package staff

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type staffRecordJSON struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"createdAt"`
}

type staffSaveRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Pin      string `json:"pin"`
	Role     string `json:"role"`
}

func (s *Service) registerStaffAdmin(mux *http.ServeMux) {
	// 员工管理属店长权限；ManagerOnly 内部校验当前登录人角色。
	mux.HandleFunc("GET /pos/v1/staff", s.withManager(func(w http.ResponseWriter, r *http.Request) {
		list, err := s.List()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := make([]staffRecordJSON, 0, len(list))
		for _, st := range list {
			out = append(out, toRecordJSON(st))
		}
		writeJSON(w, http.StatusOK, out)
	}))

	mux.HandleFunc("POST /pos/v1/staff", s.withManager(func(w http.ResponseWriter, r *http.Request) {
		in, err := decodeStaffSave(w, r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		st, err := s.Create(in)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, toRecordJSON(st))
	}))

	mux.HandleFunc("PUT /pos/v1/staff/{id}", s.withManager(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "员工编号不合法")
			return
		}
		in, err := decodeStaffSave(w, r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		st, err := s.Update(id, in)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toRecordJSON(st))
	}))

	mux.HandleFunc("POST /pos/v1/staff/{id}/active", s.withManager(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "员工编号不合法")
			return
		}
		var body struct {
			Active bool `json:"active"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		st, err := s.SetActive(id, body.Active)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toRecordJSON(st))
	}))

	mux.HandleFunc("DELETE /pos/v1/staff/{id}", s.withManager(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "员工编号不合法")
			return
		}
		if err := s.Delete(id); err != nil {
			writeServiceErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

// withManager 校验当前登录人为店长；否则 403。
func (s *Service) withManager(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, ok := s.Verify(bearer(r))
		if !ok {
			writeErr(w, http.StatusUnauthorized, "登录已过期，请重新登录")
			return
		}
		if st.Role != "manager" {
			writeErr(w, http.StatusForbidden, "仅店长可管理员工")
			return
		}
		next(w, r)
	}
}

func decodeStaffSave(w http.ResponseWriter, r *http.Request) (SaveInput, error) {
	var req staffSaveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		return SaveInput{}, errors.New("请求体不是合法 JSON")
	}
	return SaveInput{Name: req.Name, Username: req.Username, Pin: req.Pin, Role: req.Role}, nil
}

func toRecordJSON(st StaffRecord) staffRecordJSON {
	return staffRecordJSON{
		ID: st.ID, Name: st.Name, Username: st.Username,
		Role: st.Role, Active: st.Active, CreatedAt: st.CreatedAt,
	}
}

func writeServiceErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrBadRequest):
		writeErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "参数不合法: "))
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}
