# 04 开发记录 — Frontend partition · T-061 clipboard-util-extract

> 阶段 4 / dev-frontend · mode: full · 中文产出
> 上游 03_GATE_REVIEW.md Verdict = APPROVED ✓（8 维全 PASS，R-1 历史顾虑经独立核验不成立）

## Partition

dev-frontend — owns: `web/**`。

本任务全部生产改动落在 `web/**`；另有两处仓库级账本文件 `scripts/baseline.json`（测试计数）与 `docs/dev-map.md`（可复用工具表）按惯例同步 bump——这两者是所有分区共用的 bookkeeping，非跨分区生产代码，**无 partition mismatch，未触后端/DB owned paths**。

## Files changed（this partition + bookkeeping）

| 文件 | 改动 | 说明 |
|---|---|---|
| `web/src/utils/clipboard.ts` | 新建 | 导出纯函数 `copyToClipboard(text): Promise<boolean>`（02 §3 伪代码 1:1 落地） |
| `web/src/utils/__tests__/clipboard.spec.ts` | 新建 | util 单测 7 例（含裸 `## Adversarial tests`） |
| `web/src/components/LogViewer.vue` | 编辑 | `onCopy` 删本地 try/catch+fallback 整块，改 `const ok = await copyToClipboard(text)` + 成功/失败各调 message 一次；加 import |
| `web/src/components/FirewallHint.vue` | 编辑 | `copyText` 体删本地 fallback 块，改 `const ok = await copyToClipboard(text); message[ok?'success':'error'](...); return ok`；加 import |
| `web/src/components/PublicIpDetector.vue` | 编辑 | `copyText` 同 FirewallHint；加 import |
| `scripts/baseline.json` | 编辑 | version 26→27；frontend_tests 491→498；test_count 813→820；passing_count→820；notes 追加 T-061 段 |
| `docs/dev-map.md` | 编辑 | 「可复用工具」表 +1 行 `web/src/utils/clipboard.ts` |

## 实现细节（逐字保真度）

### `web/src/utils/clipboard.ts`（新）

按 02 §3 伪代码 1:1 落地：try `navigator.clipboard.writeText(text)` resolve → `true`；catch → `aria-hidden` 离屏 textarea（`position:fixed; left:-9999px`）+ `select()` + `document.execCommand('copy')` 返回其布尔；execCommand 抛错 → `false`；`finally` `removeChild`。**无 naive-ui import、无 message 调用**（R-3 缓解）。无共享可变状态（纯函数 + 局部 textarea，BC-6）。

### `LogViewer.vue::onCopy`（编辑）

保留 build text（`search.visibleLines.value.map((v) => v.parsed.raw).join('\n')`，OOS-4），替换 try/catch 整块为：
```ts
const ok = await copyToClipboard(text)
if (ok) { message.success('已复制到剪贴板') } else { message.error('复制失败：请手动选择文本复制') }
```
可观察行为字节不变：成功调 `message.success('已复制到剪贴板')` 一次、失败调 `message.error('复制失败：请手动选择文本复制')` 一次。

### `FirewallHint.vue::copyText` / `PublicIpDetector.vue::copyText`（编辑）

函数体替换为：
```ts
const ok = await copyToClipboard(text)
message[ok ? 'success' : 'error'](ok ? '已复制到剪贴板' : '复制失败：请手动选择文本复制')
return ok
```
`message` const（`useMessage()`）仍被使用（无 unused-var eslint 风险）；`copyCmd`/`copyAll`/`copyIp` 未改动，仍依赖 `copyText` 返回的 `ok` 置 "已复制 ✓" / "已复制全部 ✓" 短暂态。

## 新增测试（clipboard.spec.ts，7 例）

| 用例 | 断言 |
|---|---|
| 首选 resolve → true | 返回 true + `writeText` 调 1 次（参数正确）+ `execCommand` 未调用（未走 fallback）+ 无残留 textarea |
| reject + execCommand true → true | 返回 true + `execCommand('copy')` 被调 + textarea 清理 |
| reject + execCommand false → false | 返回 false + `execCommand('copy')` 被调 + textarea 清理 |
| reject + execCommand 抛错 → false | `resolves.toBe(false)`（无未捕获异常）+ textarea 清理（finally） |
| BC-1 空串 → true（首选） | 返回 true + `writeText('')` |
| fallback textarea 持有目标文本 | execCommand 触发时离屏 textarea.value === 目标文本，提交后清理 |
| **Adversarial**：clipboard reject + execCommand reject（双重失败）→ false + 无未捕获异常 + textarea 清理 | `resolves.toBe(false)` + `strayTextareas()===0` |

