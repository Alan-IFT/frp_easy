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
	ServerAddr string `toml:"serverAddr,omitempty"`
	ServerPort int    `toml:"serverPort,omitempty"`
	// LoginFailExit 由 RenderFrpc 恒设为 &false（T-038）：让 frpc 在首次登录失败时
	// **不**直接 exit，而是进入 frp 自身的重连循环（dial timeout / heartbeat 监控）。
	// 这与 frp-easy autoRestoreProcs 的指数 backoff 形成双层防御：
	// 网络瞬时不可达由 frp 自身 retry 处理；二进制 / 配置硬错由 autoRestoreProcs 兜底。
	// 用 *bool 而非 bool 让 nil（未设置）与 false（显式禁用 exit）语义清晰；
	// omitempty 在 *bool=nil 时省略输出，与本包既有"指针 + omitempty"约定一致。
	LoginFailExit *bool           `toml:"loginFailExit,omitempty"`
	Log           *frpLog         `toml:"log,omitempty"`
	Auth          *frpAuth        `toml:"auth,omitempty"`
	WebServer     *frpWebServer   `toml:"webServer,omitempty"`
	Proxies       []frpProxyEntry `toml:"proxies,omitempty"`
}

type frpsRoot struct {
	BindPort  int           `toml:"bindPort"`
	Log       *frpLog       `toml:"log,omitempty"`
	Auth      *frpAuth      `toml:"auth,omitempty"`
	WebServer *frpWebServer `toml:"webServer,omitempty"`
	// AllowPorts 是 [[allowPorts]] 数组段（T-040）。放 struct 末尾，让 go-toml
	// 按字段定义顺序输出时，所有表段 ([log] / [auth] / [webServer]) 在前、
	// 数组段在后，符合 TOML 规范"表段不能出现在数组段之后"。
	AllowPorts []frpsAllowPort `toml:"allowPorts,omitempty"`
}

// frpsAllowPort 是单条 [[allowPorts]] 渲染体（T-040）。
// Start/End/Single 三字段都带 omitempty：Single != 0 时不输出 start/end，反之亦然。
// frp 上游接受 single 或 start+end 二选一；本结构通过 omitempty + Validate 双层保证不会同时输出。
type frpsAllowPort struct {
	Start  int `toml:"start,omitempty"`
	End    int `toml:"end,omitempty"`
	Single int `toml:"single,omitempty"`
}

// FrpsAllowPortRange 是单条 allowPorts 配置项（T-040）。
//
// 互斥规则：
//   - Single != 0：Start 与 End 必须 0；TOML 渲染为 `single = N`。
//   - Single == 0：Start 与 End 必须均 ∈ [1,65535] 且 Start ≤ End；TOML 渲染为 `start = X\nend = Y`。
//   - 三者全 0：非法（空 entry）。
//
// 字段含义与 httpapi.AllowPortRange / openapi AllowPortRange 一一对应。
type FrpsAllowPortRange struct {
	Start  int
	End    int
	Single int
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
//
// AllowPorts（T-040）为 nil 或 空切片时，TOML 输出不含 `[[allowPorts]]` 段
// （= frp 上游"允许所有端口"默认语义）。非空时调用方 *必须* 已经过校验，
// 或由 RenderFrps 内部调用 ValidateFrpsAllowPorts 守门。
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
	AllowPorts       []FrpsAllowPortRange // T-040：nil 或 [] = 不渲染段
}

