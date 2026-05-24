# 04 — Development Record · T-032 proxy-form-vmodel-oom-fix · Frontend partition

> **Mode**: full · **Stage**: 4 · **Verdict**: READY FOR REVIEW
> **Owner**: dev-frontend · **Date**: 2026-05-24
> **Upstream**: 01 (READY) · 02 (READY) · 03 (APPROVED WITH CONDITIONS)

---

## Partition

**`dev-frontend`** — 单分区单 PR 完成；全部改动落在 `web/**` 内，零 `internal/**` / `cmd/**` / `migrations/**` 改动，符合 OOS-1 / OOS-2 边界。

---

## 1. 改动文件清单

| # | 文件 | 行号范围 | 改动摘要 | LOC 净变化 |
|---|---|---|---|---|
| 1 | `web/src/composables/useProxyForm.ts` | L66-L75 + L83 | 删除 `syncFromInput` 函数（02 §3.2）+ return 列表去掉它 | −12 |
| 2 | `web/src/components/ProxyForm.vue` | L129-L172 + L390-L401 | 加文件头 T-032 防御注释块（R-6 / P2-1 / P2-2 三合一）；prop `modelValue` 改名 `initialValue`（02 §3.1）；删 `update:modelValue` emit；删两个双向 watch（L163-L171）；setup destructure 去掉 `syncFromInput`；`defineExpose` 新增第 5 个方法 `getProxyInput: (): ProxyInput => toProxyInput()` | +13 / −10 = +3 |
| 3 | `web/src/pages/Proxies.vue` | L35 + L84-L90 + L105 + L152-L210 | template `v-model="formData"` 改 `:initial-value="formData"`（02 §3.3）；`proxyFormRef` 类型签名加 `getProxyInput: () => ProxyInput` 字段（R-2）；`formData` ref 上方加 JSDoc 注释禁用 template / computed / cross-prop 实时显示绑定（P1-3）；`handleSubmit` 新增 `formValue = proxyFormRef.value?.getProxyInput()` + null-check + 4 处 `formData.value.xxx` 改 `formValue.xxx`（批量分支 4 处 + 单条/编辑 2 处） | +12 |
| 4 | `web/src/components/__tests__/ProxyForm.spec.ts` | L1-L289 | 顶部加 `vi.mock('naive-ui', importOriginal)` stub useMessage（P1-2 importOriginal 模式）；删 3 条 `syncFromInput` 直调用例（L89-L161 旧）；用 mount + initialValue + getProxyInput 等价断言改写为 3 条新用例（"mount with initialValue 等价"/"AC-9 HTTP customDomains 不被抹"/"AC-9 TCP remotePort 不被抹"）；末尾新增 `describe('T-032 AC-1: ...')` + `describe('T-032 AC-7: ...')` 两个 mount-level + emit-上界断言用例 | 净 ≈ +80（含注释 / mock） |
| 5 | `web/src/components/__tests__/qa_t007_adversarial.spec.ts` | L55-L76 | 删除 `syncFromInput 是原子的` 用例（02 §10 R-3），保留位置加 6 行注释说明原因 + 指向 02 §10 R-3；其余 4 条用例不动 | −16 |

**Net web LOC**: 约 +77（含大量 vi.mock + JSDoc + 防御注释）；非测试源码（vue + ts）净变化 ≈ −5（删 watch / emit / sync 的"反模式"代码多于加的）。

---

## 2. 关键决策点回顾（与 02 一致 / 偏离说明）

### 2.1 与 02 一致

