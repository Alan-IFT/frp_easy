# 开发导航 — frp_easy

> 项目结构和约定导航。**新增 / 移动 / 删除模块时，请同步更新这里**。
>
> Developer agent 写代码前会读这个文件，避免重复造已经存在的轮子。

## 目录布局

（随项目增长持续更新。）

```
frp_easy/
├── (项目根目录)
├── README.md       ← 用户入口文档（标准开源结构：简介/亮点/快速开始/配置/端口/许可证；T-011 重写；T-012 快速开始置顶一键安装、许可证改 MIT）
├── LICENSE         ← MIT 许可证全文（T-012 新增；Copyright (c) 2026 Alan_IFT）
├── NOTICE          ← 上游 frp 二进制 Apache-2.0 归属说明（T-012 新增；中文）
├── openapi.yaml    ← REST API OpenAPI 3.0.3 规范（T-005 新增；覆盖全部 REST 路由，含 T-038 service-status / T-039 服务端运行态 / T-040 allowPorts；30 个 path，T-049 补齐 service-status 后与 router.go 对齐）
├── .claude/        ← AI 配置（不要把 secret 提交到这里）
├── .github/
│   └── workflows/release.yml  ← T-010 新增 / T-013 改造：push main → 刷新固定 tag `rolling` 滚动发布；push v* tag → 版本化发布（两路径共存）
├── docs/           ← SPEC、feature 文档、本导航、任务看板
│   ├── DEPLOYMENT.md       ← 部署权威文档：路径 A 发布包 / 路径 B 源码 / 路径 C 系统服务（T-008 新增）
│   ├── project-status.html  ← 项目状况总览（技术栈/功能/债务/建议，T-003 新增；T-011 刷新到 T-010 实际）
│   └── architecture.html    ← 架构总览（分层架构/模块详解/数据流/API，T-005 新增；T-011 刷新到 T-010 实际）
├── scripts/        ← verify_all、start、build、baseline、sync 辅助；start-e2e-server.{sh,ps1}（T-006 sh / T-009 ps1，PowerShell 调用路径）；
│                     package.{sh,ps1} / install-service.{sh,ps1} / uninstall-service.{sh,ps1}（T-008 新增）；
│                     install.{sh,ps1}（T-012 新增 / T-013 改：一键安装编排脚本，curl|bash / irm|iex 形态；查 releases/tags/rolling 滚动发布 → 解压 → 调 install-service.* 注册服务）
│                     T-021：scripts/*.ps1 全部 11 个统一 UTF-8 BOM（首 3 字节 EF BB BF），让 PS 5.1 + zh-CN 主机磁盘加载形态正确解码中文；verify_all E.7 + scripts/.editorconfig 双层防回归（git blob 字节为持久层、editorconfig 为编辑器层 belt）
│                     T-026：scripts/install.ps1 因 iex 入口**禁** BOM（irm 把 EF BB BF 解码为 U+FEFF 进入字符串触发 ParserError）；其余 10 个 .ps1 继续要 BOM（磁盘形态防 GBK 误解码）；主体 `& { ... } @PSBoundParameters` 子作用域包裹让 `exit N` 在交互式宿主下退子作用域不杀宿主；verify_all E.7 拆 a/b/c 白名单（E.7a 必须 BOM / E.7b 禁 BOM / E.7c anti-drift WARN）；scripts/.editorconfig 追加 [install.ps1] charset=utf-8 例外块覆盖 [*.ps1] charset=utf-8-bom
│                     T-031：scripts/install.ps1 末尾 `exit 0` 改为 `$global:LASTEXITCODE = 0` 防御性置零（消解 R4 假设；`$global:` 修饰符让 child scope 写穿到根 scope）；README Windows 推荐入口改 `pwsh -NoExit -Command "irm ... | iex"`（PowerShell `-NoExit` about_PowerShell_Exe 文档化语义）让 cmd / Win+R / Windows Terminal 入口窗口不在脚本结束时关闭；PS5.1 段同步给 `powershell -NoExit` 替代；install-service.ps1 字节零变（OQ-5 a）；verify_all 新 E.8 (forbid Read-Host/[Console]::ReadKey/裸 pause/Wait-Event in install.ps1+install-service.ps1, FR-3 红线；单走 Select-String 按行扫描天然 multiline) / E.9 (forbid wrapper.cmd/install*.bat) / E.10 (README -NoExit) 三闸门，PASS 21→24（Quick 实测）
├── migrations/     ← SQLite 迁移（权威源；NNNN_<slug>.up.sql / .down.sql）
├── cmd/frp-easy/   ← Go 程序入口（main.go；单二进制）
│                     T-019：service_windows.go（//go:build windows）实现 Windows Service ABI
│                     双入口分流（main.go 顶端 isWindowsService() → runService() 走 SCM 状态机；
│                     run(stopCh, readyCh) 两参签名，控制台分支传 nil 退化）；
│                     service_other.go（//go:build !windows）非 Windows 平台空 stub；
│                     service_windows_test.go 守护脚本契约（wrapper.cmd 不再生成 + 卸载防御性清理保留）
├── bin/            ← 构建产物（gitignore；build.ps1/build.sh 输出到这里）
├── frp_win/        ← frp 二进制运行时下载落地目录（T-014：不再内置，仅 LICENSE 占位；downloader 下载落地于此）
├── frp_linux/      ← frp 二进制运行时下载落地目录（T-014：不再内置，仅 LICENSE 占位；downloader 下载落地于此）
├── internal/       ← Go 业务代码（按子包分区，私有于本模块）
│   ├── appconf/    ← 读写 frp_easy.toml（UI 服务自身配置，非 FRP 配置）
│   ├── assets/     ← embed.FS 占位（dev 模式返回 404；Round 2 挂 dist/）
│   ├── auth/       ← argon2id 哈希 / 会话 token / CSRF token / IP 限流
│   ├── binloc/     ← runtime.GOOS 选 frp_win/ 或 frp_linux/ 二进制路径
│   ├── browseropen/← T-010 新增：跨平台浏览器自动打开（rundll32/open/xdg-open）+ TTY 检测 + --no-browser/env opt-out
│   ├── downloader/ ← frp 二进制自动下载（T-002）：异步下载 tar.gz/zip，进度追踪，原子安装
│   ├── frpcadmin/  ← frpc admin HTTP 客户端（/api/reload、/api/status）
│   ├── frpsadmin/  ← frps admin HTTP 客户端（T-039：/api/serverinfo、/api/proxy/{type}、/api/proxy/{type}/{name}、/api/traffic/{name}）
│   ├── frpconf/    ← DB → TOML 渲染器（AtomicWrite；camelCase 字段对齐 FRP 上游）
│   ├── httpapi/    ← chi router + 全部 REST handler + 中间件链（路由增删累计见下方"功能在哪里"表 router.go 行）
│   ├── logrotate/  ← T-010 新增：基于 lumberjack 的 ui.log 三轴轮转（size/backups/age）+ FRP_EASY_LOG_MAX_* env 覆盖
│   ├── logtail/    ← TailLines + ReadFrom 增量读取子进程日志文件
│   ├── procmgr/    ← frpc/frps 子进程生命周期（supervisor goroutine；跨平台 kill）
│   ├── svcprobe/   ← T-038：服务化状态探测（systemd / Windows Service 监管 / 开机自启 / 运行用户）；build tag 分平台
│   └── storage/    ← SQLite 句柄 + 迁移引擎 + DAO（admin / sessions / kv / proxies）
└── web/            ← 前端 Vue 3 + Vite（dev-frontend 分区）
    ├── index.html
    ├── package.json / vite.config.ts / tsconfig.json / vitest.config.ts / .eslintrc.cjs
    ├── playwright.config.ts   ← T-006 新增：Playwright E2E 配置（webServer + chromium project）
    ├── tests/
    │   └── e2e/               ← T-006 新增：Playwright E2E 烟雾测试
    │       ├── 01-setup.spec.ts    ← TC-01（未初始化跳转）、TC-02（setup 表单提交）
    │       ├── 02-auth.spec.ts     ← TC-03（login 表单提交）
    │       ├── 03-dashboard.spec.ts← TC-04（dashboard 元素可见）、TC-05（退出登录）
    │       └── fixtures/
    │           └── auth.ts         ← programmaticLogin / bypassWizard / setupAccount / programmaticLogout
    └── src/
        ├── main.ts         ← app 入口；组合 Pinia / Router；注册 CSRF token getter
        ├── App.vue         ← 根组件（NConfigProvider + NMessageProvider 包裹；T-006 修复；T-066: <n-config-provider :theme=activeTheme> 绑 useTheme + 新增 <n-global-style/>（config-provider 内）让 body 背景随主题切换——全站暗色主题接线点）
        ├── router.ts       ← Vue Router 4（history mode）；导航守卫
        ├── types.ts        ← 与后端 API 契约一致的类型定义（Proxy / ProcessInfo 等）
        ├── api/            ← axios 客户端 + 按端点分组的封装
        │   ├── client.ts   ← axios 实例；CSRF 拦截器；401 重定向
        │   ├── auth.ts     ← /api/v1/auth/* / /api/v1/setup
        │   ├── system.ts   ← /api/v1/system/ready, /api/v1/system/public-ip; T-018: +apiUploadBin (multipart)
        │   ├── proxies.ts  ← /api/v1/proxies CRUD（单条新增/编辑/删除；T-037 移除批量）
        │   ├── server.ts   ← /api/v1/server (FrpsConfig)
        │   ├── serverRuntime.ts ← T-041：/api/v1/server/runtime/{info,proxies,traffic/{name}} 客户端，消费 T-039 后端 REST
        │   ├── frpclient.ts← /api/v1/client (FrpcServerConn)
        │   ├── proc.ts     ← /api/v1/proc/{kind}/start|stop|restart, /proc/status
        │   ├── logs.ts     ← /api/v1/logs/{kind} tail / incremental
        │   ├── mode.ts     ← /api/v1/mode
        │   ├── downloader.ts← /api/v1/system/download-bin + download-status/{kind}
        │   └── wizard.ts   ← /api/v1/wizard/status + /wizard/complete
        ├── stores/         ← Pinia store
        │   ├── auth.ts     ← user / csrfToken；login / logout / checkMe / fetchCsrf
        │   ├── proc.ts     ← frpc/frps ProcessInfo；2s 轮询
        │   ├── proxies.ts  ← Proxy[] CRUD（T-047: fetchProxies 自捕获并暴露 error ref，区分"加载失败" vs "暂无规则"）
        │   ├── app.ts      ← initialized / binMissing / version
        │   ├── downloader.ts← frpc/frps DownloadState；1s 轮询；downloadBin/startPolling
        │   ├── wizard.ts   ← wizardHandled / shouldShow / checked；checkWizard / completeWizard
        │   └── __tests__/  ← Vitest store 测试（T-051 +2：proxies CRUD/error ref、wizard 状态流转）
        ├── composables/    ← 可复用逻辑
        │   ├── useTheme.ts     ← T-066：全局暗色主题状态层（模块级单例）。pref('light'|'dark'|'auto') + activeTheme(computed null|darkTheme) + isDark + setPref；THEME_STORAGE_KEY='frpEasy.themePref'；DEFAULT_PREF='auto'；内置 createSafeStorage(BC-13 内存降级，复刻 useLogPrefs 范式)；auto 经 useOsTheme 跟随系统（osThemeRef 惰性首调在 App.vue setup）。App.vue + AppLayout 共享单例。
        │   ├── statusUtils.ts  ← getTagType / getStateLabel（ProcessState → Naive UI 颜色）
        │   ├── useProxyForm.ts ← ProxyForm 表单逻辑（isTcpUdp / isHttpHttps 等）
        │   ├── usePortPresets.ts ← T-018 §C.2：常用端口预设清单（SSH/RDP/HTTP/HTTPS/MySQL/PostgreSQL/Redis/MongoDB/SMB/VNC）；前端 hardcode
        │   ├── useServerRuntime.ts ← T-041：frps 运行态轮询 composable（双 endpoint Promise.all + epoch race + 3 次失败自动停 + visibilitychange 自管 listener + onUnmounted 清理 timer/listener；T-042 也复用）
        │   └── log/             ← T-036：日志查看器纯逻辑层（5 个 composable + 1 个解析器；
        │                          parseLogLine 双格式 OR regex；useLogBuffer slice(-500) + kindEpoch race；
        │                          useLogSearch indexOf + 大小写敏感；useLogLevelFilter 6 等级；
        │                          useFollowTail 32px 阈值状态机；useLogPrefs localStorage + BC-13 内存降级）
        ├── utils/          ← T-042 抽取：format.ts（formatBytes + formatTime；T-048 formatTime 统一本地化）+ proxyStatus.ts（getProxyStatusTag）
        ├── test-utils/     ← T-043 测试 helper：exposed.ts（getExposed 读 defineExpose，避开 VTU exposeProxy 脆弱性）+ apiError.ts（构造 axios 形状错误模拟后端响应）
        ├── components/
        │   ├── AppLayout.vue    ← 侧边导航 + 头部 + 内容公用布局（T-002: 新增下载按钮；T-018: banner 内追加 UploadBinButton；T-064: 菜单图标 a11y——抽 menuIcon(glyph,name) helper，7 项字形两两互不相同（'设置' ⚙→⚒ 消除与'服务端配置'折叠态撞车）+ 每 icon span 挂 aria-label/title/role=img 无障碍名）
        │   ├── StatusBadge.vue  ← ProcessState → 带颜色的 NTag
        │   ├── ProxyForm.vue    ← Proxy 新增/编辑表单（type 联动字段切换；T-018: 加端口预设 Tag；T-037: 移除批量模式 / 端口探测按钮；T-062: tcp/udp 远程端口字段加纯文案提示「需在服务端端口策略允许范围内」，不读 allowPorts 不联动校验）
        │   ├── UploadBinButton.vue ← T-018 §A：手动上传 frpc/frps 二进制（multipart；进度条；前端 64 MiB 预校验）
        │   ├── AllowPortsEditor.vue ← T-040 端口策略编辑器：单端口 / 范围混合 list + 实时校验 + defineExpose getAllowPortsInput()/hasValidationError()；继承 T-032 单向数据流范式
        │   ├── ConfirmDialog.vue← 破坏性操作二次确认弹窗
        │   ├── LogViewer.vue    ← T-036 重写：日志查看器壳组件（持有 5 composable 实例 + 协调 4 子组件 + watch kind 切换；< 200 行）
        │   ├── log/             ← T-036：LogViewer 子组件（4 个）；
        │   │   ├── LogToolbar.vue        ← 工具条：搜索 + Aa + 等级多选 + 跟随 / 折行 / 自动刷新 + 高度 + 复制 / 清屏 / ↓底部 / 全屏 + 心跳 / 计数 / 失败小红点
        │   │   ├── LogList.vue           ← 滚动容器 + 5 状态分支（错误 / 加载中 / 空态 / 无命中 / 列表）+ 暂停跟随提示条 sticky banner（T-064: 滚动容器加 tabindex=0 + role=log + aria-label，纯键盘可聚焦滚动；范式对齐 paused-banner a11y）
        │   │   ├── LogLine.vue           ← 单行渲染：行号 + timestamp + level + message；先 escape 后 mark 包裹（NFR-7 / ADV-A）
        │   │   └── FullscreenLogModal.vue← n-modal 全屏包装 LogList；95vw/90vh 走 scoped :deep(.n-card) 无 inline style
        │   ├── FirewallHint.vue ← T-002: Linux ufw/iptables 命令提示（ports[] props）；T-058 (A): copyCmd/copyAll 剪贴板失败不再静默——1:1 内联 LogViewer onCopy 范式（clipboard→message.success / catch textarea+execCommand fallback → 成功 success / 失败 error）；T-064: 复制按钮加 aria-live=polite 播报复制结果
        │   ├── PublicIpDetector.vue← T-002: 公网 IP 检测按钮 + 结果显示；T-058 (A): copyIp 同款剪贴板失败反馈；T-064: 复制按钮加 aria-live=polite
        │   └── __tests__/      ← Vitest 组件测试（T-036 +6 个：LogViewer / parseLogLine / useLogBuffer / useLogSearch / useFollowTail / useLogPrefs；T-051 +1：useLogLevelFilter）
        └── pages/
            ├── Setup.vue     ← 首次安装（username + password）
            ├── Login.vue     ← 登录（429 倒计时支持）
            ├── Dashboard.vue ← frpc/frps 状态徽章 + 启动/停止/重启按钮（T-047: 自动启动开关获取失败不再静默 → warning + 失败态开关 disabled + tooltip + 刷新入口；T-056: 停止/重启破坏性操作复用 ConfirmDialog 二次确认，pendingAction 状态机驱动动态文案，启动不确认）
            ├── Proxies.vue   ← Proxy 列表 + 新增/编辑/删除（T-002: 新增 FirewallHint；T-037: 退回一行一条直接渲染，移除折叠分组；T-042: 叠加 runtime 列「运行状态 / 流量（入/出）」，消费 useServerRuntime；frps 不可达时降级灰点 + 配置 CRUD 通路零关联；T-047: 区分加载失败 n-result+重试 vs 暂无规则 empty 态；T-062: 保存成功 showPostSaveHint 显示「去仪表盘启动 frpc→」+「去服务端监控查看运行态→」引导（删除不触发），#empty 补「去服务端监控」连通入口，导航用 router.push）
            ├── Server.vue    ← frps 配置表单（T-002: 新增 PublicIpDetector + FirewallHint；T-040: 端口策略段 AllowPortsEditor；T-047: 加载三态 skeleton/n-result+重试/loaded + Dashboard 三字段补校验；T-058 (B): 「重置」→「重新加载」，加载存标量快照 loadedSnapshot，dirty 时弹 ConfirmDialog 防误丢未保存编辑，不 dirty 直接重载；T-060: dirty 检测已纳入 AllowPortsEditor 端口策略（normalizeAllowPorts 规范化字符串快照 loadedAllowPortsSnapshot + isDirty 末尾比较编辑器 getAllowPortsInput()，消除"只改端口策略→静默丢弃"路径，保留单向数据流不引 v-model 桥）；T-062: loaded 态 #action 加「查看运行态→」push('/server/monitor')，与 ServerMonitor goServerConfig→push('/server') 双向连通，加载失败/中态不显示）
            ├── ServerMonitor.vue ← T-041：frps 服务端运行态监控页（消费 T-039 API；5s 轮询 + visibilitychange 自动暂停；ServerInfo 卡片 + n-tabs 分 type proxy 表格 + 状态条 + 三态完备）
            ├── Client.vue    ← frpc 连接配置表单（serverAddr / serverPort / authToken；T-047: 加载三态 skeleton/n-result+重试/loaded；T-058 (B): 「重置」→「重新加载」+ dirty 弹 ConfirmDialog 防误丢，同 Server.vue 范式；T-062: handleSave 成功 showNextStepHint 显示「前往代理规则添加端口→」push('/proxies') 引导，失败不显示）
            ├── Logs.vue      ← 日志查看器（使用 LogViewer 组件）
            ├── Settings.vue  ← 修改密码表单
            └── Wizard.vue    ← T-002: 部署向导（顶级路由 /wizard，3 步；不在 AppLayout 内→顶栏缺失横幅向导阶段不可见）；T-057: 完成保存配置+开启自动启动后、跳转前 await appStore.fetchReady() 刷新 binMissing，按所选角色（frpc/frps/both）算缺失交集 missingForRole——缺失则不自动跳走、不发「正在跳转」toast，step3 就地 warning alert + 「进入仪表盘」手动按钮（引导用户去仪表盘顶栏横幅下载/上传）；不缺失维持原自动跳转。binWarning 用 ref 定格快照；T-058 (C): step2 frpc 标题死分支清理（原 v-if='both'/v-else 两分支文案相同 → 合并单个无条件 n-text，外层 div v-if 已控可见性，零行为变化）；T-062: step3 全就绪分支（frpc/both）加「前往代理规则添加端口」按钮 push('/proxies') 引导（缺失分支不加，与 T-057 自动跳转并存不破坏）+ step2 both 模式 frps/frpc token 不一致非阻断 warning（computed tokenMismatch，不阻止 handleNext、不进 configError）
```

