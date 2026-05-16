# harness-sync.ps1 — v0.10
# Binding sync: .harness/ (tool-agnostic SOT) → .claude/ (Claude Code path requirement).
#
# v0.10 scope (much narrower than v0.9.x):
#   .harness/agents/*.md   → .claude/agents/*.md   (byte-identical copy)
#   .harness/skills/*/     → .claude/skills/*/     (byte-identical copy)
#
# Rules are NO LONGER composed into CLAUDE.md or .github/copilot-instructions.md.
# Those are static stubs written once at init, pointing at AI-GUIDE.md.
# AI-GUIDE.md indexes .harness/rules/ by reference; no regeneration needed.
#
# Usage:
#   harness-sync.ps1                # do the sync
#   harness-sync.ps1 -Check         # report drift only; exit 0 if in sync, 1 otherwise

[CmdletBinding()]
param([switch]$Check)

$ErrorActionPreference = "Stop"

$scriptDir = $PSScriptRoot
$projectRoot = Split-Path $scriptDir -Parent

$harnessDir = Join-Path $projectRoot ".harness"
$claudeDir  = Join-Path $projectRoot ".claude"

if (-not (Test-Path $harnessDir)) {
    Write-Error "No .harness/ found at $harnessDir. Run /harness-init or /harness-adopt first."
    exit 1
}

$drift = @()

# ---------- Copy .harness/agents/ → .claude/agents/ ----------
$harnessAgents = Join-Path $harnessDir "agents"
$claudeAgents  = Join-Path $claudeDir  "agents"

if (Test-Path $harnessAgents) {
    if (-not (Test-Path $claudeAgents)) {
        if ($Check) {
            $drift += ".claude/agents/ (missing)"
        } else {
            New-Item -ItemType Directory -Path $claudeAgents -Force | Out-Null
        }
    }

    Get-ChildItem -Path $harnessAgents -Filter "*.md" -File | ForEach-Object {
        $dst = Join-Path $claudeAgents $_.Name
        $needsCopy = $true
        if (Test-Path $dst) {
            if ((Get-FileHash $_.FullName -Algorithm SHA256).Hash -eq (Get-FileHash $dst -Algorithm SHA256).Hash) {
                $needsCopy = $false
            } else {
                $drift += ".claude/agents/$($_.Name) (out of sync)"
            }
        } else {
            $drift += ".claude/agents/$($_.Name) (missing)"
        }
        if ($needsCopy -and (-not $Check)) {
            Copy-Item -Path $_.FullName -Destination $dst -Force
            Write-Host "Synced .claude/agents/$($_.Name)" -ForegroundColor Green
        }
    }

    if (Test-Path $claudeAgents) {
        Get-ChildItem -Path $claudeAgents -Filter "*.md" -File | ForEach-Object {
            if (-not (Test-Path (Join-Path $harnessAgents $_.Name))) {
                $drift += ".claude/agents/$($_.Name) (orphan - not in .harness/agents/)"
                if (-not $Check) {
                    Remove-Item $_.FullName -Force
                    Write-Host "Removed orphan .claude/agents/$($_.Name)" -ForegroundColor Yellow
                }
            }
        }
    }
}

# ---------- Copy .harness/skills/ → .claude/skills/ ----------
$harnessSkills = Join-Path $harnessDir "skills"
$claudeSkills  = Join-Path $claudeDir  "skills"

if (Test-Path $harnessSkills) {
    Get-ChildItem -Path $harnessSkills -Recurse -File | ForEach-Object {
        $rel = $_.FullName.Substring($harnessSkills.Length).TrimStart('\','/')
        $dst = Join-Path $claudeSkills $rel
        $needsCopy = $true
        if (Test-Path $dst) {
            if ((Get-FileHash $_.FullName -Algorithm SHA256).Hash -eq (Get-FileHash $dst -Algorithm SHA256).Hash) {
                $needsCopy = $false
            } else {
                $drift += ".claude/skills/$rel (out of sync)"
            }
        } else {
            $drift += ".claude/skills/$rel (missing)"
        }
        if ($needsCopy -and (-not $Check)) {
            $dstDir = Split-Path $dst -Parent
            if (-not (Test-Path $dstDir)) { New-Item -ItemType Directory -Path $dstDir -Force | Out-Null }
            Copy-Item -Path $_.FullName -Destination $dst -Force
            Write-Host "Synced .claude/skills/$rel" -ForegroundColor Green
        }
    }
}

# ---------- Report ----------
if ($Check) {
    if ($drift.Count -gt 0) {
        Write-Host "Drift detected ($($drift.Count) item(s)):" -ForegroundColor Red
        $drift | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
        Write-Host ""
        Write-Host "Fix: run scripts/harness-sync.ps1 (without -Check)" -ForegroundColor Yellow
        exit 1
    } else {
        Write-Host "In sync." -ForegroundColor Green
        exit 0
    }
}

if ($drift.Count -eq 0) {
    Write-Host "Already in sync." -ForegroundColor Green
} else {
    Write-Host ""
    Write-Host "Sync complete ($($drift.Count) item(s) updated)." -ForegroundColor Cyan
}
