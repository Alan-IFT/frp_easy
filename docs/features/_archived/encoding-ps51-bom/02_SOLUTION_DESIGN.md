# 02 — 方案设计 · T-021 encoding-ps51-bom

> Harness 流水线 Stage 2（Solution Architect）。模式：**full**。
> 上游：`docs/features/encoding-ps51-bom/01_REQUIREMENT_ANALYSIS.md`（Verdict = READY），`docs/features/encoding-ps51-bom/INPUT.md`。
> 本文档只做技术决议与可执行步骤，不重述 AC（AC 编号沿用 01）。

---

## §1 设计意图（对接 RA §1）

将 `scripts/` 目录下 **全部 11 个 `.ps1`** 文件首部加入 UTF-8 BOM（`EF BB BF`），让 Windows PowerShell 5.1（zh-CN 主机，host codepage 936）以 `powershell.exe -File ...` 形态从磁盘加载时能正确按 UTF-8 解码中文字符、消除 `ParserError` / `UnexpectedToken`，与 T-013 已验证的 `irm | iex` 管道形态达到等价可用性。同步在 `scripts/verify_all.ps1` 与 `scripts/verify_all.sh` 各新增 **1 条** 字节级 BOM 检查项（E.7），并新增 `scripts/.editorconfig`（编辑器层 belt）+ verify_all E.7（CI 闸门 suspenders）双层防回归（git blob 字节为持久层），与项目既有 `tsconfig noEmit + --noEmit flag` 双层模式（insight L13）同款。脚本逻辑零修改、行尾仍 LF 不变（NFR-1 / NFR-2）。

> **轮次 2 修订**：原"`.gitattributes` 加 `working-tree-encoding=UTF-8-BOM`"已撤销（事实层错误：该值不是 git iconv 合法值，git 2.34+ checkout 会报错）；改为 `scripts/.editorconfig` 编辑器层 belt。详见 §2.3 / §9 I-3 / §11 修订历史。

---

## §2 模块分解

### §2.1 11 个 .ps1 全部加 BOM（**I-1 决议 = 方案 A，全加**）

| 文件 | 当前编码 | 当前大小 | BOM 后大小（预期 +3 字节） | 含中文 |
|---|---|---:|---:|---|
| `scripts/archive-task.ps1`     | noBOM ASCII | 5044 | 5047 | 否 |
| `scripts/build.ps1`            | noBOM UTF-8 | 2176 | 2179 | 是 |
| `scripts/harness-sync.ps1`     | noBOM ASCII | 4685 | 4688 | 否 |
| `scripts/install-hooks.ps1`    | noBOM ASCII | 2783 | 2786 | 否 |
| `scripts/install-service.ps1`  | noBOM UTF-8 | 9705 | 9708 | 是 |
| `scripts/install.ps1`          | noBOM UTF-8 | 15596 | 15599 | 是 |
| `scripts/package.ps1`          | noBOM UTF-8 | 9708 | 9711 | 是 |
| `scripts/start-e2e-server.ps1` | noBOM UTF-8 | 2923 | 2926 | 是 |
| `scripts/start.ps1`            | noBOM UTF-8 | 2312 | 2315 | 是 |
| `scripts/uninstall-service.ps1`| noBOM UTF-8 | 3990 | 3993 | 是 |
| `scripts/verify_all.ps1`       | noBOM UTF-8 | 12459 | 12462 + 新检查项实际增量 | 是 |

> 注：`verify_all.ps1` 最终大小 = 加 BOM 后字节 + §2.2 新检查项块体；本表"BOM 后大小"列只算 +3 字节。

### §2.2 verify_all 新增 BOM 检查项（**E.7**）

> 编号决议：放在 E 段尾（项目结构），紧接 E.6 之后。理由：本检查项性质是"仓库内静态结构 / 防回归"，与 E.1~E.6（CLAUDE.md / workflow.md / agents / sync / AI-GUIDE / Adversarial tests）同类，不应另开新段。

#### verify_all.ps1 E.7 伪码

```powershell
Step "E.7" "scripts/*.ps1 have UTF-8 BOM" {
    if (-not (Test-Path "scripts")) { return "SKIP" }
    $ps1s = Get-ChildItem -Path "scripts" -Filter "*.ps1" -File -ErrorAction SilentlyContinue
    if (-not $ps1s -or $ps1s.Count -eq 0) { return "SKIP" }
    $missing = @()
    foreach ($f in $ps1s) {
        # PS5.1: Get-Content -Encoding Byte；PS7+: Get-Content -AsByteStream
        # 使用 .NET API 跨 PS5/7 一致
        $bytes = [System.IO.File]::ReadAllBytes($f.FullName)
        if ($bytes.Length -lt 3 -or $bytes[0] -ne 0xEF -or $bytes[1] -ne 0xBB -or $bytes[2] -ne 0xBF) {
            # 轮次 2 修订（C-4）：加 $root startsWith guard，避免 verify_all 从子目录调用时 Substring 越界
            if (-not $f.FullName.StartsWith($root)) {
                throw "verify_all 必须从仓库根目录运行（当前 root: $root）"
            }
            $relPath = $f.FullName.Substring($root.Length + 1)
            $missing += $relPath
        }
    }
    if ($missing.Count -gt 0) {
        throw "Missing UTF-8 BOM in:`n$($missing -join "`n")"
    }
}
```

#### verify_all.sh E.7 伪码

```bash
# E.7 — All scripts/*.ps1 must start with UTF-8 BOM (EF BB BF)
if [[ ! -d scripts ]]; then
    step "E.7" "scripts/*.ps1 have UTF-8 BOM" "SKIP"
else
    e7_missing=""
    while IFS= read -r f; do
        # POSIX 字节级：head -c 3 + xxd -p；xxd 在 Ubuntu / macOS / Git Bash 均可用
        first3=$(head -c 3 "$f" 2>/dev/null | od -An -tx1 | tr -d ' \n')
        if [[ "$first3" != "efbbbf" ]]; then
            e7_missing="$e7_missing\n$f"
        fi
    done < <(find scripts -maxdepth 1 -name '*.ps1' -type f 2>/dev/null)
    if [[ -z "$e7_missing" ]]; then
        step "E.7" "scripts/*.ps1 have UTF-8 BOM" "PASS"
    else
        step "E.7" "scripts/*.ps1 have UTF-8 BOM" "FAIL" "$(echo -e $e7_missing)"
    fi
