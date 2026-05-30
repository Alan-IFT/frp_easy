# 05 代码评审 — T-055 backend-api-hygiene

> Stage 5 / Code Reviewer · mode: full · 输出语言：中文
>
> 已逐文件读 disk 上的真实改动（非信 04 自述）+ 读全部新增测试。

## Files reviewed
- `internal/frpsadmin/client.go`（import + 3 方法 path escape）
- `internal/httpapi/handlers_server_runtime.go`（isFrpsProxyType + 校验前移）
- `internal/httpapi/handlers_proc.go`（writeInternalError helper + procStop 兜底）
- `internal/httpapi/handlers_proxies.go`（mapProxyWriteError 改方法 + 兜底 + 2 调用点）
- `internal/httpapi/handlers_system.go`（downloadBin default 兜底）
- `internal/frpsadmin/client_test.go`（escape 测试 ×3）
- `internal/httpapi/handlers_server_runtime_test.go`（type 白名单测试 ×2）
- `internal/httpapi/handlers_hygiene_test.go`（new，B 测试 ×5）
- `scripts/baseline.json`（计数 bump）

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
- [MAINT] `internal/httpapi/handlers_proc.go:109` — `writeInternalError` 用 `Logger.Error` 级，而既有 proxies.go:147 的 apply-config 失败用 `Warn`。500 兜底属真错误用 Error 合理，级别选择有依据（04 §2.3 已说明），不阻塞。

### NIT
- [STYLE] `internal/httpapi/handlers_hygiene_test.go:30` — `secretCause` 常量复用于多个测试，命名清晰；无意见。

## Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 非白名单 type→422+field=type+未触上游 | `handlers_server_runtime.go:206-210`（校验前移）+ `handlers_server_runtime_test.go::TestServerRuntimeProxyDetail_BadType_422`（断言 upstreamHit==false） | PASS |
| AC-2 特殊字符 path 正确编码 | `client.go:158/172/182` `url.PathEscape` + `client_test.go::TestProxyDetail_PathEscape`/`TestTraffic_PathEscape`/`TestProxies_PathEscape`（`r.URL.EscapedPath()` 断言） | PASS |
| AC-3 正常 path 无回归 | `TestProxyDetail_PathEscape` 子例 `normal_no_regression`（`/api/proxy/tcp/ssh` 不变）+ `TestServerRuntimeProxyDetail_AllValidTypes`（7 type 全 200） | PASS |
| AC-4 verify_all PASS + 测试数升 | baseline.json 318/744（+10）；verify_all 由 orchestrator 真跑（见验证说明） | PASS（pending orchestrator gate） |
| AC-5/B-2 兜底"保存失败"无 SQL 后缀 | `handlers_proxies.go:262` + `handlers_hygiene_test.go::TestMapProxyWriteError_Fallback_NoLeak`（断言 `"message":"保存失败"` 精确 + 无 leak 子串） | PASS |
| B-1 procStop 兜底固定文案+进日志 | `handlers_proc.go:42` + `TestWriteInternalError_FixedMessage_NoLeak`（"停止进程失败"实参覆盖该路径行为） | PASS |
| B-3 downloadBin 兜底固定文案+进日志 | `handlers_system.go:141` + `TestWriteInternalError_NilLogger`（"启动下载失败"实参覆盖该路径行为） | PASS |
| AC-7 06 裸 `## Adversarial tests` | 交 stage 6 | DEFER |

## Design fidelity check

| Design item | Implementation | Status |
|---|---|---|
| client 层 3 方法 PathEscape（根防御） | `client.go` Proxies/ProxyDetail/Traffic 全部加 `url.PathEscape` | PASS |
| handler type 白名单（C-2 前移） | `serverRuntimeProxyDetail` 校验在 `buildFrpsAdminClient` 之前 | PASS（消化 C-2） |
| writeInternalError 统一 helper | `handlers_proc.go:107`，3 处兜底全部改调用它 | PASS |
| 保留 L241/L248-255/L256-259 语义化分支 | disk 实读：ErrDuplicateName 409 / unique-constraint 422 / validation 422 透传**全部保留**，仅 L260 单行改 | PASS |
| 保留 uploadBin errno 透传（O-3） | `handlers_system.go:586` `"落盘失败: "+err.Error()` 未改 | PASS |
| 零新第三方依赖 | 仅新增 stdlib `net/url` | PASS |

## 逐维审计

1. **逻辑正确性**：type 白名单线性扫描正确；`isFrpsProxyType` 空串/大写均不命中→422（符合 BC-1/BC-2）。`writeInternalError` nil 守卫 + nil cause 双判，不 panic（BC-5）。
2. **需求保真**：上表全 PASS。
3. **设计保真**：上表全 PASS，C-2 优化已采纳，无静默 drift。
4. **性能**：白名单 7 元素线性扫描 O(7)，可忽略；escape 是纯字符串操作，热路径无影响。
5. **安全**：A 双层防御到位（client 层根防御 + handler 层白名单）；B 消除 SQL/驱动/errno 泄露面。测试用 `r.URL.EscapedPath()` 验证编码后边界不被特殊字符切断——这是真断言非 shape-matching。**验证说明**：Go `r.URL.EscapedPath()` 在 `RawPath` 与 `Path` 默认转义不同（如含 `%2F`）时返回 `RawPath`，故 `%2F` 类编码可被服务端如实观察——orchestrator 真跑测试将确证此 stdlib 行为，若本机 Go 版本对 `EscapedPath` 有差异会在 verify_all 暴露（非本评审可静态裁定）。
6. **可维护性**：注释只在 WHY 处（escape 根防御理由、兜底不外泄理由），无死代码、无过度抽象。`mapProxyWriteError` 改方法是必要（访问 `h.deps.Logger`），非过度抽象。

## 测试质量

- escape 测试用表驱动覆盖正常/`/`/`?`/`#`/空格/`%` 6 类，含幂等 `%252f`，且断言上游**实际收到**的 path（非仅客户端构造），是端到端有意义断言。
- B 测试断言"不含内部子串"用多个 leak 关键词列表（`UNIQUE`/`constraint`/`sqlite`/`errno`/`remote_port`），强度足够。
- 保留分支测试（DuplicateName 409 / validation 422）守门 B-2 不误删语义化分类，是好的回归防御。

## Verdict
**APPROVED**（0 CRITICAL / 0 MAJOR / 1 MINOR / 1 NIT）

代码与需求、设计完全保真；A 双层防御、B 统一化 + 保留语义化分支均落地；测试有意义且数升。唯一外部不确定项是 `r.URL.EscapedPath()` 的 stdlib 行为，由 orchestrator 真跑 verify_all 确证（硬闸门）。