## 功能在哪里

| 功能区域 | 文件 | 约定 |
|---|---|---|
| 程序入口 | `cmd/frp-easy/main.go` | 启动序列：appconf → storage → logrotate(ui.log) → binloc/procmgr/ratelimiter → HTTP server → ReadyGate → AC-9 自动恢复 → 可选浏览器自动打开。T-019：main() 顶端 `isWindowsService()` 分流；run() 签名扩展为 `run(stopCh <-chan struct{}, readyCh chan<- struct{}) error`，控制台传 nil 等价于现有行为；ready.Store(true) 后 close(readyCh)；signal select 追加 `case <-stopCh:` 第三路。T-046/T-063：`go purgeLoop(rootCtx, store, rl, logger)` 后台清理 goroutine——同一 goroutine/ticker（`sessionPurgeInterval`=1h）启动即清 + 周期清理过期 session（`purgeExpiredSessionsOnce`）与过期 loginfail 限流计数行（`purgeExpiredLoginFailsOnce`→`rl.PurgeExpired`），随 rootCtx 取消退出，不新增 goroutine。 |
| Windows Service ABI | `cmd/frp-easy/service_windows.go` / `service_other.go` | T-019 新增。Windows 文件实现 `serviceHandler.Execute`：START_PENDING + 1s CheckPoint 心跳 + WaitHint=5s → 收到 readyCh close 切 RUNNING (CheckPoint=0/WaitHint=0) → 主循环 select SCM Stop 控制码 → 触发 close(stopCh) → STOP_PENDING (WaitHint=30s) → 等 run() 返回 → STOPPED。`runService()` 起手 `os.Chdir(filepath.Dir(os.Executable()))` 锁 cwd（替代旧 wrapper.cmd 的 cd /d，UTF-16 原生不依赖 host codepage）。非 Windows 文件提供 `isWindowsService() bool { return false }` + `runService() error` 空 stub。 |
| 应用配置（UI 服务自身） | `internal/appconf/config.go` | `AppConfig{UIBindAddr,UIPort,DataDir,LogDir}`；Load/Validate/ListenAddr；frp_easy.toml 不存在时写默认。 |
| 嵌入前端资源（占位） | `internal/assets/assets.go` | dev 阶段返回 404 占位；Round 2 改成 `//go:embed all:dist`。 |
| 密码哈希 / 限流 | `internal/auth/` | `HashPassword`(argon2id m=64MiB/t=3/p=2) / `VerifyPassword` / `GenerateSessionToken` / `GenerateCSRFToken` / `RateLimiter`(5次/60s per IP 基于 kv)。T-063：`RateLimiter.PurgeExpired(ctx) (int,error)` 周期清理过期 `loginfail.<ip>` 计数行（过期判定 `now.After(FirstAt+failWindow)` 与 `Allow` 同源，删除集 ⊆ 惰性清理集绝不删活计数；损坏 JSON 值视为过期垃圾删除）；导出 `LoginFailKeyPrefix='loginfail.'`；`kvStore` 接口扩 `KVListByPrefix`（auth→storage 单向依赖无环）。 |
| FRP 二进制定位 | `internal/binloc/binloc.go` | `NewDefault(root)` 按 `runtime.GOOS` 选 frp_win/frp_linux；Missing() 反馈缺失项（AC-13）。 |
| FRP 二进制自动下载（T-002 / T-014） | `internal/downloader/downloader.go` | `New(root, logger) *Manager`；`Start(kind) error`；`Status(kind) (DownloadState, bool)`。异步 goroutine 下载 tar.gz/zip，io.TeeReader 追踪进度，原子 rename 安装，Zip Slip 防御（R-2）。T-014：改为下载 fatedier/frp 最新 release（GitHub API 解析 latest tag，`resolveLatestAsset`），不再用写死的 `FRPVersion`。 |
| frpc admin 客户端 | `internal/frpcadmin/client.go` | `Reload(ctx, strictConfig)` / `Status(ctx)`；5s 超时；basic auth。 |
| 服务端运行态监控页 | `web/src/pages/ServerMonitor.vue` | T-041 新增。useServerRuntime composable 持有 info / proxies / isPolling / error，5s setInterval polling + visibilitychange 自动暂停 + 3 次失败自动停 + epoch race 保护 unmount in-flight。表格按 type tabs 分组；status 三色 dot（toLowerCase 防御大小写）；流量人类友好单位（B/KiB/MiB/GiB/TiB）。 |
| frps admin 客户端（T-039） | `internal/frpsadmin/client.go` | `ServerInfo(ctx)` / `Proxies(ctx, type)` / `ProxyDetail(ctx, type, name)` / `Traffic(ctx, name)`；5s 超时；basic auth；sentinel error `ErrUnauthorized` / `ErrNotFound` / `ErrUnavailable`。 |
| DB → TOML 渲染 | `internal/frpconf/render.go` | `RenderFrpc` / `RenderFrps` / `AtomicWrite`；字段名严格对齐 FRP camelCase TOML 上游（见 02 附录 A）。T-040：`RenderFrps` 支持 `allowPorts` 数组段渲染 + 导出 `ValidateFrpsAllowPorts`（互斥 / 范围 / 闭区间重叠 / 上限 100）。 |
| HTTP 路由层 | `internal/httpapi/router.go` | chi router；中间件链 ReadyGate→Recover→RequestID→Logger(C-5脱敏)→CORS(dev)→SessionAuth→CSRF。T-001: 22 条路由；T-002: +5 条（public-ip, download-bin, download-status/{kind}, wizard/status, wizard/complete）；T-018: +1 条（system/upload-bin）；T-037: 移除 system/probe-ports + proxies/batch；T-038: +1 条（system/service-status）；T-039: +4 条（server/runtime/info|proxies|proxy/{type}/{name}|traffic/{name}）；T-040: `FrpsConfig` schema 扩 `allowPorts: []AllowPortRange`，`PUT /api/v1/server` 校验互斥 / 范围 / 重叠 / 上限。 |
| HTTP 错误映射（sentinel 收口） | `internal/httpapi/handlers_proc.go` `mapProcErr` / `handlers_proxies.go` `mapProxyWriteError` / `handlers_proc.go` `writeInternalError` | 均为 `*handlers` 方法。统一范式：下层错误用 `errors.Is(sentinel)` 分类（不在 handler 匹配驱动/内部英文文本），面向前端固定中文文案，500 兜底走 `writeInternalError`（固定中文 + 原始 error 进 logger，不外泄内部细节）。`mapProcErr`（T-065）：`binloc.ErrBinMissing`→422 BIN_MISSING / `procmgr.ErrBusy`→409 PROC_BUSY「进程正忙…」/ else→500 INTERNAL「操作进程失败」。`mapProxyWriteError`（T-059）：`storage.ErrDuplicateName`→409 / `storage.ErrDuplicateRemotePort`→422 / validation→422 固定中文 / else→500。`writeInternalError`（T-055，nil logger 守卫）：3 处 500 兜底共用。 |
| 日志尾部读取 | `internal/logtail/tail.go` | `TailLines(path, n)` / `ReadFrom(path, offset)` 增量 + 新 offset。 |
| 子进程生命周期 | `internal/procmgr/manager.go` | `Manager`；Start/Stop/Restart/Status/Shutdown；supervisor goroutine；Windows Kill / Linux SIGTERM→SIGKILL；ApplyConfigChange(frpc→reload/restart；frps→restart)。哨兵错误：`ErrBusy`（T-065，进程处于过渡/活动态而拒绝操作——当前唯一来源是 Start 的 StateStopping 分支，`fmt.Errorf("...: %w", ErrBusy)` 保留可读 cause 进日志 + `errors.Is` 可判；StateStarting/StateRunning 是 idempotent 不报错。handler `mapProcErr` 仅 `errors.Is(ErrBusy)` 判 409 PROC_BUSY，不再匹配内部英文文本）。 |
| 服务化状态探测（T-038） | `internal/svcprobe/probe.go` + `probe_<os>.go` | `Probe(ctx) Status{Supervised, Supervisor, BootAutostart, RunAs}`；Linux 用 `$INVOCATION_ID + systemctl is-enabled`，Windows 用 `svc.IsWindowsService + sc.exe qc`；build tag 分平台；探测失败降级 supervised=false（fail-safe）。 |
| 数据库迁移（权威 SQL） | `migrations/0001_init.up.sql` / `0001_init.down.sql` | 文件名 `NNNN_<slug>.up.sql / .down.sql`。**绝不修改已合并的迁移**；新改动 = 新文件。dev-db 分区 owned。 |
| 持久化层（连接 / 迁移引擎 / DAO） | `internal/storage/*.go` | Go 包 `storage`。对外 API：`Open / Close / DataDir / GetAdmin / SetAdmin / CreateSession / GetSession / DeleteSession / PurgeExpiredSessions / KVGet / KVSet / KVDelete / KVListByPrefix / ListProxies / GetProxy / UpsertProxy / DeleteProxy`。`KVListByPrefix(ctx,prefix) ([]KVEntry,error)`（T-063）是**机械**按前缀列举 KV（`LIKE escapeLike(prefix)+'%' ESCAPE '\'` + `ORDER BY key`，转义 `\ % _` 防元字符通配），不懂业务/过期语义——过期判定由调用方（`auth.RateLimiter.PurgeExpired`）按限流窗口自行做。类型 `KVEntry{Key,Value string}`。哨兵错误：`ErrCorruptReset` / `ErrVersionConflict` / `ErrNotFound` / `ErrDuplicateName`（name 列 UNIQUE→handler 409） / `ErrDuplicateRemotePort`（(type,remote_port) 组合 UNIQUE→handler 422，T-059）。SQL 驱动错误文本匹配（`isDuplicateNameError`/`isDuplicateRemotePortError`）只在本包；handler 仅 `errors.Is` 判 sentinel。dev-db owned；**其它包不写 SQL**。 |
| 迁移嵌入副本（给 Go 编译器） | `internal/storage/sqlmigrations/*.sql` | 是 `migrations/` 的字节级镜像；用 `//go:embed` 编入二进制。`TestEmbeddedMigrations_MatchDisk` 防 drift。两份必须一致。 |

