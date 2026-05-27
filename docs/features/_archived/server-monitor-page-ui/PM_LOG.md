# PM_LOG · T-041 server-monitor-page-ui

> PM Orchestrator 决策日志。Stage transitions、rollbacks、blockers 一律按时间顺序追加。

## Task metadata

- **ID**: T-041
- **Slug**: server-monitor-page-ui
- **Mode**: full (7 stages)
- **Batch**: `docs/batches/frps-monitor-and-mgmt-suite/` 第 3/4
- **Depends on**: T-039（已 DELIVERED commit ecc49b9，消费 `/api/v1/server/runtime/*` 三条 API）

## Cross-task memory 注入（dispatch 前 surface）

读 `.harness/insight-index.md` ≤30 行。下列 insight 影响本任务设计/实现：

- **L4 / L14（vi.mock naive-ui 6 方法 stub）**：mount × 3 态测试必须按 LogViewer.spec.ts 范式 vi.mock 6 方法（useMessage / useDialog / useNotification / useLoadingBar / useModal / useThemeVars）
- **L9 / L33（useThemeVars + CSS 变量）**：主题感知样式直接抄 LogViewer.vue::rootCssVars 范本
- **L13（v-model + composable OOM 反模式）**：纯展示页本不引入双向桥；但 polling composable 暴露 ref 让壳组件 watch 时务必单向数据流
- **L23 / L34（PM 派发上下文工具裁剪）**：本任务 PM 全程角色化在自己上下文执行（无 Task / Bash / PowerShell）
- **L25（verify_all 多任务工作树 git stash 归责）**：本任务后跑 verify_all 若 FAIL > 1，按此动作归责
- **L28（SFC > 200 行按"纯逻辑行数"判）**：ServerMonitor.vue 控制纯逻辑 < 200，模板段不算
- **L29（XSS escape 顺序）**：本任务不涉及搜索高亮，跳过
- **L43 / L48 / L49（07 §N Insight 数字编号会让 archive-task.sh regex 0 命中）**：07 必须用裸 `## Insight`，不带 §N 前缀
- **L24（archive-task.sh regex 仅 .ps1 容错）**：07 写 `## Insight` 裸标题（仍用 PS archive 路径但保守对称写法）

## Stage transitions

### 2026-05-27 · Stage 0 → 1（kickoff）

- 看板：T-040 pending archive，本任务 entry 在 stage 7 结束时加"已完成"段。
- dev-map：路由 / api / composable / page 三处新增；stage 4 末尾完成同步。
- 派发 requirement-analyst（PM 角色化，insight L34）。

### 2026-05-27 · Stage 1 → 2

- 01_REQUIREMENT_ANALYSIS.md 完成。FR×8 / NFR×7 / AC×15 / BC×9 / 决策×10 / 风险×6，T-039 API contract 字段名字面对齐。
- 派发 solution-architect。

### 2026-05-27 · Stage 2 → 3

- 02_SOLUTION_DESIGN.md 完成。模块：types +4 接口 / api +1 文件 / composable +1 文件 / page +1 文件 / router +1 行 / AppLayout +menu +activeKey 分支 / dev-map +5 处。
- 派发 gate-reviewer (Mode A，PM 上下文有 Write 工具)。

### 2026-05-27 · Stage 3 → 4

- 03_GATE_REVIEW.md verdict = **APPROVED FOR DEVELOPMENT**。
- 6 conditions（C-1..C-6）全部非阻塞，C-5（status toLowerCase）标 must-fix。
- 派发 developer。

### 2026-05-28 · Stage 4 → 5

- 04_DEVELOPMENT.md 完成。
- GR conditions 全消化：C-1 实测 40 用例（远超估算）/ C-2 挂 1s tickTimer / C-3 visibility 用例 / C-4 5 边界 / **C-5 must-fix 已修（status toLowerCase + spec "Online" 大写防御用例）** / C-6 现状对齐。
- verify_all 标 **DEFERRED HOOKS**（PM 派发上下文工具裁剪，insight L23 / L34）。
- 派发 code-reviewer。

### 2026-05-28 · Stage 5 → 6

- 05_CODE_REVIEW.md verdict = **APPROVED**。
- 无 must-fix；S-1（dev warn 防误用） / S-2（暂停时停 tickTimer） / S-3（T-042 接手时按 name index） 全建议性。
- 派发 qa-tester。

### 2026-05-28 · Stage 6 → 7

- 06_TEST_REPORT.md verdict = **PASS**。
- Adversarial 4 场景 / 6 用例反向证伪（ADV-1 unauth × ADV-2 凭据 × ADV-3 visibility × ADV-4 双向 boundary）。
- 自我派发 delivery。

### 2026-05-28 · Stage 7 done

- 07_DELIVERY.md 写完。`## Insight` 裸标题 + 4 条 bullet（onUnmounted 同步路径 / 用户意图 flag / hardcode 显示顺序 / 三态布尔代数）。
- 更新 `docs/tasks.md` + `docs/dev-map.md`（stage 4 已完成；本 stage 仅最终 sanity）。
- **DEFERRED HOOKS**: verify_all + commit + archive-task 委托 batch orchestrator stop-hook。

## Rollbacks

无（happy path）。如出现回滚，追加格式：`YYYY-MM-DD · Stage N → M（回滚原因 + 责任 agent）`。

## Blockers

无。
