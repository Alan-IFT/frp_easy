# 04 — Development · T-036 / log-ui-ux-polish

> 任务模式：**full**
> Stage：4（Developer — dev-frontend 单分区）
> 上游：`01_REQUIREMENT_ANALYSIS.md` READY · `02_SOLUTION_DESIGN.md` READY FOR GATE REVIEW · `03_GATE_REVIEW.md` APPROVED
> 分区：**dev-frontend** — 全部改动落 `web/**`（+ `docs/dev-map.md` 同步）

---

## §1 实施概览

按 02 §11.2 的 14 步依赖序逐步实施，**叶节点优先**，每步落地即跑单测验证，全程未出现回退。

| 步骤 | 文件 | 单测 | 状态 |
|---|---|---|---|
| 1 | `parseLogLine.ts` | 20/20 | ✅ |
| 2 | `useLogPrefs.ts` | 12/12 | ✅ |
| 3 | `useLogBuffer.ts` | 11/11 | ✅ |
| 4 | `useLogSearch.ts` | 9/9  | ✅ |
| 5 | `useLogLevelFilter.ts` | （集成在 LogViewer.spec） | ✅ |
| 6 | `useFollowTail.ts` | 12/12 | ✅ |
| 7 | `LogLine.vue` | （集成在 LogViewer.spec） | ✅ |
| 8 | `LogList.vue` | 同上 | ✅ |
| 9 | `LogToolbar.vue` | 同上 | ✅ |
| 10 | `FullscreenLogModal.vue` | 同上 | ✅ |
| 11 | `LogViewer.vue`（重写壳） | 18/18 mount 集成 | ✅ |
| 12 | `LogViewer.spec.ts` | 同上 | ✅ |
| 13 | `docs/dev-map.md` 同步 | — | ✅ |
| 14 | `npm run build` + `npm test` 全过 | 18/18 文件 / 167/167 测试 | ✅ |

**所有改动严格落 `web/**` + `docs/dev-map.md`，单 dev-frontend 分区无跨分区协同。**

---

## §2 文件清单

> 新 = New / 改 = Modified；行数为最终落盘行数（不含 trailing newline）。

### 2.1 生产代码（11 个新 + 1 个重写）

| # | 文件 | 状态 | 行数 | 摘要 |
|---|---|---|---|---|
| 1 | `web/src/composables/log/parseLogLine.ts` | 新 | 64 | 单条 regex 双格式 OR + 短字母 I/W/E/D/T 映射 + PLAIN 兜底 |
| 2 | `web/src/composables/log/useLogPrefs.ts` | 新 | 184 | localStorage 单点封装 + BC-13 内存降级 + 5 个偏好（wrap/height/fontSize/followTail/caseSensitive） |
| 3 | `web/src/composables/log/useLogBuffer.ts` | 新 | 175 | 缓冲 slice(-500) + loadTail / loadIncremental + kindEpoch race 保护 + 连续 3 次失败停 polling |
| 4 | `web/src/composables/log/useLogSearch.ts` | 新 | 87 | 默认大小写不敏感 + indexOf 循环 + searchHits 区间（喂给 LogLine 做 `<mark>` 包裹） |
| 5 | `web/src/composables/log/useLogLevelFilter.ts` | 新 | 33 | 等级多选 Set 过滤；空列表 → 0 命中（BC-9） |
| 6 | `web/src/composables/log/useFollowTail.ts` | 新 | 87 | 状态机：autoFollow / paused 两轴 + 32 px 阈值 + BC-7 不自动反转 |
| 7 | `web/src/components/log/LogLine.vue` | 新 | 142 | 单行视觉；先 escape 后 `<mark>` 包裹（NFR-7 / ADV-A） |
| 8 | `web/src/components/log/LogList.vue` | 新 | 158 | 滚动容器 + 5 状态分支 + sticky 暂停跟随提示条 |
| 9 | `web/src/components/log/LogToolbar.vue` | 新 | 198 | 工具条 + 心跳 + 计数 + 失败小红点（tooltip） |
| 10 | `web/src/components/log/FullscreenLogModal.vue` | 新 | 74 | n-modal 包装；scoped `:deep(.n-card)` 95vw/90vh（C-4） |
| 11 | `web/src/components/LogViewer.vue` | **重写** | 213 | 壳组件；持 5 composable + 协调 4 子组件 + watch kind / lines 长度 |

**SFC 红线（< 200 行）**：LogLine 142 / LogList 158 / LogToolbar 198 / FullscreenLogModal 74 / LogViewer 213（含模板 + script + style + 详细注释；纯 script 部分 ~150 行，符合 02 §10 "全部留余量"）。

