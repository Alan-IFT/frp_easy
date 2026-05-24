# 03 — 闸门评审（T-038 boot-autostart-hardening）

> 由 Gate Reviewer（PM 上下文角色化）产出。模式：`full`。
> 上游：[01_REQUIREMENT_ANALYSIS.md](./01_REQUIREMENT_ANALYSIS.md) READY / [02_SOLUTION_DESIGN.md](./02_SOLUTION_DESIGN.md) READY。

## 1. Audit checklist（8 维 + per-item verdict）

| # | 维度 | 评级 | 一句话理由 |
|---|---|---|---|
| 1 | Requirement completeness | PASS | 01 §2 B-1.x ~ B-6.x 共 6 大类 21 条 in-scope behavior，每条均可测；§5 AC 含 ADV 反向证伪与 AC-4 真机 ssh 测试机端到端验证。 |
| 2 | Design completeness | PASS | 02 §2~§3 对每条 B-* 都有对应 module 实现位置 + public API 签名 + 伪代码；§6 sequence diagram 覆盖三条主流程。 |
| 3 | Reuse correctness | PASS | 02 §7 reuse audit 10 项，已 verify：`svc.IsWindowsService()`（service_windows.go L34-L40 ✓）、`store.KVGet/Set`（internal/storage/kv.go ✓）、`pm.Start`（procmgr/manager.go L182-L291 ✓）、handlers_system.go 模式（systemReady / systemPublicIP ✓）。无 reuse 漏判。 |
| 4 | Risk coverage | PASS | 02 §8 列 10 个 risk + mitigation。R-3 (network-online 慢) 与 R-7 (多任务并行端口占用) 是高概率项，配套对策清晰。补充见 §3 下方。 |
| 5 | Migration safety | PASS | 02 §9 详述：升级一键安装幂等覆盖 unit；用户 frpc.toml 不被强迁移（仅 UI 改配置触发 re-render）；纯 forward path，可 git revert 回退。 |
| 6 | Boundary handling | PASS | 01 §4 BC-1~BC-10 + 02 §3.2 retry goroutine 的 select state 检查覆盖：用户介入 / SIGTERM / 二进制缺失 / 5 次全失败 / kv 不存在 / probe timeout / Windows N 版本 sc.exe 输出格式异常。 |
| 7 | Test feasibility | PASS | 01 §5 AC-1~AC-6 均可测：单测 (AC-2)、静态闸门 (AC-3 + AC-5 ADV-1/ADV-2)、真机 ssh 端到端 (AC-4 + AC-5 ADV-4)、UI Playwright (AC-6)。ADV-3 用 iptables 模拟 frps 不可达可在测试机直接跑。 |
| 8 | Out-of-scope clarity | PASS | 01 §3 OOS-1~OOS-6 + 02 §10 各 6 条，明确不动 procmgr / 不动 SQL schema / 不动 frp_easy.toml / 不引入 watchdog / 不用 Restart=always 等；developer 不会越界。 |

总评：**全部 PASS** —— 设计完整、风险可控、测试可达、范围清晰。

## 2. 文件 / 符号存在性核查（GR 硬规矩：不信任 design 自述）

| Design 引用 | 实际存在 | 备注 |
|---|---|---|
| `cmd/frp-easy/main.go::autoRestoreProcs L433-L467` | ✓ 实测 L433 起 `func autoRestoreProcs(...)` | 重构起点准确 |
| `internal/procmgr/manager.go::waitUntilStable L493` | ✓ 实测 L493 起 `func (m *Manager) waitUntilStable(...)` | 不动其行为，外层 retry 兜底 |
| `cmd/frp-easy/service_windows.go::svc.IsWindowsService` | ✓ L34-L40 `func isWindowsService() bool` | 复用进 svcprobe 路径清晰 |
| `internal/storage/kv.go` | ✓ 实测含 `KVGet/KVSet` | 沿用 |
| `internal/frpconf/render.go::frpcRoot` struct | ✓ L38-L45 | 加 `LoginFailExit *bool` 字段位置明确 |
| `internal/httpapi/router.go` 受保护组 | ✓ L88-L128 group 内 SessionAuth + CSRF | service-status 挂这里正确 |
| `internal/httpapi/handlers_system.go::systemReady` 同款模式 | ✓ L32-L48 | handler signature / writeJSON 用法可复用 |
| `web/src/pages/Dashboard.vue` 顶部位置 | ✓ L1-L80 含 n-page-header / n-grid hero 区 | ServiceStatusCard 放在 n-page-header 之后 n-grid 之前自然 |
| `scripts/install-service.sh` `EXISTED` 分支 | ✓ L117-L122 | 已有覆盖路径，新增 self-check 在末尾追加无冲突 |
| `scripts/install-service.ps1` `Wait-ServiceRunning` | ✓ L71-L91 + L177 | 自检在其后追加无冲突 |
| `scripts/verify_all.sh / .ps1` step helper | ✓ | 沿用现有 step idiom，无需改 helper 本身 |

