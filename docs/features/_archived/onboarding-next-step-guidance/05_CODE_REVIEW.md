# 05 代码评审 — T-062 · onboarding-next-step-guidance

> Stage 5 / Code Reviewer · 模式：full · 输出语言：中文
> 独立核验：CR 已实读全部 12 个改动文件（5 源 + 5 spec + baseline + dev-map），逐 AC 走查 01，逐 §6 走查 02。

## Files reviewed

- `web/src/pages/Wizard.vue`
- `web/src/pages/Client.vue`
- `web/src/pages/Proxies.vue`
- `web/src/pages/Server.vue`
- `web/src/components/ProxyForm.vue`
- `web/src/pages/__tests__/Wizard.spec.ts`
- `web/src/pages/__tests__/Client.spec.ts`
- `web/src/pages/__tests__/Proxies.spec.ts`
- `web/src/pages/__tests__/Server.spec.ts`
- `web/src/components/__tests__/ProxyForm.spec.ts`
- `scripts/baseline.json`
- `docs/dev-map.md`

## Findings

### CRITICAL
（无）

### MAJOR
（无）

### MINOR
- [MAINT] `web/src/pages/Client.vue` showNextStepHint / `web/src/pages/Proxies.vue` showPostSaveHint —— 引导标志保存成功后置 true 且**不重置**（重载/重新打开模态框时仍显示）。03 §3 Q2/Q3 已预答此为可接受实现细节（用户已保存过，引导仍有意义）。非缺陷，记录为已知行为，不阻塞。

### NIT
- [STYLE] `web/src/pages/Wizard.vue:157` IS-1 引导按钮在全就绪分支，会被同 tick 自动 push('/dashboard') 抢先（R-1 张力）。GR C-1 已定调验收点为"存在+可点击"，设计接受局限。纯偏好层面无更优解（移除自动跳转会破坏 T-057）。不阻塞。

## Requirement coverage check（逐 AC 走查 01）

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 frpc/both 就绪完成 → 加规则引导 + push(/proxies) | Wizard.vue:157-161（`frpc||both` div）+ goToProxies push('/proxies')；Wizard.spec IS-1 用例 trigger click 断言 pushSpy | ✅ |
| AC-2 frps 角色 → 不出现加规则引导 | Wizard.vue:157 `v-if="selectedRole==='frpc'||'both'"`；Wizard.spec 'AC-2 frps 角色' 断言 not present | ✅ |
| AC-3 both 缺失 → T-057 警告在、引导不在 | Wizard.vue:147 `v-if="binWarning.length===0"`（引导仅就绪分支）+ :164 v-else 缺失分支未加；Wizard.spec 'AC-3' 断言 | ✅ |
| AC-4 Client 保存成功 → 引导 push(/proxies)；失败不显示 | Client.vue handleSave 成功置 showNextStepHint + n-alert + goToProxies；Client.spec 4 用例（含 BC-7 失败不显示） | ✅ |
| AC-5 Proxies 新增/编辑成功 → 双入口 push(/dashboard)/(/server/monitor) | Proxies.vue handleSubmit 成功置 showPostSaveHint + 双按钮；Proxies.spec IS-3 5 用例 | ✅ |
| AC-6 Proxies 空态 → 含连通入口 + 保留新增规则文案 | Proxies.vue #empty n-empty #extra「去服务端监控」；Proxies.spec IS-4 2 用例（断言保留旧文案 + 新入口 + push） | ✅ |
| AC-7 Server loaded → 查看运行态 push(/server/monitor)；失败/中态不显示 | Server.vue loaded #action 按钮 + goToMonitor；Server.spec 5 用例（含 BC-8 失败/中态不显示） | ✅ |
| AC-8 ProxyForm tcp/udp 远程端口含端口策略文案 | ProxyForm.vue:71-73 n-text；ProxyForm.spec IS-6 3 用例（tcp/udp 含、http 不强制） | ✅ |
| AC-9 both 两端 token 非空且不等 → warning + 非阻断推进 | Wizard.vue tokenMismatch computed + alert；Wizard.spec IS-7 'AC-9' + Adversarial 用例 | ✅ |
| AC-10 token 相等/一空/非 both → 不出现 warning | Wizard.vue tokenMismatch 条件；Wizard.spec IS-7 3 用例 | ✅ |
| AC-11 T-057 全就绪自动跳转既有行为不变 | Wizard.vue:329-332 未改（CR 核 handleNext 全就绪分支仍 success toast + push('/dashboard')）；Wizard.spec 'AC-11' 护栏用例 + 既有 T-057 用例全保留 | ✅ |
| AC-12 所有导航 router.push 而非 href | 5 handler 全 `void router.push(...)`，CR grep 确认无 `<a href>`/`tag=a`；各 spec push mock 断言 | ✅ |
| AC-13 verify_all PASS + baseline bump | baseline frontend_tests 500→532 / test_count 822→854 / version 28；verify_all PENDING（orchestrator 真跑） | ✅（计数正确，真跑 PENDING） |

