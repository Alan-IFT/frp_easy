# 07 — Delivery · T-032 proxy-form-vmodel-oom-fix

> **Mode**: full · **Stage**: 7 · **Verdict**: DELIVERED (with C.1 pre-existing E2E flake noted)
> **Owner**: pm-orchestrator · **Date**: 2026-05-24

---

## 1. 一句话交付

WebUI 「新增代理规则 / 编辑规则」对话框打开时的 **Out of Memory** 死循环已在架构层根除：删除 `ProxyForm.vue` 的双向 `v-model` 反馈环（双 deep watch + emit `update:modelValue`），改为**单向数据流**——`Proxies.vue` 仅在模态框开启时把 `formData` 注入子组件作为「初始种子」（`:initial-value`），子组件内部用 `useProxyForm` composable 管理工作态，**提交时**父组件通过 `proxyFormRef.value?.getProxyInput()` 主动拉取最终值。

## 2. 修了什么 / 用户能看到的变化

| 项 | 修复前 | 修复后 |
|---|---|---|
| 打开「新增规则 / 批量新增」对话框 | 一段时间后浏览器卡死 → "Out of Memory" 错误 | 立即响应，CPU/内存平稳；可正常输入、提交 |
| 编辑现有规则 | 同上（同组件路径） | 同上，可正常编辑 |
| 批量新增（端口表达式） | 同上 | 同上 |
| 测试覆盖 | 13 条 composable unit + 5 条 adversarial = 18 | 17 条 ProxyForm.spec + 4 条 qa_t007 + 7 条 qa_t032 = **28 条**；其中 AC-1 / AC-7 用 emit 表常数上界断言守门，未来反馈环回归会被即时捕获 |
| 架构本质 | 父 ↔ 子 v-model 双向桥（每次 form 变化 emit 新对象 → 父回写 prop → 子 watch deep 触发 sync → 写新数组引用 → 子 form watch 再 emit → ♾️） | 单向数据流（父写一次种子 → 子内部独立管理 → 父显式拉取）。删 watch + emit 的硬路径不存在反馈环。 |

## 3. 改了哪些文件

| 文件 | 变化 | LOC 净 |
|---|---|---|
| `web/src/composables/useProxyForm.ts` | 删 `syncFromInput` 函数 + return 清单 | −12 |
| `web/src/components/ProxyForm.vue` | 删两个双向 watch + `emit('update:modelValue', ...)`；prop `modelValue` 改名 `initialValue`；`defineExpose` 加 `getProxyInput()`；加 T-032 防御文件头注释（R-6 / P2-1 / P2-2） | +3 |
| `web/src/pages/Proxies.vue` | template `v-model` → `:initial-value`；`proxyFormRef` 类型加 `getProxyInput: () => ProxyInput`；`handleSubmit` 改读 `proxyFormRef.value?.getProxyInput()`（含 null-check）；`formData` ref 上方加 JSDoc 注释禁用 template 实时显示绑定 | +12 |
| `web/src/components/__tests__/ProxyForm.spec.ts` | 加 `vi.mock('naive-ui', importOriginal)` stub `useMessage`；3 条 `syncFromInput` 直调用例改写为 `mount + initialValue + getProxyInput` 等价断言；新增 AC-1 / AC-7 mount + emit 上界断言用例（≥ 15 下界） | +80（含 stub 与注释） |
| `web/src/components/__tests__/qa_t007_adversarial.spec.ts` | 删 "syncFromInput 是原子的" 用例（前提"双 watch 双向桥"已不存在），保留注释指向 02 §10 R-3 | −16 |
| `web/src/components/__tests__/qa_t032_adversarial.spec.ts` | **新增**（QA 独立编写 7 用例）：1000 次 setProps emit=0 / FR-3 TCP / FR-4 HTTP customDomains / FR-5 FR-7 type 切换边界 / boundary 用例 | +167 |
| `scripts/baseline.json` | v14 → v15；`frontend_tests` 102 → 110；`test_count` 367 → 375；notes 补 T-032 | 元数据 |

非测试代码净变化约 **−5 LOC**（删反模式 > 加 JSDoc 与防御注释）。

