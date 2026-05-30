# Delivery Summary — T-067 · responsive-layout

- Task: `responsive-layout` — 让应用外壳（侧边栏/顶栏/Server·Client 表单）在窄屏/移动端可用，支撑 FRP 管理面板"手机查看进程/重启穿透"的真实运维场景。
- Mode: **full**（7-stage）· 批次 ux-ui-uplift-2026-05 第 6/最后一个 · dev-frontend 单分区（纯前端）
- Stages traversed（均在 role-collapsed 单会话内由 PM 顺序扮演各 agent 角色产出，Task 派发工具不可用，降级路径见 PM_LOG）:
  - Stage 1 Requirement Analyst — 01_REQUIREMENT_ANALYSIS.md · READY（2026-05-31）
  - Stage 2 Solution Architect — 02_SOLUTION_DESIGN.md · READY（2026-05-31）
  - Stage 3 Gate Reviewer — 03_GATE_REVIEW.md · APPROVED FOR DEVELOPMENT（8 维全 PASS，2026-05-31）
  - Stage 4 dev-frontend — 04_DEVELOPMENT.md · READY FOR REVIEW（2026-05-31）
  - Stage 5 Code Reviewer — 05_CODE_REVIEW.md · APPROVED（0 CRITICAL/MAJOR/MINOR，2 NIT，2026-05-31）
  - Stage 6 QA Tester — 06_TEST_REPORT.md · APPROVED FOR DELIVERY（0 缺陷，含裸 ## Adversarial tests 6 条，2026-05-31）
  - Stage 7 PM — 本文件
- Rollbacks: **0**
- Final verify_all result: **PENDING（预期 PASS）** — PM/dev/QA role-collapsed 上下文无 Bash/PS（insight L31），标 PENDING + 执行规格，真跑由 batch orchestrator Bash 会话执行作硬闸门。
- Baseline changes: frontend_tests **576 → 600**（+24：dev 18 + QA 独立对抗 6）；test_count **918 → 942**；go_tests **342**（不变）；version **33 → 34**。同时修复 baseline.json 既有冗余尾巴使其为合法单一 JSON 对象（GR C-4）。
- Outstanding risks:
  - e2e 03-dashboard 默认视口 1280 ≥ 折叠阈值 768 → 侧栏保持展开、菜单文本可见、退出按钮可点击，**预期零回归**（PM/CR/QA 三轮 grep 核实 + QA-ADV-6 编码同一命题）。batch orchestrator 真跑须特别复核 e2e（C.1）。
  - AppLayout 既有 8 例（T-064/T-066）+ Server/Client 既有 spec 零回归（既有用例不 stub matchMedia → happy-dom 默认判展开=既有行为；Server/Client spec 经 grep 无 width 断言）。真跑复核。
  - QA-ADV-3 折叠 trigger 用 naive-ui 内部 class（`.n-layout-toggle-button`）选择，有等价命题回退分支防脆弱（折叠 trigger 无可观察 aria 名，是 insight L45 的必要折中）。
- Files changed（11，全在 dev-frontend owned paths `web/**` + 文档元数据）:
  - 新建：`web/src/composables/useViewport.ts`、`web/src/composables/__tests__/useViewport.spec.ts`、`web/src/components/__tests__/qa_t067_adversarial.spec.ts`
  - 编辑：`web/src/components/AppLayout.vue`、`web/src/pages/Server.vue`、`web/src/pages/Client.vue`、`web/src/components/__tests__/AppLayout.spec.ts`、`web/src/pages/__tests__/Server.spec.ts`、`web/src/pages/__tests__/Client.spec.ts`、`scripts/baseline.json`、`docs/dev-map.md`
  - 未碰：后端 / store / 路由 / API / DB / Go / e2e spec / App.vue / LogViewer 子系统 / package.json（零新依赖）
