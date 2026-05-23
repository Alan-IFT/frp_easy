package downloader

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestInstall_HappyPath：写 1 KiB 内容，验证 sha256 一致、目标文件存在、Linux 下 mode = 0o755。
func TestInstall_HappyPath(t *testing.T) {
	content := bytes.Repeat([]byte("a"), 1024)
	// sha256("a"*1024) = 2edc986847e209b4016e141a6dc8716d3207350f416969382d431539bf292e4a
	wantSHA := "2edc986847e209b4016e141a6dc8716d3207350f416969382d431539bf292e4a"

	root := t.TempDir()
	m := New(root, discardLogger())
	m.goos = "linux" // 固定测 Linux，避免主机 GOOS 影响

	sha, written, finalPath, err := m.Install("frpc", bytes.NewReader(content), 64<<20)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if sha != wantSHA {
		t.Errorf("sha = %q, want %q", sha, wantSHA)
	}
	if written != 1024 {
		t.Errorf("written = %d, want 1024", written)
	}
	wantPath := filepath.Join(root, "frp_linux", "frpc")
	if finalPath != wantPath {
		t.Errorf("finalPath = %q, want %q", finalPath, wantPath)
	}
	info, statErr := os.Stat(finalPath)
	if statErr != nil {
		t.Fatalf("Stat: %v", statErr)
	}
	if info.Size() != 1024 {
		t.Errorf("file size = %d, want 1024", info.Size())
	}
	if runtime.GOOS != "windows" {
		// Linux runner：检查 0o755（Windows runner 上文件 mode 概念不同，跳过断言）
		mode := info.Mode().Perm()
		if mode != 0o755 {
			t.Errorf("mode = %o, want 0o755", mode)
		}
	}
}

// TestInstall_TooLarge：写 maxBytes+1 字节 → ErrFileTooLarge，目标文件不存在。
func TestInstall_TooLarge(t *testing.T) {
	maxBytes := int64(1024)
	content := bytes.Repeat([]byte("x"), int(maxBytes)+1)

	root := t.TempDir()
	m := New(root, discardLogger())
	m.goos = "linux"

	_, _, _, err := m.Install("frpc", bytes.NewReader(content), maxBytes)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("err = %v, want ErrFileTooLarge", err)
	}

	// 目标文件不应被写入
	wantPath := filepath.Join(root, "frp_linux", "frpc")
	if _, statErr := os.Stat(wantPath); !os.IsNotExist(statErr) {
		t.Errorf("target should NOT exist on size violation, got stat err: %v", statErr)
	}

	// 临时文件也应被清理（targetDir 下不应有 .install-*.tmp 残留）
	entries, _ := os.ReadDir(filepath.Join(root, "frp_linux"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".install-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

// TestInstall_AtMaxBytes：写恰好 maxBytes → 成功（边界含）。
func TestInstall_AtMaxBytes(t *testing.T) {
	maxBytes := int64(1024)
	content := bytes.Repeat([]byte("y"), int(maxBytes))

	root := t.TempDir()
	m := New(root, discardLogger())
	m.goos = "linux"

	_, written, _, err := m.Install("frpc", bytes.NewReader(content), maxBytes)
	if err != nil {
		t.Fatalf("Install at maxBytes should succeed: %v", err)
	}
	if written != maxBytes {
		t.Errorf("written = %d, want %d", written, maxBytes)
	}
}

// TestInstall_MaxBytesUnlimited：maxBytes <= 0 表示不限大小（下载链路）。
func TestInstall_MaxBytesUnlimited(t *testing.T) {
	content := bytes.Repeat([]byte("z"), 32<<10) // 32 KiB
	root := t.TempDir()
	m := New(root, discardLogger())
	m.goos = "linux"

	_, written, _, err := m.Install("frpc", bytes.NewReader(content), -1)
	if err != nil {
		t.Fatalf("Install unlimited should succeed: %v", err)
	}
	if int(written) != len(content) {
		t.Errorf("written = %d, want %d", written, len(content))
	}
}

// TestInstall_BadKind：kind 必须 frpc / frps。
func TestInstall_BadKind(t *testing.T) {
	root := t.TempDir()
	m := New(root, discardLogger())
	m.goos = "linux"
	_, _, _, err := m.Install("xxx", bytes.NewReader([]byte("abc")), 1024)
	if !errors.Is(err, ErrBadKind) {
		t.Errorf("err = %v, want ErrBadKind", err)
	}
}

// TestInstall_UnsupportedGOOS：不支持的 OS → ErrUnsupportedOS。
func TestInstall_UnsupportedGOOS(t *testing.T) {
	root := t.TempDir()
	m := New(root, discardLogger())
	m.goos = "freebsd"
	_, _, _, err := m.Install("frpc", bytes.NewReader([]byte("abc")), 1024)
	if !errors.Is(err, ErrUnsupportedOS) {
		t.Errorf("err = %v, want ErrUnsupportedOS", err)
	}
}

// TestInstall_WindowsFallback：在 Linux runner 上注入 m.goos="windows"，预先 WriteFile
// 占住目标路径，验证 Install 仍走 Remove+Rename 成功落盘。
//
// 注意：m.goos 的 seam 让 Install 走 windows 分支；实际 os.Rename 在 Linux 上对已存在
// 文件**会成功覆盖**（Linux Rename 是 atomic replace），所以在 Linux runner 下第一次
// rename 就会成功，不会进入 fallback 分支。要真正测试 fallback 需要 Windows runner。
// 这里的断言聚焦"无论 GOOS 注入是 windows 还是 linux，预存在的旧文件最终被覆盖、
// 新文件内容正确"——这是 fallback 路径的最终可观察行为契约。
func TestInstall_WindowsFallback_OverwriteExisting(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "frp_win")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(targetDir, "frpc.exe")
	// 预存在的旧文件
	if err := os.WriteFile(targetPath, []byte("OLD-CONTENT"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(root, discardLogger())
	m.goos = "windows"

	newContent := []byte("NEW-CONTENT-MZ-header")
	_, _, _, err := m.Install("frpc", bytes.NewReader(newContent), 64<<20)
	if err != nil {
		t.Fatalf("Install overwriting existing: %v", err)
	}

	got, _ := os.ReadFile(targetPath)
	if string(got) != string(newContent) {
		t.Errorf("file content = %q, want %q", got, newContent)
	}
}

// TestInstall_ReaderError：上游 Reader 提前出错 → Install 返回 wrapped error。
func TestInstall_ReaderError(t *testing.T) {
	root := t.TempDir()
	m := New(root, discardLogger())
	m.goos = "linux"

	failingReader := io.MultiReader(
		bytes.NewReader([]byte("partial-")),
		errReader{err: errors.New("synthetic-read-fail")},
	)
	_, _, _, err := m.Install("frpc", failingReader, 64<<20)
	if err == nil {
		t.Fatal("expected error from failing reader")
	}
	if !strings.Contains(err.Error(), "写入失败") {
		t.Errorf("err = %v, want '写入失败' wrap", err)
	}
	// 临时文件应被清理
	entries, _ := os.ReadDir(filepath.Join(root, "frp_linux"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".install-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

// errReader 总是返回预设错误。
type errReader struct {
	err error
}

func (r errReader) Read(p []byte) (int, error) {
	return 0, r.err
}
