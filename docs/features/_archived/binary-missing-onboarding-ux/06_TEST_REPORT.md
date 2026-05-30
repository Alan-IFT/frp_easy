# 06 — 测试报告（Test Report）

- 任务：T-057 `binary-missing-onboarding-ux`
- 模式：full
- Stage：6 / 7（qa-tester）
- 上游：05 = APPROVED

## 证据状态声明（insight L14 role-collapse）

本 QA stage 在 PM 上下文角色化运行，派发上下文工具被裁剪（无 Bash / PowerShell）→ **无法在本 stage 内真跑 vitest / verify_all 贴出 stdout**。按项目既有处置（insight L14 / L46，与 T-050~T-056 一致）：测试用例已编码进 suite，**全量真跑 `bash scripts/verify_all.sh`（含 e2e）作为声明完成的硬闸门，交 orchestrator 独立执行**。本报告记录测试计划、预期结果、独立反向证伪假设与静态推演结论。

## Test plan

| Acceptance criterion | Test case(s) | File |
|---|---|---|
| AC-1（IS-1/IS-2 引导关键字 + 兜底） | `it('IS-1：alert 文案含「下载/上传/顶部」引导关键字')`、`it('IS-2：手动放置仅作兜底…不再首选')` | `web/src/pages/__tests__/Dashboard.spec.ts` |
| AC-2（IS-3 alert 内无按钮） | `it('IS-3：缺失提示 alert 内不重复造下载/上传按钮')` | `Dashboard.spec.ts` |
| AC-3（缺失：不自动跳 + 无 toast + 警告 + 手动按钮） | `it('both + frps 缺失：…未自动 push、出现警告与手动按钮')`、`it('点「进入仪表盘」按钮才跳转')`、`it('frps 角色 + frps 缺失…不自动跳')` | `web/src/pages/__tests__/Wizard.spec.ts` |
| AC-4（不缺失：维持 success + 自动跳） | `it('both 全就绪：success("正在跳转") + 自动 push')` | `Wizard.spec.ts` |
| AC-5（无关缺失不误报） | `it('AC-5：frpc 但缺的是 frps（与所选无关）→ 仍走自动跳转')` | `Wizard.spec.ts` |
| AC-6（校验前 fetchReady 被调） | `it('…+ binWarning=[]')` 内 `expect(getReadyMock).toHaveBeenCalled()` | `Wizard.spec.ts` |
| AC-7（verify_all PASS） | 全量真跑交 orchestrator 硬闸门 | `scripts/verify_all.sh` |
| AC-8（baseline bump） | frontend_tests 437→454 / test_count 755→772 | `scripts/baseline.json` |
| AC-9（裸 Adversarial + both/frps 缺失反向证伪） | 本报告 `## Adversarial tests` 段 + Wizard.spec Adversarial describe | `06` + `Wizard.spec.ts` |

## Boundary tests added

- `binMissing=[]`（不缺失）→ Dashboard alert 不渲染（BC-7，Dashboard.spec）；Wizard 走自动跳转（AC-4）。
- `both` 且仅 `frps` 缺失（部分缺失，BC-2）→ binWarning=['frps']、警告列出 frps（Wizard.spec / Adversarial）。
- `frpc` 选中但缺 `frps`（无关缺失，BC-3）→ binWarning=[]、自动跳（AC-5）。
- `both` 且 frpc+frps 全缺失（BC-4）→ binWarning=['frpc','frps']（missingForRole 用例 + 手动按钮用例）。
- `fetchReady` 失败被吞（BC-5）→ 不崩、按已知值（空）自动跳、不误报（Adversarial）。
- 保存失败优先（BC-6）→ configError 设、不进缺失分支（Wizard.spec BC-6）。

## Adversarial tests

> 独立反向证伪。每条先写失败假设，再判实现是否存活。证据为静态推演（PM 上下文不可真跑，stdout 由 orchestrator 全量 verify_all 提供硬闸门）；用例已编码于 `Wizard.spec.ts` 的 `describe('Wizard.vue — Adversarial（T-057）')`。

