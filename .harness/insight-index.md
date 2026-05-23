# Insight Index — frp_easy

> 项目踩坑学到的跨任务真相。≤30 行。
> 设计/实现任务开始时读；只在证据支持的意外之后写。
> 规则见 `.harness/rules/05-insight-index.md`。

<!-- 追加新 insight 写下面，一行一条。格式：
-->
- 2026-05-16 · 向导页面必须是顶层路由（非 AppLayout 子路由），否则侧边栏导航干扰向导流程 · evidence: zero-config-quickstart
- 2026-05-16 · openapi.yaml 字段名应以 Go 常量为权威（直接读 .go），不以设计文档草稿为准；status 枚举值在设计阶段写错（done/error vs success/failed），Gate Review 捕获 · evidence: docs-and-api-schema
- 2026-05-17 · Naive UI 凡使用 useMessage/useDialog 等 composable 的组件，App.vue 根组件必须包裹对应 Provider；缺失时 headless 浏览器中 setup() 抛异常，组件输出空节点 `<!-->`，表单不可见 · evidence: e2e-smoke-tests
- 2026-05-17 · go:embed 将 dist/ 静态快照嵌入二进制，前端重建后必须重新 go build；E2E 启动脚本用 find dist/ -newer $BIN 时间戳检查驱动重建，是最轻量的解决方案 · evidence: e2e-smoke-tests
- **2026-05-19** · vitest module resolution 在 .ts/.js 共存时优先加载 .js；historical `tsc` 残留的 .js/.d.ts 会让改 .ts 测试看似无效果且无报错。开发前清理 `find web/src -type f \( -name '*.js' -o -name '*.js.map' \) -delete` · evidence: hardening-pass-audit
- **2026-05-19** · modernc.org/sqlite 的 UNIQUE 约束错误文本格式为 `UNIQUE constraint failed: <table>.<column>`，区分大小写；用 strings.Contains 双关键字（"UNIQUE constraint failed" + "<table>.<column>"）能精确区分表内多个 UNIQUE 列的冲突 · evidence: hardening-pass-audit
- **2026-05-19** · Go AtomicWrite 双重 Chmod 模式（tmp + final）必须在 rename 前后两处都 chmod，仅 chmod tmp 时 rename 后 umask 可能让最终文件权限变宽 · evidence: hardening-pass-audit
- **2026-05-23** · systemd unit `ExecStart=` 与 `WorkingDirectory=` 必须裸 token 写入（`ExecStart=/opt/frp-easy/frp-easy`），含空格路径用 C-style 字面 `\x20` 转义（`/opt/frp\x20easy/v1`）；整体双引号写法 `WorkingDirectory="/path"` 被 systemd 任意版本拒为 bad unit file setting（T-008 旧 insight 错误已纠正，原文搬至 `docs/features/_archived/insight-history.md`）；systemd-analyze verify 可作 daemon-reload 前自检但在 systemd 249-255 偶有对合法 unit 误报 fatal，应取 "warn+继续" 降级而非 fatal+rm · evidence: T-016 install-progress-and-systemd-unit-fix
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
- verify_all E.6 要求已完成任务的 06_TEST_REPORT.md 含**精确英文标题** `## Adversarial tests`；即使项目输出语言规则为中文，该段标题也必须用英文（可在英文标题后括注中文）。QA 若写 `## 对抗性测试` 会导致 E.6 FAIL、pass_count 掉到 18。证据：本任务 stage 7 首次 verify_all。
- `curl|bash` / `irm|iex` 管道形态下脚本无磁盘路径，禁用 `$0`/`${BASH_SOURCE[0]}`/`$PSScriptRoot` 自定位；正确做法是一切路径锚定"固定安装目录 + mktemp 临时目录"两个显式绝对路径，被复用的子脚本（install-service.*）则因是磁盘文件可正常自定位。证据：本任务 install.sh/install.ps1 设计。
- GitHub API 未认证请求的限流响应（HTTP 403）响应体是合法 JSON；查询 release 必须"先判 HTTP 状态码、后解析 JSON"，且查询步骤不能用 `curl -f`（否则 403/404 直接变 curl 错误，丢失分流能力）。证据：本任务 install.sh API 步骤。
- `softprops/action-gh-release@v2`（实测 v2.6.2）**没有** `clean_release_attachments` 输入参数，且对已存在的 release 不会自动把底层 git tag ref 移到新 commit。滚动发布（固定 tag 反复移动 + 资产名含 commit hash 每次不同名）必须自己加 `git tag -f` step 移 tag、加 `gh release delete-asset` step 清旧资产。证据：本任务 04 对 action 的源码级核实。
- GitHub Actions `concurrency.group` 用于滚动发布时必须含 `${{ github.ref }}`；否则 main 分支触发与 `v*` tag 触发会落入同一并发组、`cancel-in-progress` 会让两类发布互相取消。证据：本任务 release.yml 设计。
- 改为下载 frp "latest" 后，frp 版本不再受 frp_easy 控制。未来 frp 大版本若变更 TOML schema，`internal/frpconf` 渲染的 frpc.toml/frps.toml 可能被新版 frpc/frps 拒绝、导致子进程启动失败。本期按 out-of-scope 不做版本适配；后续若出现兼容性问题，需引入 frp 版本探测/适配或锁定已知兼容版本。证据：T-014 设计 §5 R-2。
- GitHub API 查询 `fatedier/frp` releases 必须带 `User-Agent` 头，否则被 GitHub 拒为 403；且 `http.Client`（不同于 curl）对 4xx/5xx 不返回 error，可天然"先判 resp.StatusCode 再解析 JSON"。证据：T-014 downloader.go resolveLatestAsset。
- **2026-05-23** · bash 双引号 + parameter expansion 的 quote-removal 陷阱：`"${p// /\\x20}"` 中 REPLACEMENT 段的 `\\` 先被 quote-removal 还原为单 `\`、再被 expansion 解析吞掉，结果丢反斜杠（实测 `frpx20easy`）；要让 REPLACEMENT 含字面反斜杠须用 4 反斜杠 `"${p// /\\\\x20}"` 或先存到单引号变量 `local esc='\x20'; "${p// /$esc}"`。验证字符级替换必须 verbatim source committed 函数，不能复用"等价" ad-hoc 测试脚本 · evidence: T-016 install-progress-and-systemd-unit-fix D-1
- **2026-05-23** · install.sh 解包后必须对运行时可写路径（frp_easy.toml、.frp_easy/、frp_linux/）chown 给 RUN_USER（systemd `User=` 同款 `${SUDO_USER:-$(id -un)}` 两段式），否则 systemd 进程以 RUN_USER 启动时 appconf.Load() 写默认配置失败 → permission denied → 死循环重启。修复模式：解包后局部 chown（绝不全量 `chown -R /opt/<app>/`）+ 预生成 frp_easy.toml 让 appconf 走"已存在"分支 · evidence: T-017 install-role-and-public-ip
- **2026-05-23** · 公网 IP 探测在国内 VM 上 api.ipify.org / ifconfig.me / icanhazip.com 三常用候选有高概率全部失败；必须提供用户手动覆盖通道（`FRP_EASY_PUBLIC_IP` 环境变量 + 函数首行 short-circuit）+ 失败横幅显式打印"登云控制台复制出口 IP"提示。仅靠多候选 URL 轮询在国内环境不够 · evidence: T-017 install-role-and-public-ip
- **2026-05-23** · 前端 TS 接口与后端 Go struct 的 JSON 字段名漂移在双方 mock 测试都 PASS 时无法被捕获；本任务出现两处 P0：`size↔sizeBytes` 与 `basename↔namePrefix`，前端 spec mock 用自定字段名，后端单测用 OpenAPI 字段名，各自绿但生产必崩。补救：spec 测试用 OpenAPI codegen（如 openapi-typescript）做"契约一锤定音"，而非两边各自从 OpenAPI 抄一遍 · evidence: T-018 05_CODE_REVIEW P0-1/P0-2
- **2026-05-23** · `scripts/verify_all.sh` 的 E.6 regex 是 `^##\s+Adversarial\s+tests`，**不允许数字编号前缀**（如 `## 2. Adversarial tests` 会 FAIL）；写 QA 06 时标题必须是裸 `## Adversarial tests` 不带任何前缀 · evidence: T-018 verify_all 首跑 E.6 FAIL
- **2026-05-23** · gate-reviewer / code-reviewer 等 review 类 sub-agent 倾向把完整 review 内容返回到消息体而不写入对应 `0X_*.md` 文件；派发时 prompt 必须显式 "必须直接写到 <文件名> 文件" 才稳；否则 PM 要手工落盘 · evidence: T-018 stage 3/5 两次 reviewer 不落盘
