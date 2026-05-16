#!/usr/bin/env bash
# harness-sync.sh - v0.10
# Binding sync: .harness/ (tool-agnostic SOT) -> .claude/ (Claude Code path requirement).
# Mirror of harness-sync.ps1. See that file for full doc.
#
# v0.10 scope: only agents + skills. Rules are no longer composed into
# CLAUDE.md or copilot-instructions.md; those are static stubs.

set -uo pipefail

CHECK=false
[[ "${1:-}" == "--check" ]] && CHECK=true

script_dir="$(cd "$(dirname "$0")" && pwd)"
project_root="$(cd "$script_dir/.." && pwd)"

harness_dir="$project_root/.harness"
claude_dir="$project_root/.claude"

if [[ ! -d "$harness_dir" ]]; then
    echo "Error: No .harness/ at $harness_dir. Run /harness-init or /harness-adopt first." >&2
    exit 1
fi

drift=()

# ---------- Copy .harness/agents/ -> .claude/agents/ ----------
harness_agents="$harness_dir/agents"
claude_agents="$claude_dir/agents"

if [[ -d "$harness_agents" ]]; then
    if [[ ! -d "$claude_agents" ]]; then
        if [[ "$CHECK" == true ]]; then
            drift+=(".claude/agents/ (missing)")
        else
            mkdir -p "$claude_agents"
        fi
    fi

    for f in "$harness_agents"/*.md; do
        [[ -f "$f" ]] || continue
        name=$(basename "$f")
        dst="$claude_agents/$name"
        needs_copy=true
        if [[ -f "$dst" ]] && cmp -s "$f" "$dst"; then
            needs_copy=false
        elif [[ -f "$dst" ]]; then
            drift+=(".claude/agents/$name (out of sync)")
        else
            drift+=(".claude/agents/$name (missing)")
        fi
        if [[ "$needs_copy" == true && "$CHECK" == false ]]; then
            cp "$f" "$dst"
            echo "Synced .claude/agents/$name"
        fi
    done

    if [[ -d "$claude_agents" ]]; then
        for f in "$claude_agents"/*.md; do
            [[ -f "$f" ]] || continue
            name=$(basename "$f")
            if [[ ! -f "$harness_agents/$name" ]]; then
                drift+=(".claude/agents/$name (orphan)")
                if [[ "$CHECK" == false ]]; then
                    rm -f "$f"
                    echo "Removed orphan .claude/agents/$name"
                fi
            fi
        done
    fi
fi

# ---------- Copy .harness/skills/ -> .claude/skills/ ----------
harness_skills="$harness_dir/skills"
claude_skills="$claude_dir/skills"

if [[ -d "$harness_skills" ]]; then
    while IFS= read -r f; do
        rel="${f#$harness_skills/}"
        dst="$claude_skills/$rel"
        needs_copy=true
        if [[ -f "$dst" ]] && cmp -s "$f" "$dst"; then
            needs_copy=false
        elif [[ -f "$dst" ]]; then
            drift+=(".claude/skills/$rel (out of sync)")
        else
            drift+=(".claude/skills/$rel (missing)")
        fi
        if [[ "$needs_copy" == true && "$CHECK" == false ]]; then
            mkdir -p "$(dirname "$dst")"
            cp "$f" "$dst"
            echo "Synced .claude/skills/$rel"
        fi
    done < <(find "$harness_skills" -type f)
fi

# ---------- Report ----------
if [[ "$CHECK" == true ]]; then
    if (( ${#drift[@]} > 0 )); then
        echo "Drift detected (${#drift[@]} item(s)):" >&2
        for d in "${drift[@]}"; do echo "  - $d" >&2; done
        echo "" >&2
        echo "Fix: run scripts/harness-sync.sh (without --check)" >&2
        exit 1
    else
        echo "In sync."
        exit 0
    fi
fi

if (( ${#drift[@]} == 0 )); then
    echo "Already in sync."
else
    echo ""
    echo "Sync complete (${#drift[@]} item(s) updated)."
fi
