# 06 — Test Report · T-016 install-progress-and-systemd-unit-fix

> 阶段 6 / 7（QA Tester）。模式：**full**。
> 上游只读输入：01 RA（13 AC）/ 02 SA（A-B-C 三组改动 + §D.4 A1-A6 测试提示）/ 03 GR（APPROVED FOR DEVELOPMENT + 8 hint）/ 04 Dev（含 heredoc hex dump 附录）/ 05 CR（APPROVED）。
> 对抗心态：不假设 04 的 hex dump 可信。独立写 reproducer 验证 `systemd_escape_path()` 的真实字节输出，不复用 04 的测试代码。

---

## 测试环境（必读）

| 维度 | 值 |
|---|---|
| 主机 OS | Windows 11 Home China 10.0.26200（开发机，**无 systemctl / 无 systemd-analyze**） |
| Bash | GNU bash 5.2.37(1)-release (x86_64-pc-msys) via Git Bash |
| PowerShell | PowerShell 7.6.0（亦兜底 Windows PowerShell 5.1 路径行为推演） |
| WSL | `wsl.exe` 存在但**无已安装的 Linux 发行版**（`wsl --list` 报"未安装 Linux 内核"），无法实机跑 Ubuntu/Debian/RHEL VM |
| Network | 在线（GitHub API / Releases CDN 可达） |
| Tree | `git status` clean，HEAD = 7cb7e00 + T-016 dev 改动（4 个 scripts/install*.{sh,ps1}） |

**可在本机跑**：verify_all、shellcheck 风格检查、bash 函数字节级独立 reproducer、PowerShell 进度行为模拟（结合 `[Console]::IsErrorRedirected` 动态探测）、静态字符串/正则锚点核对。

**不可在本机跑（必须由用户在真实 Linux VM 上验证）**：systemctl daemon-reload / enable --now、systemd-analyze verify 实机行为、journalctl 摘要采集、跨发行版（Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9）实测。这些用例标注为 `DEFERRED-TO-USER`，下方 `## Adversarial tests` 与 `## Risks remaining` 给出用户应跑的精确命令。

---

## 1. Test plan / AC 覆盖矩阵

| AC | 验收点 | 状态 | 证据 / 命令 |
|---|---|---|---|
| AC-1（curl 进度可见） | install.sh 步骤 5 在 TTY 下输出进度 | PASS（静态） | `scripts/install.sh:197-201`：`CURL_PROGRESS_FLAG="--progress-bar"` 在 `[[ -t 2 ]]` 为真时附加；保留 `-f`/`-S`/`-L`；中文报错 + exit 1 路径保留。AC-1 实机视觉验证 DEFERRED-TO-USER。 |
| AC-2（PS 进度可见） | install.ps1 步骤 5 PS 5.1+7.x 主机输出进度 | PASS（静态） | `scripts/install.ps1:156-172`：去 `-UseBasicParsing` + `$ProgressPreference` + `$isInteractive` 探测 + try/finally 恢复 prev。本机 `[Console]::IsErrorRedirected = True` 时降级 SilentlyContinue 路径已验。AC-2 实机视觉验证 DEFERRED-TO-USER。 |
| AC-3（进度不掩盖下载失败） | 下载失败仍报中文错 + exit 1 | PASS（静态） | `scripts/install.sh:201-204`（`-f` 保留 → 4xx/5xx 让 curl 非 0 → if 分支 → 中文错 + exit 1）；`scripts/install.ps1:166-169`（catch + Write-Error + exit 1）。 |
| AC-4（默认路径 active） | `/opt/frp-easy` 无空格，systemctl is-active = active | PASS（独立验证：unit 字节正确） + DEFERRED-TO-USER（实机 active）| 独立 bash reproducer 复现 install-service.sh 完整 heredoc 流程产出默认路径 unit，bytes 为 `ExecStart=/opt/frp-easy/frp-easy` / `WorkingDirectory=/opt/frp-easy`（hex 末段 `2d65 6173 79`），与 systemd.exec(5) "裸 token" 合法语法完全一致。实机 active 须用户跑。 |
| AC-5（含空格路径 active） | `FRP_EASY_INSTALL_DIR=/opt/frp easy/v1`，systemctl is-active = active | PASS（独立验证：函数 ASCII 转义正确）+ DEFERRED-TO-USER（实机 active） | D-1 已修复（v3 单引号字面赋值方案）。独立 reproducer 4 用例（无空格 / 单空格 / 多空格 / 前后空格）全部输出含字节序列 `5c 78 32 30`（字面 `\x20`），符合 systemd.exec(5) C-style 转义语法。见 §D-1 Re-evaluation。实机 `systemctl is-active = active` 须用户跑（Linux VM）。 |
| AC-6（跨发行版） | Ubuntu 22.04/24.04 / Debian 12 / RHEL 9 各通过 AC-4 | DEFERRED-TO-USER | 本机无 Linux VM；裸 token 默认路径在 NFR-1 列出的 systemd 249-255 均合法（systemd.exec(5) 稳定语法）。但 AC-5 已 FAIL，跨发行版同样会复现该 FAIL。 |
| AC-7（退出码透传 == 2） | install.sh 报"退出码 2"（不是 0） | PASS（静态） | `install.sh:280-289` set+e/set-e 三行块 + diag 打印 + exit "$rc"；`install-service.sh:166-202` daemon-reload / enable--now 两处同样三行块 + `exit 2`。链路：install-service.sh 自身 exit 2 → install.sh `$rc=2` → install.sh exit 2。 |
| AC-8（实际诊断片段） | enable--now 失败块包含 status + journalctl 实际输出 | PASS（锚点核对） | `grep -nF "==== 诊断信息：" scripts/install-service.sh` 命中 L193（status 锚点）+ L196（journalctl 锚点）；两处后紧跟 `systemctl status ... 2>&1 | sed 's/^/    /'` 与 `journalctl -u ... -n 20 2>&1 | sed 's/^/    /'`。实机片段实际可见性 DEFERRED-TO-USER。 |
| AC-9（PS 退出码透传） | install.ps1 报 install-service.ps1 实际 `$LASTEXITCODE` | PASS（静态） | `install.ps1:234-238`：`$LASTEXITCODE = 0` → `& $svc` → `if ($LASTEXITCODE -ne 0) { Write-Error ...; exit $LASTEXITCODE }`。链路完整。 |
| AC-10（macOS exit 0） | macOS 跳过服务化，exit 0 | PASS | `install.sh:258-268` 未被本任务触碰；分支保留打印 + `exit 0`。 |
| AC-11（verify_all PASS:19） | verify_all 不退化 | PASS | 本机本阶段实跑：PASS:19 / WARN:0 / FAIL:0 / SKIP:0（见 `## verify_all result`）。 |
| AC-12（升级幂等） | 重跑安装保留 frp_easy.toml / .frp_easy/ / frp_linux/ | PASS（静态） | `install.sh:228-253` 升级分支未被本任务触碰，白名单覆盖逻辑保留；NFR-8 保持。实机 mtime/sha256 不变验证 DEFERRED-TO-USER。 |
| AC-13（unit 路径 + 清理提示） | enable--now 失败块输出 unit 路径 + `systemctl disable` 命令 | PASS（静态） | `install-service.sh:199-200`：`unit 文件已写入：$UNIT_PATH` + `如需清理：sudo systemctl disable $UNIT_NAME && sudo rm -f $UNIT_PATH && sudo systemctl daemon-reload`。 |