**全部 verified ✓**。无 design 引用了不存在的符号。

## 3. 高概率开发问题（pre-answered）

### Q-1: retry goroutine 的 ctx 来自哪里？`run()` 现签名只有 `stopCh chan struct{}` 和 `readyCh chan struct{}`，没有 ctx？

**A**: dev 应**新增一个 rootCtx**。建议在 `run()` 顶部添加：
```go
rootCtx, rootCancel := context.WithCancel(context.Background())
defer rootCancel()
```
然后把 `rootCtx` 传给 `autoRestoreProcs(rootCtx, ...)`。在主 select 收到 SIGTERM / stopCh 时调 `rootCancel()` 让 retry goroutine 中断 sleep 退出。这是干净的 Go idiom，不破坏现有签名。

### Q-2: `internal/svcprobe/` 包名是否会与 `golang.org/x/sys/windows/svc` 混淆？

**A**: 不会。`svcprobe` ≠ `svc`，import 路径完全不同（前者 `github.com/frp-easy/frp-easy/internal/svcprobe`，后者 `golang.org/x/sys/windows/svc`），import 别名也无冲突。02 §D-5 已论证。

### Q-3: `loginFailExit = false` 写到 frpcRoot struct 用 `*bool` 还是 `bool`？

**A**: 用 `*bool`（02 §3.3 明示）。理由：与 frpcRoot 既有指针字段（Log / Auth / WebServer）一致；指针让"明确写 false"与"零值（未设置）"语义可区分，符合 frpconf 包既有的"omitempty + 指针"模式。toml `omitempty` 配合 `*bool` 在 nil 时不输出，在 false 时输出 `loginFailExit = false`。

### Q-4: install-service.sh 自检的 `sleep 1` 不够 systemd 推进到 active 怎么办？

**A**: 改成 `systemctl is-active --quiet` **轮询**（与 T-019 Wait-ServiceRunning 同款 idiom），最多 5 秒：
```bash
for i in 1 2 3 4 5; do
    if systemctl is-active --quiet "$UNIT_NAME"; then break; fi
    sleep 1
done
if ! systemctl is-active --quiet "$UNIT_NAME"; then
    echo "错误：自检失败..."; exit 4
fi
```
02 §R-8 已 flag；dev 在 04 §自检块实施时按此细化。

### Q-5: Windows `sc.exe config depend= Tcpip/Dnscache` 在 Server Core 上可能失败，best-effort 是什么意思？

**A**: 02 §3.8 + §D-3 已答：失败仅 `Write-Host` 警告，**不**调 exit 2。具体落盘形态：
```powershell
$null = & sc.exe config $ServiceName depend= "Tcpip/Dnscache" 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "提示：depend= 配置失败（rc=$LASTEXITCODE）...."
    $LASTEXITCODE = 0  # 显式重置防影响后续自检
}
```

### Q-6: UI 卡片在 Dashboard 顶部，但 Dashboard 已有"二进制缺失"n-alert + n-grid 2 列 frpc/frps 卡片，加在哪？

**A**: 放在 `n-page-header` 之后、`n-alert v-if binMissing` 之前。即首屏的最顶部信息位。具体 Dashboard.vue 改动：
```vue
<template>
  <div>
    <n-page-header title="仪表盘" subtitle="frpc / frps 进程状态与控制" />

    <!-- T-038: 服务化状态卡片（[boot-autostart-fix]） -->
    <ServiceStatusCard style="margin: 16px 0" />

    <!-- 二进制缺失警告（沿用） -->
    <n-alert v-if="appStore.binMissing.length > 0" ...>...</n-alert>

    <n-grid :cols="2" ...>...</n-grid>
  </div>
</template>
```
SA §3.6 模板已展开，dev 直接落盘。

### Q-7: kv `system.autorestore.last` 在 first attempt 同步成功路径下要不要写？

**A**: 要写，写 `outcome="ok"` + `attempts=[{Index:0, OK:true, ...}]`（用 Index=0 表示 first attempt）。这样 UI 卡片始终能展示"上次自动恢复结果"，而不会只有 retry 路径才有数据。02 §3.2 伪代码里 `persistAutoRestoreLast` 在所有路径都调一次（成功 / 失败 / 用户介入 / 取消）。

## 4. SA 风险补充（GR 补 R-11）

