# 04 — 开发记录（Development Record）— Frontend partition

- 任务：T-057 `binary-missing-onboarding-ux`
- 模式：full
- Stage：4 / 7（dev-frontend）
- 上游：03 = APPROVED FOR DEVELOPMENT ✔

## Partition

dev-frontend — owns: `web/**`。

### Out-of-partition coordination

- `scripts/baseline.json`（bump 计数）与 `docs/dev-map.md`（Wizard 行注记）在 `web/**` 外，但属 harness 流程标准伴随物（baseline bump 是 insight L46 硬要求；dev-map 更新是 dev-frontend hard rule #5 明确要求）。PM 决策：作为流程伴随改动随本 partition 一并处理，不构成业务越界（不碰 Go / storage / migrations）。与历史任务（如 T-056 dev 改 baseline）一致。

## Files changed（this partition）

- `web/src/pages/Dashboard.vue` — 缺失提示 `n-alert` 文案对齐 AppLayout 顶栏横幅入口（IS-1/IS-2/IS-3）。纯 template 文本，无逻辑变更，无新增按钮。
- `web/src/pages/Wizard.vue` — 引入 `useAppStore`；新增 `binWarning` ref；完成分支注入 `await appStore.fetchReady()` + `missingForRole` 分流（缺失不自动跳 / 不缺失维持原行为）；step3 模板加缺失 warning alert + 「进入仪表盘」按钮；新增 `missingForRole` / `goToDashboard`；新增 `defineExpose({__testing})`。
- `web/src/pages/__tests__/Dashboard.spec.ts` — 新增 describe「二进制缺失提示引导（T-057）」4 用例（IS-1/IS-2/IS-3/BC-7）。
- `web/src/pages/__tests__/Wizard.spec.ts` — **新建**，13 用例（missingForRole 4 + 全就绪自动跳 2 + 缺失不静默跳 3 + 保存失败 1 + Adversarial 3）。
- `scripts/baseline.json` — frontend_tests 437→454，test_count 755→772。
- `docs/dev-map.md` — Wizard 行注记完成前二进制校验行为。

## 实现要点

1. **Dashboard 文案（IS-1/IS-2/IS-3）**：仅改 `n-alert` 内文本。首选引导"顶部横幅的「一键下载」/「手动上传」"，手动放置目录退为"网络与上传都不便时…后重启（兜底）"。删除旧句式"请将对应文件放置到…目录下后重启"。alert 内不新增任何按钮 / 不挂 UploadBinButton（避免与顶栏横幅重复）。
2. **Wizard 完成分支（IS-4/IS-5/IS-6/IS-7）**：`apiPutMode` + `completeWizard` 之后、跳转前 `await appStore.fetchReady()`（复用既有 action，内部吞错），再 `binWarning.value = missingForRole(selectedRole.value)`。`length > 0` → `completing=false`、不发 success、不 push（step3 渲染 warning + 手动按钮）；`length === 0` → 维持 `message.success('配置已保存，正在跳转...')` + `void router.push('/dashboard')`。
3. **missingForRole**：镜像 `modePayload` 的 frpc/frps/both 分支，取所选 need 集合与 `appStore.binMissing` 交集。`''` 返回 `[]`（安全兜底，但完成分支必非空）。
4. **binWarning 用 ref 定格快照**（非 computed，对应 03 Q1）：step3 已展示的警告不随后续 store.binMissing 漂移（Adversarial 第 3 例证伪）。
5. **goToDashboard**：`void router.push('/dashboard')`，供「进入仪表盘」按钮 @click。

## 03 conditions 消化（insight L17：顺手消化所有 C-N）

- **C-1（SFC 纯逻辑行数 < 200，insight L22 metric）**：实测 `Wizard.vue <script setup>`（L200-377）按"非 import 非空行 非注释 非 testing-hook(defineExpose __testing 块) 纯逻辑/声明行"计数约 **125 行**（含 frpsRules/frpcRules 数据声明、handleNext、missingForRole、goToDashboard、handleSkip），远低于 200 红线。本次新增纯逻辑约 16 行。**不做物理拆分**：handleNext 是 step 校验 + 保存 + 校验分流的数据流协调中枢，拆分会破坏数据流并失去 IDE 跳转可读性（与 LogViewer 同款论证，insight L22）。物理总行数（含模板）约 379，但 metric 判定按纯逻辑行，PASS。
- **C-2（断言 fetchReady 在校验前被调）**：Wizard.spec.ts「全就绪」用例显式 `expect(getReadyMock).toHaveBeenCalled()`（apiGetReady 是 fetchReady 内部唯一网络调用，被调 = fetchReady 被调），让"补 fetch 保证新鲜"决策可回归。

## e2e 影响预判（orchestrator 全量 verify_all 关注点）

- **01-setup.spec.ts TC-02**：仅断言"提交后离开 /setup"（允许跳 /dashboard 或 /wizard），不进入 Wizard step-2 完成流程 → 不触碰本次改动分支 → 不受影响。
- **03-dashboard.spec.ts TC-04/TC-05**：用 `bypassWizard(page)`（调 `/wizard/complete` API）绕过向导，根本不渲染 Wizard 组件 → 不受影响。
- **保险设计**：实现保证"二进制不缺失 → 维持原自动跳转 + success toast"。e2e 后端为真实 build（二进制嵌入或存在），`binMissing` 通常为空（不缺失）→ 即便有路径走到 Wizard 完成，也走自动跳转分支，与改动前行为字节一致。Dashboard 改动是纯文案，不改任何 e2e 断言文本（03-dashboard 断言"仪表盘/frpc（客户端）/frps（服务端）"，未断言缺失 alert 文案）。
- 结论：预判 e2e（01/03）全量 PASS 不受本次改动影响。

## verify_all result

- **PM 上下文限制（insight L14 role-collapse）**：本 dev-frontend stage 在 PM 上下文角色化运行，派发上下文工具被裁剪（无 Bash / PowerShell），**无法自跑 `scripts/verify_all`**。
- **静态自检（已完成）**：
  - eslint：无新增 `any`、无未用变量；`missingForRole` 返回 `string[]`、`goToDashboard` 返回 `void`；类型注解完整。
  - SFC 纯逻辑行数 < 200（C-1 实测 ~125）。
  - 测试范式严格照搬 Dashboard.spec.ts（naive-ui mock + useMessage 单例 spy + getExposed + apiError + vue-router push spy），api 层 mock 齐全，binMissing 经 `api/system.apiGetReady` mock 控制（fetchReady 真实执行）。
  - 新增前端测试 17 个（Dashboard +4 / Wizard +13）；baseline 同步 bump 至 frontend_tests=454 / test_count=772（insight L46 硬要求）。
  - `## Adversarial tests` 段在 06 用裸标题（insight L40），04 不涉及。
- **真跑硬闸门**：交 orchestrator 独立真跑 `bash scripts/verify_all.sh`（全量含 e2e），作为声明完成的硬闸门（insight L14/L46：batch orchestrator 必须自己真跑，不信角色扮演 QA）。

## Verdict

**READY FOR REVIEW**（frontend partition complete；verify_all 全量真跑交 orchestrator 硬闸门，静态预测全绿）。
