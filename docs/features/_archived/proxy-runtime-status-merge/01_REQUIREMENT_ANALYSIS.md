# 01_REQUIREMENT_ANALYSIS — T-042 proxy-runtime-status-merge

> Stage 1 / Requirement Analyst · 2026-05-28
>
> 任务起源：批次 `frps-monitor-and-mgmt-suite` 第 4/4 个最后任务。用户原始需求：在既有 Proxies 配置页直接看到"我配的这条 proxy 现在到底有没有跑起来 / 流量多少"，避免来回切到 ServerMonitor 页查。

## 1. 上下文与已知能力

| 资产 | 来源任务 | 当前状态 |
|---|---|---|
| `useServerRuntime` composable（双 endpoint Promise.all + 5s polling + visibility 暂停 + 3 次失败自停 + onUnmounted 清理） | T-041 (commit pending archive) | DELIVERED，可直接复用 |
| `apiGetServerRuntimeInfo / apiGetServerRuntimeProxies` API 客户端 | T-041 | DELIVERED |
| `GET /api/v1/server/runtime/proxies` 返回 `{ proxies: Record<type, Status[]>, errors?: Record<type, string> }` | T-039 (commit ecc49b9) | DELIVERED |
| `Proxies.vue` 配置态表格（name / type / 本地地址 / 远程端口/域名 / 启用 / 操作）+ ProxyForm 新增/编辑/删除模态 | T-001 / T-032 / T-037 | DELIVERED，CRUD 通路工作正常 |
| `formatBytes(n)` / `formatTime(s)` 行内辅助函数 | T-041 `ServerMonitor.vue` setup 内 | DELIVERED 但 **未抽取**，DRY 缺位 |
| status → color 映射（online=success / offline=default / 其它=error）+ "在线/离线/未知" 文案 | T-041 `ServerMonitor.vue` columns 内 | DELIVERED 但 **未抽取**，DRY 缺位 |

## 2. 用户故事

**作为** 运维 / FRP 管理员（已经在 Proxies 配置页编辑端口转发规则），
**我希望** 在不离开本页的前提下，每条 proxy 行右侧直接看到"它在 frps 端实际跑没跑起来 / 累计流量多少 / 当前连接数"，
**这样** 我不必每改完一条规则就切到 ServerMonitor 页核对一次，调试与日常巡检都能"配置即所见"。

## 3. 范围（精确定义）

### 3.1 In-scope（必须做）

1. **Proxies.vue 表格右侧叠加 2 列**：
   - **"运行状态"列**：圆点 + 文字
     - 绿点 + "运行中"（runtime status === "online"）
     - 灰点 + "离线"（runtime status === "offline" 或 runtime 无该 proxy 名）
     - 红点 + 状态原文（其它）
     - tooltip：状态文本 + lastStartTime（formatTime 处理 "0001-..." → "—"）+ 最近错误（runtime.errors[type] 若存在）
   - **"流量（入/出）"列**：人类友好单位（B/KiB/MiB/GiB/TiB）
     - tooltip：当前连接数 `curConns`
2. **抽取共享 utils**：
   - `web/src/utils/format.ts` 导出 `formatBytes(n)` / `formatTime(s)`
   - `web/src/utils/proxyStatus.ts` 导出 `getProxyStatusTag(raw)` → `{ type: 'success'|'default'|'error', text: string, dotColor: string }`
   - `ServerMonitor.vue` 同步切到引用这两个 utils（**内部 refactor，外部行为零变**）
3. **匹配规则**：以 `proxy.name` 为主键，从 `useServerRuntime().proxies.value.proxies[type]` 数组按 name 查找；type 用配置态的 `proxy.type`。
4. **降级（degradation）**：frps 未运行 / 503 / 凭据失败 → useServerRuntime 进入 first-fail 或 stale 态 → runtime 列**全部**渲染为灰点 + 文案 "frps 未运行 / 监控不可用"；**配置 CRUD 仍正常工作**（不阻塞既有 fetchProxies / createProxy / updateProxy / deleteProxy 任何一条通路）。
5. **测试覆盖**：
   - `Proxies.spec.ts` 新增 mount × 多态用例：runtime running / offline / error 三态视觉差异 + 反向构造（配置态 vs runtime 集合的 4 种组合）+ frps 未运行降级用例
   - `format.spec.ts`：`formatBytes`（0 / 1023 / 1024 / 1536 / 1MiB / 1GiB / 超大 / undefined / NaN / 负数）；`formatTime`（空 / "0001-..." / 正常字符串）
   - `proxyStatus.spec.ts`：online / Online（大写）/ offline / error / undefined / 空字符串