**覆盖结论**：13/13 AC 均有实现且对应测试，无遗漏。

## Design fidelity check（逐 §6 走查 02）

| Design item | Implementation | Status |
|---|---|---|
| §6.1 IS-7 tokenMismatch computed（both + 双 trim 非空 + 不等） | Wizard.vue computed 实现与设计字面一致（trim 判空 + trim 后比较 BC-4） | ✅ |
| §6.1 IS-1 引导在 binWarning===0 分支 + frpc/both guard | Wizard.vue:147+157 双重条件 | ✅ |
| §6.1 不破坏 T-057（自动跳转/缺失分支/binWarning 定格） | handleNext L318-333 未改；v-else 缺失分支未改 | ✅ |
| §6.2 IS-2 showNextStepHint + useRouter + goToProxies | Client.vue 实现一致；+NAlert import（C-3） | ✅ |
| §6.3 IS-3 showPostSaveHint（两路径置位、删除不置） | Proxies.vue handleSubmit 成功置位（create/update 共用后路径）；handleDeleteConfirm 未置（BC-6） | ✅ |
| §6.3 IS-4 #empty #extra slot | Proxies.vue n-empty #extra 实现（R-3 主路径，未退化） | ✅ |
| §6.4 IS-5 loaded 态 #action 按钮 | Server.vue v-else card 内 #action（失败/中态 card 不含） | ✅ |
| §6.5 IS-6 方案 A（n-space + n-text help 文本） | ProxyForm.vue:63-74；+NText import（C-3） | ✅ |
| §11 单分区 dev-frontend，全 web/** | 改动仅 web/** + baseline + dev-map（文档惯例） | ✅ |

**保真结论**：无 DESIGN DRIFT。所有实现与 02 §6 字面一致。

## 6 维度评审

1. **逻辑正确性**：PASS。tokenMismatch 边界（一空/纯空白 BC-4/非 both）正确；showNextStepHint/showPostSaveHint 仅成功路径置位、失败 catch 不置（BC-7）；删除路径不触发（BC-6）；Server IS-5 仅 loaded 态（v-else card，BC-8）。
2. **需求保真**：PASS。13/13 AC 实现（见上表）。
3. **设计保真**：PASS。无 drift（见上表）。
4. **性能**：PASS。纯静态 UI 文案 + 同步 router.push，无 N+1、无循环、无大分配。tokenMismatch 是轻量 computed（读两 ref + trim 比较）。
5. **安全**：PASS。无输入处理、无 secret、无 SQL、无反序列化。导航目标均为硬编码现有路由字符串（无用户输入拼接）。
6. **可维护性**：PASS。命名清晰（goToProxies/goToDashboard/goToMonitor/goToMonitor、showNextStepHint/showPostSaveHint/tokenMismatch）；注释只在 WHY 处（IS 编号 + 与 T-057 并存说明 + BC 引用）；无死代码；无过度抽象（5 项内联符合 02 §3 决策）。

## 测试质量评审（红线 4：测试是否有意义而非形状匹配）

- **断言可观察契约**：全部断言 DOM 文本 / find('button') 按文本 / getExposed 句柄 / pushSpy 调用参数，**零 naive-ui 组件名查询**（CR 核对各 spec，符合 insight L45 / T-057 教训）。
- **反向证伪有效**：Wizard Adversarial 'token 不一致绝不阻断完成' —— 若 tokenMismatch 被误接进 handleNext 校验则 currentStep 停在 2、断言失败（真证伪）；'纯空白 trim 视空不误报' —— 若漏 trim 则误报、断言失败（真证伪）。Proxies Adversarial '加载失败态不出现空态连通入口' —— 若入口放数据表外无条件渲染则失败态也出现、断言失败（真证伪）。均非形状匹配。
- **T-057 既有用例零回归**：CR 核 Wizard.spec 既有 missingForRole 4 + 全就绪自动跳转 2 + 缺失分支 3 + 保存失败 1 + frpc 标题 3 + 既有 Adversarial 3 全保留未改；新增 AC-11 护栏用例额外锁死自动跳转。TestingHandle 仅追加字段（tokenMismatch/goToProxies/frpsForm/frpcForm），不改既有字段。
- **新增 router mock 零回归**：Client/Proxies/Server.spec 新增 `vi.mock('vue-router')`（复用 Wizard.spec:58-59 范式），既有用例不依赖 router；beforeEach 加 pushSpy.mockReset()。CR 确认 mock 不与既有 naive-ui mock / getExposed 冲突。
- **计数一致**：+32（Wizard 12 + Client 4 + Proxies 8 + Server 5 + ProxyForm 3）= baseline 500→532 一致（C-4）。

## Verdict

**APPROVED** —— 无 CRITICAL、无 MAJOR。13/13 AC 实现，无 DESIGN DRIFT，T-057 既有用例零回归，测试反向证伪有效。2 条 MINOR/NIT 为已知可接受行为（GR 已预答），不阻塞。verify_all 真跑由 orchestrator 作硬闸门（CR 静态核验全绿）。
