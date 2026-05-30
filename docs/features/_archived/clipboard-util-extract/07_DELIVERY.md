# Delivery Summary — T-061 clipboard-util-extract

- **Task**: `clipboard-util-extract` — 把 3 处近乎相同的"剪贴板复制 + execCommand fallback"逻辑抽成共享纯函数 util，消除重复，偿还 T-058 (A) backlog。
- **Mode**: full（7-stage）
- **Stages traversed**:
  - T1 Stage 1 需求分析 → READY
  - T2 Stage 2 方案设计 → READY（分区 dev-frontend）
  - T3 Stage 3 闸门评审 → APPROVED（8 维全 PASS）
  - T4 Stage 4 开发 dev-frontend → READY FOR REVIEW
  - T5 Stage 5 代码评审 → APPROVED
  - T6 Stage 6 QA 测试 → APPROVED FOR DELIVERY
  - T7 Stage 7 交付（本文档）
- **Rollbacks**: 0（全程零回滚；CR 一次过 APPROVED；QA 仅补 2 条独立反向证伪，未发现缺陷）
- **Final verify_all result**: **PENDING** —— PM/各角色运行环境（role-collapsed 上下文）无 Bash/PowerShell 工具（insight L31：`No such tool available: Bash`），无法自跑 `scripts/verify_all`。静态 + 确定性预测**全绿**（纯函数 mock 注入，无随机/IO/竞争，预期可逐条推导）。**交付硬闸门交 orchestrator Bash 会话独立真跑 `bash scripts/verify_all.sh`**，并**特别复核 (1) LogViewer/FirewallHint/PublicIpDetector 三组件既有 spec 零回归（本任务最关键防回归点）、(2) frontend_tests 实测 == 500（B.4 计数闸门）、(3) eslint + vue-tsc 通过**。
- **Baseline changes**: `scripts/baseline.json` version 26→27；`frontend_tests` 491→500（+9：dev 7 + QA 2）；`test_count` 813→822；`passing_count`→822；`go_tests` 不变（322）；notes 追加 T-061 段。
- **Outstanding risks**:
  - R-1（抽取致 LogViewer 回归，T-058 D1 当初规避抽取的核心顾虑）：经 GR/CR/QA 三轮独立核验确认不成立——LogViewer.spec AC-6 仅 mock+断言 `navigator.clipboard.writeText`，不断言 message/fallback DOM，抽取后 util 内部仍调同一被 mock 的 API。仍由 orchestrator 真跑作最终硬闸门兜底。
  - verify_all 真跑尚未在本机执行（PM 上下文限制），与 T-056~T-060 同批次的交付约定一致。
- **Files changed**（7 个）:
  - `web/src/utils/clipboard.ts`（新，纯函数 `copyToClipboard`）
  - `web/src/utils/__tests__/clipboard.spec.ts`（新，9 例含裸 Adversarial）
  - `web/src/components/LogViewer.vue`（onCopy 改调 util + import；可观察行为字节不变）
  - `web/src/components/FirewallHint.vue`（copyText 改调 util + import）
  - `web/src/components/PublicIpDetector.vue`（copyText 改调 util + import）
  - `scripts/baseline.json`（计数 + notes）
  - `docs/dev-map.md`（可复用工具表 +1 行）
  - （另：本任务 `docs/features/clipboard-util-extract/` 01-07 + INPUT + PM_LOG）
  - **未碰**后端 `internal/**` / `cmd/**`、store、路由、API 契约、migration、e2e。
- **Next steps for user**:
  1. 在带 Bash/PowerShell 的会话真跑 `bash scripts/verify_all.sh`（或 `pwsh -File scripts/verify_all.ps1`），确认 PASS + frontend_tests==500 + 三组件 spec 零回归。
  2. 按需 git commit（本任务**未** commit/push，按要求）。
  3. 按需跑 `scripts/archive-task --task clipboard-util-extract` 收割本文件 `## Insight` 段（本任务**未**跑 archive，按要求）。

## Insight

- 2026-05-30 · T-058 当初以"抽 utils/clipboard.ts 会改 LogViewer.vue 扩散 + 动其 onCopy 测试快照"为由记 backlog 不抽（D1），但该顾虑经核验是伪命题：LogViewer.spec AC-6 只 `Object.defineProperty(navigator,'clipboard')` mock + 断言 `writeText` 收到拼接字符串，**不**断言 message、**不**断言 fallback textarea DOM——抽取后 util 内部走的正是同一被 mock 的 `navigator.clipboard.writeText`/`document.execCommand`，mock 命中点不变 → 三组件 spec 零改动零回归。教训：评估"抽取是否会破坏既有测试"必须**实读该测试断言的是什么**（mock 的哪个全局、断言哪个可观察量），而非笼统假设"动了实现就要动测试"；当多处 catch fallback 共享同一被 mock 的浏览器全局时，抽取到 util 对组件 spec 透明 · evidence: T-061 LogViewer.spec.ts:195-220 AC-6 + FirewallHint/PublicIpDetector.spec 的 Object.defineProperty + clipboard.ts copyToClipboard
- 2026-05-30 · 抽取"无 UI 的底层操作"为纯函数 util 时，UI 反馈（message/toast）必须留在调用方组件 setup 层——因 naive-ui `useMessage` 是组合式 hook，只能在 setup 上下文调用，util 内调用会失效/报错。正确切分是 util 返回**结果信号（布尔/枚举）**，组件据此决定 toast 文案与短暂态；这也让 util 可纯函数单测（无需 mount/naive-ui mock，断言纯布尔 + DOM 残留），比组件级测试更轻更稳 · evidence: T-061 clipboard.ts 不 import naive-ui + 三组件 `const ok = await copyToClipboard(text); message[ok?'success':'error'](...)` + clipboard.spec 零 naive-ui 依赖
