package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/procmgr"

	"github.com/go-chi/chi/v5"
)

func (h *handlers) procStart(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	if !validProcKind(kind) {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "kind 只能是 frpc/frps", "kind")
		return
	}
	// 先写入 TOML（确保进程以最新配置启动）
	h.applyConfigBestEffort(r.Context(), kind)
	info, err := h.deps.ProcMgr.Start(kind)
	if err != nil {
		mapProcErr(w, err)
		return
	}
	// AC-9: 启动成功后更新 mode kv
	_ = h.persistMode(r.Context(), kind, true)
	writeJSON(w, http.StatusOK, info)
}

func (h *handlers) procStop(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	if !validProcKind(kind) {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "kind 只能是 frpc/frps", "kind")
		return
	}
	info, err := h.deps.ProcMgr.Stop(kind)
	if err != nil {
		// T-055 B-1：不向前端透传内部 error 细节；固定文案，原始 error 进日志。
		h.writeInternalError(w, "停止进程失败", err)
		return
	}
	// AC-9: 停止成功后更新 mode kv
	_ = h.persistMode(r.Context(), kind, false)
	writeJSON(w, http.StatusOK, info)
}

// persistMode 将 mode.{kind}.enabled 存入 kv（用于 AC-9 重启后自动恢复）。
func (h *handlers) persistMode(ctx context.Context, kind string, enabled bool) error {
	v := "false"
	if enabled {
		v = "true"
	}
	return h.deps.Store.KVSet(ctx, "mode."+kind+".enabled", v)
}

func (h *handlers) procRestart(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	if !validProcKind(kind) {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "kind 只能是 frpc/frps", "kind")
		return
	}
	info, err := h.deps.ProcMgr.Restart(kind)
	if err != nil {
		mapProcErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *handlers) procStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.ProcMgr == nil {
		writeJSON(w, http.StatusOK, map[string]procmgr.ProcessInfo{
			"frpc": {Kind: "frpc", State: procmgr.StateStopped},
			"frps": {Kind: "frps", State: procmgr.StateStopped},
		})
		return
	}
	writeJSON(w, http.StatusOK, h.deps.ProcMgr.StatusAll())
}

func validProcKind(k string) bool {
	return k == "frpc" || k == "frps"
}

func mapProcErr(w http.ResponseWriter, err error) {
	if errors.Is(err, binloc.ErrBinMissing) {
		writeError(w, http.StatusUnprocessableEntity, CodeBinMissing, err.Error(), "")
		return
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	if strings.Contains(low, "stopping") || strings.Contains(low, "starting") || strings.Contains(low, "running") {
		writeError(w, http.StatusConflict, CodeProcBusy, msg, "")
		return
	}
	writeError(w, http.StatusInternalServerError, CodeInternal, msg, "")
}

// writeInternalError 统一 500 兜底：向前端返回固定的面向用户文案（不含任何内部
// error 细节），同时把原始 error 记到 logger（保留可诊断性，配合 middleware
// RequestID 可关联定位）。logger 为 nil 时跳过记录（与 handlers_proxies.go:147
// 的 nil 守卫范式一致）。T-055 B：3 处兜底（procStop / mapProxyWriteError /
// downloadBin）共用本 helper，确保 SQL 约束文本 / 驱动细节 / errno 等不外泄。
func (h *handlers) writeInternalError(w http.ResponseWriter, userMsg string, cause error) {
	if h.deps.Logger != nil && cause != nil {
		h.deps.Logger.Error("internal error", "userMsg", userMsg, "cause", cause)
	}
	writeError(w, http.StatusInternalServerError, CodeInternal, userMsg, "")
}
