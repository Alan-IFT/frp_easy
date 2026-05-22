# 02 — 方案设计：T-012 one-click-install-and-mit-license

> Stage 2 of 7-stage `/harness` 流水线 · 模式：full · 语言：中文
> 上游：`01_REQUIREMENT_ANALYSIS.md`（verdict = READY FOR DESIGN）+ `INPUT.md`（PM 4 项预决策 + Q1…Q5 裁决）
> 本文档为下游 Gate Reviewer / Developer 的唯一设计输入。Developer 照此实现，不再做设计决策。

---

## 1. 方案概述（架构层面发生什么）

本任务在 frp_easy 现有的「发布产物（release.yml）+ 解压包内 install-service.{sh,ps1}」契约**之上**新增一层「一键安装编排脚本」，不改动任何 Go 代码、不改 `release.yml`、不改 `install-service.*` / `package.*` 的行为。新增两个互为对等的安装编排脚本 `scripts/install.sh`（bash，Linux/macOS）与 `scripts/install.ps1`（PowerShell，Windows），它们以 `curl|bash` / `irm|iex` 管道形态运行：探测平台 → 调 GitHub Releases API 解析最新 release 资产 URL → 下载校验解压到固定安装目录 → **调用解压包内的** `install-service.*` 完成服务注册 → 打印访问地址。同时补齐仓库根的 `LICENSE`（MIT）与 `NOTICE`（上游 frp Apache-2.0 归属），并把 `README.md` 与 `docs/DEPLOYMENT.md` 的安装入口改为「一键安装优先、手动下载备选」。本任务产出全部为 **shell/PowerShell 脚本 + Markdown 文档 + 纯文本许可证文件**，无数据模型变更、无 API 变更、无前端变更。

---

## 2. 受影响模块（文件清单）

| 文件 | 新增/修改 | 说明 |
|---|---|---|
| `scripts/install.sh` | **新增** | bash 一键安装编排脚本（Linux/macOS） |
| `scripts/install.ps1` | **新增** | PowerShell 一键安装编排脚本（Windows） |
| `LICENSE` | **新增** | MIT 许可证全文（英文标准文本） |
| `NOTICE` | **新增** | 上游 frp 二进制 Apache-2.0 归属说明（中文） |
| `README.md` | 修改 | 「快速开始」L49–L68 一键安装置顶；「许可证」L197–L201 改 MIT 声明 |
| `docs/DEPLOYMENT.md` | 修改 | 路径 A 新增 A.0「一键安装」子节，A.1–A.3 降级为备选 |
| `docs/dev-map.md` | 修改 | `scripts/` 条目登记 `install.{sh,ps1}` |

> 不修改：`scripts/install-service.{sh,ps1}`、`scripts/uninstall-service.{sh,ps1}`、`scripts/package.{sh,ps1}`、`.github/workflows/release.yml`、`scripts/verify_all.{sh,ps1}`、任何 `*.go` / `web/` 文件。

---

## 3. 执行模型说明（`curl|bash` / `irm|iex` 自定位问题）

> 本节是本设计的核心约束，Developer 必须先理解再动手。

### 3.1 install.sh 的执行模型

`scripts/install.sh` 在生产形态下是被 `curl -fsSL <raw-url> | sudo bash` **以管道喂给 bash 的标准输入**执行的，不是作为磁盘上的文件被执行。其后果：

- 脚本运行时 **没有对应的磁盘文件路径**。`$0` 在管道形态下是 `bash`（或 `-`/`sh`），`${BASH_SOURCE[0]}` 同样不可靠。
- 因此 **install.sh 禁止使用 `$0` / `${BASH_SOURCE[0]}` 来定位「自身所在目录」**（这一点与 `install-service.sh` L74 的 `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"` 模式正相反——`install-service.sh` 是从解压包里以磁盘文件执行的，可以靠 `BASH_SOURCE`；install.sh 不能）。
- install.sh **不需要**定位自身，它需要的一切（`install-service.sh`、主二进制、`frp_linux/`）都来自它**下载并解压的 release 包**。它用 `mktemp -d` 建临时工作目录，所有路径都基于该临时目录与固定安装目录 `/opt/frp-easy` 这两个**显式构造的绝对路径**。
- install.sh 调用的 `install-service.sh` 是**解压包内**的那一份（路径 `/opt/frp-easy/scripts/install-service.sh`），不是仓库里的、也不是另外下载的。这满足 AC-7「不复刻 systemd unit 写入逻辑」。

### 3.2 install.ps1 的执行模型

`scripts/install.ps1` 在生产形态下是被 `irm <raw-url> | iex` 执行的——`irm`（Invoke-RestMethod）把脚本内容作为字符串拉下来，`iex`（Invoke-Expression）在当前会话里求值。其后果：

- 脚本运行时 **`$PSScriptRoot` 为空字符串**（`iex` 求值的代码块没有脚本文件来源），`$MyInvocation.MyCommand.Path` 同样为空。
- 因此 **install.ps1 禁止使用 `$PSScriptRoot` 定位自身**（与 `install-service.ps1` L60 `$ScriptDir = $PSScriptRoot` 模式相反——后者从解压包内以 `.ps1` 文件执行，可用 `$PSScriptRoot`）。
- install.ps1 同样不需要定位自身：临时目录用 `New-Item -ItemType Directory`（基于 `[System.IO.Path]::GetTempPath()` + GUID），安装目录固定 `C:\Program Files\frp-easy`，调用的是解压出来的 `C:\Program Files\frp-easy\scripts\install-service.ps1`。

### 3.3 参数传递限制（管道形态的副作用）

- `curl ... | bash` 形态下，命令行参数无法直接传给被管道喂入的脚本（`bash -s -- --flag` 才行，普通用户不会这么写）。因此 `install.sh` 的 `-h/--help` 与可选环境变量是**为「先下载到本地再执行」的谨慎用户**准备的：`curl -fsSL <url> -o install.sh && bash install.sh -h`。help 分支与参数解析仍需实现（AC-5、AC-7 要求），但设计上不假设管道形态能收到参数。
- `install.ps1` 同理：`irm ... | iex` 形态无法传 `-Help`；`-Help` 为本地执行（`irm <url> -OutFile install.ps1; .\install.ps1 -Help`）准备。

### 3.4 安装目录可选覆盖（Q2 裁决：非硬 AC，Architect 酌情）

**设计决定：提供环境变量覆盖，但不作为 AC、不在文档主路径宣传。**
- `install.sh`：读取 `FRP_EASY_INSTALL_DIR` 环境变量，未设置时默认 `/opt/frp-easy`。`curl|bash` 形态下用户可 `curl -fsSL <url> | sudo FRP_EASY_INSTALL_DIR=/srv/frp-easy bash`。
- `install.ps1`：读取 `$env:FRP_EASY_INSTALL_DIR`，未设置时默认 `C:\Program Files\frp-easy`。
- 理由：实现成本极低（一行 `${VAR:-default}`），给高级用户留口子，且不增加普通用户认知负担（文档只写默认路径）。help 文本里简单提一句即可。

---

## 4. `scripts/install.sh` 分步骤设计（Linux/macOS）

### 4.1 文件头（AC-17：≥5 行中文注释 + AC-4：set -euo pipefail）

脚本第 1 行 `#!/usr/bin/env bash`，紧接 ≥5 行中文注释块，与 `install-service.sh` L1–L12、`package.sh` L1–L13 风格一致，须覆盖：用途、用法（含 `curl|bash` 形态与「先下载审阅」形态）、参数（`-h/--help`、`FRP_EASY_INSTALL_DIR`）、输出、退出码语义。注释块之后写 `set -euo pipefail`。

