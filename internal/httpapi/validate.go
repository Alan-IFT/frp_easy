package httpapi

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// 02 §4.1 / §5.2 校验规则集。

var (
	proxyNameRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	domainRE    = regexp.MustCompile(`^([A-Za-z0-9_-]{1,63}\.)+[A-Za-z]{2,}$`)
)

// ValidateProxyName 校验 proxy name（1-64，^[A-Za-z0-9_-]$）。
func ValidateProxyName(name string) error {
	if name == "" {
		return fmt.Errorf("name 必填")
	}
	if !proxyNameRE.MatchString(name) {
		return fmt.Errorf("name 只能含字母 / 数字 / 下划线 / 短横线，长度 1-64")
	}
	return nil
}

// ValidatePort 校验端口范围。
func ValidatePort(p int, field string) error {
	if p < 1 || p > 65535 {
		return fmt.Errorf("%s 必须在 1-65535", field)
	}
	return nil
}

// ValidateDomain 校验单个域名。
func ValidateDomain(d string) error {
	if d == "" {
		return fmt.Errorf("域名不能为空")
	}
	if len(d) > 253 {
		return fmt.Errorf("域名超过 253 字符")
	}
	if !domainRE.MatchString(d) {
		return fmt.Errorf("域名格式不合法: %s", d)
	}
	return nil
}

// ValidateProxyType 校验 proxy type 枚举。
func ValidateProxyType(t string) error {
	switch t {
	case "tcp", "udp", "http", "https":
		return nil
	}
	return fmt.Errorf("type 只能是 tcp/udp/http/https")
}

// ValidatePassword 校验管理员密码（≥12 字符 + 含字母 + 含数字）。
func ValidatePassword(p string) error {
	if len(p) < 12 {
		return fmt.Errorf("密码至少 12 个字符")
	}
	var hasLetter, hasDigit bool
	for _, r := range p {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return fmt.Errorf("密码必须同时含字母和数字")
	}
	return nil
}

// ValidateUsername 校验用户名（≥1 字符，可见 ASCII，长度 ≤32）。
func ValidateUsername(u string) error {
	u = strings.TrimSpace(u)
	if u == "" {
		return fmt.Errorf("用户名必填")
	}
	if len(u) > 32 {
		return fmt.Errorf("用户名过长（>32）")
	}
	return nil
}
