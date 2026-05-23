// Command frp-easy 是 UI 服务进程入口。
//
// 启动序列（严格按 03 §7 Q-E）：
//  1. 加载 appconf（frp_easy.toml；不存在时写默认）。
//  2. storage.Open(dataDir) → 跑迁移。
//  3. 初始化 binloc.Locator、procmgr.Manager、auth.RateLimiter。
//  4. 启动 HTTP server：ReadyGate 在 store/proc 未就绪期间写接口返 503 + Retry-After: 2。
//  5. 读 kv 中 mode.frpc.enabled / mode.frps.enabled，若 true 且对应二进制存在
//     → procmgr.Start(kind)（AC-9 自动恢复）。
//  6. 阻塞 Listen；监 SIGINT/SIGTERM → 优雅关 procmgr → 关 HTTP → 关 storage。
//
// 【C-2 · Gate Review §8 / F-3】端口被占用友好提示：
// net.Listen 报 "address already in use" / "bind: ..." 时 stderr 打中文文案后
// os.Exit(2)，**不**自动换端口（Q-10 决策：确定性优于随机，避免用户找不到入口）。
//
// 【安全提示 · T-011】UIBindAddr 为对外可达地址（0.0.0.0/::）时 stderr 打印
// 一条中性安全提示，引导用户尽快完成 setup 创建管理员账号、并说明如何改回
// 仅本机访问；绑定回环地址（127.0.0.1/::1/localhost）时不打印。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/frp-easy/frp-easy/internal/appconf"
	"github.com/frp-easy/frp-easy/internal/auth"
	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/browseropen"
	"github.com/frp-easy/frp-easy/internal/downloader"
	"github.com/frp-easy/frp-easy/internal/frpcadmin"
	"github.com/frp-easy/frp-easy/internal/httpapi"
	"github.com/frp-easy/frp-easy/internal/logrotate"
	"github.com/frp-easy/frp-easy/internal/procmgr"
	"github.com/frp-easy/frp-easy/internal/storage"
)

// Version 由构建脚本通过 -ldflags 注入；MVP 阶段写死 0.1.0。
var Version = "0.1.0"

// usageText 是 --help / -h 打印的中文帮助文本（T-008 FR-3.2 / AC-12）。
// 必须覆盖：用法、flag 列表、配置文件位置、UI 默认地址、退出码语义。
// 注意：示例不写任何引号包裹的 8+ 字符敏感字符串，避免 verify_all A.1
// secrets scan 正则误中（T-008 03_GATE_REVIEW MINOR-4）。
const usageText = `用法: frp-easy [选项]

frp-easy 是 FRP 可视化管理 UI 的单二进制服务进程。

选项:
  -h, --help         显示本帮助并退出
  -v, --version      显示版本号并退出
      --no-browser   启动后不自动打开浏览器（默认 TTY 启动时打开）

配置:
  配置文件           frp_easy.toml（与本程序同目录；可通过环境变量 FRP_EASY_CONFIG 覆盖路径）
  UI 默认地址        http://127.0.0.1:7800
  数据目录默认       ./.frp_easy

环境变量:
  FRP_EASY_CONFIG              配置文件路径（默认 ./frp_easy.toml）
  FRP_EASY_NO_BROWSER          设为非空值禁用自动打开浏览器（等价于 --no-browser）
  FRP_EASY_LOG_MAX_SIZE_MB     单 ui.log 上限 MB（默认 10）
  FRP_EASY_LOG_MAX_BACKUPS     ui.log 历史份数（默认 5）
  FRP_EASY_LOG_MAX_AGE_DAYS    ui.log 最长保留天数（默认 30）

退出码:
  0   正常退出
  1   一般错误（启动失败、配置错误等）
  2   端口被占用 / 未知 flag

更多文档：docs/DEPLOYMENT.md
`