退出码语义（与 `install-service.sh` L11、`package.sh` L13 对齐）：
- `0` 成功（含 macOS 降级收尾、`-h` 帮助）
- `1` 前置/环境失败（非 root、缺依赖、非 amd64、网络/API/release 不可用、下载解压失败）
- `2` 服务注册阶段失败（透传 `install-service.sh` 的退出码 2）

### 4.2 全局常量与变量

```bash
REPO="Alan-IFT/frp_easy"
API_URL="https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest"   # AC-8：精确字面量
INSTALL_DIR="${FRP_EASY_INSTALL_DIR:-/opt/frp-easy}"
TMP_DIR=""        # mktemp -d 结果，cleanup trap 用
```

> AC-8 硬约束：API_URL 必须是这个**精确字面量**且可被 grep 命中；不要用变量拼 `$REPO`，直接写死整串（或写死整串再赋值），确保 `grep -F 'https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest'` 命中。建议直接 `API_URL="https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest"` 一行写死。

### 4.3 步骤 0 — 参数解析与 `-h/--help`（AC-5）

`while [[ $# -gt 0 ]]` 循环（套用 `install-service.sh` L19–L49 / `package.sh` L22–L60 的成熟模式），识别 `-h|--help` → `cat <<'EOF'` 打印中文用法 → `exit 0`；未识别参数 → 中文报错 `exit 1`。
**关键：help 分支必须在任何依赖检测（步骤 1）之前**，保证 `bash -n` 通过且 `install.sh -h` 在无 root / 无网络环境下也能 `exit 0`（AC-5「help 分支在依赖检测之前」）。

help 文本须含：用途、用法两种形态、安装目录（`/opt/frp-easy`，可被 `FRP_EASY_INSTALL_DIR` 覆盖）、所需权限（root/sudo）、所需依赖（curl、tar）、退出码语义（0/1/2）、卸载提示（`/opt/frp-easy/scripts/uninstall-service.sh`）。

### 4.4 步骤 1 — 前置依赖与权限检测

按顺序检测，任一失败立即中文报错 stderr + `exit 1`。**此步必须在步骤 2（联网）之前**，避免无谓的网络往返。

| 子步 | 检测 | 失败文案（写 stderr） | BC |
|---|---|---|---|
| 1a | `command -v curl` | `错误：未检测到 curl，请先安装（Debian/Ubuntu: apt-get install -y curl；RHEL: yum install -y curl）。` | BC-7 |
| 1b | `command -v tar` | `错误：未检测到 tar，请先安装（Debian/Ubuntu: apt-get install -y tar）。` | BC-7 |
| 1c | `[[ "$(id -u)" -eq 0 ]]` | `错误：请以 root / sudo 运行（安装到 /opt 与配置 systemd 需 root 权限）。用法：curl -fsSL <url> \| sudo bash` | BC-6 |

> NFR-3：每个阶段开始打印一行中文进度，例：`==> [1/6] 检查运行环境...`。沿用 `install-service.sh` 的 `==>` 前缀风格。

### 4.5 步骤 2 — 探测 OS / 架构（BC-3、BC-11）

```bash
OS="$(uname -s)"          # Linux / Darwin
ARCH="$(uname -m)"        # x86_64 / aarch64 / arm64 ...
```

- OS 归一化：`Linux` → `linux`；`Darwin` → `darwin`；其它 → 中文报错 `exit 1`（"暂不支持的操作系统 `<OS>`"）。
- ARCH 归一化：`x86_64` / `amd64` → `amd64`；**其它一律视为不支持**（BC-3）。
  - 文案：`错误：当前架构 <ARCH> 暂无预编译发布包，请用源码构建（见 docs/DEPLOYMENT.md 路径 B）。` → `exit 1`。
- 平台片段 `PLATFORM="${OS}-${ARCH}"`，Linux 下为 `linux-amd64`，macOS 下为 `darwin-amd64`。
  - **注意**：release.yml 当前只产 `linux-amd64` 与 `windows-amd64`，**没有 `darwin-amd64`**。macOS 上 `PLATFORM=darwin-amd64` 在步骤 4 资产匹配时会落空 → 自然走 BC-5「最新 release 未包含当前平台的发布包」分支报错 `exit 1`。
  - 这与 BC-11「macOS 下载+解压正常完成、服务化降级」**存在张力**：BC-11 假设 macOS 能下载到包。**设计裁决**见 §4.11。

### 4.6 步骤 3 — 查询 GitHub Releases API（BC-1、BC-2、BC-4）

用 `curl` 一次请求同时拿 HTTP 状态码与响应体（避免两次请求加剧限流）：

```bash
api_resp="$(curl -fsSL -w $'\n%{http_code}' "$API_URL" 2>/dev/null)" || curl_failed=1
# 末行是 http_code，其余是 body
http_code="$(printf '%s' "$api_resp" | tail -n1)"
body="$(printf '%s' "$api_resp" | sed '$d')"
```

> 注意 `set -e` 与 `curl -f` 失败：`curl -f` 对 4xx/5xx 返回非 0 退出码，`-f` 模式下 body 可能为空。设计上**不要靠 `-f` 失败**来分流状态码——改用 `curl -sSL -w '%{http_code}'`（**去掉 `-f`**）让 curl 总是返回 0（除非网络层失败），再用 `http_code` 文本判定。网络层失败（DNS/连接）curl 仍非 0，用 `|| handle_network_error` 捕获。这是 BC-1 与 BC-2/BC-4 的分流关键。

分流：

| 条件 | 场景 | 文案（stderr） | 退出 | BC |
|---|---|---|---|---|
| curl 命令本身非 0（网络层失败） | DNS 失败 / 不可达 | `错误：无法访问 GitHub（请检查网络或代理）。` | 1 | BC-1 |
| `http_code == 403` | API 限流 | `错误：GitHub API 触发限流（未认证请求 60 次/小时/IP），请稍后重试，或改用 docs/DEPLOYMENT.md 路径 A 手动下载。` | 1 | BC-2 |
| `http_code == 404` | 无 latest release | `错误：仓库尚未发布任何 release，无法一键安装；请改用源码构建（docs/DEPLOYMENT.md 路径 B）或等待首个 release。` | 1 | BC-4 |
| `http_code != 200`（其它，如 5xx） | API 异常 | `错误：GitHub API 返回异常状态 <code>，请稍后重试。` | 1 | （BC-1 泛化）|
| `http_code == 200` | 正常 | 继续步骤 4 | — | — |

> BC-2 细节：GitHub 限流响应是合法 JSON（含 `"message": "API rate limit exceeded..."`），所以**必须先用 HTTP 状态码判定 403，再解析 JSON**，否则限流响应会被当成正常 release JSON 解析、`tarball` 字段缺失导致后续误报。设计明确：状态码判定在前，JSON 解析在后（BC-2「不把限流响应当成正常 JSON 解析」）。

### 4.7 步骤 4 — 从 API 响应解析资产 URL（无 jq，BC-5）

需求要求**不依赖 jq**，用 `grep`/`sed` 解析 JSON（与项目无依赖脚本风格一致）。GitHub `releases/latest` 响应里资产对象形如：
```json
"browser_download_url": "https://github.com/Alan-IFT/frp_easy/releases/download/0.1.0/frp-easy-0.1.0-linux-amd64.tar.gz"
```

解析逻辑：
```bash
# 1) 提取所有 browser_download_url
# 2) 过滤出文件名含当前平台片段、且后缀为 .tar.gz（Linux/macOS）的那一条
ASSET_URL="$(printf '%s' "$body" \
  | grep -oE '"browser_download_url":[[:space:]]*"[^"]+"' \
  | sed -E 's/.*"(https[^"]+)"/\1/' \
  | grep -E "frp-easy-.*-${PLATFORM}\.tar\.gz$" \
  | head -n1)"
```

