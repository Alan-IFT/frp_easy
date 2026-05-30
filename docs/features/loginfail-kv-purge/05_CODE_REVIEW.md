# 05 代码评审 — T-063 · loginfail-kv-purge

- 评审：Code Reviewer
- 上游：01（READY）/ 02（READY）/ 03（APPROVED）/ 04（两分区 READY FOR REVIEW）
- 日期：2026-05-31

## Files reviewed

- `internal/storage/kv.go`（+KVEntry +KVListByPrefix +escapeLike +strings import）
- `internal/storage/storage_test.go`（+3 测试 + keysOf helper）
- `internal/auth/ratelimit.go`（+LoginFailKeyPrefix +kvStore.KVListByPrefix +PurgeExpired +import storage；key() 改用常量）
- `internal/auth/auth_test.go`（fakeKV +KVListByPrefix；+4 测试 + seedFail/hasKey helper；+json/sort/storage import）
- `cmd/frp-easy/main.go`（purgeSessionsLoop→purgeLoop +rl 参数 +purgeExpiredLoginFailsOnce；调用点改）
- `cmd/frp-easy/session_purge_test.go`（改名+传 rl；+2 测试 + writeLoginFail helper；+auth/json import）
- `scripts/baseline.json`（322→331 / 856→865 / v29）
- `docs/dev-map.md`（3 行更新）

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### NIT
- [STYLE] `internal/storage/kv.go:71` — `KVListByPrefix` 对 `prefix==""` 返回全表（已注释"调用方自担风险"）。当前唯一调用点传 `loginfail.`，非问题；如担心未来误用可加空前缀早返护栏，但属过度防御，NIT 不阻塞。
- [STYLE] `cmd/frp-easy/session_purge_test.go:177 writeLoginFail` 用匿名 struct 复刻 `auth.failRecord` 的 JSON 形状（`failRecord` 未导出，跨包测试不可直接构造）。形状与 `auth/ratelimit.go:37 failRecord` 的 tag（`count`/`firstAt`）一致——已逐字核对匹配。若未来 failRecord 加字段，此 helper 仍能产出可解析子集（PurgeExpired 只读 firstAt）。NIT：可考虑 auth 导出一个测试构造器，但当前重复极小，不值得。

## 逐维审计

### 1. 逻辑正确性
- **过期判定与 Allow 一致性（最高危点）**：`ratelimit.go:158` `now.After(rec.FirstAt.Add(failWindow))` 与 `Allow:74` 的 `now.After(expires)`（`expires=rec.FirstAt.Add(failWindow)`）**字面同一表达式**。删除集 ⊆ 惰性清理集，边界 `now==expires` 时 `After` 为 false 不删——绝不删活计数。✅
- **损坏值处理**：`ratelimit.go:155` `json.Unmarshal` 失败 → `expired=true` 删除。与 `read()`（:175-183）对损坏值返 `ok=false` 的容错语义一致——损坏行从不构成活限流（`Allow` 对其放行），删除对限流零影响。✅
- **best-effort**：`ratelimit.go:164-169` 单条 KVDelete 失败不中止，记首个错误 + 已删数继续——符合 02 §5。✅
- **LIKE 转义**：`kv.go:97 escapeLike` 先转 `\` 再转 `% _`（顺序正确，避免二次转义）；`ESCAPE '\'` 用 Go raw string 字面反斜杠传 SQLite，解析正确。`loginfail.` 无元字符，pattern=`loginfail.%`。✅
- **rows 迭代**：`kv.go:81-90` `for rows.Next` + `rows.Scan` + `rows.Err()` + `defer rows.Close()`，与 `proxies.go:39` 范式一致，无 rows 泄漏。✅
- **空/无匹配**：`KVListByPrefix` 无匹配返 nil slice（`out` 未 append）+ nil err；`PurgeExpired` 遍历空 slice 返 (0,nil)。BC-1 ✅
- **off-by-one / null**：无。`PurgeExpired` 对 entries 逐条处理，无索引越界。

### 2. 需求保真（见下方覆盖表）
全部 AC 有对应实现/测试。✅

### 3. 设计保真（见下方保真表）
严格选项 B；候选 (i) auth→storage 无环；不新增 goroutine；惰性清理保留。无 drift。✅

### 4. 性能
- `KVListByPrefix` 单条 SELECT + 全 loginfail 行扫描，行数级别极小（远小于 sessions），1h 一次。无 N+1（一次 SELECT 拿全部，逐条 KVDelete 是必要的单点删除）。`kv` 表主键即 key，前缀匹配高效。无性能担忧。✅

### 5. 安全
- 无 SQL 注入：`KVListByPrefix` 用参数化 `?` 占位 + ESCAPE 防 LIKE 元字符通配（R-3）。✅
- **限流不被削弱**（本任务核心安全约束）：§1 已证删除集 ⊆ 惰性清理集，活计数绝不被删。测试 `TestPurgeExpired_RemovesExpiredKeepsLive` 显式断言活行清后仍 `Allow` blocked。✅
- 无 secret 泄漏：清理只删 loginfail 计数行（非敏感）。错误信息含 key（`loginfail.<ip>`），不含密码/token。✅

