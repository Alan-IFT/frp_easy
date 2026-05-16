#!/usr/bin/env bash
# archive-task.sh — Archive a completed task: harvest insights, move stage docs.
# Mirror of archive-task.ps1.
#
# Usage:
#   bash scripts/archive-task.sh --task <task-slug>
#   bash scripts/archive-task.sh --task <task-slug> --dry-run

set -euo pipefail

TASK=""
DRY_RUN=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --task) TASK="$2"; shift 2 ;;
        --dry-run) DRY_RUN=true; shift ;;
        *) echo "Unknown arg: $1" >&2; exit 1 ;;
    esac
done

if [[ -z "$TASK" ]]; then
    echo "Usage: archive-task.sh --task <task-slug> [--dry-run]" >&2
    exit 1
fi

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
task_dir="$repo_root/docs/features/$TASK"
archived_root="$repo_root/docs/features/_archived"
archived_task_dir="$archived_root/$TASK"
insight_index="$repo_root/.harness/insight-index.md"
insight_history="$archived_root/insight-history.md"

if [[ ! -d "$task_dir" ]]; then
    echo "Task directory not found: $task_dir" >&2
    exit 1
fi

if [[ -d "$archived_task_dir" ]]; then
    echo "Task already archived: $archived_task_dir" >&2
    exit 1
fi

# Step 1: harvest insights from 07_DELIVERY.md
delivery_file="$task_dir/07_DELIVERY.md"
declare -a harvested
if [[ -f "$delivery_file" ]]; then
    # Extract '## Insight' section bullets
    while IFS= read -r line; do
        harvested+=("$line")
    done < <(awk '/^##[[:space:]]+Insights?[[:space:]]*$/{flag=1; next} /^##[[:space:]]/{flag=0} flag && /^[[:space:]]*-[[:space:]]/' "$delivery_file" || true)
fi

if (( ${#harvested[@]} > 0 )); then
    echo "Harvested ${#harvested[@]} insight(s) from 07_DELIVERY.md:"
    for h in "${harvested[@]}"; do echo "  $h"; done
fi

# Step 2: rotate insight-index if it would exceed 30 lines
if [[ ! -f "$insight_index" ]]; then
    echo "Warning: .harness/insight-index.md missing — creating empty"
    [[ "$DRY_RUN" == false ]] && touch "$insight_index"
fi

declare -a current
if [[ -f "$insight_index" ]]; then
    while IFS= read -r line; do
        current+=("$line")
    done < <(grep -E '^[[:space:]]*-[[:space:]]' "$insight_index" || true)
fi

total_after=$(( ${#current[@]} + ${#harvested[@]} ))
declare -a rotated
if (( total_after > 30 )); then
    rotate_count=$(( total_after - 30 ))
    echo "Rotating $rotate_count old insight(s) to insight-history.md"
    for ((i=0; i<rotate_count; i++)); do
        rotated+=("${current[$i]}")
    done
    declare -a remaining
    for ((i=rotate_count; i<${#current[@]}; i++)); do
        remaining+=("${current[$i]}")
    done

    if [[ "$DRY_RUN" == false ]]; then
        mkdir -p "$archived_root"
        if [[ ! -f "$insight_history" ]]; then
            printf '# Insight history (rotated from .harness/insight-index.md)\n\n' > "$insight_history"
        fi
        printf '\n## Rotated %s\n\n' "$(date +%Y-%m-%d)" >> "$insight_history"
        for r in "${rotated[@]}"; do echo "$r" >> "$insight_history"; done

        # Rewrite insight-index: header (non-bullet lines) + remaining + harvested
        header=$(grep -vE '^[[:space:]]*-[[:space:]]' "$insight_index" || true)
        {
            echo "$header"
            for r in "${remaining[@]}"; do echo "$r"; done
            for h in "${harvested[@]}"; do echo "$h"; done
        } > "$insight_index.tmp"
        mv "$insight_index.tmp" "$insight_index"
    fi
elif (( ${#harvested[@]} > 0 )); then
    if [[ "$DRY_RUN" == false ]]; then
        for h in "${harvested[@]}"; do echo "$h" >> "$insight_index"; done
    fi
fi

# Step 3: move task dir
if [[ "$DRY_RUN" == false ]]; then
    mkdir -p "$archived_root"
    mv "$task_dir" "$archived_task_dir"
fi

# Step 4: report
echo ""
if [[ "$DRY_RUN" == true ]]; then
    echo "[DRY RUN] No files written. Would have:"
    echo "  - Appended ${#harvested[@]} insight(s) to .harness/insight-index.md"
    echo "  - Rotated ${#rotated[@]} old insight(s) to insight-history.md"
    echo "  - Moved $task_dir -> $archived_task_dir"
else
    echo "Archived task: $TASK"
    echo "  Stage docs:   $archived_task_dir"
    if (( ${#harvested[@]} > 0 )); then
        echo "  Insights:     +${#harvested[@]} to .harness/insight-index.md"
    fi
    if (( ${#rotated[@]} > 0 )); then
        echo "  Rotated:      ${#rotated[@]} -> $insight_history"
    fi
fi
