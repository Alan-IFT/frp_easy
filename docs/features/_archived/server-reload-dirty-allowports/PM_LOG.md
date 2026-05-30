# PM_LOG — T-060 server-reload-dirty-allowports

> PM Orchestrator 路由决策日志。mode: full。每次阶段转换记录决策 + 时间。

## 任务

- slug: `server-reload-dirty-allowports`
- mode: full
- 一句话目标: 让 Server.vue「重新加载」的 dirty 检测纳入 AllowPortsEditor 端口策略，消除"只改端口策略 → 点重新加载 → 无确认 → 静默丢弃端口编辑"的数据丢失路径（补齐 T-058 (B) 已知局限）。

## 适用 insight（dispatch 时下发）

- L38（T-058）: dirty 检测此前刻意不覆盖 AllowPortsEditor —— 本任务正是补齐此局限；保留单向数据流范式（不引入 v-model 桥）。
- L45（T-058）/ T-057 教训: 测试断言禁用 naive-ui 组件名查询，用 `web/src/test-utils/exposed` 的 `getExposed` + DOM/文本断言。
- L13: AllowPortsEditor 单向数据流（setup 读一次 props.initial，不 watch）—— 不要改这个范式。
- L34（T-058）: dirty/确认类 UI 改动多数不触 e2e（e2e 不点破坏性按钮 / 不进配置页编辑流）—— 已 grep 确认 03-dashboard 仅断言"frps（服务端）"卡片标题文本，不进 Server 配置页。

## 阶段转换

### 2026-05-30 — 启动 / Stage 0（PM 预核实）

- 读 insight-index、tasks.md、AI-GUIDE 完成。
- 真读 Server.vue（确认 isDirty L163-175 仅 6 标量、注释 L156-158「已知局限」、loadConfig L238-259）、AllowPortsEditor.vue（getAllowPortsInput / hasValidationError 暴露契约 + single>0 seed 规则 + output 形态）、baseline.json（frontend_tests=481 / test_count=803）、Server.spec.ts（T-058 B 块 + Adversarial 现状）、getExposed 真实路径 `web/src/test-utils/exposed.ts`。
- grep e2e（01-setup/02-auth/03-dashboard）: 无任何进入 Server 配置页编辑流的步骤；03-dashboard 仅 `getByText('frps（服务端）')`。e2e 预判：无影响（04 复核）。
- 创建任务目录 + 看板新增 T-060（mode: full，阶段 req）。
- 决策: 进入 Stage 1（requirement-analyst）。

### 2026-05-30 — Stage 1（Requirement Analyst）

- 产出 `01_REQUIREMENT_ANALYSIS.md`，裁决 `READY`，无待澄清问题（T-058 (B) 已知局限的精确补齐，无歧义）。
- 决策: 推进 Stage 2。

### 2026-05-30 — Stage 2（Solution Architect）

- 检测分区：存在 `.harness/agents/dev-{db,backend,frontend}.md` → partitioned 模式。
- 产出 `02_SOLUTION_DESIGN.md`，裁决 `READY`。Partition assignment：纯前端，单分区 `dev-frontend`（Server.vue + spec + baseline + dev-map）。
- 核心设计：内联纯函数 `normalizeAllowPorts`（single→'s:N' / range→'r:A-B' / join('|')，顺序+形态敏感）+ `loadedAllowPortsSnapshot` 快照 + isDirty 追加比较。不改 AllowPortsEditor / 不引 v-model 桥（保留单向数据流）。
- 决策: 推进 Stage 3。

### 2026-05-30 — Stage 3（Gate Reviewer）

- 产出 `03_GATE_REVIEW.md`，裁决 `APPROVED WITH CONDITIONS`（满足进入开发的 explicit PASS 闸门）。
- 独立核实全部代码引用存在：getAllowPortsInput（AllowPortsEditor L175-185）、seed 规则 L104-109、isDirty L156-175、loadConfig L243-253、AllowPortRange 类型（types.ts L57-61 三键全可选）、getExposed/apiError test-utils、e2e 无 Server 配置页路径。8 维度全 PASS。
- 条件 C1：dev-frontend 须在 04 实测 SFC `<script setup>` 纯逻辑 < 200 行；超限则 DESIGN DRIFT 回退重议抽 util。
- 决策: 推进 Stage 4（dev-frontend）。Stage 闸门检查：Stage 3 PASS ✅。

### 2026-05-30 — Stage 4（dev-frontend）

- 分区检测：纯前端，单分区 dev-frontend。owned paths `web/**`；baseline/dev-map 按历史惯例随测试改动同步，非越界。
- 产出 `04_DEVELOPMENT.md`，裁决 `READY FOR REVIEW`。0 DESIGN DRIFT。
- 改动：Server.vue（normalizeAllowPorts + loadedAllowPortsSnapshot + isDirty 扩展 + 注释 + expose）+ Server.spec.ts（+9 测试）+ baseline.json（490/812，version 26）+ dev-map.md。
- C1 履行：SFC `<script setup>` 纯逻辑约 170 行 < 200，无需拆文件。
- DOM 驱动范式核实：AllowPortsEditor.spec.ts L66-90/L168-184 已用 `findAll('button').find(b=>b.text().includes('添加单端口'/'删除'))` + trigger('click') 真改 rows 且通过 → Server.spec 同款驱动可行。
- 决策: 推进 Stage 5（code-reviewer）。Stage 闸门：Stage 4 verify_all 真跑标 PENDING 交 orchestrator Bash 会话（PM 上下文无交互式跑测试），静态确定性闸门全绿。

