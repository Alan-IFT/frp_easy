//go:build !windows

// service_other.go — 非 Windows 平台的服务化分支占位（T-019）。
//
// 用途：让 Linux / macOS 编译不需要引用 `golang.org/x/sys/windows/svc`；
//       main.go 顶端调 isWindowsService() 在这些平台恒为 false，
//       runService() 在这些平台仅作为安全网（理论不应被调用）。
//
// 实际服务化逻辑在 service_windows.go（带 //go:build windows tag）。

package main

import "errors"

// isWindowsService 在非 Windows 平台恒为 false。
// 这让 main() 顶端的服务化分流自然退化到现有控制台分支。
func isWindowsService() bool { return false }

// runService 在非 Windows 平台不应被 main() 调用（因为 isWindowsService 已恒 false）；
// 此处仅作为防御性安全网，返回明确错误避免静默假装成功。
func runService() error {
	return errors.New("service mode not supported on this platform")
}
