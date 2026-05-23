# 05 — Code Review · T-016 install-progress-and-systemd-unit-fix

> 阶段 5 / 7（Code Reviewer）。模式：**full**。
> 上游只读输入：[01_REQUIREMENT_ANALYSIS.md](01_REQUIREMENT_ANALYSIS.md) / [02_SOLUTION_DESIGN.md](02_SOLUTION_DESIGN.md) / [03_GATE_REVIEW.md](03_GATE_REVIEW.md) / [04_DEVELOPMENT.md](04_DEVELOPMENT.md)。
> 评审范围：本任务实际改动的 4 个脚本（含 install-service.ps1 零改动复核）+ `.harness/insight-index.md` 未被本阶段触碰之确认。

---

## Files reviewed

- `scripts/install.sh`
- `scripts/install.ps1`
- `scripts/install-service.sh`
- `scripts/install-service.ps1`（零改动复核）
- `.harness/insight-index.md`（确认 04 未改）

---

## 必查清单 10 项

| # | 必查项 | 状态 | 一句话理由 |
|---|---|---|---|
| 1 | 设计落实（02 §D.1） | **PASS** | 4 文件改动全部落地；install-service.ps1 零改动符合 02 §C.3 / §D.1 + 04 第二段论述（所有 sc.exe 调用后立即 `if ($LASTEXITCODE -ne 0)` 显式分流 + 上游 install.ps1 已 `$LASTEXITCODE = 0` 兜底）。 |
| 2 | GR 8 条 hint 吸收 | **PASS** | hint 1（无 Partition exception 段）✓；hint 2（heredoc hex dump 在 04 L72-114）✓；hint 3（verify warn+继续）✓；hint 4（三处 diag 打印）✓；hint 5（PS try/catch 保留）✓；hint 6（insight-index 未改）✓；hint 7（AC-8 字面前缀严格）✓；hint 8（verify_all 前/后两份 PASS:19）✓。 |
| 3 | bash 正确性 | **PASS** | `set +e; cmd; rc=$?; set -e` 三处皆紧凑无穿插；`set -e` 临时关闭不影响 `u` 与 `pipefail`；`${p// /\\x20}` 写法正确（已 hex dump 验证）；函数定义位置在 set 之后、参数解析之前；heredoc `<<EOF` 反斜杠保字面。 |
| 4 | PowerShell 正确性 | **PASS** | `$LASTEXITCODE = 0` 紧贴 `& $svc`；`$ProgressPreference` try/finally 恢复正确（外层 try/finally 恢复 prev、内层 try/catch 处理下载异常）；`[Console]::IsErrorRedirected` 在 PS 5.1 + .NET 4.x 完全支持。 |
| 5 | 退出码契约表（02 §C.4） | **PASS** | 7 场景在代码里都能复现：成功 0；前置失败 1；daemon-reload / enable--now / verify（已降级）→ 链路对齐。 |
| 6 | verify_all 不退化 | **PASS** | 04 两份输出均 PASS:19；A.1 secrets scan 正则核对本任务新增字面量（`--progress-bar`、`\x20`、`==== 诊断信息：`、`$LASTEXITCODE = 0`）均不命中。 |
| 7 | insight-index 未被改 | **PASS** | `.harness/insight-index.md` 第 18 行仍是 T-008 旧错误 insight，未被 04 触碰，符合 GR §F-4 / hint 6。 |
| 8 | R-1 至 R-8 风险缓解 | **PASS** | R-1 command -v 探测 ✓；R-2 hex dump ✓；R-3 try/catch 保留 ✓；R-4 三行块紧凑 ✓；R-5 NFR-1 不覆盖 ✓；R-6 `-n 20` + sed 缩进 ✓；R-7 warn+继续降级 ✓；R-8 阶段 7 任务。 |
| 9 | 代码风格 / 注释 / 依赖 | **PASS（注释稍多但全 WHY）** | 新增注释全部解释"为什么"（PM 默认、设计 §章节、GR finding 引用），符合 CLAUDE.md "仅 WHY 非显然时写注释" 精神。无新 import / 依赖。 |
| 10 | 回归风险（macOS / 升级 / help） | **PASS** | macOS L258-268 / 升级 L228-253 / help L31-77 / install.ps1 help L36-71 全部未动。 |

---

## Findings

### CRITICAL
（无）

### MAJOR
（无）

### MINOR

- **[MAINT] `scripts/install-service.sh:166-202`** — `set +e; cmd; rc=$?; set -e` 三行块在 daemon-reload 与 enable--now 两处重复出现；后续若新增第三/四处 systemctl 调用建议提炼 `run_with_rc()` helper。当前两次重复在可读性范围内，**不阻断**。

### NIT

- **[STYLE] `scripts/install-service.sh:158`** — `verify_out="$(systemd-analyze verify "$UNIT_PATH" 2>&1)" && verify_rc=0 || verify_rc=$?` 一行链式比 `set +e/-e` 块更紧凑，但与本任务其他两处采用的"显式 set +e/-e 三行块"风格不完全统一。现行写法在 verify 不阻断主流程的语义下合理（rc 仅用于打 warn）。
- **[STYLE] `scripts/install.sh:201`** — `curl -fSL $CURL_PROGRESS_FLAG -o "$TARBALL"` 未对 `$CURL_PROGRESS_FLAG` 引号包裹。该变量只可能是空串或 `--progress-bar`（脚本内 100% 控制赋值），不存在词分裂注入风险；如未来扩展为多 flag 数组，需改为 `${CURL_PROGRESS_FLAGS[@]}`。
- **[STYLE] `scripts/install-service.sh:194`** — `systemctl status ... 2>&1 | sed ... >&2 || true` 中的 `|| true` 防御性写法，安全但冗余。

