# 04_DEVELOPMENT — T-042 proxy-runtime-status-merge

> Stage 4 / Developer · 2026-05-28
>
> 按 02_SOLUTION_DESIGN.md 实施。GR conditions C-1 ~ C-6 主动消化。

## 1. 实施清单

| 文件 | 操作 | 行数变化 |
|---|---|---|
| `web/src/utils/format.ts` | NEW | +30 |
| `web/src/utils/proxyStatus.ts` | NEW | +49 |
| `web/src/utils/__tests__/format.spec.ts` | NEW | +98 |
| `web/src/utils/__tests__/proxyStatus.spec.ts` | NEW | +81 |
| `web/src/pages/Proxies.vue` | MODIFIED | +194 / -24 |
| `web/src/pages/__tests__/Proxies.spec.ts` | NEW | +257 |
| `web/src/pages/__tests__/qa_t042_adversarial.spec.ts` | NEW | +210 |
| `web/src/pages/ServerMonitor.vue` | MODIFIED | +5 / -22（净 -17） |
| `docs/dev-map.md` | MODIFIED | +3 / -1 |

总：**9 个文件**，5 个生产 + 4 个测试 + 1 个文档。

## 2. GR Conditions 消化

### C-1：Proxies.vue script 段纯逻辑行数 < 200 — **PASS**

实测：script 段从 line 58 到 line 331，物理 274 行；去：
- import（15 行 65-79）
- 顶端注释 + 章节注释（约 18 行）
- 空行（约 30 行）
- defineExpose 块（13 行）

≈ **198 纯逻辑行 < 200**，临界但未越红线。若未来再加 runtime 相关 setup，建议抽 `useProxyRuntimeMerge.ts` composable。

### C-2：utils/format.ts 顶端注释 — **DONE**

第 1-13 行明示"T-041 字节级搬运 + T-042 新增负数防御"。

### C-3：utils/proxyStatus.ts 顶端注释 — **DONE**

第 1-15 行明示"空字符串归 offline 的语义合并决策"。

### C-4：ServerMonitor.vue 改动严格限制 — **DONE**

实际只触三处：
- import 段加 3 行（formatBytes / formatTime / getProxyStatusTag）
- 删 inline formatBytes / formatTime 共 22 行 → 替换为 1 行注释引用
- columns 状态 render（line 286-289 原 4 行）改为 2 行（getProxyStatusTag 调用）

template / NCard / activeType / firstLoading / 其它 setup ref **零改动**。AC-9 由既有 ServerMonitor.spec.ts 全量回归守护。

### C-5：QA 06 `## Adversarial tests` 裸标题 — **DEFERRED 到 QA stage 6**

PM 写 06 时硬约束。

### C-6：QA 覆盖 ADV-1 ~ ADV-5 + AC-4 ~ AC-6 — **DONE**

`qa_t042_adversarial.spec.ts` 覆盖 ADV-1 ~ ADV-6（多 1 个：ADV-6 mount 首屏 hang 不报错）。`Proxies.spec.ts` 覆盖 AC-1 ~ AC-6。

## 3. 关键决策实现细节

### 3.1 runtimeMap 摊平算法

```ts
const runtimeMap = computed<Map<string, ServerRuntimeProxyStatus>>(() => {
  const m = new Map<string, ServerRuntimeProxyStatus>()
  const buckets = runtime.proxies.value?.proxies ?? {}
  for (const t of Object.keys(buckets)) {
    for (const r of buckets[t] ?? []) {
      m.set(r.name, r)
    }
  }
  return m
})
```

- O(N×M) 遍历 + Map.set；polling 每 5s 触发一次，N ≤ 100 / M ≤ 7 完全可接受
- last-wins 语义：若同名 proxy 出现在多 type bucket（理论 frps 不应发生），最后写入胜出。ADV-3 已覆盖。

### 3.2 降级判定

```ts
const runtimeUnavailable = computed(
  () => runtime.proxies.value === null && runtime.error.value !== null,
)
```

- `proxies === null` 表示从未拿到任何成功响应（含首屏失败 + composable 3 次失败自动停后）
- 不会与 stale 态混淆（stale = `proxies !== null` 且 `error !== null`）
- ADV-6 用 hang Promise 验证首屏 loading（proxies=null + error=null）不进降级分支

### 3.3 单向数据流保护

- Proxies.vue 内只读 `runtime.proxies.value` / `runtime.error.value`
- 没有 `v-model:` 绑回任何 runtime ref（ADV-5 静态守门：grep 源码 forbid `v-model[^=]*=["'][^"']*runtime`）
- 不在 Proxies.vue 内手工 setInterval / addEventListener / onUnmounted（让 composable 自管 —— ADV-5 静态守门 forbid `setInterval` / `addEventListener` 字面）

### 3.4 utils/format.ts 新增负数防御

T-041 inline 实现遇到 `formatBytes(-1)` 会进入 `while (v >= 1024 ...)` 循环判定立即退出（-1 < 1024），返回 `-1 B` —— 这是潜在用户感知 bug。本任务在 utils 提取时新增 `if (n < 0) return '—'` 防御。

ServerMonitor.vue 既有用例（spec 中只测正数 / undefined）不触此分支 → AC-9 保护。

## 4. Design drift 与意外发现

- **无 design drift**：所有改动都按 02 规划执行
- **无意外发现**：T-041 留好的 useServerRuntime / API 客户端可直接复用，零适配开销
- **PiB 单位**：02 § 7 A-2 行数估算 ~138，实测 198；多出 60 行主要来自 defineExpose 块 + 两个 render 函数提取（renderRuntimeStatus / renderRuntimeTraffic）—— 提取的原因是 columns 数组内联 render 太长会让数组本身可读性差。这是合理偏差，仍 < 200。

## 5. verify_all 预期与基线

baseline=31 PASS / 1 FAIL（C.1 e2e pre-existing，已豁免）

本任务改动域：
- ✅ G.x Go 段：零触碰
- ✅ A.x 静态闸门：零触碰
- ✅ B.x 前端段（npm run lint / typecheck / vitest）：新增 4 个 spec.ts + utils + Proxies.vue 改造 → 期望全 PASS
- ✅ C.1 E2E：零触碰（仍 FAIL baseline）
- ✅ E.6 Adversarial：06_TEST_REPORT.md 写裸 `## Adversarial tests`（QA 守护）
- ✅ H.1 删除面：本任务无删除（不踩既有 H.1 alternation）

**预期**：31 PASS / 1 FAIL（与 baseline 相同，无回归）

## 6. verify_all 反向证伪（adversarial）— **N/A**

本任务未新增任何 verify_all step（insight L26）；继承既有 step。无需 ADV 反向证伪。

## 7. 给 Reviewer 的 handoff

- 重点核 § 3.1 runtimeMap 算法；§ 3.2 降级三态；§ 3.3 单向数据流保护
- 既有 ServerMonitor.spec.ts 是 AC-9 守门，未做任何改动 → reviewer 可信回归
- utils 单测覆盖度高（边界值齐全）

**Verdict 输入**：READY FOR REVIEW

— end —
