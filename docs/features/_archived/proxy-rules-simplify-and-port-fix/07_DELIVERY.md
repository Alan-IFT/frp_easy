# 07 — Delivery · T-037 proxy-rules-simplify-and-port-fix

> 模式：`full` · 完整 7-stage 流水线
> Stage 7 输出（PM Orchestrator → 用户）。

## 1. 任务摘要

用户三条诉求一次性处理：

1. **去掉批量添加代理规则功能** —— 用户反馈 webUI 上"展开明细"无反应，无法管理批量规则；理论上批量 ≈ 多次单个，简化为单个。
2. **修复"远程端口 10022 显示为 223"bug** —— 单条规则保存后远程端口列字面与用户输入解耦。
3. **去掉自动探测端口功能** —— 端口由用户人工管理。

按"用户体验好 + 符合软件工程标准 + 长期易维护"决策原则，本任务等价于**回退** T-018 引入的"批量代理 / 折叠分组展示 / 自动端口探测"三类辅助能力，同时保留 T-018 引入的端口预设标签（属"快速填充" ≠ "自动探测"）。

## 2. 改动汇总

净 **删 11 文件 / 改 13 文件 / 加 2 段（PS + Bash verify_all H.1 step）**。

### 前端（dev-frontend 范围）

| 文件 | 操作 |
|---|---|
| [web/src/components/ProxyForm.vue](../../../web/src/components/ProxyForm.vue) | 删 batchMode 开关 + portsExpr 输入 + 探测按钮 + 相关 watch/emit/expose；rules 简化（-247 行 / +154 行） |
| [web/src/pages/Proxies.vue](../../../web/src/pages/Proxies.vue) | 删按钮"/批量新增"文案、删 handleSubmit 批量分支、删 columns group 渲染、删 groupedRows computed、改数据源类型 Proxy（-100 / +75） |
| [web/src/composables/useProxyGrouping.ts](../../../web/src/composables/useProxyGrouping.ts) | **整文件删除** |
| `web/src/composables/__tests__/useProxyGrouping.spec.ts` | **整文件删除** |
| [web/src/api/proxies.ts](../../../web/src/api/proxies.ts) | 删 apiBatchCreateProxies + 相关类型导入 |
| [web/src/api/system.ts](../../../web/src/api/system.ts) | 删 apiProbePorts + 相关类型导入 |
| [web/src/stores/proxies.ts](../../../web/src/stores/proxies.ts) | 删 batchCreate action + 相关类型导入 |
| [web/src/types.ts](../../../web/src/types.ts) | 删 BatchProxiesRequest / BatchProxiesResponse / PortProbeRequest / PortProbeResult / PortProbeResponse 共 5 个接口 |
| `web/src/api/__tests__/proxies.spec.ts` | **整文件删除**（4 个 batch 测试） |
| [web/src/api/\_\_tests\_\_/system.spec.ts](../../../web/src/api/__tests__/system.spec.ts) | 删 apiProbePorts 测试 2 个 |
| [web/src/components/\_\_tests\_\_/ProxyForm.spec.ts](../../../web/src/components/__tests__/ProxyForm.spec.ts) | 把 `update:batchMode`/`update:portsExpr` 断言换成更强的 `Object.keys(wrapper.emitted()).length === 0` |

### 后端（dev-backend 范围）

| 文件 | 操作 |
|---|---|
| [internal/httpapi/router.go](../../../internal/httpapi/router.go) | 删 2 路由（/proxies/batch + /system/probe-ports） |
| [internal/httpapi/handlers_proxies.go](../../../internal/httpapi/handlers_proxies.go) | 删 batchProxies + BatchProxies* types + batchBasenameRE + BatchProxiesMaxCount + humanizePortRangeErr + writeBatchProxiesError + portrange import |
| [internal/httpapi/handlers_system.go](../../../internal/httpapi/handlers_system.go) | 删 probePorts + probeOnePort + PortProbe* types + portProbeMaxCount/Timeout 常量 |
| `internal/httpapi/handlers_batch_test.go` | **整文件删除** |
| `internal/httpapi/port_probe_test.go` | **整文件删除** |
| `internal/portrange/portrange.go` + `portrange_test.go` | **整目录删除** |
| [internal/storage/proxies.go](../../../internal/storage/proxies.go) | 删 UpsertProxiesTx + isDuplicateTcpRemoteError |
| `internal/storage/proxies_batch_test.go` | **整文件删除** |
| [internal/storage/store.go](../../../internal/storage/store.go) | 删 ErrDuplicateTcpRemote 哨兵导出 |

