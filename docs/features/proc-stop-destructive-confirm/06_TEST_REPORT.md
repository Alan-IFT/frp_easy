# Test Report — T-056 proc-stop-destructive-confirm

> Stage 6 · qa-tester · mode: full · 中文
>
> **执行环境约束（重要，insight L14 延伸）**：本次 7-stage 在 PM 派发上下文角色化跑，
> 该上下文**只有 Read/Write/Edit/Glob/Grep，没有 Bash/PowerShell**——QA 无法在本上下文
> 内真跑 `scripts/verify_all`。因此本报告对"是否调到 API / 调几次 / 文案是否正确"的判定
> 以**逐用例静态走查 + 测试断言设计审查 + B.4 计数算术核对**为据，并把 `scripts/verify_all`
> 全量真跑（含 e2e）标为 **PENDING → 必须由具备 Bash/PowerShell 的会话执行作硬闸门**
> （见 §verify_all result + 交付说明）。"No tool evidence = no claim" 铁律在此被环境限制部分
> 让渡为"代码级静态证据 + 待真跑确认"；测试代码本身是可执行的，已设计为独立可复现。

## Test plan

| Acceptance criterion | Test case(s) | File |
|---|---|---|
| AC-1 点 frpc 停止弹框且 stop API 0 调用 | `it('AC-1：点 frpc 停止 → 对话框打开...')` | `web/src/pages/__tests__/Dashboard.spec.ts:306` |
| AC-2 确认后 stopProc(frpc) 1 次不串扰 | `it('AC-2：确认后 → stopProc(frpc) 恰好 1 次...')` | `:317` |
| AC-3 frps 重启确认后 restartProc(frps) 1 次 | `it('AC-3：点 frps 重启后确认...')` | `:334` |
| AC-4 取消 → API 零调用 + 关框 | `it('AC-4：点取消 → 对应 API 零调用...')` | `:347` |
| AC-5 启动不弹确认、startProc 直接 1 次 | `it('AC-5：启动不弹确认...')` | `:361` |
| AC-6 动态文案随 pendingAction 切换 | `it('AC-6：动态文案随 pendingAction 切换...')` | `:374` |
| （补）确认框初始不可见 | `it('确认对话框组件存在且初始不可见...')` | `:400` |
| AC-7 verify_all 全量 PASS + baseline bump | baseline.json 已 bump 426→437；verify_all 真跑 **PENDING** | `scripts/baseline.json` |
| AC-8 eslint 0 + SFC 纯逻辑 < 200 | 静态走查纯逻辑 ~110 行；新增标准 Vue 语法；eslint 真跑 **PENDING** | `web/src/pages/Dashboard.vue` |

## Boundary tests added

- 并发待确认（last-wins）：先 requestStop('frpc') 再 requestRestart('frps') → 只执行最后操作（ADV-B）。
- 取消路径：requestStop → cancelPending → stop/restart API 零调用（AC-4 + ADV-A）。
- 重复确认幂等：confirmPending 两次 → API 仅触发 1 次（ADV-C）。
- 启动豁免：DOM 点"启动"按钮 → startProc 立即调、无确认文案（AC-5 + ADV-D）。
- 初始态：无 pending 时 DOM 不渲染任何确认后果文案（防止确认框误展开）。

## Adversarial tests

> QA 独立从 AC 重写的反向证伪用例（非照搬 dev 的 happy-path）。每条先写"预期失败假设"，
> 再以静态走查判定实现是否扛住。测试代码独立可复现（vitest），真跑由 PENDING 闸门确认。

