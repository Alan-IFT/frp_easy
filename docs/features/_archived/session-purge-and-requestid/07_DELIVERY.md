# 07_DELIVERY — T-046 session-purge-and-requestid

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05

## 需求

两个后端正确性 / 可维护性小修：
- **F-1**：过期 session 永不清理 → sessions 表无界增长（`GetSession` 刻意不删过期行以避免每次读引入写；`PurgeExpiredSessions` 有实现但启动序列从未拉起任何清理任务）。
- **F-11**：`RequestID` 中间件用 `time.Now().UnixNano()` 生成 reqID → 同一纳秒 / 低精度时钟下碰撞，使日志关联失真。

## 改动

- **F-1**：`cmd/frp-easy/main.go` 新增 `purgeSessionsLoop(ctx, store, logger)`（启动立即清一次 + 每 `sessionPurgeInterval`=1h 一次，随 `rootCtx` 取消，无 goroutine 泄漏）+ `purgeExpiredSessionsOnce`（5s 超时、错误仅告警不致命）。在 `run()` 的 `rootCtx` 创建后 `go purgeSessionsLoop(...)`。`sessionPurgeInterval` 设为包级 `var` 便于测试。
- **F-11**：`internal/httpapi/middleware.go` 的 `randomID()` 改用 `crypto/rand` 8 字节 → 16 hex 字符；crypto/rand 极少失败，万一失败退回纳秒时间戳（单机日志关联仍够用）。

## 验证

- 新增 3 个 Go 测试（go_tests 284→287）：
  - `TestRandomID_UniqueAndHex`：10000 次紧循环断言无碰撞 + hex 格式（旧纳秒实现会在此失败 —— F-11 反向证伪）。
  - `TestPurgeExpiredSessionsOnce`：过期 session 被清、未过期 session 存活（不 over-delete）。
  - `TestPurgeSessionsLoop_ExitsOnCancel`：loop 在 ctx 取消后 2s 内返回（无 goroutine 泄漏）。
- `bash scripts/verify_all.sh`（完整含 e2e）：**PASS 32 / WARN 0 / FAIL 0**。baseline.json v19 同步（go_tests 287 / test_count 584）。

## Adversarial tests

- F-11 反向证伪：`TestRandomID_UniqueAndHex` 的 10000 次紧循环是专门针对旧实现的失败构造 —— `time.Now().UnixNano()` 在紧循环里多次落在同一纳秒必碰撞，crypto/rand 不会。
- F-1 边界：`TestPurgeExpiredSessionsOnce` 同时放一条 1ns ttl（必过期）和一条 1h ttl（存活），断言 purge 不 over-delete 活 session；`TestPurgeSessionsLoop_ExitsOnCancel` 用 done channel + 2s deadline 确定性验证 ctx 取消退出（不靠 sleep，避免脆弱）。

## Insight

- "读时不删过期行、靠后台周期清理"是 session 存储的标准范式，但**周期清理任务必须真的被启动序列拉起**，否则 GetSession 的"不删"优化会让表无界增长。清理 loop 必须随根 ctx 取消（SIGTERM/stopCh）以免 goroutine 泄漏，并把间隔设为包级 var 便于测试注入短间隔 / 长间隔。
- 请求关联 ID 必须用 crypto/rand 而非时间戳：reqID 的唯一价值是日志关联，时间戳在并发下碰撞。项目已有 `auth.GenerateCSRFToken`/`randToken` 的 crypto/rand 范式，middleware 直接用 `crypto/rand`+`hex` 即可，无需引入 auth 依赖。
