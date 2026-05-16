# Code Review — T-002 zero-config-quickstart

**审查者**：Code Reviewer  
**日期**：2026-05-16  
**阶段**：Stage 5

---

## Verdict: CHANGES REQUIRED

## Summary

后端逻辑（下载器、向导 KV 处理、IP 检测）结构清晰，并发模型正确。但有两项 AC 失败：`FirewallHint` 协议无关（对所有端口同时显示 tcp 和 udp 命令，违反 AC-16 中"协议为 udp，不是 tcp"的要求），以及 AC-14 要求的"复制全部"按钮缺失。Windows 上原子化安装路径存在可能静默销毁现有有效二进制文件的边界情况（B-4）。此外，所有 5 个 T-002 HTTP handler 测试完全缺失（设计文档明确要求）。

---

## Findings

### MAJOR（必须修复）

**M-1 [LOGIC / AC-14] `web/src/components/FirewallHint.vue` — 缺少"复制全部"按钮**

AC-14 明确规定"代码块含'复制全部'按钮"；B-16 重复了同样的要求。`FirewallHint.vue` 只提供单行复制按钮，没有一次复制所有命令的按钮。这是明确命名的验收标准。

**M-2 [LOGIC / B-16, B-17, B-18, AC-16] `web/src/components/FirewallHint.vue` — 协议无关命令生成**

`getCommands(port)` 总是返回全部四条命令（ufw tcp、ufw udp、iptables tcp、iptables udp），不考虑代理类型。设计规范 §6.4 定义了 `{port, proto, label}` 结构，但前端开发者将其简化为 `number[]`。后果：
- B-16（frps bindPort）：只需 tcp 命令，当前也生成 udp 命令，对仅限 TCP 的绑定端口有误导性
- B-17（tcp 代理）：只需 tcp 命令，当前也生成 udp 命令
- B-18（udp 代理）：只需 udp 命令，当前也生成 tcp 命令。AC-16 明确说"协议为 udp，不是 tcp"

组件需要 `proto` prop（或更丰富的端口描述符对象）来按协议过滤。同时，所有调用方（`Server.vue`、`Proxies.vue`）也需要传入协议信息。

**M-3 [LOGIC / B-4] `internal/downloader/downloader.go:223` — Windows 上 Rename 失败可能静默销毁现有二进制文件**

```go
_ = os.Remove(targetPath)           // 错误被忽略
if err := os.Rename(binTmpPath, targetPath); err != nil {
    os.Remove(binTmpPath)
    m.setFailed(kind, ...)
    return
}
```

`os.Remove(targetPath)` 无条件调用且错误被丢弃。在 Windows 上，若 Remove 成功（文件未被使用）但 Rename 随后失败（如防病毒扫描临时文件，或磁盘满了），之前有效的二进制文件被删除且无法恢复。B-4 要求"已存在的有效二进制文件不被覆盖"。安全修复：仅在 Rename 失败时才 Remove 现有文件，或采用两步替换：先将现有文件重命名为 .bak，Rename 成功后再删除 .bak。

注：Linux 上 `os.Rename` 是原子的，不需要预先 Remove，风险仅在 Windows。

**M-4 [TESTS] `internal/httpapi/` — T-002 HTTP handler 测试完全缺失**

`httpapi_test.go` 和 `qa_ac_test.go` 中没有任何针对以下 5 个新端点的测试：
- `GET /api/v1/wizard/status` — `hasAnyConfig` 逻辑（4个 KV 检查组合）是 B-6/AC-6 的核心
- `POST /api/v1/wizard/complete` — KV 持久化未通过 HTTP 测试
- `POST /api/v1/system/download-bin` — 409 PROC_BUSY、422 bad-kind 路径未测试
- `GET /api/v1/system/download-status/{kind}` — 无效 kind 的 404 未测试
- `GET /api/v1/system/public-ip` — 5 分钟缓存 TTL、always-200 合约未测试

设计文档明确要求"T-002 新增测试覆盖 5 个新 AC 范围（AC-2、AC-6、AC-12/13、AC-14/15/16）"。

---

### MINOR（建议修复）