## 可复用工具

| 需求 | 已有 | 文件 | 备注 |
|---|---|---|---|
| SQLite 连接 + 自动迁移 | 是 | `internal/storage/store.go` `Open(dataDir)` | 启动时探测 `PRAGMA integrity_check`；非 ok 改名 `data.db.broken-<UTC ts>` 重建，返回 `ErrCorruptReset`（AC-12）。 |
| 管理员凭据持久化 | 是 | `internal/storage/admin.go` | `Admin{Username, PasswordHash, UpdatedAt}`；表 CHECK(id=1) 单行硬约束。 |
| 会话存储 | 是 | `internal/storage/sessions.go` | `crypto/rand` 生成 32B base64url token + csrf token；TTL 由调用方决定。 |
| 通用 KV | 是 | `internal/storage/kv.go` | 文本 key-value；mode 开关、frps 配置、frpc 服务器连接、登录失败计数都走这里。 |
| Proxy CRUD + 乐观锁 | 是 | `internal/storage/proxies.go` | `UpsertProxy` 带 last-write-wins（Version 不匹配 → ErrVersionConflict）。 |
| 密码 hash + 验证 | 是 | `internal/auth/hash.go` | argon2id PHC 串；VerifyPassword 常数时间比对。 |
| Session/CSRF token 生成 | 是 | `internal/auth/token.go` | `crypto/rand` 32B base64url。 |
| IP 速率限制 | 是 | `internal/auth/ratelimit.go` | per-IP 5次/60s 滑窗；基于 kv 持久化失败计数；返回 retryAfter。 |
| FRP 二进制路径 | 是 | `internal/binloc/binloc.go` | NewDefault("") 自动推算仓库根目录。 |
| frpc reload / status | 是 | `internal/frpcadmin/client.go` | `New(addr, port, user, pass)` 构建客户端；Reload / Status。 |
| frps 运行态轮询（双 endpoint + epoch race） | 是 | `web/src/composables/useServerRuntime.ts` | T-041 引入。`useServerRuntime(intervalMs=5000)` → `{ info, proxies, isPolling, error, lastUpdated, consecutiveFailCount, start, stop, refresh, restart }`。F-5.6 失败保留上次数据；F-5.7 不在 mount 自动 start；BC-7 用户显式 stop 后 visibility 恢复不自动 resume。T-042 也消费。 |
| FRP TOML 渲染 | 是 | `internal/frpconf/render.go` | RenderFrpc / RenderFrps；AtomicWrite 原子写文件。 |
| 字节友好单位 / 时间空值格式化 | 是 | `web/src/utils/format.ts` | T-042 抽取。`formatBytes(n)`（B/KiB/MiB/GiB/TiB/PiB；undefined/null/NaN/负数 → "—"；0 → "0 B"）+ `formatTime(s)`（空/"0001-..." → "—"）。共享方：ServerMonitor.vue + Proxies.vue runtime 列。 |
| proxy runtime status → 视觉/文案 | 是 | `web/src/utils/proxyStatus.ts` | T-042 抽取。`getProxyStatusTag(raw)` → `{type, text, dotColor, online}`。大小写防御 + 空字符串归 "离线"（与"无此 proxy"语义合并）。共享方：ServerMonitor.vue 状态列 + Proxies.vue runtime 列。 |
| 复制文本到剪贴板（含 fallback） | 是 | `web/src/utils/clipboard.ts` | T-061 抽取（偿还 T-058 (A) backlog）。`copyToClipboard(text): Promise<boolean>`——首选 `navigator.clipboard.writeText`（安全上下文），失败回落临时离屏 `aria-hidden` textarea + `document.execCommand('copy')`；返回成功布尔，**不调 message**（message 留组件 setup 层，`useMessage` 是组合式 hook）。内网 http 非安全上下文必走 fallback（insight L37）。共享方：LogViewer.vue::onCopy + FirewallHint.vue::copyText + PublicIpDetector.vue::copyText。 |
| 全局暗色主题状态（偏好/持久化/跟随系统） | 是 | `web/src/composables/useTheme.ts` | T-066 新增。`useTheme()` → `{ pref, activeTheme(null=浅/darkTheme=暗), isDark, setPref }`。模块级单例，App.vue 绑 NConfigProvider :theme + AppLayout 顶栏 n-select 三态切换共享。localStorage key `frpEasy.themePref`，缺失/非法降级 DEFAULT_PREF='auto'；localStorage 不可用内存降级（BC-13）；auto 经 `useOsTheme` 跟随系统。darkTheme/useOsTheme 均 naive-ui 内置（零依赖）。注意 useOsTheme 须在 setup 内首调（App.vue 保证）。 |
| HTTP 中间件（全套） | 是 | `internal/httpapi/middleware.go` | ReadyGate(C-3) / Recover / RequestID / Logger(C-5) / CORS / SessionAuth / CSRF。 |
| 日志尾读 | 是 | `internal/logtail/tail.go` | TailLines / ReadFrom 增量。 |
| 子进程管理 | 是 | `internal/procmgr/manager.go` | Start/Stop/Restart/Status/Shutdown/ApplyConfigChange。 |
| ui.log 轮转 | 是 | `internal/logrotate/logrotate.go` | `New(opts)` 返回 io.WriteCloser；环境变量 `FRP_EASY_LOG_MAX_SIZE_MB/_BACKUPS/_AGE_DAYS` 覆盖默认（10/5/30）；权限恒 0o600。 |
| 浏览器自动打开 | 是 | `internal/browseropen/browseropen.go` | `ShouldOpen(noBrowserFlag) bool` + `Open(url) error`；TTY 检测 + env `FRP_EASY_NO_BROWSER` opt-out；Linux 缺 xdg-open 自动跳过。 |

