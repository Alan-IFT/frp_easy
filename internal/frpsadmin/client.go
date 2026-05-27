// Package frpsadmin 封装 frps 自带 admin HTTP API 的 basic auth 客户端。
//
// 上游 endpoint（与 frp v0.58+ webServer routes 对齐）：
//   - GET /api/serverinfo       — 全局服务状态 + 配置摘要 + 总流量
//   - GET /api/proxy/{type}     — 按类型列出 proxy（tcp/udp/http/https/stcp/sudp/xtcp）
//   - GET /api/proxy/{type}/{name} — 单条 proxy 详情
//   - GET /api/traffic/{name}   — 单条 proxy 日级流量序列
//
// 5s 默认超时（比 frpcadmin 3s 略宽：frps 端可能聚合多 client 数据）。
// basic auth 凭据由调用方（httpapi handler）从 KV 读取后注入。
//
// 错误模型（sentinel，可类型断言）：
//   - ErrUnauthorized — 上游 401（凭据失效）
//   - ErrNotFound     — 上游 404（proxy / type 不存在）
//   - ErrUnavailable  — 连接拒绝 / DNS 失败 / 超时 / 5xx（frps 未跑 / dashboard 未启用）
//
// 与 internal/frpcadmin 对称镜像：同款 New / NewWithTimeout / NewWithBaseURL 构造路径。
package frpsadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrUnauthorized 表示上游 frps dashboard 返回 401（basic auth 不匹配）。
// 调用方用 errors.Is(err, ErrUnauthorized) 判别后做友好 UX 引导。
var ErrUnauthorized = errors.New("frpsadmin: unauthorized")

// ErrNotFound 表示上游 frps dashboard 返回 404（指定 proxy / type 不存在）。
var ErrNotFound = errors.New("frpsadmin: not found")

// ErrUnavailable 表示 frps 进程不可达（连接拒绝 / DNS / 超时 / 5xx 上游错误）。
// 与 ErrUnauthorized 区别：前者是"frps 没跑"，后者是"frps 跑了但凭据不对"。
var ErrUnavailable = errors.New("frpsadmin: upstream unavailable")

// Client 是 frps admin API 客户端。
type Client struct {
	baseURL string
	user    string
	pass    string
	http    *http.Client
}

// defaultTimeout 是 New() / NewWithBaseURL() 的默认超时（FR-1.7）。
const defaultTimeout = 5 * time.Second

// New 用 addr+port 构造 Client。addr 为空时默认 127.0.0.1；port=0 不附端口。
func New(addr string, port int, user, pass string) *Client {
	return NewWithTimeout(addr, port, user, pass, 0)
}

// NewWithTimeout 同 New，但允许自定义超时（≤0 时取 defaultTimeout）。
func NewWithTimeout(addr string, port int, user, pass string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if addr == "" {
		addr = "127.0.0.1"
	}
	base := fmt.Sprintf("http://%s", addr)
	if port > 0 {
		base = fmt.Sprintf("http://%s:%d", addr, port)
	}
	return &Client{
		baseURL: base,
		user:    user,
		pass:    pass,
		http:    &http.Client{Timeout: timeout},
	}
}

// NewWithBaseURL 用完整 URL 构造（含 schema + host + port），主要为测试注入 httptest.Server.URL。
func NewWithBaseURL(baseURL, user, pass string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		user:    user,
		pass:    pass,
		http:    &http.Client{Timeout: timeout},
	}
}

// ServerInfo 是 GET /api/serverinfo 的响应（仅取常用字段，其余 omitempty 透传）。
type ServerInfo struct {
	Version               string         `json:"version,omitempty"`
	BindPort              int            `json:"bindPort,omitempty"`
	KCPBindPort           int            `json:"kcpBindPort,omitempty"`
	QUICBindPort          int            `json:"quicBindPort,omitempty"`
	VhostHTTPPort         int            `json:"vhostHTTPPort,omitempty"`
	VhostHTTPSPort        int            `json:"vhostHTTPSPort,omitempty"`
	TCPMuxHTTPConnectPort int            `json:"tcpmuxHTTPConnectPort,omitempty"`
	SubdomainHost         string         `json:"subdomainHost,omitempty"`
	MaxPoolCount          int64          `json:"maxPoolCount,omitempty"`
	MaxPortsPerClient     int64          `json:"maxPortsPerClient,omitempty"`
	HeartbeatTimeout      int64          `json:"heartbeatTimeout,omitempty"`
	ClientCounts          int            `json:"clientCounts"`
	CurConns              int            `json:"curConns"`
	ProxyTypeCount        map[string]int `json:"proxyTypeCount,omitempty"`
	TotalTrafficIn        int64          `json:"totalTrafficIn,omitempty"`
	TotalTrafficOut       int64          `json:"totalTrafficOut,omitempty"`
}

