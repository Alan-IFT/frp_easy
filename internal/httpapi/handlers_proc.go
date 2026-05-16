package httpapi

import (
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
	info, err := h.deps.ProcMgr.Start(kind)
	if err != nil {
		mapProcErr(w, err)
		return
	}
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
		writeError(w, http.StatusInternalServerError, CodeInternal, err.Error(), "")
		return
	}
	writeJSON(w, http.StatusOK, info)
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
