package logrotate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNew_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.log")
	w, err := New(Options{Path: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	// 权限断言只在 POSIX 主机做（Windows 仅 read-only bit）
	if runtime.GOOS != "windows" {
		if got := fi.Mode().Perm(); got != 0o600 {
			t.Errorf("perm = %o, want 0o600", got)
		}
	}
}

func TestNew_RotatesOnSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.log")
	// 1 MB 上限 + 写 ~1.2 MB 触发轮转
	w, err := New(Options{Path: path, MaxSizeMB: 1, MaxBackups: 2, MaxAgeDays: 1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	chunk := strings.Repeat("x", 1024) + "\n" // 1 KB
	for i := 0; i < 1300; i++ {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write @ %d: %v", i, err)
		}
	}
	// 关闭 writer 让最后一次 rotate 落地、释放文件句柄（Windows 上 ReadDir 不
	// 受锁限制但断言权限的 Stat 需要文件被释放）。
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	logFiles := []os.DirEntry{}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "ui") && strings.Contains(e.Name(), "log") {
			logFiles = append(logFiles, e)
		}
	}
	if len(logFiles) < 2 {
		t.Errorf("expected >=2 log files after rotation, got %d (entries: %v)", len(logFiles), entries)
	}
	// AC-3 显式要求：轮转产物权限保持 0o600
	// （lumberjack 的 openNew 实际拷贝 OLD 文件 mode；此断言锁定未来 lumberjack
	// 升级时该行为不被破坏）。
	if runtime.GOOS != "windows" {
		for _, e := range logFiles {
			fi, err := os.Stat(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Errorf("Stat %s: %v", e.Name(), err)
				continue
			}
			if got := fi.Mode().Perm(); got != 0o600 {
				t.Errorf("rotated file %s perm = %o, want 0o600", e.Name(), got)
			}
		}
	}
}

func TestLoadOptionsFromEnv_OverridesDefaults(t *testing.T) {
	t.Setenv("FRP_EASY_LOG_MAX_SIZE_MB", "42")
	t.Setenv("FRP_EASY_LOG_MAX_BACKUPS", "7")
	t.Setenv("FRP_EASY_LOG_MAX_AGE_DAYS", "14")
	opts := LoadOptionsFromEnv("/tmp/ui.log")
	if opts.MaxSizeMB != 42 || opts.MaxBackups != 7 || opts.MaxAgeDays != 14 {
		t.Errorf("got %+v, want size=42 backups=7 age=14", opts)
	}
}

func TestLoadOptionsFromEnv_IgnoresInvalid(t *testing.T) {
	t.Setenv("FRP_EASY_LOG_MAX_SIZE_MB", "not-a-number")
	t.Setenv("FRP_EASY_LOG_MAX_BACKUPS", "-1") // 负数
	t.Setenv("FRP_EASY_LOG_MAX_AGE_DAYS", "")  // 空
	opts := LoadOptionsFromEnv("/tmp/ui.log")
	// 全部 fallback 到零值（让 New 填默认）
	if opts.MaxSizeMB != 0 || opts.MaxAgeDays != 0 {
		t.Errorf("invalid env should fall back to zero, got %+v", opts)
	}
	// MaxBackups = -1 被拒（条件 n >= 0），返回零值
	if opts.MaxBackups != 0 {
		t.Errorf("negative backups should be rejected, got %d", opts.MaxBackups)
	}
}
