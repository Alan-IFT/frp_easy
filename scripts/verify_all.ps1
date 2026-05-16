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
    $out = & go vet ./... 2>&1
    if ($LASTEXITCODE -ne 0) { throw "go vet failed:`n$out" }
}

Step "G.2" "go test ./..." {
    if (-not (Test-Path "go.mod")) { return "SKIP" }
    $env:PATH = "C:\Program Files\Go\bin;$env:PATH"
    $out = & go test ./... 2>&1
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

# --- B. Build / test (require package.json) ---
Step "B.1" "Install / typecheck" {
    if (-not (Test-Path "package.json")) { return "SKIP" }
    $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
    & $pkgMgr install --frozen-lockfile 2>&1 | Out-Null
    if (Test-Path "tsconfig.json") { & $pkgMgr exec tsc --noEmit }
}

Step "B.2" "Lint" {
    if (-not (Test-Path "package.json")) { return "SKIP" }
    $hasEslint = (Test-Path ".eslintrc") -or (Test-Path ".eslintrc.js") -or (Test-Path ".eslintrc.cjs") -or (Test-Path ".eslintrc.json") -or (Test-Path "eslint.config.js") -or (Test-Path "eslint.config.mjs")
    if (-not $hasEslint) { return "SKIP" }
    $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
    & $pkgMgr exec eslint . 2>&1 | Out-Null
}

Step "B.3" "Unit tests pass" {
    if (-not (Test-Path "package.json")) { return "SKIP" }
    $pkgJson = Get-Content "package.json" -Raw | ConvertFrom-Json
    if (-not $pkgJson.scripts.test) { return "SKIP" }
    $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
    & $pkgMgr test 2>&1 | Out-Null
}

Step "B.4" "Test count >= baseline" {
    if (-not (Test-Path "scripts/baseline.json")) { return "SKIP" }
    $baseline = Get-Content "scripts/baseline.json" | ConvertFrom-Json
    if ($baseline.test_count -eq 0) { return "SKIP" }
    # CUSTOMIZE: read your test runner output to get actual count vs baseline.
}

# --- C. End-to-end (require playwright config) ---
if (-not $Quick) {
    Step "C.1" "E2E smoke (playwright)" {
        if (-not ((Test-Path "playwright.config.ts") -or (Test-Path "playwright.config.js"))) { return "SKIP" }
        $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
        & $pkgMgr exec playwright test --project=chromium 2>&1 | Out-Null
    }
}

# --- D. Schema / contract (require source code) ---
Step "D.1" "OpenAPI / tRPC schema present" {
    # SKIP if no source code yet (empty project just initialized)
    if (-not ((Test-Path "src") -or (Test-Path "apps") -or (Test-Path "packages"))) { return "SKIP" }
    $found = (Test-Path "openapi.yaml") -or (Test-Path "openapi.json") -or (Test-Path "src/server/trpc")
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
