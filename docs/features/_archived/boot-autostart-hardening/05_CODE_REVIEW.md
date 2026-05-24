# 05 — 代码评审（T-038 boot-autostart-hardening）

> 由 Code Reviewer（PM 上下文角色化）独立审查 dev 阶段产出。
> 上游：[04_DEVELOPMENT.md](./04_DEVELOPMENT.md)。

## 1. 6 维度审查

### Dim 1 — Logic correctness

| Item | Verdict | 一句话理由 |
|---|---|---|
| `autoRestoreProcs` first attempt 同步语义保留 | PASS | first attempt 与原实现字节级等价（同 3s context + same skip conditions），retry 仅在失败时启 goroutine，NFR-9 启动序列零字节改保留。 |
| `retryRestoreLoop` ctx 取消路径 | PASS | C-4 落实正确：每轮 `select { <-ctx.Done() | <-time.After(d) }`，先等 backoff 再判用户介入，避免 SIGTERM 时还 sleep 一整段才退。 |
| `retryRestoreLoop` 用户介入检测 | PASS | `pm.Status(kind).State != "stopped" && != "error"` —— state ∈ {starting, running, stopping} 都视为用户操作中。注意：StateStarting / StateRunning / StateStopping 在 procmgr 是字符串字面，硬编码字面字符串避免 cmd/ 引入 procmgr State 类型依赖（dev §4.1 已记录）。 |
| `persistAutoRestoreLast` ctx 处理 | MINOR-1 | 调 `context.WithTimeout(parent, 5s)`：若 parent（rootCtx）已 cancel，子 ctx 立即 done → KVSet 立即失败 → "canceled" outcome 不会被持久化。影响：shutdown 时丢一次 last-run 写入，下次 boot autoRestoreProcs 会覆盖，无功能性破坏。可接受。 |
| svcprobe Linux 进程 ID 匹配 | PASS | `systemctl is-enabled frp-easy.service` 用硬编码 unit 名，与 install-service.sh 默认 `UNIT_NAME=frp-easy` 一致；--name 自定义场景属 OOS（02 §10 已声明）。 |
| svcprobe Windows boot_autostart regex | PASS | `START_TYPE\s*:\s*2\s+AUTO_START` 在 zh-CN / en-US 系统的 sc.exe 输出格式一致（实测过 T-019）。误判风险 = R-5 已 mitigate（探测失败降级 false，fail-safe）。 |
| `loginFailExit` 字段 omitempty + *bool | PASS | C-3 落实正确：`*bool + omitempty` 在 nil 时省略输出、在 false 时输出 `loginFailExit = false`。frpconf.RenderFrpc 内 `no := false; root.LoginFailExit = &no` 不依赖外部输入，恒为 false。 |
| `applyConfigBestEffort` 路径下 frpc.toml 重渲染 | PASS | 未改 handlers_proxies / handlers_server，render.go 是唯一渲染入口，所有 UI 改配置触发 re-render 都会带上 loginFailExit=false。dev §4.2 实证过升级路径行为（D-8）。 |
| install-service.sh 自检轮询 5×1s | PASS | C-1 落实正确：`for i in 1..5; do systemctl is-active; sleep 1; done` + 终判。极少 ≥5s 才推进 active 的边界由 RestartSec=5 兜底（unit 自身 Restart=on-failure 失败后 5s 再启）。 |
| install-service.ps1 自检 sc.exe qc + query | PASS | 双断言（START_TYPE: 2 AUTO_START + STATE: 4 RUNNING），失败 exit 4 透传到 install.ps1，注释段含 [boot-autostart-fix self-check FAIL] 锚（C-7 落实）。 |

### Dim 2 — Requirement fidelity（逐条 AC 核查）

