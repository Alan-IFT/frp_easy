# 03 · Gate Review — T-001 · web-ui-mvp

> 模式：`full` · 编写：gate-reviewer · 日期：2026-05-16 · PM 自治模式
> 上游输入（只读）：
> - `docs/features/web-ui-mvp/01_REQUIREMENT_ANALYSIS.md`（verdict=READY）
> - `docs/features/web-ui-mvp/02_SOLUTION_DESIGN.md`（verdict=READY）
> - `docs/features/web-ui-mvp/INPUT.md`、`PM_LOG.md`、`.harness/insight-index.md`（空）
> 决策原则（来自 INPUT.md）：① 用户体验 > ② 软件工程规范 > ③ 长期可维护性

---

## 0. 验证清单（实际跑过的检查）

| 项 | 方法 | 结果 |
|---|---|---|
| 0.1 FRP 二进制 / TOML 存在 | `Glob frp_win/*` `Glob frp_linux/*` | `frp_win/frpc.exe`、`frps.exe`、`frpc.toml`、`frps.toml`、`frp_linux/frpc`、`frps`、`frpc.toml`、`frps.toml` **全部存在**（Win ≈37 MB、Linux ≈36 MB） |
| 0.2 上游 TOML 字段抽样 | `Read frp_win/frpc.toml` 等 | 字段 `serverAddr/serverPort/[[proxies]] name/type/localIP/localPort/remotePort` 与 02 §附录 A.3 一致；`frps.toml` 仅 `bindPort = 7000` |
| 0.3 FRP 字段 / API 上游校验 | context7 `/fatedier/frp` query-docs（2026-05-16 拉取） | 确认 `webServer.port = 7400`、`GET /api/reload[?strictConfig=true]`、`GET /api/status`、`auth.method = "token"` / `auth.token`、camelCase、`[[proxies]]` 数组——与 02 §3.4/§3.6/§附录 A 一致 |
| 0.4 三个 dev-* agent owned-paths | `Read .harness/agents/dev-db.md` `dev-backend.md` `dev-frontend.md` | PM 已按 02 §13.4 提示落地：dev-db owns `migrations/**`、`internal/storage/**`；dev-backend owns `cmd/**`、`internal/{appconf,auth,binloc,frpconf,frpcadmin,httpapi,logtail,procmgr,assets}/**`、`go.mod`、`go.sum`、`scripts/{start,build,verify_all}.{ps1,sh}`、`.gitignore`、`.gitattributes`；dev-frontend owns `web/**` |
| 0.5 verify_all 现状 | `Read scripts/verify_all.{ps1,sh}` | 当前是 npm/pnpm 模板，**无 `go vet/test/build` 步骤**；02 §10.3/§13.1 已显式把"实体化"任务派给 dev-backend，属计划内 edit，非阻塞 |
| 0.6 owned-paths 落空 / 重叠扫描 | 逐行对照 02 §13.1 表 | 见审计维度 #6；**有 1 处文档化遗漏**（详 Finding F-2） |
| 0.7 AC ↔ 设计可追溯映射 | 01 §5 AC-1~AC-15 逐条 → 02 段落 | 15/15 可追溯（详 §3 映射表） |
| 0.8 NF-S1~NF-S6 可追溯 | 01 §6.1 → 02 实现路径 | 6/6 全覆盖（详 §4） |

---

## 1. 8 维度审计