**N-1 [REQ / B-8, B-9] `web/src/pages/Wizard.vue` — 向导表单缺少 auth.token 字段**

B-8 列出 frpc 最简配置的三个字段：`serverAddr`、`serverPort` 和 `auth.token（可选）`。B-9 列出 frps 的两个字段：`bindPort` 和 `auth.token（可选）`。两个向导表单都不包含 token 字段。

**N-2 [REQ / B-10] `web/src/pages/Wizard.vue` — "both" 角色应顺序展示两个表单，而非同时显示**

B-10 规定"wizard 顺序展示 frps 配置表单（第一步）和 frpc 配置表单（第二步）"。当 `selectedRole === 'both'` 时，当前实现在同一步骤页面上同时渲染两个表单，不符合 B-10 的顺序设计意图。

**N-3 [DESIGN / NF-U4, B-2] `web/src/components/AppLayout.vue` — 无视觉进度条**

B-2 说"UI 显示进度条（0–100%）"，NF-U4 要求"进度条动画平滑无跳变"。当前实现显示"下载中 42%"文本标签，进度已传达但非视觉条形图，没有平滑动画。

**N-4 [REQ / B-4, AC-4] `web/src/components/AppLayout.vue` — 下载失败链接是纯文本，非可点击超链接**

B-4 说 UI 显示"指向 FRP GitHub Releases 页面的手动下载链接"。失败 toast 将 URL 嵌入字符串中，不是可点击的 `<a>` 元素。

---

### NITPICK（可选修复）

**P-1 `internal/downloader/downloader.go` — `ParseIPFromJSON` 导出但 httpapi 未使用**

`handlers_system.go` 有自己的内联 JSON 解析，从未调用 `ParseIPFromJSON`，导致代码重复。

**P-2 `web/src/stores/wizard.ts` — `wizardHandled` 设置但从未读取**

**P-3 `web/src/pages/Wizard.vue` — "both" 分支重复标签文本**

---

## AC 覆盖矩阵

| 标准 | 状态 |
|---|---|
| AC-1 binMissing banner + 下载按钮 | PASS |
| AC-2 202 + 轮询进度 | PASS |
| AC-3 下载成功后 binMissing 清除 | PASS |
| AC-4 失败显示错误 + GitHub 链接 | MINOR（链接为纯文本） |
| AC-5 二进制存在时无下载按钮 | PASS |
| AC-6 新 DB → /wizard 重定向 | PASS |
| AC-7 角色选择必填 | PASS |
| AC-8 frpc 向导保存 serverAddr + 启用 mode.frpc | PASS |
| AC-9 frps 向导保存 bindPort + 启用 mode.frps | PASS |
| AC-10 跳过持久化 wizard.handled，重登录不再弹出 | PASS |
| AC-11 空 serverAddr → 422 | PASS |
| AC-12 /server 页有检测按钮，不自动填充 | PASS |
| AC-13 网络不通 → HTTP 200 + error 字段 ≤3s | PASS |
| AC-14 frps hint 有"复制全部"按钮 | **FAIL** (M-1) |
| AC-15 tcp 代理 hint 含"在 frps 服务器上执行" | PASS |
| AC-16 udp 代理显示 udp 命令，不是 tcp | **FAIL** (M-2) |
| AC-17 http/https 代理不显示防火墙提示 | PASS |
| AC-18 T-001 测试仍通过（≥146） | PASS |

---

## 修复清单（开发者）

- [ ] M-1: `FirewallHint.vue` 添加"复制全部"按钮
- [ ] M-2: `FirewallHint.vue` 添加 `proto` prop，按协议过滤命令；更新 `Server.vue`、`Proxies.vue` 调用方传入协议
- [ ] M-3: `downloader.go` 修复 Windows 原子化安装：仅在 Rename 失败时才移除现有文件
- [ ] M-4: `internal/httpapi/` 添加 5 个新端点的 HTTP handler 测试
- [ ] N-1: `Wizard.vue` 向导表单添加 auth.token 可选字段
- [ ] N-3: `AppLayout.vue` 下载进度改为视觉进度条（NProgress）
- [ ] N-4: `AppLayout.vue` 失败提示中的 GitHub 链接改为可点击超链接
