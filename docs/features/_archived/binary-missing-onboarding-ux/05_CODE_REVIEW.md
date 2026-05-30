# 05 — 代码评审（Code Review）

- 任务：T-057 `binary-missing-onboarding-ux`
- 模式：full
- Stage：5 / 7（code-reviewer）
- 上游：04 = READY FOR REVIEW

## Files reviewed

- `web/src/pages/Dashboard.vue`（alert 文案 L8-21）
- `web/src/pages/Wizard.vue`（script L184-378 + step3 模板 L130-160）
- `web/src/pages/__tests__/Dashboard.spec.ts`（新增 T-057 describe）
- `web/src/pages/__tests__/Wizard.spec.ts`（新建）
- `scripts/baseline.json`
- `docs/dev-map.md`

## Findings

### CRITICAL

无。

### MAJOR

无。

### MINOR

- [MAINT] `Wizard.vue:344-348` — `missingForRole` 的角色三元链与 `modePayload`（L304-307）是两处独立表达同一"角色→kind"映射的逻辑，未来若新增第四种角色需同步两处。当前仅 frpc/frps/both 三态、且 02 §7 已记"镜像 modePayload 分支"，重复度可接受、不阻塞。NIT 级别也成立，留作注记。

### NIT

- [STYLE] `Wizard.vue:332` — `message.success('配置已保存，正在跳转...')` 文案字面量与 Wizard.spec / e2e 无强耦合断言冲突，OK。无需改。

## Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| AC-1（IS-1/IS-2 Dashboard 引导关键字 + 兜底） | `Dashboard.vue:18-20`（含「顶部横幅」「一键下载」「手动上传」「兜底」，删旧「请将对应文件放置到」） + `Dashboard.spec.ts` IS-1/IS-2 | ✅ |
| AC-2（IS-3 alert 内无按钮 / 无 UploadBinButton） | `Dashboard.vue:18-20` 纯文本无 `<n-button>`/UploadBinButton + `Dashboard.spec.ts` IS-3 断言 `findAll('button').length===0` | ✅ |
| AC-3（缺失：不自动跳 + 无 toast + 警告 + 手动按钮） | `Wizard.vue:325-328`（completing=false、不 push、不 success）+ step3 模板 L150-158 warning + 「进入仪表盘」按钮 + Wizard.spec「缺失不静默跳」3 用例 | ✅ |
| AC-4（不缺失：维持 success + 自动跳） | `Wizard.vue:329-334` + Wizard.spec「全就绪自动跳」用例 | ✅ |
| AC-5（无关缺失不误报：frpc 选中但缺 frps） | `Wizard.vue:344-348` missingForRole 取交集 + Wizard.spec AC-5 用例 | ✅ |
| AC-6（校验前 fetchReady 被调） | `Wizard.vue:322` `await appStore.fetchReady()` + Wizard.spec `expect(getReadyMock).toHaveBeenCalled()` | ✅ |
| AC-7（verify_all PASS） | 静态预测全绿；全量真跑交 orchestrator 硬闸门（PM 上下文无 Bash/PS，insight L14/L46） | ⏳ 交 orchestrator |
| AC-8（baseline bump） | `baseline.json` frontend_tests 437→454 / test_count 755→772 | ✅ |
| AC-9（06 裸 Adversarial + both/frps 缺失反向证伪） | 由 stage 6 落实；04/05 已约束裸标题（insight L40） | ⏳ stage 6 |

## Design fidelity check

