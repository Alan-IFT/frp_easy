package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/frp-easy/frp-easy/internal/assets"
	"github.com/frp-easy/frp-easy/internal/auth"
	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/downloader"
	"github.com/frp-easy/frp-easy/internal/procmgr"
	"github.com/frp-easy/frp-easy/internal/storage"

	"github.com/go-chi/chi/v5"
)

// SessionCookieName 是写入浏览器的 cookie 名（02 §5.1）。
const (
	SessionCookieName = "frp_easy_sid"
	SessionTTL        = 12 * time.Hour
)

// Dependencies 把所有 handler 需要的外部依赖集中起来，方便 main.go 注入。
type Dependencies struct {
	Store       *storage.Store
	Locator     binloc.Locator
	ProcMgr     *procmgr.Manager
	RateLimiter *auth.RateLimiter
	LogFiles    map[string]string // kind → log file path
	ConfigPaths map[string]string // kind → toml file path (runtime/frpc.toml etc)
	FrpcAdmin   FrpcAdminCreds   // frpc admin API 凭据（用于 webServer.* 渲染）
	Ready       func() bool
	Logger      *slog.Logger
	DevMode     bool   // true 时开 CORS（vite dev）
	Version     string // 注入到 /system/ready
	// Downloader manages async frp binary downloads (T-002).
	// nil is safe: download endpoints will return 503.
	Downloader *downloader.Manager
}

// FrpcAdminCreds 是 frpc admin API 凭据，持久化在 kv.frpc.admin。
type FrpcAdminCreds struct {
	Addr string
	Port int
	User string
	Pass string
}

// New 构造 chi router 并挂全部路由 + 中间件链。
//
// 中间件顺序（与 03 §F-4 / C-3 一致）：
//
//	ReadyGate → Recover → RequestID → Logger(脱敏) → CORS(dev only) → CSRF(写接口)
//	→ SessionAuth(受保护) → Handler
//
// 路由前缀：API 全部走 /api/v1/...；其它路径走 SPA 占位 handler。
// /api/v1/health 单独挂在顶层，不经过任何中间件（OPT-7）。
func New(d Dependencies) http.Handler {
	r := chi.NewRouter()

	h := &handlers{deps: d}

	// 【T-007 AC-3 / C-4】SecurityHeaders 必须在任何路由注册之前 r.Use，
	// 才能覆盖 /api/v1/health（顶层）与 SPA fallback（NotFound）。chi 文档保证
	// 全局 r.Use 对后续注册的全部路由生效，包括顶层 Get、Group、NotFound、
	// MethodNotAllowed 与 SPA 资源 handler。
	r.Use(SecurityHeaders())

	// 健康检查端点：不经过 ReadyGate 或任何其它业务中间件，服务启动中也可访问。
	// 注意：SecurityHeaders 仍会作用于此端点（顶层 r.Use 已挂）。
	r.Get("/api/v1/health", h.health)

	// 其余所有请求走完整中间件链。
	r.Group(func(r chi.Router) {
		r.Use(ReadyGate(d.Ready))
		r.Use(Recover(d.Logger))
		r.Use(RequestID())
		r.Use(Logger(d.Logger))
		r.Use(CORS(d.DevMode))

		r.Route("/api/v1", func(r chi.Router) {
			// 公开 endpoint（不需要 session）。
			r.Get("/system/ready", h.systemReady)
			r.Post("/setup", h.setup)
			r.Post("/auth/login", h.login)

			// 受保护 endpoint：先 SessionAuth → CSRF（仅写方法）。
			r.Group(func(r chi.Router) {
				r.Use(SessionAuth(d.Store, SessionCookieName))
				r.Use(CSRF())

				r.Post("/auth/logout", h.logout)
				r.Post("/auth/password", h.changePassword)
				r.Get("/auth/me", h.me)
				r.Get("/auth/csrf", h.csrf)

				r.Get("/mode", h.getMode)
				r.Put("/mode", h.putMode)

				r.Get("/proxies", h.listProxies)
				r.Post("/proxies", h.createProxy)
				r.Put("/proxies/{id}", h.updateProxy)
				r.Delete("/proxies/{id}", h.deleteProxy)

				r.Get("/server", h.getServer)
				r.Put("/server", h.putServer)
				r.Get("/client", h.getClient)
				r.Put("/client", h.putClient)

				r.Post("/proc/{kind}/start", h.procStart)
				r.Post("/proc/{kind}/stop", h.procStop)
				r.Post("/proc/{kind}/restart", h.procRestart)
				r.Get("/proc/status", h.procStatus)

				r.Get("/logs/{kind}", h.logs)

				// T-002: system utilities — public IP, binary download, wizard.
				r.Get("/system/public-ip", h.systemPublicIP)
				// T-038: 服务化状态 + 上次自动恢复结果（Dashboard "服务化状态" 卡片消费）。
				r.Get("/system/service-status", h.systemServiceStatus)
				r.Post("/system/download-bin", h.downloadBin)
				r.Get("/system/download-status/{kind}", h.downloadStatus)
				// T-027：取消下载（与 download-bin / download-status 同分组同中间件链）。
				r.Post("/system/download-cancel/{kind}", h.downloadCancel)
				// T-018: 二进制上传。
				r.Post("/system/upload-bin", h.uploadBin)
				r.Get("/wizard/status", h.wizardStatus)
				r.Post("/wizard/complete", h.wizardComplete)

				// T-039: server runtime monitoring — frps admin API 代理。
				// 4 条 GET 路由让前端查 frps 在线 client / proxy 状态 / 流量；
				// 凭据从 KV frps.config（用户填值）+ frps.dashboard.autogen（fallback）合并。
				r.Get("/server/runtime/info", h.serverRuntimeInfo)
				r.Get("/server/runtime/proxies", h.serverRuntimeProxies)
				r.Get("/server/runtime/proxy/{type}/{name}", h.serverRuntimeProxyDetail)
				r.Get("/server/runtime/traffic/{name}", h.serverRuntimeTraffic)
			})
		})
	})

	// SPA / 静态资源 fallback：非 /api/ 的请求一律交给 assets handler。
	r.NotFound(assets.Handler().ServeHTTP)
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, CodeNotFound, "method not allowed", "")
	})

	// 根路径同样走 SPA 占位（router NotFound 不接根）。
	r.Get("/", assets.Handler().ServeHTTP)

	return r
}

// handlers 持有依赖，每个 handler 是方法。
// C-1 gate: ipCache field must be declared here, not in handlers_system.go.
type handlers struct {
	deps    Dependencies
	ipCache ipCache // process-scoped public IP cache (type defined in handlers_system.go)
}