测试模拟范式（insight L37）：`Object.defineProperty(navigator,'clipboard',{value:{writeText:mock},configurable:true,writable:true})` + 显式装 `(document as ...).execCommand = mock`（happy-dom 默认无）；`beforeEach` reset、`afterEach` 清 `document.body.innerHTML`。断言纯布尔 + `document.querySelectorAll('textarea[aria-hidden="true"]')` 残留检查，**零 naive-ui 组件名查询**（util 无 UI，L45 风险天然不适用）。util 无 naive-ui 依赖，无需 naive-ui mock。

## 既有 spec 零回归分析（R-1 / R-2）

- **LogViewer.spec.ts**：未改动。`AC-6 复制全部`（L195-220）`Object.defineProperty(navigator,'clipboard',{value:{writeText:writeSpy}})` → `await t.onCopy()` → 断言 `writeSpy` 收到拼接字符串。抽取后 util 内部仍调同一被 mock 的 `navigator.clipboard.writeText`（首选路径 resolve），故 `writeSpy` 仍被调 1 次、参数仍为拼接字符串 → **零回归**。其余 LogViewer 用例（搜索/筛选/全屏/主题/XSS 等）不涉 onCopy → 不受影响。
- **FirewallHint.spec.ts / PublicIpDetector.spec.ts**：未改动。两者 `Object.defineProperty(navigator,'clipboard')` + 显式装 `document.execCommand` + `vi.mock('naive-ui')` 单例 `__messageSpies`。抽取后 util 走同一组 mock：成功路径 writeText resolve → util 返 true → 组件 `message.success('已复制到剪贴板')`；fallback 路径 writeText reject + execCommand → util 返该布尔 → 组件 message.success/error。message 调用文案与次数不变 → **零回归**。各自的 Adversarial（双重失败 → message.error 不抛）与 textarea 清理断言：清理现由 util 的 finally 负责（位置从组件移到 util，DOM 可观察结果一致），仍通过。

## Out-of-partition coordination

无。纯前端单分区，无后端/DB 改动。

## e2e 影响评估（insight L34）

e2e 烟雾测试（`web/tests/e2e/01-setup` / `02-auth` / `03-dashboard`）不点击复制按钮、无"复制"文案断言（T-058 已核实 grep），且本任务为纯 util 内部重构、用户可观察行为不变 → **e2e 零影响**，无需改 e2e spec。

## verify_all result

**PENDING** —— dev-frontend 落地环境（当前 role-collapsed PM 上下文）无 Bash/PowerShell 工具（insight L31：`No such tool available: Bash`），无法自跑 `scripts/verify_all`。

静态 + 确定性预测（insight L31：纯文本/纯函数改动无随机/IO/竞争，预期可逐项推导）：

- **eslint**：util 无 unused/未定义符号；三组件 `message` const 仍被使用、新 import 被使用；预期 PASS。
- **vue-tsc 类型检查**：`copyToClipboard(text: string): Promise<boolean>` 签名与三处调用点（`await copyToClipboard(text)` → `ok: boolean`）匹配；`message[ok?'success':'error']` 索引访问对 naive-ui MessageApi 合法（success/error 同签名 `(content) => MessageReactive`）；预期 PASS。
- **vitest run**（B.3 + B.4 计数闸门）：新增 7 例全为确定性纯函数断言（mock 注入，无 IO/随机/时序），预期全绿；既有三组件 spec 经上节分析零回归；frontend_tests 实测应 = 498（491 + 7），与 baseline 一致 → B.4 PASS。
- **结构/schema 静态闸门**：未碰后端路由/openapi/migration → 不受影响。

**交付硬闸门**：orchestrator 在其 Bash 会话独立真跑 `bash scripts/verify_all.sh`，**特别复核 LogViewer 相关 spec 零回归 + frontend_tests 实测 == 498**。

## Verdict

**READY FOR REVIEW**（frontend partition complete）—— 7 处文件改动全部落地，纯 1:1 行为搬运无设计漂移，无 DESIGN DRIFT，无 BLOCKED ON PARTITION。verify_all 真跑硬闸门交 orchestrator Bash 会话。
