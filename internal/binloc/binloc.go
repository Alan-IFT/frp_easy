// Package binloc 负责按 runtime.GOOS 定位仓库 vendored 的 frpc / frps 二进制。
//
// 设计契约见 02 §3.8、§6.5：
//   - windows → frp_win/frpc.exe、frp_win/frps.exe
//   - linux   → frp_linux/frpc、frp_linux/frps
//   - 其他 OS → 全部缺失
//
// repoRoot 优先级（高 → 低）：
//  1. 环境变量 FRP_EASY_ROOT
//  2. os.Executable() 所在目录（生产部署 = bin/frp-easy[.exe] 与 frp_win/ 同级）
//  3. os.Getwd()（开发期 / go run / go test）
package binloc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// Kind 列出 binloc 关心的进程种类。
const (
	KindFrpc = "frpc"
	KindFrps = "frps"
)

// Locator 抽象出二进制路径解析与缺失检测。
type Locator interface {
	// FRPCPath 返回当前平台 frpc 可执行文件绝对路径。
	// 文件不存在或平台不支持 → 返回 ErrBinMissing。
	FRPCPath() (string, error)
	// FRPSPath 同上，frps。
	FRPSPath() (string, error)
	// Missing 返回缺失的 kind 列表（按字母序，便于稳定测试）。
	// 实现以 os.Stat 探测，每次调用都会重新检查。
	Missing() []string
	// Root 返回最终选定的 repoRoot，供调用方记录到日志 / banner。
	Root() string
}

// ErrBinMissing 标记一个 kind 在磁盘上不存在 / 不可执行。
var ErrBinMissing = errors.New("binloc: binary missing")

// NewDefault 根据 GOOS + repoRoot 优先级构建 Locator。
//
// 入参 repoRootHint 可传空字符串（最常见）；非空时**优先**使用，跳过环境变量
// 与 os.Executable 检测，便于测试注入临时目录。
func NewDefault(repoRootHint string) Locator {
	root := resolveRoot(repoRootHint)
	return &fsLocator{
		root:     root,
		frpcRel:  frpcRelPath(runtime.GOOS),
		frpsRel:  frpsRelPath(runtime.GOOS),
		platform: runtime.GOOS,
	}
}

// resolveRoot 按上述优先级返回 repoRoot。
// 任一步骤失败 fallthrough 到下一步；最终兜底 "."。
func resolveRoot(hint string) string {
	if hint != "" {
		if abs, err := filepath.Abs(hint); err == nil {
			return abs
		}
		return hint
	}
	if env := os.Getenv("FRP_EASY_ROOT"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	if exe, err := os.Executable(); err == nil {
		if abs, err := filepath.Abs(filepath.Dir(exe)); err == nil {
			return abs
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func frpcRelPath(goos string) string {
	switch goos {
	case "windows":
		return filepath.Join("frp_win", "frpc.exe")
	case "linux":
		return filepath.Join("frp_linux", "frpc")
	default:
		return ""
	}
}

func frpsRelPath(goos string) string {
	switch goos {
	case "windows":
		return filepath.Join("frp_win", "frps.exe")
	case "linux":
		return filepath.Join("frp_linux", "frps")
	default:
		return ""
	}
}

type fsLocator struct {
	root     string
	frpcRel  string
	frpsRel  string
	platform string
}

func (l *fsLocator) Root() string { return l.root }

func (l *fsLocator) FRPCPath() (string, error) { return l.resolve(KindFrpc, l.frpcRel) }
func (l *fsLocator) FRPSPath() (string, error) { return l.resolve(KindFrps, l.frpsRel) }

func (l *fsLocator) resolve(kind, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("%w: %s on platform %s not supported", ErrBinMissing, kind, l.platform)
	}
	abs := filepath.Join(l.root, rel)
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s at %s", ErrBinMissing, kind, abs)
		}
		return "", fmt.Errorf("binloc.%s stat: %w", kind, err)
	}
	return abs, nil
}

func (l *fsLocator) Missing() []string {
	var missing []string
	if _, err := l.FRPCPath(); err != nil {
		missing = append(missing, KindFrpc)
	}
	if _, err := l.FRPSPath(); err != nil {
		missing = append(missing, KindFrps)
	}
	sort.Strings(missing)
	return missing
}
