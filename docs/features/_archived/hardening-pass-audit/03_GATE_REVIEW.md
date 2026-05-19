---
task: hardening-pass-audit
task_id: T-007
stage: 03_gate_review
mode: full
date: 2026-05-19
author: gate-reviewer
status: APPROVED WITH CONDITIONS
---

# 03 Gate Review — T-007 hardening-pass-audit

## 复核摘要

本任务范围聚焦、增量小、无 schema 迁移、无新依赖。9 个 AC 全部可机器验证或人工可观察。SA 在 02 中给出了**逐分支映射表**（AC-5）、**Q-1..Q-4 显式决议**、以及大部分实现位置的精确行号引用，整体设计质量高。

独立核对了所有引用源代码（render.go / manager.go / router.go / proxies.go / handlers_proxies.go / handlers_logs.go / tail.go / middleware.go / main.go / Dashboard.vue / Proxies.vue / ProxyForm.vue / useProxyForm.ts）及 insight-index，未发现 BLOCKER 级问题。共发现 **2 个 WARN + 5 个 NIT**，Developer 在 04 阶段处理，不需要回退到 RA 或 SA。

## AC 复核

### AC-1（frpconf AtomicWrite chmod 0o600）— PASS
- SA 设计 chmod tmpName + chmod path 双重保险，正确。
- 跨平台说明合理：Windows 上 `os.Chmod(0o600)` owner-write 位为 1 → 不触发 ReadOnly attr，不破坏 rename。
- 测试 `TestAtomicWriteOverwriteExistingPermissions` 主动加测目标已存在旧权限场景，加分。

### AC-2（日志权限 0o600）— PASS
- main.go:93 与 manager.go:378 两处目标已核实。
- SA 主动加 `os.Chmod(path, 0o600)` 兜底升级路径（老用户已有 0o644 文件）。
- REQ B-2 已明确"子进程通过 toml `log.to` 自行打开文件不在范围"。

### AC-3（SecurityHeaders 中间件）— PASS
- 实现正确：`w.Header().Set(...)` 在 `next.ServeHTTP` 之前 → 所有响应（含 panic recover）都设头。
- 挂载位置正确：`r.Use(SecurityHeaders())` 必须在 `r.Get("/api/v1/health", h.health)` 之前。
- 与 dev 模式 CORS 不冲突（不同 header 集合，无 Set 覆盖）。X-Frame-Options 不影响 fetch/XHR。

### AC-4（日志 2 MiB 上限）— PASS WITH NOTE
- **SA 已识别现状**：tail.go L111-115 已有 `maxReadPerCall = 1 << 20` (1 MiB)。REQ 假设"无上限"是审计输入错误；但 AC-4.1 仍是可执行的客观断言（5MiB 文件 offset=0 → len=2MiB），不需要回退 REQ。
- 决策方案 A（直接改常量并导出为 `MaxReadBytes`）最简洁。

### AC-5（procmgr.Start defer-unlock）— PASS
- 逐行核对 SA 给出的"逐分支映射表"：原代码 7 个状态转换点（SA 写 6 处实际 7 处，数量小差异，不影响正确性）。
- 关键不变量保持：emit 在 unlock 之后；waitUntilStable 在 unlock 之后；StateStarting 设置在持锁内；info 快照后才 emit。
- 必须确保新结构通过 `-race` 测试（AC-5.2）且现有 procmgr 测试不修改即通过（AC-5.3）。

### AC-6（ErrDuplicateName + 409）— PASS WITH NOTE
- 现有 handlers_proxies.go L249 用 `strings.Contains(low, "unique")` 文本匹配并 422；SA 改为先 `errors.Is(storage.ErrDuplicateName)` → 409，剩余兜底走原 422 分支。
- 驱动一致性：项目用 `modernc.org/sqlite`（store.go L23）；其 UNIQUE 错误文本与 mattn 一致，双关键字匹配健壮。
- **NOTE**：建议 Developer 在动手前 grep 一眼 migrations 确认 UNIQUE 索引存在与文本形式。

### AC-7（Dashboard 错误显示）— PASS
- Dashboard.vue L50 与 L120 现有 style 已含 `white-space: pre-wrap`，SA 加 `word-break: break-word` 解决长 token 不换行。
- R-4 提到"父容器固定高度 + overflow:hidden"不存在（NAlert 在 NCard 内，NCard 无固定高度）。

### AC-8（Proxies 删除清理 + 空状态）— PASS
- 当前 `handleDeleteConfirm` 确实没清 firewallPorts；SA 在 try 成功路径内加 `firewallPorts.value = []; firewallProto.value = 'both'`。
- 空状态文案"暂无代理规则，点击右上角「新增规则」开始配置"与现有按钮文案一致。

### AC-9（ProxyForm 类型切换重置）— PASS WITH CONDITIONS
- **核心 reentrancy 分析**：ProxyForm.vue L105 已有 `watch(props.modelValue, syncFromInput, { deep: true })`。`syncFromInput` 先写 type 再写 customDomains。
- Vue 3 默认 `flush: 'pre'` → watch callback 在 syncFromInput 整个微任务完后才统一触发，此时 form 已是完整新状态，watch 内调 handleTypeChange('http') 把 remotePort=null（无害）、不动 customDomains。✓ 安全。
- **但如果 Developer 误用 `{ flush: 'sync' }`** → syncFromInput 写完 type 立刻清空 customDomains，下一行赋值才补回，可能丢失合法初始值。
- **CONDITION**：watch **不要**加 `flush: 'sync'`，必须补"编辑现有 HTTP 规则加载初始 customDomains 不丢"组件单测。

## 设计复核

