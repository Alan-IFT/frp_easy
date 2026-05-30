# 04 开发记录 — T-063 · loginfail-kv-purge

- 模式：full（partitioned：dev-db → dev-backend）
- 上游：`03_GATE_REVIEW.md`（APPROVED FOR DEVELOPMENT）
- 日期：2026-05-31

---

# Development Record — DB partition（dev-db）

## 范围

storage 层加**机械的**按前缀列举 KV 能力（选项 B：过期语义不在此层）。无 schema/migration 变更。

## 改动文件

- `internal/storage/kv.go`：
  - 新增 `KVEntry{Key, Value string}` 类型。
  - 新增 `func (s *Store) KVListByPrefix(ctx, prefix) ([]KVEntry, error)`——`SELECT key, value FROM kv WHERE key LIKE escapeLike(prefix)+'%' ESCAPE '\' ORDER BY key`，QueryContext + rows 迭代（参照 proxies.go:39 范式）；无匹配返回 nil slice + nil；仅 IO/SQL 真错返 err。
  - 新增未导出 `escapeLike(s)` 转义 LIKE 元字符 `\` `%` `_`（`\` 先转义）。
  - import 加 `strings`。
- `internal/storage/storage_test.go`：+3 顶层测试。

## Schema change

无。`kv(key, value)` 表已存在（migrations/0001_init），本任务只读，不加表/列/索引/migration。

## Rollback plan

无 migration，无需回滚 DDL。代码回滚 = 还原 kv.go 三处新增（KVEntry/KVListByPrefix/escapeLike + strings import）即可，无数据残留。

## Data impact

- 影响行：N/A（只读列举，不写）。
- Backfill：无。

## 新增测试（dev-db，+3 顶层 Test）

| 测试 | 覆盖 | AC/BC |
|---|---|---|
| `TestKVListByPrefix_OnlyMatchingPrefix` | 只命中 `loginfail.` 行、升序、value 正确；近似键（`loginfailure.x` / `loginfail`（无点）/ `xloginfail.y`）+ `mode.*` / `system.*` 不泄露 | AC-2 / R-3 / BC-6 |
| `TestKVListByPrefix_EmptyResult` | 无任何 loginfail 行时返回空 slice + nil 不报错 | BC-1 |
| `TestKVListByPrefix_LikeMetacharsAreLiteral` | 前缀含 `_` 元字符时被当字面（`loginfail._` 只命中 `loginfail._x`，不因 `_` 通配命中 `loginfailYx`） | R-3 / OBS-3 |

辅助：新增测试本地 helper `keysOf([]KVEntry) []string`（诊断用）。

## 设计保真度

- 严格选项 B：storage 不做任何过期/时间判定，纯前缀列举。
- LIKE ESCAPE 转义按 §3.1 实现（防御性，对当前 `loginfail.` 调用点行为等价）。
- 所有 SQL 仍只在 internal/storage（红线满足）。

## Coordination

dev-backend 将消费 `storage.KVEntry` + `KVListByPrefix`（ratelimit.PurgeExpired）。已确认 storage 不 import auth（无环），auth→storage 单向依赖由 dev-backend 引入。

## verify_all result

PM/dev role-collapsed 上下文无 Bash/PS（insight L31），未真跑。静态自检：
- `go vet` 级别审视：kv.go 新增无语法/类型错误；`strings` 已 import；`QueryContext`/`rows.Scan`/`rows.Err`/`rows.Close` 范式与 proxies.go 一致。
- 新增 3 测试用 `newTestStore(t)`（t.TempDir 隔离），断言确定（无时间依赖、无 sleep）。
预期 PASS。go_tests 增量在 dev-backend 收尾统一 bump baseline。

## Verdict

READY FOR REVIEW (DB partition complete)

---

# Development Record — Backend partition（dev-backend）

## 范围

消费 storage 新 API：在限流层加懂窗口语义的过期清理，挂进既有后台 loop（不新增 goroutine）。

## 改动文件

- `internal/auth/ratelimit.go`：
  - import 加 `github.com/frp-easy/frp-easy/internal/storage`（auth→storage 单向依赖，无环——已核验 storage 不 import auth，仅 admin.go:14 注释提及）。
  - 导出常量 `LoginFailKeyPrefix = "loginfail."`；`key(ip)` 改为 `LoginFailKeyPrefix + ip`（消字面量散落，行为字节不变）。
  - `kvStore` 接口扩 `KVListByPrefix(ctx, prefix) ([]storage.KVEntry, error)`。
  - 新增 `func (r *RateLimiter) PurgeExpired(ctx) (purged int, err error)`：持 `r.mu` → `KVListByPrefix(LoginFailKeyPrefix)` → 逐条 `json.Unmarshal(failRecord)`，过期判定 `now.After(rec.FirstAt.Add(failWindow))`（与 `Allow:74` 字节级同义）；损坏 JSON 值视为过期删除（R-2）；best-effort（单条 KVDelete 失败不中止，返首个错误 + 已删数）。
- `internal/auth/auth_test.go`：
  - `fakeKV` 扩 `KVListByPrefix`（遍历 map + `strings.HasPrefix` + `sort.Slice` 升序对齐真 Store 的 `ORDER BY key`）。
  - import 加 `encoding/json` / `sort` / `internal/storage`。
  - +4 顶层测试（见下表）+ 私有 helper `seedFail` / `hasKey`。
- `cmd/frp-easy/main.go`：
  - `purgeSessionsLoop` → `purgeLoop`（加 `rl *auth.RateLimiter` 参数；启动即清 + 每 tick 都跑 `purgeExpiredSessionsOnce` + `purgeExpiredLoginFailsOnce`；共用同一 goroutine/ticker；ctx 取消退出）。
  - 新增 `purgeExpiredLoginFailsOnce(ctx, rl, logger)`（5s 超时、Warn 不致命、删 >0 记 Info，对称 `purgeExpiredSessionsOnce`）。
  - 调用点 `main.go:335` 改 `go purgeLoop(rootCtx, store, rl, logger)`（`rl` 已在 :270 构造）。
  - `sessionPurgeInterval` var 复用、不重命名（OOS-3），注释更新为"后台清理周期（session + loginfail 共用）"。
- `cmd/frp-easy/session_purge_test.go`：
  - `TestPurgeSessionsLoop_ExitsOnCancel` 改调 `purgeLoop` + 传 `rl`（改名连带机械同步，红线 3 允许，PM 已批准 OBS-2）。
  - import 加 `internal/auth` / `encoding/json`。
  - +2 顶层测试 + 私有 helper `writeLoginFail`（绕 RecordFailure 精确控制 firstAt；用真实时钟过去时间戳，因 `rl.now` 是 auth 私有字段跨包不可注入）。
- `scripts/baseline.json`：go_tests 322→331 / test_count 856→865 / version 28→29。
- `docs/dev-map.md`：storage API 表 +KVListByPrefix/KVEntry、auth ratelimit 行 +PurgeExpired/LoginFailKeyPrefix、main 入口行 +purgeLoop。

## API 契约（与 02 §5 一致）

- `storage.KVListByPrefix(ctx, prefix) ([]KVEntry, error)`（dev-db 已就位）。
- `auth.(*RateLimiter).PurgeExpired(ctx) (int, error)`。
- `auth.LoginFailKeyPrefix` 常量导出。
- 无 REST/HTTP 变更。

## 新增测试（dev-backend，+6 顶层 Test = auth 4 + wiring 2）

| 测试 | 文件 | 覆盖 | AC/BC |
|---|---|---|---|
| `TestPurgeExpired_RemovesExpiredKeepsLive` | auth_test | 过期删、活留、边界 `now==expires` 不删、活行清后仍 `Allow` blocked | AC-3 / BC-2/3/4 / NF-S / R-1 |
| `TestPurgeExpired_CorruptValueRemoved` | auth_test | 损坏 JSON 值视为过期删、活行不连累 | BC-5 / R-2 |
| `TestPurgeExpired_OnlyTouchesLoginfailPrefix` | auth_test | mode.* / system.* / 近似键全存活不被碰（反向证伪） | AC-7(c) / R-3 |
| `TestPurgeExpired_EmptyNoop` | auth_test | 无 loginfail 行返 0 不报错 | BC-1 |
| `TestPurgeExpiredLoginFailsOnce` | session_purge_test | wiring 一次清理过期删/活留/非 loginfail 键不碰（over-delete 防御，真 DB） | AC-4 |
| `TestPurgeLoop_RunsBothPurgesOnStart` | session_purge_test | purgeLoop 启动即触发 session+loginfail 双清理（poll-until-condition + deadline）+ ctx 取消退出 | AC-4 / IS-2 / IS-6 |

## 设计保真度

- 过期判定 §6/§8 R-1 集合包含证明落实：`PurgeExpired` 与 `Allow` 用同一表达式 `now.After(rec.FirstAt.Add(failWindow))`，边界 `==` 不删。
- 候选 (i)（auth→storage）落实；fakeKV 在测试内同步实现接口新方法（R-4）。
- 不新增 goroutine（IS-2）：`purgeLoop` 单 goroutine 跑两清理。
- 惰性清理（`Allow` 内 KVDelete）保留不动（OOS-1）。

## 并发安全（-race 静态论证）

本机无 cgo，`-race` 不跑（与 T-050 先例一致）。论证：`PurgeExpired` 取 `r.mu`（与 `RecordFailure`/`Allow`/`Reset` 同一把锁，互斥串行化）；锁内调 `KVListByPrefix`/`KVDelete` 各自取 `s.mu`，锁顺序"先 r.mu 后 s.mu"与既有 `RecordFailure`→`write`→`KVSet` 一致，无倒置。清理删除的是过期行（不在任何活窗口），即便与 `RecordFailure` 竞态删到一个刚跨边界的行，下次 `RecordFailure` 以新窗口 `Count=1` 重建，限流语义不破坏（BC-8）。

## verify_all result

PM/dev role-collapsed 上下文无 Bash/PS（insight L31），未真跑。静态自检：
- 编译：ratelimit.go import storage 无环；main.go `auth` 已 import；session_purge_test.go +auth/+encoding/json import；`fmt` 在 PurgeExpired 仍用（保留）。无未用 import（auth_test 的 json/sort/storage 均用上）。
- 测试确定性：auth 测试用注入时钟 `rl.now`；wiring 测试用真实时钟 + 远过去时间戳（活行 firstAt=now 留 ~60s 余量，无 flake）+ poll-until-condition + deadline（无固定同步 sleep，insight L10）。
- 全部 `t.TempDir()` 隔离（storage 真 DB 用例）/ in-memory fakeKV（auth 用例）。
预期 PASS；go_tests 322→331（+9）。

## Verdict

READY FOR REVIEW (Backend partition complete) — 两分区全部就位。

