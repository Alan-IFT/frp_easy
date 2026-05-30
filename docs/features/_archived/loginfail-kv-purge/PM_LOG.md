# PM_LOG — T-063 · loginfail-kv-purge

> PM Orchestrator 路由决策日志。每次 stage transition 追加一行。
> 模式：full（7-stage）。批次：ux-ui-uplift-2026-05（第 2 个任务）。
> 输出语言：中文（红线）。

## 任务概要

- 一句话目标：关闭 `loginfail.<ip>` 限流计数 KV 行的永久滞留——对称补齐 T-046 只清 sessions 表、未清 loginfail KV 的资源泄漏另一半。
- 模式：full
- 分区：partitioned（dev-db + dev-backend；无前端）
- 关键证据：
  - `internal/auth/ratelimit.go:74-80` 过期窗口仅惰性清理（同 IP 复访才 KVDelete）；永不复访的轮换源 IP 留永久滞留行。
  - `cmd/frp-easy/main.go:544-558 purgeSessionsLoop` 只清 sessions 表，未清 loginfail KV。
  - `internal/storage/kv.go` 仅有 KVGet/KVSet/KVDelete，无前缀枚举/批删 → 当初未实现 purge 的直接原因。
  - backlog 书面证据：`docs/features/_archived/web-ui-mvp/04_DEVELOPMENT_backend.md:~69`。

## 复用范式（PM 预扫，派发上下文）

- 清理 loop 范式：`main.go:527-558`（包级 var `sessionPurgeInterval` + `purgeExpiredSessionsOnce`（带 5s 超时、错误仅告警）+ `purgeSessionsLoop`（启动立即清一次 + ticker + ctx.Done 退出））。
- storage 周期清理返回 count 范式：`sessions.go:102-114 PurgeExpiredSessions`（`s.mu.Lock` + `ExecContext` + `RowsAffected`）。
- wiring 落点：`main.go:270 rl := auth.NewRateLimiter(store)`（RateLimiter 已持有 store）；`main.go:335 go purgeSessionsLoop(rootCtx, store, logger)`（现有唯一清理 goroutine，挂到此 loop 不新增）。
- ratelimit 测试范式：`internal/auth/auth_test.go:99-126 fakeKV`（in-memory kvStore）+ `rl.now` 可注入时钟。
- wiring 测试范式：`cmd/frp-easy/session_purge_test.go`（`purgeExpiredSessionsOnce` over-delete 防御 + `purgeSessionsLoop` ctx 取消退出）。

## Insight 预扫（应用到本任务的条目）

- L9（有副作用代码可测：纯函数 + 时钟/env 注入）。
- L10（同步点禁固定 time.Sleep，用 poll-until-condition + deadline）。
- L23（detached context：错误/取消路径的最终持久化用 context.Background()）——但本任务是周期清理非取消收尾，按现有 loop 范式即可（用任务交底已说明）。
- L31（PM 上下文无 Bash/PS 无法真跑 verify_all → 标 PENDING + 给执行规格，交 batch orchestrator 真跑）。
- dev-map 红线：所有 SQL 只在 internal/storage/；其它包不写裸 SQL。
- 单测红线：t.TempDir() 隔离 DataDir，不跨用例共享 db。
- baseline 红线：新增测试必须同步 bump scripts/baseline.json（go_tests/test_count/version）。

## 路由决策

