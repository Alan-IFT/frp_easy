package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/frp-easy/frp-easy/internal/auth"
)

// SetupRequest 见 02 §5.2。
type SetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SetupResponse —— 成功后同时 set-cookie。
type SetupResponse struct {
	OK bool `json:"ok"`
}

func (h *handlers) setup(w http.ResponseWriter, r *http.Request) {
	// 已 init → 409 ALREADY_INITIALIZED（AC-3）。
	admin, err := h.deps.Store.GetAdmin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "读取管理员失败", "")
		return
	}
	if admin != nil {
		writeError(w, http.StatusConflict, CodeAlreadyInitialized, "管理员已设置，请直接登录", "")
		return
	}

	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}
	if err := ValidateUsername(req.Username); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), "username")
		return
	}
	if err := ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), "password")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "哈希失败", "")
		return
	}
	if err := h.deps.Store.SetAdmin(r.Context(), req.Username, hash); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "保存管理员失败", "")
		return
	}

	// 自动登录：建 session + 写 cookie。
	sess, err := h.deps.Store.CreateSession(r.Context(), SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "创建会话失败", "")
		return
	}
	setSessionCookie(w, sess.Token, SessionTTL)

	writeJSON(w, http.StatusOK, SetupResponse{OK: true})
}

func setSessionCookie(w http.ResponseWriter, token string, ttl interface{}) {
	c := &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// HTTPS 时调用方应设置 Secure；MVP 走 127.0.0.1 HTTP 默认不设。
	}
	if d, ok := ttl.(interface{ Seconds() float64 }); ok {
		c.MaxAge = int(d.Seconds())
	}
	http.SetCookie(w, c)
}
