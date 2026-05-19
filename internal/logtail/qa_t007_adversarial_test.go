package logtail

import (
	"os"
	"path/filepath"
	"testing"
)

// AC-4 对抗：构造 offset 在 5 MiB 文件中部、然后越界、然后负数 — 边界行为。
func TestAdversarial_AC4_OffsetBoundaries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	const sz = 5 * 1024 * 1024
	data := make([]byte, sz)
	for i := range data {
		data[i] = 'A'
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// 越界 offset
	d, next, err := ReadFrom(path, sz+1000)
	if err != nil {
		t.Errorf("ADVERSARIAL: offset > size 不应报错（按设计自动从头读），got: %v", err)
	}
	// 当 offset > size 时，实现 startAt = 0，应读全 5 MiB（但上限 2 MiB） → 2 MiB
	if len(d) != MaxReadBytes {
		t.Errorf("ADVERSARIAL: offset > size 后从头读返回 %d 字节，want %d", len(d), MaxReadBytes)
	}
	_ = next

	// 负数 offset
	d, _, err = ReadFrom(path, -100)
	if err != nil {
		t.Errorf("ADVERSARIAL: negative offset 不应报错，got: %v", err)
	}
	if len(d) != MaxReadBytes {
		t.Errorf("ADVERSARIAL: negative offset 从头读返回 %d 字节，want %d", len(d), MaxReadBytes)
	}

	// 恰好 MaxReadBytes 边界
	d, next, err = ReadFrom(path, int64(MaxReadBytes))
	if err != nil {
		t.Errorf("ADVERSARIAL: offset = MaxReadBytes 失败: %v", err)
	}
	if len(d) != MaxReadBytes {
		t.Errorf("ADVERSARIAL: offset=2MiB on 5MiB file returned %d, want 2MiB", len(d))
	}
	if next != int64(2*MaxReadBytes) {
		t.Errorf("ADVERSARIAL: next = %d, want %d", next, 2*MaxReadBytes)
	}

	// 全读完
	d, next, err = ReadFrom(path, sz)
	if err != nil {
		t.Fatalf("EOF: %v", err)
	}
	if len(d) != 0 || next != sz {
		t.Errorf("ADVERSARIAL: at EOF, data=%d next=%d, want 0 / %d", len(d), next, sz)
	}
}

// AC-4 对抗：MaxReadBytes 精确值。验证常量值 == 2 * 1024 * 1024（防止有人改成 2 * 1000 * 1000）
func TestAdversarial_AC4_MaxReadBytesExact(t *testing.T) {
	want := 2 * 1024 * 1024
	if MaxReadBytes != want {
		t.Errorf("ADVERSARIAL FAIL: MaxReadBytes = %d, want %d (2 * 1024 * 1024)", MaxReadBytes, want)
	}
	// 也对应 2 << 20
	if MaxReadBytes != 2<<20 {
		t.Errorf("ADVERSARIAL FAIL: MaxReadBytes != 2 << 20")
	}
}

// AC-4 对抗：文件被截断（offset 大于 size），实现是把 startAt 设为 0 重读。
// 这是与 02 设计契合的"自动从头读"行为，但**不在 AC-4.2 的"文件不存在/offset超过文件大小时空响应"
// 范畴**。审视行为差异：
//
// 实现 (tail.go:112-114):
//   if startAt < 0 || startAt > size { startAt = 0 }
//
// AC-4.2 文字："offset 超过文件大小时行为与现状一致（空响应）"
//
// 但 sz=5MiB，offset=10MiB：startAt=0 → 读 2 MiB（非空）。
// 这与 AC-4.2 "空响应" 描述**不一致**！实际是从头读 2 MiB。
//
// 这是一个潜在的 AC 描述与实现脱节问题。需要在 report 中标记 MINOR。
func TestAdversarial_AC4_OffsetExceedsSizeReadsFromZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.log")
	if err := os.WriteFile(path, []byte("hello world"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	d, next, err := ReadFrom(path, 999999)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// 实现是从头读（不是空响应）
	if len(d) != len("hello world") {
		t.Errorf("INFO: offset far exceeds size, impl reads from head; len = %d (diff vs AC-4.2 expected empty)", len(d))
	}
	_ = next
}

// AC-4 对抗：1MiB 文件（小于 2MiB）单次返回全部，不应分片
func TestAdversarial_AC4_SmallFileSingleShot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1m.log")
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = 'X'
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	d, next, err := ReadFrom(path, 0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(d) != 1024*1024 {
		t.Errorf("ADVERSARIAL: 1 MiB file should return all in single shot, got %d", len(d))
	}
	if next != 1024*1024 {
		t.Errorf("ADVERSARIAL: next = %d, want 1 MiB", next)
	}
}