## 要遵循的模式

- **所有 SQL 写在 `internal/storage/`**，其它包通过函数调用访问。
- **每个 schema 改动 = 新 migration 文件**；权威源 `migrations/`，嵌入副本 `internal/storage/sqlmigrations/`。两份字节一致由 `TestEmbeddedMigrations_MatchDisk` 守护。
- **回滚命令**：手工执行 `migrations/<n>.down.sql`（sqlite CLI）。
- 时间字段在 DB 存 ISO 文本（`time.RFC3339Nano` 或 SQLite `datetime('now')`），Go 侧用 `helpers.parseSQLiteTime` 双格式兼容解析。
- 单测用 `t.TempDir()` 隔离 DataDir，禁止跨用例共享 db 文件。
- HTTP handler 用 `httptest.NewRecorder` + `httptest.NewRequest` 测试；不依赖真实端口。

## 要避免的模式

- 不要从 handler 里写原始 SQL；走 `internal/storage/` 的 DAO。
- 不要在 `internal/` 子包之外引用 `internal/` 包（Go 可见性规则）。
- 不要修改已合并的 migration 文件（0001_init.up.sql 等）；加新文件。
- 不要把 `frp_easy.toml` 和 `frpc.toml / frps.toml` 搞混；前者是 UI 自身配置，后者由 `internal/frpconf` 渲染。
