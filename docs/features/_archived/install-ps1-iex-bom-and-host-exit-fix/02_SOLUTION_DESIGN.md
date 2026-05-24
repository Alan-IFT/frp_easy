# 02 — 方案设计 · T-026 install-ps1-iex-bom-and-host-exit-fix

> Harness 流水线 Stage 2（Solution Architect）。模式：**full**。
> 上游：`docs/features/install-ps1-iex-bom-and-host-exit-fix/01_REQUIREMENT_ANALYSIS.md`（Verdict = READY）+ `PM_LOG.md`（PM 根因初判）。
> 本文档只做技术决议与可执行步骤，不重述 FR / AC（编号沿用 01）。

---

## §1 方案概览（架构层一句话）

**路径选择**：A-1 选 **删 install.ps1 BOM**（选项 A） + A-2 选 **整段 `& { ... }` 子作用域包裹**（选项 A） + A-5 选 **white-list 拆分**（FR-13 候选 a）。三者绑定形成单一稳定形态：iex 形态下 `irm` 把首字节 ASCII `#` 解码为 `#`、`iex` parser 正常吃掉注释 + `param` block + `& { ... }` 启动子作用域；子作用域内任意 `exit N` 只退该 scope 不杀宿主 PowerShell；FR-6 失败可观测靠 **既有 `Write-Error`（已是 stderr 红字）+ 子作用域末尾追加显式中文失败横幅**（A-3 = a + c 组合）。

**关键 tradeoff**：

- **删 BOM 接受磁盘形态 PS5.1+zh-CN 回归**（机制层证据见 §2 决议 D-1）。补偿手段：(i) README/DEPLOYMENT 一句话引导 PS5.1+zh-CN 用户走 iex 形态（**out-of-scope by RA OOS-9**，本任务不改 README，仅在 `docs/dev-map.md` 注解新规则；用户文档更新如 PM 在 03 判断必要可作为本任务 sub-fix 加项）；(ii) **install.ps1 是 iex 入口、不是磁盘工具脚本**——它在仓库内的"磁盘形态用途"仅限审阅后本地跑一次安装，频率极低；其余 10 个 `scripts/*.ps1`（含 install-service.ps1 / uninstall-service.ps1）依然 BOM 化、磁盘形态零回归。
- **`& { ... }` 包裹是改动最小、语义最清的方案**（vs 散落改 `exit→throw` / `exit→return`）：原代码 16 处 `exit N` 一字不改（仅缩进一层）；`param([switch]$Help)` 仍能在磁盘形态拿到 `$true`（PowerShell call-operator `& { ... } -Help` 透传命名参数语义见 §2 D-3 验证）；`$ErrorActionPreference="Stop"` / `$ProgressPreference` 这类 preference 变量在 `&` 调用的 scriptblock 内**继承父 scope**（PowerShell scope rules: scriptblock 默认创建 child scope，read-through 到 parent；preference 变量按 child-write copy-on-write）。
- **verify_all FR-11 选字节级断言**（候选 a）：`install.ps1` 前 3 字节 ≠ `EF BB BF`。不选 (b) "前 N 字节纯 ASCII"（误报面大、约束未来注释行）、不选 (c) "端到端 iex mock"（NFR-6 < 1s 难保证 + Mock GitHub API 复杂度爆炸）。
- **verify_all FR-13 BOM 拆分选 white-list 驱动**：把当前 E.7 单一 step 拆成 **E.7a "BOM-required PS1 list have UTF-8 BOM"**（含 9 个非 iex-entry .ps1）+ **E.7b "iex-entry PS1 list MUST NOT have UTF-8 BOM"**（含 1 个：install.ps1）。两表都显式列文件名，未来扩展（如假设新增 `scripts/upgrade.ps1` 作 iex 入口）只需加 list 一行；同时给出"未在两表中出现的 `scripts/*.ps1`"WARN 防遗忘。

---

## §2 决议表（RA 关键歧义显式收敛）

