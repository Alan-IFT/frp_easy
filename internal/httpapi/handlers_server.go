package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/frp-easy/frp-easy/internal/frpconf"
)

// FrpsConfig 是 GET/PUT /api/v1/server 的载荷（02 B-15）。
//
// AuthToken 在 GET 时默认脱敏为 "***"（除非 ?reveal=1）。
//
// AllowPorts（T-040）是端口策略白名单。GET 信任 KV 持久化数据（曾经过 PUT 校验落盘，
// 路径上无绕过可能）；PUT 时调 frpconf.ValidateFrpsAllowPorts 守门，错误时 422 +
// field="allowPorts"。
type FrpsConfig struct {
	BindPort         int              `json:"bindPort"`
	AuthMethod       string           `json:"authMethod,omitempty"`
	AuthToken        string           `json:"authToken,omitempty"`
	DashboardEnabled bool             `json:"dashboardEnabled,omitempty"`
	DashboardAddr    string           `json:"dashboardAddr,omitempty"`
	DashboardPort    int              `json:"dashboardPort,omitempty"`
	DashboardUser    string           `json:"dashboardUser,omitempty"`
	DashboardPass    string           `json:"dashboardPass,omitempty"`
	AllowPorts       []AllowPortRange `json:"allowPorts,omitempty"` // T-040
}

// AllowPortRange 是 PUT/GET /api/v1/server 中 allowPorts 数组项（T-040）。
//
// 与 frpconf.FrpsAllowPortRange 一一对应。omitempty 让"未填"字段序列化为零值，
// 校验逻辑统一在 frpconf.ValidateFrpsAllowPorts，避免双实现漂移。
type AllowPortRange struct {
	Start  int `json:"start,omitempty"`
	End    int `json:"end,omitempty"`
	Single int `json:"single,omitempty"`
}

// toFrpconfAllowPorts 把 handlers 层数组转 frpconf 层数组（字段一一对应）。
// 抽出 helper 让 putServer 校验路径与 renderAndApplyFrps 渲染路径共享同一转换源。
func toFrpconfAllowPorts(in []AllowPortRange) []frpconf.FrpsAllowPortRange {
	if len(in) == 0 {
		return nil
	}
	out := make([]frpconf.FrpsAllowPortRange, len(in))
	for i, r := range in {
		out[i] = frpconf.FrpsAllowPortRange{Start: r.Start, End: r.End, Single: r.Single}
	}
	return out
}

const (
	kvFrpsConfig    = "frps.config"
	kvFrpcServerCfg = "frpc.serverConn"
)

// FrpcServerConn 是 GET/PUT /api/v1/client 的载荷。
type FrpcServerConn struct {
	ServerAddr string `json:"serverAddr"`
	ServerPort int    `json:"serverPort"`
	AuthMethod string `json:"authMethod,omitempty"`
	AuthToken  string `json:"authToken,omitempty"`
}

func (h *handlers) getServer(w http.ResponseWriter, r *http.Request) {
	var cfg FrpsConfig
	if v, ok, err := h.deps.Store.KVGet(r.Context(), kvFrpsConfig); err == nil && ok {
		_ = json.Unmarshal([]byte(v), &cfg)
	}
	if cfg.BindPort == 0 {
		cfg.BindPort = 7000
	}
	if r.URL.Query().Get("reveal") != "1" && cfg.AuthToken != "" {
		cfg.AuthToken = "***"
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *handlers) putServer(w http.ResponseWriter, r *http.Request) {
	var cfg FrpsConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}
	if cfg.BindPort == 0 {
		cfg.BindPort = 7000
	}
	if err := ValidatePort(cfg.BindPort, "bindPort"); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), "bindPort")
		return
	}
	if cfg.DashboardEnabled {
		if cfg.DashboardPort == 0 {
			cfg.DashboardPort = 7500
		}
		if err := ValidatePort(cfg.DashboardPort, "dashboardPort"); err != nil {
			writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), "dashboardPort")
			return
		}
	}
	// T-040：allowPorts 端口策略校验。空 / nil 跳过；非空走 frpconf 包统一校验函数
	// 避免前后端双实现漂移。错误时 422 + field="allowPorts"，错误文本含 `allowPorts[i]`
	// 字面让前端能定位（虽然前端已 mirror 一遍校验，这里是 belt-and-suspenders）。
	if len(cfg.AllowPorts) > 0 {
		if err := frpconf.ValidateFrpsAllowPorts(toFrpconfAllowPorts(cfg.AllowPorts)); err != nil {
			writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), "allowPorts")
			return
		}
	}
	// 如果客户端发回的 token 仍为 "***" 占位（典型回写未修改场景），保留原值。
	if cfg.AuthToken == "***" {
		if v, ok, err := h.deps.Store.KVGet(r.Context(), kvFrpsConfig); err == nil && ok {
			var prev FrpsConfig
			if json.Unmarshal([]byte(v), &prev) == nil {
				cfg.AuthToken = prev.AuthToken
			}
		}
	}
	buf, _ := json.Marshal(cfg)
	if err := h.deps.Store.KVSet(r.Context(), kvFrpsConfig, string(buf)); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "保存失败", "")
		return
	}
	// 脱敏后返回。
	if cfg.AuthToken != "" {
		cfg.AuthToken = "***"
	}
	writeJSON(w, http.StatusOK, cfg)
	h.applyConfigBestEffort(r.Context(), "frps")
}

func (h *handlers) getClient(w http.ResponseWriter, r *http.Request) {
	var cfg FrpcServerConn
	if v, ok, err := h.deps.Store.KVGet(r.Context(), kvFrpcServerCfg); err == nil && ok {
		_ = json.Unmarshal([]byte(v), &cfg)
	}
	if r.URL.Query().Get("reveal") != "1" && cfg.AuthToken != "" {
		cfg.AuthToken = "***"
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *handlers) putClient(w http.ResponseWriter, r *http.Request) {
	var cfg FrpcServerConn
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}
	if cfg.ServerAddr == "" {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "serverAddr 必填", "serverAddr")
		return
	}
	if err := ValidatePort(cfg.ServerPort, "serverPort"); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), "serverPort")
		return
	}
	if cfg.AuthToken == "***" {
		if v, ok, err := h.deps.Store.KVGet(r.Context(), kvFrpcServerCfg); err == nil && ok {
			var prev FrpcServerConn
			if json.Unmarshal([]byte(v), &prev) == nil {
				cfg.AuthToken = prev.AuthToken
			}
		}
	}
	buf, _ := json.Marshal(cfg)
	if err := h.deps.Store.KVSet(r.Context(), kvFrpcServerCfg, string(buf)); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "保存失败", "")
		return
	}
	if cfg.AuthToken != "" {
		cfg.AuthToken = "***"
	}
	writeJSON(w, http.StatusOK, cfg)
	h.applyConfigBestEffort(r.Context(), "frpc")
}
