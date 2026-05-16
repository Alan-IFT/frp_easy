package httpapi

import (
	"encoding/json"
	"net/http"
)

// FrpsConfig 是 GET/PUT /api/v1/server 的载荷（02 B-15）。
//
// AuthToken 在 GET 时默认脱敏为 "***"（除非 ?reveal=1）。
type FrpsConfig struct {
	BindPort         int    `json:"bindPort"`
	AuthMethod       string `json:"authMethod,omitempty"`
	AuthToken        string `json:"authToken,omitempty"`
	DashboardEnabled bool   `json:"dashboardEnabled,omitempty"`
	DashboardAddr    string `json:"dashboardAddr,omitempty"`
	DashboardPort    int    `json:"dashboardPort,omitempty"`
	DashboardUser    string `json:"dashboardUser,omitempty"`
	DashboardPass    string `json:"dashboardPass,omitempty"`
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
	go h.maybeApplyConfig("frps")
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
	go h.maybeApplyConfig("frpc")
}
