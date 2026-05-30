# 03 闸门评审 — T-059 proxy-remoteport-conflict-sentinel

> 阶段 3 / Gate Reviewer · mode: full · 中文
> 角色：开发前最后检查点。独立核实，不盲信上游。

## 独立代码核实记录

逐项 grep/read 验证设计 §7 Reuse audit 引用的符号真实存在：

- `internal/storage/store.go:48-53` `ErrDuplicateName` 定义 + 注释 —— 已读，存在。
- `internal/storage/proxies.go:329-336` `isDuplicateNameError`（匹配 `UNIQUE constraint failed`+`proxies.name`）—— 已读，存在。
- `internal/storage/proxies.go:122-127`（insert）/ `172-176`（update）`isDuplicateNameError`→`ErrDuplicateName` 后接 `fmt.Errorf(...: %w)` —— 已读，存在，组合冲突现确实落 fmt.Errorf。
- `internal/storage/sqlmigrations/0001_init.up.sql:46` `CREATE UNIQUE INDEX idx_proxies_tcp_remote ON proxies(type, remote_port)` —— 已读，存在。
- `internal/httpapi/handlers_proxies.go:242-245` `errors.Is(err, storage.ErrDuplicateName)`→409 —— 已读，存在。
- `internal/httpapi/handlers_proxies.go:246-256` 待删的 `strings.Contains(low,"unique"/"constraint"/"remote_port")`→422 块 —— 已读，存在，确为待消除反模式。
- `internal/httpapi/handlers_proxies.go:257-260` validation 透传块（`requires`/`must not`/`invalid`→透传 msg）—— 已读，存在。
- `internal/httpapi/handlers_proc.go` `writeInternalError`（T-055）—— 经 grep 确认被 mapProxyWriteError L262 调用。
- **测试依赖核实**：
  - `internal/storage/proxies_test.go:51-88` 现弱断言（L81-82 NOT ErrDuplicateName + L85 含 unique）—— 已读，确为待升级。
  - `internal/storage/qa_t007_adversarial_test.go:25` (type,remote_port) 行断言 `isDuplicateNameError==false` —— 已读，升级后仍成立。
  - `internal/httpapi/handlers_proxies_test.go:54-93` 端到端 422+field=remotePort（未断言 message）—— 已读，改后仍 422。
  - `internal/httpapi/handlers_hygiene_test.go:123-134` `TestMapProxyWriteError_Validation_Preserved` **断言透传 `must be 1-65535`** —— 已读，**确认会被任务点 3 破坏，设计 §2/§8/AC-8 已列入同步更新，未遗漏**。

设计中无"引用了不存在符号"的情形。insight-index 无任何条目与本设计假设矛盾（L33 writeInternalError 范式恰被本设计正确复用）。

## 1. 审计清单（8 维度）

| # | 维度 | 结论 | 一句话理由 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | 9 条范围内行为均可测、无歧义词；AC-1~12 每条可验证（编译/单测/响应体子串断言）。 |
| 2 | 设计完整性 | PASS | 设计 §3/§6 覆盖全部 9 条行为：sentinel+助手+两路径返回+handler 三处改动+测试同步+baseline，无遗漏。 |
| 3 | 复用正确性 | PASS | §7 Reuse audit 七项全部经独立 read 核实存在且可对称复用；无新依赖；`errors/strings/fmt` 已 import。 |
| 4 | 风险覆盖 | PASS | §8 列 5 风险均为真实风险（误判、删块后退化、validation 测试破坏、baseline 分区归属、跨分区漏测），各带可执行缓解。 |
| 5 | 迁移安全 | PASS | 无 schema 迁移、无数据回填；纯代码改动 `git revert` 可回滚；不动已合并 0001。 |
| 6 | 边界处理 | PASS | 01 §4 + 02 §6 覆盖 nil/包装错误/判定顺序/驱动文本变化/NULL remote_port/响应体不含英文；name 与 remote_port 判定互斥经核实。 |
| 7 | 测试可行性 | PASS | AC-2~9 每条对应明确测试形态（storage 真冲突、助手表驱动直测、handler 映射、端到端）；现存范式 `TestIsDuplicateNameError_DirectChecks` 可 1:1 复刻。 |
| 8 | 范围外清晰度 | PASS | 01 §3 + 02 §10 明确不做 migration/前端/事务重构/通用框架（YAGNI），开发不会过度构建。 |

## 2. Findings

无 FAIL，无 WARN。

一条**强条件**（非 WARN，但开发必须执行，已在设计内明列，此处再强调防漏）：
- **C-1**：任务点 3 把 handler validation 块英文文案改固定中文，**必然破坏** `internal/httpapi/handlers_hygiene_test.go:123-134` `TestMapProxyWriteError_Validation_Preserved`（其 L131 断言响应体含 `must be 1-65535`）。dev-backend **必须**在同一改动中把该测试断言改为"断言固定中文 + 响应体不含原始英文子串"。这是**受控的预期测试更新**（属红线 3 的"PM 批准的过时断言更新"，非"删活测试过闸门"）——PM 在 PM_LOG 已显式记录批准。

## 3. 开发期高概率提问（预答）

1. **Q：`isDuplicateRemotePortError` 该匹配 `proxies.remote_port` 还是 `proxies.type, proxies.remote_port` 全串？**
   A：匹配子串 `proxies.remote_port` 即可（与 `isDuplicateNameError` 只匹配 `proxies.name` 子串对称）。组合索引违规文本含该子串；name 冲突文本不含。无需匹配完整列序串（更脆，且 sqlite 可能不保证列序顺序文本稳定）。
2. **Q：handler sentinel 分支放在哪一行？**
   A：紧接 `ErrDuplicateName`(L242-245) 之后、待删字符串块之前。删除 L246-256 整块后，sentinel 分支天然在 validation 块之前。
3. **Q：validation 固定中文用什么文案？**
   A：设计 §5 给定 `代理配置校验失败`（422 + CodeValidationFailed + field=""）。原始 error 不向前端透传；如需排障可走 logger（与 writeInternalError 同款），但本任务最小改动可只改 message 为固定中文、保留 422 透传分支结构、剥离 msg=err.Error()。dev-backend 自行决定是否额外 log（不强制，但不得把英文放进响应体）。
4. **Q：baseline.json 谁改？计数口径？**
   A：dev-backend（设计 §11 已显式划归，避免 BLOCKED ON PARTITION）。口径为 `go test -list` 实际数（与现有 baseline 一致）；只升不降。
5. **Q：storage 包能否独立 verify？**
   A：能。dev-db 落地后 `go test ./internal/storage/...` 应绿（新 sentinel + 助手 + 升级测试自包含）。dev-backend 依赖该 sentinel 导出，故 dispatch 严格 dev-db → dev-backend。

## 4. 裁决

**APPROVED**（full mode）——

设计与需求自洽、引用代码全部经独立核实存在、复用范式正确、风险覆盖充分、AC 全可测。允许进入开发阶段，须满足强条件 C-1（同步更新 `TestMapProxyWriteError_Validation_Preserved`，已 PM 批准）。

Dispatch 顺序：dev-db → dev-backend（严格串行，见设计 §11）。
