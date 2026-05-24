# 03 — Gate Review · T-037 proxy-rules-simplify-and-port-fix

> 模式：`full` · 上游：[01](./01_REQUIREMENT_ANALYSIS.md) READY · [02](./02_SOLUTION_DESIGN.md) READY
> Stage 3 输出（Gate Reviewer → PM Orchestrator）。独立审查，不修改上游。

## 1. 8-Dimension Audit

| # | Dimension | Result | 一句话理由 |
|---|---|---|---|
| 1 | Requirement completeness | **PASS** | 01 §2 B-1 ~ B-7 共 27 条 in-scope 行为，每条对应明确文件路径与可断言行为；§4 boundary 表 8 项覆盖典型回归；§8 全部 6 个歧义已 PM-DECIDED 无 BLOCKED |
| 2 | Design completeness | **PASS** | 02 §2 affected modules 表 23 行覆盖前/后/spec/守门 4 类；§6 流程对比直接论证了 10022→223 bug 的修复路径；§11 dispatch order 5 步含 verify_all 加 step 最后做的反咬保护 |
| 3 | Reuse correctness | **PASS** | 02 §7 reuse audit 7 行均与代码实证一致（已抽验 `useProxyForm` / `usePortPresets` / `extractErrorMessage` / `storage.UpsertProxy` 文件存在与函数签名稳定）；T-032 单向数据流契约被显式保护 |
| 4 | Risk coverage | **PASS** | 02 §8 R-1 ~ R-8 覆盖：编译残留、import 残留、OpenAPI 残留、升级 UX、bug 假根因、storage symbol 残留、e2e fixture 检查、verify_all 双实现对账；本 GR §4 抽验了 R-7（e2e fixtures grep `batch\|probe` 命中 0，风险事实上不存在但保留为防御） |
| 5 | Migration safety | **PASS** | 02 §9 明示无 DB migration；revert-一-commit 即整段回滚；单 binary release 模型不引入 cross-version 兼容期；老前端击删端点 chi 默认 404（OOS-7） |
| 6 | Boundary handling | **PASS** | 01 §4 表覆盖 (type, remotePort) 冲突 / 升级期既有折叠规则 / 端口边界 1/65535 / 老前端老端点；02 §6 流程对比锚定单条新增 remotePort=10022 的字面渲染断言 |
| 7 | Test feasibility | **PASS** | 01 §5 AC-1 ~ AC-8 每条均可由 grep / Glob / npm/go test / Playwright e2e 自动断言；AC-5 (10022 字面渲染) + AC-6 (单条 CRUD 测试 100%) 为 adversarial 锚点；02 §12-3 提供 verify_all H.1 step 伪代码 |
| 8 | Out-of-scope clarity | **PASS** | 01 §3 OOS-1 ~ OOS-7 + 02 §10 OOS-tech-1 ~ OOS-tech-6 双层 OOS；显式不动 schema / NInputNumber / usePortPresets / FirewallHint / 既有 verify_all step / G.* ID 冲突。开发者过度构建空间几乎为零 |

## 2. Findings

无 WARN / FAIL。本任务设计极简（纯删除 + 守门 step），8 维度均 PASS。

下列 4 条为**信息性**条件（不阻塞 stage 4 启动，但建议 developer 在自然顺手时一并消化，与 T-035 同款节奏 / insight L41）：

### C-1（INFO，01 §5 AC-1 / 02 §12-3）
verify_all H.1 step 设计的禁词模式表（17 个 token）需要在 Bash 实现中合并成一个 alternation 正则避免 N 次 git grep 重复 IO 开销。Developer 实现时 Bash 形态推荐：

```bash
git grep -nE '\b(batchMode|portsExpr|apiBatchCreate|batchProxies|UpsertProxiesTx|apiProbePorts|probePorts|useProxyGrouping|groupProxiesByPrefix|BatchProxiesRequest|BatchProxiesResponse|PortProbeRequest|PortProbeResult|PortProbeResponse|ErrDuplicateTcpRemote|isDuplicateTcpRemoteError|internal/portrange)\b' \
    -- 'web/src/**' 'internal/**' 'openapi.yaml' \
    ':(exclude)*_archived/*'
```

PowerShell 同款采用 `Select-String -Pattern '<合并正则>'` 单次扫描；二者实现差异需走 insight L26 双实现对账。

### C-2（INFO，02 §13）
verify_all G.* ID 冲突属"历史遗留 + 不动"决策。Developer **不要**在本任务里顺手"修"G.* 命名——会扩大改动面、增加 reviewer 负担、违背"删除-only"任务定位。

### C-3（INFO，01 §B-3.6）
`storage.go` 删除 `ErrDuplicateTcpRemote` 哨兵 = 删一行 `var` 块条目 + 注释。**注意**：单条 `mapProxyWriteError`（handlers_proxies.go:425）的兜底 `strings.Contains(low, "unique")` 分支仍然正确路由 `(type, remote_port)` 冲突到 422+field=remotePort。Developer 实现后建议**额外**手工跑 `go test ./internal/httpapi/... -run 'TcpRemote\|Conflict'` 确认无回归（既有测试 T-007 范围内）。

