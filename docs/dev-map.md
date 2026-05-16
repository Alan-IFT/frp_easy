# 开发导航 — frp_easy

> 项目结构和约定导航。**新增 / 移动 / 删除模块时，请同步更新这里**。
>
> Developer agent 写代码前会读这个文件，避免重复造已经存在的轮子。

## 目录布局

（随项目增长持续更新。）

```
frp_easy/
├── (项目根目录)
├── .claude/        ← AI 配置（不要把 secret 提交到这里）
├── docs/           ← SPEC、feature 文档、本导航、任务看板
├── scripts/        ← verify_all、start、build、baseline、sync 辅助
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
│   ├── downloader/ ← frp 二进制自动下载（T-002）：异步下载 tar.gz/zip，进度追踪，原子安装
│   ├── frpcadmin/  ← frpc admin HTTP 客户端（/api/reload、/api/status）
│   ├── frpconf/    ← DB → TOML 渲染器（AtomicWrite；camelCase 字段对齐 FRP 上游）
│   ├── httpapi/    ← chi router + 全部 REST handler（T-001: 22 条；T-002: +5 条）+ 中间件链
│   ├── logtail/    ← TailLines + ReadFrom 增量读取子进程日志文件
│   ├── procmgr/    ← frpc/frps 子进程生命周期（supervisor goroutine；跨平台 kill）
│   └── storage/    ← SQLite 句柄 + 迁移引擎 + DAO（admin / sessions / kv / proxies）
└── web/            ← 前端 Vue 3 + Vite（dev-frontend 分区）
    ├── index.html
    ├── package.json / vite.config.ts / tsconfig.json / vitest.config.ts / .eslintrc.cjs
    └── src/
        ├── main.ts         ← app 入口；Pinia・Router 組み立て・CSRF トークンゲッター登録
        ├── App.vue         ← ルートコンポーネント（router-view のみ）
        ├── router.ts       ← Vue Router 4 (history mode)；ナビゲーションガード
        ├── types.ts        ← バックエンド API 契約と一致する型定義（Proxy / ProcessInfo 等）
        ├── api/            ← axios クライアント + エンドポイント別ラッパー
        │   ├── client.ts   ← axios インスタンス；CSRF インターセプター；401 リダイレクト
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
        ├── stores/         ← Pinia ストア
        │   ├── auth.ts     ← user / csrfToken；login / logout / checkMe / fetchCsrf
        │   ├── proc.ts     ← frpc/frps ProcessInfo；2s ポーリング
        │   ├── proxies.ts  ← Proxy[] CRUD
        │   ├── app.ts      ← initialized / binMissing / version
        │   ├── downloader.ts← frpc/frps DownloadState；1s ポーリング；downloadBin/startPolling
        │   ├── wizard.ts   ← wizardHandled / shouldShow / checked；checkWizard / completeWizard
        │   └── __tests__/  ← Vitest ストアテスト
        ├── composables/    ← 再利用ロジック
        │   ├── statusUtils.ts  ← getTagType / getStateLabel (ProcessState → Naive UI 色)
        │   └── useProxyForm.ts ← ProxyForm フォームロジック (isTcpUdp / isHttpHttps 等)
        ├── components/
        │   ├── AppLayout.vue    ← サイドナビ + ヘッダ + コンテンツ共通レイアウト（T-002: 下載ボタン追加）
        │   ├── StatusBadge.vue  ← ProcessState → 色付き NTag
        │   ├── ProxyForm.vue    ← Proxy 新規/編集フォーム（type 連動フィールド切り替え）
        │   ├── ConfirmDialog.vue← 破壊的操作の二次確認モーダル
        │   ├── LogViewer.vue    ← ログ表示（TailLines 初期表示 + 2s 増分ポーリング）
        │   ├── FirewallHint.vue ← T-002: Linux ufw/iptables コマンドヒント（ports[] props）
        │   ├── PublicIpDetector.vue← T-002: 公網 IP 検出ボタン + 結果表示
        │   └── __tests__/      ← Vitest コンポーネントテスト
        └── pages/
            ├── Setup.vue     ← 初回セットアップ（username + password）
            ├── Login.vue     ← ログイン（429 カウントダウン対応）
            ├── Dashboard.vue ← frpc/frps 状態バッジ + 起動/停止/再起動ボタン
            ├── Proxies.vue   ← Proxy 一覧 + 新規/編集/削除（T-002: FirewallHint 追加）
            ├── Server.vue    ← frps 設定フォーム（T-002: PublicIpDetector + FirewallHint 追加）
            ├── Client.vue    ← frpc 接続設定フォーム（serverAddr / serverPort / authToken）
            ├── Logs.vue      ← ログビューア（LogViewer コンポーネント利用）
            ├── Settings.vue  ← パスワード変更フォーム
            └── Wizard.vue    ← T-002: 部署向导（トップレベルルート /wizard，3ステップ）
```

## 功能在哪里

| 功能区域 | 文件 | 约定 |
|---|---|---|
| 程序入口 | `cmd/frp-easy/main.go` | 启动序列：appconf → storage → binloc/procmgr/ratelimiter → HTTP server → ReadyGate → AC-9 自动恢复。 |
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
