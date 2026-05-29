package svcprobe

// T-050 A-2：平台无关的纯逻辑覆盖。
//
// 覆盖 parseIsEnabled —— `systemctl is-enabled` stdout 解析。此前该判定内联在
// probe_linux.go，在非 Linux 平台（含本仓库 CI 的 Windows）零断言。抽成纯函数后
// 此测试在任何平台都能跑。
//
// 反向证伪点：
//   - "enabled\n"（带换行，systemctl 真实输出）必须 true（证明有 TrimSpace）。
//   - "disabled" / "static" / "masked" / "enabled-runtime" / 空 / 命令失败空输出 必须 false。
//   - "enabledx" / " enabled foo" 这类近似串必须 false（证明是精确等于而非前缀/包含）。

import "testing"

func TestParseIsEnabled(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"enabled-plain", "enabled", true},
		{"enabled-newline", "enabled\n", true},
		{"enabled-crlf", "enabled\r\n", true},
		{"enabled-surrounding-spaces", "  enabled  ", true},
		// not enabled
		{"disabled", "disabled\n", false},
		{"static", "static\n", false},
		{"masked", "masked\n", false},
		{"linked", "linked\n", false},
		{"enabled-runtime", "enabled-runtime\n", false},
		{"empty", "", false},
		{"only-whitespace", "  \n", false},
		{"prefix-only", "enabledx", false},
		{"contains-not-equal", "is enabled now", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseIsEnabled([]byte(c.out)); got != c.want {
				t.Errorf("parseIsEnabled(%q)=%v want %v", c.out, got, c.want)
			}
		})
	}
}
