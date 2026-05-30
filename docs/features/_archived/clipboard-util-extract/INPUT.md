# INPUT — T-061 clipboard-util-extract

- **mode**: full（7-stage）
- **slug**: `clipboard-util-extract`

## 一句话目标

把 3 处近乎相同的"剪贴板复制 + execCommand fallback"逻辑抽成共享纯函数 util，消除重复（DRY / 长期易维护），偿还 T-058 (A) 记录的 backlog。

## 精确技术上下文（orchestrator 已逐处核实）

当前 3 处实现 clipboard + 临时 textarea/execCommand fallback 逻辑高度重复：
- `web/src/components/LogViewer.vue:145-171` `onCopy`（build text → clipboard → message.success；catch → textarea + execCommand → message.success/error）。用 `const message = useMessage()`（L96）。
- `web/src/components/FirewallHint.vue:81-` `copyText(text): Promise<boolean>`（T-058 加，同款 fallback，内部 message + 返回 ok 供"已复制 ✓"短暂态）。
- `web/src/components/PublicIpDetector.vue:67-` `copyText(text): Promise<boolean>`（同款）。
- 既有 utils：`web/src/utils/format.ts`、`proxyStatus.ts`（+ `__tests__/`）。

三处复制成功文案均为 `'已复制到剪贴板'`，失败文案均为 `'复制失败：请手动选择文本复制'`，textarea fallback 实现逐字相同（`aria-hidden` 离屏 + select + execCommand('copy') + finally removeChild）。

## 修复方向（dev-frontend 落地，勿扩散）

1. 新建 `web/src/utils/clipboard.ts`，导出**纯函数** `copyToClipboard(text: string): Promise<boolean>`：
   - try `await navigator.clipboard.writeText(text)` → return true。
   - catch → 创建临时 textarea（`aria-hidden`，离屏）+ select + `document.execCommand('copy')`，return 该结果；任何异常 return false；finally 移除 textarea。
   - **util 内不调用 `message`**（`useMessage` 是组合式 hook，只能在组件 setup 用）—— message 留在各组件，util 只返回成功布尔。
2. 三处组件改为调用 util，**message 文案与现状逐字一致**（成功 `'已复制到剪贴板'`、失败 `'复制失败：请手动选择文本复制'`）：
   - FirewallHint/PublicIpDetector：`copyText` 体改为 `const ok = await copyToClipboard(text); message[ok?'success':'error'](ok?'已复制到剪贴板':'复制失败：请手动选择文本复制'); return ok`（保留"已复制 ✓"短暂态对 ok 的依赖）。
   - LogViewer.onCopy：build text 后 `const ok = await copyToClipboard(text); message[ok?'success':'error'](...)`。**LogViewer 可观察行为（成功/失败各调 message）必须字节不变**，确保其既有测试零回归。
3. **insight L42 原则**：抽取时先 1:1 行为搬运，新增防御（若有）单独标注 + 测试覆盖。本任务是纯搬运，无行为变更。

## 要求

1. 完整 7 阶段文档到 `docs/features/clipboard-util-extract/`（01-07 + PM_LOG），中文。
2. **补测试**（测试数只升不降，断言**不依赖 naive-ui 组件名查询**——本批 T-057 踩坑）：
   - `web/src/utils/__tests__/clipboard.spec.ts`：clipboard.writeText resolve → true（且未走 fallback）；reject + execCommand 返回 true → true；reject + execCommand 返回 false → false；reject + execCommand 抛错 → false；断言 textarea 被清理（document.body 无残留）。
   - 三组件既有 spec 仍绿（行为不变）——若有 spec 直 mock `navigator.clipboard`，确认仍通过。
   - **同步 bump `scripts/baseline.json`** `frontend_tests` + `test_count`。
3. 06 含**裸** `## Adversarial tests` 段（禁前缀）：一条"clipboard reject + execCommand reject → copyToClipboard 返回 false 且无未捕获异常 + textarea 清理"反向证伪。
4. **不要 git commit/push、不跑 archive-task**。
5. 07 含裸 `## Insight` 段。更新 `docs/dev-map.md`「可复用工具」表加 `utils/clipboard.ts` 一行。
6. 红线：不编辑 `.claude/`/`CLAUDE.md`/`.github/`；eslint；不破坏 LogViewer 既有测试。

## 验证

orchestrator 独立真跑 `bash scripts/verify_all.sh` 作硬闸门，**特别复核 LogViewer 相关 spec 零回归**。e2e 预判：纯 util 抽取无 e2e 影响——04 确认。