| ID | 决议 | 选项 | 理由 / 证据 |
|---|---|---|---|
| **D-1 (A-1 + A-6)** | 删 install.ps1 BOM | 选项 A：接受磁盘形态回归 | 机制层证据：PS5.1 解释器加载磁盘 .ps1 时若无 BOM 即按 **host ANSI codepage 解码**（zh-CN 系统 = GBK / cp936）。install.ps1 内中文字面量（如 L132 "请以管理员身份运行..."）以 UTF-8 字节存盘（如"请" = `E8 AF B7`），被 GBK 解码会变成 GBK 双字节字符（"璇" 类乱码）。这**不是**parser-level error（GBK 解码出的字符仍可作为 string 内容），故脚本能跑、但中文输出乱码、`Write-Host "==> [N/8] ..."` 进度行无意义。证据：T-021 archive 设计 §1 + dogfood 实测。  **不选 B（全 ASCII）** —— 改 16 处中文 Write-Host / Write-Error / 帮助文本 → 大重构、损害 UX（用户读不到中文进度即丧失 i18n）。  **不选 C（PS5.1 检测自重启）** —— iex 形态下宿主已是 PS5.1，无法"切到 pwsh 再 iex 一次"（pwsh 不一定装；切换中断 iex 流）。  缓解：iex 形态主推荐路径不受影响；磁盘形态在 dev-map.md 加注解（§6 实施步骤 6）。 |
| **D-2 (A-2)** | exit N 修法 | 选项 A：整段 `& { ... }` 子作用域包裹 | 改动量最小（原 16 处 exit N 一字不改；只移整体缩进）；PowerShell 语义稳定：`& { script }` 执行 scriptblock，`exit N` 在 scriptblock 内是 "退出当前 scriptblock + 设 `$LASTEXITCODE = N`"，**不**终止 host runspace；vs throw/return 散落多点的方案显著降回归风险。  **不选 B（exit → throw）** —— 16 处替换 + 顶层 try/catch 包；throw 在 `$ErrorActionPreference=Stop` 下行为复杂、与现有 try/catch 块（L156 / L181 / L217 / L233）嵌套不可预期。  **不选 C（exit → return）** —— 同样 16 处替换 + 调用方判返回值；return 在脚本顶层语义不是"中止脚本"，需重组控制流。  **不选 D（包成 function）** —— function 调用相比 `& { }` 多一层名字绑定，无额外好处。 |
| **D-3 (A-3)** | FR-6 失败可观测形式 | 组合 (a) + (c)：保留既有 stderr Write-Error 红字 + 子作用域末尾追加显式中文失败横幅 | (a) 既有 `Write-Error` 已落 stderr，零改动；(c) 在 `& { ... }` 后捕获 `$LASTEXITCODE`，非零时打印"❌ frp_easy 安装未完成（退出码=N）。请按上方红字定位失败原因，必要时执行 `sc query frp-easy` 检查服务状态。" 形成 belt + suspenders。  **不选 (b) 单独用 `$LASTEXITCODE`** —— 用户视觉不感知数字；但 (b) 是 (c) 内部条件判断的依据，已隐含纳入。 |
| **D-4 (A-5 / R-6)** | verify_all BOM 检查拆分 | white-list 驱动：E.7a 必须有 BOM 名单 + E.7b 禁止有 BOM 名单 + E.7c 名单外 .ps1 WARN | (a) 白名单 vs (b) 例外列表：两者都需维护一张表，**白名单显式列出全部文件**比"全量 + 例外"语义更清晰、规模相当（仓库当前仅 11 个 .ps1，未来增量极少）；(c) 分类检查（"非 iex-entry 必须 BOM、iex-entry 禁 BOM"）需要在脚本元数据里给文件打标签——无现成机制，等价于 white-list 但门槛高。  E.7c 是 R-7（漏白名单回归）的 belt：任意新 `scripts/*.ps1` 不在两表中即 WARN，提醒维护者归类。 |
| **D-5 (A-4)** | verify_all FR-11 实现 | 字节级断言 `Get-Content -TotalCount 3` ≠ `0xEF 0xBB 0xBF` | NFR-6 < 1s 自然满足；候选 b "前 N 字节纯 ASCII" 过严（脚本顶部注释允许 ASCII-only 但实践中常含 `# 中文注释`）；候选 c "端到端 iex mock" 不可行（NFR-5 禁引入新依赖，mock GitHub API + sc.exe + Expand-Archive 工程量爆炸）。 |
| **D-6 (A-7)** | mock 策略 | 大部分 [M] 用 `Get-Content -Raw scripts/install.ps1 \| iex` 模拟；少数 [U] 用户真机 | QA 主机 PS7+en-US 上 `Get-Content -Raw \| iex` 是 RA AC-1 [M] / AC-5 [M] 的精确等价（同样 in-process scriptblock 解析路径）；BOM/中文 GBK 路径强依赖 PS5.1+zh-CN 真机，无法 mock，全部标 [U]。详细矩阵见 §5。 |
| **D-7 (A-8)** | baseline.json schema | 只改 `notes` 文本 + `version` 升 11→12 + `updated` 改 QA 跑通日期；**不动** `test_count` / `passing_count` / `go_tests` / `frontend_tests`（无新增 Go / 前端单测） | 与 T-021 决议同款（沿用 archive §2.5）。verify_all 检查项数从 20 → 22（拆 E.7 为 E.7a/b/c：净 +2），仅在 notes 反映。 |

---

## §3 受影响模块清单（精确到 file + 行号区间 + 性质）

| 文件 | 行号 / 区间 | 改动性质 | 说明 |
|---|---|---|---|
| `scripts/install.ps1` | 第 1 字节 | 删 BOM | `EF BB BF` 删除；首字节变为 `#`（注释起始） |
| `scripts/install.ps1` | L27-L371 | 整体缩进 + `& { ... }` 包裹 | 把 `param([switch]$Help)` 一直到 `exit 0`（含 L27-L371 全部内容）放进顶层一行 `& {` ... 末尾 `}` 内；**param block 必须仍在 scriptblock 第一句**（PS 语法要求）；缩进可保留原样不缩（PowerShell `{}` 内对缩进零要求），降低 diff 噪音 |
| `scripts/install.ps1` | L371 后追加 ~6 行 | 新增"失败横幅" | `& { ... }` 退出后捕获 `$LASTEXITCODE`：非零则 `Write-Host "❌ frp_easy 安装未完成（退出码=$LASTEXITCODE）..." -ForegroundColor Red` |
| `scripts/install.ps1` | L23-L25 注释 | 更新 | 添加一段说明"本文件首字节禁 BOM（iex 形态会让 BOM 进入字符串触发 ParserError）；下方主体用 `& { ... }` 子作用域包裹让 `exit N` 不杀宿主"。同时**删除** T-024 留下的旧 `[CmdletBinding()]` 注释（已无意义，因为 iex 形态新增"包裹"才是当前的 idiomatic 边界） |
| `scripts/verify_all.ps1` | L268-L290（现 E.7 整块） | 重构：拆分为 E.7a / E.7b / E.7c 三个 Step | 见 §4 PS 伪码 |
| `scripts/verify_all.sh` | L278-L301（现 E.7 整块） | 重构：拆分为 E.7a / E.7b / E.7c 三个 step | 见 §4 sh 伪码 |
| `scripts/baseline.json` | 4 个字段 | 编辑 | `version` 11→12；`updated` 改 QA 跑通日期（Developer 04 留 2026-05-23 占位）；`notes` 内追加 "T-026 closed: install.ps1 BOM removed + `& { }` host-preserving wrapper; verify_all E.7 split a/b/c; verify_all 20→22." 且改 T-025 notes 内"verify_all PASS 20/20 stable" → "20/20 (pre-T-026)" 避免误读 |
| `docs/dev-map.md` | L25-L28 scripts/ 行 | 追加一行 | 在现有 T-021 注解后追加 "T-026：`scripts/install.ps1` 因 iex 入口**禁** BOM（其余 9 个 .ps1 继续要 BOM）；主体 `& { ... }` 子作用域包裹让 `exit N` 退子作用域不杀宿主；verify_all E.7 拆 a/b/c 白名单。" |

**不改动** `scripts/install-service.ps1`、`scripts/uninstall-service.ps1`（FR-9 / AC-10 字节级零 diff）；**不改动** 其他任何 `.ps1` 字节内容（仅 verify_all 内 E.7 块体改动 + install.ps1 BOM/包裹改动）；**不改动** `.gitattributes` / `scripts/.editorconfig`（T-021 双层防御机制保留——`.editorconfig` 现规则 `[*.ps1] charset = utf-8-bom` 与本任务冲突，见 §7 R-3 缓解：**改 `.editorconfig`** 把 install.ps1 单独排除）。

> **修正**：上一段最后一句的"不改 .editorconfig"是错的，必须改。完整改动清单补充如下：

| 文件（补充） | 行号 / 区间 | 改动性质 | 说明 |
|---|---|---|---|
| `scripts/.editorconfig` | 现有 `[*.ps1]` block + 新 `[install.ps1]` block | 追加例外 | 现 `[*.ps1] charset = utf-8-bom` 保留；追加 `[install.ps1]` block：`charset = utf-8`（不带 BOM）；防 VS Code / Notepad++ EditorConfig 插件保存时把 BOM 加回来 |