6. **文档同步**：
   - `docs/dev-map.md` 在"可复用工具"段新增 `utils/format.ts` + `utils/proxyStatus.ts` 行；"目录布局"段 `pages/Proxies.vue` 备注追加"T-042 叠 runtime 列"

### 3.2 Out-of-scope（明确不做）

| 项 | 理由 |
|---|---|
| 在 Proxies.vue 内显示 ServerMonitor 才有的"未配置但 runtime 有"proxy 行 | 与本任务"配置即所见"语义冲突；用户找未在配置态的 proxy 应去 ServerMonitor 看 |
| 调 traffic 时序 `/api/v1/server/runtime/traffic/{name}`（折线图） | T-041 已留 API 客户端；本任务不开 UI |
| 修改 ProxyForm 新增 / 编辑 / 删除按钮、模态、表单行为 | 零回归红线 |
| 新增后端 API / openapi.yaml | 无后端改动 |
| 修改 useServerRuntime 内部行为（polling 节拍 / 失败阈值 / visibility） | T-041 已锁定单向数据流（insight L13），破坏会引入回归 |
| 新增 verify_all step | insight L26 要求新增 step ADV ≥ 4，本任务无需新增（grep 守门已被 T-037 H.1 覆盖删除面，UI 叠加层无独立静态闸门必要） |

### 3.3 Out-of-scope but worth noting

- **状态列与"启用"列共存的列名歧义**（insight L29）：既有"启用"列展示配置态 `proxy.enabled`，本任务新增"运行状态"列展示 runtime `status`。两列**必须分开**（不可合并成"状态"单列），否则触发 L29 陷阱（同列名不同语义源）。本任务采用列标题"运行状态"明确语义。

## 4. 验收准则（AC）

| ID | 准则 | 验证手段 |
|---|---|---|
| AC-1 | 用户在 Proxies 页能看到每条 proxy 行的运行态列（圆点 + 文字），无需切页 | Vitest mount happy path：配置 1 条 "ssh" tcp + runtime tcp 数组含 "ssh" online → text 包含 "运行中"，DOM 含 success 色 tag |
| AC-2 | 流量列展示 in/out 人类友好单位（B/KiB/MiB/GiB/TiB） | Vitest 检查文本含 "1.5 KiB" / "1 MiB" 等 |
| AC-3 | tooltip 展示 status 原文 + lastStartTime + 当前连接数 | Vitest 检查 NTooltip 子节点文案（component 内显式 trigger=hover popover content） |
| AC-4 | 配置态有 proxy "web" 但 runtime 无 → 灰点 + "离线" | Vitest 反向构造：proxiesStore.proxies 含 web，apiGetServerRuntimeProxies → `{ proxies: {} }` → 该行运行状态列含 "离线" 灰点 |
| AC-5 | runtime 有 proxy "abc" 但配置态无 → Proxies 表格不出现 abc 行（不影响表格） | Vitest 反向构造：检查 Proxies 表格 row 数等于配置态长度，无 "abc" 文本 |
| AC-6 | frps 未运行（API 抛错）→ runtime 列全灰 + 文案 "监控不可用" + 配置 CRUD 仍能调用既有 API | Vitest 反向构造：apiGetServerRuntimeProxies reject + 检查 store.fetchProxies / store.createProxy 各 spy 仍正常调用 |
| AC-7 | utils/format.ts `formatBytes` 边界值（0 / undefined / NaN / 负数 / 1023 / 1024 / 1MiB / 1GiB / Number.MAX_SAFE_INTEGER）均返回合理字符串 | unit test |
| AC-8 | utils/proxyStatus.ts `getProxyStatusTag` 大小写防御（Online → success） | unit test |
| AC-9 | ServerMonitor.vue 切到引用 utils 后，既有 ServerMonitor.spec.ts 全部用例继续 PASS（外部行为零变） | 跑既有 spec |
| AC-10 | Proxies.vue script 段纯逻辑行数 < 200（insight L31） | wc -l 自检：去 import / 注释 / interface |
| AC-11 | 单向数据流不破坏（insight L13）：Proxies.vue 只读 useServerRuntime 返回的 ref，不 v-model 绑回 | 代码 review + grep `v-model:` 在 Proxies.vue 内不绑 runtime ref |
| AC-12 | 06_TEST_REPORT.md `## Adversarial tests` 段裸标题（insight L41） | verify_all E.6 PASS |
| AC-13 | 整 batch verify_all `FAIL ≤ 1`（baseline） | 跑 `pwsh scripts/verify_all.ps1` |

