---
task: hardening-pass-audit
task_id: T-007
stage: 05_code_review
mode: full
date: 2026-05-19
author: code-reviewer
status: APPROVED
---

# 05 Code Review — T-007 hardening-pass-audit

## 复核摘要

独立审查 T-007 的 9 项加固改动 + OpenAPI 同步。审查方式：逐条 AC 对照源代码、grep 验证 C-1 ~ C-6 条件、读测试断言而非仅看 04 文档自述。

- 9 个 AC 都有对应实现，且实现与 02 设计一致，无 design drift。
- 6 条 Conditions（C-1 ~ C-6）逐条核实，**全部满足**。
- procmgr.Start 的 IIFE + 单 defer-unlock 重构正确，emit 顺序 / waitUntilStable 时序与设计契约一致。
- storage 层 ErrDuplicateName 识别精确（含错误文本回归保护测试）。
- 中间件 SecurityHeaders 挂载位置正确（router.go:68 在 health 注册前），且测试覆盖 health / 公开 API / NotFound / 401 / 根路径 / 直测六个场景。
- 前端 ProxyForm watch 不使用 flush:'sync'（grep 验证），编辑模式 customDomains/remotePort 不丢失测试已加入。

仅发现 **3 个 MINOR + 2 个 NIT**，全部非阻塞。无 CRITICAL / MAJOR。

## AC 实现复核

### AC-1（frpconf AtomicWrite 0o600）— PASS
- 实现：`internal/frpconf/render.go:278`（tmp Chmod）、`:303`（path Chmod）。
- 测试：`render_test.go:213` `TestAtomicWritePerm0600` 真实 stat 0o600；`:238` 验证已存在 0o644 文件被收紧。
- Windows 跳过断言正确；chmod 失败时 fail-closed 返错。

### AC-2（日志 0o600）— PASS
- ui.log：`cmd/frp-easy/main.go:98` mode 0o600 + L100 Chmod 兜底。
- frpc/frps log：`internal/procmgr/manager.go:436` mode 0o600 + L438 Chmod 兜底。
- C-5 注释 manager.go:424-432 写明 FRP 子进程权限归责边界。

### AC-3（SecurityHeaders）— PASS
- middleware.go:193-203 标准 chi 中间件签名；Set 在 next.ServeHTTP 之前。
- router.go:68 `r.Use(SecurityHeaders())` 在 health 注册（L72）之前，满足 C-4。
- 测试 6 场景全覆盖。

### AC-4（日志 2 MiB 上限）— PASS
- `internal/logtail/tail.go:92` `const MaxReadBytes = 2 << 20` 包级导出。
- tail.go:122-124 应用点；`TestReadFromCapsAt2MiB`（tail_test.go:188）真构造 5 MiB 文件三次轮询断言。
- 卫断言 `if MaxReadBytes != 2*1024*1024` 防误改。

### AC-5（procmgr.Start defer-unlock）— PASS
- manager.go:201-269 IIFE + 单 defer Unlock（L203）。
- grep manager.go Start 函数体（L182-291）`m.mu.Unlock()` 仅 L203 一处且 defer 形式，满足 AC-5.1。
- 逐分支映射核对：StateStarting/Running、StateStopping、binPath/cfgPath/mkdir 错误、cmd.Start 失败、成功 5 条路径与设计一致。emit 在解锁后。
- 现有测试 `-count=3` PASS（race 因本机无 cgo 跳过；MINOR-1 建议 CI 补）。

### AC-6（ErrDuplicateName + 409）— PASS
- sentinel：`store.go:48-53`，注释指明 schema 来源（sqlmigrations/0001_init.up.sql L32 `name TEXT NOT NULL UNIQUE`，已独立读源确认）。
- `isDuplicateNameError`：proxies.go:329-336 双关键字匹配；INSERT 路径 L122-127、UPDATE 路径 L171-175 sentinel 优先。
- handler：handlers_proxies.go:249-252 errors.Is → 409 + CodeConflict + field=name。
- 测试：proxies_test.go:12/51/91/125（含 isDuplicateNameError 5 边界 case 表驱动测试）；handlers_proxies_test.go:11/56/96。
- OpenAPI：openapi.yaml:663-668 POST `/proxies` 新增 409；:722-727 PUT `/proxies/{id}` 描述合并。