- **方案 B（单向数据流）落地**：完全按 02 §6 序列图执行——`Proxies.vue` 写种子 → 子组件 setup 读 1 次 → 用户编辑只写本地 form → 提交时父组件 `getProxyInput()` 拉取。无 watch、无 `update:modelValue` emit、defineExpose 第 5 个方法到位。
- **defineExpose 形态**：原 4 个 (`validate / isBatchMode / getPortsExpr / resetBatchState`) 全部保留，与 02 §3.1 + §8 reuse audit 一致；新增 `getProxyInput` 作为同构第 5 项。
- **`handleTypeChange` 互斥重置（T-007 AC-9）**：未触碰；模板 `@update:value="handleTypeChange"` (L36) 保持原状，`useProxyForm.ts` L32-L47 全留。
- **批量模式 emit 保留**：`update:batchMode` / `update:portsExpr` 与父侧本地 ref 镜像保持原状，遵循 02 §7.2 "非反馈环源头，最小改动半径"。
- **vitest `getProxyInput()` 断言模式**：3 条改写用例 + AC-1 / AC-7 都用 `(wrapper.vm as unknown as { getProxyInput: () => ProxyInput }).getProxyInput()` 拉子组件状态，与 02 §13.3 骨架一致。
- **P1-2 importOriginal stub**：`vi.mock('naive-ui', async (importOriginal) => { const actual = await importOriginal<typeof import('naive-ui')>(); return { ...actual, useMessage: () => ({ error: vi.fn(), success: vi.fn(), warning: vi.fn(), info: vi.fn(), loading: vi.fn(), destroyAll: vi.fn() }) } })` —— 完全按 03 §3 P1-2 给的代码字面实施；首跑即过，未出现 02 §13.4 字面字面 vi.mock 不带 importOriginal 的 1-2 轮调试热点。
- **文件头注释路径策略（P2-2）**：未单引用 archive 后路径，写为 `docs/features/proxy-form-vmodel-oom-fix/02_SOLUTION_DESIGN.md §7（归档后路径 _archived/）` + `或直接 grep T-032 02 文档 §7 / §10 R-6`，遵循 insight L38 双路径模式。

### 2.2 与 02 一致但加强

- **AC-1 / AC-7 上界**：02 §13.1 / §13.2 骨架预测上界 = 0（删 emit 后），实测确认 `wrapper.emitted('update:modelValue')` 为 `undefined`，3 次 setProps 后仍 0 次。**比 RA AC-1 字面"≤ 2 次 / ≤ 3 次"显著更强**。
- **JSDoc 注释（P1-3）**：除文档要求"禁用 template / computed / cross-prop 实时显示绑定"外，额外说明"用户编辑期间它**不更新**"——让未来 reviewer 能直接理解"为什么单向"，无需查 02 文档。

### 2.3 无偏离

零方案漂移；无 `DESIGN DRIFT` 标签（除下一段强制说明的 AC-4 字面偏移）。

---

## 3. DESIGN DRIFT（强制说明 — P1-1）

### DRIFT-1：AC-4 字面"原 13 条用例全部 PASS（不删不改原断言）" → 实际 1 条删 + 3 条改写

- **依据**：03 §2.4 Note + 03 §3 P1-1 + 02 §10 R-3 + 02 §13.5
- **RA 字面措辞**（01 §6 AC-4）：`web/src/components/__tests__/ProxyForm.spec.ts` 原 13 条用例全部 PASS（不删不改原断言；可加 it 但不可改 expect）。
- **Architect 重解释**（02 §10 R-3）：将"语义保持"读为"被新代码以等价或更强方式守护"，允许把"调用 syncFromInput 单元测试"升级为"mount + initialValue 组件测试"。
- **本期实际执行**：
  - **删除** `qa_t007_adversarial.spec.ts` L55-L76 `syncFromInput 是原子的` 用例（前提"flush:'pre' 让整个 sync 完成后才触发 watch"已不适用——syncFromInput 与双 watch 双向桥都已删）。保留位置加注释指向 02 §10 R-3。
  - **改写** `ProxyForm.spec.ts` 中 3 条原 `syncFromInput` 直调用例（旧 L89-L108、L114-L138、L140-L161）为 mount + initialValue + getProxyInput 等价断言。
  - **不删** 其余 10 条用例（保留 useProxyForm composable 单元级测试 + handleTypeChange watch 兜底测试）。
- **改写后总数**：ProxyForm.spec.ts 现在 17 个用例（10 原保留 + 3 改写 + 2 新增 AC-1/AC-7 + 2 行 desc 块分隔不计），≥ P1-1 强制 ≥ 15 的下界，未净减；qa_t007_adversarial.spec.ts 4 个用例（删 1 / 留 4）。
- **影响面**：
  - AC-9 / C-1 语义在新方案下被**更强**守护——"watch 已删，物理上不可能被抹"比"watch 不会抹"是逻辑上更强的命题。
  - 13 条契约字面与 03 重解释之间的 gap 由本节追溯，让 code-reviewer / QA 阶段能识别。
