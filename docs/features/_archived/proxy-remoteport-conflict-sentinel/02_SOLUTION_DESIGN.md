# 02 方案设计 — T-059 proxy-remoteport-conflict-sentinel

> 阶段 2 / Solution Architect · mode: full · 中文

## 1. 架构摘要

在 storage 层（DAO，唯一拥有 SQLite 驱动错误文本细节的边界）新增一个导出 sentinel `ErrDuplicateRemotePort` 与一个对称助手 `isDuplicateRemotePortError`，把 `(type, remote_port)` 组合 UNIQUE 约束冲突从"裸包装错误"翻译成显式 sentinel；HTTP handler `mapProxyWriteError` 改为用 `errors.Is` 判定该 sentinel 并返回固定中文 422，同时删除其内部对驱动错误文本的 `strings.Contains` 匹配。这把"驱动错误文本"这一脆弱依赖从 handler 层完全收敛回 storage 层单点，与既有 `ErrDuplicateName`（name 冲突 → 409）范式对称。不新增模块、不改 schema、不改前端、不改 API 形状（状态码/错误码/field 名不变）。

## 2. 受影响模块（仓库内真实路径）

| 模块 | 文件 | 改动性质 |
|---|---|---|
| storage sentinel | `internal/storage/store.go` | 新增 1 个 `var` sentinel（紧邻 `ErrDuplicateName` L53） |
| storage DAO | `internal/storage/proxies.go` | 新增助手 `isDuplicateRemotePortError`；insert(L122-127) + update(L172-176) 各加一处 sentinel 返回 |
| storage 测试 | `internal/storage/proxies_test.go` | 升级 `TestUpsertProxy_DuplicateTypeRemotePortNotSentinel` 为正向断言 + 新增助手直测/UPDATE 路径用例 |
| storage 对抗测试 | `internal/storage/qa_t007_adversarial_test.go` | 现有 (type,remote_port) 行断言保持；可选新增 remote_port 助手对抗行 |
| handler | `internal/httpapi/handlers_proxies.go` | `mapProxyWriteError` 加 sentinel 分支、删字符串匹配块、validation 文案中文化 |
| handler 卫生测试 | `internal/httpapi/handlers_hygiene_test.go` | 更新 `TestMapProxyWriteError_Validation_Preserved`（透传→固定中文）+ 新增 `ErrDuplicateRemotePort` 映射用例 |
| handler 端到端测试 | `internal/httpapi/handlers_proxies_test.go` | `TestCreateProxy_DuplicateTypeRemotePort_Returns422` 维持/补 message 断言（不含英文） |
| 基线 | `scripts/baseline.json` | bump `go_tests` + `test_count`（`go test -list` 口径） |
| 导航 | `docs/dev-map.md` | 仅在 storage 新增 export 符号时补一行（轻量） |

## 3. 模块分解（新符号）

无新模块。新增两个 storage 包内符号：

- **`storage.ErrDuplicateRemotePort`**（`store.go`，导出 var）
  - 责任：表示 `UpsertProxy` 时与 `(type, remote_port)` 组合 UNIQUE 索引 `idx_proxies_tcp_remote` 冲突。
  - 定义：`ErrDuplicateRemotePort = errors.New("storage: duplicate (type, remote_port)")`
  - 调用方契约：httpapi 据此返回 **422**（区别于 name 冲突的 409），field=remotePort。

- **`isDuplicateRemotePortError(err error) bool`**（`proxies.go`，包内未导出）
  - 责任：判定驱动错误文本是否 `(type, remote_port)` 组合 UNIQUE 冲突。
  - 实现（与 `isDuplicateNameError` 对称）：
    ```go
    func isDuplicateRemotePortError(err error) bool {
        if err == nil {
            return false
        }
        s := err.Error()
        return strings.Contains(s, "UNIQUE constraint failed") &&
            strings.Contains(s, "proxies.remote_port")
    }
    ```
  - 说明：sqlite 组合索引违规文本为 `UNIQUE constraint failed: proxies.type, proxies.remote_port`，含子串 `proxies.remote_port`；name 冲突文本 `proxies.name` 不含该子串，二者互斥不误判。

## 4. 数据模型变更

**无。** 不改 `internal/storage/sqlmigrations/0001_init.up.sql`，约束 `idx_proxies_tcp_remote ON proxies(type, remote_port)`（L46）原样保留。纯错误分类层改动。