// RenderFrpc 渲染 frpc.toml 字节。
func RenderFrpc(in FrpcRenderInput) ([]byte, error) {
	if in.ServerAddr == "" {
		return nil, errors.New("frpconf.RenderFrpc: serverAddr required")
	}
	if in.ServerPort < 1 || in.ServerPort > 65535 {
		return nil, fmt.Errorf("frpconf.RenderFrpc: serverPort %d out of range", in.ServerPort)
	}

	// T-038: loginFailExit 恒为 false —— 与 autoRestoreProcs retry 形成双层防御。
	// 解释见 frpcRoot.LoginFailExit 字段注释。
	no := false
	root := frpcRoot{
		ServerAddr:    in.ServerAddr,
		ServerPort:    in.ServerPort,
		LoginFailExit: &no,
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
	// T-040: allowPorts 段。nil / [] 跳过；非空则先校验再渲染。
	// 校验与渲染同源避免重复实现漂移；调用方（handlers_server）通常会先调一次 Validate
	// 让 PUT 时能给前端定位错误 + 422，但 RenderFrps 内部再调一次是 belt-and-suspenders。
	if len(in.AllowPorts) > 0 {
		if err := ValidateFrpsAllowPorts(in.AllowPorts); err != nil {
			return nil, err
		}
		out := make([]frpsAllowPort, 0, len(in.AllowPorts))
		for _, r := range in.AllowPorts {
			if r.Single != 0 {
				out = append(out, frpsAllowPort{Single: r.Single})
			} else {
				out = append(out, frpsAllowPort{Start: r.Start, End: r.End})
			}
		}
		root.AllowPorts = out
	}
	return toml.Marshal(&root)
}

// ValidateFrpsAllowPorts 校验 allowPorts 数组（T-040）。
//
// 规则（与 02 §3.5 + 01 OQ-1/2 一致）：
//   - 数组长度 ≤ 100；
//   - 每个 entry 满足互斥规则（Single != 0 时 Start/End 必须 0；反之亦然；三者全 0 非法）；
//   - 每个端口 ∈ [1, 65535]；
//   - Start ≤ End；
//   - 多个 entry 之间不允许区间重叠（闭区间语义：[1000,2000] 与 [2000,3000] 算重叠，
//     因为 2000 同时属于两段）。
//
// 错误文本均为中文（用户面向），含 `allowPorts[i]` 字面定位首个非法 entry。
// 暴露为导出函数让 internal/httpapi 直接调用，避免双实现漂移。
func ValidateFrpsAllowPorts(in []FrpsAllowPortRange) error {
	if len(in) > 100 {
		return fmt.Errorf("allowPorts 最多 100 条（当前 %d）", len(in))
	}
	type span struct{ lo, hi, idx int }
	spans := make([]span, 0, len(in))
	for i, r := range in {
		// 互斥 + 必填校验
		if r.Single != 0 && (r.Start != 0 || r.End != 0) {
			return fmt.Errorf("allowPorts[%d] single 与 start/end 互斥，不能同时设置", i)
		}
		if r.Single == 0 && r.Start == 0 && r.End == 0 {
			return fmt.Errorf("allowPorts[%d] 必须设置 single 或 start+end", i)
		}
		var lo, hi int
		if r.Single != 0 {
			if r.Single < 1 || r.Single > 65535 {
				return fmt.Errorf("allowPorts[%d] single=%d 超出 [1,65535]", i, r.Single)
			}
			lo, hi = r.Single, r.Single
		} else {
			if r.Start < 1 || r.Start > 65535 {
				return fmt.Errorf("allowPorts[%d] start=%d 超出 [1,65535]", i, r.Start)
			}
			if r.End < 1 || r.End > 65535 {
				return fmt.Errorf("allowPorts[%d] end=%d 超出 [1,65535]", i, r.End)
			}
			if r.Start > r.End {
				return fmt.Errorf("allowPorts[%d] start=%d 必须 ≤ end=%d", i, r.Start, r.End)
			}
			lo, hi = r.Start, r.End
		}
		spans = append(spans, span{lo, hi, i})
	}
	// overlap 检测：O(n²) on n≤100；语义清晰胜过 sweep line。
	// 闭区间相交条件：a.lo <= b.hi && b.lo <= a.hi
	for i := 0; i < len(spans); i++ {
		for j := i + 1; j < len(spans); j++ {
			a, b := spans[i], spans[j]
			if a.lo <= b.hi && b.lo <= a.hi {
				return fmt.Errorf("allowPorts[%d] 与 allowPorts[%d] 区间重叠", a.idx, b.idx)
			}
		}
	}
	return nil
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
//
// 【T-007 AC-1】临时文件创建后立即 Chmod 到 0o600；rename 完成后对目标文件再次
// Chmod 0o600（覆盖已存在文件保留旧权限的 corner case）。POSIX 立即生效；Windows
// 上 os.Chmod 仅当 mode 关闭 owner-write 位时设 ReadOnly attr，本处 0o600 含
// owner-write 位（=1），不会触发 ReadOnly，对 rename 不变量无影响。
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
	// 临时文件立即收紧权限：os.CreateTemp 在 POSIX 下用 0o600 调 open，但实际生效
	// = 0o600 & ~umask；在 umask=0 / 0o002 等场景下其它本地用户仍可读，临时窗口
	// 内含 frps_token 明文存在泄露风险。chmod 一次彻底关掉 group/other 位。
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("frpconf.AtomicWrite chmod tmp: %w", err)
	}
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
	// 目标文件再次 chmod：rename 在某些场景（目标文件已存在被覆盖）可能保留旧
	// 权限位，此处 fail-closed 保证最终输出始终是 0o600。
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("frpconf.AtomicWrite chmod final: %w", err)
	}
	return nil
}