// ProxyStatus 是 GET /api/proxy/{type} 单条 proxy 状态。
type ProxyStatus struct {
	Name            string         `json:"name"`
	Type            string         `json:"type,omitempty"`
	Status          string         `json:"status,omitempty"`
	LastStartTime   string         `json:"lastStartTime,omitempty"`
	LastCloseTime   string         `json:"lastCloseTime,omitempty"`
	TodayTrafficIn  int64          `json:"todayTrafficIn,omitempty"`
	TodayTrafficOut int64          `json:"todayTrafficOut,omitempty"`
	CurConns        int            `json:"curConns,omitempty"`
	ClientVersion   string         `json:"clientVersion,omitempty"`
	Conf            map[string]any `json:"conf,omitempty"`
}

// ProxyDetail 是 GET /api/proxy/{type}/{name} 响应。
// frps 上游单条 proxy 详情与 list 项形状一致，但单独类型让未来字段扩展更安全。
type ProxyDetail = ProxyStatus

// Traffic 是 GET /api/traffic/{name} 响应。
type Traffic struct {
	Name       string  `json:"name"`
	TrafficIn  []int64 `json:"trafficIn"`
	TrafficOut []int64 `json:"trafficOut"`
}

// proxiesEnvelope 解包 frps /api/proxy/{type} 上游 `{"proxies":[...]}` 形状。
// 调用方拿到的是扁平数组（D-3.5）。
type proxiesEnvelope struct {
	Proxies []ProxyStatus `json:"proxies"`
}

// ServerInfo 调 GET /api/serverinfo。
func (c *Client) ServerInfo(ctx context.Context) (ServerInfo, error) {
	var out ServerInfo
	if err := c.doGet(ctx, "/api/serverinfo", &out); err != nil {
		return ServerInfo{}, err
	}
	return out, nil
}

// Proxies 调 GET /api/proxy/{type}，返回扁平 proxy 数组（envelope unwrap）。
// proxyType ∈ {tcp,udp,http,https,stcp,sudp,xtcp}；无效 type 由上游返 404 → ErrNotFound。
func (c *Client) Proxies(ctx context.Context, proxyType string) ([]ProxyStatus, error) {
	var env proxiesEnvelope
	if err := c.doGet(ctx, "/api/proxy/"+proxyType, &env); err != nil {
		return nil, err
	}
	// 上游可能返回 nil 数组（无 proxy）；统一成空 slice 让 JSON 序列化稳定。
	if env.Proxies == nil {
		return []ProxyStatus{}, nil
	}
	return env.Proxies, nil
}

// ProxyDetail 调 GET /api/proxy/{type}/{name}。
func (c *Client) ProxyDetail(ctx context.Context, proxyType, name string) (ProxyDetail, error) {
	var out ProxyDetail
	if err := c.doGet(ctx, "/api/proxy/"+proxyType+"/"+name, &out); err != nil {
		return ProxyDetail{}, err
	}
	return out, nil
}

// Traffic 调 GET /api/traffic/{name}。
func (c *Client) Traffic(ctx context.Context, name string) (Traffic, error) {
	var out Traffic
	if err := c.doGet(ctx, "/api/traffic/"+name, &out); err != nil {
		return Traffic{}, err
	}
	return out, nil
}

// doGet 是 4 个公开方法的共用路径：构造请求 → basic auth → 执行 → 分类错误 → 反序列化。
// 错误分类（D-3.1 / D-3.2）：
//   - http.Do() 错误 → ErrUnavailable 包装（连接拒绝 / DNS / 超时同源语义）
//   - status 401 → ErrUnauthorized
//   - status 404 → ErrNotFound
//   - status ≥ 500 → ErrUnavailable 包装（5xx 与"上游不可用"同语义）
//   - 其它非 2xx → 普通 fmt.Errorf（带 body 前 4 KiB 上下文）
func (c *Client) doGet(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("frpsadmin: build request: %w", err)
	}
	if c.user != "" || c.pass != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
		// happy path
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("frpsadmin: unexpected status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("frpsadmin: read body: %w", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("frpsadmin: decode: %w", err)
	}
	return nil
}
