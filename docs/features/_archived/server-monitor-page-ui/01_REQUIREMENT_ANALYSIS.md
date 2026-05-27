# 01 — Requirement Analysis · T-041 server-monitor-page-ui

> Stage 1 / 7。把用户原始需求拆成可验收的 FR / NFR / AC，不写"怎么做"。

## 1. 用户原话

> 查看所有 frpc 的在线状态、连接状态。新页面 UI 集中展示。

承接批次 `frps-monitor-and-mgmt-suite`：第 3/4 个任务，依赖 T-039（已 DELIVERED，commit ecc49b9）。本任务范围由 batch PM 收敛为"纯前端、独立监控页"，**不改后端**。后续 T-042 才把运行态合并到 Proxies.vue（不在本任务）。

## 2. 历史关联

| 历史任务 | 关联 | 怎样关联 |
|---|---|---|
| T-039 frpsadmin-server-runtime-api | **强依赖** | 直接消费 `/api/v1/server/runtime/info` + `/proxies` + `/traffic/{name}` 三条 REST |
| T-036 log-ui-ux-polish | **范式** | 5 个 composable + 壳组件 + useThemeVars + CSS 变量、mount × 多态 spec、SFC 拆分判断、message 6 方法 stub |
| T-032 proxy-form-vmodel-oom-fix | **范式** | 单向数据流；composable 暴露 ref 让壳 watch 时不引入回环 |
| T-038 boot-autostart-hardening | **范式** | ServiceStatusCard.vue 的 5s 轮询 + visibilitychange 暂停模式（可作 useServerRuntime 借鉴） |
| T-040 frps-allow-ports-policy | 并行 batch | 共享 Server.vue 域；本任务只新增页面，不动 Server.vue（无冲突） |

## 3. 目标 / 非目标

### 3.1 目标

- **G-1**：用户在新页面一处看完 frps 服务端"现在的全貌"——server 元信息 + 所有连接 / 所有 proxy 当前状态 + 流量。
- **G-2**：5 秒自动刷新让数据"足够实时但不抖屏"；用户可暂停 / 手动 refresh / tab 切后台自动暂停省资源。
- **G-3**：三态（loading / empty / error）完备，error 时给出可执行的下一步引导，不只是"加载失败"。
- **G-4**：polling 逻辑抽 composable 让 T-042 直接 import 复用（"一处实现，两处消费"）。
- **G-5**：组件可维护性符合项目 SFC > 200 行红线（按 insight L28"纯逻辑行数"判，不算模板）。

### 3.2 非目标

- ❌ 不改后端（T-039 已经提供数据源）
- ❌ 不合并到 Proxies.vue（T-042 负责）
- ❌ 不做历史趋势图 / 时间序列可视化（T-039 traffic API 暂只在 ProxyDetail 抽屉里点击展开时一次性调用，不做图表绘制）
- ❌ 不做 proxy 启停 / 删除 / 编辑（只读视图，写操作在既有 Proxies.vue 完成）
- ❌ 不做 dashboard 凭据配置 UI（在既有 Server.vue 已具备）
- ❌ 不做权限分级（项目当前单管理员模型）

## 4. 假设

- **A-1**：用户已在 Server.vue 启用 Dashboard 开关 + 保存 → frps.toml 含 [webServer] 段 + frps 已重启加载。本任务在 dashboard 未启用时返 503 + 引导。
- **A-2**：T-039 API 形状不变（详见下方 §5.4 接口契约）。
- **A-3**：MVP 单 frps 场景，proxy 总数 ≤ 200（性能假设；超规模另议）。
- **A-4**：浏览器支持 `document.visibilityState`（Chrome 33+ / Firefox 18+ / Safari 7+，覆盖项目目标用户）。
- **A-5**：用户对"暂停"按钮和"手动 refresh"按钮的语义有共识（无需教学态）。

## 5. 功能需求（FR）

### 5.1 FR-1 ServerInfo 卡片（顶部）

