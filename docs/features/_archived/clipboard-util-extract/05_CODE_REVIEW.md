# 05 代码评审 — T-061 clipboard-util-extract

> 阶段 5 / Code Reviewer · mode: full · 中文产出
> 上游 04_DEVELOPMENT.md Verdict = READY FOR REVIEW ✓

## Files reviewed（逐一实读）

- `web/src/utils/clipboard.ts`（新）
- `web/src/utils/__tests__/clipboard.spec.ts`（新）
- `web/src/components/LogViewer.vue`（onCopy L146-156 + import L88）
- `web/src/components/FirewallHint.vue`（copyText L70-74 + import L39）
- `web/src/components/PublicIpDetector.vue`（copyText L70-74 + import L48）
- `scripts/baseline.json`（计数 + notes）
- `docs/dev-map.md`（可复用工具表 L175）

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### NIT
- [STYLE] `web/src/utils/clipboard.ts` — `catch {}` 空捕获块（首选路径失败、execCommand 抛错）无 `void e`，但 ESLint 配置允许无参 catch（与三处原实现一致），且语义清晰（任何失败均回落/置 false）。属设计 OOS-5 范围内的 1:1 搬运，不阻塞。

## 逐维审计

1. **逻辑正确性 — PASS**
   - 首选路径 `await navigator.clipboard.writeText` resolve → 直接 `return true`，不进 fallback（execCommand 不调用），与 02 §3 一致。
   - fallback 路径：textarea 赋值 + `aria-hidden` + 离屏 + `appendChild` + `select` + `execCommand('copy')`，返回其布尔；execCommand 抛错被内层 catch 吞为 `false`；`finally removeChild` 保证任意路径 textarea 清理（BC-5）。逐字与三处原实现的 catch 分支同构。
   - 边界：空串无特判（BC-1）；无共享可变状态，局部 textarea，多次调用互不干扰（BC-6）。
   - 无 off-by-one、无 null 解引用（`document.body` 在浏览器/happy-dom 环境恒存在，与原实现假设一致，OOS-5 不新增 SSR 防御）。

2. **需求保真 — PASS**（见下「需求覆盖核对」表，AC-1~AC-7 全 ✅）

3. **设计保真 — PASS**（见下「设计保真核对」表，无漂移）

4. **性能 — PASS** — 纯函数，无循环/IO/分配热点；textarea 创建+移除一次性，与原实现等价，无回归。

5. **安全 — PASS** — util 不引入新输入面；`textarea.value = text` 是 DOM property 赋值（非 innerHTML），无 XSS 注入面（与原实现一致）。util 无 naive-ui import、无 message 调用（R-3 缓解到位，确认 `clipboard.ts` 仅 import 无、纯 DOM API）。

6. **可维护性 — PASS** — 命名清晰（`copyToClipboard` 动词短语 + 布尔返回语义明确）；JSDoc 注明返回语义 + "不弹 toast" 约束；顶部注释列出三个共享方 + 抽取来源（偿还 T-058 backlog）+ insight L37/L42 依据，符合 format.ts/proxyStatus.ts 既有 utils 注释范式；无死代码、无过度抽象（单一职责函数）。三组件删除了各自的本地 fallback 块（净减重复），message 反馈正确留在组件 setup 层。

## 测试质量审查（tests are code）

`web/src/utils/__tests__/clipboard.spec.ts`（7 例）—— 有意义、非 shape-matching：
- 首选成功用例**显式断言 `execCommand` 未被调用**（证伪"成功也走 fallback"），强于仅断返回值。
- fallback 三态（true/false/抛错）分别锁死返回布尔。
- "fallback textarea 持有目标文本"用例在 `execCommand` mock 内捕获 DOM 中 textarea.value，验证 select 前文本确已写入离屏节点（行为正确性，非仅清理）。
- 每条 fallback 路径后断言 `strayTextareas()===0`（DOM 残留检查，BC-5）。
- Adversarial 双重失败 `resolves.toBe(false)`（验证无未捕获异常——若 util 漏 catch execCommand 抛错，此处会 reject 而非 resolve false，反向证伪到位）。
- 模拟范式遵循 insight L37：`Object.defineProperty(navigator,'clipboard',{configurable:true})` + 显式装 `document.execCommand`；`beforeEach` reset + `afterEach` 清 body。零 naive-ui 组件名查询（L45）。