| # | 维度 | 评级 | 理由（一句话） |
|---|---|---|---|
| 1 | Requirement completeness | **PASS** | 01 文档 24 条 In-scope + 15 条 AC + 4.1~4.4 边界 + 6.1~6.5 NF 全部可观察可测，0 BLOCKED ON USER |
| 2 | Design completeness | **PASS** | 02 文档 §1~§14 + 附录 A 齐备，REST 契约 22 条端点、TS 类型、错误码、状态机、TOML 渲染、进程模型逐项落实 |
| 3 | Reuse correctness | **PASS** | 仓库零业务代码现状已实际核查；reuse 项仅"vendored FRP 二进制 + Harness 骨架"两项，与 §8 表一致；新引入的 chi / argon2 / go-toml/v2 / modernc.org/sqlite / Naive UI / Pinia / Vue Router / Axios / Vitest 每项均给出"why" |
| 4 | Risk coverage | **WARN** | 7 条风险 + 缓解到位；但**缺一条"端口 8080 已被占用时如何用户引导改端口"** 的风险条目（Q-10 决策要求"不静默换端口"，但 ops 路径未在 §9 体现）。详 Finding F-3 |
| 5 | Migration safety | **PASS** | 0001_init.up/down.sql 成对、MVP 无历史数据迁移；§4.3 `PRAGMA integrity_check` + 自动改名 `*.broken-<ts>` 实现 AC-12；§11 显式"绝不修改已合并迁移" |
| 6 | Boundary handling | **WARN** | 01 §4.1~§4.4 边界齐备且 02 全部映射到 SQL CHECK / handler 校验；但**并发场景仅覆盖"双 tab 编辑同一 proxy"（R-6）**，未提"UI 服务启动期间持久化加载未完成时拒写"（01 §4.2 第 3 条）在哪个中间件实现。详 Finding F-4 |
| 7 | Test feasibility | **WARN** | AC-1/2/3/4/7/8/10/11/14 均 `curl` + `netstat` + `ps` 可机械化；AC-5/6 借 frpc `GET /api/status` 可自动；AC-9 重启 UI 可脚本化；AC-12/13 需文件级注入；**AC-15 端到端（`ssh -p 6000 user@127.0.0.1` 或 `nc`）依赖测试机有 sshd / nc**，不能在裸 CI 跑通——必须显式标注"人工 / 集成机"。详 Finding F-5 |
| 8 | Out-of-scope clarity | **PASS** | 01 §3 列 13 项 O-*；02 §12 用技术语言重申（不引入消息总线 / 不做 RBAC / 不做 TLS / 不做 SSE）。dev-* 不会误造 |

> **汇总**：PASS = 5、WARN = 3、FAIL = 0。

---

## 2. Findings（按上游文档分配责任）

> 编号 F-N。`severity` ∈ {INFO, WARN, FAIL}。`owner` 指应该回去改的上游 agent，本 gate 不改文档。

### F-1 · INFO · 06 §6.1 加密强度备选参数文档化建议（**非阻塞**）

- **来源**：02 §6.2、R-3
- **现状**：argon2id 参数 `m=64 MiB, t=3, p=2`；R-3 缓解项提到"4C/4G 备选 m=32 MiB"，但**未在 §6.2 主文档体现"如何切换"**。
- **建议**：dev-backend 在 `internal/auth/hash.go` 顶部 doc-comment 给出"低配机器可把常量调成 m=32768"的注释；本期不必引入运行时可调。
- **责任**：dev-backend 实施期注释即可，**不需要回改 02 文档**。

### F-2 · WARN · 02 §13.1 owned-paths 表与 dev-* 文件 owned globs 在 `internal/assets/**` 上文档化不闭环

- **来源**：02 §13.1 第 21 行 `internal/assets/embed.go` 列为 dev-backend；`dev-backend.md` 现 owned 列含 `internal/assets/**` ✅；但 02 §13.2 step-1 派 dev-db 时 dev-backend 还没动 → step-4 才回来补 `embed.go`。**`web/dist/` 产物是 dev-frontend 出，**`internal/assets/dist/` 物理上落在 dev-backend owned 区——dev-frontend 写 dist 文件**算越界**吗？
- **现状**：02 §10.2 vite.config 配 `outDir: '../internal/assets/dist'`，构建命令归 dev-frontend，但写出位置在 dev-backend owned tree（`internal/**`）。
- **建议**：02 应在 §13.4 明确"`internal/assets/dist/` 是**构建产物**，dev-frontend 通过 `npm run build` 写入合法，不算 owned-path 越界"（或反向把 `internal/assets/dist/**` 显式从 dev-backend owned 排除）。
- **责任**：solution-architect 一句澄清；**不阻塞 dev 启动**——可在 PM 派 Stage 4 时口头释疑，dev-frontend 也已在 `.gitignore` §10.5 中把 `internal/assets/dist/` 排除，物理路径生成时机正确。降为 CONDITION。

