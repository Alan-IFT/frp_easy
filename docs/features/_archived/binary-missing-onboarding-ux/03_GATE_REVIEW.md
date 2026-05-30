# 03 — 闸门评审（Gate Review）

- 任务：T-057 `binary-missing-onboarding-ux`
- 模式：full
- Stage：3 / 7（gate-reviewer）
- 上游 verdict：01 = READY ✔，02 = READY ✔

## 独立代码核实（不信任上游，逐符号 grep/read）

| 设计引用 | 文件:行 | 核实结果 |
|---|---|---|
| Dashboard 缺失 alert `v-if="appStore.binMissing.length>0"` | `Dashboard.vue:9-17` | ✔ 存在，文案确实只导向手动拷贝 |
| AppLayout 顶栏横幅含一键下载 + UploadBinButton | `AppLayout.vue:11-66`（按钮 L24-52） | ✔ 存在，文案"网络不便时可手动上传" |
| Wizard 顶级路由 /wizard 不在 AppLayout 内 | `router.ts:10`（vs `/` AppLayout L11-26） | ✔ 平级，向导阶段顶栏不可见 — 设计前提成立 |
| binMissing 在进入 wizard 前已 fetch | `router.ts:40` `await app.fetchReady()` | ✔ beforeEach 任何导航前 fetch |
| `appStore.fetchReady()` 吞错不抛 | `stores/app.ts:25-35` try/catch | ✔ 复用安全，失败维持原值 |
| `appStore.binMissing` / `frpcMissing` / `frpsMissing` | `stores/app.ts:6,20,21` | ✔ 存在 |
| Wizard 完成分支 `handleNext` step 2 / modePayload | `Wizard.vue:248-306`（modePayload L283-286） | ✔ 角色分支可镜像 |
| Wizard `message.success('配置已保存，正在跳转...')` + `router.push` | `Wizard.vue:298-299` | ✔ 存在，待按缺失分流 |
| Wizard step 3 模板（完成态） | `Wizard.vue:130-144` | ✔ 可加条件渲染 warning alert |
| e2e 01-setup 不进 Wizard 完成 | `tests/e2e/01-setup.spec.ts:22` 仅断言离开 /setup | ✔ |
| e2e 03-dashboard 用 bypassWizard | `tests/e2e/03-dashboard.spec.ts:9,19` | ✔ 绕过 Wizard 组件 |
| Dashboard.spec.ts 测试范式 | `pages/__tests__/Dashboard.spec.ts` | ✔ naive-ui mock + useMessage 单例 spy + getExposed + apiError + vue-router push spy 齐全，Wizard.spec.ts 可照搬 |
| test-utils getExposed / apiError | `test-utils/exposed.ts` / `apiError.ts` | ✔ 存在（insight L45） |
| frontend_tests 口径 | `vitest.config.ts:9` exclude tests/e2e | ✔ `vitest run` 全量计数；baseline=437 |

无任何"引用不存在的符号"问题。

## 1. Audit checklist（8 维）

| # | 维度 | 结论 | 一句话理由 |
|---|---|---|---|
| 1 | Requirement completeness | PASS | IS-1~IS-7 均可测；缺失判定语义（per-role 交集）、跳转行为（缺失手动/不缺失自动）明确无歧义 |
| 2 | Design completeness | PASS | 02 §6 flow 覆盖全部 IS；缺失/不缺失双分支 + missingForRole + step3 条件渲染齐全 |
| 3 | Reuse correctness | PASS | 复用 fetchReady/binMissing/n-alert/push 全部经代码核实存在；不重复造下载上传按钮（IS-3）正确 |
| 4 | Risk coverage | PASS | R-1（行数）/R-2（fetch 失败）/R-3（e2e）/R-4（误报）/R-5（文案误删结构）覆盖真实风险，均带缓解 |
| 5 | Migration safety | PASS | 无数据迁移；不缺失路径行为字节不变（向后兼容），回滚=git revert |
| 6 | Boundary handling | PASS | BC-1~BC-7 覆盖空集/部分缺失/无关缺失/全缺失/fetch 失败/保存失败/alert 不渲染 |
| 7 | Test feasibility | PASS | AC-1~AC-9 每条可断言；push spy 验证"未自动跳"、getExposed 读 binWarning/goToDashboard 句柄 |
| 8 | Out-of-scope clarity | PASS | OOS-1~OOS-6 明确；Partition 单 dev-frontend、改动文件清单封闭防扩散 |

## 2. Findings（WARN / FAIL）

无 FAIL。无阻塞性 WARN。

记录 2 条**非阻塞建议**（developer 顺手消化，不阻塞 stage 4，参 insight L17）：

- **C-1（建议，对应 R-1）**：04 必须实测并在文档列出 Wizard.vue `<script setup>` 的"非 import 非 testing hook 纯逻辑行数"（insight L22 metric），确认 < 200。若逼近，按 metric justify、不做物理拆分（拆 form 校验中枢破坏数据流，与 LogViewer 同款论证）。
- **C-2（建议，对应 AC-6/IS-7）**：04 的 Wizard.spec.ts 应显式断言"完成流程在校验前调用了 `appStore.fetchReady`"（spy on store action 或 mock `api/system`.apiGetReady 计数），让"补 fetch 保证新鲜"这一设计决策可回归验证，而非仅靠 binMissing 值间接体现。

## 3. High-probability questions during development（预答）

- **Q1：binWarning 用 ref 数组还是 computed？** → 用 `ref<string[]>`（完成时一次性快照），不用 computed——因为它是"完成那一刻所选角色的缺失结论"，不应随后续 binMissing 响应式变化而改变 step3 已展示的警告（语义是定格）。02 §3 已定为 ref，正确。
- **Q2：缺失分支要不要也 `completing=true` 显示 spin？** → 不要。缺失分支不跳转，spin（"正在跳转"语义）会误导。02 §6 已写 `completing=false`，正确。
- **Q3：Dashboard alert 文案改后，T-056 现有 Dashboard.spec 会不会断言旧文案而挂？** → 现有 Dashboard.spec.ts 不断言该 alert 文案（grep 确认无"请将对应文件放置"相关断言），只测 mode/proc/确认状态机。改文案安全；新增 IS-1/IS-2/IS-3 断言为增量。
- **Q4：missingForRole 对 `selectedRole===''`（空）怎么办？** → 完成分支只在 step2 通过校验后到达，此时 selectedRole 必非空（step1 已挡空，`Wizard.vue:240-243`）。missingForRole 对 '' 返回 []（need 为空数组），安全兜底。
- **Q5："进入仪表盘"按钮点击后要不要清 binWarning？** → 不必，点击即 `router.push('/dashboard')` 离开向导组件，组件卸载。保持简单。

## 4. Verdict

**APPROVED FOR DEVELOPMENT**

（full mode 等价 `APPROVED`；用此串以兼容 plan→full resume 语义。设计可直接进入 stage 4，dev-frontend 实现。C-1/C-2 为非阻塞建议，developer 顺手消化。）
