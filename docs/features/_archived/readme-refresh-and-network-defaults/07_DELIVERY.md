# 07 交付 — T-011 readme-refresh-and-network-defaults

> Harness 流水线 stage 7 产出。PM Orchestrator 撰写。

## 摘要

T-011 把 frp_easy 的 README 重写为标准开源项目文档、刷新两个项目状况 HTML 到 T-011 实际状态、全量审计 docs/ 过时内容，并按用户授权调整了两项网络默认值：

1. **UI 默认端口 8080 → 7800** —— 8080 是最高冲突率端口之一；7800 与 frp 自身端口族（7000/7400/7500）同段、便于记忆、不与四者重叠、属不常用端口。
2. **UI 默认绑定地址 127.0.0.1 → 0.0.0.0** —— frp_easy 本质是远程内网穿透管理工具，运维场景天然需要从其他设备访问 Web UI；项目已内置 argon2id + session + CSRF + 限流，0.0.0.0 不等于无认证暴露。main.go 的 NF-S4 WARN 重构为面向新默认值的中性安全提示（三要素：对外可达事实 / 引导尽快完成 setup / 给出改回 127.0.0.1 的操作）。

## 流水线轨迹

| 阶段 | Agent | 结果 |
|---|---|---|
| 1 需求 | Requirement Analyst | READY FOR DESIGN（7 FR / 5 NF / 24 AC；3 个开放问题经 PM 裁决） |
| 2 设计 | Solution Architect | READY（单 Developer 分区，端口/绑定只改字面量不动逻辑） |
| 3 闸门 | Gate Reviewer | APPROVED FOR DEVELOPMENT（带 3 条开发期条件 F-1/F-2/F-3） |
| 4 开发 | Developer | 完成，无 DESIGN DRIFT；双 shell verify_all PASS 19 |
| 5 评审 | Code Reviewer | APPROVED（2 MINOR：architecture.html 路由表缺 6 条 → 已路由回 Developer 修复） |
| 6 测试 | QA Tester | PASS（24 AC 全部通过对抗性验证；独立实跑二进制 6 场景） |
| 7 交付 | PM | 本文档 |

## 改动文件清单

**网络默认值与代码（块 1+2）**
- `internal/appconf/config.go`、`internal/appconf/config_test.go`（新增 `TestLoad_ExplicitLoopbackNotOverwritten`）
- `cmd/frp-easy/main.go`（usageText、包注释、NF-S4 WARN 重构为 `exposureNotice()`）
- `internal/browseropen/browseropen_test.go`

**前端配置与脚本**
- `web/vite.config.ts`、`web/playwright.config.ts`
- `scripts/start.{sh,ps1}`、`scripts/start-e2e-server.{sh,ps1}`、`scripts/package.{sh,ps1}`、`scripts/baseline.json`
- `openapi.yaml`

**文档**
- `README.md`（全量重写为标准开源结构）
- `docs/project-status.html`、`docs/architecture.html`（深度刷新到 T-011；architecture.html API 路由表补齐至 28 行）
- `docs/DEPLOYMENT.md`、`docs/dev-map.md`（过时点审计同步，含 Go 1.22+ → 1.25 等）

## verify_all 结果

```
=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

PowerShell 与 Git Bash 双 shell 均 PASS 19。Go 测试 166→167（新增 AC-20 测试），test_count 223→224，baseline.json 已同步。

## 给用户的提示（需关注）

- **许可证待定**：仓库根目录无 `LICENSE` 文件。许可证选择属项目维护者的法律决策，本任务未代为选定。README 许可证章节如实写"待项目维护者确定"，并注明随附的 frp 二进制（`frp_linux/` `frp_win/`）属上游 fatedier/frp 的 Apache-2.0。建议维护者尽快确定并补 `LICENSE` 文件。
- **老用户配置不受影响**：已存在的 `frp_easy.toml` 若显式写了 `UIBindAddr`/`UIPort`，升级后保持原值不变（`Load()` 仅对空字段回填）。新默认值只对全新安装生效。
- **绑定 0.0.0.0 的安全前提**：首次启动到完成 setup 之间存在无密码窗口，0.0.0.0 下局域网设备可在此窗口访问。请在受信任网络首启，并尽快完成 setup。仅本机使用可把 `UIBindAddr` 改回 `127.0.0.1`。

## Insight

- verify_all E.6 要求已完成任务的 06_TEST_REPORT.md 含**精确英文标题** `## Adversarial tests`；即使项目输出语言规则为中文，该段标题也必须用英文（可在英文标题后括注中文）。QA 若写 `## 对抗性测试` 会导致 E.6 FAIL、pass_count 掉到 18。证据：本任务 stage 7 首次 verify_all。
