# 02 方案设计 — T-063 · loginfail-kv-purge

- 模式：full
- 作者：Solution Architect
- 上游：`01_REQUIREMENT_ANALYSIS.md`（Verdict=READY）
- 日期：2026-05-31

## 1. 架构摘要（Architecture summary）

在 storage 层（`internal/storage/kv.go`）新增一个**机械的、不懂业务语义**的按前缀列举 KV 键值的方法 `KVListByPrefix(ctx, prefix) ([]KVEntry, error)`；在限流器层（`internal/auth/ratelimit.go`，懂窗口语义）新增 `PurgeExpired(ctx) (int, error)`，它调用 `KVListByPrefix("loginfail.")` 拿到全部计数行，按与 `Allow` 字节级同义的过期判定（`now.After(FirstAt + failWindow)`）逐条 `KVDelete` 过期行，返回删除条数。在 wiring 层（`cmd/frp-easy/main.go`），把这次清理挂进**既有 `purgeSessionsLoop` 的同一 goroutine / ticker**——把该 loop 泛化为同时跑 session purge 与 ratelimit purge 两个 once-清理，不新增 goroutine、不新增 ticker。**采用任务交底倾向的选项 B**：过期语义单点收敛在 ratelimit 层（与限流窗口常量同源），storage 只提供机械前缀列举。

## 2. 受影响模块（Affected modules）

| 文件 | 包/层 | 改动 | 分区 |
|---|---|---|---|
| `internal/storage/kv.go` | storage | 新增 `KVEntry` 类型 + `KVListByPrefix` 方法 | dev-db |
| `internal/storage/storage_test.go` | storage 测试 | 新增 `KVListByPrefix` 前缀正确性 + 前缀误匹配防御单测 | dev-db |
| `internal/auth/ratelimit.go` | auth/限流 | 扩 `kvStore` 接口加 `KVListByPrefix`；新增 `RateLimiter.PurgeExpired`；导出 `LoginFailKeyPrefix` 常量 | dev-backend |
| `internal/auth/auth_test.go` | auth 测试 | 扩 `fakeKV` 实现新接口方法；新增 `PurgeExpired` 过期/活/边界单测 | dev-backend |
| `cmd/frp-easy/main.go` | wiring | 把 ratelimit purge 挂进 `purgeSessionsLoop`；新增 `purgeExpiredLoginFailsOnce` | dev-backend |
| `cmd/frp-easy/session_purge_test.go`（或新建 `loginfail_purge_test.go`） | wiring 测试 | 新增 ratelimit purge wiring 单测 + 同一 loop 触发两清理的验证 | dev-backend |
| `scripts/baseline.json` | 基线 | bump go_tests / test_count / version | dev-backend |
| `docs/dev-map.md` | 文档 | storage 对外 API 表 + auth ratelimit 行 + main purge loop 行 | dev-backend（文档随代码） |

> 注意：`internal/storage/**` 属 **dev-db**；`internal/auth/**` + `cmd/**` + `scripts/verify_all.*` + `docs/dev-map.md` 属 **dev-backend**（见 `.harness/agents/dev-backend.md` owned paths）。baseline.json 不在任一 owned glob 显式列出，按惯例随测试改动由 dev-backend 收尾 bump（与 T-059 等前例一致）。

## 3. 模块分解（新增 public API）

### 3.1 storage 层（dev-db，纯机械）

```go
// KVEntry 是一条 KV 行（KVListByPrefix 返回用）。
type KVEntry struct {
    Key   string
    Value string
}

// KVListByPrefix 返回所有 key 以 prefix 开头的 KV 行（升序按 key）。
// prefix 为空串时返回全部行（调用方自担风险，本项目不会这么用）。
// 仅 IO/SQL 真错时返回 err；无匹配返回空 slice + nil。
//
// 实现：SQL `SELECT key, value FROM kv WHERE key LIKE ? ESCAPE '\' ORDER BY key`，
// 参数为 escapeLike(prefix) + "%"。escapeLike 把 prefix 中的 LIKE 元字符
// `\` `%` `_` 转义（key 'loginfail.' 本身不含这些字符，转义是面向未来调用方的防御）。
func (s *Store) KVListByPrefix(ctx context.Context, prefix string) ([]KVEntry, error)
```

- **为何只列举不删**：过期判定需读 JSON 值内 `firstAt` 并与窗口常量比较，这是 ratelimit 的业务语义，不属于 storage。storage 给"机械的按前缀取行"，过期判定 100% 留在 ratelimit（选项 B，过期语义单点一致）。
- **为何加 `ORDER BY key`**：确定性输出，让单测断言稳定（非功能必需，但零成本，便于测试与日志）。
- **LIKE ESCAPE**：SQLite `LIKE` 中 `%` 和 `_` 是通配符；`loginfail.` 不含这两个字符（`.` 是字面字符），所以 `LIKE 'loginfail.%'` 对本任务已安全。但为防御未来调用方传含元字符的前缀（BC-6 精神），实现统一走 `escapeLike(prefix) + "%"` + `ESCAPE '\'`，转义 `\ % _` 三个字符。这是纯防御，对当前唯一调用点 `loginfail.` 无行为差异。

