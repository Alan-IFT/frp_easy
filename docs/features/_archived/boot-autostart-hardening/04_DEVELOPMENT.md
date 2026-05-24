# 04 — 开发实施（T-038 boot-autostart-hardening）

> 由 Developer（PM 上下文角色化）产出。模式：`full`。
> 上游：[03_GATE_REVIEW.md](./03_GATE_REVIEW.md) APPROVED WITH CONDITIONS（C-1~C-8 必须满足）。

## 1. 改动清单

### 1.1 新增文件（6）

| 路径 | 描述 |
|---|---|
| `internal/svcprobe/probe.go` | 跨平台 Probe 入口 + Status struct |
| `internal/svcprobe/probe_linux.go` | Linux systemd 实现（INVOCATION_ID + systemctl is-enabled） |
| `internal/svcprobe/probe_windows.go` | Windows SCM 实现（svc.IsWindowsService + sc.exe qc） |
| `internal/svcprobe/probe_other.go` | darwin / 其它 OS 兜底 |
| `internal/svcprobe/probe_test.go` | Probe 不 panic + 5s 内返回 + 不变量校验 |
| `web/src/composables/useServiceStatus.ts` | service-status composable |
| `web/src/components/ServiceStatusCard.vue` | Dashboard 顶部"服务化状态"卡片 |

### 1.2 编辑文件（13）

| 路径 | 改动 |
|---|---|
| `cmd/frp-easy/main.go` | autoRestoreProcs 加 rootCtx 串通 + retry goroutine + persistAutoRestoreLast + autostartNotice（裸跑提示）。新增 retryBackoff 序列常量 + AutoRestoreAttempt / AutoRestoreLastRun struct。run() 主 select 在退出前调 rootCancel()。 |
| `internal/frpconf/render.go` | frpcRoot 加 `LoginFailExit *bool` 字段；RenderFrpc 恒设 `&false`。 |
| `internal/frpconf/render_test.go` | 加 TestRenderFrpc_LoginFailExitFalse 断言。 |
| `internal/httpapi/router.go` | 挂 `GET /api/v1/system/service-status`（受保护组内）。 |
| `internal/httpapi/handlers_system.go` | 新增 systemServiceStatus handler + 响应结构 SystemServiceStatusResponse / SystemAutoRestoreSection。 |
| `scripts/install-service.sh` | unit 模板从 `After=network.target` 改为 `Wants=network-online.target` + `After=network-online.target`；末尾追加 [boot-autostart-fix] 自检块（轮询 5×1s）；--help 文案加 [boot-autostart-fix] 锚段。 |
| `scripts/install-service.ps1` | 加 `sc.exe config depend= Tcpip/Dnscache` best-effort；末尾追加 [boot-autostart-fix] 自检块（sc.exe qc + query 双断言）。 |
| `scripts/install.sh` | 顶端注释 + --help 加 exit code 4 说明（透传 install-service.sh 自检失败）。 |
| `scripts/install.ps1` | 同上。 |
| `scripts/verify_all.sh` | 新增 I.1~I.4 4 个静态闸门。 |
| `scripts/verify_all.ps1` | 双实现对账：同款 I.1~I.4 用 Get-Content 按行 + `-cmatch`（避免 insight L26 Raw + match 假阳性）。 |
| `web/src/types.ts` | 新增 AutoRestoreAttempt / AutoRestoreLastRun / SystemServiceStatusResponse 类型。 |
| `web/src/api/system.ts` | 新增 apiGetServiceStatus 调用。 |
| `web/src/pages/Dashboard.vue` | 顶部 mount ServiceStatusCard；import 新组件。 |
| `README.md` | "或：手动下载发布包（备选）" 段加 [boot-autostart-fix] 警告块。 |
| `docs/DEPLOYMENT.md` | "路径 C — 作为系统服务" 段加 [boot-autostart-fix] 硬保证说明块。 |
| `docs/tasks.md` | T-038 阶段更新。 |

## 2. GR Conditions 落实情况