| AC | 假设（"我预期失败当…"） | Reproducer | 推演结果 |
|---|---|---|---|
| AC-3 / AC-9（核心） | 若完成分支无视 binMissing 直接 success+push（旧静默跳行为），则 both 选中 + frps 缺失时会悄悄跳到仪表盘、binWarning 为空 | `Wizard.spec.ts` `it('ADV：both 选中但 frps 缺失 → 警告出现且未静默自动跳')`（QA 独立从 AC 构造） | 存活 — 实现先 `missingForRole` 求交集 `['frps']` → 进 `length>0` 分支 → `completing=false`、不 push、不 success；断言 `binWarning=['frps']` + `w.text()` 含「尚未就绪」+ `pushSpy` 未以 `/dashboard` 调用 + success 未以「正在跳转」调用。**反模式被拦。** |
| AC-5 | 若 missingForRole 用 `binMissing.length>0` 整体判断而非 per-role 交集，则 frpc 选中、缺 frps（无关）会被误报警告、卡在向导 | `Wizard.spec.ts` `it('AC-5：frpc 但缺的是 frps…→ 仍走自动跳转')` | 存活 — `missingForRole('frpc')` 的 need=['frpc'] 与 binMissing=['frps'] 交集为 [] → 自动跳转、success 正常。**误报被拦。** |
| BC-5 | 若 fetchReady 失败未被吞 / 完成分支不处理异常，则 ready 探测 reject 会让完成流程崩或误进 catch 设 configError | `Wizard.spec.ts` `it('ADV：fetchReady 失败被吞 → 不崩、按已知 binMissing(空) 走自动跳转，不误报警告')` | 存活 — `appStore.fetchReady` 内部 try/catch（`stores/app.ts:32-34`）吞错不抛 → 完成分支续走，binMissing 维持空 → binWarning=[] → 自动跳、configError 仍为 ''。**崩溃/误报被拦。** |
| AC-3 定格语义 | 若 binWarning 用 computed 跟随 store，则完成后后台下载完成清空 binMissing 会让 step3 已展示的警告凭空消失（用户困惑） | `Wizard.spec.ts` `it('ADV：binWarning 定格快照…store 清空也不抹掉已展示警告')` | 存活 — binWarning 是 ref 一次性快照；mount 后 `useAppStore().binMissing = []` 不改 binWarning 值，`w.text()` 仍含「尚未就绪」。**定格语义成立。** |

（裸 `## Adversarial tests` 标题，无数字 / §N 前缀，符合 verify_all E.6 锚定与 insight L40。）

## verify_all result（预期，交 orchestrator 真跑硬闸门）

- Total tests: 755 → 772（前端 frontend_tests 437→454，+17；Go 不变 318）
- Pass: 预期 772（静态推演全绿）
- Fail: 预期 0（声明完成硬条件）
- Warn: 预期 0
- New tests added: 17（Dashboard +4 / Wizard +13）
- Baseline updated: yes（version 23→24，frontend_tests 437→454，test_count 755→772）
- e2e（含 01-setup / 03-dashboard）预期不受影响：01-setup 仅断言离开 /setup；03-dashboard 用 bypassWizard 绕过向导；且实现保证不缺失维持原自动跳转（详见 04 e2e 预判）。

## Defects found

无。

## Stability

- 用例均为确定性单元测试（无定时器、无网络、无随机），无 flake 风险面。push spy / message spy / api mock 在 beforeEach 全 reset，无跨用例泄漏。
- 真稳定性（连跑）由 orchestrator 全量 verify_all 验证。

## Verdict

**APPROVED FOR DELIVERY**（AC-1~AC-6 + AC-8 + AC-9 已实现并测试覆盖；AC-7 verify_all 全量真跑交 orchestrator 硬闸门，静态推演全绿、0 预期失败。）