### Spec / Docs / 守门

| 文件 | 操作 |
|---|---|
| [openapi.yaml](../../../openapi.yaml) | 删 `/proxies/batch` 路径段 + `/system/probe-ports` 路径段 + 5 个 schema |
| [docs/dev-map.md](../../../docs/dev-map.md) | 5 处描述更新（api 段、composables 段、ProxyForm 段、Proxies 页段、HTTP 路由层段）+ 删 portrange 行 + storage 哨兵列表精简 |
| [scripts/verify_all.ps1](../../../scripts/verify_all.ps1) | 加 `Step "H.1" "T-037 deletion surface clean ..."` 守门 17 个禁词 |
| [scripts/verify_all.sh](../../../scripts/verify_all.sh) | 同款 Bash `step "H.1"`，与 PS 对账（insight L26） |

### 不动（OOS 显式声明）

- migrations/ SQLite schema（`(type, remote_port)` 部分唯一索引保留 —— 单条 mapProxyWriteError 仍兜底）
- usePortPresets.ts（端口预设属"快速填充" ≠ "自动探测"，PM-DECIDED）
- NInputNumber 配置 / FirewallHint / PublicIpDetector / UploadBinButton
- verify_all 既有 step 全部不动；G.\* ID 冲突历史遗留不修
- T-032 单向数据流契约（initialValue prop + getProxyInput()）完整保留

## 3. 用户验证

### 用户原报告 3 条

| # | 用户问题 | 修复手段 | 用户应观察到的现象 |
|---|---|---|---|
| 1 | 批量配置规则展开明细无反应，无法管理 | 整个批量功能 + 折叠分组**物理删除** —— 不再存在批量入口 / 不再有折叠 / 旧批量规则在升级后自动散开为多条独立行 | 顶部"新增规则"按钮（无"/批量新增"）；列表每条规则独立一行展示；编辑 / 删除按钮直接作用于每条规则 |
| 2 | 远程端口 10022 → 显示 223 | 删除 columns 中 group 渲染分支后，远程端口列只走 `String(row.remotePort)` 单一路径；与 localPort 派生字符串完全解耦 | 新增 / 编辑后，列表"远程端口/域名"列字面显示用户输入的端口号（10022 / 65535 / 1 字面） |
| 3 | 自动探测端口功能去掉 | 删除 ProxyForm 上"探测可用性"按钮 + apiProbePorts + 后端 /system/probe-ports 端点 + 相关类型 | 表单不再有"探测可用性"按钮 / Tag；端口完全由用户自行填入 |

### 升级提示（建议用户预期）

- 升级到 T-037 后版本，**已有的批量添加规则会自动散开为多条独立行**（数据库不变，仅 UI 展示改变），可逐条编辑 / 删除。
- 系统/批量/折叠相关端点不再可访问（chi 默认返 404），老 API 客户端需要随版本升级。

## 4. verify_all 最终闸门

按 insight L30 / 04 §3 / 06 §3 隔离 T-036 工作树污染后跑 `bash scripts/verify_all.sh --quick`：

```
=== Summary ===
  PASS: 27
  WARN: 0
  FAIL: 0
  SKIP: 0
```

全 27 step PASS，含本任务新守门 H.1。Stage 6 反向证伪 PASS（AT-1 注入→FAIL，移除→PASS）+ PS/Bash 双实现对账 PASS（AT-2）。