### 3.2 auth/限流层（dev-backend，懂语义）

```go
// LoginFailKeyPrefix 是 RateLimiter 持久化键的统一前缀（导出，供 PurgeExpired 与
// 测试引用，避免 "loginfail." 字面量散落）。key(ip) == LoginFailKeyPrefix + ip。
const LoginFailKeyPrefix = "loginfail."

// kvStore 接口扩一个方法（RateLimiter 持久化所需最小接口）：
type kvStore interface {
    KVGet(ctx context.Context, key string) (string, bool, error)
    KVSet(ctx context.Context, key, value string) error
    KVDelete(ctx context.Context, key string) error
    KVListByPrefix(ctx context.Context, prefix string) ([]storage.KVEntry, error) // 新增
}

// PurgeExpired 删除所有已过期的 loginfail.<ip> 计数行，返回删除条数。
// 过期判定与 Allow 字节级同义：now.After(rec.FirstAt.Add(failWindow))。
// 损坏的 JSON 值视为过期（垃圾回收，见风险 R-2 决定）。
// 与 RecordFailure/Allow/Reset 共享 r.mu，保证读-判-删的原子性。
func (r *RateLimiter) PurgeExpired(ctx context.Context) (purged int, err error)
```

**`kvStore` 接口引入 `storage.KVEntry` 的依赖方向问题**：当前 `internal/auth/ratelimit.go` 不 import `internal/storage`（用本地接口解耦，便于注入 fakeKV）。引入 `storage.KVEntry` 会让 auth 依赖 storage。**两个候选**：

- **候选 (i)（采用）**：`KVListByPrefix` 返回 `[]storage.KVEntry`，auth import storage。代价：auth → storage 单向依赖（无环——storage 不依赖 auth）。这是项目已有的合理方向（dev-backend 消费 dev-db），且 `cmd/frp-easy/main.go` 早已用 `auth.NewRateLimiter(store)` 把 `*storage.Store` 注入 auth，依赖事实上已存在（只是经接口隐式）。
- 候选 (ii)（不采用）：在 auth 包内定义本地 `kvEntry` 类型，接口方法返回 `[]struct{Key,Value string}` 或 auth 本地类型。代价：storage 与 auth 各一份等价类型，需手工对齐，违反"不重复定义同一类型"（dev-backend "bad" 清单）。

**决定**：采用候选 (i)。`internal/storage` 是底层包，auth 依赖它无环、符合分层。fakeKV 测试仍可工作——`auth_test.go` 的 `fakeKV` 实现 `KVListByPrefix` 返回 `[]storage.KVEntry` 即可（auth_test 已可 import storage）。

### 3.3 wiring 层（dev-backend）

```go
// purgeExpiredLoginFailsOnce 清理一次过期 loginfail 计数行（带 5s 超时，错误仅告警）。
// 与 purgeExpiredSessionsOnce 对称。
func purgeExpiredLoginFailsOnce(ctx context.Context, rl *auth.RateLimiter, logger *slog.Logger) {
    pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    n, err := rl.PurgeExpired(pctx)
    if err != nil {
        logger.Warn("loginfail purge failed", "err", err)
        return
    }
    if n > 0 {
        logger.Info("loginfail purge", "removed", n)
    }
}

// purgeSessionsLoop 泛化签名，增加 rl 参数；启动时与每个 ticker tick 都跑两个 once。
// 重命名为 purgeLoop（语义已不止 sessions）—— 见 §9 兼容性。
func purgeLoop(ctx context.Context, store *storage.Store, rl *auth.RateLimiter, logger *slog.Logger) {
    purgeExpiredSessionsOnce(ctx, store, logger)
    purgeExpiredLoginFailsOnce(ctx, rl, logger)
    ticker := time.NewTicker(sessionPurgeInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            purgeExpiredSessionsOnce(ctx, store, logger)
            purgeExpiredLoginFailsOnce(ctx, rl, logger)
        }
    }
}
```

