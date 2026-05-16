---
name: verify
description: Run the project's total verification script (scripts/verify_all). Use this when an agent claims a task is done - this is the hard gate. Returns PASS / WARN / FAIL.
allowed-tools: Bash, PowerShell, Read
---

# Verify skill — frp_easy (Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy))

Single entry point: `scripts/verify_all.*`. Runs the full check suite
(static rules, build, tests, schema, structure).

## Steps

1. On Windows: `pwsh -File scripts/verify_all.ps1`
2. On Unix: `bash scripts/verify_all.sh`
3. Read the exit code:
   - `0` PASS — task may be marked done
   - `1` WARN — task may proceed but log warnings in the development doc
   - `2` FAIL — task is **not** done; route back to developer

## Output

- Console: per-step PASS/WARN/FAIL.
- Appended to `scripts/verification_history.log` for trend tracking.

## What to do on FAIL

- Read the failing step details.
- Route the task back to the developer (do not fix it yourself unless you are the developer).
- Save the failing report to the task's `04_DEVELOPMENT.md`.

## Do not

- Modify `verify_all.*` to make a check pass.
- Skip steps because "they aren't relevant".
- Run with `--quick` for delivery; only for in-progress checks.
