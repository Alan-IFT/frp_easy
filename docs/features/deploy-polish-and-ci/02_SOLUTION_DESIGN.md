# 02 — 方案设计：T-010 deploy-polish-and-ci

> Stage 2 of 7 · PM-authored

---

## 1. 总体策略

4 条独立工作线，按风险从小到大排：

| Line | 目标 | 范围 | 依赖 |
|---|---|---|---|
| L1 | `<ORG>` 占位符消除 | `docs/DEPLOYMENT.md`、`scripts/install-service.sh` | 无 |
| L2 | 日志轮转 | 新包 `internal/logrotate/`、`cmd/frp-easy/main.go` | lumberjack v2 |
| L3 | 浏览器自动打开 | 新包 `internal/browseropen/`、`cmd/frp-easy/main.go` | golang.org/x/term（go.mod 已有？） |
| L4 | GitHub Actions release | 新文件 `.github/workflows/release.yml` | 无 |

L1 / L4 是纯文本；L2 / L3 是 Go 新包 + main.go 接线。互不依赖，可任意顺序实施；按上表顺序提交得到最干净的 diff 视图。

## 2. L1 — `<ORG>` 占位符消除

### 2.1 命中位置

`Alan-IFT` 替换：

- `docs/DEPLOYMENT.md`
  - 第 14-17 行：占位符约定表（保留 `<INSTALL_DIR>` 与 `<VERSION>`，删除 `<ORG>` 行——因不再是占位符）
  - 第 41 行：`https://github.com/<ORG>/frp_easy/releases/latest` → `https://github.com/Alan-IFT/frp_easy/releases/latest`
  - 第 151 行：`git clone https://github.com/<ORG>/frp_easy.git` → 同上 git URL
- `scripts/install-service.sh`
  - 第 115 行：`Documentation=https://github.com/<ORG>/frp_easy` → 同上 URL（systemd Documentation 字段）

### 2.2 不动

- `docs/features/_archived/` 下所有历史文档（约定不改归档）
- `01_REQUIREMENT_ANALYSIS.md` 后续生成的文档（PM/Architect 在新文档中直接写 `Alan-IFT`）

### 2.3 行为变更

无（仅文本）。systemd Documentation 字段被 `systemctl status` 读出后显示真实 URL。

## 3. L2 — 日志轮转

### 3.1 新包结构

```
internal/logrotate/
  logrotate.go      ── 包装 lumberjack.Logger，对外返回 io.WriteCloser
  logrotate_test.go ── 覆盖写满轮转 + 权限保持
```

### 3.2 API

```go
package logrotate

// Options 是轮转参数；从环境变量装填，未设的取默认。
type Options struct {
    Path       string // 完整路径，必填
    MaxSizeMB  int    // 单文件上限 MB，默认 10
    MaxBackups int    // 历史份数，默认 5
    MaxAgeDays int    // 最长保留天数，默认 30
    Mode       os.FileMode // 文件权限，默认 0o600
}

// New 返回轮转 writer；调用方负责 Close。
func New(opts Options) (io.WriteCloser, error)

// LoadOptionsFromEnv 从 FRP_EASY_LOG_MAX_* 环境变量覆盖默认。
func LoadOptionsFromEnv(path string) Options
```

### 3.3 实现

```go
// 简化实现
func New(opts Options) (io.WriteCloser, error) {
    if opts.MaxSizeMB == 0 { opts.MaxSizeMB = 10 }
    if opts.MaxBackups == 0 { opts.MaxBackups = 5 }
    if opts.MaxAgeDays == 0 { opts.MaxAgeDays = 30 }
    if opts.Mode == 0 { opts.Mode = 0o600 }
    // 提前创建并 chmod，让 lumberjack 后续 reopen 走同权限
    f, err := os.OpenFile(opts.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, opts.Mode)
    if err != nil { return nil, err }
    _ = f.Close()
    _ = os.Chmod(opts.Path, opts.Mode)
    return &lumberjack.Logger{
        Filename:   opts.Path,
        MaxSize:    opts.MaxSizeMB,
        MaxBackups: opts.MaxBackups,
        MaxAge:     opts.MaxAgeDays,
        Compress:   false, // 简化运维：gzip 历史不易 grep
    }, nil
}
```

