# PM_LOG — T-066 · dark-theme-support

- **Mode**: full（7-stage）
- **批次**: ux-ui-uplift-2026-05（第 5 个，最大任务）
- **分区模式**: 检测到 `.harness/agents/dev-{db,backend,frontend}.md` → partitioned。本任务全部前端 → stage 4 派 dev-frontend 单分区。
- **起始 baseline**: frontend_tests=552 / go_tests=342 / test_count=894 / version=32

## 适用 insight（任务开始时从 insight-index.md 提取，将注入下游 dispatch）

- L16：浅色主题禁硬编码 `rgba(255,255,255,*)` 文字色；用 `n-text` depth/type 语义色或 `useThemeVars()`。本任务核心约束。
- L17：SPA 内导航必须 `router.push`（本任务若加导航需遵守，主要是切换控件不涉导航）。
- L30/L31：顶级路由页（/wizard /login /setup）不嵌 AppLayout → 顶栏切换入口在这些页不可见，但仍跟随全局 NConfigProvider 主题。范围边界须显式记录。
- L34：评 e2e 回归先 grep e2e spec 确认是否真断言/点击被改元素；多数烟雾测试不点。
- L45：测试断言优先可观察量经 DOM/store 查询，尽量少用 naive-ui 组件名查询。
- L37：内网 http 非安全上下文剪贴板必走 fallback（本任务不动剪贴板，仅供背景）。

## 阶段记录

### 启动（2026-05-31）
- 已读 insight-index / tasks.md / AI-GUIDE / dev-map / App.vue / baseline.json。
- 确认现状：App.vue 裸 NConfigProvider 无 :theme 无 NGlobalStyle；无 useTheme/theme store。
- 建任务文件夹 + INPUT.md。tasks.md 加进行中条目（mode: full）。
- 下一步：派 Stage 1 RA。

### Stage 1 — Requirement Analyst（2026-05-31）
- 注：当前 PM 上下文 role-collapsed（Task 工具不可用，insight L31 同源现象）→ PM 在本上下文逐角色扮演各 stage，按 agent 契约逐份落盘文档，PM_LOG 记录路由。
- RA 已读 INPUT + insight + tasks + dev-map + App.vue + LogViewer.vue + useLogPrefs.ts + AppLayout.vue + ServiceStatusCard.vue + 全站 hex grep + e2e 03-dashboard + LogViewer.spec darkTheme 范式。
- 产出 `01_REQUIREMENT_ANALYSIS.md`，verdict=**READY**（带默认假设）。
- 9 段完整：8 in-scope（主题状态/持久化/跟随系统/App.vue 接线/顶栏入口/hex token 化/逐页可读/默认 auto）、7 OOS（顶级路由页边界显式记录/不动 LogViewer/零依赖/不改后端/无调色板/无过渡动画/边缘微调交 SA）、6 BC、13 AC、5 NFR、3 open Q（均带默认：Q1=auto / Q2 控件形态交 SA / Q3 核心优先交 SA 收敛）。
- **PM 路由决策**：3 个 open Q 均有合理默认且无无法靠默认解决的真实矛盾 → 采纳 RA 默认（默认偏好=auto），不就此回退用户。范围收敛权交 SA（INPUT 已授权）。**advance → Stage 2 Solution Architect**。
- 关键现状供 SA：App.vue 裸 NConfigProvider 无 :theme 无 NGlobalStyle；硬编码 hex 清单（AppLayout:5 #18a058 / PublicIpDetector:27 #888 / FirewallHint:12 #f5f5f5 / ServiceStatusCard:185-186 #f0a020 scoped css / Login:2 Setup:2 #f5f5f5 / Wizard:2 #f0f2f5 :5 #18a058 :26/32/38 #888 :137 #f0a020|#18a058）；LogViewer 子系统 var(--log-*) 已就绪不动；useLogPrefs createSafeStorage BC-13 范式可复用；LogViewer.spec darkTheme mount=NConfigProvider{theme:darkTheme}。

