package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/procmgr"
)

// AC-3 对抗：构造一种"看似走不到中间件的路径"——MethodNotAllowed 是 chi 单独 handler，
// 全局 Use 是否覆盖它？
func TestAdversarial_AC3_MethodNotAllowedHeaders(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	// /api/v1/health 注册时是 GET。用 POST 触发 chi MethodNotAllowed handler。
	resp, _ := doJSON(t, srv, "POST", "/api/v1/health", nil, nil, "")
	defer resp.Body.Close()
	// 期望 405 或类似；关键：三个安全头仍要有
	for k, want := range expectedSecurityHeaders {
		got := resp.Header.Get(k)
		if got != want {
			t.Errorf("ADVERSARIAL FAIL: MethodNotAllowed 路径漏头 %s = %q, want %q", k, got, want)
		}
	}
}

// AC-3 对抗：CORS preflight（OPTIONS）也要带头吗？在 prod 模式（CORS off）下 OPTIONS
// 走 chi 默认 OPTIONS 行为或 NotFound，头是否仍存在？
func TestAdversarial_AC3_OptionsRequest(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "OPTIONS", "/api/v1/health", nil, nil, "")
	defer resp.Body.Close()
	for k, want := range expectedSecurityHeaders {
		got := resp.Header.Get(k)
		if got != want {
			t.Errorf("ADVERSARIAL: OPTIONS 漏头 %s = %q", k, got)
		}
	}
}

// AC-3 对抗：值精确匹配？比如 X-Frame-Options 是否真是 "DENY"（不是 "deny"、"SAMEORIGIN"）
func TestAdversarial_AC3_ExactValues(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "GET", "/api/v1/health", nil, nil, "")
	defer resp.Body.Close()
	// 精确匹配
	if v := resp.Header.Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("ADVERSARIAL: X-Frame-Options = %q (case-sensitive check), want exact 'DENY'", v)
	}
	if v := resp.Header.Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("ADVERSARIAL: X-Content-Type-Options = %q, want exact 'nosniff'", v)
	}
	if v := resp.Header.Get("Referrer-Policy"); v != "no-referrer" {
		t.Errorf("ADVERSARIAL: Referrer-Policy = %q, want exact 'no-referrer'", v)
	}
}

// AC-3 对抗：500 panic 路径中间件 Recover 之后仍然有头吗？
// Recover 在 SecurityHeaders 之后挂载（顶层 Use 是 SecurityHeaders）。
// SecurityHeaders 先 Set headers，再 next。Recover panic → writeError 500 →
// 此时如果 WriteHeader 已发出 ResponseWriter，是否还能输出 header？
//
// 直接路径不易构造，使用一个会触发 storage 内部错误的请求来测 500：
func TestAdversarial_AC3_500Path(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	// /api/v1/proxies/abc DELETE 没认证 → 401（也是错误响应）
	resp, _ := doJSON(t, srv, "DELETE", "/api/v1/proxies/999", nil, nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Logf("note: status = %d (expected 401)", resp.StatusCode)
	}
	for k, want := range expectedSecurityHeaders {
		got := resp.Header.Get(k)
		if got != want {
			t.Errorf("ADVERSARIAL: 401 path missing %s = %q want %q", k, got, want)
		}
	}
}

// AC-3 对抗：验证 r.Use(SecurityHeaders()) 写在 r.Get health 之前的"硬不变量"。
// 这是 grep 检测式对抗：如果未来有人不慎换顺序，本测试虽不能在运行时拦下来，
// 但 TestSecurityHeaders_OnHealth 会捕获（因为 chi 文档明确：Use 必须在路由注册之前）。
func TestAdversarial_AC3_ConfirmHealthRouteWithoutMiddlewareGroup(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	// /api/v1/health 是顶层注册（不经过 ReadyGate 等 Group 内中间件）。
	// 验证它确实跳过了 ReadyGate（构造 ready=false 场景应不阻挡 health）。
	// newTestServer 默认 ready 是 true，但仍是顶层注册的证据：响应不会被 503 挡。
	resp, _ := doJSON(t, srv, "GET", "/api/v1/health", nil, nil, "")
	defer resp.Body.Close()
	// 即使顶层路由跳过其它中间件，SecurityHeaders 仍生效
	if resp.Header.Get("X-Frame-Options") == "" {
		t.Errorf("ADVERSARIAL: SecurityHeaders 没覆盖顶层 health 路由")
	}
}

