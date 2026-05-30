package frpsadmin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServerInfo_Success(t *testing.T) {
	payload := ServerInfo{
		Version:         "0.58.1",
		BindPort:        7000,
		ClientCounts:    3,
		CurConns:        12,
		TotalTrafficIn:  102400,
		TotalTrafficOut: 51200,
		ProxyTypeCount:  map[string]int{"tcp": 2, "http": 1},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/serverinfo" {
			t.Errorf("path = %s", r.URL.Path)
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != "admin" || p != "pw" {
			t.Errorf("auth: %s/%s ok=%v", u, p, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := NewWithBaseURL(srv.URL, "admin", "pw", 2*time.Second)
	got, err := c.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if got.Version != "0.58.1" || got.BindPort != 7000 || got.ClientCounts != 3 {
		t.Errorf("got = %+v", got)
	}
	if got.ProxyTypeCount["tcp"] != 2 {
		t.Errorf("proxyTypeCount: %+v", got.ProxyTypeCount)
	}
}

func TestServerInfo_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	_, err := c.ServerInfo(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestServerInfo_Unavailable_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	_, err := c.ServerInfo(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("expected ErrUnavailable for 5xx, got %v", err)
	}
}

func TestServerInfo_Unavailable_ConnRefused(t *testing.T) {
	// 启 server 后立即关，让端口悬空（拿不到 listen 但可拿到 URL）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := NewWithBaseURL(url, "u", "p", time.Second)
	_, err := c.ServerInfo(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("expected ErrUnavailable for conn refused, got %v", err)
	}
}

func TestProxies_Envelope_Unwrap(t *testing.T) {
	// frps 上游 /api/proxy/{type} 返回 {"proxies":[...]} 包装
	payload := map[string]any{
		"proxies": []ProxyStatus{
			{Name: "ssh", Type: "tcp", Status: "online", TodayTrafficIn: 1024, CurConns: 1},
			{Name: "rdp", Type: "tcp", Status: "offline"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/proxy/tcp" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	got, err := c.Proxies(context.Background(), "tcp")
	if err != nil {
		t.Fatalf("Proxies: %v", err)
	}
	if len(got) != 2 || got[0].Name != "ssh" || got[1].Name != "rdp" {
		t.Errorf("unwrap failed: %+v", got)
	}
	if got[0].TodayTrafficIn != 1024 {
		t.Errorf("trafficIn: %v", got[0].TodayTrafficIn)
	}
}

func TestProxies_EmptyArray(t *testing.T) {
	// 上游可能返回 {"proxies":null} 或缺字段；客户端应输出 []ProxyStatus{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	got, err := c.Proxies(context.Background(), "stcp")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Errorf("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected len 0, got %d", len(got))
	}
}

func TestProxies_BadType_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	_, err := c.Proxies(context.Background(), "nonsense-type")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestProxyDetail_Success(t *testing.T) {
	payload := ProxyDetail{
		Name: "ssh", Type: "tcp", Status: "online",
		LastStartTime: "2026-05-27 10:00:00", ClientVersion: "0.58.1",
		Conf: map[string]any{"remote_port": 6022.0},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/proxy/tcp/ssh" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	got, err := c.ProxyDetail(context.Background(), "tcp", "ssh")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "ssh" || got.Status != "online" {
		t.Errorf("got = %+v", got)
	}
}

func TestProxyDetail_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	_, err := c.ProxyDetail(context.Background(), "tcp", "ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTraffic_Success(t *testing.T) {
	payload := Traffic{
		Name:       "ssh",
		TrafficIn:  []int64{0, 0, 1024, 2048, 0, 0, 512},
		TrafficOut: []int64{0, 0, 512, 1024, 0, 0, 256},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/traffic/ssh" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	got, err := c.Traffic(context.Background(), "ssh")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "ssh" || len(got.TrafficIn) != 7 || got.TrafficIn[2] != 1024 {
		t.Errorf("got = %+v", got)
	}
}

func TestTraffic_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	_, err := c.Traffic(context.Background(), "ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBasicAuth_Applied(t *testing.T) {
	var seenUser, seenPass string
	var seenOk bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUser, seenPass, seenOk = r.BasicAuth()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "myuser", "mypass", time.Second)
	_, _ = c.ServerInfo(context.Background())
	if !seenOk || seenUser != "myuser" || seenPass != "mypass" {
		t.Errorf("auth not applied: u=%q p=%q ok=%v", seenUser, seenPass, seenOk)
	}
}

func TestNew_BuildsBaseURL(t *testing.T) {
	c := New("127.0.0.1", 7500, "admin", "pw")
	if c.baseURL != "http://127.0.0.1:7500" {
		t.Errorf("baseURL = %s", c.baseURL)
	}
}

func TestNew_DefaultAddr(t *testing.T) {
	c := New("", 7500, "u", "p")
	if !strings.Contains(c.baseURL, "127.0.0.1") {
		t.Errorf("default addr not applied: %s", c.baseURL)
	}
}

func TestNew_NoPort(t *testing.T) {
	c := New("example.com", 0, "u", "p")
	if c.baseURL != "http://example.com" {
		t.Errorf("baseURL = %s", c.baseURL)
	}
}

func TestNewWithTimeout_DefaultsOnZero(t *testing.T) {
	c := NewWithTimeout("127.0.0.1", 7500, "u", "p", 0)
	if c.http.Timeout != defaultTimeout {
		t.Errorf("timeout = %v, want %v", c.http.Timeout, defaultTimeout)
	}
}

// --- T-055 A-2/A-3：path segment escape 防注入 ---

// TestProxyDetail_PathEscape 验证 type/name 含特殊字符时上游收到的 path 被正确编码，
// 不改变 path/query 语义边界（A-2）。同时验证普通无特殊字符的 path 无回归（A-3）。
func TestProxyDetail_PathEscape(t *testing.T) {
	cases := []struct {
		name       string
		proxyType  string
		proxyName  string
		wantEscape string // 期望上游收到的 r.URL.EscapedPath()
	}{
		{"normal_no_regression", "tcp", "ssh", "/api/proxy/tcp/ssh"},
		{"name_with_slash", "tcp", "a/b", "/api/proxy/tcp/a%2Fb"},
		{"name_with_query", "tcp", "x?y", "/api/proxy/tcp/x%3Fy"},
		{"name_with_fragment", "tcp", "x#y", "/api/proxy/tcp/x%23y"},
		{"name_with_space", "tcp", "x y", "/api/proxy/tcp/x%20y"},
		{"name_with_percent", "tcp", "a%2f", "/api/proxy/tcp/a%252f"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var seenURI string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 用 r.RequestURI（请求行原文，逐字节保留）断言编码后的 path，
				// 比 r.URL.EscapedPath()（可能因 RawPath/Path 规范化而重算）更可靠。
				seenURI = r.RequestURI
				_, _ = w.Write([]byte(`{"name":"x"}`))
			}))
			defer srv.Close()
			client := NewWithBaseURL(srv.URL, "u", "p", time.Second)
			_, err := client.ProxyDetail(context.Background(), c.proxyType, c.proxyName)
			if err != nil {
				t.Fatalf("ProxyDetail: %v", err)
			}
			if seenURI != c.wantEscape {
				t.Errorf("upstream RequestURI = %q, want %q", seenURI, c.wantEscape)
			}
		})
	}
}