### F-3 · WARN · 02 §9 Risk 表缺"UI 端口被占"运维风险条目

- **来源**：01 Q-10（8080 被占 → UI 退出 + 打中文错误，不换端口）
- **现状**：02 §6/§9/§11 未把"用户拿到这条中文错误后该怎么办"列入风险或 rollout 条目。dev-backend 实现时容易把消息写得太工程师向（如 `bind: address already in use`）。
- **建议**：dev-backend 在 `main.go` 启动失败路径打印形如：「frp_easy UI 启动失败：端口 8080 已被占用。请关闭占用进程，或编辑 `frp_easy.toml` 中 `UIPort = 8081` 后重试。」属实现层 UX，但**dev-backend 落地时必须遵循**——本 gate 列为 CONDITION。
- **责任**：dev-backend 落地，**无需回改 02**。

### F-4 · WARN · 02 §3.9 中间件链未显式覆盖"启动未完成期间拒写 503"

- **来源**：01 §4.2 并发 / 时序第 3 条："UI 服务启动期间（持久化加载尚未完成）不接受任何写请求，统一返回 503 + 重试提示。"
- **现状**：02 §3.9 列中间件 `Recover → RequestID → Logger → CORS(dev) → CSRF(写接口) → SessionAuth(受保护) → Handler`，**没有 ReadyGate 中间件**；§5.2 也未在错误码表列 `503 NOT_READY`。
- **建议**：dev-backend 在 chi 链最前端加 ReadyGate 中间件（写接口在 `store.Ready()` 为 false 时返回 503 + `Retry-After: 2`）；同时 dev-backend 在 `04_DEVELOPMENT.md` 显式标 DESIGN DRIFT 引用本 Finding。
- **责任**：dev-backend 落地补 + 留 trace；**不阻塞**，列为 CONDITION。

### F-5 · WARN · AC-15 端到端测试依赖外部 sshd / nc，verify_all 自动化困难

- **来源**：01 §5 AC-15、02 §10.3 verify_all 设计
- **现状**：02 §10.3 把 verify_all 实体化范围限定为 `go vet/test/build + npm lint/build/test`，**未把 AC-15 的"建立 TCP 连接成功"列入**。AC-15 在裸 CI 无 sshd 时不能机械化。
- **建议**：QA Tester（Stage 6）阶段把 AC-15 标为**人工 / 集成机执行**，verify_all 仅保留可机械化部分；或在 `qa-tester` 阶段引入 `nc -l -p 22 &` + `nc 127.0.0.1 6000` 替代 sshd（同机自测可行）。
- **责任**：QA Tester 阶段决定形式，**不阻塞 dev**；列为 CONDITION 提醒 PM 在派 Stage 6 时附带本 Finding。

### F-6 · INFO · verify_all 当前为 npm 模板，未含 Go 步骤

- **来源**：实际 `Read scripts/verify_all.{ps1,sh}`
- **现状**：步骤 B.1~B.4 假设 npm/pnpm；E.1~E.6 是 Harness 自检。Go vet/test/build 缺失。
- **建议**：02 §13.1 已把这条作为 dev-backend "edit" 任务列入；本 Finding 仅记录"基线状态"，不属设计缺陷。**非阻塞**。
- **责任**：dev-backend Stage 4 落地。

### F-7 · INFO · 02 §6.5 `webServer.port=7500`（frps dashboard）未与 Q-10 选择的 UI 端口 8080 显式列冲突表

