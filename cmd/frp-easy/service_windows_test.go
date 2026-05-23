//go:build windows

// service_windows_test.go — T-019 Windows Service ABI 单测。
//
// 用途：以静态文本契约方式守护两个关键脚本不规：
//   1. install-service.ps1 不再生成 frp-easy-svc.cmd 包装脚本
//      （sc.exe binPath 直接指向 frp-easy.exe，进程内 svc.Run 锁 cwd）。
//   2. uninstall-service.ps1 仍保留 frp-easy-svc.cmd 防御性清理逻辑
//      （旧版升级用户磁盘上可能仍有该残留）。
//
// 这两个测试不启真服务、不调 sc.exe，仅做脚本文本 grep；
// 在 Linux CI 上被 //go:build windows tag 自动跳过（R-3 缓解）。

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readRepoFile 读 repo 根下相对路径的文件内容。
// cmd/frp-easy 包目录在 <repo>/cmd/frp-easy/，因此 repo 根 = ../../。
func readRepoFile(t *testing.T, relPath string) string {
	t.Helper()
	abs := filepath.Join("..", "..", filepath.FromSlash(relPath))
	b, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return string(b)
}

// TestInstallServiceScriptNoWrapperGen 验证 install-service.ps1 不再写
// frp-easy-svc.cmd 包装脚本——即不含 `Set-Content -Path $WrapperPath` 关键词。
// 这是 T-019 移除 wrapper.cmd 改走 in-process svc.Run 锁 cwd 后的契约。
func TestInstallServiceScriptNoWrapperGen(t *testing.T) {
	content := readRepoFile(t, "scripts/install-service.ps1")
	// 黑名单 1：生成 wrapper 的写动作。
	if strings.Contains(content, "Set-Content -Path $WrapperPath") {
		t.Errorf("install-service.ps1 仍包含旧版 wrapper.cmd 生成块 (Set-Content -Path $WrapperPath); T-019 应已移除")
	}
	// 黑名单 2：wrapper here-string 起始标识（防止改名变量但保留生成块）。
	if strings.Contains(content, "$WrapperContent = @\"") {
		t.Errorf("install-service.ps1 仍包含 wrapper here-string (\\$WrapperContent = @\"); T-019 应已移除")
	}
	// 白名单：sc.exe binPath 必须指向 frp-easy.exe 即 $BinaryPath。
	if !strings.Contains(content, "binPath= \"`\"$BinaryPath`\"\"") {
		t.Errorf("install-service.ps1 的 sc.exe binPath 未指向 $BinaryPath；预期 in-process 服务化")
	}
}

// TestUninstallStillCleansWrapper 验证 uninstall-service.ps1 仍保留 frp-easy-svc.cmd
// 防御性清理：T-018 及更早版本升级用户磁盘上仍可能残留该 .cmd 文件。
func TestUninstallStillCleansWrapper(t *testing.T) {
	content := readRepoFile(t, "scripts/uninstall-service.ps1")
	// 必须仍提到 frp-easy-svc.cmd（无论改注释成"防御性清理"还是"删除自己生成的"）。
	if !strings.Contains(content, "frp-easy-svc.cmd") {
		t.Errorf("uninstall-service.ps1 不再提及 frp-easy-svc.cmd；T-019 仍需保留防御性清理")
	}
	// 必须仍执行 Remove-Item 清理动作。
	if !strings.Contains(content, "Remove-Item -Force $WrapperPath") {
		t.Errorf("uninstall-service.ps1 不再 Remove-Item -Force $WrapperPath；T-019 仍需保留清理动作")
	}
}
