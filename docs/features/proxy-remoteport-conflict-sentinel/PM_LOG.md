# PM_LOG — T-059 proxy-remoteport-conflict-sentinel

> PM Orchestrator 路由决策日志。mode: full（7-stage）。中文。

## 任务

- **slug**: `proxy-remoteport-conflict-sentinel`
- **mode**: full
- **目标**: 把 `(type, remote_port)` 唯一冲突在 storage 层 sentinel 化（新增 `ErrDuplicateRemotePort`），消除 handler 层 `mapProxyWriteError` 对 SQL 驱动错误文本的脆弱 `strings.Contains` 匹配，与既有 `ErrDuplicateName` 范式对齐。偿还 T-055 backlog。

## 关联历史

- **T-055 backend-api-hygiene**（直接前置）：改过 `mapProxyWriteError` 为 `*handlers` 方法，引入 `writeInternalError`（500 兜底固定文案 + 原始 error 进日志）。本任务延续其"不向前端泄露内部文本"原则，把 422 unique 分支从字符串匹配换成 sentinel。insight L33 记录了 writeInternalError 范式。
- **T-007**（更早）：引入 `ErrDuplicateName` sentinel + handler 409 映射，本任务对称扩展。

## 预核实技术事实（orchestrator 逐处核对）

- DB: `migrations/0001_init.up.sql` L36 `remote_port INTEGER` + L46 `CREATE UNIQUE INDEX idx_proxies_tcp_remote ON proxies(type, remote_port)`。sqlite 违规文本含 `UNIQUE constraint failed: proxies.type, proxies.remote_port`。
- storage: `store.go:48-53` `ErrDuplicateName`；`proxies.go:329-336` `isDuplicateNameError`（匹配 `UNIQUE constraint failed`+`proxies.name`）；insert L122-127 / update L172-176 两处用之，(type,remote_port) 冲突现落到 `fmt.Errorf(...: %w)`。
- handler: `handlers_proxies.go:230-263` `mapProxyWriteError`；L246-256 用 `strings.Contains(low,"unique"/"constraint"/"remote_port")` 给 422（要消除）；L257-260 validation 块透传英文 msg。
- **测试依赖（grep 已确认，传 Developer）**：
  - `storage/proxies_test.go:51-88` `TestUpsertProxy_DuplicateTypeRemotePortNotSentinel` 现为弱断言（NOT ErrDuplicateName + 含 unique）→ 本任务升级为正向 `ErrDuplicateRemotePort`。
  - `storage/qa_t007_adversarial_test.go:25` 表驱动含 (type,remote_port) 行断言 `isDuplicateNameError == false`（仍成立，勿破坏）。
  - `httpapi/handlers_proxies_test.go:56-93` `TestCreateProxy_DuplicateTypeRemotePort_Returns422` 走真 storage 冲突 → 422 + field=remotePort（message 未断言，可保留；改后仍 422）。
  - **`httpapi/handlers_hygiene_test.go:123-134` `TestMapProxyWriteError_Validation_Preserved` 断言 validation 错误原文 `must be 1-65535` 透传** —— 任务点 3 把 validation 块英文 msg 改固定中文会破坏此测试，**必须同步更新**。

## 红线提醒（传所有下游）

- 不改已合并 migration（0001）。
- SQL 文本匹配只留在 storage 层（DAO 拥有驱动细节）。
- 不在 internal 子包外引用 internal。
- 不编辑 `.claude/` / `CLAUDE.md` / `.github/`。
- 测试数只升不降；同步 bump `scripts/baseline.json` 的 `go_tests` + `test_count`。
- 不 git commit/push、不跑 archive-task（orchestrator 负责）。

## 阶段路由

| 时间 | 阶段 | 动作 | 结果 |
|---|---|---|---|
| 2026-05-30 | 0 init | 创建任务夹、读 insight-index、核实技术上下文、检测分区模式（dev-db/dev-backend/dev-frontend 存在） | 完成；分区模式 |
| 2026-05-30 | 1 req | 派 Requirement Analyst（INPUT.md 含逐处核实的技术上下文 + 测试依赖 grep 结果）→ 01 产出 | **READY**，无开放问题 → 推进 |
| 2026-05-30 | 2 design | 派 Solution Architect → 02 产出（无新模块/无 schema/对称复刻 ErrDuplicateName 范式；Partition assignment：dev-db→dev-backend 串行；baseline.json 显式划 dev-backend） | **READY** → 推进 |
| 2026-05-30 | 3 gate | 派 Gate Reviewer（Mode A 自写，上下文有 Write）→ 03 产出，8 维度全 PASS，独立核实全部引用代码存在 | **APPROVED**（强条件 C-1：validation 文案中文化必破坏 TestMapProxyWriteError_Validation_Preserved，PM 显式批准同步更新该断言——属红线 3 的"PM 批准的过时断言更新"，非删活测试）→ 闸门通过，推进 stage 4 |
| 2026-05-30 | 3.5 gate | baseline.json 现值记录：test_count=799 / go_tests=318 / frontend_tests=481 | 记录 |