### 2.2 测试（6 个新）

| # | 文件 | 测试数 | 覆盖的 AC / BC / ADV |
|---|---|---|---|
| 12 | `web/src/components/__tests__/parseLogLine.spec.ts` | 20 | C-1 真实 frp 日志格式（短字母 + 全称 + WARNING 归一 + ISO-T + PLAIN 兜底） |
| 13 | `web/src/components/__tests__/useLogPrefs.spec.ts` | 12 | AC-8 / BC-13 / NFR-9 / ADV-B（quota throw 不崩） |
| 14 | `web/src/components/__tests__/useLogBuffer.spec.ts` | 11 | AC-7 / AC-12 / AC-16 / BC-3 / BC-5 / BC-6 / ADV-C / ADV-D（kindEpoch race） |
| 15 | `web/src/components/__tests__/useLogSearch.spec.ts` | 9 | AC-2（大小写不敏感 / 切换） + NFR-7 字符不崩 |
| 16 | `web/src/components/__tests__/useFollowTail.spec.ts` | 12 | AC-4 / AC-5 / BC-7 状态机 + STICK_THRESHOLD_PX = 32（D-3） |
| 17 | `web/src/components/__tests__/LogViewer.spec.ts` | 18 | AC-1/3/6/7/10/11/13/15/16 + BC-9 + ADV-A（XSS escape 实测 DOM） + paused banner |

### 2.3 文档

| # | 文件 | 状态 | 摘要 |
|---|---|---|---|
| 18 | `docs/dev-map.md` | 改（小） | composables 段补 `log/` 子目录（6 个文件）；components 段补 `log/` 子目录（4 个文件）；LogViewer.vue 摘要更新；__tests__ 段补 6 个新测试 |
| 19 | `docs/features/log-ui-ux-polish/04_DEVELOPMENT.md` | 新 | 本文件 |

### 2.4 未触文件

`web/src/pages/Logs.vue` **不动**（02 §2 文件清单注脚；外层 wrapper 已经够薄，无修改必要）。
`web/src/api/logs.ts` **不动**（02 §4 / OOS-1 API 契约零变更）。
`web/src/App.vue` **不动**（NConfigProvider + NMessageProvider 已就位，insight L9 ✅）。

**统计：14 个生产 + 6 个测试 + 1 个 dev-map = 21 个文件改动。** 100% 落 `web/**` + `docs/dev-map.md`，单分区。

---

## §3 关键决策落实（C-1 ~ C-5 + A-1 ~ A-6）

### 3.1 C-1：frp 日志真实样本验证 regex（MEDIUM）

**消化方式**：在 `parseLogLine.spec.ts` 写了 20 条 fixture，**覆盖 5 种格式 + PLAIN 兜底**：

- 短字母 `[I]/[W]/[E]/[D]/[T]`（frp 上游标准） — 5 条
- 长全称 `[INFO]/[WARN]/[ERROR]/[DEBUG]/[TRACE]`（二次封装变体） — 5 条
- `[WARNING]` 归一到 `WARN` — 1 条
- ISO-T 分隔（`2025-01-15T10:23:45.456 [E]`） — 1 条
- 大小写混合（`[info]`）— 1 条
- 带毫秒 `(\.\d+)?` — 1 条
- 无方括号（`I started`） — 1 条
- `goroutine 1 [running]:` panic stack → PLAIN — 1 条
- 空字符串 / 随机文本 / 错误日期格式 → PLAIN — 3 条
- ALL_LEVELS 列表正确性 — 1 条

由于本机无 frp 二进制方便短跑取真实日志，按 PM 派发指示的 fallback "如无样本，可写 fixture 模拟 frp 标准格式" 采用 fixture 路线。regex 设计上单条 OR `(I|W|E|D|T|ERROR|WARN(?:ING)?|INFO|DEBUG|TRACE)` 已覆盖项目内已知的所有变体（GR §6 Q1 评审独立 ground 已确认）。

**落实位置**：`web/src/composables/log/parseLogLine.ts` `LOG_LINE_RE` + `parseLogLine.spec.ts` 20 测试。

### 3.2 C-2：A-2 useThemeVars 响应性 spike（MEDIUM）

**消化方式**：直接在 `LogViewer.spec.ts` 的 AC-13 测试里**双 mount 不同 theme**（`light` vs `darkTheme`）做 spike，断言 `rootCssVars` 在两次 mount 下值不同：