**总结**：13 条 AC 中 **AC-5 FAIL**，AC-4 / AC-6 / AC-7 / AC-8 / AC-12 部分 DEFERRED-TO-USER（实机系统集成），其余 PASS。

---

## 2. Boundary tests added

由于 `## Hard rules` 第 3 条 "QA 不写生产代码"，本任务边界覆盖通过 `## Adversarial tests` 中的独立 reproducer 完成，未新增持久测试文件（项目无对应 bash/PS 脚本单元测试套件，且 verify_all 已覆盖现有 Go/Vue 测试）。如未来要把 `systemd_escape_path()` 加入 verify_all，建议新增 `scripts/tests/test_systemd_escape.sh` 与 `[A.4]` 步骤位。

**Edge cases probed in reproducers**：

- 无空格路径（AC-4）—— `systemd_escape_path "/opt/frp-easy"` 直通无修改 ✓
- 单空格路径（AC-5）—— `systemd_escape_path "/opt/frp easy/v1"` **应**输出 `/opt/frp\x20easy/v1` ✗
- 多空格路径 —— `systemd_escape_path "/opt/a b c/d"` **应**输出 `/opt/a\x20b\x20c/d` ✗
- 前后空格边界 —— `systemd_escape_path " /opt/a /"` **应**输出 `\x20/opt/a\x20/` ✗
- bash 反斜杠语义 —— `${p// /\\x20}`（2 反斜杠）vs `${p// /\\\\x20}`（4 反斜杠）字节差异已 hex dump 对比

---

## Adversarial tests

**契约**：每条 AC 一个独立 reproducer + 预测失败假设 + tool 输出证据。**不**复用 04 的测试代码——04 hex dump 附录 §"用例 (b)" 与 committed 源代码冲突（详见 D-1），单独验证从需求出发的真实字节行为。

| AC | Hypothesis ("I expect failure when…") | Reproducer | Outcome |
|---|---|---|---|
| AC-1 | curl `-fSL` 缺 `-s` 在管道形态 `\| sudo bash` 下进度污染 stdin | 本机离线模拟：检视 `[[ -t 2 ]]` 在 sudo bash 子 shell 下值；02 §A.2 论证 stderr 物理隔离于 stdin | 静态分析：`install.sh` 步骤 5 的 curl 是 install.sh 进程内的独立 child，stderr 继承 sudo bash 的 stderr（终端），不污染 stdin。**Survived** —— 但实机视觉验证 DEFERRED-TO-USER。 |
| AC-2 | PS 5.1 上去 `-UseBasicParsing` 触发 IE COM 加载错误 / 极简环境失败 | 在 PS 7.6 上跑 `$ProgressPreference` 控制；推演 PS 5.1 路径 | 见 A2 详情。PS 7.6 本机：`[Environment]::UserInteractive=True`、`[Console]::IsErrorRedirected=True` → `$isInteractive=False` → `SilentlyContinue`（降级路径正确触发）。**Survived**（PS 5.1 真机视觉验证 DEFERRED-TO-USER）。 |
| AC-3 | curl `-f` 在 GitHub CDN 404 时进度条已开始打印导致用户混淆 | 检视 `-f` 与 `--progress-bar` 在 4xx 时的交互；curl manpage：`-f` 让 header 阶段就 fail，progress 顶多打几字节 | 静态：`-f` 触发非 0 退出 → if 分支 → 中文错 + exit 1，与进度条正交。**Survived**。 |
| **AC-4** | `systemd_escape_path "/opt/frp-easy"` 在 bash 5.2 下意外修改无空格路径 | 见 A0 独立 reproducer | 默认路径无空格 → 无替换 → unit 文件字节正确（`2d65 6173 79` 末段）。**Survived**。 |
| **AC-5** | `systemd_escape_path "/opt/frp easy/v1"` 在 bash 5.x 下不产生 `\x20` 字面而产生 `x20`（无反斜杠） | 见 A1 独立 reproducer（NEW，QA 自写，不依赖 04 hex dump） | v1 初评：**FAILED** —— 实测输出 `/opt/frpx20easy/v1`（bytes `7270 7832 30`）→ D-1 BLOCKER。**v2 重评：Survived**（D-1 已修复，独立验证 4 用例 hex 含 `5c 78 32 30`，参考 §D-1 Re-evaluation）。 |
| AC-6 | systemd 249 vs 255 对裸 token 解析差异 | 静态：systemd.exec(5) 自 v220 起裸 token 语法稳定 | **Survived（默认路径）/ FAILED（含空格路径，沿 AC-5）**。DEFERRED-TO-USER 实机跨发行版。 |
| AC-7 | install.sh `if ! cmd; then $?` 反模式 + `set -e` 让 install-service.sh exit 2 被吞掉 | A3 reproducer：构造 fake install-service.sh 返回 2 | 见 A3 详情。新代码 `set +e; bash; rc=$?; set -e` + `[diag]` 打印 + `exit "$rc"` 透传 == 2。**Survived**。 |
| AC-8 | enable--now 失败但 status/journalctl 命令本身失败导致诊断为空 | 检视 `2>&1 \| sed ... \|\| true` 兜底 | 静态：`\|\| true` 让 status/journalctl 失败也不影响主流；锚点字面前缀 `==== 诊断信息：` 始终先打印，QA grep 至少匹配 2 行（即使子命令输出为空）。**Survived**。 |
| AC-9 | PS 5.1 `& $svc` 触发 terminating error 时 `$LASTEXITCODE` 保留陈旧值 | 静态 + 设计推演 | `install.ps1:234` 在 `& $svc` 前 `$LASTEXITCODE = 0` 重置。若 svc 因 terminating error 退出，PS 异常被外层 try/finally 捕获不到（无 catch 包裹 `& $svc` 本身）—— 这是设计 §C.3 第 391 行论述的 "如果 install-service.ps1 因 ErrorActionPreference=Stop 触发 terminating error 而未走到 exit N，$LASTEXITCODE 可能保留上一条命令的值"。`= 0` 重置正好兜住该边角。**Survived**。 |
| AC-10 | macOS 路径被本任务意外触碰 | `git diff` 检视 install.sh:258-268 | 未触碰，分支保留 `exit 0`。**Survived**。 |
| AC-11 | A.1 secrets scan 正则误中本任务新增字面量 | 检视 A.1 正则 + 本任务新增字面量 | A.1 正则 `(api[_-]?key\|secret\|password\|token)[[:space:]]*[:=][[:space:]]*["'][^"']{8,}["']` 不命中 `--progress-bar` / `\x20` / `==== 诊断信息：` / `$LASTEXITCODE = 0`。verify_all 实跑 PASS:19。**Survived**。 |
| AC-12 | 升级分支误覆盖 frp_easy.toml | `git diff scripts/install.sh` 升级分支（L228-253）未触碰 | 白名单覆盖保留：cp -a 仅复制 frp-easy / scripts / README.txt 等，不触 frp_easy.toml / .frp_easy/ / frp_linux/。**Survived**。 |
| AC-13 | enable--now 失败诊断块缺 unit 路径或 disable 命令 | grep 锚点 | `install-service.sh:199-200` 字面前缀 "unit 文件已写入：" + "如需清理：sudo systemctl disable" 均命中。**Survived**。 |