### 3.4 main.go 接线

替换当前 `os.OpenFile(uiLogPath, ...)`：

```diff
- logFile, _ := os.OpenFile(uiLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
- if logFile != nil { _ = os.Chmod(uiLogPath, 0o600) }
- logger := newLogger(logFile)
+ logWriter, err := logrotate.New(logrotate.LoadOptionsFromEnv(uiLogPath))
+ if err != nil { fmt.Fprintf(os.Stderr, "WARN: 日志轮转初始化失败 %v；将仅写 stderr\n", err) }
+ defer func() { if logWriter != nil { _ = logWriter.Close() } }()
+ logger := newLogger(logWriter) // logWriter 为 nil 时 fallback 到 stderr-only
```

`newLogger` 签名稍改：`func newLogger(w io.Writer) *slog.Logger`。

### 3.5 测试

- `TestNew_RotatesOnSize`：写超 1 MB，期望产生 .1 历史文件
- `TestNew_PreservesPermissions`：写入后 Stat().Mode() == 0o600（Linux 主机；Windows 跳过 mode 检查）
- `TestLoadOptionsFromEnv_OverridesDefaults`：set env → opts 字段反映

## 4. L3 — 浏览器自动打开

### 4.1 新包结构

```
internal/browseropen/
  browseropen.go      ── Open(url) + 平台分支 + WARN 日志
  browseropen_test.go ── interactive TTY / 非 TTY mock + 平台命令选择
```

### 4.2 API

```go
package browseropen

// Open 调用平台默认浏览器打开 URL。成功返回 nil，失败返回 err（调用方决定是否 WARN）。
func Open(url string) error

// ShouldOpen 决定当前进程是否应当自动打开浏览器。
// 规则：interactive TTY && FRP_EASY_NO_BROWSER 未设 && --no-browser flag 未传。
// flag 通过参数注入避免本包反向依赖 cmd 包。
func ShouldOpen(noBrowserFlag bool) bool
```

### 4.3 实现

```go
func Open(url string) error {
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "windows":
        // rundll32 url.dll,FileProtocolHandler <url> 是最稳的 Windows 打开方式；
        // start "" "<url>" 在含特殊字符 URL 时 cmd 引号转义易错。
        cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
    case "darwin":
        cmd = exec.Command("open", url)
    default: // linux, freebsd, ...
        cmd = exec.Command("xdg-open", url)
    }
    return cmd.Start()
}

func ShouldOpen(noBrowserFlag bool) bool {
    if noBrowserFlag { return false }
    if os.Getenv("FRP_EASY_NO_BROWSER") != "" { return false }
    // systemd / Windows Service 通常 stdin 非 TTY → 天然 false
    return term.IsTerminal(int(os.Stdin.Fd()))
}
```

### 4.4 main.go 接线

flag 解析阶段新增 `noBrowser bool`：

```go
fs.BoolVar(&noBrowser, "no-browser", false, "")
```

启动序列尾部、`ready.Store(true)` 之后：

```go
if browseropen.ShouldOpen(noBrowser) {
    url := fmt.Sprintf("http://%s", addr)
    if cfg.UIBindAddr == "0.0.0.0" || cfg.UIBindAddr == "::" {
        url = fmt.Sprintf("http://127.0.0.1:%d", cfg.UIPort)
    }
    if err := browseropen.Open(url); err != nil {
        logger.Warn("auto-open browser failed", "err", err, "url", url)
    } else {
        logger.Info("opened browser", "url", url)
    }
}
```

### 4.5 测试

