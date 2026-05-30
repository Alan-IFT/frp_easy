# 05 代码评审 — T-059 proxy-remoteport-conflict-sentinel

> 阶段 5 / Code Reviewer · mode: full · 中文
> 独立视角，逐行对照 01 AC / 02 设计 / 04 开发记录。仅发现问题，不改代码。

## Files reviewed

- `internal/storage/store.go`（新增 `ErrDuplicateRemotePort`）
- `internal/storage/proxies.go`（新增助手 + insert/update 两处接入）
- `internal/storage/proxies_test.go`（升级 + 新增）
- `internal/storage/qa_t007_adversarial_test.go`（强化 + 新增）
- `internal/httpapi/handlers_proxies.go`（`mapProxyWriteError` 三处改动）
- `internal/httpapi/handlers_hygiene_test.go`（改名+改断言 + 新增 + import）
- `internal/httpapi/handlers_proxies_test.go`（补断言 + import）
- `scripts/baseline.json`（计数 bump）

## Findings

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### NIT

- [STYLE] `internal/storage/proxies.go:354` `isDuplicateRemotePortError` 与 `isDuplicateNameError` 高度相似，理论上可抽 `containsUniqueViolation(err, col string)` 公共助手。但项目仅两处约束、抽取会牺牲两个独立直测护栏的可读性，且违背设计 §10 的 YAGNI 边界——**不建议抽取**，保持两个对称助手更清晰。纯偏好，不阻断。

## Requirement coverage check（逐条 01 AC）

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 编译/vet | import 完整（store/proxies/handler 生产文件 errors/strings/fmt 仍用；proxies_test 移除 unused strings；hygiene_test 加 encoding/json；proxies_test[httpapi] 加 strings） | 待 orchestrator 真跑确认 |
| AC-2 storage 插入返回 ErrDuplicateRemotePort | `proxies.go:126-128` + `proxies_test.go` TestUpsertProxy_DuplicateTypeRemotePortReturnsSentinel（正向 + 负向 NOT ErrDuplicateName） | OK |
| AC-3 助手正向 | `proxies_test.go` TestIsDuplicateRemotePortError_DirectChecks normal/wrapped+errno=true | OK |
| AC-4 助手负向 | 同上 nil/name 冲突/无关/缺 UNIQUE 前缀=false（含互斥项） | OK |
| AC-5 storage 更新路径 | `proxies.go:175-177` + TestUpsertProxy_UpdateToDuplicateRemotePortReturnsSentinel（UPDATE 改 remotePort 触发组合冲突） | OK |
| AC-6 handler 映射 422+remotePort+固定中文+不泄露 | `handlers_proxies.go:248-251` + TestMapProxyWriteError_DuplicateRemotePort（断言 code/field/含'远程端口'/不含 UNIQUE/constraint/proxies./remote_port/duplicate） | OK |
| AC-7 name 不退化 409 | `handlers_proxies.go:240-242`（未动）+ TestMapProxyWriteError_DuplicateName_Preserved（既有，未破坏） | OK |
| AC-8 validation 改固定中文 + 测试同步 | `handlers_proxies.go:255-262` + TestMapProxyWriteError_Validation_FixedMessage_NoLeak（断言固定中文+不泄露 storage 英文+进日志） | OK |
| AC-9 端到端 422 不退化 | `handlers_proxies_test.go` TestCreateProxy_DuplicateTypeRemotePort_Returns422（真 storage 冲突，补 message 不含英文断言） | OK |
| AC-10 计数 | baseline go_tests 318→322 / test_count 799→803，与净增 4 吻合 | OK（计数闸门待真跑） |
| AC-11 对抗段 | 06 待 QA 产出（裸 ## Adversarial tests） | 转 stage 6 |
| AC-12 verify_all PASS | PM 上下文无 Bash，全量真跑交 orchestrator 硬闸门 | PENDING |

## Design fidelity check（逐条 02 设计）

