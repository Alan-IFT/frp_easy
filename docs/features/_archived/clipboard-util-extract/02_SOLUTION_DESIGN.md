# 02 方案设计 — T-061 clipboard-util-extract

> 阶段 2 / Solution Architect · mode: full · 中文产出
> 上游 `01_REQUIREMENT_ANALYSIS.md` Verdict = READY ✓

## 1. 架构摘要

把当前在 `LogViewer.vue` / `FirewallHint.vue` / `PublicIpDetector.vue` 三处逐字重复的 "clipboard.writeText → 失败回落临时 textarea + execCommand('copy')" 逻辑抽到单一纯函数 `web/src/utils/clipboard.ts::copyToClipboard(text): Promise<boolean>`。util 只负责"把文本写进剪贴板并返回是否成功"，**不**碰 UI（`message`/`useMessage` 留在组件 setup）。三处组件改为调用 util，并在拿到布尔结果后按现状逐字调用 `message.success`/`message.error`。系统层无新依赖、无新 API、无 schema 变更，纯前端内部重构 + 测试网扩充。

## 2. 受影响模块（Affected modules）

| 文件 | 改动 | 说明 |
|---|---|---|
| `web/src/utils/clipboard.ts` | 新建 | 导出纯函数 `copyToClipboard` |
| `web/src/utils/__tests__/clipboard.spec.ts` | 新建 | util 单测（含 Adversarial） |
| `web/src/components/LogViewer.vue` | 编辑 | `onCopy` 改调 util；可观察行为字节不变 |
| `web/src/components/FirewallHint.vue` | 编辑 | `copyText` 体改调 util；删本地 fallback 块 |
| `web/src/components/PublicIpDetector.vue` | 编辑 | `copyText` 体改调 util；删本地 fallback 块 |
| `scripts/baseline.json` | 编辑 | bump `frontend_tests` + `test_count` |
| `docs/dev-map.md` | 编辑 | 「可复用工具」表 +1 行 |

## 3. 模块分解（新模块）

### `web/src/utils/clipboard.ts`

**职责**：跨组件共享的"复制文本到剪贴板"底层操作，含非安全上下文 fallback。无 UI 副作用，无共享可变状态。

**公共 API**：

```ts
/**
 * 把 text 写入系统剪贴板。
 * 首选 navigator.clipboard.writeText（安全上下文）；失败回落临时离屏 textarea + execCommand('copy')。
 * @returns true=复制成功；false=两条路径都失败（调用方据此决定 UI 反馈）。
 * 不抛错、不弹 toast（message 留组件层，useMessage 是组合式 hook）。
 */
export async function copyToClipboard(text: string): Promise<boolean>
```

**实现伪代码**（dev-frontend 1:1 搬运现状，无新行为）：

```ts
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('aria-hidden', 'true')
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    let ok = false
    try {
      ok = document.execCommand('copy')
    } catch {
      ok = false
    } finally {
      document.body.removeChild(ta)
    }
    return ok
  }
}
```

注：此块与现状 `LogViewer.onCopy` 的 catch 分支（去掉 message 调用）逐行同构；`FirewallHint`/`PublicIpDetector` 的 `copyText` catch 分支同款。属纯搬运（insight L42）。

## 4. 数据模型变更

无。

## 5. API 契约

无后端 API 变更。前端内部函数契约见 §3。

## 6. 调用流（Sequence / flow）

```
组件按钮 @click → copyXxx()
  → const ok = await copyToClipboard(text)
        ├─ try: navigator.clipboard.writeText(text) resolve → return true
        └─ catch: 临时 textarea + select + execCommand('copy')
                   ├─ true  → return true
                   ├─ false → return false
                   └─ throw → catch → return false
                   finally: removeChild(textarea)
  → ok ? message.success('已复制到剪贴板') : message.error('复制失败：请手动选择文本复制')
  → (FirewallHint/PublicIpDetector) ok 时置短暂态 '已复制 ✓' / '已复制全部 ✓'，setTimeout 2s 复位
```

三处组件改动后的逐字目标：

- **LogViewer.onCopy**（保留 build text，替换 try/catch 整块）：
  ```ts
  async function onCopy() {
    const text = search.visibleLines.value.map((v) => v.parsed.raw).join('\n')
    const ok = await copyToClipboard(text)
    if (ok) {
      message.success('已复制到剪贴板')
    } else {
      message.error('复制失败：请手动选择文本复制')
    }
  }
  ```
  可观察行为：成功调 `message.success('已复制到剪贴板')` 一次、失败调 `message.error('复制失败：请手动选择文本复制')` 一次——与现状字节一致。既有 `LogViewer.spec.ts::AC-6` 只断言 `navigator.clipboard.writeText` 收到拼接字符串（不断言 message），抽取后 util 内部仍调同一被 mock 的 `navigator.clipboard.writeText`，故零回归。

- **FirewallHint.copyText / PublicIpDetector.copyText**（替换整个函数体）：
  ```ts
  async function copyText(text: string): Promise<boolean> {
    const ok = await copyToClipboard(text)
    message[ok ? 'success' : 'error'](ok ? '已复制到剪贴板' : '复制失败：请手动选择文本复制')
    return ok
  }
  ```
  `copyCmd`/`copyAll`/`copyIp` 不变（仍依赖 `copyText` 返回的 `ok` 决定短暂态）。

## 7. 复用审计（Reuse audit）

