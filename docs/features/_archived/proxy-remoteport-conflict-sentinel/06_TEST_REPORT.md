# 06 测试报告 — T-059 proxy-remoteport-conflict-sentinel

> 阶段 6 / QA Tester · mode: full · 中文
> 诚实声明：本 QA 会话工具集仅 Read/Write/Edit/Glob/Grep，**无 Bash/PowerShell**，无法真跑 `go test` / `scripts/verify_all`。按 QA 铁律"无工具证据不声称结果"（insight L31），全量真跑标 **PENDING** 交 orchestrator Bash 会话核对；下方给出**确定性执行规格**（每用例的预期 OLD vs NEW，可由 Go 子串匹配语义逐 fixture 推导，无随机/IO/并发），结果偏离即回退信号。

## Test plan（每条 AC 至少一个测试）

| 验收标准 | 测试用例 | 文件 |
|---|---|---|
| AC-2 storage 插入返回 ErrDuplicateRemotePort | `TestUpsertProxy_DuplicateTypeRemotePortReturnsSentinel`（正向 errors.Is(ErrDuplicateRemotePort) + 负向 NOT ErrDuplicateName） | `internal/storage/proxies_test.go` |
| AC-3 助手正向 | `TestIsDuplicateRemotePortError_DirectChecks`（normal / wrapped+errno → true） | `internal/storage/proxies_test.go` |
| AC-4 助手负向 | 同上（nil / name 冲突 / 无关 / 缺 UNIQUE 前缀 → false） | `internal/storage/proxies_test.go` |
| AC-5 storage 更新路径 | `TestUpsertProxy_UpdateToDuplicateRemotePortReturnsSentinel` | `internal/storage/proxies_test.go` |
| AC-6 handler 映射 422+remotePort+固定中文+不泄露 | `TestMapProxyWriteError_DuplicateRemotePort` | `internal/httpapi/handlers_hygiene_test.go` |
| AC-7 name 不退化 409 | `TestMapProxyWriteError_DuplicateName_Preserved`（既有，未破坏） | `internal/httpapi/handlers_hygiene_test.go` |
| AC-8 validation 改固定中文 + 同步更新测试 | `TestMapProxyWriteError_Validation_FixedMessage_NoLeak`（C-1 改名+改断言） | `internal/httpapi/handlers_hygiene_test.go` |
| AC-9 端到端 422 不退化 | `TestCreateProxy_DuplicateTypeRemotePort_Returns422`（真 storage 冲突，补 message 不含英文） | `internal/httpapi/handlers_proxies_test.go` |
| AC-2/AC-5 真 DB 端到端 | `TestAdversarial_AC6_RealDBDuplicateNameSentinel`（强化 p3 → 正向 ErrDuplicateRemotePort） | `internal/storage/qa_t007_adversarial_test.go` |

## Boundary tests added

- nil error → 助手返回 false（AC-4 直测）。
- 包装错误（`fmt.Errorf(...: %w)` + `(2067)` errno 后缀）→ 助手仍命中（wrapped 直测）。
- 互斥边界：name 冲突文本 → `isDuplicateRemotePortError`=false；组合文本 → `isDuplicateNameError`=false（双向直测覆盖）。
- 大小写边界：lowercase 文本 → false（记录已知局限，与 name 版同款，对抗段证伪）。
- UPDATE 路径改 remotePort 触发组合冲突（区别于 INSERT 路径）。
- http/https 类型 remote_port=NULL 不触发组合冲突——既有行为，部分索引对 NULL 不去重，不在新增覆盖（设计 §10 OOS）。

## 确定性执行规格（OLD 行为 vs NEW 行为；orchestrator 真跑逐项核对）

子串匹配是纯确定性逻辑，无 IO/随机/并发，预期可逐 fixture 推导：

### storage 层（go test ./internal/storage/...）

