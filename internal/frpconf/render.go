// Package frpconf 把 DB 里的应用层模型渲染成 FRP 上游 1.x 系列识别的 TOML。
//
// 字段命名严格按 02 §附录 A：camelCase + 嵌套表（auth.* / webServer.* /
// log.* / [[proxies]] 数组段）。前端 / 业务层不关心 TOML 拼写。
//
// 渲染流程：调用 RenderFrpc / RenderFrps 拿到 []byte，再用 AtomicWrite 写入
// 目标路径（临时文件 + os.Rename），避免子进程在被读时拿到半写文件。
package frpconf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// ProxyInput 是单条 [[proxies]] 数组段的渲染输入。
// 字段含义与 storage.Proxy 一一对应（但本包不引入 storage 包，避免循环依赖）。
//
// 互斥规则：
//   - Type ∈ {"tcp","udp"}：RemotePort 必填，CustomDomains 必空。
//   - Type ∈ {"http","https"}：CustomDomains 必填非空，RemotePort 必为 nil。
type ProxyInput struct {
	Name          string
	Type          string // tcp/udp/http/https
	LocalIP       string
	LocalPort     int
	RemotePort    *int
	CustomDomains []string
}

// frpcAuth / frpcWebServer / frpcLog / frpcProxy 是序列化的内部表示。
// 用首字母小写的字段（私有），TOML 标签控制 camelCase 输出。
// 用指针 + omitempty 来区分"未设置"和"零值"——TOML 编码器对空字符串
// 与 nil 行为不同；指针更直观。
type frpcRoot struct {
	ServerAddr string          `toml:"serverAddr,omitempty"`
	ServerPort int             `toml:"serverPort,omitempty"`
	Log        *frpLog         `toml:"log,omitempty"`
	Auth       *frpAuth        `toml:"auth,omitempty"`
	WebServer  *frpWebServer   `toml:"webServer,omitempty"`
	Proxies    []frpProxyEntry `toml:"proxies,omitempty"`
}

type frpsRoot struct {
	BindPort  int           `toml:"bindPort"`
	Log       *frpLog       `toml:"log,omitempty"`
	Auth      *frpAuth      `toml:"auth,omitempty"`
	WebServer *frpWebServer `toml:"webServer,omitempty"`
}

type frpAuth struct {
	Method string `toml:"method,omitempty"`
	Token  string `toml:"token,omitempty"`
}

type frpWebServer struct {
	Addr     string `toml:"addr,omitempty"`
	Port     int    `toml:"port,omitempty"`
	User     string `toml:"user,omitempty"`
	Password string `toml:"password,omitempty"`
}

type frpLog struct {
	To      string `toml:"to,omitempty"`
	Level   string `toml:"level,omitempty"`
	MaxDays int    `toml:"maxDays,omitempty"`
}

type frpProxyEntry struct {
	Name          string   `toml:"name"`
	Type          string   `toml:"type"`
	LocalIP       string   `toml:"localIP,omitempty"`
	LocalPort     int      `toml:"localPort"`
	RemotePort    int      `toml:"remotePort,omitempty"`
	CustomDomains []string `toml:"customDomains,omitempty"`
}

// FrpcRenderInput 是渲染 frpc.toml 的全部输入（02 §3.4）。
//
// AuthToken 为空时整个 auth.* 段不写出（FRP 默认 = 无 token 鉴权）。
// AdminAddr / AdminPort / AdminUser / AdminPass 控制 webServer.* 段——
// 渲染时强制 Addr=127.0.0.1，避免 admin API 对外暴露。
type FrpcRenderInput struct {
	ServerAddr string
	ServerPort int
	AuthMethod string // 当前固定 "token" 或 ""；非 "token" / "" 视为错误
	AuthToken  string
	Proxies    []ProxyInput
	AdminAddr  string // 推荐 127.0.0.1
	AdminPort  int    // 推荐 7400
	AdminUser  string
	AdminPass  string
	LogPath    string // 子进程日志输出路径（log.to）
	LogLevel   string // 默认 "info"
	LogMaxDays int    // 默认 7
}

// FrpsRenderInput 是渲染 frps.toml 的全部输入（02 §3.4 + 01 B-15）。
type FrpsRenderInput struct {
	BindPort         int
	AuthMethod       string
	AuthToken        string
	DashboardEnabled bool
	DashboardAddr    string // 推荐 127.0.0.1
	DashboardPort    int    // 推荐 7500
	DashboardUser    string
	DashboardPass    string
	LogPath          string
	LogLevel         string
	LogMaxDays       int
}

