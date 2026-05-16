package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ModeState 是 GET/PUT /api/v1/mode 的请求 / 响应体。
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
	writeJSON(w, http.StatusOK, req)
}

func readBoolKV(ctx context.Context, h *handlers, key string) bool {
	v, ok, err := h.deps.Store.KVGet(ctx, key)
	if err != nil || !ok {
		return false
	}
	b, _ := strconv.ParseBool(v)
	return b
}

// 维持 fmt 引用（部分文件后续添加日志时复用）。
var _ = fmt.Sprintf
