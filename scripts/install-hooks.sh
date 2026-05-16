#!/usr/bin/env bash
# install-hooks.sh - Install the harness-kit git pre-commit hook.
#
# Why: .harness/ is the source of truth; CLAUDE.md + .github/copilot-instructions.md
# are generated. Claude Code keeps them fresh via a Stop hook in .claude/settings.json,
# but that Stop hook is Claude-Code-specific — it does NOT fire for GitHub Copilot,
# Cursor, or hand-edits. This pre-commit hook is the tool-agnostic backstop: any
# commit that includes stale generated artifacts is blocked, regardless of who or
# what edited .harness/.
#
# Usage:
#   bash scripts/install-hooks.sh
#
# To disable: rm .git/hooks/pre-commit

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
git_dir="$repo_root/.git"

if [ ! -d "$git_dir" ]; then
    echo "Not a git repo: $repo_root has no .git/. Run 'git init' first." >&2
    exit 1
fi

hooks_dir="$git_dir/hooks"
mkdir -p "$hooks_dir"
hook_path="$hooks_dir/pre-commit"

cat > "$hook_path" <<'EOF'
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
EOF

chmod +x "$hook_path"
echo "Installed pre-commit hook at $hook_path"
echo "  Runs harness-sync --check before every commit."
echo "  Disable: rm $hook_path"
