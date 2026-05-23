// install.go: 抽出"原子 rename + Linux chmod + Windows fallback"为共享 Install 方法。
//
// 设计契约见 docs/features/upload-bin-multiport-ip-probe/02_SOLUTION_DESIGN.md §A.2。
// 复用方：
//   - downloader.doDownload（下载链路，archive 解压后的 binTmp → Install，maxBytes = -1）
//   - httpapi.uploadBin（上传链路，multipart file → Install，maxBytes = 64 MiB）
//
// 设计要点：
//   - "原子 rename + chmod + Windows fallback" 整段提炼，让下载 / 上传走相同代码路径。
//   - maxBytes <= 0 表示不限大小（下载已先 io.Copy 到 binTmp、由 archive 大小天然兜底）；
//     > 0 时超过该字节数返回 ErrFileTooLarge（caller 转 413）。
//   - 临时文件名前缀 `.install-*.tmp`（与原 `.dl-bin-*.tmp` 区分；upload 来源不同）。
//   - GOOS 注入 seam 用 m.goos 字段（与 doDownload 共享，T-010 insight L28）。
package downloader

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
)

// ErrFileTooLarge 表示 Install 在拷贝时检测到 src 字节数超过调用方限制。
// caller 应据此返回 413（写接口）/ 中断（下载链路一般传 -1 不触发）。
var ErrFileTooLarge = errors.New("downloader: source exceeds maxBytes limit")

// Install 把 src 流写入 kind 对应的"权威安装路径"，原子 rename + Linux chmod。
//
// 步骤：
//  1. resolveParams(kind, goos) → targetDir / targetPath。
//  2. MkdirAll(targetDir, 0o755)。
//  3. CreateTemp(targetDir, ".install-*.tmp") + defer Remove（成功 rename 后此 Remove 是 no-op）。
//  4. sha256 = TeeReader 边写边算；io.Copy 受 io.LimitReader(maxBytes+1) 截断（仅当 maxBytes>0）。
//  5. written > maxBytes → ErrFileTooLarge（临时文件 defer 清理）。
//  6. tmp.Sync + Close。
//  7. os.Rename atomic；Windows 上 rename 失败 → os.Remove 后 retry。
//  8. Linux 下 os.Chmod(targetPath, 0o755)（T-007 L17 双 chmod 模式）。
//  9. 返回 (sha256Hex, written, finalPath, nil)。
//
// 错误：
//   - 不支持的 OS / kind → resolveParams 已返 ErrUnsupportedOS。
//   - 落盘失败 → 普通 error（caller 转 500）。
//   - 超限 → ErrFileTooLarge sentinel。
func (m *Manager) Install(kind string, src io.Reader, maxBytes int64) (sha256Hex string, written int64, finalPath string, err error) {
	goos := m.goos
	if goos == "" {
		goos = runtime.GOOS
	}

	targetDir, targetPath, _, _, perr := m.resolveParams(kind, goos)
	if perr != nil {
		return "", 0, "", perr
	}
	if kind != "frpc" && kind != "frps" {
		return "", 0, "", ErrBadKind
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", 0, "", fmt.Errorf("创建目录失败: %w", err)
	}

	tmp, err := os.CreateTemp(targetDir, ".install-*.tmp")
	if err != nil {
		return "", 0, "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	// 失败时清理临时文件；成功 rename 后该路径已不存在，Remove 返回 ENOENT 可忽略。
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	hasher := sha256.New()
	tee := io.TeeReader(src, hasher)

	var lim io.Reader = tee
	if maxBytes > 0 {
		// 多读 1 字节以判 "刚好等于" vs "超过"。
		lim = io.LimitReader(tee, maxBytes+1)
	}

	written, err = io.Copy(tmp, lim)
	if err != nil {
		return "", written, "", fmt.Errorf("写入失败: %w", err)
	}
	if maxBytes > 0 && written > maxBytes {
		return "", written, "", ErrFileTooLarge
	}

	if err := tmp.Sync(); err != nil {
		return "", written, "", fmt.Errorf("sync 失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", written, "", fmt.Errorf("close 临时文件失败: %w", err)
	}

	// 原子 rename + Windows fallback（沿用 doDownload 现有行为，T-002 insight L10）。
	renameErr := os.Rename(tmpPath, targetPath)
	if renameErr != nil && goos == "windows" {
		// Windows 不允许 Rename 覆盖已存在文件，先 Remove 旧文件再 Rename。
		if removeErr := os.Remove(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", written, "", fmt.Errorf("移除旧版本失败: %w", removeErr)
		}
		renameErr = os.Rename(tmpPath, targetPath)
	}
	if renameErr != nil {
		return "", written, "", fmt.Errorf("安装失败: %w", renameErr)
	}

	// Linux：rename 后再 chmod（T-007 insight L17，双 chmod 模式之"final 一段"；
	// tmp 文件本身由 CreateTemp 设的 0o600，重命名后必须再 chmod 才能让 frpc/frps 可执行）。
	if goos == "linux" {
		if cerr := os.Chmod(targetPath, 0o755); cerr != nil {
			// best-effort：chmod 失败不致命（用户可手动），但日志记录之。
			if m.logger != nil {
				m.logger.Warn("chmod failed", "path", targetPath, "err", cerr)
			}
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), written, targetPath, nil
}

