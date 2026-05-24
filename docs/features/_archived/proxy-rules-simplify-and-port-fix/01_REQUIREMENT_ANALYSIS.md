# 01 — Requirement Analysis · T-037 proxy-rules-simplify-and-port-fix

> 模式：`full`（7-stage）  ·  输入：[INPUT.md](./INPUT.md)
> Stage 1 输出（Requirement Analyst → PM）。所有歧义由 PM 按用户授权预先决策（标 `[PM-DECIDED]`），不阻塞后续 stage。

## 1. Goal

剔除代理规则（proxy）UI/API 的批量创建路径、折叠分组视图、自动端口探测三类辅助能力，并修复"添加单条规则时远程端口 10022 被错误显示为 223"的现网 bug，让代理规则管理回归"一条规则 = 一次操作 = 一行展示"的简单形态。

## 2. In-scope behaviors

按系统层从前到后编号；每条均可由 QA 写测试断言。

### B-1. 删除批量创建入口（UI）

- **B-1.1** [Proxies.vue](../../../web/src/pages/Proxies.vue) 顶部按钮文案改回"新增规则"（去掉 "/ 批量新增"）。空状态文案、模态框标题、提交按钮文案同步去除"批量"语义。
- **B-1.2** [ProxyForm.vue](../../../web/src/components/ProxyForm.vue) 删除 `批量模式` 开关 `<n-switch v-model:value="batchMode">`、端口表达式 `<n-input v-model:value="portsExpr">` 控件、`canUseBatch` / `batchMode` / `portsExpr` 三个 ref 及其 watch / emit。
- **B-1.3** ProxyForm `defineExpose` 中删除 `isBatchMode` / `getPortsExpr` / `resetBatchState` 三个口子；保留 `validate` / `getProxyInput`（T-032 单向数据流契约不变）。
- **B-1.4** ProxyForm `rules.portsExpr` 条目整体删除；`rules.name` 校验正则恢复为单一形态 `^[A-Za-z0-9_-]{1,64}$`（即去掉批量分支的 58 字符上限分歧）。
- **B-1.5** 端口预设标签（`PORT_PRESETS` / `applyPreset`）**保留**——理由：预设标签属"快速填充"而非"自动探测"，用户语义中"端口由人工管理"指的是不让软件自动判定"是否可用"，不禁止预设值填充。点击后由用户继续编辑/校验。`[PM-DECIDED]`

### B-2. 删除批量创建入口（API / Store / Composable）

- **B-2.1** [web/src/api/proxies.ts](../../../web/src/api/proxies.ts) 删除 `apiBatchCreateProxies` 函数及其类型导入。
- **B-2.2** [web/src/stores/proxies.ts](../../../web/src/stores/proxies.ts) 删除 `batchCreate` action 及相关类型导入。
- **B-2.3** [web/src/types.ts](../../../web/src/types.ts) 删除 `BatchProxiesRequest` / `BatchProxiesResponse` 接口定义。
- **B-2.4** [web/src/composables/useProxyGrouping.ts](../../../web/src/composables/useProxyGrouping.ts) **整体文件删除**——折叠分组仅为批量创建配套视图，无批量则无意义。
- **B-2.5** 同步删除 [web/src/composables/\_\_tests\_\_/useProxyGrouping.spec.ts](../../../web/src/composables/__tests__/useProxyGrouping.spec.ts)。
- **B-2.6** [web/src/api/\_\_tests\_\_/proxies.spec.ts](../../../web/src/api/__tests__/proxies.spec.ts) 删除 `apiBatchCreateProxies` 相关测试用例（保留其余单条 CRUD 测试）。
- **B-2.7** [Proxies.vue](../../../web/src/pages/Proxies.vue) 的 `handleSubmit` 删除 `isBatchMode()` 分支；`columns` / `groupedRows` 改用 `proxiesStore.proxies` 直接渲染（每条 = 一行 single row 形态）。
- **B-2.8** [Proxies.vue](../../../web/src/pages/Proxies.vue) 表格数据源类型从 `GroupedProxyRow` 退化为 `Proxy`，移除 `kind === 'group'` 分支与 `expanded` 状态。

