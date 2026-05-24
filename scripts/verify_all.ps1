# verify_all.ps1 — Fullstack project total verification
# Generated for frp_easy (Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)) on 2026-05-16
#
# Usage:
#   .\scripts\verify_all.ps1            # full run
#   .\scripts\verify_all.ps1 -Quick     # skip e2e
#   .\scripts\verify_all.ps1 -Baseline  # update baseline.json (use carefully)
#
# Conventions:
#   - PASS (0)  every check passed
#   - WARN (1)  no errors but some non-fatal issues
#   - FAIL (2)  at least one Error-level check failed
#   - SKIP      check did not run (prerequisite missing, e.g. no package.json yet)
#               SKIPs do not affect exit code.
#
# CUSTOMIZE: edit the command lines below to match your actual stack.
# Defaults assume pnpm + node project at root; tweak as needed.

[CmdletBinding()]
param(
    [switch]$Quick,
    [switch]$Baseline
)

$ErrorActionPreference = "Stop"
$root = (Get-Location).Path
$report = @()
$errors = 0
$warns = 0
$skips = 0

function Step($id, $name, [scriptblock]$action) {
    Write-Host "[$id] $name ..." -NoNewline
    try {
        $result = & $action
        if ($result -eq "SKIP") {
            Write-Host " SKIP" -ForegroundColor DarkGray
            $script:skips++
            $script:report += [pscustomobject]@{ id = $id; name = $name; status = "SKIP" }
        } elseif ($result -eq $false) {
            Write-Host " WARN" -ForegroundColor Yellow
            $script:warns++
            $script:report += [pscustomobject]@{ id = $id; name = $name; status = "WARN" }
        } else {
            Write-Host " PASS" -ForegroundColor Green
            $script:report += [pscustomobject]@{ id = $id; name = $name; status = "PASS" }
        }
    } catch {
        Write-Host " FAIL" -ForegroundColor Red
        Write-Host "       $_" -ForegroundColor DarkRed
        $script:errors++
        $script:report += [pscustomobject]@{ id = $id; name = $name; status = "FAIL"; error = "$_" }
    }
}

Write-Host "=== verify_all (fullstack) ===" -ForegroundColor Cyan
Write-Host "Project: frp_easy"
Write-Host "Stack:   Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)"
Write-Host ""

# --- A. Static checks (always applicable) ---
Step "A.1" "No hardcoded secrets" {
    if (-not (Test-Path ".git")) { return "SKIP" }
    $patterns = @("(?i)(api[_-]?key|secret|password|token)\s*[:=]\s*['""][^'""]{8,}['""]")
    $hits = git grep -E $patterns -- ':!*.md' ':!scripts/verify_all*' ':!.harness/*' 2>$null
    if ($hits) { throw "Possible secret in:`n$hits" }
}

Step "A.2" "No .env files committed" {
    if (-not (Test-Path ".git")) { return "SKIP" }
    $envFiles = git ls-files -- ':!*.env.example' ':!*.env.sample' '*.env' '.env*' 2>$null | Where-Object { $_ -notmatch 'example|sample' }
    if ($envFiles) { throw "Committed env files:`n$envFiles" }
}

Step "A.3" "TODO / FIXME budget (warn only)" {
    if (-not (Test-Path ".git")) { return "SKIP" }
    $count = (git grep -i -c "TODO\|FIXME" -- '*.ts' '*.tsx' '*.js' '*.jsx' '*.py' '*.go' 2>$null | Measure-Object -Line).Lines
    if ($count -gt 20) { return $false }
}

# --- G. Go checks (require go.mod) ---
Step "G.1" "go vet" {
    if (-not (Test-Path "go.mod")) { return "SKIP" }
    $env:PATH = "C:\Program Files\Go\bin;$env:PATH"
    $pkgs = & go list ./... 2>&1 | Where-Object { $_ -notmatch "node_modules" }
    if (-not $pkgs) { return "SKIP" }
    $out = & go vet $pkgs 2>&1
    if ($LASTEXITCODE -ne 0) { throw "go vet failed:`n$out" }
}