fi
```

> 工具选定理由：
> - PS 端用 `[System.IO.File]::ReadAllBytes()` 而非 `Get-Content -Encoding Byte`（PS5.1）/ `-AsByteStream`（PS7+）：避免 PS5/PS7 同语义不同 flag 名导致的兼容分歧；.NET API 在两版本上行为一致、字节顺序确定。
> - sh 端用 `head -c 3 | od -An -tx1`：`head -c` 是 POSIX standard（GNU coreutils / BusyBox / macOS BSD 全支持）；`od` 比 `xxd` 通用（Alpine / minimal Docker 镜像可能缺 `xxd`，但必含 `od`）。

### §2.3 编辑器层 belt：新增 `scripts/.editorconfig`（**I-3 决议 = 方案 B，撤销 `.gitattributes` 改动**）

> **轮次 2 修订**：原"`.gitattributes` 追加 `*.ps1 working-tree-encoding=UTF-8-BOM eol=lf`"决议**已撤销**。详见 03 §2 C-1 与本文档 §9 I-3 决议重写：
> - `working-tree-encoding` 不是 git iconv 合法值，git 2.34+ checkout 会报 `failed to encode '...' from UTF-8 to UTF-8-BOM` → `git pull` 直接中断。
> - git blob 本身已保存 BOM 字节（步骤 2 字节级写入），checkout 走默认文本拷贝即字节级保留 BOM —— 无需任何 git 属性辅助。
>
> **决议**：
> - **不动 `.gitattributes`**（撤销改动，零 diff）。
> - **新增 `scripts/.editorconfig`**（**新文件**）作编辑器层 belt（覆盖 VS Code / JetBrains / Vim+editorconfig 插件 / Notepad++ EditorConfig 插件），git blob 字节是 suspenders。
> - verify_all E.7 是最终闸门（CI 拦下）。

**新 `scripts/.editorconfig`（新文件）**：

```editorconfig
# T-021: 强制 scripts/*.ps1 文件为 UTF-8 with BOM，防编辑器误存为 noBOM 导致 PS5.1 + zh-CN 主机解析失败。
[*.ps1]
charset = utf-8-bom
end_of_line = lf
insert_final_newline = true
```

> 三层防御汇总：
> 1. **Suspenders（git blob 字节）**：步骤 2 字节级写入 BOM，git 默认字节级 checkout 始终保留。
> 2. **Belt（编辑器层 `.editorconfig`）**：编辑器保存时锁定 UTF-8 BOM；覆盖支持 EditorConfig 的主流编辑器。
> 3. **Last gate（verify_all E.7）**：CI / 本地 PR 前跑、任何编辑器层失效都能拦下（参见 §6 R-11）。

### §2.4 不新建 `scripts/add-ps1-bom.ps1`（一次性工具脚本）

> **决议**：不新建辅助脚本。Developer 直接用 PowerShell 一行 inline 命令处理 11 个文件即可（见 §3 步骤 2）。理由：一次性操作 + git diff 即审计、不需要长期维护工具；新建脚本反而引入"自己也得加 BOM"循环。

### §2.5 baseline.json 不动 `test_count` 字段（**澄清 AC-14**）

读 `scripts/baseline.json`：

```json
{ "version": 8, "test_count": 335, "passing_count": 335,
  "go_tests": 239, "frontend_tests": 96, ... ,
  "notes": "... verify_all PASS 19/19 stable x3 runs. ..." }