- `TestShouldOpen_NoBrowserFlag`：传 `true` → 期望 false
- `TestShouldOpen_EnvVar`：set `FRP_EASY_NO_BROWSER=1` → 期望 false（teardown unset）
- `TestOpen_CommandSelection`：把 `exec.Command` mock 化（用接口 `commander` 注入），三平台 path 命中正确 argv[0]
  - 实现路径：在测试里把全局 `var commandFunc = exec.Command` 替换为闭包，断言收到的 name + args
  - 不真的执行系统命令（CI 没有桌面）

### 4.6 usageText 更新

```
选项:
  -h, --help          显示本帮助并退出
  -v, --version       显示版本号并退出
      --no-browser    启动后不自动打开浏览器（默认 TTY 启动时打开）

环境变量:
  FRP_EASY_CONFIG              配置文件路径（默认 ./frp_easy.toml）
  FRP_EASY_NO_BROWSER          设为非空值禁用自动打开浏览器
  FRP_EASY_LOG_MAX_SIZE_MB     单 ui.log 上限 MB（默认 10）
  FRP_EASY_LOG_MAX_BACKUPS     ui.log 历史份数（默认 5）
  FRP_EASY_LOG_MAX_AGE_DAYS    ui.log 最长保留天数（默认 30）
```

## 5. L4 — GitHub Actions release workflow

### 5.1 文件 `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags: [ 'v*' ]

permissions:
  contents: write  # 上传 release assets 需要

jobs:
  build-and-release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json

      - name: Build (Linux + Windows)
        run: bash scripts/build.sh --all

      - name: Package (Linux + Windows)
        run: bash scripts/package.sh --windows

      - name: Compute version from tag
        id: ver
        run: echo "version=${GITHUB_REF_NAME#v}" >> "$GITHUB_OUTPUT"

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          name: ${{ steps.ver.outputs.version }}
          files: |
            bin/release/*.tar.gz
            bin/release/*.zip
          generate_release_notes: true
```

### 5.2 触发

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions 自动跑，2-3 分钟后 Releases 页面出 `frp-easy-0.1.0-linux-amd64.tar.gz` + `frp-easy-0.1.0-windows-amd64.zip`。

### 5.3 不在本期范围

- 测试 workflow（lint/test on PR）：用户没要求，scope creep
- 多 Go 版本矩阵：1.22 足够
- macOS / ARM：用户没要求

### 5.4 验证

PR / push 阶段不触发；只有 `v*` tag push 触发。本期 verify_all 不直接验证 workflow 执行；workflow 语法正确性靠 GitHub UI 在用户推 tag 时反馈。

## 6. dev-map 更新

追加：

```
├── .github/workflows/release.yml ← T-010 新增：tag push 触发构建并发布到 GitHub Releases
└── internal/
    ├── browseropen/  ← T-010 新增：跨平台浏览器打开（rundll32/open/xdg-open）+ TTY 检测
    └── logrotate/    ← T-010 新增：基于 lumberjack 的 ui.log 轮转（size + backups + age）
```

## 7. 依赖矩阵

| 包 | 版本 | 用途 | 许可 |
|---|---|---|---|
| `gopkg.in/natefinch/lumberjack.v2` | v2.x | 日志轮转 | MIT |
| `golang.org/x/term` | latest | TTY 检测 | BSD-3 |

两者纯 Go 无 cgo；模块图增量极小。`go mod tidy` 后 go.sum 增加 ~10 行。

## 8. 实施顺序

1. L1（占位符）— 单 commit 文本改动
2. L2（logrotate）— 新增包 + 测试 + main.go 接线
3. L3（browseropen）— 新增包 + 测试 + main.go flag + usageText
4. L4（CI workflow）— 单文件新增
5. dev-map 更新（一次性写完整段）
6. verify_all 全绿

## 9. 回退预案

若任一线 broken：

- L1：revert 单 commit
- L2：main.go 回 `os.OpenFile`；删除新包；`go mod tidy`
- L3：main.go 删 ShouldOpen 调用 + flag；删除新包
- L4：删除 workflow 文件
