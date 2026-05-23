# 03 — Gate Review · T-019 windows-service-scm-1053-fix

> Harness 流水线 Stage 3（Gate Reviewer）。模式：**full**。
> 上游：`01_REQUIREMENT_ANALYSIS.md`（READY） + `02_SOLUTION_DESIGN.md`（READY）。
> 本评审独立核验：所有"设计引用既有代码"的位置 reviewer 都跑了 Read/Grep 真读真核对，未盲信。
> 注：本评审由 gate-reviewer sub-agent 完整产出文本，因其工具集仅 Read/Glob/Grep（无 Write），由 PM 代为落盘（insight L41 红线场景）。

---

## 1. Audit checklist（8 维）

| # | 维度 | 状态 | 一句理由 |
|---|---|---|---|
| 1 | Requirement completeness | **PASS** | 01 §2 IS-1~12 每条 in-scope 行为均对应 §5 至少一条 AC；§3 OOS、§4 BC、§6 NFR、§8 PM-resolved 全部齐备。 |
| 2 | Design completeness | **WARN** | 02 §3.1 内部出现 `run(stopCh)` → "最终选实现 A `run(stopCh, readyCh)`" 两版签名前后不一致（详见 F-1）；其余 SCM 状态机、reuse audit、序列图均完整可实施。 |
| 3 | Reuse correctness | **PASS** | reviewer 真读：`main.go` L88-93 main 入口、L280-290 signal select、`procmgr.Manager.Shutdown()` L393-397、`install-service.ps1` L68-82 wrapper 生成块、L96/L102 binPath 引号转义、`uninstall-service.ps1` L70-77 wrapper 清理、`go.sum` L26 `golang.org/x/sys v0.44.0` 全部按 02 §7 描述精确存在；`x/sys/windows/svc` 子包在 v0.44.0 中可用（属 x/sys 自 v0.x 起即固定子包，已被 Caddy/Vault/etcd/Kubelet 在生产采用）。 |
| 4 | Risk coverage | **PASS** | 02 §8 R-1~R-8 覆盖：慢启 1053 / SCM Stop 链路 / Linux CI 编译 / 中文 cwd / 升级残留 / sc.exe 引号 / Ctrl+C / CI 缺口；每条带具体缓解。R-4 已显式回答 insight L17（host codepage wrapper.cmd）反向消解问题——移除 wrapper.cmd 后改走 `os.Executable()` + `GetModuleFileNameW`（UTF-16 原生），不再经 GB18030，问题在新设计层面被彻底消解。 |
| 5 | Migration safety | **PASS** | 02 §9 给出干净安装 / 升级 / 回滚 / 数据兼容四路径；R-5 + §3.4 install-service.ps1 顶端追加防御性 `Remove-Item frp-easy-svc.cmd`，T-018→T-019 升级路径不留旧 wrapper 残留；无 schema 变更（§4 明确）。回滚策略明确（仅脚本回退也能工作，因新二进制双入口向后兼容）。 |
| 6 | Boundary handling | **WARN** | BC-1~14 在 01 §4 列得很全；02 §3.1 Execute 在 readyCh 关闭后未显式 reset CheckPoint = 0 再切 RUNNING（详见 F-2，非 fatal 但属规范偏离）。BC-13（极端慢盘 > 30s）由 1s 心跳兜底，CheckPoint 单调递增即可让 SCM 不报 1053（svc 包内部 SetServiceStatus 接收 CheckPoint 累加即续命）。 |
| 7 | Test feasibility | **WARN** | AC-15 要求"Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 全部 verify_all PASS:19" — 但 reviewer 真读 `docs/features/_archived/install-role-and-public-ip/06_TEST_REPORT.md` 仅发现 T-017 QA 在腾讯云 Ubuntu 单台 VM 跑过（行 140 用户原话场景），项目历史从未在 4 发行版上同时跑过 verify_all；NFR-2 体积 ≤ 1 MB 增长在 §5 没有对应 AC 兜底（详见 F-3 / F-7）。其余 AC-1~14、16~19 可由 Win11/Server 单机复现。 |
| 8 | Out-of-scope clarity | **PASS** | 01 §3 OOS-1~13 + 02 §10 设计层 OOS 明确：不动 Linux 路径、不动 uninstall 语义、不引第三方壳、不切服务账户、不写 Event Log、不加 --service-debug、不引 Job Object 硬清理。Developer 不会在这些方向偶发越界开发。 |