| ID | 要求 |
|---|---|
| FR-1.1 | 显示 frps 版本（`info.version`） |
| FR-1.2 | 显示总客户端连接数（`info.clientCounts`） |
| FR-1.3 | 显示总当前连接数（`info.curConns`） |
| FR-1.4 | 显示总流量 in / out（`info.totalTrafficIn` / `totalTrafficOut`），人类友好单位 KB / MiB / GiB（1024 进制；与项目其它字节量同款） |
| FR-1.5 | 显示运行时长（**T-041 决策点 D-1**：T-039 ServerInfo 不直接返回 uptime；从 `bindPort` 段元信息合成不可靠。**收敛为 D-1：本任务不展示 uptime，FR-1.5 降级为"展示 server 监听端口 bindPort + dashboard 端口"作元信息**。后续若 T-039 扩 uptime 字段再补） |
| FR-1.6 | 卡片三态：loading → empty → 数据；error 时整卡片置为 error 态（不显示陈旧 0 数字误导） |

### 5.2 FR-2 Proxies 表格（中部）

| ID | 要求 |
|---|---|
| FR-2.1 | 数据源：`/api/v1/server/runtime/proxies` 响应 `{ proxies: { tcp: [], udp: [], http: [], ... }, errors?: { type: 错误文案 } }` |
| FR-2.2 | 按 type 分组展示。**T-041 决策点 D-2**：用 Naive UI `n-tabs` 标签页切换 type，每个 tab 显示该 type 的表格。空 type tab 显示"暂无 X 类型 proxy"，error type tab 显示 errors[type] 文案 |
| FR-2.3 | 每行字段：name / status（绿/灰/红 三色 dot） / 当前连接数 `curConns` / 累计流量 in `todayTrafficIn` / 累计流量 out `todayTrafficOut`（人类友好单位）/ lastStartTime / lastCloseTime |
| FR-2.4 | status 三态颜色映射：`status === "online"` 绿、`status === "offline"` 灰、其它（含 "error"）红。**T-041 决策点 D-3**：frps 上游实测只有 online/offline 两态，红色保留兜底（未来 frps 扩 status 字段或 conf 段有 error 字段时容纳） |
| FR-2.5 | 时间字段 lastStartTime / lastCloseTime 形如 frps 原生 ISO 字符串。**T-041 决策点 D-4**：直接显示（不做相对时间化"3 分钟前"，避免 polling 抖屏 + 跨时区误判）。空字符串 / "0001-01-01..." 显示"—" |
| FR-2.6 | 表格三态：loading（首屏 skeleton）/ empty（"暂无连接的 proxy"）/ error（按 type 显示 errors[type] 文案，整体仍可渲染其它成功 type） |

### 5.3 FR-3 顶部状态条

| ID | 要求 |
|---|---|
| FR-3.1 | 显示 `lastUpdated` 时间（相对：刚刚 / N 秒前 / N 分钟前 / N 小时前；刷新时跟随更新） |
| FR-3.2 | "暂停/恢复轮询"按钮，按钮文案根据 `isPolling` 切换 |
| FR-3.3 | "立即刷新"按钮（手动 refresh，不依赖 5s 节拍） |
| FR-3.4 | 当 polling 因连续 3 次错误自动停止时，显示红色提示条 + "重新启动轮询"按钮 |
| FR-3.5 | 当 tab 切后台 polling 自动暂停时，恢复 visibility 时自动复位（无需用户操作）。本条**不**需要任何 UI 文案——静默切换 |

### 5.4 FR-4 API client

新增 `web/src/api/serverRuntime.ts`，导出三个函数：

```typescript
apiGetServerRuntimeInfo(): Promise<ServerRuntimeInfo>
apiGetServerRuntimeProxies(): Promise<ServerRuntimeProxiesResponse>
apiGetServerRuntimeTraffic(name: string): Promise<ServerRuntimeTraffic>
```

类型同步到 `web/src/types.ts`：

```typescript
export interface ServerRuntimeInfo {
  version?: string
  bindPort?: number
  kcpBindPort?: number
  quicBindPort?: number
  vhostHTTPPort?: number
  vhostHTTPSPort?: number
  subdomainHost?: string
  clientCounts: number
  curConns: number
  proxyTypeCount?: Record<string, number>
  totalTrafficIn?: number
  totalTrafficOut?: number
}

export interface ServerRuntimeProxyStatus {
  name: string
  type?: string
  status?: string  // "online" | "offline" | 兜底其它
  lastStartTime?: string
  lastCloseTime?: string
  todayTrafficIn?: number
  todayTrafficOut?: number
  curConns?: number
  clientVersion?: string
  // conf 上游 map[string]any，前端透传不解；只在 ProxyDetail 抽屉里展示
}

export interface ServerRuntimeProxiesResponse {
  proxies: Record<string, ServerRuntimeProxyStatus[]>
  errors?: Record<string, string>
}

export interface ServerRuntimeTraffic {
  name: string
  trafficIn: number[]
  trafficOut: number[]
}
```