| 需求 | 既有代码 | 文件路径 | 决策 |
|---|---|---|---|
| 剪贴板复制 + fallback 逻辑 | 三处内联实现（LogViewer.onCopy / FirewallHint.copyText / PublicIpDetector.copyText） | `web/src/components/{LogViewer,FirewallHint,PublicIpDetector}.vue` | 抽取合并为单一 util（消除重复，本任务核心） |
| 前端纯工具函数的目录/范式 | `format.ts`（formatBytes/formatTime）、`proxyStatus.ts`（getProxyStatusTag） | `web/src/utils/*.ts` + `__tests__/*.spec.ts` | 复刻范式：新文件放 `web/src/utils/`，配 `__tests__/clipboard.spec.ts`，并在 dev-map「可复用工具」表登记 |
| util 单测的 clipboard/execCommand 模拟范式 | T-058 三组件 spec | `web/src/components/__tests__/{FirewallHint,PublicIpDetector}.spec.ts`、`LogViewer.spec.ts` | 复用：`Object.defineProperty(navigator,'clipboard',{value:{writeText:mock},configurable:true})` + 显式装 `document.execCommand`（insight L37）。util 无 UI，无需 mount/message mock |
| dev-map「可复用工具」表行格式 | format.ts / proxyStatus.ts 既有行 | `docs/dev-map.md:173-174` | 照格式追加一行 |

无新增第三方依赖。

## 8. 风险分析（Risk analysis）

| # | 风险 | 缓解 |
|---|---|---|
| R-1 | 抽取后 LogViewer 既有测试回归（T-058 D1 当初规避抽取的核心顾虑） | `LogViewer.spec.ts::AC-6` 只 mock 并断言 `navigator.clipboard.writeText`，**不**断言 message 也不断言 fallback DOM；抽取后 util 内部仍调同一被 mock 的 API。CR/QA 复审 + orchestrator 真跑 verify_all **特别复核 LogViewer spec 零回归**作硬闸门 |
| R-2 | 文案/行为漂移（成功/失败 message 文字或调用次数变化） | §6 给出三处逐字目标；message 调用全留组件层、文案字符串原样保留；FirewallHint/PublicIpDetector 用三元保持单次调用语义；CR 逐字比对 |
| R-3 | util 误用 `useMessage`（组合式 hook 在非 setup 上下文报错/返回无效对象） | 需求 OOS-5 + §3 明确 util 不 import naive-ui、不调 message；CR 检查 util 无 naive-ui import |
| R-4 | 测试模拟不当导致 clipboard.spec 脆弱（happy-dom/jsdom 默认无 execCommand；每个用例 navigator.clipboard 未还原泄漏） | 遵循 insight L37：`Object.defineProperty` + `configurable:true` + 显式装 `document.execCommand`；`beforeEach` reset、`afterEach` 清 `document.body.innerHTML`。util 无 naive-ui 依赖，断言纯布尔 + DOM textarea 残留检查，零组件名查询（L45 风险不适用 util，但组件 spec 已守） |
| R-5 | baseline 计数漏 bump → B.4 闸门 FAIL | dev-frontend 落地时同步 bump `frontend_tests`/`test_count`，净增 = clipboard.spec 新增用例数；CR 核对增量算术 |
| R-6 | e2e 受影响 | insight L34：e2e 烟雾测试（01-setup/02-auth/03-dashboard）不点复制按钮、无"复制"断言（T-058 已核实）。纯 util 内部重构无 e2e 影响——04 dev-frontend 复核 grep e2e spec 确认 |

## 9. 迁移 / 上线计划

无 schema / API 变更，无需迁移。后向兼容：用户可观察行为完全不变。回滚：纯前端代码改动，`git revert` 即可（本任务不 commit，由 orchestrator/用户决定）。

## 10. 范围外澄清（设计边界）

- 不在 util 内做 `navigator.clipboard` 存在性预检 / SSR 防御（现状三处均直接 try，util 1:1 沿用；OOS-5）。
- 不重构 LogViewer 的 text 拼接（留组件，util 只收已拼字符串；OOS-4）。
- 不动 `copyCmd`/`copyAll`/`copyIp`/`copyIp` 的短暂态逻辑（仅 `copyText` 内部实现换底）。
- 不触后端/store/路由/DB（OOS-3）。

## 11. 分区分配（Partition assignment）

> 项目存在 `.harness/agents/dev-{db,backend,frontend}.md` → partitioned 模式，本节必填。

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `web/src/utils/clipboard.ts` | dev-frontend | new | — |
| `web/src/utils/__tests__/clipboard.spec.ts` | dev-frontend | new | depends on clipboard.ts |
| `web/src/components/LogViewer.vue` | dev-frontend | edit | depends on clipboard.ts |
| `web/src/components/FirewallHint.vue` | dev-frontend | edit | depends on clipboard.ts |
| `web/src/components/PublicIpDetector.vue` | dev-frontend | edit | depends on clipboard.ts |
| `scripts/baseline.json` | dev-frontend | edit | depends on clipboard.spec.ts |
| `docs/dev-map.md` | dev-frontend | edit | — |

### Dispatch order

1. dev-frontend（单分区即可覆盖整任务）

### Parallelism

None — 纯前端单分区，无跨分区依赖。util 是其余文件的前置依赖，分区内部按 clipboard.ts → 三组件改造 → spec → baseline → dev-map 顺序落地。

## 12. 裁决（Verdict）

**READY** —— 设计完整，无歧义，无新依赖，可进入 Stage 3 闸门评审。