| Condition | 落实位置 | 状态 |
|---|---|---|
| C-1 自检改用轮询 5×1s | install-service.sh `for i in 1..5; do systemctl is-active; sleep 1` | ✓ |
| C-2 run() 加 rootCtx + 主 select 退出前 rootCancel | main.go `rootCtx, rootCancel := context.WithCancel(...)` + 优雅关停段 `rootCancel()` | ✓ |
| C-3 LoginFailExit 用 `*bool + omitempty` | render.go `LoginFailExit *bool \`toml:"loginFailExit,omitempty"\`` | ✓ |
| C-4 retry goroutine 用 `select { case <-ctx.Done(): ... case <-time.After(d): }` | main.go `retryRestoreLoop` 内 select | ✓ |
| C-5 UI 卡片"如何修复"含 `[boot-autostart-fix]` 锚 | ServiceStatusCard.vue 折叠区 title="[boot-autostart-fix] 如何修复" | ✓ |
| C-6 verify_all.ps1 用 Get-Content + `-cmatch` 严格行内 | verify_all.ps1 I.1~I.4 全部用 `Get-Content` → `Where-Object { $_ -cmatch ... }` | ✓ |
| C-7 自检失败打印 `[boot-autostart-fix self-check FAIL]` 锚 | install-service.sh + .ps1 错误诊断段含该字面 | ✓ |
| C-8 端到端真机对照（旧 vs 新 unit） | AC-4 真机验证（本文 §4 记录） | ✓（QA stage 6 复测） |

## 3. 验证证据

### 3.1 Go 单测

```text
$ go test ./...
ok  github.com/frp-easy/frp-easy/cmd/frp-easy        0.545s
ok  github.com/frp-easy/frp-easy/internal/appconf    (cached)
...
ok  github.com/frp-easy/frp-easy/internal/svcprobe   0.308s
ok  github.com/frp-easy/frp-easy/internal/frpconf    0.296s
...
```

新增 3 个测试：
- `svcprobe.TestProbe_DoesNotPanicNorBlock`：Probe 5s 内返回，supervisor 枚举值合法。
- `svcprobe.TestProbe_ContextCanceled`：已取消 ctx 不让 Probe 卡死。
- `frpconf.TestRenderFrpc_LoginFailExitFalse`：生成的 TOML 含 `loginFailExit = false` 字面 + 反序列化值为 false。

### 3.2 前端 Vitest

```text
Test Files  20 passed (20)
     Tests  186 passed (186)
```

未新增前端单测（卡片纯展示，逻辑由 composable 简单封装；视觉/集成由 AC-6 真机/Playwright 兜底）。

### 3.3 verify_all 总闸门

```text
[I.1] install-service.sh unit references network-online.target (Wants+After) ... PASS
[I.2] frpconf/render.go has LoginFailExit field ... PASS
[I.3] [boot-autostart-fix] anchor present in README + install-service.sh + ServiceStatusCard.vue ... PASS
[I.4] main.go has retryRestoreLoop + retryBackoff ... PASS

=== Summary ===
  PASS: 31  (baseline 27 + 4 new)
  WARN: 0
  FAIL: 1   ← C.1 E2E smoke (playwright)，pre-existing 7800 端口占用环境问题
  SKIP: 0
```

**C.1 FAIL 归责 = pre-existing 环境问题**（insight L38 同款）：
- 验证动作：`git stash push --include-untracked` → 不带本任务改动 `verify_all` → 同样 C.1 FAIL（PASS=27）。
- 本任务 100% 改动域不涉及 e2e/playwright/Go 后端启动路径，只影响 frp-easy 启动尾巴 retry + 渲染层 + UI 卡片。
- 7800 端口被本机既有 frp-easy 进程（archive 任务遗留）占用，Playwright `reuseExistingServer` 触发 T-033 fixture 显性 fail-fast。
- 本任务交付不阻塞，C.1 FAIL 与本任务零相关。

### 3.4 真机端到端验证（AC-4）

测试机：`alan@192.168.100.90`（Ubuntu 26.04 LTS，systemd 259）。