### A0 — 独立 reproducer（AC-4 默认路径，QA 自写）

**Hypothesis**：默认路径 `/opt/frp-easy`（无空格）通过 `systemd_escape_path()` 后字节不变；heredoc 写出的 unit 文件 ExecStart / WorkingDirectory 与 systemd.exec(5) "裸 token" 语法兼容。

**Reproducer**：

```bash
# Source verbatim copy of systemd_escape_path from scripts/install-service.sh:24-27
systemd_escape_path() {
    local p="$1"
    printf '%s' "${p// /\\x20}"
}
BINARY="/opt/frp-easy/frp-easy"
INSTALL_DIR="/opt/frp-easy"
ESC_BINARY="$(systemd_escape_path "$BINARY")"
ESC_INSTALL_DIR="$(systemd_escape_path "$INSTALL_DIR")"
cat <<EOF
ExecStart=${ESC_BINARY}
WorkingDirectory=${ESC_INSTALL_DIR}
EOF
```

**Actual output**：

```
ExecStart=/opt/frp-easy/frp-easy
WorkingDirectory=/opt/frp-easy
```

**Verdict**：**Survived**。默认路径无替换发生（pattern ` ` 不匹配），字节直通。

### A1 — 独立 reproducer（AC-5 含空格路径，QA 自写）

**Hypothesis**："I expect failure" —— bash 5.x 的参数扩展 `${p// /\\x20}` 在 `"..."` 内做额外一轮反斜杠脱壳（quote removal），把 `\\x20` 还原为 `x20` 后再做替换，**不**产生字面反斜杠。04 §"用例 (b)" 的 hex dump 与此预测矛盾，我**怀疑 04 hex dump 不可信**（要么 04 tmp 脚本用了不同的转义模式，要么 hex 字节被错抄），独立验证。

**Reproducer**（QA 自写，未复用 04 代码）：

```bash
cat > /tmp/qa_install_flow.sh << 'OUTER'
#!/usr/bin/env bash
set -euo pipefail
# Verbatim copy of systemd_escape_path from scripts/install-service.sh:24-27
systemd_escape_path() {
    local p="$1"
    printf '%s' "${p// /\\x20}"
}
BINARY="/opt/frp easy/v1/frp-easy"
INSTALL_DIR="/opt/frp easy/v1"
RUN_USER="frpuser"
ESC_BINARY="$(systemd_escape_path "$BINARY")"
ESC_INSTALL_DIR="$(systemd_escape_path "$INSTALL_DIR")"
# Same heredoc structure as install-service.sh:126-144
TMP_UNIT="/tmp/qa_real_unit.service"
cat > "$TMP_UNIT" <<EOF
[Service]
Type=simple
ExecStart=${ESC_BINARY}
WorkingDirectory=${ESC_INSTALL_DIR}
User=${RUN_USER}
EOF
echo "=== final unit file contents ==="
cat "$TMP_UNIT"
echo "=== hex dump ==="
grep -E "ExecStart|WorkingDirectory" "$TMP_UNIT" | while IFS= read -r line; do
    echo "$line"
    printf '%s\n' "${line#*:}" | xxd
done
OUTER
bash /tmp/qa_install_flow.sh
```

**Actual output**（实跑 in Git Bash 5.2.37 on Windows 11）：

```
=== final unit file contents ===
[Service]
Type=simple
ExecStart=/opt/frpx20easy/v1/frp-easy
WorkingDirectory=/opt/frpx20easy/v1
User=frpuser

=== hex dump ===
ExecStart=/opt/frpx20easy/v1/frp-easy
00000000: 4578 6563 5374 6172 743d 2f6f 7074 2f66  ExecStart=/opt/f
00000010: 7270 7832 3065 6173 792f 7631 2f66 7270  rpx20easy/v1/frp
00000020: 2d65 6173 790a                           -easy.
WorkingDirectory=/opt/frpx20easy/v1
00000000: 576f 726b 696e 6744 6972 6563 746f 7279  WorkingDirectory
00000010: 3d2f 6f70 742f 6672 7078 3230 6561 7379  =/opt/frpx20easy
00000020: 2f76 310a                                /v1.
```

