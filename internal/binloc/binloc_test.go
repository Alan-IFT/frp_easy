package binloc

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// 构造一个临时仓库根：含 frp_win/ 或 frp_linux/ 与对应的二进制文件。
func setupRoot(t *testing.T, withFrpc, withFrps bool) string {
	t.Helper()
	root := t.TempDir()
	var dir, frpcName, frpsName string
	switch runtime.GOOS {
	case "windows":
		dir = filepath.Join(root, "frp_win")
		frpcName, frpsName = "frpc.exe", "frps.exe"
	case "linux":
		dir = filepath.Join(root, "frp_linux")
		frpcName, frpsName = "frpc", "frps"
	default:
		t.Skipf("unsupported platform %s", runtime.GOOS)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if withFrpc {
		if err := os.WriteFile(filepath.Join(dir, frpcName), []byte("fake"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if withFrps {
		if err := os.WriteFile(filepath.Join(dir, frpsName), []byte("fake"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLocator_BothPresent(t *testing.T) {
	root := setupRoot(t, true, true)
	loc := NewDefault(root)

	if p, err := loc.FRPCPath(); err != nil {
		t.Errorf("FRPCPath: %v", err)
	} else if filepath.Dir(p) != filepath.Join(root, platformDir()) {
		t.Errorf("unexpected frpc dir: %s", p)
	}
	if _, err := loc.FRPSPath(); err != nil {
		t.Errorf("FRPSPath: %v", err)
	}
	if m := loc.Missing(); len(m) != 0 {
		t.Errorf("Missing: want empty, got %v", m)
	}
	if loc.Root() != root {
		t.Errorf("Root: got %s want %s", loc.Root(), root)
	}
}

func TestLocator_MissingFrpc(t *testing.T) {
	root := setupRoot(t, false, true)
	loc := NewDefault(root)

	if _, err := loc.FRPCPath(); err == nil {
		t.Error("expected ErrBinMissing for frpc")
	}
	if _, err := loc.FRPSPath(); err != nil {
		t.Errorf("FRPSPath should succeed: %v", err)
	}
	m := loc.Missing()
	if len(m) != 1 || m[0] != KindFrpc {
		t.Errorf("Missing: got %v", m)
	}
}

func TestLocator_BothMissing(t *testing.T) {
	root := t.TempDir() // 完全空目录
	loc := NewDefault(root)
	m := loc.Missing()
	if len(m) != 2 {
		t.Errorf("expected 2 missing, got %v", m)
	}
	// 字母序：frpc < frps
	if m[0] != KindFrpc || m[1] != KindFrps {
		t.Errorf("expected sorted, got %v", m)
	}
}

func TestLocator_HintWinsOverEnv(t *testing.T) {
	// 把环境变量指向不存在的目录；hint 给真实临时目录。
	t.Setenv("FRP_EASY_ROOT", "C:\\definitely\\does\\not\\exist\\frpeasy-test")
	root := setupRoot(t, true, true)
	loc := NewDefault(root)
	if loc.Root() != root {
		t.Errorf("hint should override env: got %s want %s", loc.Root(), root)
	}
}

func TestLocator_EnvUsedWhenHintEmpty(t *testing.T) {
	root := setupRoot(t, true, true)
	t.Setenv("FRP_EASY_ROOT", root)
	loc := NewDefault("")
	// Root 可能被 Abs 规范化，比较绝对值。
	absExpected, _ := filepath.Abs(root)
	if loc.Root() != absExpected {
		t.Errorf("env should be used: got %s want %s", loc.Root(), absExpected)
	}
}

func platformDir() string {
	switch runtime.GOOS {
	case "windows":
		return "frp_win"
	case "linux":
		return "frp_linux"
	default:
		return ""
	}
}
