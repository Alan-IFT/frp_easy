# 04 开发记录 — T-059 proxy-remoteport-conflict-sentinel

> 阶段 4 / Developer（分区：dev-db → dev-backend，严格串行）· mode: full · 中文
> 闸门：03 GR 裁决 APPROVED（强条件 C-1 已 PM 批准）。

## DESIGN DRIFT 声明

无。实现与 02 方案 §3/§5/§6/§11 完全一致，未偏离已批准设计。

仅一处实现细节落地（设计 §6 给定的开放选择，由 dev-backend 在范围内决定）：validation 块固定中文化时，原始 error 通过 `h.deps.Logger.Warn("proxy write validation error", "cause", err)` 进日志（nil 守卫），复用 handler 既有 `h.deps.Logger` 通道（与 `writeInternalError` 同款），未新增方法、未引入依赖。

---

## 分区一：dev-db（internal/storage/**）

### 改动文件

- `internal/storage/store.go`：新增导出 sentinel `ErrDuplicateRemotePort = errors.New("storage: duplicate (type, remote_port)")`，紧邻 `ErrDuplicateName`，附对称注释（指向 0001 第 46 行的 `idx_proxies_tcp_remote` 索引；说明 handler 据此返回 422）。
- `internal/storage/proxies.go`：
  - 新增助手 `isDuplicateRemotePortError(err error) bool`（匹配 `UNIQUE constraint failed` + `proxies.remote_port`，与 `isDuplicateNameError` 1:1 对称；nil 守卫）。
  - `UpsertProxy` **insert** 路径：在 `isDuplicateNameError(err)` 检查后加 `if isDuplicateRemotePortError(err) { return ErrDuplicateRemotePort }`，再 fallthrough 原 `fmt.Errorf("storage.UpsertProxy insert: %w", err)`。
  - `UpsertProxy` **update** 路径：同款插入，`tx.Rollback()` 后、`fmt.Errorf("storage.UpsertProxy update: %w", err)` 前。
- `internal/storage/proxies_test.go`：
  - `TestUpsertProxy_DuplicateTypeRemotePortNotSentinel` → 改名 `..._ReturnsSentinel`，弱断言（NOT ErrDuplicateName + 含 unique）升级为正向 `errors.Is(err, ErrDuplicateRemotePort)` + 负向 NOT ErrDuplicateName。**改名不增减顶层 Test 计数**。
  - 新增 `TestUpsertProxy_UpdateToDuplicateRemotePortReturnsSentinel`（UPDATE 路径触发组合冲突 → ErrDuplicateRemotePort）。
  - 新增 `TestIsDuplicateRemotePortError_DirectChecks`（表驱动：normal/wrapped+errno true；nil/name 冲突/无关/缺 UNIQUE 前缀 false——含关键互斥项"name 冲突不误判为 remote_port"）。
  - 移除文件级不再使用的 `strings` import（升级后无 `strings.` 调用，go build 会因 unused import 失败，故移除——非删测试）。
- `internal/storage/qa_t007_adversarial_test.go`：
  - 强化 `TestAdversarial_AC6_RealDBDuplicateNameSentinel` 的 p3 真 DB 用例：弱断言（非 nil + 非 ErrDuplicateName）→ 正向 `errors.Is(err, ErrDuplicateRemotePort)`。
  - 新增 `TestAdversarial_T059_RemotePortErrorTextVariants`（remote_port 文本变体对抗，记录与 name 版相同的大小写敏感已知局限）。

### Schema change

无。不改任何 migration。`idx_proxies_tcp_remote ON proxies(type, remote_port)`（0001 第 46 行）原样保留。

### Rollback plan

纯 Go 代码改动，`git revert` 即可。无 DDL、无数据回填、无持久化副作用。

### Data impact

- Affected rows：N/A（无数据变更）。
- Backfill required：no。

### Coordination

dev-backend 在本任务消费 `storage.ErrDuplicateRemotePort`（handler `errors.Is`）。dispatch 顺序 dev-db → dev-backend 已遵守。

### dev-db 净增顶层 Test

**+3**（UpdateToDuplicateRemotePort / IsDuplicateRemotePortError_DirectChecks / Adversarial_T059_RemotePortErrorTextVariants）。改名 1 个、强化 2 个既有断言（不计净增、不删测试）。

### Verdict

READY FOR REVIEW（DB partition complete）。

---

## 分区二：dev-backend（internal/httpapi/** + scripts/baseline.json）

