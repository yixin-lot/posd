package staff

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type staffJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Register 挂载登录、当前用户与员工管理接口。
func (s *Service) Register(mux *http.ServeMux) {
	s.registerAuth(mux)
	s.registerStaffAdmin(mux)
}

func (s *Service) registerAuth(mux *http.ServeMux) {
	mux.HandleFunc("POST /pos/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
			Pin      string `json:"pin"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		token, st, err := s.Login(body.Username, body.Pin)
		if err != nil {
			var locked *LockedError
			if errors.As(err, &locked) {
				writeJSON(w, http.StatusLocked, map[string]any{
					"code":     http.StatusText(http.StatusLocked),
					"message":  locked.Error(),
					"retryIn":  locked.Remaining,
				})
				return
			}
			if errors.Is(err, ErrUnauthorized) {
				writeErr(w, http.StatusUnauthorized, "账号或 PIN 不正确")
				return
			}
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"token": token,
			"staff": staffJSON{ID: st.ID, Name: st.Name, Role: st.Role},
		})
	})

	mux.HandleFunc("POST /pos/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		// 服务端删会话：否则 token 泄漏后仍然可用。
		_ = s.Logout(bearer(r))
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /pos/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		st, ok := s.Verify(bearer(r))
		if !ok {
			writeErr(w, http.StatusUnauthorized, "登录已过期，请重新登录")
			return
		}
		writeJSON(w, http.StatusOK, staffJSON{ID: st.ID, Name: st.Name, Role: st.Role})
	})
}

// Middleware 保护 /pos/v1/*（健康检查与登录除外）。
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		// 图片是给 <img src> 用的，浏览器不会带 Authorization，必须放行
		if !strings.HasPrefix(p, "/pos/v1/") || p == "/pos/v1/health" || p == "/pos/v1/auth/login" ||
			strings.HasPrefix(p, "/pos/v1/images/") {
			next.ServeHTTP(w, r)
			return
		}
		st, ok := s.Verify(bearer(r))
		if !ok {
			writeErr(w, http.StatusUnauthorized, "登录已过期，请重新登录")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithContext(r.Context(), st)))
	})
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"code": http.StatusText(code), "message": message})
}