---

## 2. Findings

### F-1（WARN · 责任 02 §3.1）

§3.1 给出 `runService` 与 `Execute` 的完整 Go 代码 sample 时，goroutine 内部写：

```go
go func() {
    runErrCh <- run(stopCh) // run 签名改为接收 stopCh + 可关 readyCh
}()
```

但同节末尾"注"段落改口："最终选实现 A：`run(stopCh <-chan struct{}, readyCh chan<- struct{}) error`"。§3.3 也使用两参版本。**§3.1 的 sample 代码必须改为 `run(stopCh, readyCh)`** 否则 Developer 复制粘贴会编译失败或第二轮回头改。属"设计文档自我矛盾"，需 architect 修订 02 §3.1 sample 一行（不影响其它判断）。

### F-2（WARN · 责任 02 §3.1 Execute 状态机第 5 步）

Execute 跳出 HEARTBEAT 循环后直接 `s <- svc.Status{State: svc.Running, Accepts: accepted}`，未显式 reset CheckPoint=0。按 MSDN `SetServiceStatus`：RUNNING 状态下 dwCheckPoint 与 dwWaitHint 应为 0（这两个字段语义仅在 PENDING 状态有意义）。`svc.Status{}` 零值字段会被发为 0，所以**实际行为正确**（Go 零值救场），但代码清晰度上建议显式：

```go
s <- svc.Status{State: svc.Running, Accepts: accepted, CheckPoint: 0, WaitHint: 0}
```

非阻塞，Developer 写 Go 零值即过。

### F-3（WARN · 责任 01 §5 AC-15）

AC-15 写"Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 全部 verify_all PASS:19 且 `systemctl is-active frp-easy` = active（回归不退化），与 T-017 同款 4 发行版断言"。但 reviewer 实际核查 `docs/features/_archived/install-role-and-public-ip/06_TEST_REPORT.md` —— **T-017 QA 仅在腾讯云 Ubuntu 单台 VM 实测过**，"4 发行版同款"是 RA 误认。本任务对 Linux 路径零字节改动（OOS-2），AC-15 在物理上 Developer/QA 无法拿到 4 套环境执行。建议 PM 在派发 QA 前把 AC-15 降级为：

> "Ubuntu 22.04 / Windows Server 2019+ 至少各一台 `verify_all PASS:19`；其它 Linux 发行版作 best-effort（CI 静态构建通过即可，无需运行时端到端断言）"。

非 fatal，但若 QA 拒按 AC-15 字面执行会卡 declare-done。

### F-4（WARN · 责任 01 §5）

NFR-2 写"frp-easy.exe 体积相对 T-018 release 增长 ≤ 1 MB" —— 但 §5 AC-1~19 没有对应"体积断言" AC。QA 若严格按 AC 列表跑会跳过体积验证；若主动验证又没有可固定的复现命令（T-018 release 哪个 sha？怎么取参考体积？）。建议追加 AC-20：

> "QA 用 `ls -l frp-easy.exe` 对比 T-018 main HEAD 与本任务合并后构建产物，增长 ≤ 1,048,576 字节"。

非阻塞但属 NFR 与 AC 表脱节。

### F-5（WARN · 责任 02 §3.3 / §7 reuse audit）

