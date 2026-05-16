# install-hooks.ps1 - Install the harness-kit git pre-commit hook.
#
# Why: .harness/ is the source of truth; CLAUDE.md + .github/copilot-instructions.md
# are generated. Claude Code keeps them fresh via a Stop hook in .claude/settings.json,
# but that Stop hook is Claude-Code-specific — it does NOT fire for GitHub Copilot,
# Cursor, or hand-edits. This pre-commit hook is the tool-agnostic backstop: any
# commit that includes stale generated artifacts is blocked, regardless of who or
# what edited .harness/.
#
# Usage:
#   pwsh -File scripts/install-hooks.ps1
#
# To disable: delete .git/hooks/pre-commit.

[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path $PSScriptRoot -Parent
$gitDir = Join-Path $repoRoot ".git"

if (-not (Test-Path $gitDir)) {
    Write-Error "Not a git repo: $repoRoot has no .git/. Run 'git init' first."
    exit 1
}

$hooksDir = Join-Path $gitDir "hooks"
if (-not (Test-Path $hooksDir)) {
    New-Item -ItemType Directory -Path $hooksDir -Force | Out-Null
}

$hookPath = Join-Path $hooksDir "pre-commit"

$hookContent = @'
#!/bin/sh
# harness-kit pre-commit hook.
# Blocks the commit if .harness/ has drifted from CLAUDE.md or .github/copilot-instructions.md.
# Tool-agnostic: catches edits from Claude Code, Copilot, Cursor, or hand-typed.
set -e
_drift=0
if command -v pwsh >/dev/null 2>&1 && [ -f scripts/harness-sync.ps1 ]; then
    pwsh -File scripts/harness-sync.ps1 -Check >/dev/null 2>&1 || _drift=1
elif command -v bash >/dev/null 2>&1 && [ -f scripts/harness-sync.sh ]; then
    bash scripts/harness-sync.sh --check >/dev/null 2>&1 || _drift=1
else
    echo "harness-kit pre-commit: neither pwsh nor bash found; skipping drift check." >&2
    exit 0
fi
if [ "$_drift" = "1" ]; then
    echo "" >&2
    echo "harness-kit: drift between .harness/ and .claude/." >&2
    echo "  .claude/agents/ and/or .claude/skills/ are stale relative to .harness/." >&2
    echo "" >&2
    echo "  Fix: pwsh -File scripts/harness-sync.ps1   (Windows)" >&2
    echo "       bash scripts/harness-sync.sh          (macOS / Linux)" >&2
    echo "  Then: git add .claude/ && git commit ..." >&2
    echo "" >&2
    echo "  Note: edits to .harness/rules/ do NOT need sync (referenced by AI-GUIDE.md, not composed)." >&2
    echo "  Bypass once (NOT recommended): git commit --no-verify" >&2
    exit 1
fi
'@

[System.IO.File]::WriteAllText($hookPath, $hookContent)

if ($IsLinux -or $IsMacOS) {
    & chmod +x $hookPath
}

Write-Host "Installed pre-commit hook at $hookPath" -ForegroundColor Green
Write-Host "  Runs harness-sync --check before every commit." -ForegroundColor DarkGray
Write-Host "  Disable: Remove-Item $hookPath" -ForegroundColor DarkGray
