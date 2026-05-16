package httpapi

import (
	"net/http"
)

// SystemReady 是 GET /api/v1/system/ready 的响应体。
type SystemReady struct {
	Initialized bool     `json:"initialized"`
	BinMissing  []string `json:"binMissing"`
	Version     string   `json:"version"`
}

func (h *handlers) systemReady(w http.ResponseWriter, r *http.Request) {
	admin, err := h.deps.Store.GetAdmin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "读取管理员失败", "")
		return
	}
	missing := []string{}
	if h.deps.Locator != nil {
		missing = h.deps.Locator.Missing()
	}
	resp := SystemReady{
		Initialized: admin != nil,
		BinMissing:  missing,
		Version:     h.deps.Version,
	}
	writeJSON(w, http.StatusOK, resp)
}
