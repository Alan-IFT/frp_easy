# 03 — Gate Review · T-016 install-progress-and-systemd-unit-fix

> 阶段 3 / 7（Gate Reviewer）。模式：**full**。
> 上游只读输入：[01_REQUIREMENT_ANALYSIS.md](01_REQUIREMENT_ANALYSIS.md)（Verdict = READY） + [02_SOLUTION_DESIGN.md](02_SOLUTION_DESIGN.md)（Verdict = READY）。
> 评审范围：13 条 AC 完整性、设计是否真正解决线上 bug、风险与边界、分区分配、verify_all 不退化。

---

## 1. 8 维度审计

| # | 维度 | 状态 | 一句话理由 |
|---|---|---|---|
| 1 | Requirement completeness | **PASS** | 13 条 AC 全部对应可观察命令；§4 边界、§5 OOS、§8 PM 默认全显式；唯一弱点见 F-2 关于诊断片段精确锚点。 |
| 2 | Design completeness | **WARN** | A/B/C 三组每条 FR 均有具体修改点，但 §C.1 对"线上 bug：install.sh 报告退出码 0"的根因推理**未闭环**——见 F-1。 |
| 3 | Reuse correctness | **PASS** | §7 复用表与实际文件比对一致：`install-service.sh` L107-133 原子写 mv-rename 与双重 chmod 模式真实存在；`install.ps1` L214-218 `$LASTEXITCODE` 链路代码真实存在；bash 内建 `[[ -t 2 ]]` 与 `--progress-bar`、`$ProgressPreference` 均是无依赖原语。 |
| 4 | Risk coverage | **PASS** | R-1 至 R-8 共 8 条，3 条最关键风险（R-2/R-4/R-1）均给出可执行缓解。 |
| 5 | Migration safety | **PASS** | 无 DB 迁移；脚本改动是非破坏性纯修复；§9 已显式覆盖"已安装 v0（旧含引号 unit）→ 重跑 install.sh → 步骤 6 升级分支 → 写新 unit 覆盖旧 unit"路径，符合 NFR-8 / AC-12。 |
| 6 | Boundary handling | **PASS** | BC-A.1-5 / BC-B.1-6 / BC-C.1-4 全部映射到设计点；元字符路径走 OOS-2 显式排除并由 systemd-analyze verify 兜底。 |
| 7 | Test feasibility | **WARN** | 12/13 条 AC 直接可由 QA 跑命令观察。AC-8 仍是该批最软的（见 F-2）。 |
| 8 | Out-of-scope clarity | **PASS** | §10 列了 9 条 OOS，Developer 无重大过度建设风险。 |

---

## 2. Findings（具体问题 + 责任上游文档）

### F-1（WARN，责任 02 §C.1）—— 退出码报 0 的根因推理不完整

**问题**：用户证据明确"`==> 写入 unit 文件 ...` 成功 → `Failed to start ...: bad unit file setting` → install-service.sh 报退出码 0"。但 install-service.sh L137-145 在 daemon-reload 或 enable--now 失败时**明确写了** `exit 2`（已读源码核实）。02 §C.1 给出的解释（"`if ! cmd` 加 `set -e` 在 if 上下文不生效，但 then 里 `$?` 在某些 bash 下可能不可靠"）只解释了 install.sh 端 `if ! bash $SVC` → `then rc=$?` 取错误值的可能，**并未解释 install-service.sh 自身为何"报退出码 0"**。

bash 5.x 下 `if ! cmd; then rc=$?` 的实测语义：then 块第一行 `$?` 取的是 cmd 真实退出码——与用户观察矛盾。

可能的真实原因（02 未论述任何一条）：
- (a) install.sh 步骤 8 的最终 `exit 0`（L307）被混淆——但用户证据明确是 install.sh 自己报"install-service.sh 退出码 0"那一行；
- (b) `systemctl daemon-reload` 在 systemd 看来"reload 本身成功"返回 0（即使 unit 语法非法，systemd 仅在 enable/start 时真正验证）；接着 `systemctl enable --now` 拆解为 enable（创建 symlink，**成功**返回 0）+ start（启动失败但 systemctl 仍以 0 退出在某些版本上是已知行为）—— 这能解释用户证据中"Created symlink ... → Failed to start ... → 但退出码 0"；
- (c) bash EXIT trap（L98）执行的 `rm -f` 改写退出码（实际不会，rm -f 无文件时返回 0 但**不**改写脚本退出码，除非 trap 内 `exit` 显式调用）。