| 01 AC | 实施位置 | 核查结果 |
|---|---|---|
| AC-1 verify_all PASS | verify_all I.1~I.4 全 PASS（详见 04 §3.3） | ✓ |
| AC-2 单测：svcprobe / autorestore / render loginFailExit / install-service | svcprobe_test.go + render_test.go.TestRenderFrpc_LoginFailExitFalse + 现有 frpc render test |  ✓（autoRestoreProcs retry 路径单测缺失 — 见下方 MAJOR-1） |
| AC-3 静态闸门守 README / UI / install-service.sh `[boot-autostart-fix]` | verify_all I.3 PASS | ✓ |
| AC-4 真机 reboot 验证（旧 vs 新） | 04 §3.4 实证：reboot 后 frpc 直接 running，无 "auto-restore failed" 警告 | ✓ |
| AC-5 ADV-1 临改 install-service.sh `network.target` → verify_all FAIL → 恢复 PASS | QA stage 6 实施 | 延期至 QA |
| AC-5 ADV-2 删 loginFailExit → verify_all 静态闸门 FAIL | I.2 守 LoginFailExit 字面，反向证伪可达 | ✓（可由 QA 反向证伪） |
| AC-5 ADV-3 iptables 模拟 frps 不可达 → service-status API 返回 exhausted | 测试机可达；属 QA stage 6 | 延期至 QA |
| AC-5 ADV-4 旧 build vs 新 build reboot 对照 | 04 §3.4 已对照（旧 build journalctl 真实展示 "auto-restore failed network is unreachable"，新 build 零警告） | ✓ |
| AC-6 UI 卡片渲染 | ServiceStatusCard.vue mount 到 Dashboard 首屏；Playwright 由 QA stage 6 覆盖 | 实施完成，待 QA 实测渲染 |

### Dim 3 — Design fidelity（02 设计文档 vs 04 实施）

| 02 设计点 | 04 实施 | 差异 |
|---|---|---|
| `system.autorestore.last` 单 key | `system.autorestore.{kind}` per-kind key | **设计漂移**（dev §4.2 明示）。实施改进，文档已同步更新。理由清晰可接受，**Minor**。 |
| `@/api/system` / `@/composables/...` alias | 改为相对路径 `../api/system` / `../composables/...` | **设计漂移**（dev §4.5 明示）。vite/tsc 配置实际无 `@/` alias，dev 修正。设计 02 §3.5 / §3.6 文档未及时更新。**Nit**：让 04 留 trace 即可，不阻塞。 |
| autoRestoreProcs first-attempt 同步保留 | 严格保留，仅失败后启 goroutine | ✓ |
| retry backoff 序列 5/15/45/120/300s | `retryBackoff = []time.Duration{5s, 15s, 45s, 120s, 300s}` 字面常量 | ✓ |
| `Wants=network-online.target + After=network-online.target` | install-service.sh unit 模板含两行 | ✓ |
| svcprobe build tag 分文件 | probe_linux.go / probe_windows.go / probe_other.go 各 build tag | ✓ |
| Linux supervised = INVOCATION_ID 非空 | `os.Getenv("INVOCATION_ID") != ""` | ✓ |
| Windows supervised = svc.IsWindowsService | 沿用 cmd/frp-easy/service_windows.go::isWindowsService 同款判定 | ✓ |
| service-status API 路径 `GET /api/v1/system/service-status` | router.go 受保护组内挂 `/system/service-status` | ✓ |
| autostartNotice 在 ready gate 后打印 + 双轨 stderr + ui.log | main.go run() 在 `autoRestoreProcs` 后调 `autostartNotice` | ✓ |
| `[boot-autostart-fix]` 锚字串三处一致 | README + install-service.sh `--help` + ServiceStatusCard.vue 各 1 处 | ✓ |
| install.sh / install.ps1 exit code 4 透传 | install.sh + install.ps1 顶端注释 + --help 文案均加 | ✓ |

### Dim 4 — Performance

