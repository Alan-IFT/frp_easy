# 代码评审 — T-067 · responsive-layout

> Harness Stage 5 · Code Reviewer · 全中文 · 独立视角，逐条走查需求与设计

## Files reviewed

- `web/src/composables/useViewport.ts`（新）
- `web/src/components/AppLayout.vue`（编辑）
- `web/src/pages/Server.vue`（编辑）
- `web/src/pages/Client.vue`（编辑）
- `web/src/composables/__tests__/useViewport.spec.ts`（新）
- `web/src/components/__tests__/AppLayout.spec.ts`（编辑）
- `web/src/pages/__tests__/Server.spec.ts`（编辑）
- `web/src/pages/__tests__/Client.spec.ts`（编辑）

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### NIT
- [STYLE] `web/src/components/__tests__/AppLayout.spec.ts` — 窄屏 describe 块内重新定义了一份 `FakeMql`/`makeFakeMql`，与 useViewport.spec.ts 内的同名工厂重复。可抽到 test-utils 共享，但两处用途略不同（一处测 composable、一处测组件挂载）且抽取会引入跨 spec 依赖——保留就地复制（与项目既有"受控 mock 就地定义"风格一致，T-066 useTheme.spec / AppLayout.spec 也各自定义 osThemeRef）。不阻塞。
- [MAINT] `web/src/composables/useViewport.ts` — `addListener` 老 Safari 兼容分支在 happy-dom / 现代环境永不命中，属防御性代码。保留合理（与浏览器兼容目标一致，零运行时成本），无需删。

## Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 verify_all PASS | 交 batch orchestrator 真跑（PENDING 执行规格） | ⏳ 待真跑 |
| AC-2 窄屏 collapsed=true / 宽屏=false | `useViewport.ts` isNarrow + `AppLayout.vue:143-147` collapsed 初值+watch；`AppLayout.spec` 窄/宽初始 2 例 + `useViewport.spec` 初值 2 例 | ✅ |
| AC-3 窄屏仍可手动展开不锁死 | `AppLayout.vue:145-147` watch 仅跨阈值触发（isNarrow 未变则 collapsed 不被覆盖）+ 保留 `@expand/@collapse`/`show-trigger`；设计 §6 精确论证 | ✅（逻辑正确，见下方逻辑走查；测试由 QA stage 6 补对抗用例锁死） |
| AC-4 表单 max-width 化 | `Server.vue` 5 处 + `Client.vue` 3 处 `width:100%; max-width:Npx`；`Server.spec`/`Client.spec` 各 2 例（正向 + 反向证伪无裸像素宽） | ✅ |
| AC-5 顶栏不溢出 + 关键入口可达 | `AppLayout.vue:3` n-space `:wrap="true"` + 版本号 `!isNarrow` 隐藏 + min-height；`AppLayout.spec` 顶栏关键入口 1 例 | ✅ |
| AC-6 e2e 视口 > 阈值零回归 | `useViewport.ts` NARROW_MAX_WIDTH=767.98 < 1280；`useViewport.spec` 常量 < 1280 断言；CR 已 grep 03-dashboard 确认 | ✅ |
| AC-7 既有 spec 零回归 | AppLayout 既有 8 例不动（默认 happy-dom 宽屏 → 展开）；Server/Client 既有 spec 无 width 断言 | ✅（CR 实读断言确认） |
| AC-8 baseline bump | dev +18，QA stage 6 后 PM 统一 bump（C-4） | ⏳ stage 6 后 |

## Design fidelity check