### B-3. 删除批量创建入口（后端 / OpenAPI）

- **B-3.1** [internal/httpapi/router.go](../../../internal/httpapi/router.go) 删除 `r.Post("/proxies/batch", h.batchProxies)` 一行。
- **B-3.2** [internal/httpapi/handlers_proxies.go](../../../internal/httpapi/handlers_proxies.go) 删除 `batchProxies` handler、`BatchProxiesRequest` / `BatchProxiesResponse` / `BatchConflict` 类型、`BatchProxiesMaxCount` 常量、`batchBasenameRE` 正则、`humanizePortRangeErr` / `writeBatchProxiesError` 辅助。
- **B-3.3** [internal/httpapi/handlers_batch_test.go](../../../internal/httpapi/handlers_batch_test.go) **整体文件删除**。
- **B-3.4** [internal/portrange/portrange.go](../../../internal/portrange/portrange.go) 与 [portrange_test.go](../../../internal/portrange/portrange_test.go) **整个目录删除**——端口表达式解析器是批量专用，无其他调用方（grep 确认仅 handlers_proxies.go 引用）。
- **B-3.5** [internal/storage/proxies.go](../../../internal/storage/proxies.go) 删除 `UpsertProxiesTx` 函数、`isDuplicateTcpRemoteError` 辅助；[internal/storage/proxies_batch_test.go](../../../internal/storage/proxies_batch_test.go) 整体删除。
- **B-3.6** [internal/storage/store.go](../../../internal/storage/store.go) 删除 `ErrDuplicateTcpRemote` 哨兵错误的导出（仅 UpsertProxiesTx 使用，已无调用方）。**保留** `(type, remote_port)` SQL 部分唯一索引——单条 UpsertProxy 路径走 `mapProxyWriteError` 的 `strings.Contains(low, "unique")` 兜底分支返回 422，行为已被 hardening-pass-audit / web-ui-mvp 测试覆盖；不写新 migration。`[PM-DECIDED]`
- **B-3.7** [openapi.yaml](../../../openapi.yaml) 删除 `/proxies/batch` 路径段、`BatchProxiesRequest` / `BatchProxiesResponse` schema 定义。

### B-4. 删除自动端口探测（前端）

- **B-4.1** [ProxyForm.vue](../../../web/src/components/ProxyForm.vue) 删除"探测可用性"按钮、`probing` / `probeStatus` / `probeText` 三个 ref、`handleProbe` / `reasonText` 函数、端口变化清除探测结果的 watch、`apiProbePorts` 导入。
- **B-4.2** [web/src/api/system.ts](../../../web/src/api/system.ts) 删除 `apiProbePorts` 函数。
- **B-4.3** [web/src/types.ts](../../../web/src/types.ts) 删除 `PortProbeRequest` / `PortProbeResult` / `PortProbeResponse` 接口。
- **B-4.4** [web/src/api/\_\_tests\_\_/system.spec.ts](../../../web/src/api/__tests__/system.spec.ts) 删除 `apiProbePorts` 相关测试用例（保留 `apiGetPublicIP` / `apiUploadBin` 测试）。

### B-5. 删除自动端口探测（后端 / OpenAPI）

- **B-5.1** [internal/httpapi/router.go](../../../internal/httpapi/router.go) 删除 `r.Post("/system/probe-ports", h.probePorts)` 一行。
- **B-5.2** [internal/httpapi/handlers_system.go](../../../internal/httpapi/handlers_system.go) 删除 `probePorts` / `probeOnePort` 函数、`PortProbeRequest` / `PortProbeResult` / `PortProbeResponse` 类型、`portProbeMaxCount` / `portProbeTimeout` 常量。
- **B-5.3** [internal/httpapi/port_probe_test.go](../../../internal/httpapi/port_probe_test.go) **整体文件删除**。
- **B-5.4** [openapi.yaml](../../../openapi.yaml) 删除 `/system/probe-ports` 路径段、`PortProbeRequest` / `PortProbeResult` / `PortProbeResponse` schema 定义。