// RenderFrpc 渲染 frpc.toml 字节。
func RenderFrpc(in FrpcRenderInput) ([]byte, error) {
	if in.ServerAddr == "" {
		return nil, errors.New("frpconf.RenderFrpc: serverAddr required")
	}
	if in.ServerPort < 1 || in.ServerPort > 65535 {
		return nil, fmt.Errorf("frpconf.RenderFrpc: serverPort %d out of range", in.ServerPort)
	}

	root := frpcRoot{
		ServerAddr: in.ServerAddr,
		ServerPort: in.ServerPort,
	}
	if in.AuthToken != "" {
		method := in.AuthMethod
		if method == "" {
			method = "token"
		}
		if method != "token" {
			return nil, fmt.Errorf("frpconf.RenderFrpc: unsupported auth.method %q (only \"token\" in MVP)", method)
		}
		root.Auth = &frpAuth{Method: method, Token: in.AuthToken}
	}
	if in.AdminPort > 0 {
		addr := in.AdminAddr
		if addr == "" {
			addr = "127.0.0.1"
		}
		root.WebServer = &frpWebServer{
			Addr:     addr,
			Port:     in.AdminPort,
			User:     in.AdminUser,
			Password: in.AdminPass,
		}
	}
	if in.LogPath != "" {
		root.Log = buildLog(in.LogPath, in.LogLevel, in.LogMaxDays)
	}
	for _, p := range in.Proxies {
		entry, err := proxyToEntry(p)
		if err != nil {
			return nil, err
		}
		root.Proxies = append(root.Proxies, entry)
	}
	return toml.Marshal(&root)
}

// RenderFrps 渲染 frps.toml 字节。
func RenderFrps(in FrpsRenderInput) ([]byte, error) {
	if in.BindPort < 1 || in.BindPort > 65535 {
		return nil, fmt.Errorf("frpconf.RenderFrps: bindPort %d out of range", in.BindPort)
	}
	root := frpsRoot{BindPort: in.BindPort}
	if in.AuthToken != "" {
		method := in.AuthMethod
		if method == "" {
			method = "token"
		}
		if method != "token" {
			return nil, fmt.Errorf("frpconf.RenderFrps: unsupported auth.method %q", method)
		}
		root.Auth = &frpAuth{Method: method, Token: in.AuthToken}
	}
	if in.DashboardEnabled {
		addr := in.DashboardAddr
		if addr == "" {
			addr = "127.0.0.1"
		}
		port := in.DashboardPort
		if port == 0 {
			port = 7500
		}
		root.WebServer = &frpWebServer{
			Addr:     addr,
			Port:     port,
			User:     in.DashboardUser,
			Password: in.DashboardPass,
		}
	}
	if in.LogPath != "" {
		root.Log = buildLog(in.LogPath, in.LogLevel, in.LogMaxDays)
	}
	return toml.Marshal(&root)
}

func buildLog(path, level string, maxDays int) *frpLog {
	if level == "" {
		level = "info"
	}
	if maxDays == 0 {
		maxDays = 7
	}
	return &frpLog{To: path, Level: level, MaxDays: maxDays}
}

func proxyToEntry(p ProxyInput) (frpProxyEntry, error) {
	if p.Name == "" {
		return frpProxyEntry{}, errors.New("frpconf: proxy name required")
	}
	if p.LocalPort < 1 || p.LocalPort > 65535 {
		return frpProxyEntry{}, fmt.Errorf("frpconf: proxy %q localPort %d out of range", p.Name, p.LocalPort)
	}
	localIP := p.LocalIP
	if localIP == "" {
		localIP = "127.0.0.1"
	}
	e := frpProxyEntry{
		Name:      p.Name,
		Type:      p.Type,
		LocalIP:   localIP,
		LocalPort: p.LocalPort,
	}
	switch p.Type {
	case "tcp", "udp":
		if p.RemotePort == nil {
			return e, fmt.Errorf("frpconf: %s proxy %q requires remotePort", p.Type, p.Name)
		}
		if len(p.CustomDomains) > 0 {
			return e, fmt.Errorf("frpconf: %s proxy %q must not set customDomains", p.Type, p.Name)
		}
		e.RemotePort = *p.RemotePort
	case "http", "https":
		if len(p.CustomDomains) == 0 {
			return e, fmt.Errorf("frpconf: %s proxy %q requires customDomains", p.Type, p.Name)
		}
		if p.RemotePort != nil {
			return e, fmt.Errorf("frpconf: %s proxy %q must not set remotePort", p.Type, p.Name)
		}
		e.CustomDomains = p.CustomDomains
	default:
		return e, fmt.Errorf("frpconf: proxy %q invalid type %q", p.Name, p.Type)
	}
	return e, nil
}

// AtomicWrite 写 content 到 path：先写临时文件（同目录），再 os.Rename。
//
// Windows 上 os.Rename 在目标已存在时仍可成功（modernc 等同 MoveFileEx 覆盖）；
// 若运行环境不支持原子覆盖，调用方应在感知失败时回退到直接 WriteFile。
func AtomicWrite(path string, content []byte) error {
	if path == "" {
		return errors.New("frpconf.AtomicWrite: empty path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("frpconf.AtomicWrite mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".frpconf-*.tmp")
	if err != nil {
		return fmt.Errorf("frpconf.AtomicWrite tempfile: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("frpconf.AtomicWrite write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("frpconf.AtomicWrite sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("frpconf.AtomicWrite close: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("frpconf.AtomicWrite rename: %w", err)
	}
	return nil
}
