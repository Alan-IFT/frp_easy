# 02 — Solution Design · T-037 proxy-rules-simplify-and-port-fix

> 模式：`full` · 上游：[01_REQUIREMENT_ANALYSIS.md](./01_REQUIREMENT_ANALYSIS.md)（verdict=READY）
> Stage 2 输出（Solution Architect → Gate Reviewer）。

## 1. Architecture summary

本任务**削减**功能面积：在前端、API、Store、Composable、后端 handler、portrange 包、storage 批量 DAO、OpenAPI、单元测试、e2e 测试 10 个层面，删除"批量创建代理规则 / 折叠分组展示 / 自动端口探测"三类辅助能力。**不写新代码、不写新 migration、不引入新依赖**——所有改动是 delete + 局部 inline 简化。

唯一的"修复"语义来自 [Proxies.vue](../../../web/src/pages/Proxies.vue) 列定义的退化：删除 `kind === 'group'` 分支后，"远程端口/域名"列只走 `String(row.remotePort)` 单一路径——10022 → 223 bug 因展示路径物理上不再存在而被根治（详见 §6 流程对比）。

新增 1 个 verify_all 守门 step（PowerShell + Bash 对账）防止删除面在未来被静默回退。

## 2. Affected modules

### 前端（dev-frontend 分区）

| 文件 | 改动 | 范围 |
|---|---|---|
| [web/src/components/ProxyForm.vue](../../../web/src/components/ProxyForm.vue) | edit (大幅删) | 删 batchMode 开关 + portsExpr 输入 + 探测按钮 + 相关 watch/emit/expose；rules 简化 |
| [web/src/pages/Proxies.vue](../../../web/src/pages/Proxies.vue) | edit (中等删) | 按钮文案 + handleSubmit 批量分支 + columns 折叠分支 + 数据源类型 + import 清理 |
| [web/src/composables/useProxyGrouping.ts](../../../web/src/composables/useProxyGrouping.ts) | **delete file** | 整体删除 |
| [web/src/composables/\_\_tests\_\_/useProxyGrouping.spec.ts](../../../web/src/composables/__tests__/useProxyGrouping.spec.ts) | **delete file** | 整体删除 |
| [web/src/api/proxies.ts](../../../web/src/api/proxies.ts) | edit (小) | 删 apiBatchCreateProxies + 相关导入 |
| [web/src/api/system.ts](../../../web/src/api/system.ts) | edit (小) | 删 apiProbePorts + 相关导入 |
| [web/src/stores/proxies.ts](../../../web/src/stores/proxies.ts) | edit (小) | 删 batchCreate action + 相关导入 |
| [web/src/types.ts](../../../web/src/types.ts) | edit (小) | 删 BatchProxiesRequest/Response + PortProbeRequest/Result/Response 共 5 个接口 |
| [web/src/api/\_\_tests\_\_/proxies.spec.ts](../../../web/src/api/__tests__/proxies.spec.ts) | edit | 删 apiBatchCreateProxies 测试用例 |
| [web/src/api/\_\_tests\_\_/system.spec.ts](../../../web/src/api/__tests__/system.spec.ts) | edit | 删 apiProbePorts 测试用例 |

### 后端（dev-backend 分区）

