package httpapi

// T-055 B：500 兜底固定文案 + 不外泄内部细节 + 进日志 单测。
//
// procStop / downloadBin 的 500 兜底分支在黑盒 HTTP 测试中实际不可达
// （ProcMgr/Downloader 是具体类型，其 Stop/Start 仅返 sentinel，且 handler 已前置
//  validProcKind/sentinel 分类）。故 B-1/B-2/B-3 的"固定文案 + 不含内部子串 + 进日志"
// 行为统一在 writeInternalError helper 的直接单测中验证（02 §6 测试 seam 论证）。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/procmgr"
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

// TestMapProxyWriteError_Validation_FixedMessage_NoLeak 验证 T-059：
// storage 层校验类错误（含 "invalid"/"requires"/"must not"）走 422，但**不再透传**
// storage 生成的英文原文——改为固定中文"代理配置校验失败"，响应体不含原始英文；
// 原始 error 进 logger（Warn）便于排障。
//
// （此前为 TestMapProxyWriteError_Validation_Preserved，断言透传 "must be 1-65535"。
// T-059 有意改变该行为：不向前端泄露内部英文文本，与 T-055 writeInternalError 原则一致。
// PM 据红线 3 显式批准此断言更新，见 PM_LOG 强条件 C-1。）
func TestMapProxyWriteError_Validation_FixedMessage_NoLeak(t *testing.T) {
	h, logBuf := newCapturingHandlers(t)
	rec := httptest.NewRecorder()
	h.mapProxyWriteError(rec, errors.New("storage.UpsertProxy: udp proxy must not set customDomains"))
	if rec.Code != 422 {
		t.Errorf("validation status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "代理配置校验失败") {
		t.Errorf("validation 应返回固定中文文案: %s", body)
	}
	// 不得向前端泄露 storage 生成的英文文本。
	for _, leak := range []string{"must not set", "customDomains", "UpsertProxy", "udp proxy"} {
		if strings.Contains(body, leak) {
			t.Errorf("validation 响应泄露内部英文 %q: %s", leak, body)
		}
	}
	// 原始 error 应进日志便于排障。
	if !strings.Contains(logBuf.String(), "must not set customDomains") {
		t.Errorf("原始 validation error 未进日志: %s", logBuf.String())
	}
}

// TestMapProxyWriteError_DuplicateRemotePort 验证 T-059 AC-6：
// (type, remote_port) 组合冲突 sentinel ErrDuplicateRemotePort → 422 + field=remotePort
// + 固定中文，响应体不含任何 SQL/驱动英文文本。
func TestMapProxyWriteError_DuplicateRemotePort(t *testing.T) {
	h, _ := newCapturingHandlers(t)
	rec := httptest.NewRecorder()
	h.mapProxyWriteError(rec, storage.ErrDuplicateRemotePort)
	if rec.Code != 422 {
		t.Errorf("ErrDuplicateRemotePort status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	var eb ErrorBody
	if err := json.Unmarshal([]byte(body), &eb); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if eb.Error.Code != CodeConflict {
		t.Errorf("code = %q, want %q", eb.Error.Code, CodeConflict)
	}
	if eb.Error.Field != "remotePort" {
		t.Errorf("field = %q, want remotePort", eb.Error.Field)
	}
	if !strings.Contains(eb.Error.Message, "远程端口") {
		t.Errorf("message 应为固定中文（含'远程端口'）: %q", eb.Error.Message)
	}
	// 响应体不得含 SQL/驱动英文子串。
	for _, leak := range []string{"UNIQUE", "constraint", "proxies.", "remote_port", "duplicate"} {
		if strings.Contains(body, leak) {
			t.Errorf("响应泄露 SQL/驱动文本 %q: %s", leak, body)
		}
	}
}

// --- T-065：mapProcErr sentinel 收口直测 ---
//
// mapProcErr 此前靠 strings.Contains 匹配 procmgr 内部英文文本分类 409，并把原始 error
// 文本透传 500 —— 脆弱反模式 + 信息泄露。现收口为 errors.Is(procmgr.ErrBusy)→409 固定中文 +
// fallback writeInternalError(500 固定中文 + 原始 error 进日志，不外泄)。直测范式同
// mapProxyWriteError（newCapturingHandlers + httptest.NewRecorder + 捕获型 slog，insight L28）。

// procBusyErr 模拟 procmgr.Start 在 StateStopping 分支返回的错误：含内部英文 cause + wrap ErrBusy。
var procBusyErr = fmt.Errorf("procmgr.Start(frpc): currently stopping: %w", procmgr.ErrBusy)

// TestMapProcErr_Busy_409_FixedMessage_NoLeak 验证 T-065 AC-3：
// 包了 procmgr.ErrBusy 的错误 → 409 PROC_BUSY + 固定中文，响应体不含 procmgr 内部英文 cause。
func TestMapProcErr_Busy_409_FixedMessage_NoLeak(t *testing.T) {
	h, _ := newCapturingHandlers(t)
	rec := httptest.NewRecorder()
	h.mapProcErr(rec, procBusyErr)

	if rec.Code != 409 {
		t.Errorf("ErrBusy status = %d, want 409", rec.Code)
	}
	body := rec.Body.String()
	var eb ErrorBody
	if err := json.Unmarshal([]byte(body), &eb); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if eb.Error.Code != CodeProcBusy {
		t.Errorf("code = %q, want %q", eb.Error.Code, CodeProcBusy)
	}
	if !strings.Contains(eb.Error.Message, "进程正忙") {
		t.Errorf("message 应为固定中文（含'进程正忙'）: %q", eb.Error.Message)
	}
	// 不得向前端泄露 procmgr 内部英文 cause。
	for _, leak := range []string{"procmgr", "currently stopping", "Start(frpc)", "process busy"} {
		if strings.Contains(body, leak) {
			t.Errorf("409 响应泄露 procmgr 内部文本 %q: %s", leak, body)
		}
	}
}

// TestMapProcErr_Internal_500_NoLeak 验证 T-065 AC-4：
// 非 sentinel、非 binMissing 的错误 → 500 INTERNAL + 固定中文'操作进程失败'，
// 响应体不含原始 error 子串；原始 error 进 logger。
func TestMapProcErr_Internal_500_NoLeak(t *testing.T) {
	h, logBuf := newCapturingHandlers(t)
	rec := httptest.NewRecorder()
	// 模拟 Start 的非"忙"错误分支（如 no config path / mkdir 失败）。
	cause := errors.New("procmgr.Start(frps): no config path configured")
	h.mapProcErr(rec, cause)

	if rec.Code != 500 {
		t.Errorf("非 sentinel 错误 status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "操作进程失败") {
		t.Errorf("500 应返回固定中文'操作进程失败': %s", body)
	}
	if !strings.Contains(body, `"code":"INTERNAL"`) {
		t.Errorf("code 应为 INTERNAL: %s", body)
	}
	// 不得向前端泄露 procmgr 内部 cause。
	for _, leak := range []string{"procmgr", "no config path", "Start(frps)", "configured"} {
		if strings.Contains(body, leak) {
			t.Errorf("500 响应泄露 procmgr 内部文本 %q: %s", leak, body)
		}
	}
	// 原始 cause 必须进日志（保留可诊断性）。
	if !strings.Contains(logBuf.String(), "no config path configured") {
		t.Errorf("原始 cause 未进日志: %s", logBuf.String())
	}
}

// TestMapProcErr_BinMissing_422_Preserved 验证 T-065 AC-5（回归护栏）：
// binloc.ErrBinMissing 仍走 422 BIN_MISSING，行为不变（不被新分类破坏）。
func TestMapProcErr_BinMissing_422_Preserved(t *testing.T) {
	h, _ := newCapturingHandlers(t)
	rec := httptest.NewRecorder()
	h.mapProcErr(rec, fmt.Errorf("%w: frpc at /opt/frpc", binloc.ErrBinMissing))

	if rec.Code != 422 {
		t.Errorf("ErrBinMissing status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"BIN_MISSING"`) {
		t.Errorf("code 应为 BIN_MISSING: %s", rec.Body.String())
	}
}