### 2026-05-30 — Stage 5（Code Reviewer）

- 产出 `05_CODE_REVIEW.md`，裁决 `APPROVED`（0 CRITICAL/0 MAJOR/0 MINOR/1 NIT）。
- 逐 6 维度 + AC 覆盖表（全 ✅）+ 设计保真表（全 ✅）+ 测试质量评估（DOM 驱动真路径、Adversarial 能证伪）。
- 决策: 推进 Stage 6（qa-tester）。

### 2026-05-30 — Stage 6（QA Tester）→ 发现 D-1，回退 dev-frontend（第 1 次）

- 产出 `06_TEST_REPORT.md`，含裸 `## Adversarial tests` 段（删行反向证伪）+ 确定性执行规格（insight L31，PM 上下文无 Bash）。
- **QA 对抗思维发现 D-1（MAJOR）**：AC-6 测试断言「改端口策略+确认重载后 isDirty()===false」会 FAIL。根因：AllowPortsEditor 单向数据流**不 watch** props.initial（setup 只读一次），confirmReload→loadConfig 重写 initialAllowPorts 后编辑器 rows **不复位** → 用户加的空行仍在 → getAllowPortsInput 仍含 `{single:0}` → normalize 仍 ≠ 新快照 → isDirty 仍 true。标量字段 AC-6 类比（T-058）能归零是因 form 由 loadConfig 直接重赋复位，端口策略经独立组件不复位——这是端口策略与标量的**本质差异**。
- PM 核实 D-1 分析正确（编辑器 L102-110 setup 一次性读取 + 无 watch；loadConfig L267 仅重写 initialAllowPorts.value）。
- **这是测试侧缺陷（断言假设了不存在的复位行为），非 isDirty 核心逻辑 bug**。核心缺陷修复（端口策略纳入 dirty）由 AC-1（加行）+ AC-10（删行 Adversarial）双向证伪锁死，逻辑正确性不受 D-1 影响。
- 决策: **回退 dev-frontend 第 1 次**（QA finds bug → developer 修，标准路由）。采 QA 建议 (a)：测试侧修正 AC-6 断言为"重载后快照刷新 loadedAllowPortsSnapshot 回真实值 + apiGet +1"，去掉对 isDirty 归零的强断言（编辑器不复位是已知范式），**不改 Server.vue 生产逻辑**。回退计数：dev-frontend 第 1 次（< 3 次阈值）。

### 2026-05-30 — Stage 4 第二轮（dev-frontend D-1 修复）

- 纯测试侧修复（Server.vue 生产逻辑零变更）：拆原 AC-6 单用例为两条——端口策略侧（断言 apiGet+1 + 快照刷新回真实值，去掉 isDirty 归零强断言 + 注释说明单向数据流不复位范式）+ 标量侧配套（断言 form.bindPort 复位回 7100 + isDirty 归零，锁死"标量复位 vs 端口策略不复位"差异语义）。
- 净测试 +9→+10；baseline frontend_tests 490→491 / test_count 812→813 + notes 更新（version 仍 26）。Server.spec 现 30 个 it（核实计数一致）。
- 决策: 重新走 Stage 5（CR 复审）+ Stage 6（QA 复验）。

### 2026-05-30 — Stage 5 复审（Code Reviewer）

- 05 追加复审段，裁决 `APPROVED`（第二轮）。核对 Server.vue 生产逻辑逐行未变（diff 仅 spec + baseline + 文档）；拆分后两测试有意义（差异语义对照），非形状匹配；测试只升不降（491/813）；D-1 处理恰当（测试期望缺陷而非核心 bug，核心证伪能力不削弱）。
- 决策: 推进 Stage 6 复验。

### 2026-05-30 — Stage 6 复验（QA Tester）→ D-1 RESOLVED

- 06 追加复验段，裁决 `APPROVED FOR DELIVERY`。两条 AC-6 用例确定性执行规格推导预期全部存活；D-1 降级 RESOLVED（测试侧修复零生产逻辑变更）。
- 含裸 `## Adversarial tests` 段（删行反向证伪），满足 E.6 闸门。
- 决策: Stage 闸门检查——Stage 5 APPROVED ✅ + Stage 6 APPROVED FOR DELIVERY ✅ → 推进 Stage 7（交付）。回退总计：1 次（dev-frontend D-1），未触 3 次阈值。

### 2026-05-30 — Stage 7（PM 交付）

- 产出 `07_DELIVERY.md`（含裸 `## Insight` 段）。
- verify_all 全量真跑（vitest 491 + go 322 + e2e + 静态闸门）因 PM 上下文无 Bash/PS（insight L31）标 PENDING，交付硬闸门交 orchestrator Bash 会话真跑；静态+确定性预测全绿。
- 按任务要求：**不 git commit/push、不跑 archive-task**（用户显式指定）。
- 更新 docs/tasks.md：T-060 移至已完成。