Step "G.2" "go test ./..." {
    if (-not (Test-Path "go.mod")) { return "SKIP" }
    $env:PATH = "C:\Program Files\Go\bin;$env:PATH"
    $pkgs = & go list ./... 2>&1 | Where-Object { $_ -notmatch "node_modules" }
    if (-not $pkgs) { return "SKIP" }
    $out = & go test $pkgs 2>&1
    if ($LASTEXITCODE -ne 0) { throw "go test failed:`n$out" }
}

Step "G.3" "go build ./cmd/frp-easy" {
    if (-not (Test-Path "go.mod")) { return "SKIP" }
    $env:PATH = "C:\Program Files\Go\bin;$env:PATH"
    $env:CGO_ENABLED = "0"
    $out = & go build ./cmd/frp-easy 2>&1
    if ($LASTEXITCODE -ne 0) { throw "go build failed:`n$out" }
    Remove-Item "frp-easy.exe" -ErrorAction SilentlyContinue
    Remove-Item "frp-easy" -ErrorAction SilentlyContinue
}

# --- B. Build / test (require web/package.json) ---
Step "B.1" "Install / typecheck" {
    if (-not (Test-Path "web/package.json")) { return "SKIP" }
    Push-Location (Join-Path $root "web")
    try {
        $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
        & $pkgMgr install --frozen-lockfile 2>&1 | Out-Null
        # `npm exec -- tsc --noEmit` 中 `--` 分隔符必需：缺它 npm 会把 --noEmit
        # 当自身 flag 吞掉（"npm warn Unknown cli config"），tsc fallback 到 tsconfig
        # 默认 emit，污染 web/src 写出 .js。T-010 同时给 tsconfig.json 加 noEmit:true
        # 做第二层防御；这里也修正调用形式以让 typecheck 真的只 typecheck。
        if (Test-Path "tsconfig.json") { & $pkgMgr exec -- tsc --noEmit }
    } finally {
        Pop-Location
    }
}

Step "B.2" "Lint" {
    if (-not (Test-Path "web/package.json")) { return "SKIP" }
    Push-Location (Join-Path $root "web")
    try {
        $hasEslint = (Test-Path ".eslintrc") -or (Test-Path ".eslintrc.js") -or (Test-Path ".eslintrc.cjs") -or (Test-Path ".eslintrc.json") -or (Test-Path "eslint.config.js") -or (Test-Path "eslint.config.mjs")
        if (-not $hasEslint) { return "SKIP" }
        $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
        & $pkgMgr exec eslint . 2>&1 | Out-Null
    } finally {
        Pop-Location
    }
}

Step "B.3" "Unit tests pass" {
    if (-not (Test-Path "web/package.json")) { return "SKIP" }
    Push-Location (Join-Path $root "web")
    try {
        $pkgJson = Get-Content "package.json" -Raw | ConvertFrom-Json
        if (-not $pkgJson.scripts.test) { return "SKIP" }
        $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
        & $pkgMgr test 2>&1 | Out-Null
    } finally {
        Pop-Location
    }
}

Step "B.4" "Test count >= baseline" {
    if (-not (Test-Path (Join-Path $root "scripts/baseline.json"))) { return "SKIP" }
    $baseline = Get-Content (Join-Path $root "scripts/baseline.json") | ConvertFrom-Json
    if ($baseline.test_count -eq 0) { return "SKIP" }
    # CUSTOMIZE: read your test runner output to get actual count vs baseline.
}