| Item | Verdict | 一句话理由 |
|---|---|---|
| retry goroutine sleep 期间 CPU 占用 | PASS | `time.After(d)` 是 runtime timer，sleep CPU = 0。`select { <-ctx.Done() | <-time.After(d) }` 不轮询。 |
| service-status API 调用频次 | PASS | 仅 Dashboard mount + 用户点"刷新"触发，无前端 polling。系统级状态变化 = 用户跑 install-service.sh 后页面手动刷新，不需要实时反映。 |
| systemctl is-enabled spawn 频率 | PASS | 单次 < 50ms（实测）；5s context timeout 兜底极端慢盘。Dashboard 一次访问 = 一次 spawn，可接受。 |
| 二进制缺失 / 配置缺失早返回不启 retry goroutine | PASS | 永久失败短路，避免无意义反复 retry，节省资源。 |
| autoRestoreProcs first attempt 仍 ≤ 3s（procmgr.waitUntilStable 兜底） | PASS | ready gate opening 时延与旧版字节级一致（first attempt 同步）。 |

### Dim 5 — Security

| Item | Verdict | 一句话理由 |
|---|---|---|
| service-status API 走 SessionAuth 中间件 | PASS | router.go 挂在受保护组内（与 mode / proxies 等同等级别）。匿名访问返 401（dev §3.4 真机实测）。 |
| sc.exe qc 输出含敏感信息？ | PASS | sc.exe qc 只返回服务配置（binPath、start type、account），无密码/token；regex 仅匹配 START_TYPE 行不泄露。 |
| systemctl is-enabled spawn 命令注入风险 | PASS | 命令参数硬编码 `["systemctl", "is-enabled", "frp-easy.service"]`，无变量插值，无 shell 解释，零注入面。 |
| kv `system.autorestore.{kind}` JSON 序列化用户可控字符串注入？ | PASS | Reason 字段来自 `err.Error()`（procmgr 内部错误），不来自用户输入。即使被注入，json.Marshal 自动转义，API 返回再走 writeJSON 二次编码，前端 v-text 渲染，三层防御。 |
| autostartNotice 在 stderr 打印中文路径 | PASS | 路径硬编码 `/opt/frp-easy/scripts/install-service.sh`，无变量插值。 |
| install-service.sh / .ps1 自检失败时 status 输出 | PASS | journalctl 内容由 sed `s/^/    /` 缩进，不通过 shell eval，无注入面。 |
| frpc.toml `loginFailExit = false` 是否削弱安全？ | PASS | 字段控制的是 frpc 在登录失败时是否退出，与 token 验证 / 加密通道无关。frp 自身仍校验 token / TLS，仅"放弃 vs 重连"语义切换。 |

### Dim 6 — Maintainability

| Item | Verdict | 理由 |
|---|---|---|
| `internal/svcprobe/` 包独立 + build tag 分文件 | PASS | `probe_<os>.go` 三文件 + build tag 干净分离，符合 NFR-5。`probe.go` 入口 + Status struct 统一契约。 |
| retryBackoff 序列字面常量在 main.go 顶部 | PASS | 一目了然，常量值与设计 D-1 字面一致。 |
| `AutoRestoreAttempt` / `AutoRestoreLastRun` struct 在 main.go vs 应该在 internal/ | NIT-1 | 设计 02 §3.4 把 struct 放在 handler 内（SystemAutoRestoreSection 等），实施把 marshal 端的 struct 放在 main.go。两个包都需要类型——若做"共享"应抽到 internal/autorestore/types.go 包。但当前两侧通过 JSON 字符串通信（kv value），无 Go 类型耦合需要，独立各自定义合理。Nit，不阻塞。 |
| install-service.sh 注释段长度 | PASS | network-online 改动有 5 行块注释解释 why；自检块有锚字串注释。符合 .harness/rules/00-core.md "comments only where WHY non-obvious"。 |
| ServiceStatusCard.vue 模板段长度 | PASS | 单 SFC ~180 行物理 / 纯逻辑 < 100 行（script 段 ~50 行），符合 insight L34 "组件 > 200 行物理但纯逻辑判断"约定，未触发拆分红线。 |
| dev-map.md 更新延期到 stage 7 | NIT-2 | 04 §4.6 已明示 "stage 7 PM 一并更新"。沿用 T-036 / T-037 节奏。Nit。 |
| 主 select 优雅关停顺序 `rootCancel() → pm.Shutdown() → srv.Shutdown()` | PASS | rootCancel 先让 retry goroutine 退（不阻 10s shutdown ctx），再 pm.Shutdown 关 frpc/frps，最后关 HTTP。顺序正确。 |