### 改动文件

- `internal/httpapi/handlers_proxies.go` `mapProxyWriteError`：
  - `ErrDuplicateName` 409 分支后新增 `if errors.Is(err, storage.ErrDuplicateRemotePort)` → `writeError(w, 422, CodeConflict, "该类型下远程端口已被占用，请改用其它端口", "remotePort")`。
  - **删除**原 L246-256 `strings.Contains(low, "unique"/"constraint"/"remote_port")` → 422 整块（职责已被两 sentinel 完全覆盖）。
  - validation 块（`requires`/`must not`/`invalid`）：面向前端 `msg` 由透传 `err.Error()` 改固定中文 `代理配置校验失败`（422 + CodeValidationFailed + field=""）；原始 error 进 `h.deps.Logger.Warn`（nil 守卫）。
  - `strings` import 保留（L163 `TrimSpace` + validation 块判定仍用；注意：validation 判定的是 storage **自己生成**的英文业务文案，非驱动文本，属业务分类非脆弱驱动依赖，故保留判定逻辑、仅改对前端的 msg）。
- `internal/httpapi/handlers_hygiene_test.go`：
  - `TestMapProxyWriteError_Validation_Preserved` → 改名 `..._FixedMessage_NoLeak`，断言由"透传 must be 1-65535"改为"固定中文 + 响应体不含 storage 英文（must not set/customDomains/UpsertProxy/udp proxy）+ 原始 error 进日志"。**强条件 C-1，PM 批准**（红线 3 例外：有意改变行为，非删活测试，计数不降）。
  - 新增 `TestMapProxyWriteError_DuplicateRemotePort`（ErrDuplicateRemotePort → 422 + code=CONFLICT + field=remotePort + message 含'远程端口' + 响应体不含 UNIQUE/constraint/proxies./remote_port/duplicate）。
  - 加 `encoding/json` import（新测试解析 ErrorBody）。
- `internal/httpapi/handlers_proxies_test.go` `TestCreateProxy_DuplicateTypeRemotePort_Returns422`：补 message 断言（含'远程端口'）+ 响应体不含 SQL 英文 / 不含旧文案'字段冲突'。加 `strings` import。
- `scripts/baseline.json`：`go_tests` 318→322、`test_count` 799→803、`passing_count`→803、`version` 24→25、notes 追加 T-059 段。

### API 契约对齐（02 §5）

| 触发 | 状态码 | code | field | message |
|---|---|---|---|---|
| ErrDuplicateName | 409 | CONFLICT | name | 代理名称已存在，请改用其它名称（不变） |
| ErrDuplicateRemotePort（新） | 422 | CONFLICT | remotePort | 该类型下远程端口已被占用，请改用其它端口 |
| validation（requires/must not/invalid） | 422 | VALIDATION_FAILED | "" | 代理配置校验失败（固定中文，不再透传英文） |
| 兜底 | 500 | INTERNAL | "" | 保存失败（不变） |

实现与设计表一致。

### dev-backend 净增顶层 Test

**+1**（TestMapProxyWriteError_DuplicateRemotePort）。改名 1 个、补断言 1 个（不计净增、不删测试）。

### 跨包总计

dev-db +3 + dev-backend +1 = **净增 4 个顶层 Test**。baseline go_tests 318→322 / test_count 799→803 与之吻合（`go test -list` 口径）。

### verify_all result

PM 上下文（扮演 dev 角色）无 Bash/PowerShell 工具，**未能自跑** `scripts/verify_all`。静态自检：

- 编译：所有改动文件 import 完整（storage proxies_test 移除 unused `strings`；hygiene_test 加 `encoding/json`；proxies_test 加 `strings`），无 unused/缺失 import；`errors`/`strings`/`fmt` 在生产文件中均已 import 且仍被使用。
- sentinel 互斥：name 文本含 `proxies.name` 不含 `proxies.remote_port`；组合文本含 `proxies.remote_port` 不含独立 `proxies.name` —— 两助手互斥，直测正负例覆盖。
- 计数：净增 4，baseline 已同步 bump。

**verify_all 全量真跑（go build/vet/test + 前端 + 计数闸门）交 orchestrator Bash 会话作硬闸门。**

### Verdict

READY FOR REVIEW（backend partition complete）。全部改动在 owned paths 内（internal/httpapi/** + scripts/baseline.json，后者经 02 §11 显式划归本分区），无 BLOCKED ON PARTITION。
