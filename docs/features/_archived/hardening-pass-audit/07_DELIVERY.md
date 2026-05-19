---
task: hardening-pass-audit
task_id: T-007
stage: 07_delivery
mode: full
date: 2026-05-19
author: pm-orchestrator
status: DELIVERED
---

# 07 Delivery — T-007 hardening-pass-audit

## 摘要

由 PM 在用户授权（"用户体验好/工程标准/长期可维护"原则自主决策）下发起的综合加固任务。基于并行三方审计（安全/代码质量/UX）识别的优先项，本任务聚焦 9 项高价值低风险修复，跨后端 + 前端 + OpenAPI，未引入新依赖、无 schema 迁移、无新路由。

完成 9/9 AC、满足 6/6 Conditions、未发现 BLOCKER/CRITICAL/MAJOR 级问题。**verify_all PASS 18/18**（含 E2E 烟雾不退化）。

## 交付内容

### 后端安全加固
1. **AC-1** — `internal/frpconf/render.go` 临时配置文件与最终文件双重 `Chmod 0o600`（防同主机用户窃取 frps_token）
2. **AC-2** — `cmd/frp-easy/main.go` ui.log、`internal/procmgr/manager.go` frpc.log/frps.log 改为 OpenFile 0o600 + Chmod 兜底升级路径
3. **AC-3** — `internal/httpapi/middleware.go` 新增 `SecurityHeaders` 中间件（X-Content-Type-Options/X-Frame-Options/Referrer-Policy）；`router.go` 顶层挂载在 health 之前
4. **AC-4** — `internal/logtail/tail.go` `MaxReadBytes` 包级导出常量 2 MiB（原私有 1 MiB），防日志雪崩 DoS

### 后端质量
5. **AC-5** — `internal/procmgr/manager.go` `Start()` 重构为 IIFE + 单 `defer m.mu.Unlock()`，函数体内显式 Unlock 调用 0 次
6. **AC-6** — `internal/storage/store.go` 新增 `ErrDuplicateName` sentinel；`proxies.go` 双关键字识别 `UNIQUE constraint failed: proxies.name`；`handlers_proxies.go` 检测后返 409 + 中文友好消息；`openapi.yaml` 同步两处 409 描述

### 前端 UX
7. **AC-7** — `Dashboard.vue` 进程错误信息加 `word-break: break-word`，长 token / stacktrace 完整显示
8. **AC-8** — `Proxies.vue` 删除成功清 firewallPorts/firewallProto；n-data-table `#empty` slot 加空状态文案
9. **AC-9** — `useProxyForm.ts` `handleTypeChange` 按目标 type 互斥重置；新增 `watch` 兜底（默认 flush，非 sync）；`ProxyForm.spec.ts` 新增编辑模式 customDomains/remotePort 不丢失测试

### 测试增强
- 新增 Go 单元/集成测试约 27 个（AC-1/4/6 主测 + AC-3/5/6 中间件/handler/storage 路径）
- QA 阶段补充 24 个 Go 对抗测试（render/storage/httpapi/logtail/procmgr）+ 5 个 Vitest 对抗测试
- 测试 baseline 升级 164 → 213

## verify_all 输出

```
=== Summary ===
  PASS: 18
  WARN: 0
  FAIL: 0
  SKIP: 0
```

含 [G.2] go test ./... PASS、[B.3] Vitest PASS、[C.1] Playwright E2E 烟雾 PASS、[B.4] 测试数 ≥ baseline PASS、[E.6] Adversarial tests section 已校验 PASS。

## 流水线产出

- `01_REQUIREMENT_ANALYSIS.md` — 9 AC + 4 Open Questions
- `02_SOLUTION_DESIGN.md` — 逐 AC 实现路径 + 7 分支映射表（AC-5）+ Q-1..4 决议
- `03_GATE_REVIEW.md` — APPROVED WITH CONDITIONS（0 BLOCKER, 2 WARN, 5 NIT, 6 Conditions）
- `04_DEVELOPMENT.md` — 9 AC 全实现 + 6 Conditions 全满足
- `05_CODE_REVIEW.md` — APPROVED（0 CRITICAL/MAJOR, 3 MINOR, 2 NIT, 无 design drift）
- `06_TEST_REPORT.md` — PASS（含 Adversarial tests 段全 AC 覆盖）

## 不变量回归保护

- 测试数只升不降：164 → 213，AC-3/4/5/6/9 都有对抗测试，未来误改会立即被卡住
- E2E 烟雾不退化：Playwright 5 个测试用例（TC-01..05）全部 PASS，与 T-006 baseline 一致
- 现有 procmgr 测试集 `manager_test.go` 未被修改即通过新 Start 重构后的代码（C-3 不变量）

## Insight

以下为本任务捕获的项目特有非显然事实，建议 archive 后由 `scripts/archive-task` 收割到 `.harness/insight-index.md`：

- **2026-05-19** · vitest module resolution 在 .ts/.js 共存时优先加载 .js；historical `tsc` 残留的 .js/.d.ts 会让改 .ts 测试看似无效果且无报错。开发前清理 `find web/src -type f \( -name '*.js' -o -name '*.js.map' \) -delete` · evidence: hardening-pass-audit
- **2026-05-19** · modernc.org/sqlite 的 UNIQUE 约束错误文本格式为 `UNIQUE constraint failed: <table>.<column>`，区分大小写；用 strings.Contains 双关键字（"UNIQUE constraint failed" + "<table>.<column>"）能精确区分表内多个 UNIQUE 列的冲突 · evidence: hardening-pass-audit
- **2026-05-19** · Go AtomicWrite 双重 Chmod 模式（tmp + final）必须在 rename 前后两处都 chmod，仅 chmod tmp 时 rename 后 umask 可能让最终文件权限变宽 · evidence: hardening-pass-audit

## 后续建议（非本任务范围）

PM 在 PM_LOG.md 已列出延后项；以下源于本任务过程发现，建议未来独立任务考虑：

- MINOR-2：`handlers_proxies.go:261` 422 兜底文案在新 sentinel 路径下"可能 name 重复"已不可达，建议精简为"端口已被占用：同 type 下 remotePort 不能重复"
- MINOR-3：`useProxyForm.ts` `handleTypeChange(tcp)` 不清 remotePort 是有意选择（tcp/udp 共用 remotePort 语义），建议补注释
- CI 环境补跑 `go test -race ./internal/procmgr/...`（本地 Windows 无 cgo，开发用 `-count=3` 替代）
- 安全：自动 Secure cookie（绑定地址 ≠ 127.0.0.1 时启用）、frpc admin 凭据派生密钥替代明文 KV、下载器 SHA256 校验 —— 三项独立深度安全任务
- UX：Wizard 步骤进度反馈、日志虚拟滚动、Version int64→string API 契约调整 —— 三项独立 UX/契约任务

## 验收

本任务可移交 `scripts/archive-task --task hardening-pass-audit`。
