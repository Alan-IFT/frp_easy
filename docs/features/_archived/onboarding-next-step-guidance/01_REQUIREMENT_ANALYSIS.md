# 01 需求分析 — T-062 · onboarding-next-step-guidance

> Stage 1 / Requirement Analyst · 模式：full · 分区：dev-frontend · 输出语言：中文
> 批次：ux-ui-uplift-2026-05（第 1 个任务）

## 1. 目标（一句话）

在配置类页面与向导完成态补"正向下一步"引导与跨页连通导航，消除"配好了不知道下一步"的体验断点，并在 Wizard `both` 模式下对两端 token 不一致给出非阻断预警。

## 2. 范围内行为（可测，编号）

> 全部为前端 UI 文案 + SPA 内导航（`router.push`）+ 一处条件比较。无后端 / store / API / 路由守卫 / 数据流改动。

**IS-1（Wizard 完成页加规则引导）** —— `web/src/pages/Wizard.vue` step3 完成态：
- 当所选角色为 `frpc` 或 `both`，**且**该角色对应二进制全就绪（即 `binWarning.length === 0`，进入既有"全就绪自动跳转"分支）时，在 step3 完成内容内展示一个"前往『代理规则』添加要转发的端口 →"链接/按钮，点击触发 `router.push('/proxies')`。
- 当所选角色为 `frps`（纯服务端，无 frpc 转发规则概念）时，**不**展示该引导。
- 当进入既有"缺失就地警告"分支（`binWarning.length > 0`）时，**不**新增加规则引导（保持 T-057 缺失态聚焦于补齐二进制，避免引导冲突）。

**IS-2（Client.vue 保存成功后加规则引导）** —— `web/src/pages/Client.vue`：
- `handleSave()` 成功（`apiPutClient` resolve、`message.success` 之后）后，在页面就地展示一个"前往『代理规则』添加要转发的端口 →"链接/按钮，点击触发 `router.push('/proxies')`。
- 引导仅在至少保存成功过一次后可见（初始加载态不显示）。保存失败（catch 分支）不显示。

**IS-3（Proxies.vue 保存成功后引导去启动/看运行态）** —— `web/src/pages/Proxies.vue`：
- `handleSubmit()` 新增或编辑代理成功后，在页面就地展示引导，含两个 SPA 内导航入口："去仪表盘启动 frpc"（`router.push('/dashboard')`）与"去服务端监控查看运行态"（`router.push('/server/monitor')`）。
- 该引导仅在保存成功后可见；保存失败（catch）不显示。

**IS-4（Proxies.vue 空态补连通入口）** —— `web/src/pages/Proxies.vue` `#empty` 模板（现有 L31-33 "暂无代理规则，点击右上角「新增规则」开始配置"）：
- 在现有空态文案基础上**补充**一个 SPA 内导航入口（"去仪表盘启动"或"去服务端监控"，二选一或两者，由 Architect 定具体放置），点击触发对应 `router.push`。
- **不重复**现有"点击右上角「新增规则」开始配置"文案。

**IS-5（Server.vue 加查看运行态链接）** —— `web/src/pages/Server.vue`：
- 在已加载（loaded）态的配置表单区（建议 `#action` 按钮区或页头附近）加一个"查看运行态 →"链接/按钮，点击触发 `router.push('/server/monitor')`。
- 与 ServerMonitor.vue 既有反向跳转（`goServerConfig()` → `router.push('/server')`，ServerMonitor.vue:218-220）形成双向连通。
- 加载失败态 / 加载中态**不**显示该链接（仅 loaded 态显示，避免在无配置语境下误导）。

**IS-6（ProxyForm.vue 远程端口纯文案提示）** —— `web/src/components/ProxyForm.vue`：
- 在远程端口字段（`isTcpUdp` 为真时显示的 `remotePort` `n-form-item`，L56-68）追加 help 文案或 placeholder，内容语义为"需在服务端『端口策略』允许范围内"。
- **纯文案**：不读取服务端端口策略数据、不做跨页校验联动、不改 `useProxyForm` 校验规则。

**IS-7（Wizard both 模式 token 不一致非阻断预警）** —— `web/src/pages/Wizard.vue` step2：
- 当 `selectedRole === 'both'`，**且** `frpsForm.authToken` 与 `frpcForm.authToken` 二者均非空（trim 后非空）、**且**二者不相等时，在 step2 展示一个非阻断 warning 提示（语义如"两端 token 不一致，frpc 将无法连接 frps"）。
- **非阻断**：该提示不阻止 `handleNext()`/`完成配置`，不抛错，不进入 `configError`。用户仍可继续完成向导。
- 当任一 token 为空、或二者相等、或角色非 `both` 时，**不**显示该预警。

