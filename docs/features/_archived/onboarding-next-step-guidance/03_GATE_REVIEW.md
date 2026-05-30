# 03 闸门评审 — T-062 · onboarding-next-step-guidance

> Stage 3 / Gate Reviewer · 模式：full · 输出语言：中文
> 上游：01_REQUIREMENT_ANALYSIS.md（READY）+ 02_SOLUTION_DESIGN.md（READY）
> 独立核验：GR 已实读 Wizard.vue / Client.vue / Proxies.vue / Server.vue / ProxyForm.vue / router.ts / Wizard.spec.ts，并 grep 各组件 import 现状。

## 1. 审计清单（8 维度）

| # | 维度 | 结论 | 一句话理由 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | 7 项 IS 均可测、无歧义词；触发条件（角色/就绪/token 双非空）均二值明确。 |
| 2 | 设计完整性 | PASS | 6.1~6.5 逐文件覆盖全部 7 项 IS；每项给出 template 片段 + handler + defineExpose 增量。 |
| 3 | 复用正确性 | PASS | 复用审计准确——已实证 useRouter 范式（Wizard.vue:199）、n-empty #extra slot、vue-router mock（Wizard.spec:58-59）均存在；零新依赖正确。 |
| 4 | 风险覆盖 | PASS | R-1（IS-1 自动跳转张力）、R-2（spec 引入 router mock）、R-3（n-empty #extra）、R-4（baseline bump）、R-5（e2e）均为真实风险且各有缓解；无明显遗漏。 |
| 5 | 迁移安全 | PASS | 纯增量 UI，无 DB/migration/store/API 变更；回滚=git revert，无状态残留。 |
| 6 | 边界处理 | PASS | BC-1~BC-10 覆盖 frps 角色不显示、缺失态不加、token 一空一非空/含空白、删除不触发、加载失败态不显示、router.push 而非 href。 |
| 7 | 测试可行性 | PASS（含 1 条件，见 C-1） | 13 AC 均可测（DOM 文本 + getExposed + push mock）；唯 AC-1 在 IS-1 自动跳转语境下需明确断言策略（见发现 F-1）。 |
| 8 | 范围外清晰度 | PASS | OOS-1~OOS-7 明确；IS-6 纯文案 / IS-7 非阻断边界清晰，开发者不会过度构建（无联动、无阻断）。 |

## 2. 发现（WARN / 条件）

> 无 FAIL，无需回退上游。以下为 APPROVED WITH CONDITIONS 的条件项与开发期注意点。

**F-1（R-1 张力评估 — 不回退，转为开发条件 C-1）**
- 责任文档：02 §6.1 / §8 R-1。
- 问题：IS-1 在 Wizard step3 全就绪分支加引导按钮，而该分支同 tick 自动 `router.push('/dashboard')`，引导按钮的"手动 push('/proxies')"在真实运行中几乎无机会被用户点到（自动跳转先行）。
- GR 评估：**不构成需求层缺陷，不回退 RA**。理由：(1) 需求 IS-1 + AC-1 的验收点是"引导入口存在且点击触发 push('/proxies')"，未要求阻止自动跳转；(2) AC-11 显式护栏要求 T-057 自动跳转行为不变——移除自动跳转会破坏 T-057（设计已正确拒绝）；(3) Client.vue（IS-2）保存成功后**无自动跳转**，那里的同款引导是高 UX 价值的主路径，IS-1 是补充。GR 判定：设计的"附加不阻断、接受局限"是与既有约束兼容的正确取舍。
- 转为条件 C-1（见 §4）：开发者实现时按"引导按钮存在性 + 可点击 goToProxies() 触发 push('/proxies')"为验收点（getExposed 调 goToProxies + push mock 断言），不在测试中断言"阻止了自动跳转"。

**F-2（import 增量明确化 — 开发期便利，非缺陷）**
- 责任文档：无（设计正确，GR 补充事实供开发者）。
- GR grep 核实各组件 import 现状，列出 IS 实现所需的新 import（开发者照此即可，避免遗漏）：
  - Wizard.vue：IS-1/IS-7 用 NAlert/NButton/NText —— **均已 import（L187-188）**，零新 import。
  - Client.vue：IS-2 用 n-alert —— 当前 import（L90）无 NAlert，**需新增 NAlert**；NButton/NSpace/useMessage 已有。
  - Proxies.vue：IS-3 用 n-alert —— 当前 import（L82）无 NAlert，**需新增 NAlert**；NEmpty/NSpace/NButton/NText 中 NEmpty/NSpace/NButton 已有，IS-4 #extra slot 内若用 NButton 已有，**若 IS-3 alert 内用 n-space 已有**。仅需新增 NAlert（若 IS-4 文案用 NText 则也加 NText，当前无）。
  - Server.vue：IS-5 仅用 n-button —— **NButton 已 import（L121）**，零新 import。
  - ProxyForm.vue：IS-6 方案 A 用 NText —— 当前 import（L97）有 NSpace/NTag 但**无 NText**，**需新增 NText**。

