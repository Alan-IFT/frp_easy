# 02 — Solution Design · T-016 install-progress-and-systemd-unit-fix

> 阶段 2 / 7（Solution Architect）。模式：**full**。
> 上游只读输入：[01_REQUIREMENT_ANALYSIS.md](01_REQUIREMENT_ANALYSIS.md)（Verdict = READY，13 条 AC + 6 条 PM 默认）。
> 本设计严格遵守 §8 PM 默认决策与 §5 OOS；不引入新依赖（NFR-5）。

---

## 1. Architecture summary（架构摘要）

本任务**不引入新模块、不改动 Go 代码**，全部修复落在 4 个安装脚本与 1 条 insight。改动可归为三组正交补丁：

1. **A 组（UX）**：让 `scripts/install.sh` / `scripts/install.ps1` 的步骤 5 下载阶段在交互式终端显示可视进度，非交互式场景自动降级为不破坏中文文本日志的形式。
2. **B 组（正确性）**：把 `scripts/install-service.sh` 生成的 systemd unit 中 `ExecStart` / `WorkingDirectory` 从"整体双引号包路径"（systemd 拒收）改为 systemd.unit(5) 规定的"裸 token + 含空格走 C-style 反斜杠转义 `\x20`"语法；并在 `daemon-reload` 前用 `systemd-analyze verify` 做主动自检，失败回退。
3. **C 组（可观测）**：拆解 `if ! cmd; then rc=$?` 反模式，把 Linux 路径退出码透传按 §8.4 默认 (a) 改为 `cmd; rc=$?` 直行；增强 `install-service.sh` enable--now 失败时的诊断（自动打印 `systemctl status --no-pager` + `journalctl -n 20` + unit 文件绝对路径 + 清理提示）。Windows 端核查 `$LASTEXITCODE` 透传链路并补上等价 enable--now 失败诊断（如适用）。

不动 release.yml、Go 代码、Web UI、verify_all 检查项数量。

---

## 2. Affected modules（受影响模块）

| 文件 | 类型 | 改动概述 |
|---|---|---|
| `scripts/install.sh` | edit | 步骤 5（L193）curl flag 替换为带进度条 + TTY 探测；步骤 7（L262-272）退出码透传 `if !` 重构 |
| `scripts/install.ps1` | edit | 步骤 5（L151）下载逻辑改为可见进度（PS 5.x 与 7+ 双兼容方案）；步骤 7（L214-218）退出码透传复核（多数无需改） |
| `scripts/install-service.sh` | edit | unit 模板（L111-129）`ExecStart=` / `WorkingDirectory=` 改为 systemd 转义语法；新增 systemd-analyze verify 自检；enable--now 失败诊断增强（L142-145） |
| `scripts/install-service.ps1` | edit（小） | 仅在退出码透传链路有 PS 版本差异时补加 `exit $LASTEXITCODE`；当前实现已合规则零改动（待 C.3 确认） |
| `.harness/insight-index.md` | edit（阶段 7 落地） | 第 18 行 systemd unit 双引号 insight 替换为正确范式（具体替换文本见 §D.5；阶段 7 写入 07_DELIVERY.md 的 `## Insight` 段，由 `scripts/archive-task` 收割） |

无新文件、无 schema/迁移变更。

---

## 3. Module decomposition（新模块拆分）

**N/A** —— 本任务无新模块。所有改动是对既有脚本函数的局部修改。

---

## 4. Data model changes（数据模型变更）

**N/A** —— 本任务不涉及 DB / migrations / schema。

---

## 5. API contracts（API 契约）

**N/A** —— 本任务不涉及 REST 路由或 JSON shape。脚本退出码语义在 §C 详述（属用户可观察契约）。

---

## A. 下载进度显示设计

### A.1 Linux `curl` flag 选择（修复 `scripts/install.sh` 第 193 行）

**当前代码**：

```bash
if ! curl -fsSL -o "$TARBALL" "$ASSET_URL"; then
```

各 flag 当前语义（man curl 7.x，权威源 https://curl.se/docs/manpage.html，所有目标发行版 curl ≥ 7.61，flag 行为稳定）：

- `-f` / `--fail`：HTTP 状态码 ≥ 400 时 curl 返回非 0 退出码且**不写**响应体。
- `-s` / `--silent`：抑制进度条与错误信息。
- `-S` / `--show-error`：与 `-s` 组合时**只**在出错时显示错误（不显示进度条）。
- `-L` / `--location`：跟随 3xx redirect（GitHub Releases 资产是 302 重定向到 CDN，必需保留）。
- `--progress-bar`：用单行 `#` 进度条替代默认多行表格式 meter（默认 meter 在非常窄终端下也能用，但 `--progress-bar` 更紧凑、UX 更好）。

**决策**：将第 193 行 curl 命令的 flag 从 `-fsSL` 改为 **`-fSL`（去掉 `s`） + 条件附加 `--progress-bar`**。

**最终命令字符串**（伪代码，落地时由 Developer 写）：

```bash
# 在 TMP_DIR / TARBALL 赋值之后、curl 调用之前：
CURL_PROGRESS_FLAG=""
if [[ -t 2 ]]; then
    CURL_PROGRESS_FLAG="--progress-bar"
fi
if ! curl -fSL $CURL_PROGRESS_FLAG -o "$TARBALL" "$ASSET_URL"; then
    echo "错误：发布包下载失败，请检查网络后重试。" >&2
    exit 1
fi
```

**为什么保留 `-f` 满足 FR-A.5**：`-f` 在 HTTP 4xx/5xx 时让 curl 返回非 0；本步骤的错误分流逻辑（`if !`）依赖该非 0 退出码触发"发布包下载失败"中文报错并 `exit 1`。`-f` 行为本身与进度显示正交：进度条向 stderr 输出，遇到 4xx/5xx 时 curl 已在 header 阶段就 fail，进度条至多打印一两个字节后即终止，不会"看似下载成功"。这与 BC-3（API 步骤）不同——API 步骤需要看 4xx/5xx 响应体做分流（403 限流 / 404 未发布），所以那里**不能**用 `-f`；本步骤不需要响应体，能用 `-f`。

**为什么不用 `-S` 单独保留 silent 模式**：`curl -sS` 仅在错误时显示错误，正常下载时**完全静默**，与 FR-A.1 反目标。

**为什么不去掉 `--progress-bar` 走默认 meter**：默认 meter 形如多列百分比/速率/ETA 表格，在窄终端下会换行污染中文进度行；`--progress-bar` 是单行覆盖式 `#` 进度条，紧凑、不污染。