- Next steps for user:
  - batch orchestrator 跑 `scripts/verify_all` 作硬闸门（预期 PASS / frontend_tests==600 / go_tests==342 / test_count==942），特别复核 e2e C.1（1280 视口保持展开）+ AppLayout 既有 8 例 + Server/Client 既有 spec 零回归。
  - 按批次约定 **未 commit / 未 push / 未 archive**，由 batch orchestrator 统一处理。
  - 本任务为批次 ux-ui-uplift-2026-05 最后一个，批次收尾。

## Insight

- 2026-05-31 · 应用外壳响应式断点宜抽**模块级单例 composable + 原生 `window.matchMedia`**（复刻 useTheme.ts 范式），优于 naive-ui `useBreakpoint`：后者分界点是 config-provider 的固定 `640/1024/1280…`，既不对齐业务想要的 768 阈值、边界方向（< 还是 <=）也不透明；原生 matchMedia 用 `(max-width:767.98px)` 让 768 整数判展开、阈值与 e2e 视口关系完全可控，零依赖。单例 + `initialized` 守卫使 change 监听全 app 仅注册一次（监听不在组件 onUnmounted 清理——单例存活整个生命周期，与 useTheme osThemeRef 同），无泄漏无重复 · evidence: T-067 web/src/composables/useViewport.ts + useViewport.spec listeners.length===1
- 2026-05-31 · "自动折叠只是默认态、必须尊重用户手动操作不锁死"的正确实现是 **`watch(isNarrow)` 非 immediate + collapsed 初值单独取 `ref(isNarrow.value)`**：watch 仅在视口真正跨阈值（isNarrow 值变化）时重置 collapsed；用户在同一断点区间内手动展开时 isNarrow 未变 → watch 不触发 → collapsed 保持用户值不被收回。**若误用 `watch(...,{immediate:true})` 或每渲染周期把 collapsed 设回 isNarrow，窄屏手动展开会被立即收回 = 锁死**——这是"默认态 vs 用户意图"两个关注点的正交分离（初值定默认、watch 管跨区间、手动操作在区间内优先）· evidence: T-067 AppLayout.vue:143-147 + 06 §Adversarial QA-ADV-3 失败假设
- 2026-05-31 · 引入"视口宽度自动折叠/隐藏"的 UI 行为时，**折叠阈值必须 < e2e 默认视口宽度**，否则 e2e（playwright `devices['Desktop Chrome']`=1280×720）会在默认视口被判窄屏、自动折叠隐藏菜单文本，让断言菜单/导航文本的烟雾测试（03-dashboard）假性失败。判别动作：开工前先读 `web/playwright.config.ts` 的 `devices`/viewport 定锚阈值上界（本任务 767.98 < 1280），并把"常量 < 1280"写成单测反向证伪（insight L34 的视口维度延伸：不止"e2e 是否点击该元素"，还要"该元素的可见性是否被视口相关逻辑改变"）· evidence: T-067 useViewport.ts NARROW_MAX_WIDTH=767.98 + 06 §Adversarial QA-ADV-6 + playwright.config.ts:21 devices['Desktop Chrome']
- 2026-05-31 · 表单固定像素宽改响应式用 **`width:100%; max-width:Npx`**（而非纯 `width:100%` 或纯改小 px）：宽屏 form-item 内容区 > Npx 故视觉=原 Npx 宽（桌面零回归），窄屏 form-item 收窄随之收窄不溢出；关键是 `max-width` 封顶让同 form-item 内并排的兄弟元素（如鉴权 Token 行的 input + "查看明文"按钮）在宽屏仍并排不被 input 占满整行推开。改 width style 对断言"DOM 文本/按钮/getExposed/apiGet 调用"的既有 spec 透明（这类 spec 不断言像素宽，grep `width|max-width` 0 命中即可确认零回归，insight L37 实读断言判别的又一例）· evidence: T-067 Server.vue/Client.vue width:100%+max-width + Server/Client.spec 无 width 断言 + 06 回归核查
