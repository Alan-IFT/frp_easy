// Package downloader — unit tests.
// C-4 gate condition: ALL tests use net/http/httptest.NewServer to mock the CDN.
// No real external network calls are made.
package downloader

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- helpers ---

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// buildTarGz creates an in-memory tar.gz archive from a map of entryPath → content.
func buildTarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0755,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar WriteHeader: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar Write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

// buildZip creates an in-memory zip archive from a map of entryPath → content.
func buildZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip Create: %v", err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("zip Write: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close: %v", err)
	}
	return buf.Bytes()
}

// waitForDone polls until Status(kind) leaves StatusDownloading or the timeout expires.
func waitForDone(t *testing.T, m *Manager, kind string, timeout time.Duration) DownloadState {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, _ := m.Status(kind)
		if st.Status != StatusDownloading {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	st, _ := m.Status(kind)
	t.Fatalf("timeout waiting for %s download; current: %+v", kind, st)
	return DownloadState{}
}

// --- Test 1: tar.gz download success + progress 0→100 ---

func TestDownload_TarGz_Success_Progress(t *testing.T) {
	// Create a valid tar.gz archive containing frpc (linux layout).
	archiveContent := buildTarGz(t, map[string]string{
		"frp_" + FRPVersion + "_linux_amd64/frpc": "#!/bin/sh\necho frpc binary",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", itoa(len(archiveContent)))
		w.WriteHeader(http.StatusOK)
		w.Write(archiveContent)
	}))
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.baseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st := waitForDone(t, m, "frpc", 5*time.Second)

	if st.Status != StatusSuccess {
		t.Errorf("expected status=%s, got %s (error=%q)", StatusSuccess, st.Status, st.Error)
	}
	if st.Progress != 100 {
		t.Errorf("expected progress=100, got %d", st.Progress)
	}

	// Verify the binary was installed.
	expectedPath := filepath.Join(root, "frp_linux", "frpc")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected binary at %s: %v", expectedPath, err)
	}
}

// --- Test 2: zip format success ---

func TestDownload_Zip_Success(t *testing.T) {
	// Create a valid zip archive containing frpc.exe (windows layout).
	archiveContent := buildZip(t, map[string]string{
		"frp_" + FRPVersion + "_windows_amd64/frpc.exe": "fake frpc exe content",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(archiveContent)
	}))
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.baseURL = srv.URL
	m.goos = "windows"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st := waitForDone(t, m, "frpc", 5*time.Second)

	if st.Status != StatusSuccess {
		t.Errorf("expected status=%s, got %s (error=%q)", StatusSuccess, st.Status, st.Error)
	}
	if st.Progress != 100 {
		t.Errorf("expected progress=100, got %d", st.Progress)
	}

	// Verify the binary was installed.
	expectedPath := filepath.Join(root, "frp_win", "frpc.exe")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected binary at %s: %v", expectedPath, err)
	}
}

// --- Test 3: ErrAlreadyInProgress (concurrent Start) ---

func TestDownload_ErrAlreadyInProgress(t *testing.T) {
	// Blocking server: hangs until unblock is closed.
	unblock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-unblock:
			// Server unblocked; respond with nothing (extraction will fail, that's OK).
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			// Request was cancelled (e.g., server closed).
		}
	}))
	defer func() {
		select {
		case <-unblock: // already closed
		default:
			close(unblock)
		}
		srv.Close()
	}()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.baseURL = srv.URL
	m.goos = "linux"
	m.client = &http.Client{Timeout: 5 * time.Second}

	// First Start: state becomes StatusDownloading synchronously.
	if err := m.Start("frpc"); err != nil {
		t.Fatalf("first Start: %v", err)
	}

	// Second Start on same kind: must return ErrAlreadyInProgress.
	err := m.Start("frpc")
	if !errors.Is(err, ErrAlreadyInProgress) {
		t.Errorf("expected ErrAlreadyInProgress, got %v", err)
	}

	// Different kind must NOT be blocked.
	// (frps is independent; if goos=linux and frps also uses linux, frps can start.)
	errFrps := m.Start("frps")
	if errors.Is(errFrps, ErrAlreadyInProgress) {
		t.Errorf("frps should not be blocked by frpc download")
	}

	// Unblock the server so goroutines can terminate.
	close(unblock)
	// Wait for frpc goroutine to finish (we don't care about success/failure here).
	waitForDone(t, m, "frpc", 3*time.Second)
}

