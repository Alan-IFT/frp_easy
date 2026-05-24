# 05 — Code Review · T-032 proxy-form-vmodel-oom-fix

> **Mode**: full · **Stage**: 5 · **Verdict**: APPROVED
> **Owner**: code-reviewer · **Date**: 2026-05-24
> **Upstream**: 01 (READY) · 02 (READY) · 03 (APPROVED WITH CONDITIONS) · 04 (READY FOR REVIEW)

> **PM 落盘说明**：本会话 code-reviewer 实际收到的工具白名单仅含 `Read / Glob / Grep`，未含 Write —— T-030 frontmatter 修复在 SDK Opus 派发路径仍未生效（继 03 Stage 3 之后第 2 次本任务内复现，全局第 5-6 次累计复现）。PM 已将完整 review 内容代为落盘。建议下一个 trivial 任务做端到端工具白名单验证。

---

## 1. Files reviewed

- `web/src/composables/useProxyForm.ts` (73 行)
- `web/src/components/ProxyForm.vue` (411 行)
- `web/src/pages/Proxies.vue` (334 行)
- `web/src/components/__tests__/ProxyForm.spec.ts` (288 行，17 用例)
- `web/src/components/__tests__/qa_t007_adversarial.spec.ts` (72 行，4 用例)

跨核对的上游：01 / 02 / 03 / 04 文档全文 + `web/src/types.ts` L16-L25（ProxyInput 契约）+ `web/vitest.config.ts`（happy-dom 环境）。

---

## 2. 架构契约审计

### 2.1 双 watch + emit 桥已彻底删除 ✓

- `ProxyForm.vue` L141-L168：仅有 `defineProps<{ initialValue, editMode, existingProxy }>()` + `defineEmits<{ update:batchMode, update:portsExpr }>()`，**完全没有** `modelValue` prop、**完全没有** `update:modelValue` emit、**完全没有** `watch(form, ..., { deep: true })` / `watch(() => props.modelValue, ...)` 任一向。
- `Grep update:modelValue` 在 `web/src/` 命中仅限测试文件中的负向断言 (`expect(wrapper.emitted('update:modelValue')).toBeUndefined()`) 与注释；源码侧 0 命中。
- 干净度：✅ 没有"只删一边"。
- `defineModel` 反模式核查：`Grep defineModel` 仅命中 ProxyForm.vue L132-L133 的**禁用注释**，无任何实际 `defineModel(...)` 调用。

### 2.2 defineExpose 5 个方法完整 + 父组件类型签名同步 ✓

- `ProxyForm.vue` L395-L409 `defineExpose({ validate, isBatchMode, getPortsExpr, resetBatchState, getProxyInput })` —— 5 个全在。
- `Proxies.vue` L84-L90 `proxyFormRef = ref<{ validate / isBatchMode / getPortsExpr / resetBatchState / getProxyInput: () => ProxyInput }>()` —— 类型签名同步加 `getProxyInput`；`ProxyInput` 类型 L74 已 import。
- `vue-tsc --noEmit` 在 04 §5.3 实测 PASS，证明类型对齐生效。

### 2.3 handleSubmit 字段引用全部改为 formValue.xxx ✓

- `Proxies.vue` L168 `const formValue = proxyFormRef.value?.getProxyInput()`；L169-L172 null-check 存在并 `message.error('表单组件未就绪')` + `return`。
- 批量分支：`basename: formValue.name` / `type: formValue.type` / `localIP: formValue.localIP || '127.0.0.1'` / `enabled: formValue.enabled !== false` —— 4 处全部改完，与 02 §3.3 字面一致。
- 单条/编辑分支：`updateProxy(editingProxy.value.id, formValue)` / `createProxy(formValue)` —— 2 处改完。
- `Grep formData\.value` 在 Proxies.vue handleSubmit 内 0 命中 → ✅ 没有"漏改回老 ref"。

### 2.4 handleTypeChange 互斥重置语义保持（T-007 AC-9 红线） ✓

- `useProxyForm.ts` L32-L47 与 T-007 引入时完全一致：`tcp/udp → form.value.customDomains = []`；`http/https → form.value.remotePort = null`；L41-L47 `watch(() => form.value.type, ...)` 兜底。
- `ProxyForm.vue` L36 `@update:value="handleTypeChange"` 仍绑在 type select 上。
- 4 条对应 spec 用例全部 PASS（04 §5.2 实测）。