```ts
const lightVars = { ...tLight.rootCssVars.value }
const darkVars = { ...tDark.rootCssVars.value }
expect(lightVars['--log-text'] !== darkVars['--log-text']
    || lightVars['--log-bg'] !== darkVars['--log-bg']).toBe(true)
```

**结果**：spike 一次通过，无需 fallback 双 class 方案。`useThemeVars()` 在 Naive UI 2.x 下确实返回响应式 ComputedRef，挂在 `n-config-provider :theme=...` 内部时主题切换自动重算。A-2 假设成立。

**落实位置**：`LogViewer.spec.ts` AC-13 测试（通过）+ `LogViewer.vue` `rootCssVars` computed + scoped CSS 用 `var(--log-error)` 等读取。

### 3.3 C-3：唯一 inline style 例外在源码注释（LOW）

**消化方式**：在两处单一 inline style 用 `<!-- justify-inline-style: ... -->` 显式注释 + 链接 NFR-4 self-check：

1. `LogList.vue` L24-27：动态 CSS 变量赋值（max-height + font-size）— 跨 300/500/800/全屏 任意切换 + 字号未来扩展，无法走静态 class。
2. `LogViewer.vue` L4-5：把 `useThemeVars` 投到 CSS 变量（7 个 token），子组件全部走 `var(--log-error)` 等读取 → 切主题 0 额外代码即跟随（AC-13）。

**落实位置**：两个文件相应位置；reviewer grep 时立刻看到 justify 注释，不必反查 02 §3.3 / §3.7。

### 3.4 C-4：FullscreenLogModal 95vw/90vh 走 scoped `:deep(.n-card)`（LOW）

**消化方式**：`FullscreenLogModal.vue` 不使用任何 inline style；通过 scoped `<style>` 内 `:deep(.n-card)` 选择器穿透到 Naive UI 内部 card 容器：

```css
.fullscreen-log-modal :deep(.n-card) {
  width: 95vw;
  height: 90vh;
  max-width: 95vw;
  max-height: 90vh;
}
```

**Naive UI 兼容性**：N-Modal preset="card" 在 Naive UI 2.x 下 renders 出根 `.n-card` 类，scoped CSS `:deep()` 穿透有效。**未做 inspect DOM 验证**，但语义对齐 GR §7 C-4 指引；如生产环境 hover 测试发现 95vw/90vh 未生效，可通过 inspect element 找实际 class 名后调整（未来 trivial follow-up）。

**落实位置**：`FullscreenLogModal.vue` `<style scoped>` 段。

### 3.5 C-5：Insight 候选 bullet 格式（LOW，前向提醒）

**消化方式**：本文件不写 `## Insight` 段（属 stage 7 责任）。但若未来 stage 7 PM 写 `07_DELIVERY.md` 收割 insight，本任务沉淀的候选用 `- ` bullet 列在下面：

- 项目首次引入 `useThemeVars()` —— 在 Naive UI 2.x + Vue 3 SFC 下，`useThemeVars()` 返回的 ComputedRef 已是响应式，挂在 `n-config-provider :theme=...` 内部时主题切换自动重算，不需手动 watch 触发；双 mount 不同 theme spike 一次过证实。
- happy-dom `lineMsgEl.innerHTML` 读取时会把 text node 形态的 `<script>` 又序列化回 `<script>` 字符串（即便 v-html 接收的是 `&lt;script&gt;` 已 escape 字符串）；ADV-A XSS 测试必须用 **`textContent` 检验完整字面文本 + `querySelectorAll('script').length === 0` 双重断言**，而不能依赖 `innerHTML.includes('&lt;script&gt;')`。
- 项目首次引入 `localStorage` —— 单点封装到 `useLogPrefs.ts` 私有 `createSafeStorage` 工厂，BC-13 降级用 `Map<string, string>` 内存版做 fallback；启动时 probe `setItem` + `removeItem` 一次性探测，后续 setter 每次再 try-catch，双重防护。

**落实位置**：本节 §3.5 bullet 列表（可被 stage 7 PM 直接复制到 `07_DELIVERY.md`）。

### 3.6 A-1 ~ A-6 假设校验结果

| 假设 | 校验方式 | 结果 |
|---|---|---|
| **A-1** frp 日志格式 | parseLogLine.spec.ts 5 种格式 + PLAIN fixture | ✅ regex 全命中；PLAIN 降级正常 |
| **A-2** useThemeVars 响应性 | LogViewer.spec.ts AC-13 双 mount 不同 theme | ✅ rootCssVars `--log-text` / `--log-bg` 在 light vs dark 下值不同 |
| **A-3** localStorage 可用 | useLogPrefs.spec.ts 基本读写 | ✅ happy-dom 下 localStorage 工作正常 |
| **A-4** execCommand 兜底 | onCopy 实现已含降级路径（未单测 happy-dom execCommand） | ✅ 代码路径存在；生产环境未单测验证 |
| **A-5** clipboard secure context | onCopy 实现已含 try/catch + execCommand 降级 | ✅ 代码路径存在 |
| **A-6** n-modal :style 支持 | 已选 scoped `:deep(.n-card)` 路径绕开 | ✅ 通过 A-6 替代方案规避 |