// --- Test 4: HTTP non-2xx → StatusFailed ---

func TestDownload_HTTP404_StatusFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.baseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st := waitForDone(t, m, "frpc", 5*time.Second)
	if st.Status != StatusFailed {
		t.Errorf("expected status=%s, got %s", StatusFailed, st.Status)
	}
	if st.Error == "" {
		t.Error("expected non-empty error message")
	}
}

// --- Test 5: Zip Slip attempt → filtered, legitimate binary still extracted ---

func TestDownload_ZipSlip_MaliciousEntryFiltered(t *testing.T) {
	root := t.TempDir()

	// Archive contains:
	//   - malicious entry: "../evil.txt" (contains "..")
	//   - legitimate entry: "frp_dir/frpc.exe" (valid)
	archiveContent := buildZip(t, map[string]string{
		"../evil.txt":           "evil content",
		"frp_dir/frpc.exe":      "legit frpc content",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(archiveContent)
	}))
	defer srv.Close()

	m := New(root, discardLogger())
	m.baseURL = srv.URL
	m.goos = "windows"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st := waitForDone(t, m, "frpc", 5*time.Second)

	// The legitimate binary should be extracted successfully.
	if st.Status != StatusSuccess {
		t.Errorf("expected status=%s, got %s (error=%q)", StatusSuccess, st.Status, st.Error)
	}

	// The malicious file must NOT exist anywhere outside targetDir.
	// "../evil.txt" relative to frp_win/ would resolve to root/evil.txt.
	evilPath := filepath.Join(root, "evil.txt")
	if _, err := os.Stat(evilPath); !os.IsNotExist(err) {
		t.Errorf("malicious file should not exist at %s", evilPath)
	}

	// The legitimate binary must exist.
	expectedPath := filepath.Join(root, "frp_win", "frpc.exe")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected binary at %s: %v", expectedPath, err)
	}
}

// --- Test 6: Network timeout → StatusFailed ---

func TestDownload_NetworkTimeout_StatusFailed(t *testing.T) {
	// Server hangs indefinitely; client has a very short timeout.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			// Connection closed by client timeout.
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.baseURL = srv.URL
	m.goos = "linux"
	// Short timeout so the test completes quickly.
	m.client = &http.Client{Timeout: 50 * time.Millisecond}

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait up to 2s (timeout is 50ms, so it should fail very quickly).
	st := waitForDone(t, m, "frpc", 2*time.Second)
	if st.Status != StatusFailed {
		t.Errorf("expected status=%s after timeout, got %s", StatusFailed, st.Status)
	}
	if st.Error == "" {
		t.Error("expected non-empty error message on timeout")
	}
}

// --- itoa helper (avoids fmt import in this file) ---

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// --- Test 7: Bad kind returns ErrBadKind without starting goroutine ---

func TestDownload_BadKind(t *testing.T) {
	m := New(t.TempDir(), nil)
	err := m.Start("invalid")
	if err != ErrBadKind {
		t.Errorf("want ErrBadKind, got %v", err)
	}
}

// --- Test 8: ParseIPFromJSON parses expected JSON formats ---

func TestParseIPFromJSON(t *testing.T) {
	ip, err := ParseIPFromJSON([]byte(`{"ip":"1.2.3.4"}`))
	if err != nil || ip != "1.2.3.4" {
		t.Errorf("ParseIPFromJSON: ip=%s err=%v", ip, err)
	}
	_, err = ParseIPFromJSON([]byte(`{"ip":""}`))
	if err == nil {
		t.Error("want error for empty ip")
	}
	_, err = ParseIPFromJSON([]byte(`not json`))
	if err == nil {
		t.Error("want error for invalid json")
	}
}
