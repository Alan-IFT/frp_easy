# 01 需求分析 — T-061 clipboard-util-extract

> 阶段 1 / Requirement Analyst · mode: full · 中文产出

## 1. 目标（Goal）

把当前分散在 3 个 Vue 组件里近乎逐字重复的"剪贴板复制 + 临时 textarea/execCommand fallback"逻辑抽取为单一共享纯函数 util，消除重复代码，降低长期维护成本，偿还 T-058 (A) 决策 D1 记录的 backlog。

## 2. 在范围内行为（In-scope，可测）

1. 新建 `web/src/utils/clipboard.ts`，导出纯函数 `copyToClipboard(text: string): Promise<boolean>`。
2. `copyToClipboard` 行为契约：
   - 2.1 首选路径：`await navigator.clipboard.writeText(text)` 成功 resolve → 返回 `true`，**不**触发 textarea fallback。
   - 2.2 fallback 路径：当首选路径 reject（throw）时，创建临时 `<textarea>`（属性 `aria-hidden="true"`、离屏定位）、赋值 `text`、`appendChild` 到 `document.body`、`select()`、调 `document.execCommand('copy')`，返回该调用的布尔结果。
   - 2.3 fallback 内 `execCommand` 抛异常 → 返回 `false`（捕获，不外抛）。
   - 2.4 无论 fallback 成功或失败或抛错，临时 textarea 必须在 `finally` 块从 `document.body` 移除（不残留 DOM 节点）。
   - 2.5 util **不**调用 `message` / `useMessage`（组合式 hook 只能在组件 setup 内用）；util 只返回成功布尔，UI 反馈（toast）留各组件。
3. 三处复制点改为调用 util，message 文案与现状逐字一致（成功 `'已复制到剪贴板'`、失败 `'复制失败：请手动选择文本复制'`）：
   - 3.1 `FirewallHint.vue` `copyText(text)`：`const ok = await copyToClipboard(text); message[ok ? 'success' : 'error'](ok ? '已复制到剪贴板' : '复制失败：请手动选择文本复制'); return ok`。保留 `copyCmd`/`copyAll` 对返回 `ok` 的依赖（"已复制 ✓" / "已复制全部 ✓" 短暂态仅成功时触发）。
   - 3.2 `PublicIpDetector.vue` `copyText(text)`：同 3.1。保留 `copyIp` 对 `ok` 的依赖。
   - 3.3 `LogViewer.vue` `onCopy()`：保留 build text（`search.visibleLines` map raw join），改为 `const ok = await copyToClipboard(text); ok ? message.success('已复制到剪贴板') : message.error('复制失败：请手动选择文本复制')`。**LogViewer 可观察行为（成功调 `message.success`、失败调 `message.error`，各一次）必须字节不变**。
4. 抽取为纯 1:1 行为搬运，**无任何行为变更**（insight L42）。

## 3. 不在范围内（Out-of-scope）

- OOS-1 不改变任何复制成功/失败的用户可观察行为（toast 文案、短暂态、调用次数）。
- OOS-2 不引入新的复制入口或新组件。
- OOS-3 不动后端 / store / 路由 / API 契约 / 数据库。
- OOS-4 不重构 LogViewer 的 text 拼接逻辑（`search.visibleLines.map(...).join('\n')` 留在组件内，util 只接收已拼好的字符串）。
- OOS-5 不为 util 增加超出现状的新防御（如 `navigator.clipboard` 不存在时的预检、`document` 不存在的 SSR 防御）——现状三处实现均假设浏览器环境且直接 try `navigator.clipboard.writeText`，util 1:1 沿用。若未来需要新防御，单独任务 + 单独测试覆盖（L42）。
- OOS-6 不 git commit/push、不跑 archive-task（按用户要求）。

## 4. 边界条件（Boundary conditions）

- BC-1 空字符串 `text=''`：util 不对内容做特判，照常 `writeText('')` / textarea 赋空值；返回值由底层 API 决定。util 不抛错。
- BC-2 `navigator.clipboard.writeText` reject（内网 http 非安全上下文典型场景，insight L37）→ 走 fallback。
- BC-3 fallback `execCommand` 返回 `false`（浏览器拒绝）→ util 返回 `false`，组件弹 `message.error`。
- BC-4 fallback `execCommand` 抛异常（如 jsdom/happy-dom 默认无该 API）→ util 捕获并返回 `false`，不外抛。
- BC-5 任意路径下临时 textarea 不得残留在 `document.body`（`finally` removeChild）。
- BC-6 并发/连点：util 无共享可变状态（纯函数 + 局部 textarea），多次调用互不干扰。

