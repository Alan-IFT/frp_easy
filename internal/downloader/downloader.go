// Package downloader manages async download, extraction, and atomic installation
// of frp binaries (frpc/frps) from GitHub Releases.
//
// Design ref: docs/features/zero-config-quickstart/02_SOLUTION_DESIGN.md §3.1
// Gate Review C-2: New(root string, logger *slog.Logger) *Manager
package downloader

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Download state constants.
const (
	StatusIdle        = "idle"
	StatusDownloading = "downloading"
	StatusSuccess     = "success"
	StatusFailed      = "failed"
)

// Sentinel errors returned by Start.
var (
	ErrAlreadyInProgress = errors.New("downloader: download already in progress")
	ErrUnsupportedOS     = errors.New("downloader: unsupported OS (only windows/linux amd64)")
	ErrBadKind           = errors.New("downloader: kind must be 'frpc' or 'frps'")
)

// DownloadState is the current state of a single kind's download (JSON-serializable).
type DownloadState struct {
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Error    string `json:"error,omitempty"`
}

// Manager manages concurrent async downloads for frpc and frps.
// root is binloc.Locator.Root() — the repository root directory.
type Manager struct {
	mu     sync.Mutex
	states map[string]*DownloadState
	root   string

	// T-025：拆分 HTTP client。apiClient 走短超时（60s 总）适合 GitHub Release API
	// JSON 查询；downloadClient 不设 Client.Timeout，由 Transport 阶段性上限
	// （dial / TLS / ResponseHeader）兜底，适合 archive 长链路下载。
	apiClient      *http.Client
	downloadClient *http.Client

	logger *slog.Logger

	// Unexported override fields for package-internal tests (C-4).
	apiBaseURL string // empty = use https://api.github.com（T-014 测试注入 seam）
	goos       string // empty = runtime.GOOS
}

// New creates a Manager.
// C-2 gate condition: signature must be func New(root string, logger *slog.Logger) *Manager.
func New(root string, logger *slog.Logger) *Manager {
	return &Manager{
		states: map[string]*DownloadState{
			"frpc": {Status: StatusIdle},
			"frps": {Status: StatusIdle},
		},
		root:           root,
		apiClient:      &http.Client{Timeout: 60 * time.Second},
		downloadClient: newDownloadHTTPClient(),
		logger:         logger,
	}
}

// newDownloadHTTPClient 返回 archive 下载专用 *http.Client。
//
// T-025: Client.Timeout 故意设为 0（无总超时）—— Transport 层的 dial / TLS /
// ResponseHeaderTimeout 已防御死连接。若未来想改成有总超时，请先读 T-025
// 02_SOLUTION_DESIGN.md §6 R-1（位于 docs/features/download-bin-timeout-fix/
// 或归档后 docs/features/_archived/download-bin-timeout-fix/）。
//
// 字段值来源：dial / KeepAlive / IdleConnTimeout / ForceAttemptHTTP2 / MaxIdleConns /
// ExpectContinueTimeout 沿用 stdlib DefaultTransport；显式新增
// TLSHandshakeTimeout=30s（国内 GitHub CDN TLS 偶慢于 stdlib 默认 10s）
// 与 ResponseHeaderTimeout=60s（RA D-2）。
func newDownloadHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 0, // 显式 0，文档化"无总超时"决策
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   30 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		},
	}
}

// Start triggers an async download for kind ("frpc" or "frps").
// Returns ErrAlreadyInProgress, ErrUnsupportedOS, or ErrBadKind on validation failure.
// frpc and frps downloads are independent and can run concurrently.
func (m *Manager) Start(kind string) error {
	if kind != "frpc" && kind != "frps" {
		return ErrBadKind
	}
	goos := m.goos
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "linux" && goos != "windows" {
		return ErrUnsupportedOS
	}

	m.mu.Lock()
	st := m.states[kind]
	if st.Status == StatusDownloading {
		m.mu.Unlock()
		return ErrAlreadyInProgress
	}
	// Set state synchronously before launching goroutine — prevents double-start race.
	st.Status = StatusDownloading
	st.Progress = 0
	st.Error = ""
	m.mu.Unlock()

	go m.doDownload(kind, goos)
	return nil
}