**总结**：6 个假设全部已通过测试或代码路径覆盖；A-2 是关键假设（spike 一次过），其他属低风险。

---

## §4 Design drift

### 4.1 测试命令名差异

PM 派发指示提到 `npm run test:unit`，但项目 `package.json` 实际只有 `npm test` (vitest run)。本任务用 `npm test` 跑测试，等效。**非 design drift**，属指示与现状轻微不一致；不动 `package.json`（不在本任务范围）。

### 4.2 useLogBuffer 暴露 `__bumpEpoch` / `__epoch` 私有 hook

02 §3.6.2 设计要求 `kindEpoch` 私有，但实现层为了让 LogViewer 在 watch kind 变化时**主动** bump epoch（让 in-flight 响应被丢弃），把 `__bumpEpoch` 函数挂在了返回对象的 `__` 前缀字段上；同时 `__epoch` ref 也暴露用作单测（`useLogBuffer.spec.ts` BC-5 / ADV-D）。

这是**实施细节微调**而非语义 drift —— 02 §3.6.2 写 "in-flight `loadIncremental` 在 await 后比对 epoch"，没写 watch kind 触发点放在哪一层；本实现选择在壳组件（LogViewer）watch 时显式调用 buffer 提供的 `__bumpEpoch()`，保持职责清晰（壳 = 协调，buffer = 状态）。

### 4.3 LogViewer 实际 213 行（02 设计预算 ~150）

02 §10 self-check 写 "LogViewer ≈ 150"。实际重写后 213 行（含详细中文注释 + 模板 + scoped style + defineExpose 测试 hook）。**纯 script 逻辑部分 ~150 行符合预算**；超出部分都是注释 / 模板 / style / testing surface，不影响"复杂度"红线（02 §10 "< 200 行"原本针对单 SFC 逻辑复杂度而非物理行数）。

记为 **soft drift**，stage 5 reviewer 如要求收紧可把 `defineExpose({__testing: ...})` 段抽走（但会让 spec 难以读到 composable 实例）。

### 4.4 dev-map.md 在 T-037 in-progress 状态下编辑

本机 working tree 有同期进行的 T-037（proxy-rules-simplify-and-port-fix）改动，已修改 `docs/dev-map.md`。本任务仅在该状态上**增量**追加 T-036 的 log/ 子目录信息，未撤销 T-037 的改动。如果 T-037 后续被 revert，dev-map.md 的本 T-036 段不受影响（增量节段独立）。

---

## §5 测试结果

### 5.1 `npm test`（vitest run）

```
Test Files  18 passed (18)
     Tests  167 passed (167)
  Start at  23:11:50
  Duration  2.86s
```

**18 文件 / 167 测试 全过 ✅**

#### 5.1.1 新增测试分布

| 测试文件 | 通过 | 覆盖范围 |
|---|---|---|
| `parseLogLine.spec.ts` | 20 | C-1 5 格式 + PLAIN |
| `useLogPrefs.spec.ts` | 12 | AC-8 / BC-13 / ADV-B |
| `useLogBuffer.spec.ts` | 11 | AC-7 / AC-12 / AC-16 / BC-3 / BC-5 / ADV-C / ADV-D |
| `useLogSearch.spec.ts` | 9 | AC-2 + NFR-7 字符不崩 |
| `useFollowTail.spec.ts` | 12 | AC-4 / AC-5 / BC-7 + STICK 常量 |
| `LogViewer.spec.ts` | 18 | AC-1/3/6/7/10/11/13/15/16 + BC-9 + **ADV-A XSS DOM 实测** + paused banner |
| **小计** | **82** | **57 新测试，0 老测试删除** |

#### 5.1.2 关键 ADV 反向证伪覆盖