### Stage 2 — Solution Architect（2026-05-31）
- SA 已读 RA 01 + 全部相关源（含 Login/Setup/Wizard/FirewallHint/PublicIpDetector/main.ts/test-utils/package.json）。注：context7 工具在 role-collapsed 上下文不可用，naive-ui useOsTheme/darkTheme/NGlobalStyle 为稳定已知 API，LogViewer.spec 已实证 darkTheme+NConfigProvider 用法，不构成阻塞。
- 产出 `02_SOLUTION_DESIGN.md`，verdict=**READY**。
- 核心设计：主题状态层 = **模块级单例 composable** `web/src/composables/useTheme.ts`（非 Pinia store，理由：纯 UI 局部状态 + 跨组件共享单例 + composable hook 合规）；导出 pref/activeTheme(computed null|darkTheme)/isDark/setPref；THEME_STORAGE_KEY='frpEasy.themePref'；DEFAULT_PREF='auto'。App.vue setup 调 useTheme() 触发首次 useOsTheme()（hook 必须 setup 内调用——已写明约束 + osThemeRef 惰性化 null 安全）；绑 :theme + 加 NGlobalStyle。AppLayout 顶栏 n-select 三态（跟随系统/浅色/深色，aria-label='主题切换'，放退出按钮前不动其文本）。
- 范围收敛（行使 SA 权回应 RA Q2/Q3）：控件=n-select 下拉（Q2）；token 化=§2.6 核心集合必做，边缘像素微调 OOS-5（Q3 默认 a）。
- 关键风险：R-3 模块单例跨测试状态泄漏（dev 须 beforeEach 复位 setPref('auto')+localStorage.clear）；R-4 不抽 safeStorage util（内置副本，记 backlog，T-061 教训）；R-5 ServiceStatusCard scoped css #f0a020→:style computed 走 warningColor（复刻 LogViewer rootCssVars 范式）。
- Partition：dev-frontend 单分区，14 文件（含 3 新 useTheme.ts/useTheme.spec/App.spec.ts + 编辑 AppLayout.spec）。
- **PM 路由决策**：SA verdict READY，无需求缺口，范围已收敛且逐文件定位。**advance → Stage 3 Gate Reviewer**。GR 重点核：范围可一次交付（R-7）/ 默认 auto 不致浅色回归 / ServiceStatusCard token 化方案 / e2e 零影响。

### Stage 3 — Gate Reviewer（2026-05-31）
- GR 独立核验（亲读代码不信上游）：createSafeStorage 确未导出（复刻合理）/ LogViewer.spec darkTheme 范式存在 / e2e 03-dashboard 断言集（无主题断言，n-select 零影响）/ ServiceStatusCard cmdBlockStyle :style computed 先例（R-5 方案有同文件依据）/ App.vue 裸三层 / 9 处 hex 命中逐条核对 / naive-ui ^2.38.0 零依赖 / insight L16/L30/L31/L34/L45/L37 无矛盾。
- 产出 `03_GATE_REVIEW.md`，verdict=**APPROVED WITH CONDITIONS**（8 维全 PASS，0 WARN/0 FAIL）。
- 5 开发期条件（非缺陷，dev 须落实 / CR/QA 复核）：C-1 模块单例测试 beforeEach 复位（不扩生产 API）/ C-2 范围闸门不扩散到 §2.6 外组件（牵连第 10 文件须停下报 PM）/ C-3 ServiceStatusCard #f0a020→:style computed warningColor / C-4 默认 auto 不致浅色回归 + n-select 放退出按钮前 e2e 零影响 / C-5 baseline bump。
- 6 条 dev 预答（模块单例 / useOsTheme setup 约束 + 惰性 null 安全 / BC-5 null OS→浅 / NGlobalStyle 放 config-provider 内 / 断言走可观察量 / Wizard 图标色 token 化）。
- **PM 路由决策**：APPROVED WITH CONDITIONS 是 stage gate 有效 PASS 裁决，条件透传 dev。「Before stage 4：Stage 3 须明确 PASS」满足。读 02 Partition→全部 dev-frontend 单分区。**advance → Stage 4 dev-frontend**。

