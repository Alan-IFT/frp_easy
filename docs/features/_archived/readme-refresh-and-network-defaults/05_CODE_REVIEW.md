# 05 代码评审 — T-011 readme-refresh-and-network-defaults

> Harness 流水线 stage 5 产出。Code Reviewer 独立审计，不信任上游自述。
> 改动尚未 commit，依据工作区实际文件状态评审。

## Verdict：APPROVED（0 BLOCKER / 0 MAJOR / 2 MINOR / 2 NIT）

实现忠实于需求与设计：24 条 AC 全部有可验证落点，Gate Review 3 条开发期条件 F-1/F-2/F-3 全部满足，无设计漂移，无红线违规，FRP 代理端口禁改清单（5 文件）完整保留，NF-2 兼容性由新增测试 `TestLoad_ExplicitLoopbackNotOverwritten` 真实覆盖。`baseline.json` go_tests=167 经实测全仓库 `func Test` 计数精确吻合。

## Findings

### BLOCKER / MAJOR
无。

### MINOR
- **M-1 `docs/architecture.html:545-573` API 路由表只列 21 条，缺 6 条**：`GET /health`、`GET /system/public-ip`、`POST /system/download-bin`、`GET /system/download-status/{kind}`、`GET /wizard/status`、`POST /wizard/complete`。`internal/httpapi/router.go:72,119-123` 证实这些路由真实存在。属 FR-3.1 文档过时审计的实质遗漏。
- **M-2 `docs/architecture.html` 文件内/文件间自相矛盾**：同文件 `downloader` 模块卡片（行 382）已写 `POST /api/v1/system/download-bin`，但路由表不含；`docs/project-status.html:380` 写"T-001: 22 条；T-002: +5 条"，与路由表 21 条冲突。属 NF-5 文档自洽缺陷。

→ PM 裁决：M-1/M-2 路由回 Developer 修复（用户明确要求过时文档更新到实际版本，且修复成本仅补 6 行 `<tr>`）。

### NIT
- N-1 `config_test.go:101` 测试夹具含 `127.0.0.1:7800`，验证 host:port 被拒，端口数字无关语义，保留正确。
- N-2 `cmd/frp-easy/main.go` `exposureNotice` 文案在 `UIBindAddr=::` 罕见路径下仍硬编码 `0.0.0.0:` 串，三要素不受损。可选微调。

## 24 条 AC 覆盖核查
24 条 AC 全部有实现落点，无 CRITICAL 级悬空。AC-21 双 shell verify_all PASS 19 待 QA 实跑兜底。

## Gate Review 3 条条件核查
- F-1：e2e 脚本仅改 UIPort，未补重复 UIBindAddr 行 ✅
- F-2：产品代码无新增 TODO/FIXME ✅
- F-3：baseline.json go_tests=167 / test_count=224，实测精确吻合 ✅

## 红线文件核查
`.harness/`、`.claude/`、`CLAUDE.md`、`.github/copilot-instructions.md`、`docs/features/_archived/**` 均未被改动。FRP 代理端口 8080（5 文件白名单）未被误改。无红线违规。

## 逻辑 / 性能 / 安全
- 逻辑：`Load()` NF-2 兼容路径正确；安全提示正向枚举触发，不误伤用户自填 IP。
- 性能：纯字面量 + 文档改动，无热路径影响。
- 安全：默认 0.0.0.0 未削弱任何认证机制（NF-3 满足）；启动期安全提示明示"setup 前无密码保护"并给出关闭对外访问的精确操作。