### C-4（INFO，02 §12-4）
"远程端口 10022 → 223" 修复证伪应在 dev 阶段**实际操作 UI**，不只跑单元测试。本任务的核心用户验证不在测试代码里——它在"用户开 Web UI、新建一条规则、看列表展示是否字面 10022"这件事上。Dev 自检后 QA 用 Playwright 复跑（AC-5），二级冗余。

## 3. High-probability questions during development

### Q1. 删除 `useProxyGrouping.ts` 后，Proxies.vue 表格里"启用"列对原 group row 显示的"部分启用"语义如何处理？

**Pre-answered（PM-DECIDED）**：
- 原 "部分启用" 是 group row 聚合状态（all/any/none），仅在折叠展示下有意义。
- 删除折叠 → 每条规则单独显示自己的 enabled 状态（NSwitch / NTag 二选一已有实现）。
- "本地地址"列同理：原 group 显示 "127.0.0.1:6000-6010" 形式，删除后单行直接 "127.0.0.1:6000"。
- columns 简化后**字节级与 T-018 之前的形态**对齐（T-018 之前列 = "名称 / 类型 / 本地地址 / 远程端口/域名 / 启用 / 操作"，render 全走 single row 分支）。

### Q2. 删除 `internal/portrange/` 包后，`go.mod` / `go.sum` 是否需要 `go mod tidy`？

**Pre-answered**：
- `portrange` 是 `internal/` 子包，不在 `go.mod` 的 require 列表里（internal 包是同 module 自包含）。
- 删除整个目录 → 仅当 grep 全仓无 `import "github.com/frp-easy/frp-easy/internal/portrange"` 残留时，`go build ./...` 即 PASS。
- `go mod tidy` 不必跑（无外部依赖被牵动），但跑了也无害。

### Q3. OpenAPI 删除 `BatchProxiesRequest` 等 5 个 schema 时，其它路径若曾 `$ref` 它们会编译失败吗？

**Pre-answered**：
- Stage 3 已 grep 验证：openapi.yaml 中 `BatchProxies` / `PortProbe` 引用仅来自 `/proxies/batch` 与 `/system/probe-ports` 两个 path 段自身（02 §2 spec 段）。
- 删 path + 删 schema 同步进行 → 无 dangling `$ref`。
- 如使用 `npx swagger-cli validate openapi.yaml`（项目未配 CI 时手工跑）做最终校验。

### Q4. ProxyForm 删除 batchMode 后，`defineEmits` 中的 `update:batchMode` / `update:portsExpr` 完全删除还是保留为空？

**Pre-answered**：
- **完全删除** `defineEmits` 整段（无其它 emit 留下）。同步删除 [Proxies.vue](../../../web/src/pages/Proxies.vue) 模板里 `@update:batch-mode` / `@update:ports-expr` 监听以及 `batchMode` / `portsExpr` 父组件 ref。
- 父组件 NModal title / 按钮文案改为常量（"新增规则" / "编辑规则" / "保存"），不再随 batchMode 切换。

### Q5. 删除 e2e 测试中关于批量 / 探测的 spec？

**Pre-answered**：
- Stage 3 已 grep 验证：`web/tests/e2e/**/*.ts` 内对 `batch` / `probe` / `portsExpr` 等的命中数为 **0**（既有 e2e 仅覆盖 setup / auth / dashboard 三 spec，不含代理规则深度测试）。
- 因此**无需删除任何 e2e 文件**。本任务的 e2e adversarial 新增（AC-5）应**新增** 1 个 spec 文件 `web/tests/e2e/04-proxies.spec.ts` 或就近放入既有 03-dashboard 的扩展，专门断言"新建单条 tcp 规则 remotePort=10022 → 列表字面显示 10022"。Dev 阶段决定具体落点。

## 4. Conditions for stage 4

无强制 conditions（本任务设计极简、风险低）。建议条件（developer 在自然顺手时消化）：

- **建议-1**：C-1 的 Bash 合并正则模式，避免 17 次 git grep 串行。
- **建议-2**：C-3 的额外 `go test -run 'TcpRemote\|Conflict'` 自检（5 秒成本，价值是隔离风险）。
- **建议-3**：C-4 的"用户实际打开 UI 复现"二级证伪（这是用户报告的 bug；只跑测试代码不足以证明修复）。

## 5. Verdict

**APPROVED FOR DEVELOPMENT**

- 8 维度审计全 PASS；无 WARN / FAIL。
- 4 条 informational 条件（C-1 ~ C-4）不阻塞 stage 4，是为 developer 节省 stage 5 来回的友好提示。
- 5 个高概率开发问题已预答（§3 Q1-Q5），developer 无需中途回查上游。
- 与 01 PM-DECIDED 6 项决策、02 OOS 13 项边界、insight-index 关键条目（L26 双实现对账 / L28 v-model OOM / L29 vitest mock 模式 / L42 archive-task regex）全部对齐。

→ Stage 4 (Developer) 可启动。建议 dispatch order 严格按 02 §11 五步。