## 2. Findings 汇总

| ID | 严重度 | 简述 | 处理 |
|---|---|---|---|
| MAJOR-1 | MAJOR | `autoRestoreProcs` retry 路径无 Go 单测（只有真机 reboot AC-4 实证） | **不阻塞**：单测需要 mock procmgr.Manager + storage.Store 双依赖且要假时间。真机 AC-4（dev §3.4）已铁证 + QA stage 6 会跑 ADV-3（iptables 模拟）。但建议未来 trivial 任务加 retry 路径单测覆盖 backoff 序列 + ctx cancel 路径。 |
| MINOR-1 | MINOR | persistAutoRestoreLast 在 parent ctx canceled 时丢一次 last-run 写入 | **接受**：仅影响 shutdown 时的"canceled" outcome 落盘，下次启动 autoRestoreProcs 会覆盖。 |
| NIT-1 | NIT | AutoRestoreLastRun struct 分散在 main.go + handlers_system.go | **接受**：两侧通过 JSON 字符串通信，无 Go 类型耦合需要。 |
| NIT-2 | NIT | dev-map.md 更新延期到 stage 7 | **沿用既有节奏**。 |
| 设计漂移 1（已 trace） | MINOR | `system.autorestore.last` 单 key → `system.autorestore.{kind}` per-kind | dev §4.2 已记录设计改进理由，文档已同步 |
| 设计漂移 2（已 trace） | NIT | `@/` alias → 相对路径 import | dev §4.5 已记录原因 |

无 **CRITICAL** finding。

## 3. 与历史任务横向对比

- T-035 install-sh-role-cli-arg-passthrough：CR 同款"GR conditions 100% 落实 → APPROVED 一次过"节奏 ✓
- T-036 log-ui-ux-polish：CR 同款"组件物理行数超 200 但纯逻辑 < 200 即接受"判定 ✓
- T-037 proxy-rules-simplify-and-port-fix：CR 同款"verify_all 守门删除面防回退" 已应用到本任务的新增面（I.x 守门 + loginFailExit / network-online / boot-autostart-fix 锚） ✓

## 4. Verdict

**APPROVED**

无 CRITICAL / MAJOR-blocking finding。MAJOR-1（retry 单测缺失）属可接受：真机 AC-4 + QA stage 6 ADV-3 提供端到端证据，比 mock 单测可信度更高。MINOR/NIT 累计 5 条，均不阻塞 stage 6。

Dev 阶段实施质量：GR §5 C-1~C-8 全部一次性落实到位，无返工。02 设计漂移 2 处（per-kind key + 相对路径 import）均有 dev §4 trace + 改进理由清晰，属合理调整不是错误。

下一步：PM 派发 QA Tester（stage 6）跑 06_TEST_REPORT.md。QA 必须包含：
- **AC-5 ADV-1**：临改 install-service.sh `network-online.target` → verify_all I.1 FAIL → 恢复 PASS（grep 闸门反向证伪）
- **AC-5 ADV-3**：测试机用 iptables 阻断 frps 端口 → reboot → 观察 frpc retry 序列 + 最终 outcome="exhausted" 写入 kv
- **AC-6**：测试机 Web UI 浏览器手动访问 + 截图证 Dashboard 顶部 ServiceStatusCard 渲染正确
- **AC-4 重复实证**（GR C-8）：旧 build（仅二进制回退）vs 新 build reboot 对照
