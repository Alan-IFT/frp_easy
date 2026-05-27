# 03 — Gate Review · T-041 server-monitor-page-ui

> Stage 3 / 7。Gate Reviewer 检视 01 / 02，给出 APPROVED / REJECTED / CONDITIONAL。

## 1. 上游产物校验

| 文档 | 存在 | 完整 | 自检通过 |
|---|---|---|---|
| 01_REQUIREMENT_ANALYSIS.md | ✅ | FR×8 / NFR×7 / AC×15 / BC×9 / 决策×10 / 风险×6 | ✅ |
| 02_SOLUTION_DESIGN.md | ✅ | 总览 + 文件清单 + 详细代码 + 测试计划 + 实现顺序 + SA self-check | ✅ |

## 2. 上下游对齐

| 检查 | 结论 |
|---|---|
| 02 §3.1 类型定义字段名是否与 T-039 后端 client.go JSON 标签一致 | ✅ 字段名匹配（`clientCounts` / `curConns` / `totalTrafficIn` 等），camelCase 一致 |
| 02 §3.2 API client 三个函数是否与 T-039 路由表对齐 | ✅ `/api/v1/server/runtime/info` / `/proxies` / `/traffic/{name}` 三条对应 |
| 02 §3.3 composable 失败处理与 T-039 handler 错误返回（502 / 503 / 404）的语义一致 | ✅ extractErrorMessage 走 axios error.response.data.error.message 路径，与 writeError 形状对齐 |
| AC-3 / AC-4 / AC-5 "dashboard 未启用 / 不可达 / 凭据失败" 三态文案是否对齐 T-039 handler writeError 文案 | ✅ T-039 handler 字面"frps dashboard 未启用..." / "frps 进程不可达..." / "frps dashboard 凭据校验失败..."，goServerHint computed 用 includes('dashboard 未启用')|includes('凭据') 命中 |

## 3. 红线扫描

| 红线 | 触发风险 | 评估 |
|---|---|---|
| 不允许静默方案漂移 | D-1（不展示 uptime）= 01 §FR-1.5 收敛 | ✅ 已在 01 §5.1 + 02 §3 显式标注；非静默 |
| 下游不能改上游文档 | 不涉及（T-039 已 archived） | ✅ |
| 测试数只升不降 | 新增 spec：useServerRuntime ~11 用例 + ServerMonitor ~11 用例 + qa_t041_adversarial ~4 用例 | ✅ 净增 ~26 |
| 不能有 secret | composable / api / page 不引入凭据 | ✅ |
| 完成前必须 verify_all | 02 §7 step 7 显式列出 | ✅ |
| 编辑 .harness 不编辑 .claude/CLAUDE.md | 本任务不动 agent 契约 | ✅ |

## 4. 决策合理性

| 决策 | 合理性评分 | 备注 |
|---|---|---|
| D-1（不展示 uptime） | ✅ 合理 | T-039 API 确无此字段；后续 T-039 扩展时增量补 |
| D-2（n-tabs 按 type 分组） | ✅ 合理 | 避免 T-037 group row 反模式（insight L26） |
| D-3（status 红色兜底） | ✅ 合理 | 容忍上游 frps 版本演进 |
| D-4（直接显示时间字符串） | ✅ 合理 | polling 不抖屏 |
| D-5（composable 显式 start） | ✅ 合理 | F-5.7 测试友好 + SSR 安全 |
| D-6（3 次失败自动停） | ✅ 合理 | 与 T-036 useLogBuffer 范式对齐（insight L27 H.1 范式延伸） |
| D-7（5 s 默认间隔） | ✅ 合理 | 见 D-7 注解（与 T-036 2s 日志 / T-038 10s 服务态 中间档） |
| D-8（不在本任务做 traffic 图） | ✅ 合理 | NFR-6 不引入新 npm 包；后续可加 echarts |
| D-10（不做 detail 抽屉） | ✅ 合理 | 范围克制；T-042 增量 |
| D-3.5（不引入 abort）| ✅ 合理 | epoch race 已防过期写入；abort 增量复杂度 > 收益 |

## 5. Conditions（可在 stage 4 一并消化）

下列均非阻塞，但 developer 应顺手在 04 处理，使 stage 5 一次过：

