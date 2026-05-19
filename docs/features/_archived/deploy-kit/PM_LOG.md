# PM_LOG — T-008 deploy-kit

## 任务背景

用户提问：当前项目是否可以直接写部署文档让用户跟着部署？若不能，由 PM 自主决策实现，原则：用户体验好 / 符合软件工程标准 / 长期易使用易维护。

## PM 评估（2026-05-19）

仅文档不足够。当前缺口：
1. 无预打包发布物（用户必须装 Go 1.22+ + Node 18+）
2. README 把部署/开发/更新/dev-mode 混在一起，无专用部署文档
3. 无 systemd / Windows Service 一键安装脚本
4. 二进制无 `--version` / `--help` flag

## 决策

按 /harness 7-stage 跑 T-008 deploy-kit。交付清单见 04_DEVELOPMENT.md。

## 阶段记录

- 2026-05-19 stage:req — 派发 Requirement Analyst
- 2026-05-19 stage:design — 01 READY，派发 Solution Architect
- 2026-05-19 stage:gate — 02 设计完成，派发 Gate Reviewer
- 2026-05-19 stage:dev — 03 verdict APPROVED FOR DEVELOPMENT（条件式），派发 Developer (dev-backend)
- 2026-05-19 stage:review — 04 完成 verify_all 18 PASS，派发 Code Reviewer
- 2026-05-19 stage:dev (rework) — 05 verdict CHANGES REQUIRED（1 MAJOR systemd 含空格路径 + 5 MINOR 改进），回路 Developer 快速修补
- 2026-05-19 stage:test — Developer rework 完成（MAJOR + 5 MINOR 全修），verify_all 18 PASS。派发 QA Tester；QA agent 因 API 额度耗尽中止，PM 接手亲跑 19 条本机 AC + 6 条对抗用例 + verify_all，6 条 Adversarial 全 PASS，写 06_TEST_REPORT.md，verdict READY FOR DELIVERY
- 2026-05-19 stage:delivery — 写 07_DELIVERY.md + archive-task + commit

## PM partition-override 授权（2026-05-19）

针对 Gate Review MAJOR-1：`.harness/agents/dev-backend.md` 的 owned paths 列举式声明未显式覆盖本任务新增的脚本与文档路径。PM 在此**显式授权**本任务 dev-backend 触达以下文件，不视为越界：

- `scripts/package.sh`、`scripts/package.ps1`（新建）
- `scripts/install-service.sh`、`scripts/install-service.ps1`（新建）
- `scripts/uninstall-service.sh`、`scripts/uninstall-service.ps1`（新建）
- `cmd/frp-easy/main.go`（编辑：新增 flag 解析）
- `docs/DEPLOYMENT.md`（新建）
- `README.md`（编辑：重排）
- `docs/dev-map.md`（编辑：追加索引）

此授权仅适用于 T-008 deploy-kit；不修改 agent.md 默认 owned paths 以保留对其它任务的约束。