main.go L267-277 `browseropen.ShouldOpen(noBrowser)` 在 close(readyCh) 之后被调用，会尝试 `open` URL。服务模式下没有 TTY，`browseropen.ShouldOpen` 通常依赖 `os.Stdout.Fd()` isatty 检测 → 服务模式下返回 false → 不开浏览器；但 02 §3.3 没有显式断言这一点。Developer 在 QA 报"服务装好后 SCM 日志里偶尔看到 browseropen warn"时会怀疑设计缺陷。建议在 §3.3 一句话补充：

> "服务模式下 `browseropen.ShouldOpen` 依靠 isatty 自然返回 false，不打开 GUI；无需在 service_windows.go 显式禁用。"

非阻塞，可以由 Developer 在 04 自己 grep `browseropen.ShouldOpen` 实现确认。

### F-6（WARN · 责任 01 §7 / 02 §10 NFR-9 描述）

main.go L138-140 在 `UIBindAddr == "0.0.0.0"` 时 `fmt.Fprint(os.Stderr, exposureNotice(...))` —— 该 stderr 在服务模式下被 SCM 丢弃（无 TTY、不被 lumberjack 接管，因为 lumberjack 仅作 logger writer 的 backend，**不**重定向标准流）。01 §7 T-011 行说"日志（lumberjack 写入 .frp_easy\logs\ui.log）保留该行" —— 这个说法**不准确**：exposureNotice 走的是裸 `fmt.Fprint(os.Stderr)` 而非 logger，不会被 lumberjack 捕获。安全提示在 Windows 服务模式下实际**会丢失**。属 01 §7 描述与代码实际不符。**对 1053 修复本身无影响**，建议作为 follow-up 任务处理（不阻塞本任务 declare-done）。

### F-7（WARN · 责任 01 §5 与 NFR-1 衔接）

NFR-1 写"`golang.org/x/sys` 已在 indirect 依赖中（modernc.org/sqlite 引入的），提升为 direct 不增加交付物体积超出 NFR-2"。reviewer 真读 go.sum L26 `golang.org/x/sys v0.44.0` 确认 v0.44.0 hash 已固定 —— **NFR-1 描述准确**。但 NFR-1 没有写"提升后 go.mod direct 块需要新增 require 行，且 indirect 块的同一行需删除"，Developer 第一次改 go.mod 时若 `go mod tidy` 跑一遍可能让 x/sys 因为只被 service_windows.go（windows-only）引用，在 Linux 上 `go mod tidy` 回滚为 indirect（Go 1.17+ module graph pruning + build tag 相互作用）。建议 Developer 在 04 §实施步骤里显式：

> "改完 go.mod 后**不要**在 Linux 主机跑 `go mod tidy`，否则 x/sys 会因 windows-only import 被退回 indirect；用 `GOOS=windows go mod tidy` 或手工编辑 go.mod 后只跑 `go build ./...` 双平台验证。"

属 02 §11 dispatch order 第 1 步隐含坑，建议补一句注释。非阻塞。

---

## 3. High-probability questions during development

预测 Developer 实施 04 时会问的 5 个问题，预先回答：

**Q-D1**：`run(stopCh, readyCh)` 中两个 chan 都传 nil 时如何安全 select？

> **答**：Go 的 `select` 对 nil channel 的 case 永远阻塞（不报 panic），等价于该分支不存在。控制台分支 `run(nil, nil)` 时 select 仅 sigCh 与 serveErr 两路有效，行为与现状完全一致；NFR-9 不破断言成立。`close(readyCh)` 前必须显式 `if readyCh != nil { close(readyCh) }`（02 §3.3 已写）。

**Q-D2**：service_windows_test.go 在 Linux CI 上怎么不报"找不到 svc 包"？

> **答**：02 §2.1 已显式 `//go:build windows`；Go 工具链对 build tag 的处理是**编译前过滤源文件**，Linux `go test ./cmd/frp-easy` 根本不会扫描 service_windows_test.go，svc 包 import 也不会被解析。R-3 缓解成立。无需改 verify_all.sh（NFR-5）。