| 文件 | 改动 | 范围 |
|---|---|---|
| [internal/httpapi/router.go](../../../internal/httpapi/router.go) | edit (小) | 删 2 行路由注册（batch + probe-ports） |
| [internal/httpapi/handlers_proxies.go](../../../internal/httpapi/handlers_proxies.go) | edit (大幅删) | 删 batchProxies handler + BatchProxies* types + humanizePortRangeErr + writeBatchProxiesError + batchBasenameRE + BatchProxiesMaxCount + portrange import |
| [internal/httpapi/handlers_system.go](../../../internal/httpapi/handlers_system.go) | edit (中等删) | 删 probePorts + probeOnePort + PortProbe* types + portProbeMaxCount/Timeout 常量 |
| [internal/httpapi/handlers_batch_test.go](../../../internal/httpapi/handlers_batch_test.go) | **delete file** | 整体删除 |
| [internal/httpapi/port_probe_test.go](../../../internal/httpapi/port_probe_test.go) | **delete file** | 整体删除 |
| [internal/portrange/portrange.go](../../../internal/portrange/portrange.go) | **delete file** | 整个包删除 |
| [internal/portrange/portrange_test.go](../../../internal/portrange/portrange_test.go) | **delete file** | 整体删除 |
| [internal/storage/proxies.go](../../../internal/storage/proxies.go) | edit (中等删) | 删 UpsertProxiesTx + isDuplicateTcpRemoteError |
| [internal/storage/proxies_batch_test.go](../../../internal/storage/proxies_batch_test.go) | **delete file** | 整体删除 |
| [internal/storage/store.go](../../../internal/storage/store.go) | edit (小) | 删 ErrDuplicateTcpRemote 哨兵导出 |

### Spec / Docs / 闸门

| 文件 | 改动 | 范围 |
|---|---|---|
| [openapi.yaml](../../../openapi.yaml) | edit | 删 `/api/v1/proxies/batch` 路径段 + `/api/v1/system/probe-ports` 路径段 + 5 个 schema (BatchProxiesRequest/Response, PortProbeRequest/Result/Response) |
| [docs/dev-map.md](../../../docs/dev-map.md) | edit | 删/降级 portrange、useProxyGrouping、apiBatchCreate、apiProbePorts、batchBasenameRE 相关条目 |
| [docs/architecture.html](../../../docs/architecture.html) | edit（仅在 grep 命中时） | 同步删除批量 / 探测说明 |
| [scripts/verify_all.ps1](../../../scripts/verify_all.ps1) | edit (加 step) | 新 step `H.1 "T-037 deletion surface clean"` 守门删除面 |
| [scripts/verify_all.sh](../../../scripts/verify_all.sh) | edit (加 step) | 同款 H.1 step（双实现对账，insight L26） |

**不动**：所有 `migrations/` 与 `internal/storage/sqlmigrations/` 文件（OOS-1）；`usePortPresets.ts`（OOS-5）；`FirewallHint.vue` / `PublicIpDetector.vue` / `UploadBinButton.vue`（OOS-6）。

## 3. Module decomposition

**N/A** — 本任务无新模块。所有改动是 delete 或 inline 简化。

## 4. Data model changes

**无 schema migration**。

**保留**：
- 表 `proxies` 全部列定义不变；
- `(type, remote_port)` 部分唯一索引 `idx_proxies_tcp_remote` 不变（OOS-1，§Q-2）；
- 单条 `UpsertProxy` 仍能触发并由 `mapProxyWriteError` 兜底分支返回 422，与 T-007 hardening 覆盖测试对接。

**移除（仅 Go 代码符号，DB schema 不动）**：
- `storage.ErrDuplicateTcpRemote` 哨兵导出（无调用方 → dead code）；
- `storage.UpsertProxiesTx` 批量事务函数（仅 batchProxies 调用 → dead code）；
- `storage.isDuplicateTcpRemoteError` 辅助（仅 UpsertProxiesTx 调用 → dead code）。

## 5. API contracts

### 删除端点（chi router 自动 404，不留 deprecation 桩）

```
POST /api/v1/proxies/batch         → 404 NOT FOUND（chi r.NotFound 默认）
POST /api/v1/system/probe-ports    → 404 NOT FOUND
```

### 不变端点（行为契约不动）

```
GET    /api/v1/proxies             → 200 [ProxyResponse]
POST   /api/v1/proxies             → 201 ProxyResponse / 422 / 409
PUT    /api/v1/proxies/{id}        → 200 ProxyResponse / 422 / 409
DELETE /api/v1/proxies/{id}        → 200 {ok:true} / 404
```