- **来源**：02 §附录 A.2、01 Q-10
- **现状**：本工具 UI 端口 8080，frpc admin 7400，frps dashboard 7500，frps bindPort 默认 7000，frpc/frps proxy remotePort 用户自由。**目前无重叠**，但 dev-backend 应在 `appconf` 默认值表头注释中显式写明这些"内部占用端口"。
- **建议**：dev-backend 文档化即可，不需要回改 02。**非阻塞**。

---

## 3. AC ↔ 设计 可追溯映射（01 §5 → 02 段落）

| AC | 设计实现路径 | 状态 |
|---|---|---|
| AC-1 全新启动 302 → /setup | 02 §5.2 `GET /api/v1/system/ready` 返回 `initialized=false` + §5.3 SPA fallback → Vue Router 跳 `/setup` | ✅ |
| AC-2 setup 成功 + 凭据非明文 | 02 §3.3 `HashPassword`（argon2id）+ §4.1 `admin.password_hash` + §6.2 PHC 串 | ✅ |
| AC-3 已 setup 后 /setup 拒绝 | 02 §5.2 `POST /setup` 在 `initialized=true` 时返回 409 `ALREADY_INITIALIZED`；SPA 据 `system/ready` 跳 `/dashboard` | ✅ |
| AC-4 5 次失败 → 429 + Retry-After | 02 §3.3 `RateLimiter`（5 次/60s）+ §5.1 错误码 `RATE_LIMITED`、§5.2 login 行注 429 含 Retry-After | ✅ |
| AC-5 新增 tcp 规则 5 秒内生效 | 02 §7.2 序列图 + §3.4 `RenderFrpc` + §3.6 `Reload(ctx, strict)` + §3.5 `ApplyConfigChange` 5s 超时 + R-1 reload 校验 | ✅ |
| AC-6 删除规则 5 秒内不再持有 | 同 AC-5 反向，§5.2 `DELETE /api/v1/proxies/{id}` → ApplyConfigChange | ✅ |
| AC-7 frps running + PID | 02 §3.5 `ProcessInfo{State, PID, ...}` + §5.2 `GET /api/v1/proc/status` | ✅ |
| AC-8 stop 后 stopped + PID 清空 | 02 §3.5 Stop 流程（Linux SIGTERM→3s→KILL，Win Kill）+ R-2 端口探测兜底 | ✅ |
| AC-9 开关跨重启保留 | 02 §4.1 `kv` 表 `mode.frpc.enabled` / `mode.frps.enabled` + main.go 启动时按 kv 自动 Start | ✅（在 02 §3.5 隐含；建议 dev-backend 在 main.go 启动序列显式调用） |
| AC-10 422 + 字段名 | 02 §5.1 错误体含 `field` + §5.2 errs 列 `VALIDATION_FAILED` / `CONFLICT` + §4.1 SQL UNIQUE + `idx_proxies_tcp_remote` 部分索引 | ✅ |
| AC-11 logs 500 行 + 2s 增量 | 02 §3.7 `TailLines(n)` + `ReadFrom(offset)` + §5.2 `GET /api/v1/logs/{kind}?lines=500 \| ?offset=N` | ✅ |
| AC-12 持久化损坏改名 + 进入 /setup | 02 §4.3 `PRAGMA integrity_check` + 改名 `*.broken-<RFC3339>` + `ErrCorruptReset` | ✅ |
| AC-13 缺二进制不崩溃 | 02 §3.8 `Locator.Missing()` + §6.5 + §5.2 `system/ready` 返回 `binMissing: [...]` + start 接口 422 `BIN_MISSING` | ✅ |
| AC-14 默认绑定 127.0.0.1 | 02 §3.1 `AppConfig.UIBindAddr = "127.0.0.1"` + §12 重申"UI 仅 HTTP，监听 127.0.0.1" + 01 NF-S4 警告路径 | ✅ |
| AC-15 端到端：tcp 连接成功 | 02 §7.2 + 渲染产物 + 子进程拉起；**验证手段需外部** sshd / nc——见 Finding F-5 | ⚠ 可达但非纯自动 |

