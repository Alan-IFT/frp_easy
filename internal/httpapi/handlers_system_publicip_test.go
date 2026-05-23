package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// makeJSONSource 返回一个简单 ipSource，给 httptest 配 JSON {"ip":"..."} 响应。
func makeJSONSource(name string, srv *httptest.Server, maxBody int64) ipSource {
	return ipSource{
		name:    name,
		url:     srv.URL,
		parser:  parseIPFromIPField,
		maxBody: maxBody,
	}
}

// TestFetchPublicIP_EnvOverride 验证 AC-B.4：FRP_EASY_PUBLIC_IP 命中时 short-circuit。
// 关键：不应发任何 HTTP（用计数器断言）。
func TestFetchPublicIP_EnvOverride(t *testing.T) {
	t.Setenv("FRP_EASY_PUBLIC_IP", "10.0.0.5")

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`{"ip":"8.8.8.8"}`))
	}))
	defer srv.Close()

	sources := []ipSource{makeJSONSource("test-src", srv, 32<<10)}
	res := fetchPublicIP(context.Background(), sources)

	if res.IP != "10.0.0.5" {
		t.Errorf("IP = %q, want 10.0.0.5", res.IP)
	}
	if res.Source != "env" {
		t.Errorf("Source = %q, want env", res.Source)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("HTTP calls = %d, want 0 (env should short-circuit)", got)
	}
}

// TestFetchPublicIP_FirstWins 起 2 个 source，慢的 200ms 后才响应，快的立即响应。
// 断言：胜出者是快的，并且总耗时 < 200ms。
func TestFetchPublicIP_FirstWins(t *testing.T) {
	t.Setenv("FRP_EASY_PUBLIC_IP", "") // 防止外部 env 干扰

	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":"1.2.3.4"}`))
	}))
	defer fast.Close()

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(500 * time.Millisecond):
		}
		w.Write([]byte(`{"ip":"9.9.9.9"}`))
	}))
	defer slow.Close()

	sources := []ipSource{
		makeJSONSource("slow", slow, 32<<10),
		makeJSONSource("fast", fast, 32<<10),
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := fetchPublicIP(ctx, sources)
	elapsed := time.Since(start)

	if res.IP != "1.2.3.4" {
		t.Errorf("IP = %q, want 1.2.3.4 (fast source)", res.IP)
	}
	if res.Source != "fast" {
		t.Errorf("Source = %q, want fast", res.Source)
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("elapsed = %v, want < 300ms (cancel should fire)", elapsed)
	}
}

// TestFetchPublicIP_AllFail 全部源 502 → ErrMsg "检测超时，请手动查询"。
func TestFetchPublicIP_AllFail(t *testing.T) {
	t.Setenv("FRP_EASY_PUBLIC_IP", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	sources := []ipSource{
		makeJSONSource("a", srv, 32<<10),
		makeJSONSource("b", srv, 32<<10),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := fetchPublicIP(ctx, sources)

	if res.IP != "" {
		t.Errorf("IP = %q, want empty", res.IP)
	}
	if res.ErrMsg == "" {
		t.Error("ErrMsg should be set")
	}
}

// TestFetchPublicIP_NonIPText 单源返回 "not-an-ip" → 跳过，取其它源。
func TestFetchPublicIP_NonIPText(t *testing.T) {
	t.Setenv("FRP_EASY_PUBLIC_IP", "")

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":"not-an-ip"}`))
	}))
	defer bad.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":"4.4.4.4"}`))
	}))
	defer good.Close()

	sources := []ipSource{
		makeJSONSource("bad", bad, 32<<10),
		makeJSONSource("good", good, 32<<10),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := fetchPublicIP(ctx, sources)

	if res.IP != "4.4.4.4" {
		t.Errorf("IP = %q, want 4.4.4.4", res.IP)
	}
	if res.Source != "good" {
		t.Errorf("Source = %q, want good", res.Source)
	}
}

