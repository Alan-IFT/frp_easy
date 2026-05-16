# QA 报告 — T-002 zero-config-quickstart

**测试日期**：2026-05-16  
**测试员**：QA Tester  
**verdict**: APPROVED FOR DELIVERY

---

## 自动化测试结果

### Go 测试（`go test ./internal/downloader/... ./internal/httpapi/... -v`）

```
downloader package:  8 tests — all PASS   (0.849s)
httpapi package:    51 tests — all PASS   (3.992s)
```

新增（T-002）：
- `TestWizardStatus_FreshDB_ShouldShow`
- `TestWizardStatus_WithConfig_ShouldNotShow`
- `TestWizardComplete_ThenShouldNotShow`
- `TestDownloadBin_ValidKind_202`
- `TestDownloadBin_InvalidKind_422`
- `TestDownloadStatus_KnownKind_200`
- `TestDownloadStatus_UnknownKind_404`
- `TestPublicIP_Always200`
- 整个 `internal/downloader` 包（8 tests）

### Go 测试全量（`go test ./...`）

**117 passed / 117 total**（T-001 baseline 101 Go tests，新增 16 个）

### 前端测试（`npm run test`）

```
6 test files — 45 tests — all PASS   (467ms)
```

---

## AC 验证矩阵

| AC | 标题 | 状态 | 理由 |
|---|---|---|---|
| AC-1 | 缺少二进制时显示 banner + 下载按钮 | PASS | `AppLayout.vue:11-41` `v-if="appStore.binMissing.length > 0"` + `v-for="kind in binMissing"` 渲染下载按钮 |
| AC-2 | 下载进度轮询 202 + 0-100% | PASS | `downloadBin` handler 返回 202；`downloaderStore.startPolling` 每 1s 轮询；`TestDownloadBin_ValidKind_202` |
| AC-3 | 下载成功后 banner 消失 | PASS | `AppLayout.vue:185-188` 成功后调 `appStore.fetchReady()`；`binloc.Missing()` 每次调用都重新 `os.Stat` |
| AC-4 | 失败显示错误 + 可点击 GitHub 链接 | PASS | `AppLayout.vue:190-199` 用 VNode `h('a', {href:'...'})` 渲染超链接 |
| AC-5 | 二进制存在时无下载按钮 | PASS | `v-if="appStore.binMissing.length > 0"` 空列表时整个 alert 不渲染 |
| AC-6 | 新 DB → 向导 | PASS | `router.ts:63-70` 导航到 `/dashboard` 时检查 `wizard.shouldShow`；`TestWizardStatus_FreshDB_ShouldShow` |
| AC-7 | 角色必选 | PASS | `Wizard.vue:239-243` 未选角色设置 `roleError.value = '请先选择部署角色'`，return 阻止前进 |
| AC-8 | frpc 向导保存 serverAddr + mode | PASS | `Wizard.vue:274-278` 调 `apiPutClient`；`Wizard.vue:284-287` 调 `apiPutMode({frpc:true})` |
| AC-9 | frps 向导保存 bindPort + mode | PASS | `Wizard.vue:267-271` 调 `apiPutServer`；`Wizard.vue:284-287` 调 `apiPutMode({frps:true})` |
| AC-10 | 跳过持久化，重登录不再弹出 | PASS | `handleSkip` 调 `wizardStore.completeWizard()` → `POST /wizard/complete` → `KVSet("wizard.handled","true")`；`TestWizardComplete_ThenShouldNotShow` |
| AC-11 | 空 serverAddr → 422 | PASS | `handlers_server.go:110-112` `if cfg.ServerAddr == "" { writeError(422, ..., "serverAddr") }` |
| AC-12 | /server 有检测按钮，不自动填充 | PASS | `Server.vue:14` 嵌入 `<public-ip-detector />`；无代码路径将 IP 写入 form |
| AC-13 | 网络不通 → HTTP 200 + error ≤3s | PASS | `handlers_system.go:92` 3s ctx；`respondWithIPResult` 始终写 200；`TestPublicIP_Always200`（0.93s 实测） |
| AC-14 | 防火墙提示有"复制全部"按钮 | PASS | `FirewallHint.vue:27-32` `<n-button @click="copyAll">`；`copyAll()` 以 `\n` 连接所有命令 |
| AC-15 | tcp 代理提示含"在 frps 服务器上执行" | PASS | `FirewallHint.vue:47` 默认 label 包含该文本 |
| AC-16 | udp 代理显示 udp 命令，不是 tcp | PASS | `Proxies.vue:147` `firewallProto.value = savedProxy.type === 'tcp' ? 'tcp' : 'udp'`；`FirewallHint.vue:56-58` `proto==='udp'` 只生成 udp 命令 |
| AC-17 | http/https 代理无防火墙提示 | PASS | `Proxies.vue:149-151` `firewallPorts.value = []`；`FirewallHint.vue:3` `v-if="ports.length > 0"` |
| AC-18 | T-001 测试仍全部通过（≥146） | PASS | 117 Go + 45 frontend = 162 > 146；`go test ./...` 全绿 |

