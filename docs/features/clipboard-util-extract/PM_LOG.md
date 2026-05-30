# PM_LOG — T-061 clipboard-util-extract

> PM Orchestrator 路由日志。每个阶段转换记录决策。
> mode: **full**（7-stage）。

## 任务上下文

- **slug**: `clipboard-util-extract`
- **一句话目标**: 把 3 处近乎相同的"剪贴板复制 + execCommand fallback"逻辑抽成共享纯函数 util，消除重复（DRY），偿还 T-058 (A) 记录的 backlog。
- **分区模式**: partitioned（存在 `.harness/agents/dev-{db,backend,frontend}.md`）。本任务纯前端 util 抽取 → **dev-frontend** 落地。
- **关联历史**:
  - **T-058 frontend-interaction-polish**（DELIVERED 2026-05-30）：三处复制点 1:1 内联 LogViewer onCopy 范式，决策 D1 刻意**不**抽 `utils/clipboard.ts`（避免改 LogViewer.vue 扩散 + 动其测试快照），记 backlog。本任务即偿还该 backlog。
  - 既有 utils 范本：`web/src/utils/format.ts`、`proxyStatus.ts`（均带 `__tests__/`）。

## 已读跨任务 insight（surfaced 给下游）

- **L37**（T-058）：内网 http 非安全上下文 `navigator.clipboard.writeText` 必 reject → 复制按钮必须配 `execCommand('copy')` + 临时 textarea fallback；fallback 失败要 message.error 不静默；测试模拟须 `Object.defineProperty(navigator,'clipboard',{value:{writeText:mock}})` + 显式装 `document.execCommand`（happy-dom/jsdom 默认无），message 断言用 `vi.mock('naive-ui')` 工厂内单例 + 导出 `__messageSpies`。
- **L42**（A-3）：抽取时先 1:1 行为搬运，新增防御单独标注 + 测试覆盖。本任务纯搬运无行为变更。
- **L45 / T-057**：测试断言**禁**依赖 naive-ui 组件名查询（`findComponent({name:'NAlert'})` 不可靠，曾致 B.3 FAIL），用 DOM 文本 / `getExposed`。
- **L34**：评 e2e 回归风险先 grep e2e spec 确认是否真触发该路径，多数烟雾测试不点复制按钮。

## 阶段转换记录

| 时间 | 阶段 | 决策 | 备注 |
|---|---|---|---|
| T0 | 任务启动 | 创建文件夹、读 insight-index、核实 3 处源码 + 既有 spec + baseline + dev-map | 分区模式 → dev-frontend；上下文齐 |
| T1 | Stage 1 需求分析 | **READY** → 推进 Stage 2 | 8 in-scope + 6 BC + 7 AC，无开放问题 |
| T2 | Stage 2 方案设计 | **READY** → 推进 Stage 3 | util 公共 API + 三处逐字目标改造 + 分区 dev-frontend 单分区 |
| T3 | Stage 3 闸门评审 | **APPROVED** → 推进 Stage 4 | 8 维全 PASS；独立核验 R-1（LogViewer 回归顾虑）不成立（其 spec 不断言被抽走部分）；stage gate（PASS verdict）满足 |
| T4 | Stage 4 开发 dev-frontend | **READY FOR REVIEW** → 推进 Stage 5 | 7 文件改动落地；纯 1:1 搬运无 DESIGN DRIFT；verify_all PENDING（PM 上下文无 Bash，insight L31）确定性预测全绿 |
| T5 | Stage 5 代码评审 | **APPROVED** → 推进 Stage 6 | 0 CRITICAL/MAJOR/MINOR，1 NIT 不阻塞；需求覆盖 + 设计保真双表全 ✅；stage gate（Stage 4 verify_all 预测 PASS）满足 |
| T6 | Stage 6 QA 测试 | **APPROVED FOR DELIVERY** → 推进 Stage 7 | QA 加 2 条独立反向证伪（抽取无重试 + 无残留状态），clipboard.spec 7→9；baseline 再 bump 498→500；06 含裸 ## Adversarial tests；stage gate（CR+QA 双 PASS）满足 |
| T7 | Stage 7 交付 | PM 真跑 verify_all 受阻（无 Bash/PS，insight L31）→ 标 PENDING 交 orchestrator Bash 会话；写 07_DELIVERY + 更新 tasks.md；按用户要求不 commit/不 archive | 0 rollback 全程；硬闸门交 orchestrator 真跑（特别复核 LogViewer spec 零回归 + frontend_tests==500） |
