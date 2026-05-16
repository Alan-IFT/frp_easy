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
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// FRPVersion is the target FRP binary version.
// Verified by running frp_win/frpc.exe --version on the vendored binary.
const FRPVersion = "0.68.1"

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
	client *http.Client
	logger *slog.Logger

	// Unexported override fields for package-internal tests (C-4).
	baseURL string // empty = use GitHub CDN
	goos    string // empty = runtime.GOOS
}

// New creates a Manager.
// C-2 gate condition: signature must be func New(root string, logger *slog.Logger) *Manager.
func New(root string, logger *slog.Logger) *Manager {
	return &Manager{
		states: map[string]*DownloadState{
			"frpc": {Status: StatusIdle},
			"frps": {Status: StatusIdle},
		},
		root:   root,
		client: &http.Client{Timeout: 60 * time.Second},
		logger: logger,
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

	targetDir, targetPath, downloadURL, archiveExt, entryName, err := m.resolveParams(kind, goos)
	if err != nil {
		m.setFailed(kind, err.Error())
		return
	}

	m.logger.Info("download started",
		"kind", kind, "goos", goos, "url", downloadURL)

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

	resp, err := m.client.Do(req)
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

	// Step 3: Atomic rename: temp → target (NF-S1).
	// On Linux, os.Rename atomically replaces any existing file.
	// On Windows, Rename fails if the target already exists; we fall back to
	// Remove-then-Rename.  If Remove fails (e.g. permissions, file in use) we
	// report the error rather than silently destroying the existing binary.
	renameErr := os.Rename(binTmpPath, targetPath)
	if renameErr != nil && runtime.GOOS == "windows" {
		// Windows does not allow overwriting an existing file with Rename.
		// Remove the old binary first; ignore ErrNotExist (file may already be absent).
		if removeErr := os.Remove(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
			os.Remove(binTmpPath)
			m.setFailed(kind, fmt.Sprintf("移除旧版本失败: %v", removeErr))
			return
		}
		renameErr = os.Rename(binTmpPath, targetPath)
	}
	if renameErr != nil {
		os.Remove(binTmpPath)
		m.setFailed(kind, fmt.Sprintf("安装失败: %v", renameErr))
		return
	}

	// Step 4: Set executable permission on Linux.
	if goos == "linux" {
		if err := os.Chmod(targetPath, 0o755); err != nil {
			m.logger.Warn("chmod failed", "path", targetPath, "err", err)
		}
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

// resolveParams computes all path/URL parameters for a given kind + GOOS.
func (m *Manager) resolveParams(kind, goos string) (targetDir, targetPath, downloadURL, archiveExt, entryName string, err error) {
	base := m.baseURL
	if base == "" {
		base = "https://github.com/fatedier/frp/releases/download"
	}

	switch goos {
	case "linux":
		targetDir = filepath.Join(m.root, "frp_linux")
		archiveExt = ".tar.gz"
		downloadURL = fmt.Sprintf("%s/v%s/frp_%s_linux_amd64.tar.gz", base, FRPVersion, FRPVersion)
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
		downloadURL = fmt.Sprintf("%s/v%s/frp_%s_windows_amd64.zip", base, FRPVersion, FRPVersion)
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
