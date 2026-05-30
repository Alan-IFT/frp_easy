# 02 — 方案设计（Solution Design）

- 任务：T-057 `binary-missing-onboarding-ux`
- 模式：full
- Stage：2 / 7（solution-architect）
- 上游 verdict：01 = READY ✔

## 1. Architecture summary

纯前端、纯展示层改动。两处：(A) 改 `Dashboard.vue` 缺失提示 `n-alert` 的静态文案，把"手动拷贝到目录后重启"的硬核首选路径，改写为"用顶部横幅一键下载/手动上传"为首选 + "或手动放置…后重启"为兜底，与 `AppLayout.vue` 顶栏横幅已有的入口（一键下载 + UploadBinButton）信息架构对齐。(B) 改 `Wizard.vue` step 2 的 `handleNext` 完成分支：保存配置 + 开启模式成功后、跳转前，`await appStore.fetchReady()` 刷新 `binMissing`，按所选角色计算缺失集合；缺失 → 进 step 3 展示就地 warning + 手动"进入仪表盘"按钮、不自动跳；不缺失 → 维持现有 success toast + 自动跳转。无后端、无 store、无 API、无路由守卫改动。

## 2. Affected modules

| 文件 | 改动 |
|---|---|
| `web/src/pages/Dashboard.vue` | 改 `n-alert`（二进制文件缺失）内文案（template，无逻辑变更） |
| `web/src/pages/Wizard.vue` | `handleNext` step 2 完成分支加二进制校验分流 + step 3 模板加 warning alert + "进入仪表盘"按钮 + 新 reactive 状态 + 引入 `useAppStore` |
| `web/src/pages/__tests__/Dashboard.spec.ts` | 加 IS-1/IS-2/IS-3 文案与无按钮断言（少量） |
| `web/src/pages/__tests__/Wizard.spec.ts` | **新建**：完成流程缺失/不缺失分支、fetchReady 调用、手动跳转、Adversarial |
| `scripts/baseline.json` | bump `frontend_tests` + `test_count` |
| `docs/dev-map.md` | Wizard 行追加"完成前校验所选角色二进制就绪（缺失则就地警告 + 手动跳转）" |

`web/src/stores/app.ts`：**不改**（复用既有 `fetchReady` action + `binMissing` state + `frpcMissing`/`frpsMissing` getter）。

## 3. Module decomposition（无新模块）

无新增组件/模块。Wizard.vue 内新增的逻辑全部内联于既有 `<script setup>`，新增项：

- state：
  - `binWarning = ref<string[]>([])` — 完成校验后所选角色缺失的二进制 kind 列表（空 = 不缺失）。
- computed / helper：
  - `missingForRole(role): ('frpc'|'frps')[]` — 纯函数，按角色返回应检查的 kind 与 binMissing 的交集。
- 改造 `handleNext` 完成分支（见 §6 flow）。
- 新增 `goToDashboard()` — `router.push('/dashboard')`，供"进入仪表盘"按钮调用。

## 4. Data model changes

无。

## 5. API contracts

无新增/变更。复用：
- `apiPutServer` / `apiPutClient` / `apiPutMode`（Wizard 现有调用，payload 不变）。
- `appStore.fetchReady()` → 内部 `apiGetReady()`（`/api/v1/system/ready`），返回 `{ initialized, binMissing, version }`；`fetchReady` 已 try/catch 吞错（`stores/app.ts:25-35`），失败时 binMissing 维持原值、不抛。

## 6. Sequence / flow（Wizard 完成分支，改造后）

```
handleNext (currentStep === 2):
  validate forms → 失败 return（不变）
  submitting = true
  try:
    apiPutServer (if frps/both)        ── 不变
    apiPutClient (if frpc/both)        ── 不变
    apiPutMode(modePayload)            ── 不变
    await wizardStore.completeWizard() ── 不变（best-effort try/catch）
    currentStep = 3                    ── 进入 step 3

    // ▼ 新增：刷新 + 按角色校验
    await appStore.fetchReady()        // 吞错，不抛；保证 binMissing 新鲜
    binWarning.value = missingForRole(selectedRole.value)

    if (binWarning.value.length > 0):
      // IS-5 缺失分支：不自动跳、不发"正在跳转"toast
      // step 3 模板按 binWarning.length>0 渲染 warning alert + "进入仪表盘"按钮
      completing = false   // 不显示"正在跳转"spin
    else:
      // IS-6 不缺失分支：维持原行为
      completing = true
      message.success('配置已保存，正在跳转...')
      void router.push('/dashboard')
  catch (e):
    configError = extractErrorMessage(...)   ── 不变
  finally:
    submitting = false
```

`missingForRole`：
```
function missingForRole(role): ('frpc'|'frps')[] {
  const need = role === 'both' ? ['frpc','frps']
             : role === 'frpc' ? ['frpc']
             : role === 'frps' ? ['frps'] : []
  return need.filter(k => appStore.binMissing.includes(k))
}
```

step 3 模板（条件渲染）：
- `binWarning.length === 0`：维持现有"配置完成！…现在跳转到仪表盘" + spin（completing 时）。
- `binWarning.length > 0`：展示 warning alert（列出 `binWarning` 的 frpX）+ 引导文案（顶部横幅一键下载/手动上传）+ "进入仪表盘"按钮（点击 → `goToDashboard()`）。