**引用证据**：curl 项目官方 manpage `--progress-bar` 与 `-f, --fail` 章节（curl 7.61+ 起一致；Ubuntu 22.04 curl 7.81、Debian 12 curl 7.88、RHEL 9 curl 7.76 均覆盖）。建议 Developer 落地前用 context7 MCP 拉取 `curl/curl` 仓库的 `docs/cmdline-opts/progress-bar.d` 二次核对（无变动 = 直接落地）。

### A.2 TTY 检测（FR-A.3、BC-A.5、FR-A.6）

**问题**：`curl -fsSL .../install.sh | sudo bash` 形态下，**install.sh 本身**的 stdin 不是 TTY（被 curl pipe 占用），但 stderr 通常仍连接到终端（除非用户 `| sudo bash 2>install.log`）。进度条由 curl 写 stderr。

**决策**：检测 **stderr 是否是 TTY**（`[[ -t 2 ]]`），是则附加 `--progress-bar`，否则裸跑 `-fSL` 等价当前静默行为（避免日志膨胀，满足 FR-A.6 / BC-A.5）。

**理由**：

- `[[ -t 2 ]]` 在 bash 内建，POSIX 兼容；不引依赖。
- 进度条目的就是给坐在终端前的人看；如果 stderr 已被重定向（`2>install.log`），就该自动降级。
- curl 自己的 `--progress-bar` 也只在 stderr 是 tty 时才"刷新"行（写 `\r`），重定向到文件时一样会写——但用户重定向了 stderr 就意味着主动选择记录到文件，那 `\r` + `#` 覆盖式输出反而污染日志。所以**我们自己**在脚本里就把 `--progress-bar` 拿掉，更干净。

**与"管道形态"细节的关系**：install.sh 主体的 stdin 是被 curl 输出，与下载步骤的 curl 是两个独立进程；下载步骤的 curl 拿到的是 install.sh 进程**继承**的 stdin/stdout/stderr，那是 `sudo bash` 的 fd，stderr 通常仍是终端（NFR-9 已确认这一物理隔离）。

### A.3 Windows 路径方案（`scripts/install.ps1` 步骤 5）

**问题分解**：

| PS 版本 | `Invoke-WebRequest -UseBasicParsing` | 进度行为 |
|---|---|---|
| PS 5.1（Windows PowerShell，.NET Framework） | `-UseBasicParsing` 显式抑制响应解析 DOM，**同时**抑制 `Write-Progress` 进度条（已确认行为，PowerShell 5.x 官方 doc `about_Preference_Variables` + Invoke-WebRequest cmdlet help） | **无进度** |
| PS 7+（PowerShell Core，.NET 5+） | `-UseBasicParsing` 已为 no-op（自 PS 6 起所有 IWR 都是 basic parsing），进度由 `$ProgressPreference` 控制，默认 `Continue` | **默认有进度**，但 IRM/IEX 管道场景下 `$Host.UI.SupportsVirtualTerminal` 可能为 false → 进度条变成滚动文本 |

**决策（PM 默认 §8.1 = (d) 组合形式 + 非交互降级）**：

采用 **`Invoke-WebRequest` + 显式控制 `$ProgressPreference`** 方案。**不**改 `System.Net.Http.HttpClient` 自实现（额外 ~30 行代码 + 字节计数 Tee + 速率计算 + Write-Progress 调度，超出 NFR-5 "不引新依赖" 的精神边界，且 BC-A 已允许"任一形式"，PS 内建机制够用）。

**最终方案**（伪代码）：

```powershell
# 步骤 5 下载前：
$prevProgress = $ProgressPreference
$isInteractive = [Environment]::UserInteractive -and -not [Console]::IsErrorRedirected
if ($isInteractive) {
    $ProgressPreference = 'Continue'   # 显式确保 PS 5.x 与 7+ 都尝试显示
} else {
    $ProgressPreference = 'SilentlyContinue'   # 非交互（管道到文件/CI）静默
}
try {
    # 关键变更：去掉 -UseBasicParsing（仅 PS 5.x 有意义；保留它则 PS 5.x 必无进度）
    # PS 7+ 下 -UseBasicParsing 已是 no-op，去掉无副作用。
    Invoke-WebRequest -Uri $assetUrl -OutFile $zipPath -ErrorAction Stop
} catch {
    Write-Error "发布包下载失败，请检查网络后重试。"
    exit 1
} finally {
    $ProgressPreference = $prevProgress
}
```

**为什么去 `-UseBasicParsing`**：

- PS 5.x：保留 `-UseBasicParsing` = 必无进度（与 FR-A.2 冲突）。
- PS 5.x：去掉它 → 触发 IE COM 引擎解析响应 body 的 DOM，但**下载二进制 zip 时无 HTML 解析**，副作用为零（实测：IWR 对非 text/html 内容跳过 DOM parsing）。
- PS 7+：`-UseBasicParsing` 已为 no-op，去掉它无任何行为差异。
- 风险：极小概率在没有 IE COM 环境的剥离版 Windows Server Core 上 PS 5.x IWR 无 `-UseBasicParsing` 会报错。**缓解**：catch 块兜底——任何下载异常都走"发布包下载失败"分支，行为与现状一致；且 Server Core 不是本项目主要目标平台（NFR-1 未列）。

**为什么不用 `System.Net.Http.HttpClient` 自实现**：会增加 30+ 行 PowerShell + 字节计数循环 + `Write-Progress` 调度，违反"最小修改"精神，且 BC-A.2 允许快网络下完全不出现进度——不需要精确的速率/ETA。

**`-UseBasicParsing` 行为引用证据**：Microsoft Learn `Invoke-WebRequest` cmdlet reference（PowerShell 5.1 vs 7.x 双版本页面）；`about_Preference_Variables` 的 `$ProgressPreference` 章节。Developer 落地前用 context7 MCP 二次核对（`microsoft/powershell` 或 `MicrosoftDocs/PowerShell-Docs`）。

---

## B. systemd unit 语法修复设计

### B.1 systemd 语法权威与最终模板

**权威源**：systemd 官方手册（freedesktop.org / systemd.io）：

- **systemd.unit(5)** —— 通用 directive 语法、引号/转义规则。
- **systemd.service(5)** —— `ExecStart=` 字段的精确语义（含 binary path、参数、特殊前缀字符 `@-+:!`）。
- **systemd.exec(5)** —— `WorkingDirectory=` 字段语义。

**核心事实**（来自上述 manpage，所有目标发行版 systemd ≥ 249 均覆盖）：