---

## Requirement coverage

| AC | Implementation | Status |
|---|---|---|
| AC-1（curl 进度可见） | `install.sh:197-201` | PASS |
| AC-2（PS 进度可见） | `install.ps1:149-172` | PASS |
| AC-3（进度不掩盖下载失败） | `install.sh:201-204`（保留 `-f` + 中文报错 + exit 1） | PASS |
| AC-4（默认路径 systemctl is-active = active） | `install-service.sh:117-148`（裸 token unit 模板） | PASS（待 QA 实机） |
| AC-5（含空格路径 systemctl is-active = active） | `install-service.sh:24-27, 119-120`（`\x20` 转义，hex dump 已验证） | PASS（待 QA 实机） |
| AC-6（跨发行版） | 同 AC-4 + NFR-1 覆盖矩阵 | PASS（待 QA 实机） |
| AC-7（退出码透传 == 2） | `install.sh:280-289` + `install-service.sh:184-202` | PASS |
| AC-8（实际诊断片段） | `install-service.sh:193-197`（字面前缀 + status/journalctl + sed 缩进） | PASS |
| AC-9（PS 退出码透传） | `install.ps1:234-239` | PASS |
| AC-10（macOS 退出 0） | `install.sh:258-268`（未动） | PASS |
| AC-11（verify_all PASS:19） | 04 L120-168 实测两次 PASS:19 | PASS |
| AC-12（升级幂等保留配置） | `install.sh:228-253`（未动） | PASS |
| AC-13（unit 路径 + 清理提示） | `install-service.sh:174-175, 199-200` | PASS |

## Design fidelity

| Design item | Implementation | Status |
|---|---|---|
| A.1 curl `-fSL` + `--progress-bar` 条件 | `install.sh:197-201` | PASS |
| A.2 `[[ -t 2 ]]` TTY 探测 | `install.sh:198` | PASS |
| A.3 PS 去 `-UseBasicParsing` + `$ProgressPreference` | `install.ps1:156-172` | PASS |
| B.1 systemd unit 模板（裸 token + `\x20`） | `install-service.sh:126-144` | PASS |
| B.2.1 `systemd_escape_path()` | `install-service.sh:24-27` | PASS |
| B.3 systemd-analyze verify（warn+继续） | `install-service.sh:157-164` | PASS（GR §F-3 hint 3 显式授权） |
| C.1 install.sh set +e/-e 三行块 | `install.sh:280-283` | PASS |
| C.2 install-service.sh 强诊断块 | `install-service.sh:184-202` | PASS |
| C.3 PS `$LASTEXITCODE = 0` 重置 | `install.ps1:234` | PASS |
| install-service.ps1 零改动 | 链路合规 | PASS |
| AC-8 字面前缀字符串 | `install-service.sh:193, 196` 严格匹配 | PASS |
| GR §F-1 diag 打印（三处） | `install.sh:284, install-service.sh:171, 188` | PASS |

**无 DESIGN DRIFT**。systemd-analyze verify "warn+继续" 由 GR §F-3 显式授权。

---

## Verdict

# APPROVED

无 CRITICAL / MAJOR；1 条 MINOR（helper 抽取建议，非阻断）+ 3 条 NIT（风格）。QA 可进入阶段 6。

---

## 给阶段 6 QA 的提示

**务必覆盖**：

1. **AC-4 + AC-5 实机系统集成**：本审计在 Windows 开发机进行，无 systemctl，无法实机验证。QA 必须在 Ubuntu 22.04 + 24.04 + Debian 12 + RHEL 9 至少 2 个发行版各跑一次默认路径 + 含空格路径安装，确认 `systemctl is-active frp-easy = active`。
2. **AC-7 退出码透传链路**：`chmod 000 /opt/frp-easy/frp-easy` 后重跑 install-service.sh，必须同时看到：
   - `    [diag] systemctl enable --now frp-easy rc=2`
   - `    [diag] install-service.sh rc=2`（通过 install.sh 间接调用时）
   - `错误：服务注册失败（install-service.sh 退出码 2）。`
3. **AC-8 grep 锚点**：`grep -F "==== 诊断信息：" install.log` 应至少匹配 2 行（status + journalctl）。
4. **AC-12 升级幂等**：跑两次安装，第二次走升级分支且 `frp_easy.toml` / `.frp_easy/` / `frp_linux/` mtime+sha256 不变。
5. **systemd-analyze verify warn+继续路径手测**：每发行版记录 verify_rc 与 stderr 摘要到 06。

**可放过**（OOS / 已论证）：Alpine / busybox curl / Windows Server Core / 含 shell 元字符路径 / systemd < 230 / macOS launchd / Windows Event Log 诊断 / PS 5.x Server Core IWR 失败。

**Adversarial tests 段建议**：复用 02 §D.4 的 A1-A6，至少执行 A1（含空格路径）、A2（断网）、A3（删 binary）、A5（含空格路径启动失败诊断）四条。
