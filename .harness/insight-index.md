# Insight Index — frp_easy

> 项目踩坑学到的跨任务真相。≤30 行。
> 设计/实现任务开始时读；只在证据支持的意外之后写。
> 规则见 `.harness/rules/05-insight-index.md`。

<!-- 追加新 insight 写下面，一行一条。格式：
- YYYY-MM-DD · <一句话事实> · evidence: <任务 slug 或 commit sha>
-->
- 2026-05-16 · Windows os.Rename 不能覆盖已存在文件，需先 Remove 再 Rename；但 Remove 成功后 Rename 失败会丢失原文件，正确模式是先试 Rename 失败再 Remove+Rename · evidence: zero-config-quickstart
- 2026-05-16 · 向导页面必须是顶层路由（非 AppLayout 子路由），否则侧边栏导航干扰向导流程 · evidence: zero-config-quickstart
- 2026-05-16 · openapi.yaml 字段名应以 Go 常量为权威（直接读 .go），不以设计文档草稿为准；status 枚举值在设计阶段写错（done/error vs success/failed），Gate Review 捕获 · evidence: docs-and-api-schema
- 2026-05-17 · Naive UI 凡使用 useMessage/useDialog 等 composable 的组件，App.vue 根组件必须包裹对应 Provider；缺失时 headless 浏览器中 setup() 抛异常，组件输出空节点 `<!-->`，表单不可见 · evidence: e2e-smoke-tests
- 2026-05-17 · go:embed 将 dist/ 静态快照嵌入二进制，前端重建后必须重新 go build；E2E 启动脚本用 find dist/ -newer $BIN 时间戳检查驱动重建，是最轻量的解决方案 · evidence: e2e-smoke-tests
- **2026-05-19** · vitest module resolution 在 .ts/.js 共存时优先加载 .js；historical `tsc` 残留的 .js/.d.ts 会让改 .ts 测试看似无效果且无报错。开发前清理 `find web/src -type f \( -name '*.js' -o -name '*.js.map' \) -delete` · evidence: hardening-pass-audit
- **2026-05-19** · modernc.org/sqlite 的 UNIQUE 约束错误文本格式为 `UNIQUE constraint failed: <table>.<column>`，区分大小写；用 strings.Contains 双关键字（"UNIQUE constraint failed" + "<table>.<column>"）能精确区分表内多个 UNIQUE 列的冲突 · evidence: hardening-pass-audit
- **2026-05-19** · Go AtomicWrite 双重 Chmod 模式（tmp + final）必须在 rename 前后两处都 chmod，仅 chmod tmp 时 rename 后 umask 可能让最终文件权限变宽 · evidence: hardening-pass-audit
- **2026-05-19** · systemd unit 中 `ExecStart=${PATH}` 与 `WorkingDirectory=${PATH}` 必须用双引号包路径（`ExecStart="${PATH}"`，systemd 5.0+ 语法），否则路径含空格时 systemd 按空格分参导致启动失败。Code Review MAJOR-1 直接证据 · evidence: T-008 deploy-kit
- **2026-05-19** · Windows Service 通过 sc.exe 创建时，binPath 指向 wrapper.cmd 包装而非 .exe 本身可锁定 cwd（`cd /d "$InstallDir" && "$BinaryPath"`），但 `Set-Content -Encoding ASCII` 写 .cmd 会让中文路径乱码，需 `-Encoding Default`（host codepage） · evidence: T-008 deploy-kit
- **2026-05-19** · Go stdlib `flag.NewFlagSet(name, flag.ContinueOnError)` + `fs.SetOutput(io.Discard)` 是中文化 / 自定义错误输出的标准范式；显式 `errors.Is(err, flag.ErrHelp)` 分流 `-help` 单 dash 形式仍可触发（非死代码），与已注册 `-h`/`--help` BoolVar 不冲突 · evidence: T-008 deploy-kit
- **2026-05-19** · verify_all A.1 secrets scan 正则 `(api_key|secret|password|token)[\s]*[:=][\s]*["'][^"']{8,}["']` 会误中文档/脚本内的样例字面量；写 `frp_easy.toml.example` 之类时只列字段名 = 默认值，避免任何 8+ 字符引号串 · evidence: T-008 deploy-kit
- **2026-05-19** · `sudo` 调用 bash 脚本时 `id -un` 返回 root；要拿到真实调用者用 `${SUDO_USER:-$(id -un)}` 优先 `$SUDO_USER` 才符合 "默认 user = 当前调用者" 的意图 · evidence: T-008 deploy-kit
- **2026-05-19** · Windows PowerShell 的 `bash` 命令默认解析到 `C:\Users\<user>\AppData\Local\Microsoft\WindowsApps\bash.exe`（WSL shim），即使 Git Bash 已安装；用户未装 Linux 发行版时返回乱码错误。Playwright `webServer.command: 'bash ...'` 在 PowerShell 调用链下会失败。修复模式：`process.platform === 'win32' ? 'pwsh ... -File .ps1' : 'bash .sh'` 双脚本配对 · evidence: T-009 polish-pass
- **2026-05-19** · PowerShell 写 TOML 配置文件必须用 `[System.IO.File]::WriteAllText($path, $content, [System.Text.UTF8Encoding]::new($false))` 强制无 BOM；`Out-File -Encoding utf8` 在 Windows PowerShell 5.x 会写 BOM 让 BurntSushi/toml 解析失败（PS7 默认无 BOM 但项目支持 PS5 时仍要显式） · evidence: T-009 polish-pass
- **2026-05-19** · TOML 字符串中 Windows 反斜杠路径会被当转义；写脚本生成的临时 TOML 配置时 `-replace '\\' '/'` 把所有反斜杠换正斜杠是最简单的方式，Go 在 Windows 上同样接受正斜杠路径 · evidence: T-009 polish-pass
- **2026-05-19** · `npm exec <pkg> --someflag` 中 `--someflag` 会被 npm 自己当成 flag 吞掉（output `npm warn Unknown cli config "--someflag"`），子进程实际收不到。必须用 `npm exec -- <pkg> --someflag`（`--` 分隔符强制透传）。T-010 verify_all B.1 的 `npm exec tsc --noEmit` 被静默 emit 反复污染 web/src/，根因即此 · evidence: T-010 deploy-polish-and-ci
- **2026-05-19** · `web/.gitignore` 含 `web/src/**/*.js` 时，`git status` / `git ls-files` 看不到 tsc 残留产物 —— 让"已清残留"自检永远通过、Reviewer 才能用 Glob 抓到。验证残留清理务必用 `find ... | wc -l` 而非 `git status` · evidence: T-010 deploy-polish-and-ci
- **2026-05-19** · `tsconfig.json` 设 `"noEmit": true` 是真正一劳永逸防 tsc 误 emit 的方式；调用方加 `--noEmit` flag 是 belt，tsconfig 是 suspenders。新项目应当默认两者都有 · evidence: T-010 deploy-polish-and-ci
- **2026-05-19** · Go 跨平台 `runtime.GOOS switch` 的单测如果直接读 `runtime.GOOS`，不论分支多漂亮都只能测当前主机的那一支 —— 必须用 `var goosFunc = func() string { return runtime.GOOS }` 这种可注入 seam 配 stubGOOS helper 才能 table-driven 跑遍三平台。其他平台不变量同理（`os.Getenv` → `getenvFunc`、`os.Stdin.Fd()` → `stdinFd`） · evidence: T-010 deploy-polish-and-ci
- **2026-05-19** · GitHub Actions `actions/setup-go@v5` 的 `go-version` 应当与 `go.mod` 顶部 `go X.Y` 对齐；不对齐时 setup-go 拉指定版本后又被 `GOTOOLCHAIN=auto`（默认 Go 1.21+ 行为）二次拉真实版本，CI 时间翻倍 · evidence: T-010 deploy-polish-and-ci
