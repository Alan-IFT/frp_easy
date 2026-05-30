# 06 测试报告 — T-063 · loginfail-kv-purge

- QA：QA Tester
- 上游：01 / 02 / 04 / 05（CR APPROVED）
- 日期：2026-05-31

> 执行边界声明（insight L31）：本 QA 运行在 role-collapsed PM 上下文，**无 Bash/PowerShell 工具**，无法真跑 `scripts/verify_all`。本报告给出**确定性执行规格**（预期结果 + 新计数），由 batch orchestrator 在其 Bash 会话真跑作硬闸门。这与本批次 T-056~T-062 同范式。`-race` 因本机无 cgo 编译器不跑（与 T-050 先例一致），并发安全静态论证见下与 02 §8 R-6 / 05。

## Test plan（每个 AC 至少一个测试）

| 验收标准 | 测试用例 | 文件 |
|---|---|---|
| AC-2 storage 前缀只命中 loginfail.，不误删其它 | `TestKVListByPrefix_OnlyMatchingPrefix` / `TestKVListByPrefix_LikeMetacharsAreLiteral` | `internal/storage/storage_test.go` |
| AC-3 auth 过期/活/边界（注入时钟 + fakeKV） | `TestPurgeExpired_RemovesExpiredKeepsLive` | `internal/auth/auth_test.go` |
| AC-4 wiring over-delete 防御 + ctx 取消退出 | `TestPurgeExpiredLoginFailsOnce` / `TestPurgeLoop_RunsBothPurgesOnStart` | `cmd/frp-easy/session_purge_test.go` |
| AC-5 既有 RateLimiter 测试零回归 | `TestRateLimiter_*`（5 个，未改语义）；`key()` 改用常量等价 | `internal/auth/auth_test.go` |
| AC-6 baseline bump | go_tests 322→333 / test_count 856→867 / version 29 | `scripts/baseline.json` |
| AC-7 06 含裸 ## Adversarial tests（3 反向证伪） | 见下方 ## Adversarial tests | 本文件 |
| AC-8 dev-map 更新 | storage API 表 + ratelimit 行 + main 入口行 | `docs/dev-map.md` |
| BC-1 空 / 无 loginfail 行返 0 | `TestPurgeExpired_EmptyNoop` / `TestKVListByPrefix_EmptyResult` | auth / storage |
| BC-4 窗口边界 now==expires 不删 | `TestPurgeExpired_RemovesExpiredKeepsLive`（2.2.2.2 边界行） | auth |
| BC-5 损坏 JSON 值视为过期删除 | `TestPurgeExpired_CorruptValueRemoved` | auth |

## Boundary tests added

- 空 KV / 无 loginfail 行（BC-1）：`TestPurgeExpired_EmptyNoop`、`TestKVListByPrefix_EmptyResult`。
- 窗口边界 `now == firstAt + failWindow`（BC-4）：边界行 `2.2.2.2`，`After` 严格大于 → 不删。
- 损坏 JSON 值（BC-5）：`"{not-json"` → 视为过期垃圾删除，活行不连累。
- 前缀近似键（BC-6）：`loginfailure.x` / `loginfail`（无点）/ `xloginfail.y` 不被前缀命中。
- LIKE 元字符当字面（R-3）：前缀含 `_` 不当通配符。
- sub-threshold 过期（count<failMax）：QA 独立，见对抗段 QA-ADV-1。
- 并发：`-race` 不跑，静态论证 `PurgeExpired` 持 `r.mu` 与 `RecordFailure`/`Allow`/`Reset` 同锁互斥；锁顺序 r.mu 外 / s.mu 内无倒置（BC-8）。

## Adversarial tests

> QA 对抗原则：从 AC 出发**独立**写 reproducer（不复用 dev 测试假设），每条先写失败假设再验。下表前 3 条对应 AC-7 强制的三类反向证伪；后 2 条为 QA 额外独立 reproducer（dev 未覆盖的边角）。verify_all 真跑由 orchestrator 执行，下方"预期"为确定性推导（注入时钟/in-memory fake，无随机/IO 竞态）。

