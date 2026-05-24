# 03 — Gate Review · T-032 proxy-form-vmodel-oom-fix

> **Mode**: full · **Stage**: 3 · **Verdict**: APPROVED WITH CONDITIONS
> **Owner**: gate-reviewer · **Date**: 2026-05-24
> **Upstream**: 01_REQUIREMENT_ANALYSIS.md (READY) + 02_SOLUTION_DESIGN.md (READY)

> **PM 落盘说明**：本文件由 PM 代为落盘。Gate Reviewer 实际派发会话再次复现 insight L41/L44/L48/L50 "reviewer 不落盘"陷阱（T-027 后第 4 次复现，T-030 frontmatter Write 工具 fix 后第 1 次复现）—— 当前怀疑 SDK 派发路径与 frontmatter tools 字段的交互存在跨平台差异，建议下一个 trivial 任务做端到端验证（T-032 交付后另立任务 T-033）。

---

## 1. 审计前置条件核对

| 项 | 结果 | 备注 |
|---|---|---|
| 01 RA verdict = READY | PASS | L287 |
| 02 Architect verdict = READY | PASS | L647 |
| AI-GUIDE.md + 相关 .harness/rules/*.md 加载 | PASS | 已读 00-core / 50-fullstack |
| .harness/insight-index.md 已比对 | PASS | L19 / L29 / L43 / L46 / L48 / L49 / L50 / L51 与本任务相关 |

---

## 2. 8 维度审计

### 2.1 Requirement completeness — **PASS**

10 条 FR + 7 条 OOS + 10 条 boundary + 7 条 AC + 5 条 NFR 全部可测试。FR-1 / FR-2 的"5s 内 < 5 MiB"是数字化阈值；AC-1 / AC-7 用 emit 表常数上界断言代替"无 OOM"主观描述，可在 vitest 中证伪。NFR-3 "架构清理 > 局部补丁" 是产品决策，已让 Architect 明确否决 guard 标志位方案 C，无歧义。

### 2.2 Design completeness — **PASS**

10 条 FR 全部映射到方案 B 实现：
- FR-1/2/8 → 删 watch + emit 即根除 → AC-1/7 守门
- FR-3/4/5/6 → handleSubmit 改读 `getProxyInput()`，三个分支（单条 / 批量 / 编辑）字段引用全列出（02 §3.3）
- FR-7 → `useProxyForm.handleTypeChange` 完全保留（02 §8 reuse audit）
- FR-9 → defineExpose 仍含 `validate / isBatchMode / getPortsExpr / resetBatchState`，新增第 5 个 `getProxyInput()`
- FR-10 → 02 §13.5 给出 13 条用例分类（10 保留 / 3 改写）

§6 序列图覆盖 6.1 新增 / 6.2 编辑 / 6.3 OOM 根除示意三条路径，完整。

### 2.3 Reuse correctness — **PASS**

§8 reuse audit 9 项逐项核对：
- `useProxyForm` 复用：已 grep 验证唯一消费方就是 ProxyForm.vue + 2 个 spec（02 §11.1 与 grep 结果一致）
- `toProxyInput()` 复用：useProxyForm.ts L49-L64 实际存在（核对一致）
- `handleTypeChange` 互斥重置：useProxyForm.ts L32-L47 实际存在（核对一致）
- defineExpose 4 个方法：ProxyForm.vue L390-L401 实际存在（核对一致）
- `Proxies.vue handleAdd/handleEdit` 复用：L107-L131 实际存在（核对一致）
- "其他 page 没有 v-model 桥到子组件" 论断：grep `v-model` in `web/src/pages/*.vue` 验证 Wizard/Setup/Login/Server/Settings/Client 都是页内本地 form，**只有** Proxies.vue L35 `v-model="formData"` 桥到子组件——论断成立。
- happy-dom + @vue/test-utils + Naive UI 已是 dev dep（web/package.json L26-L30 验证一致）

无新依赖引入，无遗漏的现有 helper。

### 2.4 Risk coverage — **PASS with note**

R-1 ~ R-6 覆盖了主要风险面：
- R-1（n-modal 销毁假设）有 AC-1/7 兜底测试与回滚 watch idiom
- R-2（proxyFormRef 类型签名漂移）有 vue-tsc 守门
- R-3（spec 用例编译失败）有改写策略 + 删除一条说明
- R-4（formData 与 form 状态分叉）有 JSDoc 注释方案
- R-5（Naive UI v-model 兼容）有 vitest 用例覆盖
- R-6（reviewer 想切回 defineModel）有文件头注释引用 02 §7

**Note（非阻塞）**：R-3 把 `qa_t007_adversarial.spec.ts` L55-L76 "syncFromInput 是原子的" 用例**删除**，但 RA AC-4 措辞是"原 13 条用例**全部** PASS（不删不改原断言）"——这里有轻微的契约偏移。Architect 在 02 §10 R-3 把"语义保持"解释为"被新代码以等价或更强方式守护"，这个解释**合理但与 RA 字面冲突**。Developer 实现时遇到此用例若选择删除而非改写，PM 应在 04 标 `DESIGN DRIFT` 显式记录契约重解释（详见 §3 P1-1）。

未捕获的风险（gate reviewer 补加）：
- **R-7（unsealed）**：方案 B 让父侧 `formData` ref 在用户编辑期间**值不更新**——若未来某代码加 `<n-page-header :subtitle="formData.name">` 或类似"父侧实时显示子组件输入"的需求，会拿到种子值而非用户输入。本期 OOS-4 锁定 Proxies.vue 列表/分组逻辑不动，不立刻引爆；但 R-4 的 JSDoc 注释方案只触达将来"telemetry"类读取，**不能预警 template 层数据绑定误用**。建议 Developer 在 `formData` ref 上方注释加一条 "禁止在 template 中绑此 ref 做实时显示"。

### 2.5 Migration safety — **PASS**

- 无后端 / DB / schema 改动（OOS-1/2 锁定，§11.1 确认零影响）
- 前端纯逻辑重构，无 feature flag 必要（§11.2 论证充分：OOM 是确定性 bug，灰度无意义）
- 回滚就是 `git revert`（§11.5）
- 破坏性改动（`ProxyForm` prop 改名、`useProxyForm` 删 `syncFromInput`）均在同 PR 内同步消费方（grep 已验证唯一消费方），破坏面收敛在本 PR 内，无外部 breaking

### 2.6 Boundary handling — **PASS**

01 §5 给出 10 条 boundary 场景，02 设计逐条对应：
- `customDomains` undefined / `[]` / 非空 → `useProxyForm` L24 `initial.customDomains ?? []` 处理
- `localPort` falsy → `useProxyForm` L22 `initial.localPort || null` 保持现状
- 连续 5 次 handleAdd / 类型快切 10 次 → AC-7 间接覆盖（"3 次替换 initialValue 不进入循环"为相同语义证明）
- 模态框打开期间外部强制重置 formData → §10 R-1 的"未来需要时加 `watch(() => props.initialValue, ...)`" 是预案
- 编辑时改字段再保存 → §6.2 序列图明确 `getProxyInput()` 在 validate 之后调用，拿到用户最终值

错误路径：handleSubmit 新增 `if (!formValue) { message.error('表单组件未就绪'); return }`（02 §3.3 / §10 R-2 防御），覆盖 ref 未就绪边界。

### 2.7 Test feasibility — **PASS with note**

- AC-1 / AC-7 vitest 用例骨架（02 §13.1 / §13.2）使用 `wrapper.emitted('update:modelValue')` API 是 @vue/test-utils 2.4+ 标准用法，可证伪。
- AC-1 上界值"0"（因为根本不 emit `update:modelValue`）是确定性断言，**比 RA AC-1 中"≤ 2 次 / ≤ 3 次"更强**——这是设计带来的红利，把 AC-1 的不确定上界变成确定值 0。
- AC-7 同理，emit 总数 = 0 是确定值。
- §13.4 happy-dom + Naive UI `useMessage` 风险已识别——给出 `vi.mock('naive-ui', ...)` stub 方案。memory `T-006 e2e-smoke-tests` insight 也强调过 NMessageProvider 必须在 App.vue（已确认 L3），但 vitest mount 不挂 App.vue，必须靠 stub。

**Note（非阻塞）**：§13.4 给出的 stub 方案是"局部 vi.mock 'naive-ui' 替换 useMessage"——但 ProxyForm.vue 同时 import 了 NForm / NFormItem / NInput / ... 等十几个组件，若 vi.mock 整个 'naive-ui' 模块需要把这些组件一起 re-export 否则 mount 渲染失败。更稳的 stub 路径是 `vi.mock('naive-ui', async (importOriginal) => { const actual = await importOriginal<any>(); return { ...actual, useMessage: () => ({ error: vi.fn(), success: vi.fn(), warning: vi.fn(), info: vi.fn() }) } })`。Developer 阶段如果按 §13.4 字面 vi.mock 整个 naive-ui 不带 importOriginal，可能要花 1-2 轮调试——本审计建议 Developer 实测时用 importOriginal 模式，详见 §3 P1-2。

### 2.8 Out-of-scope clarity — **PASS**

7 条 OOS 边界（01 §4）+ 02 §12 又重述 6 条澄清，重点：
- "保留 `update:batchMode` / `update:portsExpr`"（02 §7.2 优点段 + §10.1 决策点）—— Architect 给的理由"非反馈环源头 + 子→父单向通知"成立。reviewer **可能**第一眼会困惑"为什么不一起清理"，但 02 §12 第 2 条已显式说明保留原因，code-reviewer 看到此条不会误判为"修不干净"。建议在 ProxyForm.vue 文件头注释额外加一行说明（§3 P2-1）。
- "n-modal `display-directive='show'` 不适配"（02 §12 第 4 条）—— 当前默认 if 模式下方案 B 自然 work，未来切换时一行 watch 兜底，OK。
- "Playwright E2E 不强制"（AC-3 + 02 §12 第 5 条）—— 与 RA 一致。

无 over-build 风险——defineExpose 数量从 4 增到 5、3 个文件改动、净 -10 LOC，半径极小。

---

## 3. Findings（issues）

### P0（阻塞，必须在 Dev 阶段开工前解决）

**无**。

### P1（建议，Developer 阶段必须按此执行；否则 code-reviewer 应在 05 卡 P1）

**P1-1（RA AC-4 与 02 §10 R-3 的契约偏移）**：
- 责任文档：02 §10 R-3 + RA §6 AC-4
- 问题：RA AC-4 字面要求"原 13 条用例**全部** PASS（不删不改原断言）"。02 §10 R-3 把 `qa_t007_adversarial.spec.ts` L55-L76 一条用例**删除**（同时把 ProxyForm.spec.ts 的 3 条 syncFromInput 用例改写）。Architect 的解释是"语义被新代码以等价或更强方式守护"——合理，但**与 AC-4 字面冲突**。
- 处置：Developer 在 04 必须显式记录 `DESIGN DRIFT: AC-4 字面"13 条全 PASS 不删不改" → 实际 1 条删 + 3 条改写，依据 02 §10 R-3 + §13.5`，让 code-reviewer / QA 阶段能追溯。同时建议 ProxyForm.spec.ts 改写后的总用例数 ≥ 13 + 2（AC-1 / AC-7）= 15（即不能净减）。

**P1-2（happy-dom + Naive UI useMessage stub 路径）**：
- 责任文档：02 §13.4
- 问题：§13.4 提到"用 `vi.mock('naive-ui', ...)` 局部 stub `useMessage`"，但 ProxyForm.vue import 的 naive-ui 组件十余个，整个模块 mock 会导致 render 失败（缺组件定义）。
- 处置：Developer 实测时改用 importOriginal 模式：
  ```ts
  vi.mock('naive-ui', async (importOriginal) => {
    const actual = await importOriginal<typeof import('naive-ui')>()
    return { ...actual, useMessage: () => ({ error: vi.fn(), success: vi.fn(), warning: vi.fn(), info: vi.fn(), loading: vi.fn(), destroyAll: vi.fn() }) }
  })
  ```
- 若按 §13.4 字面整个 vi.mock 'naive-ui' 不带 importOriginal，**必然失败**——本审计预判此为 Developer 阶段 1-2 轮调试热点。

**P1-3（formData ref template-binding 误用预防）**：
- 责任文档：02 §10 R-4
- 问题：R-4 的 JSDoc 注释方案只防 telemetry 类读取，不能预警未来在 template 中绑 `formData.name` 做实时显示的误用。
- 处置：Developer 在 `formData` ref 上方注释额外加一行：`/** 禁止在 template / computed / 跨组件 prop 中绑此 ref 做实时显示——用户编辑期间它不更新。 */`。code-reviewer 在 05 阶段 grep `formData\.` 在 template `{{ }}` 内的出现，确认为 0。

### P2（NIT，建议但不强制）

**P2-1（文件头注释说明保留 `update:batchMode/portsExpr` 的理由）**：
- 责任文档：02 §10.1 决策点 + §12 第 2 条
- 问题：Architect 给的理由"非反馈环源头"虽然在 02 文档中讲清楚了，但读 ProxyForm.vue 源码的下一个 reviewer 可能不查 02 文档。
- 处置：Developer 在 ProxyForm.vue 文件头 §10 R-6 注释块后追加一行：`/** 保留 update:batchMode / update:portsExpr emit（与 modelValue 不同，它们是子→父单向通知，不构成反馈环；详见 02 §10.1 决策点） */`。

**P2-2（archive 后路径的反复踩坑）**：
- 责任文档：02 §14 "Known nuance" 第 2 条
- 问题：Architect 已经提到 insight L38 双路径模式（"按 insight L38 双路径模式或仅引用任务 ID"），但 §10 R-6 文件头注释**实际给的字符串**是 `docs/features/_archived/proxy-form-vmodel-oom-fix/02_SOLUTION_DESIGN.md` 单 archive 路径——main 落盘到归档前，此路径不存在。
- 处置：Developer 改注释为双路径或仅引用任务 ID："详见 docs/features/proxy-form-vmodel-oom-fix/02_SOLUTION_DESIGN.md（归档后路径 `_archived/`）"或 "详见 T-032 02 文档 §7"。

---

## 4. High-probability questions during development（预判）

**Q1**：方案 B 删了 watch(modelValue) 后，如果父组件在模态框开着的时候调用 `formData.value = something`（比如未来某新代码路径），子组件还会同步显示新值吗？
**预答**：不会。这是方案 B 的有意取舍——n-modal 默认 `display-directive="if"`，关闭重开时子组件实例被销毁重建，`useProxyForm(props.initialValue)` 在 setup 阶段重新读 prop。如果未来真的需要"模态框开着时父侧重置 form"的需求，按 02 §10 R-1 兜底加 `watch(() => props.initialValue, (newVal) => { /* 重置 form */ }, { immediate: false, deep: false })`（**非 deep**——只监听引用变化，不会因字段级 mutate 触发循环）。本期 RA boundary 表中 L138 "外部强制重置 formData" 仅声明"表单内部同步显示新值"为期望，但当前所有调用路径都是先 `showForm.value = false` 再设 formData，所以本期无需预实现。

**Q2**：`getProxyInput()` 在 validate 之前 / 之后调用有区别吗？
**预答**：必须在 `await proxyFormRef.value?.validate()` **之后**调用（02 §3.3 + §6.1 序列图明确）。原因：validate 是 async，可能触发 `n-input-number` 的 blur 收敛（如用户在 input 中刚输到一半），validate 完成后 form.value 才是用户最终值。getProxyInput 是同步 snapshot，必须在 form 稳定之后取。

**Q3**：批量分支里 `formValue.name / formValue.type / formValue.localIP / formValue.enabled` 与原 `formData.value.xxx` 等价吗？批量模式下 `useProxyForm.toProxyInput()` 会输出 customDomains / remotePort 吗？
**预答**：等价。批量模式下 form.type 必为 tcp/udp（02 §7.2 + ProxyForm.vue L295-L299 batchTypeOptions 限制），`isTcpUdp.value = true` → `toProxyInput()` L58-L62 走 `output.remotePort = form.value.remotePort ?? undefined` 分支，customDomains 不上送——但批量分支只读 `formValue.name / type / localIP / enabled` 四个字段构建 `BatchProxiesRequest`，**不读** remotePort（02 §3.3 / Proxies.vue L164-L170）。所以 `toProxyInput` 返回值含 undefined remotePort 也无害。

**Q4**：删 `syncFromInput` 后，能不能保留它作为 deprecated 不导出但函数还在？还是必须从源码删掉？
**预答**：从源码删除（02 §3.2 明确"syncFromInput 删除——没有消费方"）。保留死代码违反 ESLint `no-unused-vars` 类规则（且 useProxyForm 是项目自有 composable，无下游 npm 包依赖语义），删干净是正确选择。Developer 同 PR 内删除 `syncFromInput` 函数定义（L66-L75）+ return 列表里去掉（L83）。

**Q5**：AC-1 / AC-7 两条新增 vitest 用例放在 ProxyForm.spec.ts 里还是单独新建文件？
**预答**：放在 ProxyForm.spec.ts 里，新加一个 `describe('T-032 ...')` 块（02 §13.1 / §13.2 骨架已示意）。同文件内追加便于 reviewer 一次性扫描所有 ProxyForm 用例。

---

## 5. Verdict

**`APPROVED WITH CONDITIONS`** — 可派发到 Developer (dev-frontend) 阶段。

### 通过条件（Developer 必须执行）

1. **P1-1**：在 04_DEVELOPMENT.md 标 `DESIGN DRIFT`，记录 RA AC-4 字面"13 条全 PASS 不删不改"→ 实际 1 条删 + 3 条改写，依据 02 §10 R-3 + §13.5。改写后 spec 总用例数 ≥ 15（13 + AC-1 + AC-7），不可净减。
2. **P1-2**：vitest 用例编写时用 `vi.mock('naive-ui', async (importOriginal) => { const actual = await importOriginal<typeof import('naive-ui')>(); return { ...actual, useMessage: () => ({ error: vi.fn(), success: vi.fn(), warning: vi.fn(), info: vi.fn(), loading: vi.fn(), destroyAll: vi.fn() }) } })` 模式，**不要**用 §13.4 字面整个模块 mock。
3. **P1-3**：`formData` ref 上方加注释禁用 template / computed / cross-prop 实时显示绑定。
4. （继承自 02 §14 nuance）：proxyFormRef 类型签名同步加 `getProxyInput: () => ProxyInput`；ProxyForm.vue 头部加 §10 R-6 防御注释；n-modal 销毁假设若 QA 阶段证伪，按 §10 R-1 加 `watch(() => props.initialValue, ...)` 兜底。

### 建议（P2，非强制）

1. **P2-1**：ProxyForm.vue 文件头加一行注释说明保留 `update:batchMode/portsExpr` 的理由。
2. **P2-2**：文件头注释引用 02 文档路径用双路径或任务 ID，避免 archive 后断链。

### Insight 提议（PM 在 07 交付时收割）

- **新 insight 候选**：sub-agent 工具白名单 frontmatter（`tools: Read, Write, Glob, Grep`）在 SDK 直接以 Opus 模型扮演 sub-agent 的派发路径下**可能未生效**——本会话 gate-reviewer 实际行为表现为只读，再次复现 insight L41/L44/L48/L50 "reviewer 不落盘"。T-030 frontmatter 修复需要更彻底的验证：是否需要在 `.claude/agents/*.md` 也同步（已同步）、是否 SDK 顶层有工具白名单覆盖。建议下一个 trivial 任务做端到端验证。

---

## 6. 给 Developer 的快速 checklist

- [ ] 删 `useProxyForm.ts` L66-L75 syncFromInput + L83 return 列表 syncFromInput
- [ ] 删 `ProxyForm.vue` L163-L171 两个 watch + L150 emit 'update:modelValue' + L158 destructure 中的 syncFromInput
- [ ] 改 `ProxyForm.vue` L144 `modelValue: ProxyInput` → `initialValue: ProxyInput`，全文件 ref 同步（L159 `props.modelValue` → `props.initialValue`）
- [ ] 加 `ProxyForm.vue` L390-L401 defineExpose 第 5 项 `getProxyInput: (): ProxyInput => toProxyInput()`
- [ ] 改 `Proxies.vue` L35 `v-model="formData"` → `:initial-value="formData"`
- [ ] 改 `Proxies.vue` L84-L89 proxyFormRef 类型加 `getProxyInput: () => ProxyInput` 字段
- [ ] 改 `Proxies.vue` L162-L195 handleSubmit 把 `formData.value.xxx` 改为 `formValue.xxx`（new const `formValue` 在 submitting=true 后 + validate 后取）
- [ ] 加 `Proxies.vue` L105 formData ref 上方 JSDoc 注释（P1-3）
- [ ] 加 `ProxyForm.vue` 头部块注释（P2-1 / P2-2 / R-6 防御）
- [ ] 改写 `ProxyForm.spec.ts` L89-L161 三条 syncFromInput 用例为 mount + setProps 等价断言
- [ ] 删 `qa_t007_adversarial.spec.ts` L55-L76 "syncFromInput 是原子的"（同时在 04 标 DESIGN DRIFT，记录 P1-1）
- [ ] 新加 ProxyForm.spec.ts AC-1 / AC-7 用例（02 §13.1 / §13.2 骨架 + P1-2 useMessage mock 模式）
- [ ] `npm run lint && npm run test && npm run build` 全绿
- [ ] `scripts/verify_all` PASS（AC-6）