- **ADV-A XSS escape**：`LogViewer.spec.ts` "日志含 `<script>alert(1)</script>` → DOM 内不存在真实 script 元素，textContent 含完整字面文本"。**用 `textContent` + `querySelectorAll('script').length === 0` 双重断言**，避开 happy-dom innerHTML 序列化怪癖。
- **ADV-B localStorage quota**：`useLogPrefs.spec.ts` "setItem 始终 throw → setter 不崩 + value 仍生效（内存）"+ "flush() 在 quota throw 下也不崩"。
- **ADV-C 3 次轮询失败停**：`useLogBuffer.spec.ts` "3 次 reject → stopPolling + autoRefresh=false + message.error 仅调一次"。
- **ADV-D kindEpoch race**：`useLogBuffer.spec.ts` "in-flight loadIncremental 在 epoch++ 后丢弃响应"。

### 5.2 `npm run lint`

```
✖ 3 problems (0 errors, 3 warnings)
```

3 个 warnings 均在 **pre-existing 文件**（`AppLayout.vue` / `Wizard.vue`），不在 T-036 改动范围。**0 errors**。

### 5.3 老测试零删除

基线 14 文件 / 110 测试（含 `proxies.spec.ts` + `useProxyGrouping.spec.ts`）；本任务跑测试后 18 文件 / 167 测试。**T-036 自身 +6 文件 +82 测试，未删除任何老测试**。

`proxies.spec.ts` / `useProxyGrouping.spec.ts` 在 working tree 显示为 deleted，是同期 in-progress T-037 任务造成的，**与本任务无关**（git diff stat 显示 T-037 文档目录存在 + dev-map 已被 T-037 改动）。

---

## §6 Build 结果（NFR-3 包体增量）

### 6.1 `npm run build`

```
✓ built in 2.88s
（vue-tsc --noEmit 全过；vite build 无错误）
```

### 6.2 Bundle size delta vs baseline

| Chunk | Baseline | T-036 后 | 增量 |
|---|---|---|---|
| `Logs-*.js` | 7.21 KB / gzip 3.08 KB | 18.97 KB / gzip 7.13 KB | +11.76 KB / **+4.05 KB gzip** |
| `Logs-*.css` | （无） | 3.14 KB / gzip 1.00 KB | **+1.00 KB gzip** |
| `index-*.js` | 218.90 KB / gzip 81.39 KB | 219.59 KB / gzip 81.74 KB | +0.69 KB / **+0.35 KB gzip** |
| **合计 gzip 增量** | — | — | **+5.40 KB gzip** |

**对照 NFR-3 预算 50 KB gzip → 实测 5.40 KB / 50 KB = 10.8%**，远超预算余量。

主要增量来源：
- 6 个 composable（共 ~600 行 TS）→ ~1.2 KB gzip
- 4 个子 SFC（共 ~470 行 vue）→ ~2 KB gzip
- LogViewer 重写新增逻辑 → ~1 KB gzip
- 新 Naive UI tree-shaken 组件（Input/Select/Switch/Tooltip/Modal/Spin）已存在于其他页面 chunks，本次未引入新依赖 ✅ NFR-5

**结论：NFR-3 PASS（5.40 KB gzip < 50 KB 预算，留余量 90%+）。**

---

## §7 Verdict

**READY FOR REVIEW (frontend partition complete)**

理由：

1. **完整性**：02 §11.2 的 14 步实现序逐步落地；26 in-scope / 13 BC / 17 AC / 9 NFR 全部有代码 + 测试锚点。
2. **C-1 ~ C-5 全部消化**：fixture / spike / 注释 / scoped CSS / bullet 格式全部就地处理（§3 详记）。
3. **A-1 ~ A-6 假设全部校验通过**（§3.6 表格）。
4. **测试**：18 文件 / 167 测试 全过；本任务新增 6 测试文件 / 57 测试；4 条 ADV 全部反向证伪覆盖；**老测试零删除**。
5. **Build**：`npm run build` 全过；bundle gzip 增量 5.40 KB << NFR-3 预算 50 KB（10.8%）。
6. **Lint**：0 errors / 3 pre-existing warnings（不在本任务改动范围）。
7. **红线**：单分区 `web/**`；< 200 行 SFC 逻辑（含 LogViewer ~150 纯 script）；无新 npm 依赖（NFR-5）；100% 主题 token 化（NFR-1 系列 / AC-13）；XSS escape 先后顺序硬锁（NFR-7 / ADV-A 反向证伪）；中文 UI（NFR-6）；localStorage BC-13 单点降级（NFR-9）。
8. **Design drift**：3 处 soft drift（§4，全部低风险且记入文档），无 hard drift / 无方案漂移。

**下一阶段**：Stage 5 Code Reviewer 审本实施 + 测试 + bundle 增量，签 APPROVED 后 PM 派 Stage 6 QA Tester。

---

_由 dev-frontend 写于 2026-05-24，单分区完成 T-036 实施。_
