# 01 需求分析 — T-063 · loginfail-kv-purge

- 模式：full
- 批次：ux-ui-uplift-2026-05（第 2 个任务）
- 作者：Requirement Analyst
- 日期：2026-05-31

## 1. 目标（Goal）

为 `loginfail.<ip>` 限流计数 KV 行加后台周期清理，关闭 T-046 只清 sessions 表、未对称清理 loginfail KV 而留下的同类资源泄漏：永不复访的轮换源 IP（NAT 后扫描器/攻击者）每个留下一条永久滞留的 `loginfail.<ip>` KV 行，无界增长。

## 2. 范围内行为（In-scope，可测）

- **IS-1**：存在一种后台机制，周期性删除**已过期**的 `loginfail.<ip>` KV 行。"已过期"的判定语义与现有限流窗口一致：一条 `loginfail.<ip>` 记录在 `FirstAt + failWindow`（即 `FirstAt + 60s`）之后视为过期（与 `internal/auth/ratelimit.go:74` `Allow` 内的惰性清理判定 `now.After(rec.FirstAt.Add(failWindow))` 字节级同义）。
- **IS-2**：该清理机制**复用现有 `cmd/frp-easy/main.go:546 purgeSessionsLoop` 的同一后台 goroutine / ticker 节奏**，不新增独立 goroutine、不新增独立 ticker。
- **IS-3**：清理能力的"按前缀枚举或按前缀清理 KV"的底层机械操作落在 `internal/storage/` 包（所有 SQL 红线）；过期判定（懂窗口语义）落在 `internal/auth/` 限流器层。具体函数边界由 Solution Architect 定（任务交底倾向选项 B：storage 暴露机械前缀列举/删除 + RateLimiter 暴露 `PurgeExpired`）。
- **IS-4**：清理一次的操作返回被删除的条数（供日志观测，对齐 `PurgeExpiredSessions (int64, error)` 范式）。删除条数 > 0 时记一条 Info 日志；清理出错时仅 Warn 不致命（对齐 `purgeExpiredSessionsOnce` 范式）。
- **IS-5**：清理操作前缀严格限定 `loginfail.`，**不得**触及其它 KV 命名空间（`mode.*`、frps 配置键、frpc admin creds 键 `system.*` 等）。
- **IS-6**：清理 goroutine 随 root context 取消而退出（无 goroutine 泄漏），与 `purgeSessionsLoop` 既有 ctx.Done 退出语义一致。

## 3. 范围外（Out-of-scope）

- **OOS-1**：不改限流算法本身（5 次/60s per IP 滑窗、retryAfter 计算、`Allow`/`RecordFailure`/`Reset` 既有行为字节不变）。本任务只**增加**周期主动清理，惰性清理（`Allow` 内的 `KVDelete`）保留不动。
- **OOS-2**：不新增 schema / migration。KV 表（`kv`）已存在；本任务不加列、不加索引、不加表。
- **OOS-3**：不改清理周期为可配置项（暴露 env 覆盖等）。复用 `sessionPurgeInterval`（1h）现有节奏即可；若 SA 需独立间隔常量，限定为包级 var（对齐 `sessionPurgeInterval` 可测范式），不暴露给用户配置。
- **OOS-4**：不做 sessions 表清理与 loginfail 清理的语义合并/重构（两者过期判定不同：session 用 `expires_at` 列，loginfail 用 JSON 值内 `FirstAt` + 窗口常量）。仅在调度层（同一 loop）合并触发。
- **OOS-5**：不涉及任何前端 / API 路由 / HTTP handler 变更（纯后端 + storage）。
- **OOS-6**：不对 `loginfail.<ip>` 的 KV 值 JSON 格式（`{count, firstAt}`）做变更。

## 4. 边界条件（Boundary conditions）