### 2.5 P1-3 JSDoc 注释 ✓

- `Proxies.vue` L106-L110 formData ref 上方 3 行 JSDoc 注释：包含「仅作为初始种子」「用户编辑期间它**不更新**」「禁止在 template / computed / 跨组件 prop 中绑此 ref 做实时显示」三要点。
- 反向扫描：`Grep formData` 在 Proxies.vue 中无 template `{{ }}` 实时绑定 → ✅ 无误用。

### 2.6 P2-1 / P2-2 文件头注释 ✓

- `ProxyForm.vue` L130-L140 块注释包含：
  - R-6 防御：`defineModel 的循环检测对此场景无效`
  - P2-1：`保留 update:batchMode / update:portsExpr emit：它们是子→父单向通知（不构成反馈环）`
  - P2-2：双路径 + 任务 ID 兜底，符合 insight L38

---

## 3. DESIGN DRIFT 核对

### DRIFT-1（已记录在 04 §3）：spec 改写半径

- 上界（P1-1 强制 ≥ 15）：ProxyForm.spec.ts 现状 **17 条** = 原 10 条保留 + 3 条改写 + 2 条新增 AC-1 / AC-7。≥ 15，未净减 ✅。
- `qa_t007_adversarial.spec.ts` L52-L57 保留 6 行注释指向 02 §10 R-3 → 后续 reviewer 可追溯。
- **code-reviewer 同意 Architect 的契约重解释**，不要求字面回退。理由：恢复 syncFromInput 死代码违反 ESLint 与 02 §3.2。

---

## 4. AC-1 / AC-7 + Naive UI stub 核对

### 4.1 vi.mock importOriginal 模式（P1-2） ✓

`ProxyForm.spec.ts` L11-L24：用了 `importOriginal` + spread `...actual` → 所有 N* 组件保留真实定义，仅 `useMessage` 被 stub；6 方法全列：error / success / warning / info / loading / destroyAll。与 03 §3 P1-2 字面给的代码骨架完全一致。

### 4.2 AC-1 用例（L257-L267） ✓

`mount(ProxyForm, { props: { initialValue: defaultSeed() } })` + 10 nextTick + 50ms + `expect(wrapper.emitted('update:modelValue')).toBeUndefined()`。上界从 RA "≤ 2/3" 加强为 undefined。

### 4.3 AC-7 用例（L269-L287） ✓

3 次 `wrapper.setProps({ initialValue })` 后 emit 上界 = 0；`update:batchMode` / `update:portsExpr` ≤ 1 兜底。比 RA 字面 "上界常数" 更严格（直接断 0）。

---

## 5. Reuse audit 复核（02 §8）

| Need | 02 §8 决策 | 实际状态 |
|---|---|---|
| `useProxyForm` form 状态 | 复用，保留全部 API 除 syncFromInput | ✅ L17-L72 完整保留；`syncFromInput` 已删 |
| `toProxyInput()` | 复用 | ✅ L49-L64 不动，被 `getProxyInput()` 调用 |
| `handleTypeChange` 互斥 | 复用，AC-9 红线 | ✅ L32-L47 不动 |
| 4 个原 defineExpose | 扩展第 5 个 | ✅ L395-L409 同构扩展 |
| Proxies.vue handleAdd/handleEdit | 复用为种子写入 | ✅ L113-L137 不动 |
| `update:batchMode` / `update:portsExpr` | 保留 | ✅ L165-L168 / L195-L202 不动 |
| @vue/test-utils + happy-dom | 复用 | ✅ ProxyForm.spec.ts L2 import mount |
| Naive UI form ref validate | 复用 | ✅ L396 defineExpose validate 不动 |
| 单向数据流（其它页面同构） | 复用 | ✅ 与 Wizard/Setup/Login/Server 等同模式 |

---

## 6. OOS 遵守

- `web/src/types.ts`（OOS-2）：未改，ProxyInput / Proxy 契约保持现状。
- `web/src/api/proxies.ts`：本任务未列入 04 §1 改动清单。
- `web/src/stores/proxies.ts`：本任务未列入 04 §1 改动清单。
- `web/src/composables/useProxyGrouping.ts` / `usePortPresets.ts`：未列入改动。
- 后端 `internal/**` / `cmd/**`：04 §1 改动 5 文件全在 `web/src/**`。

