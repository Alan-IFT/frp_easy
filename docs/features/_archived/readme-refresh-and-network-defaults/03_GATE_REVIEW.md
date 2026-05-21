# 03 Gate Review — T-011 readme-refresh-and-network-defaults

> Harness 流水线 stage 3 产出。Gate Reviewer 独立核验，不信任上游。
> 上游：`01_REQUIREMENT_ANALYSIS.md`（verdict=`READY FOR DESIGN`）、`02_SOLUTION_DESIGN.md`（verdict=`READY`）。
> 任务模式：full（7-stage）。verdict `APPROVED FOR DEVELOPMENT` 语义等价于 full 模式的 `APPROVED WITH CONDITIONS`。

## 1. 八维审计清单

| # | 维度 | 结论 | 一句话理由 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | 24 条 AC 全部含明确验证手段（grep / 单测 / 编译 / 浏览器），FR-1~FR-6 + FR-3b 边界清晰，3 个开放问题已 PM 裁决回填。 |
| 2 | 设计完整性 | PASS | 02 §12 把 24 条 AC 逐条映射到精确文件+行号改动清单，无 AC 悬空。 |
| 3 | 复用正确性 | PASS | §4 复用审计 9 项经读码核实属实（`Default()`/`Load()`/`Validate()`/浏览器改写/端口占用逻辑均存在可复用），"无新模块"正确。 |
| 4 | 风险覆盖 | WARN | R-1~R-8 覆盖误改代理端口、NF-2、安全提示触发条件等真实风险；但 §2.4 对 e2e server 脚本现状的事实描述有误（见 F-1）。 |
| 5 | 迁移安全 | PASS | 无 DB 迁移；NF-2 兼容性靠"仅空字段回填"实现，`Load()` 条件判断不动只改回填目标值，老用户显式值不受影响；`git revert` 可回滚。 |
| 6 | 边界处理 | PASS | 边界表覆盖首启/旧配置显式值/缺字段/端口占用/回环绑定/无网络打开 HTML/归档只读 7 类；正向枚举 `0.0.0.0`/`::` 正确处理 IPv6 与用户自填具体 IP。 |
| 7 | 测试可行性 | PASS | 24 条 AC 均可独立验证；新增 `TestLoad_ExplicitLoopbackNotOverwritten` 覆盖 AC-20 且满足红线 3；AC-18/19 标注"运行/人工"合理。 |
| 8 | 范围边界 | PASS | FR-4.4 + 01 §9 + 02 §13 三处显式枚举 FRP 代理端口禁改清单（5 文件），AC-15 专项 grep 兜底。 |

## 2. Findings

### F-1（WARN）— e2e server 脚本现状描述有误
设计 §2.4 称 e2e server 脚本"未显式写 `UIBindAddr`"。实测不符：`scripts/start-e2e-server.sh:52`、`start-e2e-server.ps1:58` **本就已写 `UIBindAddr = "127.0.0.1"`**。Developer 只改 `UIPort` 行数字即可，**不得**照设计字面"补 UIBindAddr 行"以免重复键。

### F-2（WARN）— verify_all 项数与 A.3 风险
`baseline.json` 的 notes 写"18/18"是 B.5 哨兵加入前的过时文案；`verification_history.log` 近期实际为 `pass:19`，AC-8/AC-21 写 19 准确。隐患：A.3（TODO/FIXME 预算）是 WARN-only，开发期若引入 TODO/FIXME 致总数超 20，pass_count 掉到 18，AC-8（==19）/AC-21（≥19）失败。本任务文档改动不应引入 TODO。

### F-3（WARN）— baseline.json 数字无自动校验
新增 AC-20 测试后须把 baseline.json `go_tests` 166→167、`test_count` 223→224。verify_all B.4 仅查 `test_count != 0`，不真正比对数；G.2 `go test ./...` 实跑兜底。QA 阶段须人工核对 baseline.json 数字与实跑一致。

## 3. 已核实属实的正面依据
- 行号核对全部命中（config.go / main.go / config_test.go / browseropen_test.go / vite / playwright / openapi.yaml / project-status.html / architecture.html / DEPLOYMENT.md）。
- 安全后果处理到位：重构后安全提示三要素齐全（对外可达事实 / 引导尽快 setup 并明示 setup 前无密码保护 / 给出改回 127.0.0.1 操作），用"提示："非"WARN:"，正向枚举不误伤用户自填 IP。
- 端口误改护栏充分：FR-4.4 + §10.4 枚举 5 个禁改文件，AC-15 grep 兜底，要求逐文件定点编辑非全仓库 sed。
- 测试只升不降：新增 1 条测试，go_tests 166→167，符合红线 3。
- 下游不改上游：改动清单不含 `.harness/`、`.claude/`、`CLAUDE.md`、`.github/copilot-instructions.md`、`docs/features/_archived/**`。
- LICENSE 现状：仓库根无 LICENSE；`frp_linux/LICENSE`、`frp_win/LICENSE` 存在。FR-1.4/AC-4b 与现状一致。

## 4. Verdict

**APPROVED FOR DEVELOPMENT**（带 3 条开发期条件）

- **条件 1（F-1）**：e2e server 脚本的 `UIBindAddr = "127.0.0.1"` 行已存在，只改 `UIPort` 行数字，不得补 UIBindAddr 行。
- **条件 2（F-2）**：不得引入 TODO/FIXME 注释；完成后实跑双 shell verify_all，确认 pass_count=19，若为 18 先排查 A.3/D.1 降级。
- **条件 3（F-3）**：新增 AC-20 测试后同步更新 baseline.json（go_tests 166→167、test_count 223→224）；QA 须人工核对。
