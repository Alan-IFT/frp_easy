# 01 需求分析 — T-059 proxy-remoteport-conflict-sentinel

> 阶段 1 / Requirement Analyst · mode: full · 中文

## 1. 目标（一句话）

把 `(type, remote_port)` 组合唯一约束冲突在 storage 层翻译成专用 sentinel 错误（`ErrDuplicateRemotePort`），使 HTTP handler 仅凭 `errors.Is` 判定冲突类型，从而消除 handler 层对 SQLite 驱动错误文本的脆弱字符串匹配。

## 2. 范围内行为（可测、编号）

1. storage 层暴露一个新的导出 sentinel 错误，专门表示 `(type, remote_port)` 组合 UNIQUE 约束冲突，与既有 name 冲突 sentinel 区分。
2. `UpsertProxy` 的**插入**路径：当底层驱动报 `(type, remote_port)` 组合 UNIQUE 冲突时，返回该新 sentinel（而非裸包装错误，亦非 name 冲突 sentinel）。
3. `UpsertProxy` 的**更新**路径：同插入路径，返回该新 sentinel。
4. storage 层提供一个判定助手，把驱动错误文本判定为"是否 `(type, remote_port)` 组合冲突"，与既有 name 判定助手对称（同一文件、同一形态）。该助手是 storage 层唯一持有驱动错误文本细节的地方。
5. HTTP handler `mapProxyWriteError` 用 `errors.Is(err, <新 sentinel>)` 判定 `(type, remote_port)` 冲突，返回 HTTP 422、错误码 CONFLICT、`field=remotePort`、固定中文用户文案（不含任何 SQL/驱动英文文本）。
6. handler 删除原先对 `unique`/`constraint`/`remote_port` 的 `strings.Contains` 字符串匹配整块（其职责已被两个 sentinel 完全覆盖）。
7. handler 中 storage 校验类错误（原 storage 生成的英文 `requires`/`must not`/`invalid` 文案）改为面向前端的固定中文文案，原始 error 不向前端透传（与 T-055 "不向前端泄露内部文本"原则一致）。
8. name 冲突仍返回 HTTP 409 + `field=name` + 既有中文文案（行为不变）。
9. 测试套件新增对新 sentinel 的正/负向覆盖（storage 真冲突路径 + 助手直测 + handler 映射）；现存依赖旧行为的测试同步更新为新断言。
10. `scripts/baseline.json` 的 `go_tests` 与 `test_count` 同步上调到 `go test -list` 实际口径。

## 3. 范围外（本次明确不做）

- 不改已合并的 migration（`0001_init.up.sql`），约束本身不动。
- 不改前端 Vue 代码（`field=remotePort` 已是前端既有可识别字段，错误展示链路不变）。
- 不引入新的 HTTP 状态码或错误码（422 + CodeConflict 维持）。
- 不重构 `UpsertProxy` 的事务/版本校验逻辑。
- 不处理 name 与 (type,remote_port) **同时**冲突的优先级语义之外的新场景（沿用"先判 name、后判 remote_port"的既有判定顺序，因驱动单次违规只报一个约束）。
- 不改 `writeInternalError` helper 本身（仅复用其"日志通道 + 固定文案"原则）。

## 4. 边界条件

- **nil error**：助手对 `nil` 返回 false（与 `isDuplicateNameError` 对称，已有范式）。
- **包装错误**：助手对被 `fmt.Errorf(...: %w)` 包装、含 `(2067)` 错误码后缀的文本仍能命中（子串匹配，已有范式）。
- **判定顺序**：单次 INSERT/UPDATE 违规只触发一个 UNIQUE 约束，sqlite 文本只含其一；name 与 remote_port 判定互斥，先 name 后 remote_port 的顺序不产生歧义。
- **驱动文本变化（核心动机）**：sentinel 化后，handler 不再依赖文本；若未来驱动改文本，storage 助手的直测会立即捕获回归，handler 分类逻辑零改动。
- **空 remote_port**：http/https 类型 `remote_port` 为 NULL，部分唯一索引 `(type, remote_port)` 对 NULL 不去重（sqlite NULL 语义），不触发本冲突路径——属既有行为，不在本次新增覆盖。
- **422 响应体**：必须可被断言"不含 SQL/驱动英文子串"（如不含 `UNIQUE`、`constraint`、`proxies.`、`remote_port` 原文）。

