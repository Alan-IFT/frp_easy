# 交付报告 — T-002 zero-config-quickstart

**交付日期**：2026-05-16  
**任务 ID**：T-002  
**Slug**：zero-config-quickstart  
**PM**：PM Orchestrator

---

## 功能摘要

T-002 实现了"零配置快速上手"体验，涵盖：

1. **frp 二进制自动下载**：首次启动检测到 frpc/frps 缺失时，UI 顶部弹出横幅，一键触发异步下载（GitHub Releases），进度条实时显示 0-100%，成功后横幅自动消失，失败时提供可点击的手动下载链接。

2. **部署向导**：新安装时自动跳转 `/wizard`，3 步流程（选角色 → 填配置 → 完成），支持 frpc-only / frps-only / 两者，向导完成后标记 `wizard.handled` 防止重复弹出。

3. **公网 IP 自动检测**：`/server` 页面新增"检测公网 IP"按钮，调用 ipify/my-ip.io，超时 3 秒，结果仅展示不自动填充，网络不通时返回友好错误。

4. **防火墙提示 UI**：`FirewallHint` 组件，按协议（tcp/udp）过滤 ufw + iptables 命令，含"复制全部"按钮，http/https 代理不显示。

---

## 新增文件

| 文件 | 说明 |
|---|---|
| `internal/downloader/downloader.go` | FRP 二进制异步下载管理器（Zip Slip 防御、原子安装、进度跟踪） |
| `internal/downloader/downloader_test.go` | 8 个单元测试，全部使用 `httptest.NewServer`，无真实网络依赖 |
| `internal/httpapi/handlers_wizard.go` | 向导 HTTP 处理器（wizardStatus、wizardComplete、hasAnyConfig） |
| `web/src/pages/Wizard.vue` | 3 步向导页面（顶层路由，非 AppLayout 子路由） |
| `web/src/components/FirewallHint.vue` | 防火墙提示组件（proto prop、复制全部按钮） |
| `web/src/components/PublicIpDetector.vue` | 公网 IP 检测组件（仅展示，不自动填充） |
| `web/src/stores/downloader.ts` | 下载状态 Pinia store（1s 轮询） |
| `web/src/stores/wizard.ts` | 向导状态 Pinia store |
| `web/src/api/downloader.ts` | apiDownloadBin、apiDownloadStatus |
| `web/src/api/wizard.ts` | apiGetWizardStatus、apiWizardComplete |

---

## 修改文件

| 文件 | 改动要点 |
|---|---|
| `internal/httpapi/router.go` | `Downloader` 加入 `Dependencies`；`ipCache` 加入 `handlers` struct；5 条新路由 |
| `internal/httpapi/handlers_system.go` | 新增 `systemPublicIP`、`downloadBin`、`downloadStatus` handler + ipCache 类型 |
| `internal/httpapi/qa_ac_test.go` | 新增 9 个 T-002 HTTP handler 集成测试（含 `newTestServerWithDownloader`） |
| `cmd/frp-easy/main.go` | 初始化 `downloader.New()`，注入到 `Dependencies.Downloader` |
| `web/src/router.ts` | `/wizard` 顶层路由 + beforeEach wizard shouldShow 守卫 |
| `web/src/pages/Server.vue` | 嵌入 `PublicIpDetector`（bindPort 上方）、`FirewallHint proto="tcp"` |
| `web/src/pages/Proxies.vue` | 保存后显示 `FirewallHint`，传 `proto` 按代理类型（tcp/udp） |
| `web/src/components/AppLayout.vue` | binMissing 横幅含下载按钮 + `NProgress` 进度条 + 失败超链接（VNode） |
| `web/src/types.ts` | 新增 `DownloadState`、`PublicIPResponse`、`WizardStatus`、`DownloadBinRequest` |
| `web/src/api/system.ts` | 新增 `apiGetPublicIP` |

---

## verify_all 输出

```
=== Summary ===
  PASS: 12
  WARN:  0
  FAIL:  0
  SKIP:  6
```

---

## 测试基线变化

| 维度 | T-001 结束 | T-002 结束 | 增量 |
|---|---|---|---|
| Go tests | 101 | 117 | +16 |
| Frontend tests | 45 | 45 | 0 |
| 合计 | 146 | 162 | +16 |

---

## 已知后续事项

- **[MINOR] 向导已处理后直接访问 `/wizard` 不被重定向**：router guard 仅拦截 `/dashboard` 导航，建议 T-003 补充 `/wizard` 路由的 already-handled 守卫。
- `ParseIPFromJSON` 在 `downloader` 包中导出但 `handlers_system.go` 有自己的内联解析，建议后续统一。

---

## Insight

**下载器原子安装 Windows/Linux 差异**：Linux `os.Rename` 原子覆盖目标文件，不需要预先 Remove；Windows 不允许覆盖已存在的文件，必须先 Remove 再 Rename。正确模式是先尝试 Rename，仅在失败时（GOOS=windows）才执行 Remove+Rename，且 Remove 失败时必须报错而非静默继续，否则会在 Remove 成功但 Rename 失败的罕见路径下永久丢失原有二进制。

**向导路由必须是顶层路由**：向导页面不应嵌套在 `AppLayout` 子路由中，否则侧边栏导航会干扰向导流程。顶层路由 + 全屏布局是正确模式。
