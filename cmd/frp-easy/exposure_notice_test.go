package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestExposureNoticeText 锁定提示文案三要素（局域网可达事实、引导 setup、改回仅本机操作）。
func TestExposureNoticeText(t *testing.T) {
	got := exposureNotice(7800, "frp_easy.toml")
	wantFragments := []string{
		"局域网/公网内的设备均可访问",
		"完成 setup 向导",
		"argon2id",
		`UIBindAddr 改为 "127.0.0.1"`,
		"7800",
		"frp_easy.toml",
	}
	for _, frag := range wantFragments {
		if !strings.Contains(got, frag) {
			t.Errorf("exposureNotice 缺少片段 %q，实际输出：\n%s", frag, got)
		}
	}
}

// TestExposureNoticeReachesLogger 验证 T-022 修复后 ui exposure notice 走 logger，
// 让 Windows Service / systemd 服务模式（stderr 被宿主丢弃）也能从 ui.log 拿到提示。
func TestExposureNoticeReachesLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	notice := exposureNotice(7800, "frp_easy.toml")
	logger.Warn("ui exposure notice",
		"addr", "0.0.0.0",
		"port", 7800,
		"config_path", "frp_easy.toml",
		"message", notice)

	out := buf.String()
	wantInLog := []string{
		`"level":"WARN"`,
		`"msg":"ui exposure notice"`,
		`"addr":"0.0.0.0"`,
		`"port":7800`,
		`"config_path":"frp_easy.toml"`,
		"局域网/公网内的设备均可访问",
		"完成 setup 向导",
	}
	for _, frag := range wantInLog {
		if !strings.Contains(out, frag) {
			t.Errorf("logger 输出缺少片段 %q，实际：\n%s", frag, out)
		}
	}
}
