# 07_DELIVERY — T-048 frontend-consistency-cleanup

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05 · dev-frontend 子 agent 实现，orchestrator 真跑 verify_all 闸门

## 需求

消除前端跨页/跨组件的不一致与重复实现（前端 UX 审计 C2/C4/C5/D3/D4/A4/E1，均低风险）。

## 改动

- **C4** UploadBinButton.vue 删局部 `formatBytes`（仅到 MiB、无负数防御），复用 `utils/format`。
- **C5** `utils/format.ts::formatTime` 改本地化 `toLocaleString('zh-CN',{hour12:false})`（保留空/"0001-"/Invalid-Date→"—" 防御）；Dashboard / ServiceStatusCard 删本地 formatTime 复用之 —— 消除"同一份 RFC3339 时间，监控页裸 ISO、仪表盘本地化"的跨页不一致（三份实现归一）。
- **C2** ServiceStatusCard.vue 加载/失败/时间戳的硬编码 `rgba(255,255,255,*)`/`#f00` 改 `n-text depth`/`type=error` 语义色，命令块背景改 `useThemeVars().codeColor` —— 修复浅底白字不可读。
- **D4** Dashboard.vue 两处"查看完整日志"由 `tag=a href`（SPA 整页刷新丢状态）改 `router.push`。
- **D3** Dashboard.vue `n-grid` 加 `cols="1 m:2" responsive="screen"`（对齐 ServerMonitor）。
- **A4** PublicIpDetector.vue catch 改 `extractErrorMessage` 透传后端精确原因。
- **E1** Dashboard.vue 进程操作 message 统一 `kindLabel`（frpc→客户端 frpc / frps→服务端 frps）+ `stateVerb` 按 store 真实 state 措辞（已启动/已停止/正在启动…），替代含糊的"指令已发送"。

## 验证

- `npx vitest run`：**342 passed**（327→342，+15）。`vue-tsc --noEmit` 净（补全 stateVerb 的 stopping 分支）；`eslint .` 净。
- orchestrator 真跑 `bash scripts/verify_all.sh`：**PASS 32 / WARN 0 / FAIL 0**。baseline.json v21（frontend_tests 342 / test_count 629）。
- C5 formatTime 级联：format.spec / ServerMonitor.spec 断言裸 ISO 的用例改为**时区稳定断言**（含年份 + `not.toMatch(/T..:..:..Z/)` + 同引擎 `toLocaleString` 对齐），非弱化。

## Adversarial tests

- formatTime 对空/"0001-"/无效字符串仍 →"—"（防 "Invalid Date" 外泄）；D4 改后断言 `router.push` 被调用而非 href 跳转；A4 反向用例刻意用非结构化 Error 验证 `extractErrorMessage` fallback 分支（不外泄裸 message）。

## Insight

- 同一数据的展示格式必须跨页统一：`formatTime` 三份实现（裸返回 / 两种 toLocaleString）让用户在监控页见裸 ISO、仪表盘见本地化 —— 抽到单一 util 并全局复用。本地化时间的测试断言必须时区稳定（断言"含年份/非裸 ISO" + 同引擎对齐），不能 hardcode 期望字符串。
- 浅色主题下严禁硬编码 `rgba(255,255,255,*)` 文字色（不可读）；用 `n-text` 的 `depth`/`type` 语义色或 `useThemeVars()` 变量，随主题自适应。
- SPA 内导航必须 `router.push`，`href`/`tag=a` 触发整页刷新丢 Pinia 状态 + 重跑路由守卫。