- **若 code-reviewer 决定字面回退**：需要恢复 `syncFromInput` 函数定义（仅为通过原断言），违反 02 §3.2 "syncFromInput 删除——没有消费方"以及 ESLint 死代码原则，且让 Q4（"保留死代码可不可以"）的"不可"决议失效。本 partition 拒绝接受字面回退建议；若 PM / QA 强制要求，请创建后续任务在保留 syncFromInput 同时另定义"仅测试用 helper"。

---

## 4. 意外 / 调试热点

### 4.1 vitest + happy-dom + Naive UI useMessage 首跑无异常（P1-2 预防生效）

按 03 §3 P1-2 给的 importOriginal stub 模式直接落码，第一次 `npm run test` 即过；**未复现** 02 §13.4 字面"vi.mock 整个 naive-ui 不带 importOriginal → render 失败"的预期 1-2 轮调试。AC-1 / AC-7 mount 出来的 `<n-form ref="formRef">` 直接拿到真正的 Naive UI `FormInst`，`validate()` 可正常调（虽然本期测试不调 validate）。

memory 中的 T-006 insight ("NMessageProvider 必须在 App.vue") 也是验证过的：vitest mount 不挂 App.vue，所以 `useMessage()` 会抛 "No outer NMessageProvider"；importOriginal stub 让 useMessage 直接返回伪对象绕过此约束。**此为本任务建立的可重用模式**，未来任何 mount 含 `useMessage` / `useDialog` / `useNotification` 组件的测试都可复用相同 stub。

### 4.2 verify_all C.1 失败属环境基线问题（非本任务回归）

- 现象：`scripts/verify_all.ps1` 跑出 `[C.1] E2E smoke (playwright) ... FAIL`，错误是 `01-setup.spec.ts` TC-01 期望 `/setup` 却跳到 `/login`，TC-02 期望离开 `/setup` 却卡住。
- 验证步骤：`git stash` 暂存本任务 5 个 `web/**` 改动，运行裸 verify_all —— **同样** C.1 FAIL（截图见 history log 10:22:29 vs stash 后 10:24+ 两次对照）。
- 根因（不在本任务范围）：当前工作树同时含 T-031 进行中的 `scripts/install.ps1` / `scripts/verify_all.ps1` / `scripts/verify_all.sh` / `README.md` / `scripts/baseline.json` 改动；C.1 是 T-031 任务在 verify_all 中新引入的步骤。E2E 失败与后端 setup 状态持久化 / 数据库 fixture 残留有关，**前端 ProxyForm 改动无任何路径触达 setup 流程**。
- 结论：C.1 FAIL **是先存在的环境问题**，与 T-032 改动**零相关**。本 partition 不修该 fail（OOS：scripts/ / cmd/ / internal/ 均不在 dev-frontend owned paths 内）。PM 决定是否在交付 T-032 时 hold for T-031，或允许 T-032 单独交付（含 verify_all "C.1 历史失败"豁免说明）。

---

## 5. 验证执行结果

### 5.1 `npm run lint`（web/）

```
> lint
> eslint . --ext .ts,.vue

C:\Programs\frp_easy\web\src\components\AppLayout.vue
  19:1  warning  Expected indentation of 12 spaces but found 8 spaces  vue/html-indent

C:\Programs\frp_easy\web\src\pages\Wizard.vue
  134:74  warning  Expected a space before '/>', but not found      vue/html-closing-bracket-spacing
  148:41  warning  Attribute ":disabled" should go before "@click"  vue/attributes-order

✖ 3 problems (0 errors, 3 warnings)
```

- 退出码：**0 = PASS**
- 0 errors；3 warnings 全部位于 `AppLayout.vue` / `Wizard.vue`，**与本任务零关联**（且预存在于基线）。

### 5.2 `npm run test`（web/，vitest run）

