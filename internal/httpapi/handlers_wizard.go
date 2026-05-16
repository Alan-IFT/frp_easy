package httpapi

import (
	"net/http"
)

// kvWizardHandled is the KV key used to persist "wizard has been shown/handled".
// Value "true" means the wizard should not auto-show on next login.
const kvWizardHandled = "wizard.handled"

// WizardStatus is the response for GET /api/v1/wizard/status.
type WizardStatus struct {
	Handled    bool `json:"handled"`
	ShouldShow bool `json:"shouldShow"`
}

// wizardStatus handles GET /api/v1/wizard/status.
//
// shouldShow = !handled && !hasAnyConfig
// hasAnyConfig = frpc.serverConn KV exists
//             || frps.config KV exists
//             || mode.frpc.enabled == "true"
//             || mode.frps.enabled == "true"
func (h *handlers) wizardStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if wizard has already been handled (completed or skipped).
	handled := false
	if v, ok, err := h.deps.Store.KVGet(ctx, kvWizardHandled); err == nil && ok && v == "true" {
		handled = true
	}

	// Determine hasAnyConfig: any pre-existing configuration means the user
	// has already used the system and doesn't need the wizard.
	hasAnyConfig := false

	if !hasAnyConfig {
		if _, ok, err := h.deps.Store.KVGet(ctx, kvFrpcServerCfg); err == nil && ok {
			hasAnyConfig = true
		}
	}
	if !hasAnyConfig {
		if _, ok, err := h.deps.Store.KVGet(ctx, kvFrpsConfig); err == nil && ok {
			hasAnyConfig = true
		}
	}
	if !hasAnyConfig {
		if v, ok, err := h.deps.Store.KVGet(ctx, "mode.frpc.enabled"); err == nil && ok && v == "true" {
			hasAnyConfig = true
		}
	}
	if !hasAnyConfig {
		if v, ok, err := h.deps.Store.KVGet(ctx, "mode.frps.enabled"); err == nil && ok && v == "true" {
			hasAnyConfig = true
		}
	}

	writeJSON(w, http.StatusOK, WizardStatus{
		Handled:    handled,
		ShouldShow: !handled && !hasAnyConfig,
	})
}

// wizardComplete handles POST /api/v1/wizard/complete.
// Both "complete" and "skip" actions call this endpoint — either way
// wizard.handled is set to "true" and the wizard will not auto-show again.
func (h *handlers) wizardComplete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.deps.Store.KVSet(ctx, kvWizardHandled, "true"); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "保存失败", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
