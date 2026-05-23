# 04 — Development Record · T-016 install-progress-and-systemd-unit-fix

> 阶段 4 / 7（Developer）。模式：**full**。
> 上游只读输入：[01_REQUIREMENT_ANALYSIS.md](01_REQUIREMENT_ANALYSIS.md) / [02_SOLUTION_DESIGN.md](02_SOLUTION_DESIGN.md) / [03_GATE_REVIEW.md](03_GATE_REVIEW.md)（APPROVED FOR DEVELOPMENT，含 6 条 Findings 与 8 条执行 hint）。
> 实施严格遵守 02 §D.1 落地清单与 GR §3 的 8 条 hint。

---

## Summary

按 02 §D.1 三组改动落地：
- **A 组（UX）** install.sh 步骤 5 把 `curl -fsSL` 改为 `curl -fSL` 并按 `[[ -t 2 ]]` 探测条件附加 `--progress-bar`；install.ps1 步骤 5 去掉 `-UseBasicParsing` 并外包 `$ProgressPreference` 临时开关 + `$isInteractive` 探测。
- **B 组（正确性）** install-service.sh 新增 `systemd_escape_path()` bash 函数（仅转空格→`\x20`），unit 模板的 `ExecStart=` / `WorkingDirectory=` 改为裸 token + `\x20` 转义；写出后新增 `systemd-analyze verify` 自检（**GR §F-3 降级路径**：失败仅 warn + 继续，不 rm unit）。
- **C 组（可观测）** install.sh 步骤 7 的 `if ! bash $SVC; then rc=$?` 改为 `set +e; bash $SVC; rc=$?; set -e` 三行块 + 失败时 `[diag]` 打印；install-service.sh daemon-reload / enable--now 两处同样改为 `set +e/-e` 包裹 + diag 打印，enable--now 失败块扩展为打印 `==== 诊断信息：systemctl status ... ====` 与 `==== 诊断信息：journalctl -u ... -n 20 ====` 两条字面前缀（AC-8 grep 锚点）+ unit 路径 + 清理提示；install.ps1 步骤 7 加 `$LASTEXITCODE = 0` 重置。install-service.ps1 经审视零改动。

verify_all 实施前后均 PASS:19，无退化。

## Files changed

| 文件 | 改动概要 | 行号范围（实施后） |
|---|---|---|
| `scripts/install.sh` | A.1+A.2：步骤 5 `curl -fSL $CURL_PROGRESS_FLAG`，`[[ -t 2 ]]` 决定附加 `--progress-bar`；C.1：步骤 7 退出码透传改 `set +e; bash; rc=$?; set -e` 三行块 + `[diag]` 打印 | 193-204；275-289 |
| `scripts/install.ps1` | A.3：步骤 5 去 `-UseBasicParsing` + `$ProgressPreference`/`$isInteractive` try/finally 包裹；C.3：步骤 7 `& $svc` 前加 `$LASTEXITCODE = 0` 重置 | 149-172；228-237 |
| `scripts/install-service.sh` | B.2.1：新增 `systemd_escape_path()` bash 函数（仅转空格→`\x20`，v2 D-1 修复后用单引号字面赋值 `local esc='\x20'` 绕开 bash 双引号 quote-removal 陷阱）；B.2：unit 模板 `ExecStart=${ESC_BINARY}` / `WorkingDirectory=${ESC_INSTALL_DIR}`（去外引号）；B.3 降级：`systemd-analyze verify` warn+继续；C.2：daemon-reload / enable--now 改 `set +e/-e` + diag 打印 + status / journalctl 强诊断块（含 AC-8 锚点字面前缀）+ unit 路径 + 清理提示 | 19-29（函数注释）；30-34（函数体）；133-155（模板）；164-209（verify + reload + enable + 诊断） |
| `scripts/install-service.ps1` | **无改动**（02 §C.3 / §D.1 已确认 `$LASTEXITCODE` 链路本身合规；本脚本所有 sc.exe 调用后立即 `if ($LASTEXITCODE -ne 0)` 显式分流；上游 install.ps1 已在 `& $svc` 前重置 `$LASTEXITCODE = 0` 兜底） | — |
| `.harness/insight-index.md` | **本阶段不动**（GR §F-4 hint 6：04 只写预拟 insight 文本，由 07 PM 决定原地替换 / archive-task） | — |

无新增模块、无新文件、无 schema/migrations 变更。

## Implementation notes

### A 组关键决策