| Design item | Implementation | Status |
|---|---|---|
| useViewport.ts 模块单例 + matchMedia 767.98 + safeMatchMedia null 降级 + initialized 守卫 | `useViewport.ts` 逐条吻合 02 §3 | ✅ |
| 复刻 useTheme 模块单例范式不抽公共 util | 模块级 isNarrow ref + 惰性 init，独立文件不耦合 useTheme | ✅ |
| AppLayout collapsed=ref(isNarrow.value) 初值 + watch（非 immediate，C-3） | `AppLayout.vue:144-147` 正是此写法，无双重赋值 | ✅ |
| FR-2 共存：手动 trigger 保留 | `@collapse/@expand`/`show-trigger` 未删（既有模板行保留） | ✅ |
| 顶栏 n-space wrap + 版本号窄屏隐藏（FR-4/FR-5） | `:wrap="true"` + `appStore.version && !isNarrow` | ✅ |
| 内容区 padding 响应式（FR-8） | `:style="{ padding: isNarrow ? '12px' : '24px' }"` | ✅ |
| Server/Client max-width（C-2 保留并排） | `width:100%; max-width:Npx`，Token 行 input 上限封顶按钮并排 | ✅ |
| 不改菜单/路由/menuIcon a11y/主题 n-select/退出按钮（OOS-3） | grep 确认 menuOptions/activeKey/n-select/退出按钮文本未动 | ✅ |
| App.vue 不改（设计 §10） | App.vue 未在改动列表 | ✅ |

## 逐维走查

1. **逻辑正确性**：
   - **FR-2 不锁死核心逻辑**：`watch(isNarrow, n => collapsed.value = n)` 是非 immediate（Vue watch 默认）。用户窄屏手动展开 → `collapsed=false`，此时 `isNarrow` 仍 true（视口未变）→ watch 回调不触发 → collapsed 保持 false（展开）。仅当视口真正跨 768 阈值时 isNarrow 变化才触发 watch 重置。逻辑正确，无锁死。
   - **初值**：`collapsed = ref(isNarrow.value)` —— 首次渲染即反映断点默认态。useViewport() 在 ref 之前调用，isNarrow 已由 init() 读 matchMedia.matches 确定。正确。
   - **边界**：NARROW_MAX_WIDTH=767.98 让 `(max-width:767.98px)` 在 768 整数判 false（展开），1280 判 false（展开，e2e 零回归）。BC-1/BC-2 正确。
   - **降级**：safeMatchMedia 在无 matchMedia / 抛错时返 null → init 早返 → isNarrow 留 false（展开）→ 退化为既有行为。BC-6 正确。
   - **监听守卫**：`initialized` 布尔守卫 + 模块单例 → matchMedia change 监听只注册一次（spec 断言 listeners.length===1）。NFR-4/BC-5 无泄漏无递归。
2. **需求保真**：AC-2~AC-7 全部有实现 + 测试覆盖（见上表）。AC-3 不锁死的对抗证伪由 QA stage 6 补（CR 已确认生产逻辑正确）。
3. **设计保真**：零 drift（见 design fidelity 表，逐条吻合）。
4. **性能**：matchMedia 监听全 app 仅 1 个，change 回调 O(1) 赋值，无热路径同步 IO、无 N+1、无无界循环。
5. **安全**：无新输入处理、无密钥、无序列化、无 SQL。纯布局 CSS + 一个 boolean ref。无安全面。
6. **可维护性**：useViewport 命名清晰、注释只写 WHY（边界 767.98 理由 / 单例不清理理由）；无死代码（addListener 兼容分支属防御性，NIT 已记）；不过度抽象（不抽公共 util，与设计一致）。

## 测试有意义性核查（红线 4）

- useViewport.spec：用受控 FakeMql 的 `fire()` 真模拟视口跨阈值并断言 isNarrow 翻转——非 shape-matching，是行为验证。降级用例真删 matchMedia/抛错断言不崩。监听守卫真数 listeners 长度。常量边界用 `< 1280` 反向证伪 e2e 回归风险。
- AppLayout.spec 窄屏块：用 `.n-layout-sider--collapsed` class（渲染后可观察量，DOM 查询非组件名查询，insight L45）断言折叠态；`fire()` 真触发自动折叠/展开。
- Server/Client.spec：断言渲染后控件 inline style 含 max-width + width:100%，反向证伪"无裸像素宽残留"——锁死 AC-4 意图。
- 断言全用可观察量（isNarrow 布尔 / sider class / style 字符串 / DOM 文本），符合 insight L45。

## Verdict

**APPROVED**（0 CRITICAL / 0 MAJOR / 0 MINOR，2 NIT）

代码逻辑正确（FR-2 不锁死核心逻辑经走查确认）、需求全覆盖、设计零 drift、测试有意义。2 NIT 不阻塞。进入 Stage 6 QA。