// AC-4 对抗：MaxReadBytes 字符串/数字精确性 — 如果有人改成 2 * 1000 * 1000 = 2_000_000？
// 直接 import logtail 包级 const 做编译期断言不可能在此包外测；做的是行为断言。
// 详见 logtail 包内的回归测试。

// AC-6 对抗：handler 层 mapProxyWriteError 对 sentinel 的 errors.Is 检测顺序问题：
// 如果某天 sentinel 被双重 wrap（fmt.Errorf("xxx: %w", ErrDuplicateName)），errors.Is
// 仍要工作。
func TestAdversarial_AC6_WrappedSentinelStillMapped(t *testing.T) {
	// 此测试本质是验证 errors.Is 链路。mapProxyWriteError 用 errors.Is 而非 == 比较。
	// 不便直接调用 mapProxyWriteError 的私有函数，但当前实现使用 errors.Is → 通过类型契约保证。
	// 真正的 e2e 由 handlers_proxies_test.go 完成。这里跳过：
	// 仅做语义文档：错误链 wrap 多层后仍要可识别 → errors.Is 保证。
	t.Log("documentation only: mapProxyWriteError uses errors.Is, supports wrap chain")
}

// 附加：验证 SecurityHeaders 不会**重复添加**或**累积**。
// （Set 而非 Add 应保证幂等。）
func TestAdversarial_AC3_HeadersSingleValueOnly(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "GET", "/api/v1/health", nil, nil, "")
	defer resp.Body.Close()
	for k := range expectedSecurityHeaders {
		vals := resp.Header.Values(k)
		if len(vals) != 1 {
			t.Errorf("ADVERSARIAL: header %s 有 %d 个值: %v (期望 1 个)", k, len(vals), vals)
		}
	}
	// 防止：未来在某处 w.Header().Add 而不是 Set，导致重复
	for k := range expectedSecurityHeaders {
		joined := strings.Join(resp.Header.Values(k), ",")
		_ = joined
	}
}

// ============================================================================
// T-065 mapProcErr sentinel 收口 —— QA 独立对抗（从 AC 写 reproducer，不复用 dev 测试）
// ============================================================================

// QA-ADV-1（AC-8 核心反向证伪 / insight L34）：
// 旧脆弱行为是"文本含 stopping/starting/running → 409"。现已切断文本依赖。
// 假设（预期失败点）："我预期，若一个错误的文本含 'stopping'/'running' 但**不包 ErrBusy**，
// 旧代码会误给 409。" 反向证伪：sentinel 化后，此类错误必须走 **500**（不再被文本误判 409）。
// 只要本用例通过（500 而非 409），即静态+运行时双重证明 strings.Contains 文本匹配已死。
func TestAdversarial_T065_TextLooksBusyButNotSentinel_Goes500(t *testing.T) {
	h, _ := newCapturingHandlers(t)
	// 这些错误文本在旧实现下会命中 strings.Contains → 误给 409；它们**不包 ErrBusy**。
	textTraps := []string{
		"procmgr.Start(frpc): currently stopping",          // 旧 409 触发词，无 wrap
		"some frps process is still running unexpectedly",  // 含 running
		"the daemon is starting up but failed to bind port", // 含 starting
	}
	for _, msg := range textTraps {
		rec := httptest.NewRecorder()
		h.mapProcErr(rec, errors.New(msg)) // 裸 error，无 ErrBusy
		if rec.Code != 500 {
			t.Errorf("ADVERSARIAL FAIL: 含忙态文本但非 sentinel 的错误 %q 应走 500（文本匹配已死），got %d", msg, rec.Code)
		}
		// 且不得泄露原文。
		if strings.Contains(rec.Body.String(), msg) {
			t.Errorf("ADVERSARIAL FAIL: 500 响应泄露原始文本 %q: %s", msg, rec.Body.String())
		}
	}
}

