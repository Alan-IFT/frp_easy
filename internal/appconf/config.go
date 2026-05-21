// Package appconf 负责读 / 写 frp_easy 自身的应用配置 frp_easy.toml。
//
// 这是与 FRP 自身的 frpc.toml / frps.toml **不同**的文件 —— 这里只管 UI 服务
// 本身的运行参数（监听地址 / 端口 / 数据目录 / 日志目录），不涉及任何 FRP
// 业务字段。FRP 业务配置由 internal/frpconf 渲染。
//
// 【I-2 · Gate Review §8 / F-7】内部占用端口表（写死给 dev 与 ops 一眼可查）：
//
//	| 用途                          | 端口（默认） | 由谁监听            |
//	| ----------------------------- | ----------- | ------------------- |
//	| 本 UI 服务（HTTP）            | 7800        | cmd/frp-easy（本进程） |
//	| frpc admin API（reload/status）| 7400        | frpc 子进程         |
//	| frps dashboard（web UI 自带） | 7500        | frps 子进程         |
//	| frps bindPort（FRP 控制通道） | 7000        | frps 子进程         |
//	| FRP 业务 proxy.remotePort     | 用户填      | frps 子进程对外暴露 |
//
// 这五者目前无重叠。修改默认前先核对，避免引入冲突。
package appconf

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	toml "github.com/pelletier/go-toml/v2"
)

// AppConfig 是 UI 服务自身的配置。
//
// 字段命名遵循 02 §3.1：UIBindAddr / UIPort / DataDir / LogDir。
// LogDir 留空时由 Validate 自动填为 DataDir/logs。
type AppConfig struct {
	UIBindAddr string `toml:"UIBindAddr"`
	UIPort     int    `toml:"UIPort"`
	DataDir    string `toml:"DataDir"`
	LogDir     string `toml:"LogDir"`
}

// Default 返回出厂默认值（02 §3.1 / Q-10 决策 · T-011 网络默认值变更）。
//
// 默认监听 0.0.0.0（所有网卡），便于从其他设备访问 Web UI —— frp_easy 本质是
// 远程内网穿透管理工具，运维场景天然需要跨设备访问。仅需本机访问时，可把
// UIBindAddr 改为 127.0.0.1。绑定对外地址（0.0.0.0/::）时 main.go 会在 stderr
// 打印一条安全提示，引导用户尽快完成 setup。
func Default() *AppConfig {
	return &AppConfig{
		UIBindAddr: "0.0.0.0",
		UIPort:     7800,
		DataDir:    "./.frp_easy",
		LogDir:     "./.frp_easy/logs",
	}
}

// Load 从 path 读 frp_easy.toml；不存在则写默认值后返回 default。
//
// 行为：
//  1. path 不存在 → 写 Default() 到 path（含父目录）→ 返回 Default()。
//  2. path 存在 → toml.Unmarshal → 补默认 → Validate → 返回。
//  3. 解析失败 → 不覆盖文件，返回错误（让用户手动修复，不静默清空）。
func Load(path string) (*AppConfig, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("appconf: abs path: %w", err)
	}

	cfg := Default()
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			// 首次启动：写默认。
			if dirErr := os.MkdirAll(filepath.Dir(abs), 0o755); dirErr != nil {
				return nil, fmt.Errorf("appconf: mkdir for default: %w", dirErr)
			}
			out, mErr := toml.Marshal(cfg)
			if mErr != nil {
				return nil, fmt.Errorf("appconf: marshal default: %w", mErr)
			}
			if wErr := os.WriteFile(abs, out, 0o644); wErr != nil {
				return nil, fmt.Errorf("appconf: write default: %w", wErr)
			}
			if vErr := cfg.Validate(); vErr != nil {
				return nil, vErr
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("appconf: read %s: %w", abs, err)
	}

	// 已存在：反序列化（默认值打底，未提供字段保留默认）。
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("appconf: parse %s: %w", abs, err)
	}
	// 补默认（避免用户把字段留空）。
	if cfg.UIBindAddr == "" {
		cfg.UIBindAddr = "0.0.0.0"
	}
	if cfg.UIPort == 0 {
		cfg.UIPort = 7800
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./.frp_easy"
	}
	if cfg.LogDir == "" {
		cfg.LogDir = filepath.Join(cfg.DataDir, "logs")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate 校验字段合理性。
func (c *AppConfig) Validate() error {
	if c == nil {
		return errors.New("appconf: nil config")
	}
	if c.UIPort < 1 || c.UIPort > 65535 {
		return fmt.Errorf("appconf: UIPort %d out of range [1,65535]", c.UIPort)
	}
	if c.UIBindAddr == "" {
		return errors.New("appconf: UIBindAddr required")
	}
	// 接受 host 或 host:port —— 但本配置只允许 host（端口由 UIPort 字段管）。
	if ip := net.ParseIP(c.UIBindAddr); ip == nil {
		// 允许 hostname / "localhost"。仅在显然带端口（":数字"）时拒。
		if _, _, err := net.SplitHostPort(c.UIBindAddr); err == nil {
			return fmt.Errorf("appconf: UIBindAddr should be host only, not host:port (got %q)", c.UIBindAddr)
		}
	}
	if c.DataDir == "" {
		return errors.New("appconf: DataDir required")
	}
	if c.LogDir == "" {
		return errors.New("appconf: LogDir required")
	}
	return nil
}

// ListenAddr 返回供 net.Listen("tcp", ...) 用的字符串。
func (c *AppConfig) ListenAddr() string {
	return net.JoinHostPort(c.UIBindAddr, strconv.Itoa(c.UIPort))
}
