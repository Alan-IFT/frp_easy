package procmgr

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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

// TestErrBusy_IsSentinel 验证 T-065 FR-1：ErrBusy 是有效哨兵——包了它的错误
// 经 errors.Is 沿 wrap 链可判（含多层包裹），而无关错误不命中。
func TestErrBusy_IsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("procmgr.Start(frpc): currently stopping: %w", ErrBusy)
	if !errors.Is(wrapped, ErrBusy) {
		t.Errorf("单层包裹应 errors.Is(ErrBusy)==true: %v", wrapped)
	}
	// 多层包裹（BC-2）仍可判。
	doubleWrapped := fmt.Errorf("outer: %w", wrapped)
	if !errors.Is(doubleWrapped, ErrBusy) {
		t.Errorf("多层包裹应 errors.Is(ErrBusy)==true: %v", doubleWrapped)
	}
	// cause 可读文本保留（进日志可诊断）。
	if msg := wrapped.Error(); !strings.Contains(msg, "currently stopping") {
		t.Errorf("wrap 后应保留可读 cause 'currently stopping'，got %q", msg)
	}
	// 无关错误不命中。
	if errors.Is(errors.New("some other failure"), ErrBusy) {
		t.Errorf("无关错误不应命中 ErrBusy")
	}
}

// TestStart_StoppingReturnsErrBusy 验证 T-065 FR-2 / AC-2：当进程处于 StateStopping
// 过渡态时调 Start，返回的错误经 errors.Is(ErrBusy) 为真（真覆盖 manager.go Start 的
// StateStopping 分支，白盒注入状态，无需真起子进程——R-2 裁定）。
func TestStart_StoppingReturnsErrBusy(t *testing.T) {
	m := New(Config{
		Locator:     &fakeLocator{frpcPath: "/some/path"},
		ConfigPaths: map[string]string{"frpc": "/tmp/frpc.toml"},
	})
	// 白盒注入 StateStopping（package 内可访问未导出字段）。
	m.mu.Lock()
	m.processes["frpc"].info.State = StateStopping
	m.mu.Unlock()

	_, err := m.Start("frpc")
	if err == nil {
		t.Fatal("StateStopping 下 Start 应返回错误")
	}
	if !errors.Is(err, ErrBusy) {
		t.Errorf("StateStopping 下 Start 错误应 errors.Is(ErrBusy)==true，got %v", err)
	}
	// 可读 cause 仍在（进日志）。
	if !strings.Contains(err.Error(), "stopping") {
		t.Errorf("错误应保留可读 cause（含 'stopping'），got %q", err.Error())
	}
}

// TestStart_NonBusyErrorsNotErrBusy 验证 FR-2 不误纳：Start 的非"忙"错误分支
// （bin missing / no config path）不得被误判为 ErrBusy（否则 handler 会错给 409）。
func TestStart_NonBusyErrorsNotErrBusy(t *testing.T) {
	// no config path 分支。
	m1 := New(Config{Locator: &fakeLocator{frpcPath: "/some/path"}})
	_, err1 := m1.Start("frpc")
	if err1 == nil {
		t.Fatal("no config path 应报错")
	}
	if errors.Is(err1, ErrBusy) {
		t.Errorf("no config path 错误不应命中 ErrBusy（应走 500），got %v", err1)
	}
	// bin missing 分支。
	m2 := New(Config{
		Locator:     &fakeLocator{frpcErr: errors.New("binloc: missing")},
		ConfigPaths: map[string]string{"frpc": "/tmp/frpc.toml"},
	})
	_, err2 := m2.Start("frpc")
	if err2 == nil {
		t.Fatal("bin missing 应报错")
	}
	if errors.Is(err2, ErrBusy) {
		t.Errorf("bin missing 错误不应命中 ErrBusy，got %v", err2)
	}
}