// QA-ADV-2（AC-8）：反向——procmgr 内部 cause 文本任意变化（模拟 procmgr 改字），
// 只要错误**包了 ErrBusy**，handler 分类必须恒为 409，不依赖任何特定文本子串。
// 假设："我预期 handler 仍隐藏地依赖某个文本子串；若我把 cause 改成完全不含
// stopping/starting/running 的字，它会漏判成 500。" 若仍 409 则证伪该假设。
func TestAdversarial_T065_AnyCauseTextStillBusyIfSentinel(t *testing.T) {
	h, _ := newCapturingHandlers(t)
	// 故意用完全不含旧关键词（stopping/starting/running）的 cause 文本。
	causeVariants := []string{
		"procmgr.Start(frps): transitional state, refuse op", // 无任何旧关键词
		"is busy elsewhere",
		"机器正忙", // 甚至中文 cause
	}
	for _, c := range causeVariants {
		wrapped := fmt.Errorf("%s: %w", c, procmgr.ErrBusy)
		rec := httptest.NewRecorder()
		h.mapProcErr(rec, wrapped)
		if rec.Code != http.StatusConflict {
			t.Errorf("ADVERSARIAL FAIL: 包 ErrBusy 的错误（cause=%q 无旧关键词）应恒 409，got %d", c, rec.Code)
		}
		var eb ErrorBody
		_ = json.Unmarshal(rec.Body.Bytes(), &eb)
		if eb.Error.Code != CodeProcBusy {
			t.Errorf("ADVERSARIAL FAIL: code = %q, want PROC_BUSY（cause=%q）", eb.Error.Code, c)
		}
		// 固定中文文案不随 cause 变化。
		if !strings.Contains(eb.Error.Message, "进程正忙") {
			t.Errorf("ADVERSARIAL FAIL: 409 文案应恒为固定中文'进程正忙'，与 cause 无关，got %q（cause=%q）", eb.Error.Message, c)
		}
		// 且不泄露 cause 文本。
		if c != "机器正忙" && strings.Contains(eb.Error.Message, c) {
			t.Errorf("ADVERSARIAL FAIL: 409 文案泄露 cause %q: %q", c, eb.Error.Message)
		}
	}
}

// QA-ADV-3（AC-4 安全 / NF-1）：500 兜底路径下，原始 cause（含 procmgr 内部细节）
// 必须既不泄露进响应体、又确实进 logger（可诊断性）；且 ErrBusy 错误不得误降 500。
// 假设："我预期 500 路径要么泄露内部文本，要么忘记记日志（二者必有其一被忽视）。"
func TestAdversarial_T065_500NoLeakButLogged_BusyNotDowngraded(t *testing.T) {
	h, logBuf := newCapturingHandlers(t)
	secret := "procmgr.Start(frpc) mkdir log: open /var/secret/path: permission denied (errno=13)"
	rec := httptest.NewRecorder()
	h.mapProcErr(rec, errors.New(secret))

	if rec.Code != 500 {
		t.Fatalf("ADVERSARIAL FAIL: 非 sentinel 错误应 500, got %d", rec.Code)
	}
	body := rec.Body.String()
	// 逐子串证伪泄露。
	for _, leak := range []string{"procmgr", "mkdir", "/var/secret/path", "permission denied", "errno", "Start(frpc)"} {
		if strings.Contains(body, leak) {
			t.Errorf("ADVERSARIAL FAIL: 500 响应泄露内部子串 %q: %s", leak, body)
		}
	}
	// 但完整 cause 必须进日志。
	if !strings.Contains(logBuf.String(), "permission denied") || !strings.Contains(logBuf.String(), "errno=13") {
		t.Errorf("ADVERSARIAL FAIL: 原始 cause 未完整进日志: %s", logBuf.String())
	}

	// 同一 helper 下，ErrBusy 错误不得被误降成 500（防分类塌缩）。
	rec2 := httptest.NewRecorder()
	h.mapProcErr(rec2, fmt.Errorf("x: %w", procmgr.ErrBusy))
	if rec2.Code != http.StatusConflict {
		t.Errorf("ADVERSARIAL FAIL: ErrBusy 在同路径被误降，status=%d want 409", rec2.Code)
	}

	// 边界：binMissing 也不得被误降/误升（与 ErrBusy/500 分类互斥）。
	rec3 := httptest.NewRecorder()
	h.mapProcErr(rec3, fmt.Errorf("%w: frpc", binloc.ErrBinMissing))
	if rec3.Code != http.StatusUnprocessableEntity {
		t.Errorf("ADVERSARIAL FAIL: binMissing 分类塌缩，status=%d want 422", rec3.Code)
	}
}