## 4. 决策回放

| 决策 | 选择 | 否决 |
|---|---|---|
| `defineModel` 宏（Vue 3.4+ 官方推荐双向 idiom） | **否决** | `defineModel` 的循环检测是值相等比较（`!==`），但 `toProxyInput()` 每次返回**全新对象字面量** → `!==` 始终为 true → 循环检测对当前场景**无效**。即使能 work，下次有人改 toProxyInput 返回新引用 OOM 又回来。架构层未根治。 |
| 双向桥 + guard 标志位补丁 | **否决** | RA §10.2 明令禁止；imperative guard 是经典反模式，新加 watch 或异步分支会漏；vitest 难以证全。 |
| 单向数据流 + 父侧 `getProxyInput()` 拉取 | **采用** | 架构上根除反馈环，物理不可能复发 OOM；与已有 4 个 `defineExpose` 方法同构（自然演化）；删代码 > 加代码；测试更易写（emit 表常数上界）；与其它页面（Wizard/Setup/Login/Server）单向模式一致。 |

## 5. 验证

### 5.1 verify_all 最终结果（PM 复跑）

```
=== verify_all (fullstack) ===
[A.1] No hardcoded secrets ........................... PASS
[A.2] No .env files committed ........................ PASS
[A.3] TODO / FIXME budget (warn only) ................ PASS
[G.1] go vet ......................................... PASS
[G.2] go test ./... .................................. PASS
[G.3] go build ./cmd/frp-easy ........................ PASS
[B.1] Install / typecheck ............................ PASS   ← 含 vue-tsc，AC-5 / R-2 类型守门生效
[B.2] Lint ........................................... PASS
[B.3] Unit tests pass ................................ PASS   ← 110/110 frontend 全过
[B.4] Test count >= baseline ......................... PASS   ← baseline v15 = 110
[B.5] No tsc residue in web/src/ ..................... PASS
[C.1] E2E smoke (playwright) ......................... FAIL   ← 见 §5.2
[D.1] OpenAPI / tRPC schema present .................. PASS
[E.1] CLAUDE.md present .............................. PASS
[E.2] workflow.md present ............................ PASS
[E.3] All 7 agent definitions present ................ PASS
[E.4] Binding in sync (.harness/ -> .claude/) ........ PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md .. PASS
[E.6] Adversarial tests section present .............. PASS   ← 06 §Adversarial tests 裸标题命中
[E.7a / E.7b / E.7c] PS1 BOM 三段 .................... PASS
[E.8 / E.9 / E.10] T-031 install 守门三段 ............ PASS

=== Summary ===
  PASS: 24
  WARN: 0
  FAIL: 1  ← 仅 C.1
  SKIP: 0
```

### 5.2 C.1 失败 — 归责非本任务（强证据链）

C.1 是 `tests/e2e/01-setup.spec.ts` TC-01/TC-02 失败：测试期望未初始化时跳转 `/setup`，实际跳到 `/login`。

**归责证据链**：
1. **物理无关联**：T-032 改动全部在 `web/src/components/ProxyForm.vue` / `web/src/pages/Proxies.vue` / `web/src/composables/useProxyForm.ts` 三处；E2E setup 测试覆盖 `web/src/pages/Setup.vue` / `web/src/pages/Login.vue` / 后端 `internal/httpapi` setup endpoint。三组文件**零交集**（grep 验证 `tests/e2e/01-setup.spec.ts` 内不引用任何 ProxyForm / useProxyForm 路径）。
2. **Dev §4.2 git stash 对照**：dev-frontend agent 在 04 §4.2 报告 `git stash` 暂存 T-032 5 文件后裸跑 verify_all 同样 C.1 FAIL —— 改动前后行为完全相同。
3. **QA §5.3 git stash 复测**：qa-tester 独立复测，同样结论。
4. **T-031 已归档后仍 FAIL**：PM 在 T-031 delivered（tasks.md 已归档）之后再次复跑，C.1 仍 FAIL —— 排除 T-031 进行中工作树污染假设，确认是**长期存在的 E2E flake / 环境依赖问题**（可能是 Playwright webServer 启动竞态、data.db fixture 残留、auth 状态泄漏等）。
5. **前端守门项 B.1-B.5 全 PASS** —— T-032 实际改动的代码层无回归。