## 3. 范围外（本次明确不做）

- **OOS-1**：ProxyForm 远程端口与服务端端口策略的**跨页实时校验联动**（读 allowPorts 数据后高亮越界端口）—— 中-高成本，用户明确不做（IS-6 仅纯文案）。
- **OOS-2**：Wizard token 不一致的**强制阻断**（高级用户可能有特殊需求，仅非阻断预警）。
- **OOS-3**：后端 / store / API / 路由守卫 / `useProxyForm` 校验规则 / `useServerRuntime` 数据流任何改动。
- **OOS-4**：新增依赖、新增路由、新增 store 字段。
- **OOS-5**：T-057 既有 Wizard 完成流逻辑（missingForRole 缺失分支、binWarning 定格快照、全就绪自动跳转节奏）的行为改动 —— 新引导只能**并存追加**，不得改既有分支行为。
- **OOS-6**：Server.vue / Client.vue 的 T-058「重新加载」+ dirty 确认逻辑、T-060 端口策略 dirty 检测的任何改动。
- **OOS-7**：e2e 测试改动（PM 已核实新引导文案对现有 e2e 零冲突）。

## 4. 边界条件

- **BC-1（Wizard frps 角色）**：纯 frps 角色无 frpc 转发规则概念，IS-1 加规则引导**不出现**。
- **BC-2（Wizard 缺失态）**：`binWarning.length > 0` 时 IS-1 加规则引导**不出现**（聚焦补二进制）。
- **BC-3（Wizard token 一空一非空）**：仅一端有 token 时 IS-7 预警**不出现**（"都非空"是触发前提；一端空可能是用户尚未填，非冲突）。
- **BC-4（Wizard token 含前后空白）**：token 比较前需 trim 判空（避免纯空白被当作"非空 token"误报）；是否 trim 后再比较相等性由 Architect 定（保守建议：判"是否触发预警"用 trim 后非空 + 原值比较，与提交时 `authToken || undefined` 语义对齐）。
- **BC-5（Proxies 编辑 vs 新增成功）**：IS-3 引导对"新增成功"与"编辑成功"两条路径都应出现（二者都意味着规则已变更，用户可能想去启动/看运行态）。
- **BC-6（Proxies 删除成功）**：IS-3 引导是否在删除成功后出现 —— 删除不是"配好了"语义，建议**不**触发 IS-3 引导（删除有独立的 `handleDeleteConfirm`，与保存路径分离）。
- **BC-7（Client 保存失败）**：IS-2 引导在保存失败时**不**出现。
- **BC-8（Proxies/Server 加载失败态）**：Server.vue 加载失败（`loadError`）/ 加载中（`loading`）时 IS-5 链接**不**出现（仅 loaded 态）；Proxies 加载失败态（`proxiesStore.error`）走 `n-result`，IS-4 空态引导仅在 `#empty`（无失败、无数据）时出现。
- **BC-9（导航目标存在性）**：`/proxies`、`/dashboard`、`/server/monitor` 均为 router.ts 现有路由（已核实：proxies L17 / dashboard L16 / server/monitor L20），导航不会 404。
- **BC-10（router.push 而非 href）**：所有导航入口必须 `router.push`，禁 `href`/`<a tag>`（insight L17，整页刷新会丢 Pinia 状态 + 重跑守卫）。

## 5. 验收标准（可验证）