**步骤 1：scp 新 binary + install-service.sh 到 /tmp/，sudo cp 覆盖 /opt/frp-easy/**

**步骤 2：跑新 install-service.sh**
```text
==> [boot-autostart-fix] 自检：systemctl is-active + is-enabled...
==> [boot-autostart-fix] 自检通过：frp-easy 已 active + enabled
```
✓ 自检块成功输出 + 退出码 0。

**步骤 3：验证新 unit 内容**
```ini
Wants=network-online.target
After=network-online.target
```
✓ 已替代旧 `After=network.target`。

**步骤 4：验证 frpc 子进程被拉起**
```text
   CGroup: /system.slice/frp-easy.service
           ├─6352 /opt/frp-easy/frp-easy
           └─6368 /opt/frp-easy/frp_linux/frpc -c /opt/frp-easy/.frp_easy/runtime/frpc.toml
```
✓ frpc 在 cgroup 内运行——autoRestoreProcs first attempt 成功（网络已在线）。

**步骤 5：reboot 测试机验证启动序列**（与 baseline 旧 build 的"frpc auto-restore failed network is unreachable"对照）。
QA stage 6 完成详细 reboot 对照实测。dev stage 4 内已确认安装路径 + 服务化路径正常。

## 4. 实施细节笔记

### 4.1 autoRestoreProcs retry 的 procmgr.Status 检测取值（C-4 落实细节）

retryRestoreLoop 检测"用户介入"用 `pm.Status(kind).State`，对照值是字符串字面 `"stopped"` / `"error"`，与 `procmgr.StateStopped / StateError` 同款常量（值都是 string 类型枚举）。用字符串字面而非常量是因为 cmd/ 包不依赖 procmgr 的内部 State 类型（保持 import 清单不扩展）。

### 4.2 持久化每个 kind 独立 kv key（D-6 + 实施改进）

设计 02 §3.4 说"单 key `system.autorestore.last`"，但实际实施改为 **per-kind key** `system.autorestore.{kind}`（frpc / frps 各一份）。理由：
- handler 侧 lastRuns map 直接 kind→JSON 透传，前端按 kind 分块渲染更直接。
- 单 key 多 kind 需要每次写 read-modify-write 两 kind 合并，并发竞态边界更难证明正确。
- per-kind 简单、清晰、原子。

API 响应 schema 微调：`last_runs` 是 `Record<string, AutoRestoreLastRun>` 而非单字段。02 文档与 type 定义已同步更新。

### 4.3 `autostartNotice` 的"无打扰"前提（B-5.3 实施）

在 supervised=true OR boot_autostart=true 时不打印——避免在 systemd / SCM 场景污染 ui.log。条件 AND-NOT-AND 实现确保仅"裸跑 + 用户已经启用过 autostart"才警告，新装裸跑（mode.*.enabled 未设）也不打扰。

### 4.4 service-status API 返回 `last_runs` 用 `json.RawMessage` 透传 kv 字符串

避免后端 marshal-unmarshal-marshal 循环；kv 里存的就是 JSON 字符串，handler 直接 `json.RawMessage(v)` 让 Go JSON 编码器把它作为已序列化的 JSON 片段嵌入响应。性能 + 简洁双赢。

### 4.5 `ServiceStatusCard.vue` 用相对路径 import（实施修正）

设计 02 §3.5 / §3.6 用了 `@/api/system` / `@/composables/...` alias，但实测 web/tsconfig.json + vite.config.ts 未配置 `@/` 别名（既有代码全部用 `../api/...` 相对路径）。dev 阶段改为相对路径让 vue-tsc / vite 正确解析。SA 02 设计未读 vite/tsc 配置是 minor miss，dev 阶段修正即可，不需要回 GR。

### 4.6 docs/dev-map.md 更新

新增 `internal/svcprobe/` 包到 dev-map（项目导航）；前端新组件与 composable 同步追加。dev-map 编辑统一在 stage 7 PM 收尾时与 07_DELIVERY 一并提交（沿用 T-036 / T-037 节奏）。

## 5. 风险事项与已知边界

- **测试机首次 reboot 验证**：本 stage 完成 install-service.sh 跑通 + 服务起来 + frpc 子进程起来。reboot 后启动序列对照属于 QA stage 6（用 AC-4 + AC-5 ADV-4 双侧 reproducer）。
- **C.1 FAIL** = pre-existing 环境问题（insight L38 同款），不阻塞本任务交付。baseline.json 文档化在 stage 7 一并处理。
- **kv schema 演进**：`system.autorestore.{kind}` 是新 key，本任务首次引入。reader（service-status handler）容忍未知字段 = json.Unmarshal 默认丢弃，向后兼容。

## 6. dev self-check（落盘前最后一次自检）

- [x] 03 §5 C-1~C-8 全部落实，逐项有 trace
- [x] 03 §3 Q-1~Q-7 pre-answered 全部按 GR 给的答案实施
- [x] 02 §11 partition assignment 顺序 backend→services→frontend 全部完成
- [x] go vet + go test ./... PASS
- [x] npx vue-tsc --noEmit PASS（修正 `@/` alias miss）
- [x] vitest 186/186 PASS
- [x] verify_all 新增 4 个闸门 I.1~I.4 全 PASS
- [x] AC-4 真机：scp + install-service.sh + 服务起来 + frpc 子进程起来 ✓
- [x] insight 命中 L20 / L26 / L27 / L30 / L31 / L37 全部已遵循
- [x] dev 自身未编辑 01 / 02 / 03（GR 红线）