**最可能根因**：(b)——systemd 的 enable --now 在 unit 已写但 start 阶段失败的特定时序下返回 0，与 systemd 版本/启动模式相关的已知行为。

**修复路径**：SA 的 `set +e; cmd; rc=$?; set -e` + systemd-analyze verify 自检 + C.2 强诊断三件套**应该能消除症状**（verify 主动自检会在 daemon-reload 之前就抓住语法错误返回 2，绕过 systemctl 的退出码模糊性）。但 Developer 应在 04 实施时**额外**把 `set +e; systemctl daemon-reload; rc=$?; set -e` 与 `set +e; systemctl enable --now ...; rc=$?; set -e` 两处的 rc **打印到 stderr 作为诊断**，验证修复后真的报 2 而非 0。

### F-2（WARN，责任 01 AC-8 + 02 §C.2）—— AC-8 诊断片段验证标准模糊

**问题**：AC-8 原文"stderr/stdout 包含至少一条 systemctl status 或 journalctl 的实际输出片段（不仅是'请查看 journalctl'提示）"——QA 拿这条 AC 跑：怎么算"实际输出片段"？字数下限？是否要求出现关键字如 `Loaded:` `Active:`？

**建议**：Developer 在 04 实施时锚定 02 §C.2 设计的具体字符串前缀（"==== 诊断信息：systemctl status $UNIT_NAME --no-pager ====" 与 "==== 诊断信息：journalctl -u $UNIT_NAME --no-pager -n 20 ===="），并在 04_DEVELOPMENT.md 中以"该前缀作为 AC-8 的 QA grep 锚点"形式写进 Open issues for review，让 QA 拿到精确的 grep 模式。**不构成阻断**。

### F-3（WARN，责任 02 §B.3）—— `systemd-analyze verify` 极端误报风险

**问题**：systemd-analyze verify 在 NFR-1 列出的 4 个发行版上跨版本（systemd 249-255）行为不完全一致。多个公开报告（fedora bugzilla / systemd github issues 2021-2023）显示 verify 历史上有**误报为 fatal** 的情况（如对 `Documentation=` URL 不可达警告），并在 systemd 250+ 收紧后才稳定。

**建议（不构成阻断）**：Developer 在 04 实施 B.3 时，在 `systemd-analyze verify` 调用上加 `2>&1` 捕获 stderr。若 NFR-1 任一目标发行版手测时 verify 误报 fatal 阻断正确 unit，**降级方案**：把 verify 失败的语义从"exit 2 + rm unit"改为"打印 verify 输出到 stderr 作为 WARN + 继续 daemon-reload"，由 daemon-reload + enable--now 作为最终事实源。这种降级属允许的实现细节调整（不构成 DESIGN DRIFT，因为 RA 没硬约束 verify 是 fatal 路径）。

### F-4（NOTE，责任 02 §D.5 + `.harness/rules/05-insight-index.md`）—— insight 更新策略合规

**核查结果**：`.harness/rules/05-insight-index.md` 允许"任务结束时编辑"该索引文件，并非纯 append-only。SA §D.5 提议的"原地替换 + 老 insight 搬到 docs/features/_archived/insight-history.md（前缀 `[CORRECTED by T-016]`）"策略**符合规则精神**。**通过**。

### F-5（NOTE，责任 02 §11）—— 分区分配越权但有明确说明

**核查**：`.harness/agents/dev-backend.md` Owned paths 明确列出 `scripts/start.{ps1,sh}、scripts/build.{ps1,sh}、scripts/verify_all.{ps1,sh}`，**未列** `scripts/install*.{sh,ps1}` 或 `scripts/install-service.{sh,ps1}`。dev-backend Hard rule 7 字面会让 Code Reviewer 阶段 5 误判 BLOCKED ON PARTITION。

**建议**：**PM 在派发阶段 4 时改派通用 `developer` agent 而非 dev-backend**——通用 developer 无 owned paths 限制（"only agent that writes production code"），更适配本任务（涉及 systemd / PowerShell / curl 等系统工程）。若坚持 dev-backend，Developer 必须在 04 顶部加 `## Partition exception` 段显式说明 SA 授权 + 引用 02 §11，否则 Code Reviewer 会回退。

### F-6（PASS，责任 02 §9）—— verify_all 不退化