---

## 7. verify_all C.1 归责复核

- 04 §4.2 给出 `git stash` 暂存本任务 5 文件后裸跑 verify_all 同样 C.1 FAIL 的对照证据。
- C.1 是 `01-setup.spec.ts` TC-01/TC-02 失败，与 setup 流程 / 数据库 fixture 残留有关，与 ProxyForm.vue 物理无关联。
- 工作树同时含 T-031 进行中的 scripts/install.ps1 / verify_all.ps1 / scripts/baseline.json 改动。
- **code-reviewer 同意 04 的归责结论**：C.1 FAIL 是先存在的环境基线问题，非 T-032 引入。前端守门项 B.1-B.4 全 PASS，本 review 不卡此条。

---

## 8. 6 维度 Findings

### 8.1 Logic correctness — PASS

- 边界：`useProxyForm.ts` L21-L26 对 `initial.localIP / localPort / remotePort / customDomains / enabled / version` 均有 nullish / falsy 兜底，与 01 §5 boundary 表 9 条对齐。
- 错误路径：`Proxies.vue` L168-L172 `getProxyInput()` 返回 undefined 时 message.error + return。
- 并发：handleSubmit L165 `submitting.value = true` 配合按钮 `:loading="submitting"` 防重入。
- 无 off-by-one / null leak。

### 8.2 Requirement fidelity — PASS（见 §9 RA AC 覆盖表）

### 8.3 Design fidelity — PASS（见 §10 design drift 表）

### 8.4 Performance — PASS

- 删 watch + emit 反馈环本身就是性能修复，AC-1 / AC-7 上界=0 是确定性证明。

### 8.5 Security — PASS

- 表单字段验证规则（`ProxyForm.vue` L310-L391）不动，name 正则 / portsExpr 正则 / customDomains 域名正则 与 T-018 一致。

### 8.6 Maintainability — PASS

- 文件头块注释（R-6 / P2-1 / P2-2）+ formData ref JSDoc（P1-3）让未来 reviewer 不需要查 02 文档即可理解"为什么单向"。
- 删 syncFromInput + 双 watch 是"删代码 > 加代码"的可维护性正向变化。
- 命名清晰：`initialValue`（种子语义）比原 `modelValue`（双向语义）准确。

---

## 9. Requirement coverage check（RA AC 1-7 + FR 1-10）

| Criterion | Implementation | Status |
|---|---|---|
| AC-1（mount + emit 上界 ≤ 2/3） | `ProxyForm.spec.ts:257-267` (上界=0 更强) | ✅ |
| AC-2（手动 E2E < 5 MiB） | 设计层根除反馈环 | ✅（架构保证） |
| AC-3 / FR-3/4/5/6（增删改 emit 值） | `ProxyForm.spec.ts:116-192` + `Proxies.vue:174-208` | ✅ |
| AC-4（原 13 用例 PASS） | 17 用例（10 留 + 3 改写 + 2 新增），DRIFT-1 接受 | ✅ with DRIFT |
| AC-5（vue-tsc PASS） | 04 §5.3 PASS；`Proxies.vue:84-90` 类型签名同步 | ✅ |
| AC-6（verify_all PASS） | B.1-B.4 全 PASS；C.1 非本任务回归（§7） | ✅（C.1 已归责清楚） |
| AC-7（连续替换 initialValue 上界=常数） | `ProxyForm.spec.ts:269-287` 上界=0 | ✅ |
| FR-1/2/8（OOM 不复发） | 删 watch+emit 架构层根除 | ✅ |
| FR-7（type 切换互斥重置） | `useProxyForm.ts:32-47` + spec 用例 | ✅ |
| FR-9（defineExpose 4 方法保留） | `ProxyForm.vue:395-409` 4 原 + 1 新 = 5 | ✅ |
| FR-10（原 13 spec PASS） | 见 AC-4 行 | ✅ with DRIFT |

---

## 10. Design fidelity check（02 §3 / §6 / §8）