1. **保留 `-f`**（FR-A.5）：`-f` 让 curl 在 HTTP 4xx/5xx 时退非 0，触发既有"发布包下载失败"中文报错 + exit 1 的分流不变。与步骤 3 GitHub API 不同——那里要看 403/404 响应体做限流/未发布分流，故那里**不能**用 `-f`（既有代码 L128 也确实没用）。
2. **`[[ -t 2 ]]` 探测 stderr 是否 TTY**（FR-A.3 / FR-A.6 / BC-A.5）：进度条由 curl 写 stderr；用户 `2>install.log` 重定向时自动降级为不附加 `--progress-bar`，避免 `\r` 覆盖序列污染日志。`[[ -t N ]]` 是 bash 内建，无依赖（NFR-5）。
3. **PowerShell 端 `Invoke-WebRequest` + `$ProgressPreference`**（02 §A.3）：去掉 `-UseBasicParsing` 解放 PS 5.x 的 Write-Progress；PS 7+ 该 flag 已是 no-op。`$prevProgress` 保存/恢复保证不污染调用方环境；`try/finally` 在 catch+exit 路径也会执行恢复。

### B 组关键决策

1. **裸 token + `\x20` 转义**（02 §B.2）：systemd.exec(5) 明确 `WorkingDirectory=` 路径含特殊字符必须 C-style 转义；整体双引号被任意版本 systemd 拒为 `bad unit file setting`（T-008 旧 insight 反例的真因）。OOS-2 限定仅支持 ASCII + 空格，单字符替换足够。
2. **bash heredoc 转义陷阱**：`${p// /\\x20}` 中 `\\` 是字面反斜杠（bash 内的单字符 `\`），结果字符串字面即 4 字符 `\x20`。heredoc 用 `<<EOF`（unquoted）做变量展开，但反斜杠保持字面。已通过附录 hex dump 验证（见下方）。
3. **systemd-analyze verify 降级路径**（GR §F-3 hint 4）：原设计 §B.3 是 "fatal: 失败 rm unit + exit 2"；GR 提出 systemd 249-255 历史误报风险（如 Documentation= URL 不可达），采纳降级为 "warn + 继续"。落地的 if 块仅打印诊断到 stderr 然后继续 daemon-reload，最终事实源是 daemon-reload + enable--now。这与 02 §B.3 是 **GR §F-3 显式允许的"非阻断"实现细节调整**，不构成 DESIGN DRIFT（RA 未硬约束 verify 为 fatal 路径）。

### C 组关键决策

1. **`set +e; cmd; rc=$?; set -e` 三行块**（02 §C.1，PM 默认 §8.4 (a)）：脚本顶部 `set -euo pipefail` 让直接调用非 0 命令立即终止；`if ! cmd; then rc=$?` 反模式则因 bash `set -e` 在 `if` 上下文不生效 + `!` 反转后 then 块内 `$?` 跨 bash 版本语义差异而不可靠。三行块意图明确、跨版本一致。
2. **diag 打印**（GR §F-1 hint 4）：install.sh 步骤 7 与 install-service.sh daemon-reload / enable--now 三处的 set +e/-e 块后均加 `[[ $rc -ne 0 ]] && echo "    [diag] <step> rc=$rc" >&2`，仅失败时打。这是为 QA AC-7 与未来线上排障留可观察证据，回应 GR §F-1 "退出码报 0 的根因推理不完整"——通过 diag 打印固化每一步真实退出码，绕过 systemctl 的退出码模糊性。
3. **AC-8 字面前缀锚点**（GR §F-2 hint 5 / hint 7）：install-service.sh enable--now 失败块严格使用 `==== 诊断信息：systemctl status $UNIT_NAME --no-pager ====` 与 `==== 诊断信息：journalctl -u $UNIT_NAME --no-pager -n 20 ====` 两条字面前缀作为 AC-8 grep 锚点（见下方 "Open issues for review"）。
4. **install-service.ps1 零改动**（02 §D.1 / §C.3 显式允许）：审视该脚本，所有 sc.exe 调用都立即 `if ($LASTEXITCODE -ne 0)` 显式分流；上游 install.ps1 已在 `& $svc` 前加 `$LASTEXITCODE = 0` 重置兜住 "terminating error 未走到 exit" 的角落。零改动符合 02 §D.1 决策。

### 设计偏差（DESIGN DRIFT）

- **systemd-analyze verify 路径**：02 §B.3 设计为 "失败 rm unit + exit 2"，本实施按 GR §F-3 hint 3 降级为 "warn + 继续"。**不算 DESIGN DRIFT**，因为：
  1. GR 已明示该降级为允许的实现细节调整；
  2. RA AC 未硬约束 verify 为 fatal 路径（AC-4/AC-5 验收点是 `systemctl is-active frp-easy = active`，不涉及 verify 自身退出语义）；
  3. 02 §B.3 在文末已为该降级留口（"若 NFR-1 任一目标发行版手测时 verify 误报 fatal..."）。

无其它实施偏差。

## Heredoc 字节验证（GR hint 2 R-2 硬要求）

**04 v2 修复**：原 v1 附录的 hex dump 基于错误的 ad-hoc 测试脚本（与 committed 源码 `${p// /\\x20}` 不一致），QA 在 06 §A1 独立 reproducer 中发现该函数实测输出 `/opt/frpx20easy/v1`（无反斜杠）。本节为修复 D-1 后的真实字节验证。

**修复实现**：`scripts/install-service.sh:24-28` 改为单引号字面赋值 `local esc='\x20'` 到变量，参数扩展引用 `$esc` 不再触发 bash 双引号 + parameter expansion 内嵌反斜杠的 quote-removal 陷阱。

**验证方法**：临时脚本 `/tmp/dev_v2_hex_verify.sh`（用完即删，已删除）verbatim 复制 install-service.sh L24-28 的函数 + 同结构 heredoc 模板，在 Git Bash 5.2.37 上跑两条路径用例并对 ExecStart / WorkingDirectory 两行做 `xxd` 字节级 dump。

**Windows 开发机限制说明**：实施在 Windows 开发机上完成。Windows 上可跑 bash（Git Bash 5.2.37 实测）但无 `systemctl` / `systemd-analyze`，**无法实机 sudo 跑 install-service.sh 验证 systemd 行为**；hex dump 仅证明 unit 文件字节正确，systemd 自身验证由 QA / 用户在目标 Linux 发行版完成。

### 用例 (a) — 默认路径 `/opt/frp-easy`（无空格）

期望：`ExecStart=/opt/frp-easy/frp-easy` 与 `WorkingDirectory=/opt/frp-easy`，无 `\x20`。

实测输出（v2 修复后）：

```
=== Case: default ===
--- grep ExecStart / WorkingDirectory ---
ExecStart=/opt/frp-easy/frp-easy
WorkingDirectory=/opt/frp-easy
--- xxd of those two lines ---
| ExecStart=/opt/frp-easy/frp-easy
00000000: 4578 6563 5374 6172 743d 2f6f 7074 2f66  ExecStart=/opt/f
00000010: 7270 2d65 6173 792f 6672 702d 6561 7379  rp-easy/frp-easy
00000020: 0a                                       .
| WorkingDirectory=/opt/frp-easy
00000000: 576f 726b 696e 6744 6972 6563 746f 7279  WorkingDirectory
00000010: 3d2f 6f70 742f 6672 702d 6561 7379 0a    =/opt/frp-easy.
```

**断言**：两行 hex 均不含字节序列 `5c 78 32 30`（即 `\x20`），与期望一致；pattern ` ` 不匹配 → 无替换发生，字节直通。

### 用例 (b) — 含空格路径 `/opt/frp easy/v1`

期望：`ExecStart=/opt/frp\x20easy/v1/frp-easy` 与 `WorkingDirectory=/opt/frp\x20easy/v1`，含 4 字符字面 `\x20`（hex `5c 78 32 30`）。

实测输出（v2 修复后）：

```
=== Case: spaced ===
--- grep ExecStart / WorkingDirectory ---
ExecStart=/opt/frp\x20easy/v1/frp-easy
WorkingDirectory=/opt/frp\x20easy/v1
--- xxd of those two lines ---
| ExecStart=/opt/frp\x20easy/v1/frp-easy
00000000: 4578 6563 5374 6172 743d 2f6f 7074 2f66  ExecStart=/opt/f
00000010: 7270 5c78 3230 6561 7379 2f76 312f 6672  rp\x20easy/v1/fr
00000020: 702d 6561 7379 0a                        p-easy.
| WorkingDirectory=/opt/frp\x20easy/v1
00000000: 576f 726b 696e 6744 6972 6563 746f 7279  WorkingDirectory
00000010: 3d2f 6f70 742f 6672 705c 7832 3065 6173  =/opt/frp\x20eas
00000020: 792f 7631 0a                             y/v1.
```

**断言**：
- ExecStart 行 offset 0x14-0x17 为 `5c 78 32 30`（字面 `\x20`），正好替换原空格位置；ASCII 视图列 `rp\x20easy/v1/fr` 直观可见反斜杠。
- WorkingDirectory 行 offset 0x13-0x16 为 `5c 78 32 30`，ASCII 视图列 `=/opt/frp\x20eas`，反斜杠就位。
- 两行均为单一 4 字符 `\x20` 转义序列（非 6 字符 `\\x20` 双反斜杠陷阱，也非 v1 报告里的 `x20` 无反斜杠错误），D-1 BLOCKER 已修复，R-2 风险确认排除。

临时脚本 `/tmp/dev_v2_hex_verify.sh` 已删除，仓库与 /tmp 无垃圾文件残留。

## verify_all

### 实施前 baseline（PowerShell 7 / Windows 11）

```
=== verify_all (fullstack) ===
[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO / FIXME budget (warn only) ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[B.5] No tsc residue in web/src/ ... PASS
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agent definitions present in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa) ... PASS
[E.6] Adversarial tests section present in completed task reports ... PASS
=== Summary === PASS: 19  WARN: 0  FAIL: 0  SKIP: 0
```

### 实施后回归（同机 / 同命令）

```
=== verify_all (fullstack) ===
[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO / FIXME budget (warn only) ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[B.5] No tsc residue in web/src/ ... PASS
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agent definitions present in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa) ... PASS
[E.6] Adversarial tests section present in completed task reports ... PASS
=== Summary === PASS: 19  WARN: 0  FAIL: 0  SKIP: 0
```

**Delta**：PASS:19 → PASS:19；WARN/FAIL/SKIP 全部 0；NFR-7 / AC-11 满足。A.1 secrets scan 未误中新增字面量（`--progress-bar`、`\x20` 转义、`==== 诊断信息：` 块、`$LASTEXITCODE = 0` 均不命中正则 `(api[_-]?key|secret|password|token)[[:space:]]*[:=][[:space:]]*['"][^'"]{8,}['"]`）。

## Design drift (if any)

**无 DESIGN DRIFT**。systemd-analyze verify "warn + 继续" 降级路径由 GR §F-3 hint 3 显式允许，已在 "Implementation notes / 设计偏差" 段说明。

## Open issues for review

给 Code Reviewer / QA 的关注点：

1. **AC-8 grep 锚点（GR §F-2 / hint 7）**：QA 验证 AC-8 时使用以下两条字面前缀作为精确 grep 锚点：
   - `==== 诊断信息：systemctl status frp-easy --no-pager ====`
   - `==== 诊断信息：journalctl -u frp-easy --no-pager -n 20 ====`
   两条均在 `scripts/install-service.sh` enable--now 失败块内（v2 修复后约 L200-208）。QA 在 06_TEST_REPORT.md `## Adversarial tests` 段制造 enable--now 失败时，`grep -F "==== 诊断信息：" install.log` 应至少匹配两行。
2. **GR §F-1 退出码诊断验证**：QA 模拟 enable--now 失败（如 chmod 000 binary）后，install.sh stderr 应同时包含：
   - `    [diag] systemctl enable --now frp-easy rc=2`（来自 install-service.sh）
   - `    [diag] install-service.sh rc=2`（来自 install.sh）
   - `错误：服务注册失败（install-service.sh 退出码 2）。`
   这三条联合证明退出码透传链路修复了线上 "报告退出码 0" 的 bug。
3. **systemd-analyze verify 降级路径手测确认**：QA 在 Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 四个发行版各跑一次 AC-4 / AC-5，确认 verify 既不误报 fatal、又能在真实语法错误时给出 stderr 提示。若任一发行版手测发现 verify 在合法 unit 上输出 warning（非 fatal），属允许的"warn+继续"路径，不构成 FAIL。
4. **PS 5.x 去 `-UseBasicParsing` 兜底（R-3）**：极简 Windows Server Core 无 IE COM 时 PS 5.x IWR 可能报错；现有 `try { ... } catch { Write-Error "发布包下载失败..." ; exit 1 }` 已能兜住。NFR-1 未列 Server Core，本任务范围内不专门测。
5. **install-service.ps1 零改动决定**：02 §C.3 / §D.1 显式允许零改动；该脚本所有 sc.exe 调用后立即检查 `$LASTEXITCODE` 并显式分流（L97-100、L103-106、L119-123），上游 install.ps1 已加 `$LASTEXITCODE = 0` 重置兜底，链路合规。如 Reviewer 发现 corner case 需补，可在 install-service.ps1 头部 sc.exe 调用前依次加 `$LASTEXITCODE = 0` 重置。
6. **分区分配**（GR §F-5）：PM 已按 GR 建议改派通用 `developer`（即本 agent），无需写 `## Partition exception` 段。本任务全部改动落在 `scripts/install*.{sh,ps1}` 与 `scripts/install-service.{sh,ps1}`，与 dev-backend.md / dev-frontend.md / dev-db.md 任一 Owned paths 列表无冲突（通用 developer 无 owned paths 约束）。

## Dev-map updates

**无变更**。本任务未新增 / 移动 / 删除模块或文件；`docs/dev-map.md` `scripts/` 段中既有的 `install.{sh,ps1}`（T-012 行）与 `install-service.{sh,ps1}`（T-008 行）描述仍准确，未触发更新条件。

## Insight to surface

预拟一行（GR §F-4 hint 6，由 07 PM 决定原地替换 .harness/insight-index.md 第 18 行还是走 archive-task 流程）：

```
- 2026-05-23 · systemd unit `ExecStart=` / `WorkingDirectory=` 路径必须裸 token 写入（`ExecStart=/opt/frp-easy/frp-easy`），含空格路径用 C-style 字面 `\x20`（`/opt/frp\x20easy/v1`）；整体双引号写法 `WorkingDirectory="/path"` 被 systemd 任意版本拒为 bad unit file setting（T-008 旧 insight 第 18 行错误已纠正）；写 unit 后 systemd-analyze verify 可作 daemon-reload 前的早期自检，但 systemd 249-255 历史上对合法 unit 偶有误报 fatal，应取 "warn + 继续" 降级而非 fatal+rm · evidence: T-016 install-progress-and-systemd-unit-fix
```

## verify_all (after D-1 fix)

D-1 BLOCKER 修复后回归（PowerShell 7 / Windows 11 / `scripts\verify_all.ps1` 2026-05-23）：

```
=== verify_all (fullstack) ===
Project: frp_easy
Stack:   Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)

[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO / FIXME budget (warn only) ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[B.5] No tsc residue in web/src/ ... PASS
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agent definitions present in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa) ... PASS
[E.6] Adversarial tests section present in completed task reports ... PASS

=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

**Delta**：v1 实施后回归 PASS:19 → v2 D-1 修复后回归 PASS:19；WARN/FAIL/SKIP 全 0。A.1 secrets scan 未误中新增 `local esc='\x20'` 字面量（正则 `(api[_-]?key|secret|password|token)[[:space:]]*[:=][[:space:]]*['"][^'"]{8,}['"]` 要求关键字命中，`esc` 不在词表内）。

## D-1 fix 复盘

**真因**：bash 双引号 + parameter expansion 的 quote-removal 顺序。`printf '%s' "${p// /\\x20}"` 中 REPLACEMENT 段 `\\x20` 在双引号内先被 quote-removal 处理一轮，`\\` 还原为单个 `\`；随后参数扩展把这个单 `\` 当作 string 内的转义引导符吞掉，留下 `x20` 三字符。要让 REPLACEMENT 保留字面 `\x20`（hex `5c 78 32 30`），必须绕开双引号 quote-removal——v3 方案用单引号字面赋值 `local esc='\x20'`（单引号内不做 quote-removal），再以 `$esc` 参数扩展引用，replacement 即为字面 4 字符 `\x20`。

**为什么 v1 hex dump 是假的**：v1 阶段 Developer 在 tmp 脚本 `tmp_verify_heredoc.sh` 里写的 `systemd_escape_path()` 实现**与 committed `install-service.sh:24-27` 不完全一致**——大概率是 tmp 脚本里用了 `\\\\x20`（4 反斜杠）或 sed 实现的早期试验版本，但 commit 时落到代码里的是 `\\x20`（2 反斜杠）。Developer 没有以"先 commit 再用 commit 后的源码 verbatim copy 回测"的工作流校验，导致 tmp 脚本与产物分叉；hex dump 看起来正确，但被验证的不是真实代码。CR 阶段直接信任了 hex dump 字面，没独立从 committed `install-service.sh` source 函数复跑，BLOCKER 漏到 QA。

**教训**：heredoc / parameter expansion 类的字节级行为验证有两条铁律 ——
1. **永远不复用"测试脚本"里的函数定义**：tmp 验证脚本必须 `source` committed 文件或 verbatim copy 该文件的对应行号区间（带行号注释，便于审计 diff），杜绝 tmp 实现与产物分叉。
2. **CR / QA 必须独立复现而非信任 hex dump 字面**：bash 转义规则反人类，hex dump 给的是结果不是过程；reviewer 应自己写一句 `bash -c '...verbatim function...; out=$(...); printf "%s" "$out" | xxd'` 跑一次，再决定是否接受。

本次修复在改 install-service.sh 同时把 tmp 脚本路径写进 04（含 cleanup 声明），便于 CR 重跑同一流程独立比对。

## Verdict

READY FOR REVIEW
