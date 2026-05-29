# BATCH_REPORT — project-optimization-2026-05

> 2026-05-30 · 用户目标"优化项目"（原则：用户体验好 / 符合软件工程标准 / 长期易使用易维护）· 范围：全面深挖 · 全程 AI 自主决策 + 执行 + commit + push。

## 结论

**11 个任务全部 DELIVERED。基线从启动时的 FAIL（2 个失败，红线违规）恢复并加固到 PASS 32 / WARN 0 / FAIL 0（含 e2e）。** 测试从启动时实际红的 451 基线 → **734（308 Go + 426 前端）**。每个任务由 orchestrator **真跑 `scripts/verify_all`** 作为硬闸门（杜绝上一批次"role-play QA 漏跑导致带红交付"的失败模式）。

## 启动时发现的关键问题（红线违规）

verify_all 实测 **PASS 29 / FAIL 2**——上一批次 T-038~T-042 带红测试树 + 未守住的闸门交付：
1. **B.3 单元测试 FAIL**：前端 vitest 39 个失败（38 = VTU `vm.__testing` 脆弱性，1 = `useServerRuntime` 错误消息漂移）。
2. **E.6 FAIL**：T-038/039/040 三个归档报告缺 `## Adversarial tests` 段（实为标题带数字前缀）。
3. **闸门本身有洞**：`.ps1` 的 B.3 不查退出码（vitest 失败也报 PASS）；B.4「测试数≥基线」双实现都是空操作；baseline.json 停在 T-036。三洞叠加让红树通过 `pwsh verify_all.ps1` 假报 PASS。

## 任务结果

| ID | Slug | 结果 | 关键改动 | verify_all |
|---|---|---|---|---|
| T-043 | frontend-test-suite-repair | DELIVERED | 修 39 前端失败（新增 getExposed/apiError test-utils + 7 spec 健壮化 + extractErrorMessage 契约对齐） | PASS（B.3 红→绿） |
| T-044 | verify-gate-hardening | DELIVERED | .ps1 B.3 真查退出码 + B.4 双实现真计数（go test -list + vitest）+ baseline 刷新 + E.6 三报告标题归一 | PASS 31/0/0（基线恢复） |
| T-052 | e2e-decouple-port | DELIVERED | e2e 改独立端口 17800 + webServer.env 注入，根治 C.1 假性失败 | PASS 32/0/0（首次含 e2e 全绿） |
| T-045 | backend-deadcode-cleanup | DELIVERED | 删 procmgr 死发布订阅 + 死函数 + var _ hack 净~90 行 | PASS 32/0/0 |
| T-046 | session-purge-and-requestid | DELIVERED | 过期 session 定时清理 loop + RequestID crypto/rand（+3 测试） | PASS 32/0/0 |
| T-050 | backend-test-coverage | DELIVERED | validate.go/procmgr 终态断言/autoRestore/svcprobe/procmgr 子进程生命周期（+21 测试） | PASS 32/0/0 |
| T-053 | autorestore-canceled-persist-fix | DELIVERED | （T-050 测试覆盖中发现的 bug）canceled 分支改 detached ctx 持久化 | PASS 32/0/0 |
| T-047 | frontend-honest-states | DELIVERED | Server/Client 加载+错误三态 + Proxies 失败/空区分 + Dashboard 开关不静默 + 校验（+30 测试） | PASS 32/0/0 |
| T-048 | frontend-consistency-cleanup | DELIVERED | 删重复 formatBytes + formatTime 本地化统一 + 可读语义色 + router.push + 响应式 + 文案统一（+15 测试） | PASS 32/0/0 |
| T-051 | frontend-test-coverage | DELIVERED | proxies/wizard store + useProxyForm + statusUtils/composable + api/client.ts（+84 测试） | PASS 32/0/0 |
| T-049 | docs-contract-drift-fix | DELIVERED | openapi 补 service-status（30 path）+ dev-map 树补 svcprobe/utils/test-utils + 修计数 + HTML 时效声明 | PASS 32/0/0 |

> 各任务阶段文档：`docs/features/<slug>/07_DELIVERY.md`（pending archive）。

## 聚合统计

- 任务：**11 DELIVERED / 0 failed / 0 blocked / 0 skipped**（10 原计划 + 1 执行中发现 bug 补入的 T-053）。
- 测试净增：Go **265→308**（基线刷新后口径 287→308，+21）；前端 **186→426**（+240，其中 39 个从红修绿、201 个新增覆盖）。
- 后端净删死代码 ~90 行；前端消除 3 处重复工具实现。
- 修复 2 个真实 bug：红基线（红线违规）、retryRestoreLoop canceled-persist。
- 最终 `bash scripts/verify_all.sh`：**PASS 32 / WARN 0 / FAIL 0**（含 C.1 e2e）。
- 停批信号：无触发（全程绿色推进）。

## 用户需关注 / 后续建议（非阻塞）

1. **`-race` 未跑**：本机无 C 编译器（cgo 不可用），procmgr 并发测试未跑 `-race`；建议在有 gcc/clang 的环境补跑 `CGO_ENABLED=1 go test -race ./internal/procmgr/...`。
2. **`.ps1` verify_all 未在本会话运行**：用户的 PowerShell deny 规则拦截了本会话直接调 pwsh；`.ps1` 改动与 `.sh` 严格对称并逐行复核，但建议用户本机跑一次 `pwsh scripts/verify_all.ps1` 确认。
3. **两处遗留小不一致（已记录未改，建议未来对齐）**：`useServiceStatus` 未用项目约定的 `extractErrorMessage`（端点不会 5xx，影响小）；`proxies` store CRUD 不更新 error ref（T-047 有意分工）。
4. **建议未来 T 候选**：verify_all 加一道"router.go 路由集 == openapi paths 集"静态闸门，物理防止路由漏登 openapi。
5. **归档**：本批次 11 个任务的阶段文档仍 pending archive（与既有 T-040/041/042 同状态）；可在合适时机跑 `scripts/archive-task` 收割 insight 到 `.harness/insight-index.md`。

## 推送

批次结束 commit BATCH_REPORT + insight-index 后 `git push origin main`（用户授权；触发 release.yml 滚动发布刷新）。
