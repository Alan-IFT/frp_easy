// Package auth 提供管理员凭据哈希、会话 token 生成、登录失败限流。
//
// 哈希算法选择见 02 §6.2：**argon2id**（OWASP 2023 首选；抗 GPU/ASIC）。
// 参数：m=64 MiB（65536 KiB）、t=3、p=2、salt=16B、key=32B；
// 存储格式 PHC 标准串：$argon2id$v=19$m=65536,t=3,p=2$<b64salt>$<b64hash>。
//
// 【I-1 · Gate Review §8 / F-1】
// 低配机器（< 4C / < 4G RAM）哈希耗时可能 > 300 ms，登录被刷时压力大。
// 此情况可把 argon2Memory 常量调小为 32768（32 MiB），t / p 不变。
// 重新编译后只影响"今后新建"的哈希；旧 PHC 串里的 m 参数会让 Verify
// 仍按旧参数解析，无需迁移。本期不引入运行时可调，避免增加配置面。
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2 参数（PHC 串里的 m/t/p 三元组与下面常量一一对应）。
const (
	argon2Memory      uint32 = 64 * 1024 // KiB → 64 MiB。低配机器可调为 32 * 1024（见包注释）。
	argon2Time        uint32 = 3
	argon2Parallel    uint8  = 2
	argon2SaltLen     uint32 = 16
	argon2KeyLen      uint32 = 32
	argon2Version            = argon2.Version // 当前 = 0x13 (19)
)

// HashPassword 计算 plain 的 argon2id PHC 串。
//
// 返回示例：$argon2id$v=19$m=65536,t=3,p=2$<b64salt>$<b64hash>
// （base64 使用 RawStdEncoding，与 PHC 规范一致 —— 不加 padding）。
func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", errors.New("auth.HashPassword: empty password")
	}
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth.HashPassword salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, argon2Time, argon2Memory, argon2Parallel, argon2KeyLen)
	b64salt := base64.RawStdEncoding.EncodeToString(salt)
	b64hash := base64.RawStdEncoding.EncodeToString(key)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2Version, argon2Memory, argon2Time, argon2Parallel, b64salt, b64hash), nil
}

// VerifyPassword 用 constant-time 比对验证 plain 与 encoded PHC 串。
// 返回 (true, nil) 表示通过；(false, nil) 表示密码错；(false, err) 表示 PHC 串损坏。
func VerifyPassword(plain, encoded string) (bool, error) {
	if encoded == "" {
		return false, errors.New("auth.VerifyPassword: empty encoded")
	}
	parts := strings.Split(encoded, "$")
	// 期望形如 ["", "argon2id", "v=19", "m=65536,t=3,p=2", "<salt>", "<hash>"]
	if len(parts) != 6 {
		return false, fmt.Errorf("auth.VerifyPassword: bad PHC parts=%d", len(parts))
	}
	if parts[1] != "argon2id" {
		return false, fmt.Errorf("auth.VerifyPassword: unsupported algo %q", parts[1])
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("auth.VerifyPassword: parse version: %w", err)
	}
	if version != argon2Version {
		return false, fmt.Errorf("auth.VerifyPassword: unsupported version %d", version)
	}
	var memory, time uint32
	var parallel uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &parallel); err != nil {
		return false, fmt.Errorf("auth.VerifyPassword: parse params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("auth.VerifyPassword: decode salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("auth.VerifyPassword: decode hash: %w", err)
	}
	if len(want) == 0 || len(salt) == 0 {
		return false, errors.New("auth.VerifyPassword: empty salt or hash")
	}
	if memory == 0 || time == 0 || parallel == 0 {
		return false, fmt.Errorf("auth.VerifyPassword: invalid params m=%d t=%d p=%d", memory, time, parallel)
	}
	got := argon2.IDKey([]byte(plain), salt, time, memory, parallel, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) == 1 {
		return true, nil
	}
	return false, nil
}