| 时间 | Stage | Agent | 决策 | 备注 |
|---|---|---|---|---|
| 2026-05-31 | init | PM | 创建任务，登记看板，mode=full，partitioned（dev-db→dev-backend） | 无前端 |
| 2026-05-31 | 1 | requirement-analyst | 派发中 | 交底已含 T-046/web-ui-mvp backlog/ratelimit 证据 |
| 2026-05-31 | 1→2 | PM | RA 产出 01，Verdict=READY 无阻塞用户问题（Q-1/Q-2/Q-3 为设计层技术取舍留 SA）→ 推进 Stage 2 | 注：本 PM 上下文无 Task 工具暴露（role-collapsed，insight L31 同类），PM 按各 agent 契约逐角色扮演并落盘每阶段文档，每次 transition 记录于此 |
| 2026-05-31 | 2 | solution-architect | 派发中 | 关键边界：internal/auth/** 属 dev-backend（非 dev-db）；选项 B 涉及 dev-db（storage 机械前缀列举）+ dev-backend（ratelimit.PurgeExpired + main wiring）双分区 |
| 2026-05-31 | 2→3 | PM | SA 产出 02，Verdict=READY；采用选项 B（storage 机械 KVListByPrefix + ratelimit.PurgeExpired 懂窗口语义）；候选 (i) auth→storage 依赖（无环）；含完整 §11 分区分配 dev-db→dev-backend 严格串行 → 推进 Stage 3 | — |
| 2026-05-31 | 3 | gate-reviewer | 派发中（带 T-034 两模式输出提醒；本 role-collapsed 上下文 PM 直接落盘 = Mode A 等价） | 8 维审计 + 独立核验设计代码声明 |
| 2026-05-31 | 3→4 | PM | GR 产出 03，Verdict=APPROVED FOR DEVELOPMENT（full 模式 = APPROVED）；8 维全 PASS、无 FAIL；独立核验 11 条设计代码声明全属实（含 R-5 storage 不 import auth 经 admin.go:14 确认仅注释、无环）；OBS-1/2/3 非阻塞 → Stage gate 满足，推进 Stage 4 | 派发 dev-db 先行 |
| 2026-05-31 | 4 | dev-db | 派发中 | 先做 storage KVEntry + KVListByPrefix + 单测（dev-backend 编译依赖） |
| 2026-05-31 | 4(dev-db) | dev-db | READY FOR REVIEW；kv.go +KVEntry +KVListByPrefix +escapeLike（LIKE ESCAPE 防御）+strings import；storage_test.go +3 顶层 Test（OnlyMatchingPrefix/EmptyResult/LikeMetacharsAreLiteral）；无 migration | dev-db 分区完成，标记 complete |
| 2026-05-31 | 4 | dev-backend | 派发中 | 消费 storage 新 API：ratelimit.PurgeExpired + auth_test + main wiring + wiring 测试 + baseline + dev-map |
| 2026-05-31 | 4(dev-backend) | dev-backend | READY FOR REVIEW；ratelimit.go +LoginFailKeyPrefix +kvStore.KVListByPrefix +PurgeExpired（过期判定与 Allow 同源、损坏值删、best-effort）+import storage；auth_test fakeKV +KVListByPrefix +4 测试；main.go purgeSessionsLoop→purgeLoop +purgeExpiredLoginFailsOnce + 调用点改 rl；session_purge_test 改名+传 rl +2 测试；baseline 322→331/856→865/v29；dev-map 3 行 | 两分区全部就位 |
| 2026-05-31 | 4→5 | PM | Stage gate：verify_all 因 role-collapsed 无 Bash/PS（insight L31）标 PENDING（预期 PASS / go_tests==331），与本批次 T-056~T-062 同范式，交 batch orchestrator 真跑作硬闸门；静态+确定性预测全绿 → 推进 Stage 5 | — |
| 2026-05-31 | 5 | code-reviewer | 派发中（带 T-034 两模式输出提醒；role-collapsed PM 直接落盘 = Mode A 等价） | 审 dev-db + dev-backend 两分区代码 |
| 2026-05-31 | 5→6 | PM | CR 产出 05，Verdict=APPROVED（0 CRITICAL/0 MAJOR/0 MINOR/2 NIT）；逐表达式核对过期判定与 Allow 同源（限流不削弱）+ 设计保真 11 项全 ✅ + 既有测试回归复核全过 → 推进 Stage 6 | — |
| 2026-05-31 | 6 | qa-tester | 派发中 | 独立对抗 reproducer + 裸 ## Adversarial tests + verify_all 执行规格 |
| 2026-05-31 | 6→7 | PM | QA 产出 06，Verdict=APPROVED FOR DELIVERY，0 缺陷；+2 QA 独立 reproducer（QA-ADV-1 count 解耦/QA-ADV-2 清后不污染），go_tests 再 +2 至 333（baseline 同步 322→333/856→867）；06 含裸 ## Adversarial tests（AT-1/2/3）；Stage 5+6 均 PASS → Stage gate 满足，推进 Stage 7 交付 | 0 rollback 全程 |
| 2026-05-31 | 7 | PM | 产出 07_DELIVERY.md（含裸 ## Insight 2 条）；更新 docs/tasks.md 移 T-063 至已完成；DELIVERED | 按批次约定**不**跑 archive-task / 不 commit，由 batch orchestrator 统一处理（与 INPUT 交底一致） |

## 最终交付状态

- Verdict: **DELIVERED** · 0 rollback 全程 · verify_all=PENDING（预期 PASS / go_tests==333）
- 7 阶段全部 PASS（GR APPROVED FOR DEVELOPMENT / CR APPROVED 一次过 / QA APPROVED FOR DELIVERY 0 缺陷）
- 三次同 stage 回退闸门：未触发（0 rollback）
- 硬闸门 verify_all 真跑交 batch orchestrator（PM/dev/QA role-collapsed 无 Bash/PS，insight L31）
- 未 commit / 未 archive（批次约定）
