package httpapi

// T-039: server runtime monitoring handlers.
//
// 4 条 REST 路由代理 frps admin HTTP API，让前端不必直接访问 frps 内部 dashboard：
//
//   GET /api/v1/server/runtime/info                  → frpsadmin.ServerInfo()
//   GET /api/v1/server/runtime/proxies               → frpsadmin.Proxies() × N 个 type 聚合
//   GET /api/v1/server/runtime/proxy/{type}/{name}   → frpsadmin.ProxyDetail()
//   GET /api/v1/server/runtime/traffic/{name}        → frpsadmin.Traffic()
//
// 凭据策略（FR-3 / D-1 / Q-2）：
//   - 用户在 Server 设置页填的 user/pass 优先；
//   - 未填则 fallback 到 KV `frps.dashboard.autogen`（main.go::ensureFrpsDashboardCreds 启动期生成）；
//   - DashboardEnabled=false → handler 503 + 友好引导，**不**自动翻 true（尊重用户禁用意图，
//     收敛了 01 §FR-3.3 原文，理由见 02 §3.4 design drift 段）。
//
// 错误映射（FR-2.7）：
//   - frpsadmin.ErrUnauthorized → HTTP 502（区分本应用的 401 SessionAuth 失败）
//   - frpsadmin.ErrNotFound     → HTTP 404
//   - frpsadmin.ErrUnavailable  → HTTP 503
//   - 其它                       → HTTP 502

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/frp-easy/frp-easy/internal/frpsadmin"
	"github.com/go-chi/chi/v5"
)

// kvFrpsDashboardAutogen 是 ensureFrpsDashboardCreds 持久化 autogen 凭据的 KV key。
// 与 frps.config（用户填写值）分开 key，让用户能区分"自己填的"和"系统兜底"。
const kvFrpsDashboardAutogen = "frps.dashboard.autogen"

// FrpsDashboardCreds 是 KV `frps.dashboard.autogen` 的 JSON value 结构。
// 与 cmd/frp-easy/main.go::ensureFrpsDashboardCreds 写入字面一致。
type FrpsDashboardCreds struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

// frpsProxyTypes 是 GET /api/v1/server/runtime/proxies 聚合的 type 清单（FR-1.4 全集）。
var frpsProxyTypes = []string{"tcp", "udp", "http", "https", "stcp", "sudp", "xtcp"}

// ServerRuntimeProxiesResponse 是 GET /api/v1/server/runtime/proxies 的响应。
// 部分 type 失败 → `errors[type] = 错误文案`；整体仍 200（Q-1 PM-DECIDED）。
// 全部 type 都 fatal（凭据失效 / 连接失败）→ handler 直接返 5xx，不进 200 body。
type ServerRuntimeProxiesResponse struct {
	Proxies map[string][]frpsadmin.ProxyStatus `json:"proxies"`
	Errors  map[string]string                  `json:"errors,omitempty"`
}

// resolveFrpsDashboard 从 KV 读 frps.config + frps.dashboard.autogen，合并出"实际生效"的 dashboard 凭据。
//
// 优先级（FR-3.4 / FR-3.5）：
//  1. frps.config 中用户填写的 user/pass 非空 → 用用户值；
//  2. 否则从 frps.dashboard.autogen 读 autogen；
//  3. DashboardEnabled=false → 返回 enabled=false 让 handler 走 503。
//
// 返回 (addr, port, user, pass, enabled, err)：
//   - enabled=false 表示用户未启用 dashboard，handler 应返 503；
//   - err 仅在 KV 也无 autogen 时返回（理论上 ensureFrpsDashboardCreds 保证有，兜底）。
func (h *handlers) resolveFrpsDashboard(ctx context.Context) (addr string, port int, user, pass string, enabled bool, err error) {
	var cfg FrpsConfig
	if v, ok, _ := h.deps.Store.KVGet(ctx, kvFrpsConfig); ok {
		_ = json.Unmarshal([]byte(v), &cfg)
	}
	if !cfg.DashboardEnabled {
		return "", 0, "", "", false, nil
	}
	addr = cfg.DashboardAddr
	if addr == "" {
		addr = "127.0.0.1"
	}
	port = cfg.DashboardPort
	if port == 0 {
		port = 7500
	}
	user = cfg.DashboardUser
	pass = cfg.DashboardPass
	if user == "" || pass == "" {
		var auto FrpsDashboardCreds
		if v, ok, _ := h.deps.Store.KVGet(ctx, kvFrpsDashboardAutogen); ok {
			_ = json.Unmarshal([]byte(v), &auto)
		}
		if user == "" {
			user = auto.User
		}
		if pass == "" {
			pass = auto.Pass
		}
	}
	if user == "" || pass == "" {
		return addr, port, "", "", true, errors.New("dashboard 凭据缺失")
	}
	return addr, port, user, pass, true, nil
}