调用点 `main.go:335` 由 `go purgeSessionsLoop(rootCtx, store, logger)` 改为 `go purgeLoop(rootCtx, store, rl, logger)`（`rl` 已在 main.go:270 构造）。`sessionPurgeInterval` 包级 var 复用不动（OOS-3：不新增独立间隔；语义上它现在是"后台清理周期"，注释相应更新，不重命名以减小 diff）。

## 4. 数据模型变更（Data model changes）

**无**。不加表、不加列、不加索引、不加 migration（OOS-2）。`kv` 表已存在（`migrations/0001_init`）。`KVListByPrefix` 只读现有 `kv(key, value)`。

> 性能旁注：`kv` 表主键即 `key`（PRIMARY KEY），`LIKE 'loginfail.%'` 前缀匹配在 SQLite 上对 rowid/主键表能用前缀范围（取决于 collation）；但即便全表扫，loginfail 行数级别极小（远小于 sessions），1h 一次，NFR-2 无忧。不加索引。

## 5. API 契约（API contracts）

**无 REST / HTTP 变更**（OOS-5）。纯进程内函数调用：

- `storage.KVListByPrefix(ctx, prefix) ([]KVEntry, error)` — 见 §3.1。
- `auth.(*RateLimiter).PurgeExpired(ctx) (int, error)` — 见 §3.2。
- `auth.LoginFailKeyPrefix` 常量导出。

错误语义：两者均"仅 IO/SQL 真错返 err"；`PurgeExpired` 内单条 `KVDelete` 失败的处理见 §6 流程（累计已删数 + 返回首个错误，best-effort 清理剩余）。

## 6. 序列 / 流程（Sequence / flow）

```
后台 goroutine（rootCtx）
  └─ purgeLoop(rootCtx, store, rl, logger)
       ├─ 启动立即：
       │    ├─ purgeExpiredSessionsOnce(...)            # T-046 既有，不动
       │    └─ purgeExpiredLoginFailsOnce(rootCtx, rl, logger)
       │         └─ pctx = WithTimeout(rootCtx, 5s)
       │              └─ rl.PurgeExpired(pctx):
       │                   r.mu.Lock()
       │                   entries = kv.KVListByPrefix(pctx, "loginfail.")   # storage: SELECT ... LIKE
       │                   for e in entries:
       │                       rec, ok = json.Unmarshal(e.Value)
       │                       expired = (!ok)  ||  now.After(rec.FirstAt + failWindow)   # 损坏=过期(R-2)
       │                       if expired: kv.KVDelete(pctx, e.Key); purged++
       │                   r.mu.Unlock()
       │                   return purged, firstErr
       └─ 每个 ticker tick（sessionPurgeInterval=1h）：重复上面两个 once
       └─ <-ctx.Done(): return（无 goroutine 泄漏）
```

**过期判定与 `Allow` 的一致性证明**（NFR-1 核心）：`Allow`（ratelimit.go:74-79）用 `expires := rec.FirstAt.Add(failWindow); if now.After(expires) { KVDelete; return allow }`。`PurgeExpired` 用**完全相同**的 `now.After(rec.FirstAt.Add(failWindow))`。故 `PurgeExpired` 删除的行集 ⊆ `Allow` 在同一 `now` 下会判 allow 并惰性删除的行集——即清理只删"惰性清理本就会删"的行，**绝不**删任何 `Allow` 仍判 blocked 的活行。窗口边界 `now == expires` 时 `After` 为 false → 不删（BC-4 满足，与惰性清理同义）。`now` 取 `r.now()`（与 `Allow`/`RecordFailure` 同一时钟源，测试可注入）。

## 7. 复用审计（Reuse audit）