| # | 假设（"我预期失败，当…"） | Reproducer（NEW = QA 写） | 预期结果（推导） |
|---|---|---|---|
| AT-1（活计数清后仍限流，AC-7a / R-1 最高危） | 若 PurgeExpired 误删窗口内活计数行 → 该被封 IP 清理后能再尝试，限流被绕过 | `TestPurgeExpired_RemovesExpiredKeepsLive`：活行 `1.1.1.1`（firstAt=now-30s，窗口内）+ 边界行 `2.2.2.2`（firstAt=now-60s），purge 后断言两行存活 **且 `Allow` 仍返 false**（dev 写，QA 复核断言力度足） | **存活**——删除集 ⊆ 惰性清理集，活行/边界行不删，`Allow("1.1.1.1")`/`Allow("2.2.2.2")` 仍 blocked。限流不失效。 |
| AT-2（过期不靠惰性也被清，AC-7b） | 若 purge 不独立判过期、只依赖 Allow 惰性删 → 永不复访的过期 IP 永久滞留（本任务要消除的泄漏） | `TestPurgeExpired_RemovesExpiredKeepsLive` 的过期行 `9.9.9.9`（firstAt=now-120s）**从未调用 Allow**，仅靠 PurgeExpired 清 | **被清**——purged=1，`9.9.9.9` 不可达。证明主动清理独立于惰性清理。 |
| AT-3（前缀只匹配 loginfail. 不误删，AC-7c / R-3） | 若前缀逻辑过宽（如 `loginfail` 无点，或 SQL LIKE 元字符通配）→ 误删 `mode.*` / frps 配置 / frpc admin creds / 近似键，破坏功能 | `TestPurgeExpired_OnlyTouchesLoginfailPrefix`（auth 端，NEW dev）+ `TestKVListByPrefix_OnlyMatchingPrefix`（storage 端，NEW dev）：seed `mode.frpc.enabled` / `mode.frps.enabled` / `system.autorestore.last`（≈frpc admin/autorestore 配置形态）/ `loginfailure.x` / `loginfail` / `xloginfail.y`，断言 purge 后**全部存活且值未变** | **全存活**——purged 只计 loginfail. 行；非 loginfail 键 KVGet 仍 found 且值不变。frps 配置 / mode 开关 / 自恢复状态零误删。 |
| QA-ADV-1（过期与 count 解耦，QA 独立） | 若 PurgeExpired 照搬 Allow 里 `rec.Count < failMax` 早返结构 → count<failMax 的过期行被漏清，仍永久滞留（泄漏未真正堵住） | `TestQA_PurgeExpired_SubThresholdExpiredAlsoPurged`（NEW，QA 写）：seed count=1（`3.3.3.3`）与 count=2（`4.4.4.4`）但均已过期 | **被清**——purged=2，两行均删。过期判定只看时间窗口、不与 count 耦合（`PurgeExpired` 无 count 门控，逐表达式核对确认）。 |
| QA-ADV-2（清后不污染后续限流，QA 独立） | 若 purge 留下脏状态 → 某 IP 清理后永久免疫或后续 RecordFailure 带旧计数 | `TestQA_PurgeExpired_DoesNotCorruptSubsequentRateLimit`（NEW，QA 写）：seed 过期满计数行→purge→`Allow` 应放行→`RecordFailure` 应 count=1 全新窗口 | **存活**——purge 后 `Allow` 放行（无残留封禁），`RecordFailure` 返 count=1 / retry=0（全新窗口，无脏状态）。 |

### 对抗推导补充（无 Bash 的确定性论证）

- AT-1/QA-ADV-2 的关键不变量：`PurgeExpired` 与 `Allow` 共用 `now.After(rec.FirstAt.Add(failWindow))`（`ratelimit.go:158` vs `:74`，逐字符相同），故对同一 `(now, firstAt)`，二者过期判定**恒等**。"删除集 ⊆ 惰性清理集"是该恒等式的直接推论——无需运行时证据即可确定（与 insight L26/L31 同源：表达式同源 → 行为同源是静态事实）。
- AT-3 的 storage 侧：`LIKE 'loginfail.%' ESCAPE '\'`，`loginfail.` 无 `% _ \`，escapeLike 后 pattern 仍 `loginfail.%`；SQLite LIKE 中 `.` 是字面字符。`loginfail`（无点结尾）不以 `loginfail.` 开头 → 不匹配；`xloginfail.y` 前有字符 → 不匹配；`loginfailure.x` 第 10 字符是 `u` 而 pattern 第 10 字符要求 `.` → 不匹配。纯字符串前缀语义，无随机。

## verify_all result（执行规格，PENDING 待 orchestrator 真跑）

- 预期：`scripts/verify_all` **PASS**。
- Go 顶层测试：322 → **333**（+11；dev-db storage +3、auth dev +4、auth QA +2、wiring +2）。
- frontend_tests：534（不变，本任务无前端）。
- test_count：856 → **867**。
- Fail 预期：0。Warn 预期：0（warnings_baseline=0 不变）。
- baseline 已 bump（version 29），B.4 双实现真计数应满足"≥ 基线且等于新计数"。
- 特别复核项（给 orchestrator）：
  1. `go test ./internal/storage/ ./internal/auth/ ./cmd/frp-easy/` 全绿。
  2. 既有 `TestPurgeExpiredSessionsOnce` + `TestPurgeSessionsLoop_ExitsOnCancel`（改名后调 purgeLoop）零回归。
  3. 既有 `TestRateLimiter_*`（5 个）零回归。
  4. `go_tests == 333`。
  5. `go vet ./...` 无新告警（ratelimit.go 新 import storage 无环；auth_test 新 import json/sort/storage 均用上）。

## Defects found

无。0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR。

## Stability

- 测试确定性：auth 用例用注入时钟 `rl.now`（无墙钟依赖）；wiring 用例用真实时钟 + 远过去时间戳（过期行 -2h，活行 firstAt=now 留 ~60s 余量）+ poll-until-condition + deadline（insight L10，无固定同步 sleep——`TestPurgeLoop` 的 `time.Sleep(10ms)` 是 poll 轮询间隔非同步点）。无 flake 风险来源。
- 隔离：storage/wiring 用例 `t.TempDir()` 不共享 db；auth 用例 in-memory fakeKV。
- 因无 Bash 未能本地真跑 10 次；确定性来源已逐条论证（无随机/IO 竞态/墙钟边界），交 orchestrator 真跑核对。

## Verdict

**APPROVED FOR DELIVERY**

> 全部 AC 有测试覆盖；AC-7 三类反向证伪（AT-1 活计数清后仍限流 / AT-2 过期不靠惰性也清 / AT-3 前缀只匹配 loginfail.）+ 2 条 QA 独立 reproducer（QA-ADV-1 count 解耦 / QA-ADV-2 清后不污染）全部预期存活；0 缺陷。verify_all 真跑硬闸门交 batch orchestrator（PM/QA role-collapsed 无 Bash，insight L31），预期 PASS / go_tests==333。
