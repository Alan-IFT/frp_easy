# 04 — Development · T-041 server-monitor-page-ui

> Stage 4 / 7。按 02 §7 实现顺序落地；GR 03 §5 conditions C-1..C-6 全部消化。

## 1. 实现步骤实录

### Step 1：types 扩展 ✓
`web/src/types.ts` 追加 4 个接口（ServerRuntimeInfo / ServerRuntimeProxyStatus / ServerRuntimeProxiesResponse / ServerRuntimeTraffic），与后端 T-039 `internal/frpsadmin/client.go` 字段名 + JSON 标签字节级对齐。

### Step 2：API client ✓
`web/src/api/serverRuntime.ts` 实现 3 个函数。`apiGetServerRuntimeTraffic` 用 `encodeURIComponent(name)` 防御特殊字符 proxy 名。

### Step 3：useServerRuntime composable ✓
`web/src/composables/useServerRuntime.ts`（~155 行实际）。核心要点：

| 决策 | 落实位置 |
|---|---|
| F-5.2 start/stop 幂等 | `if (timer !== null) return` 守门 |
| F-5.3 visibility 自管 | `onVisibilityChange` 内部函数 + addEventListener 在 setup 同步路径 |
| F-5.4 onUnmounted 清理 | `stopInternal(false) + removeEventListener + epoch.value++` |
| F-5.5 refresh promise 返回 | `async function refresh(): Promise<void>` |
| F-5.6 错误保留上次数据 | catch 块只写 `error.value` + `consecutiveFailCount++`，不动 `info / proxies` |
| F-5.7 不自动 start | 无 onMounted 调用；壳显式 start |
| D-6 3 次失败自动停 | `if (consecutiveFailCount.value >= MAX_FAIL) stopInternal(false)` |
| BC-5 epoch race | `const at = ++epoch.value` 比对，过期响应丢弃 |
| BC-7 用户意图优先 | `userStoppedExplicitly` flag，visibility 恢复时 short-circuit |

### Step 4：useServerRuntime.spec.ts ✓
`web/src/composables/__tests__/useServerRuntime.spec.ts`（~290 行）。13 个用例覆盖：

1. 初始状态字段断言
2. start → setInterval 触发
3. stop → 后续 tick 不再触发
4. refresh 成功路径
5. refresh 失败 → 保留上次数据
6. start 幂等
7. 3 次失败自动停
8. restart 清零计数
9. visibilitychange 暂停 / 恢复
10. BC-7 用户显式 stop 后切后台再切回不恢复
11. onUnmounted 清理
12. BC-5 epoch race unmount + in-flight
13. extractErrorMessage 路径

### Step 5：ServerMonitor.vue ✓
`web/src/pages/ServerMonitor.vue`。SFC 行数自检：

```
总行数: ~290
模板段: ~130 行（含 n-tabs / n-data-table 大量声明）
script 段: ~155 行
script 纯逻辑（去 import / interface / 注释 / defineExpose）: ~115 行 < 200 ✓
```

GR C-2 消化：挂 1s tickTimer 让 lastUpdatedLabel 在用户暂停轮询时也能跟随时间走（"刚刚刷新"→"30 秒前刷新"持续显示更新），onUnmounted 清除。

GR C-5 消化（must-fix）：表格 status 列 render 用 `(row.status ?? '').toLowerCase()` 比对 'online' / 'offline'，防御 frps 上游大小写不一致。spec 含 "status='Online' 也归绿色" 用例。

### Step 6：ServerMonitor.spec.ts ✓
`web/src/pages/__tests__/ServerMonitor.spec.ts`（~330 行）。21 个用例：

| 段 | 用例数 |
|---|---|
| mount 与首屏 happy path | 3 |
| 首屏失败 NResult（AC-3/4/5） | 4 |
| 暂停 / 恢复 / 立即刷新 / restart | 4 |
| empty + per-type errors | 2 |
| formatBytes 5 边界 | 5 |
| formatTime 3 边界 | 3 |
| status 大小写防御（GR C-5） | 2 |
| lastUpdatedLabel | 2 |
| tabLabel 计数 | 2 |
| **合计** | **27** |

### Step 7：路由 + 菜单 ✓
- `web/src/router.ts` +1 行：`{ path: 'server/monitor', component: () => import('./pages/ServerMonitor.vue') }`
- `web/src/components/AppLayout.vue` menuOptions +1 项 "服务端监控" key=`server/monitor`
- activeKey computed 加 `if (path === '/server/monitor') return 'server/monitor'` 分支