## 5. 验收标准（每条可验证）

- **AC-1**：`go build ./...` 通过；`go vet ./...` 无新增告警。
- **AC-2**（storage 插入）：连续插入两条 `type=tcp` 且 `remote_port` 相同、`name` 不同的 proxy，第二条 `UpsertProxy` 返回的 error 满足 `errors.Is(err, ErrDuplicateRemotePort) == true` 且 `errors.Is(err, ErrDuplicateName) == false`。
- **AC-3**（storage 助手正向）：助手对 `"UNIQUE constraint failed: proxies.type, proxies.remote_port"` 及其包装/带 `(2067)` 形态返回 true。
- **AC-4**（storage 助手负向）：助手对 `nil`、name 冲突文本、无关错误、缺前缀文本返回 false。
- **AC-5**（storage 更新路径）：UPDATE 触发 `(type, remote_port)` 冲突同样返回 `ErrDuplicateRemotePort`。
- **AC-6**（handler 映射）：`mapProxyWriteError` 收到 `ErrDuplicateRemotePort` → HTTP 422、`code=CONFLICT`、`field=remotePort`、`message` 为固定中文且响应体不含 SQL/驱动英文子串。
- **AC-7**（handler name 不退化）：`mapProxyWriteError` 收到 `ErrDuplicateName` → 仍 409 + `field=name` + 既有中文。
- **AC-8**（handler validation 改文案）：`mapProxyWriteError` 收到 storage 校验类英文错误（如 `... must not set remotePort`）→ 422 + 固定中文，响应体不含原始英文。对应被破坏的现存测试 `TestMapProxyWriteError_Validation_Preserved` 同步更新为新断言。
- **AC-9**（端到端回归）：现存 `TestCreateProxy_DuplicateTypeRemotePort_Returns422`（POST 真冲突）改后仍 422 + field=remotePort 通过。
- **AC-10**（计数）：`scripts/verify_all` 的测试计数闸门通过；`baseline.json` 的 `go_tests`/`test_count` 等于 `go test -list` 实际值（只升不降）。
- **AC-11**（对抗）：`06_TEST_REPORT.md` 含裸 `## Adversarial tests` 段，含一条"驱动错误文本若变化，handler 分类不受影响（因不再依赖文本）"的反向论证。
- **AC-12**：`scripts/verify_all` 全量 PASS（orchestrator 真跑硬闸门）。

## 6. 非功能需求

- **安全 / 隐私**：HTTP 错误响应不得泄露 SQL/驱动内部文本（延续 T-055 原则）。这是本任务的强约束，不仅是 nice-to-have。
- **可维护性**：驱动错误文本匹配集中在 storage 单点；handler 仅依赖 sentinel 类型契约。
- **兼容性**：对外 API 契约（状态码、错误码、field 名）不变，仅 422 与 validation 的 `message` 文案中文化；前端无需改动。

## 7. 关联任务

- **T-055 backend-api-hygiene**（`docs/features/backend-api-hygiene/`）：直接前置，记录本 backlog；引入 `writeInternalError`（500 兜底固定文案 + 原始 error 进日志）。本任务延续其设计；insight-index L33 记录该范式。
- **T-007 hardening-pass-audit**：引入 `ErrDuplicateName` + handler 409 映射的对称范式，本任务为 `(type, remote_port)` 补齐同款。

## 8. 给用户的待澄清问题

无。上下文已由 orchestrator 逐处核实，修复方向、文案策略、测试依赖均已明确给定。

## 9. 裁决

**READY** —— 无悬而未决的歧义，进入方案设计阶段。