### B-6. 修复"远程端口 10022 显示为 223" bug

- **B-6.1 根因**（按代码 walkthrough 的最高似然路径）：[Proxies.vue](../../../web/src/pages/Proxies.vue) `columns` 中"远程端口/域名"列对 group row 渲染 `row.portRangeText`，而 `portRangeText` 在 [useProxyGrouping.ts:113](../../../web/src/composables/useProxyGrouping.ts#L113) 由 `compressPorts(sorted.map((p) => p.localPort))` 生成——**用 localPort 当远程端口展示**。当用户已有 ≥2 条 `<basename>-<N>` 同 type 同 basename 规则时，单条新增（即便远程端口正确写入 DB = 10022）会被并入 group row → 远程端口列显示 localPort 区间字符串（如 "22, 100" / "22-25, 223"）而非真实 remotePort。10022/223 数字对不一定字面成立，但**展示与 DB 真实 remotePort 解耦**是确定性根因。
- **B-6.2 修复路径**：B-2.7 / B-2.8 删除分组分支后，列渲染回退到 single row 的 `String(row.proxy.remotePort)`，bug 物理上不可能复发（无 group row 代码路径存在）。
- **B-6.3 验证**：QA 用 Playwright e2e 在干净 DB 上添加单条 tcp 规则 `name=test-10022`、`localPort=80`、`remotePort=10022`，刷新页面后从列表表格读取"远程端口/域名"列的渲染字符串，断言 `=== "10022"`（adversarial 必跑用例）。
- **B-6.4 数据层校验**：QA 用 Vitest mount 测试断言 `apiCreateProxy(input)` 传入的 `remotePort` 字段值与 NInputNumber 用户输入值 1:1 透传（无类型转换 / 截断），覆盖 1 / 10022 / 65535 三个值。

### B-7. 文档与导航同步

- **B-7.1** [docs/dev-map.md](../../../docs/dev-map.md) 的"功能在哪里"表删除 `internal/portrange/portrange.go`、`useProxyGrouping.ts`、`usePortPresets.ts` 中"探测"语义、批量入口的描述；"web/src/api/proxies.ts" / "web/src/api/system.ts" 的 T-018 标记同步精简。
- **B-7.2** [docs/architecture.html](../../../docs/architecture.html) 若引用了批量端点 / 探测端点，同步删除/降级（仅在 grep 命中时改）。
- **B-7.3** **不写**新 SPEC 文档；行为变更已由 INPUT.md + 本 01 完整记录。

## 3. Out-of-scope

- **OOS-1** 不修改 SQLite schema —— 不写新 migration（保留 `(type, remote_port)` 部分唯一索引；单条创建 422 兜底路径已覆盖）。
- **OOS-2** 不修改 NInputNumber 行为或替换组件 —— bug 根因不在 NInputNumber 而在分组渲染（B-6.1）。
- **OOS-3** 不动 T-032 单向数据流契约（`initialValue` prop + `defineExpose getProxyInput()`）。
- **OOS-4** 不动 T-027 下载/上传互斥相关 409 文案。
- **OOS-5** 不删除端口预设标签 `usePortPresets.ts`（B-1.5）。
- **OOS-6** 不删除公网 IP 检测（PublicIpDetector）、上传二进制（UploadBinButton）、防火墙提示（FirewallHint）—— 这些与"端口探测"语义不同。
- **OOS-7** 不向后兼容老前端 / 老 API 客户端 —— 单二进制部署、前后端同版本随 release 发布；删除即清空，不留 deprecated 桩。

## 4. Boundary conditions

| 场景 | 期望行为 |
|---|---|
| 用户拥有 ≥2 条已折叠的旧规则（例 ssh-22 / ssh-80） | 升级后回退到 2 行 single row 展示；DB 数据不变；frpc reload 行为不变 |
| 用户已在跑 frpc + 大量代理规则 | 升级期前端短暂 5xx 不可接受；后端单 binary 部署 → 重启窗口为常规 release 平滑时间 |
| 老前端版本调用已删的 `/proxies/batch` 或 `/system/probe-ports` | chi router → 404 Not Found（标准 chi 默认，无需特殊处理） |
| `(type, remote_port)` 冲突在单条创建路径 | mapProxyWriteError 兜底分支 422 + field=remotePort（既有行为，无回归） |
| 用户填空名称 / 非法端口 / 域名错误 | 现有单条校验路径不变（ValidateProxyName / ValidatePort / ValidateDomain） |
| `remotePort = 10022`（典型场景） | DB 存 10022；前端列表 "远程端口/域名"列字面渲染 `"10022"`（B-6 验证） |
| `remotePort = 65535` / `remotePort = 1`（边界） | 同上：字面渲染 `"65535"` / `"1"` |
| 重复添加同一 `(type, remotePort)` | 422 错误，前端 message.error 显示后端 humanizeError 文本 |

## 5. Acceptance criteria

| ID | 准则 | 验证手段 |
|---|---|---|
| AC-1 | `grep -rn "batchMode\|portsExpr\|apiBatchCreate\|batchProxies\|UpsertProxiesTx\|portrange\.\|apiProbePorts\|probePorts\|useProxyGrouping\|groupProxiesByPrefix\|BatchProxies\|PortProbe\|ErrDuplicateTcpRemote" web/src internal/ openapi.yaml` 在 src/spec 层**返回 0 行**（_archived/ 与 docs/features/ 历史档案豁免） | verify_all 新增 step 守门 + 手工 grep |
| AC-2 | `web/src/composables/useProxyGrouping.ts` / `web/src/composables/__tests__/useProxyGrouping.spec.ts` / `internal/portrange/` / `internal/httpapi/handlers_batch_test.go` / `internal/httpapi/port_probe_test.go` / `internal/storage/proxies_batch_test.go` **文件不存在** | Glob 确认 |
| AC-3 | `npm run build` / `npm test` / `go build ./...` / `go test ./...` 全部 PASS（无残留引用 / 编译错误） | verify_all D.* / Test.* steps |
| AC-4 | `scripts/verify_all` PASS（含全部既有 step + 新 step 见 AC-1） | Stage 7 |
| AC-5 | 新增单条 tcp 规则 `name=t037-smoke`、`localPort=80`、`remotePort=10022`，列表"远程端口/域名"列渲染 `"10022"` | Playwright e2e 新 spec（adversarial） |
| AC-6 | 单条创建/编辑/删除 / `(type, remotePort)` 冲突 / name 重复冲突 / version 冲突的现有测试 100% 通过 | go test ./internal/httpapi/... ./internal/storage/... ./web/src/... |
| AC-7 | OpenAPI 文件无 `/proxies/batch` / `/system/probe-ports` / 相关 schema；通过 YAML 解析无错 | grep + `npx swagger-cli validate openapi.yaml`（如 CI 已有）或人工 review |
| AC-8 | dev-map.md 中"端口表达式解析"、"折叠分组"、"端口探测"三处描述同步移除/降级；新增/移动/删除的模块在导航表中字节级一致 | 人工 review + 文件 diff |

## 6. Non-functional requirements

- **NFR-1（兼容性）**：单 binary release 部署模型 + 前后端同版本随 release 发布 → 不要求 backwards compatibility（OOS-7）。
- **NFR-2（安全）**：删除 `/system/probe-ports` 等于关闭一个被 SessionAuth+CSRF 保护但仍可被授权用户做内网端口扫描的辅助通道，**净降低**攻击面（FR-C.3.7 / R-8 in T-018 02 §C.3 的反向）。
- **NFR-3（性能）**：列表渲染由 O(N) 折叠分组退化为 O(N) 直接渲染——常数因子降低；200 条规则上限 unchanged。
- **NFR-4（可维护性）**：移除三块跨前后端 + Composable + Vitest + e2e 的耦合代码，长期维护 surface 显著缩小（这是本任务的核心收益）。

## 7. Related tasks

- **T-018** (`upload-bin-multiport-ip-probe`, 归档) —— 本任务回退的功能引入者。**不复用其 02 设计**——本任务的设计图是 T-018 的反向；02 仅作为"删除什么"清单的反向 reference。
- **T-032** (`proxy-form-vmodel-oom-fix`, 归档) —— ProxyForm 单向数据流契约的引入者；insight L28（vue v-model OOM 反模式）；本任务的 ProxyForm 改动**必须不动** `initialValue` prop / `defineExpose getProxyInput()` / `toProxyInput()`。
- **T-007** (`hardening-pass-audit`, 归档) —— 单条 `(type, remotePort)` 冲突 422 路径的测试覆盖来源（mapProxyWriteError 兜底分支），见 [internal/httpapi/handlers_proxies.go:425-438](../../../internal/httpapi/handlers_proxies.go#L425)。
- **T-027** (`download-cancel-and-upload-decouple`, 归档) —— "互斥的 409 错误必须给用户明确路径解锁"（insight L16）；本任务无新增 409，无文案影响。

## 8. Open questions for user

按用户授权 "你来决策" + "我只看结果是否符合需求"，本节**全部由 PM 在 RA 阶段直接决策**，不阻塞流水线。决策列表（含理由）：

| # | 歧义 | PM 决策 | 理由 |
|---|---|---|---|
| Q-1 | 端口预设标签（usePortPresets）保留还是删除？ | **保留** | 预设属"快速填充" ≠ "自动探测"；用户语义只针对端口可用性自动判定 |
| Q-2 | SQL `(type, remote_port)` 部分唯一索引保留还是写新 migration 删除？ | **保留** | 单条创建路径仍需该唯一性约束（避免两条 tcp 抢同一 remote_port）；写迁移引入回滚风险，无收益 |
| Q-3 | 是否给老前端调用 `/proxies/batch` 加 410 Gone 兜底而非依赖 chi 默认 404？ | **不加**（OOS-7） | 单 binary 同版本部署；老前端 / curl 直击老端点为低优先 |
| Q-4 | 是否保留 `BatchProxiesMaxCount = 32` 之类常量给未来潜在功能？ | **不保留** | YAGNI；未来如需要再走 /harness 重新设计 |
| Q-5 | T-018 引入的 `usePortPresets` 是否随删？ | **不删** | 见 Q-1；UI 自带 SSH/RDP/HTTP 等常用端口标签是良好用户体验 |
| Q-6 | dev-map.md 是否标 `T-037 deleted` 痕迹？ | **直接删除条目** | dev-map 是"现状"视角，历史在 archive；维持 dev-map 简洁是 T-001/T-007/T-027 既有约定 |

## 9. Verdict

**READY**

- 所有歧义已由 PM 决策（§8）；无 BLOCKED ON USER 路径。
- In-scope behaviors（B-1 ~ B-7）按系统层枚举，每条对应明确文件路径与可断言行为；删除集与保留集互不重叠。
- Acceptance criteria（AC-1 ~ AC-8）每条可由 QA 写测试或脚本断言；AC-5 / AC-6 为 adversarial 验证锚点。
- Boundary conditions 覆盖升级期、老前端误调用、(type, remotePort) 冲突、端口边界等典型回归点。
- 与历史任务的关联（§7）明确——T-032 单向数据流契约不破坏；T-018 功能反向回退。

→ Stage 2 (Solution Architect) 可启动。