既有三组件 spec 未改动——经核对其 mock 命中 util 内部同一 `navigator.clipboard.writeText` / `document.execCommand`，行为不变（详见设计保真表 R-1 行）。

## 需求覆盖核对

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 util 存在且导出 copyToClipboard + 编译/eslint 通过 | `web/src/utils/clipboard.ts:25` `export async function copyToClipboard` | ✅（eslint/tsc 真跑闸门交 orchestrator） |
| AC-2 clipboard.spec 四态 + textarea 残留断言 | `clipboard.spec.ts`：resolve→true 未走 fallback / reject+true / reject+false / reject+抛错 + 各路径 strayTextareas()===0 | ✅ |
| AC-3 三组件既有 spec 零回归 | LogViewer.spec 未改（AC-6 mock writeText 命中 util）；FirewallHint/PublicIpDetector spec 未改（mock 命中 util） | ✅（真跑零回归交 orchestrator 复核） |
| AC-4 baseline bump + B.4 通过 | `baseline.json` frontend_tests 491→498 / test_count 813→820 / version 27；净增 7 = clipboard.spec 7 例 | ✅（算术核对：491+7=498 ✓，813+7=820 ✓） |
| AC-5 06 含裸 ## Adversarial tests + 双重失败反向证伪 | clipboard.spec 已含；06 待 QA 产出（QA 阶段核） | ⏳（转 QA） |
| AC-6 dev-map 可复用工具表 +1 行 | `dev-map.md:175` clipboard.ts 行 | ✅ |
| AC-7 改动文件集仅限白名单 | 7 文件全在白名单（util/spec/3 组件/baseline/dev-map），未碰后端/store/路由/API | ✅ |

## 设计保真核对

| Design item | Implementation | Status |
|---|---|---|
| 02 §3 util 公共 API `copyToClipboard(text): Promise<boolean>` | `clipboard.ts:25` 签名精确一致 | ✅ |
| 02 §3 util 不调 message/useMessage | `clipboard.ts` 无 naive-ui import、无 message 调用 | ✅ |
| 02 §6 LogViewer.onCopy 保留 build text + 成功/失败各调 message 一次 | `LogViewer.vue:149-155` 逐字一致 | ✅ |
| 02 §6 FirewallHint/PublicIpDetector copyText 三元 message + return ok | `FirewallHint.vue:70-74` / `PublicIpDetector.vue:70-74` 逐字一致 | ✅ |
| 02 §6 copyCmd/copyAll/copyIp 不变（依赖 ok 短暂态） | 三处短暂态函数未改动，仍 `if (await copyText/copyToClipboard...)` | ✅ |
| R-1 LogViewer.spec AC-6 不断言被抽走部分 → 零回归 | 实读确认 AC-6 仅断 writeSpy，util 内部命中同一 mock | ✅ 无漂移 |
| 02 §11 分区 dev-frontend 单分区，改动全在 web/** + 账本文件 | 7 文件均符合 | ✅ |

## Verdict

**APPROVED** —— 无 CRITICAL / MAJOR / MINOR（仅 1 NIT，不阻塞）。util 抽取为纯 1:1 行为搬运，无设计漂移；三组件可观察行为字节不变；既有 spec 零回归经独立核对成立；新测试有意义且覆盖四态 + 边界 + Adversarial。

AC-5 的"06 含裸 ## Adversarial tests"转 QA 阶段产出核对。verify_all 真跑（eslint/tsc/vitest 实测 == 498 + LogViewer spec 零回归）作硬闸门交 orchestrator Bash 会话。
