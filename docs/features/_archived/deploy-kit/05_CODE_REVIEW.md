# 05 — Code Review：T-008 deploy-kit

> Stage 5 of 7-stage `/harness` 流水线
> 上游：01（24 AC）+ 02（设计）+ 03（4 项 MINOR）+ 04（Developer 报告）
> 角色：Code Reviewer（独立视角，永不盲信 Developer 自我报告）

---

## 1. 审阅摘要

整体质量**高**。Developer 严格按 02_SOLUTION_DESIGN 的 §3–§8 与 §11 五批顺序落地；10 个改动文件均存在、可读、结构清晰；Stage 3 的 4 条 MINOR 均有具体落地证据。

- **设计契合度**：12/12 设计要点均忠实实现（双重 chmod 原子写、Windows .cmd 包装锁 cwd、sc.exe 等号语法、`flag.ErrHelp` 显式分流、ContinueOnError + SetOutput(io.Discard)）。
- **代码质量**：bash 脚本一致 `set -euo pipefail`，引号到位；PowerShell 一致 `$ErrorActionPreference = "Stop"` + WindowsPrincipal 管理员检测；Go flag 解析符合标准库习语。
- **跨平台兼容**：未使用 GNU 扩展（无 `realpath -m` / `stat -c` / `sed -i`）；`realpath` 有 fallback。
- **AC 覆盖**：抽样核对 AC-3 / AC-10 / AC-13 / AC-14 实现，证据充分。

发现 **1 项 MAJOR**（systemd unit 中 `ExecStart=${BINARY}` 路径含空格场景下被 systemd 解析为多参数） + 5 MINOR。无 CRITICAL。**Verdict：CHANGES REQUIRED**（MAJOR 项需修复，体量小）。

---

## 2. 逐文件审查（摘要）

### 2.1 `cmd/frp-easy/main.go`

flag 解析块 L99–L114 设计契合；`flag.NewFlagSet("frp-easy", flag.ContinueOnError)` + `SetOutput(io.Discard)` + `errors.Is(err, flag.ErrHelp)` 分流正确。

注：由于 `-h` / `--help` 已用 `BoolVar` 注册，`flag.ErrHelp` 分支实际为冗余防御代码（不会被命中），但无害。AC-11/12/13/14 均通过 `showHelp` / `showVersion` 分支真实生效。

### 2.2–2.10 各脚本与文档

- `package.sh` / `package.ps1`：staging 组装与 tar/zip 打包正确；sanity check 真实存在但在 Linux 主机降级宽松（见 MINOR-R5）。
- `install-service.sh`：双重 chmod 原子写真实落地；`SUDO_USER` fallback `id -un` 优先级合理（忠实兑现 Open Question 4 意图）。**MAJOR-1**：unit 文件中 `ExecStart=${BINARY}` 与 `WorkingDirectory=${INSTALL_DIR}` 未引号包裹，含空格目录下解析失败。
- `uninstall-service.sh`：友好降级 + `reset-failed` 状态机清理 + 数据目录保留提示完整。
- `install-service.ps1`：`WindowsPrincipal.IsInRole(Administrator)` 管理员检测正确；wrapper.cmd 锁 cwd 真实落地；sc.exe 等号语法正确。**MINOR-R2**：wrapper.cmd 用 `-Encoding ASCII`，中文路径会乱码。
- `uninstall-service.ps1`：wrapper.cmd 真实删除，路径正确。
- `docs/DEPLOYMENT.md`：3 路径 + 决策表 + F.1–F.5 故障排查齐全；抽样 3 条命令对脚本参数 100% 匹配。
- `README.md`：重排正确，端口表/配置说明/目录结构未误删；NIT-1：`<VERSION>` 占位符未在 README 顶部说明。
- `docs/dev-map.md`：索引追加正确。

---

## 3. MINOR 落地核对（4 条逐一确认）

| MINOR | 04 报告位置 | 实测核对 | 真伪 |
|---|---|---|---|
| MINOR-1 `flag.ErrHelp` 显式分流 | main.go L99–L106 | `if errors.Is(err, flag.ErrHelp) { ... return nil } ... os.Exit(2)` 真实存在 | 真 |
| MINOR-3 Windows .cmd binPath stop 风险注释 | install-service.ps1 L13–L17 | 头注释字面存在 | 真 |
| MINOR-4 避免 8+ 字符引号密码字面量 | toml.example / usageText / README.txt | 三处均仅写 4 字段无 password/token/secret | 真（A.1 PASS 是直接证据） |
| MINOR-5 package sanity check | package.sh L100–L106；package.ps1 L67–L78 | sh + ps1 真实调用 `--version` | 真 |

4 条 MINOR 全部真实落地，无虚假。