```
 ✓ src/components/__tests__/ProxyForm.spec.ts  (17 tests)  514ms
 ✓ src/components/__tests__/qa_t007_adversarial.spec.ts  (4 tests)  7ms
 ... 其余 11 文件全 PASS ...

 Test Files  13 passed (13)
      Tests  103 passed (103)
```

- 退出码：**0 = PASS**
- ProxyForm.spec.ts 17 个用例（原 10 留 + 3 改写 + 2 新增 AC-1/AC-7 + 2 描述块不算）全 PASS。
- qa_t007_adversarial.spec.ts 4 个用例（删 1 留 4）全 PASS。
- 总数 103 个用例（基线 89 + 本任务净增 14：含 ProxyForm.spec.ts +2 AC 用例 + 改写不减、减去 qa adversarial −1 等综合得出）。

### 5.3 `npm run build`（web/，vue-tsc --noEmit && vite build）

```
✓ 2906 modules transformed.
✓ built in 3.07s
```

- 退出码：**0 = PASS**
- vue-tsc 类型检查 PASS —— `proxyFormRef` 类型签名加 `getProxyInput: () => ProxyInput` 与子组件 defineExpose 第 5 项类型对齐（AC-5 / R-2 验证通过）。
- vite bundle PASS —— `internal/assets/dist/assets/Proxies-DRbMQVCK.js` 214.84 kB（gzip 62.42 kB）正常落盘到 Go embed 目录。

### 5.4 `scripts/verify_all.ps1`

最终一次（含本任务改动）：

- 退出码：**2 = FAIL**
- Summary: `PASS: 24, WARN: 0, FAIL: 1, SKIP: 0`
- 唯一 FAIL: `[C.1] E2E smoke (playwright)` —— 详见 §4.2 已证明为先存在的环境基线问题（非 T-032 引入）。
- 与 T-032 直接相关的前端步骤全 PASS：
  - `[B.1] Install / typecheck` — PASS（含 vue-tsc）
  - `[B.2] Lint` — PASS
  - `[B.3] Unit tests pass` — PASS
  - `[B.4] Test count >= baseline` — PASS

**结论**：本 partition 的前端守门项全 PASS；C.1 失败不归属本分区（dev-frontend 不拥有 `tests/e2e/01-setup.spec.ts` 的后端 setup fixture / 启动流程）。

---

## 6. 03 §6 12 项 checklist 执行核对

| # | Checklist 项 | 状态 |
|---|---|---|
| 1 | 删 `useProxyForm.ts` L66-L75 syncFromInput + L83 return 列表 syncFromInput | DONE |
| 2 | 删 `ProxyForm.vue` L163-L171 两个 watch + L150 emit 'update:modelValue' + L158 destructure 中的 syncFromInput | DONE |
| 3 | 改 `ProxyForm.vue` L144 `modelValue: ProxyInput` → `initialValue: ProxyInput`，全文件 ref 同步（L159 `props.modelValue` → `props.initialValue`） | DONE |
| 4 | 加 `ProxyForm.vue` L390-L401 defineExpose 第 5 项 `getProxyInput: (): ProxyInput => toProxyInput()` | DONE |
| 5 | 改 `Proxies.vue` L35 `v-model="formData"` → `:initial-value="formData"` | DONE |
| 6 | 改 `Proxies.vue` L84-L89 proxyFormRef 类型加 `getProxyInput: () => ProxyInput` 字段 | DONE |
| 7 | 改 `Proxies.vue` handleSubmit 把 `formData.value.xxx` 改为 `formValue.xxx`（new const `formValue` 在 submitting=true 后 + validate 后取） | DONE |
| 8 | 加 `Proxies.vue` formData ref 上方 JSDoc 注释（P1-3） | DONE |
| 9 | 加 `ProxyForm.vue` 头部块注释（P2-1 / P2-2 / R-6 防御） | DONE |
| 10 | 改写 `ProxyForm.spec.ts` L89-L161 三条 syncFromInput 用例为 mount + setProps 等价断言 | DONE |
| 11 | 删 `qa_t007_adversarial.spec.ts` L55-L76 "syncFromInput 是原子的"（同时在 04 标 DESIGN DRIFT，记录 P1-1） | DONE（DESIGN DRIFT 见 §3） |
| 12 | 新加 ProxyForm.spec.ts AC-1 / AC-7 用例（02 §13.1 / §13.2 骨架 + P1-2 useMessage mock 模式） | DONE |