- **C-1（测试基线对齐）**：02 §4.1 / §4.2 用例数估算 ~26。dev 实测应有同等量级；如显著低于（例 < 18），需 04 §verify_all 段明示。
- **C-2（lastUpdatedLabel 不刷新问题）**：02 §3.4 注释中提到 `tickRef` 是 dummy（仅 polling 5 s 节拍刷新 lastUpdatedLabel）。这意味着用户暂停轮询后 lastUpdatedLabel 不会刷新（"刚刚刷新"会停留 5+ 分钟才跳"5 分钟前刷新"）。**建议 04 在页面 mount 时挂一个 1s 轻量 setInterval 仅为 tickRef + 1**（onUnmounted 清除）。或者保持现状 + 在 02/04 文档显式说明"暂停状态下时间标签不再实时跳"。Developer 自决，但要文档化。
- **C-3（FR-3.5 visibility 静默切换）**：02 §3.3 visibility 恢复时调 `void refresh()`。需测试覆盖：恢复 visible 后用户立即点"立即刷新"是否会并发触发两个 refresh 重叠（epoch race 会丢一个，但 isRefreshing 是 ServerMonitor.vue 层的，可能 UI flash "立即刷新" 按钮 disabled 一瞬）。建议 04 spec 加 1 个用例验证 visibility 恢复 → setInterval 立即拉一次。
- **C-4（formatBytes 小数位规则）**：02 §3.4 formatBytes 当前规则 "u===0 整数；u>0 toFixed(1) + 末尾 .0 去掉"。但 1023 字节 → "1023 B"（正常）；1024 → "1 KiB"；1500 → 1.46 KiB（toFixed → "1.5"，被 .replace 保留 → "1.5 KiB"，正确）；1536 → 1.5 KiB（正确）。**规则合理，无需改**。AC-13 验证三个边界（0 / 1024 / 1536）覆盖足够。
- **C-5（status 上游字段大小写）**：frps 上游可能返回 "Online"（首字母大写）还是 "online" 全小写？需 dev 在实现时用 toLowerCase 防御。建议 02 §3.4 status 比对前做 `s.toLowerCase()`，否则可能全部走红色兜底。**04 必须修：`const s = (row.status ?? '').toLowerCase()`**。
- **C-6（路由 children 顺序与导航 menu 对齐）**：02 §3.5 路由放在 `server` 之后；02 §3.6 menu 同位置。✓ 一致。无需改。

## 6. 风险审视

R-1（onUnmounted 在 setup 同步）：composable 现实现 `if (typeof document !== 'undefined') document.addEventListener(...)` 在 setup 同步路径 + `onUnmounted(() => {...})` 也在同步路径，✓。

R-2（fake timers 不拦 visibilitychange）：02 §3.3 已用 `opts.visibilityHidden` inject seam，测试可注入 mock fn，✓。

R-5（NResult 等组件未注册）：vi.mock importOriginal + spread 自动注入 真实 default export，所有 Nxxx 组件可用，✓。

## 7. 测试覆盖审视

- AC-1 → spec "mount + tick 显示 server info 卡 + tabs" ✓
- AC-2 → 由 spec polling timer trigger 覆盖 ✓
- AC-3 / AC-4 / AC-5 → spec "首屏失败 + 文案命中" ✓
- AC-6 → composable spec "refresh 失败保留上次数据" ✓
- AC-7 → composable spec "document.hidden = true → clearInterval；恢复 → 自动重启" ✓
- AC-8 → spec "点暂停按钮" ✓
- AC-9 → spec "点立即刷新" ✓
- AC-10 → composable spec "onUnmounted clearInterval + removeEventListener" ✓
- AC-11 → composable spec "连续 3 次失败 isPolling 自动 false" ✓
- AC-12 → spec "一类 type 错误一类有数据" ✓
- AC-13 → spec "流量字段 0/1024/1536" ✓
- AC-14 / AC-15 → spec "status online/offline" ✓
- BC-1 → spec "empty proxies" ✓
- BC-3 → spec "lastStartTime 0001-01-01" ✓
- BC-5 → composable spec "epoch race" ✓
- BC-7 → composable spec "用户显式 stop 后切后台再切回" ✓

QA stage 6 adversarial 4 项覆盖 AC-4 / AC-5 / AC-7 / D-6，与 spec 主用例正向覆盖互补。

## 8. Verdict

**APPROVED FOR DEVELOPMENT**。

Conditions C-1 ~ C-6 全部非阻塞、04 阶段顺手消化即可。其中 **C-5（status 大小写防御）是 must-fix**（看似小，但是 frps 上游字面真有可能不一致），C-2（tickRef 不刷新）和 C-3（visibility 恢复并发）是建议改善（可文档化代替修复，由 dev 自决）。

---

**Reviewer**：Gate Reviewer（PM 上下文角色化）
**Date**：2026-05-27
