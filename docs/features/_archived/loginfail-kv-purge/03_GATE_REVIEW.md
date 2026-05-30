# 03 闸门评审 — T-063 · loginfail-kv-purge

- 模式：full
- 评审：Gate Reviewer
- 上游：`01_REQUIREMENT_ANALYSIS.md`（READY）+ `02_SOLUTION_DESIGN.md`（READY）
- 日期：2026-05-31

## 独立代码核验（先验证设计声明，再裁决）

逐条核验设计中引用的既有符号与依赖断言（gate-reviewer 红线 2/3：不盲信，读真代码）：

| 设计声明 | 核验方法 | 结果 |
|---|---|---|
| `Allow` 过期判定 `now.After(rec.FirstAt.Add(failWindow))` | 读 `internal/auth/ratelimit.go:74-79` | ✅ 字面如此；`expires := rec.FirstAt.Add(failWindow); if now.After(expires)` |
| `failWindow=60s` / `failMax=5` 常量 | 读 `ratelimit.go:13-16` | ✅ 存在 |
| `kv` 表只读 + 现有 KV API | 读 `internal/storage/kv.go` | ✅ 仅 KVGet/KVSet/KVDelete，无前缀列举（证实 backlog 根因） |
| `PurgeExpiredSessions (int64, error)` 范式 | 读 `internal/storage/sessions.go:103-114` | ✅ `s.mu.Lock` + `ExecContext` + `RowsAffected` |
| `purgeSessionsLoop` / `purgeExpiredSessionsOnce` / `sessionPurgeInterval` | 读 `cmd/frp-easy/main.go:528-558` | ✅ 三者俱在；loop 启动即清 + ticker + ctx.Done 退出 |
| 调用点 `go purgeSessionsLoop(rootCtx, store, logger)` | 读 `main.go:335` | ✅；`rl` 已在 `main.go:270` 构造，wiring 可拿到 |
| `fakeKV` in-memory + `rl.now` 注入时钟 | 读 `internal/auth/auth_test.go:101-126` | ✅；fakeKV 实现三方法，扩 KVListByPrefix 可行 |
| `ListProxies` rows 迭代范式 | 读 `internal/storage/proxies.go:39` | ✅ QueryContext + rows 迭代可参照 |
| R-4：kvStore 无第三实现者 | grep `func ... KVDelete` 全仓 | ✅ 仅 `*storage.Store`(kv.go:43) + `fakeKV`(auth_test.go:121) |
| R-5：storage 不 import auth（无环） | grep `internal/auth` in storage + 读 admin.go:14 | ✅ 仅 admin.go **注释**引用（非 import），storage 不 import auth，候选 (i) 无环 |
| wiring 测试范式 `TestPurgeExpiredSessionsOnce` / `..._ExitsOnCancel` | 读 `session_purge_test.go` | ✅ over-delete 防御 + ctx 取消退出范式可对称复刻 |

全部设计代码声明经核验属实，**无虚构符号、无依赖环**。

## 8 维审计

| # | 维度 | 判定 | 理由 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | IS-1~IS-6 全可测；过期判定锚定到 `Allow` 既有表达式（IS-1）消除"过期"歧义；BC-1~BC-8 覆盖空/活/混合/边界/损坏/误匹配/取消/并发。 |
| 2 | 设计完整性 | PASS | 每个 in-scope 行为有对应函数：IS-1/3→`PurgeExpired`+`KVListByPrefix`；IS-2→`purgeLoop` 同 goroutine；IS-4→返回 count+Info/Warn 日志；IS-5→前缀 `loginfail.`+LIKE ESCAPE；IS-6→复用 ctx.Done 退出。 |
| 3 | 复用正确性 | PASS | §7 复用审计逐项核验属实（loop/once/interval/count 形态/rows 迭代/fakeKV/时钟/过期表达式 8 项均指向真实文件行）；未重造已有轮子。 |
| 4 | 风险覆盖 | PASS | R-1（误删活计数，最高危）给了 §6 集合包含证明（PurgeExpired 删除集 ⊆ 惰性清理集）；R-2~R-6 含损坏值/前缀误匹配/接口扩展/依赖环/并发，每条带缓解 + 测试映射。未见遗漏的明显风险。 |
| 5 | 迁移安全 | PASS | 无 DB migration、无 API 破坏（OOS-2/OOS-5）；内部未导出符号重命名（purgeSessionsLoop→purgeLoop）仅 main.go + 其测试引用，§9 已点明同步改测试，无外部引用。 |
| 6 | 边界处理 | PASS | BC-4 窗口边界 `==` 不删（与 `After` 严格大于一致）、BC-5 损坏值 §8 R-2 定夺为删（并论证对限流零影响）、BC-6 前缀误匹配 LIKE ESCAPE + 无点变体测试、BC-8 并发持 `r.mu` 静态论证——均有设计落点。 |
| 7 | 测试可行性 | PASS | AC-2（storage 前缀单测，t.TempDir）/AC-3（fakeKV+注入时钟过期/活/边界）/AC-4（wiring over-delete 防御 + ctx 取消）/AC-5（既有 RateLimiter 测试零回归）全部可用既有范式测；无不可验证 AC。`-race` 本机无 cgo 静态论证（与 T-050 一致先例）。 |
| 8 | 范围外清晰 | PASS | OOS-1~OOS-6 明确（不改限流算法/不加 migration/不加配置项/不合并过期判定/不碰前端 API/不改 JSON 格式）；§10 进一步澄清惰性清理保留、KVListByPrefix 通用但唯一调用点 loginfail.，开发者不会过度构建。 |

