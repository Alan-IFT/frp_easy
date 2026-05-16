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