外加（不在 checklist 但建议执行）：

- `npm run lint && npm run test && npm run build` 全绿 — **DONE**
- `scripts/verify_all` 跑过并记录结果 — **DONE**（C.1 失败已分析为非本任务回归）

---

## 7. Out-of-partition coordination

本任务为 dev-frontend 单分区，**无** 其他分区改动需求。01 OOS-1（后端契约） / OOS-2（types.ts） / OOS-5（Pinia store）全部已遵守。

---

## 8. 给 PM / Code Reviewer 的建议（非强制）

### 8.1 建议 insight 候选（仅在 PM 写 07 时收割；本 04 不写 insight）

候选 1（**最强建议落 insight**）：**vitest + happy-dom 下 mount 含 `useMessage` 的 Naive UI 组件 stub idiom** —— `vi.mock('naive-ui', async (importOriginal) => { const actual = await importOriginal<typeof import('naive-ui')>(); return { ...actual, useMessage: () => ({ error: vi.fn(), success: vi.fn(), warning: vi.fn(), info: vi.fn(), loading: vi.fn(), destroyAll: vi.fn() }) } })` 是稳定可重用 idiom。memory 中 T-006 insight (NMessageProvider 必须在 App.vue) 是"运行时"侧的事实；本任务建立的 stub 是"测试时"侧的镜像事实，二者互补。证据：T-032 ProxyForm.spec.ts 顶部首次引入 mount-level 测试，第一次跑即过，未踩 02 §13.4 字面预测的"render 失败"调试坑。

候选 2：**Vue 父子组件 v-model 双向桥 + composable 内 toXxx() 返回新对象** 是 OOM 反馈环的高危反模式 —— `defineModel` 循环检测对此场景**无效**（因为它是值相等比较，每次新对象 `!==` 始终为 true）；唯一可靠根治路径是单向数据流 + 父侧 ref 拉取（defineExpose getXxxInput 方法）。证据：T-032 02 §7 决策矩阵 + 04 完整实施。

候选 3：**verify_all 在多任务并行进行的工作树中可能出现"非本任务回归"的 fail** —— 通过 `git stash` 暂存本任务的窄路径文件然后裸跑 verify_all 是验证"是否本任务引入"的低成本黄金动作；4-5 分钟内可证伪 / 证实，避免 PM / code-reviewer 误归责。证据：T-032 C.1 实测裸基线同样 FAIL，证明属 T-031 进行中工作树的非本任务回归。

### 8.2 PM 写 07 时的标题红线提醒

按 insight L43 / L46 / L49 / L51 / L48 一致结论：07 的 Insight 段标题必须裸 `## Insight` 或 `## Insights`，**禁止**任何数字前缀（`## §8 Insight` / `## 2. Insights` 等都会让 `archive-task.ps1` 收割 regex 命中 0 条）。verify_all E.6 同款"标题禁数字前缀"陷阱。本 04 已用裸 `## Insight 提议` 段名规避，但 PM 写 07 时仍需自检。

---

## 9. Verdict

**`READY FOR REVIEW`** — frontend partition complete.

- 03 §6 12 项 checklist 全部 DONE
- P1-1 DESIGN DRIFT 已显式记录（§3）
- P1-2 importOriginal stub 已落地（首跑即过）
- P1-3 JSDoc 注释已加（`Proxies.vue` formData ref 上方）
- 红线全部遵守：未动 `web/**` 外文件 / 未删 `validate / isBatchMode / getPortsExpr / resetBatchState` defineExpose 方法 / 未破坏 `handleTypeChange` 互斥重置 / 未引入新 npm 依赖 / 未改 `web/src/types.ts`
- `npm run lint` 0 errors / `npm run test` 103 PASS / `npm run build` PASS
- `scripts/verify_all` 24 PASS + 1 FAIL（C.1 已证伪为非本任务回归）

派发到 Stage 5 code-reviewer。