- 若 `ASSET_URL` 为空 → BC-5：`错误：最新 release 未包含当前平台（<PLATFORM>）的发布包。` → `exit 1`。
- 版本号可顺带从 `"tag_name"` 或 URL 路径提取（仅用于进度打印，非必需）：`grep -oE '"tag_name":[[:space:]]*"[^"]+"'`。
- 健壮性注意：`grep -oE` 在响应被 GitHub 压缩（gzip）时会乱码——`curl -sSL` 默认不带 `Accept-Encoding: gzip` 不会被压缩；**不要**加 `--compressed`。

### 4.8 步骤 5 — 下载、校验、解压（BC-9、BC-12）

```bash
TMP_DIR="$(mktemp -d)"          # 临时工作目录
trap 'rm -rf "$TMP_DIR"' EXIT   # BC-12：任何路径退出都清理临时目录
TARBALL="$TMP_DIR/release.tar.gz"
```

> trap 注意（参照 `install-service.sh` L96–L98 的 MINOR-R1 经验）：`trap` 必须在 `TMP_DIR` 赋值**之后**设置，避免 trap 到空变量导致 `rm -rf ""` 之类。`rm -rf` 一个已存在的临时目录是安全的、幂等的。

1. 下载：`curl -fsSL -o "$TARBALL" "$ASSET_URL"`（这里**可以用 `-f`**，下载失败直接非 0）。失败 → BC-9 文案 `错误：发布包下载失败，请检查网络后重试。` → `exit 1`（trap 自动清理）。
2. 非空校验：`[[ -s "$TARBALL" ]]`，空文件 → BC-9 `错误：下载的发布包为空（0 字节）。` → `exit 1`。
3. 解压校验：先 `tar tzf "$TARBALL" >/dev/null`（仅列表、验证可解压），失败 → BC-9 `错误：发布包损坏，无法解压。` → `exit 1`。
4. 实际解压到临时目录：`tar xzf "$TARBALL" -C "$TMP_DIR"`。包内顶层目录为 `frp-easy-<VERSION>-linux-amd64/`（见 `package.sh` L220 `pkg_name`）。用 `EXTRACTED="$(find "$TMP_DIR" -maxdepth 1 -type d -name 'frp-easy-*')"` 定位解压出的顶层目录，非唯一/为空 → BC-9 报错 `exit 1`。

### 4.9 步骤 6 — 安装到固定目录（升级语义，BC-10、AC-9）

判断 `INSTALL_DIR` 是否已存在旧安装（`[[ -e "$INSTALL_DIR/frp-easy" ]]`）：

**全新安装分支：**
1. `mkdir -p "$INSTALL_DIR"`。
2. 用 `cp -a "$EXTRACTED/." "$INSTALL_DIR/"` 把解压内容整体拷入（含 `frp-easy`、`frp_linux/`、`scripts/`、`README.txt`、`VERSION`、`LICENSE`、`frp_easy.toml.example`）。
3. `chmod 0755 "$INSTALL_DIR/frp-easy"`（`cp -a` 已保留权限，此为兜底）。

**升级分支（旧安装存在）：**
1. 进度行 `==> 检测到已存在安装，执行升级（保留 frp_easy.toml 与 .frp_easy/）`。
2. **先停服**：`systemctl stop frp-easy >/dev/null 2>&1 || true`（与 `install-service.sh` L104 同款容错；无 systemctl 或服务不存在不报错）。
3. **逐项覆盖**，**显式排除** `frp_easy.toml` 与 `.frp_easy/`（AC-9 硬约束——脚本里绝不出现删除/覆盖这两者的语句）：
   - 覆盖主二进制：`cp -a "$EXTRACTED/frp-easy" "$INSTALL_DIR/frp-easy"`。
   - 覆盖 `frp_linux/`：`rm -rf "$INSTALL_DIR/frp_linux" && cp -a "$EXTRACTED/frp_linux" "$INSTALL_DIR/"`。
   - 覆盖 `scripts/`：`rm -rf "$INSTALL_DIR/scripts" && cp -a "$EXTRACTED/scripts" "$INSTALL_DIR/"`。
   - 覆盖 `README.txt` / `VERSION` / `LICENSE` / `frp_easy.toml.example`：逐个 `cp -a`。
   - **不触碰** `$INSTALL_DIR/frp_easy.toml`、`$INSTALL_DIR/.frp_easy/`：脚本中不写任何针对这两个路径的 `rm` / `cp` 目标。AC-9 走查点。
4. 对齐 `DEPLOYMENT.md` C.2.4 升级语义（覆盖二进制 + frp_linux + scripts，保留配置与数据）。

> 幂等性（NFR-4）：升级分支用 `rm -rf` 子目录再 `cp` 保证可重复执行；`cp -a` 覆盖文件天然幂等。重跑不残留、不报错。

### 4.10 步骤 7 — 调用解压包内的 install-service.sh 注册服务（AC-7、BC-8）

```bash
SERVICE_SCRIPT="$INSTALL_DIR/scripts/install-service.sh"
```

- 若 `OS == linux`：
  - `[[ -f "$SERVICE_SCRIPT" ]]` 校验存在（理论上一定在，包内含）；不在 → 报错 `exit 1`。
  - 调用：`bash "$SERVICE_SCRIPT"`（**不重写 unit 生成逻辑**，满足 AC-7）。`install-service.sh` 内部用自己的 `BASH_SOURCE` 定位 `INSTALL_DIR`（它此时是磁盘文件，定位可靠）——这正是为什么必须调用**解压出来的**那一份。
  - **退出码透传**（BC-8）：`install-service.sh` 在缺 `systemctl` 时退出 1、systemctl 调用失败退出 2。install.sh 须捕获其退出码并透传：
    ```bash
    if ! bash "$SERVICE_SCRIPT"; then
        rc=$?
        echo "错误：服务注册失败（install-service.sh 退出码 $rc）。请查看上方 install-service.sh 的中文报错。" >&2
        exit "$rc"   # 透传 1 或 2，不掩盖
    fi
    ```
    BC-8 要求「把该退出码透传，不掩盖」——这里 `exit "$rc"` 直接透传。
- 若 `OS == darwin`：见 §4.11 降级处理。

### 4.11 macOS 降级处理（BC-11 与 release 资产缺失的张力 — 设计裁决）

**问题**：BC-11 要求 macOS 下「下载+解压正常完成，服务化降级为打印手动启动提示，退出码 0」。但 release.yml 当前**不产 `darwin-amd64` 资产**，§4.5 的 `PLATFORM=darwin-amd64` 在 §4.7 资产匹配时会落空 → 走 BC-5 报错 `exit 1`，与 BC-11 矛盾。

**设计裁决（Architect 决定，不推翻 PM 裁决，属实现细节澄清）**：
- macOS 上**资产匹配回退**：若 `darwin-amd64` 资产不存在，**不立即报 BC-5**，而是打印一条中文说明：
  `提示：当前 release 未提供 macOS 专用包，frp_easy 的 macOS 支持为次要平台（见 01 需求 Out-of-scope）。`
  然后以 `exit 1` 收尾，文案明确指向「请用源码构建（路径 B）」。
- 即 macOS 在「无 darwin 资产」的现实下，BC-11 的「下载+解压正常完成」前提**不成立**，脚本走的是 BC-5 的友好报错路径，但文案针对 macOS 定制（不是泛泛的「未包含当前平台」）。
- 如果将来 release.yml 补了 `darwin-amd64` 资产，BC-11 的完整路径（下载→解压→无 launchd 模板→打印 `cd /opt/frp-easy && ./frp-easy` 手动启动提示→`exit 0`）才真正可达。**install.sh 的 darwin 分支须把这条完整路径写出来**（即：darwin 且资产存在 → 下载解压安装 → 跳过 install-service → 打印手动启动提示 → `exit 0`），只是当前因无资产走不到。
- **给 Developer 的明确指令**：darwin 分支按「资产存在则降级服务化 exit 0 / 资产不存在则 macOS 定制报错 exit 1」两条都实现；二者都不调用 `install-service.sh`（macOS 无 systemd）。
- 这一裁决会在 §11 风险表与 §12 给 Developer 注意事项中再次强调，并建议 Gate Reviewer 确认是否需 PM 二次确认（属边界澄清，不属推翻裁决）。