**Verdict**：**FAILED**。在 offset 0x14-0x17 期望 `5c 78 32 30`（字面 4 字符 `\x20`），实测为 `78 32 30 65`（= `x20e`）—— **反斜杠字节 `5c` 完全缺失**。systemd 会按字面读 `WorkingDirectory=/opt/frpx20easy/v1`，该目录不存在 → enable--now 必然失败。

**对照实验**（确认正确 fix 是 4 反斜杠源 → 1 字面反斜杠输出）：

```bash
# Fixed candidate: 4 backslashes in source
bash -c 'p="/opt/frp easy"; r="${p// /\\\\x20}"; printf "[%s]\n" "$r"; printf "%s" "$r" | xxd | head -1'
# Output:
# [/opt/frp\x20easy]
# 00000000: 2f6f 7074 2f66 7270 5c78 3230 6561 7379  /opt/frp\x20easy
```

确认：源码须改 `${p// /\\x20}` → `${p// /\\\\x20}`（或换 sed/printf 实现）才能产出字面 `\x20`。详见 **Defect D-1**。

**04 hex dump 不可信结论**：04 附录 §"用例 (b)" 第 100-106 行声称 bytes 含 `5c 78 32 30`，但用 committed 源码 verbatim copy 复现得到完全不同的字节序列。**04 的 hex dump 与 committed 代码不一致**——这是 04 阶段未被 05 Code Review 捕获的事实层问题（05 直接相信了 04 的 hex dump，未独立复现）。

### A2 — PS 进度方案降级路径（AC-2）

**Hypothesis**：PS 7.6 在 Claude harness 下 `[Console]::IsErrorRedirected = True`，`$isInteractive = False`，因此走 `SilentlyContinue` 分支（不打印进度，避免日志膨胀）。在真实 Windows 终端（管理员 PowerShell 双击运行）`IsErrorRedirected = False`，走 `Continue` 分支（显示进度）。

**Reproducer**（QA 在本机 PS 7.6 跑）：

```powershell
$PSVersionTable.PSVersion          # 7.6.0
[Environment]::UserInteractive     # True
[Console]::IsErrorRedirected       # True（harness 重定向）
```

**Outcome**：本机环境下 `$isInteractive = True -and -not True = False` → `SilentlyContinue`——降级路径正确。**Survived**。

**Deferred to user**：PS 5.1（Windows PowerShell）的实机视觉验证、IWR 去 `-UseBasicParsing` 在 PS 5.1 上对二进制 zip 下载的副作用——理论上无（IE COM DOM parsing 跳过非 text/html），需用户在普通 Windows 11 + 管理员 PS 5.1 跑 `irm ... | iex` 直观确认 Write-Progress 进度条出现。

### A3 — 退出码透传（AC-7）

**Hypothesis**：旧代码 `if ! bash $SVC; then rc=$?` 在 bash 5.x `set -e` 上下文下 then 内 `$?` 跨版本不可靠；新代码 `set +e; bash; rc=$?; set -e` 三行块 + `exit "$rc"` 必能透传 install-service.sh 的真实 exit code。

**Reproducer**（QA 独立写 fake install-service.sh 返回 2）：

```bash
# Construct minimal install.sh-like wrapper
cat > /tmp/fake-svc.sh << 'EOF'
#!/usr/bin/env bash
echo "[fake] simulating exit 2" >&2
exit 2
EOF
chmod +x /tmp/fake-svc.sh

cat > /tmp/qa_rc_test.sh << 'EOF'
#!/usr/bin/env bash
set -euo pipefail
SERVICE_SCRIPT=/tmp/fake-svc.sh
set +e
bash "$SERVICE_SCRIPT"
rc=$?
set -e
[[ $rc -ne 0 ]] && echo "    [diag] install-service.sh rc=$rc" >&2
if [[ $rc -ne 0 ]]; then
    echo "错误：服务注册失败（install-service.sh 退出码 ${rc}）。" >&2
    exit "$rc"
fi
EOF
bash /tmp/qa_rc_test.sh; echo "wrapper exited: $?"
```

**Actual output**：

```
[fake] simulating exit 2
    [diag] install-service.sh rc=2
错误：服务注册失败（install-service.sh 退出码 2）。
wrapper exited: 2
```

**Verdict**：**Survived**。透传链路返回 2，与 install-service.sh exit 2 一致，与线上 "报告 0" 旧 bug 不一致——修复有效。

### A4 — 进度日志重定向降级（FR-A.6 / BC-A.5）

**Hypothesis**：`bash install.sh > install.log 2>&1` 重定向时 `[[ -t 2 ]]` 为 false → `CURL_PROGRESS_FLAG=""` 空 → curl 退到无 `--progress-bar` 状态 → 不产生 `\r` 覆盖序列污染日志。

**Reproducer**：

```bash
bash -c '[[ -t 2 ]] && echo "tty" || echo "not tty"' 2>/dev/null     # stderr 关闭
bash -c '[[ -t 2 ]] && echo "tty" || echo "not tty"' 2>/tmp/x.log    # stderr 重定向
bash -c '[[ -t 2 ]] && echo "tty" || echo "not tty"'                 # 直接终端
```

**Actual output** in this harness:

```
not tty
not tty
not tty   # harness 也算非 tty
```

**Verdict**：**Survived**——`[[ -t 2 ]]` 在重定向时正确返回 false，CURL_PROGRESS_FLAG 保持为空字符串，curl 不附加 `--progress-bar`，日志无 `\r` 污染。实机交互式终端验证 DEFERRED-TO-USER（用户在真实 SSH 终端跑 `curl|sudo bash` 应看到 `#` 进度条）。

### A5 — enable--now 失败诊断（AC-8 / AC-13）

**Hypothesis**：QA 拿 `grep -F "==== 诊断信息：" install-service.sh` 应至少匹配 2 行（status + journalctl 锚点），且字面前缀严格 == 04 hint 7 给的字符串。

**Reproducer**：