- **BC-1（空 KV / 无 loginfail 行）**：KV 表无任何 `loginfail.` 行时，清理一次返回 0、不报错、不 panic。
- **BC-2（全部活计数）**：所有 `loginfail.` 行均在窗口内（`now < FirstAt + 60s`），清理一次删除 0 条——**绝不**误删活计数（否则限流失效，违反 OOS-1）。
- **BC-3（混合）**：部分过期、部分活，清理只删过期的，活计数原样保留且后续 `Allow`/`RecordFailure` 行为不变。
- **BC-4（窗口边界）**：`now == FirstAt + 60s`（恰好等于）的处理必须与 `Allow` 的 `now.After(expires)` 判定一致——即恰好等于时**不**视为过期（`After` 是严格大于），避免清理比惰性清理更激进而造成行为分叉。
- **BC-5（损坏的 JSON 值）**：某 `loginfail.<ip>` 行的 value 非合法 `{count, firstAt}` JSON（理论上不应出现，但 KV 表是文本表，需防御）。判定语义由 SA 定，候选：(a) 视为过期删除（清掉脏数据，符合"清理"意图）/ (b) 跳过保留（保守，不删不确定的）。倾向 (a)：脏 loginfail 行对限流无意义且属于应被回收的垃圾。RA 不替 SA 定夺，列入 SA 风险分析。
- **BC-6（前缀误匹配）**：键如 `loginfailure.x`、`xloginfail.y`、`loginfail`（无点无 ip）等近似键不得被前缀逻辑误删——前缀必须是精确的 `loginfail.`（含点）。SQL `LIKE 'loginfail.%'` 的 `.` 在 SQLite LIKE 中是字面字符（非通配），但 `_`/`%` 是通配——`loginfail.` 不含这两个字符，故 `LIKE 'loginfail.%'` 安全；SA 需在设计中确认 LIKE 转义边界。
- **BC-7（ctx 取消）**：清理进行中 root ctx 被取消，loop 在合理时间内（对齐既有 2s 测试窗口）退出，不泄漏 goroutine。
- **BC-8（并发）**：清理与正常登录失败计数（`RecordFailure` 写）并发——清理删除的是已过期行（不在任何活窗口内），与正在 `RecordFailure` 的活 IP 行无交集；即便竞态删到一个刚好跨过期边界的行，下一次 `RecordFailure` 会以新窗口（`Count=1`）重建，限流语义不被破坏（最坏退化为该 IP 多得一次窗口，等价于窗口本就到期）。`-race` 因本机无 C 编译器不跑，需静态论证。

## 5. 验收标准（Acceptance criteria，可验证）

- **AC-1**：编译通过，`scripts/verify_all` PASS（PM 上下文无 Bash/PS，标 PENDING + 执行规格，交 batch orchestrator 真跑）。
- **AC-2**：存在 storage 层单测证明前缀列举/删除只命中 `loginfail.` 行、不误删其它前缀（覆盖 BC-5/BC-6 中的前缀边界），用 `t.TempDir()` 隔离 DataDir。
- **AC-3**：存在 auth 层（或清理函数所在层）单测，用注入时钟（`rl.now` 现有范式）+ in-memory `fakeKV`（现有范式）证明：过期行被删、窗口内活行不被删、窗口边界 `==` 不删（BC-2/BC-3/BC-4）。
- **AC-4**：存在 wiring 层（`cmd/frp-easy`）单测证明：清理被挂进 `purgeSessionsLoop`（或等价单 loop）后，清理一次能删过期 loginfail 行、不删活行、不删非 loginfail 键（对齐 `session_purge_test.go::TestPurgeExpiredSessionsOnce` over-delete 防御范式）；且 loop ctx 取消正常退出（对齐 `TestPurgeSessionsLoop_ExitsOnCancel`）。
- **AC-5**：现有限流行为零回归——`internal/auth/auth_test.go` 的 `TestRateLimiter_*` 全绿不改语义（除非新增清理 API 需要补充用例）。
- **AC-6**：`scripts/baseline.json` 的 `go_tests` / `test_count` / `version` 已同步 bump（新增 Go 顶层测试数）。
- **AC-7**：`06_TEST_REPORT.md` 含**裸标题** `## Adversarial tests` 段，至少覆盖三条反向证伪：(a) 窗口内活计数清理后仍能正常触发限流（不被清掉）；(b) 过期行确实被清掉（不靠惰性也能清）；(c) 前缀只匹配 `loginfail.`，`mode.*` / frps 配置 / frpc admin creds 等其它 KV 不被误删。
- **AC-8**：`docs/dev-map.md` 已更新（storage 对外 API 表 + auth ratelimit 行 + main.go purge loop 行反映新增能力）。

