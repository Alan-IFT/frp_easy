# 05_CODE_REVIEW — T-042 proxy-runtime-status-merge

> Stage 5 / Code Reviewer · 2026-05-28
>
> 输入：04_DEVELOPMENT.md + 实际文件 diff
> 输出：本文档（verdict 在文末）

## 1. 代码 / 设计一致性核验

| 02 § | 实现位置 | 一致 |
|---|---|---|
| § 3.1 utils/format.ts API | `web/src/utils/format.ts` | ✓（含 02 设计书没写但 04 § 3.4 documented 的负数防御 + PiB 单位扩展） |
| § 3.2 utils/proxyStatus.ts API + 语义合并 | `web/src/utils/proxyStatus.ts` | ✓ |
| § 3.3 Proxies.vue setup 扩展 | `web/src/pages/Proxies.vue` L96-115 + L211-244 | ✓ |
| § 3.3 onMounted 扩展 | `web/src/pages/Proxies.vue` L310-315 | ✓ |
| § 3.3 columns 追加 2 列于 "操作" 前 | `web/src/pages/Proxies.vue` L281-290 | ✓ |
| § 3.4 ServerMonitor refactor 限定三处 | `web/src/pages/ServerMonitor.vue` import 段 + inline 删除 + columns 状态 render | ✓ |
| § 3.5 4 个 spec.ts | `__tests__/format.spec.ts` + `__tests__/proxyStatus.spec.ts` + `__tests__/Proxies.spec.ts` + `__tests__/qa_t042_adversarial.spec.ts` | ✓ |

## 2. 代码质量评估

### 2.1 SFC 行数（insight L31）— PASS

- `Proxies.vue` script 段物理 274 行；纯逻辑 ~198 行 < 200 ✓
- `ServerMonitor.vue` refactor 后净减 17 行 — 持续在 < 200 行健康区

### 2.2 单向数据流（insight L13）— PASS

- 代码层：Proxies.vue 内只用 `runtime.proxies.value` / `runtime.error.value` 读
- 静态层：ADV-5 用 fs 读 Proxies.vue 源码 grep `v-model[^=]*=["'][^"']*runtime` 反向证伪 0 命中
- 进程层：`setInterval` / `addEventListener` 也 grep forbid（让 useServerRuntime 自管）

### 2.3 DRY（utils 抽取）— PASS

- formatBytes / formatTime 两处使用同源
- getProxyStatusTag 两处使用同源
- 未来若再加第三方使用（如 Server.vue 状态徽章），无需重复实现

### 2.4 大小写防御 / 边界值（insight T-041 GR C-5 范式）— PASS

- proxyStatus.ts 内 `toLowerCase()` 兜底
- formatBytes 覆盖 undefined / null / NaN / 负数 / 0 / 1023 / 1024 / 1MiB / 1GiB / MAX_SAFE_INTEGER → PiB
- formatTime 覆盖 "" / null / undefined / "0001-..." / 正常字符串
- 全部边界在 unit test 中显式断言

### 2.5 命名一致性 — PASS

- `runtimeMap` / `runtimeUnavailable` / `renderRuntimeStatus` / `renderRuntimeTraffic` 命名清晰
- 与 T-041 useServerRuntime API 名（`info` / `proxies` / `error` / `start` / `refresh`）对齐

### 2.6 注释充分性 — PASS

- utils 顶端注释明示来源（T-041 字节级搬运）+ 决策（负数防御 / 语义合并）+ 共享方
- Proxies.vue 关键 setup 块（runtimeMap / runtimeUnavailable / renderRuntimeStatus）都有 inline 注释
- 既有 T-032 注释保留（formData 单向种子）

## 3. 测试覆盖度评估

| 测试目标 | 覆盖手段 | 覆盖度 |
|---|---|---|
| formatBytes 边界 | format.spec.ts 13 个用例 | 高 |
| formatTime 边界 | format.spec.ts 7 个用例 | 高 |
| getProxyStatusTag 状态 | proxyStatus.spec.ts 11 个用例 | 高 |
| Proxies happy path | Proxies.spec.ts 3 个用例 | 中 |
| 反向构造 AC-4 / AC-5 | Proxies.spec.ts 2 个用例 | 高 |
| 降级 AC-6 | Proxies.spec.ts 2 个用例 | 高 |
| CRUD 通路零回归 | Proxies.spec.ts 3 个用例 | 中 |
| 列拓扑（拒同列名歧义） | Proxies.spec.ts 2 个用例 | 高 |
| adversarial ADV-1 ~ ADV-6 | qa_t042_adversarial.spec.ts 6 类 | 高 |
| 既有 ServerMonitor.spec.ts 回归 | 既有 30+ 用例（未改一行） | 高（AC-9 守门） |

## 4. 潜在问题与缓解

### Issue-1：Proxies.vue 纯逻辑行数 ~198 临界

**严重度**：低
**缓解**：04 § 2 C-1 已 documented；未来若再加 runtime 相关 setup ≥ 5 行，建议拆 `composables/useProxyRuntimeMerge.ts`。当前不需要返工。

### Issue-2：双 polling 实例（ServerMonitor + Proxies 各一份）

**严重度**：低
**缓解**：02 § 6 决策矩阵已选择此方案，理由可接受；frps 请求量翻倍可接受。若未来需共享，需重写 useServerRuntime 为单例 + ref count——属下个任务。

### Issue-3：runtimeMap 同名 last-wins（同名不同 type）

**严重度**：极低（frps 端理论不会发生）
**缓解**：ADV-3 已 documented；若未来需严格区分，需 `Map<type+name, Status>` 复合 key。当前不需要。

### Issue-4：tooltip 文案换行用 `\n` + `white-space: pre-line`

**严重度**：低
**缓解**：Naive UI NTooltip 透传 default slot 文本；用 inline style 触发 pre-line 是标准用法。若未来需结构化，可改 `default: () => h('div', ...)`。

### 没有 P0 / P1 issue。

## 5. 回归风险评估

| 路径 | 风险 |
|---|---|
| Proxies.vue 配置 CRUD（新增/编辑/删除） | 零（未触碰任何 handler / store 调用） |
| ServerMonitor.vue 既有功能 | 极低（只换 import + 3 行 render，behavior 字节级保持） |
| useServerRuntime composable | 零（未改一字节） |
| 后端 API / openapi.yaml | 零（无后端改动） |
| verify_all step | 零（无新增 step） |

## 6. 建议（可选优化，不阻塞 review）

- `getProxyStatusTag` 颜色硬编码 hex 可考虑改为读 CSS 变量；目前不影响功能
- format.spec.ts 中 `formatBytes(Number.MAX_SAFE_INTEGER)` 验证 PiB；可考虑追加一个明确等于 "8 PiB" 之类的字面断言以更精确

## 7. 闸门判定

- 设计一致性：A
- 代码质量：A
- 测试覆盖：A
- 回归风险：A（极低）
- 文档同步：A（dev-map.md 同步）

## Verdict

**APPROVED**

无需返工。可进入 stage 6 QA。

— end —
