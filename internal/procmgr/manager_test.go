package procmgr

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// fakeLocator 不返回任何 binary。
type fakeLocator struct {
	frpcPath string
	frpsPath string
	frpcErr  error
	frpsErr  error
}

func (f *fakeLocator) FRPCPath() (string, error) { return f.frpcPath, f.frpcErr }
func (f *fakeLocator) FRPSPath() (string, error) { return f.frpsPath, f.frpsErr }
func (f *fakeLocator) Missing() []string {
	var m []string
	if f.frpcErr != nil {
		m = append(m, "frpc")
	}
	if f.frpsErr != nil {
		m = append(m, "frps")
	}
	return m
}

type fakeReloader struct {
	calls int
	err   error
}

func (f *fakeReloader) Reload(ctx context.Context, strict bool) error {
	f.calls++
	return f.err
}

func TestStart_BinMissing(t *testing.T) {
	loc := &fakeLocator{frpcErr: errors.New("binloc: missing")}
	m := New(Config{
		Locator:     loc,
		ConfigPaths: map[string]string{"frpc": "/tmp/frpc.toml", "frps": "/tmp/frps.toml"},
		LogFiles:    map[string]string{"frpc": filepath.Join(t.TempDir(), "frpc.log")},
	})
	_, err := m.Start("frpc")
	if err == nil {
		t.Fatal("expected error when binary missing")
	}
	info := m.Status("frpc")
	if info.State != StateStopped {
		t.Errorf("state should remain stopped on bin missing, got %v", info.State)
	}
}

func TestStart_NoConfigPath(t *testing.T) {
	loc := &fakeLocator{frpcPath: "/some/path"}
	m := New(Config{Locator: loc})
	_, err := m.Start("frpc")
	if err == nil {
		t.Fatal("expected error when no config path")
	}
}

func TestStatus_DefaultStopped(t *testing.T) {
	m := New(Config{Locator: &fakeLocator{}})
	if s := m.Status("frpc").State; s != StateStopped {
		t.Errorf("default state: %s", s)
	}
	if s := m.Status("frps").State; s != StateStopped {
		t.Errorf("default state frps: %s", s)
	}
	all := m.StatusAll()
	if len(all) != 2 {
		t.Errorf("StatusAll len = %d", len(all))
	}
}

func TestInvalidKind(t *testing.T) {
	m := New(Config{Locator: &fakeLocator{}})
	if _, err := m.Start("xtcp"); err == nil {
		t.Error("expected error for invalid kind")
	}
	if _, err := m.Stop("xtcp"); err == nil {
		t.Error("expected error for invalid kind")
	}
	if err := m.ApplyConfigChange("xtcp"); err == nil {
		t.Error("expected error for invalid kind")
	}
}

func TestApplyConfigChange_NotRunning(t *testing.T) {
	// 未运行状态下 ApplyConfigChange 应直接 no-op + nil。
	m := New(Config{Locator: &fakeLocator{}})
	if err := m.ApplyConfigChange("frpc"); err != nil {
		t.Errorf("expected no-op nil, got %v", err)
	}
}

func TestStop_Idempotent(t *testing.T) {
	m := New(Config{Locator: &fakeLocator{}})
	if _, err := m.Stop("frpc"); err != nil {
		t.Errorf("first stop: %v", err)
	}
	if _, err := m.Stop("frpc"); err != nil {
		t.Errorf("second stop: %v", err)
	}
}