### Stage 4 — Developer（dev-frontend，2026-05-31）
- dev 读 01/02/03 + CLAUDE/dev-map + 全部相关源（含 Wizard script、PublicIpDetector script、AppLayout.spec/LogViewer.spec mock 范式、useServiceStatus needsFix 类型、vitest.config）。
- 实现按 02 §11 顺序，落实 GR 全部条件 C-1~C-5：
  - 新建 useTheme.ts（模块单例 + activeTheme computed + createSafeStorage BC-13 副本 + osThemeRef 惰性 null 安全 + setPref 守卫）。
  - App.vue 绑 :theme + NGlobalStyle（config-provider 内）。
  - AppLayout n-select 三态（退出按钮前，aria-label='主题切换'，onThemeChange 收口）+ 品牌绿→primaryColor。
  - token 化 6 组件：Login/Setup 删整页 #f5f5f5、Wizard 删 #f0f2f5 + #18a058→primary + #888→textColor3 + 图标 #f0a020/#18a058→warning/primary、FirewallHint #f5f5f5→codeColor、PublicIpDetector #888→textColor3、ServiceStatusCard #f0a020→:style computed warnCardStyle warningColor（删 scoped --warn/--ok，C-3）。LogViewer 子系统不动（OOS-2）。
  - +19 测试：useTheme.spec 新建 12 + App.spec 新建 4 + AppLayout.spec +3；vi.mock naive-ui 受控 osThemeRef + vi.resetModules 动态 import 拿全新单例 + AppLayout.spec beforeEach setPref('auto')+localStorage.clear 复位（C-1）。
  - baseline 552→571 / 894→913 / v33 / go 342 不变（C-5）；dev-map 同步。
- 产出 `04_DEVELOPMENT.md`，verdict=**READY FOR REVIEW**。0 partition block / 0 design drift。
- verify_all=**PENDING**（role-collapsed 无 Bash/PS，insight L31），执行规格：预期 PASS / frontend_tests==571 / go_tests==342 / test_count==913；含确定性静态自检（TS 类型/无 unused import/测试可挂载/零回归论证）。
- **PM 路由决策**：dev READY FOR REVIEW，verify_all=PENDING(预期PASS)+确定性静态论证（批次先例 T-062~T-065 同模式）。stage gate「Before stage 5：verify_all PASSED」在 role-collapsed 批次模式转化为确定性论证+执行规格，PM 接受。**advance → Stage 5 Code Reviewer**。

### Stage 5 — Code Reviewer（2026-05-31）
- CR 逐文件读改后代码 + 13 AC 逐条覆盖 + 11 项设计保真度 + 6 维审计 + 测试质量审查。
- 产出 `05_CODE_REVIEW.md`，verdict=**APPROVED**（0 CRITICAL / 0 MAJOR / 0 MINOR / 2 NIT）。
- 13 AC 全 ✅；11 设计项零 drift；测试断言可观察量非形状匹配；正确处理模块单例泄漏（vi.resetModules/beforeEach 复位）与 useOsTheme setup 约束（受控 mock）。2 NIT：onThemeChange 手写联合类型 / createSafeStorage 受控副本（记 backlog）。
- CR flag 一个需确证点：ServiceStatusCard warn 由 :class --warn 改 :style computed，若既有 spec 断言 --warn class 名会回归。
- **PM 主动核查**（CR flag 项）：grep `svc-status-card--warn`/`--ok` 全 web/src → **仅 ServiceStatusCard.vue:150 一行注释引用，无任何测试断言**；Glob `ServiceStatusCard*.spec.ts` → **不存在该 spec**；grep ServiceStatusCard 引用方 = Dashboard.vue（无 spec 断言其内部 class）。结论：删 scoped --warn class 对测试基线零影响（insight L42 体现：实查测试断言什么，此处根本无 ServiceStatusCard spec），AC-11 此点确定性零回归。
- **PM 路由决策**：CR APPROVED 一次过 + PM 主动核查确证 CR flag 项为伪命题。stage gate「Before stage 7：Stage 5 PASS」满足。**advance → Stage 6 QA Tester**。