**Q-D3**：`os.Chdir(filepath.Dir(exe))` 是否会和 main.go L131 `cfgPath := envOr("FRP_EASY_CONFIG", "frp_easy.toml")` 的相对路径解析时机冲突？

> **答**：runService 在 svc.Run 之前 Chdir，svc.Run 触发 serviceHandler.Execute，Execute 内 `go run(stopCh, readyCh)` 才进 main.go run() L131 走 envOr → 此时 cwd 已锁定到 exe 目录 → `frp_easy.toml` 相对路径正确解析到 `<InstallDir>\frp_easy.toml`。与 wrapper.cmd 的 `cd /d "$InstallDir"` 语义等价。**无冲突**。

**Q-D4**：Wait-ServiceRunning 30s 超时退出码 2 会让 install.ps1 步骤 7 透传 2，但 01 §7 T-016 行说"修复后应为 0"。冲突吗？

> **答**：不冲突。01 §7 T-016 行说的是"成功路径退出码 0"；Wait-ServiceRunning 超时本身就是失败路径（服务 30 秒未 RUNNING = 1053 修复失效），install.ps1 透传 2 正确反映 IS-1 / AC-1 失败。02 §3.4 写得清楚：`exit 2` 仅在 Wait-ServiceRunning 返回 false 时触发。

**Q-D5**：service_other.go 用 `stringError` 自定义 error type 避免引入新依赖 —— 为什么不直接 `errors.New(...)`？

> **答**：02 §3.2 sample 注释解释了"避免引入额外依赖"——但其实 `errors` 已经在 main.go L24 import 中，service_other.go 用 errors.New 完全不增依赖。**建议 Developer 直接用** `errors.New("service mode not supported on this platform")`，简洁度优于 stringError type。属 micro-nit，Developer 自由选择，不卡审。

---

## 4. Verdict

**APPROVED WITH CONDITIONS**

允许 Developer 进入 Stage 4，但合并前必须满足以下条件：

1. **C-1（必须）**：Developer 在 04_DEVELOPMENT.md §实施步骤里显式记录 F-7 提示（go.mod 改完后不在 Linux 跑 `go mod tidy`；用 `GOOS=windows go mod tidy` 或手工编辑）。
2. **C-2（必须）**：Developer 把 02 §3.1 sample 中 `runErrCh <- run(stopCh)` 实际写为 `runErrCh <- run(stopCh, readyCh)`（即按 02 §3.1 注末 + §3.3 的两参签名实施，忽略 §3.1 sample 的笔误）。
3. **C-3（建议，QA 接受时由 PM 裁决）**：QA 在 06_TEST_REPORT.md 把 AC-15 实际跑的 Linux 发行版**如实记录**（即使仅 Ubuntu 22.04 单台），PM 在归档时把降级写入 07_DELIVERY.md。AC-15 不作为 declare-done 的硬卡点（与 T-017 历史一致）。
4. **C-4（建议）**：QA 报告里追加体积对比 AC（F-4），把 `frp-easy.exe` 增长 ≤ 1 MB 落到可复现证据，否则 NFR-2 仅停留在文档断言。
5. **C-5（提示，无需阻塞）**：F-5（browseropen 服务模式 isatty 自然返回 false）与 F-6（exposureNotice stderr 服务模式丢失）属既有代码与新设计的衔接边角，本任务不修复，留作后续任务（建议 PM 在 backlog 加一条 "T-020 service-mode-stderr-bridge"）。

条件 C-1 / C-2 是 Developer 自己即可在 04 落地的 hygiene 项，不需回退到 Architect 重写 02；F-1/F-2 已在本评审 §2 明确指出，Developer 按本评审 §3 Q-D1 / Q-D5 的预答案直接照做即可。

设计层面**无 BLOCKED 风险**：reuse audit 8 项全部真读核验通过；R-1~R-8 风险缓解充分；NFR-1/2/5/7/8/9 均有具体实现锚点；AC-17 已写进 01 §5（精确英文裸标题 `## Adversarial tests`，符合 insight L29 + L40 红线）。