### AC-7（Dashboard 错误显示）— PASS
- Dashboard.vue:50（frpc）与 :120（frps）两处含 `white-space: pre-wrap; word-break: break-word`。

### AC-8（Proxies 删除清理 + 空状态）— PASS
- Proxies.vue:118-132 try 成功路径清 `firewallPorts/firewallProto`；catch 不清。
- L15-18 n-data-table #empty slot + NEmpty + Q-4 文案。

### AC-9（ProxyForm 类型切换）— PASS
- useProxyForm.ts:32-39 按目标 type 互斥重置；L41-47 watch 默认 flush（grep 全 web/src 无 `flush:'sync'`）。
- ProxyForm.spec.ts:114 / :140 编辑模式 customDomains/remotePort 不丢失测试 PASS（C-1 满足）。

## Conditions 复核

| 条件 | 验证证据 | 状态 |
|---|---|---|
| C-1 ProxyForm watch 无 flush:'sync' + 编辑测试 | grep 实证 + spec.ts:114,140 PASS | ✅ |
| C-2 sqlite UNIQUE 文本与 migration 约束已验证 | sqlmigrations/0001_init.up.sql:32 + 端到端测试用真实驱动文本 | ✅ |
| C-3 procmgr.Start 显式 Unlock=0 + 现有测试不改通过 | grep 实证 + manager_test.go -count=3 PASS | ✅ |
| C-4 SecurityHeaders Use 在 health Get 之前 | router.go:68 < L72 + 端到端测试 | ✅ |
| C-5 supervise chmod 处归责注释 | manager.go:424-432 | ✅ |
| C-6 重建 dist → go build → verify_all PASS | 04 文档 verify_all.sh full mode 18/18 PASS | ✅ |

## 设计契约一致性

无 design drift。02 设计的每个决议（chmod 双重、SecurityHeaders 顶层 r.Use、MaxReadBytes 包级常量、IIFE 7 分支映射、sentinel 放 store.go、OpenAPI Q-3 同步、Q-4 空状态文案、watch 默认 flush）均落地。

## 测试质量

- Go 测试：所有新测试有真实断言；isDuplicateNameError 5 边界 case；middleware_test.go 包含直接测试 + 通过 router 测试组合健壮。
- Vitest 测试：15 个 ProxyForm 测试均断言具体行为，含 watch oldType 短路逻辑测试、toProxyInput tcp 模式不上送残留 customDomains 的防御性回归。

## 发现的问题

### MINOR-1（procmgr / AC-5）：本任务环境 `-race` 未跑
本地 Windows 无 cgo，已用 `-count=3` 替代。建议 CI 补跑 `-race`。**非阻塞**。

### MINOR-2（handlers_proxies.go）：422 兜底分支文案模糊
handlers_proxies.go:261 文案"可能 name 重复或 (type,remotePort) 冲突"在新逻辑下"name 重复"永远走 409，文案有误导。建议下次加固任务改为"端口已被占用：同 type 下 remotePort 不能重复"。**非阻塞**。

### MINOR-3（前端 ProxyForm watch 兜底语义）：handleTypeChange(tcp) 不清 remotePort
设计选择保留 remotePort（"用户上次填的 HTTP 域名不残留"满足）。spec.ts:53-62 把此设为硬契约。建议 useProxyForm.ts 注释中写明这是有意选择。**非阻塞**。

### NIT-1（OpenAPI）：PUT /proxies/{id} 409 description 合并后未提供两种 example
NIT-4 建议用 `oneOf` examples 给两种 409 各一示例 body。未来 docs 任务可加。**非阻塞**。

### NIT-2（INSIGHT 候选）：web/src/**/*.js 残留导致 vitest 优先 .js 的开发陷阱
Developer 上报开发环境陷阱（已通过 `find ... -delete` 清理）。建议作为 INSIGHT 候选写入 `insight-index.md`。**非阻塞**。

## 决议

**VERDICT: APPROVED**

- 9 个 AC 全部实现到位，无 design drift。
- 6 条 Conditions 全部满足（含独立 grep / 读源码 / 读测试断言验证）。
- 测试质量高，含驱动错误文本回归保护、reentrancy 防护、边界条件三类 defensive 测试。
- 仅 3 MINOR + 2 NIT，全部非阻塞。
- verify_all.sh full mode 18/18 PASS。

可移交 QA Tester。