| 需求 | 已有 | 文件路径 | 决定 |
|---|---|---|---|
| 周期清理 loop（启动即清 + ticker + ctx 退出） | `purgeSessionsLoop` | `cmd/frp-easy/main.go:546` | 泛化为 `purgeLoop`，加 rl 参数，复用同 goroutine/ticker |
| 单次清理（5s 超时 + Warn/Info 日志） | `purgeExpiredSessionsOnce` | `cmd/frp-easy/main.go:531` | 对称新增 `purgeExpiredLoginFailsOnce` |
| 包级可测间隔 var | `sessionPurgeInterval` | `cmd/frp-easy/main.go:528` | 复用（OOS-3，不新增） |
| 周期清理返回 count 形态 | `PurgeExpiredSessions (int64, error)` | `internal/storage/sessions.go:103` | 形态对齐（PurgeExpired 返回 int） |
| KV 行 SELECT 迭代 | `ListProxies` rows 迭代 | `internal/storage/proxies.go:39` | 参照 QueryContext + rows.Next + rows.Scan + rows.Err 范式 |
| in-memory kvStore fake | `fakeKV` | `internal/auth/auth_test.go:102` | 扩一个 `KVListByPrefix` 方法（遍历 map 前缀匹配） |
| 注入时钟 | `rl.now` | `internal/auth/ratelimit.go:48` | 复用，PurgeExpired 用 `r.now()` |
| wiring once 测试范式 | `TestPurgeExpiredSessionsOnce` / `TestPurgeSessionsLoop_ExitsOnCancel` | `cmd/frp-easy/session_purge_test.go` | 对称新增 loginfail 版 |
| 过期判定语义 | `Allow` 的 `now.After(rec.FirstAt.Add(failWindow))` | `internal/auth/ratelimit.go:74` | 字节级复用同表达式 |

## 8. 风险分析（Risk analysis）

- **R-1（误删活计数 → 限流失效，最高危）**：若过期判定比 `Allow` 激进，会删窗口内活行让攻击者绕过限流。**缓解**：§6 已证明 `PurgeExpired` 用与 `Allow` 完全相同的表达式，删除集 ⊆ 惰性清理集；AC-3 单测显式断言"窗口内行不被删"+ "窗口边界 == 不删" + adversarial AT-1 反向证伪（活行清理后仍能触发 429）。
- **R-2（损坏 JSON 值）**：某 `loginfail.<ip>` 行 value 非合法 `{count, firstAt}`（理论不应出现，KV 是文本表）。**决定：视为过期删除**（`json.Unmarshal` 失败 → `expired=true`）。理由：(a) 损坏的 loginfail 行对限流无意义——`Allow`/`RecordFailure` 内 `read()` 对解析失败也返 `(failRecord{}, false)`，即把损坏行当作"无记录"放行，故删除它与 `Allow` 的现有容错语义一致（都不把损坏行当作活限流），不会因删除放宽限流；(b) 符合"清理垃圾"的任务意图。与 `Allow` 的一致性：`Allow` 遇损坏行返 `read` 的 `ok=false` → `Allow` 返 `(true, 0)` 放行但**不删**（惰性清理只在 `rec.Count>=failMax` 且窗口过期分支删）。故 PurgeExpired 删损坏行是比惰性清理**额外**的垃圾回收——但因损坏行从不构成活限流（`Allow` 对它已放行），删除它对限流行为**零影响**（删前删后该 IP 都被放行）。AC/adversarial 覆盖此点。
- **R-3（前缀误匹配误删其它 KV）**：`mode.frpc.enabled`、frps 配置键、`system.autorestore.last`、frpc admin creds 等若被前缀逻辑误命中会破坏功能。**缓解**：前缀精确为 `loginfail.`（含点）；`KVListByPrefix` 用 LIKE ESCAPE 防元字符；AC-2 storage 单测 + AT-3 adversarial 反向证伪（造 `mode.*` / `loginfailure.x`（无点变体）/ `system.*` 行，断言清理后全部存活）。
- **R-4（接口扩展破坏既有 fakeKV / 其它 kvStore 实现者）**：`kvStore` 接口加方法会让所有实现者必须新增方法。**缓解**：实现者只有 `*storage.Store`（生产）与 `fakeKV`（测试）两个；两者都在本任务内同步加方法。grep 确认无第三个实现者（见 §11 实现交底 grep 清单）。
- **R-5（auth → storage 依赖引入环）**：**缓解**：storage 是底层包不 import auth（已 grep 确认），单向依赖无环；`main.go` 早已 `auth.NewRateLimiter(store)` 注入，依赖事实已存在。
- **R-6（并发：清理与 RecordFailure 竞态）**：**缓解**：`PurgeExpired` 持 `r.mu`（与 `RecordFailure`/`Allow`/`Reset` 同锁），读-判-删原子；DB 层 `s.mu` 兜底。`-race` 本机无 cgo 不跑，静态论证：所有 RateLimiter 方法串行化于 `r.mu`，无新锁顺序（PurgeExpired 内只取 `r.mu` 一把锁，调 storage 方法各自取 `s.mu`，与既有 `RecordFailure`→`write`→`KVSet` 的锁顺序 `r.mu` 外、`s.mu` 内一致，无倒置）。