// TestTraffic_PathEscape 验证 Traffic 的 name segment 同样被 escape（A-2）。
func TestTraffic_PathEscape(t *testing.T) {
	var seenURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenURI = r.RequestURI
		_, _ = w.Write([]byte(`{"name":"x","trafficIn":[],"trafficOut":[]}`))
	}))
	defer srv.Close()
	client := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	if _, err := client.Traffic(context.Background(), "x/../etc?a=1"); err != nil {
		t.Fatalf("Traffic: %v", err)
	}
	want := "/api/traffic/x%2F..%2Fetc%3Fa=1"
	if seenURI != want {
		t.Errorf("upstream RequestURI = %q, want %q", seenURI, want)
	}
}

// TestProxies_PathEscape 验证 Proxies 的 type segment 同样被 escape（A-2）。
func TestProxies_PathEscape(t *testing.T) {
	var seenURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenURI = r.RequestURI
		_, _ = w.Write([]byte(`{"proxies":[]}`))
	}))
	defer srv.Close()
	client := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	if _, err := client.Proxies(context.Background(), "tcp/../admin"); err != nil {
		t.Fatalf("Proxies: %v", err)
	}
	want := "/api/proxy/tcp%2F..%2Fadmin"
	if seenURI != want {
		t.Errorf("upstream RequestURI = %q, want %q", seenURI, want)
	}
}

func TestDoGet_UnexpectedStatus_400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad query"))
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	_, err := c.ServerInfo(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	// 400 不在 sentinel 分类内（不是 401/404/5xx）—— 应是普通 fmt.Errorf
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnavailable) {
		t.Errorf("400 should not match any sentinel, got %v", err)
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "bad query") {
		t.Errorf("err lacks detail: %v", err)
	}
}
