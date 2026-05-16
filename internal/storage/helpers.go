package storage

import (
	"fmt"
	"strings"
	"time"
)

// parseSQLiteTime 解析 SQLite 文本时间列。
//
// 支持以下格式（按尝试顺序）：
//  1. RFC3339 / RFC3339Nano（应用层用 time.Format(time.RFC3339Nano) 写入时）。
//  2. SQLite datetime('now') 默认输出 "YYYY-MM-DD HH:MM:SS"（UTC，空格分隔）。
//
// 返回的 time.Time 总是 UTC。
func parseSQLiteTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}
	// 先试 RFC3339Nano / RFC3339
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	// SQLite datetime('now') 输出
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t.UTC(), nil
	}
	// 带小数秒的变种
	if t, err := time.Parse("2006-01-02 15:04:05.999999999", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized SQLite time format: %q", s)
}