func main() {
	// 【T-019】Windows Service 双入口分流：被 SCM 拉起时走 runService()
	// 实现完整 SetServiceStatus 状态机（解决 sc.exe start 1053 错误）；
	// 控制台 / dev / 双击 .exe 启动时 isWindowsService() 返回 false，
	// 退化到现有 run(nil, nil) 控制台分支（NFR-9 不破断言）。
	if isWindowsService() {
		if err := runService(); err != nil {
			// 服务化分支主路径错误已通过 SCM 上报；这里只兜底防 panic 后未退出。
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(nil, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run 是 frp-easy UI 服务的主启动序列。
//
// 参数：
//   - stopCh：非 nil 时为外部关停信号源（Windows Service Stop control code），
//     与 sigCh / serveErr 三路并存。nil 表示"无外部关停源"，select 对应
//     case 永远阻塞，退化到现有控制台分支（仅依赖 SIGINT/SIGTERM）。
//   - readyCh：非 nil 时在 HTTP server / autoRestoreProcs / ready.Store(true)
//     完成后由 run() 内部 close(readyCh) 通知调用方"启动序列结束"。
//     Windows Service Execute 用此通道决定何时把 SCM 状态从
//     START_PENDING 切到 RUNNING。
//
// 控制台 / dev 调用：`run(nil, nil)` —— 两参均 nil 等价于现有行为，
// NFR-9 启动序列零字节改。
func run(stopCh <-chan struct{}, readyCh chan<- struct{}) error {
	// 0. flag 解析（T-008 FR-3 / AC-11/12/13/14）
	//
	// 必须早于 appconf.Load：用户即使没有 frp_easy.toml 也能跑 --version / --help。
	// 用 ContinueOnError + 自定义 stderr 中文化，避免污染 flag.CommandLine。
	// 03_GATE_REVIEW MINOR-1：显式分流 flag.ErrHelp 与"真未知 flag"两种 err。
	fs := flag.NewFlagSet("frp-easy", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // 让我们接管 stderr/stdout 输出
	var (
		showVersion bool
		showHelp    bool
		noBrowser   bool
	)
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&showVersion, "v", false, "")
	fs.BoolVar(&showHelp, "help", false, "")
	fs.BoolVar(&showHelp, "h", false, "")
	fs.BoolVar(&noBrowser, "no-browser", false, "")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(os.Stdout, usageText)
			return nil
		}
		fmt.Fprintf(os.Stderr, "frp-easy: 未识别的参数。运行 'frp-easy --help' 查看用法。\n")
		os.Exit(2)
	}
	if showHelp {
		fmt.Fprint(os.Stdout, usageText)
		return nil
	}
	if showVersion {
		fmt.Fprintf(os.Stdout, "frp-easy %s\n", Version)
		return nil
	}

	// 1. appconf
	cfgPath := envOr("FRP_EASY_CONFIG", "frp_easy.toml")
	cfg, err := appconf.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("加载 %s 失败：%w", cfgPath, err)
	}
	// 【T-022】安全提示移到 logger 初始化后；详见步骤 3。原放在此处仅写 stderr，
	// Windows Service / systemd 服务模式下 stderr 被 SCM/journald 丢弃 → ui.log
	// 拿不到该提示，运维盲区。

	// 解析数据目录绝对路径
	dataDir, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("DataDir 路径错误：%w", err)
	}
	logDir, err := filepath.Abs(cfg.LogDir)
	if err != nil {
		return fmt.Errorf("LogDir 路径错误：%w", err)
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败：%w", err)
	}

	// 2. storage
	store, openErr := storage.Open(dataDir)
	if openErr != nil && !errors.Is(openErr, storage.ErrCorruptReset) {
		return fmt.Errorf("打开数据库失败：%w", openErr)
	}
	defer store.Close()

	// 3. logger（slog → 同时写 ui.log 与 stderr）
	//
	// 【T-007 AC-2】ui.log 权限收紧到 0o600：仅 owner 可读写。
	// 【T-010 AC-3】通过 internal/logrotate 接入 lumberjack 做 size+age+count 三轴轮转，
	// 长跑 systemd / Windows Service 不再爆盘。失败时降级到 stderr-only。
	uiLogPath := filepath.Join(logDir, "ui.log")
	logWriter, lwErr := logrotate.New(logrotate.LoadOptionsFromEnv(uiLogPath))
	if lwErr != nil {
		fmt.Fprintf(os.Stderr, "WARN: 日志轮转初始化失败 %v；将仅写 stderr。\n", lwErr)
	}
	defer func() {
		if logWriter != nil {
			_ = logWriter.Close()
		}
	}()
	logger := newLogger(logWriter)
	if errors.Is(openErr, storage.ErrCorruptReset) {
		logger.Warn("data.db corrupt detected; renamed and reset", "dataDir", dataDir)
	}

	// 【T-022】安全提示：UI 绑定对外可达地址（0.0.0.0/::）时，引导用户尽快完成 setup。
	// 双轨输出：(a) stderr 给控制台用户人类可读多行文案；(b) logger.Warn 让
	// Windows Service / systemd 服务模式下（stderr 被 SCM/journald 丢弃）也能从
	// ui.log 拿到提示。正向枚举两个 unspecified 地址，不误伤用户自填的具体 LAN IP。
	if cfg.UIBindAddr == "0.0.0.0" || cfg.UIBindAddr == "::" {
		notice := exposureNotice(cfg.UIPort, cfgPath)
		fmt.Fprint(os.Stderr, notice)
		logger.Warn("ui exposure notice", "addr", cfg.UIBindAddr, "port", cfg.UIPort, "config_path", cfgPath, "message", notice)
	}

	// 4. binloc / procmgr / ratelimiter / frpcadmin（先 nil，需要时构造）
	loc := binloc.NewDefault("")
	logger.Info("locator resolved", "root", loc.Root(), "missing", loc.Missing())

	runtimeDir := filepath.Join(dataDir, "runtime")
	frpcTOML := filepath.Join(runtimeDir, "frpc.toml")
	frpsTOML := filepath.Join(runtimeDir, "frps.toml")
	frpcLog := filepath.Join(logDir, "frpc.log")
	frpsLog := filepath.Join(logDir, "frps.log")

	// frpc admin 凭据：从 kv 读；首次启动生成。
	adminCfg := ensureFrpcAdminCreds(store, logger)
	reloader := frpcadmin.New(adminCfg.Addr, adminCfg.Port, adminCfg.User, adminCfg.Pass)

	pm := procmgr.New(procmgr.Config{
		Locator:     loc,
		ConfigPaths: map[string]string{"frpc": frpcTOML, "frps": frpsTOML},
		LogFiles:    map[string]string{"frpc": frpcLog, "frps": frpsLog},
		Reloader:    reloader,
	})

	rl := auth.NewRateLimiter(store)

	// T-002: binary downloader — installs frpc/frps from GitHub Releases.
	dl := downloader.New(loc.Root(), logger)

	// 5. ReadyGate 状态：HTTP 启动期间 false；启动序列结束后翻 true。
	ready := &atomic.Bool{}
	ready.Store(false)

	deps := httpapi.Dependencies{
		Store:       store,
		Locator:     loc,
		ProcMgr:     pm,
		RateLimiter: rl,
		LogFiles:    map[string]string{"frpc": frpcLog, "frps": frpsLog},
		ConfigPaths: map[string]string{"frpc": frpcTOML, "frps": frpsTOML},
		FrpcAdmin: httpapi.FrpcAdminCreds{
			Addr: adminCfg.Addr,
			Port: adminCfg.Port,
			User: adminCfg.User,
			Pass: adminCfg.Pass,
		},
		Ready:      ready.Load,
		Logger:     logger,
		DevMode:    false,
		Version:    Version,
		Downloader: dl,
	}
	handler := httpapi.New(deps)

	addr := cfg.ListenAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// 【C-2】端口被占友好提示
		if isAddrInUse(err) {
			fmt.Fprintf(os.Stderr,
				"frp_easy UI 启动失败：端口 %d 已被占用。请关闭占用进程，或编辑 %s 中 UIPort = %d 后重试。\n",
				cfg.UIPort, cfgPath, cfg.UIPort+1)
			os.Exit(2)
		}
		return fmt.Errorf("监听 %s 失败：%w", addr, err)
	}

	srv := &http.Server{
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	fmt.Fprintf(os.Stderr, "frp_easy UI 已启动：http://%s （Ctrl+C 退出）\n", addr)
	logger.Info("http listening", "addr", addr, "version", Version)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ln)
	}()

	// 6. 启动序列尾巴：恢复模式开关（AC-9）
	autoRestoreProcs(store, pm, loc, logger, map[string]string{"frpc": frpcTOML, "frps": frpsTOML})
	ready.Store(true)
	logger.Info("ready gate opened")

	// 【T-019】通知 Windows Service Execute 切 SCM 状态到 RUNNING。
	// readyCh == nil 时（控制台分支）无操作，行为与现状等价。
	if readyCh != nil {
		close(readyCh)
	}

	// 【T-010 AC-2】TTY 启动时自动打开浏览器；systemd / service 模式天然被 TTY 检测排除。
	// 0.0.0.0 / :: 绑定时把 URL 改写为 127.0.0.1，浏览器无法访问 unspecified address。
	if browseropen.ShouldOpen(noBrowser) {
		openURL := fmt.Sprintf("http://%s", addr)
		if cfg.UIBindAddr == "0.0.0.0" || cfg.UIBindAddr == "::" {
			openURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.UIPort)
		}
		if bErr := browseropen.Open(openURL); bErr != nil {
			logger.Warn("auto-open browser failed", "err", bErr, "url", openURL)
		} else {
			logger.Info("opened browser", "url", openURL)
		}
	}

	// 7. 信号 / Windows Service Stop / HTTP fatal 三路 select
	// 【T-019】追加 case <-stopCh 让 Windows Service Execute 能通过 close(stopCh)
	// 触发优雅关停链路；stopCh == nil 时该 case 永久阻塞（Go select 对 nil
	// channel 的 case 视为不存在），等价于现有控制台分支行为。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-sigCh:
		logger.Info("signal received, shutting down", "signal", s.String())
	case <-stopCh:
		logger.Info("stopCh received, shutting down")
	case e := <-serveErr:
		if e != nil && !errors.Is(e, http.ErrServerClosed) {
			logger.Error("http server fatal", "err", e)
		}
	}

	// 优雅关停
	ready.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pm.Shutdown()
	_ = srv.Shutdown(ctx)
	return nil
}