```bash
grep -nF "==== 诊断信息：" scripts/install-service.sh
grep -nF "unit 文件已写入：" scripts/install-service.sh
grep -nF "如需清理：sudo systemctl disable" scripts/install-service.sh
```

**Actual output**：

```
182:#   "==== 诊断信息：systemctl status $UNIT_NAME --no-pager ===="
183:#   "==== 诊断信息：journalctl -u $UNIT_NAME --no-pager -n 20 ===="
193:    echo "==== 诊断信息：systemctl status $UNIT_NAME --no-pager ====" >&2
196:    echo "==== 诊断信息：journalctl -u $UNIT_NAME --no-pager -n 20 ====" >&2
199:    echo "unit 文件已写入：$UNIT_PATH（可 cat 审阅）。" >&2
200:    echo "如需清理：sudo systemctl disable $UNIT_NAME && sudo rm -f $UNIT_PATH && sudo systemctl daemon-reload" >&2
```

**Verdict**：**Survived**——字面前缀字符串严格命中（L193 / L196），unit 路径 + 清理提示均在（L199 / L200），AC-8 / AC-13 grep 锚点合格。实机诊断片段实际输出 DEFERRED-TO-USER。

### A6 — systemd-analyze verify 降级路径（02 §B.3 + GR §F-3）

**Hypothesis**：`command -v systemd-analyze` 在主机不可用时跳过自检不阻塞主流；verify 失败时不 `rm unit` 不 `exit 2`，仅打 warn 并继续 daemon-reload。

**Reproducer**：

```bash
sed -n '157,164p' scripts/install-service.sh
```

**Actual output**：

```bash
if command -v systemd-analyze >/dev/null 2>&1; then
    verify_out="$(systemd-analyze verify "$UNIT_PATH" 2>&1)" && verify_rc=0 || verify_rc=$?
    if [[ "$verify_rc" -ne 0 ]]; then
        echo "警告：systemd-analyze verify 报告问题（退出码 $verify_rc）：" >&2
        printf '%s\n' "$verify_out" | sed 's/^/    /' >&2
        echo "    继续 daemon-reload 让 systemd 自己判定（若 reload/enable 失败将由下方诊断块详述）。" >&2
    fi
fi
```

**Verdict**：**Survived**——`command -v` 探测保护、`verify_rc=0 || verify_rc=$?` 抑制 errexit、warn 打印后继续主流；不阻塞 daemon-reload。GR §F-3 降级语义落实。实机各发行版 verify 实际行为 DEFERRED-TO-USER。

---

## 用户应在真实 Linux VM 上跑的精确命令（DEFERRED-TO-USER）

修复 D-1 后（见下方 Defects 段），用户应在 Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 至少 2 个发行版各跑以下用例。**注意**：先在测试机跑，不要直接在生产机器。

```bash
# === AC-4：默认路径 + AC-1 进度可见 ===
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash
# 期望：步骤 5 stderr 出现 `#` 进度条或百分比
systemctl is-active frp-easy   # 期望 active
systemctl status frp-easy --no-pager | grep -i "bad unit file"  # 期望无匹配

# === AC-5：含空格自定义路径（D-1 修复后才会通过）===
sudo FRP_EASY_INSTALL_DIR="/opt/frp easy/v1" bash <(curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh)
cat /etc/systemd/system/frp-easy.service | grep -E "ExecStart=|WorkingDirectory="
# 期望：ExecStart=/opt/frp\x20easy/v1/frp-easy（4 字符 \x20 字面）
systemd-analyze verify /etc/systemd/system/frp-easy.service  # 期望 exit 0
systemctl is-active frp-easy  # 期望 active

# === AC-7 + AC-8 + AC-13：退出码透传 + 诊断块 ===
sudo chmod 000 /opt/frp-easy/frp-easy
sudo bash /opt/frp-easy/scripts/install-service.sh 2>&1 | tee /tmp/svc.log
echo "rc=$?"     # 期望 2
grep -F "==== 诊断信息：" /tmp/svc.log | wc -l   # 期望 >= 2
grep -F "unit 文件已写入：" /tmp/svc.log   # 期望 1 行命中
grep -F "如需清理：sudo systemctl disable" /tmp/svc.log   # 期望 1 行命中
grep -F "[diag] systemctl enable --now frp-easy rc=" /tmp/svc.log   # 期望 1 行命中

# === AC-12：升级幂等 ===
sudo stat /opt/frp-easy/frp_easy.toml > /tmp/before.txt   # 已安装后跑
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash
sudo stat /opt/frp-easy/frp_easy.toml > /tmp/after.txt
diff /tmp/before.txt /tmp/after.txt   # 期望 empty（mtime 不变）
sudo sha256sum /opt/frp-easy/frp_easy.toml   # 与 before 一致
ls /opt/frp-easy/frp_linux/   # frpc/frps 仍在
```

```powershell
# === AC-2：PS 5.1 + 7.x 各跑一遍 ===
# Windows 11 管理员 PowerShell:
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
# 期望：步骤 5 主机出现 PS Write-Progress 进度条或百分比

