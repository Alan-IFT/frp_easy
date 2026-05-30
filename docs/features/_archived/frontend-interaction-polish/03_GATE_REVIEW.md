# 03 Gate Review — T-058 frontend-interaction-polish

- **评审角色**: gate-reviewer
- **日期**: 2026-05-30
- **输入**: 01_REQUIREMENT_ANALYSIS.md + 02_SOLUTION_DESIGN.md
- **职责**: 在 stage 4（开发）启动前，审需求完整性 + 设计可行性 + 范围纪律。不写代码。

## 1. 需求完整性

| 检查 | 结论 |
|---|---|
| 三处改动 AC 是否齐全 | PASS — A/B/C 各有可验证 AC，横切 AC-X1~X7 覆盖范围/测试/红线 |
| 验收是否可测 | PASS — A 用 message spy + clipboard mock；B 用 getExposed + apiGet mock 调用计数；C 用文本计数 |
| 范围边界是否明确 | PASS — AC-X1 白名单 6 文件（5 SFC + baseline，util 明确不抽） |

## 2. 设计可行性

| 检查 | 结论 |
|---|---|
| D1 不抽 util | PASS — 证据充分（LogViewer.spec:196-211 直接测 onCopy 内联，抽取会扩散+动快照），符合 insight L42 谨慎原则 |
| D2 dirty 不含 AllowPortsEditor | PASS — 决策有据（单向数据流 insight L13，纳入会扩散），安全侧偏置合理，已承诺记局限 |
| D3 快照浅拷贝 | PASS — form 全标量字段，`{ ...form.value }` 浅拷贝足够，无嵌套引用陷阱 |
| ConfirmDialog 复用 | PASS — T-056/Proxies 已验证范式，emit confirm + v-model:show 契约清晰 |
| (A) fallback 范式 | PASS — 1:1 搬运 LogViewer:147-171 已验证正确实现 |
| (C) 纯模板合并 | PASS — 外层 v-if 已控可见性，删内层 v-if/v-else 零行为变化 |

## 3. 范围纪律（红线）

- AC-X1 文件白名单清晰，无后端/store/路由/AppLayout 触碰。PASS。
- AC-X6 红线（不编辑 .claude/CLAUDE.md/.github；eslint；SFC 纯逻辑 < 200 行）已列入。Server.vue 当前 ~277 物理行但纯逻辑（非 import/template/expose）远 < 200，加 ~4 个 ref + 3 个小函数后仍 < 200。PASS（按 insight L22 纯逻辑行数口径）。

## 4. 与 insight 对齐核查

- **insight L45**（naive-ui 组件名查询不可靠）→ 02 §5 AC-X4 已硬约束断言用 DOM class/文本/getExposed。✅
- **insight L40**（裸 `## Adversarial tests` 标题）→ AC-X3 已要求。✅
- **insight L46**（加测试必须 bump baseline + orchestrator 真跑 verify_all）→ 02 §5 baseline bump + AC-X2 已要求。✅
- **insight L42**（抽 util 先 1:1 搬运）→ D1 决策不抽、内联搬运，更保守。✅

## 5. Conditions（dev 应主动消化，非阻塞 stage 4 启动）

- **C-1**（关键）：测试断言逐条自检不依赖 naive-ui 组件名查询（T-057 刚因 `findAllComponents({name:'NAlert'})` 返回空 FAIL 一次）。写完测试后必须逐用例确认查询方式。
- **C-2**：(A) message spy 必须用 `vi.mock('naive-ui')` 返回的**单例** spy 对象（参考 Wizard.spec messageSpies），否则 `useMessage()` 每次返回新对象无法断言（参考 Server.spec 现状——其 useMessage mock 每次新建，无法断言 success 调用，本任务 A 须改单例）。
- **C-3**：04 须确认 e2e spec 不断言 Server/Client"重置"/"重新加载"文案与 Wizard 标题（R2 闭环）。
- **C-4**：06 必须含裸 `## Adversarial tests`，至少一条 clipboard reject + execCommand 失败 → message.error 反向证伪。
- **C-5**：dirty 局限（仅 allowPorts 改动不触确认）写入 06 已知局限段。

## 6. Verdict

**APPROVED FOR DEVELOPMENT**

需求完整、设计可行、范围受控、insight 对齐。Conditions C-1~C-5 为开发期应主动消化项（非阻塞）。放行 stage 4。