// exposureNotice 构造 UI 绑定对外地址时打印到 stderr 的中文安全提示。
// 三要素：① 对局域网/公网可达的事实；② 引导尽快完成 setup（明示 setup 前
// 界面无密码保护）；③ 给出改回仅本机访问的精确操作。措辞中性、建设性，
// 不用 WARN 级别字样 —— 新默认值下每次启动都触发，WARN 会误导为配置出错。
func exposureNotice(port int, cfgPath string) string {
	return fmt.Sprintf(`提示：frp_easy UI 当前监听 0.0.0.0:%d，局域网/公网内的设备均可访问本管理界面。
  · 请尽快用浏览器打开 UI 完成 setup 向导，创建管理员账号（完成 setup 前界面无密码保护）。
  · frp_easy 已内置认证加固：argon2id 密码哈希、会话 Cookie、CSRF 防护、登录失败限流。
  · 如仅需本机访问，可编辑 %s，将 UIBindAddr 改为 "127.0.0.1" 后重启。
`, port, cfgPath)
}

func envOr(key, dflt string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return dflt
}

// isAddrInUse 通过错误链识别"端口已被占用"。
// 同时覆盖 Windows (WSAEADDRINUSE) 与 Linux (EADDRINUSE) + 文本 fallback。
func isAddrInUse(err error) bool {
	var oe *net.OpError
	if errors.As(err, &oe) {
		var se *os.SyscallError
		if errors.As(oe.Err, &se) {
			// syscall.EADDRINUSE 在两个平台都有同名常量。
			if errors.Is(se.Err, syscall.EADDRINUSE) {
				return true
			}
		}
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "address already in use") ||
		strings.Contains(s, "only one usage of each socket") || // Windows wording
		strings.Contains(s, "bind:") && strings.Contains(s, "in use") {
		return true
	}
	return false
}

