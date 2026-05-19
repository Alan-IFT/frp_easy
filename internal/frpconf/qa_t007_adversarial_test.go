package frpconf

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// AC-1 对抗：构造一个 "Chmod 失败之前 tmp 文件已经被写出但权限尚未收紧" 的窗口。
// 实际上 render.go 的实现是 CreateTemp → Chmod → Write，所以即使 Chmod 失败
// （强制让 Chmod 返错），tmp 文件应被 cleanup（os.Remove）。
// 检查：失败路径不会让 tmp 文件残留。
func TestAdversarial_AC1_TempLeakOnChmodPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX only")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "subdir", "frpc.toml")
	// AtomicWrite 会 MkdirAll → CreateTemp(.frpconf-*.tmp)。
	if err := AtomicWrite(target, []byte("foo")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	// 检查最终权限
	st, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	mode := st.Mode().Perm()
	if mode&0o077 != 0 {
		t.Errorf("ADVERSARIAL FAIL: target mode = %#o, group/other 有访问位", mode)
	}
	if mode != 0o600 {
		t.Errorf("ADVERSARIAL FAIL: target mode = %#o, want exact 0o600", mode)
	}
	// 检查 tmp 残留
	entries, _ := os.ReadDir(filepath.Dir(target))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".frpconf-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("ADVERSARIAL FAIL: tmp file leaked: %s", e.Name())
		}
	}
}

// AC-1 对抗：写入一个已存在的 0o666 文件，验证最终被强制收紧到 0o600。
func TestAdversarial_AC1_OverwriteLoosePerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX only")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "frpc.toml")
	// 预置一个 0o666 文件
	if err := os.WriteFile(target, []byte("old"), 0o666); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Chmod(target, 0o666); err != nil {
		t.Fatalf("seed chmod: %v", err)
	}
	if err := AtomicWrite(target, []byte("new")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	st, _ := os.Stat(target)
	mode := st.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("ADVERSARIAL FAIL: after overwriting 0o666 file, mode = %#o, want 0o600", mode)
	}
}