| Design item（02） | Implementation | Status |
|---|---|---|
| Dashboard 仅改 alert 文本、不动 v-if/type/title/结构 | `Dashboard.vue:12-21` 结构未变，仅文本 | ✅ |
| Wizard 引入 useAppStore | `Wizard.vue:197,203` | ✅ |
| binWarning = ref<string[]>（定格快照，非 computed） | `Wizard.vue:213` | ✅ |
| 完成分支：fetchReady → missingForRole → 分流 | `Wizard.vue:322-334` 与 02 §6 flow 字节对应 | ✅ |
| missingForRole 取所选 need ∩ binMissing | `Wizard.vue:344-348` | ✅ |
| step3 条件渲染（不缺失原文案 / 缺失 warning+按钮） | `Wizard.vue:142-159` | ✅ |
| goToDashboard = router.push('/dashboard') | `Wizard.vue:351-353` | ✅ |
| 不重复造下载/上传按钮（IS-3/OOS-1） | Dashboard alert + Wizard step3 均无下载/上传按钮 | ✅ |
| 不改 store/API/路由守卫/AppLayout | git 改动域仅 Dashboard.vue/Wizard.vue/2 spec/baseline/dev-map | ✅ |
| 单 partition dev-frontend | 改动域全在 web/**（+ baseline/dev-map 流程伴随物） | ✅ |

## 逐维审查

1. **Logic correctness**：
   - 缺失判定 `length > 0` 分支 vs `else` 互斥完备，无第三态。
   - `missingForRole('')` 返回 `[]`（need 空数组）安全；完成分支 selectedRole 必非空（step1 挡空 L261-263），双保险。
   - BC-3（无关缺失）：`filter(k => binMissing.includes(k))` 只看 need 集合 → frpc 选中、缺 frps 时 need=['frpc']、binMissing=['frps']、交集=[] → 走自动跳。正确。
   - BC-5（fetchReady 失败）：`appStore.fetchReady` 内部 try/catch（`stores/app.ts:32-34`），失败时 binMissing 维持原值、不抛 → 完成分支不进 catch、按已知值判断。Wizard.spec Adversarial 第 2 例证伪。正确。
   - BC-6（保存失败）：`apiPutServer/apiPutClient/apiPutMode` 任一 reject → 进 catch（L335）设 configError，不到达 fetchReady/missingForRole → 不误进缺失分支。Wizard.spec BC-6 用例证伪。正确。
   - binWarning 定格：ref 一次性赋值，step3 模板读 `binWarning.length`，store.binMissing 后续变化不影响。Adversarial 第 3 例证伪。正确。
2. **Requirement fidelity**：见上表，AC-1~AC-6 + AC-8 已实现并测试覆盖；AC-7/AC-9 交后续 stage。
3. **Design fidelity**：见上表，与 02 §6 flow 字节对应，无 drift。
4. **Performance**：完成流程多一次 `apiGetReady` GET（轻量、仅完成一次、非热路径）。可接受。
5. **Security**：纯前端展示，无新输入面、无 XSS（文案静态、binWarning.join 仅 'frpc'/'frps' 字面 enum 值，非用户输入）。无凭据泄漏。
6. **Maintainability**：注释只解释 WHY（"用 ref 定格快照不用 computed"、"缺失不阻断但不自动跳走"），无冗余。命名清晰（missingForRole / binWarning / goToDashboard）。无死代码。SFC 纯逻辑 ~125 行 < 200（04 C-1 实测，insight L22 metric）。

## 测试质量审查（hard rule #4）

- Dashboard.spec T-057 块：IS-1 断言 4 个引导关键字（非 shape-match，真验文案语义）；IS-2 正反双断言（含「兜底」+ 不含旧句式）；IS-3 双重保险（button 计数 0 + UploadBinButton 计数 0）；BC-7 空不渲染。有意义。
- Wizard.spec：missingForRole 4 个交集语义用例覆盖 both/无关/全缺/空；完成流程用例用 push spy 验"未自动跳 vs 手动跳"（行为级而非状态级）；Adversarial 第 1 例（both 选中 frps 缺失）直接证伪"旧静默跳"反模式，第 2 例证伪 fetch 失败不崩不误报，第 3 例证伪定格快照。非 shape-match，有反向证伪价值。
- 测试模拟失败用 `apiError()`（BC-6 / Adversarial fetch 失败），读句柄用 `getExposed`（insight L45 合规）。

## Verdict

**APPROVED**（0 CRITICAL / 0 MAJOR；1 MINOR + 1 NIT 为注记，不阻塞。verify_all 全量真跑交 orchestrator 硬闸门，静态与设计保真度全绿。）