## 5. API 契约

对外契约**不变形状**，仅 422/validation 的 `message` 文案中文化：

| 触发 | 状态码 | code | field | message（改后） |
|---|---|---|---|---|
| name 冲突（`ErrDuplicateName`） | 409 | CONFLICT | name | `代理名称已存在，请改用其它名称`（不变） |
| (type,remote_port) 冲突（`ErrDuplicateRemotePort`，**新**） | 422 | CONFLICT | remotePort | `该类型下远程端口已被占用，请改用其它端口` |
| storage 校验类（`requires`/`must not`/`invalid`） | 422 | VALIDATION_FAILED | "" | `代理配置校验失败`（固定中文，**不再透传英文**） |
| 其它（兜底） | 500 | INTERNAL | "" | `保存失败`（不变，原始 error 进日志） |
| 版本冲突 / NotFound | 409 / 404 | — | — | 不变 |

错误信封沿用现有 `writeError(w, status, code, message, field)` / `ErrorBody`。

## 6. 流程（请求穿过新代码）

```
POST/PUT /api/v1/proxies
  → handlers_proxies.go createProxy/updateProxy
    → storage.UpsertProxy(ctx, p)
        insert/update Exec → 驱动返回 UNIQUE 违规 error
          ├ isDuplicateNameError(err)?        → return ErrDuplicateName
          ├ isDuplicateRemotePortError(err)?  → return ErrDuplicateRemotePort   ← 新增（在 name 之后）
          └ else                              → return fmt.Errorf("...: %w", err)
    → h.mapProxyWriteError(w, err)
        errors.Is(ErrNotFound)            → 404
        errors.Is(ErrVersionConflict)     → 409 version
        errors.Is(ErrDuplicateName)       → 409 name
        errors.Is(ErrDuplicateRemotePort) → 422 remotePort 固定中文            ← 新增（在 name 之后）
        validation 文本命中               → 422 固定中文（不透传英文）          ← 改文案
        else                              → h.writeInternalError 500 固定文案
```

判定顺序：name 在 remote_port 之前（storage 与 handler 两层均如此）。因 sqlite 单次违规只报一个约束，两者互斥，顺序不产生歧义；保持与既有 name 优先一致即可。

## 7. Reuse audit

| 需要 | 现有代码 | 文件路径 | 决策 |
|---|---|---|---|
| 错误文本→sentinel 判定助手范式 | `isDuplicateNameError` | `internal/storage/proxies.go:329-336` | 1:1 对称复刻为 `isDuplicateRemotePortError` |
| 导出 sentinel 定义范式 | `ErrDuplicateName` | `internal/storage/store.go:48-53` | 紧邻新增 `ErrDuplicateRemotePort` + 同款注释 |
| handler sentinel 映射范式 | `errors.Is(err, storage.ErrDuplicateName)` 分支 | `internal/httpapi/handlers_proxies.go:242-245` | 之后新增 `ErrDuplicateRemotePort` 分支 |
| 500 兜底固定文案 + 原始 error 进日志 | `h.writeInternalError` | `internal/httpapi/handlers_proc.go`（T-055） | 复用其"不泄露内部文本"原则；validation 块改固定中文与之一致 |
| 错误信封写出 | `writeError` | `internal/httpapi/`（既有） | 复用 |
| storage 真冲突端到端测试范式 | `TestCreateProxy_DuplicateTypeRemotePort_Returns422` | `internal/httpapi/handlers_proxies_test.go:54-93` | 维持并补 message 断言 |
| 助手表驱动直测范式 | `TestIsDuplicateNameError_DirectChecks` | `internal/storage/proxies_test.go:123-148` | 复刻为 remote_port 版 |

无新增依赖。`errors`/`strings`/`fmt` 均已 import。

## 8. 风险分析

1. **风险**：`isDuplicateRemotePortError` 误把 name 冲突文本判为 true（若驱动文本含 `proxies.remote_port` 子串但实为 name 冲突）。
   - **缓解**：sqlite name 冲突文本严格为 `proxies.name`，不含 `remote_port`；组合冲突文本严格为 `proxies.type, proxies.remote_port`，含 `proxies.remote_port` 而不含独立 `proxies.name`。AC-3/AC-4 直测覆盖正负向，含 name 冲突文本→false 用例。