**结论**：C.1 失败**不是 T-032 引入**；本任务交付不修该 fail。PM 建议立即创建 T-033 follow-up 任务专门调查 C.1 根因（建议 stage 1 用 `/harness-explore` 调研可行性，避免 T-032 scope creep）。

### 5.3 OOM 修复的"用户角度"验证

由于 OOM 是定性现象（浏览器卡死），按 RA AC-2 用代理证据链证明：

1. **架构层根除**：`Grep update:modelValue web/src/` 在生产源码 0 命中 → 反馈环硬路径删除。
2. **AC-1 / AC-7 上界 = 0**：1000 次 setProps（QA qa_t032 §1） + 3 次 setProps（dev AC-7）后 emit 计数仍 = 0 → 父侧任何引用 churn 都不会触发回流。
3. **稳定性 10 连跑零 flake**：ProxyForm.spec.ts + qa_t032 共 24 用例 10 次跑全过。

## 6. RA 验收准则覆盖

| AC | 状态 | 证据 |
|---|---|---|
| **AC-1** mount + emit 上界 ≤ 2/3 | ✅ 超额完成（上界 = 0） | `ProxyForm.spec.ts:257-267` + `qa_t032:42-50` |
| **AC-2** 手动 E2E < 5 MiB | ✅（代理证据） | 见 §5.3 |
| **AC-3** 单条 / 批量 / 编辑路径正确 | ✅ | ProxyForm.spec / qa_t032 8+ 用例覆盖 |
| **AC-4** 原 13 spec PASS（带 DRIFT-1 重解释） | ✅ with DRIFT | 17 用例 ≥ 15 下界；DRIFT-1 已在 04 §3 / 05 §3 / 06 §8 三方接受 |
| **AC-5** vue-tsc PASS | ✅ | `npm run build` PASS；`qa-tester` 主动证伪：临时删 `getProxyInput` 类型字段 vue-tsc 立刻 TS2339 fail，恢复后 EXIT=0 |
| **AC-6** verify_all PASS | ⚠️ 24/1 with C.1 归责非本任务 | 见 §5.2 |
| **AC-7** 连续替换 initialValue 不进入循环 | ✅ 超额（1000 次仍 = 0） | `qa_t032:42-50` |

**FR-7（type 切换互斥重置）/ FR-9（defineExpose 4 方法保留）/ FR-10（原 spec PASS）** 全 ✅。

## 7. 阶段链回顾

| Stage | Agent | 输出 | Verdict |
|---|---|---|---|
| 1 | requirement-analyst | `01_REQUIREMENT_ANALYSIS.md` | READY |
| 2 | solution-architect | `02_SOLUTION_DESIGN.md`（推荐方案 B 单向数据流，否决 defineModel + guard） | READY |
| 3 | gate-reviewer | `03_GATE_REVIEW.md`（PM 代落盘） | APPROVED WITH CONDITIONS（P0=0 / P1=3） |
| 4 | dev-frontend | `04_DEVELOPMENT.md` + 实施 5 文件改动 | READY FOR REVIEW |
| 5 | code-reviewer | `05_CODE_REVIEW.md`（PM 代落盘） | APPROVED（0 P0 / 0 P1 / 1 P2 / 3 NIT） |
| 6 | qa-tester | `06_TEST_REPORT.md` + 新增 `qa_t032_adversarial.spec.ts` 7 用例 + baseline v15 | APPROVED FOR DELIVERY |
| 7 | pm-orchestrator (本文件) | `07_DELIVERY.md` + verify_all 复跑 + archive | DELIVERED |

## 8. 残留 / Follow-up

### 8.1 C.1 E2E flake（建议 T-033）

C.1 在 T-031 archived 后仍持续 FAIL，归责为长期环境基线问题。建议下一个任务 `/harness-explore` 调研：
- 是 Playwright webServer 启动竞态？（dev server 启动后还需要 frpc/frps binary 下载完成）
- 是 SQLite data.db 在多次 E2E 跑之间未清理？
- 是后端 setup endpoint 在已初始化时不再跳转 `/setup`？
- 是否需要在 E2E 中加 setup 重置 fixture？