# === AC-9：PS 退出码透传 ===
Remove-Item "C:\Program Files\frp-easy\frp-easy.exe"
& "C:\Program Files\frp-easy\scripts\install-service.ps1"
$LASTEXITCODE   # 期望 1（缺二进制）
```

---

## verify_all result

**实施前 baseline**（从 04 引用，避免重跑）：PASS:19 / WARN:0 / FAIL:0 / SKIP:0。

**本阶段实跑**（PowerShell 7.6 / Windows 11 / `scripts\verify_all.ps1` 2026-05-23）：

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

- Total tests: 231 → 231（QA 未新增持久测试文件；reproducer 是临时验证脚本，按 02 §10 OOS-6 "不修改步骤总数" 精神不引入 verify_all 新步骤位）
- New tests added: 0（持久），4 个独立 reproducer 作为对抗用例已落入本报告
- Baseline updated: **no**（test_count 不变 → baseline.json 保持 231）

---

## Regression checks

| 检查项 | 实施前（04 引用） | 实施后（本阶段实跑） | Delta |
|---|---|---|---|
| verify_all | PASS:19 / WARN:0 / FAIL:0 / SKIP:0 | PASS:19 / WARN:0 / FAIL:0 / SKIP:0 | 0（无退化）|
| A.1 secrets scan 是否命中新增字面量 | 不命中（04 已分析） | 不命中（A.1 PASS 实证） | 0 |
| 测试总数（baseline） | 231 | 231 | 0 |

二次 verify_all 不需重跑（项目无随机性测试，且 04 在两份输出中已实证；本阶段第三跑也为 PASS:19）。**未发现回归**。

---

## Defects found

### D-1 — BLOCKER — AC-5 含空格路径 systemctl 启动必然失败

**已修复**：见本文末 §D-1 Re-evaluation（2026-05-23）。Developer v3 方案（单引号字面赋值 `local esc='\x20'`）已 commit；QA 4 用例独立 reproducer 全部输出含字节 `5c 78 32 30`（字面 `\x20`），AC-5 重评 PASS。下方原 v1 段落作为历史证据保留。

**严重度**：BLOCKER（线上 bug 修复目标之一直接 FAIL，task 不可发布）。

**文件 / 位置**：`scripts/install-service.sh:24-27`

```bash
systemd_escape_path() {
    local p="$1"
    printf '%s' "${p// /\\x20}"
}
```

**Symptom**：bash 5.x 参数扩展 `${p// /\\x20}` 在 `"..."` 内做的反斜杠脱壳让 `\\x20` 退化为 `x20`（无反斜杠）。`systemd_escape_path "/opt/frp easy/v1"` 实际返回 `/opt/frpx20easy/v1` 而非期望的 `/opt/frp\x20easy/v1`。systemd 会按字面读 `WorkingDirectory=/opt/frpx20easy/v1` —— 该目录不存在 —— enable --now 必然失败。AC-5 直接 FAIL。

**Reproducer**（见 `## Adversarial tests` A1，QA 独立编写未复用 04）：

```bash
bash -c 'systemd_escape_path() { local p="$1"; printf "%s" "${p// /\\x20}"; }; r=$(systemd_escape_path "/opt/frp easy/v1"); echo "[$r]"; printf "%s" "$r" | xxd'
```

Output:
```
[/opt/frpx20easy/v1]
00000000: 2f6f 7074 2f66 7270 7832 3065 6173 792f  /opt/frpx20easy/
00000010: 7631                                     v1
```

期望 hex 应含 `5c 78 32 30`（4 字符 `\x20` 字面），实测为 `78 32 30 65`（= `x20e`）—— **反斜杠字节 `5c` 完全缺失**。

**和 04 hex dump 的矛盾**：04_DEVELOPMENT.md 附录 §"用例 (b)" 第 100-106 行声称同一函数源码下 bytes 含 `5c 78 32 30`。本 QA 独立用 verbatim copy 复现得到完全不同结果。**04 hex dump 不可信** —— 推测 04 阶段 tmp 脚本使用了 `\\\\x20`（4 反斜杠）而非 committed 的 `\\x20`（2 反斜杠），或字节被错抄。05 Code Review 直接信任 04 hex dump 未独立复现，是该 BLOCKER 漏网的根因。

**修复方向**（不由 QA 实施，仅给 Dev 提示 — 见 `.harness/agents/qa-tester.md` Hard rule 1）：源码 `${p// /\\x20}` 改为 `${p// /\\\\x20}`（4 反斜杠在源码 → bash 内 quote removal 一轮后 → pattern-replacement quote removal 再一轮 → 输出 1 个反斜杠 + `x20` = 4 字符字面 `\x20`）。**重新跑** A1 reproducer 确认字节为 `5c 78 32 30`。或换用 `sed` 实现避开 bash 参数扩展双重 quote removal 的坑：`printf '%s' "$p" | sed 's/ /\\x20/g'`（sed 内单层 quote removal）。

**额外要求**：修复后 Dev 必须**重新跑 hex dump 验证**（用 QA A1 reproducer 同款独立流程，**不**复用 04 的 tmp 脚本），把新 hex dump 贴入 04_DEVELOPMENT.md 附录覆盖旧的；CR 阶段须独立复现而非仅信任 hex dump 字面。

**Routing**：本 defect 阻断 T-016 交付；PM 路由回阶段 4 Developer 修复 + 阶段 5 重审 + 阶段 6 QA 重测 AC-5 reproducer A1。

### D-2 — MINOR — 04 hex dump 附录与 committed 源码不一致

**严重度**：MINOR（文档与代码不一致；本身不影响运行，但误导审计与未来排障）。

**文件 / 位置**：`docs/features/install-progress-and-systemd-unit-fix/04_DEVELOPMENT.md` §"Heredoc 字节验证 / 用例 (b)" 第 92-114 行。

**Symptom**：04 附录第 100-106 行 hex 字节序列 `7270 5c78 3230 6561 7379`（含 `5c`）与 committed `install-service.sh:24-27` 的源码 verbatim copy 在 bash 5.x 上的真实输出 `7270 7832 3065 6173 79`（无 `5c`）不一致。可能原因：04 tmp 脚本用了不同转义模式（如 `\\\\x20`）但被错记为 `\\x20`，或字节抄错。

**Routing**：D-1 修复时一并更新 04 附录 hex dump 为真实输出；本 defect 自动消解。

---

## Stability

- verify_all 本机本阶段跑 1 次；04 阶段已跑 2 次（前/后）。3 次均 PASS:19 完全一致。无随机失败。
- 独立 reproducer A0-A6 每个跑 1 次后稳定输出，未观察到非确定性。

**评估**：无 flakiness 风险。但稳定 reproduce 的 BLOCKER D-1 本身就是结构性失败，不是 flake。

---

## Risks remaining

修复 D-1 + 重验后，以下用例 **仍需用户在真实 Linux / Windows VM 上自测**（本 QA 阶段在 Windows 开发机无法覆盖）：

1. **AC-4 实机 active**：Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 各跑一次默认路径一键安装，确认 `systemctl is-active frp-easy` 返回 `active`，且 `systemctl status frp-easy --no-pager` 不含 `bad unit file setting`。
2. **AC-5 实机 active（D-1 修复后）**：上述任一发行版跑 `sudo FRP_EASY_INSTALL_DIR="/opt/frp easy/v1" bash <(curl ... install.sh)`，确认 unit 文件 `cat | grep ExecStart` 显示字面 `\x20`，且 `systemctl is-active` 返回 `active`。
3. **AC-1 实机视觉**：交互式 SSH 终端跑 `curl ... | sudo bash`，确认步骤 5 stderr 显示 `#` 进度条。
4. **AC-2 实机视觉**：Windows 11 管理员 PowerShell 5.1 与 7.x 各跑 `irm ... | iex`，确认步骤 5 出现 PS Write-Progress 进度条 / 百分比。
5. **AC-6 跨发行版**：上面 4 条在 4 个发行版各跑（至少 2 个：Ubuntu 22.04 + 1 个非 Ubuntu）。
6. **AC-7 实机退出码 == 2**：`sudo chmod 000 /opt/frp-easy/frp-easy && sudo bash /opt/frp-easy/scripts/install-service.sh; echo $?` 应输出 2，且 stderr 含 `[diag] systemctl enable --now frp-easy rc=2`。
7. **AC-8 实机诊断片段**：上一步 stderr 应包含 `==== 诊断信息：systemctl status ... ====` 后跟实际 systemctl status 输出（不仅是字面前缀）；journalctl 同理。
8. **AC-12 升级幂等**：跑两次一键安装，第二次走升级分支后 `frp_easy.toml` / `.frp_easy/` / `frp_linux/` 的 mtime + sha256 应不变。
9. **systemd-analyze verify 误报概率**：每发行版记录 verify 在合法 unit 上是否输出 warning（非 fatal），归档到下次 retro insight；GR §F-3 已允许 warn+继续，不阻断本任务。
10. **D-2 文档纠正**：D-1 修复时同步把 04 附录 hex dump 改为真实字节（来自 QA A1 reproducer 同款流程），避免误导未来审计。

**Risks the user can SKIP (OOS 已明确)**：

- Alpine / busybox curl（NFR-1 未列）
- Windows Server Core PS 5.1 IE COM 缺失（R-3 已论证由 try/catch 兜底）
- 含 shell 元字符（`$`、反引号、`"`、`\`、控制字符）的安装路径（OOS-2）
- systemd < 230 / WSL1 无 systemd（BC-B.4 OOS）
- macOS launchd 等价服务化（NFR-2 / OOS-8）

---

## Verdict

# APPROVED FOR DELIVERY（D-1 修复后 v2 重评，2026-05-23）

**理由**（v2 重评）：

1. **D-1 已修复**：Developer v3 方案（`scripts/install-service.sh:30-34` 单引号字面赋值 `local esc='\x20'` 绕开 bash 双引号 quote-removal 陷阱）经 QA 4 用例独立 reproducer 验证，全部输出含字节序列 `5c 78 32 30`（字面 `\x20`），与 systemd.exec(5) C-style 转义语法一致。详见本文末 §D-1 Re-evaluation。
2. **D-2 已消解**：04_DEVELOPMENT.md 附录 v2 hex dump 与 QA 独立 reproducer 一致；Developer 新增的 "D-1 fix 复盘" 段对真因（bash 双引号 + parameter expansion 内嵌反斜杠的双重 quote-removal）有准确描述。
3. **AC 覆盖**：13 条 AC 中 AC-1/AC-2/AC-3/AC-5/AC-7/AC-8/AC-9/AC-10/AC-11/AC-13 在 QA 静态 + 独立 reproducer 范围内 Survived；AC-4/AC-6/AC-12 默认路径分支 Survived；AC-5 含空格路径分支字节验证通过（实机 active 由用户在 Linux VM 自测，归入 DEFERRED-TO-USER）。
4. **verify_all 不退化**：PASS:19（v1 baseline）→ PASS:19（v2 D-1 修复后实跑），WARN/FAIL/SKIP 全 0。A.1 secrets scan 未误中新增 `local esc='\x20'` 字面量。
5. **历史 v1 verdict**：CHANGES REQUIRED（1 BLOCKER + 1 MINOR）。v2 路由：Developer 修复 → CR 独立复现 → QA 重跑 4 用例 reproducer → 当前 APPROVED FOR DELIVERY。

**遗留 DEFERRED-TO-USER**（见 `## Risks remaining`）：实机跨发行版 active 验证、AC-1/AC-2 视觉进度条、AC-7 实机退出码 == 2、AC-8 实机诊断片段、AC-12 升级 mtime/sha256 不变。这些由用户在真实 Linux / Windows VM 上自测，不阻断本任务交付。

**Routing**：本阶段 6 完成 → PM 派发阶段 7（Closer）。

---

## D-1 Re-evaluation (2026-05-23)

### 重评范围

聚焦 D-1 修复后 AC-5 的字节级独立验证。不重做 AC-1/AC-2/AC-3/AC-7/AC-8/AC-9/AC-10/AC-11/AC-12/AC-13 的 reproducer——v1 这些已 Survived，本次代码 diff 仅 4 行（`scripts/install-service.sh:30-34` 函数体）+ 注释 11 行（L19-29），影响面严格限于 `systemd_escape_path()` 函数；其它 AC 链路未变，无回归风险。

### 静态核对 install-service.sh:19-34

| 关注点 | 核对结果 |
|---|---|
| L30-34 函数体 | `local p="$1"; local esc='\x20'; printf '%s' "${p// /$esc}"`——三行实现，单引号字面赋值 `\x20` 到 `esc` 变量；参数扩展引用 `$esc` 不再触发双引号 quote-removal。✅ |
| L19-29 注释 | 准确描述真因（bash 5.x 双引号 + parameter expansion quote-removal 陷阱）与 v3 修复原理（单引号字面赋值绕开）。引用 02 §B.2.1 + T-016 D-1 fix 链路完整。✅ |
| 函数签名稳定性 | 函数名 `systemd_escape_path` / 入参 `"$1"` / 输出 stdout 单行—— v1/v3 一致，调用方 `ESC_BINARY="$(systemd_escape_path "$BINARY")"` 与 `ESC_INSTALL_DIR="$(systemd_escape_path "$INSTALL_DIR")"` 无需调整。✅ |

### 独立 reproducer（QA 自写，4 用例）

**Reproducer 脚本**（`/tmp/qa_d1_reeval.sh`，verbatim copy `install-service.sh:30-34`，已用完即删）：

```bash
systemd_escape_path() {
    local p="$1"
    local esc='\x20'
    printf '%s' "${p// /$esc}"
}
```

**Case 1: 单空格路径 `/opt/frp easy/v1`（AC-5 primary）**

实测输出（Git Bash 5.2.37 on Windows 11）：
```
[/opt/frp\x20easy/v1]
00000000: 2f6f 7074 2f66 7270 5c78 3230 6561 7379  /opt/frp\x20easy
00000010: 2f76 31                                  /v1
```

**断言**：offset 0x08-0x0b 为 `5c 78 32 30`（字面 4 字符 `\x20`），反斜杠字节 `5c` 就位。ASCII 视图列 `/opt/frp\x20easy` 直观可见反斜杠。**PASS**。

**Case 2: 无空格路径 `/opt/frp-easy`（AC-4 回归）**

```
[/opt/frp-easy]
00000000: 2f6f 7074 2f66 7270 2d65 6173 79         /opt/frp-easy
```

**断言**：pattern ` ` 不匹配，bytes 与输入完全一致（`2f6f 7074 2f66 7270 2d65 6173 79`），无任何额外字节注入。AC-4 默认路径分支 v3 修复未引入回归。**PASS**。

**Case 3: 多空格路径 `/opt/a b c/d`**

```
[/opt/a\x20b\x20c/d]
00000000: 2f6f 7074 2f61 5c78 3230 625c 7832 3063  /opt/a\x20b\x20c
00000010: 2f64                                     /d
```

**断言**：两处空格均被替换为字面 `\x20`，offset 0x06-0x09 与 0x0b-0x0e 各含一次 `5c 78 32 30`。bash 参数扩展 `${p// /...}` 的全局替换（双斜杠形态）工作正常。**PASS**。

**Case 4: 前后空格路径 ` /tmp/a /`**

```
[\x20/tmp/a\x20/]
00000000: 5c78 3230 2f74 6d70 2f61 5c78 3230 2f    \x20/tmp/a\x20/
```

**断言**：行首与中段两处空格均转义为 `\x20`，offset 0x00-0x03 为 `5c 78 32 30`（行首位置不丢字节），offset 0x0a-0x0d 为 `5c 78 32 30`（中段位置不丢字节）。无边界字符丢失。**PASS**。

### 04 v2 hex dump 可信性核对

04_DEVELOPMENT.md 已新增 "**04 v2 修复**" 标记段（L62-64），声明原 v1 附录 hex dump 不可信，重新基于 v3 committed 源码生成。逐字段对比本 QA 独立 reproducer：

| 字段 | 04 v2 附录（用例 b ExecStart） | QA 独立 reproducer（Case 1） | 一致性 |
|---|---|---|---|
| offset 0x10-0x17 | `7270 5c78 3230 6561` | offset 0x08-0x0f：`7270 5c78 3230 6561`（`/opt/frp` 前缀位置略不同因 04 附录前缀含 `ExecStart=`，但反斜杠序列字节完全一致）| ✅ |
| 反斜杠字节 `5c` | 在 offset 0x14 位置出现 | 在 offset 0x08 位置出现（位置差因 04 含 `ExecStart=` 12 字节前缀） | ✅（字节序列一致）|
| ASCII 视图含 `\x20` 字面 | `rp\x20easy/v1/fr` | `/opt/frp\x20easy` | ✅ |

**结论**：04 v2 hex dump 与 QA 独立 reproducer 字节序列一致；Developer 在 04 中已声明 v2 用 committed 源码 verbatim copy 而非 v1 的分叉 tmp 脚本，可信度由 QA 独立验证背书。**D-2 自动消解**。

### AC-5 重评结论

**PASS（独立验证：函数 ASCII 转义正确 + heredoc 字节符合 systemd.exec(5) `\x20` 语法）**

- 字节级验证：4 个用例覆盖无空格 / 单空格 / 多空格 / 前后空格边界，全部输出含字节序列 `5c 78 32 30`（字面 `\x20`）或保持输入字节不变（无空格用例）。
- systemd 语义：unit 文件 `WorkingDirectory=/opt/frp\x20easy/v1` 与 `ExecStart=/opt/frp\x20easy/v1/frp-easy` 字节合法，符合 systemd.exec(5) C-style 转义规范。
- v1 vs v3 字节对比：v1 实测输出 `7270 7832 30`（`rpx20`，无 `5c`）→ v3 实测输出 `7270 5c78 3230 65`（`rp\x20e`，含 `5c`）—— 字节差异严格定位在反斜杠是否注入，与 04 v2 附录 D-1 fix 复盘段的真因描述精准吻合。

**DEFERRED-TO-USER**：`systemctl is-active frp-easy` 在真实 Linux VM（Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 至少 2 个）上返回 `active` 的实机验证，命令清单见上文 `## 用户应在真实 Linux VM 上跑的精确命令`。本 QA 阶段在 Windows 开发机无法实机覆盖。

### verify_all 重跑（v2 D-1 修复后）

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

=== Summary ===  PASS: 19  WARN: 0  FAIL: 0  SKIP: 0
```

**Delta**：v1 baseline PASS:19 → v2 D-1 修复后 PASS:19。无退化。A.1 secrets scan 未误中新增 `local esc='\x20'` 字面量（正则 `(api[_-]?key|secret|password|token)...` 关键字词表不含 `esc`）。

### v2 重评最终结论

| 检查项 | 结论 |
|---|---|
| D-1 BLOCKER 修复 | ✅ Survived（4 用例字节验证 PASS）|
| D-2 MINOR 消解 | ✅ 04 v2 hex dump 与 QA 独立 reproducer 一致 |
| AC-5 字节级 | ✅ PASS（独立验证）|
| AC-5 实机 active | DEFERRED-TO-USER（Linux VM）|
| AC-4 回归 | ✅ Survived（无空格输入直通）|
| verify_all | ✅ PASS:19 / WARN:0 / FAIL:0 / SKIP:0 |
| 其它 12 条 AC | ✅ v1 已 Survived，本次 diff 仅触及 `systemd_escape_path()` 函数体，无回归风险 |

**Verdict**：**APPROVED FOR DELIVERY**。