| 用例 | 输入 error 文本 | isDuplicateNameError | isDuplicateRemotePortError | UpsertProxy 返回 |
|---|---|---|---|---|
| name 冲突 | `UNIQUE constraint failed: proxies.name` | true | false | ErrDuplicateName |
| 组合冲突（NEW 路径） | `UNIQUE constraint failed: proxies.type, proxies.remote_port` | false | **true** | **ErrDuplicateRemotePort**（OLD: 裸 fmt.Errorf wrapped） |
| 组合冲突 wrapped+errno | `storage.UpsertProxy insert: UNIQUE...remote_port (2067)` | false | true | ErrDuplicateRemotePort |
| lowercase 组合 | `unique constraint failed: ...remote_port` | false | false | wrapped（已知大小写局限） |
| nil | — | false | false | — |

推导依据：`isDuplicateRemotePortError` = `Contains(s,"UNIQUE constraint failed") && Contains(s,"proxies.remote_port")`。组合文本含两子串→true；name 文本含 `proxies.name` 不含 `proxies.remote_port`→false；lowercase 不含大写 `UNIQUE constraint failed`→false。

### handler 层（go test ./internal/httpapi/...）

| 用例 | 输入 | NEW 响应 | OLD 响应（被本任务改变） |
|---|---|---|---|
| ErrDuplicateRemotePort | sentinel | 422 / CONFLICT / field=remotePort / "该类型下远程端口已被占用，请改用其它端口" / 体内无 SQL 英文 | 此前组合冲突走 wrapped→handler strings.Contains 命中 unique→422 / "字段冲突：可能 name 重复或 (type,remotePort) 冲突" |
| ErrDuplicateName | sentinel | 409 / field=name / 既有中文 | 同（不变） |
| validation（`...must not set customDomains`） | error | 422 / VALIDATION_FAILED / "代理配置校验失败" / 体内无英文 / 原文进 logger.Warn | 此前透传英文原文（must not set...）到响应体 |
| 端到端 POST 组合冲突 | 真 storage | 422 / field=remotePort / 体内无 `字段冲突`/SQL 英文 | 此前 422 / field=remotePort（field 不变，但 message 与判定来源变） |
| 兜底裸 SQL error | error | 500 / "保存失败" / 不泄露 | 同（不变） |

### 计数闸门（baseline.json B.4）

| 指标 | OLD | NEW | 推导 |
|---|---|---|---|
| go_tests | 318 | 322 | 净增 4 顶层 Test（dev-db 3 + dev-backend 1；改名/强化不计净增） |
| test_count | 799 | 803 | 同步 +4 |
| frontend_tests | 481 | 481 | 未碰前端 |

`go test -list '.*' ./...` 顶层 Test* 计数应 = 322（Go 包）。B.4 闸门"测试数 ≥ 基线"应 PASS（净增非降）。

## Adversarial tests

> 本段为裸标题（无前缀），满足 verify_all E.6 与交付闸门。每条对抗用例对应 AC，先写失败假设再证伪。

**AT-1（核心反向论证：驱动错误文本变化不再影响 handler 分类）**
- 失败假设："若 sentinel 化不彻底，handler 仍隐性依赖驱动文本，则未来 modernc.org/sqlite 改错误文本（如列序变为 `proxies.remote_port, proxies.type` 或加前缀）会让 (type,remote_port) 冲突静默错分到 500。"
- 独立证伪（不靠 dev 测试代码，从 AC 重推）：sentinel 化后，**handler `mapProxyWriteError` 中已无任何对驱动错误文本的 `strings.Contains`**（grep `handlers_proxies.go` 确认 unique/constraint/remote_port 字符串块已删；validation 块判定的是 storage **自己生成**的固定英文业务文案，非驱动文本）。handler 仅 `errors.Is(err, storage.ErrDuplicateRemotePort)`。故无论驱动文本如何变化，只要 storage 助手仍能识别（识别失败则 storage 直测 `TestIsDuplicateRemotePortError_DirectChecks` / `TestAdversarial_T059_RemotePortErrorTextVariants` 立即红），handler 分类逻辑零改动、零受影响。**文本依赖被收敛到 storage 单点，并由直测护栏锁死。** 结论：实现存活——handler 层对驱动文本的脆弱依赖已彻底消除。
- 工具证据：`TestAdversarial_T059_RemotePortErrorTextVariants` 把"文本变体→分类"做成可执行护栏（normal/wrapped+errno→true，lowercase→false 记录局限，name 文本→false 互斥）；orchestrator 真跑该 Go 测试即为执行证据。预期：PASS。