| Design item | Implementation | Status |
|---|---|---|
| §3 ErrDuplicateRemotePort 定义紧邻 ErrDuplicateName | `store.go` 新增 var + 对称注释 | OK |
| §3 isDuplicateRemotePortError 对称伪代码 | `proxies.go:354-360` 与伪代码逐字一致（UNIQUE constraint failed + proxies.remote_port + nil 守卫） | OK |
| §6 判定顺序 name 在 remote_port 之前（storage + handler 两层） | storage insert L123/L126、update L172/L175；handler L240/L248 —— 两层均 name 先 | OK |
| §5 API 契约（409 name / 422 remotePort / 422 validation 固定中文 / 500 兜底不变） | handler 映射表与设计表逐行一致 | OK |
| §3 SQL 文本匹配只留 storage 层 | handler 已删 unique/constraint/remote_port 字符串块；validation 块判定的是 storage 生成的业务英文（非驱动文本），保留判定、仅改对前端 msg —— 符合"驱动文本只在 storage"原则 | OK |
| §8 风险2 删块后无未覆盖 unique 落 500 | proxies 表仅两 UNIQUE 约束，两 sentinel 穷尽；TestCreateProxy_DuplicateTypeRemotePort_Returns422 走真 storage 验证组合冲突仍 422 非 500 | OK |
| §11 分区边界 | storage 改动全在 internal/storage（dev-db）；handler+baseline 全在 internal/httpapi + scripts/baseline.json（dev-backend，§11 显式划归）；无越界 | OK |
| §10 OOS 不抽通用框架/不动 migration/不动事务逻辑 | 无 migration 改动；UpsertProxy 事务/版本逻辑未动；未抽通用框架 | OK |

无 DESIGN DRIFT。

## 逻辑正确性细查

- **sentinel 互斥（核心正确性）**：name 冲突文本 `UNIQUE constraint failed: proxies.name` —— 含 `proxies.name`、**不含** `proxies.remote_port`，故 `isDuplicateNameError`=true / `isDuplicateRemotePortError`=false。组合冲突文本 `...: proxies.type, proxies.remote_port` —— 含 `proxies.remote_port`、**不含** `proxies.name`，故反之。两助手对同一 error 不会同时 true；即便同时 true，handler 也先判 name（顺序保护）。互斥正确，直测正负例覆盖。
- **判定顺序**：sqlite 单次 INSERT/UPDATE 违规只报一个 UNIQUE 约束，文本只含其一，name 先判不会误吞 remote_port 冲突。
- **nil 守卫**：助手对 nil 返回 false；handler validation 块 logger nil 守卫（`h.deps.Logger != nil`），与既有范式一致，nil logger 不 panic。
- **包装错误**：助手用子串匹配，对 `fmt.Errorf(...: %w)` 包装 + `(2067)` 后缀仍命中（wrapped 直测覆盖）。

## 测试有效性（非 shape-matching）

- storage 真冲突用例用真实 sqlite（`Open` + 真 INSERT 触发约束），非伪造 error 字符串——`TestAdversarial_AC6_RealDBDuplicateNameSentinel` 的 p3 现强化为正向 `ErrDuplicateRemotePort`，是端到端真路径断言。
- 助手直测用真实驱动文本格式 + 包装 + 互斥负例，能真正捕获"驱动改文本"回归。
- handler 测试断言响应体**不含**具体 SQL/驱动英文子串（leak 黑名单），是有意义的"不泄露"断言，非仅匹配状态码。
- C-1 改名测试（Validation_FixedMessage_NoLeak）断言行为已改变（固定中文 + 不泄露 + 进日志），与本任务意图一致，是受控更新非削弱。

## 性能 / 安全

- 性能：纯错误分类，无新 query / 循环 / 分配；零热路径影响。
- 安全：**正向改善**——422 与 validation 响应不再向前端透传 SQL/驱动/storage 内部英文文本（消除信息泄露面），与 T-055 原则一致，并有 leak 黑名单断言锁死。无新输入、无 authz 变更、无注入面。

## Verdict

**APPROVED**（0 CRITICAL，0 MAJOR，0 MINOR，1 NIT 不阻断）

实现与需求/设计逐条吻合，sentinel 互斥正确，测试有效且为受控更新，安全性正向改善，无设计漂移。唯一外部依赖项：AC-1/AC-10/AC-12 的全量 `verify_all` 真跑由 orchestrator Bash 会话作交付硬闸门（PM 上下文无 Bash，静态自检全绿）。可进入 stage 6 QA。
