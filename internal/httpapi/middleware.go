package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/frp-easy/frp-easy/internal/storage"
)

// ctxKey 是放进 request context 的 key 类型。
type ctxKey string

const (
	ctxKeySession ctxKey = "session"
	ctxKeyReqID   ctxKey = "request-id"
)

// ReadyGate 是 chi 链最前端的中间件 —— 实现 Gate Review C-3。
//
// 当 ready() 返回 false 时：
//   - 写方法（POST/PUT/PATCH/DELETE）→ 503 NOT_READY + Retry-After: 2 + 中文 message。
//   - 读方法（GET/HEAD/OPTIONS）→ 透传，让客户端能拿 /api/v1/system/ready 决定下一步。
func ReadyGate(ready func() bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ready != nil && !ready() && isWriteMethod(r.Method) {
				w.Header().Set("Retry-After", "2")
				writeError(w, http.StatusServiceUnavailable, CodeNotReady,
					"服务正在初始化，请稍后再试", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isWriteMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// Recover 兜底 panic → 500 INTERNAL，避免进程退出。
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if logger != nil {
						logger.Error("panic recovered",
							"err", rec,
							"path", r.URL.Path,
							"stack", string(debug.Stack()))
					}
					writeError(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误", "")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID 给每个请求挂 X-Request-ID 头（来自客户端或本地生成）。
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = randomID()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), ctxKeyReqID, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// randomID 用 time.Now().UnixNano() 简单生成，请求级别足够。
func randomID() string {
	return time.Now().UTC().Format("20060102T150405.000000000")
}

// 【C-5】Logger 中间件 + 脱敏过滤器。
//
// 输出结构化日志（slog）。请求体如果是 JSON，会复制一份用 redact 把
// password / oldPassword / newPassword / authToken / token 字段替换为 "***" 后再记
// （**绝不写明文凭据**，对应 NF-S6）。
var redactKeys = []string{
	"password", "oldPassword", "newPassword", "authToken", "token",
}

// Logger 返回 slog 日志中间件。
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			// 读完 body 后回填，让 handler 仍能读取。
			var bodyForLog []byte
			if r.Body != nil && r.ContentLength > 0 && r.ContentLength < 64*1024 {
				if buf, err := io.ReadAll(r.Body); err == nil {
					bodyForLog = buf
					r.Body = io.NopCloser(bytes.NewReader(buf))
				}
			}

			ww := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(ww, r)

			logger.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.status,
				"dur_ms", time.Since(start).Milliseconds(),
				"reqID", reqIDFromCtx(r.Context()),
				"body", string(redact(bodyForLog, redactKeys...)),
			)
		})
	}
}

// redact 把 body 当作 JSON 解析，把指定 key 的值替换为 "***"，再序列化回去。
// body 不是 JSON 或解析失败 → 返回 "<binary or non-json>" 占位，不泄露原文。
func redact(body []byte, keys ...string) []byte {
	if len(body) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return []byte("<non-json>")
	}
	redactValue(v, keys)
	out, _ := json.Marshal(v)
	return out
}

func redactValue(v any, keys []string) {
	switch x := v.(type) {
	case map[string]any:
		for k, vv := range x {
			if containsFold(keys, k) {
				x[k] = "***"
				continue
			}
			redactValue(vv, keys)
		}
	case []any:
		for _, item := range x {
			redactValue(item, keys)
		}
	}
}

func containsFold(ks []string, k string) bool {
	for _, kk := range ks {
		if strings.EqualFold(kk, k) {
			return true
		}
	}
	return false
}

// statusRecorder 截获 status code 给 logger 用。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// SecurityHeaders 给所有响应注入最低基线安全头（T-007 AC-3）：
//
//   - X-Content-Type-Options: nosniff —— 阻止浏览器对响应做 MIME sniff。
//   - X-Frame-Options: DENY —— 禁止任何 origin iframe 嵌入；适配 frp_easy 当前
//     单机本地 UI 定位；如未来需要 SSO / OAuth 跳转再独立调整。
//   - Referrer-Policy: no-referrer —— 不向任何 link / form 跳转目标泄露
//     UI URL，避免可能的端口 / 路径侧信道。
//
// 设计：在 next.ServeHTTP 之前 Set，确保即便下游 handler panic 被 Recover 兜底
// 仍能写出三个头。挂载位置：router.New 顶层 r.Use，先于 /api/v1/health 注册
// 之前（chi 文档保证全局 Use 覆盖后续注册的所有路由，包括 NotFound / SPA fallback）。
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			next.ServeHTTP(w, r)
		})
	}
}

// CORS 仅 dev 模式打开（02 §3.9）。MVP 期默认 prod 关闭。
func CORS(allow bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allow {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-CSRF-Token,X-Request-ID,Cookie")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SessionAuth 解析 cookie，验证 session，把 *storage.Session 放进 context。
// 未通过则 401。
func SessionAuth(store *storage.Store, cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(cookieName)
			if err != nil || c.Value == "" {
				writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "未登录", "")
				return
			}
			sess, err := store.GetSession(r.Context(), c.Value)
			if err != nil || sess == nil {
				writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "会话已失效", "")
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeySession, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CSRF 校验写接口（POST/PUT/PATCH/DELETE）必须带 X-CSRF-Token 头，
// 且与 session 的 csrf_token 字段一致（NF-S3 双保险）。
func CSRF() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isWriteMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			sess, _ := r.Context().Value(ctxKeySession).(*storage.Session)
			if sess == nil {
				// SessionAuth 没注入 session 的写接口（比如 /setup、/login）不走 CSRF。
				next.ServeHTTP(w, r)
				return
			}
			tok := r.Header.Get("X-CSRF-Token")
			if tok == "" || tok != sess.CSRFToken {
				writeError(w, http.StatusForbidden, CodeCSRFFailed, "CSRF token 校验失败", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func reqIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyReqID).(string)
	return v
}

func sessionFromCtx(ctx context.Context) *storage.Session {
	v, _ := ctx.Value(ctxKeySession).(*storage.Session)
	return v
}