**实测核查**：`scripts/verify_all.sh` secrets scan 正则 `(api[_-]?key|secret|password|token)[[:space:]]*[:=][[:space:]]*["'][^"']{8,}["']`。逐项核对本任务新增内容（A 组 `--progress-bar`、B 组 `\x20` 转义 + `Type=simple` 等、C 组 `==== 诊断信息：` 块、`journalctl -u $UNIT_NAME`、`$LASTEXITCODE = 0`）—— 均不命中正则。**A.1 secrets scan 不会因本任务退化**。

---

## 3. 给阶段 4 Developer 的执行 hint（防坑清单）

1. **分区例外（F-5）**：如 PM 仍派 dev-backend，Developer 在 04 顶部新增 `## Partition exception` 段，引用 02 §11 + 本 GR §F-5。建议 PM 改派通用 `developer` agent。
2. **B.2.1 heredoc 转义陷阱（R-2）**：写完 install-service.sh 后实机 sudo 跑一次默认路径安装后 `cat /etc/systemd/system/frp-easy.service | xxd | grep -E "(ExecStart|WorkingDirectory)" -A 1` 看字节，把 hex dump 作为附录贴进 04_DEVELOPMENT.md。同样跑一次 `sudo FRP_EASY_INSTALL_DIR="/opt/frp easy/v1" bash install.sh` 后再贴一次（确认 `\x20` 出现）。**硬要求**。
3. **B.3 systemd-analyze verify 风险（F-3）**：在 verify 调用块加 `2>&1` 捕获 stderr。若 NFR-1 任一目标发行版手测时 verify 误报 fatal，降级为"warn + 继续"。
4. **C.1 退出码诊断（F-1）**：在 `set +e; bash $SVC; rc=$?; set -e` 后**额外加一行** `[[ $rc -ne 0 ]] && echo "    [diag] install-service.sh rc=$rc" >&2`，便于 QA AC-7 / 用户复现时拿到证据。同样在 install-service.sh 的 daemon-reload / enable--now 两处 `set +e/-e` 块后加 diag 打印。
5. **PS 5.x 去 `-UseBasicParsing` 兜底**：保留现有 try/catch 不变；R-3 已经被现有错误分流自然兜住。
6. **insight 更新（F-4）**：04 阶段**不**直接编辑 `.harness/insight-index.md`（避免与 archive-task 冲突），而是在 04_DEVELOPMENT.md 的 "Insight to surface" 段写一行 + 在 07_DELIVERY.md 时由 PM 决定原地替换还是 archive 流程。
7. **AC-8 锚点字面化（F-2）**：实施时 install-service.sh L142-145 替换块中**严格使用** "`==== 诊断信息：systemctl status $UNIT_NAME --no-pager ====`" 与 "`==== 诊断信息：journalctl -u $UNIT_NAME --no-pager -n 20 ====`" 两条字面前缀，并在 04_DEVELOPMENT.md Open issues for review 中记这两条作为 QA 验证 AC-8 的 grep 锚点。
8. **verify_all 跑两次**：实施前一次取 baseline（应是 PASS:19），实施后一次确认无退化。

---

## 4. Verdict

# APPROVED FOR DEVELOPMENT

**理由**：
1. 01 + 02 已构成可执行设计；13 条 AC 全部映射到具体观察点。
2. Findings F-1（根因诊断缺口）、F-2（AC-8 验证锚点）、F-3（systemd-analyze 极端误报）均为 WARN，已通过执行 hint 中的具体补强动作转化为 Developer 阶段可吸收的实施约束，不构成阻断。
3. F-5（分区分配越权）属 PM 派发选择问题，PM 决策改派通用 `developer` agent。
4. F-6（verify_all secrets scan 不退化）已逐项核对正则 + 本任务新增字面量 = 不命中。
5. F-4（insight 更新）经核 `.harness/rules/05-insight-index.md` 实际允许"任务结束时编辑"，SA 策略合规。

**不返工 SA / RA**：F-1 / F-2 / F-3 都不是设计错误，而是诊断深度/验证锚点的补强项；F-1 修复路径（`set +e/-e` + systemd-analyze + 强诊断）能消除症状即使根因未完全闭环，可由 Developer 在 04 通过 diag 打印补上证据链。F-5 是分区分配的实施层问题，由 PM 派发裁决。

下游交接：PM 派发阶段 4 通用 `developer` agent，把本评审 §3 的 8 条执行 hint 作为 Developer 输入。
