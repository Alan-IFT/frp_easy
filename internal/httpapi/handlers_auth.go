package httpapi

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/frp-easy/frp-easy/internal/auth"
)

// LoginRequest 见 02 §5.2。
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	OK bool `json:"ok"`
}

// MeResponse —— GET /auth/me。
type MeResponse struct {
	Username string `json:"username"`
}

// CSRFResponse —— GET /auth/csrf。
type CSRFResponse struct {
	CSRFToken string `json:"csrfToken"`
}

// ChangePasswordRequest —— POST /auth/password。
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if allowed, retryAfter := h.deps.RateLimiter.Allow(ip); !allowed {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, CodeRateLimited,
			"登录尝试过多，请稍后再试", "")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}
	admin, err := h.deps.Store.GetAdmin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "读取管理员失败", "")
		return
	}
	if admin == nil {
		writeError(w, http.StatusUnauthorized, CodeSetupRequired, "请先完成初始化", "")
		return
	}

	if req.Username != admin.Username {
		recordLoginFail(h.deps.RateLimiter, ip, w)
		writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "用户名或密码错误", "")
		return
	}
	ok, err := auth.VerifyPassword(req.Password, admin.PasswordHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "密码校验失败", "")
		return
	}
	if !ok {
		recordLoginFail(h.deps.RateLimiter, ip, w)
		writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "用户名或密码错误", "")
		return
	}

	_ = h.deps.RateLimiter.Reset(ip)
	sess, err := h.deps.Store.CreateSession(r.Context(), SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "创建会话失败", "")
		return
	}
	setSessionCookie(w, sess.Token, SessionTTL)
	writeJSON(w, http.StatusOK, LoginResponse{OK: true})
}

func recordLoginFail(rl *auth.RateLimiter, ip string, w http.ResponseWriter) {
	count, retryAfter, _ := rl.RecordFailure(ip)
	_ = count
	if retryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())+1))
	}
}

func (h *handlers) logout(w http.ResponseWriter, r *http.Request) {
	sess := sessionFromCtx(r.Context())
	if sess != nil {
		_ = h.deps.Store.DeleteSession(r.Context(), sess.Token)
	}
	// 清 cookie
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handlers) me(w http.ResponseWriter, r *http.Request) {
	admin, err := h.deps.Store.GetAdmin(r.Context())
	if err != nil || admin == nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "管理员信息不可读", "")
		return
	}
	writeJSON(w, http.StatusOK, MeResponse{Username: admin.Username})
}

func (h *handlers) csrf(w http.ResponseWriter, r *http.Request) {
	sess := sessionFromCtx(r.Context())
	if sess == nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "未登录", "")
		return
	}
	w.Header().Set("X-CSRF-Token", sess.CSRFToken)
	writeJSON(w, http.StatusOK, CSRFResponse{CSRFToken: sess.CSRFToken})
}

func (h *handlers) changePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}
	admin, err := h.deps.Store.GetAdmin(r.Context())
	if err != nil || admin == nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "未登录", "")
		return
	}
	ok, err := auth.VerifyPassword(req.OldPassword, admin.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "旧密码不正确", "oldPassword")
		return
	}
	if err := ValidatePassword(req.NewPassword); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), "newPassword")
		return
	}
	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "哈希失败", "")
		return
	}
	if err := h.deps.Store.SetAdmin(r.Context(), admin.Username, newHash); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "保存失败", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