### 5.5 FR-5 useServerRuntime composable

`web/src/composables/useServerRuntime.ts`：

```typescript
export interface UseServerRuntimeReturn {
  info: Ref<ServerRuntimeInfo | null>
  proxies: Ref<ServerRuntimeProxiesResponse | null>
  isPolling: Ref<boolean>
  error: Ref<string | null>
  lastUpdated: Ref<number>
  start: () => void
  stop: () => void
  refresh: () => Promise<void>
}

export function useServerRuntime(intervalMs?: number): UseServerRuntimeReturn
```

行为契约：
- F-5.1：默认 `intervalMs = 5000`
- F-5.2：`start()` 启动 setInterval；`stop()` 清除
- F-5.3：自动暂停在 `document.hidden` 触发（监听 visibilitychange）；恢复时若之前在 polling 则自动 start
- F-5.4：组件 onUnmounted 自动 stop + 解绑 visibilitychange listener（避免内存泄漏）
- F-5.5：`refresh()` 立即拉一次，不依赖 timer
- F-5.6：拉取失败保留上一次 info / proxies（不清空）；error 写入 `error` ref；连续 3 次失败 → 自动 stop + error 文案"轮询已停止：连续 N 次失败"（与 useLogBuffer BC-6 范式对齐）
- F-5.7：组件初次 mount 时不自动 start（让壳组件显式 `start()`，避免 SSR / 测试环境意外副作用）

### 5.6 FR-6 ServerMonitor.vue 页面

| ID | 要求 |
|---|---|
| FR-6.1 | 文件 `web/src/pages/ServerMonitor.vue`，setup script，使用 useServerRuntime |
| FR-6.2 | onMounted 调 `start()` + `refresh()`；onUnmounted 由 composable 内部清理 |
| FR-6.3 | 顶部 ServerInfo 卡片（FR-1）+ 顶部状态条（FR-3）+ 中部 Proxies 表格（FR-2） |
| FR-6.4 | 整体 wrap 在 `<n-card>` 或类似容器；与既有 Server.vue / Dashboard.vue 视觉风格一致 |

### 5.7 FR-7 路由 + 导航入口

| ID | 要求 |
|---|---|
| FR-7.1 | `web/src/router.ts` 加 `{ path: 'server/monitor', component: ServerMonitor.vue }`（嵌在 AppLayout 子路由下，自然继承 SessionAuth 守护） |
| FR-7.2 | `web/src/components/AppLayout.vue` 的 menuOptions 加菜单项"服务端监控"，key `'server/monitor'`，置于"服务端配置"之后 |
| FR-7.3 | activeKey 计算逻辑兼容多段路径（既有逻辑用了 `path.startsWith('/logs/')` 特判；本任务加 `'/server/monitor'` 也走同款分支） |

### 5.8 FR-8 dev-map.md 同步

新增条目：
- 目录布局 `web/src/`：`pages/ServerMonitor.vue` / `composables/useServerRuntime.ts` / `api/serverRuntime.ts`
- 功能在哪里表：3 行
- 可复用工具表：useServerRuntime 一行（强调 T-042 也消费）

## 6. 非功能需求（NFR）

| ID | 要求 |
|---|---|
| NFR-1 (性能) | mount + 首次 refresh 完成 ≤ 1 s（happy-dom 测）；5 s 节拍 polling 不阻塞 main thread > 50 ms |
| NFR-2 (内存) | 长时间 polling（4 小时）不出现内存膨胀；composable 在 stop 时清空所有 timer / listener |
| NFR-3 (bundle) | 新增页面 chunk gzip 增量 ≤ 15 KB（参考 T-036 LogViewer 5.40 KB 实测，本任务无图表无大依赖） |
| NFR-4 (主题) | 支持 light / dark 切换实时跟随（沿 LogViewer.vue useThemeVars 范本；状态 dot 颜色用 themeVars.successColor / warningColor / errorColor） |
| NFR-5 (无障碍) | 表格 / 卡片用 semantic HTML（n-table th + scope）；polling 暂停 / 恢复按钮 aria-label 完备 |
| NFR-6 (无依赖) | 不引入新 npm 包（用既有 naive-ui + vue + axios） |
| NFR-7 (XSS) | proxy name / lastStartTime 等字段从 frps 上游来；用 Vue 文本插值（双花括号）自动 escape，不走 v-html |

