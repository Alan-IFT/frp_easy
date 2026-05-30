# 01 需求分析 — T-058 frontend-interaction-polish

- **任务**: 三项前端交互一致性小修打包
- **模式**: full（7-stage）
- **作者角色**: requirement-analyst
- **日期**: 2026-05-30

## 1. 背景与关联历史

本任务是 batch `project-optimization-2026-05` 系列的延续，承接前端交互/诚实状态优化主线：

- **T-047 frontend-honest-states**：Server/Client 三态（loading/error/loaded）、Dashboard 开关不静默。本任务 (B) 在其三态结构上扩展"重置"语义。
- **T-048 frontend-consistency-cleanup**：PublicIpDetector 改用 `extractErrorMessage`。本任务 (A) 在 PublicIpDetector 复制路径上继续补失败反馈。
- **T-056 proc-stop-destructive-confirm**：Dashboard 破坏性操作复用 `ConfirmDialog` + `pendingAction` 状态机。本任务 (B) 复用同款 `ConfirmDialog` 范式。
- **T-057 binary-missing-onboarding-ux**：上一任务，刚因 dev 用 `findAllComponents({name:'NAlert'})` 按 naive-ui 组件名查询返回空导致 B.3 一度 FAIL（见任务书第 3.A 条警示）。本任务测试断言**必须**避开按 naive-ui 组件名查询。
- **T-036 log-ui-ux-polish**：LogViewer 的 `onCopy`（L145-172）是项目内**已验证正确**的剪贴板复制范式（try clipboard → catch fallback execCommand → message 反馈）。本任务 (A) 抄此范式。

无决策冲突需上报用户。

## 2. 问题陈述（三处，orchestrator 已逐处核实）

### (A) 剪贴板写入失败静默吞错

- `web/src/components/FirewallHint.vue` `copyCmd`(L75-85) 与 `copyAll`(L87-97)：`catch {}` 完全静默。
- `web/src/components/PublicIpDetector.vue` `copyIp`(L65-76)：同款静默 `catch`。
- **场景命中率**：内网 http 部署下 `navigator.clipboard` 在非安全上下文（非 HTTPS / 非 localhost）不可用，`writeText` 直接 reject。用户点"复制"无任何反应，误以为已复制 → 粘贴空白 → 困惑。

### (B) Server/Client「重置」静默丢弃未保存编辑

- `web/src/pages/Server.vue:97`、`web/src/pages/Client.vue:69`：「重置」按钮 `@click="loadConfig()"`。
- **问题**：点击直接重新拉取配置覆盖表单，用户填了一半的内容瞬间消失，无任何提示。文案"重置"也让用户误以为是"重置为默认值"而非"丢弃编辑重新加载"。

### (C) Wizard 死分支

- `web/src/pages/Wizard.vue:85-90`：`<n-text v-if="selectedRole === 'both'">frpc 客户端配置</n-text>` 与 `<n-text v-else>frpc 客户端配置</n-text>` 两分支文案完全相同，是冗余死分支。外层 `<div v-if="frpc||both">`（L84）已控可见性。

## 3. 验收标准（AC）

### A — 剪贴板失败反馈
- **AC-A1**: FirewallHint `copyCmd` / `copyAll` 在 `navigator.clipboard.writeText` 成功时 `message.success('已复制到剪贴板')`，并保留现有"已复制 ✓"短暂态视觉反馈。
- **AC-A2**: clipboard 不可用（writeText reject）时走临时 textarea + `document.execCommand('copy')` fallback；fallback 成功 `message.success`、失败 `message.error('复制失败：请手动选择文本复制')`。
- **AC-A3**: PublicIpDetector `copyIp` 同款行为。
- **AC-A4**: 不再有任何空 `catch {}`。

### B — 重置防误丢
- **AC-B1**: 文案"重置" → "重新加载"（Server.vue + Client.vue）。
- **AC-B2**: 点击"重新加载"时若表单 **dirty**（与上次加载快照不同）则弹 `ConfirmDialog`（内容含"将放弃当前未保存的修改并重新加载配置"语义）；确认后才 reload，取消则不 reload（不调 loadConfig/apiGet）。
- **AC-B3**: 不 dirty 时直接 reload（不打扰，不弹确认）。
- **AC-B4**: dirty 检测用"加载时存一份快照 + 浅比较当前表单"。**退路（成本过高时）**：rename + 总是确认。02 给出取舍决策。

### C — 死分支清理
- **AC-C1**: 合并两分支为单个无条件 `<n-text strong ...>frpc 客户端配置</n-text>`。
- **AC-C2**: 选 'both' / 'frpc' 都正确显示一次"frpc 客户端配置"标题，无回归。

### 横切要求
- **AC-X1**: 改动**仅限** FirewallHint.vue / PublicIpDetector.vue / Server.vue / Client.vue / Wizard.vue（+ 可选 utils/clipboard.ts）+ 各自测试 + baseline.json（+ 改了 util 则 dev-map）。禁扩散。
- **AC-X2**: 测试只升不降，同步 bump `scripts/baseline.json` 的 `frontend_tests` + `test_count`。
- **AC-X3**: 06 含裸 `## Adversarial tests` 段（禁前缀），至少一条"clipboard reject → fallback 也失败 → message.error 出现"反向证伪。
- **AC-X4**: 测试断言**禁止**按 naive-ui 组件名查询（`findComponent({name:'NAlert'})` 等不可靠，T-057 已踩坑）；用 DOM class / 文本定位 / `getExposed`。
- **AC-X5**: 模拟 API/clipboard 失败用真实形状（clipboard mock reject + execCommand mock），不用裸 `new Error()` 当结构化错误。
- **AC-X6**: 红线——不编辑 `.claude/`/`CLAUDE.md`/`.github/`；eslint 通过；SFC 纯逻辑 < 200 行。
- **AC-X7**: 不 git commit/push、不跑 archive-task。

## 4. 范围外（明确不做）

- 不抽取 LogViewer 现有 `onCopy` 到 util（若会牵动其测试快照则记为 backlog，见 02 决策）。
- 不改后端、store、路由守卫、其它任何 SFC。
- 不改 Wizard 行为（仅 (C) 纯模板清理）。

## 5. 风险与未知

- **R1**: Server.vue 含 `AllowPortsEditor` 子组件（端口策略），其内部状态是否纳入 dirty 检测会影响成本 → 02 决策。
- **R2**: e2e spec 是否断言 Server/Client/Wizard 的"重置"文案 → 04 需确认（任务书预判不触 login/dashboard/proxies 核心 e2e 路径）。
- **R3**: 是否抽 `utils/clipboard.ts` 取决于是否牵动 LogViewer 测试快照 → 02 决策（择低风险）。
