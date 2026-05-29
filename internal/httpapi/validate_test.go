package httpapi

// T-050 A-4：validate.go 6 个导出校验函数的 table-driven 测试。
//
// 覆盖目标：ValidateProxyName / ValidatePort / ValidateDomain / ValidateProxyType /
// ValidatePassword / ValidateUsername 的 valid + invalid 两侧分支。原有测试几乎只走
// happy path；本文件直接调函数（不经 HTTP）补齐错误路径。
//
// 反向证伪点（若实现退化将被抓到）：
//   - ProxyName：空串、超长(>64)、非法字符(空格/中文/.) 必须报错；边界 64 字符必须通过。
//   - Port：0 与 65536 必须报错；边界 1 / 65535 必须通过。
//   - Domain：空、超长(>253)、缺点号、首尾点、非法字符 必须报错；正常多级域名通过。
//   - ProxyType：仅 tcp/udp/http/https 通过；其余（含大小写变体、空串）报错。
//   - Password：<12 字符、纯字母、纯数字 报错；含字母+数字且≥12 通过。
//   - Username：空/纯空白、>32 报错；trim 后非空且 ≤32 通过。

import (
	"strings"
	"testing"
)

func TestValidateProxyName(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"simple", "web", false},
		{"with-underscore", "my_proxy_1", false},
		{"with-dash", "edge-node-2", false},
		{"max-len-64", strings.Repeat("a", 64), false},
		{"single-char", "x", false},
		// invalid
		{"empty", "", true},
		{"too-long-65", strings.Repeat("a", 65), true},
		{"space", "my proxy", true},
		{"dot", "a.b", true},
		{"chinese", "代理", true},
		{"slash", "a/b", true},
		{"at-sign", "a@b", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateProxyName(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateProxyName(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	cases := []struct {
		name    string
		in      int
		wantErr bool
	}{
		{"min-1", 1, false},
		{"typical", 7000, false},
		{"max-65535", 65535, false},
		// invalid boundaries
		{"zero", 0, true},
		{"negative", -1, true},
		{"over-max-65536", 65536, true},
		{"way-over", 100000, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePort(c.in, "bindPort")
			if (err != nil) != c.wantErr {
				t.Errorf("ValidatePort(%d) err=%v, wantErr=%v", c.in, err, c.wantErr)
			}
			// 错误信息必须包含 field 名，便于前端定位。
			if err != nil && !strings.Contains(err.Error(), "bindPort") {
				t.Errorf("ValidatePort(%d) error %q should contain field name", c.in, err.Error())
			}
		})
	}
}

func TestValidateDomain(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"two-level", "example.com", false},
		{"three-level", "a.b.example.com", false},
		{"with-dash", "my-host.example.com", false},
		// invalid
		{"empty", "", true},
		{"no-dot", "localhost", true},
		{"trailing-dot", "example.com.", true},
		{"leading-dot", ".example.com", true},
		{"tld-too-short", "example.c", true},
		{"space", "exa mple.com", true},
		{"over-253", strings.Repeat("a", 60) + "." + strings.Repeat("b", 60) + "." +
			strings.Repeat("c", 60) + "." + strings.Repeat("d", 60) + "." +
			strings.Repeat("e", 60) + ".com", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateDomain(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateDomain(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestValidateProxyType(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"tcp", "tcp", false},
		{"udp", "udp", false},
		{"http", "http", false},
		{"https", "https", false},
		// invalid
		{"empty", "", true},
		{"uppercase-TCP", "TCP", true},
		{"stcp", "stcp", true},
		{"xtcp", "xtcp", true},
		{"garbage", "garbage", true},
		{"trailing-space", "tcp ", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateProxyType(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateProxyType(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"letters-and-digits-12", "abcdef123456", false},
		{"long-mixed", "MyStr0ngPass987654", false},
		{"exactly-12-mixed", "aaaaaaaaaaa1", false},
		// invalid
		{"empty", "", true},
		{"too-short-11", "abcdef12345", true},
		{"all-letters-12", "abcdefghijkl", true},
		{"all-digits-12", "123456789012", true},
		{"long-but-no-digit", "abcdefghijklmnop", true},
		{"long-but-no-letter", "1234567890123456", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePassword(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidatePassword(len=%d) err=%v, wantErr=%v", len(c.in), err, c.wantErr)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"simple", "admin", false},
		{"trimmed-ok", "  admin  ", false},
		{"max-len-32", strings.Repeat("u", 32), false},
		{"single-char", "a", false},
		// invalid
		{"empty", "", true},
		{"only-spaces", "    ", true},
		{"too-long-33", strings.Repeat("u", 33), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateUsername(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateUsername(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			}
		})
	}
}