422 错误码契约保持（VALIDATION_FAILED + field 名）。

## 6. Sequence / flow

### 旧流程（含 group row 渲染 bug 触发路径）

```
用户在 Proxies.vue 列表上看代理规则
   ↓
proxiesStore.proxies → groupProxiesByPrefix() → GroupedProxyRow[]
   ↓
渲染列 "远程端口/域名":
   if row.kind === 'group':
       return row.portRangeText          ← 来自 compressPorts(map(p.localPort))
                                            ↑ ❌ 用 localPort 当远程端口展示
   else:
       return String(row.proxy.remotePort)
```

→ 用户实际场景：`ssh-22 (local=22, remote=22)` + `ssh-100 (local=100, remote=10022)` 同 basename "ssh" 同 type tcp → 折叠 group row → portRangeText = `"22, 100"`（**而不是用户期望的 `"22, 10022"`**）。10022 → 223 的数字字面对应取决于具体名称组合，但**展示与真实 remotePort 解耦**是确定性 bug 来源。

### 新流程（本设计）

```
用户在 Proxies.vue 列表上看代理规则
   ↓
proxiesStore.proxies → 直接作为 NDataTable 的 :data
   ↓
渲染列 "远程端口/域名":
   if row.remotePort: return String(row.remotePort)
   if row.customDomains?.length: return row.customDomains.join(', ')
   return '—'
```

→ 任何 remotePort 都被字面渲染；不存在"用 localPort 替代 remotePort"的代码路径；bug 物理上不可能复发。

### 单条新增流程（不变，作为对照）

```
用户点 "新增规则" → ProxyForm 模态框（无 batchMode 开关）
   ↓
填入 name / localPort / remotePort（NInputNumber）/ type
   ↓
点 "保存" → validate → proxyFormRef.getProxyInput() → ProxyInput{remotePort: 10022}
   ↓
proxiesStore.createProxy(input) → POST /api/v1/proxies
   ↓
buildProxyForInsert → storage.UpsertProxy → SQLite INSERT
   ↓
响应 ProxyResponse{remotePort: 10022}
   ↓
proxiesStore.proxies.push(p)
   ↓
列表渲染 → "远程端口/域名" 列字面 "10022" ✓
```

## 7. Reuse audit

