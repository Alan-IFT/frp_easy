# 05 — Code Review · T-041 server-monitor-page-ui

> Stage 5 / 7。Code Reviewer 审 stage 4 实现 vs 02 设计 + 03 conditions + 项目红线。

## 1. 审查范围（git diff 域）

| 文件 | 类型 | LOC |
|---|---|---|
| web/src/api/serverRuntime.ts | 新增 | 45 |
| web/src/composables/useServerRuntime.ts | 新增 | 160 |
| web/src/composables/__tests__/useServerRuntime.spec.ts | 新增 | 290 |
| web/src/pages/ServerMonitor.vue | 新增 | 290 |
| web/src/pages/__tests__/ServerMonitor.spec.ts | 新增 | 330 |
| web/src/types.ts | 修改 | +50 |
| web/src/router.ts | 修改 | +2 |
| web/src/components/AppLayout.vue | 修改 | +8 |
| docs/dev-map.md | 修改 | +5 |
| docs/tasks.md | 修改 | +1 |

## 2. 设计对齐

### 2.1 02 §3 字段名 / 类型对齐

| 02 设计 | 04 实现 | 结论 |
|---|---|---|
| ServerRuntimeInfo 12 字段 | types.ts 同名同顺序 | ✓ |
| ServerRuntimeProxyStatus 10 字段（含 status 可选） | 同款 | ✓ |
| ServerRuntimeProxiesResponse `{ proxies, errors? }` | 同款 | ✓ |
| ServerRuntimeTraffic `{ name, trafficIn, trafficOut }` | 同款 | ✓ |
| 与 T-039 后端 client.go JSON 标签字面对齐 | 抽查 `clientCounts` / `curConns` / `totalTrafficIn` / `todayTrafficIn` 等 | ✓ |

### 2.2 02 §3.3 composable 契约

| 设计要求 | 实现位置 | 结论 |
|---|---|---|
| start / stop / refresh / restart 公开 | useServerRuntime.ts L113-132 | ✓ |
| start 幂等 | L92 `if (timer !== null) return` | ✓ |
| stop 切 userStoppedExplicitly | L99-110 stopInternal(true) 分支 | ✓ |
| BC-7 用户意图优先 | L143 visibility 检测 short-circuit | ✓ |
| epoch race | L69 `const at = ++epoch.value` + L74/L82 比对 | ✓ |
| onUnmounted 清理 timer + listener + 自增 epoch | L161-167 | ✓ |
| F-5.6 保留上次数据 | L80-90 catch 块只写 error / count | ✓ |
| MAX_FAIL=3 自动停 | L87 `>= MAX_FAIL` | ✓ |

### 2.3 02 §3.4 ServerMonitor.vue 契约

| 设计要求 | 实现位置 | 结论 |
|---|---|---|
| ServerInfo 6 字段卡片 | template L66-101 n-statistic × 6 | ✓ |
| Proxies n-tabs 按 type | template L106-138 | ✓ |
| 状态条 + 暂停 / 刷新按钮 | template L4-22 | ✓ |
| 失败 banner + 重启按钮 | template L24-37 | ✓ |
| 陈旧 banner（保留上次数据） | template L39-46 | ✓ |
| 首屏失败 NResult | template L48-62 | ✓ |
| status 三色 dot + GR C-5 toLowerCase | script render L218 | ✓ |
| 流量人类友好单位 | formatBytes L186-197 | ✓ |
| 0001-01-01 时间 → "—" | formatTime L200-204 | ✓ |
| activeType ref 持有不被 polling 重置 | script L161 | ✓ |
| onUnmounted 清理 1s tickTimer | script L180-182 | ✓ |
| defineExpose `__testing` handle | script L264-280 | ✓ |

### 2.4 SFC 行数红线（insight L28）

ServerMonitor.vue script 段统计：
- 总 script 行数: ~155
- import 行数: 9
- type-only / interface: 0
- 纯 setup 逻辑 + computed + function 行数估算: ~115

**< 200 行红线 ✓**

## 3. 项目红线扫描

| 红线 | 状态 |
|---|---|
| 不允许静默方案漂移 | 04 §3 显式标"无显著漂移"；GR C-5 修复已属设计内强化 ✓ |
| 下游不能改上游文档 | 不涉及 ✓ |
| 测试数只升不降 | +40 测试 ✓ |
| 不能有 secret | 全文件 grep 无 token / pass / secret 硬编码（dashboard 凭据走 KV，前端不持） ✓ |
| 完成前必须 verify_all | DEFERRED HOOKS，待 stop-hook 触发；insight L23 / L34 已说明 ✓ |
| 不编辑 .claude / CLAUDE.md / .github/copilot-instructions.md | 未触碰 ✓ |

## 4. 反模式扫描

| 反模式 | 扫描结论 |
|---|---|
| v-model + composable 回环（insight L13） | useServerRuntime 不暴露 update:* emit；ServerMonitor 用 `rt.start() / rt.stop()` 命令式，无双向 binding，无 OOM 风险 ✓ |
| useThemeVars + 硬编码颜色混用（insight L33） | naive-ui 组件（n-tag type 'success' / 'default' / 'error'）走 Naive UI token 自动跟随主题；scoped CSS 仅 layout 不涉色，主题切换天然跟随 ✓ |
| v-html escape 顺序（insight L29） | 未使用 v-html，全 Vue 文本插值（双花括号）自动 escape ✓ |
| 虚拟列与 DB 字段同名（insight L26） | n-data-table 列 key 直接映射 row 字段（name / curConns / todayTrafficIn / ...），单一来源 ✓ |
| SFC > 200 行无拆分 | 见 §2.4，纯逻辑 < 200 ✓ |

