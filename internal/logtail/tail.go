// Package logtail 高效读取日志文件的末 n 行，以及增量 polling。
//
// 设计目标（02 §3.7、NF-P2）：100 MB 日志文件读末 500 行 P95 ≤ 500 ms。
// 实现策略：os.File.Seek 到末尾 → 4 KiB 缓冲反向扫描 → 计满 n 个换行即停。
// 绝不全文加载。
package logtail

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// 反向扫描缓冲大小。4 KiB 是文件系统典型 page size，性价比高。
const tailChunkSize = 4 * 1024

// TailLines 返回文件末 n 行（按文件中顺序，从旧到新）。
//
// n <= 0 时返回空切片；n 大于实际行数时返回全部行。
// 行不含行末 '\n'；最后一行若无换行符则视作完整一行。
//
// 文件不存在 → 返回 (nil, err)；调用方应判断 os.IsNotExist。
func TailLines(path string, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("logtail.TailLines open: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("logtail.TailLines stat: %w", err)
	}
	size := info.Size()
	if size == 0 {
		return []string{}, nil
	}

	// 反向扫描 —— 从文件末尾，每次读 chunk 大小的字节，向前累加到 buf。
	// 一旦 buf 中换行符数量 ≥ n+1，就能截出末 n 行（首个不完整段也包含在内，
	// 后续按行切割时再丢弃首段）。
	var (
		buf           []byte
		offset        = size
		wantedAtLeast = n
	)

	for offset > 0 {
		readSize := int64(tailChunkSize)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("logtail.TailLines seek: %w", err)
		}
		chunk := make([]byte, readSize)
		n2, err := io.ReadFull(f, chunk)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("logtail.TailLines read: %w", err)
		}
		// 把新读到的块拼到 buf 前面（保留所有已积累内容）。
		newBuf := make([]byte, 0, n2+len(buf))
		newBuf = append(newBuf, chunk[:n2]...)
		newBuf = append(newBuf, buf...)
		buf = newBuf
		// 末 n 行需要 n+1 个换行（首段不完整除外）。
		if countByte(buf, '\n') >= wantedAtLeast+1 {
			break
		}
	}

	// 按 '\n' 切。最后一段可能没有 \n（文件末尾无换行），属正常。
	lines := splitLines(buf)
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

// ReadFrom 从指定 offset 开始读到 EOF，返回 (data, newOffset, error)。
//
// 调用方持续保存 newOffset，下次再传入 —— 即可做 polling 增量。
// 文件被截断（size < offset）时：自动从头开始读，返回全部内容 + 新 offset。
func ReadFrom(path string, offset int64) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, fmt.Errorf("logtail.ReadFrom open: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, offset, fmt.Errorf("logtail.ReadFrom stat: %w", err)
	}
	size := info.Size()
	startAt := offset
	if startAt < 0 || startAt > size {
		startAt = 0
	}
	if startAt == size {
		return nil, size, nil
	}
	if _, err := f.Seek(startAt, io.SeekStart); err != nil {
		return nil, offset, fmt.Errorf("logtail.ReadFrom seek: %w", err)
	}
	// 限制单次读取最多 1 MiB，避免突发巨增导致响应体过大。
	const maxReadPerCall = 1 << 20
	want := size - startAt
	if want > maxReadPerCall {
		want = maxReadPerCall
	}
	data := make([]byte, want)
	n, err := io.ReadFull(f, data)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, offset, fmt.Errorf("logtail.ReadFrom read: %w", err)
	}
	return data[:n], startAt + int64(n), nil
}

func countByte(b []byte, c byte) int {
	n := 0
	for _, x := range b {
		if x == c {
			n++
		}
	}
	return n
}

// splitLines 按 '\n' 切；末尾如有空段（文件以 \n 结尾）忽略。
func splitLines(b []byte) []string {
	var out []string
	start := 0
	for i, x := range b {
		if x == '\n' {
			line := string(b[start:i])
			// 兼容 Windows CRLF：去掉行末 '\r'
			if l := len(line); l > 0 && line[l-1] == '\r' {
				line = line[:l-1]
			}
			out = append(out, line)
			start = i + 1
		}
	}
	if start < len(b) {
		// 末段无换行（文件未以 \n 结尾）
		line := string(b[start:])
		if l := len(line); l > 0 && line[l-1] == '\r' {
			line = line[:l-1]
		}
		out = append(out, line)
	}
	return out
}