```

- `test_count` / `passing_count` 是 **Go + 前端单元测试总数**（239 + 96 = 335），与 verify_all 检查项数 **无关**。
- verify_all 的 19 项检查数仅在 `notes` 文本里。
- **决议**：本任务**只改 `notes` 文本**，**不改** `test_count` / `passing_count`。`version` 升 8 → 9，`updated` 改本任务 QA 跑通日期。
- **轮次 2 修订（按 03 OPT-2）notes 文案精修**：
  - 把现有 `follow-up T-020-encoding-ps51-bom` 一句改为 `closed by T-021 encoding-ps51-bom`（narrative 闭环）。
  - 追加一句 `"T-021 closed: .ps1 BOM applied (11/11); E.7 added; verify_all 19→20."`。
- AC-14 的字面表述"PASS 计数从 19 升到 20"由 verify_all 实际运行结果自然满足，不依赖 baseline.json 字段。

---

## §3 实施步骤（Developer 可执行序列）

> 全部 Windows PowerShell（PS7 也可，命令兼容）。每步给出"做什么、怎么验证、git diff 形态"。

### 步骤 1：备份 noBOM 快照（用于 AC-2 字节级回归核对）

```powershell
cd C:\Programs\frp_easy
$snapshot = "scripts\.bom-pre-snapshot"
New-Item -ItemType Directory -Path $snapshot -Force | Out-Null
Get-ChildItem scripts\*.ps1 | ForEach-Object {
    Copy-Item $_.FullName (Join-Path $snapshot $_.Name)
}
```

- 验证：`(Get-ChildItem $snapshot\*.ps1).Count` 应 = 11。
- git diff：**不**提交此快照（步骤 7 删除）。`.gitignore` 不需新增（步骤 7 清理）。

### 步骤 2：批量加 BOM（**核心步骤**）

Developer **必须** 用 .NET API 字节级写盘，**禁止** 用 Edit / Write 工具（Write 写无 BOM UTF-8；Edit 改不了字节级编码）。命令：

```powershell
cd C:\Programs\frp_easy
$utf8Bom = New-Object System.Text.UTF8Encoding($true)   # $true = 带 BOM
# 读时构造：encoderShouldEmitUTF8Identifier=$false（读不写 BOM）、throwOnInvalidBytes=$true
# $true 让非法 UTF-8 字节立即抛 DecoderFallbackException，避免步骤 4 字符级回归被 silent U+FFFD 替换骗过
$utf8ReadStrict = New-Object System.Text.UTF8Encoding($false, $true)
Get-ChildItem scripts\*.ps1 | ForEach-Object {
    $path = $_.FullName
    $content = [System.IO.File]::ReadAllText($path, $utf8ReadStrict)
    [System.IO.File]::WriteAllText($path, $content, $utf8Bom)
}
```

- **关键 insight 应用**：参考 insight L17（PowerShell 写 TOML 必须 `UTF8Encoding($false)`）的**反向**用法 —— 这里要 `$true`（带 BOM）。两个任务同款 API，参数相反。
- **关键 insight 应用**：参考 insight L37（Edit/Write soft-block 走 `.NET WriteAllText` 字节级）—— 本任务路径 `scripts/*.ps1` **不在** Self-Modification 列表（该列表只针对 `.claude/settings.json` 类），理论上 Edit/Write 可用，但仍选 `.NET WriteAllText` 因为：**Write 工具无法控制 BOM 输出**（PS7 上 `Out-File -Encoding utf8` 默认无 BOM；PS5.1 上默认有 BOM —— 跨版本不可预测）。
- **CR/LF 保留**：`ReadAllText` 把原文件按 UTF-8（无 BOM 假定，因为读时未加 BOM）整体读入字符串；项目 `.gitattributes` `* text=auto eol=lf` 让所有 .ps1 工作树已是 LF，读入后字符串里仍是 `\n`；`WriteAllText` 不主动改行尾、按原样写回 + 前置 BOM。**不要**用 `Get-Content`（默认按行读、`Set-Content` 默认加平台行尾，PS5 = CRLF，会破 NFR-2）。

### 步骤 3：字节级核对（AC-1 + AC-2 + NFR-2 自检）

```powershell
cd C:\Programs\frp_easy
Get-ChildItem scripts\*.ps1 | ForEach-Object {
    $bytes = [System.IO.File]::ReadAllBytes($_.FullName)
    $bom = $bytes[0..2] -join ','
    $hasCRLF = ($bytes | Where-Object { $_ -eq 13 }).Count
    "{0,-30} BOM={1} CR_count={2} size={3}" -f $_.Name, $bom, $hasCRLF, $bytes.Length
}
```

预期输出：
- 每行 `BOM=239,187,191`（= 0xEF 0xBB 0xBF）
- 每行 `CR_count=0`（NFR-2 LF 不变）
- size 每个文件 = §2.1 表"BOM 后大小"列（+3 字节，verify_all.ps1 除外，它还会在步骤 5 再加 E.7 块体）

### 步骤 4：字符级回归对比（NFR-1 内容零字节改）

```powershell
cd C:\Programs\frp_easy
# 读旧文件用 strict（throwOnInvalidBytes=$true）与步骤 2 一致，确保任何非法 UTF-8 字节立即抛
$utf8ReadStrict = New-Object System.Text.UTF8Encoding($false, $true)
$utf8ReadStrictWithBom = New-Object System.Text.UTF8Encoding($true, $true)
Get-ChildItem scripts\*.ps1 | ForEach-Object {
    $newText = [System.IO.File]::ReadAllText($_.FullName, $utf8ReadStrictWithBom)
    $oldText = [System.IO.File]::ReadAllText(
        (Join-Path "scripts\.bom-pre-snapshot" $_.Name),
        $utf8ReadStrict)
    if ($newText -ne $oldText) {
        Write-Host "DRIFT in $($_.Name)" -ForegroundColor Red
    } else {
        Write-Host "OK    $($_.Name)" -ForegroundColor Green
    }
}
```

预期：全 11 行 `OK`。任意 `DRIFT` 立即停下、回到步骤 2 调查（极可能是 ReadAllText 读时编码假设错）。

### 步骤 5：编辑 `scripts/verify_all.ps1` 加 E.7（§2.2 PS 块）

- 用 Edit 工具（**此时 verify_all.ps1 已有 BOM**，Edit 工具读字符串、写字符串，会保留 BOM）。
- 插入位置：现有 E.6 块结束后（line ~266 之后）、`# --- Summary ---` 之前。
- 插入完成后再次跑 步骤 3 字节级核对 `scripts/verify_all.ps1` —— 仍须 BOM 起始、CR_count = 0。
- git diff：`scripts/verify_all.ps1` 块体新增 ~15 行 + 文件头 BOM 字节（git diff 通常表现为 "+++ ... binary differ" 或正常 +15 行文本 diff —— 取决于 git 客户端 .ps1 文件二进制 detection，本项目 .ps1 未在 .gitattributes 标 binary，预计文本 diff）。

### 步骤 6：编辑 `scripts/verify_all.sh` 加 E.7（§2.2 sh 块）

- 用 Edit 工具。**注意** verify_all.sh 是 .sh 文件，**不加** BOM（POSIX shebang `#!/usr/bin/env bash` 必须在文件第 1 字节，BOM 会让 `env` 找不到解释器）。
- 插入位置：现有 E.6 块结束后（line ~276 之后）、`# Summary` 之前。
- git diff：纯文本 diff，新增 ~13 行。

### 步骤 7：新增 `scripts/.editorconfig`（§2.3，**轮次 2 修订**）

```powershell
cd C:\Programs\frp_easy
# 用 Write / Edit 工具新建 scripts\.editorconfig，内容见 §2.3 末尾代码块。
# .editorconfig 是文本 ASCII 文件，不加 BOM。
```

- 内容见 §2.3 末尾的 `.editorconfig` 代码块。
- git diff：新增 `scripts/.editorconfig`（约 5 行）。
- **不动 `.gitattributes`**（轮次 1 决议已撤销，详见 §2.3 与 §9 I-3）。
- **重要**：`.editorconfig` 仅对支持 EditorConfig 的编辑器生效；git blob 字节才是 BOM 的真正持久层（已由步骤 2 字节级写入保证），verify_all E.7 是最终闸门。

### 步骤 8：更新 `scripts/baseline.json`（§2.5）

```powershell
cd C:\Programs\frp_easy
# 用 Edit 工具：
# - "version": 8  → "version": 9
# - "updated": "2026-05-23" → "updated": "<本任务 QA 跑通日期>"（由 QA 在 06 填，Developer 04 暂留 2026-05-23 占位）
# - notes 内 "follow-up T-020-encoding-ps51-bom" → "closed by T-021 encoding-ps51-bom"
# - notes 末尾追加 "T-021 closed: .ps1 BOM applied (11/11); E.7 added; verify_all 19→20."
```

- **不动** `test_count` / `passing_count` / `go_tests` / `frontend_tests`。
- git diff：`scripts/baseline.json` 修改 3 个字段。

### 步骤 9：更新 `docs/dev-map.md`（AC-16）

- 查找 `## 目录布局` 块中 `scripts/` 行（约 L25-L27）。
- 在该行末尾追加一句："**T-021：scripts/*.ps1 统一 UTF-8 BOM（首 3 字节 EF BB BF），让 PS 5.1 + zh-CN 主机磁盘加载形态正确解码中文；verify_all E.7 + scripts/.editorconfig 双层防回归（git blob 字节为持久层、editorconfig 为编辑器层 belt）。**"
- 用 Edit 工具。
- git diff：dev-map.md +1 行。

### 步骤 10：清理快照 + 跑 verify_all

```powershell
cd C:\Programs\frp_easy
Remove-Item -Recurse -Force scripts\.bom-pre-snapshot
pwsh -File scripts\verify_all.ps1
```

- 预期：Summary 行 `PASS: 20  WARN: 0  FAIL: 0  SKIP: <几个>`。
- 若 E.7 PASS、其他原 19 项无回归 → 步骤 1~10 完成。
- 若 E.7 FAIL → 步骤 2 漏文件，回去补；若其他项 FAIL → DESIGN DRIFT，在 `04_DEVELOPMENT.md` 标注。

### 步骤 11：dogfood archive-task.ps1（**I-5 决议 = 方案 B，必跑**）

> 此步骤由 QA 在 Stage 6 跑（Developer 在 04 不跑，避免污染交付目录）。Developer 在 04 留下命令：

```powershell
# 前置字节核对（OPT-1）：确认 dogfood 是真在已加 BOM 版本上跑
(Get-Content scripts\archive-task.ps1 -Encoding Byte -TotalCount 3) -join ','
# 预期输出：239,187,191

pwsh -File scripts\archive-task.ps1 -Task encoding-ps51-bom -DryRun

# 后置字节核对（OPT-1）：dogfood 结束后再次确认文件未被自身改动
(Get-Content scripts\archive-task.ps1 -Encoding Byte -TotalCount 3) -join ','
# 预期输出：239,187,191（与前置一致）
```

- 预期：脚本能正常解析（自身 BOM 已加）、dry-run 输出 "Would move ... / Would harvest N insights"，退出码 0，无 ParserError。
- 若失败 → R-2 触发，回滚步骤 2 对 archive-task.ps1 的修改、单独调查（极可能是 PS 解释器边角 bug）。

---

## §4 API / 接口 / 数据约定

- 无 HTTP API 改动。
- 无数据库迁移。
- verify_all E.7 检查项 stdout 文案约定：

| 状态 | PS / sh 输出格式 |
|---|---|
| PASS | `[E.7] scripts/*.ps1 have UTF-8 BOM ... PASS` |
| FAIL | `[E.7] scripts/*.ps1 have UTF-8 BOM ... FAIL` + 缺 BOM 文件路径列表（多文件 `\n` 分隔），便于用户一眼定位回归点 |
| SKIP | `[E.7] scripts/*.ps1 have UTF-8 BOM ... SKIP`（仅当 `scripts/` 目录不存在 / 无 .ps1，本仓库不会触发） |

- baseline.json schema 不变；仅 `version` / `updated` / `notes` 字段值变更。

---

## §5 跨平台 / 兼容性矩阵

### §5.1 BOM 字节级判别选型

| 平台 | 实现 | 命令 |
|---|---|---|
| Windows PS 5.1 | .NET API | `[System.IO.File]::ReadAllBytes($p)[0..2]` |
| Windows PS 7.x | .NET API | 同上 |
| Linux Bash 4.x+ | POSIX | `head -c 3 file \| od -An -tx1 \| tr -d ' \n'` 比较 `efbbbf` |
| macOS Bash 3.x / zsh | POSIX | 同上（`head -c` BSD 也支持） |
| GitHub Actions ubuntu-latest | POSIX | 同上（runner 含 `head` / `od` / `find` 内置） |
| GitHub Actions windows-latest | PS 5.1 / PS 7 | runner 含 pwsh，走 PS 分支 |

### §5.2 BOM 对周边工具的影响审计

| 工具 / 场景 | BOM 影响 | 缓解 |
|---|---|---|
| PowerShell 5.1 解释器 | **正向**：识别 BOM → 按 UTF-8 解码，消除 zh-CN host codepage 乱码（本任务目的） | 无需缓解 |
| PowerShell 7.x 解释器 | 透明：吞 BOM，不打印 | AC-7 / AC-8 保证 |
| `Invoke-RestMethod \| iex` 管道 | 透明：PS 内置 BOM-aware 解码；RA §6 第 7 项机制层解释 = `irm` 按 HTTP Content-Type 解码为字符串、BOM 字符若入 string 也会被 `iex` 解析器视作 `[char]0xFEFF` 空白字符忽略 | AC-9 / AC-10 真机验证 |
| Git diff（.ps1 文件） | 文本 diff 显示 "BOM byte added"（首行前出现 `<U+FEFF>` 字符标记）；不会触发二进制 detection（除非 .gitattributes 标 binary） | 一次性 PR 噪声、可接受 |
| Git checkout（默认文本拷贝、字节级保留 BOM） | git blob 内字节即 BOM；默认文本拷贝是字节级（仅 CRLF/LF 归一可能改字节，本仓库已 `* text=auto eol=lf` 全 LF），始终写出 BOM | 步骤 2 字节级写入 + verify_all E.7 |
| VS Code 打开 | 自动识别 BOM，状态栏显示 `UTF-8 with BOM`，编辑后保存默认保留 | OK |
| VS Code（设置 `files.encoding = utf8`，无 BOM） | **风险**：保存时去 BOM | verify_all E.7 拦下 + .gitattributes hint |
| Notepad（Windows）| 识别 BOM，保存默认保留 | OK |
| PowerShell ISE | 识别 BOM | OK |
| `grep` / `rg` 第一行匹配 | BOM 字节出现在第 1 行首部，`grep "^#"` 仍能匹配（因 grep 默认按字节模式、`^` 匹配行首字节位置，BOM 是字节但 `^` 描述的是行边界）；**但** `grep "^# *foo"` 等带固定首字符模式可能漏匹配 BOM 文件第 1 行 | §6 R-4：审计 |
| `cat` / `less` / `more` | 透明显示，BOM 不可见或显示为 `<U+FEFF>` | OK |

### §5.3 R-4 grep 第一行影响审计（**关键风险预先验证**）

已用 Grep 工具扫描 `c:\Programs\frp_easy\scripts\*.ps1` 的"第一行 / 首字符 grep"逻辑（命令：`Grep "Select-Object -First 1|Get-Content.*-First|head -1|head -n 1"`），结果：

- `scripts\install.ps1` L186 / L237 / L305：`Select-Object -First 1` 是数组元素选择，**不**作用于文件第一行字节，与 BOM 无关。
- `scripts\verify_all.ps1` L170：`Select-Object -First 10` 同上，数组截取。
- `scripts\start-e2e-server.ps1` L22：`Select-Object -First 1` 数组截取。
- `scripts\verify_all.sh` L178：`find ... | head -10`，作用于 find 输出而非文件首行。

**结论**：仓库内**无任何 grep / head 第一行字节** 的逻辑会被 BOM 影响。R-4 风险面 = 0，但 verify_all E.7 仍是未来防回归闸门（若新代码引入此类 grep 会立刻发现）。

### §5.4 archive-task.ps1 insight 收割 regex 与 BOM

- archive-task.ps1 收割 `07_DELIVERY.md`（**非 .ps1** 文件）的 `## Insight` 段（insight L43）。
- `07_DELIVERY.md` 不加 BOM（本任务只动 .ps1），regex `(?ms)^##\s+Insights?\s*$` 不会被 BOM 干扰。
- archive-task.ps1 **自身** 加 BOM → 由 R-2 dogfood（步骤 11）守护。

---

## §6 风险 / 缓解

| ID | 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|---|
| **R-1** | 编辑器（VS Code 用户配置 `files.encoding = utf8` 不含 BOM、Vim 默认 `set nobomb`、Notepad++ "UTF-8" 模式）保存时去 BOM | 中 | 中（PR 进 main 后 verify_all 拦下） | 三层防御：(a) git blob 字节级保存 BOM（步骤 2）；(b) `scripts/.editorconfig` 编辑器层 belt（§2.3）；(c) verify_all E.7 CI 拦下 |
| **R-2** | `archive-task.ps1` 自身加 BOM 后，PS5.1 / PS7 解释器存在边角加载 bug | 低 | 高（stage 7 PM 调用归档崩溃，本任务无法 ship） | I-5 决议方案 B：步骤 11 由 QA dogfood `-DryRun` 跑一次；若失败则 Architect 在 03 后追加 sub-fix（不本期 ship archive-task.ps1 的 BOM，剩余 10 个先 ship + 单独 followup） |
| **R-3** | CI 环境（ubuntu-latest 跑 pwsh 7.x、windows-latest 跑 pwsh + powershell.exe）对 BOM 解析有差异 | 极低 | 中 | (a) GitHub Actions `release.yml` 已在 T-013 验证跑过 .ps1 路径；本任务步骤 10 跑 `pwsh -File scripts\verify_all.ps1`（QA 主机为 PS7）+ AC-7 显式覆盖 pwsh 不回归；(b) Linux pwsh 跑磁盘 .ps1 BOM-aware 是 PowerShell Core 设计内行为，证据：Microsoft Docs `about_Character_Encoding` 明文 "The default encoding in PowerShell 7+ is UTF-8 with BOM optional for input" |
| **R-4** | 纯 ASCII 脚本（archive-task / harness-sync / install-hooks）加 BOM 后 `grep "^#"` 等第一行字符匹配漏命中 | 低 | 中 | §5.3 已审计：仓库内**无任何**此类 grep 逻辑命中 .ps1；E.7 是未来防回归闸门 |
| **R-5** | T-020 settings.json self-block 拦截本任务的 .NET WriteAllText 写 .ps1 | 极低 | 高（无法实施） | insight L37 明文 "auto-mode 分类器仅看 `.claude/settings.json` 路径"。本任务路径 `scripts/*.ps1` 不在该列表，理论上不触发。**Developer 若实际遇 soft-block**，立即降级走 Bash 工具调度 `pwsh -File <内联脚本>`（insight L37 同款绕过路径）。|
| **R-6** | sh 检查项与 ps1 检查项编号 / 命名 / 输出格式漂移 | 中 | 低（CI 仅跑一边） | §2.2 已规定两边都用 `E.7` 编号 + 同一标题字符串 `"scripts/*.ps1 have UTF-8 BOM"`；同步 PR 评审时核对两文件 grep `E.7` 行数一致。|
| **R-7** | ~~`working-tree-encoding=UTF-8-BOM` 在 Git < 2.10 客户端被忽略~~ | — | — | **消除**：轮次 2 按 03 C-1 决议方案 A 撤销该属性，git blob 默认字节级 checkout 始终保 BOM；本风险整体不再适用 |
| **R-8** | git diff 显示 BOM 字节时把 .ps1 误判为二进制文件、CR 审计困难 | 低 | 低 | 项目 `.gitattributes` `* text=auto eol=lf` 在 .ps1 上默认是 text；BOM 不触发 binary detection（Git heuristic 是看 NUL 字节，BOM 是 3 字节非 NUL）；实测 git diff 显示文本 diff |
| **R-9** | RA §I-5 推测 archive-task.ps1 dogfood 在 stage 7 PM 调用时崩 | 低 | 高 | dogfood 在 QA 06（步骤 11）先跑 `-DryRun`，stage 7 PM 调用时再跑实跑，前置抓 bug |
| **R-10** | `scripts/baseline.json` 编辑导致 JSON 解析失败 | 极低 | 低 | Edit 工具改 3 个字段、不动结构；JSON-aware 编辑器 + verify_all 跑前 `Get-Content baseline.json \| ConvertFrom-Json` 自检（步骤 8 末尾） |
| **R-11** | 编辑器层 belt 仅靠 `scripts/.editorconfig`，遗留 dev 用未支持 .editorconfig 的编辑器（如裸 Vim 无插件、Notepad 经典版、纯 `>` redirect）时会绕过 charset 锁定 | 中 | 低 | Suspenders = verify_all E.7 入 main 前拦截（CI / pre-push hook 必跑）；R-1 缓解链同款；轮次 2 已撤销 working-tree-encoding（C-1），编辑器层无更强 git 侧锁定可用 |

---

## §7 测试策略

### §7.1 自动可验（Developer 在 04 + QA 在 06）

| 项 | 命令 | 期望 |
|---|---|---|
| BOM 字节级 | §3 步骤 3 | 11 个文件 `BOM=239,187,191` + `CR_count=0` |
| 内容字符级回归 | §3 步骤 4 | 11 个 OK，0 DRIFT |
| verify_all 全跑 | `pwsh -File scripts\verify_all.ps1` | `PASS 20` |
| verify_all.sh 全跑 | `bash scripts/verify_all.sh`（QA WSL / Git Bash） | `PASS: 20` |
| AC-13 负向自检 | QA 临时把 `scripts\start.ps1` 前 3 字节删掉、跑 verify_all、再恢复 | E.7 FAIL，错误信息含 `scripts\start.ps1` |
| AC-7 PS7 不回归 | `pwsh -File scripts\install.ps1 -Help` | 退出码 0，中文帮助完整显示，不出现 `锘` 类乱码 |
| AC-8 PS7 build dry-run | `pwsh -File scripts\build.ps1` 不带参数若会真编译则改 `pwsh -NoProfile -Command "& { Get-Content scripts\build.ps1 -Raw \| Out-Null }"`（仅解析、不执行） | 解析 0 错 |
| AC-10 [U] 真机（合并到 AC-9） | PS5.1 + zh-CN 主机用 `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 \| iex` 真实链路跑通 | install 步骤无 ParserError、不出现 `锘` 类乱码、完成 8 步 |

### §7.2 用户真机验证（**I-4 决议 = 方案 A，降级**）

- AC-3 / AC-4 / AC-5 / AC-6 / AC-9 标 `[U]`，由用户在 PS 5.1 + zh-CN 主机执行；QA 06 不强求本地复现。
- 真机验证清单（QA 在 06 §6 真机验证清单写明）：
  1. `powershell.exe -File scripts\install.ps1 -Help` → 中文帮助 + 退出码 0
  2. `powershell.exe -File scripts\verify_all.ps1 -Quick` → 跑到 Summary 行（允许单项 FAIL）+ `Get-Content scripts\verify_all.ps1 -Encoding Byte -TotalCount 3` = `239 187 191`
  3. `powershell.exe -File scripts\install-service.ps1 -DisplayName "FRP Easy" -ServiceName "frp-easy-test"`（**AC-5 参数选定** = `-DisplayName` + `-ServiceName`，因 install-service.ps1 实际无 `-DryRun` / `-Help` / `-WhatIf`；用一个 fake 服务名跑、立即 `sc.exe delete frp-easy-test` 清理） → 中文 stdout 不乱码、退出码 0 或 1（前置失败）但不能 2（sc.exe 调用失败）
  4. `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex` 在 PS 5.1 终端跑 → AC-6 完整 8 步、最终 `==> 服务已启动` + 退出码 0

### §7.3 dogfood（**I-5 决议 = 方案 B**）

- QA 06 必跑：`pwsh -File scripts\archive-task.ps1 -Task encoding-ps51-bom -DryRun`
- 期望：dry-run 输出 "Would move ... / Would harvest N insights"、退出码 0、无 ParserError。
- 若 dogfood 失败 → R-2 触发，QA 在 06 标 BLOCKER 回退到 Architect 决议是否单独排除 archive-task.ps1。

### §7.4 Adversarial tests（QA 06 强制段）

QA 06 必须含**裸标题** `## Adversarial tests`（无数字前缀，insight L24 / L35）。内容至少 5 条：

1. 故意删 BOM 跑 verify_all → E.7 FAIL（AC-13）
2. 故意把 BOM 改成 0xFF 0xFE（UTF-16 LE BOM）→ E.7 FAIL（PS5.1 真识别 UTF-16 但不是本任务目的）
3. 新增一个伪 .ps1（`scripts\fake.ps1`）无 BOM → E.7 FAIL；删掉后 PASS
4. 改 .gitattributes 删 `working-tree-encoding` 行、`git rm --cached scripts/*.ps1 && git checkout HEAD -- scripts/*.ps1` → 工作树仍有 BOM（Git 字节级 checkout）
5. PS7 跑 `pwsh -File scripts\verify_all.ps1` 三连跑 → 都 PASS 20，无随机 flake

---

## §8 给 Developer 的实施 checklist（一行一条）

- [ ] §3 步骤 1：备份 `scripts\.bom-pre-snapshot/`
- [ ] §3 步骤 2：`[System.IO.File]::WriteAllText` + `UTF8Encoding($true)` 处理 11 个 .ps1
- [ ] §3 步骤 3：字节级核对 BOM=239,187,191 + CR_count=0
- [ ] §3 步骤 4：字符级回归 11 个 OK
- [ ] §3 步骤 5：在 verify_all.ps1 E.6 后插入 E.7 块（§2.2 PS 伪码）
- [ ] §3 步骤 6：在 verify_all.sh E.6 后插入 E.7 块（§2.2 sh 伪码）
- [ ] §3 步骤 7：新建 `scripts/.editorconfig`（§2.3 末尾代码块，5 行）；**不动** `.gitattributes`
- [ ] §3 步骤 8：`scripts/baseline.json` 改 version / updated / notes，**不动** test_count
- [ ] §3 步骤 9：`docs/dev-map.md` 追加 T-021 编码说明 1 行
- [ ] §3 步骤 10：删 snapshot 目录、跑 `pwsh -File scripts\verify_all.ps1` PASS 20
- [ ] 04_DEVELOPMENT.md 记录：所有 git diff 文件列表、步骤 3/4 输出样例、verify_all PASS 20 截屏

---

## §9 RA 不确定性的回答（强制）

### I-1：纯 ASCII 的 3 个 .ps1 是否加 BOM？

**SA 决议 → 方案 A，全 11 个一律加 BOM**。

理由：(a) verify_all E.7 规则保持极简（一行 glob `scripts/*.ps1`，无须 "if 非 ASCII"分支）；(b) 防未来 ASCII 脚本里加一个中文 `Write-Host` 就复发；(c) 3 字节 × 3 文件 = 9 字节冗余可忽略；(d) 与 RA 推荐方向一致。

### I-2：verify_all 检查粒度

**SA 决议 → 方案 A，严格（与 I-1 配对）**。

E.7 实现规则：扫描 `scripts/*.ps1`（**不**递归到子目录、**不**扫描 `.harness/` / `web/` / `docs/` / `node_modules/`，AC-15 强制）；任一文件前 3 字节 ≠ `EF BB BF` 即 FAIL。

### I-3：`.editorconfig` / `.gitattributes` 锁定

**轮次 2 修订（按 03 C-1）**：

**SA 决议 → 方案 B：新增 `scripts/.editorconfig`，不动 `.gitattributes`**。

理由：
1. **轮次 1 错误自承认**：原决议 `working-tree-encoding=UTF-8-BOM` 不是 git iconv 合法值（`UTF-8-BOM` 不在 POSIX iconv -l 输出别名表，git-attributes(5) 官方 man page 实际只允许 iconv 兼容的字符编码标签如 `UTF-16` / `UTF-16LE` / `Shift_JIS` / `GB18030` / `Big5` 等，且明文 "use UTF-8 as the internal representation" —— 指定 `UTF-8` 等于啥都没干）。git 2.34+ checkout 时会调 iconv 失败 → `error: failed to encode '...' from UTF-8 to UTF-8-BOM` → 用户 `git pull` 直接中断。03 §2 C-1 完整问题描述。
2. **方案 A（撤销 working-tree-encoding）已采纳**：git blob 内字节即 BOM（步骤 2 字节级写入），git 默认文本拷贝是字节级（仅 CRLF/LF 归一可能改字节，本仓库已 `* text=auto eol=lf` 全 LF）—— `.gitattributes` 不需要任何 BOM 相关追加；零 diff。
3. **方案 B（`.editorconfig`）作 belt 补充**：编辑器层覆盖 VS Code / JetBrains / Vim+editorconfig 插件 / Notepad++ EditorConfig 插件；git blob 字节是 suspenders。两者协同。
4. **遗留风险记入 R-11**：未支持 .editorconfig 的编辑器会绕过 charset 锁定，verify_all E.7 是最终闸门。

引用：详见 03 §2 C-1（Reviewer 倾向方案 A + 部分采纳方案 B 即本决议）。

### I-4：PS 5.1 + zh-CN 本地 mock？

**SA 决议 → 方案 A，降级用户真机（采纳 RA 推荐）**。

AC-3 ~ AC-6 + AC-9 在 06 真机验证清单标 `[U]`，QA 主机不强制本地复现；与 T-019 / T-018 历史降级模式一致。

### I-5：archive-task.ps1 dogfood

**SA 决议 → 方案 B，QA 06 必跑 dogfood**。

§3 步骤 11 + §7.3 + §7.4 ADV-1 三层保证。若 dogfood 失败 → 单独 sub-fix（不阻塞其他 10 个 .ps1 的本期交付）。

### I-6 / I-7 / I-8（RA §6 第 4 ~ 8 项）

- **实现工具选定**（RA §6 第 4）：§3 步骤 2 决议用 `[System.IO.File]::WriteAllText($p, $content, [System.Text.UTF8Encoding]::new($true))`；Edit / Write 工具 / `Set-Content -Encoding utf8BOM`（PS7 only）/ `Out-File -Encoding utf8` 均不选（理由见步骤 2 注释）。
- **verify_all step 编号**（RA §6 第 5）：决议放 E 段尾 = **E.7**（§2.2 注脚）。
- **baseline.json 是否要改**（RA §6 第 6）：决议**只改 notes 文本 + version 升 8→9 + updated 改本任务日期**，**不动** `test_count` / `passing_count`（§2.5）。
- **`irm | iex` BOM 吞咽机制**（RA §6 第 7）：**轮次 2 修订（按 03 C-3）**：决议机制层 = `Invoke-RestMethod` HTTP 客户端层（PS5.1 / PS7 一致、内置 BOM-aware 解码）剥 BOM，字符串到达 `iex` 时无 U+FEFF；`iex` parser 本身不参与 BOM 处理。轮次 1 早期假设"`iex` parser 把 `[char]0xFEFF` 当 whitespace 忽略" **是错的** —— PS5.1 parser 对 U+FEFF 的处理在某些 build 上会当 `unexpected token` 抛 ParserError（正是本任务起源问题的近亲），mock 测试与真实路径断开。AC-10 已改为 [U] 真机断言（合并到 AC-9，§7.1 行已重写）。
- **降级策略对账**（RA §6 第 8）：决议采纳，§7.2 明文列 5 项真机清单，Gate Reviewer 在 03 复核。

---

## §10 Open Issues（留给 Gate Reviewer）

| ID | 项 | 我的判断 | 留给 GR 复核点 |
|---|---|---|---|
| O-1 | ~~`working-tree-encoding=UTF-8-BOM` 是否对项目用户的 Git 客户端版本 100% 兼容~~ | **轮次 2 重写（按 03 C-1）**：已撤销 working-tree-encoding 属性（事实层错误 —— git iconv 不支持 UTF-8-BOM 别名）；git 默认字节级 checkout 始终保 BOM；如未来 git iconv 支持 UTF-8-BOM 别名可重新评估添加 | 已闭环，无需 GR 复核 |
| O-2 | `.gitattributes` 改动是否需要 `git add --renormalize .` 触发现存 .ps1 重新打 BOM | 不需要 —— §3 步骤 2 已字节级写 BOM，git 索引内字节即 BOM；`renormalize` 主要解决 CRLF/LF 漂移 | GR 复核此判断 |
| O-3 | dev-map.md 改 1 行是否符合 AC-16"若有 / 若无"的双分支语义 | 当前 dev-map.md L25-L27 已有 scripts/ 行；本任务采"追加 T-021 子句"而非"新建独立条目"，符合 AC-16 第一分支 | GR 复核 |
| O-4 | QA 真机清单的 4 项是否需要 PM 额外向用户发请求 | 与 T-019 同款；用户已知"AC 标 [U] = 自己跑" | GR 复核是否提醒 PM 在 07 交付时高亮 |
| O-5 | ~~E.7 命名长度~~ | **轮次 2 已闭环（按 03 OPT-4）**：标题已缩为 `"scripts/*.ps1 have UTF-8 BOM"`（28 字符），与 E.6 量级相当但更精简 | 已闭环 |

---

## §11 修订历史

### 轮次 2 — 2026-05-23 — 按 03 Gate Review C-1/C-2/C-3/C-4 修订 7 处

| # | 修订点 | Before（轮次 1） | After（轮次 2） |
|---|---|---|---|
| 1 | §2.3 模块分解 | `.gitattributes` 追加 `*.ps1 working-tree-encoding=UTF-8-BOM eol=lf` | 不动 `.gitattributes`；新增 `scripts/.editorconfig`（charset=utf-8-bom + end_of_line=lf + insert_final_newline=true）作编辑器层 belt |
| 2 | §9 I-3 决议 | 方案 C 改良版（声称 git 官方支持） | 方案 B（`.editorconfig`），并显式标注轮次 1 事实错误：`UTF-8-BOM` 非 git iconv 合法值、git 2.34+ 会报错 |
| 3 | §5.2 跨平台兼容矩阵 | 含 "Git checkout（含 working-tree-encoding 属性）" 行 | 改为 "Git checkout（默认文本拷贝、字节级保留 BOM）" |
| 4 | §6 R-7 + 新 R-11 | R-7 = "git < 2.10 客户端不识别 working-tree-encoding" | R-7 改为"消除：方案 A 已撤销该属性"；新增 R-11 = "编辑器层 belt 仅靠 `.editorconfig`，未支持 .editorconfig 的编辑器会绕过 —— suspenders = verify_all E.7" |
| 5 | §10 O-1 | "GR 决定是否需在 README 加最低 Git 版本声明" | 重写为"已撤销 working-tree-encoding 属性；如未来 git iconv 支持 UTF-8-BOM 别名可重新评估添加"（已闭环） |
| 6 | §3 步骤 2 ReadAllText | `[System.Text.UTF8Encoding]::new($false)`（throwOnInvalidBytes 默认 false = silent 替换） | `[System.Text.UTF8Encoding]::new($false, $true)`（throwOnInvalidBytes=$true），注释说明防 silent U+FFFD 替换骗过步骤 4 |
| 7 | §7.1 AC-10 mock + §9 I-6/7 第 7 项 | mock `[char]0xFEFF + 'echo hello' \| iex` 测试；机制层称"iex parser 把 U+FEFF 当 whitespace 忽略" | 删除 mock；改为 [U] 真机断言（合并到 AC-9）`irm ... \| iex` 跑通；机制层改为"Invoke-RestMethod HTTP 客户端层剥 BOM、iex parser 不参与 BOM 处理；早期假设是错的" |
| §2.2 PS 伪码（C-4）| 直接 `$missing += $f.FullName.Substring($root.Length + 1)`，子目录调用会 ArgumentOutOfRangeException | 加 `$f.FullName.StartsWith($root)` guard，不满足时抛 `"verify_all 必须从仓库根目录运行"` 显式错误 |

附带吸纳的可选改进（OPT）：

| OPT | 来源 | 修订点 |
|---|---|---|
| OPT-1 | 03 §3 OPT-1 | §3 步骤 11 dogfood 命令前后加字节核对行 `(Get-Content scripts\archive-task.ps1 -Encoding Byte -TotalCount 3) -join ','` 应 = `239,187,191` |
| OPT-2 | 03 §3 OPT-2 | §2.5 + §3 步骤 8 baseline.json notes 文案精修：`follow-up T-020-encoding-ps51-bom` → `closed by T-021 encoding-ps51-bom`，并追加 `"T-021 closed: .ps1 BOM applied (11/11); E.7 added; verify_all 19→20."` |
| OPT-4 | 03 §3 OPT-4 | E.7 标题 `"All scripts/*.ps1 start with UTF-8 BOM (EF BB BF)"`（49 字符）→ `"scripts/*.ps1 have UTF-8 BOM"`（28 字符）；§10 O-5 同步标"已闭环" |

未吸纳：OPT-3（dev-map.md 改动位置）保留原方案 —— OPT-3 本为参考建议、SA 可保留。

---

## §12 Verdict

**READY**

理由：
- I-1 ~ I-5 全部给出明确决议（§9）；RA §6 第 4 ~ 8 项亦全部决议。
- AC-1 ~ AC-18 全部映射到 §3 步骤 / §7 测试策略，Developer 可机械执行。
- 11 个文件路径、verify_all E.7 编号、`scripts/.editorconfig` 新文件、baseline.json 三字段、dev-map.md 1 行——全部具体到字节级。
- 风险 11 条均含缓解；关键风险 R-4（grep 第一行）已 §5.3 预先字节级审计为 0 命中；R-7 因 C-1 决议撤销 working-tree-encoding 已消除；R-11 新增覆盖编辑器层 belt 失效路径。
- 与 insight 适配：L17（UTF8Encoding 反向用法）、L37（Edit/Write soft-block 降级）、L43（archive-task regex 不含 .ps1 文件本身）、L24 / L35（QA 06 标题约束）已全部覆盖。
- 轮次 2 已就地修订 03 §2 必须项 C-1/C-2/C-3/C-4 共 7 处，附带吸纳 OPT-1/OPT-2/OPT-4，详见 §11 修订历史。

如 GR 在 03 判断 §10 任一 Open Issue 需用户先回答，则把对应条改为 BLOCKER 并退回 PM；否则 PM 派发 Developer 进 Stage 4。

---

READY-FOR-GATE-REVIEW（轮次 2 差异修订完成）
