package httpapi

import (
	"context"
	"encoding/json"

	"github.com/frp-easy/frp-easy/internal/frpconf"
	"github.com/frp-easy/frp-easy/internal/storage"
)

// renderAndApply は DB から設定を読み出し、FRP TOML を生成してファイルに書き込み、
// 最後に procmgr.ApplyConfigChange を呼ぶ。
//
// TOML 書き込みまたは reload 失敗はエラーを返す（呼び出し元が判断して対応）。
// プロセスが停止中の場合でも TOML は書き込む（次回 Start 時に有効化される）。
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
		return nil // テスト環境等で未設定の場合は skip
	}
	path := h.deps.ConfigPaths["frpc"]
	logPath := ""
	if h.deps.LogFiles != nil {
		logPath = h.deps.LogFiles["frpc"]
	}

	// 1. frpc サーバー接続情報を KV から読む
	var conn FrpcServerConn
	if v, ok, err := h.deps.Store.KVGet(ctx, kvFrpcServerCfg); err == nil && ok {
		_ = json.Unmarshal([]byte(v), &conn)
	}

	// 2. 有効な Proxy 一覧を DB から読む（enabled のみ）
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

	// 5. ファイルに原子書き込み
	if err := frpconf.AtomicWrite(path, data); err != nil {
		return err
	}

	// 6. procmgr に設定変更を通知（プロセスが停止中でも無害）
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

	// 1. frps 設定を KV から読む
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

	// 3. ファイルに原子書き込み
	if err := frpconf.AtomicWrite(path, data); err != nil {
		return err
	}

	// 4. procmgr に通知
	if h.deps.ProcMgr != nil {
		_ = h.deps.ProcMgr.ApplyConfigChange("frps")
	}
	return nil
}

// proxyToFrpconf は storage.Proxy を frpconf.ProxyInput に変換するユーティリティ。
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