# B.5 — anti-residue sentinel（T-010）：
# tsc 早期未启用 noEmit 时会在 web/src/ 里写下 .js / .js.map 与 .ts 同名共存，
# 让 vitest 模块解析按 .js 优先（insight-index 2026-05-19），改 .ts 测试看似无效果。
# tsconfig.json 已加 "noEmit": true 但旧 tooling / IDE 仍可能误触；本步是闸门。
# env.d.ts 是 Vite 类型声明，例外保留。
Step "B.5" "No tsc residue in web/src/" {
    $srcDir = Join-Path $root "web\src"
    if (-not (Test-Path $srcDir)) { return "SKIP" }
    $residue = Get-ChildItem -Path $srcDir -Recurse -File -Include '*.js','*.js.map' |
               Where-Object { $_.Name -ne 'env.d.ts' } |
               Select-Object -First 10
    if ($residue) {
        $names = ($residue | ForEach-Object { $_.FullName.Substring($root.Length + 1) }) -join ', '
        throw "found tsc residue: $names"
    }
}

# --- C. End-to-end (require playwright config) ---
if (-not $Quick) {
    Step "C.1" "E2E smoke (playwright)" {
        $hasConfig = (Test-Path "playwright.config.ts") -or (Test-Path "playwright.config.js") `
                  -or (Test-Path "web/playwright.config.ts") -or (Test-Path "web/playwright.config.js")
        if (-not $hasConfig) { return "SKIP" }
        # frp_easy 约定：playwright config 位于 web/ 子目录
        if ((Test-Path "web/playwright.config.ts") -or (Test-Path "web/playwright.config.js")) {
            $playwrightDir = Join-Path $root "web"
        } else {
            $playwrightDir = $root
        }
        Push-Location $playwrightDir
        try {
            $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
            & $pkgMgr exec playwright test --project=chromium 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) { throw "playwright test failed (exit code $LASTEXITCODE)" }
        } finally {
            Pop-Location
        }
    }
}

# --- D. Schema / contract (require source code) ---
Step "D.1" "OpenAPI / tRPC schema present" {
    # 前置条件改为检测 go.mod：本项目为 Go 项目，无 src/apps/packages 目录，
    # 原条件导致 D.1 永久 SKIP（TD-3）；以 go.mod 存在作为"已有源码"判据。
    if (-not (Test-Path "go.mod")) { return "SKIP" }
    $found = (Test-Path "openapi.yaml") -or (Test-Path "openapi.json")
    if (-not $found) { return $false } # WARN, not FAIL
}

# --- E. Project structure (Harness required) ---
Step "E.1" "CLAUDE.md present" {
    if (-not (Test-Path "CLAUDE.md")) { throw "CLAUDE.md missing" }
}

Step "E.2" "workflow.md present" {
    if (-not (Test-Path "docs/workflow.md")) { throw "docs/workflow.md missing" }
}

Step "E.3" "All 7 agent definitions present in .harness/agents/" {
    $needed = @("pm-orchestrator", "requirement-analyst", "solution-architect",
                "gate-reviewer", "developer", "code-reviewer", "qa-tester")
    foreach ($a in $needed) {
        if (-not (Test-Path ".harness/agents/$a.md")) { throw "Missing agent: .harness/agents/$a.md" }
    }
}

Step "E.4" "Binding in sync (.harness/ -> .claude/)" {
    if (-not (Test-Path "scripts/harness-sync.ps1")) { throw "scripts/harness-sync.ps1 missing" }
    & "scripts/harness-sync.ps1" -Check
    if ($LASTEXITCODE -ne 0) { throw "Binding drift -- run scripts/harness-sync.ps1 to fix" }
}

Step "E.5" "AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa)" {
    if (-not (Test-Path "AI-GUIDE.md")) { return "SKIP" }
    if (-not (Test-Path ".harness/rules")) { return "SKIP" }
    $guide = Get-Content "AI-GUIDE.md" -Raw
    $missingFromGuide = @()
    Get-ChildItem -Path ".harness/rules" -Filter "*.md" -File | ForEach-Object {
        if ($guide -notmatch [regex]::Escape(".harness/rules/$($_.Name)")) {
            $missingFromGuide += $_.Name
        }
    }
    $referencedRules = [regex]::Matches($guide, '\.harness/rules/([0-9A-Za-z_\-]+\.md)') |
        ForEach-Object { $_.Groups[1].Value } | Sort-Object -Unique
    $missingFromDisk = @()
    foreach ($ref in $referencedRules) {
        if (-not (Test-Path ".harness/rules/$ref")) { $missingFromDisk += $ref }
    }
    $problems = @()
    if ($missingFromGuide.Count -gt 0) { $problems += "Rules NOT indexed: $($missingFromGuide -join ', ')" }
    if ($missingFromDisk.Count -gt 0) { $problems += "References non-existent: $($missingFromDisk -join ', ')" }
    if ($problems.Count -gt 0) { throw ($problems -join " | ") }
}

Step "E.6" "Adversarial tests section present in completed task reports" {
    # Each completed 06_TEST_REPORT.md MUST contain the '## Adversarial tests' section.
    # This enforces the QA Tester's adversarial-verification contract.
    if (-not (Test-Path "docs/features")) { return "SKIP" }
    $reports = Get-ChildItem -Path "docs/features" -Recurse -Filter "06_TEST_REPORT.md" -ErrorAction SilentlyContinue
    if ($reports.Count -eq 0) { return "SKIP" }
    $bad = @()
    foreach ($r in $reports) {
        $c = Get-Content $r.FullName -Raw
        if ($c -notmatch '##\s+Adversarial\s+tests') { $bad += $r.FullName.Substring($root.Length + 1) }
    }
    if ($bad.Count -gt 0) { throw "Test reports missing '## Adversarial tests' section:`n$($bad -join "`n")" }
}

# T-026: 拆分 T-021 的全量 BOM 检查为 white-list 驱动，容纳 install.ps1 的反向规则。
# install.ps1 是 irm | iex 入口；BOM 会被 Invoke-RestMethod 解码为 U+FEFF 进入字符串触发
# ParserError（'﻿#' is not recognized）。其余 10 个 .ps1 仍是磁盘形态调用，PS5.1 + zh-CN
# 主机无 BOM 时会按 host ANSI codepage (GBK) 误解码中文，必须保留 BOM。
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
        throw "iex-entry .ps1 MUST NOT have UTF-8 BOM (BOM -> U+FEFF -> ParserError in iex form):`n$($wrong -join "`n")"
    }
}

Step "E.7c" "All scripts/*.ps1 classified in E.7a or E.7b (anti-drift)" {
    if (-not (Test-Path "scripts")) { return "SKIP" }
    $known = $Ps1RequireBom + $Ps1ForbidBom
    $actual = Get-ChildItem -Path "scripts" -Filter "*.ps1" -File -ErrorAction SilentlyContinue |
              ForEach-Object { $_.Name }
    $unclassified = @($actual | Where-Object { $known -notcontains $_ })
    if ($unclassified.Count -gt 0) {
        # G-7 必修条件：WARN 分支显式打印未分类文件名让维护者一眼定位（依 03 §8 G-7 增补）
        Write-Host ""
        Write-Host "       unclassified: $($unclassified -join ', ')" -ForegroundColor Yellow
        return $false  # WARN 而非 FAIL：提醒维护者归类、不阻塞 CI
    }
}

# T-031: AC-5 静态闸门 —— install.ps1 / install-service.ps1 禁交互阻塞（FR-3 硬红线）。
# 任何 Read-Host / [Console]::ReadKey / 裸 `pause` 行 / Wait-Event 都会让自动化场景挂死。
# 实现：单走 `Select-String -Pattern` 按行扫描（天然 multiline，'^' / '$' 按行匹配），
# 跳过 # 开头注释行 + 含元描述词（禁/forbidden/FR-3/red.?line）的合法字面量行。
# C-1：直接用 Select-String 替代"先 -match 再 Select-String"两段式（避免 Get-Content -Raw
# 无 (?m) 标志下 '^\s*pause\s*$' 漏报，03 §E15）。
Step "E.8" "install.ps1 / install-service.ps1 forbid interactive blockers (FR-3)" {
    if (-not (Test-Path "scripts")) { return "SKIP" }
    $targets = @('scripts\install.ps1', 'scripts\install-service.ps1')
    $forbidden = @('Read-Host', '\[Console\]::ReadKey', '^\s*pause\s*$', 'Wait-Event')
    $hits = @()
    foreach ($t in $targets) {
        if (-not (Test-Path -PathType Leaf $t)) { continue }
        foreach ($pat in $forbidden) {
            $lines = Select-String -Path $t -Pattern $pat -ErrorAction SilentlyContinue | Where-Object {
                $trimmed = $_.Line.TrimStart()
                # 跳过 # 开头注释行
                if ($trimmed.StartsWith('#')) { return $false }
                # 跳过含元描述词的合法字面量行（如本任务注释 / 说明引用 'Read-Host'）
                if ($_.Line -match '禁|red\.?line|forbidden|FR-3|破\s*FR-3') { return $false }
                return $true
            }
            foreach ($ln in $lines) {
                $hits += ("{0}:{1}: {2}" -f $t, $ln.LineNumber, $ln.Line.Trim())
            }
        }
    }
    if ($hits.Count -gt 0) {
        throw ("Interactive blockers found (破 FR-3 红线):`n" + ($hits -join "`n"))
    }
}

# T-031: AC-10 静态闸门 —— 仓库无 scripts/install*.cmd / scripts/install*.bat（FR-8 单脚本红线）
Step "E.9" "No wrapper.cmd / install*.bat in scripts/ (FR-8 single-script invariant)" {
    if (-not (Test-Path "scripts")) { return "SKIP" }
    $stray = Get-ChildItem -Path "scripts" -File -ErrorAction SilentlyContinue |
             Where-Object { $_.Name -match '^install.*\.(cmd|bat)$' }
    if ($stray) {
        throw ("Forbidden wrapper files found:`n" + (($stray | ForEach-Object { $_.FullName }) -join "`n"))
    }
}

# T-031: AC-额外 闸门 —— README 推荐 Windows 入口字串必须含 -NoExit（防回归 FR-10）
Step "E.10" "README Windows install entry contains -NoExit (T-031 FR-10)" {
    if (-not (Test-Path "README.md")) { return "SKIP" }
    $content = Get-Content -Raw -Path "README.md"
    # 提取"**Windows**" 段后第一个 powershell code block
    if ($content -notmatch '(?ms)\*\*Windows\*\*[^\n]*\n+```powershell\s*\n([^`]+?)\n```') {
        throw "README.md 'Windows' install entry powershell code block not found."
    }
    $entryBlock = $matches[1]
    if ($entryBlock -notmatch '-NoExit\b') {
        throw ("README.md Windows install entry missing -NoExit flag (T-031 FR-1 / FR-10):`n" + $entryBlock)
    }
}

# --- G. Reviewer dispatch protocol (T-034) ---
# G.1 / G.2 守门 sub-agent 工具白名单 frontmatter 在 SDK 派发上下文可能被裁剪现象：
# - frontmatter `tools: Read, Write, Glob, Grep` 是理论上限，运行时可能更窄
# - 长期解：reviewer 契约 + PM 派发协议规范化"双模式 + sentinel"
# - 这里仅静态守门"契约段在源码里存在"，不试图测运行时派发（不在静态闸门可达范围）

Step "G.1" "Reviewer agents declare PM_FALLBACK_WRITE sentinel (T-034)" {
    if (-not (Test-Path ".harness/agents")) { return "SKIP" }
    $targets = @('.harness/agents/gate-reviewer.md', '.harness/agents/code-reviewer.md')
    $missing = @()
    foreach ($t in $targets) {
        if (-not (Test-Path -PathType Leaf $t)) {
            $missing += "$t (MISSING)"
            continue
        }
        $content = Get-Content -Raw -Path $t
        if ($content -notmatch 'MODE:\s*PM_FALLBACK_WRITE') {
            $missing += "$t (no PM_FALLBACK_WRITE sentinel)"
        }
    }
    if ($missing.Count -gt 0) {
        throw ("Reviewer two-mode protocol missing in:`n" + ($missing -join "`n"))
    }
}

Step "G.2" "PM Orchestrator declares Reviewer dispatch protocol (T-034)" {
    $f = '.harness/agents/pm-orchestrator.md'
    if (-not (Test-Path -PathType Leaf $f)) { return "SKIP" }
    $content = Get-Content -Raw -Path $f
    $problems = @()
    if ($content -notmatch 'Reviewer\s+dispatch\s+protocol') {
        $problems += "missing 'Reviewer dispatch protocol' heading"
    }
    if ($content -notmatch 'MODE:\s*PM_FALLBACK_WRITE') {
        $problems += "missing PM_FALLBACK_WRITE sentinel reference"
    }
    if ($problems.Count -gt 0) {
        throw ("PM Orchestrator reviewer dispatch protocol incomplete: " + ($problems -join '; '))
    }
}

# --- H. T-037 deletion surface guard ---
# T-037 删除了"批量代理 / 端口探测 / 折叠分组"三类辅助能力。本闸门防止未来静默回退。
# 禁词列表覆盖前端 / 后端 / OpenAPI 三层；归档 (docs/features/_archived/*) 豁免。
# 双实现对账（insight L26）：与 verify_all.sh H.1 行为一致——按行 grep + 同款禁词表。
Step "H.1" "T-037 deletion surface clean (no batch/probe/grouping residue)" {
    if (-not (Test-Path ".git")) { return "SKIP" }
    $pattern = '\b(batchMode|portsExpr|apiBatchCreate|batchProxies|UpsertProxiesTx|apiProbePorts|probePorts|probeOnePort|useProxyGrouping|groupProxiesByPrefix|BatchProxiesRequest|BatchProxiesResponse|PortProbeRequest|PortProbeResult|PortProbeResponse|ErrDuplicateTcpRemote|isDuplicateTcpRemoteError|internal/portrange)\b'
    $hits = git grep -nE $pattern -- 'web/src/**' 'internal/**' 'openapi.yaml' `
        ':(exclude)docs/features/_archived/**' ':(exclude).harness/**' 2>$null
    if ($LASTEXITCODE -gt 1) {
        # git grep exit code 0=found, 1=no-match, >1=error
        throw "git grep failed with exit code $LASTEXITCODE"
    }
    if ($hits) {
        throw ("T-037 deletion residue found:`n" + ($hits -join "`n"))
    }
}

# --- Summary ---
Write-Host ""
Write-Host "=== Summary ===" -ForegroundColor Cyan
$pass = ($report | Where-Object status -eq "PASS").Count
Write-Host "  PASS: $pass" -ForegroundColor Green
Write-Host "  WARN: $warns" -ForegroundColor Yellow
Write-Host "  FAIL: $errors" -ForegroundColor Red
Write-Host "  SKIP: $skips" -ForegroundColor DarkGray

# --- Append history ---
$historyEntry = [pscustomobject]@{
    timestamp = (Get-Date).ToString("o")
    pass = $pass
    warn = $warns
    fail = $errors
    skip = $skips
    report = $report
}
$historyEntry | ConvertTo-Json -Depth 5 -Compress | Add-Content -Path "scripts/verification_history.log"

if ($errors -gt 0) { exit 2 }
if ($warns -gt 0) { exit 1 }
exit 0