## Findings（WARN / FAIL）

无 FAIL。无阻塞性 WARN。以下为非阻塞观察，记录供开发者注意（不构成回退）：

- **OBS-1（非阻塞，给 Developer）**：`PurgeExpired` 持 `r.mu` 期间会调 `KVListByPrefix` + 逐条 `KVDelete`（各取 `s.mu`）。loginfail 行数极小（NFR-2）锁持有时间可忽略；但开发者应保持锁顺序为"先 `r.mu` 后 `s.mu`"（与既有 `RecordFailure`→`write`→`KVSet` 一致），勿在持 `r.mu` 时回调任何会反向取 `r.mu` 的路径（当前设计无此风险，KVListByPrefix/KVDelete 不碰 RateLimiter）。
- **OBS-2（非阻塞，给 Developer）**：§9 已点明 `session_purge_test.go` 对 `purgeSessionsLoop` 的两处引用需同步改 `purgeLoop` 并传 `rl`。这是改名连带的测试更新（非新增/非删除用例），属红线 3 允许的机械同步，不需 PM 额外批准。开发者改完务必确认既有两个 session 测试仍绿。
- **OBS-3（非阻塞，给 Developer）**：`KVListByPrefix` 的 `escapeLike` 转义对当前唯一调用点 `loginfail.`（不含 `\%_`）行为等价于不转义；务必加一条 storage 单测覆盖"含 `%`/`_` 的近似键不被误当通配"（如造键 `loginfail._x` 与 `loginfailX`，断言前缀 `loginfail.` 不命中 `loginfailX`），坐实 R-3/BC-6。

## 开发期高概率问题（预答）

1. **Q：`PurgeExpired` 内单条 `KVDelete` 失败怎么办——中止还是继续？**
   A：§5 已定 best-effort——累计已删数，记录首个错误，继续删剩余，最后返回 `(purged, firstErr)`。`purgeExpiredLoginFailsOnce` 对 err 只 Warn 不致命（对齐 session 范式）。下一周期会重试。
2. **Q：损坏 JSON 的 loginfail 行删不删？**
   A：删（§8 R-2）。理由：`Allow` 的 `read()` 对解析失败返 `ok=false` → 该 IP 本就被放行，故删损坏行对限流零影响，纯垃圾回收。Adversarial 应有一条覆盖。
3. **Q：`kvStore` 接口加方法返回 `[]storage.KVEntry`，auth 要 import storage 吗？会不会循环依赖？**
   A：要 import；无环（已核验 storage 不 import auth，仅 admin.go 注释提及）。采用设计 §3.2 候选 (i)，勿在 auth 包内复造等价类型（候选 ii 被否）。
4. **Q：清理间隔要不要新加一个 var？**
   A：不要（OOS-3）。复用 `sessionPurgeInterval`（1h），同一 `purgeLoop` 触发两清理。可更新该 var 注释为"后台清理周期"，不重命名（减小 diff）。
5. **Q：要不要在 storage 层用一条 `DELETE ... WHERE key LIKE ... AND <过期>` 一步到位？**
   A：不要。过期判定在 JSON 值内（SQL 判时间不便，且窗口语义属 ratelimit），坚持选项 B：storage 只列举，过期判定回 ratelimit（过期语义单点一致，与 `Allow` 同源）。

## 裁决（Verdict）

**APPROVED FOR DEVELOPMENT**

> full 模式下等价于 `APPROVED`：8 维全 PASS，无 FAIL、无阻塞 WARN，设计代码声明全部经独立核验属实，正确性核心（R-1 不误删活计数）有集合包含证明。OBS-1/2/3 为给开发者的非阻塞注意事项，不回退。
>
> 派发顺序：dev-db（storage KVEntry+KVListByPrefix+单测）→ dev-backend（ratelimit.PurgeExpired + auth_test + main wiring + wiring 测试 + baseline + dev-map）。严格串行（dev-backend 编译依赖 dev-db 的类型/方法）。