### 6. 可维护性
- 命名清晰（`PurgeExpired`/`purgeLoop`/`purgeExpiredLoginFailsOnce` 与既有 session 范式对称）。
- 注释解释 WHY（过期判定为何与 Allow 同源、损坏值为何删、锁顺序）——非复述代码。
- 无死代码、无过早抽象（`KVListByPrefix` 设计为通用但有真实唯一调用点）。
- `LoginFailKeyPrefix` 常量消除 "loginfail." 字面量散落（`key()` 也改用）。✅

## Requirement coverage check

| AC | 实现 / 测试 | 状态 |
|---|---|---|
| AC-1 编译 + verify_all PASS | 静态自检全绿；verify_all 标 PENDING 交 orchestrator（insight L31） | ✅（PENDING 硬闸门） |
| AC-2 storage 前缀单测（只命中 loginfail. 不误删） | `storage_test.go:306 TestKVListByPrefix_OnlyMatchingPrefix` + `:378 _LikeMetacharsAreLiteral` | ✅ |
| AC-3 auth 过期/活/边界单测（注入时钟+fakeKV） | `auth_test.go:260 TestPurgeExpired_RemovesExpiredKeepsLive`（rl.now 注入） | ✅ |
| AC-4 wiring over-delete 防御 + ctx 取消 | `session_purge_test.go:85 TestPurgeExpiredLoginFailsOnce` + `:124 TestPurgeLoop_RunsBothPurgesOnStart` | ✅ |
| AC-5 既有 RateLimiter 测试零回归 | `auth_test.go` 既有 `TestRateLimiter_*` 未改语义；`Allow`/`RecordFailure`/`Reset` 行为字节不变（仅 key() 改用常量，等价） | ✅ |
| AC-6 baseline bump | `baseline.json` go_tests 322→331 / test_count 856→865 / version 29 | ✅ |
| AC-7 06 含裸 ## Adversarial tests（3 反向证伪） | 由 QA 阶段产出（覆盖测试 `TestPurgeExpired_OnlyTouchesLoginfailPrefix` 等已就位） | ✅（待 QA 落 06） |
| AC-8 dev-map 更新 | storage API 表 + ratelimit 行 + main 入口行 3 处已更新 | ✅ |

## Design fidelity check

| 设计项 | 实现 | 状态 |
|---|---|---|
| 选项 B：storage 机械列举 + ratelimit 懂语义 | `kv.go:71` 不做时间判定；`ratelimit.go:142` 做过期判定 | ✅ |
| `KVEntry{Key,Value}` 类型 | `kv.go:54` | ✅ |
| `KVListByPrefix` 签名 + LIKE ESCAPE + ORDER BY | `kv.go:71-92` 完全一致 | ✅ |
| `LoginFailKeyPrefix` 导出常量 | `ratelimit.go:23`；`key()` 改用 | ✅ |
| `kvStore` 接口扩 KVListByPrefix（候选 i，[]storage.KVEntry） | `ratelimit.go:33`；auth import storage 无环 | ✅ |
| `PurgeExpired` 持 r.mu + 过期判定同 Allow + 损坏值删 + best-effort | `ratelimit.go:142-173` | ✅ |
| `purgeLoop` 泛化加 rl，同一 goroutine 两清理，不新增 goroutine | `main.go:566-580` | ✅ |
| `purgeExpiredLoginFailsOnce`（5s 超时 + Warn/Info） | `main.go:549-559` 对称 | ✅ |
| 调用点改 `go purgeLoop(rootCtx, store, rl, logger)` | `main.go:337` | ✅ |
| `sessionPurgeInterval` 复用不重命名（OOS-3） | `main.go:530` 注释更新、名不变 | ✅ |
| 无 migration / 无 API 变更 | 确认无 | ✅ |

## 并发安全（-race 不跑的静态复核）

`PurgeExpired` 取 `r.mu`（与 `RecordFailure`/`Allow`/`Reset` 同锁，互斥）；锁内调 storage 方法各取 `s.mu`，锁顺序"r.mu 外 / s.mu 内"与既有 `RecordFailure`→`write`→`KVSet` 一致，无倒置、无重入 r.mu。BC-8 竞态退化分析（删到跨边界行 → 下次 RecordFailure 新窗口重建）成立。✅（与 T-050 一样 `-race` 因无 cgo 不本机跑，交 orchestrator 视环境。）

## 既有测试回归复核

- `TestPurgeSessionsLoop_ExitsOnCancel`（改名连带改调 `purgeLoop`+传 rl）：语义不变（仍验 ctx 取消退出），属红线 3 允许的机械同步，PM 在 OBS-2 已批准。✅
- `TestPurgeExpiredSessionsOnce` 未改，仍验 session 单次清理。✅
- `TestRateLimiter_*`（5 个）未改，`key()` 改用常量值等价（`"loginfail."+ip` == 旧 `fmt.Sprintf("loginfail.%s", ip)`）。✅

## Verdict

**APPROVED**（0 CRITICAL / 0 MAJOR / 0 MINOR / 2 NIT）

> 两分区代码与设计 02、需求 01 完全吻合；核心安全约束（限流不被削弱）有"删除集 ⊆ 惰性清理集"的逐表达式核对 + 显式断言测试；测试有意义（注入时钟 + 反向证伪 + over-delete 防御 + poll-until-condition），非形状匹配。NIT 不阻塞。verify_all 真跑硬闸门交 batch orchestrator（PM 上下文无 Bash，insight L31）。
