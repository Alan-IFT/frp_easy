// Package downloader — QA 对抗性边界测试（T-014）。
// 这些测试由 QA Tester 独立编写，针对 02_SOLUTION_DESIGN.md §3.4.5 列出但
// 开发者新增测试未覆盖的降级分支：HTTP 200 但响应体非法 JSON、tag_name 缺失、
// HTTP 500 等其它非 200 状态码。全程 httptest，无真实网络（沿用 C-4 约束）。
package downloader

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// QA-1: API 返回 HTTP 200 但响应体不是合法 JSON → failed + "解析" 消息，不 panic。
// 失败假设：若实现先 json.Unmarshal 再判错，非法 JSON 可能导致后续逻辑用空 ghRelease，
// 误进入"未找到匹配资产"分支而非"解析失败"分支。
func TestAdversarial_ResolveLatest_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"tag_name": "v1.0.0", "assets": [ THIS IS NOT JSON `))
	}))
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForDone(t, m, "frpc", 5*time.Second)
	if st.Status != StatusFailed {
		t.Fatalf("malformed JSON 应进入 failed, got %s", st.Status)
	}
	if !strings.Contains(st.Error, "解析") {
		t.Errorf("期望错误消息含 '解析', got %q", st.Error)
	}
}

// QA-2: API 返回 HTTP 200 + 合法 JSON 但缺 tag_name → failed + "版本号" 消息。
// 失败假设：tag_name 为空时若不早返回，资产匹配可能仍命中并返回空版本号下载。
func TestAdversarial_ResolveLatest_MissingTagName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// 合法 JSON、有匹配后缀的资产，但 tag_name 缺失。
		w.Write([]byte(`{"assets":[{"name":"frp_1.0.0_linux_amd64.tar.gz","browser_download_url":"http://x/a"}]}`))
	}))
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForDone(t, m, "frpc", 5*time.Second)
	if st.Status != StatusFailed {
		t.Fatalf("缺 tag_name 应进入 failed, got %s", st.Status)
	}
	if !strings.Contains(st.Error, "版本号") {
		t.Errorf("期望错误消息含 '版本号', got %q", st.Error)
	}
}

// QA-3: API 返回 HTTP 500（非 200 非 403）→ failed + "HTTP 500" 消息。
// 失败假设：若 switch 只列了 200/403，500 可能落入 default 但消息不含状态码。
func TestAdversarial_ResolveLatest_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"server error"}`))
	}))
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frps"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForDone(t, m, "frps", 5*time.Second)
	if st.Status != StatusFailed {
		t.Fatalf("HTTP 500 应进入 failed, got %s", st.Status)
	}
	if !strings.Contains(st.Error, "500") {
		t.Errorf("期望错误消息含 '500', got %q", st.Error)
	}
}