## 5. 非功能性约束

- **性能**：Proxies.vue 每次 polling tick（5s）需在表格上做 N（≤ 100 条）× M（≤ 7 type）× O(1) 的 name 查找。可接受方案：从 useServerRuntime.proxies.value 建一个 `Map<string, ProxyStatus>`（key=name）computed 缓存，避免每行 render 都遍历。
- **可访问性**：圆点 + 文字双通道（不靠颜色单一信号）。
- **i18n**：项目当前中文 only，无 i18n 框架；文案硬编码可接受。
- **维护性**：utils 函数纯函数 + 全部边界值有 unit test；ServerMonitor 与 Proxies 共用同一份实现，未来调整流量单位 / 状态色映射只改一处。

## 6. 风险与缓解

| 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|
| useServerRuntime 在 Proxies.vue 卸载时若没自清，会泄漏 timer | 低 | 中（5s 一次内存泄漏） | T-041 已实现 onUnmounted 自清（insight L11），本任务**禁止**在 Proxies.vue 内手工再 stop / cleanup（让 composable 自管） |
| 配置态 N 大 + runtime 多 type → 每 render 遍历 O(N×M) | 低 | 低（N ≤ 100） | computed 化 Map，挂载于 setup 顶层一次 |
| frps 未运行 → useServerRuntime 进入 stale → Proxies.vue 还在调它的 start() 会 spam 失败请求 | 中 | 中 | T-041 已实现"3 次失败自停"机制（D-6），延续即可；Proxies.vue 不开 banner，因为 banner 已在 ServerMonitor 页提供，Proxies 页只静默降级（灰点 + tooltip） |
| Proxies.vue script 段超 200 行触发 insight L31 红线 | 中 | 中 | utils 抽取已减负；预估抽完后 < 200 |
| 既有 Proxies.spec.ts 因新列改动而 broken | 高 | 中 | 当前 `web/src/components/__tests__/` 与 `web/src/pages/__tests__/` 中**没有** Proxies.spec.ts（实测 Glob 命中 0）；本任务**新建**该文件，不存在 broken 风险 |
| utils 抽取后 ServerMonitor.vue 行为漂移 | 低 | 高 | 严格按 T-041 内联实现字节级搬运到 utils；既有 ServerMonitor.spec.ts 全部用例不改一行作为回归守门 |

## 7. 相关历史任务

| 任务 | 关联点 |
|---|---|
| T-001 web-ui-mvp | Proxies.vue 原型 |
| T-032 proxy-form-vmodel-oom-fix | 单向数据流范式（必须延续，禁 v-model 桥） |
| T-037 proxy-rules-simplify-and-port-fix | Proxies.vue 最近一次大改（移除批量 / 折叠组 / 端口探测）；本任务在其之上叠加 |
| T-039 frpsadmin-server-runtime-api | 后端 API（数据源） |
| T-041 server-monitor-page-ui | `useServerRuntime` composable + utils 原型来源 |
| T-007 hardening-pass-audit | qa_t007_adversarial.spec.ts 范式参考 |
| T-036 log-ui-ux-polish | 多组件 / utils 抽取 + spec.ts 守门范式 |

## 8. 决策原则映射（用户传达 → 本任务实施）

| 用户原则 | 本任务体现 |
|---|---|
| 用户体验好 | 单视图聚合配置态/运行态；frps 不可用时降级 + CRUD 不挂 |
| 软件工程标准 | DRY 抽 utils；mount × 多态 + 反向构造测试；utils 边界值单测 |
| 长期易使用易维护 | utils 可复用；既有列零改动；单向数据流；SFC 行数自检 |

## 9. 给下游的硬约束

- Architect：**必须**沿用 T-041 的 useServerRuntime 实例（每个 Proxies.vue 一份；不增 polling 通道）；utils 抽取必须字节级搬运（不优化 / 不重命名 / 不改算法），避免 ServerMonitor 行为漂移。
- Developer：**禁**在 Proxies.vue 内自己写 setInterval / addEventListener 任何形态；**禁**在 useServerRuntime 上加新 ref / 新方法；**禁**修改 ServerMonitor.vue 任何渲染分支（只允许 import 替换）。
- QA：测试报告必须用 `## Adversarial tests` **裸标题**（insight L41）；至少覆盖本节 AC-4 / AC-5 / AC-6 反向构造。

— end —