1. **`ExecStart=`** 的值是"命令行"，按 shell-like word splitting 切分，但**不**经过 shell。第一个 token 必须是绝对路径或在 `$PATH` 中可解析的可执行文件名。systemd 自 v240 起新增支持"用双引号包整个 token 让其内部空格被视作单 token"语法（`ExecStart="/path with spaces/bin" arg1`），但**该语法仅对 `ExecStart=` 等命令行 directive 生效，且整个 token 必须是双引号包裹的 quoted token**。
2. **`WorkingDirectory=`** 的值是"路径"，**不是**命令行；word splitting 规则不同。systemd.exec(5) 明确：含特殊字符（空格等）的路径**必须使用 C-style escape 序列**，例如空格 = `\x20`、tab = `\x09`、反斜杠本身 = `\x5c`。**不能**用整体双引号包裹 `WorkingDirectory="/foo bar"` —— 该写法被 systemd parser 拒绝为 `bad unit file setting`（这正是 T-008 insight 第 18 行的反例落地证据）。
3. **裸 unquoted 写法** `ExecStart=/opt/frp-easy/frp-easy` 与 `WorkingDirectory=/opt/frp-easy` 是**默认路径（无空格）下唯一无歧义的形式**，所有 systemd ≥ 230 均接受。

**根因诊断（针对 T-008 insight 18 行）**：

T-008 当时把 `ExecStart="${BINARY}"` 与 `WorkingDirectory="${INSTALL_DIR}"` 整体双引号写法当作"防空格 + 防转义"通用范式记入 insight。但实际上：

- 对 `ExecStart=`：systemd ≥ 240 接受 `ExecStart="/path with spaces/bin"`（quoted-executable-path 语法），systemd 230-239 **不**接受。Ubuntu 22.04（systemd 249）形式上接受，但 T-008 加引号时**路径是 `/opt/frp-easy/frp-easy`**——没有空格的路径被双引号包裹时，systemd 早期版本 parser 可能直接报 `bad unit file setting`（与 manpage 描述不完全一致，属 systemd 历史实现细节）。
- 对 `WorkingDirectory=`：systemd 任何版本**都不**接受整体双引号——该字段的解析路径与 `ExecStart=` 不同，引号字符进入字符串本身，生成的路径变成 `"/opt/frp-easy"` 字面（含引号），导致目录不存在或 `bad unit file setting`。**这就是当前 bug 的根因**。

### B.2 最终 unit 模板（裸 unquoted + `\x20` 转义）

**默认路径**（`/opt/frp-easy`，无空格）：

```ini
[Unit]
Description=FRP Easy — frp 可视化管理 UI
After=network.target
Documentation=https://github.com/Alan-IFT/easy_frp

[Service]
Type=simple
ExecStart=/opt/frp-easy/frp-easy
WorkingDirectory=/opt/frp-easy
User=<RUN_USER>
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

**含空格路径**（如 `FRP_EASY_INSTALL_DIR=/opt/frp easy/v1`），转义后：

```ini
ExecStart=/opt/frp\x20easy/v1/frp-easy
WorkingDirectory=/opt/frp\x20easy/v1
```

### B.2.1 `install-service.sh` 中如何做转义

**问题**：`systemd-escape` 命令在所有目标发行版上是否一定可用？

- Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 —— `systemd-escape` 随 `systemd` 包安装，所有上述发行版默认有；可视为可用。
- 但 `systemd-escape --path` 是**反向**操作（把路径"转成 unit 名"，例如 `/foo/bar` → `foo-bar`），**不**用于 `ExecStart=` 内空格转义。
- `systemd-escape` 没有"把字符串里空格替换为 `\x20`"的开箱即用模式。

**决策**：**用纯 bash 字符串替换实现 `\x20` 转义，不依赖 `systemd-escape`**。范围严格限定为"空格"一个字符（OOS-2 已排除 shell 元字符、控制字符、非 ASCII），所以一行 bash 内建替换足够：

```bash
systemd_escape_path() {
    # 仅转义空格为 \x20；其它字符按 OOS-2 不支持
    local p="$1"
    printf '%s' "${p// /\\x20}"
}

ESC_BINARY="$(systemd_escape_path "$BINARY")"
ESC_INSTALL_DIR="$(systemd_escape_path "$INSTALL_DIR")"

