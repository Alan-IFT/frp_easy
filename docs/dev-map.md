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
├── openapi.yaml    ← REST API OpenAPI 3.0.3 规范（28 条路由，T-005 新增）
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
├── migrations/     ← SQLite 迁移（权威源；NNNN_<slug>.up.sql / .down.sql）
├── cmd/frp-easy/   ← Go 程序入口（main.go；单二进制）
├── bin/            ← 构建产物（gitignore；build.ps1/build.sh 输出到这里）
├── frp_win/        ← Windows FRP 二进制（vendored，git 保留）
├── frp_linux/      ← Linux FRP 二进制（vendored，git 保留）
├── internal/       ← Go 业务代码（按子包分区，私有于本模块）
│   ├── appconf/    ← 读写 frp_easy.toml（UI 服务自身配置，非 FRP 配置）
│   ├── assets/     ← embed.FS 占位（dev 模式返回 404；Round 2 挂 dist/）
│   ├── auth/       ← argon2id 哈希 / 会话 token / CSRF token / IP 限流
│   ├── binloc/     ← runtime.GOOS 选 frp_win/ 或 frp_linux/ 二进制路径
│   ├── browseropen/← T-010 新增：跨平台浏览器自动打开（rundll32/open/xdg-open）+ TTY 检测 + --no-browser/env opt-out
│   ├── downloader/ ← frp 二进制自动下载（T-002）：异步下载 tar.gz/zip，进度追踪，原子安装
│   ├── frpcadmin/  ← frpc admin HTTP 客户端（/api/reload、/api/status）
│   ├── frpconf/    ← DB → TOML 渲染器（AtomicWrite；camelCase 字段对齐 FRP 上游）
│   ├── httpapi/    ← chi router + 全部 REST handler（T-001: 22 条；T-002: +5 条）+ 中间件链
│   ├── logrotate/  ← T-010 新增：基于 lumberjack 的 ui.log 三轴轮转（size/backups/age）+ FRP_EASY_LOG_MAX_* env 覆盖
│   ├── logtail/    ← TailLines + ReadFrom 增量读取子进程日志文件
│   ├── procmgr/    ← frpc/frps 子进程生命周期（supervisor goroutine；跨平台 kill）
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
        ├── App.vue         ← 根组件（NConfigProvider + NMessageProvider 包裹；T-006 修复）
        ├── router.ts       ← Vue Router 4（history mode）；导航守卫
        ├── types.ts        ← 与后端 API 契约一致的类型定义（Proxy / ProcessInfo 等）
        ├── api/            ← axios 客户端 + 按端点分组的封装
        │   ├── client.ts   ← axios 实例；CSRF 拦截器；401 重定向
        │   ├── auth.ts     ← /api/v1/auth/* / /api/v1/setup
        │   ├── system.ts   ← /api/v1/system/ready, /api/v1/system/public-ip
        │   ├── proxies.ts  ← /api/v1/proxies CRUD
        │   ├── server.ts   ← /api/v1/server (FrpsConfig)
        │   ├── frpclient.ts← /api/v1/client (FrpcServerConn)
        │   ├── proc.ts     ← /api/v1/proc/{kind}/start|stop|restart, /proc/status
        │   ├── logs.ts     ← /api/v1/logs/{kind} tail / incremental
        │   ├── mode.ts     ← /api/v1/mode
        │   ├── downloader.ts← /api/v1/system/download-bin + download-status/{kind}
        │   └── wizard.ts   ← /api/v1/wizard/status + /wizard/complete
        ├── stores/         ← Pinia store
        │   ├── auth.ts     ← user / csrfToken；login / logout / checkMe / fetchCsrf
        │   ├── proc.ts     ← frpc/frps ProcessInfo；2s 轮询
        │   ├── proxies.ts  ← Proxy[] CRUD
        │   ├── app.ts      ← initialized / binMissing / version
        │   ├── downloader.ts← frpc/frps DownloadState；1s 轮询；downloadBin/startPolling
        │   ├── wizard.ts   ← wizardHandled / shouldShow / checked；checkWizard / completeWizard
        │   └── __tests__/  ← Vitest store 测试
        ├── composables/    ← 可复用逻辑
        │   ├── statusUtils.ts  ← getTagType / getStateLabel（ProcessState → Naive UI 颜色）
        │   └── useProxyForm.ts ← ProxyForm 表单逻辑（isTcpUdp / isHttpHttps 等）
        ├── components/
        │   ├── AppLayout.vue    ← 侧边导航 + 头部 + 内容公用布局（T-002: 新增下载按钮）
        │   ├── StatusBadge.vue  ← ProcessState → 带颜色的 NTag
        │   ├── ProxyForm.vue    ← Proxy 新增/编辑表单（type 联动字段切换）
        │   ├── ConfirmDialog.vue← 破坏性操作二次确认弹窗
        │   ├── LogViewer.vue    ← 日志显示（TailLines 首次显示 + 2s 增量轮询）
        │   ├── FirewallHint.vue ← T-002: Linux ufw/iptables 命令提示（ports[] props）
        │   ├── PublicIpDetector.vue← T-002: 公网 IP 检测按钮 + 结果显示
        │   └── __tests__/      ← Vitest 组件测试
        └── pages/
            ├── Setup.vue     ← 首次安装（username + password）
            ├── Login.vue     ← 登录（429 倒计时支持）
            ├── Dashboard.vue ← frpc/frps 状态徽章 + 启动/停止/重启按钮
            ├── Proxies.vue   ← Proxy 列表 + 新增/编辑/删除（T-002: 新增 FirewallHint）
            ├── Server.vue    ← frps 配置表单（T-002: 新增 PublicIpDetector + FirewallHint）
            ├── Client.vue    ← frpc 连接配置表单（serverAddr / serverPort / authToken）
            ├── Logs.vue      ← 日志查看器（使用 LogViewer 组件）
            ├── Settings.vue  ← 修改密码表单
            └── Wizard.vue    ← T-002: 部署向导（顶级路由 /wizard，3 步）
```

## 功能在哪里

| 功能区域 | 文件 | 约定 |
|---|---|---|
| 程序入口 | `cmd/frp-easy/main.go` | 启动序列：appconf → storage → logrotate(ui.log) → binloc/procmgr/ratelimiter → HTTP server → ReadyGate → AC-9 自动恢复 → 可选浏览器自动打开。 |
| 应用配置（UI 服务自身） | `internal/appconf/config.go` | `AppConfig{UIBindAddr,UIPort,DataDir,LogDir}`；Load/Validate/ListenAddr；frp_easy.toml 不存在时写默认。 |
| 嵌入前端资源（占位） | `internal/assets/assets.go` | dev 阶段返回 404 占位；Round 2 改成 `//go:embed all:dist`。 |
| 密码哈希 / 限流 | `internal/auth/` | `HashPassword`(argon2id m=64MiB/t=3/p=2) / `VerifyPassword` / `GenerateSessionToken` / `GenerateCSRFToken` / `RateLimiter`(5次/60s per IP 基于 kv)。 |
| FRP 二进制定位 | `internal/binloc/binloc.go` | `NewDefault(root)` 按 `runtime.GOOS` 选 frp_win/frp_linux；Missing() 反馈缺失项（AC-13）。 |
| FRP 二进制自动下载（T-002） | `internal/downloader/downloader.go` | `New(root, logger) *Manager`；`Start(kind) error`；`Status(kind) (DownloadState, bool)`。异步 goroutine 下载 tar.gz/zip，io.TeeReader 追踪进度，原子 rename 安装，Zip Slip 防御（R-2）。 |
| frpc admin 客户端 | `internal/frpcadmin/client.go` | `Reload(ctx, strictConfig)` / `Status(ctx)`；5s 超时；basic auth。 |
| DB → TOML 渲染 | `internal/frpconf/render.go` | `RenderFrpc` / `RenderFrps` / `AtomicWrite`；字段名严格对齐 FRP camelCase TOML 上游（见 02 附录 A）。 |
| HTTP 路由层 | `internal/httpapi/router.go` | chi router；中间件链 ReadyGate→Recover→RequestID→Logger(C-5脱敏)→CORS(dev)→SessionAuth→CSRF。T-001: 22 条路由；T-002: +5 条（public-ip, download-bin, download-status/{kind}, wizard/status, wizard/complete）。 |
| 日志尾部读取 | `internal/logtail/tail.go` | `TailLines(path, n)` / `ReadFrom(path, offset)` 增量 + 新 offset。 |
| 子进程生命周期 | `internal/procmgr/manager.go` | `Manager`；Start/Stop/Restart/Status/Shutdown；supervisor goroutine；Windows Kill / Linux SIGTERM→SIGKILL；ApplyConfigChange(frpc→reload/restart；frps→restart)。 |
| 数据库迁移（权威 SQL） | `migrations/0001_init.up.sql` / `0001_init.down.sql` | 文件名 `NNNN_<slug>.up.sql / .down.sql`。**绝不修改已合并的迁移**；新改动 = 新文件。dev-db 分区 owned。 |
| 持久化层（连接 / 迁移引擎 / DAO） | `internal/storage/*.go` | Go 包 `storage`。对外 API：`Open / Close / DataDir / GetAdmin / SetAdmin / CreateSession / GetSession / DeleteSession / PurgeExpiredSessions / KVGet / KVSet / KVDelete / ListProxies / GetProxy / UpsertProxy / DeleteProxy`。哨兵错误：`ErrCorruptReset` / `ErrVersionConflict` / `ErrNotFound`。dev-db owned；**其它包不写 SQL**。 |
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
| FRP TOML 渲染 | 是 | `internal/frpconf/render.go` | RenderFrpc / RenderFrps；AtomicWrite 原子写文件。 |
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