**AT-2（AC-2/AC-5 互斥证伪）**
- 失败假设："`isDuplicateRemotePortError` 会把 name 冲突误判为 true（若实现错写成只匹配 `remote_port`）。"
- 独立证伪：name 冲突文本 `UNIQUE constraint failed: proxies.name` **不含子串** `proxies.remote_port`（逐字符核对），故助手返回 false。`TestIsDuplicateRemotePortError_DirectChecks` 的 `name-conflict-not-remote-port` 用例 + `TestAdversarial_T059_...` 的同款用例双重锁死。结论：实现存活——互斥成立。预期：PASS。

**AT-3（AC-6/AC-8 不泄露证伪）**
- 失败假设："422/validation 响应体仍泄露 SQL 或 storage 生成的英文文本（信息泄露）。"
- 独立证伪：`TestMapProxyWriteError_DuplicateRemotePort` 对响应体断言 leak 黑名单 `{UNIQUE, constraint, proxies., remote_port, duplicate}` 全不出现；`TestMapProxyWriteError_Validation_FixedMessage_NoLeak` 断言 `{must not set, customDomains, UpsertProxy, udp proxy}` 全不出现 + 固定中文出现 + 原文进 logger。两固定中文文案均为纯中文常量，不含上述英文子串。结论：实现存活——无泄露。预期：PASS。

**AT-4（AC-9 回归证伪：删字符串块后组合冲突不退化到 500）**
- 失败假设："删除 handler unique 字符串块后，某真 unique 冲突无 sentinel 覆盖，落到 500。"
- 独立证伪：proxies 表 DDL（0001）仅两处 UNIQUE 约束（name 列 + idx_proxies_tcp_remote）；两 sentinel 穷尽覆盖。`TestCreateProxy_DuplicateTypeRemotePort_Returns422` 走**真 storage** INSERT 触发组合冲突，断言 422（非 500）+ field=remotePort。结论：实现存活——无退化（未来若新增第三个 UNIQUE 约束需补对应 sentinel，已记 OOS）。预期：PASS。

## verify_all result

- **PENDING（QA 会话无 Bash/PowerShell，未能真跑）**。交 orchestrator Bash 会话执行 `bash scripts/verify_all.sh` 作交付硬闸门。
- 预期（确定性推导）：go build/vet 绿（import 完整）；go test 绿（净增 4 全 PASS，按上方执行规格）；前端不受影响；B.4 计数闸门 PASS（322/803 = baseline）；E.6 因 06 含裸 `## Adversarial tests` 段绿。
- Total tests: 799 → **803**；New tests added（净增顶层 Test）: **4**；Baseline updated: yes（go_tests 318→322 / test_count 799→803 / version 24→25）。

## Defects found

无（基于静态确定性推导；真跑若现偏离即回退信号交 PM 路由回 developer）。

## Stability

- 本任务新增/改动测试均为**确定性**：纯子串匹配 + 真 sqlite 同步 INSERT/UPDATE（无 sleep、无并发、无网络、无时间依赖），无 flake 来源。
- 真跑稳定性（3× 重复）交 orchestrator 会话；预期零 flake。

## Verdict

**APPROVED FOR DELIVERY**（条件：orchestrator Bash 会话真跑 `verify_all` PASS 作硬闸门）。

所有 AC 均有测试覆盖；对抗段证明 handler 对驱动文本的脆弱依赖已彻底消除（核心目标达成）；安全性正向改善（不泄露）；无设计漂移；计数只升不降。唯一外部依赖：全量真跑（QA 无 Bash，已诚实标 PENDING + 提供确定性执行规格供核对）。