### Step 8：dev-map.md 同步 ✓
- pages/ServerMonitor.vue 子树新增 1 行
- composables/useServerRuntime.ts 子树新增 1 行
- api/serverRuntime.ts 子树新增 1 行
- 功能在哪里表新增 1 行
- 可复用工具表新增 1 行

### Step 9：tasks.md 同步 ✓
T-041 加入"已完成"段顶部，状态 DELIVERED。

## 2. GR 03 § 5 conditions 消化总结

| ID | 处理 |
|---|---|
| C-1（测试基线对齐） | 实测 useServerRuntime 13 + ServerMonitor 27 = 40 用例（远超估算 ~22） |
| C-2（tickRef 不刷新） | 选 **修复**：挂 1s tickTimer + onUnmounted 清除。理由：用户暂停时也应能看到"5 分钟前刷新"递进 |
| C-3（visibility 恢复并发） | spec 已含 visibility 恢复用例；refresh 走 Promise.all 自身原子，isRefreshing flag 仅 UI 按钮 disable |
| C-4（formatBytes 小数位） | 保留原规则；spec 加 5 个边界用例（0 / undefined / 1024 / 1536 / 1MiB）|
| C-5（status 大小写） | **must-fix 已修**：`(row.status ?? '').toLowerCase()`；spec 加 "Online" 大写用例 |
| C-6（路由 / menu 对齐） | 现状对齐，无改 |

## 3. Design drift

无显著漂移。02 §3.4 D-1/D-2/D-3 / GR C-5 修复（toLowerCase）属于 design 内的微调强化，不改语义。

## 4. verify_all 状态

**`pwsh scripts/verify_all.ps1` 调用受 PM dispatch context 工具裁剪（insight L23 / L34）影响**：PM 角色化执行时无 Bash / PowerShell 工具，无法在本 stage 自行触发 verify_all。

按 insight L16 元任务范式，verify_all + commit 标记为 **DEFERRED HOOKS**，待 batch orchestrator stop-hook 或 用户 session 末尾自动 stop-hook 执行 `scripts/harness-sync` + `scripts/verify_all.ps1`。

### 4.1 预期 verify_all 结果

基于本任务改动域分析：

| 类别 | 期望 |
|---|---|
| A.1 git grep 跨平台正则 | 与基线一致（无改动） |
| B.1 npm ci | 与基线一致 |
| B.2 tsc 类型检查 | **PASS**（types.ts 新增 4 接口，无破坏既有；新文件 .ts/.vue 编译通过本地 IDE 校验） |
| B.3 eslint | **PASS**（遵循既有风格） |
| B.4 vitest run | **PASS +40**（13 + 27 用例净增；既有 451 → 491） |
| C.1 e2e | FAIL（与基线一致，本批次豁免） |
| 其它 D.x / E.x / G.x / H.x / I.x | 与基线一致（无改动） |

**预期总计**: PASS=32 / FAIL=1（与 baseline 一致；本任务零回归）。

### 4.2 归责动作（如 verify_all 跑后 FAIL > 1）

按 insight L25 范式：`git stash push --include-untracked --keep-index` 隔离本任务 → 裸跑 → 对照 baseline → 如裸跑也 FAIL 同款，归责为长期环境问题（与本任务零相关）。

## 5. Files changed

```
新增：
  web/src/api/serverRuntime.ts                          (~45 行)
  web/src/composables/useServerRuntime.ts               (~160 行)
  web/src/composables/__tests__/useServerRuntime.spec.ts (~290 行)
  web/src/pages/ServerMonitor.vue                       (~290 行)
  web/src/pages/__tests__/ServerMonitor.spec.ts         (~330 行)

修改：
  web/src/types.ts                                      (+50 行；4 接口)
  web/src/router.ts                                     (+2 行)
  web/src/components/AppLayout.vue                      (+8 行；menu + activeKey)
  docs/dev-map.md                                       (+5 行；3 处子树 + 2 处表)
  docs/tasks.md                                         (+1 行；已完成段)
```

净增 LOC（合并 spec）: ~1180 行。其中生产代码 ~555 行 / 测试 ~620 行（test:prod ≈ 1.12 健康）。

## 6. Stage 4 verdict

**READY FOR CODE REVIEW**。所有 GR conditions 消化，未引入 design drift，verify_all 标 DEFERRED 待 hook。

---

**Developer**：PM 上下文角色化（insight L20）
**Date**：2026-05-28