非本任务 FAIL 归责：[parseLogLine.ts:56](../../../web/src/composables/log/parseLogLine.ts#L56) 与 [LogViewer.spec.ts:213](../../../web/src/components/__tests__/LogViewer.spec.ts#L213) TS 错误是另一进行中任务 T-036（log-ui-ux-polish）的 untracked 文件，与本任务无关；详见 06 §6。

## 5. 收益

- **代码行数**：净删 ~1,200 行（含测试 / 文档）。
- **维护面缩减**：3 块跨前后端 + Composable + Vitest + e2e 的耦合代码移除。
- **安全面缩减**：关闭一个被授权用户做内网端口扫描的辅助通道（NFR-2）。
- **用户体验**：列表回归"一条规则一行"的简单形态，不再有展开/折叠迷惑。
- **bug 物理根治**：远程端口显示与 localPort 派生字符串完全解耦，未来无法因新增折叠分组逻辑再次复发同款 bug（H.1 守门防御静默回退）。

## 6. 决策提示给用户

按用户授权"你来决策、我只看结果"原则做的几个关键决策（详见 01 §8 + 02 §10 + 03 §C-*）：

- **保留端口预设标签 usePortPresets**：因"快速填充" ≠ "自动探测"。用户语义指的是不让软件自动判可用性，不禁止预设端口号填入。
- **不写 SQL migration**：`(type, remote_port)` 部分唯一索引在单条创建路径仍有用，写迁移引入回滚风险无收益。
- **不留 deprecation API 桩**：单 binary 同版本部署模型，老前端击删端点 chi 默认 404 即可。
- **不修 verify_all G.\* ID 冲突**：历史遗留属 T-034 引入，本任务不扩散修复面。

如以上有任何不符合预期，请告知我立即调整。

## Insight

- 2026-05-24 · 跨任务工作树污染归责的高效路径：本任务与并行进行中的 T-036（log-ui-ux-polish）共享工作树，T-036 的 untracked 文件含预先存在的 TS 编译错误，会让 verify_all B.1 / B.3 在裸跑时 FAIL。**归责动作的最稳形态**是 `git stash push --include-untracked --keep-index` 把 T-036 modifications + untracked 全 stash → **再加一步** `mv <git-stash-未捕获的-untracked.spec.ts> /tmp/`（git stash 对部分 untracked 文件路径捕获不到，尤其在 staged + working tree 已经分流时）→ 跑 verify_all 拿到隔离后 PASS 数 → `mv` 恢复 + `git stash pop` 恢复。"verify_all 跑出 PASS 不带 T-036 / FAIL 带 T-036"双侧对照是 insight L30 的延伸形态，可作为多任务并行 dev 的标准范式 · evidence: T-037 04 §3 + 06 §3 + 07 §4 验证 LogViewer.spec.ts git stash 不能捕获、需手工 mv 后才能纯净跑 verify_all
- 2026-05-24 · UI 表格"虚拟列"与底层 DB 字段名同名时是隐性 bug 高发面：T-018 引入 Proxies.vue group row 的"远程端口/域名"列同时承担 single row 的 `remotePort` 字面 + group row 的 `portRangeText`（compressPorts(localPort 数组)）—— 同列名但**语义来源不同**让用户输入的远程端口值在折叠场景下被静默替换为 localPort 派生字符串。修复路径不是"加 group row 用 remotePort 计算"而是**整体删除 group row**（T-037 选择），因为该展示功能与"批量创建"语义耦合，批量删除后折叠展示也失去意义。规则：UI 列名与 DB 字段名同名时，该列的所有 render 分支必须**字面引用同一字段**，否则视为反模式 · evidence: T-037 02 §6 流程对比 + 04 §5 / 05 §3 修复路径论证
- 2026-05-24 · verify_all H.1 双实现（PS + Bash）共用同一 alternation 正则 + 同款豁免（`docs/features/_archived/**` + `.harness/**`）让"删除面静默回退"从纸面规则升级为运行时硬约束。AT-1 反向证伪（注入 `batchMode` 字面 → H.1 FAIL → 删除 → PASS）证明禁词正则确实捕获字面、非假阳性。这是"删除型任务"特有的守门范式：删除完代码后**必须**加一个 grep step 守门未来回退（与"新增型任务"的"加 step 守门新功能正确性"对称），让 T-037 的删除决策长期不可逆 · evidence: T-037 04 §1 步骤 5 + 06 AT-1 / AT-2 实测