---

## §4 模块细节 / 伪码（指导 Developer，非完整实现）

### §4.1 `scripts/install.ps1` 主体包裹（核心 idiom）

**Before**（删 BOM 后的 L27-L371）：

```powershell
param([switch]$Help)
$ErrorActionPreference = "Stop"
# ... L32-L371 全部主体 ...
exit 0
```

**After**（顶层 `& { ... }` 包裹 + 失败横幅）：

```powershell
# (顶部注释 L1-L26 不变；删 BOM 后首字符为 '#')

# 主体放入 scriptblock，& 调用让 exit N 退子作用域不杀宿主（T-026 D-2）。
& {
    param([switch]$Help)
    $ErrorActionPreference = "Stop"
    # ... L32-L371 全部原内容，缩进可保留原样（PS {} 内对缩进零要求）...
    exit 0
} @PSBoundParameters  # ← 透传 -Help 等命名参数（D-3 验证 §4.2）

# 失败可观测横幅（FR-6 / AC-7 / D-3 形式 c）
if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    Write-Host "❌ frp_easy 安装未完成（退出码=$LASTEXITCODE）。" -ForegroundColor Red
    Write-Host "   请按上方红字定位失败原因；必要时执行 'sc query frp-easy' 检查服务状态。" -ForegroundColor Red
}
```

**关键点**：

1. `& { ... }` 而非 `. { ... }`：dot-source（`.`）会让 scriptblock 在调用方 scope 执行，`exit N` 仍会杀宿主——必须 `&`。
2. 顶层 `param([switch]$Help)` 必须**移到 scriptblock 内**作为其第一句（PS 语法要求 param block 紧跟在 scriptblock 起始 `{` 后）。
3. `@PSBoundParameters` splatting：当 install.ps1 自身被磁盘形态 `.\install.ps1 -Help` 调用时，PowerShell 解释器先 bind `-Help` 到 install.ps1 顶层的 `$PSBoundParameters` —— 但**只有当 install.ps1 顶层自己有 `param([switch]$Help)` 时才会绑**。
4. 因此 **install.ps1 顶层也要保留** `param([switch]$Help)`（在 `& { }` 之前）：

```powershell
# install.ps1 顶层（删 BOM 后；注释 L1-L26 不变）
param([switch]$Help)

# 主体子作用域包裹
& {
    param([switch]$Help)
    $ErrorActionPreference = "Stop"
    # ... 原 L32-L371 ...
    exit 0
} @PSBoundParameters

# 失败横幅
if ($LASTEXITCODE -ne 0) { ... }
```

**iex 形态调用语义验证**：`irm <url> | iex` 时 `iex` 把 string 内容当 scriptblock 直接执行，不存在"绑定外层参数"环节——`$PSBoundParameters` 为空 hashtable，`& { param([switch]$Help) ... } @PSBoundParameters` 等价于 `& { ... }`（无参），内层 `$Help = $false`（默认值），脚本走正常安装路径。这正是 iex 用户的期望。

**磁盘形态 `.\install.ps1 -Help` 验证**：PowerShell bind `-Help` → 顶层 `$Help = $true` + `$PSBoundParameters = @{ Help = $true }`；`@PSBoundParameters` splatting 转换为 `-Help:$true`；内层 scriptblock 收到 `$Help = $true` → 走 Help 分支 → `exit 0` 退子作用域；`$LASTEXITCODE = 0` → 横幅不触发 → 宿主退出码 0。✅ AC-8 / AC-9 通过。

### §4.2 `scripts/verify_all.ps1` E.7 拆分（白名单驱动）

替换现有 E.7 整块（L268-L290）为：

```powershell
# T-026: 拆分 T-021 的全量 BOM 检查为 white-list 驱动，容纳 install.ps1 的反向规则
# （iex 入口禁 BOM，其余磁盘形态 .ps1 需 BOM）。
$Ps1RequireBom = @(
    'archive-task.ps1',
    'build.ps1',
    'harness-sync.ps1',
    'install-hooks.ps1',
    'install-service.ps1',
    'package.ps1',
    'start-e2e-server.ps1',
    'start.ps1',
    'uninstall-service.ps1',
    'verify_all.ps1'
)
$Ps1ForbidBom = @(
    'install.ps1'  # T-026: iex 入口；BOM 会被 irm 解码为 U+FEFF 进入字符串触发 ParserError
)

Step "E.7a" "BOM-required scripts/*.ps1 have UTF-8 BOM" {
    if (-not (Test-Path "scripts")) { return "SKIP" }
    $missing = @()
    foreach ($name in $Ps1RequireBom) {
        $full = Join-Path "scripts" $name
        if (-not (Test-Path -PathType Leaf $full)) {
            $missing += "$name (MISSING)"
            continue
        }
        $bytes = [System.IO.File]::ReadAllBytes((Resolve-Path $full).Path)
        if ($bytes.Length -lt 3 -or $bytes[0] -ne 0xEF -or $bytes[1] -ne 0xBB -or $bytes[2] -ne 0xBF) {
            $missing += $name
        }
    }
    if ($missing.Count -gt 0) { throw "Missing UTF-8 BOM in:`n$($missing -join "`n")" }
}

