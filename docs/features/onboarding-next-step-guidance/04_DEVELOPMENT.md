# 04 开发记录 — Frontend partition · T-062 · onboarding-next-step-guidance

> Stage 4 / dev-frontend · 模式：full · 输出语言：中文
> 上游：01（READY）+ 02（READY）+ 03（APPROVED WITH CONDITIONS，C-1~C-6）

## Partition

dev-frontend — owns: `web/**`（+ baseline.json / dev-map.md 文档惯例）。所有改动在 owned paths 内，零越界后端/DB/storage/migration。

## Files changed（this partition only）

源文件（5）：
- `web/src/pages/Wizard.vue` — IS-1（step3 加规则引导，仅 frpc/both + binWarning===0 就绪分支）+ IS-7（step2 both token 不一致非阻断 warning，computed tokenMismatch）+ defineExpose 追加 tokenMismatch/goToProxies/frpsForm/frpcForm。新 import：`computed`（vue）。N* 组件零新 import（NAlert/NButton/NText 已在）。
- `web/src/pages/Client.vue` — IS-2（handleSave 成功置 showNextStepHint=true → n-alert 引导按钮 push('/proxies')，失败不置 BC-7）。新 import：`useRouter`（vue-router）+ `NAlert`（naive-ui）。
- `web/src/pages/Proxies.vue` — IS-3（handleSubmit 新增/编辑成功置 showPostSaveHint=true → 双入口去 dashboard / monitor，删除不触发 BC-6）+ IS-4（#empty n-empty #extra 补「去服务端监控」入口）。新 import：`useRouter` + `NAlert`。
- `web/src/pages/Server.vue` — IS-5（loaded 态 #action 加「查看运行态→」push('/server/monitor')，加载失败/中态不显示 BC-8）。新 import：`useRouter`。NButton 已在（零 N* 新 import）。
- `web/src/components/ProxyForm.vue` — IS-6（tcp/udp 远程端口字段加 n-text 纯文案「需在服务端「端口策略」允许范围内」，方案 A）。新 import：`NText`。

测试文件（5）：
- `web/src/pages/__tests__/Wizard.spec.ts` — +12（IS-7 5 + IS-1 5 + Adversarial 2）。TestingHandle 追加 tokenMismatch/goToProxies/frpsForm/frpcForm。
- `web/src/pages/__tests__/Client.spec.ts` — +4。新增 vue-router push mock + putMock resolve setup（beforeEach）。
- `web/src/pages/__tests__/Proxies.spec.ts` — +8（IS-3 5 + IS-4 2 + Adversarial 1）。新增 vue-router push mock。
- `web/src/pages/__tests__/Server.spec.ts` — +5。新增 vue-router push mock。
- `web/src/components/__tests__/ProxyForm.spec.ts` — +3（IS-6 tcp/udp/http）。

基线 + 文档（2）：
- `scripts/baseline.json` — version 27→28、frontend_tests 500→532、test_count 822→854、go_tests 不变 322、passing_count 854、updated 2026-05-31、notes 追加 T-062 段（C-4）。
- `docs/dev-map.md` — Wizard/Client/Proxies/Server/ProxyForm 5 行追加 T-062 导航引导职责说明。

## 条件落实（对照 03 §4 C-1~C-6）

- **C-1（IS-1 验收点 + AC-11 护栏）**：达成。IS-1 测试以"引导按钮存在 + 点击触发 push('/proxies')"为验收（Wizard.spec IS-1 用例 trigger click 断言 pushSpy）；新增 AC-11 用例锁死 T-057 全就绪自动跳转既有行为不变（success toast '配置已保存，正在跳转...' + push('/dashboard')）。既有 T-057 用例（missingForRole/缺失分支/binWarning 定格快照/全就绪自动跳转）全部保留未改，零回归。
- **C-2（spec 引入 router mock 实读既有范式）**：达成。已实读 Client/Proxies/Server.spec 的 mount/Holder 范式 + beforeEach；新增 `vi.mock('vue-router', () => ({ useRouter: () => ({ push: pushSpy }) }))`（复用 Wizard.spec:58-59 既有范式），各 beforeEach 加 pushSpy.mockReset()。既有用例不依赖 router，零回归。
- **C-3（import 精确）**：达成。Client +NAlert、Proxies +NAlert、ProxyForm +NText；Wizard（+computed，N* 零新）、Server（+useRouter，N* 零新）。
- **C-4（baseline bump）**：达成（见上）。
- **C-5（裸 ## Adversarial tests + 反向证伪）**：交 06_TEST_REPORT.md（QA 阶段）。本阶段各 spec 内已含 Adversarial 用例（Wizard 2：token 不阻断/纯空白不误报；Proxies 1：失败态不出现空态入口）。
- **C-6（router.push 禁 href + 不碰后端/store/路由守卫/数据流）**：达成。所有 5 项导航均 `void router.push(...)`，无 `<a href>`/`tag=a`（insight L17）。未碰 router.ts、store、API、路由守卫、useProxyForm 校验、useServerRuntime。

## 设计保真度

- 无 DESIGN DRIFT。完全按 02 §6.1~6.5 实现。
- 02 §6.5 方案 A 采用（n-space vertical 包裹 input + n-text help 文本）。
- 02 §6.3 IS-4 空态采用 n-empty #extra slot（R-3 主路径，无需退化）。
- R-1 张力按 C-1 处理（接受局限，验收点为存在+可点击）。

## 测试设计要点（落实红线）

- 全部断言用 DOM 文本（`w.text()`）/ `w.findAll('button').find(b => b.text().includes(...))` 按文本 / getExposed 句柄 / pushSpy 调用参数。**零 naive-ui 组件名查询**（insight L45 / T-057 教训）。
- 导航断言用 vi.mock('vue-router') 的 pushSpy（复用 Wizard.spec 既有范式）。
- IS-3 测试通过 getExposed 直接置 showPostSaveHint=true 驱动 DOM（避免依赖真实 ProxyForm 子组件 getProxyInput 的脆弱完整 handleSubmit 路径），断言可观察契约（DOM 引导 + handler push）。
- IS-6 用 mount(ProxyForm, {props:{initialValue}}) 范式（ProxyForm.spec 既有）+ 多 tick settle 断言 help 文本。

## verify_all 结果

**PENDING** —— dev-frontend 上下文无 Bash/PowerShell（insight L31），无法真跑 `scripts/verify_all`。执行规格交 07_DELIVERY.md。静态自检：
- 5 源文件改动均语法完整（Edit 工具成功落盘，old_string 精确匹配）。
- import 增量已对齐（C-3）。
- baseline frontend_tests=532 与新增用例计数（500+32）一致。
- 无越界改动（仅 web/** + baseline + dev-map）。

## Verdict

**READY FOR REVIEW**（frontend partition complete）—— 5 项 IS 全部实现，+32 前端测试，条件 C-1~C-6 全落实（C-5 的裸标题段在 06 产出），verify_all 标 PENDING 待 orchestrator 真跑硬闸门。