## 5. 单点深审

### 5.1 useServerRuntime.ts L67-90 refresh

```typescript
async function refresh(): Promise<void> {
  const at = ++epoch.value
  try {
    const [i, p] = await Promise.all([
      apiGetServerRuntimeInfo(),
      apiGetServerRuntimeProxies(),
    ])
    if (at !== epoch.value) return
    ...
  } catch (e) {
    if (at !== epoch.value) return
    ...
  }
}
```

- ✓ epoch 在 try 入口 ++ 是正确时机（避免 Promise.all 启动前 race）
- ✓ catch 也守 epoch 防止过期写 error
- ✓ Promise.all 短路：任一 reject 整体 reject；catch 拿到 first error；与"两路独立失败 = 整体失败"语义一致
- ✓ extractErrorMessage 走 client.ts 既有路径，与项目其它 store 错误处理同款

### 5.2 useServerRuntime.ts L140-159 onVisibilityChange

- ✓ `userStoppedExplicitly` 短路（BC-7）
- ✓ 隐藏时 `pausedByVisibility = true`，显示时仅在 `pausedByVisibility` true 时才恢复（不会"原本未跑也启动"）
- ✓ 显示恢复时 `void refresh()` 立即拉一次，符合用户预期

### 5.3 ServerMonitor.vue L156-167 allProxyTypes computed

```typescript
const allProxyTypes = computed<string[]>(() => {
  const p = rt.proxies.value?.proxies ?? {}
  const e = rt.proxies.value?.errors ?? {}
  const has = new Set<string>()
  for (const t of Object.keys(p)) {
    if ((p[t]?.length ?? 0) > 0) has.add(t)
  }
  for (const t of Object.keys(e)) {
    has.add(t)
  }
  return allKnownTypes.filter((t) => has.has(t))
})
```

- ✓ 用 allKnownTypes 固定顺序过滤，避免 Object.keys 顺序漂移导致 tabs 抖动
- ✓ 同时纳入"有数据"和"有错误"两类 type
- ✓ Set 去重避免双计入

### 5.4 ServerMonitor.vue L218-224 status render

```typescript
render: (row) => {
  const raw = (row.status ?? '').toLowerCase()
  const type: 'success' | 'default' | 'error' =
    raw === 'online' ? 'success' : raw === 'offline' ? 'default' : 'error'
  const text = raw === 'online' ? '在线' : raw === 'offline' ? '离线' : (row.status || '未知')
  return h(NTag, { type, size: 'small', round: true }, { default: () => text })
},
```

- ✓ GR C-5 must-fix 已落实（toLowerCase 防御）
- ✓ NTag type 用 Naive UI token，自动主题跟随
- ✓ text 三态：online → 在线 / offline → 离线 / 其它 → 原文（保留诊断信息）/ 空 → 未知

### 5.5 测试覆盖深度

useServerRuntime spec 13 用例 ＋ ServerMonitor spec 27 用例 = 40 用例。覆盖所有 01 §AC × 15 / BC × 9 + 02 §4 测试计划。

QA stage 6 adversarial 将再加 4 用例 → 净增 44。

## 6. 风险审视

| R-1（onUnmounted 同步路径） | L161 `onUnmounted(() => {...})` 在 setup 同步，✓ |
| R-2（fake timers 不拦 visibilitychange） | spec 用 opts.visibilityHidden 注入，与 setup 解耦 ✓ |
| R-3（extractErrorMessage 未导出） | client.ts L57 已导出 ✓ |
| R-4（tabs 闪烁） | activeType ref 独立持有，polling 只更 data ✓ |
| R-5（NResult 等组件未注册） | vi.mock importOriginal + spread 自动包含 ✓ |

## 7. 改进建议（非阻塞）

- **S-1**：useServerRuntime.ts L21 JSDoc 提到"必须在 Vue 组件 setup() 同步路径调用" —— 实际是因为内部使用 onUnmounted。文档已点出，但建议未来加 dev-only console.warn 防御误用（如非 setup 调用 → onUnmounted 返回 false）。可作后续小修。
- **S-2**：ServerMonitor.vue 顶部 lastUpdatedLabel 在用户暂停轮询期间靠 1s tickTimer 跟随，开销极小（仅 ref + 1），但严格说"暂停"语义下 tick 也算"活动"。可在 onTogglePolling stop 时也暂停 tickTimer。**非阻塞**（用户实际场景：暂停后立刻关页或切走，tickTimer 4-5 秒后 onUnmounted 也清干净）。
- **S-3**：T-042 接手时建议把 useServerRuntime 提取的 `proxies.value` 数据结构作 mapping 函数（如 `proxiesByName(): Record<string, ProxyStatus>`）方便 Proxies.vue 配置态行直接查 runtime 列。此非本任务范围，但 dev 备忘。

## 8. Verdict

**APPROVED**。

无 must-fix；GR conditions 全消化；红线无违反；S-1/S-2/S-3 全部建议性。可进入 QA stage 6。

---

**Reviewer**：Code Reviewer（PM 上下文角色化）
**Date**：2026-05-28