## 9. 迁移 / 上线计划（Migration / rollout）

- 无 DB migration、无 API 破坏性变更，无需 MIGRATION.md。
- **函数重命名兼容性**：`purgeSessionsLoop` → `purgeLoop`、参数加 `rl`。这是内部未导出符号（小写），无外部引用，仅 main.go + 其测试引用。`session_purge_test.go` 引用 `purgeSessionsLoop` 需同步改为 `purgeLoop`（并传 rl）。`purgeExpiredSessionsOnce` 名不变（仍只清 session）。这是纯内部重构，无上线风险。
- 回滚：还原三处 diff（kv.go / ratelimit.go / main.go）即可，无数据残留（清理只删过期垃圾，回滚后退回惰性清理，行为收敛）。

## 10. 范围外澄清（Out-of-scope clarifications）

- 本设计不改限流算法、不改 KV JSON 格式、不加配置项、不动惰性清理路径（与 01 OOS 一致）。
- 不把 session purge 与 loginfail purge 的过期判定逻辑合并（两者判定源不同）——仅在调度层（`purgeLoop`）合并触发。
- `KVListByPrefix` 设计为通用前缀列举（未来可复用于其它 KV 命名空间清理），但本任务唯一调用点是 `PurgeExpired` 的 `loginfail.`。

## 11. 分区分配（Partition assignment）

| 文件 | 分区 | 新增/编辑 | 依赖 |
|---|---|---|---|
| `internal/storage/kv.go` | dev-db | 编辑（+KVEntry +KVListByPrefix） | — |
| `internal/storage/storage_test.go` | dev-db | 编辑（+KVListByPrefix 单测） | 上行 |
| `internal/auth/ratelimit.go` | dev-backend | 编辑（+LoginFailKeyPrefix +kvStore 接口方法 +PurgeExpired） | dev-db（消费 KVListByPrefix/KVEntry） |
| `internal/auth/auth_test.go` | dev-backend | 编辑（fakeKV +KVListByPrefix；+PurgeExpired 单测） | 上行 |
| `cmd/frp-easy/main.go` | dev-backend | 编辑（purgeLoop 泛化 +purgeExpiredLoginFailsOnce；调用点改 rl） | 依赖 ratelimit.PurgeExpired |
| `cmd/frp-easy/session_purge_test.go` | dev-backend | 编辑（purgeSessionsLoop→purgeLoop 改名 + 传 rl；+loginfail wiring 单测） | 上行 |
| `scripts/baseline.json` | dev-backend | 编辑（bump go_tests/test_count/version） | 所有测试就位后 |
| `docs/dev-map.md` | dev-backend | 编辑（storage API 表 + ratelimit 行 + main loop 行） | 收尾 |

### Dispatch order（派发顺序）

1. **dev-db** — 先在 storage 层加 `KVEntry` + `KVListByPrefix` + 其单测（dev-backend 的 ratelimit 编译依赖该类型/方法存在）。
2. **dev-backend** — 消费 storage 新 API：ratelimit.PurgeExpired + auth_test + main.go wiring + wiring 测试 + baseline bump + dev-map。

### Parallelism（并行性）

无并行 — 严格串行。dev-backend 的 ratelimit.go 引用 `storage.KVEntry` 与 `KVListByPrefix`，必须 dev-db 先就位才能编译。

### grep 交底（给 Developer，证 R-4/R-5）

- `kvStore` 接口实现者：grep `KVListByPrefix\|func.*KVDelete` → 仅 `*storage.Store`（生产）+ `fakeKV`（auth_test）；无第三实现者。Developer 实现时若发现还有别处实现 kvStore，按 `BLOCKED ON DESIGN` 回报。
- storage 不 import auth：grep `internal/auth` in `internal/storage/*.go` → 应为空（确认无环）。

## 12. 裁决（Verdict）

**READY** — 设计完整，过期判定与 `Allow` 字节级一致的正确性证明在 §6/§8 R-1，损坏值处理在 §8 R-2 定夺，前缀防御在 §3.1/§8 R-3，依赖方向在 §3.2 候选 (i) 定夺。无阻塞，Junior 开发者可据此实现无需再做设计决策。
