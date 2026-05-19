// Package logrotate 提供基于 lumberjack 的 ui.log 大小 / 数量 / 时间三轴轮转。
//
// 设计动机（T-010）：原 main.go 直接 os.OpenFile + O_APPEND 写 ui.log，长跑
// systemd / Windows Service 场景下日志无限增长会爆盘。lumberjack 是 Go 生态
// 实际事实标准（k8s / Prometheus / etcd 均在用），MIT，纯 Go 无 cgo。
//
// 关键不变量：
//   - 文件权限恒为 0o600（与 T-007 ui.log 收紧后的策略一致；轮转产生的历史份
//     也是 0o600，由 lumberjack 在 chown/chmod 上自身保证）。
//   - 不压缩（Compress=false）：gzip 历史使运维 grep 多一步 zcat / zgrep，违
//     背"长期易维护"原则。空间相比 ui.log 体量可忽略。
//   - 默认值（10 MB × 5 份 × 30 天）按 frp_easy 体量 / 部署面对的家用/小企业
//     运维场景估算：实测日 INFO 量级 ~100 KB，10 MB 约 100 天滚动；5 份 × 30
//     天的边界由 lumberjack 取交集，先到先丢。
package logrotate

import (
	"io"
	"os"
	"strconv"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Options 是轮转参数。零值由 New 填充默认值。
type Options struct {
	Path       string      // ui.log 完整路径，必填
	MaxSizeMB  int         // 单文件上限 MB（默认 10）
	MaxBackups int         // 历史份数（默认 5）
	MaxAgeDays int         // 最长保留天数（默认 30）
	Mode       os.FileMode // 文件权限（默认 0o600）
}

// New 返回轮转 writer；调用方负责 Close（main.go 用 defer）。
//
// 行为：
//  1. 提前 OpenFile + Chmod，让首次写之前权限已经是 opts.Mode（lumberjack
//     在文件已存在时不会重置权限，需要调用方预置）。
//  2. 返回 *lumberjack.Logger 包装为 io.WriteCloser。
//  3. 任何 OS 错误（mkdir 父目录失败、chmod 失败）都返回，main.go 决定是否
//     降级到 stderr-only。
func New(opts Options) (io.WriteCloser, error) {
	if opts.MaxSizeMB == 0 {
		opts.MaxSizeMB = 10
	}
	if opts.MaxBackups == 0 {
		opts.MaxBackups = 5
	}
	if opts.MaxAgeDays == 0 {
		opts.MaxAgeDays = 30
	}
	if opts.Mode == 0 {
		opts.Mode = 0o600
	}
	f, err := os.OpenFile(opts.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, opts.Mode)
	if err != nil {
		return nil, err
	}
	if cerr := f.Close(); cerr != nil {
		return nil, cerr
	}
	if cerr := os.Chmod(opts.Path, opts.Mode); cerr != nil {
		// Windows 上 chmod 仅影响 read-only bit；非 fatal，但仍 surfacing。
		// 主流程会 WARN 不 abort。
		return nil, cerr
	}
	return &lumberjack.Logger{
		Filename:   opts.Path,
		MaxSize:    opts.MaxSizeMB,
		MaxBackups: opts.MaxBackups,
		MaxAge:     opts.MaxAgeDays,
		Compress:   false,
	}, nil
}

// LoadOptionsFromEnv 用 FRP_EASY_LOG_MAX_* 环境变量覆盖默认。
// 任何解析失败（非数字 / 负数）静默 fallback 到默认，避免环境变量 typo 阻塞启动。
func LoadOptionsFromEnv(path string) Options {
	opts := Options{Path: path}
	if v := os.Getenv("FRP_EASY_LOG_MAX_SIZE_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			opts.MaxSizeMB = n
		}
	}
	if v := os.Getenv("FRP_EASY_LOG_MAX_BACKUPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			opts.MaxBackups = n
		}
	}
	if v := os.Getenv("FRP_EASY_LOG_MAX_AGE_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			opts.MaxAgeDays = n
		}
	}
	return opts
}
