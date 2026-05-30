# INPUT — T-065 mapprocerr-sentinel-hygiene（PM 提供给下游）

模式：**full**（7-stage）。批次 `ux-ui-uplift-2026-05` 第 4 个。输出语言：中文。

## 一句话目标

把 `internal/httpapi/handlers_proc.go:mapProcErr` 对 procmgr 内部错误文本的脆弱 `strings.Contains`
匹配收口为 procmgr sentinel（`procmgr.ErrBusy`），并让其 500 路径走 `writeInternalError` 不泄露内部文本——
完成与 T-059（mapProxyWriteError）/ T-055（writeInternalError）相同方向的对称收口。

## 证据 / 现状（PM 已读源码确认）

`internal/httpapi/handlers_proc.go:88-100` mapProcErr：
- binloc.ErrBinMissing → 422 CodeBinMissing（透传 err.Error()，OOS：不动，binloc sentinel 已是好范式）。
- `low := strings.ToLower(msg); if strings.Contains(low,"stopping"|"starting"|"running")` → 409 CodeProcBusy（透传 msg）。
  靠匹配 procmgr 内部 error 文本分类。procmgr 改一个字（"currently stopping"→"is stopping"）即静默漏判退化成 500——与 T-059 反模式完全同源。
- 末尾 `writeError(w, 500, CodeInternal, msg, "")`：把 procmgr 原始 error 文本（含 `procmgr.Start(frpc): ...` 内部细节）直接透传前端。
- 同文件 procStop（:42）已用 `h.writeInternalError`——mapProcErr 是唯一漏网一致性缺口。

procmgr 层"忙"语义返回点（PM 初查，需 RA/SA 复核全集）：
- `manager.go Start:171` StateStopping 分支 → `fmt.Errorf("procmgr.Start(%s): currently stopping", kind)` —— 唯一显式"忙"返回。
- `Start` 的 StateStarting/StateRunning：idempotent 早返回 ProcessInfo + nil（不报错）。
- `Stop`：idempotent，不返"忙"错误；`Restart`=Stop+Start，透传 Start 错误。
- mapProcErr 调用点：procStart:25 / procRestart:67。

## 范围与设计方向（用户给定）

1. procmgr 层（dev-backend）：定义 sentinel（如 `procmgr.ErrBusy`），让"进程处于过渡/活动态而拒绝操作"的分支返回包装了 ErrBusy 的错误
   （`fmt.Errorf("...: %w", ErrBusy)` 保留可读 cause 进日志，`errors.Is` 可判）。先通读 manager.go 找全所有"忙"语义返回点，覆盖现 handler 匹配的 stopping/starting/running 全集，不遗漏、不误纳。
2. handler 层（dev-backend）：mapProcErr 改 `errors.Is(err, procmgr.ErrBusy)` → 409 CodeProcBusy，面向前端固定中文文案（不透传内部英文）；其余非 sentinel 错误走 `h.writeInternalError`（500 固定中文 + 原始 error 进 logger）；删除 strings.Contains 整块。保留 uploadBin errno 透传（不在本任务范围，不动）。
3. 核对 CodeProcBusy 等错误码常量语义不变。

## 红线 / 关键纪律（insight L35）

- 开工前必须 grep 出所有断言旧行为的测试（断言 procmgr 英文文本透传、断言 500 含内部细节、断言 409 文案是旧英文的），纳入同分区一起改。
- 判别标准（必须全满足）：用例数不降 + 改的是断言内容（验证新意图）而非删用例 + PM 在 PM_LOG 显式批准该断言更新并记录理由。
- 不破坏 procmgr 生命周期行为（Start/Stop/Restart 状态机不变，只是错误返回类型从裸 fmt.Errorf 改为 wrap sentinel）。

## 硬约束

- 改动仅 `internal/procmgr/` + `internal/httpapi/`；不碰前端/storage/DB。
- 单测用 `t.TempDir()`；`-race` 本机跑不了（无 C 编译器），静态论证。procmgr 测试可能用编译独立 helper 程序（insight L9）。
- 新增/改动测试后同步 bump `scripts/baseline.json` 的 go_tests/test_count（+version）；净用例数不变也要确认 baseline go_tests ≤ 实际计数。
- 更新 `docs/dev-map.md`（procmgr sentinel 行 + handler 错误映射行）。

## 产出要求

- 7 份阶段文档 + PM_LOG.md，全中文。PM_LOG 必须含对断言更新的显式批准段。
- 06_TEST_REPORT.md 含裸标题 `## Adversarial tests` 段。
- 07_DELIVERY.md 含裸标题 `## Insight` 段。

## 交付与验证边界

- PM 上下文无 Bash/PS，标 verify_all=PENDING + 执行规格（预期 PASS、go_tests 新计数、列出所有改了断言的测试名 OLD→NEW 期望）。真跑由 batch orchestrator 执行作硬闸门。
- 不要 commit / push / archive。
- 当前 baseline：go_tests=333 / frontend_tests=552 / test_count=885 / version=30。