// Status returns a copy of the current DownloadState for kind.
// ok=false means kind is invalid (not "frpc" or "frps").
func (m *Manager) Status(kind string) (DownloadState, bool) {
	if kind != "frpc" && kind != "frps" {
		return DownloadState{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return *m.states[kind], true
}

// doDownload performs the download, extraction, and atomic install in a goroutine.
func (m *Manager) doDownload(kind, goos string) {
	startTime := time.Now()

	targetDir, targetPath, archiveExt, entryName, err := m.resolveParams(kind, goos)
	if err != nil {
		m.setFailed(kind, err.Error())
		return
	}

	// T-014：解析 fatedier/frp 最新 release 资产 URL（不再用写死的 FRPVersion 构造）。
	downloadURL, version, err := m.resolveLatestAsset(goos)
	if err != nil {
		m.setFailed(kind, err.Error()) // err 已是面向用户的中文消息
		return
	}

	m.logger.Info("download started",
		"kind", kind, "goos", goos, "version", version, "url", downloadURL)

	// C-2: Ensure target directory exists before creating temp files.
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		m.setFailed(kind, fmt.Sprintf("创建目录失败: %v", err))
		return
	}

	// Step 1: Download archive to a temp file.
	archiveTmp, err := os.CreateTemp(targetDir, ".dl-archive-*.tmp")
	if err != nil {
		m.setFailed(kind, fmt.Sprintf("创建临时文件失败: %v", err))
		return
	}
	archiveTmpPath := archiveTmp.Name()
	defer func() {
		archiveTmp.Close()
		os.Remove(archiveTmpPath) // always clean up archive temp
	}()

	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		m.setFailed(kind, fmt.Sprintf("构建下载请求失败: %v", err))
		return
	}

	resp, err := m.downloadClient.Do(req)
	if err != nil {
		m.setFailed(kind, fmt.Sprintf("下载超时: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		m.setFailed(kind, fmt.Sprintf("HTTP %d: 下载失败", resp.StatusCode))
		return
	}

	// Track progress via io.TeeReader + progressWriter (NF-O1, 02 §3.1).
	pw := &progressWriter{
		contentLength: resp.ContentLength,
		onProgress: func(pct int) {
			m.setProgress(kind, pct)
		},
	}
	teeReader := io.TeeReader(resp.Body, pw)

	bytesWritten, err := io.Copy(archiveTmp, teeReader)
	if err != nil {
		m.setFailed(kind, fmt.Sprintf("下载写入失败: %v", err))
		return
	}

	m.logger.Info("download complete, extracting",
		"kind", kind, "bytes", bytesWritten,
		"elapsed", time.Since(startTime).Round(time.Millisecond))

	// Rewind archive temp file before extraction.
	if _, err := archiveTmp.Seek(0, io.SeekStart); err != nil {
		m.setFailed(kind, fmt.Sprintf("重置文件指针失败: %v", err))
		return
	}

	// Step 2: Extract the target binary to a separate temp file.
	binTmp, err := os.CreateTemp(targetDir, ".dl-bin-*.tmp")
	if err != nil {
		m.setFailed(kind, fmt.Sprintf("创建二进制临时文件失败: %v", err))
		return
	}
	binTmpPath := binTmp.Name()

	var extractErr error
	switch archiveExt {
	case ".tar.gz":
		extractErr = extractFromTarGz(archiveTmpPath, entryName, binTmp)
	case ".zip":
		extractErr = extractFromZip(archiveTmpPath, entryName, binTmp)
	default:
		extractErr = fmt.Errorf("unknown archive extension: %s", archiveExt)
	}
	binTmp.Close()

	if extractErr != nil {
		os.Remove(binTmpPath)
		m.setFailed(kind, fmt.Sprintf("解压失败: %v", extractErr))
		return
	}

	// Step 3 + 4 (T-018 §A.2 refactor)：原子 rename + Linux chmod + Windows fallback
	// 全部走共享 Install；下载链路传 maxBytes = -1（不限大小，因 archive 已落盘）。
	binTmpFile, openErr := os.Open(binTmpPath)
	if openErr != nil {
		os.Remove(binTmpPath)
		m.setFailed(kind, fmt.Sprintf("打开解压临时文件失败: %v", openErr))
		return
	}
	_, _, _, installErr := m.Install(kind, binTmpFile, -1)
	binTmpFile.Close()
	// Install 内部用 CreateTemp 写自己的 .install-*.tmp；binTmpPath 已读尽不再需要。
	os.Remove(binTmpPath)
	if installErr != nil {
		m.setFailed(kind, fmt.Sprintf("安装失败: %v", installErr))
		return
	}

	elapsed := time.Since(startTime)
	m.logger.Info("download installed",
		"kind", kind, "path", targetPath,
		"elapsed", elapsed.Round(time.Millisecond))

	m.mu.Lock()
	m.states[kind].Status = StatusSuccess
	m.states[kind].Progress = 100
	m.states[kind].Error = ""
	m.mu.Unlock()
}

// resolveParams 计算路径与格式参数（不含 downloadURL —— 后者由 resolveLatestAsset 提供）。
func (m *Manager) resolveParams(kind, goos string) (targetDir, targetPath, archiveExt, entryName string, err error) {
	switch goos {
	case "linux":
		targetDir = filepath.Join(m.root, "frp_linux")
		archiveExt = ".tar.gz"
		switch kind {
		case "frpc":
			targetPath = filepath.Join(targetDir, "frpc")
			entryName = "frpc"
		case "frps":
			targetPath = filepath.Join(targetDir, "frps")
			entryName = "frps"
		}

	case "windows":
		targetDir = filepath.Join(m.root, "frp_win")
		archiveExt = ".zip"
		switch kind {
		case "frpc":
			targetPath = filepath.Join(targetDir, "frpc.exe")
			entryName = "frpc.exe"
		case "frps":
			targetPath = filepath.Join(targetDir, "frps.exe")
			entryName = "frps.exe"
		}

	default:
		err = ErrUnsupportedOS
	}
	return
}

// ghRelease 是 GitHub Release API 响应的最小子集。
type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// resolveLatestAsset 查询 fatedier/frp 最新 release，返回匹配 goos 的资产下载 URL 与版本号。
// 失败时返回的 error 已是面向用户的中文消息（可直接进 setFailed）。
//
// 实现要点：
//   - 设 User-Agent 头（GitHub API 对无 UA 的请求返回 403，会被误判为限流）。
//   - 先判 HTTP 状态码、再解析 JSON（限流 403 响应体也是合法 JSON）。
//   - 按平台后缀匹配 assets[]（_linux_amd64.tar.gz / _windows_amd64.zip），
//     比硬拼文件名鲁棒，且能精确实现"资产未匹配 → failed"分支。
func (m *Manager) resolveLatestAsset(goos string) (downloadURL, version string, err error) {
	apiBase := m.apiBaseURL
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	url := apiBase + "/repos/fatedier/frp/releases/latest"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("构建查询请求失败: %v", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "frp_easy")

	resp, err := m.apiClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("无法访问 GitHub（请检查网络或代理）: %v", err)
	}
	defer resp.Body.Close()

	// 先判状态码、后解析 JSON（限流 403 响应体也是合法 JSON）。
	switch resp.StatusCode {
	case http.StatusOK:
		// 继续
	case http.StatusForbidden:
		return "", "", fmt.Errorf("GitHub API 触发限流（未认证请求 60 次/小时/IP），请稍后重试，或按文档手动下载 frp 二进制")
	default:
		return "", "", fmt.Errorf("查询 frp 最新版本失败：HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB 上限，防御超大响应
	if err != nil {
		return "", "", fmt.Errorf("读取 GitHub 响应失败: %v", err)
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", "", fmt.Errorf("解析 GitHub 响应失败: %v", err)
	}
	if rel.TagName == "" {
		return "", "", fmt.Errorf("GitHub 响应缺少版本号字段")
	}

	// 按平台后缀匹配资产。
	var suffix string
	switch goos {
	case "linux":
		suffix = "_linux_amd64.tar.gz"
	case "windows":
		suffix = "_windows_amd64.zip"
	default:
		return "", "", ErrUnsupportedOS
	}
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, suffix) && a.DownloadURL != "" {
			return a.DownloadURL, rel.TagName, nil
		}
	}
	return "", "", fmt.Errorf("未找到匹配当前平台的 frp 资产（%s），请按文档手动下载", suffix)
}

// setProgress updates progress for kind (called from progressWriter).
func (m *Manager) setProgress(kind string, pct int) {
	m.mu.Lock()
	m.states[kind].Progress = pct
	m.mu.Unlock()
}

// setFailed marks kind as failed with errMsg.
func (m *Manager) setFailed(kind string, errMsg string) {
	m.logger.Error("download failed", "kind", kind, "err", errMsg)
	m.mu.Lock()
	m.states[kind].Status = StatusFailed
	m.states[kind].Error = errMsg
	m.mu.Unlock()
}

// progressWriter tracks bytes written and fires onProgress callbacks.
// Used as the write-side of io.TeeReader(resp.Body, progressWriter).
type progressWriter struct {
	contentLength int64
	written       int64
	lastPct       int
	onProgress    func(pct int)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	pw.written += int64(len(p))

	var pct int
	if pw.contentLength > 0 {
		// Real progress: cap at 95 while downloading; 100 is set on success.
		pct = int(pw.written * 95 / pw.contentLength)
		if pct > 95 {
			pct = 95
		}
	} else {
		// Pseudo-progress: every 512 KB = +2%, max 95%.
		pct = int(pw.written/(512*1024)) * 2
		if pct > 95 {
			pct = 95
		}
	}

	if pct != pw.lastPct {
		pw.lastPct = pct
		pw.onProgress(pct)
	}
	return len(p), nil
}

// extractFromTarGz extracts the entry with base name entryName from a .tar.gz archive.
// R-2 Zip Slip prevention: entries containing ".." or absolute paths are skipped.
func extractFromTarGz(archivePath, entryName string, out *os.File) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		// Zip Slip prevention (R-2).
		if strings.Contains(hdr.Name, "..") || filepath.IsAbs(hdr.Name) {
			continue
		}
		if filepath.Base(hdr.Name) != entryName {
			continue
		}
		// Found the target binary.
		if _, err := io.Copy(out, tr); err != nil {
			return fmt.Errorf("copy tar entry: %w", err)
		}
		return nil
	}
	return fmt.Errorf("binary %q not found in tar.gz archive", entryName)
}

// extractFromZip extracts the entry with base name entryName from a .zip archive.
// R-2 Zip Slip prevention: entries containing ".." or absolute paths are skipped.
func extractFromZip(archivePath, entryName string, out *os.File) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		// Normalise separators for Zip Slip check (R-2).
		name := filepath.ToSlash(f.Name)
		if strings.Contains(name, "..") || filepath.IsAbs(f.Name) || strings.HasPrefix(name, "/") {
			continue
		}
		if filepath.Base(f.Name) != entryName {
			continue
		}
		// Found the target binary.
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		if copyErr != nil {
			return fmt.Errorf("copy zip entry: %w", copyErr)
		}
		return nil
	}
	return fmt.Errorf("binary %q not found in zip archive", entryName)
}

// ipifyResponse is used to parse {"ip":"..."} JSON from IP detection services.
type ipifyResponse struct {
	IP string `json:"ip"`
}

// ParseIPFromJSON parses an IP address from a {"ip":"..."} JSON payload.
// It is exported so that httpapi/handlers_system.go can reuse the same parser.
func ParseIPFromJSON(data []byte) (string, error) {
	var r ipifyResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return "", err
	}
	if r.IP == "" {
		return "", errors.New("ip 字段为空")
	}
	return r.IP, nil
}
