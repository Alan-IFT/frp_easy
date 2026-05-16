package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// ModeState 是 GET/PUT /api/v1/mode 的请求/响应体。
type ModeState struct {
	Frpc bool `json:"frpc"`
	Frps bool `json:"frps"`
}

const (
	kvModeFrpc = "mode.frpc.enabled"
	kvModeFrps = "mode.frps.enabled"
)

func (h *handlers) getMode(w http.ResponseWriter, r *http.Request) {
	state := ModeState{
		Frpc: readBoolKV(r.Context(), h, kvModeFrpc),
		Frps: readBoolKV(r.Context(), h, kvModeFrps),
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *handlers) putMode(w http.ResponseWriter, r *http.Request) {
	var req ModeState
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}
	if err := h.deps.Store.KVSet(r.Context(), kvModeFrpc, strconv.FormatBool(req.Frpc)); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "保存失败", "")
		return
	}
	if err := h.deps.Store.KVSet(r.Context(), kvModeFrps, strconv.FormatBool(req.Frps)); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "保存失败", "")
		return
	}
	// B-7: 模式开关变更时立即启停进程
	if h.deps.ProcMgr != nil {
		h.applyModeToProc(r.Context(), "frpc", req.Frpc)
		h.applyModeToProc(r.Context(), "frps", req.Frps)
	}
	writeJSON(w, http.StatusOK, req)
}

// applyModeToProc 若 enabled=true 则先写 TOML 再 Start，否则 Stop。
func (h *handlers) applyModeToProc(ctx context.Context, kind string, enable bool) {
	if enable {
		h.applyConfigBestEffort(ctx, kind) // 先写 TOML
		if _, err := h.deps.ProcMgr.Start(kind); err != nil {
			if h.deps.Logger != nil {
				h.deps.Logger.Warn("mode start failed", "kind", kind, "err", err)
			}
		}
	} else {
		if _, err := h.deps.ProcMgr.Stop(kind); err != nil {
			if h.deps.Logger != nil {
				h.deps.Logger.Warn("mode stop failed", "kind", kind, "err", err)
			}
		}
	}
}

func readBoolKV(ctx context.Context, h *handlers, key string) bool {
	v, ok, err := h.deps.Store.KVGet(ctx, key)
	if err != nil || !ok {
		return false
	}
	b, _ := strconv.ParseBool(v)
	return b
}

