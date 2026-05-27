# 07 — Delivery Summary · T-041 server-monitor-page-ui

> Stage 7 / 7。PM 收尾：传达任务结果 + 留 Insight。

- **Task**: T-041 · server-monitor-page-ui — frps 服务端运行态监控页（消费 T-039 API）
- **Mode**: full（1 → 2 → 3 → 4 → 5 → 6 → 7）
- **Batch**: `frps-monitor-and-mgmt-suite` 第 3/4
- **Depends on**: T-039 (DELIVERED commit ecc49b9，提供 `/api/v1/server/runtime/*` 三条 REST)
- **Date**: 2026-05-28

## Stages traversed

| Stage | Agent | 时间 | 产出 | 结论 |
|---|---|---|---|---|
| 1 | requirement-analyst | 2026-05-27 | 01_REQUIREMENT_ANALYSIS.md（FR×8 / NFR×7 / AC×15 / BC×9 / 决策×10） | ✓ |
| 2 | solution-architect | 2026-05-27 | 02_SOLUTION_DESIGN.md（文件清单 + 详细代码 + 测试计划） | ✓ |
| 3 | gate-reviewer | 2026-05-27 | 03_GATE_REVIEW.md verdict **APPROVED**（含 6 conditions C-1..C-6） | ✓ |
| 4 | developer | 2026-05-28 | 04_DEVELOPMENT.md（GR conditions 全消化；must-fix C-5 status toLowerCase） | ✓ |
| 5 | code-reviewer | 2026-05-28 | 05_CODE_REVIEW.md verdict **APPROVED**（无 must-fix；S-1/S-2/S-3 全建议性） | ✓ |
| 6 | qa-tester | 2026-05-28 | 06_TEST_REPORT.md verdict **PASS**（adversarial × 4 场景 / 6 用例反向证伪） | ✓ |
| 7 | (PM) | 2026-05-28 | 本文 | — |

## Rollbacks

无。Happy path 一次过。

## Files changed

| 类别 | 文件 |
|---|---|
| 新增（生产） | web/src/api/serverRuntime.ts |
| 新增（生产） | web/src/composables/useServerRuntime.ts |
| 新增（生产） | web/src/pages/ServerMonitor.vue |
| 新增（测试） | web/src/composables/\_\_tests\_\_/useServerRuntime.spec.ts |
| 新增（测试） | web/src/pages/\_\_tests\_\_/ServerMonitor.spec.ts |
| 新增（测试） | web/src/pages/\_\_tests\_\_/qa_t041_adversarial.spec.ts |
| 修改 | web/src/types.ts（+4 接口 / 50 行） |
| 修改 | web/src/router.ts（+1 路由） |
| 修改 | web/src/components/AppLayout.vue（+1 menu 项 + activeKey 分支） |
| 修改 | docs/dev-map.md（+5 处） |
| 修改 | docs/tasks.md（+1 行已完成段） |
| 新增（文档） | docs/features/server-monitor-page-ui/{01-07}.md + PM_LOG.md |

净 LOC：生产 ~555 / 测试 ~620 / 文档 ~2400。test:prod 比 ≈ 1.12。

## Baseline 变化（预期）

| 指标 | Before | After 预期 |
|---|---|---|
| 前端 vitest 用例 | 186 | 232（+46：13 + 27 + 6） |
| Go 测试 | 265 | 265（零后端改动） |
| 总数 | 451 | 497 |
| verify_all PASS | 31 | 32 |
| verify_all FAIL | 1（C.1 长期环境） | 1（C.1 不变） |

## 关键决策回顾

1. **D-1 不展示 uptime**：T-039 ServerInfo 不返回 uptime；硬合成不可靠 → 范围克制。
2. **D-2 n-tabs 按 type 分组**：避免 T-037 group row "虚拟列与 DB 字段同名"反模式（insight L26）。
3. **D-3 status 红色兜底 + GR C-5 toLowerCase 防御**：容忍上游 frps 版本演进 + 字面大小写漂移。
4. **D-5 composable 显式 start**：F-5.7 测试友好 + SSR 安全（不在 mount 自动 start）。
5. **D-6 3 次失败自动停**：与 T-036 useLogBuffer BC-6 范式对齐。
6. **D-7 5s 默认间隔**：日志 2s（流式）/ 服务态 10s（稀疏）中间档。
7. **BC-7 用户意图优先**：用户显式 stop 后 visibility 恢复**不**自动 resume；`userStoppedExplicitly` flag 严格守门。

## verify_all 状态

**DEFERRED HOOKS**: PM 派发上下文工具裁剪（insight L23 / L34）让本 stage 无法直接 spawn pwsh/bash 跑 `scripts/verify_all.ps1` + `scripts/archive-task.ps1` + `git commit`。

委托给 **batch orchestrator stop-hook** 或用户 session 末尾 stop-hook 触发：
1. `pwsh scripts/verify_all.ps1` — 预期 PASS=32 / FAIL=1（与 baseline 一致）
2. `git add -A` + `git commit -m "feat(T-041): server-monitor-page-ui — frps 运行态监控页（消费 T-039 API；5s 轮询 + visibilitychange 自动暂停 + 3 次失败自动停）"`
3. `pwsh scripts/archive-task.ps1 -Task server-monitor-page-ui` — 归档 07 `## Insight` 段到 `.harness/insight-index.md`

