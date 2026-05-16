package httpapi

import (
	"context"
	"encoding/json"

	"github.com/frp-easy/frp-easy/internal/frpconf"
	"github.com/frp-easy/frp-easy/internal/storage"
)

// renderAndApply 从 DB 读取配置，生成 FRP TOML 写入文件，
// 最后调用 procmgr.ApplyConfigChange。
//
// TOML 写入或 reload 失败时返回 error（由调用方决策）。
// 即使进程已停止也写入 TOML（下次 Start 时生效）。
func (h *handlers) renderAndApply(ctx context.Context, kind string) error {
	switch kind {
	case "frpc":
		return h.renderAndApplyFrpc(ctx)
	case "frps":
		return h.renderAndApplyFrps(ctx)
	}
	return nil
}

func (h *handlers) renderAndApplyFrpc(ctx context.Context) error {
	if h.deps.ConfigPaths == nil {
		return nil // 测试环境等未配置时跳过
	}
	path := h.deps.ConfigPaths["frpc"]
	logPath := ""
	if h.deps.LogFiles != nil {
		logPath = h.deps.LogFiles["frpc"]
	}

	// 1. 从 KV 读取 frpc 服务端连接信息
	var conn FrpcServerConn
	if v, ok, err := h.deps.Store.KVGet(ctx, kvFrpcServerCfg); err == nil && ok {
		_ = json.Unmarshal([]byte(v), &conn)
	}

	// 2. 从 DB 读取已启用的 Proxy 列表
	all, err := h.deps.Store.ListProxies(ctx)
	if err != nil {
		return err
	}
	var proxies []frpconf.ProxyInput
	for _, p := range all {
		if !p.Enabled {
			continue
		}
		pi := frpconf.ProxyInput{
			Name:          p.Name,
			Type:          p.Type,
			LocalIP:       p.LocalIP,
			LocalPort:     p.LocalPort,
			RemotePort:    p.RemotePort,
			CustomDomains: p.CustomDomains,
		}
		proxies = append(proxies, pi)
	}

	// 3. frpc admin 凭据（webServer.* 段）
	admin := h.deps.FrpcAdmin

	// 4. TOML 生成
	in := frpconf.FrpcRenderInput{
		ServerAddr: conn.ServerAddr,
		ServerPort: conn.ServerPort,
		AuthMethod: conn.AuthMethod,
		AuthToken:  conn.AuthToken,
		Proxies:    proxies,
		AdminAddr:  admin.Addr,
		AdminPort:  admin.Port,
		AdminUser:  admin.User,
		AdminPass:  admin.Pass,
		LogPath:    logPath,
	}
	data, err := frpconf.RenderFrpc(in)
	if err != nil {
		return err
	}

	// 5. 原子写入文件
	if err := frpconf.AtomicWrite(path, data); err != nil {
		return err
	}

	// 6. 通知 procmgr 配置已变更（进程停止时无害）
	if h.deps.ProcMgr != nil {
		_ = h.deps.ProcMgr.ApplyConfigChange("frpc")
	}
	return nil
}

func (h *handlers) renderAndApplyFrps(ctx context.Context) error {
	if h.deps.ConfigPaths == nil {
		return nil
	}
	path := h.deps.ConfigPaths["frps"]
	logPath := ""
	if h.deps.LogFiles != nil {
		logPath = h.deps.LogFiles["frps"]
	}

	// 1. 从 KV 读取 frps 配置
	var cfg FrpsConfig
	if v, ok, err := h.deps.Store.KVGet(ctx, kvFrpsConfig); err == nil && ok {
		_ = json.Unmarshal([]byte(v), &cfg)
	}
	if cfg.BindPort == 0 {
		cfg.BindPort = 7000
	}

	// 2. TOML 生成
	in := frpconf.FrpsRenderInput{
		BindPort:         cfg.BindPort,
		AuthMethod:       cfg.AuthMethod,
		AuthToken:        cfg.AuthToken,
		DashboardEnabled: cfg.DashboardEnabled,
		DashboardAddr:    cfg.DashboardAddr,
		DashboardPort:    cfg.DashboardPort,
		DashboardUser:    cfg.DashboardUser,
		DashboardPass:    cfg.DashboardPass,
		LogPath:          logPath,
	}
	data, err := frpconf.RenderFrps(in)
	if err != nil {
		return err
	}

	// 3. 原子写入文件
	if err := frpconf.AtomicWrite(path, data); err != nil {
		return err
	}

	// 4. 通知 procmgr
	if h.deps.ProcMgr != nil {
		_ = h.deps.ProcMgr.ApplyConfigChange("frps")
	}
	return nil
}

// proxyToFrpconf 将 storage.Proxy 转换为 frpconf.ProxyInput 的工具函数。
func proxyToFrpconf(p storage.Proxy) frpconf.ProxyInput {
	return frpconf.ProxyInput{
		Name:          p.Name,
		Type:          p.Type,
		LocalIP:       p.LocalIP,
		LocalPort:     p.LocalPort,
		RemotePort:    p.RemotePort,
		CustomDomains: p.CustomDomains,
	}
}