**结论：15/15 可追溯，1 条（AC-15）需 QA 阶段补人工 / 替代手段。**

---

## 4. NF-S1~NF-S6 安全条款 → 实现路径映射

| NF | 要求 | 02 实现段落 | 覆盖 |
|---|---|---|---|
| NF-S1 加盐哈希落盘 | argon2id `m=64MiB,t=3,p=2`，PHC 串 | 02 §6.2 + §3.3 `HashPassword` | ✅ |
| NF-S2 Cookie HttpOnly/SameSite=Lax/HTTPS 时 Secure | 02 §5.1 鉴权章节明文列出 `HttpOnly; SameSite=Lax`，HTTPS 时附 `Secure` | ✅ |
| NF-S3 CSRF | 02 §5.1 写接口需 `X-CSRF-Token` 头，§5.2 `GET /api/v1/auth/csrf` 端点 + sessions 表 `csrf_token` 列 | ✅ |
| NF-S4 默认 127.0.0.1，0.0.0.0 时警告 | 02 §3.1 `UIBindAddr` 默认 + §3.9 中间件链未列 startup warning——**建议 dev-backend 在 main.go 启动序列 if 非 127.0.0.1 → stderr 输出 WARN** | ✅（落地点指明） |
| NF-S5 token 脱敏 | 02 §5.2 `GET /api/v1/server` 默认返回 `auth.token: "***"`，`?reveal=1` 才回明文；客户端连接信息同理 | ✅ |
| NF-S6 日志脱敏 | 02 §3.9 logger 中间件位置确定；但**未显式说明脱敏过滤器在哪里**——R-? 也未列 | ⚠ 半覆盖 |

**Finding F-8（追加，WARN）**：02 §3.9 logger 未给出"脱敏过滤器"实现点。dev-backend 落地需在 `internal/httpapi/middleware/logger.go` 实现 `redact(body, []string{"password","oldPassword","newPassword","authToken","token"})`；建议 PM 在 Stage 4 dispatch 时附本 Finding，dev-backend 在 04_DEVELOPMENT.md 显式记录实现位置。**列为 CONDITION，不阻塞**。

---

## 5. 跨平台 / 进程信号 / 二进制选择审计

| 项 | 设计段 | 评估 |
|---|---|---|
| 二进制选择 | 02 §3.8 `runtime.GOOS` switch + §6.5 代码片段 | ✅ Windows → `frp_win/frpc.exe`、Linux → `frp_linux/frpc` |
| 路径处理 | 02 §3.1 `DataDir`、§3.7 日志路径、`filepath.Join` 隐含 | ✅（dev-backend 必须用 `filepath`，**不许写死分隔符**——已写入 01 NF-C3） |
| 进程信号 | 02 §3.5 注明 Linux SIGTERM→3s→SIGKILL、Windows `cmd.Process.Kill()`、`Setpgid` / `CREATE_NEW_PROCESS_GROUP` | ✅ |
| frpc/frps 缺失不崩溃 | 02 §3.8 `Locator.Missing()` 返回缺失项列表 + §5.2 `system/ready.binMissing` + 对应 start 接口返回 422 `BIN_MISSING` | ✅ 完整支撑 AC-13 |
| 启动脚本 | 02 §10.3 `scripts/start.{ps1,sh}` + `build.{ps1,sh}` 双产物 | ✅ |

---

## 6. 依赖合理性审计（每条依赖 → 一句 why）

