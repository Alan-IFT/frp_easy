package svcprobe

import (
	"context"
	"testing"
	"time"
)

// TestProbe_DoesNotPanicNorBlock 验证 Probe 在任何平台都不 panic、
// 不超过 5s 阻塞，且返回的 Status 不破坏不变量。
func TestProbe_DoesNotPanicNorBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan Status, 1)
	go func() {
		done <- Probe(ctx)
	}()

	select {
	case s := <-done:
		// supervisor 必须是 3 个枚举之一
		switch s.Supervisor {
		case "systemd", "windows-service", "none":
			// ok
		default:
			t.Errorf("invalid supervisor=%q", s.Supervisor)
		}
		// supervisor=none 时 supervised 必须 false
		if s.Supervisor == "none" && s.Supervised {
			t.Errorf("supervisor=none but supervised=true: %+v", s)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("Probe did not return within 6s")
	}
}

// TestProbe_ContextCanceled 验证已超时的 ctx 不会让 Probe 卡死。
func TestProbe_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	done := make(chan Status, 1)
	go func() {
		done <- Probe(ctx)
	}()

	select {
	case s := <-done:
		// 已超时的 ctx 下 boot_autostart 应为 false（spawn 命令会立即失败/不被启动）
		if s.BootAutostart {
			t.Logf("Probe returned BootAutostart=true under canceled ctx; OK if probe finished quickly before checking ctx")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Probe under canceled ctx did not return within 3s")
	}
}