---

## 4. 安全审查

| 风险点 | 状态 |
|---|---|
| `--user "; rm -rf /"` 注入 | 经 `getent passwd` 校验 + 用户名含分号会被拒；接受 |
| sc.exe 等号语法 | 一致使用 `xxx= "yyy"` 等号后空格 + 引号包路径，正确 |
| 临时文件异常清理 | `install-service.sh` 未 `trap rm TMP_UNIT`（MINOR-R1） |
| 双重 chmod 真实性 | L127 + L129 真实双重 chmod |
| 内联 toml.example 敏感串 | 三处均无 password/token，A.1 PASS 直接证据 |
| Windows wrapper.cmd 引号包路径 | 正确加引号 |

---

## 5. 问题分级

### CRITICAL

无。

### MAJOR

**MAJOR-1 · systemd unit `ExecStart` / `WorkingDirectory` 路径未引号包裹**

- 文件：`scripts/install-service.sh` L115–L116
- 现状：
  ```ini
  ExecStart=${BINARY}
  WorkingDirectory=${INSTALL_DIR}
  ```
- 风险：若 INSTALL_DIR 含空格（如 `/opt/frp easy/`），systemd 把空格当作参数分隔符，启动失败。
- 修复方案 A（推荐）：systemd 5.0+ 支持双引号包路径
  ```ini
  ExecStart="${BINARY}"
  WorkingDirectory="${INSTALL_DIR}"
  ```
- 修复方案 B：脚本入口检测 INSTALL_DIR 含空格时 fail-fast，给中文提示。

### MINOR

- **MINOR-R1** · `install-service.sh` L94 `TMP_UNIT` 异常退出残留：建议 `trap 'rm -f "$TMP_UNIT"' EXIT`。
- **MINOR-R2** · `install-service.ps1` L60 wrapper.cmd 用 `-Encoding ASCII`：InstallDir 含中文/非 ASCII 字符时乱码 → cmd.exe 找不到目录。建议 `-Encoding Default`。
- **MINOR-R3** · `install-service.ps1` / `uninstall-service.ps1` 中 `Start-Sleep -Seconds 1` 后立即 config / delete：服务实际 stop 时间 > 1 秒会失败。建议轮询 `sc.exe query` 直到 `STOPPED` 或超时（5–10 秒）。
- **MINOR-R4** · `install-service.sh` L155 `sudo $SCRIPT_DIR/uninstall-service.sh` 路径未引号包裹。建议 `"$SCRIPT_DIR/uninstall-service.sh"`。
- **MINOR-R5** · `package.sh` L100–L106 sanity check 在 Linux 主机也降级为 WARN（仅在主机能跑 ELF 时检测），命中范围太宽。建议 `uname -s` 判断当前是 Linux 时把 WARN 升为 FAIL。

### NIT

- **NIT-1** · README.md L22 最短示例 `<VERSION>` 占位符未在 README 顶部说明含义。
- **NIT-2** · `package.sh` 长参数解析未对 `${2:-}` 为空做防御。
- **NIT-3** · `cmd/frp-easy/main.go` `flag.ErrHelp` 分支几乎是死代码（冗余防御无害）。

---

## 6. 8 维度审计

| # | 维度 | 结果 |
|---|---|---|
| 1 | 逻辑正确性 | WARN（MAJOR-1 + MINOR-R3） |
| 2 | 需求保真 | PASS |
| 3 | 设计保真 | PASS（SUDO_USER 微调有合理理由） |
| 4 | 性能 | PASS |
| 5 | 安全 | PASS |
| 6 | 维护性 | PASS |
| 7 | 跨平台 | PASS（无 GNU 扩展、realpath fallback） |
| 8 | 文档准确性 | PASS（抽样 3 条命令对脚本 100% 匹配） |

---

## 7. Verdict

**CHANGES REQUIRED**（1 MAJOR + 5 MINOR）

理由：MAJOR-1（含空格路径下 systemd unit 解析失败）必须修复——用户解压到 `/opt/frp easy/` 或 `C:\Program Files\frp easy\` 会踩坑，与"傻瓜部署"承诺冲突。修复体量小（unit 模板加双引号或脚本入口 fail-fast），建议路由回 Developer 走快速一轮修补。

**路由清单**：

1. **MAJOR-1（必修）**：`install-service.sh` L115–L116 给 `ExecStart` / `WorkingDirectory` 加双引号包路径。
2. **MINOR-R1 ~ R5（建议同步修）**：体量小、价值显著，可一并落地。
3. **NIT**：可不修；后批清理。

修复完成后 Developer 重跑 `scripts/verify_all` 确认 18 项 PASS 不动，回 Stage 5 重审或直接进 Stage 6 QA。