// newLogger 构造 slog logger：tee 到 logWriter（若 nil 则仅 stderr）。
// logWriter 实际是 *lumberjack.Logger 的 io.WriteCloser 包装（T-010 logrotate）。
func newLogger(logWriter io.Writer) *slog.Logger {
	if logWriter != nil {
		w := io.MultiWriter(logWriter, os.Stderr)
		return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

// frpcAdminCreds 是 frpc admin API 凭据持久化结构。
type frpcAdminCreds struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

const kvFrpcAdmin = "frpc.admin"

// ensureFrpcAdminCreds 读 kv.frpc.admin；不存在则生成 + 持久化。
func ensureFrpcAdminCreds(store *storage.Store, logger *slog.Logger) frpcAdminCreds {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if v, ok, err := store.KVGet(ctx, kvFrpcAdmin); err == nil && ok {
		var c frpcAdminCreds
		if json.Unmarshal([]byte(v), &c) == nil && c.Port > 0 {
			return c
		}
	}
	user, _ := auth.GenerateCSRFToken()
	pass, _ := auth.GenerateCSRFToken()
	c := frpcAdminCreds{
		Addr: "127.0.0.1",
		Port: 7400,
		User: "frp_easy_" + user[:8],
		Pass: pass,
	}
	b, _ := json.Marshal(c)
	if err := store.KVSet(ctx, kvFrpcAdmin, string(b)); err != nil {
		logger.Warn("persist frpc admin creds failed", "err", err)
	}
	return c
}

// autoRestoreProcs 按 kv.mode.frpc.enabled / kv.mode.frps.enabled 自动 Start
// （AC-9）。二进制缺失则记 warn 不报错。configPaths 用于 TOML 预检（OPT-8）。
func autoRestoreProcs(store *storage.Store, pm *procmgr.Manager, loc binloc.Locator, logger *slog.Logger, configPaths map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	missing := map[string]bool{}
	for _, k := range loc.Missing() {
		missing[k] = true
	}
	for _, kind := range []string{"frpc", "frps"} {
		v, ok, err := store.KVGet(ctx, "mode."+kind+".enabled")
		if err != nil || !ok {
			continue
		}
		b, _ := strconv.ParseBool(v)
		if !b {
			continue
		}
		if missing[kind] {
			logger.Warn("auto-restore skipped: binary missing", "kind", kind)
			continue
		}
		// TOML 预检：配置文件不存在时跳过（避免子进程立即以 error 状态退出）
		if tomlPath, ok := configPaths[kind]; ok {
			if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
				logger.Warn("auto-restore skipped: config file missing", "kind", kind, "path", tomlPath)
				continue
			}
		}
		// 注意：本期 main 不重写 frpc.toml/frps.toml；若 runtime 目录里没文件
		// 子进程会启动失败 → procmgr 把 state 标 error。这是已知行为，前端
		// 引导用户先编辑 server/client/proxies 后再开 mode 开关。
		if _, err := pm.Start(kind); err != nil {
			logger.Warn("auto-restore failed", "kind", kind, "err", err)
		}
	}
}