2. **风险**：删除 handler 字符串匹配块后，某个**未被任一 sentinel 覆盖**的真 unique 冲突落到 500（行为退化）。
   - **缓解**：proxies 表仅两处 UNIQUE 约束（name 列、(type,remote_port) 索引），两个 sentinel 已穷尽覆盖。`TestCreateProxy_DuplicateTypeRemotePort_Returns422` 走真 storage 验证组合冲突仍 422 而非 500；若未来新增 UNIQUE 约束，需同步新 sentinel（记 OOS）。
3. **风险**：validation 块文案中文化破坏现存 `TestMapProxyWriteError_Validation_Preserved`（断言透传 `must be 1-65535`）。
   - **缓解**：该测试**必须同步更新**为断言固定中文 + 响应体不含原始英文。已在 §2 列入受影响测试、AC-8 明确。这是预期的、受控的测试更新（非"删测试过闸门"）。
4. **风险**：分区归属——`scripts/baseline.json` 不在任何 dev-* 的 owned-paths 明列。
   - **缓解**：见 §11，本设计显式把 `scripts/baseline.json` 划给 `dev-backend`（它拥有 `scripts/verify_all.*` 且为最后落地分区），避免 BLOCKED ON PARTITION。
5. **风险**：storage 改动属 dev-db、handler 改动属 dev-backend，跨分区可能漏 storage 真冲突端到端测试（在 httpapi 包，归 dev-backend）。
   - **缓解**：§11 明确 dispatch 顺序 dev-db → dev-backend；dev-backend 落地后跑全量 verify_all 覆盖两包测试。

## 9. 迁移 / 上线计划

- **向后兼容**：对外 API 状态码/错误码/field 名不变，前端无需改动；仅 422/validation 的 `message` 由英文/混合文案变为固定中文（前端按 field/code 展示，不依赖 message 原文）。
- **无 schema 迁移**，无数据回填，无 feature flag。
- **回滚**：纯代码改动，`git revert` 即可；无持久化副作用。

## 10. 范围外澄清（设计边界）

- 不处理 http/https 类型（remote_port 为 NULL，部分索引对 NULL 不去重）的冲突——本就不触发该路径。
- 不改 `UpsertProxy` 事务/版本逻辑。
- 不为"未来可能新增的第三个 UNIQUE 约束"预留通用映射框架（YAGNI；新约束时按本范式补对应 sentinel）。
- 不改 `writeInternalError` 本身。

## 11. Partition assignment（分区已存在 dev-db/dev-backend/dev-frontend，必填）

| 文件 | 分区 | 新建/编辑 | 依赖 |
|---|---|---|---|
| `internal/storage/store.go` | dev-db | edit（加 `ErrDuplicateRemotePort`） | — |
| `internal/storage/proxies.go` | dev-db | edit（加助手 + 两处 sentinel 返回） | — |
| `internal/storage/proxies_test.go` | dev-db | edit（升级 + 新增用例） | 依赖 proxies.go |
| `internal/storage/qa_t007_adversarial_test.go` | dev-db | edit（可选补 remote_port 对抗行） | 依赖 proxies.go |
| `internal/httpapi/handlers_proxies.go` | dev-backend | edit（sentinel 分支 + 删字符串块 + 文案） | 依赖 storage 导出 sentinel |
| `internal/httpapi/handlers_hygiene_test.go` | dev-backend | edit（改 validation 断言 + 加 remote_port 映射用例） | 依赖 handlers_proxies.go |
| `internal/httpapi/handlers_proxies_test.go` | dev-backend | edit（补 message 断言） | 依赖 handlers_proxies.go |
| `scripts/baseline.json` | dev-backend | edit（bump 计数；本设计显式归此分区） | 依赖全部测试落地 |
| `docs/dev-map.md` | dev-backend | edit（轻量补 export 符号，若需） | — |

## Dispatch order

1. **dev-db**（storage sentinel + 助手 + storage 测试，自包含可独立 verify storage 包）
2. **dev-backend**（handler 映射 + handler 测试 + baseline bump + 全量 verify_all）

## Parallelism

无——严格串行。dev-backend 的 handler `errors.Is(storage.ErrDuplicateRemotePort)` 依赖 dev-db 导出的 sentinel，必须 dev-db 先落地编译通过后再派 dev-backend。

## 12. 裁决

**READY** —— 设计完整、可直接实现，无需进一步设计决策。分区与 dispatch 顺序明确。
