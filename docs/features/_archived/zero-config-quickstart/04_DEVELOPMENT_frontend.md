# Development Record — Frontend partition

## Partition
dev-frontend — owns: `web/**`

## Files changed (this partition only)

### New files
- `web/src/api/downloader.ts` — POST /download-bin + GET /download-status/{kind}
- `web/src/api/wizard.ts` — GET /wizard/status + POST /wizard/complete
- `web/src/stores/downloader.ts` — Pinia store; frpc/frps DownloadState + 1s 轮询
- `web/src/stores/wizard.ts` — Pinia store; checkWizard / completeWizard
- `web/src/components/FirewallHint.vue` — ufw + iptables 提示组件（ports[] prop，v-if="ports.length > 0"）
- `web/src/components/PublicIpDetector.vue` — 公网 IP 检测按钮 + 结果展示
- `web/src/pages/Wizard.vue` — 部署向导页面（顶层独立路由 /wizard，无 AppLayout）

### Modified files
- `web/src/types.ts` — 追加 DownloadState / PublicIPResponse / WizardStatus / DownloadBinRequest
- `web/src/api/system.ts` — 追加 apiGetPublicIP
- `web/src/router.ts` — 添加 /wizard 顶层路由 + beforeEach 守卫逻辑
- `web/src/pages/Server.vue` — bindPort 字段上方嵌入 PublicIpDetector；保存成功后展示 FirewallHint
- `web/src/components/AppLayout.vue` — 二进制缺失 banner 中每个缺失项添加一键下载按钮
- `web/src/pages/Proxies.vue` — 代理创建/更新成功后展示 FirewallHint
- `web/src/docs/dev-map.md` — 新增模块记录

## Wizard 步骤流程

步骤 1 — 选择角色：
- 单选三项：仅 frpc / 仅 frps / 两者都配置
- 未选直接点"下一步"给提示，不跳转

步骤 2 — 填写配置：
- 仅 frpc：serverAddr（必填）+ serverPort（默认 7000）
- 仅 frps：bindPort（默认 7000）
- 两者：同时展示 frps + frpc 配置字段
- 点"完成配置"：调 PUT /server 和/或 PUT /client 保存，再调 PUT /mode 启用，最后 POST /wizard/complete，router.push('/dashboard')

步骤 3 — 完成：
- 显示成功图标 + 说明文字，自动跳转 /dashboard

跳过按钮（任何步骤均可）：
- 直接调 POST /wizard/complete，router.push('/dashboard')

## FirewallHint 组件 Props

| Prop | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `ports` | `number[]` | 是 | — | 需要开放的端口列表；空数组时组件不渲染（v-if） |
| `label` | `string` | 否 | `'在 frps 服务器上执行以下命令开放端口：'` | NAlert 标题 |

展示内容：每个端口输出 4 条命令（ufw tcp/udp + iptables tcp/udp），每条命令有独立复制按钮，复制成功后 2 秒内显示"已复制 ✓"。组件可关闭（NAlert closable）。

## PublicIpDetector 组件

无 props。内部状态管理：
- loading：按钮 loading 状态
- result：PublicIPResponse | null
- copied：复制 IP 按钮状态

行为：点击"检测公网 IP"调 apiGetPublicIP()；成功显示 NAlert success + IP + 复制按钮，有 advisory 时附加说明；失败显示 NAlert warning + error 信息。结果不自动填入任何表单字段。

## 路由守卫逻辑

在 beforeEach 中，当满足以下条件时触发 wizard 检查：
1. auth.user !== null（已登录）
2. to.path === '/dashboard'（正在导航到 dashboard）
3. wizard.checked === false（本 session 未检查过）

调用 wizardStore.checkWizard()，若 shouldShow=true 则返回 '/wizard'，否则继续导航。checked=true 后不再重复调用 API。

/wizard 定义为顶层路由（与 /setup、/login 同级），不嵌套于 AppLayout children。

## Out-of-partition coordination