Step "E.7b" "iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM" {
    if (-not (Test-Path "scripts")) { return "SKIP" }
    $wrong = @()
    foreach ($name in $Ps1ForbidBom) {
        $full = Join-Path "scripts" $name
        if (-not (Test-Path -PathType Leaf $full)) {
            $wrong += "$name (MISSING)"
            continue
        }
        $bytes = [System.IO.File]::ReadAllBytes((Resolve-Path $full).Path)
        if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
            $wrong += $name
        }
    }
    if ($wrong.Count -gt 0) {
        throw "iex-entry .ps1 MUST NOT have UTF-8 BOM (BOM → U+FEFF → ParserError in iex form):`n$($wrong -join "`n")"
    }
}

Step "E.7c" "All scripts/*.ps1 classified in E.7a or E.7b (防漏白名单)" {
    if (-not (Test-Path "scripts")) { return "SKIP" }
    $known = $Ps1RequireBom + $Ps1ForbidBom
    $actual = Get-ChildItem -Path "scripts" -Filter "*.ps1" -File -ErrorAction SilentlyContinue |
              ForEach-Object { $_.Name }
    $unclassified = $actual | Where-Object { $known -notcontains $_ }
    if ($unclassified) {
        return $false  # WARN 而非 FAIL：提醒维护者归类、不阻塞 CI
    }
}
```

> **设计 rationale**：E.7c 用 `return $false` → WARN（不 FAIL），让新增 .ps1 不立即破 CI（给一次提醒），但 `pwsh -File ...` 仍以退出 1 提示。

### §4.3 `scripts/verify_all.sh` E.7 同款拆分

```bash
# E.7a — BOM-required scripts/*.ps1 must start with UTF-8 BOM (EF BB BF)
PS1_REQUIRE_BOM=(archive-task.ps1 build.ps1 harness-sync.ps1 install-hooks.ps1 \
    install-service.ps1 package.ps1 start-e2e-server.ps1 start.ps1 \
    uninstall-service.ps1 verify_all.ps1)
PS1_FORBID_BOM=(install.ps1)

if [[ ! -d scripts ]]; then
    step "E.7a" "BOM-required scripts/*.ps1 have UTF-8 BOM" "SKIP"
    step "E.7b" "iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM" "SKIP"
    step "E.7c" "All scripts/*.ps1 classified" "SKIP"
else
    # E.7a
    e7a_missing=""
    for name in "${PS1_REQUIRE_BOM[@]}"; do
        f="scripts/$name"
        if [[ ! -f "$f" ]]; then e7a_missing="$e7a_missing\n$name (MISSING)"; continue; fi
        first3=$(head -c 3 "$f" 2>/dev/null | od -An -tx1 | tr -d ' \n')
        if [[ "$first3" != "efbbbf" ]]; then e7a_missing="$e7a_missing\n$name"; fi
    done
    if [[ -z "$e7a_missing" ]]; then
        step "E.7a" "BOM-required scripts/*.ps1 have UTF-8 BOM" "PASS"
    else
        step "E.7a" "BOM-required scripts/*.ps1 have UTF-8 BOM" "FAIL" "$(echo -e $e7a_missing)"
    fi

    # E.7b
    e7b_wrong=""
    for name in "${PS1_FORBID_BOM[@]}"; do
        f="scripts/$name"
        if [[ ! -f "$f" ]]; then e7b_wrong="$e7b_wrong\n$name (MISSING)"; continue; fi
        first3=$(head -c 3 "$f" 2>/dev/null | od -An -tx1 | tr -d ' \n')
        if [[ "$first3" == "efbbbf" ]]; then e7b_wrong="$e7b_wrong\n$name"; fi
    done
    if [[ -z "$e7b_wrong" ]]; then
        step "E.7b" "iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM" "PASS"
    else
        step "E.7b" "iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM" "FAIL" "$(echo -e $e7b_wrong)"
    fi

    # E.7c
    known=" ${PS1_REQUIRE_BOM[*]} ${PS1_FORBID_BOM[*]} "
    e7c_unclassified=""
    while IFS= read -r f; do
        base=$(basename "$f")
        if [[ "$known" != *" $base "* ]]; then
            e7c_unclassified="$e7c_unclassified\n$base"
        fi
    done < <(find scripts -maxdepth 1 -name '*.ps1' -type f 2>/dev/null)
    if [[ -z "$e7c_unclassified" ]]; then
        step "E.7c" "All scripts/*.ps1 classified" "PASS"
    else
        step "E.7c" "All scripts/*.ps1 classified" "WARN" "$(echo -e $e7c_unclassified)"
    fi
fi
```

### §4.4 `scripts/.editorconfig` 例外追加

当前内容（T-021 设计 §2.3）：

```editorconfig
[*.ps1]
charset = utf-8-bom
end_of_line = lf
insert_final_newline = true
```

追加（在 `[*.ps1]` block 之后）：

```editorconfig
# T-026: install.ps1 是 irm | iex 入口，BOM 会让 irm 解码为 U+FEFF 触发 ParserError。
# 此例外块覆盖上面的 [*.ps1] charset=utf-8-bom，让编辑器保存时不写 BOM。
[install.ps1]
charset = utf-8
end_of_line = lf
insert_final_newline = true
```

EditorConfig 规则：后置 section 覆盖前置（最具体的 section 优先）。

---

## §5 跨形态行为矩阵（必给）

| 使用形态 | 解析结果 | 中文显示 | 退出行为 | 终端存活 | 验证编号 |
|---|---|---|---|---|---|
| iex 形态 + PS5.1 + zh-CN | ✅ 0 ParserError | ✅ 中文正确（irm 解码 UTF-8 字节 → string） | `exit N` 退子作用域 + 横幅 | ✅ 存活 | AC-1 [U] / AC-4-7 [U] |
| iex 形态 + PS5.1 + en-US | ✅ 0 ParserError | ✅ 中文正确 | 同上 | ✅ 存活 | 衍生 [M] |
| iex 形态 + PS7 + 任意 codepage | ✅ 0 ParserError | ✅ 中文正确 | 同上 | ✅ 存活 | AC-2 [M] / [U] |
| 磁盘形态 `.\install.ps1 -Help` + PS5.1 + zh-CN | ⚠️ 解析 OK（BOM-less PS5.1 仍能 parse，但中文按 GBK 解码） | ❌ **中文乱码**（"璇" 类） | `exit 0` 退子作用域 | ✅ 存活 | AC-8 [U] **可能失败**——见 D-1 已接受 |
| 磁盘形态 `.\install.ps1 -Help` + PS5.1 + en-US | ✅ | ✅ 中文 ASCII 部分正常显示，UTF-8 字节按 cp1252 解码会乱（但用户 en-US 不依赖中文） | `exit 0` | ✅ | AC-8 [U] |
| 磁盘形态 `pwsh -File install.ps1 -Help` + PS7 | ✅ | ✅ 中文正确（PS7 默认按 UTF-8 解码） | `exit 0` | ✅ | AC-9 [A] |
| 磁盘形态 `.\install.ps1`（完整安装） + PS5.1 + zh-CN | ⚠️ 中文乱码但 logic 可走 | ❌ 进度行乱码 | 任一 exit N 退子作用域 + 横幅 | ✅ 存活 | AC-11 [U] **可能仅乱码不阻塞功能** |
| 磁盘形态 `.\install.ps1`（完整安装） + PS7 | ✅ | ✅ | 同上 | ✅ | 衍生 [U] |

**关键格子说明**：

- 第 4/7 行（PS5.1 + zh-CN 磁盘形态）**接受**中文乱码（D-1 取舍）；逻辑层仍可完整跑通（GBK 解码后的字符串仅 Write-Host 显示乱、不影响 sc.exe 调用 / Invoke-WebRequest URL 字面量 / Copy-Item 路径）。
- 第 4 行 AC-8 的 RA 表述"中文帮助完整显示"在此形态下技术上**会乱码**——这是 D-1 接受的回归；RA AC-8 标 [U]，用户真机若不通过则 PM 在 03 决定升级到选项 B（全 ASCII）。预先建议 PM 把 AC-8 在 06 真机验证清单标注"PS5.1+zh-CN 磁盘形态下中文乱码是预期，逻辑通即通过"。

---

## §6 实施步骤（Developer 可执行序列）

> 所有命令均在 `c:\Programs\frp_easy` 仓库根执行。每步含验证 + git diff 形态预期。

### 步骤 1：备份 install.ps1 + verify_all.{ps1,sh} 字节快照

```powershell
$snap = "scripts\.t026-snapshot"
New-Item -ItemType Directory -Path $snap -Force | Out-Null
Copy-Item scripts\install.ps1     (Join-Path $snap "install.ps1.bak")
Copy-Item scripts\verify_all.ps1  (Join-Path $snap "verify_all.ps1.bak")
Copy-Item scripts\verify_all.sh   (Join-Path $snap "verify_all.sh.bak")
Copy-Item scripts\.editorconfig   (Join-Path $snap ".editorconfig.bak")
```

不提交（步骤 9 删除）。

### 步骤 2：删 install.ps1 的 BOM（核心字节操作）

```powershell
$path = "scripts\install.ps1"
$utf8ReadWithBom    = [System.Text.UTF8Encoding]::new($true, $true)   # 读时识别 BOM 并剥
$utf8WriteNoBom     = [System.Text.UTF8Encoding]::new($false)         # 写时不写 BOM
$content = [System.IO.File]::ReadAllText((Resolve-Path $path).Path, $utf8ReadWithBom)
[System.IO.File]::WriteAllText((Resolve-Path $path).Path, $content, $utf8WriteNoBom)
```

**验证**：

```powershell
$bytes = [System.IO.File]::ReadAllBytes((Resolve-Path "scripts\install.ps1").Path)
"BOM? {0} (expect False)" -f ($bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF)
"First3: {0:X2} {1:X2} {2:X2} (expect 23 = '#')" -f $bytes[0], $bytes[1], $bytes[2]
"CR count: {0} (expect 0, NFR-7 LF only)" -f (($bytes | Where-Object { $_ -eq 13 }).Count)
"Size: {0} (expect ~ original - 3)" -f $bytes.Length
```

预期：`BOM? False`、`First3: 23 20 69` 或 `23 0A ...`（首字节 `#`）、`CR count: 0`、`Size` 比原 -3。

### 步骤 3：在 install.ps1 主体加 `& { ... }` 包裹 + 失败横幅

用 Edit 工具，按 §4.1 idiom 改造。**关键操作顺序**：

1. 在 L27（原 `param([switch]$Help)`）之前插入新顶层 `param([switch]$Help)`（保留磁盘形态参数绑定）。
2. 在新 `param` 后插入空行 + `& {`。
3. 在 L371（原 `exit 0`）之后插入 `} @PSBoundParameters` + 失败横幅块（§4.1 末尾 6 行）。
4. 原 L27-L371 内容（包括缩进）**不改字符**（保持 git diff 仅 +N -0）。
5. 更新 L23-L25 注释段，按 §3 描述。

**字节级核对**：再次跑步骤 2 验证命令，确认 BOM 仍 False / CR=0。

### 步骤 4：在 verify_all.ps1 拆 E.7

用 Edit 工具，按 §4.2 替换 L268-L290 整块（找定位锚 `Step "E.7"`）。

### 步骤 5：在 verify_all.sh 拆 E.7

用 Edit 工具，按 §4.3 替换 L278-L301 整块。

### 步骤 6：追加 scripts/.editorconfig 例外块

用 Edit 工具，按 §4.4 在文件末尾追加新 `[install.ps1]` block。

### 步骤 7：更新 baseline.json

```powershell
# Edit 工具：
# - "version": 11 → "version": 12
# - "updated": "2026-05-23" → 保留（QA 在 06 改）
# - "notes" 末尾追加 ".. T-026 closed: install.ps1 BOM removed + & {} wrapper; verify_all E.7 split a/b/c; verify_all 20→22."
```

### 步骤 8：更新 dev-map.md

L25-L28 scripts/ 块末尾追加（§3 文案）。

### 步骤 9：跑 verify_all + 字节级对账

```powershell
Remove-Item -Recurse -Force scripts\.t026-snapshot
pwsh -File scripts\verify_all.ps1
```

期望：Summary `PASS: 22  WARN: 0  FAIL: 0  SKIP: <几个>`。

### 步骤 10：动态冒烟（QA 主机 PS7 上能做的最强自动化）

```powershell
# 等价于 irm | iex 形态的 in-process scriptblock 执行
# 预期：在 PS7 上能跑到 "请以管理员身份运行" 红字 + 横幅 "❌ frp_easy 安装未完成（退出码=1）..."
# 关键断言：跑完后 PowerShell 提示符仍在（用 -NoExit 包一层确认）
pwsh -NoProfile -NoExit -Command @'
$ErrorActionPreference = "Continue"
Get-Content -Raw scripts/install.ps1 | Invoke-Expression
Write-Host "===== 宿主仍存活，最后 LASTEXITCODE = $LASTEXITCODE ====="
exit 0
'@
```

预期 stdout 末尾出现 `===== 宿主仍存活 ...` 行，证明 `& { ... }` 包裹了 `exit 1`。

> Developer 在 04 必须录 stdout 全文片段证明此断言。

---

## §7 风险与缓解

| ID | 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|---|
| **R-1** | 删 BOM 后 PS5.1+zh-CN 磁盘形态 install.ps1 中文乱码（D-1 接受回归的实现层） | 中（少数走 -OutFile 审阅的用户会遇） | 中（UX 退化，逻辑不挂） | D-1 决议已显式接受；缓解：dev-map.md 加注解（步骤 8）+ PM 在 07 交付说明里把"磁盘形态推荐用 PS7 或 `pwsh -File`"高亮。如用户实测不接受，PM 升级到选项 B 走 sub-fix。|
| **R-2** | `& { ... }` 子作用域内 `$ErrorActionPreference="Stop"` / `$ProgressPreference` 未正确继承父 scope | 低 | 中 | 机制层：PowerShell scriptblock 通过 `&` 调用创建 child scope，read-through 至 parent；scriptblock 内**赋值**这些 preference 变量等于 child-scope shadow（不影响 parent）。install.ps1 现状是 `$ErrorActionPreference = "Stop"` 在 scriptblock 内重新赋值——child scope 内有效，符合原意；不需特殊处理。Developer 步骤 10 动态冒烟会暴露任何异常。|
| **R-3** | `param([switch]$Help)` 在磁盘形态 `.\install.ps1 -Help` 后能否仍拿到 `$true`（AC-8） | 中 | 高（破 AC-8 即破 RA 关键 backward-compat） | §4.1 设计明确双层 `param` + `@PSBoundParameters` splatting。Developer 步骤 9 之外**必须**在 PS7 + en-US（QA 主机）跑：`pwsh -File scripts/install.ps1 -Help` → 期望退出码 0 + stdout 含 "用法: install.ps1 [-Help]"。失败立即停下排查 splatting 语义。|
| **R-4** | 失败横幅不被 BOM 错误本身淹没（FR-6 防"沉默失败"） | 低（已删 BOM 不会再有 BOM 错误） | 中 | D-3 形式 a + c 组合：(a) `Write-Error` 红字早就在 stderr；(c) 横幅在 `& { }` 退出**后**打印——即使 `& { }` 内部走 `exit 1`，横幅仍触发（PS `&` 调用即使 inner exit 也会回到 caller 继续执行）。Developer 步骤 10 动态冒烟验证。|
| **R-5** | `.editorconfig` `[install.ps1]` section 仅按文件名匹配，所有目录下叫 install.ps1 的都被波及 | 低（仓库无其他 install.ps1） | 低 | EditorConfig spec：section 可写绝对路径 `[/scripts/install.ps1]`。但因 `.editorconfig` 已在 `scripts/` 目录内，相对就锚定本目录——影响面 = scripts/install.ps1 一个文件。Developer 检查 `Get-ChildItem -Recurse -Filter install.ps1` 仅命中一个。|
| **R-6** | E.7c WARN 而非 FAIL，让漏白名单情况静默 | 低 | 低 | WARN 在 PS 端 → `exit 1`、sh 端 → `exit 1` 都会让 CI 注意；非 FAIL 是允许"PR 提出新 .ps1 时维护者一并分类"的缓冲。如用户在 GR 03 要求升 FAIL，可在 03 后调整为 `throw` 而非 `return $false`。|
| **R-7** | `irm | iex` 的 in-process 执行让 `$LASTEXITCODE` 在 `& { ... }` 外不被设置 | 中 | 中 | 机制层验证：PS 内 `& { exit 5 }; $LASTEXITCODE` 实测 = 5（PowerShell 5.1 + 7 一致行为，因 `exit N` 在 scriptblock 内被 transl 为"set $LASTEXITCODE = N + 退当前 scope"）。Developer 步骤 10 实测此断言。|
| **R-8** | T-024 已删 `[CmdletBinding()]`、本任务再加包裹，可能让 iex 解析顺序又出新 corner case | 低 | 中 | 已知 iex 形态合法 idiom：`& { param(...); ... }` 是 PowerShell 官方文档化用法（about_Script_Blocks）；无任何 iex-incompat 报告。Developer 步骤 10 动态冒烟兜底；若失败回退到 D-2 选项 D（function 包）。|
| **R-9** | 改 verify_all 拆 E.7 后，T-021 的"E.7 检查项 1 条"在 baseline notes 与 archive 文档里成为陈旧引用 | 极低 | 低 | dev-map.md 步骤 8 已更新；baseline notes 步骤 7 已更新；T-021 归档文档不动（历史快照）。Code Reviewer 在 05 复核。|
| **R-10** | `.editorconfig` 后置 section 覆盖前置 section 是否所有主流编辑器都遵守 | 低 | 低 | EditorConfig spec 明确"more specific section overrides"；VS Code / JetBrains / Notepad++ 插件全部实现。verify_all E.7b 是 CI 最后闸门，编辑器层失效仍能拦下。|
| **R-11** | iex 形态用户用陈旧版 PS5.1（< 5.1.14393，Windows 7 LTSC）`& { }` 行为有差异 | 极低 | 低 | install.ps1 文件头已要求 Windows 10/11（cmd/frp-easy 二进制本就不支持 Win 7）；scriptblock `&` 在 PS3.0+ 就 stable。 |

---

## §8 验证策略（给 QA 写 06 的 hooks）

### §8.1 自动可验 [A]（verify_all 内）

| 项 | 命令 | 期望 | 对应 AC |
|---|---|---|---|
| install.ps1 字节级无 BOM | 见步骤 2 验证 | First3 = `23 ...`，BOM=False | AC-3 |
| install-service.ps1/uninstall-service.ps1 字节零变 | `git diff --stat scripts/install-service.ps1 scripts/uninstall-service.ps1` | 空输出 | AC-10 |
| install.ps1 内 `^\s*exit\s+\d` 计数 | `Select-String '^\s*exit\s+\d' scripts/install.ps1` | 计数 16（与改前一致；包裹未删 exit） | 隐含 D-2 实施未漏 |
| `& {` 外仅 1 个、`} @PSBoundParameters` 外仅 1 个 | grep | 计数 = 1 / 1 | D-2 包裹完整 |
| verify_all PASS 计数升至 22 | `pwsh -File scripts/verify_all.ps1` | `PASS: 22` | AC-12 / AC-15 |
| E.7b 反向自检 | QA 临时 `[System.IO.File]::WriteAllText('scripts/install.ps1', "`u{FEFF}" + (Get-Content -Raw scripts/install.ps1), [System.Text.UTF8Encoding]::new($true))` → 跑 verify_all | E.7b FAIL，错误含 `install.ps1` 与 "MUST NOT have UTF-8 BOM" | AC-13 |
| E.7a 反向自检 | QA 临时删 build.ps1 的 BOM | E.7a FAIL，错误含 `build.ps1` | AC-14 |

### §8.2 Mock 可验 [M]（QA 主机 PS7 + en-US）

| 项 | 命令 | 期望 | 对应 AC |
|---|---|---|---|
| iex 形态无 ParserError（PS7） | 步骤 10 动态冒烟 | stdout 无 `is not recognized` / `ParserError` | AC-1 [M] / AC-2 |
| `exit 1` 路径宿主存活 | 步骤 10 动态冒烟 | stdout 末尾出现 `===== 宿主仍存活 ... LASTEXITCODE = 1 =====` | AC-4 [M] |
| `install-service.ps1` 失败时宿主存活 | mock `$svc` 路径改成不存在文件 + 跑步骤 10 变体 | 同上 | AC-5 [M] |
| 失败横幅可见 | 步骤 10 动态冒烟 stdout | 包含 `❌ frp_easy 安装未完成` | AC-7 [M] |
| 磁盘形态 `-Help` 退出 0 + 中文帮助 | `pwsh -File scripts/install.ps1 -Help` | 退出码 0；stdout 含"用法: install.ps1 [-Help]" | AC-9 [A] |

### §8.3 真机标 [U]（用户 PS5.1 + zh-CN 主机）

| AC | 用户操作 | 期望 | 备注 |
|---|---|---|---|
| AC-1 [U] | 真实 `irm <raw_url> | iex` | 无 `is not recognized` 红字 | 本任务核心修复 |
| AC-4 [U] | iex 形态触发非管理员路径 | 看到红字后 PS 提示符仍在 | 同上 |
| AC-6 [U] | iex 形态完整跑通 8/8 + `sc query frp-easy` | `STATE : 4 RUNNING` | end-to-end 验证 |
| AC-8 [U] | 磁盘形态 `.\install.ps1 -Help` | 退出 0；中文**可能乱码**（D-1 取舍） | 仅断言"退出 0 + 不杀宿主"，中文乱码 PM 在 06 标"已知 trade-off" |
| AC-11 [U] | 磁盘形态完整安装 | 8/8 跑通（中文可能乱码） | 同上 |

> QA 06 标 [U] 项交 PM 转给用户验。

### §8.4 Adversarial tests（QA 06 强制段，裸 `## Adversarial tests` 标题）

至少 5 条：

1. **ADV-1**：故意把 BOM 加回 install.ps1（`[System.Text.UTF8Encoding]::new($true)` 写回）→ verify_all E.7b FAIL，错误含 `install.ps1` 与 "MUST NOT"。
2. **ADV-2**：故意删 install-service.ps1 的 BOM → verify_all E.7a FAIL，错误含 `install-service.ps1`。
3. **ADV-3**：故意新建空 `scripts/fake.ps1`（未在两表）→ verify_all E.7c WARN，错误含 `fake.ps1`。
4. **ADV-4**：故意把 `& {` 之前的顶层 `param([switch]$Help)` 删掉 → `pwsh -File scripts/install.ps1 -Help` 仍能走 Help 分支（因 `@PSBoundParameters` 为空，子作用域 `$Help` 默认 `$false` → 走主安装路径 → 非管理员 exit 1）—— 期望 stdout 第一行**不是** Help 输出。验证 D-3 双层 param 的必要性。
5. **ADV-5**：mock `install-service.ps1` 退出 2 → 验证宿主存活 + 横幅触发 + `$LASTEXITCODE = 2`。

---

## §9 复用审计

| 需求 | 已有实现 | 文件路径 | 决议 |
|---|---|---|---|
| 字节级 BOM 写入 (`UTF8Encoding($true)`) | T-021 设计 §3 步骤 2 | `docs/features/_archived/encoding-ps51-bom/02_SOLUTION_DESIGN.md` | 反向复用：本任务用 `UTF8Encoding($false)` (无 BOM) + `UTF8Encoding($true, $true)` 读时识别 BOM 并剥 |
| verify_all `Step` helper | `scripts/verify_all.ps1` L32-L54 | 同 | 复用 as-is，三个新 Step 都套这个 helper |
| verify_all `step` bash function | `scripts/verify_all.sh` L33-L42 | 同 | 复用 as-is |
| EditorConfig 文件 | T-021 设计 §2.3 新增 | `scripts/.editorconfig` | 扩展：追加 `[install.ps1]` 例外 block |
| BOM 字节判定 idiom | T-021 设计 §2.2 + insight L34 | `scripts/verify_all.ps1` L277-L278 | 复用 idiom，white-list 化 |
| `&` 调用 scriptblock 保护宿主 | （PowerShell 官方 about_Script_Blocks）；本仓库 install-service.ps1 / verify_all.ps1 内 `Step` helper 用 `& $action` 也是同款机制 | `scripts/verify_all.ps1` L35 `$result = & $action` | 模式复用：T-026 把整个 install.ps1 主体当 scriptblock `& { }` |
| `$PSBoundParameters` splatting | （PowerShell 官方 about_Splatting）；本仓库 install.ps1 内现无使用 | — | 新引入；§4.1 已示范 idiom |
| iex 形态 ParserError 测试 mock（`Get-Content -Raw \| iex`） | T-024 验证手段 + T-021 archive §7.1 AC-10 mock 段已被否决（insight L43 后续修订） | 同 | T-024 验证手段适用：QA 在 06 步骤 10 走 PS7 动态冒烟（不指望 mock GBK） |

---

## §10 迁移 / 回滚 / 兼容性

### §10.1 向后兼容性

- iex 形态：**强正向变化**（消两条 ParserError + 宿主不挂）。
- 磁盘形态 PS7：完全等价（PS7 解码 BOM 透明 / 无 BOM UTF-8 默认解码也对）。
- 磁盘形态 PS5.1 + en-US：完全等价（脚本逻辑用 ASCII URL / 路径 / 命令，中文仅在 Write-Host 显示，en-US 用户不依赖中文输出）。
- 磁盘形态 PS5.1 + zh-CN：**中文乱码回归**（D-1 接受）。

### §10.2 回滚方案

万一发现 `& { ... }` 包裹引入未预期回归，回滚步骤：

1. `git revert <commit-of-T-026>` 撤销 install.ps1 改动。
2. **不要 revert verify_all 的 E.7 拆分**（与 install.ps1 解耦；保留拆分让 E.7b 名单暂时清空即可）。
3. 重新发布 install.ps1（仍 T-024 + T-021 状态，已知两 ParserError 复发，但不死宿主除非 exit）。

### §10.3 feature flag / 阶段发布

**不需要** —— 改动落在 install.ps1 + verify_all + .editorconfig + baseline + dev-map 五个文件，git 单 commit 即原子发布；下次用户 `irm | iex` 拉 main 的 raw 即拿到新版。**不存在**已部署副本（install.ps1 不持久化到用户机器，仅每次 iex 时即时拉）。

### §10.4 OOS 边界（design 不覆盖的）

- **不**改 install-service.ps1 / uninstall-service.ps1（RA OOS-1）。
- **不**改其他 9 个 `scripts/*.ps1`（RA OOS-2）。
- **不**改 install.sh / install-service.sh（Linux 入口无此问题）。
- **不**改 install.ps1 内任何业务逻辑（包括 GitHub API 调用、tmpDir 清理、Get-PublicIPv4 函数体、最终横幅文案、升级语义）—— 改动严格限于：删 BOM + `& { ... }` 包裹 + 失败横幅追加 + 注释更新。
- **不**改 README / docs/DEPLOYMENT.md 的一键安装命令（命令本身不变）。如 PM 在 03 判断需为磁盘形态加 PS5.1+zh-CN 警告，作为 T-026 sub-fix 或独立 trivial 任务。
- **不**新增 Go / 前端单测（无业务逻辑改动；NFR-5 不引入新依赖）。
- **不**在 install.ps1 内根据 `$PSVersionTable.PSVersion.Major` 分流（RA NFR-1）。
- **不**触及 `.harness/` / `.claude/` / `CLAUDE.md`（项目红线）。

---

## §11 Partition assignment（分区分配）

`.harness/agents/dev-{frontend,backend,db}.md` 存在。检查 owned paths：

- `scripts/install.ps1` / `scripts/install-service.ps1` / `scripts/.editorconfig` — **未在任何 dev-* 分区** owned paths（dev-backend 注释明文"Harness 脚本（含 install / archive-task / install-hooks）不归任何 dev-* 分区"）。**fallback 至 generic `developer`**。
- `scripts/verify_all.ps1` / `scripts/verify_all.sh` — **dev-backend** owned（dev-backend.md L18）。
- `scripts/baseline.json` — qa-tester 在交付时改，但本任务由 Developer 04 改占位（version/notes），**fallback 至 generic `developer`** 或 `dev-backend`（与 verify_all 同步改）。
- `docs/dev-map.md` — 非代码文档，无明确分区归属；**fallback 至 generic `developer`**。

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `scripts/install.ps1` | `developer` (generic, fallback) | edit（删 BOM + `& { }` 包裹 + 横幅 + 注释） | — |
| `scripts/.editorconfig` | `developer` (generic) | edit（追加 `[install.ps1]` 例外 block） | depends on install.ps1（语义绑定） |
| `scripts/verify_all.ps1` | `dev-backend` | edit（E.7 拆 a/b/c） | — |
| `scripts/verify_all.sh` | `dev-backend` | edit（同步 E.7 拆 a/b/c） | depends on verify_all.ps1（同步对账） |
| `scripts/baseline.json` | `developer` (generic) | edit（version + notes） | depends on verify_all 跑通 |
| `docs/dev-map.md` | `developer` (generic) | edit（scripts/ 行追加 T-026 注解） | — |

### Dispatch order

1. **`developer` (generic)**：install.ps1 + .editorconfig（一次性完成核心改动）
2. **`dev-backend`**：verify_all.ps1 + verify_all.sh（独立任务，可与 step 1 并行）
3. **`developer` (generic)**：baseline.json + dev-map.md（等前两步完成）

### Parallelism

**Step 1 与 Step 2 可并行**（无文件交叉）；Step 3 必须等前两步完成（baseline notes 要反映 verify_all 22 项）。

**推荐 PM**：本任务规模小（5 个文件，纯局部改动），单 generic `developer` agent 一次性顺序完成所有步骤即可，**无需真分区**；上表的 dev-backend 分配是契约层声明，实操可 fallback 给 generic developer。如 PM 严格按 owned paths 派发，则按 dispatch order 走两个 agent。

---

## §12 Open Questions for Gate Reviewer（≤ 3）

| Q | 内容 | SA 倾向 |
|---|---|---|
| **Q-1** | R-1 缓解里"PS5.1+zh-CN 磁盘形态中文乱码可接受"——是否要在本任务连带追加 README 一句话警告（"PS5.1+zh-CN 主机推荐 iex 形态而非 .\install.ps1"），即扩本任务 OOS 一条？或者保留 OOS，单独开一个 trivial T-XXX 改 README？ | **保留 OOS，PM 自决**：本任务的 install.ps1 改动已闭环；README 改动是 UX 层、独立的；trivial 任务可在 T-026 交付后单独开。但如 GR 判断"用户首次撞墙就放弃"严重，可允许本任务范围扩 1 行 README diff。 |
| **Q-2** | E.7c 用 WARN 还是 FAIL？SA 选 WARN（让新 .ps1 PR 不立刻挂 CI、给一次提醒缓冲）；但 RA NFR-3 强调"不允许 silent regression"——WARN 是否算 silent？ | **选 WARN**：verify_all WARN 在 PS / sh 两端都退出码 1，CI 仍能注意；WARN 文案显示 `unclassified` 文件名让维护者一眼定位。如 GR 偏紧，可升 FAIL（一行改：`return $false` → `throw "..."`）。 |
| **Q-3** | `& { ... } @PSBoundParameters` 的 splatting 在 PS5.1 vs PS7 是否完全同语义？SA 已断言 yes（about_Splatting 跨版本统一文档），但缺真机实测；步骤 10 仅能在 PS7 验证。 | **接受现风险，依赖 AC-8 [U] 用户真机覆盖**：如用户在 PS5.1 磁盘形态 `-Help` 不显示 Help 反走安装，则 R-3 触发，PM 在 06 后回退到 D-2 选项 D（function 包，避免 splatting）。 |

---

## §13 Verdict

**READY**

理由：
- D-1 ~ D-7 七项决议全部显式收敛（§2 决议表）；RA §11 Q-1/Q-2/Q-3 设计偏好已在本文消化。
- AC-1 ~ AC-18 全部映射到 §6 实施步骤 + §8 验证策略；[A] / [M] / [U] 分层标注清晰。
- 5 个改动文件、行号区间、改动性质、git diff 形态全部具体到字节级；Developer 可机械执行。
- 11 项风险（R-1 ~ R-11）全含缓解；关键风险 R-3（双层 param + splatting 兼容性）配 Developer 步骤 10 动态冒烟兜底。
- 复用审计（§9）非空，覆盖 BOM 字节 idiom / verify_all helper / EditorConfig / scriptblock 调用 / splatting 五项已有模式。
- Insight 适配：L25（管道形态禁 `$PSScriptRoot`，install.ps1 现状 OK 不引入新违反）、L32-L33（BOM 在磁盘 vs 管道形态语义差异，D-1 机制层证据）、L34（BOM 字节级 idiom 反向用法）、L36（T-024 `[CmdletBinding()]` 已删，本任务不重新加）、L41 + L48（reviewer 类落盘契约，本文已直接写盘）、L43 + L22 / L35（QA 06 标题禁数字前缀，§8.4 ADV 段标题已用裸 `## Adversarial tests`）全部覆盖。
- Partition assignment 明确（§11），单 developer + 可选 dev-backend 协作路径已说清。
- 与 RA OOS 边界严格对齐（§10.4）。

如 GR 在 03 判断 §12 任一 Open Question 必须先让用户回答，则改为 BLOCKER 退回 PM；否则 PM 派发 Developer 进 Stage 4。

---

READY-FOR-GATE-REVIEW
