# INPUT — T-059 proxy-remoteport-conflict-sentinel

> PM Orchestrator 提供给 Requirement Analyst 的任务输入。mode: **full**。

## 一句话目标

把 `(type, remote_port)` 唯一冲突在 storage 层 sentinel 化（新增 `ErrDuplicateRemotePort`），消除 handler 层 `mapProxyWriteError` 对 SQL 驱动错误文本的脆弱 `strings.Contains` 匹配，与既有 `ErrDuplicateName` 范式对齐。偿还 T-055 记录的 backlog。

## 背景（脆弱反模式）

当前 handler `internal/httpapi/handlers_proxies.go:246-256` 在 **handler 层**用 `strings.Contains(low, "unique"/"constraint"/"remote_port")` 匹配 SQLite 驱动错误文本来给 422。这意味着：驱动（modernc.org/sqlite）升级若改了错误文本，handler 会静默漏判把组合冲突错分到 500 兜底。正确范式是 storage 层（DAO，拥有驱动细节）把驱动错误文本翻译成 sentinel，handler 只用 `errors.Is` 判 sentinel。项目已有 `ErrDuplicateName`（name 列 UNIQUE → 409）这一对称范式（T-007 引入），本任务为 `(type, remote_port)` 组合 UNIQUE 补上同款 sentinel（→ 422）。

## 精确技术上下文（orchestrator 已逐处核实）

- **DB 约束** `internal/storage/sqlmigrations/0001_init.up.sql`：
  - L32（近似）`name TEXT NOT NULL UNIQUE`，违规文本含 `UNIQUE constraint failed: proxies.name`。
  - L46 `CREATE UNIQUE INDEX idx_proxies_tcp_remote ON proxies(type, remote_port)`，违规文本含 `UNIQUE constraint failed: proxies.type, proxies.remote_port`。
- **storage**（`internal/storage/`）：
  - sentinel 定义 `store.go:35-54`（`ErrDuplicateName` 在 L48-53）。
  - `proxies.go` `UpsertProxy` insert（L122-127）/ update（L172-176）用 `isDuplicateNameError(err)`（L329-336，匹配 `UNIQUE constraint failed`+`proxies.name`）→ 返回 `ErrDuplicateName`；其它 unique 冲突（即 (type,remote_port)）现落到 `fmt.Errorf("...: %w", err)` 包装错误。
- **handler**（`internal/httpapi/handlers_proxies.go`）：
  - `mapProxyWriteError`（L230-263，T-055 已改为 `*handlers` 方法）：L242-245 `ErrDuplicateName` → 409 + field=name；L246-256 字符串匹配给 422（要消除）；L257-260 validation 块（`requires`/`must not`/`invalid`）透传英文 msg；L262 `h.writeInternalError(w, "保存失败", err)` 兜底 500。

## 修复方向（不扩散，仅 dev-db/dev-backend 落地）

1. **storage 新增 sentinel** `ErrDuplicateRemotePort`（紧邻 `ErrDuplicateName`）。
2. **storage 新增助手** `isDuplicateRemotePortError(err)`（匹配 `UNIQUE constraint failed`+`proxies.remote_port`，与 `isDuplicateNameError` 对称）；`UpsertProxy` insert+update 两处在 `isDuplicateNameError` 检查**之后**加 `if isDuplicateRemotePortError(err) { return ErrDuplicateRemotePort }`，再 fallthrough 原 `fmt.Errorf`。SQL 文本匹配只留 storage 层。
3. **handler** `mapProxyWriteError`：`ErrDuplicateName` 分支后加 `if errors.Is(err, storage.ErrDuplicateRemotePort)` → 422 + field=remotePort + 固定中文（如"该类型下远程端口已被占用，请改用其它端口"）；**删除** L246-256 字符串匹配整块；L257-260 validation 块的面向前端 `msg`（storage 生成英文）改固定中文文案（与 T-055 不泄露内部文本原则一致）。

## 已确认的测试依赖（grep 已查，必须在实现中同步处理）

- `internal/storage/proxies_test.go:51-88` `TestUpsertProxy_DuplicateTypeRemotePortNotSentinel` 现为弱断言（NOT ErrDuplicateName + 含 unique）→ 升级为正向断言 `ErrDuplicateRemotePort`。
- `internal/storage/qa_t007_adversarial_test.go:25` 表驱动含 (type,remote_port) 行断言 `isDuplicateNameError == false`（仍成立，勿破坏）。
- `internal/httpapi/handlers_proxies_test.go:56-93` `TestCreateProxy_DuplicateTypeRemotePort_Returns422`（走真 storage 冲突，断言 422 + code=CONFLICT + field=remotePort；未断言 message）→ 改后仍应 422 通过。
- **`internal/httpapi/handlers_hygiene_test.go:123-134` `TestMapProxyWriteError_Validation_Preserved` 断言 validation 错误原文 `must be 1-65535` 透传** —— 任务点 3 把 validation 块英文 msg 改固定中文会破坏此测试，**必须同步更新该测试断言**（改为断言固定中文 + 不含 SQL 英文）。

## 验收硬要求（写进 AC）

- storage：插入两条 `(tcp, 相同 remote_port)`，第二条返回 `ErrDuplicateRemotePort`（既非 `ErrDuplicateName`、亦非裸 wrapped error）。`isDuplicateRemotePortError` 正/负用例。
- handler：构造 storage 返回 `ErrDuplicateRemotePort` → 422 + field=remotePort + 固定中文（响应体不含任何 SQL/驱动英文文本）。name 冲突仍 409。
- 测试数只升不降；同步 bump `scripts/baseline.json` 的 `go_tests` + `test_count`（`go test -list` 口径）。
- 06 含**裸** `## Adversarial tests` 段（禁前缀）：一条"驱动错误文本变化也不影响分类"的反向论证（sentinel 化后不再依赖文本）。
- `scripts/verify_all` 全量 PASS（由 orchestrator 真跑硬闸门）。

## 红线

- 不改已合并 migration（0001）；SQL 文本匹配只在 storage 层；不在 internal 子包外引用 internal；不编辑 `.claude/` / `CLAUDE.md` / `.github/`；不 git commit/push、不跑 archive-task（orchestrator 负责）。

## 关联历史

- T-055 backend-api-hygiene（直接前置，记录了此 backlog；引入 writeInternalError 范式 / insight L33）。
- T-007（引入 ErrDuplicateName + 409 映射的对称范式）。
