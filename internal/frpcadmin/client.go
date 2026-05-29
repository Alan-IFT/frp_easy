// Package frpcadmin 封装 frpc 自带 admin API 的 HTTP basic auth 客户端。
//
// 上游 endpoint（已在 02 §3.6 / §附录 A 经 context7 校验）：
//   - GET /api/reload[?strictConfig=true] — 200 表示重载成功（body 通常为空 JSON）。
//   - GET /api/status                     — 200 + JSON，按 type 分组的 proxy 状态。
//
// 短超时（3s）由 Client 持有的 http.Client 控制，避免阻塞 HTTP handler。
package frpcadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 是 frpc admin API 客户端。
//
// baseURL 形如 "http://127.0.0.1:7400"。user / pass 由 UI 服务启动时生成
// 并持久化（02 §3.4 + §7 Q-B）；同一对凭据同时写入 frpc.toml 与本客户端。
type Client struct {
	baseURL string
	user    string
	pass    string
	http    *http.Client
}

// New 建一个 Client。timeout 建议 3 秒；传 0 时取 3s 默认。
func New(addr string, port int, user, pass string) *Client {
	return NewWithTimeout(addr, port, user, pass, 0)
}

// NewWithTimeout 同上但允许指定 timeout。
func NewWithTimeout(addr string, port int, user, pass string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
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

// NewWithBaseURL 用完整 URL（含 schema + host + port）构建 —— 主要用于测试
// 注入 httptest.Server.URL。
func NewWithBaseURL(baseURL, user, pass string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		user:    user,
		pass:    pass,
		http:    &http.Client{Timeout: timeout},
	}
}

// Reload 调 GET /api/reload。
// strict=true 时附 ?strictConfig=true（02 §3.6）。
func (c *Client) Reload(ctx context.Context, strict bool) error {
	u := c.baseURL + "/api/reload"
	if strict {
		v := url.Values{}
		v.Set("strictConfig", "true")
		u += "?" + v.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("frpcadmin.Reload build: %w", err)
	}
	c.applyAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("frpcadmin.Reload http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("frpcadmin.Reload status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// 200 时 body 可忽略。
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// ProxyStatus 是 /api/status 返回的单条 proxy 状态摘要。
// 字段命名按 frpc 上游返回（snake_case）反序列化。
type ProxyStatus struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Err        string `json:"err"`
	LocalAddr  string `json:"local_addr"`
	Plugin     string `json:"plugin"`
	RemoteAddr string `json:"remote_addr"`
}

// Status 调 GET /api/status。
// 返回按 type 分组的 map：key = "tcp" / "udp" / "http" / "https"；value = 该类
// 下所有 proxy 状态数组。frpc 上游返回结构本身就是分组形式。
func (c *Client) Status(ctx context.Context) (map[string][]ProxyStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/status", nil)
	if err != nil {
		return nil, fmt.Errorf("frpcadmin.Status build: %w", err)
	}
	c.applyAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("frpcadmin.Status http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("frpcadmin.Status status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("frpcadmin.Status read: %w", err)
	}
	out := map[string][]ProxyStatus{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("frpcadmin.Status decode: %w", err)
	}
	return out, nil
}

func (c *Client) applyAuth(req *http.Request) {
	if c.user != "" || c.pass != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
}

// ErrUnauthorized 给上层一个可类型断言的错误码。当前 Reload/Status 仅返回
// 通用错误带 status 文本；如调用方需要精确判别，可后续扩展。
var ErrUnauthorized = errors.New("frpcadmin: unauthorized")
