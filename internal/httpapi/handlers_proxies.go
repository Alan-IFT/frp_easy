package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/frp-easy/frp-easy/internal/storage"

	"github.com/go-chi/chi/v5"
)

// ProxyInput / ProxyResponse 见 02 §5.2。
type ProxyInput struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	LocalIP       string   `json:"localIP,omitempty"`
	LocalPort     int      `json:"localPort"`
	RemotePort    *int     `json:"remotePort,omitempty"`
	CustomDomains []string `json:"customDomains,omitempty"`
	Enabled       *bool    `json:"enabled,omitempty"`
	Version       int64    `json:"version,omitempty"` // PUT 时必填
}

// ProxyResponse 含 ID + Version + UpdatedAt。
type ProxyResponse struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	LocalIP       string   `json:"localIP"`
	LocalPort     int      `json:"localPort"`
	RemotePort    *int     `json:"remotePort,omitempty"`
	CustomDomains []string `json:"customDomains,omitempty"`
	Enabled       bool     `json:"enabled"`
	Version       int64    `json:"version"`
	UpdatedAt     string   `json:"updatedAt"`
}

func toResponse(p storage.Proxy) ProxyResponse {
	return ProxyResponse{
		ID:            p.ID,
		Name:          p.Name,
		Type:          p.Type,
		LocalIP:       p.LocalIP,
		LocalPort:     p.LocalPort,
		RemotePort:    p.RemotePort,
		CustomDomains: p.CustomDomains,
		Enabled:       p.Enabled,
		Version:       p.Version,
		UpdatedAt:     p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (h *handlers) listProxies(w http.ResponseWriter, r *http.Request) {
	list, err := h.deps.Store.ListProxies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "读取失败", "")
		return
	}
	out := make([]ProxyResponse, 0, len(list))
	for _, p := range list {
		out = append(out, toResponse(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handlers) createProxy(w http.ResponseWriter, r *http.Request) {
	var in ProxyInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}
	p, field, err := buildProxyForInsert(&in)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), field)
		return
	}
	// 条数上限检查（B-20：≤200 条）
	existing, cntErr := h.deps.Store.ListProxies(r.Context())
	if cntErr != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "读取失败", "")
		return
	}
	if len(existing) >= 200 {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "代理规则已达上限（200 条），请删除部分规则后重试", "")
		return
	}
	if err := h.deps.Store.UpsertProxy(r.Context(), p); err != nil {
		mapProxyWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toResponse(*p))
	h.applyConfigBestEffort(r.Context(), "frpc")
}

func (h *handlers) updateProxy(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "id 非法", "id")
		return
	}
	var in ProxyInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}
	if in.Version <= 0 {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "version 必填用于乐观锁", "version")
		return
	}
	p, field, err := buildProxyForUpdate(&in, id)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), field)
		return
	}
	if err := h.deps.Store.UpsertProxy(r.Context(), p); err != nil {
		mapProxyWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(*p))
	h.applyConfigBestEffort(r.Context(), "frpc")
}

func (h *handlers) deleteProxy(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "id 非法", "id")
		return
	}
	if err := h.deps.Store.DeleteProxy(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "未找到该规则", "")
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "删除失败", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	h.applyConfigBestEffort(r.Context(), "frpc")
}

func (h *handlers) applyConfigBestEffort(ctx context.Context, kind string) {
	if err := h.renderAndApply(ctx, kind); err != nil {
		if h.deps.Logger != nil {
			h.deps.Logger.Warn("apply config failed", "kind", kind, "err", err)
		}
	}
}

// maybeApplyConfig 为向后兼容保留（当前使用 applyConfigBestEffort）。
func (h *handlers) maybeApplyConfig(kind string) {
	if h.deps.ProcMgr == nil {
		return
	}
	_ = h.deps.ProcMgr.ApplyConfigChange(kind)
}

// buildProxyForInsert 把 ProxyInput 转 storage.Proxy（新建）。
func buildProxyForInsert(in *ProxyInput) (*storage.Proxy, string, error) {
	field, err := validateProxyInput(in)
	if err != nil {
		return nil, field, err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	localIP := strings.TrimSpace(in.LocalIP)
	if localIP == "" {
		localIP = "127.0.0.1"
	}
	return &storage.Proxy{
		Name:          in.Name,
		Type:          in.Type,
		LocalIP:       localIP,
		LocalPort:     in.LocalPort,
		RemotePort:    in.RemotePort,
		CustomDomains: in.CustomDomains,
		Enabled:       enabled,
	}, "", nil
}

func buildProxyForUpdate(in *ProxyInput, id int64) (*storage.Proxy, string, error) {
	p, field, err := buildProxyForInsert(in)
	if err != nil {
		return nil, field, err
	}
	p.ID = id
	p.Version = in.Version
	return p, "", nil
}

// validateProxyInput 校验业务规则，返回出错字段名（用于 422 响应）。
func validateProxyInput(in *ProxyInput) (string, error) {
	if err := ValidateProxyName(in.Name); err != nil {
		return "name", err
	}
	if err := ValidateProxyType(in.Type); err != nil {
		return "type", err
	}
	if err := ValidatePort(in.LocalPort, "localPort"); err != nil {
		return "localPort", err
	}
	switch in.Type {
	case "tcp", "udp":
		if in.RemotePort == nil {
			return "remotePort", errInline("remotePort 必填")
		}
		if err := ValidatePort(*in.RemotePort, "remotePort"); err != nil {
			return "remotePort", err
		}
		if len(in.CustomDomains) > 0 {
			return "customDomains", errInline("tcp/udp 不接受 customDomains")
		}
	case "http", "https":
		if in.RemotePort != nil {
			return "remotePort", errInline("http/https 不接受 remotePort")
		}
		if len(in.CustomDomains) == 0 {
			return "customDomains", errInline("customDomains 至少 1 项")
		}
		for _, d := range in.CustomDomains {
			if err := ValidateDomain(d); err != nil {
				return "customDomains", err
			}
		}
	}
	return "", nil
}

func errInline(s string) error { return errors.New(s) }

// mapProxyWriteError 把 storage 层错误映射到 HTTP 响应。
func mapProxyWriteError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "未找到该规则", "")
		return
	}
	if errors.Is(err, storage.ErrVersionConflict) {
		writeError(w, http.StatusConflict, CodeConflict, "规则已被其它会话修改，请刷新后重试", "version")
		return
	}
	// SQL UNIQUE / 部分索引等冲突
	msg := err.Error()
	low := strings.ToLower(msg)
	if strings.Contains(low, "unique") || strings.Contains(low, "constraint") {
		field := "name"
		if strings.Contains(low, "remote_port") {
			field = "remotePort"
		}
		writeError(w, http.StatusUnprocessableEntity, CodeConflict, "字段冲突：可能 name 重复或 (type,remotePort) 冲突", field)
		return
	}
	if strings.Contains(low, "requires") || strings.Contains(low, "must not") || strings.Contains(low, "invalid") {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, msg, "")
		return
	}
	writeError(w, http.StatusInternalServerError, CodeInternal, "保存失败: "+msg, "")
}