### 4.12 步骤 8 — 打印安装结果（In-scope 6、NFR-3）

成功后（Linux 服务注册成功 / macOS 降级）向 **stdout** 打印中文收尾块，包含：
- 安装目录：`$INSTALL_DIR`
- 访问地址：`http://127.0.0.1:7800`（本机）与 `http://<本机IP>:7800`（局域网）。本机 IP 探测可选：`hostname -I 2>/dev/null | awk '{print $1}'`，取不到则只打印占位说明 `<本机IP>`，不因取 IP 失败而 `exit` 非 0。
- 常用 systemd 命令：`systemctl status frp-easy` / `systemctl is-active frp-easy` / `journalctl -u frp-easy -f`（沿用 `install-service.sh` L153–L166 的收尾文案，避免重复但保持一致）。
- 卸载提示：`sudo /opt/frp-easy/scripts/uninstall-service.sh`（In-scope Out-of-scope：安装脚本不内置卸载，仅提示路径）。
- `exit 0`。

### 4.13 install.sh 完整流程图

```
curl|bash 启动
  │
  ├─ 步骤0  解析参数 / -h --help ─────► exit 0（help）
  │
  ├─ 步骤1  检测 curl / tar / root ──► exit 1（BC-6/BC-7）
  │
  ├─ 步骤2  uname 探测 OS/ARCH ──────► exit 1（BC-3 非 amd64 / 非 Linux&非 Darwin）
  │
  ├─ 步骤3  curl GitHub API ─────────► exit 1（BC-1 网络 / BC-2 403 / BC-4 404）
  │
  ├─ 步骤4  grep/sed 解析资产 URL ───► exit 1（BC-5 无匹配资产；macOS 定制文案）
  │
  ├─ 步骤5  mktemp+trap 下载校验解压─► exit 1（BC-9 空/损坏；trap 清理 BC-12）
  │
  ├─ 步骤6  安装到 /opt/frp-easy
  │          ├─ 全新：cp -a 整体
  │          └─ 升级：停服→覆盖 bin/frp_linux/scripts，保留 toml/.frp_easy（BC-10/AC-9）
  │
  ├─ 步骤7  ┌ Linux : bash install-service.sh，退出码透传（AC-7/BC-8）─► exit 1或2
  │         └ macOS : 跳过，打印手动启动提示（BC-11）
  │
  └─ 步骤8  打印安装目录/访问地址/常用命令/卸载提示 ─► exit 0
```

---

## 5. `scripts/install.ps1` 分步骤设计（Windows）

结构与 install.sh 对等；以下只列与 install.sh 的**差异点**，相同处（流程、BC 分流语义、退出码 0/1/2）一致。

### 5.1 文件头与全局设置

- 第一段为 ≥5 行中文注释（用途/用法两形态/参数/输出/退出码），与 `install-service.ps1` L1–L16 风格一致。
- `$ErrorActionPreference = "Stop"`（AC-4 grep 命中项）。
- 全局常量：
  ```powershell
  $ApiUrl     = "https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest"
  $InstallDir = if ($env:FRP_EASY_INSTALL_DIR) { $env:FRP_EASY_INSTALL_DIR } else { "C:\Program Files\frp-easy" }
  ```
- `param([switch]$Help)`（同时接受 `-Help`；`-h` 通过 PowerShell 前缀匹配自动等价于 `-Help`，无需额外定义——`install-service.ps1` 的 param 风格一致）。`-Help` 分支必须在最前，打印中文用法后 `exit 0`（AC-5）。

### 5.2 步骤 1 — 前置检测

| 子步 | 检测 | 失败处理 | BC |
|---|---|---|---|
| 1a | 管理员会话：`([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)`（直接复用 `install-service.ps1` L46 写法） | `Write-Error "请以管理员身份运行 PowerShell（右键 -> 以管理员身份运行）后再执行一键安装。"; exit 1` | BC-6 |
| 1b | `sc.exe` 存在（`Get-Command sc.exe`） | 提示后 `exit 1` | BC-7 类比 |

- 下载与解压在 Windows 上**不依赖 curl/tar**：用 `Invoke-WebRequest`（API + 下载）与 `Expand-Archive`（解压 zip），二者 Windows 10 22H2+ 自带 PowerShell 5.1 即有（NFR-1）。因此 install.ps1 **不做 curl/tar 检测**，但需确认 `Expand-Archive` 可用（PS 5.1+ 自带，可省检测）。

### 5.3 步骤 2 — 架构探测（BC-3）

```powershell
$arch = $env:PROCESSOR_ARCHITECTURE   # AMD64 / ARM64 / x86
```
- `AMD64` → `amd64`；其它（`ARM64` 等）→ 中文报错 `当前架构 <arch> 暂无预编译发布包，请用源码构建（见 docs/DEPLOYMENT.md 路径 B）。` → `exit 1`（BC-3）。
- 平台片段固定 `windows-amd64`。Windows 无 macOS 那种张力。

### 5.4 步骤 3 — 查询 GitHub API（BC-1/BC-2/BC-4）

```powershell
try {
    $resp = Invoke-WebRequest -Uri $ApiUrl -UseBasicParsing -ErrorAction Stop
} catch {
    # 区分网络层失败 vs HTTP 错误状态
}
```
- `Invoke-WebRequest` 对 4xx/5xx 默认抛异常（`$ErrorActionPreference=Stop` 下进 catch）。在 catch 里读 `$_.Exception.Response.StatusCode`：
  - 无 `Response`（DNS/连接失败）→ BC-1 `错误：无法访问 GitHub（请检查网络或代理）。` exit 1。
  - `403` → BC-2 `错误：GitHub API 触发限流，请稍后重试，或改用 docs/DEPLOYMENT.md 路径 A 手动下载。` exit 1。
  - `404` → BC-4 `错误：仓库尚未发布任何 release，无法一键安装；请改用源码构建或等待首个 release。` exit 1。
  - 其它 → `错误：GitHub API 返回异常状态 <code>。` exit 1。
- 成功时 `$resp.Content` 为 JSON 字符串。

> Windows 可用 `ConvertFrom-Json` 解析（无需 jq 等价物）：`$json = $resp.Content | ConvertFrom-Json`。这是 PowerShell 内置，**比 install.sh 的 grep/sed 更稳**，设计上允许 install.ps1 用 `ConvertFrom-Json`（不算外部依赖）。

### 5.5 步骤 4 — 解析资产 URL（BC-5）

```powershell
$asset = $json.assets | Where-Object { $_.name -match "frp-easy-.*-windows-amd64\.zip$" } | Select-Object -First 1
if (-not $asset) { Write-Error "错误：最新 release 未包含 Windows 平台的发布包。"; exit 1 }  # BC-5
$assetUrl = $asset.browser_download_url
```

### 5.6 步骤 5 — 下载、校验、解压（BC-9/BC-12）