// TestFetchPublicIP_UserAgent 验证 AC-B.8：所有出站请求带 UA=frp_easy。
func TestFetchPublicIP_UserAgent(t *testing.T) {
	t.Setenv("FRP_EASY_PUBLIC_IP", "")

	var gotUA atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA.Store(r.Header.Get("User-Agent"))
		w.Write([]byte(`{"ip":"1.1.1.1"}`))
	}))
	defer srv.Close()

	sources := []ipSource{makeJSONSource("x", srv, 32<<10)}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = fetchPublicIP(ctx, sources)

	if v, ok := gotUA.Load().(string); !ok || v != "frp_easy" {
		t.Errorf("User-Agent = %v, want frp_easy", gotUA.Load())
	}
}

// TestFetchPublicIP_HTMLPolluted 验证 R-9 / AC-B.7：1 MiB HTML 中先有 192.168.1.1
// （私有段，应跳过）、后含 8.8.8.8（公网，应取）。
// 用 256 KiB maxBody 测试 LimitReader 是否截断在合理位置。
func TestFetchPublicIP_HTMLPolluted(t *testing.T) {
	t.Setenv("FRP_EASY_PUBLIC_IP", "")

	// 构造：header 含 192.168.1.1，紧随其后含 8.8.8.8，再加 800 KiB 填充。
	body := []byte("<html>private=192.168.1.1 public=8.8.8.8\n")
	pad := strings.Repeat("x", 800<<10)
	body = append(body, []byte(pad)...)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(body)
	}))
	defer srv.Close()

	sources := []ipSource{
		{name: "html", url: srv.URL, parser: parseFirstIPv4FromHTML, maxBody: 256 << 10},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := fetchPublicIP(ctx, sources)

	if res.IP != "8.8.8.8" {
		t.Errorf("IP = %q, want 8.8.8.8 (private 192.168.x.x should be skipped)", res.IP)
	}
	if res.Source != "html" {
		t.Errorf("Source = %q, want html", res.Source)
	}
}

// TestFetchPublicIP_IPv6 ipv6 应该带 advisory 提示。
func TestFetchPublicIP_IPv6(t *testing.T) {
	t.Setenv("FRP_EASY_PUBLIC_IP", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":"2001:db8::1"}`))
	}))
	defer srv.Close()

	sources := []ipSource{makeJSONSource("v6", srv, 32<<10)}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := fetchPublicIP(ctx, sources)

	if res.IP != "2001:db8::1" {
		t.Errorf("IP = %q, want 2001:db8::1", res.IP)
	}
	if !strings.Contains(res.Advisory, "方括号") {
		t.Errorf("Advisory should mention 方括号, got %q", res.Advisory)
	}
}

// TestFetchPublicIP_EmptySources 边界：sources 切片为空 → 直接返回 ErrMsg。
func TestFetchPublicIP_EmptySources(t *testing.T) {
	t.Setenv("FRP_EASY_PUBLIC_IP", "")
	res := fetchPublicIP(context.Background(), nil)
	if res.ErrMsg == "" {
		t.Error("expected ErrMsg on empty sources")
	}
}

// TestParseIPCnJSON_BothShapes 验证 ip.cn 两种响应格式都能解析。
func TestParseIPCnJSON_BothShapes(t *testing.T) {
	// 顶层 ip
	ip, err := parseIPCnJSON([]byte(`{"ip":"1.2.3.4"}`))
	if err != nil || ip != "1.2.3.4" {
		t.Errorf("top-level: ip=%q err=%v", ip, err)
	}
	// 嵌套 data.ip
	ip, err = parseIPCnJSON([]byte(`{"code":0,"data":{"ip":"5.6.7.8"}}`))
	if err != nil || ip != "5.6.7.8" {
		t.Errorf("nested: ip=%q err=%v", ip, err)
	}
	// 空字段
	_, err = parseIPCnJSON([]byte(`{}`))
	if err == nil {
		t.Error("expected error on empty json")
	}
}

// TestParseBilibiliJSON 验证 bilibili 响应格式。
func TestParseBilibiliJSON(t *testing.T) {
	ip, err := parseBilibiliJSON([]byte(`{"code":0,"data":{"addr":"9.9.9.9"}}`))
	if err != nil || ip != "9.9.9.9" {
		t.Errorf("ip=%q err=%v", ip, err)
	}
	_, err = parseBilibiliJSON([]byte(`{"data":{}}`))
	if err == nil {
		t.Error("expected error on missing addr")
	}
}
