# archive-task.ps1 — Archive a completed task: harvest insights, move stage docs.
#
# Usage:
#   pwsh -File scripts/archive-task.ps1 -Task <task-slug>
#   pwsh -File scripts/archive-task.ps1 -Task <task-slug> -DryRun
#
# What it does:
#   1. Find docs/features/<task-slug>/
#   2. If 07_DELIVERY.md has an '## Insight' (or '## Insights') section, append its bullets
#      to .harness/insight-index.md.
#   3. Move docs/features/<task-slug>/ -> docs/features/_archived/<task-slug>/
#   4. If .harness/insight-index.md exceeds 30 insight lines, rotate the oldest to
#      docs/features/_archived/insight-history.md.
#
# Never deletes. Only moves and appends.

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Task,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path $PSScriptRoot -Parent

$taskDir = Join-Path $repoRoot "docs/features/$Task"
$archivedRoot = Join-Path $repoRoot "docs/features/_archived"
$archivedTaskDir = Join-Path $archivedRoot $Task
$insightIndex = Join-Path $repoRoot ".harness/insight-index.md"
$insightHistory = Join-Path $archivedRoot "insight-history.md"

if (-not (Test-Path $taskDir)) {
    Write-Error "Task directory not found: $taskDir"
    exit 1
}

if (Test-Path $archivedTaskDir) {
    Write-Error "Task already archived: $archivedTaskDir"
    exit 1
}

# Step 1: harvest insights from 07_DELIVERY.md (if present)
$deliveryFile = Join-Path $taskDir "07_DELIVERY.md"
$harvestedInsights = @()
if (Test-Path $deliveryFile) {
    $content = Get-Content $deliveryFile -Raw
    # Match '## Insight' or '## Insights' section, until next '##' or EOF
    if ($content -match '(?ms)^##\s+Insights?\s*$(.*?)(?=^##\s|\z)') {
        $section = $matches[1]
        # Extract lines that start with '- '
        $harvestedInsights = $section -split "`n" | Where-Object { $_ -match '^\s*-\s+' } | ForEach-Object { $_.Trim() }
    }
}

if ($harvestedInsights.Count -gt 0) {
    Write-Host "Harvested $($harvestedInsights.Count) insight(s) from 07_DELIVERY.md:" -ForegroundColor Cyan
    $harvestedInsights | ForEach-Object { Write-Host "  $_" -ForegroundColor DarkGray }
}

# Step 2: rotate insight-index.md if it would exceed 30 lines
if (-not (Test-Path $insightIndex)) {
    Write-Warning ".harness/insight-index.md missing — creating empty"
    if (-not $DryRun) {
        New-Item -ItemType File -Path $insightIndex -Force | Out-Null
    }
}

$currentInsights = @()
if (Test-Path $insightIndex) {
    $currentInsights = Get-Content $insightIndex | Where-Object { $_ -match '^\s*-\s+' }
}

$totalAfter = $currentInsights.Count + $harvestedInsights.Count
$rotated = @()
if ($totalAfter -gt 30) {
    $rotateCount = $totalAfter - 30
    $rotated = $currentInsights | Select-Object -First $rotateCount
    $remaining = $currentInsights | Select-Object -Skip $rotateCount

    Write-Host "Rotating $rotateCount old insight(s) to insight-history.md" -ForegroundColor Yellow

    if (-not $DryRun) {
        # Append rotated to history
        if (-not (Test-Path $archivedRoot)) { New-Item -ItemType Directory -Path $archivedRoot -Force | Out-Null }
        if (-not (Test-Path $insightHistory)) {
            "# Insight history (rotated from .harness/insight-index.md)`n" | Set-Content $insightHistory
        }
        "`n## Rotated $(Get-Date -Format 'yyyy-MM-dd')`n" | Add-Content $insightHistory
        $rotated | Add-Content $insightHistory

        # Rewrite insight-index keeping only remaining + new
        $header = Get-Content $insightIndex | Where-Object { $_ -notmatch '^\s*-\s+' }
        $newContent = ($header -join "`n") + "`n" + ($remaining -join "`n") + "`n" + ($harvestedInsights -join "`n") + "`n"
        Set-Content -Path $insightIndex -Value $newContent
    }
} elseif ($harvestedInsights.Count -gt 0) {
    if (-not $DryRun) {
        $harvestedInsights | ForEach-Object { Add-Content -Path $insightIndex -Value $_ }
    }
}

# Step 3: move task directory to _archived/
if (-not $DryRun) {
    if (-not (Test-Path $archivedRoot)) { New-Item -ItemType Directory -Path $archivedRoot -Force | Out-Null }
    Move-Item -Path $taskDir -Destination $archivedTaskDir
}

# Step 4: report
if ($DryRun) {
    Write-Host ""
    Write-Host "[DRY RUN] No files written. Would have:" -ForegroundColor Yellow
    Write-Host "  - Appended $($harvestedInsights.Count) insight(s) to .harness/insight-index.md"
    Write-Host "  - Rotated $($rotated.Count) old insight(s) to insight-history.md"
    Write-Host "  - Moved $taskDir -> $archivedTaskDir"
} else {
    Write-Host ""
    Write-Host "Archived task: $Task" -ForegroundColor Green
    Write-Host "  Stage docs:   $archivedTaskDir"
    if ($harvestedInsights.Count -gt 0) {
        Write-Host "  Insights:     +$($harvestedInsights.Count) to .harness/insight-index.md"
    }
    if ($rotated.Count -gt 0) {
        Write-Host "  Rotated:      $($rotated.Count) -> $insightHistory"
    }
}