## 6. 非功能需求（NFR）

- **NFR-1（安全/正确性）**：清理绝不能放宽限流——删除条件必须 ≤ 惰性清理的删除条件（即只删 `Allow` 也会删的过期行），不得删任何 `Allow` 仍判 blocked 的活行。这是本任务最高优先正确性约束。
- **NFR-2（性能）**：清理是 O(loginfail 行数) 的周期扫描，1h 一次，规模远低于 sessions；无性能担忧。前缀删除若能在 storage 用单条 `DELETE ... WHERE key LIKE 'loginfail.%' AND <过期>` 更高效，但过期判定在 JSON 值内（SQL 判时间不便，见交底），故倾向 storage 只做前缀列举、过期判定回 ratelimit 层（选项 B），由 SA 权衡。
- **NFR-3（并发安全）**：复用 storage.Store 自身 `s.mu` 兜底 DB 写；RateLimiter 既有 `r.mu` 保护读-判-写复合操作。新增清理路径不得引入新的锁顺序倒置。`-race` 静态论证（本机无 cgo）。
- **NFR-4（无新依赖）**：纯标准库，不引入任何第三方库。

## 7. 相关任务（Related tasks）

- **T-046 session-purge-and-requestid**（`docs/features/session-purge-and-requestid/`）：直接对称范式。提供 `purgeSessionsLoop` / `purgeExpiredSessionsOnce` / 包级 var `sessionPurgeInterval`（main.go:527-558）+ `storage.PurgeExpiredSessions (int64, error)`（sessions.go:102-114）。本任务在调度层复用同一 loop，在 storage 层对齐"周期清理返回 count"形态。**不重新设计，扩展即可。**
- **T-001 web-ui-mvp**（`docs/features/_archived/web-ui-mvp/04_DEVELOPMENT_backend.md:69`）：明确记载本 backlog——"RateLimiter 的 kv 持久化键是 loginfail.<ip> 形式，storage 无前缀枚举 API，定期 purge 现状未实现，将来应补"。本任务即偿还该项。
- **T-053 autorestore-canceled-persist-fix**：detached context 教训（insight L23）。注意：本任务是**周期清理**非取消收尾，按现有 `purgeSessionsLoop` 范式（用 root ctx 派生带超时）即可，无需 detached ctx。
- **insight L9/L10**：有副作用代码可测（时钟注入 + in-memory fake）；同步点禁固定 sleep，用 poll-until-condition + deadline。

## 8. 给用户的开放问题（Open questions）

无阻塞性开放问题。本任务为 T-046 的对称扩展，证据、范围、约束在任务交底中已充分明确。以下为已在边界条件/范围外中明确定夺、留给 Solution Architect 在设计中确认的技术取舍点（非用户决策）：

- **Q-1（留 SA，非用户）**：损坏 JSON 值的 loginfail 行处理（BC-5）—— RA 倾向"视为过期删除"，SA 在风险分析中定夺并给理由。
- **Q-2（留 SA，非用户）**：清理间隔复用 `sessionPurgeInterval`（1h）还是独立包级 var —— RA 倾向复用同一 loop 触发即可（OOS-3），SA 定函数边界。
- **Q-3（留 SA，非用户）**：选项 A（storage 暴露带时间判定的 PurgeKVByPrefix）vs 选项 B（storage 只暴露前缀列举/删除 + RateLimiter.PurgeExpired 做过期判定）—— 任务交底倾向 B（过期语义单点一致），SA 最终权衡。

## 9. 裁决（Verdict）

**READY** — 无阻塞性用户开放问题。Q-1/Q-2/Q-3 为设计层技术取舍，由 Solution Architect 在 `02_SOLUTION_DESIGN.md` 定夺，不阻塞推进。