## Outstanding risks

| 风险 | 严重度 | 备注 |
|---|---|---|
| R-1: 用户未启用 dashboard 时进监控页看 503 | 低 | UX 引导文案 + "前往服务端配置"按钮已覆盖；ADV-2 反向证伪 |
| R-2: T-042 接手后 useServerRuntime 数据形态需扩展（按 name index 查 Proxies 行） | 低 | CR §7 S-3 已记 |
| R-3: 1s tickTimer 在用户暂停后仍 tick | 极低 | 开销 1 个 ref +1 操作；onUnmounted 清干净 |

## Next steps for user

1. 跑 `pwsh scripts/verify_all.ps1` 确认 PASS=32 / FAIL=1。
2. 单 commit：`feat(T-041): server-monitor-page-ui — ...`（不 push，按用户指示）。
3. 跑 `pwsh scripts/archive-task.ps1 -Task server-monitor-page-ui` 归档。
4. 进入 batch 第 4/4 任务 T-042 proxy-runtime-status-merge。

## Insight

> 仅记 hard-won 项目特有事实（≥10 分钟才能从 codebase 推出来的）；避免泛规范。

- 2026-05-28 · Vue composable 内调用 `onUnmounted` 强制要求"在 setup 同步路径 / 同 tick" —— `addEventListener(...) + onUnmounted(...)` 必须连写不能 await 后再注册，否则 unmount 时只清 listener、timer 泄漏。useServerRuntime.ts L160-167 把 listener 与 onUnmounted 紧邻放置是项目范本；spec 用 mount(Holder).unmount() 方式触发生命周期才能验证 timer / listener 同时清。· evidence: T-041 useServerRuntime.spec "onUnmounted 清理" + spy removeEventListener 命中
- 2026-05-28 · "用户显式暂停"+"系统自动暂停（visibility）" 两类 stop 路径必须用一个 flag 区分语义（如本任务 `userStoppedExplicitly`），否则 visibility 恢复时会"善意地"覆盖用户意图。BC-7 反向证伪："用户 stop → 切后台 → 切回不自动恢复" 用例必须独立存在才能锁死语义，否则未来 dev 重构 visibilitychange 路径时极易引入回归。· evidence: T-041 useServerRuntime.ts L98-110 stopInternal(setUserFlag) + spec BC-7 用例
- 2026-05-28 · `naive-ui` 的 `n-tabs` activeKey 用 `Object.keys(data)` 顺序绑定时会因 polling 后端返回顺序漂移导致 tab 闪烁（用户体验灾难）。正确路径是**前端 hardcode 显示顺序列表**（如 `allKnownTypes = ['tcp','udp','http','https','stcp','sudp','xtcp']` const 数组）+ `Set` 收集"有数据 / 有 errors"的 type → `allKnownTypes.filter(t => has.has(t))`。这与 T-018 端口预设清单"前端 hardcode 主导显示"是同类范式（信息架构稳定性 > 数据驱动）。· evidence: T-041 ServerMonitor.vue allProxyTypes computed
- 2026-05-28 · 三态 UI（loading / empty / error）容易踩"loading 与 error 互斥但代码上没显式互斥"的陷阱：本任务用三个 computed `firstLoading` / `firstLoadFailed` / `showStaleBanner` 通过判断 `info === null && proxies === null` 自然形成三态切换（loading: 全 null + 无 error；first-fail: 全 null + 有 error；stale: 至少一非 null + 有 error）。任何一个 condition 写错会让 UI 出现"loading 转圈 + 红色错误同时显示"的尴尬状态。设计阶段就把三态写成布尔代数式（互斥矩阵）能避免 dev 阶段返工。· evidence: T-041 ServerMonitor.vue firstLoading / firstLoadFailed / showStaleBanner 三个 computed
- 2026-05-28 · `## Adversarial tests` 标题与 `## Insight` 同款规则：**禁带任何前缀**（数字编号 `## 3. Adversarial tests`、`§N` 前缀、章节标号均不行）。verify_all E.6 regex `^##\s+Adversarial\s+tests\s*$` 严格行首裸标题锚定，本任务 06_TEST_REPORT.md 初版写 `## 3. Adversarial tests` 让 E.6 由 PASS 转 FAIL（FAIL 数 1→2 触发 batch strong-signal stop）；改回裸 `## Adversarial tests` 后 PASS=31 / FAIL=1 回 baseline。这是 insight L43/L48/L49 系列"`## Insight` 禁数字前缀"的姐妹陷阱 —— 任何 verify_all E 段静态闸门锚定的 H2 标题都禁带前缀，应在 PM 写 06 模板时硬约束。· evidence: T-041 06_TEST_REPORT.md L48 字面 `## 3. Adversarial tests` 触发 verify_all E.6 FAIL + 一行 Edit 修复为裸标题后 PASS 回归