后端 API 端点（GET /api/v1/system/public-ip、POST /api/v1/system/download-bin、GET /api/v1/system/download-status/{kind}、GET /api/v1/wizard/status、POST /api/v1/wizard/complete）由 dev-backend 在 internal/httpapi/ 实现。前端按 02_SOLUTION_DESIGN.md §5 契约对齐，无需等待后端即可构建（TypeScript 类型检查不依赖真实 API）。

## npm run build 结果

```
vue-tsc --noEmit && vite build
vite v5.4.21 building for production...
transforming...
✓ 2902 modules transformed.
✓ built in 2.70s
```

TypeCheck PASS，Vite build PASS，输出到 internal/assets/dist/。

## Verdict

READY FOR REVIEW (frontend partition complete)

---

### Code Review 修复（2026-05-16）

**修复问题：M-1（AC-14）— FirewallHint.vue 缺少"复制全部"按钮**

**修改文件：**
- `web/src/components/FirewallHint.vue` — 在 NAlert 底部添加"复制全部"按钮；点击后将所有端口的所有命令用 `\n` 连接后写入剪贴板；2 秒内显示"已复制全部 ✓"反馈。

**修复问题：M-2（AC-16）— FirewallHint.vue 协议无关命令生成**

**修改文件：**
- `web/src/components/FirewallHint.vue` — 添加 `proto` prop（类型 `'tcp' | 'udp' | 'both'`，默认 `'both'`）；`getCommands(port)` 根据 proto 过滤命令：`'tcp'` 只生成 tcp 的 ufw/iptables 规则，`'udp'` 只生成 udp 规则，`'both'` 保持全部 4 条命令；`getAllCommands()` 复用同一逻辑；同时更新 `copyAll`。
- `web/src/pages/Server.vue` — `<firewall-hint>` 增加 `proto="tcp"`（frps bindPort 和 dashboardPort 均为 TCP）。
- `web/src/pages/Proxies.vue` — 新增 `firewallProto` ref（类型 `'tcp' | 'udp' | 'both'`，默认 `'both'`）；`handleSubmit` 中根据 `savedProxy.type` 赋值（tcp→`'tcp'`，udp→`'udp'`）；`<firewall-hint>` 绑定 `:proto="firewallProto"`。

**修复问题：N-1（B-8, B-9）— Wizard.vue 缺少 auth.token 字段**

**修改文件：**
- `web/src/pages/Wizard.vue` — frps 表单（Step 2）新增"鉴权 Token"输入字段（NInput, type=password, 可选）；frpc 表单新增同样字段；`frpsForm.authToken` 和 `frpcForm.authToken` 初始化为空字符串；`handleNext` 中 `apiPutServer` 和 `apiPutClient` 调用时传入 `authToken` 和 `authMethod`（token 非空时才设置）。

**修复问题：N-3（B-2, NF-U4）— AppLayout.vue 下载进度改为视觉进度条**

**修改文件：**
- `web/src/components/AppLayout.vue` — 引入 `NProgress`；按钮标签从"下载中 42%"改为"下载中..."；在每个下载按钮下方添加 `<n-progress type="line">` 组件（height=4, 无 indicator）；仅在 `status === 'downloading'` 时显示。

**修复问题：N-4（B-4, AC-4）— AppLayout.vue 下载失败链接改为可点击超链接**

**修改文件：**
- `web/src/components/AppLayout.vue` — `handleDownload` 中失败 toast 从 `message.error(string)` 改为 `message.error(() => h('span', ...))` 渲染函数，将 GitHub Releases URL 包装为 `<a href="..." target="_blank">手动下载</a>` 可点击超链接。`h` 函数已在原有 import 中。

**构建验证：**
```
vue-tsc --noEmit && vite build
✓ 2902 modules transformed.
✓ built in 2.77s
```
TypeCheck PASS，Vite build PASS。

**verify_all 结果（修复后）：**
PASS 12 / WARN 0 / FAIL 0 / SKIP 6（delta: 0 新失败）