| Need | Existing code | File path | Decision |
|---|---|---|---|
| 单条 Proxy CRUD（List/Create/Update/Delete） | `apiListProxies` / `apiCreateProxy` / `apiUpdateProxy` / `apiDeleteProxy` | [web/src/api/proxies.ts](../../../web/src/api/proxies.ts) | **Reuse as-is** —— 所有路径保留 |
| 单条 Proxy form 表单逻辑 | `useProxyForm` | [web/src/composables/useProxyForm.ts](../../../web/src/composables/useProxyForm.ts) | **Reuse as-is** —— T-032 单向数据流契约不动 |
| 端口预设标签快速填充 | `usePortPresets` / `PORT_PRESETS` | [web/src/composables/usePortPresets.ts](../../../web/src/composables/usePortPresets.ts) | **Reuse as-is**（保留预设，OOS-5） |
| 表格"远程端口/域名"渲染 | columns[3].render single-row 分支 | [web/src/pages/Proxies.vue](../../../web/src/pages/Proxies.vue#L267) | **Inline 化** —— 删 group 分支后只留 single 分支并提升为顶层 |
| 单条 Proxy 写盘 + 版本冲突 + name UNIQUE + (type, remotePort) UNIQUE | `storage.UpsertProxy` | [internal/storage/proxies.go:89](../../../internal/storage/proxies.go#L89) | **Reuse as-is** —— 422 兜底 + 409 路径既有 |
| ApiError 渲染 / extractErrorMessage | `extractErrorMessage` | [web/src/api/client.ts](../../../web/src/api/client.ts) | **Reuse as-is** —— 错误提示路径不变 |
| FRP TOML 渲染（含 remotePort） | `frpconf.RenderFrpc` | [internal/frpconf/render.go](../../../internal/frpconf/render.go) | **Reuse as-is** —— 不动 |

**无新增依赖。无新增模块。** 完全是删除 + inline 化。

## 8. Risk analysis

| # | 风险 | 缓解 |
|---|---|---|
| R-1 | 删除 `useProxyGrouping` 后 Proxies.vue 表格出现编译错误（类型断言 / import 残留） | Dev 阶段全文 grep `GroupedProxyRow` / `useProxyGrouping` / `groupProxiesByPrefix` 在 web/src 下命中 0 才允许 `npm run build`；verify_all H.1 守门将此类残留升级为运行时硬约束 |
| R-2 | 删除 `internal/portrange/` 包后 Go 编译报 "import path not found" 残留 | Dev 阶段全仓 grep `"github.com/.../internal/portrange"` 命中 0 才允许 `go build`；verify_all G.* 之既有 step 跑全包编译 + vet 已能捕 |
| R-3 | OpenAPI 文件删 schema 时遗漏 `$ref`，导致 OpenAPI validator 报"unresolved reference" | Dev 阶段同步删除 `/proxies/batch` 与 `/system/probe-ports` 路径段 + 5 个 schema；用 PowerShell `Select-String -Pattern 'BatchProxies\|PortProbe'` 抽全文最终命中 0 |
| R-4 | 升级现网（既有 DB 已有 ≥2 条 ssh-* tcp 折叠规则）后列表"突然散开" 让用户疑惑 | 这是**预期行为**（用户明确要求 "去掉批量"），并非回归；07_DELIVERY 章节 "用户须知" 段需要明示 |
| R-5 | 10022 → 223 bug 的"假根因"（如 NInputNumber 截断）未被覆盖 → 修复 ineffective | Stage 6 QA adversarial 用 Vitest mount 强制 `NInputNumber v-model 输入值 = 10022` → `createProxy` 拦截被传 `remotePort: 10022`；端到端 e2e 在干净 DB 上新建一条规则后断言列表渲染字符串 `"10022"`，避免单元测试盲点 |
| R-6 | 删除 `ErrDuplicateTcpRemote` 后 storage_test.go 残留引用 → 编译失败 | Dev 阶段先 grep 仅 `proxies_batch_test.go` 引用（已确认）；删除 batch test 文件后编译应清；verify_all G.1 (go vet) 兜底 |
| R-7 | T-018 引入的 `apiBatchCreateProxies` 在 e2e fixture / setup 脚本中可能被引用 | 检查 [web/tests/e2e/](../../../web/tests/e2e/) 路径下所有 .ts 文件；grep `batch\|probe` 命中需逐条评估（保留单条 CRUD fixture，删批量 fixture） |
| R-8 | verify_all H.1 新 step 在 PowerShell 与 Bash 双实现行为不一致（insight L26 教训复发） | 设计强制：PowerShell 用按行扫描 `Select-String -SimpleMatch -Path web/src/**/*` + Bash 用 `git grep -E`；二者均限定路径 `web/src/ internal/ openapi.yaml`，排除 `docs/features/_archived/`；Stage 6 ADV 拆 PS / Bash 各跑一次反向证伪（临时还原一行符号 → step FAIL → 删除符号 → step PASS） |

## 9. Migration / rollout plan

- **DB 不动**：无 migration 写盘。
- **API 删端点**：单 binary release 模型，前后端同版本随 release 发布；老前端 / curl 击删端点 → chi 默认 404（OOS-7 / Q-3）。
- **回滚**：本任务的"回滚 = 复活"=git revert，所有删除集中在一个 task；revert 一个 commit 即可恢复全部三类能力。无需 rollback migration。
- **环境兼容**：Go module + Vite 双链路无外部依赖变更；`go.sum` / `package-lock.json` 不动。
- **release notes 文案建议**（07 章节展开）：
  > 本版本简化代理规则管理：去掉了"批量创建 / 折叠分组展示 / 自动端口探测"三项辅助功能，回归"一条规则一行展示"的简单形态。原批量创建的代理规则会**自动散开**为多条独立行（数据本身不变），可通过列表上的"编辑"/"删除"按钮逐条管理。

## 10. Out-of-scope clarifications

承自 01 §3 OOS-1 ~ OOS-7，技术层补充：

- **OOS-tech-1**：不重构 `useProxyForm.ts` —— T-032 契约稳定，本任务无理由触碰。
- **OOS-tech-2**：不修改 NInputNumber 配置 —— bug 根因不在该组件（§6 已论证）。
- **OOS-tech-3**：不引入端口编辑组件库替代 NInputNumber。
- **OOS-tech-4**：不写新 spec / 不写新 README 段 —— release notes 文案够用。
- **OOS-tech-5**：不修改 verify_all 既有 step（G.* / E.* 不动）；仅追加 H.1。
- **OOS-tech-6**：不修复 verify_all 中 G.1 / G.2 ID 冲突（Go 步骤 vs T-034 reviewer 协议步骤同 ID）—— 历史遗留，记录在 §13 而非动手。

## 11. Partition assignment

项目用 single Developer 模式（`.harness/agents/developer.md` 存在；同时也有 `.harness/agents/dev-frontend.md` / `dev-backend.md` —— 本任务跨前后端 + DB-symbol 删除三分区都涉及，但**改动以"删除"为主，无新建模块、无跨分区契约协商**，按 Harness 规则 §4 单 Developer 一次接管更高效）。

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| 前端 7 文件（见 §2 前端段） | developer | edit (含 2 delete file) | — |
| 后端 11 文件（见 §2 后端段） | developer | edit (含 4 delete file) | — |
| openapi.yaml | developer | edit | 后端删除 handler 之后同步 |
| dev-map.md / architecture.html | developer | edit | 全部代码改完后最后批改 |
| scripts/verify_all.{ps1,sh} | developer | edit (加 H.1 双实现) | 全部代码改完后加，避免开发期自咬 |

### Dispatch order

单 Developer，**顺序**：
1. 前端 删除 + 编辑（npm run build / npm test PASS）
2. 后端 删除 + 编辑（go build ./... / go test ./... PASS）
3. OpenAPI 同步删（grep BatchProxies/PortProbe 命中 0）
4. dev-map / architecture.html 同步删
5. verify_all.{ps1,sh} 加 H.1（最后加，避免开发期自咬）
6. 全量 `scripts/verify_all` 跑 → 期望 PASS

### Parallelism

无 —— 单 Developer 严格顺序。

## 12. 提示给 Stage 4 Developer

1. **重要 insight 引用**：
   - L28（v-model OOM）：**不**回退 ProxyForm `initialValue` prop / `defineExpose getProxyInput()` 单向数据流。
   - L29（Vitest mock naive-ui）：删除 useProxyGrouping 相关测试时，剩下的 ProxyForm 测试若需 mount 仍需 `vi.mock('naive-ui', importOriginal+spread+6方法 stub)` 模式。
   - L26（verify_all 双实现对账）：H.1 在 .ps1 / .sh 双侧都必须存在且行为一致；写完后用 §8 R-8 的反向证伪流程验证。
   - L42（archive-task.sh insight regex）：07 §Insight 用裸 `## Insight`（无数字前缀），bullet `- ` 列表（insight L27）。

2. **删除顺序建议**（避免半路 broken build）：
   - 先后端 / 前端的 import / 使用方（handler、handleSubmit 分支、template 中的 `<n-switch v-model:value="batchMode">` 等）。
   - 再"被使用方"的定义（types.ts 接口、portrange 包、storage UpsertProxiesTx）。
   - 这样任何 mid-step `go vet` / `tsc` 都能给出"未使用 import / 未声明符号"的明确信号。

3. **verify_all H.1 step 设计**（伪代码）：

   ```powershell
   # PowerShell（按行扫描；与 Bash 实现对账，insight L26）
   Step "H.1" "T-037 deletion surface clean (no batch/probe/grouping residue)" {
       $patterns = @(
           'batchMode', 'portsExpr', 'apiBatchCreate', 'batchProxies',
           'UpsertProxiesTx', 'apiProbePorts', 'probePorts',
           'useProxyGrouping', 'groupProxiesByPrefix',
           'BatchProxiesRequest', 'BatchProxiesResponse',
           'PortProbeRequest', 'PortProbeResult', 'PortProbeResponse',
           'ErrDuplicateTcpRemote', 'isDuplicateTcpRemoteError',
           'internal/portrange'
       )
       $targets = @('web/src', 'internal', 'openapi.yaml')
       $hits = @()
       foreach ($p in $patterns) {
           foreach ($t in $targets) {
               if (-not (Test-Path $t)) { continue }
               $found = Select-String -Pattern $p -Path "$t/**/*" -SimpleMatch `
                   -Exclude '*.spec.ts.snap' 2>$null `
                   | Where-Object { $_.Path -notmatch '_archived' }
               if ($found) {
                   foreach ($f in $found) {
                       $hits += "$($f.Path):$($f.LineNumber): $($f.Line.Trim())"
                   }
               }
           }
       }
       if ($hits.Count -gt 0) {
           throw "T-037 deletion residue found:`n" + ($hits -join "`n")
       }
   }
   ```

   Bash 用 `git grep -nE "<合并 alternation 正则>" -- web/src/ internal/ openapi.yaml ':!*_archived/*'` 单 invocation 实现等价语义（insight L26：grep multiline 与 PS Get-Content -Raw 边界 case 谨慎；本 step 不涉及 multiline，纯 line-anchored）。

4. **远程端口 10022 → 223 修复证伪**（dev 阶段自检）：
   - 启动后端 + 前端 dev server
   - 干净 DB（rm data.db）登录后新建单条规则 `name=t037-smoke, type=tcp, localPort=80, remotePort=10022`
   - 列表刷新，"远程端口/域名" 列必须字面显示 `10022`（不能是 `223` 或 `80` 或 `—`）

## 13. 已知遗留 / 不动

- **verify_all G.* ID 冲突**：Go 编译 step (`G.1` go vet, `G.2` go test, `G.3` go build at line 82-100) 与 T-034 reviewer 协议 step (`G.1` reviewer sentinel, `G.2` PM protocol at line 400-419) 共用 ID。本任务**不修**——属于 T-034 引入的命名遗漏，影响 Summary 报告中 ID 出现两次但每条 step 仍独立 PASS/FAIL（不影响判定语义），是文档清洁度问题不是行为问题；未来 trivial 任务（候选 T-038）可改 reviewer 协议 step 为 `R.1 / R.2`。
- **architecture.html**：若 grep 命中"批量"/"探测"段则同步删除，否则不动。

## 14. Verdict

**READY**

- 所有 §2 表格行均来自 §7 Reuse audit + §8 Risk + §11 Partition 的实证 grep / Read，无凭空文件路径。
- §12 给 Stage 4 Developer 提供了删除顺序、insight 引用、verify_all H.1 伪代码与 bug 修复证伪步骤，无需进一步设计决策。
- 无新增依赖、无新建模块、无新增数据库 migration。
- 风险（§8）共 8 条覆盖了删除后编译 / OpenAPI / 升级 UX / bug 假根因 / 双实现对账多条主要失败模式，每条均有缓解。
- 与 01 verdict 中 PM 决策（Q-1 ~ Q-6）严格对齐，无设计漂移。

→ Stage 3 (Gate Reviewer) 可启动。