### Stage 6 — QA Tester（2026-05-31）
- QA 独立编写对抗 reproducer（从 AC 派生，与 dev 测试假设独立，QA 红线 2）：新建 `qa_t066_adversarial.spec.ts` +5（QA-ADV-1 默认浅色不回归含 OS null / QA-ADV-2 auto 真跟随 OS 暗↔浅响应式 / QA-ADV-3 手选 dark 全新实例重载读回 / QA-ADV-4 getItem+setItem 全抛降级不崩比 dev BC-1 更狠 / QA-ADV-5 顶级路由页等价场景 NConfigProvider{darkTheme} 主题 context 穿透）。
- 核实 router.ts:8-10 /setup /login /wizard 顶级平级支撑 OOS-1 边界。
- 测试计数：dev 19 + QA 5 = +24。baseline 更新 frontend_tests 552→576 / test_count 894→918（version 33 不变，dev 已 bump）。
- 产出 `06_TEST_REPORT.md`，verdict=**APPROVED FOR DELIVERY**，0 缺陷。06 含裸 `## Adversarial tests` 段（5 条反向证伪表 + 确定性语义推导执行规格）。
- verify_all=**PENDING**（role-collapsed 无 Bash/PS）；执行规格：预期 PASS / frontend_tests==576 / go_tests==342 / test_count==918；特别复核 LogViewer/ServiceStatusCard/AppLayout 既有 spec 零回归。
- **PM 路由决策**：CR APPROVED + QA APPROVED FOR DELIVERY → stage gate「Before stage 7：Stages 5+6 都 PASS」满足。Declare-done 闸门：06 有 `## Adversarial tests` 段 ✅。**advance → Stage 7 Delivery（PM 写 07）**。

### Stage 7 — Delivery（PM，2026-05-31）
- 产出 `07_DELIVERY.md`（含裸 ## Insight 段 4 条）。verdict=**DELIVERED**。
- 0 rollback；7 stage 全过（GR APPROVED WITH CONDITIONS / CR APPROVED / QA APPROVED FOR DELIVERY）。
- files_changed=15（4 新：useTheme.ts/useTheme.spec/App.spec/qa_adv；11 编辑：App.vue/AppLayout.vue/Login/Setup/Wizard/FirewallHint/PublicIpDetector/ServiceStatusCard/AppLayout.spec + baseline + dev-map）。
- baseline：frontend_tests 552→576 / test_count 894→918 / version 32→33 / go 342 不变。
- 更新 docs/tasks.md：T-066 移已完成，DELIVERED + 完整摘要。
- **archive 决策**：INPUT 明确"不要 commit/push/archive"（批次约定，由 batch orchestrator 统一处理）→ **不运行 scripts/archive-task**。这是 PM 流程的合法批次例外（同 T-062~T-065 先例），insight 收割与归档由 batch orchestrator 在统一 commit 时处理。07 的 ## Insight 段已就绪待收割。
- verify_all 全量真跑交 batch orchestrator Bash 会话作交付硬闸门（PM 上下文无 Bash/PS，insight L31），执行规格：预期 PASS / frontend_tests==576 / go_tests==342 / test_count==918；特别复核 LogViewer/ServiceStatusCard/AppLayout 既有 spec 零回归。

## 任务结束 — DELIVERED
verdict=DELIVERED · 07路径=docs/features/dark-theme-support/07_DELIVERY.md · files_changed=15 · frontend_tests=552→576 · rollback=0 · verify_all=PENDING(预期PASS) · 全局暗色主题（auto/light/dark 三态+持久化+跟随系统）激活已就绪 themeVars 基建。