## 7. 验收标准（AC）

| ID | 给定 | 操作 | 期望 |
|---|---|---|---|
| AC-1 | dashboard 已启用 + frps 跑 + 至少 1 个 proxy | 打开 /server/monitor | 看到 server 元信息 + proxy 列表非空 + lastUpdated 显示"刚刚" |
| AC-2 | 同 AC-1 状态下停留 6 秒 | 不操作 | 表格数据自动刷新一次；lastUpdated 跳到"刚刚" |
| AC-3 | dashboard **未**启用 | 打开 /server/monitor | 看到友好文案"frps dashboard 未启用。请到 Server 设置页打开 Dashboard 开关并保存。"+ 不崩溃 |
| AC-4 | frps 进程**未跑** | 打开 /server/monitor | 看到"frps 进程不可达..."+ retry 按钮可见 |
| AC-5 | dashboard 凭据错（手工改 frps.toml） | 打开 /server/monitor | 看到"frps dashboard 凭据校验失败..."+ retry 按钮可见 |
| AC-6 | 网络中断（mock fetch reject） | polling 中拉 1 次失败 | 上一次 info / proxies 保留；顶部红色文案"连接断开"或类似 |
| AC-7 | 切到后台 tab | document.hidden = true 触发 | polling 暂停（setInterval 不再触发）；恢复 visible 时自动恢复 |
| AC-8 | 点"暂停"按钮 | onClick | isPolling=false；按钮文案变"恢复轮询"；点恢复后 isPolling=true |
| AC-9 | 点"立即刷新" | onClick | API 调一次 + lastUpdated 跳"刚刚"，不影响 5 s 节拍 |
| AC-10 | 组件 unmount | router 切走 | 所有 timer + visibilitychange listener 解绑（spy 验证） |
| AC-11 | 连续 3 次 polling 失败 | mock 3 次 reject | polling 自动 stop；显示"连续 N 次失败"+ "重启轮询"按钮 |
| AC-12 | 后端返回部分 type errors（例 tcp 成功 / xtcp 错误） | refresh | tcp tab 正常显示数据；xtcp tab 显示 errors[xtcp] 文案 |
| AC-13 | 流量字段 1024 字节 | 渲染 | 显示 "1 KiB"；1.5 MiB 显示 "1.5 MiB"；0 显示 "0 B" |
| AC-14 | proxy status="online" | 渲染 | 状态 dot 绿色 |
| AC-15 | proxy status="offline" | 渲染 | 状态 dot 灰色 |

## 8. 边界 / 反向 case（BC）

| ID | 场景 | 期望 |
|---|---|---|
| BC-1 | API 返回 `proxies: {}` 空 map | 显示"暂无连接的 proxy" |
| BC-2 | API 返回 `info.totalTrafficIn = 0` | 显示 "0 B"（不显示空） |
| BC-3 | proxy.lastStartTime 为空字符串 | 显示 "—" |
| BC-4 | 用户在 polling 间隔内频繁点"立即刷新" | 并发请求不引起 UI flicker；最后一次响应胜出（用 epoch race 同款 T-036 BC-5） |
| BC-5 | 组件 mount 后立即 unmount（< 100 ms） | 已发出的请求响应到达时不再写组件（避免 Vue warn / unhandled promise） |
| BC-6 | tab 后台切回 + 立即手动 refresh | 不重复 setInterval；refresh 走单次 promise |
| BC-7 | 用户暂停轮询后切后台 → 切回 | 不自动恢复（用户意图优先） |
| BC-8 | API timeout 中断 | error 文案显示 "请求超时"；保留上一次数据 |
| BC-9 | 上游返 200 但 body 非法 JSON | catch error；显示通用错误文案 |

## 9. 错误 / 错误恢复

