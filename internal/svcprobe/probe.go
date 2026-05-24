// Package svcprobe 跨平台探测当前 frp-easy 进程的服务化状态。
//
// 用途：让 internal/httpapi 在 GET /api/v1/system/service-status 时拿到
//
//	(a) 当前进程是否被 systemd / Windows SCM 拉起（"被监管"）；
//	(b) 服务是否已配置为开机自启（boot-time autostart）；
//	(c) 进程实际运行用户名（user-visible 信息）。
//
// 用三个 build-tag 文件实现：probe_linux.go / probe_windows.go / probe_other.go。
// 探测失败一律降级为 supervised=false / boot_autostart=false，**不** panic、
// 不返回 error —— 调用方拿到的总是一个安全可用的 Status 值。
//
// 设计依据：T-038 02_SOLUTION_DESIGN §3.1。
package svcprobe

import "context"

// Status 是一次 Probe 的结果快照。
type Status struct {
	Supervised    bool   `json:"supervised"`
	Supervisor    string `json:"supervisor"`     // "systemd" | "windows-service" | "none"
	BootAutostart bool   `json:"boot_autostart"` // 服务已 enabled / AUTO_START
	RunAs         string `json:"run_as"`         // 当前进程 owner（user-visible）
	ProbeError    string `json:"probe_error,omitempty"`
}

// Probe 返回当前进程的服务化状态。
//
// 总预算 5s（适用于 Linux systemctl is-enabled / Windows sc.exe qc 可能阻塞的外部调用）；
// ctx 已超时 → 立即返回 supervised=false + ProbeError="probe timeout"。
//
// 实现见 probe_<os>.go 文件（build tag 分流）。
func Probe(ctx context.Context) Status {
	return probe(ctx)
}
