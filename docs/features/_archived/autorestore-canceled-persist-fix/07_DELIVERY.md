# 07_DELIVERY — T-053 autorestore-canceled-persist-fix

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05

## 需求

修复 T-050 加测试时发现的真实 bug：`cmd/frp-easy/main.go::retryRestoreLoop` 的 `canceled` 分支在 `case <-ctx.Done():` 内用**已取消的 ctx** 调 `persistAutoRestoreLast` → 其内部 `context.WithTimeout(已取消ctx, 5s)` 一出生即失效 → `KVSet` 报 `context.Canceled` 必失败 → `Outcome="canceled"` 永远写不进 kv，UI 的 `GET /api/v1/system/service-status` 看不到这条 last_run。

## 改动

- `cmd/frp-easy/main.go`：canceled 分支改用 detached `context.Background()` 调 `persistAutoRestoreLast`（该函数自带 5s 超时兜底），让这条 best-effort 最终写不被父 ctx 取消连累。仅此一处，业务逻辑其余不变。
- `cmd/frp-easy/autorestore_test.go`：把 `TestAutoRestore_CanceledMidway` 从"只断言不写 exhausted/ok"（规避 buggy 路径）升级为**正向断言** canceled outcome 必落 kv（`arPollLastRun(..., "canceled")`），并保留"绝不升级成 exhausted/ok"的短路守门。用 10s backoff 确保 select 里 `ctx.Done()` 先于 `time.After` 命中。

## 验证

- `go build ./...` 净；`go test ./cmd/frp-easy/... -run AutoRestore -v`：5/5 PASS（含升级后的 CanceledMidway）。
- `bash scripts/verify_all.sh`（完整含 e2e）：**PASS 32 / WARN 0 / FAIL 0**。测试数不变（修改既有测试，未增删），baseline 无需调整。

## Adversarial tests

- `TestAutoRestore_CanceledMidway` 是修复的反向证伪：修复前该正向断言（canceled 必落 kv）会因 KVSet 被 canceled ctx 拒绝而在 2s deadline 超时 FAIL；修复后稳定 PASS。

## Insight

- 错误/取消路径上的"最终状态持久化"必须用 **detached context**（`context.Background()` + 自带超时），不能复用触发该路径的已取消 ctx —— 否则这条"我被取消了"的记录本身会被取消连累，永远写不出去。这是 ctx 取消语义的常见陷阱：取消应停止"进行中的工作"，但不应阻止"记录我已停止"这一收尾动作。
- 真机场景下该 canceled 写仍与进程 shutdown 的 store.Close 存在竞态（best-effort）；对"上次自恢复结果"这类观测字段可接受，若要强保证需在 run() 用 waitgroup 等 retry goroutine 收尾后再 Close（本任务未做，超出范围）。