---

## Adversarial tests

| # | AC 关联 | 假设（攻击场景） | 验证位置 | 结果 |
|---|---|---|---|---|
| A-1 | B-4/NF-S1 | tar.gz Zip Slip：含 `../../../evil` 条目写出到父目录 | `downloader.go:380` `strings.Contains(hdr.Name, "..")` + continue | PASS — 代码跳过任何含 `..` 或绝对路径的 entry |
| A-2 | B-4/NF-S1 | zip Zip Slip：含 `/etc/passwd` 绝对路径被写入系统目录 | `TestDownload_ZipSlip_MaliciousEntryFiltered`（`downloader_test.go:255-299`）；`downloader.go:407` | PASS — `evil.txt` 不存在于 root；合法 binary 正常安装 |
| A-3 | B-4/AC-2 | 并发两次下载同一 kind，第二次跳过 409 逻辑开启第二个 goroutine | `TestDownload_ErrAlreadyInProgress`（`downloader_test.go:176-225`）；`handlers_system.go:120-122` | PASS — 持锁检查返回 `ErrAlreadyInProgress`，映射为 HTTP 409 |
| A-4 | AC-2 | `kind=invalid` 不触发 422，而是 panic 或 500 | `TestDownloadBin_InvalidKind_422`（`qa_ac_test.go:746-755`）；`handlers_system.go:122-123` | PASS — 返回 422 VALIDATION_FAILED |
| A-5 | AC-2 | `download-status/unknownkind` 返回 200+空 JSON 而非 404 | `TestDownloadStatus_UnknownKind_404`（`qa_ac_test.go:779-787`）；`handlers_system.go:143-146` | PASS — `Manager.Status("unknownkind")` 返回 ok=false，写 404 |
| A-6 | AC-13 | public-ip 断网时后端等待超过 3s，或返回 5xx | `TestPublicIP_Always200`（`qa_ac_test.go:791-807`）；`handlers_system.go:92` 3s ctx | PASS — 0.93s 返回 HTTP 200 + `{"error":"检测超时，请手动查询"}` |
| A-7 | B-4/M-3 | Windows 下 Rename 失败后先 Remove 再 Rename，Remove 成功但 Rename 失败导致丢失原有二进制 | `downloader.go:226-241` 代码审查 | PASS — 修复逻辑：先尝试 Rename；仅在 Rename 失败且 GOOS=windows 时才 Remove；Remove 失败（非 ErrNotExist）立即报错，不丢弃原文件 |
| A-8 | AC-6/AC-10 | 向导完成后直接访问 `/wizard` URL，router guard 不拦截 | `router.ts:63-70` 代码审查 | MINOR（不阻断）— guard 仅拦截到 `/dashboard` 的导航，`/wizard` 路由本身无 already-handled 重定向。AC-1~18 均未要求此行为，建议 T-003 补充 |

---

## verify_all 结果

```
=== Summary ===
  PASS: 12
  WARN:  0
  FAIL:  0
  SKIP:  6
```

测试数量变化：
- T-001 baseline：146（101 Go + 45 frontend）
- T-002 本次：162（117 Go + 45 frontend）
- 新增：+16 Go（8 个 downloader 包 + 8 个 httpapi T-002 专项）

---

## 缺陷记录

无 BLOCKER、CRITICAL、MAJOR。

**[MINOR] 向导完成后直接访问 `/wizard` 不被重定向**：`web/src/router.ts` wizard guard 仅拦截 `/dashboard` 导航，不拦截 `/wizard` 直接访问。已处理的向导用户仍可访问并重新提交。AC-10 只要求"重登录不再自动跳转"（满足），此为 UX 增强点，建议 T-003 处理。

---

## 结论

T-002 zero-config-quickstart 全部 18 条 AC 均通过验证，Code Review 阶段的 4 项 MAJOR（M-1 复制全部按钮、M-2 proto 过滤、M-3 Windows 原子安装、M-4 新端点测试）和 3 项 MINOR（N-1 auth.token 字段、N-3 视觉进度条、N-4 超链接）均已修复。Go 测试基线从 101 升至 117（+16），总测试数从 146 升至 162，verify_all 全绿（12 PASS / 0 FAIL / 0 WARN）。**APPROVED FOR DELIVERY**