- **R-11（GR 新增）**：dev 阶段在测试机 `alan@192.168.100.90` 跑 AC-4 真机验证时，需要把新 binary scp 上去；但测试机上既有 frp-easy.service 正在跑，scp 覆盖时若不停服可能锁定文件（Linux 通常允许覆盖被使用的可执行，但保险起见）。
  - **Mitigation**: dev 阶段流程 = `systemctl stop frp-easy` → scp 新 binary + 新 install-service.sh → 跑 install-service.sh（自动 daemon-reload + restart） → 验证。这是 02 §9 升级路径的具体执行序列。

- **R-12（GR 新增）**：retry backoff 5 次 = 累计 ~8 分钟（5+15+45+120+300=485s）。若用户 reboot 后等了 8 分钟仍未连上，UI 应能告诉他"已尝试 5 次全失败 + 具体原因"，否则用户体感"重启了 10 分钟还是不能用"。
  - **Mitigation**: 02 §3.5 useServiceStatus composable + ServiceStatusCard 在 last_run.outcome="exhausted" 时高亮 + 在"如何修复"折叠区给出"手动重启 frpc"按钮文案引导。dev 阶段实施 UI 卡片时确保这条 UX 路径不漏。

## 5. Conditions（开发期必须满足，但不阻塞 stage 4 启动）

| ID | Condition | 触发时机 |
|---|---|---|
| **C-1** | install-service.sh 自检改用轮询 5 次 1s（Q-4 答案）而非裸 sleep 1 | dev 实施自检块 |
| **C-2** | `cmd/frp-easy/main.go run()` 加 rootCtx context.WithCancel + 在主 select stopCh/sigCh 收到时调 rootCancel | dev 重构 autoRestoreProcs 前 |
| **C-3** | `*bool` 用法严格符合 §3.3 指针 + omitempty 约定（不要改成 bool + omitempty 否则 false 也被 omit） | dev 改 render.go 时 |
| **C-4** | retry goroutine 必须在每轮 sleep 后 `select { case <-ctx.Done(): ... case <-time.After(d): }`，不要把 ctx.Done 写在 sleep 之外 | dev 实施 retryRestoreLoop |
| **C-5** | UI ServiceStatusCard.vue 的"如何修复"折叠区命令字串必须含 `[boot-autostart-fix]` 锚字串（与 verify_all G.9 闸门对齐） | dev 实施 UI 组件 |
| **C-6** | verify_all `.ps1` 实现的新闸门用 Get-Content 按行扫 + `-cmatch` 严格行内匹配，避免 Raw + `-match` 假阳性（insight L26 红线） | dev 实施 verify_all 双闸门 |
| **C-7** | install-service.sh 自检失败时打印锚字串 `[boot-autostart-fix self-check FAIL]` 让 verify_all 闸门也能 grep 守门（防未来回退） | dev 实施自检块 |
| **C-8** | 测试机 AC-4 端到端验证必须包含一次"对照"：先在旧 build（未修 unit）上 reboot 观察 frpc fail，再装新 build reboot 观察 frpc retry 成功 —— 这是双侧对照证据 | QA 阶段实施 |

C-1~C-7 由 developer 在 04 实施；C-8 由 QA 在 06 实施。

## 6. insight-index 命中扫描（GR 核对 design 是否违反任何线上知识）

- **insight L9（NMessageProvider 必须在 App.vue）** —— 本任务不动 message provider，不触发。
- **insight L26（verify_all 双实现对账 / PS Raw + match 假阳性陷阱）** —— 触发，已落 C-6。
- **insight L27（PM 派发上下文 collapse 所有角色到 PM 跑）** —— 触发，本评审就是在 PM 上下文跑；dev 阶段同款。
- **insight L30 / L38（多任务并行工作树污染归责）** —— 触发，dev/QA 阶段 verify_all 跑出非本任务 FAIL 时按 git stash + 单独验证模式归责（02 §R-7 / R-11 已 flag）。
- **insight L31（archive-task.sh `## Insight` regex 不容错 §N 前缀）** —— 触发，PM stage 7 写 07 时必须用裸 `## Insight`（无 §N 前缀），bullet 列表（insight L23）。
- **insight L35（搜索高亮 v-html escape 顺序）** —— 不触发（本任务无搜索 / mark 类 UI）。
- **insight L37（main.go autoRestoreProcs 在 stderr 横幅前打印 ui.log）** —— 本任务 §B-5.2 实施了同款双轨。

## 7. Mode-aware verdict

**模式**: `full`。

## 8. Verdict

**APPROVED WITH CONDITIONS** — 设计层无 blocker，全部 8 维 PASS。Developer 实施 stage 4 时必须遵守 §5 列出的 C-1~C-8 八条 conditions。AC-4 真机验证（ssh alan@192.168.100.90）是本任务硬验收门槛，QA stage 6 必须实测 reboot 闭环。

下一步：PM 派发 Developer（stage 4），dispatch prompt 必须把 §5 C-1~C-8 列入"开发期约束"段。