## 5. 验收标准（Acceptance criteria，可验证）

- AC-1 `web/src/utils/clipboard.ts` 存在并导出 `copyToClipboard`；`scripts/verify_all` 编译/类型检查/eslint 通过。
- AC-2 新增 `web/src/utils/__tests__/clipboard.spec.ts`，覆盖：
  - writeText resolve → 返回 `true` 且 `execCommand` 未被调用（未走 fallback）。
  - writeText reject + execCommand 返回 `true` → 返回 `true`。
  - writeText reject + execCommand 返回 `false` → 返回 `false`。
  - writeText reject + execCommand 抛错 → 返回 `false`（无未捕获异常）。
  - 上述 fallback 各路径后断言 `document.body` 无残留 `textarea[aria-hidden="true"]`。
- AC-3 三组件既有 spec（`LogViewer.spec.ts`、`FirewallHint.spec.ts`、`PublicIpDetector.spec.ts`）**全绿、零回归**。特别复核 LogViewer 的 `AC-6 复制全部`（onCopy → writeText 收到拼接字符串）仍通过。
- AC-4 `scripts/baseline.json` 的 `frontend_tests` 与 `test_count` 同步 bump（净增 = clipboard.spec 用例数）；`scripts/verify_all` 的 B.4 测试计数闸门通过（只升不降）。
- AC-5 `06_TEST_REPORT.md` 含**裸** `## Adversarial tests` 段（无 `§N`/`N.` 前缀），含至少一条"clipboard reject + execCommand reject → `copyToClipboard` 返回 `false` 且无未捕获异常 + textarea 清理"反向证伪。
- AC-6 `docs/dev-map.md`「可复用工具」表新增 `web/src/utils/clipboard.ts` 一行。
- AC-7 改动文件集仅限：`web/src/utils/clipboard.ts`（新）+ `web/src/utils/__tests__/clipboard.spec.ts`（新）+ `LogViewer.vue` + `FirewallHint.vue` + `PublicIpDetector.vue` + `scripts/baseline.json` + `docs/dev-map.md` + 本任务 docs。未碰后端/store/路由/API。

## 6. 非功能需求（NFR）

- NFR-1 兼容性：util 行为对内网 http 非安全上下文（`navigator.clipboard` reject）必须有 fallback——这是项目复制功能的核心约束（insight L37），抽取后不得削弱。
- NFR-2 可维护性：单一实现，三处共享；未来改 fallback 策略只需改一处。
- NFR-3 测试稳定性：clipboard.spec 测试模拟须 `Object.defineProperty(navigator,'clipboard',{value:{writeText:mock}})` + 显式装 `document.execCommand`（jsdom/happy-dom 默认无），不依赖 naive-ui 组件名查询（util 无 UI，本身就无此风险；组件 spec 已遵守 L45）。

## 7. 关联任务（Related tasks）

- **T-058 frontend-interaction-polish**（`docs/features/frontend-interaction-polish/`，DELIVERED）：三处复制点统一为 LogViewer onCopy 范式，决策 **D1 刻意不抽 `utils/clipboard.ts`**（理由：抽取会改 LogViewer.vue 扩散 + 动其 onCopy 测试快照），记 backlog。本任务即偿还该 backlog——关键差异：T-058 时 LogViewer 的 onCopy 直接 mock `navigator.clipboard`，抽取后该 mock 仍命中 util 内部走的同一 API，故零回归（已由 orchestrator 核实三处 spec 的 mock 机制）。
- **T-048 / T-042 utils 抽取范式**：`web/src/utils/format.ts`、`proxyStatus.ts`——既有"组件内联逻辑 → 抽 utils + `__tests__/` + dev-map 可复用工具表登记"的标准范式，本任务复刻。

## 8. 给用户的开放问题（Open questions）

无。技术上下文已由 orchestrator 逐处核实，文案/行为/文件集/测试要求全部明确，无歧义。

## 9. 裁决（Verdict）

**READY** —— 无开放问题，可进入 Stage 2 方案设计。
