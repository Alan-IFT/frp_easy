# PM_LOG — T-010 deploy-polish-and-ci

> PM Orchestrator 的实时决策日志。Stage 推进、agent 派发、回环、blocker 都记这里。

---

## 2026-05-19 T0 — 任务起源 + 调研

用户授权"全自主决策 + 全自主执行"。读 `.harness/insight-index.md`（25 行）+ `docs/tasks.md` + 当前 verify_all 历史。Baseline 18/18 PASS（2026-05-19 22:36）。

调研覆盖：DEPLOYMENT.md / install-service.{sh,ps1} / main.go / appconf / dev-map / project-status.html。grep TODO/FIXME 无业务代码残留。

**识别遗留问题（按 UX 影响排序）**：

| 优先级 | 问题 | 证据 | 影响 |
|---|---|---|---|
| **P0** | `<ORG>` 占位符未解析（git remote 实为 `Alan-IFT`） | DEPLOYMENT.md 第 17/41/151 行 + install-service.sh 第 115 行 | 用户照文档操作会遇到字面量 `<ORG>` 导致 URL 404 / systemd Documentation 字段含尖括号 |
| **P1** | 无日志轮转 | `cmd/frp-easy/main.go` newLogger 仅 `O_APPEND`，无 size/age 限制；grep "lumberjack\|Rotate\|MaxSize" 无命中 | 长跑服务 ui.log/frpc.log/frps.log 无限增长，最终爆盘 |
| **P1** | 无浏览器自动打开 | `cmd/frp-easy/main.go` 仅 stderr 打 URL；grep "webbrowser\|xdg-open" 无命中 | Windows 普通用户双击 .exe 后需手动复制 URL 到浏览器 |
| **P2** | 无 CI/CD（无 .github/workflows/） | `ls .github/workflows/` 不存在 | DEPLOYMENT.md A.1 引用的 GitHub Releases 是空的；用户照"路径 A 下载发布产物"会 404 |

附带的"傻瓜部署服务端 vs 客户端"诊断：当前 frp-easy 是单二进制 + 部署向导（T-002）覆盖角色选择，部署路径无需分叉，结构已合理。FirewallHint UI（T-002）已覆盖端口提示。**结论**：服务端/客户端部署本身不需要新做，但需要 P0/P1/P2 改造让首次部署体验闭环。

**端口管理**：appconf/config.go 已写死端口表注释（UI 8080 / frpc admin 7400 / frps dashboard 7500 / frps bind 7000 / proxy.remotePort 用户填），main.go 已有 isAddrInUse 友好提示 + exit code 2。无新增需要。

## 决策

按"UX 好 + 工程标准 + 长期易维护"原则，4 项全做。模式选择：**PM-driven 全 7-stage**（仿 T-008 rework / T-009 节奏）：
- 01/02/03 PM 一次性写完（清理性 + 机械性，无设计争议）
- 04 PM 直接 dev
- 05 派 code-reviewer 子 agent
- 06 派 qa-tester 子 agent（保证 Adversarial 段独立性）
- 07 PM 写交付

---

## 2026-05-19 T1 — 进入 dev 阶段

完成 01/02/03。开始 04_DEVELOPMENT。
