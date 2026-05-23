package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/frp-easy/frp-easy/internal/portrange"
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

// --- T-018 §C.1 批量创建代理规则 ---

// BatchProxiesRequest 是 POST /api/v1/proxies/batch 的请求体（02 §C.1.1）。
//
// 设计：localPort 与 remotePort 必须 1:1 映射（FR-C.1.2 / R-6），所以仅一个 portsExpr。
// 用户想要 "本地 6000 → 远程 7000" 这类偏移映射时应用单条 POST /api/v1/proxies。
type BatchProxiesRequest struct {
	Basename  string `json:"basename"`
	Type      string `json:"type"` // tcp | udp
	LocalIP   string `json:"localIP,omitempty"`
	PortsExpr string `json:"portsExpr"`
	Enabled   *bool  `json:"enabled,omitempty"` // 缺省 true
}

// BatchProxiesResponse 是 POST /api/v1/proxies/batch 的成功响应（201）。
type BatchProxiesResponse struct {
	Created int             `json:"created"`
	Items   []ProxyResponse `json:"items"`
}

// BatchConflict 是单条冲突明细（响应 ErrorBody 内扩展 conflicts 字段）。
type BatchConflict struct {
	Port   int    `json:"port"`
	Reason string `json:"reason"`
}

// BatchProxiesMaxCount 是单次批量上限（PM-DECIDED 32，FR-C.1.3）。
const BatchProxiesMaxCount = 32

// batchBasenameRE 校验 basename：留 6 字符给 "-65535" 后缀（FR-C.1.4）。
var batchBasenameRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,58}$`)

// batchProxies handles POST /api/v1/proxies/batch（T-018 §C.1）。
//
// 实现：
//  1. 解析请求体；
//  2. basename / type 校验；
//  3. portrange.Parse 展开（上限 32）；
//  4. 总条数 + N ≤ 200 校验；
//  5. 构造 storage.Proxy 数组，name = `<basename>-<port>`；
//  6. UpsertProxiesTx 单事务插入；任一冲突 → 全部回滚 + 4xx + 明细；
//  7. 成功 → applyConfigBestEffort("frpc") 一次（FR-C.1.8），返回 201。
func (h *handlers) batchProxies(w http.ResponseWriter, r *http.Request) {
	var req BatchProxiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}

	// 1. basename / type 校验
	if !batchBasenameRE.MatchString(req.Basename) {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed,
			"basename 非法（^[A-Za-z0-9_-]{1,58}$；需为派生 name 留 6 字符余量）", "basename")
		return
	}
	if req.Type != "tcp" && req.Type != "udp" {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed,
			"batch 仅支持 tcp/udp（http/https 走域名，无端口批量意义）", "type")
		return
	}

	// 2. 端口表达式解析（上限 32）
	ports, err := portrange.Parse(req.PortsExpr, BatchProxiesMaxCount)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed,
			humanizePortRangeErr(err), "portsExpr")
		return
	}

	// 3. 总数上限（叠加）
	existing, lerr := h.deps.Store.ListProxies(r.Context())
	if lerr != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "读取现有规则失败", "")
		return
	}
	if len(existing)+len(ports) > 200 {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed,
			"代理规则已达上限（200 条），请删除部分规则后重试", "")
		return
	}

	// 4. 构造 storage.Proxy 数组
	localIP := strings.TrimSpace(req.LocalIP)
	if localIP == "" {
		localIP = "127.0.0.1"
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	ps := make([]*storage.Proxy, 0, len(ports))
	for _, p := range ports {
		rp := p
		ps = append(ps, &storage.Proxy{
			Name:       fmt.Sprintf("%s-%d", req.Basename, p),
			Type:       req.Type,
			LocalIP:    localIP,
			LocalPort:  p,
			RemotePort: &rp,
			Enabled:    enabled,
		})
	}

	// 5. 单事务插入
	_, err = h.deps.Store.UpsertProxiesTx(r.Context(), ps)
	if err != nil {
		writeBatchProxiesError(w, err)
		return
	}

	// 6. 单次 reload frpc（FR-C.1.8，避免 32 次连续 reload）
	h.applyConfigBestEffort(r.Context(), "frpc")

	// 7. 响应
	items := make([]ProxyResponse, len(ps))
	for i, p := range ps {
		items[i] = toResponse(*p)
	}
	writeJSON(w, http.StatusCreated, BatchProxiesResponse{
		Created: len(items),
		Items:   items,
	})
}

// humanizePortRangeErr 把 portrange 错误转成面向用户的中文消息。
func humanizePortRangeErr(err error) string {
	if err == nil {
		return ""
	}
	var dup *portrange.DuplicateError
	if errors.As(err, &dup) {
		return fmt.Sprintf("端口表达式含重复项：%d", dup.Port)
	}
	var many *portrange.TooManyError
	if errors.As(err, &many) {
		return fmt.Sprintf("单次端口数超过 %d 上限（当前 %d）", many.Max, many.Count)
	}
	var bad *portrange.BadSyntaxError
	if errors.As(err, &bad) {
		if bad.Token != "" {
			return fmt.Sprintf("端口表达式语法错误（token=%q）", bad.Token)
		}
		return "端口表达式语法错误"
	}
	switch {
	case errors.Is(err, portrange.ErrEmpty):
		return "端口表达式必填"
	case errors.Is(err, portrange.ErrPortOutOfRange):
		return "端口必须在 1-65535 之间"
	case errors.Is(err, portrange.ErrRangeReversed):
		return "端口范围左端必须 ≤ 右端"
	case errors.Is(err, portrange.ErrBadSyntax):
		return "端口表达式语法错误"
	}
	return err.Error()
}

// writeBatchProxiesError 把 UpsertProxiesTx 的错误映射成 4xx。
func writeBatchProxiesError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrDuplicateName) {
		writeError(w, http.StatusConflict, CodeConflict,
			"批量创建失败：派生名称与已有规则冲突", "name")
		return
	}
	if errors.Is(err, storage.ErrDuplicateTcpRemote) {
		writeError(w, http.StatusConflict, CodeConflict,
			"批量创建失败：(type, remote_port) 已存在", "remotePort")
		return
	}
	writeError(w, http.StatusInternalServerError, CodeInternal, "保存失败: "+err.Error(), "")
}

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
	// 【T-007 AC-6】name 列 UNIQUE 冲突走 sentinel：409 + 友好中文 + field=name。
	// 必须放在下面 strings.Contains(low, "unique") 之前，确保 name 冲突走
	// 专用语义化 409 路径；(type,remotePort) 组合冲突仍走原 422 兜底分支。
	if errors.Is(err, storage.ErrDuplicateName) {
		writeError(w, http.StatusConflict, CodeConflict, "代理名称已存在，请改用其它名称", "name")
		return
	}
	// SQL UNIQUE / 部分索引等冲突（剩余只可能是 (type,remote_port) 组合冲突）
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