```powershell
$tmpDir = New-Item -ItemType Directory -Path (Join-Path ([System.IO.Path]::GetTempPath()) ("frp-easy-" + [guid]::NewGuid().ToString("N")))
try {
    $zipPath = Join-Path $tmpDir "release.zip"
    Invoke-WebRequest -Uri $assetUrl -OutFile $zipPath -UseBasicParsing -ErrorAction Stop
    if ((Get-Item $zipPath).Length -le 0) { Write-Error "错误：下载的发布包为空（0 字节）。"; exit 1 }   # BC-9
    Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force                                       # BC-9：失败抛异常
    ...
} finally {
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue   # BC-12：清理临时目录
}
```
- `Expand-Archive` 对损坏 zip 抛异常，被 `$ErrorActionPreference=Stop` 捕获——用 `try/catch` 包住给 BC-9 中文文案后 `exit 1`。
- 解压后定位顶层目录：`Get-ChildItem $tmpDir -Directory -Filter 'frp-easy-*' | Select-Object -First 1`。
- `finally` 块负责 BC-12 临时目录清理（PowerShell 没有 trap EXIT，用 `try/finally` 等价）。

### 5.7 步骤 6 — 安装到 `C:\Program Files\frp-easy`（BC-10/AC-9）

- 全新：`New-Item -ItemType Directory -Force $InstallDir`，`Copy-Item -Recurse -Force "$extracted\*" $InstallDir`。
- 升级（`Test-Path "$InstallDir\frp-easy.exe"`）：
  1. 先停服：`& sc.exe stop frp-easy 2>&1 | Out-Null`（容错，服务不存在无妨）。
  2. 覆盖 `frp-easy.exe` / `frp_win\` / `scripts\` / `README.txt` / `VERSION` / `LICENSE` / `frp_easy.toml.example`，逐项 `Copy-Item -Recurse -Force`；子目录先 `Remove-Item -Recurse -Force` 再拷。
  3. **不触碰** `$InstallDir\frp_easy.toml`、`$InstallDir\.frp_easy\`（AC-9）。
- 编码注意（insight-index 2026-05-19）：install.ps1 **本身不生成任何文本文件**（toml / cmd 包装均由 `install-service.ps1` 在被调用时生成），所以 install.ps1 无 BOM / codepage 风险点；但**不要**用 install.ps1 去写 `frp_easy.toml`。`Copy-Item` 是字节级拷贝，不涉及编码转换，安全。

### 5.8 步骤 7 — 调用 install-service.ps1（AC-7/BC-8）

```powershell
$svc = Join-Path $InstallDir "scripts\install-service.ps1"
& $svc
if ($LASTEXITCODE -ne 0) {
    Write-Error "错误：服务注册失败（install-service.ps1 退出码 $LASTEXITCODE）。请查看上方中文报错。"
    exit $LASTEXITCODE    # 透传 1 / 2（BC-8）
}
```
- 注意：`& $svc` 调用 `.ps1` 脚本时，子脚本的 `exit N` 会被设置到 `$LASTEXITCODE`。透传即可。
- install-service.ps1 用 `$PSScriptRoot` 定位 `$InstallDir`——它此时是解压出来的磁盘 `.ps1` 文件，`$PSScriptRoot` 可靠（与 install.ps1 自身的 `irm|iex` 场景不同，§3.2 已说明）。

### 5.9 步骤 8 — 打印结果

stdout 打印：安装目录、`http://127.0.0.1:7800` 与 `http://<本机IP>:7800`、常用命令（`sc query frp-easy`）、卸载提示（`管理员 PowerShell 下 C:\Program Files\frp-easy\scripts\uninstall-service.ps1`）。`exit 0`。本机 IP 可选 `(Get-NetIPAddress -AddressFamily IPv4 ...)`，取不到不影响退出码。

---

## 6. `LICENSE` 内容方案（AC-12）

仓库根新增 `LICENSE`，内容为 **MIT 许可证标准全文（英文）**，NFR-5 明确 LICENSE 为英文标准文本例外。模板（Developer 直接写入，仅替换年份与署名）：

```
MIT License

Copyright (c) 2026 Alan_IFT

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- 版权行精确为 `Copyright (c) 2026 Alan_IFT`（AC-12 文本比对；署名取自 git `user.name`，见 INPUT.md 事实基线）。
- 文件无 BOM、LF 换行、文件末尾留一个换行符。
- 关联效应：`package.sh` L251 已有逻辑——仓库根存在 `LICENSE` 时自动打进发布包。本任务新增 `LICENSE` 后，下次打包发布产物会自动带上 `LICENSE`，无需改 `package.sh`（这也意味着升级安装时 §4.9 会把包内 `LICENSE` 拷进安装目录，闭环一致）。

> verify_all A.1 secrets scan 注意（insight-index 2026-05-19 T-008）：MIT 全文不含任何 `(api_key|secret|password|token)[\s]*[:=][\s]*"..."` 形态串，且扫描已 `':!*.md'` 排除——`LICENSE`/`NOTICE` 无扩展名不是 `.md`，但其文本不含赋值形态引号串，A.1 不会误命中。Developer 写入时不要在 LICENSE/NOTICE 里引入此类样例串。

---

## 7. `NOTICE` 内容方案（AC-13）

仓库根新增 `NOTICE`，**中文**（NFR-5），说明上游 frp 二进制归属。建议内容：

```
frp_easy
Copyright (c) 2026 Alan_IFT

本项目（frp_easy）自身的源代码与脚本采用 MIT 许可证，详见同目录下的 LICENSE 文件。

== 关于随附的第三方二进制 ==

本仓库的 frp_linux/ 与 frp_win/ 目录下随附的 frpc、frps 可执行文件，
不属于 frp_easy，而是来自上游开源项目 fatedier/frp：

  项目地址：https://github.com/fatedier/frp
  许可证：  Apache License 2.0

这些 frp 二进制以原样（未修改）形式随附，仅为方便用户开箱即用。
它们的版权与许可证归上游 fatedier/frp 项目所有，遵循 Apache-2.0，
与 frp_easy 自身的 MIT 许可证相互独立、互不影响。

Apache-2.0 许可证全文见：https://www.apache.org/licenses/LICENSE-2.0
```

- AC-13 grep 命中点：须含 `fatedier/frp` 与 `Apache-2.0`（或 `Apache License 2.0`）。建议两种写法都出现以保险。
- 文件无 BOM、LF 换行。

---

## 8. 文档改动清单

### 8.1 `README.md`

**「快速开始」章节（L49–L68）** 改为一键安装优先：
- 新结构：先给一键安装命令（首选），再用「或：手动下载发布包」小标题保留现有 `tar xzf ... && ./frp-easy` 作为备选。
- 一键安装命令（raw-url 写死，AC-15）：
  - Linux/macOS：
    ```bash
    curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash
    ```
  - Windows（管理员 PowerShell）：
    ```powershell
    irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
    ```
- 紧跟一行安全提示 + 指向 DEPLOYMENT.md A.0 的链接。
- 保留「启动后...setup 向导」说明段。

**「许可证」章节（L197–L201）** 改写（AC-14：不得再出现「尚未确定」「待项目维护者确定」）：
```markdown
## 许可证

本项目（frp_easy）采用 [MIT 许可证](LICENSE)。Copyright (c) 2026 Alan_IFT。