| 错误来源 | 表现 | 恢复 |
|---|---|---|
| 网络中断 | error ref 写入；保留上一次数据；UI 顶部红色提示条 | tab 重新可见 + 网络恢复时下次 5 s 节拍自动恢复 |
| 后端 503 dashboard 未启用 | 友好引导文案 + 引用 /server 入口锚 | 用户点链接到 Server.vue 启用 |
| 后端 502 凭据错 | 文案引导"清空 user/pass 重生成" | 用户操作 |
| 后端 503 frps 不可达 | 文案引导"启动 frps" + retry 按钮 | 用户启动后点 retry |
| 连续 3 次失败 | 自动 stop polling + 红色 banner | "重启轮询"按钮手动恢复 |

## 10. 决策矩阵

| ID | 决策 | 备选 | 选择理由 |
|---|---|---|---|
| D-1 | 不展示 uptime | 展示 / 不展示 | T-039 API 未返回 uptime；从其它字段不可靠合成。本任务范围 KISS |
| D-2 | n-tabs 切 type 分组 | 单表 + group | 单 frps 场景 type 数量 ≤ 7，tabs UX 清晰；group row 会触发 T-037 反模式（虚拟列 vs 字段同名） |
| D-3 | status 红色兜底 | 仅 online/offline | frps 未来扩字段时容纳；冗余成本极低 |
| D-4 | 直接显示时间字符串 | 相对时间（"3 分钟前"） | polling 抖屏 + 跨时区误判；用户监控场景对"具体时间"敏感 |
| D-5 | composable 暴露 start/stop/refresh 命令 | onMounted 自动 start | F-5.7 测试 / SSR 友好；显式调用易控 |
| D-6 | 连续 3 次失败 stop | 无限重试 | 与 T-036 useLogBuffer 范式对齐；避免无效请求堆积 |
| D-7 | polling 默认 5 s | 1 s / 2 s / 10 s | 用户体感"实时"与"后端压力"折衷；T-036 日志 2 s（流式更急）/ T-038 服务化状态 10 s（变化稀疏）；监控页中等档 5 s |
| D-8 | 不在本任务做 traffic 时序图 | 折线图 + traffic API | T-039 traffic API 已 ready 但绘图需依赖（echarts/chart.js），违反 NFR-6；后续任务可加 |
| D-9 | n-card 内嵌 ServerInfo + Tabs + 状态条 | 独立卡片 | 单页 KISS；与 Dashboard.vue 视觉一致 |
| D-10 | proxy 详情 ProxyDetail 抽屉本任务不做 | 抽屉点击展开 | 范围克制；表格行已含核心字段；ProxyDetail / Traffic 调用可作 T-042 增量 |

## 11. 风险 / 缓解

| ID | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R-1 | dashboard 未启用时用户在 /server/monitor 看到 503 + 不知道怎么办 | UX | FR-3 + AC-3 文案引导 + 路由跳 /server 按钮 |
| R-2 | 5 s 轮询在 100 个 proxy 时让 UI 抖屏 | 性能 / UX | proxy 表格 keyed by `name`（Vue diff 局部 patch）；不全表重渲染 |
| R-3 | composable 在 hot reload / unmount race 下 timer 泄漏 | 内存 | onUnmounted 显式 clearInterval + removeEventListener；epoch race 防过期响应写组件 |
| R-4 | 同时进 3 个 page tab 都开了 /server/monitor → frps dashboard 抗 3× 请求 | 性能 | dashboard 内置限流 + 5 s 间隔单 tab 单请求；MVP 不限 tab 数 |
| R-5 | T-039 API 字段未来加 `lastErr` / `clientAddr` 等 | UX | composable / API client 已用 omitempty 兼容；UI 后续按需加列 |
| R-6 | 用户用 Naive `n-tabs` 切 type 时 active tab state 跨 polling 刷新被重置 | UX | tabs activeKey 用 ref 持有，polling 只更新数据不动 activeKey |

## 12. RA self-check

- [x] 所有 FR 都有对应 AC ✔
- [x] 所有 AC 都可在 vitest mount + happy-dom 验证（无须真实 frps） ✔
- [x] 与 T-039 API contract 对齐（字段名 / sentinel 错误） ✔
- [x] 决策矩阵覆盖所有"为什么这样不那样"的 hot question ✔
- [x] 不写"怎么做"——具体技术选型留 02 ✔
- [x] 红线确认：纯前端、不动后端、不动现有 Proxies.vue ✔

---

**Verdict**：READY FOR ARCHITECT.