**F-3（Client/Proxies/Server.spec 新增 vue-router mock — R-2 已缓解，开发条件 C-2）**
- 责任文档：02 §8 R-2。
- GR 核实：Wizard.spec.ts:58-59 已用 `vi.mock('vue-router', () => ({ useRouter: () => ({ push: pushSpy }) }))` 范式，与 getExposed + naive-ui mock 共存无碍。Client/Proxies/Server.spec 此前无该 mock（组件原不 import useRouter）。
- 条件 C-2：开发者在这三个 spec 引入同款 vue-router mock 时，须**实读**各 spec 既有 Holder/mount 范式（insight L37 教训），确保模块级 mock 不与既有 stub 冲突，且既有用例（不依赖 router）零回归。

**F-4（Proxies 空态 #extra slot 退化路径 — R-3 已缓解）**
- 责任文档：02 §8 R-3。
- GR 核实：naive-ui NEmpty 原生支持 `#extra` slot。设计已给退化路径（若不可用，n-empty 后并列 n-button）。无阻塞。

## 3. 开发期高概率问题（预答）

1. **Q：IS-1 引导按钮在全就绪分支会被自动跳转"吃掉"，是否应改放缺失分支？**
   A：不改。按 BC-2 缺失态聚焦补二进制不加引导；按 AC-11 不能移除自动跳转。验收点是按钮存在 + 可点击 push（C-1）。设计已定调，开发者照实现。
2. **Q：Proxies IS-3 引导用 ref 标志（showPostSaveHint），编辑模态框关闭后是否还显示？应何时清除？**
   A：设计未要求清除时机。建议：保存成功后置 true 即可（持续显示直到用户离开页面或下次操作）；删除路径（handleDeleteConfirm）不置（BC-6）。若开发者认为需在重新打开新增模态框时清除，可在 handleAdd 置 false——属实现细节，不构成设计缺陷，记 04 即可。
3. **Q：Client IS-2 showNextStepHint 在 loadConfig/handleReloadClick 重载后是否应重置？**
   A：设计未要求。建议保持简单：仅 handleSave 成功置 true。重载不重置不影响正确性（用户已保存过，引导仍有意义）。属实现细节。
4. **Q：ProxyForm IS-6 用方案 A（NText help 文本）还是方案 B（placeholder）？**
   A：设计已决策方案 A（§6.5），理由是常驻可稳定 DOM 断言。需新增 NText import（F-2）。
5. **Q：测试断言导航如何做？**
   A：复用 Wizard.spec:58-59 的 `vi.mock('vue-router')` + pushSpy 范式（C-2）。getExposed 取 goToProxies/goToDashboard/goToMonitor handler 调用后断言 pushSpy 收到对应路径；或 find('button') 按文本点击后断言。零 naive-ui 组件名查询（insight L45）。

## 4. 条件（APPROVED WITH CONDITIONS）

- **C-1**：IS-1 测试以"引导按钮存在 + getExposed 调 goToProxies 触发 push('/proxies')"为验收，不断言"阻止自动跳转"；AC-11 显式回归用例锁死 T-057 全就绪自动跳转（success toast + push('/dashboard')）与缺失分支行为不变。
- **C-2**：Client/Proxies/Server.spec 引入 vue-router mock 前实读既有 mount 范式，确保既有用例零回归（实读断言判断 mock 影响，insight L37）。
- **C-3**：按 F-2 精确补 import（Client +NAlert、Proxies +NAlert[+NText 若用]、ProxyForm +NText；Wizard/Server 零新 import）。
- **C-4**：新增测试同步 bump baseline.json（frontend_tests/test_count/version），04 记录精确增量（红线 + AC-13）。
- **C-5**：06_TEST_REPORT 含裸标题 `## Adversarial tests` 段，至少含：both token 不一致→warning 出现（且非阻断推进）、token 相等/一空不误报、T-057 全就绪自动跳转未被新引导破坏、缺失分支不加规则引导。
- **C-6**：所有导航 router.push（insight L17），DOM 中无 `<a href>` 整页跳转（AC-12）。不碰后端/store/路由守卫/数据流（OOS-3）。

## 5. 裁决

**APPROVED WITH CONDITIONS** —— 需求与设计一致、完整、可实现，无 FAIL。R-1 张力经独立评估不构成需求层缺陷（不回退）。开发可进行，须满足 C-1~C-6。
