# Delivery Summary — T-063 · loginfail-kv-purge

- Task: `loginfail-kv-purge` — 关闭 `loginfail.<ip>` 限流计数 KV 行的永久滞留（对称补齐 T-046 只清 sessions 表、未清 loginfail KV 的资源泄漏另一半）。
- Mode: full（7-stage；partitioned dev-db → dev-backend；无前端）
- 批次：ux-ui-uplift-2026-05（第 2 个任务）
- Stages traversed:
  - 1 requirement-analyst → 01（READY）· 2026-05-31
  - 2 solution-architect → 02（READY，选项 B）· 2026-05-31
  - 3 gate-reviewer → 03（APPROVED FOR DEVELOPMENT；8 维全 PASS）· 2026-05-31
  - 4 dev-db → storage KVEntry/KVListByPrefix +3 测试（READY FOR REVIEW）· 2026-05-31
  - 4 dev-backend → ratelimit.PurgeExpired + main purgeLoop wiring +6 测试（READY FOR REVIEW）· 2026-05-31
  - 5 code-reviewer → 05（APPROVED；0 CRITICAL/MAJOR/MINOR，2 NIT）· 2026-05-31
  - 6 qa-tester → 06（APPROVED FOR DELIVERY；0 缺陷；+2 QA 独立 reproducer）· 2026-05-31
  - 7 PM → 本文 · 2026-05-31
- Rollbacks: **0**（全程无回退）
- Final verify_all result: **PENDING（预期 PASS）** — PM/dev/QA role-collapsed 上下文无 Bash/PS（insight L31），verify_all 全量真跑交 batch orchestrator Bash 会话作硬闸门。静态 + 确定性预测全绿。
- Baseline changes: go_tests **322 → 333**（+11）/ test_count **856 → 867** / frontend_tests 534（不变）/ version **28 → 29**。
- Outstanding risks:
  - `-race` 本机无 cgo 未跑（与 T-050 先例一致）；并发安全静态论证：`PurgeExpired` 持 `r.mu` 与 `RecordFailure`/`Allow`/`Reset` 同锁互斥，锁顺序 r.mu 外 / s.mu 内无倒置（02 §8 R-6 / 05 / 06）。orchestrator 视环境决定是否 `-race`。
  - 周期清理与进程 shutdown 的 store.Close 理论上有 best-effort 竞态（同 insight L24 精神），但清理删的是过期垃圾，丢一轮无影响、下轮重清——可接受，未做强同步（超范围）。
- Files changed（7 文件 + 2 文档元数据）:
  - `internal/storage/kv.go`（+KVEntry +KVListByPrefix +escapeLike +strings import）
  - `internal/storage/storage_test.go`（+3 测试 + keysOf helper）
  - `internal/auth/ratelimit.go`（+LoginFailKeyPrefix +kvStore.KVListByPrefix +PurgeExpired +import storage；key() 改用常量）
  - `internal/auth/auth_test.go`（fakeKV +KVListByPrefix；dev +4 + QA +2 测试 + seedFail/hasKey helper；+json/sort/storage import）
  - `cmd/frp-easy/main.go`（purgeSessionsLoop→purgeLoop +rl 参数 +purgeExpiredLoginFailsOnce；调用点改）
  - `cmd/frp-easy/session_purge_test.go`（改名+传 rl；+2 测试 + writeLoginFail helper；+auth/json import）
  - `scripts/baseline.json`（计数 bump）
  - `docs/dev-map.md`（storage API 表 + ratelimit 行 + main 入口行 3 处）
- Next steps for user:
  - batch orchestrator 在 Bash 会话真跑 `scripts/verify_all` 作硬闸门，特别复核：(1) 既有 `TestPurgeExpiredSessionsOnce` + `TestPurgeSessionsLoop_ExitsOnCancel`（改名后调 purgeLoop）零回归；(2) 既有 `TestRateLimiter_*` 5 个零回归；(3) `go_tests == 333`；(4) `go vet ./...` 无新告警（auth→storage 新依赖无环）。
  - 按批次约定**未 commit / 未 archive**，由 batch orchestrator 统一处理。

## 关键改动一句话

把限流计数行的清理从"只惰性（同 IP 复访才删）"补成"惰性 + 主动周期清理"：storage 加机械的 `KVListByPrefix`，ratelimit 加懂窗口语义的 `PurgeExpired`（过期判定与 `Allow` 字节级同源 → 删除集 ⊆ 惰性清理集，绝不削弱限流），挂进既有 `purgeLoop`（原 `purgeSessionsLoop` 泛化）同一 goroutine，不新增 goroutine。

## Insight

- 2026-05-31 · "资源泄漏修复"的对称性陷阱：T-046 给 sessions 表加了周期清理却漏了同属"过期即垃圾"的 loginfail KV 行——根因是两者过期判定**源不同**（session 用 DB 列 `expires_at` 可直接 SQL `DELETE WHERE`，loginfail 用 JSON 值内 `firstAt`+窗口常量，SQL 判时间不便），导致当初 storage 只为前者建了清理通路。修对称泄漏的正确分层是 storage 出"机械的按前缀列举"（`KVListByPrefix`，不懂任何过期语义）、过期判定回到懂窗口语义的那一层（`RateLimiter.PurgeExpired` 复用与 `Allow` 字节级同一的 `now.After(firstAt+window)` 表达式）——而非在 storage 里塞业务时间判定。"过期语义单点收敛"让正确性退化为静态事实：删除集 ⊆ 惰性清理集是同源表达式的直接推论，无需运行时证据即可确定（同 insight L26/L31）· evidence: T-063 internal/storage/kv.go KVListByPrefix + internal/auth/ratelimit.go PurgeExpired（与 Allow:74 共用表达式）+ 06 §Adversarial AT-1/AT-2
- 2026-05-31 · 给"周期清理过期计数"写测试时，最易漏的反向证伪是 **sub-threshold 过期行（count < 上限）也必须被清**：dev 测试惯性只 seed 达上限的行（count=failMax），但限流计数行在 count<failMax 时同样会过期、同样永久滞留——若 purge 误把过期判定与 `count>=failMax` 耦合（如照搬 `Allow` 里 `rec.Count < failMax` 的早返结构），sub-threshold 过期行会被漏清，泄漏没真正堵住。QA 独立 reproducer 必须显式造 count=1/2 的过期行断言被清（过期判定只看时间窗口、与 count 解耦）· evidence: T-063 auth_test.go TestQA_PurgeExpired_SubThresholdExpiredAlsoPurged + PurgeExpired 无 count 门控