| 依赖 | 02 引用 why | 评级 |
|---|---|---|
| `github.com/go-chi/chi/v5` | §3.9/§8/§10.1：std net/http 兼容、零反射、中间件链清晰；轻于 gin | ✅ |
| `github.com/pelletier/go-toml/v2` | §8/§10.1：标准库无 TOML，需精确 camelCase 序列化 | ✅ |
| `golang.org/x/crypto/argon2` | §6.2：OWASP 首推；官方扩展库 | ✅ |
| `modernc.org/sqlite` | §6.1/§8：纯 Go 免 cgo，跨平台单二进制；接受 +10MB 体积（R-4） | ✅ |
| Vue 3 / Vue Router 4 / Pinia | §3.11/§10.2/§8：官方正典；状态管理用 Pinia 而非 Vuex 是 Vue 3 当前推荐 | ✅ |
| Naive UI | §8/§10.2：Vue 3 原生 + TS 类型完整 + 组件覆盖足够；不引 Element Plus 节省一层 | ✅ |
| Axios | §8/§10.2：拦截器 / CSRF / 错误处理成熟 | ✅ |
| Vite + @vitejs/plugin-vue + vue-tsc | §10.2：业界默认 Vue 工具链 | ✅ |
| ESLint + eslint-plugin-vue | §10.2：lint 步骤一致 | ✅ |
| Vitest | §10.2：Vite 同源测试框架 | ✅ |

**结论：无冗余、无可疑选择；体积代价（R-4 + R-5）已显式接受**。

---

## 7. 开发期高概率问题预答（pre-FAQ）

> dev-* 真开干时会问，预答省往返。

### Q-A：`internal/storage` 启动时如果 `data.db` 不存在（首次启动），是错误还是正常？

**答**：正常。02 §4.3 `storage.Open` 流程为：尝试打开 → 失败或 corrupt 则改名重建 → 重跑迁移；**首次启动 = data.db 不存在 = 直接新建 + 迁移**，不应记录 ErrCorruptReset，仅在真损坏路径触发。dev-db 在 `storage_test.go` 必须分两个用例覆盖（fresh / corrupt-reset）。

### Q-B：frpc admin API 的 `webServer.user/password` 谁生成、谁存？

**答**：02 §3.4 `FrpcRenderInput.AdminUser/AdminPass` 明确**由 UI 服务启动时生成并持久化**。落地：dev-backend 在 `main.go` 首次启动序列里 `crypto/rand` 出 32 字节，写入 `kv` 表 `frpc.admin`（JSON `{addr, port, user, pass}`），后续启动直接读取；该凭据**永远不向前端暴露**，仅给 `internal/frpcadmin.Client` 用。

### Q-C：前端表单 `customDomains` 输入怎么做？

**答**：02 §5 类型 `customDomains?: string[]`；UX 推荐 Naive UI `n-dynamic-tags` 或 `n-input` 回车分割；后端校验仅每项必须匹配 `^([A-Za-z0-9-]{1,63}\.)+[A-Za-z]{2,}$` 且长度 ≤253。dev-frontend 直接定义独立组件 `DomainListInput.vue`，不要散在 `Proxies.vue` 里。

### Q-D：`/api/v1/proc/{kind}/start` 在二进制缺失时返回 422 `BIN_MISSING`，前端怎么显示？

**答**：02 §5.1 错误体 `{ error: { code, message, field? } }`；dev-frontend 在 axios response interceptor 里 switch `error.code`，遇 `BIN_MISSING` 时给顶部 banner + 模式开关 disable（AC-13）。不必每次 toast，banner 持久显示直到 `system/ready.binMissing=[]`。

### Q-E：dev-backend 的 main.go 如何串接子进程"自动恢复"（AC-9）？

**答**：建议顺序：
1. `appconf.Load` → `storage.Open` → 跑迁移
2. 初始化 `binloc.NewDefault(repoRoot)`、`procmgr.New(...)`
3. 启动 HTTP 服务（已就绪后才可接受写）
4. 读 `kv.mode.frpc.enabled` / `kv.mode.frps.enabled`；若 true 且对应二进制存在 → `procmgr.Start(kind)`
5. 主 goroutine 阻塞 `Listen`，监 SIGINT/SIGTERM → 优雅停 procmgr 后退出