// buildFrpsAdminClient 构造 frpsadmin.Client；失败时 w 已写出响应、返回 nil 让 handler 直接 return。
//
// frpsAdminFactory 是测试 seam（默认 frpsadmin.New）；让 handler 单测能注入 mock httptest URL。
var frpsAdminFactory = func(addr string, port int, user, pass string) *frpsadmin.Client {
	return frpsadmin.New(addr, port, user, pass)
}

func (h *handlers) buildFrpsAdminClient(w http.ResponseWriter, r *http.Request) *frpsadmin.Client {
	addr, port, user, pass, enabled, err := h.resolveFrpsDashboard(r.Context())
	if !enabled {
		writeError(w, http.StatusServiceUnavailable, CodeInternal,
			"frps dashboard 未启用。请到 Server 设置页打开 Dashboard 开关并保存。", "")
		return nil
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeInternal,
			"frps dashboard 凭据不可用："+err.Error(), "")
		return nil
	}
	return frpsAdminFactory(addr, port, user, pass)
}

// writeFrpsadminError 是 4 个 handler 共用的上游错误映射（FR-2.7）。
func (h *handlers) writeFrpsadminError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, frpsadmin.ErrUnauthorized):
		writeError(w, http.StatusBadGateway, CodeInternal,
			"frps dashboard 凭据校验失败（401）。请到 Server 设置页清空 user/pass 由 frp_easy 重新生成。", "")
	case errors.Is(err, frpsadmin.ErrNotFound):
		writeError(w, http.StatusNotFound, CodeNotFound, "未找到对应的 proxy", "")
	case errors.Is(err, frpsadmin.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, CodeInternal,
			"frps 进程不可达。请确认 frps 已启动且 dashboard 端口配置正确。", "")
	default:
		writeError(w, http.StatusBadGateway, CodeInternal,
			"调用 frps dashboard 失败："+err.Error(), "")
	}
}

// GET /api/v1/server/runtime/info
func (h *handlers) serverRuntimeInfo(w http.ResponseWriter, r *http.Request) {
	c := h.buildFrpsAdminClient(w, r)
	if c == nil {
		return
	}
	info, err := c.ServerInfo(r.Context())
	if err != nil {
		h.writeFrpsadminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// GET /api/v1/server/runtime/proxies
//
// 聚合 N 个 type；per-type 失败收集到 errors map（Q-1 PM-DECIDED）。
// 全 fatal（凭据失效 / 连接失败）→ 直接走错误映射返 5xx，避免给前端无效 200 body。
//
// C-2 优化：循环内记下第一个 fatal err，避免事后重调上游拿 sentinel（02 §3.2 GR C-2）。
func (h *handlers) serverRuntimeProxies(w http.ResponseWriter, r *http.Request) {
	c := h.buildFrpsAdminClient(w, r)
	if c == nil {
		return
	}
	resp := ServerRuntimeProxiesResponse{
		Proxies: map[string][]frpsadmin.ProxyStatus{},
		Errors:  map[string]string{},
	}
	var firstFatal error
	fatalCount := 0
	for _, t := range frpsProxyTypes {
		list, err := c.Proxies(r.Context(), t)
		if err != nil {
			if errors.Is(err, frpsadmin.ErrUnauthorized) || errors.Is(err, frpsadmin.ErrUnavailable) {
				fatalCount++
				if firstFatal == nil {
					firstFatal = err
				}
			}
			resp.Errors[t] = err.Error()
			continue
		}
		resp.Proxies[t] = list
	}
	if fatalCount == len(frpsProxyTypes) && firstFatal != nil {
		h.writeFrpsadminError(w, firstFatal)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /api/v1/server/runtime/proxy/{type}/{name}
func (h *handlers) serverRuntimeProxyDetail(w http.ResponseWriter, r *http.Request) {
	c := h.buildFrpsAdminClient(w, r)
	if c == nil {
		return
	}
	pt := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")
	detail, err := c.ProxyDetail(r.Context(), pt, name)
	if err != nil {
		h.writeFrpsadminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// GET /api/v1/server/runtime/traffic/{name}
func (h *handlers) serverRuntimeTraffic(w http.ResponseWriter, r *http.Request) {
	c := h.buildFrpsAdminClient(w, r)
	if c == nil {
		return
	}
	name := chi.URLParam(r, "name")
	traffic, err := c.Traffic(r.Context(), name)
	if err != nil {
		h.writeFrpsadminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, traffic)
}
