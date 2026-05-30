# 开发记录 — Frontend partition · T-067 · responsive-layout

> Harness Stage 4 · dev-frontend · 全中文 · 上游 03 verdict=APPROVED FOR DEVELOPMENT

## Partition

dev-frontend — owns: `web/**`（+ `scripts/baseline.json` / `docs/dev-map.md` 文档元数据，按既有任务惯例 dev 一并 bump）。本任务全部改动在 owned paths 内，无越界（无后端 / DB）。

## Files changed（本分区）

- `web/src/composables/useViewport.ts` — **新建**：模块单例 composable，原生 matchMedia `(max-width:767.98px)` → `isNarrow: Ref<boolean>`。NARROW_MAX_WIDTH=767.98（< 1280 e2e 视口，BC-2）；safeMatchMedia null 降级（BC-6）；initialized 守卫监听只注册一次（NFR-4/BC-5）；复刻 useTheme 模块单例范式，不抽公共 util（避免反向耦合 theme/viewport 两域）。
- `web/src/components/AppLayout.vue` — 编辑：
  - import `useViewport` + vue `watch`；`const { isNarrow } = useViewport()`；`collapsed = ref(isNarrow.value)` 初值（C-3）+ `watch(isNarrow, n => collapsed.value = n)`（非 immediate，仅跨阈值触发，FR-1/FR-3）。手动 `@expand/@collapse`/`show-trigger` 保留共存（FR-2 不锁死：同区间手动展开时 isNarrow 未变 watch 不触发）。
  - 顶栏 `n-layout-header` `height:56px`→`min-height:56px` + padding `0 16px`→`8px 16px`（换行不裁切）；`n-space` 加 `:wrap="true"`（FR-4 优雅换行）；版本号 `n-text` 加 `&& !isNarrow`（窄屏隐藏非关键元素，FR-4）。
  - 内容区 `n-layout-content` padding `24px`→`:style="{ padding: isNarrow ? '12px' : '24px' }"`（FR-8 可选优化）。
  - **未改**：menuOptions / menuIcon a11y（T-064）/ 主题切换 n-select（T-066）/ 退出登录按钮文本与位置 / 路由 / activeKey / 二进制横幅逻辑。
- `web/src/pages/Server.vue` — 编辑：5 处 `style="width: Npx"` → `style="width: 100%; max-width: Npx"`（bindPort 200 / authToken 360 / dashboardPort 200 / dashboardUser 240 / dashboardPass 240）。C-2：max-width 封顶使鉴权 Token 行 input 不占满整行、"查看明文"按钮仍并排（input 与 button 平级在 form-item 内，max-width 上限保留并排观感）。
- `web/src/pages/Client.vue` — 编辑：3 处 `style="width: Npx"` → `style="width: 100%; max-width: Npx"`（serverAddr 300 / serverPort 200 / authToken 360）。同 C-2 并排保留。
- `web/src/composables/__tests__/useViewport.spec.ts` — **新建**：9 用例（C-1 vi.stubGlobal matchMedia 受控 MQL + vi.resetModules + 动态 import 全新单例）。
- `web/src/components/__tests__/AppLayout.spec.ts` — 编辑：+5 用例（4 自动折叠/共存 + 1 顶栏关键入口）。既有 8 例（T-064 菜单 5 + T-066 主题 3）不动（默认 happy-dom 宽屏 matchMedia matches=false → isNarrow=false → 展开，零回归）。
- `web/src/pages/__tests__/Server.spec.ts` — 编辑：+2 用例（max-width 正向 + 反向证伪无裸像素宽）。
- `web/src/pages/__tests__/Client.spec.ts` — 编辑：+2 用例（同上）。
- `scripts/baseline.json` — 编辑：bump frontend_tests / test_count / version + 修复既有结构（清理第 11-12 行冗余尾巴，C-4）。**最终计数在 stage 6 QA 补对抗用例后由 PM 统一 bump**（dev +18，QA 待定）。
- `docs/dev-map.md` — 编辑：composables 表加 useViewport 行 + AppLayout / Server / Client 行加响应式注。

## Out-of-partition coordination

无。纯前端单分区，无需后端 / DB 协调。

## 设计保真度

- 无 DESIGN DRIFT。完全按 02 §3/§6 实现：useViewport 模块单例 + matchMedia 767.98 + AppLayout watch + 顶栏 wrap + 表单 max-width。
- C-1（测试 vi.stubGlobal matchMedia + resetModules 全新单例）：已落实于 useViewport.spec + AppLayout.spec 窄屏 describe 块。
- C-2（max-width 保留并排）：已用 `width:100%; max-width:Npx`。
- C-3（collapsed ref(isNarrow.value) 初值 + 非 immediate watch）：已落实。
- C-4（baseline 结构修复 + bump）：PM 在 stage 6 后统一处理（含清理冗余尾巴）。
- C-5（顶栏 wrap 不破 e2e）：wrap 只窄屏换行，1280 视口顶栏不换行；退出按钮文本/role/位置未改（T-066 主题 n-select 仍在退出按钮前，未动）；版本号窄屏隐藏不影响 e2e（e2e 1280 视口展示版本号，且 e2e 不断言版本号）。

## 测试新增明细（dev 18）

| 文件 | 数量 | 覆盖 |
|---|---|---|
| useViewport.spec.ts（新） | 9 | 窄/宽初值 2 + 跨阈值响应式 2 + 单例引用/监听守卫 2 + 无 matchMedia/抛错降级 2 + 常量 < 1280 边界 1 |
| AppLayout.spec.ts（+） | 5 | 窄屏初始折叠 / 宽屏初始展开 / 宽→窄自动折叠 / 窄→宽自动展开 + 顶栏关键入口始终存在 |
| Server.spec.ts（+） | 2 | max-width 正向（width:100%+max-width）+ 反向证伪无裸像素宽 |
| Client.spec.ts（+） | 2 | 同上 |

## verify_all result

**PENDING**（dev role-collapsed 上下文无 Bash/PS，insight L31）。

执行规格（交 batch orchestrator Bash 会话真跑作硬闸门）：
- 预期 `scripts/verify_all` **PASS**。
- `frontend_tests` dev 后 = 576 + 18 = **594**（QA stage 6 补对抗用例后再增；PM 统一 bump baseline 到最终值）。
- `go_tests` = 342（不变，未碰 Go）。
- e2e（C.1）：playwright `devices['Desktop Chrome']` 视口 1280 ≥ 768 阈值 → 侧栏保持展开、菜单文本可见，03-dashboard TC-04/TC-05 零回归（静态核实，已 grep 确认 TC-04 断言 Dashboard 页正文 + '仪表盘'.first()、TC-05 按 name '退出登录' 点击）。
- 特别复核：AppLayout 既有 8 例 + Server/Client 既有 spec 零回归。

## Verdict

**READY FOR REVIEW**（frontend partition complete）
