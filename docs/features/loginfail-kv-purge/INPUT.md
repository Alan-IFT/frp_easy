# INPUT — T-063 · loginfail-kv-purge

**Mode**: full（7-stage 流水线）
**批次**: ux-ui-uplift-2026-05（第 2 个任务）
**输出语言**: 中文（红线）

## 一句话目标

关闭 `loginfail.<ip>` 限流计数 KV 行的永久滞留——T-046 加了 sessions 表的后台定时清理，但**没有对称清理 loginfail KV**，留下同一类资源泄漏的另一半。

## 证据 / 现状

- `internal/auth/ratelimit.go:74-80`：过期窗口**仅惰性清理**（同一 IP 再次访问时 `Allow` 内才 `KVDelete`）。永不复访的 IP（NAT 后轮换源 IP 的扫描器、攻击者）每个留下一条 `loginfail.<ip>` KV 行，永久滞留、无界增长。
- `cmd/frp-easy/main.go:544-558 purgeSessionsLoop` + `527-542 purgeExpiredSessionsOnce`：T-046 加的后台 loop **只清 sessions 表**，未清 loginfail KV。调用点在 `main.go:335 go purgeSessionsLoop(rootCtx, store, logger)`。
- `internal/storage/kv.go`：KV 层只有 `KVGet/KVSet/KVDelete`，**无前缀枚举/批删能力**——这是当初未实现 purge 的直接原因。
- 书面 backlog 证据：`docs/features/_archived/web-ui-mvp/04_DEVELOPMENT_backend.md:~69` 明确记"RateLimiter 的 kv 持久化键是 loginfail.<ip> 形式，storage 无前缀枚举 API，定期 purge 现状未实现，将来应补"。

## 范围与设计方向（供 RA 理解，技术选型由 SA 定）

1. **storage 层**（dev-db 分区，所有 SQL 留此层）：加按前缀清理过期 KV 的能力。
2. **wiring**（dev-backend 分区）：挂到**既有 `purgeSessionsLoop` 同一 ticker**，不新增 goroutine。
3. 过期判定语义必须与 `ratelimit.go` 的窗口（`failWindow=60s` 常量 / `failRecord.FirstAt` 字段）一致，避免清掉仍在窗口内的活计数导致限流失效。

## 硬约束 / 红线

- **所有 SQL 只在 `internal/storage/`**；ratelimit/main 通过函数调用，不写裸 SQL。
- 不破坏现有限流行为（5 次/60s per IP）：清理只能删**已过期**条目，绝不能删窗口内的活计数。
- 单测用 `t.TempDir()` 隔离 DataDir，不跨用例共享 db；同步点禁固定 `time.Sleep`，用 poll-until-condition + deadline。
- **新增测试必须同步 bump `scripts/baseline.json`** 的 `go_tests` / `test_count`（+version）。
- 更新 `docs/dev-map.md`（storage 对外 API 表 + auth ratelimit 行 + main.go purge loop 行）。

## 历史范式（PM 预扫）

- T-046 session-purge-and-requestid（`docs/features/session-purge-and-requestid/`）：清理 loop + storage 周期清理返回 count 范式，**直接复用对象**。
- T-053 autorestore-canceled-persist-fix：detached context 教训（insight L23），但本任务是周期清理非取消收尾。

## 产出要求

- RA 写 `docs/features/loginfail-kv-purge/01_REQUIREMENT_ANALYSIS.md`，全中文，full 模式 9 段。
- 先扫 `docs/tasks.md` 找 T-046 / web-ui-mvp backlog / ratelimit 历史，复用其范式，不重新设计。
- 重点验收点：(a) 过期 loginfail 行被清；(b) 窗口内活计数绝不被清（限流不失效）；(c) 前缀只匹配 `loginfail.` 不误删其它 KV（mode.* / frps 配置 / frpc admin creds）；(d) 不新增 goroutine（复用 purgeSessionsLoop ticker）；(e) ctx 取消正常退出。
