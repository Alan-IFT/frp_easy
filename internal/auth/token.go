package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateSessionToken 返回 32 字节 crypto/rand 随机数的 base64url（无填充）编码。
// 用作 session cookie 值（02 §3.3）。
func GenerateSessionToken() (string, error) {
	return randTokenN(32)
}

// GenerateCSRFToken 同上，独立的 32 字节随机串。用作 X-CSRF-Token 头值。
func GenerateCSRFToken() (string, error) {
	return randTokenN(32)
}

func randTokenN(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: rand read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