- 9 IS → 9 AC 一对一映射，无遗漏。
- Module map 估行数合理（后端 ~480 / DB ~80 / 前端 ~90）。无新依赖、无迁移、无新路由。
- 跨平台一致性：AC-1/AC-2/AC-5 都处理了 Windows/POSIX 差异。
- 测试位置全部存在或可新建。

### 与 insight-index 一致性

- 2026-05-16 第 1 条（Windows os.Rename 覆盖）：AtomicWrite 保留 rename 不变量。✓
- 2026-05-16 第 3 条（openapi 以 Go 常量为权威）：SA 决定本任务内同步 openapi.yaml。✓
- 2026-05-17 第 1 条（NMessageProvider 必须在 App.vue）：本任务不动 App.vue。✓
- 2026-05-17 第 2 条（go:embed 时间戳重建）：完成前端改动后须重建二进制。✓

### Open Questions 决议
- Q-1（Chmod vs O_EXCL）：选 (a)。✓
- Q-2（中间件挂顶层）：选 (a)。✓
- Q-3（openapi 同步）：选 (a)。✓
- Q-4（空状态文案）：选 (b)。✓

## 跨切关注

### 测试策略
- AC-3 HTTP headers：可自动化（httptest）。
- AC-7 前端 alert 样式：自动化困难，选 grep + 人工验证。
- AC-9 测试缺一项（见 WARN-1）。

### 安全
- SecurityHeaders 与 CORS 共存无冲突。
- X-Frame-Options: DENY 对 127.0.0.1 单机定位无副作用。
- ErrDuplicateName 不泄露 SQL 原始错误。
- AC-1 chmod 失败时返错（fail closed）。

### 可维护性
- AC-5 是纯维护性改进，无行为变化。
- AC-6 sentinel 让未来 sqlite 驱动错误文本变化的破坏面集中在 `isDuplicateNameError`。
- AC-9 `handleTypeChange(newType?)` 兼容旧 select 调用。

### 性能
- 日志 2 MiB 单次峰值可忽略。
- procmgr.Start 重构不增 goroutine、不延长持锁。

## 发现的问题

### WARN-1（前端 / AC-9）：ProxyForm reentrancy 测试缺失 + flush 隐患
- SA 列出的 4 个测试中没有"编辑现有 HTTP 规则加载初始 customDomains 不被清空"用例。
- 建议（Developer 在 04 处理）：
  1. 不要在 `watch(() => form.value.type, ...)` 加 `flush: 'sync'`。
  2. 补组件单测：mount ProxyForm with `modelValue = { type: 'http', customDomains: ['example.com'], ... }`，断言 `form.value.customDomains` 等于 `['example.com']`。同样测 TCP+remotePort 编辑路径。

### WARN-2（后端 / AC-6）：UNIQUE 约束的 schema 验证未在设计中显式确认
- SA 假设 proxies 表对 name 列有 UNIQUE 约束。
- 建议（Developer 在 04 处理）：
  1. 实施 AC-6 前 `grep -n "UNIQUE\|CREATE.*INDEX" internal/storage/sqlmigrations/*.sql` 确认。
  2. 把发现写入 `isDuplicateNameError` 注释。
  3. 测试 AC-6.1 必须真的跑通。

### NIT-1（后端 / AC-5）：原 unlock 调用次数计数误差
SA §AC-5 写"6 处"，实际 7 处。重构正确性不受影响。

### NIT-2（后端 / AC-4）：MaxReadBytes 单位描述
`2 << 20 = 2 MiB`。✓ 正确。提示 Developer 注释里也可写 `2 * 1024 * 1024`。

### NIT-3（后端 / AC-2）：升级路径 chmod 责任边界
FRP 子进程可能用 `log.to` 重开同名文件并改回权限。在 manager.go::supervise 注释明确"chmod 仅保证 UI 进程 tee 的那次创建是 0o600，上游 FRP 后续 OpenFile mode 由其内部决定"。

### NIT-4（OpenAPI / AC-6）：PUT 409 描述合并
建议用 `oneOf` examples 给两种 409 各一示例 body。非阻塞。

### NIT-5（前端 / AC-8）：删除失败时不清 firewallPorts
清空动作在 `await deleteProxy` 成功之后；不要放到 finally 块。

## 是否回退

**否**。无 BLOCKER。

## 决议

**VERDICT: APPROVED WITH CONDITIONS**

**Conditions（Developer 在 04 阶段必须满足）**：
- **C-1（AC-9 强制）**：ProxyForm.vue 新增的 `watch(() => form.value.type, ...)` **不得**用 `{ flush: 'sync' }`；必须新增组件单测覆盖"编辑 HTTP 规则加载时 customDomains 不丢失"与"编辑 TCP 规则加载时 remotePort 不丢失"。
- **C-2（AC-6 强制）**：实施 sentinel 识别前必须验证 `internal/storage/sqlmigrations/*.sql` 中 proxies.name 的 UNIQUE 约束形式，并据此确认 `UNIQUE constraint failed: proxies.name` 是 modernc.org/sqlite 实际输出文本。
- **C-3（AC-5 强制）**：procmgr.Start 重构后 `go test ./internal/procmgr/... -race` 必须 PASS，且现有所有 procmgr 测试不修改测试代码即通过；AC-5.1 grep 必须满足。
- **C-4（AC-3 强制）**：`r.Use(SecurityHeaders())` 必须在 `r.Get("/api/v1/health", h.health)` 之前调用。
- **C-5（AC-2 推荐）**：在 manager.go::supervise 加 chmod 时注释说明 FRP 子进程权限的归责边界。
- **C-6（verify_all 强制）**：因前端有改动，Developer 完成前必须重建 `web/dist` → 重新 `go build` → 跑 `scripts/verify_all` PASS。

可移交 Developer。按 02 §"实现顺序建议" 派发：dev-db → dev-backend → dev-frontend。
