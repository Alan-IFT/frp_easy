package logtail

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLines(t *testing.T, lines []string, trailingNewline bool) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "log.txt")
	body := strings.Join(lines, "\n")
	if trailingNewline {
		body += "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTailLines_SmallFile(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	path := writeLines(t, lines, true)

	got, err := TailLines(path, 3)
	if err != nil {
		t.Fatalf("TailLines: %v", err)
	}
	want := []string{"c", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("len: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestTailLines_FewerThanRequested(t *testing.T) {
	lines := []string{"a", "b"}
	path := writeLines(t, lines, true)
	got, err := TailLines(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v", got)
	}
}

func TestTailLines_NoTrailingNewline(t *testing.T) {
	lines := []string{"alpha", "beta", "gamma"}
	path := writeLines(t, lines, false)
	got, err := TailLines(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "beta" || got[1] != "gamma" {
		t.Errorf("got %v", got)
	}
}

func TestTailLines_NonExistent(t *testing.T) {
	_, err := TailLines(filepath.Join(t.TempDir(), "missing.log"), 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTailLines_ZeroN(t *testing.T) {
	path := writeLines(t, []string{"a", "b"}, true)
	got, err := TailLines(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestTailLines_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// 写 ~1 MiB（10000 行，每行约 100 字节），验证反向扫描跨多 chunk。
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(f, "line-%05d: %s\n", i, strings.Repeat("x", 80))
	}
	f.Close()

	got, err := TailLines(path, 500)
	if err != nil {
		t.Fatalf("TailLines: %v", err)
	}
	if len(got) != 500 {
		t.Fatalf("want 500 got %d", len(got))
	}
	if !strings.HasPrefix(got[0], "line-09500:") {
		t.Errorf("first tail line: %q", got[0])
	}
	if !strings.HasPrefix(got[499], "line-09999:") {
		t.Errorf("last tail line: %q", got[499])
	}
}

func TestTailLines_CRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "win.log")
	if err := os.WriteFile(path, []byte("a\r\nb\r\nc\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := TailLines(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Errorf("got %v", got)
	}
}

func TestReadFrom_Incremental(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stream.log")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, off1, err := ReadFrom(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello\n" || off1 != 6 {
		t.Errorf("first: data=%q off=%d", data, off1)
	}

	// 追加一段，再增量读
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("world\n")
	_ = f.Close()

	data, off2, err := ReadFrom(path, off1)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "world\n" {
		t.Errorf("incremental data: %q", data)
	}
	if off2 != off1+6 {
		t.Errorf("offset: %d -> %d", off1, off2)
	}

	// 再读一次（无新内容） → 空 data，offset 不变
	data, off3, err := ReadFrom(path, off2)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 || off3 != off2 {
		t.Errorf("idle: data=%q off=%d", data, off3)
	}
}

func TestReadFrom_TruncatedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trunc.log")
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 调用方记录的 offset 在 truncate 后 > 文件实际 size
	data, off, err := ReadFrom(path, 999)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "12345" || off != 5 {
		t.Errorf("data=%q off=%d", data, off)
	}
}

// TestReadFromCapsAt2MiB 验证 AC-4.1：5 MiB 文件通过 3 次轮询切片，
// 每次最多 MaxReadBytes (2 MiB)。
func TestReadFromCapsAt2MiB(t *testing.T) {
	const total = 5 * 1024 * 1024 // 5 MiB
	if MaxReadBytes != 2*1024*1024 {
		t.Fatalf("MaxReadBytes = %d, expected 2 MiB (=%d)", MaxReadBytes, 2*1024*1024)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "big.log")
	payload := make([]byte, total)
	for i := range payload {
		payload[i] = byte('a' + (i % 26))
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	// 第 1 次：offset=0 → len=2 MiB, next=2 MiB
	data, next, err := ReadFrom(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != MaxReadBytes {
		t.Fatalf("first chunk len = %d, want %d", len(data), MaxReadBytes)
	}
	if next != int64(MaxReadBytes) {
		t.Errorf("first next = %d, want %d", next, MaxReadBytes)
	}

	// 第 2 次：offset=2 MiB → len=2 MiB, next=4 MiB
	data, next, err = ReadFrom(path, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != MaxReadBytes {
		t.Fatalf("second chunk len = %d, want %d", len(data), MaxReadBytes)
	}
	if next != int64(2*MaxReadBytes) {
		t.Errorf("second next = %d, want %d", next, 2*MaxReadBytes)
	}

	// 第 3 次：offset=4 MiB → len=1 MiB（剩余）, next=5 MiB
	data, next, err = ReadFrom(path, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1024*1024 {
		t.Errorf("third chunk len = %d, want %d", len(data), 1024*1024)
	}
	if next != int64(total) {
		t.Errorf("third next = %d, want %d", next, total)
	}

	// 第 4 次：offset==size → 空响应，next 不变
	data, next, err = ReadFrom(path, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Errorf("after EOF data should be empty, got %d bytes", len(data))
	}
	if next != int64(total) {
		t.Errorf("after EOF next = %d, want %d", next, total)
	}
}

// TestReadFrom_SmallFileNoSplit 验证 AC-4.2：< MaxReadBytes 时一次返回全部。
func TestReadFrom_SmallFileNoSplit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.log")
	payload := make([]byte, 100*1024) // 100 KiB
	for i := range payload {
		payload[i] = 'x'
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	data, next, err := ReadFrom(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(payload) {
		t.Errorf("small file data len = %d, want %d (single-shot)", len(data), len(payload))
	}
	if next != int64(len(payload)) {
		t.Errorf("small file next = %d, want %d", next, len(payload))
	}
}
