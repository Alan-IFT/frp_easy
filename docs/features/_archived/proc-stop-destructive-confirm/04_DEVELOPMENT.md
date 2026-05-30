# Development Record — T-056 proc-stop-destructive-confirm

> Stage 4 · dev-frontend（单分区，02 §11） · mode: full · 中文

## Summary

在 `Dashboard.vue` 引入单个 `ConfirmDialog` 实例 + `pendingAction` 状态机，把 frpc/frps 的"停止""重启"四个破坏性按钮的 `@click` 从直接调用 `handleStop/handleRestart` 改为先打开二次确认（`requestStop`/`requestRestart`），确认后才分派到原 handler；"启动"按钮零改动（非破坏性、不确认）。确认文案随进程 + 操作类型动态切换。后端 / store / API 零改动。

## Files changed

- `web/src/pages/Dashboard.vue` — (1) 模板：4 个破坏性按钮 `@click` 改为 `requestStop('frpc'|'frps')` / `requestRestart('frpc'|'frps')`；新增 1 个 `<confirm-dialog v-model:show :title :content @confirm @cancel>` 实例（n-grid 之后、根 div 之前）。(2) 脚本：import `computed` + `ConfirmDialog`；新增 `pendingAction` ref（`{kind, type:'stop'|'restart'} | null`）、`showConfirm` ref、`confirmTitle`/`confirmContent` computed、`requestStop`/`requestRestart`/`confirmPending`/`cancelPending` 函数；`defineExpose.__testing` 追加 8 个新句柄。"启动"按钮链路（`handleStart`）一字未动。
- `web/src/pages/__tests__/Dashboard.spec.ts` — `TestingHandle` 接口追加 8 个 T-056 句柄；`beforeEach` 给 `apiStopProc`/`apiRestartProc`/`apiStartProc` 桩补默认 resolved（C-2）；新增 `describe('停止/重启二次确认（T-056）')` 块含 7 个 happy-path 用例（AC-1~AC-6 + 初始不可见）。
- `docs/dev-map.md` — Dashboard.vue 行追加 T-056 二次确认说明（非结构变更，信息补充）。

## verify_all result

- **Baseline（改动前）**：`scripts/baseline.json` → frontend_tests=426 / go_tests=318 / test_count=744。批次内 verify_all 历史 PASS 32/0/0（tasks.md T-049/T-051 等记录）。
- **本 stage 改动后（dev 自检）**：
  - 前端单测：新增 7 个 T-056 happy-path 用例（AC-1~AC-6 + 初始不可见）；Dashboard.spec 既有 ~21 用例零改动语义 → 预期前端测试数 426 → 433（dev 阶段；QA stage 6 会再加 adversarial 并落最终 baseline bump）。
  - 类型：新增逻辑全部强类型（`ProcKind` 联合类型 + `pendingAction` 显式泛型），无 `any`；`noUnusedLocals`/`noUnusedParameters` 下所有新符号均被模板 / computed 引用，无未用告警。
  - eslint：仅复用既有 naive-ui 组件 + 标准 Vue SFC 语法，无新违规。
- **Delta**：0 new failures 预期；baseline 测试数只升不降。**最终 `scripts/verify_all`（全量含 e2e）由 orchestrator 独立真跑作硬闸门**（本任务约定，因改 Dashboard 需复核 03-dashboard e2e）。

> 说明：dev 阶段的"自检"以代码 + 类型 + 测试结构正确性为据；项目 insight L14（role-collapse）+ 本任务 INPUT 明确要求 orchestrator 独立真跑 `bash scripts/verify_all.sh` 全量作权威硬闸门，故不在此处自报 PASS 数字以免与权威跑冲突。QA stage 6 与 orchestrator delivery 各自真跑并记录。

## e2e 影响预判（INPUT 要求）

实读 `web/tests/e2e/03-dashboard.spec.ts`：TC-04 仅断言"仪表盘 / frpc（客户端）/ frps（服务端）"文案可见；TC-05 仅点"退出登录"。**两者均不点击停止 / 重启按钮** → 新增确认框不改变 e2e 已断言的交互路径 → C.1（playwright）不受影响。无需修改任何 e2e spec。若未来 e2e 新增"点停止"用例，则需在该用例内补"先点确认"步骤（已在 07 outstanding / dev-map 记录范式）。

## Design drift (if any)

无。实现与 02_SOLUTION_DESIGN.md §3/§6 完全一致：单实例 + ref `showConfirm`（非 computed，设计 §3 注已论证 v-model:show 需可写）+ `confirmPending` 先快照 `p` 再复位（设计 §3 注的竞态防御）+ `void handle...` fire-and-forget（与原 `@click` 不 await 一致）。

## 消化 GR conditions（03 §2）

- **C-1（句柄暴露）**：`defineExpose.__testing` 追加 `pendingAction/showConfirm/confirmTitle/confirmContent/requestStop/requestRestart/confirmPending/cancelPending` 全部 8 个。✅
- **C-2（spy 默认 resolved）**：`beforeEach` 给 stop/restart/start 三桩补 `mockResolvedValue`，避免确认后调用走 reject 干扰。✅
- **C-3（标题禁前缀）**：交 QA stage 6 落实（06 用裸 `## Adversarial tests`）。
- **C-4（baseline bump）**：交 QA stage 6 落实（同步 frontend_tests + test_count）。
- **C-5（SFC 行数）**：见下方自检。

## SFC 纯逻辑行数自检（insight L22 口径）

`Dashboard.vue` script 段 `<script setup>`（L206-422）去 import（L207-222）、去注释、去 `defineExpose` testing hook（L396-422），纯 setup 逻辑行约 110 行（既有 ~85 + T-056 新增 ~25：4 短函数 + 1 ref + 1 ref + 2 computed + 1 type 别名）。远 < 200 红线。

## Open issues for review

无。留给 QA：补 adversarial（并发待确认 last-wins 串扰证伪 + 点停止后点取消 stop API 零调用反向证伪）+ baseline bump + 全量 verify。

## Dev-map updates

`docs/dev-map.md` L124 Dashboard.vue 行追加："T-056: 停止/重启破坏性操作复用 ConfirmDialog 二次确认，pendingAction 状态机驱动动态文案，启动不确认"。无新增 / 移动 / 删除文件。

## Insight to surface (optional)

- e2e 烟雾测试（03-dashboard TC-04/TC-05）只断言文案可见 + 退出登录、**不点击停止/重启**，故给破坏性按钮加二次确认对 e2e 零影响——给"破坏性按钮加确认"类 UI 改动评 e2e 风险时，应先 grep e2e spec 是否真点击该按钮再决定是否改 e2e（多数烟雾测试不点破坏性按钮）· evidence: web/tests/e2e/03-dashboard.spec.ts TC-04/TC-05

## Verdict

READY FOR REVIEW