### 8.2 reviewer "不落盘" 陷阱（建议 T-034）

本任务 stage 3 + stage 5 两次 reviewer 把 review 内容塞消息体让 PM 代为落盘，是 insight L41/L44/L48/L50 第 5-6 次累计复现。T-030 frontmatter 加 Write 工具的 fix 显然在 SDK Opus 派发路径未生效。建议下一个 trivial 任务做端到端工具白名单验证（找一个简单的 reviewer 派发，断言 reviewer 实际是否能 Write）。

## Insight

- **2026-05-24** · **Vue 父子双向 v-model 桥 + composable `toXxx()` 每次返回新对象 = OOM 反馈环高危反模式**。`defineModel` 宏的循环检测是值相等比较（`if (value !== modelValue.value) emit(...)`），新对象字面量永远 `!==`，**对此场景无效**。唯一可靠根治路径：单向数据流（父侧 ref 写种子 + 子组件 setup 时读一次 + 父侧 `defineExpose getXxxInput()` 主动拉取）。架构层根除，物理不可能复发。Vitest 用 `mount + setProps + emit 表常数上界`（最理想上界 = 0，因为已删 emit）守门未来回归。证据：T-032 02 §7 决策矩阵 + 03 §3 P1-2 + 04 §3 全实施。

- **2026-05-24** · **vitest + happy-dom mount 含 `useMessage` / `useDialog` / `useNotification` 的 Naive UI 组件**必须用 `vi.mock('naive-ui', async (importOriginal) => { const actual = await importOriginal<typeof import('naive-ui')>(); return { ...actual, useMessage: () => ({ error: vi.fn(), success: vi.fn(), warning: vi.fn(), info: vi.fn(), loading: vi.fn(), destroyAll: vi.fn() }) } })` 的 **importOriginal + spread + 6 方法 stub** 模式。直接整个 `vi.mock('naive-ui', ...)` 不带 importOriginal 会让所有 N* 组件定义丢失 → render 失败。这是 insight L9 (NMessageProvider 必须在 App.vue) 在测试侧的镜像 idiom：运行时挂 provider；测试时不挂 App.vue 必须 stub `useMessage`。本任务首次引入 mount-level 测试即建立此可复用范式。证据：T-032 03 §3 P1-2 + 04 §4.1 首跑即过未踩 02 §13.4 字面"render 失败"调试坑。

- **2026-05-24** · **verify_all 在多任务并行进行的工作树中"非本任务 fail" 归责黄金动作 = `git stash` 暂存窄路径文件 → 裸跑 verify_all → 对照 Summary 数字**。本任务 dev 阶段 + QA 阶段 + PM 阶段 3 次独立用此动作证伪"C.1 是 T-032 引入"假设，4-5 分钟内完成归因，避免 reviewer 误归责。改进版：归档后再复跑一次确认非"旁路任务工作树污染"，而是"长期环境基线问题"。证据：T-032 04 §4.2 / 06 §5.3 / 07 §5.2 三处独立 git stash 对照 + T-031 归档后 C.1 仍 FAIL 的终极证据。

- **2026-05-24** · **sub-agent 工具白名单 frontmatter `tools: Read, Write, Glob, Grep` 在 SDK Opus 派发路径下可能未生效** —— 本任务 stage 3 gate-reviewer + stage 5 code-reviewer 两次同款复现"reviewer 拿到的工具集仅 Read/Glob/Grep，把 review 内容塞消息体让 PM 代为落盘"。T-030 frontmatter fix 已同步到 `.claude/agents/*.md`，但 SDK 派发解析路径不明。这是 insight L41/L44/L48/L50 第 5-6 次累计复现，需要 T-034 trivial 任务做端到端工具白名单验证。短期 workaround：派发 reviewer 时显式预告"若工具集无 Write，按 fallback 模式塞消息体让 PM 代写"，避免 reviewer 反复检查与 narrative。
