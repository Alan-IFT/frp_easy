# 任务看板 — frp_easy

> 由 **PM Orchestrator** 维护。每个任务开始时记录到这里，并随生命周期更新。
>
> 新任务开始前，应该查这个看板找相关历史工作。

## 进行中

| ID | Slug | 阶段 | 模式 | 文档目录 |
|---|---|---|---|---|

## 已完成

| ID | Slug | 结果 | 完成 | 文档目录 |
|---|---|---|---|---|
| T-051 | frontend-test-coverage | DELIVERED | 2026-05-30 | `docs/features/frontend-test-coverage/`（pending archive；batch project-optimization-2026-05；proxies/wizard store + useProxyForm + statusUtils/useLogLevelFilter/useServiceStatus + api/client.ts +84 测试；frontend_tests 342→426；verify_all PASS 32/0/0） |
| T-053 | autorestore-canceled-persist-fix | DELIVERED | 2026-05-30 | `docs/features/autorestore-canceled-persist-fix/`（pending archive；batch project-optimization-2026-05；修 retryRestoreLoop canceled 分支用已取消 ctx 持久化→canceled outcome 落不进 kv；改 detached ctx + 升级测试正向断言；verify_all PASS 32/0/0） |
| T-050 | backend-test-coverage | DELIVERED | 2026-05-30 | `docs/features/backend-test-coverage/`（pending archive；batch project-optimization-2026-05；validate.go/procmgr 终态断言/autoRestore/svcprobe/procmgr 子进程生命周期 +21 测试；go_tests 287→308；发现 retryRestoreLoop canceled-persist bug→T-053；verify_all PASS 32/0/0） |
| T-048 | frontend-consistency-cleanup | DELIVERED | 2026-05-30 | `docs/features/frontend-consistency-cleanup/`（pending archive；batch project-optimization-2026-05；删重复 formatBytes + formatTime 本地化统一 + 可读语义色 + router.push + 响应式 + extractErrorMessage + 进程文案统一；frontend_tests 327→342；verify_all PASS 32/0/0） |
| T-047 | frontend-honest-states | DELIVERED | 2026-05-30 | `docs/features/frontend-honest-states/`（pending archive；batch project-optimization-2026-05；Server/Client 加载+错误三态 + Proxies 失败/空区分 + Dashboard 开关不静默 + Server dashboard 校验；frontend_tests 297→327；verify_all PASS 32/0/0） |
| T-046 | session-purge-and-requestid | DELIVERED | 2026-05-30 | `docs/features/session-purge-and-requestid/`（pending archive；batch project-optimization-2026-05；过期 session 定时清理 loop + RequestID crypto/rand + 3 新测试；go_tests 284→287；verify_all PASS 32/0/0） |
| T-045 | backend-deadcode-cleanup | DELIVERED | 2026-05-30 | `docs/features/backend-deadcode-cleanup/`（pending archive；batch project-optimization-2026-05；删 procmgr 无人订阅发布订阅+死函数+var _ hack 净~90 行；go_tests 285→284；verify_all PASS 32/0/0） |
| T-052 | e2e-decouple-port | DELIVERED | 2026-05-30 | `docs/features/e2e-decouple-port/`（pending archive；batch project-optimization-2026-05；e2e 改独立端口 17800 + webServer.env 注入，根治 C.1 假性失败；全量 verify_all 首次本机全绿 PASS 32/0/0） |
| T-044 | verify-gate-hardening | DELIVERED | 2026-05-30 | `docs/features/verify-gate-hardening/`（pending archive；batch project-optimization-2026-05；.ps1 B.3 真查退出码 + B.4 双实现真计数 + baseline 刷新 285/297/582 + E.6 三报告标题归一化；基线回 PASS 31/0/0） |
| T-043 | frontend-test-suite-repair | DELIVERED | 2026-05-30 | `docs/features/frontend-test-suite-repair/`（pending archive；batch project-optimization-2026-05；修 39 个前端测试失败恢复 B.3 绿，新增 getExposed/apiError test-utils） |
| T-042 | proxy-runtime-status-merge | DELIVERED | 2026-05-28 | `docs/features/proxy-runtime-status-merge/`（pending archive） |
| T-041 | server-monitor-page-ui | DELIVERED | 2026-05-28 | `docs/features/server-monitor-page-ui/`（pending archive） |
| T-040 | frps-allow-ports-policy | DELIVERED | 2026-05-27 | `docs/features/frps-allow-ports-policy/`（pending archive） |
| T-039 | frpsadmin-server-runtime-api | DELIVERED | 2026-05-27 | `docs/features/_archived/frpsadmin-server-runtime-api/` |
| T-038 | boot-autostart-hardening | DELIVERED | 2026-05-25 | `docs/features/_archived/boot-autostart-hardening/` |
| T-036 | log-ui-ux-polish | DELIVERED | 2026-05-24 | `docs/features/_archived/log-ui-ux-polish/` |
| T-037 | proxy-rules-simplify-and-port-fix | DELIVERED | 2026-05-24 | `docs/features/_archived/proxy-rules-simplify-and-port-fix/` |
| T-035 | install-sh-role-cli-arg-passthrough | DELIVERED | 2026-05-24 | `docs/features/_archived/install-sh-role-cli-arg-passthrough/` |
| T-033 | e2e-setup-spec-flake-fix | DELIVERED | 2026-05-24 | `docs/features/_archived/e2e-setup-spec-flake-fix/` |
| T-034 | reviewer-write-tool-dispatch-verify | DELIVERED | 2026-05-24 | `docs/features/_archived/reviewer-write-tool-dispatch-verify/` |
| T-032 | proxy-form-vmodel-oom-fix | DELIVERED | 2026-05-24 | `docs/features/_archived/proxy-form-vmodel-oom-fix/` |
| T-031 | install-ps1-host-close-on-completion | DELIVERED | 2026-05-24 | `docs/features/_archived/install-ps1-host-close-on-completion/` |
| T-030 | reviewer-agent-write-tool-frontmatter | DELIVERED | 2026-05-24 | trivial 直接修复（.harness/agents/gate-reviewer.md + code-reviewer.md frontmatter `tools` 字段 + harness-sync 同步 .claude/agents/，无阶段文档）：insight L41/L48 + T-027 第 3 次复现"reviewer 不落盘"陷阱（200+ 行 Markdown 塞消息体让 PM 代写），根因是 frontmatter tools 仅 Read/Glob/Grep；加 Write 工具与 developer/qa-tester 对齐 |
| T-029 | readme-ps51-zhcn-disk-form-warning | DELIVERED | 2026-05-24 | trivial 直接修复（README.md +1 段，无阶段文档）：T-026 删 install.ps1 BOM 后 PS5.1+zh-CN 磁盘形态会显示中文乱码（OOS-9 接受），README 一键安装 Windows 段加引导块明示"iex 形态首选；磁盘形态请用 PS7 跑 pwsh 或保持 iex" |
| T-028 | archive-task-insight-regex-tolerance | DELIVERED | 2026-05-24 | trivial 直接修复（archive-task.ps1 ~6 行，无阶段文档）：07 §N Insight 数字编号前缀让 regex 不命中，T-019/T-021/T-024/T-026 4 次复现；改 regex `^##\s+(?:[^\s\n]+\s+)?Insights?\s*$` 容忍可选前缀 token；N=0 时显式 Write-Warning 不静默 |
| T-026 | install-ps1-iex-bom-and-host-exit-fix | DELIVERED | 2026-05-24 | `docs/features/_archived/install-ps1-iex-bom-and-host-exit-fix/` |
| T-027 | download-cancel-and-upload-decouple | DELIVERED | 2026-05-24 | `docs/features/_archived/download-cancel-and-upload-decouple/` |
| T-025 | download-bin-timeout-fix | DELIVERED | 2026-05-23 | `docs/features/_archived/download-bin-timeout-fix/` |
| T-024 | install-ps1-iex-cmdletbinding-fix | DELIVERED | 2026-05-23 | trivial 直接修复（install.ps1 删 `[CmdletBinding()]` 一行 + 注释，无阶段文档）：iex inline 不允许该 attribute，会触发 "Unexpected attribute 'CmdletBinding'"；`param([switch]$Help)` 保留兼容磁盘形态 |
| T-023 | upload-bin-content-type-fix | DELIVERED | 2026-05-23 | trivial 直接修复（system.ts + spec.ts，无阶段文档）：apiClient 实例 default `Content-Type: application/json` 污染 FormData 请求让 axios 不再自动补 multipart boundary，显式 `headers: { 'Content-Type': undefined }` 抵消 |
| T-022 | service-mode-stderr-bridge | DELIVERED | 2026-05-23 | trivial 直接修复（main.go 单文件 ~10 行 + 2 个单测，无阶段文档）：exposureNotice 改走 logger，让服务模式 ui.log 也能拿到提示 |
| T-021 | encoding-ps51-bom | DELIVERED | 2026-05-23 | `docs/features/_archived/encoding-ps51-bom/` |
| T-019 | windows-service-scm-1053-fix | DELIVERED | 2026-05-23 | `docs/features/_archived/windows-service-scm-1053-fix/` |
| T-020 | claude-settings-context7-fix | DELIVERED | 2026-05-23 | `docs/features/_archived/claude-settings-context7-fix/` |
| T-018 | upload-bin-multiport-ip-probe | DELIVERED | 2026-05-23 | `docs/features/_archived/upload-bin-multiport-ip-probe/` |
| T-017 | install-role-and-public-ip | DELIVERED | 2026-05-23 | `docs/features/_archived/install-role-and-public-ip/` |
| T-016 | install-progress-and-systemd-unit-fix | DELIVERED | 2026-05-23 | `docs/features/_archived/install-progress-and-systemd-unit-fix/` |
| T-015 | fix-dirty-release-version | DELIVERED | 2026-05-22 | trivial 直接修复（单行级，无阶段文档）：`npm install --frozen-lockfile`→`npm ci` |
| T-014 | frp-binary-auto-download | DELIVERED | 2026-05-22 | `docs/features/_archived/frp-binary-auto-download/` |
| T-013 | rolling-release-install | DELIVERED | 2026-05-22 | `docs/features/_archived/rolling-release-install/` |
| T-012 | one-click-install-and-mit-license | DELIVERED | 2026-05-22 | `docs/features/_archived/one-click-install-and-mit-license/` |
| T-011 | readme-refresh-and-network-defaults | DELIVERED | 2026-05-21 | `docs/features/_archived/readme-refresh-and-network-defaults/` |
| T-010 | deploy-polish-and-ci | DELIVERED | 2026-05-19 | `docs/features/_archived/deploy-polish-and-ci/` |
| T-009 | polish-pass | DELIVERED | 2026-05-19 | `docs/features/_archived/polish-pass/` |
| T-008 | deploy-kit | DELIVERED | 2026-05-19 | `docs/features/_archived/deploy-kit/` |
| T-007 | hardening-pass-audit | DELIVERED | 2026-05-19 | `docs/features/_archived/hardening-pass-audit/` |
| T-006 | e2e-smoke-tests | DELIVERED | 2026-05-17 | `docs/features/_archived/e2e-smoke-tests/` |
| T-005 | docs-and-api-schema | DELIVERED | 2026-05-16 | `docs/features/_archived/docs-and-api-schema/` |
| T-004 | tech-debt-cleanup | DELIVERED | 2026-05-16 | `docs/features/_archived/tech-debt-cleanup/` |
| T-003 | readme-and-health-report | DELIVERED | 2026-05-16 | `docs/features/_archived/readme-and-health-report/` |
| T-002 | zero-config-quickstart | DELIVERED | 2026-05-16 | `docs/features/_archived/zero-config-quickstart/` |
| T-001 | web-ui-mvp | DELIVERED | 2026-05-16 | `docs/features/_archived/web-ui-mvp/` |

## 约定

- **ID** 顺序编号：`T-001`、`T-002`、...
- **Slug** 小写连字符，≤40 字符（例：`csv-export-orders`）。
- **阶段** 之一：`req`（需求）、`design`（方案）、`gate`（闸门）、`dev`（开发）、`review`（评审）、`test`（测试）、`delivery`（交付）、`blocked`（阻塞）、`done`（完成）。
- **文档目录** 是 `docs/features/<slug>/` 下的相对路径。

## 任务怎么关联

新任务开始时，Requirement Analyst 会扫描这个看板找相关历史：

- 同一模块 → 先读之前的 `02_SOLUTION_DESIGN.md`。
- 同一 feature → 在之前的方案基础上扩展，**不要重新设计**。
- 决策冲突 → 标记给用户。
