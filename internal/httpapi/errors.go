// Package httpapi 是 chi router + handlers + middlewares 的入口。
//
// 错误码清单按 02 §5.1 + Gate Review C-3（追加 NOT_READY 实现 ReadyGate）。
package httpapi

import (
	"encoding/json"
	"net/http"
)

// 错误码（02 §5.1 + C-3）。
const (
	CodeSetupRequired      = "SETUP_REQUIRED"
	CodeAlreadyInitialized = "ALREADY_INITIALIZED"
	CodeUnauthenticated    = "UNAUTHENTICATED"
	CodeCSRFFailed         = "CSRF_FAILED"
	CodeRateLimited        = "RATE_LIMITED"
	CodeValidationFailed   = "VALIDATION_FAILED"
	CodeConflict           = "CONFLICT"
	CodeNotFound           = "NOT_FOUND"
	CodeBinMissing         = "BIN_MISSING"
	CodeProcBusy           = "PROC_BUSY"
	CodeInternal           = "INTERNAL"
	CodeNotReady           = "NOT_READY" // 【C-3】ReadyGate 503 专用
)

// ErrorBody 是统一错误响应体。
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 见 02 §5.1。
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// writeError 写统一错误响应。
func writeError(w http.ResponseWriter, status int, code, msg, field string) {
	writeJSON(w, status, ErrorBody{Error: ErrorDetail{Code: code, Message: msg, Field: field}})
}