这一段 02 §3.5/§4.1/§7.1 各处有线索但**无单一序列图**——dev-backend 在 04_DEVELOPMENT.md 里画一张启动序列图即可，**不算 DESIGN DRIFT**。

---

## 8. Verdict（裁定 — full mode 词汇）

### **APPROVED WITH CONDITIONS**

> 8 维度无 FAIL，3 项 WARN 均可在 Stage 4/6 落地阶段消化；不需要回退 requirement-analyst 或 solution-architect。条件清单如下，PM 在派 Stage 4 时一并下发给 dev-backend / dev-frontend / dev-db，落地时显式回应。

### 条件清单（共 5 条 CONDITION + 2 条 INFO）

| 编号 | 责任分区 / 阶段 | 必须在哪里回应 | 内容摘要 |
|---|---|---|---|
| C-1（F-2） | PM / solution-architect | PM 派 Stage 4 时口头释疑 + 可选回补 02 §13.4 | 明确 `internal/assets/dist/` 作为构建产物，dev-frontend `npm run build` 写入合法不算越界 |
| C-2（F-3） | dev-backend | `04a_DEVELOPMENT_backend.md` | UI 端口被占时给中文用户向错误信息（示例文案见 Finding F-3） |
| C-3（F-4） | dev-backend | `04a_DEVELOPMENT_backend.md` + DESIGN DRIFT 标记 | 在 chi 中间件链最前端加 ReadyGate（store 未就绪 → 503 + Retry-After: 2）；§5.1 错误码表追加 `NOT_READY` |
| C-4（F-5） | QA Tester（Stage 6） | `06_TEST_REPORT.md` | AC-15 标人工 / 集成机或用 `nc` 替代 sshd；verify_all 仅保留可机械化部分 |
| C-5（F-8） | dev-backend | `04a_DEVELOPMENT_backend.md` | logger 中间件实现脱敏过滤器（字段名 `password/oldPassword/newPassword/authToken/token`），位置 `internal/httpapi/middleware/logger.go` |
| I-1（F-1） | dev-backend | `internal/auth/hash.go` doc-comment | 备注"低配机器可把 m 调成 32768" |
| I-2（F-7） | dev-backend | `internal/appconf` doc-comment | 内部占用端口表（UI=8080、frpc admin=7400、frps dashboard=7500、frps bindPort=7000） |

### 路由建议

- **不回退 requirement-analyst**（0 条 BLOCKED ON REQUIREMENT）。
- **不回退 solution-architect**（0 条 BLOCKED ON DESIGN；F-2 可由 PM 一句话澄清而非重出 02）。
- **PM 直接派 Stage 4**，按 02 §13.2 顺序：dev-db → dev-backend（第一轮，跳过 `internal/assets/embed.go`）→ dev-frontend（并行可在 dev-backend 第一轮起后启动）→ dev-backend（第二轮，补 embed + verify_all 最终跑通）。
- **dev-backend 第一轮**收到本文件时同时接收 C-2/C-3/C-5/I-1/I-2 五条条件，必须在 `04a_DEVELOPMENT_backend.md` 显式回应；C-4 在 Stage 6 派 qa-tester 时再下发。

---

## 附：本 review 未采取的动作（合规声明）

- 未编辑 `01_REQUIREMENT_ANALYSIS.md`、`02_SOLUTION_DESIGN.md`、`INPUT.md`、`PM_LOG.md`、任何 `.harness/agents/*.md`（gate-reviewer 红线 #1 / 红线 #2）。
- 未提出"应该怎么改"的设计替代方案（仅指出问题与责任，符合 gate-reviewer 红线 #5）。
- 未越级触发 Stage 4 任何 dev-* agent。
- context7 调用一次（`/fatedier/frp` query-docs），低于 3 次预算。
- 全文中文、无 emoji（遵循 CLAUDE.md 输出语言）。