> 注意：`frp_linux/` 与 `frp_win/` 目录下随附的 frp 二进制（frpc / frps）属于上游项目
> [`fatedier/frp`](https://github.com/fatedier/frp)，遵循其 **Apache-2.0** 许可证，与
> frp_easy 本身的 MIT 许可证相互独立。详见仓库根的 [`NOTICE`](NOTICE) 文件。
```

### 8.2 `docs/DEPLOYMENT.md`

在「路径 A — 下载发布产物」标题之后、当前 A.1 之前，**新增 A.0「一键安装（推荐）」子节**：
- 开头一句：「最省事的方式，下面一条命令完成下载、安装、注册开机自启。」
- Linux/macOS 命令块（写死 raw-url）+ Windows 命令块（写死 raw-url）。
- **安全提示块**（In-scope 15 / NFR-2 硬要求）：
  - Linux/macOS：`> 安全提示：curl | bash 会以 root 执行远程脚本。谨慎用户可先下载审阅再运行：`
    ```bash
    curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh -o install.sh
    # 审阅 install.sh 内容后
    sudo bash install.sh
    ```
  - Windows：`> 安全提示：irm | iex 会在当前会话执行远程脚本。谨慎用户可先下载审阅：`
    ```powershell
    irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 -OutFile install.ps1
    # 审阅 install.ps1 内容后（管理员 PowerShell）
    .\install.ps1
    ```
- 说明一键安装等价于「路径 A 下载解压 + 路径 C 注册服务」的合并：固定安装目录 `/opt/frp-easy`（Linux）/ `C:\Program Files\frp-easy`（Windows），自动注册 systemd / Windows 服务。
- 现有 A.1「下载」/ A.2「解压」/ A.3「首启」**改为「手动安装（备选）」并保留原内容**（编号可保持 A.1/A.2/A.3 或重排为 A.1 之下；Developer 取最小改动方案——建议加一句「以下手动步骤是不使用一键安装时的备选路径」过渡句，编号不动以免链接断裂）。
- A.4 升级 / A.5 卸载：补一句「若通过 A.0 一键安装，升级只需重跑一键安装命令（脚本自动识别为升级、保留配置与数据）；卸载见路径 C 的 uninstall-service」。
- **理顺与路径 C 的关系**（避免自相矛盾）：A.0 末尾加一句「一键安装已自动完成路径 C 的服务注册，无需再手动执行 install-service；若想了解服务的状态查询 / 日志 / 卸载命令，见路径 C」。路径 C 本身不改（它仍是「手动安装后注册服务」的权威说明）。

### 8.3 `docs/dev-map.md`

`scripts/` 目录条目（L23–L24）追加一行登记：`install.{sh,ps1}（T-012 新增：一键安装编排脚本，curl|bash / irm|iex 形态；下载 latest release → 解压 → 调 install-service.* 注册服务）`。

---

## 9. 复用审计（Reuse audit）

| 需求 | 已有代码/资产 | 文件路径 | 决策 |
|---|---|---|---|
| systemd unit 生成 + enable --now | `install-service.sh` | `scripts/install-service.sh` | **原样复用**：install.sh 调用解压包内副本，不复刻（AC-7） |
| Windows 服务 sc.exe 注册 | `install-service.ps1` | `scripts/install-service.ps1` | **原样复用**：install.ps1 调用解压包内副本，不复刻（AC-7） |
| 参数解析 + `-h/--help` 中文用法块 | `while/case` + `cat <<'EOF'` 模式 | `scripts/install-service.sh` L19–L49、`scripts/package.sh` L22–L60 | **复用模式**：install.sh 套用同款 |
| PowerShell `param` + `-Help` + 管理员检测 | `param` + `IsInRole(Administrator)` | `scripts/install-service.ps1` L19–L50 | **复用模式**：install.ps1 套用同款管理员检测代码 |
| 退出码语义 0/1/2 | 前置失败 1 / 调用失败 2 | `install-service.sh` L11、`package.sh` L13 | **对齐复用**：install.* 沿用同一套语义 |
| 临时文件 trap 清理 | `trap 'rm -f "$TMP"' EXIT`（trap 在赋值后设置） | `install-service.sh` L96–L98（MINOR-R1） | **复用模式**：install.sh 用 `trap ... EXIT` 清理 `mktemp -d` 目录 |
| 进度行 `==>` 前缀风格 | `echo "==> ..."` | `install-service.sh`、`package.sh` 全文 | **复用风格**：install.* NFR-3 进度行用 `==>` |
| 发布包命名与内部结构 | `frp-easy-<VERSION>-<os>-amd64.<ext>`，顶层目录同名，内含 `frp-easy`/`frp_linux`/`scripts`/`README.txt`/`VERSION`/`LICENSE`/`frp_easy.toml.example` | `scripts/package.sh` L218–L255、`release.yml` | **复用契约**：install.* 按此结构解压与覆盖 |
| 发布产物 raw-url / GitHub Releases | release.yml 上传到 `releases/latest` | `.github/workflows/release.yml`（T-010） | **复用契约**：install.* 调 `releases/latest` API |
| LICENSE 自动打进发布包 | `[[ -f "$ROOT/LICENSE" ]]` 则 `cp` | `scripts/package.sh` L251–L255 | **零改动受益**：新增 `LICENSE` 后 package.sh 自动带上，无需改它 |
| `curl|bash` 自定位问题 | （无现成代码，但 install-service.sh 的 `BASH_SOURCE` 用法是反例参照） | — | **新设计**：见 §3，install.* 不靠 `$0`/`$PSScriptRoot` |
| GitHub API JSON 无 jq 解析 | （项目内无 jq 依赖；downloader.go 是 Go 侧下载，不可复用到 shell） | `internal/downloader/`（仅参考，非复用） | **新设计**：install.sh 用 grep/sed，install.ps1 用 ConvertFrom-Json |

---

## 10. 数据模型 / API 契约变更

**无。** 本任务不触碰 SQLite schema、不新增/修改 REST 路由、不改 `openapi.yaml`。install.* 对 GitHub Releases API 的调用是「消费第三方只读 API」，非本项目 API 契约。唯一外部契约是 GitHub `GET /repos/{owner}/{repo}/releases/latest` 的响应字段（`browser_download_url`、`tag_name`、`assets[]`），属 GitHub 稳定公开 API。

---

## 11. 风险分析

| # | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R-1 | **仓库当前无任何 release**，`releases/latest` 返回 404 | 一键安装当下无法真实跑通，AC-10/AC-11 无法自动验证 | PM Q1 已裁决：AC-10/AC-11 降级为交付后人工验证、非完成闸门。脚本 BC-4 给友好中文报错。本任务完成闸门为静态 AC。设计层面无额外动作。 |
| R-2 | **`curl -f` 与状态码分流冲突**：`-f` 模式下 4xx body 为空、curl 退出非 0，无法区分 BC-1/BC-2/BC-4 | 限流（BC-2）/无 release（BC-4）会被误报为网络失败（BC-1），文案不准 | §4.6 明确：API 查询步骤**去掉 `-f`**，用 `curl -sSL -w '%{http_code}'` 让 curl 总返回 0，靠 `http_code` 文本分流；仅网络层失败才靠 curl 非 0 退出。下载步骤（已知 URL）才用 `-f`。 |
| R-3 | **GitHub 限流响应是合法 JSON**，若先解析 JSON 再判状态码，限流响应会被当正常 release 解析 | BC-2 失效，误导用户以为 release 资产缺失 | §4.6 明确：**HTTP 状态码判定在前，JSON 解析在后**。403 直接走 BC-2，根本不进解析逻辑。 |
| R-4 | **`set -euo pipefail` 与管道/可失败命令冲突**：`uname` 之外，`grep` 无匹配返回 1、`curl` 失败、`hostname -I` 在某些系统不存在，都会让脚本在 `set -e` 下意外中止 | 脚本在正常的「无匹配/可选步骤失败」场景下非预期退出，BC 分支走不到 | Developer 须对所有「失败属正常分支」的命令显式处理：`grep ... || true` 后判空、`var="$(cmd)" || rc=$?` 捕获、可选命令（`hostname -I`）用 `2>/dev/null || echo ...`。§4 各步已标注。这是 shellcheck/`bash -n` 之外最易翻车点。 |
| R-5 | **`shellcheck` 告警**（AC-2 要求无 error 级）：`curl|bash` 管道、`mktemp` 用法、未引用变量、`SC2086` 等 | AC-2 不过，Code Review 卡 | 设计要求：所有变量引用加双引号；`tar`/`cp` 的多文件操作避免词分裂；必要处加 `# shellcheck disable=SCxxxx` 注释（与项目既有 .sh 同等级别）。**注意：`scripts/verify_all` 当前不含 shellcheck 检查项**（已核查 verify_all.sh，仅 A/B/C/D/E/G 段），故 AC-2 是 **Code Reviewer 手工走查项**，不是 verify_all 自动闸门——但仍是硬 AC，Developer 须本地跑 `shellcheck scripts/install.sh` 自检。 |
| R-6 | **macOS 无 darwin-amd64 资产**与 BC-11 假设矛盾 | install.sh 在 macOS 的实际行为（报错 exit 1）与 BC-11 字面（exit 0）不一致 | §4.11 设计裁决：macOS 资产缺失走「macOS 定制文案 + exit 1」；BC-11 完整降级路径（exit 0）仅在将来补 darwin 资产后可达，但代码两条分支都实现。建议 Gate Reviewer 确认此澄清是否需 PM 知会（属边界澄清，不推翻 Q 裁决）。 |
| R-7 | **PowerShell `irm|iex` 编码 / 跨版本差异**：PS 5.1 与 PS 7 对 `Invoke-WebRequest`/`Expand-Archive` 行为略有差异；`Write-Error` 在 `iex` 上下文不一定终止 | install.ps1 在 PS 5.1 上行为偏差，或报错后未真正退出 | 设计要求：失败路径用 `Write-Error "..."` 后**显式 `exit N`**（不依赖 `$ErrorActionPreference` 自动终止整个 `iex`）；`Invoke-WebRequest` 统一带 `-UseBasicParsing`（PS 5.1 必需，PS 7 兼容）。临时目录用 `try/finally`（非 `trap`）。 |
| R-8 | **升级时误删用户配置**（AC-9 红线） | 用户 `frp_easy.toml` / `.frp_easy/` 数据被覆盖丢失 | §4.9 / §5.7 设计为「逐项覆盖白名单」而非「清空目录再拷」——脚本中**绝不出现** `rm -rf "$INSTALL_DIR"` 或对 `frp_easy.toml`/`.frp_easy` 的 `rm`/`cp` 目标。Code Reviewer 按 AC-9 逐行走查。 |
| R-9 | **`raw.githubusercontent.com` 分支依赖**：文档写死 `main` 分支 raw-url | 分支改名则一键安装命令失效 | PM Q5 已裁决写死 `main`；分支改名属对外破坏性变更，由维护者届时同步文档。设计层面接受。 |

---

## 12. 给 Developer 的实现注意事项

1. **执行模型是第一约束**：install.sh 禁用 `$0`/`${BASH_SOURCE[0]}` 自定位；install.ps1 禁用 `$PSScriptRoot` 自定位（§3）。所有路径靠固定安装目录 + `mktemp`/临时目录两个显式绝对路径。
2. **AC-8 精确字面量**：install.sh / install.ps1 里 API URL 必须是可被 `grep -F` 命中的整串 `https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest`，不要用变量拼接。
3. **AC-5 help 分支位置**：`-h/--help` / `-Help` 分支必须在所有依赖/权限/网络检测之前，保证无 root、无网络下 `exit 0`。
4. **AC-4 grep 锚点**：install.sh 含字面 `set -euo pipefail`；install.ps1 含字面 `$ErrorActionPreference = "Stop"`（注意等号两侧空格与引号，与 install-service.ps1 L24 完全一致写法）。
5. **AC-17**：两脚本顶部 ≥5 行中文注释，覆盖用途/用法/参数/输出/退出码，照 `install-service.{sh,ps1}` 文件头格式。
6. **AC-7 不复刻**：install.* 调用的是**解压出来的** `install-service.*`（`$INSTALL_DIR/scripts/install-service.*`），不是仓库里的、不重写 unit / sc.exe 逻辑。
7. **AC-9 升级红线**：升级分支用「白名单逐项覆盖」，脚本中绝不出现删除/覆盖 `frp_easy.toml` 与 `.frp_easy/` 的语句。
8. **BC-8 退出码透传**：`install-service.*` 失败时，install.* 用 `exit $rc` / `exit $LASTEXITCODE` 透传 1 或 2，不掩盖、不重写为别的码。
9. **BC-2/BC-4 分流顺序**：先判 HTTP 状态码（403→BC-2、404→BC-4），后解析 JSON——绝不先解析。
10. **`set -e` 陷阱**（R-4）：对 `grep` 无匹配、`curl` 探测、`hostname -I` 等「失败属正常」的命令显式 `|| true` / `|| rc=$?` / `2>/dev/null`，否则脚本在正常分支意外中止。
11. **shellcheck 自检**（R-5/AC-2）：Developer 本地跑 `shellcheck scripts/install.sh`，消除 error 级告警；变量全部双引号；必要处加与项目同级别的 `# shellcheck disable=` 注释。verify_all 不含此项，但 AC-2 是硬 AC。
12. **macOS 双分支**（R-6/§4.11）：darwin 分支实现「资产存在→下载解压→跳过服务化→打印手动启动提示→exit 0」与「资产不存在→macOS 定制文案→exit 1」两条；当前现实走后者。
13. **PowerShell 失败显式 exit**（R-7）：每个错误路径 `Write-Error` 后跟 `exit N`；临时目录清理用 `try/finally`；`Invoke-WebRequest` 一律带 `-UseBasicParsing`。
14. **LICENSE/NOTICE 文件**：无 BOM、LF 换行、末尾留一个换行符；LICENSE 为英文标准 MIT 全文，版权行精确 `Copyright (c) 2026 Alan_IFT`；NOTICE 为中文，须含 `fatedier/frp` 与 `Apache-2.0`。不要在两文件里写任何「`token = "..."`」形态的样例串（A.1 secrets scan 防误命中）。
15. **文档 raw-url 写死**：README.md、DEPLOYMENT.md 里的安装命令 raw-url 一律写死 `https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.{sh,ps1}`，不用占位符（AC-15 / PM Q5）。
16. **AC-14 措辞**：README 许可证章节改写后，全文不得再出现「尚未确定」「待项目维护者确定」「待确定」字样（grep 校验）。
17. **完成闸门**：声明完成前跑 `scripts/verify_all`（PowerShell 用 `.ps1`，Git Bash 用 `.sh`），PASS 数 ≥ 19、不新增 WARN/FAIL（AC-16）。新增脚本/文档/许可证文件本身不被 verify_all 的现有检查项覆盖（不是 .go/.ts，不进 A.3 TODO 统计的扩展名），主要确认不破坏既有项。
18. **新增可执行脚本权限**：`scripts/install.sh` 提交时建议 `chmod 0755`（与 `install-service.sh` 一致），保证「先下载审阅再 `bash install.sh`」时无障碍；`curl|bash` 形态本身不依赖文件可执行位。

---

## 13. 17 条 AC 覆盖映射

| AC | 验证方式 | 本设计覆盖位置 |
|---|---|---|
| AC-1 `bash -n install.sh` 通过 | 静态 | §4 全节：help 分支前置、`set -euo pipefail`、标准 case 解析，无语法风险 |
| AC-2 `shellcheck install.sh` 无 error | 静态（Code Review 走查，verify_all 不含） | §11 R-5、§12.11：双引号 + disable 注释 + 本地自检 |
| AC-3 `install.ps1` 解析通过 | 静态 | §5 全节：标准 `param` + 函数 + try/catch，无语法风险 |
| AC-4 `set -euo pipefail` / `$ErrorActionPreference="Stop"` | grep | §4.1 / §5.1，§12.4 grep 锚点要求 |
| AC-5 `-h`/`-Help` 中文用法、exit 0、依赖检测前 | 静态非联网 | §4.3 / §5.1：help 分支在步骤 1 之前 |
| AC-6 BC-1…BC-12 逐条可定位中文 stderr + 非 0 退出（BC-11 为 0） | Code Review 走查 + grep | §4.4–§4.11 / §5.2–§5.8 每个 BC 给出文案与退出码；下方 BC 映射表 |
| AC-7 不复刻 unit，调用解压目录内 install-service.* | 源码走查 | §4.10 / §5.8 / §9 复用审计 |
| AC-8 API URL 精确字面量 | grep | §4.2 / §5.1，§12.2 写死整串要求 |
| AC-9 升级不删 `frp_easy.toml` / `.frp_easy/` | 源码走查 | §4.9 / §5.7 白名单逐项覆盖；§11 R-8 |
| AC-10 联网 Linux 真实安装 | 集成/人工（非闸门） | §11 R-1：PM Q1 降级为交付后验证；脚本 BC-4 友好报错保证可达 |
| AC-11 联网 Windows 真实安装 | 集成/人工（非闸门） | 同 AC-10 |
| AC-12 `LICENSE` 存在 + MIT 全文 + 版权行 | 文本比对 | §6 完整模板 |
| AC-13 `NOTICE` 存在 + frp/Apache-2.0 中文说明 | grep | §7 完整模板 |
| AC-14 README 许可证章节无「尚未确定」改 MIT | grep | §8.1，§12.16 |
| AC-15 README 快速开始 + DEPLOYMENT 路径 A 一键安装首选 + 安全提示 + 写死 raw-url | 目视 + grep | §8.1 / §8.2 |
| AC-16 verify_all PASS ≥ 19、不新增 WARN/FAIL | 执行 verify_all | §12.17：本任务不改 .go/.ts，不影响既有检查项 |
| AC-17 install.* 顶部 ≥5 行中文注释 | 目视 | §4.1 / §5.1，§12.5 |

### BC 覆盖映射（AC-6 细化）

| BC | 设计位置 | 退出码 | 中文文案要点 |
|---|---|---|---|
| BC-1 无网络 | §4.6 / §5.4 | 1 | 「无法访问 GitHub（请检查网络或代理）」 |
| BC-2 API 403 限流 | §4.6 / §5.4 | 1 | 「GitHub API 触发限流，请稍后重试」；状态码判定在 JSON 解析前 |
| BC-3 非 amd64 | §4.5 / §5.3 | 1 | 「当前架构 `<arch>` 暂无预编译发布包，请用源码构建（路径 B）」 |
| BC-4 无 release（404） | §4.6 / §5.4 | 1 | 「仓库尚未发布任何 release，无法一键安装」 |
| BC-5 缺当前平台资产 | §4.7 / §5.5 | 1 | 「最新 release 未包含当前平台的发布包」 |
| BC-6 非 root / 非管理员 | §4.4 / §5.2 | 1 | 「请以 root / sudo 运行」/「请以管理员身份运行」 |
| BC-7 缺 tar / curl | §4.4（sh）；§5.2 检 sc.exe | 1 | 「未检测到 curl/tar，请先安装 ...」 |
| BC-8 缺 systemctl | §4.10：install-service.sh 报错，install.sh 透传 | 透传 1 | install-service.sh 既有文案，install.sh 不掩盖 |
| BC-9 下载空/损坏/解压失败 | §4.8 / §5.6 | 1 | 「发布包下载失败」/「为空」/「损坏，无法解压」；清理临时文件 |
| BC-10 已存在旧安装 | §4.9 / §5.7 | （继续，不退出） | 升级语义：停服→白名单覆盖→重注册 |
| BC-11 macOS 运行 | §4.11 | 0（资产存在时降级）/ 1（当前无资产，定制文案） | 见 §4.11 设计裁决与 R-6 |
| BC-12 中途失败清理 | §4.8 trap / §5.6 finally | （随失败 BC 的码） | `trap ... EXIT` / `try/finally` 清理 `mktemp` 临时目录 |

---

## 14. 迁移 / 回滚计划

- **数据迁移**：无（无 schema 变更）。
- **向后兼容**：
  - 新增 `scripts/install.{sh,ps1}` 是纯新增文件，不影响任何现有脚本/构建/CI。
  - `LICENSE` 新增后 `package.sh` L251 自动把它打进后续发布包——这是预期的正向闭环（DEPLOYMENT.md C.2.4 与 §4.9 升级语义一致）。无破坏性。
  - README / DEPLOYMENT 改的是文档措辞，旧的手动安装路径全部保留为「备选」，老用户操作不受影响。
- **回滚**：本任务全部改动是「新增文件 + 文档/许可证文本」，回滚 = `git revert` 对应 commit 即可，无数据态、无服务态需要回退。
- **特性开关**：不需要。一键安装是新增的可选入口，用户不调用就走原手动路径。

---

## 15. Out-of-scope 澄清（本设计不覆盖）

- 不实现卸载功能（已有 `uninstall-service.*`，install.* 仅在输出里提示其路径）。
- 不实现自定义版本下载（固定 `releases/latest`）；安装目录的环境变量覆盖（§3.4）是 Architect 酌情加的便利项，**非 AC、文档主路径不宣传**。
- 不实现 macOS launchd 服务化（无模板，§4.11 降级处理）。
- 不实现 ARM / RISC-V 多架构（BC-3 友好报错）。
- 不改 `release.yml` / `install-service.*` / `package.*` 的行为（仅复用其契约）。
- 不做 GPG / 数字签名校验（仅校验非空 + 可解压）。
- 不打 git tag、不发 GitHub Release（PM Q1 裁决：对外发版属用户决策）。
- 不把一键安装接入 verify_all 做真实联网测试。

---

## 16. Partition assignment（分区分配）

> 已核查：`.harness/agents/` 下无 `dev-*.md` 分区 agent 文件（仅 7 个标准 agent）。本项目为**单 Developer 模式**。按角色契约本节可省略，此处仍列表以明确归属。

| 文件 | 分区 | 新增/修改 | 依赖 |
|---|---|---|---|
| `scripts/install.sh` | developer（单分区） | 新增 | 依赖解压包内 `install-service.sh`（运行时，非构建期） |
| `scripts/install.ps1` | developer | 新增 | 依赖解压包内 `install-service.ps1`（运行时） |
| `LICENSE` | developer | 新增 | — |
| `NOTICE` | developer | 新增 | — |
| `README.md` | developer | 修改 | 依赖 `LICENSE`/`NOTICE` 已存在（章节引用它们） |
| `docs/DEPLOYMENT.md` | developer | 修改 | 依赖 `install.{sh,ps1}` 已存在（命令指向它们） |
| `docs/dev-map.md` | developer | 修改 | 依赖 `install.{sh,ps1}` 已存在 |

**派发顺序**：单 Developer 顺序实现，建议 `LICENSE` / `NOTICE` → `install.sh` / `install.ps1` → `README.md` / `DEPLOYMENT.md` / `dev-map.md`（文档引用前两组成果）。
**并行性**：无（单 Developer）。

---

## 17. Verdict

**READY** — 设计已精确到文件 / 步骤 / 失败分支 / 退出码 / 中文文案级别，17 条 AC 全部有设计覆盖（§13），12 条 BC 全部映射（§13 BC 表），复用审计非空（§9），风险均带缓解（§11）。无阻塞项。

> 提请 Gate Reviewer 关注一处需确认的设计裁决（非阻塞）：§4.11 / R-6 —— BC-11「macOS 下载+解压正常完成」与 release.yml 当前不产 `darwin-amd64` 资产存在事实张力。本设计已裁决为「macOS 资产缺失走 exit 1 + 定制文案，BC-11 完整 exit 0 降级路径代码实现但当前不可达」，属边界澄清而非推翻 PM 裁决。若 Gate Reviewer 认为需 PM 知会，请在 `03_GATE_REVIEW.md` 注明。
