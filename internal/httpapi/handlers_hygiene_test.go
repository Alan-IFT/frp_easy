package httpapi

// T-055 B：500 兜底固定文案 + 不外泄内部细节 + 进日志 单测。
//
// procStop / downloadBin 的 500 兜底分支在黑盒 HTTP 测试中实际不可达
// （ProcMgr/Downloader 是具体类型，其 Stop/Start 仅返 sentinel，且 handler 已前置
//  validProcKind/sentinel 分类）。故 B-1/B-2/B-3 的"固定文案 + 不含内部子串 + 进日志"
// 行为统一在 writeInternalError helper 的直接单测中验证（02 §6 测试 seam 论证）。

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frp-easy/frp-easy/internal/storage"
)

// newCapturingHandlers 构造一个挂了捕获型 logger 的 *handlers，返回 handler + 日志 buffer。
func newCapturingHandlers(t *testing.T) (*handlers, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	return &handlers{deps: Dependencies{Logger: logger}}, &buf
}

// secretCause 模拟一个含内部敏感细节的底层 error（SQL 约束 / 驱动 / errno 等）。
const secretCause = "UNIQUE constraint failed: proxies.remote_port (driver=sqlite errno=2067)"

// TestWriteInternalError_FixedMessage_NoLeak 验证 B-1/B-3 共用路径：
// 响应只含固定文案，不含 cause 的任何内部子串；原始 cause 进日志。
func TestWriteInternalError_FixedMessage_NoLeak(t *testing.T) {
	h, logBuf := newCapturingHandlers(t)
	rec := httptest.NewRecorder()
	cause := errors.New(secretCause)

	h.writeInternalError(rec, "停止进程失败", cause)

	if rec.Code != 500 {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "停止进程失败") {
		t.Errorf("响应缺固定文案: %s", body)
	}
	// 关键断言：内部细节不得出现在前端响应体。
	for _, leak := range []string{secretCause, "UNIQUE", "constraint", "sqlite", "errno", "remote_port"} {
		if strings.Contains(body, leak) {
			t.Errorf("响应泄露内部细节 %q: %s", leak, body)
		}
	}
	// code 应为 INTERNAL。
	if !strings.Contains(body, `"code":"INTERNAL"`) {
		t.Errorf("code 非 INTERNAL: %s", body)
	}
	// 原始 cause 必须进日志（保留可诊断性）。
	if !strings.Contains(logBuf.String(), "UNIQUE constraint failed") {
		t.Errorf("原始 cause 未进日志: %s", logBuf.String())
	}
}

// TestWriteInternalError_NilLogger 验证 BC-5：logger 为 nil 时不 panic，固定文案照常返回。
func TestWriteInternalError_NilLogger(t *testing.T) {
	h := &handlers{deps: Dependencies{Logger: nil}}
	rec := httptest.NewRecorder()
	h.writeInternalError(rec, "启动下载失败", errors.New(secretCause))
	if rec.Code != 500 {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "启动下载失败") {
		t.Errorf("响应缺固定文案: %s", body)
	}
	if strings.Contains(body, "sqlite") || strings.Contains(body, "errno") {
		t.Errorf("响应泄露内部细节: %s", body)
	}
}

// TestMapProxyWriteError_Fallback_NoLeak 验证 B-2：mapProxyWriteError 兜底 500
// 返回固定文案"保存失败"（去掉 ": "+SQL 拼接），不含裸 SQL 子串；原始 error 进日志。
// 同时验证前置语义化分支（ErrDuplicateName 409 / validation 透传）未被破坏。
func TestMapProxyWriteError_Fallback_NoLeak(t *testing.T) {
	h, logBuf := newCapturingHandlers(t)

	// 兜底：构造一个不命中任何前置分支的裸 SQL 错误。
	rec := httptest.NewRecorder()
	h.mapProxyWriteError(rec, errors.New("disk I/O error: database is locked (errno=5)"))
	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"message":"保存失败"`) {
		t.Errorf("兜底文案应严格为'保存失败'（无 SQL 后缀）: %s", body)
	}
	for _, leak := range []string{"disk I/O", "database is locked", "errno"} {
		if strings.Contains(body, leak) {
			t.Errorf("兜底响应泄露内部细节 %q: %s", leak, body)
		}
	}
	if !strings.Contains(logBuf.String(), "database is locked") {
		t.Errorf("原始 error 未进日志: %s", logBuf.String())
	}
}

// TestMapProxyWriteError_DuplicateName_Preserved 验证 B-2 保留：
// ErrDuplicateName 仍走 409 语义化分支，不被兜底吞掉。
func TestMapProxyWriteError_DuplicateName_Preserved(t *testing.T) {
	h, _ := newCapturingHandlers(t)
	rec := httptest.NewRecorder()
	h.mapProxyWriteError(rec, storage.ErrDuplicateName)
	if rec.Code != 409 {
		t.Errorf("ErrDuplicateName status = %d, want 409", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "代理名称已存在") {
		t.Errorf("409 文案被破坏: %s", rec.Body.String())
	}
}

// TestMapProxyWriteError_Validation_Preserved 验证 B-2 保留：
// validation 类错误（含 "invalid"）仍走 422 透传分支，不被兜底吞掉。
func TestMapProxyWriteError_Validation_Preserved(t *testing.T) {
	h, _ := newCapturingHandlers(t)
	rec := httptest.NewRecorder()
	h.mapProxyWriteError(rec, errors.New("remotePort invalid: must be 1-65535"))
	if rec.Code != 422 {
		t.Errorf("validation status = %d, want 422", rec.Code)
	}
	// validation 分支是有意透传（语义化），文案应保留原文。
	if !strings.Contains(rec.Body.String(), "must be 1-65535") {
		t.Errorf("validation 透传被破坏: %s", rec.Body.String())
	}
}
