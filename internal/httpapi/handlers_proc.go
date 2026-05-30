package httpapi

import (
	"context"
	"errors"
	"net/http"

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
		h.mapProcErr(w, err)
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
		h.mapProcErr(w, err)
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

// mapProcErr 把 ProcMgr.Start / Restart 的错误映射为 HTTP 响应。
//
// 【T-065】此前靠 strings.ToLower + strings.Contains 匹配 procmgr 返回的内部英文文本
// （"stopping"/"starting"/"running"）分类 409 PROC_BUSY，并把原始 error 文本直接透传
// 前端的 500 —— 脆弱反模式（procmgr 改文本即静默漏判）+ 信息泄露。现收口为：
//   - binloc.ErrBinMissing → 422 BIN_MISSING（保持现状，binloc sentinel 已是好范式）
//   - procmgr.ErrBusy（sentinel，errors.Is 沿 wrap 链判定）→ 409 PROC_BUSY，固定中文文案
//     （不透传 procmgr 内部英文 cause）
//   - 其余 → h.writeInternalError（500 INTERNAL 固定中文 + 原始 error 进 logger，不外泄）
//
// 与 mapProxyWriteError（handlers_proxies.go，T-059）/ writeInternalError（T-055）同方向收口。
// 注意：procmgr 当前唯一的"忙"错误来自 Start 的 StateStopping 分支（StateStarting/StateRunning
// 是 idempotent 不报错），故删除对 "starting"/"running" 文本的匹配同时消除了既有空匹配。
func (h *handlers) mapProcErr(w http.ResponseWriter, err error) {
	if errors.Is(err, binloc.ErrBinMissing) {
		writeError(w, http.StatusUnprocessableEntity, CodeBinMissing, err.Error(), "")
		return
	}
	if errors.Is(err, procmgr.ErrBusy) {
		writeError(w, http.StatusConflict, CodeProcBusy, "进程正忙（启动或停止进行中），请稍后重试", "")
		return
	}
	h.writeInternalError(w, "操作进程失败", err)
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
