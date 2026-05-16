package httpapi

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/frp-easy/frp-easy/internal/logtail"

	"github.com/go-chi/chi/v5"
)

// LogsTailResponse 是 ?lines=N 模式的响应。
type LogsTailResponse struct {
	Lines []string `json:"lines"`
}

// LogsIncrementalResponse 是 ?offset=N 模式的响应。
type LogsIncrementalResponse struct {
	Data       string `json:"data"`
	NextOffset int64  `json:"nextOffset"`
}

func (h *handlers) logs(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	if !validProcKind(kind) {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "kind 只能是 frpc/frps", "kind")
		return
	}
	path, ok := h.deps.LogFiles[kind]
	if !ok || path == "" {
		writeError(w, http.StatusNotFound, CodeNotFound, "未配置日志路径", "")
		return
	}
	// 若文件不存在 → 返回空（避免 UI 误报错），属正常状态（进程从未启动）。
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if r.URL.Query().Get("offset") != "" {
			writeJSON(w, http.StatusOK, LogsIncrementalResponse{Data: "", NextOffset: 0})
		} else {
			writeJSON(w, http.StatusOK, LogsTailResponse{Lines: []string{}})
		}
		return
	}

	if offStr := r.URL.Query().Get("offset"); offStr != "" {
		off, _ := strconv.ParseInt(offStr, 10, 64)
		data, next, err := logtail.ReadFrom(path, off)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "读取日志失败", "")
			return
		}
		writeJSON(w, http.StatusOK, LogsIncrementalResponse{Data: string(data), NextOffset: next})
		return
	}
	n := 500
	if linesStr := r.URL.Query().Get("lines"); linesStr != "" {
		if v, err := strconv.Atoi(linesStr); err == nil && v > 0 && v <= 5000 {
			n = v
		}
	}
	lines, err := logtail.TailLines(path, n)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "读取日志失败", "")
		return
	}
	if lines == nil {
		lines = []string{}
	}
	writeJSON(w, http.StatusOK, LogsTailResponse{Lines: lines})
}