| Design item | Implementation | Status |
|---|---|---|
| `ProxyForm.vue` prop 改名 `initialValue` | `ProxyForm.vue:160` | ✅ |
| 删 `update:modelValue` emit | `ProxyForm.vue:165-168` 只有 batchMode / portsExpr | ✅ |
| 删两个双向 watch | 全文 grep 0 命中 | ✅ |
| `defineExpose` 加 `getProxyInput` | `ProxyForm.vue:408` | ✅ |
| `useProxyForm` 删 `syncFromInput` | 全文 0 命中 | ✅ |
| `Proxies.vue` template `:initial-value` | `Proxies.vue:35` | ✅ |
| `Proxies.vue` proxyFormRef 类型加 `getProxyInput` | `Proxies.vue:89` | ✅ |
| handleSubmit 6 处 `formData.value.xxx` 改 `formValue.xxx` | `Proxies.vue:178-183/203/206` 全改 | ✅ |
| P1-3 JSDoc | `Proxies.vue:106-110` | ✅ |
| P2-1 文件头保留 batchMode/portsExpr 说明 | `ProxyForm.vue:137-139` | ✅ |
| P2-2 路径双路径 / 任务 ID | `ProxyForm.vue:134-135` | ✅ |
| P1-2 vi.mock importOriginal 6 方法 | `ProxyForm.spec.ts:11-24` | ✅ |
| spec 总数 ≥ 15 | 17 条 ProxyForm.spec.ts + 4 条 qa_t007 | ✅ |

无 design drift（除已记录的 AC-4 字面 DRIFT-1）。

---

## 11. Findings 分级

### P0（阻塞）

**无**。

### P1（必须修，但本期已全部满足）

**无新增**。03 P1-1 / P1-2 / P1-3 三条已在 04 全部落地。

### P2（建议但不阻塞）

**P2-A**（NIT-tier）[LOGIC] `Proxies.vue:165-172`：
- `submitting.value = true` 后 early return（formValue undefined）依赖 finally 块（L220-L222）兜底重置。
- 实际正确，但代码阅读时容易引起"早 return 是否泄漏 submitting=true"的疑问。
- **非阻塞**，保持原状。

### NIT（纯偏好）

- **NIT-1** [STYLE]：handleEdit 中 `customDomains: ... ? [...] : []` widening — 原行为，本任务未引入。
- **NIT-2** [STYLE]：`(wrapper.vm as unknown as { getProxyInput: () => ProxyInput })` 三处重复 cast 字面 — 抽 helper ROI 低，保持现状。
- **NIT-3** [NAME]：未来若有人扩展再 rename `formData` → `formInitialSeed`（OOS-7 本期不做）。

---

## 12. Insight 候选（PM 在 07 交付时收割）

1. **sub-agent 工具白名单 frontmatter 在 SDK Opus 派发路径下未生效（高优先级）**：本任务 stage 3 + stage 5 两次同款复现 + 全局第 5-6 次累计复现。T-030 修复（frontmatter 加 Write 工具）需要更彻底的端到端验证。
2. **vitest + happy-dom mount Naive UI 组件 stub idiom 已落地为可复用范式**：importOriginal + 6 方法 stub 首跑即过。
3. **Vue 父子双向 v-model 桥 + composable toXxx 返回新对象 = OOM 反模式；defineModel 循环检测对此无效**：值得收割为前端架构 insight。
4. **verify_all 多任务并行工作树 fail 归责黄金动作 = git stash + 裸跑**：T-032 实测 4-5 分钟内完成归因。

---

## 13. Verdict

**`APPROVED`** — 0 P0 / 0 P1 / 1 P2 (非阻塞) / 3 NIT。

派发到 Stage 6 QA Validation。

### 通过依据

- 架构契约（双 watch + emit 桥 / defineModel 反模式）100% 清除。
- 03 §3 P1-1 / P1-2 / P1-3 三条强制条件全部满足。
- 02 §3 / §6 / §8 设计 13 项实施项全部对齐。
- AC-1 / AC-7 上界 = 0，比 RA 字面要求更强。
- T-007 AC-9 互斥重置红线未破。
- OOS-1/2/3/4/5/7 全部遵守。
- verify_all 前端守门项 B.1-B.4 全 PASS；C.1 已证伪为非本任务回归。

### Developer 动作清单

**无**。本期无 CHANGES REQUIRED。
