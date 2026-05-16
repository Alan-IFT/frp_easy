package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/frp-easy/frp-easy/internal/downloader"
	"github.com/go-chi/chi/v5"
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

// --- Public IP detection (B-13, B-14, AC-12, AC-13) ---

// ipResult holds the result of one public-IP lookup attempt.
type ipResult struct {
	IP       string
	Advisory string // non-empty for IPv6 addresses
	ErrMsg   string
}

// ipCache is a process-scoped in-memory cache for the public IP result (TTL 5 min).
// The zero value is safe to use.
// C-1 gate: the `ipCache ipCache` field is declared in router.go's handlers struct.
type ipCache struct {
	mu        sync.Mutex
	result    *ipResult
	fetchedAt time.Time
}

// PublicIPResponse is the response body for GET /api/v1/system/public-ip.
// Always HTTP 200 (B-14 requirement — never 4xx/5xx on timeout).
type PublicIPResponse struct {
	IP       string `json:"ip,omitempty"`
	Error    string `json:"error,omitempty"`
	Advisory string `json:"advisory,omitempty"` // IPv6 usage hint
}

// DownloadBinRequest is the request body for POST /api/v1/system/download-bin.
type DownloadBinRequest struct {
	Kind string `json:"kind"` // "frpc" | "frps"
}

// DownloadStatusResponse is an alias for downloader.DownloadState (used for clarity in docs).
type DownloadStatusResponse = downloader.DownloadState

// systemPublicIP handles GET /api/v1/system/public-ip.
// Caches results for 5 minutes to avoid hammering external services (B-14).
func (h *handlers) systemPublicIP(w http.ResponseWriter, r *http.Request) {
	const cacheTTL = 5 * time.Minute

	h.ipCache.mu.Lock()
	if h.ipCache.result != nil && time.Since(h.ipCache.fetchedAt) < cacheTTL {
		result := *h.ipCache.result
		h.ipCache.mu.Unlock()
		h.deps.Logger.Debug("public-ip cache hit", "ip", result.IP)
		respondWithIPResult(w, result)
		return
	}
	h.ipCache.mu.Unlock()

	// Fetch from external services (max 3s total, NF-P1).
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	result := fetchPublicIP(ctx)

	// Store in cache (concurrent requests may both miss and both fetch — acceptable for MVP).
	h.ipCache.mu.Lock()
	h.ipCache.result = &result
	h.ipCache.fetchedAt = time.Now()
	h.ipCache.mu.Unlock()

	respondWithIPResult(w, result)
}

// downloadBin handles POST /api/v1/system/download-bin → 202 or 409 (AC-2, AC-4).
func (h *handlers) downloadBin(w http.ResponseWriter, r *http.Request) {
	if h.deps.Downloader == nil {
		writeError(w, http.StatusServiceUnavailable, CodeInternal, "下载器未初始化", "")
		return
	}

	var req DownloadBinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}

	if err := h.deps.Downloader.Start(req.Kind); err != nil {
		switch {
		case errors.Is(err, downloader.ErrAlreadyInProgress):
			writeError(w, http.StatusConflict, CodeProcBusy, "下载已在进行中", "")
		case errors.Is(err, downloader.ErrBadKind):
			writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "kind 必须为 frpc 或 frps", "kind")
		case errors.Is(err, downloader.ErrUnsupportedOS):
			writeError(w, http.StatusServiceUnavailable, CodeInternal, "不支持的操作系统", "")
		default:
			writeError(w, http.StatusInternalServerError, CodeInternal, err.Error(), "")
		}
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// downloadStatus handles GET /api/v1/system/download-status/{kind} → 200 DownloadState (AC-2).
func (h *handlers) downloadStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.Downloader == nil {
		writeError(w, http.StatusServiceUnavailable, CodeInternal, "下载器未初始化", "")
		return
	}

	kind := chi.URLParam(r, "kind")
	st, ok := h.deps.Downloader.Status(kind)
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, "无效的 kind，必须是 frpc 或 frps", "")
		return
	}

	writeJSON(w, http.StatusOK, st)
}

// --- internal helpers for public IP detection ---

// fetchPublicIP tries two external services sequentially, returning on first success.
// Total budget is enforced by ctx (caller sets 3s timeout).
// Each service gets at most 1.5s (per 02 §3.3).
func fetchPublicIP(ctx context.Context) ipResult {
	type candidate struct {
		url  string
		name string
	}
	services := []candidate{
		{"https://api.ipify.org?format=json", "ipify"},
		{"https://api.my-ip.io/json", "my-ip.io"},
	}

	for _, svc := range services {
		if ctx.Err() != nil {
			break
		}
		perCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		ip, err := fetchIPFromURL(perCtx, svc.url)
		cancel()
		if err == nil && ip != "" {
			return buildIPResult(ip)
		}
	}
	return ipResult{ErrMsg: "检测超时，请手动查询"}
}

// fetchIPFromURL performs a single GET and returns the "ip" field from the JSON response.
func fetchIPFromURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	return downloader.ParseIPFromJSON(data)
}

// buildIPResult constructs an ipResult and adds an IPv6 advisory if applicable.
func buildIPResult(ip string) ipResult {
	r := ipResult{IP: ip}
	if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() == nil {
		r.Advisory = fmt.Sprintf("IPv6 地址，frpc serverAddr 填写时请加方括号 [%s]", ip)
	}
	return r
}

// respondWithIPResult writes a PublicIPResponse (always HTTP 200, B-14).
func respondWithIPResult(w http.ResponseWriter, result ipResult) {
	resp := PublicIPResponse{}
	if result.ErrMsg != "" {
		resp.Error = result.ErrMsg
	} else {
		resp.IP = result.IP
		resp.Advisory = result.Advisory
	}
	writeJSON(w, http.StatusOK, resp)
}

// health handles GET /api/v1/health — 轻量存活检查，不经过 ReadyGate。
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": h.deps.Version,
	})
}