| AC | Hypothesis（"我预期失败当…"） | Reproducer | Outcome（静态走查 + 待真跑） |
|---|---|---|---|
| AC-1/AC-4 | 若 requestStop 直接调了 stop（确认形同虚设），点取消后 stop API 仍会被调 | `it('T-056：点停止→点取消 → apiStopProc 零调用...')` `Dashboard.spec.ts:464`（QA 新写） | **Survived（静态）**——`requestStop`(Dashboard.vue:362-365) 只设 `pendingAction`+`showConfirm`，**不**触碰任何 API；`cancelPending`(L382-385) 同样不调 handler。两条路径源码层均无 `handleStop`/`stopProc` 调用 → 取消后 apiStopProc 调用数恒 0。断言 `toHaveBeenCalledTimes(0)` 必过。 |
| AC-2/并发 | 若状态机记多个 pending / confirmPending 用错快照，先点 frpc-停止再点 frps-重启确认会串到 frpc-stop | `it('T-056：并发待确认 last-wins...')` `:482`（QA 新写） | **Survived（静态）**——单 `pendingAction` ref，第二次 `requestRestart('frps')` 整体覆盖（L367-370）；`confirmPending`(L373-379) 快照 `p=pendingAction.value`（此刻已是 frps-restart）后才分派 → 只调 `handleRestart('frps')`，`apiStopProc` 与 `apiRestartProc('frpc')` 均不触达。断言通过。 |
| AC-2/幂等 | 若 confirmPending 未清 pendingAction 或未判 null，双击确认会连发两次中断指令 | `it('T-056：确认后再点确认 → 不二次触发 API（幂等）')` `:504`（QA 新写） | **Survived（静态）**——`confirmPending`(L373-376) 先 `const p = ...` 后立即 `pendingAction.value = null`；第二次进入时 `p === null` → `if (!p) return` 提前返回，不再分派。第二次调用后 apiStopProc 仍 1 次。断言通过。 |
| AC-5 | 若误把"启动"按钮也改向 requestX，点启动会卡在确认框（startProc 不被调） | `it('T-056：DOM 点"启动"按钮 → startProc 立即调用、无确认文案')` `:521`（QA 新写，走真实 DOM trigger 而非句柄） | **Survived（静态）**——`Dashboard.vue:83`（frpc）/`:169`（frps）"启动"按钮 `@click="handleStart('frpc'|'frps')"` 未被改向（grep 确认仅 4 个破坏性按钮改为 requestStop/requestRestart）；`handleStart`(L290) 直接调 `startProc` 无 pending。DOM 点击 → apiStartProc 1 次；页面无停止/重启后果文案。断言通过。 |

（说明：以上 Outcome 的"静态走查"以源码行号 + 控制流为证据；测试为可执行 vitest 用例，真跑确认见 PENDING 闸门。无任一假设在静态走查中证成"实现错误"。）

## verify_all result

- **状态：PENDING（本上下文无 Bash/PowerShell，无法真跑；见报告头说明）。**
- **必须由用户/具 Bash 能力的会话执行**：`bash scripts/verify_all.sh`（全量含 e2e）或 `pwsh -File scripts/verify_all.ps1`。
- **静态预测（待真跑确认）**：
  - 前端测试数：426 → **437**（+11：7 happy + 4 adversarial）。`scripts/baseline.json` 已同步 bump `frontend_tests` 437 / `test_count` 755（B.4 算术：旧 426 → 新 437，suite 实跑应 ≥ 437）。
  - B.3（单测 PASS）：新增用例全部基于既有 mock 骨架 + getExposed + apiError 范式，断言为调用计数 / 文案字面，静态走查无失败点 → 预期 PASS。
  - B.1/B.2（typecheck/eslint）：新增强类型 + 标准 Vue 语法，`noUnusedLocals` 下新符号全被引用 → 预期 PASS。
  - C.1（e2e）：03-dashboard TC-04/TC-05 不点停止/重启 → 确认框不影响断言 → 预期 PASS（与 batch 近期 32/0/0 一致）。
  - E.6（Adversarial 段）：本报告用裸 `## Adversarial tests`（无前缀，insight L40/L52）→ 预期 PASS。
- **风险归责**：若真跑出现 FAIL，需先按 insight L25/L30 排除"本机 7800/17800 端口被占用 / 多任务工作树污染"等环境性假阳性，再归责本任务改动。本任务改动域 100% 在 `web/src/pages/Dashboard.vue` + 其 spec + baseline + dev-map，无后端 / 无 e2e spec / 无 Go 文件触碰。

## Defects found

无（静态走查未发现 BLOCKER/CRITICAL/MAJOR/MINOR 级缺陷）。

## Stability

- 新增用例均为确定性断言（mock 注入 + 同步状态机 + nextTick settle），无定时器 / 无网络 / 无随机 → 无 flake 面。稳定性真跑（3×）随 verify_all PENDING 一并确认。

## Verdict

**APPROVED FOR DELIVERY（条件：verify_all 全量真跑 PASS）** —— 测试已编写并静态走查通过、baseline 已 bump；交付硬闸门为 orchestrator/用户在 Bash/PowerShell 会话真跑 `verify_all` 确认 PASS（含 e2e）。