- **AC-1**：选 frpc/both 角色 + 二进制就绪完成向导 → step3 出现"前往『代理规则』…"入口，点击后 `router.push` 被以 `/proxies` 调用。（DOM 文本 + push mock 断言）
- **AC-2**：选 frps 角色完成向导 → step3 **不**出现加规则引导。（DOM 文本 not present）
- **AC-3**：选 both 角色但 frpc/frps 缺失（binWarning>0）→ step3 出现 T-057 缺失警告，**不**出现加规则引导。（既有 T-057 警告仍在 + 新引导 not present）
- **AC-4**：Client.vue 保存成功后 → 引导入口出现，点击 `router.push('/proxies')`；保存失败后不出现。
- **AC-5**：Proxies.vue 新增成功 / 编辑成功后 → 出现"去仪表盘启动"+"去服务端监控"入口，点击各自 `router.push('/dashboard')` / `('/server/monitor')`。
- **AC-6**：Proxies.vue 空态（无数据无失败）→ `#empty` 含 SPA 导航入口，点击 `router.push` 到对应路由；现有"点击右上角「新增规则」"文案仍在。
- **AC-7**：Server.vue loaded 态 → 出现"查看运行态 →"入口，点击 `router.push('/server/monitor')`；加载失败/加载中态不出现。
- **AC-8**：ProxyForm.vue 远程端口字段（tcp/udp）→ DOM 含"端口策略"相关纯文案 help/placeholder；http/https 类型时（无 remotePort 字段）不强制要求该文案。
- **AC-9**：Wizard both 模式两端 token 均非空且不等 → step2 出现 token 不一致 warning；点击完成配置仍能推进（非阻断，不进 configError）。
- **AC-10**：Wizard both token 相等 / 一端空 / 非 both 角色 → token 不一致 warning **不**出现。
- **AC-11（回归护栏）**：T-057 既有行为不变 —— 全就绪自动跳转分支仍 `message.success('配置已保存，正在跳转...')` + `router.push('/dashboard')`；缺失分支仍展示 T-057 warning + 「进入仪表盘」按钮。新引导不改这些既有可观察行为。
- **AC-12（导航方式护栏）**：所有新增导航点用 `router.push`（测试以 mock router.push 或 vue-router mock 断言调用参数），DOM 中新增入口不是 `<a href>` 整页跳转。
- **AC-13（verify_all 闸门）**：`scripts/verify_all` PASS（PM 上下文标 PENDING，由 orchestrator 真跑）；新增前端测试同步 bump `scripts/baseline.json` 的 `frontend_tests`/`test_count`/`version`。

## 6. 非功能需求（仅列实质性的）

- **NFR-1（导航一致性）**：所有 SPA 内导航统一 `router.push`（insight L17），与项目既有范式（T-048 Dashboard 日志链接改 router.push、ServerMonitor goServerConfig）一致。
- **NFR-2（顶级路由约束）**：Wizard 是顶级路由 `/wizard`，不嵌 AppLayout（insight L30/L31，已核 router.ts:10），其引导必须**就地呈现**，不能依赖 AppLayout 顶栏入口。
- **NFR-3（测试纪律）**：新增测试断言全用 DOM 文本 / `find('button')` 按文本 / getExposed 句柄，零 naive-ui 组件名查询（insight L45 / T-057 教训）；如需断言 message，用 `vi.mock('naive-ui')` 单例 spy；导航断言 mock `useRouter().push` 或 vue-router。复用 `web/src/test-utils/`。
- **NFR-4（向后兼容 / e2e）**：新引导文案不破坏 e2e（PM 已 grep 核实 01-setup/02-auth/03-dashboard 无新文案断言、用 bypassWizard 绕过向导、不进 Proxies/Server 编辑流，insight L34）。

## 7. 相关历史任务

- **T-057** `docs/features/binary-missing-onboarding-ux/` —— Wizard 完成流 missingForRole/binWarning 定格快照/全就绪自动跳转。本任务 IS-1 必须与之并存（AC-3/AC-11 护栏）。insight L30/L31 来自此任务。
- **T-047** `docs/features/frontend-honest-states/` —— Proxies 三态（n-result 失败 / #empty 空 / data-table 加载）、Server/Client 加载三态。IS-4 加在现有 #empty 上，IS-5 仅 loaded 态。
- **T-040** `docs/features/frps-allow-ports-policy/` —— AllowPortsEditor 端口策略 + 单向数据流。IS-6 文案呼应"服务端『端口策略』允许范围"。
- **T-058** `docs/features/frontend-interaction-polish/` —— Wizard step2 frpc 标题已合并单分支；Server/Client「重新加载」+ dirty 确认（IS-5/OOS-6 须不破坏）。
- **T-041** `docs/features/server-monitor-page-ui/` —— ServerMonitor goServerConfig→push('/server') 既有反向跳转（IS-5 补正向）。
- **T-042** `docs/features/proxy-runtime-status-merge/` —— Proxies.vue runtime 列（IS-3/IS-4 不碰）。

## 8. 给用户的开放问题

无。用户在 INPUT.md 中已对所有关键决策点给出明确指示：
- 各项与既有分支的并存策略（IS-1 仅就绪态、缺失态不加）；
- token 预警非阻断（OOS-2）；
- ProxyForm 纯文案不联动（OOS-1）；
- Proxies 空态在现有文案上补充不重复（IS-4）。

Architect 需细化的实现层选择（放置位置、Proxies 空态选"仪表盘"还是"监控"或两者、token trim 比较细节 BC-4）属设计决策，留给 Stage 2，不构成需求层模糊。

## 9. 裁决

**READY** —— 无遗留开放问题，可进入 Stage 2（Solution Architect）。