# 在 heredoc 中：
# ExecStart=${ESC_BINARY}
# WorkingDirectory=${ESC_INSTALL_DIR}
```

**注意 heredoc 双重转义陷阱**：`${p// /\\x20}` 在 bash 中 `\\` 是字面反斜杠，`x20` 是字面 `x20`，最终结果是字符串字面量 `\x20`（6 个字符序列 `\`、`x`、`2`、`0`——其实是 4 个字符）。写到 heredoc 时**不能**用 `<<EOF`（不带引号的 heredoc 会做变量展开但反斜杠保持字面），ok；也可以稳妥起见用 `<<'EOF'` 完全字面 heredoc 配合 `envsubst` 或 `sed` 注入——但 envsubst 不在所有发行版默认有。**最终决定**：仍用 unquoted heredoc `<<EOF`（与现行第 111-129 行一致），仅把 `${BINARY}` / `${INSTALL_DIR}` 改为转义后的 `${ESC_BINARY}` / `${ESC_INSTALL_DIR}`，并**去掉**它们外层的双引号。Developer 落地后必须人工 cat 一次默认路径 unit 与 含空格路径 unit 验证文本字节，不能只依赖 systemd-analyze。

**关于 OOS-2**：含 shell 元字符（`$`、反引号、`"`、`\`）的路径由 RA §5 OOS 排除。本设计**不**为此类路径加防护代码；如果用户传入该类路径，`systemd-analyze verify` 步骤会捕获语法错误并失败退出（见 B.3）。

### B.3 unit 写出后的主动自检

**决策**：在 `mv -f "$TMP_UNIT" "$UNIT_PATH"` 之后、`systemctl daemon-reload` 之前，**新增一步** `systemd-analyze verify`：

```bash
chmod 0644 "$UNIT_PATH"

# 新增：unit 语法自检（systemd ≥ 220 提供 systemd-analyze verify）
if command -v systemd-analyze >/dev/null 2>&1; then
    if ! systemd-analyze verify "$UNIT_PATH" 2>/tmp/frp-easy-unit-verify.err; then
        echo "错误：systemd unit 文件语法校验失败：" >&2
        cat /tmp/frp-easy-unit-verify.err >&2
        echo "      unit 文件路径：$UNIT_PATH" >&2
        echo "      已自动回退（删除该 unit 文件）。请提交 issue 附上上述错误。" >&2
        rm -f "$UNIT_PATH"
        rm -f /tmp/frp-easy-unit-verify.err
        exit 2
    fi
    rm -f /tmp/frp-easy-unit-verify.err
fi

# 然后才 daemon-reload
if ! systemctl daemon-reload; then
    ...
fi
```

**理由**：

- `systemd-analyze verify` 在 unit 文件上跑后会检查所有 directive 语法、依赖、字段值合法性；语法错误直接报"Failed to parse"或"bad unit file setting"。能在 daemon-reload 之前抓住 99% 的语法错误。
- 失败时**主动回退**（`rm -f $UNIT_PATH`）避免遗留半成品 unit；这与 FR-C.5（半成功留 unit 给用户审阅）有张力，但**注意**：B.3 的失败是**语法**层失败（unit 根本不可能 reload 成功），不是 enable--now 运行时失败；语法失败时留下 unit 反而误导用户。区分清楚：
  - **语法层失败**（B.3 命中）→ 回退删 unit + exit 2，对应 AC-4 / AC-5 路径的最早拦截点。
  - **运行时失败**（enable --now 失败）→ 留 unit 在盘 + 强诊断（C.2），对应 FR-C.5 / AC-13。
- `systemd-analyze` 不一定每个发行版都有？所有 NFR-1 目标发行版（Ubuntu 22.04+、Debian 12、RHEL 9）的 systemd 包都附带它；BC-B.4 已 OOS 老版本。Older 的极端容器化环境如果没有，本步骤会因 `command -v` 检测失败而**跳过**（不影响主流程），最差降级为现行行为。

**`systemd-analyze verify` 引用证据**：systemd.io 文档 systemd-analyze(1)，verify 子命令章节明确"Will load the given unit file(s) and print warnings if any errors are detected"。

---

## C. 退出码透传 / 失败检测设计

### C.1 `scripts/install.sh` L262-272 退出码透传修复

**当前代码（bug）**：

```bash
if ! bash "$SERVICE_SCRIPT"; then
    rc=$?
    echo "错误：服务注册失败（install-service.sh 退出码 ${rc}）。请查看上方 install-service.sh 的中文报错。" >&2
    exit "$rc"
fi
```

**bash 行为查证**：

- POSIX `$?` 定义：last foreground pipeline 的退出状态。
- 在 `if ! cmd; then ...; fi` 中，`!` 是 reserved word 反转 pipeline 退出状态。
- bash manual（GNU bash 5.x）`Pipelines` 章节：`!` 的作用是 "the exit status of the pipeline is the logical negation of the exit status as described previously"。
- 关键：`$?` 在 `then` 分支内的值，**根据 bash 实测与 POSIX 规范**，是 `!` **反转之前**的真实 cmd 退出码 vs `!` **反转后**的逻辑值（0 或 1）—— 这在不同 shell（bash vs dash vs zsh）下不一致。**bash 实测**：`then` 内 `$?` 是 cmd 的真实退出码（未被 `!` 反转）。但 `set -e` + `if !` 在某些 bash 版本上的交互曾经有 bug（bash < 4.4），保险起见**不应**依赖该行为。

**§8.4 PM 默认 = (a)**：拆开为 `bash ...; rc=$?` 直行。

**最终修复模式**（伪代码）：

```bash
# install.sh 步骤 7（L262-272 处）
echo "==> [7/8] 注册 systemd 开机自启服务..."
if [[ ! -f "$SERVICE_SCRIPT" ]]; then
    echo "错误：未找到 ${SERVICE_SCRIPT}，发布包结构异常。" >&2
    exit 1
fi

# 关键：临时关闭 set -e 让 SERVICE_SCRIPT 非 0 退出不直接终止本脚本，
# 才能把真实退出码捕获到 rc 变量。
set +e
bash "$SERVICE_SCRIPT"
rc=$?
set -e

if [[ "$rc" -ne 0 ]]; then
    echo "错误：服务注册失败（install-service.sh 退出码 ${rc}）。请查看上方 install-service.sh 的中文报错。" >&2
    exit "$rc"
fi
```

**为什么需要 `set +e` / `set -e` 包裹**：脚本顶部 `set -euo pipefail`（L23）开启了 errexit；`bash "$SERVICE_SCRIPT"` 直接调用如返回非 0，shell 立即终止——拿不到 `rc`。`if !` 形式之所以"能跑"，是因为 `if` / `while` 等条件上下文中 `set -e` 不生效（bash 文档 The Set Builtin），但这恰恰是引入 `$?` 模糊语义的根因。`set +e ... set -e` 显式包裹是最干净的：意图明确，跨 bash 版本可读。

**与 `pipefail` 的兼容性**：本调用是单条命令而非 pipeline，`pipefail` 不影响。

### C.2 `scripts/install-service.sh` enable--now 失败诊断增强

**当前代码（L142-145）**：

```bash
if ! systemctl enable --now "$UNIT_NAME"; then
    echo "错误：systemctl enable --now $UNIT_NAME 失败；请查看 journalctl -u $UNIT_NAME。" >&2
    exit 2
fi
```

**§8.2 PM 默认 = (b)**：自动打印 status + journalctl 最近 20 行。

**最终修复模式**（伪代码）：

```bash
# install-service.sh L142-145 改为：
set +e
systemctl enable --now "$UNIT_NAME"
enable_rc=$?
set -e

if [[ "$enable_rc" -ne 0 ]]; then
    echo "错误：systemctl enable --now $UNIT_NAME 失败（退出码 $enable_rc）。" >&2
    echo "" >&2
    echo "==== 诊断信息：systemctl status $UNIT_NAME --no-pager ====" >&2
    systemctl status "$UNIT_NAME" --no-pager 2>&1 | sed 's/^/    /' >&2 || true
    echo "" >&2
    echo "==== 诊断信息：journalctl -u $UNIT_NAME --no-pager -n 20 ====" >&2
    journalctl -u "$UNIT_NAME" --no-pager -n 20 2>&1 | sed 's/^/    /' >&2 || true
    echo "" >&2
    echo "unit 文件已写入：$UNIT_PATH（可 cat 审阅）。" >&2
    echo "如需清理：sudo systemctl disable $UNIT_NAME && sudo rm -f $UNIT_PATH && sudo systemctl daemon-reload" >&2
    exit 2
fi
```

**为什么用 `sed 's/^/    /'`**：把 status / journalctl 输出前缀 4 空格缩进，让用户能一眼看出"诊断块"边界（与外层中文行视觉分离）。

**为什么 `2>&1 ... >&2`**：把子命令的 stdout 与 stderr 都引到 install-service.sh 的 stderr —— 错误诊断属错误流，统一去 stderr 才不污染上游 install.sh 步骤 7 的 stdout 进度。

**对应 AC-8（诊断片段实际可见）、AC-13（unit 路径 + 清理提示）**。

### C.3 `scripts/install.ps1` 步骤 7 退出码透传

**当前代码（L214-218）**：

```powershell
& $svc
if ($LASTEXITCODE -ne 0) {
    Write-Error "服务注册失败（install-service.ps1 退出码 $LASTEXITCODE）。请查看上方中文报错。"
    exit $LASTEXITCODE
}
```

**PS 5.x vs 7+ 行为查证**：

- `& <scriptpath>.ps1` 在两个版本均会执行脚本；`$LASTEXITCODE` 在两个版本均会被设置为脚本内 `exit N` 中的 N（PowerShell 文档 about_Operators 与 about_Automatic_Variables：`$LASTEXITCODE`"Contains the exit code of the last Windows-based program that was run."）。
- **历史细节**：`$LASTEXITCODE` 严格意义只对**原生命令（external executables）**保证；对**调用的 PowerShell 脚本**，`exit N` 语句**也**会更新 `$LASTEXITCODE`（自 PowerShell 2 起一致行为）。两版本无差异。
- **唯一陷阱**：如果 install-service.ps1 因 `$ErrorActionPreference = "Stop"` 触发 terminating error 而**未**走到 `exit N`，`$LASTEXITCODE` 可能保留上一条命令的值（陈旧值）—— 这是真实风险，但仅在 install-service.ps1 自身没正确 catch 异常时发生；当前实现已 catch 关键路径。

**决策**：**install.ps1 L214-218 的当前代码已合规**——`$LASTEXITCODE` 检查与透传链路正确。**唯一改进**：在 `& $svc` 之前先 `$LASTEXITCODE = 0` 重置，防止"未触发 exit"陷阱。

**最终修复模式**（伪代码）：

```powershell
# install.ps1 L214 之前插入：
$LASTEXITCODE = 0
& $svc
if ($LASTEXITCODE -ne 0) {
    Write-Error "服务注册失败（install-service.ps1 退出码 $LASTEXITCODE）。请查看上方中文报错。"
    exit $LASTEXITCODE
}
```

**对应 FR-C.4 / AC-9**。

**关于 install-service.ps1 失败诊断增强**：Windows 端 sc.exe 失败时，等价 `journalctl` 的是 Windows 事件查看器（Event Viewer），但脚本里读 Event Log 需要 `Get-WinEvent -LogName System` + filter，且容易因 locale 字符串变化失效。**决定不做 PS 端诊断块增强**——sc.exe 自己的退出码与 stderr 已经直接打印（current L98-106、L119-122 已展示退出码），用户能 `sc query frp-easy` 查状态。这与 §8.5 PM 默认 = (b) 一致（install-service.* 本身不引入新诊断块，因 Windows 端缺等价 journalctl）。

### C.4 退出码契约表（用户可观察）

| 场景 | install-service.sh 退出码 | install.sh 透传后退出码 | 当前是否 OK |
|---|---|---|---|
| 成功 | 0 | 0（继续步骤 8） | OK |
| 非 root | 1 | 1 | 当前 if! 形式也 OK，但加固后仍 OK |
| 缺 systemctl | 1 | 1 | 同上 |
| user 不存在 | 1 | 1 | 同上 |
| daemon-reload 失败 | 2 | **2**（当前 bug：实测报 0） | 修复后 OK |
| enable --now 失败 | 2 | **2**（当前 bug：实测报 0） | 修复后 OK |
| systemd-analyze verify 失败（B.3 新增） | 2 | 2 | 修复后 OK |

---

## 6. Sequence / flow（流程）

```
用户执行：curl -fsSL .../install.sh | sudo bash
   │
   ├─ install.sh 步骤 1-4：环境检测 / OS 探测 / API 查询 / 解析资产 URL（不改）
   │
   ├─ install.sh 步骤 5：下载发布包
   │     ├─ 探测 [ -t 2 ]，决定是否附加 --progress-bar
   │     ├─ curl -fSL [--progress-bar] -o $TARBALL $ASSET_URL
   │     │     ├─ 成功 → 进度条占满下载时长，stderr 单行覆盖
   │     │     └─ 失败 → curl 退出非 0 → 中文报错 → exit 1（FR-A.5）
   │     ├─ tar tzf / xzf 校验 + 解压
   │
   ├─ install.sh 步骤 6：安装到 /opt/frp-easy（升级 or 全新；不改）
   │
   ├─ install.sh 步骤 7：注册服务
   │     ├─ macOS 分支：exit 0（不改）
   │     ├─ Linux 分支：
   │     │     ├─ set +e; bash $SERVICE_SCRIPT; rc=$?; set -e
   │     │     ├─ install-service.sh 流程：
   │     │     │     ├─ 前置检测（root / systemctl / binary / user）
   │     │     │     ├─ 计算 ESC_BINARY / ESC_INSTALL_DIR（空格 → \x20）
   │     │     │     ├─ 原子写 unit 文件（ExecStart= / WorkingDirectory= 裸 token）
   │     │     │     ├─ systemd-analyze verify $UNIT_PATH
   │     │     │     │     ├─ 失败 → rm $UNIT_PATH; exit 2（回退）
   │     │     │     │     └─ 成功 → 继续
   │     │     │     ├─ systemctl daemon-reload（失败 → exit 2）
   │     │     │     ├─ systemctl enable --now $UNIT_NAME
   │     │     │     │     ├─ 失败 → 打印 status + journalctl + unit 路径 + 清理提示; exit 2
   │     │     │     │     └─ 成功 → 打印就绪提示; exit 0
   │     │     ├─ 回到 install.sh：if [[ $rc -ne 0 ]]; then exit $rc（透传 1/2）
   │     │     └─ 成功 → 步骤 8 打印结果
```

---

## 7. Reuse audit（复用审计）

| 需求 | 既有代码 | 文件路径 | 决策 |
|---|---|---|---|
| TTY 检测 | bash 内建 `[[ -t N ]]` | shell builtin | 直接用 |
| curl 进度条 | curl `--progress-bar` flag | 系统 curl | 直接用 |
| PowerShell 进度控制 | `$ProgressPreference` 内建变量 + `Invoke-WebRequest` 内建进度 | PS runtime | 直接用 |
| systemd unit 原子写 | `install-service.sh` L107-133 现有 mv-rename + 双重 chmod 模式 | `scripts/install-service.sh` | **复用**，仅改 heredoc 内 `ExecStart=` / `WorkingDirectory=` 两行 |
| systemd 语法校验 | `systemd-analyze verify` 命令 | 系统 systemd 包 | 直接用（command -v 探测保护） |
| systemctl status / journalctl 摘要 | 直接 shell 调用 | 系统 systemd 包 | 直接用 + sed 缩进 |
| 退出码捕获模式 `set +e; cmd; rc=$?; set -e` | （无既有项目内复用） | 通用 bash 模式 | **新引入但是标准模式**，无依赖 |
| Windows `$LASTEXITCODE` 链路 | `install.ps1` L215-218 当前实现 | `scripts/install.ps1` | 复用 + 仅加 `$LASTEXITCODE = 0` 重置 |
| 中文报错文案 | 既有 `echo "错误：..." >&2` 模式 | 全部脚本 | 复用风格 |
| 升级语义保留（NFR-8） | `install.sh` 步骤 6（L220-245） + `install.ps1` 同位置 | 既有 | 不改 |

**结论**：无新模块、无新依赖；所有改动都是对既有脚本中既定函数与既定模式的局部修改。NFR-5 满足。

---

## 8. Risk analysis（风险分析）

| ID | 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|---|
| R-1 | `systemd-analyze verify` 在极简容器/裁剪发行版上不可用 | 低 | 跳过自检导致语法错误延后到 daemon-reload 才暴露（回到当前行为） | `command -v systemd-analyze` 探测保护 + daemon-reload 失败仍走 enable--now 失败诊断块（C.2），用户最终能看到错误。BC-B.4 已 OOS 老 systemd。 |
| R-2 | bash heredoc 中 `\x20` 字面量被 shell 误解释 | 中 | 写出的 unit 文件含字面 `\\x20` 而非 `\x20`，systemd 解析失败 | 用 `<<EOF` unquoted heredoc + 变量先 `printf '%s' "${p// /\\x20}"` 预转义；落地后 Developer 必须人工 `cat /etc/systemd/system/frp-easy.service` 验证字节并跑 systemd-analyze verify。AC-5（含空格路径）专门测此场景。 |
| R-3 | PS 5.x 去掉 `-UseBasicParsing` 后在某些精简 Windows 环境（Server Core / Nano）触发 IE COM 加载错误 | 极低 | 下载失败 → 走"发布包下载失败"分支 → exit 1 | Server Core 不在 NFR-1 主测平台；现行错误分流路径已能 catch（current try/catch 不变）。AC-2 在常规 Win 11 PS 5.1 测试覆盖主用例。 |
| R-4 | `set +e ... set -e` 包裹忘记恢复 errexit 状态 | 低 | install.sh / install-service.sh 后续命令失败不再立即终止 | 严格的 `set +e` 紧跟 `cmd` 紧跟 `rc=$?` 紧跟 `set -e` 三行块，不混入其它逻辑；Code Reviewer 阶段 5 必检该模式正确性。 |
| R-5 | `--progress-bar` 在某些 minimal busybox curl 上不支持 | 极低 | curl 报"unknown option"退出 | 所有目标发行版的 curl 都是 GNU 完整版，不是 busybox；如果用户在 Alpine 上跑，本就不在 NFR-1 范围。仍然加 `2>/dev/null` 兜底是过度防御，**不加**。 |
| R-6 | enable --now 失败时 status + journalctl 输出过长污染终端 | 低 | 用户视觉负担 | journalctl `-n 20` 限制行数 + status `--no-pager` 不分页 + sed 缩进；最差 20 行 status + 20 行 journal ≈ 40 行，可接受。 |
| R-7 | unit 文件 systemd-analyze verify "失败但其实可用"的 false positive（systemd-analyze 比 systemd 自身 parser 更严格） | 低 | 误删可用 unit 导致 exit 2 | systemd-analyze verify 失败时**只在 fatal 错误**才返回非 0（warnings 不影响退出码）；保险起见，落地时 Developer 在四个目标发行版各跑一次默认路径 + 含空格路径，确认本设计的 unit 文本能通过 verify。 |
| R-8 | T-008 insight 18 行已被引用到其它任务/文档（伪复用） | 低 | 替换后误导旧文档 | 阶段 6 QA 跑 `git grep "systemd unit.*双引号"` 确认无其它文件复制了错误事实；如有，归 OOS 不在本任务修，PM 另起小任务。 |

---

## 9. Migration / rollout plan（迁移 / 滚动发布计划）

**向后兼容**：

- **已安装 v0（旧 unit 含双引号）的环境**：用户重跑 install.sh → 步骤 6 走"升级"分支 → 步骤 7 调用新版 install-service.sh → 因新 install-service.sh L101-104 检测到既有 unit 走 "刷新并重启" 分支 → 写新 unit（裸 token 语法）覆盖旧 unit → systemd-analyze verify pass → daemon-reload → enable --now → active。用户**无感知**地把损坏 unit 升级为可用 unit，符合 NFR-8 升级/幂等语义、AC-12。
- **新安装**：直接走新代码路径，AC-4 / AC-5 / AC-6 命中。

**Feature flags**：无；脚本改动是非破坏性的纯修复，不需开关。

**回滚**：

- 若线上发现新 unit 语法在某发行版仍失败：`git revert <commit>`，重新发布 rolling tag（push main 触发 release.yml 自动滚动），用户 5 分钟内一条命令重跑即可回到旧行为。
- B.3 systemd-analyze verify 步骤自带回退（删 unit + exit 2），不会留半成品。

**数据迁移**：无（unit 文件是 systemd 状态而非数据）。

**verify_all 影响**：A.1 secrets scan 正则 `(api_key|secret|password|token)[[:space:]]*[:=][[:space:]]*["'][^"']{8,}["']` 不会误中本任务的 ASCII 进度条字符 / `\x20` 转义 / 中文报错；NFR-7 不退化。

---

## 10. Out-of-scope clarifications（设计边界）

本设计**不**覆盖：

- 含 shell 元字符（`$`、反引号、`"`、`\`、控制字符、非 ASCII）的安装路径 —— OOS-2。设计假设：仅 ASCII + 空格。
- macOS launchd 等价服务化 —— OOS-8、NFR-2。macOS 路径仍打印手动启动提示并 exit 0。
- Windows 端等价 journalctl 诊断块 —— C.3 末段已论述：Event Log 读取代价/可靠性比收益高，不做。
- install-service.* 步骤的进度显示 —— OOS-5 / §8.5 PM 默认 (b)：daemon-reload / enable --now / sc.exe create 都是秒级动作，不需要进度。
- `--help` 文案补充 "步骤 5 会显示下载进度" —— §8.6 PM 默认 (b)：UX 改进不是 CLI 契约，不写。
- frp 二进制下载步骤（T-014 引入的 UI 内下载） —— 与本任务正交，不涉及。
- release.yml / GitHub Actions —— OOS-7。
- README / DEPLOYMENT 文档大改 —— OOS-10。
- verify_all 检查项数量 —— OOS-9。

---

## 11. Partition assignment（分区分配）

项目存在 `.harness/agents/dev-{db,backend,frontend}.md` 三个分区。本任务全部改动落在 `scripts/**`，按 `dev-backend.md` Owned paths 第 4 行 `scripts/start.{ps1,sh}、scripts/build.{ps1,sh}、scripts/verify_all.{ps1,sh}` 列表，**install.{sh,ps1} 与 install-service.{sh,ps1} 严格说不在三个分区的 owned paths 列表内**——属"Harness 脚本外的部署脚本"。

**决策**：按"语义就近"原则把所有 scripts/install*.* 改动派给 `dev-backend`（理由：install-service.* 与 systemd / Windows Service 注册属系统服务工程，与后端运行环境最近；install.* 是部署编排，亦更接近后端域；frontend 分区无相关知识；dev-db 无关）。落地时 Developer 在 `04_DEVELOPMENT.md` 中显式说明"超出 owned paths 但 SA 授权"以避免 Code Reviewer 阶段 5 的 `BLOCKED ON PARTITION` 误报。

| 文件 | Partition | New / Edit | Dependency |
|---|---|---|---|
| `scripts/install-service.sh` | dev-backend | edit（B.1 / B.2 unit 模板 + B.3 systemd-analyze + C.2 诊断块） | — |
| `scripts/install.sh` | dev-backend | edit（A.1 / A.2 curl 进度 + TTY 探测 + C.1 退出码透传） | depends on `install-service.sh`（运行时调用） |
| `scripts/install.ps1` | dev-backend | edit（A.3 PowerShell 进度方案 + C.3 `$LASTEXITCODE` 重置） | depends on `install-service.ps1`（运行时调用） |
| `scripts/install-service.ps1` | dev-backend | edit（**仅** `$LASTEXITCODE` 重置考量；如 SA 复核后无需改则零改动） | — |
| `.harness/insight-index.md` | dev-backend | edit（阶段 7 由 `archive-task` 自动收割；Developer 不直接改） | — |

## Dispatch order

1. **dev-backend**（单分区覆盖全部改动）

## Parallelism

N/A —— 单分区。可在内部并行编辑 4 个脚本（无文件级依赖），但 verify_all 必须串行收尾跑一次。

---

## D. 整体收尾设计

### D.1 文件清单（一句话粒度）

| 文件 | 改什么 |
|---|---|
| `scripts/install.sh` | 第 193 行 `curl -fsSL` 改为 `curl -fSL`（去 `-s`）；新增 `[[ -t 2 ]]` 探测附加 `--progress-bar`；第 262-272 行 `if ! bash ...` 改为 `set +e; bash ...; rc=$?; set -e` 三行块 + `if [[ $rc -ne 0 ]]`。 |
| `scripts/install.ps1` | 第 151 行 `Invoke-WebRequest -OutFile -UseBasicParsing` 去掉 `-UseBasicParsing`，外包 `$ProgressPreference` 临时开关与 `$isInteractive` 探测；第 214 行前加 `$LASTEXITCODE = 0` 重置。 |
| `scripts/install-service.sh` | 新增 `systemd_escape_path()` bash 函数（仅转空格→`\x20`）；第 119-120 行 `ExecStart="${BINARY}"` / `WorkingDirectory="${INSTALL_DIR}"` 改为 `ExecStart=${ESC_BINARY}` / `WorkingDirectory=${ESC_INSTALL_DIR}`（去外引号）；第 133 行 chmod 之后新增 `systemd-analyze verify` 自检块（失败 rm unit + exit 2）；第 142-145 行 enable--now 失败块扩展为打印 status + journalctl 摘要 + unit 路径 + 清理提示。 |
| `scripts/install-service.ps1` | **无需改动**（C.3 分析确认 `$LASTEXITCODE` 链路本身合规）。如 Developer 落地时发现 PS 7 行为有 corner case，仅在该文件相应位置补 `$LASTEXITCODE = 0` 重置。 |
| `.harness/insight-index.md` | 阶段 7 替换第 18 行（具体文本见 §D.5）；本阶段不动。 |

### D.2 兼容矩阵

| 发行版 | systemd 版本 | A 组（progress） | B 组（unit syntax） | C 组（退出码 / 诊断） |
|---|---|---|---|---|
| Ubuntu 22.04 LTS | 249 | curl 7.81 支持 `--progress-bar` ✓ | unquoted ExecStart + `\x20` ✓；systemd-analyze 可用 ✓ | bash 5.x set +e/-e ✓；journalctl ✓ |
| Ubuntu 24.04 LTS | 255 | curl 8.x ✓ | ✓；systemd-analyze 可用 ✓ | ✓ |
| Debian 12 | 252 | curl 7.88 ✓ | ✓；systemd-analyze 可用 ✓ | ✓ |
| RHEL 9 / CentOS Stream 9 | 252 | curl 7.76 ✓ | ✓；systemd-analyze 可用 ✓ | ✓ |
| macOS | n/a | curl ≥ 7.64 ✓ | 跳过（exit 0 降级） | n/a |
| Windows 11 | n/a | PS 5.1 IWR 去 `-UseBasicParsing` + `$ProgressPreference=Continue` ✓ | n/a | `$LASTEXITCODE` 链路 ✓ |
| Windows 11 + PS 7.4 | n/a | PS 7 IWR 进度默认显示 ✓ | n/a | ✓ |

**已知盲区**：
- Alpine Linux / busybox curl：OOS（NFR-1 未列）。
- WSL1：无 systemd，install-service.sh 前置检测会拒。
- Windows Server Core：R-3 已论述风险与降级。

### D.3 风险与缓解

见 §8 风险表 R-1 至 R-8（共 8 条 ≥ 设计契约要求的 3 条）。最关键 3 条复述：

1. **R-2（heredoc `\x20` 转义陷阱）**：mitigated by 落地后人工 cat unit 验证 + systemd-analyze verify + AC-5 含空格路径专测。
2. **R-4（set +e/-e 包裹失误）**：mitigated by 三行块约束 + Code Review 必检。
3. **R-1（systemd-analyze 老 systemd 不可用）**：mitigated by `command -v` 探测保护 + 降级到既有行为。

### D.4 测试策略提示（给阶段 6 QA）

QA 在 `06_TEST_REPORT.md` 的 `## Adversarial tests` 段建议覆盖：

**正面用例**（对应 AC-1 至 AC-6, AC-10, AC-11, AC-12）：

```bash
# AC-1：Ubuntu 22.04 干净 VM 跑一键安装，观察步骤 5 进度
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash
# 验证：步骤 5 stderr 出现 `#` 进度条或百分比

# AC-4：安装后服务状态
systemctl is-active frp-easy   # 期望 active
systemctl status frp-easy --no-pager | grep -i "bad unit file"  # 期望无匹配
```

**对抗性场景**：

```bash
# A1：自定义含空格安装路径（AC-5）
sudo FRP_EASY_INSTALL_DIR="/opt/frp easy/v1" bash install.sh
cat /etc/systemd/system/frp-easy.service | grep -E "ExecStart=|WorkingDirectory="
# 期望看到 \x20 转义，例如：ExecStart=/opt/frp\x20easy/v1/frp-easy
systemd-analyze verify /etc/systemd/system/frp-easy.service  # 期望 exit 0
systemctl is-active frp-easy  # 期望 active

# A2：断网试进度降级（AC-3）
sudo iptables -A OUTPUT -d github.com -j REJECT  # 临时阻断
bash install.sh
# 期望：步骤 5 报"发布包下载失败" + exit 1；进度条不掩盖错误
sudo iptables -D OUTPUT -d github.com -j REJECT  # 恢复

# A3：删 binary 试退出码透传（AC-7）
sudo chmod 000 /opt/frp-easy/frp-easy
sudo bash /opt/frp-easy/scripts/install-service.sh
echo "rc=$?"  # 期望 2

# 通过 install.sh 间接调用（验证透传链路）
# 删 install dir 后重跑 install.sh，让 SERVICE_SCRIPT 找不到 binary（rc=1）
sudo rm -f /opt/frp-easy/frp-easy
sudo bash -c 'curl -fsSL .../install.sh | bash'
# 期望：步骤 7 报告"退出码 1"（不是 0）

# A4：stdout 重定向降级（FR-A.6 / BC-A.5）
sudo bash install.sh > /tmp/install.log 2>&1
# 期望：/tmp/install.log 不含 `\r` 覆盖序列、不含多个 # 进度条快照行

# A5：含空格路径 unit 启动失败诊断（AC-8 / AC-13）
sudo FRP_EASY_INSTALL_DIR="/opt/frp easy/v1" bash install.sh
sudo chmod 000 "/opt/frp easy/v1/frp-easy"  # 制造启动失败
sudo bash "/opt/frp easy/v1/scripts/install-service.sh"
# 期望 stderr 包含：
#   - "==== 诊断信息：systemctl status frp-easy --no-pager ===="
#   - 实际 status 输出（缩进 4 空格）
#   - "==== 诊断信息：journalctl -u frp-easy --no-pager -n 20 ===="
#   - "unit 文件已写入：/etc/systemd/system/frp-easy.service"
#   - "如需清理：sudo systemctl disable frp-easy && sudo rm -f ..."

# A6：systemd-analyze verify 失败回退（R-7 反向测试）
# 手工写一个明显错误的 unit（如 [Service] 段下乱写 Type=invalid）模拟 verify 失败
# 期望：unit 文件被 rm，install-service.sh exit 2，install.sh 上游报 "退出码 2"
```

**Windows 端**：

```powershell
# AC-2：PS 5.1 + PS 7.4 各跑一遍
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
# 期望：步骤 5 主机输出可见进度（progress bar 或百分比）

# AC-9：删 exe 试退出码透传
Remove-Item "C:\Program Files\frp-easy\frp-easy.exe"
& "C:\Program Files\frp-easy\scripts\install-service.ps1"
$LASTEXITCODE  # 期望 1（缺二进制）
```

**verify_all 回归**（AC-11）：

```bash
./scripts/verify_all.sh
# 期望最末行 PASS:19（与当前 main HEAD 一致）
```

### D.5 insight-index 更新建议（阶段 7 落地文本）

**第 18 行当前文本**（即将被替换）：

```markdown
- **2026-05-19** · systemd unit 中 `ExecStart=${PATH}` 与 `WorkingDirectory=${PATH}` 必须用双引号包路径（`ExecStart="${PATH}"`，systemd 5.0+ 语法），否则路径含空格时 systemd 按空格分参导致启动失败。Code Review MAJOR-1 直接证据 · evidence: T-008 deploy-kit
```

**预拟替换文本**（阶段 7 写入 07_DELIVERY.md `## Insight` 段；保留同一行号，append-only 视角下"先归档老 insight 到 docs/features/_archived/insight-history.md 再追加"——但本任务是**修正**事实，应当**原地替换**第 18 行）：

```markdown
- **2026-05-23** · systemd unit 中 `ExecStart=` 与 `WorkingDirectory=` 路径必须裸 token 写入（`ExecStart=/opt/frp-easy/frp-easy`），含空格路径用 C-style 转义 `\x20`（`/opt/frp\x20easy/v1`）；**整体双引号写法** `WorkingDirectory="/path"` 被 systemd 任意版本拒为 `bad unit file setting`（T-008 旧 insight 错误已纠正）。systemd-analyze verify 是写出后 daemon-reload 前的必备自检 · evidence: T-016 install-progress-and-systemd-unit-fix
```

**为什么"原地替换"而非"追加新行 + 保留老行"**：
- 老 insight 是**反例**，留着会误导新任务（违反 insight-index 的"用证据击败先验"精神）。
- `.harness/rules/05-insight-index.md` 的"≤30 行 append-only"约束在"事实纠错"场景下有例外余地：错误事实留在历史档案 `docs/features/_archived/insight-history.md` 即可，主索引保持正确。
- 阶段 7 `scripts/archive-task` 行为：把 07_DELIVERY 的 `## Insight` 行追加到 `.harness/insight-index.md`；若超 30 行才轮转老的。本任务建议 Developer 在 07_DELIVERY 写一段**明确指令**：

  ```markdown
  ## Insight

  本任务修正 .harness/insight-index.md 第 18 行的错误事实。请 archive-task 之后手动：
  1. 把第 18 行原文搬到 docs/features/_archived/insight-history.md 顶部，前缀 "[CORRECTED by T-016]"
  2. 把下面这条新事实写入第 18 行（替换，不是追加）：
  - **2026-05-23** · <上面预拟的替换文本，去掉 markdown bullet 前缀让 archive-task 直接 append 时格式一致>
  ```

  archive-task 的具体语义见 `.harness/rules/05-insight-index.md` 与归档脚本本身；如脚本不支持"原地替换"模式，Developer 在 04_DEVELOPMENT 阶段直接手动编辑 `.harness/insight-index.md`（属允许范围：05-insight-index 规则允许任务结束时**编辑**该索引文件，并非纯 append-only）。

---

## 12. Verdict

**READY**

理由：

1. 上游 01 RA verdict = READY；本设计严格在 §8 PM 默认决策框架内做技术选型，未偏离也未新增需求。
2. 三组改动（A 进度 / B unit 语法 / C 退出码）的每一条 FR 都有对应的具体修改点 + 引用证据 + 测试用例。
3. 复用审计无新模块、无新依赖（NFR-5 满足）。
4. 8 条风险全部带缓解；3 条核心风险（R-2 / R-4 / R-1）已分别在落地约束、Code Review 检查、降级路径上设防。
5. 分区分配虽然 scripts/install*.* 严格不在三个 dev-* owned paths 内，但 §11 已给出明确的"语义就近 dev-backend + Developer 在 04 显式说明"机制，避免 Code Reviewer 阶段 5 的 `BLOCKED ON PARTITION` 误报。
6. 13 条 AC 全部映射到具体可观察命令（§D.4），QA 可独立执行。
7. 未发现 RA 文档需要返工的缺口。

下游交接：Gate Reviewer 阶段 3 复核本设计是否覆盖所有 AC，通过后 PM 派 `dev-backend` 实施。