## 强条件批准记录（红线 3 例外授权）

- **C-1（PM 批准）**：任务点 3 要求把 handler validation 块的英文文案中文化，这会破坏 `internal/httpapi/handlers_hygiene_test.go:123-134` `TestMapProxyWriteError_Validation_Preserved`（断言透传 `must be 1-65535`）。该断言是验证"旧行为=透传英文"，本任务**有意改变该行为**（不向前端泄露内部英文文本，与 T-055 原则一致）。故同步更新该断言为"断言固定中文 + 响应体不含原始英文"是**受控的预期更新**，非"删活测试过闸门"。PM 据红线 3 显式批准此更新；测试**计数不下降**（仅改断言内容，用例数不减，且本任务整体净增测试）。

## Stage 4 开发（分区顺序 dev-db → dev-backend）

| 时间 | 分区 | 动作 | 结果 |
|---|---|---|---|
| 2026-05-30 | dev-db | storage: ErrDuplicateRemotePort sentinel + isDuplicateRemotePortError 助手 + insert/update 两处接入 + storage 测试升级/新增（+3 净增顶层 Test）+ 移除 proxies_test unused strings import | READY FOR REVIEW（DB partition complete） |
| 2026-05-30 | dev-backend | handler: ErrDuplicateRemotePort→422 分支 + 删 unique 字符串块 + validation 文案中文化 + Warn 日志；handler 测试升级/新增（+1 净增顶层 Test）；baseline bump 318→322 / 799→803 | READY FOR REVIEW（backend partition complete） |
| 2026-05-30 | PM | 跨包净增顶层 Test = 4（dev-db 3 + dev-backend 1），baseline 同步 bump 一致；C-1 受控测试更新已落地（改名+改断言，计数不降） | 两分区完成 → 推进 stage 5 |

### stage 4→5 闸门：verify_all

PM/orchestrator 当前会话工具集仅 Read/Write/Edit/Glob/Grep，**无 Bash/PowerShell**，无法自跑 `scripts/verify_all`。按项目既有交付惯例（T-055~T-058 baseline notes 一致）：静态闸门全绿（import 完整、sentinel 互斥、计数同步），**全量 verify_all 真跑标 PENDING，交给有 Bash 的 orchestrator 会话作交付硬闸门**。基于静态正确性继续推进 stage 5 code review。

## Stage 5 代码评审

| 时间 | 阶段 | 动作 | 结果 |
|---|---|---|---|
| 2026-05-30 | 5 review | 派 Code Reviewer（Mode A 自写，有 Write）→ 05；逐条 AC + 设计保真核对，重点查 sentinel 互斥 + 不泄露 + 删块后无退化 | **APPROVED**（0 CRITICAL/MAJOR/MINOR，1 NIT 不抽公共助手）→ 推进 |
| 2026-05-30 | 6 test | 派 QA Tester → 06；每 AC 有测试 + 裸 ## Adversarial tests（AT-1 驱动文本变化不影响分类的反向论证）；QA 无 Bash，真跑标 PENDING + 确定性执行规格 | **APPROVED FOR DELIVERY**（条件：orchestrator 真跑 verify_all PASS）→ 推进 |
| 2026-05-30 | 7 delivery | PM 写 07_DELIVERY.md（含裸 ## Insight ×2）；补 dev-map storage 哨兵清单；更新 tasks.md | 交付完成 |

## 闸门检查汇总

- stage 4→5：dev verify_all 真跑 PENDING（PM 无 Bash），静态全绿，按项目惯例继续（T-055~T-058 一致）。
- stage 5→6：CR APPROVED（无 CRITICAL/MAJOR）。
- stage 7 前：CR + QA 均 PASS（QA 真跑硬闸门 PENDING 交 orchestrator）。
- **0 rollback，无 BLOCKED，无连续回退。**

## 交付后待办（交 orchestrator）

1. orchestrator Bash 会话真跑 `bash scripts/verify_all.sh`（预期 PASS：go_tests=322 / test_count=803）——交付硬闸门。
2. PASS 后跑 `scripts/archive-task --task proxy-remoteport-conflict-sentinel`（收割 07 §Insight ×2 → insight-index）。
3. git commit/push（本任务按要求未做）。

## 早期阶段路由（续表 1-3.5，见上）