## 7. Reuse audit

| Need | Existing code | File path | Decision |
|---|---|---|---|
| 刷新二进制缺失状态 | `appStore.fetchReady()` | `web/src/stores/app.ts:25` | 复用原样（内部吞错） |
| 缺失集合数据源 | `appStore.binMissing` | `web/src/stores/app.ts:6` | 复用 |
| 角色→kind 映射 | （Wizard 内 `modePayload` 已有同款 frpc/frps/both 分支逻辑，`Wizard.vue:283-286`） | `web/src/pages/Wizard.vue` | 镜像同款分支写 `missingForRole` |
| warning alert UI | `n-alert type="warning"`（Dashboard 缺失提示同款；AppLayout 横幅同款） | `web/src/pages/Dashboard.vue:9` / `AppLayout.vue:11` | 复用组件 |
| 路由跳转 | `router.push('/dashboard')` | `web/src/pages/Wizard.vue:299` | 复用，仅时机改为按钮触发 |
| 测试 mock 范式 | naive-ui mock + useMessage 单例 spy + getExposed + apiError + push spy | `web/src/pages/__tests__/Dashboard.spec.ts` | Wizard.spec.ts 照搬 |
| 顶栏下载/上传入口 | 一键下载按钮 + UploadBinButton | `web/src/components/AppLayout.vue:24-52` | **不重复造**（OOS-1/IS-3），文案指向它 |

## 8. Risk analysis

- **R-1：Wizard.vue 行数逼近 200 行红线**。当前 `<script setup>` 约 148 行（含较多 form rules）。新增逻辑约 15-20 行纯逻辑。
  - 缓解：按 insight L22，红线按"script 段非 import 非 testing hook 纯逻辑行数"判定。新增后纯逻辑行数（含 frpsRules/frpcRules 数据声明、handleNext、handleSkip、missingForRole、goToDashboard）经核算仍 < 200。04 须实测列出纯逻辑行数。若逼近，按 metric 判定 + 在 04 落 justify，不做物理拆分（拆 form 校验中枢会破坏数据流，与 LogViewer 同款论证）。
- **R-2：fetchReady 失败导致 binMissing 过期**。
  - 缓解：`fetchReady` 吞错（已有），失败时 binMissing 维持调用前值（router.beforeEach 进 wizard 前已 fetch 过一次，至少有进入向导时的值）。降级到"用上次已知值判断"，不崩、不阻断保存。BC-5 覆盖。
- **R-3：破坏 e2e（01-setup / 03-dashboard）**。
  - 缓解：已核实——01-setup TC-02 仅断言离开 /setup（不进 Wizard 完成）；03-dashboard 用 `bypassWizard`（调 wizard/complete API）绕过向导组件。二者均不走 Wizard 完成分支。且实现保证"不缺失→自动跳转不变"，e2e 后端二进制存在（不缺失）→ 即便走到也维持原行为。零影响。
- **R-4：把"与所选角色无关的缺失"误报为警告**（如选 frpc 但缺 frps）。
  - 缓解：`missingForRole` 只取所选角色 need 集合与 binMissing 的交集，BC-3 / AC-5 反向证伪。
- **R-5：Dashboard 文案改动误删 v-if 或破坏既有 alert 结构**。
  - 缓解：仅改 alert 文本内容，不动 `v-if` / type / title / 外层结构；AC-2 断言 alert 内无按钮。

## 9. Migration / rollout plan

无数据迁移。纯前端文案 + 逻辑分支。向后兼容：不缺失路径行为字节不变（自动跳转 + toast），仅缺失路径行为变化（改手动跳 + 警告）。回滚 = git revert 两文件。

## 10. Out-of-scope clarifications

- 不改后端、store、API、路由守卫、wizard store、AppLayout、Wizard step 1/2 表单逻辑（与 01 §3 OOS 一致）。
- 不在 Wizard 内造下载/上传按钮（IS-3 / OOS-1）。
- 设计不覆盖"向导内直接下载二进制"——刻意把下载/上传留在仪表盘横幅（信息架构单一入口原则）。

## 11. Partition assignment

`.harness/agents/dev-*.md` 存在 → 必填。本任务纯前端，单 partition。

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `web/src/pages/Dashboard.vue` | dev-frontend | edit（alert 文案） | — |
| `web/src/pages/Wizard.vue` | dev-frontend | edit（完成校验 + step3 警告 + 状态） | — |
| `web/src/pages/__tests__/Dashboard.spec.ts` | dev-frontend | edit（加文案/无按钮断言） | depends on Dashboard.vue |
| `web/src/pages/__tests__/Wizard.spec.ts` | dev-frontend | new（完成流程缺失/不缺失 + Adversarial） | depends on Wizard.vue |
| `scripts/baseline.json` | dev-frontend | edit（bump 计数） | depends on 新增测试数 |
| `docs/dev-map.md` | dev-frontend | edit（Wizard 行注记） | — |

### Dispatch order

1. dev-frontend（唯一）

### Parallelism

None — 单 partition。

## 12. Verdict

**READY** — 设计完整，可进入 stage 3（gate-reviewer）。无需上游回滚。
