---
name: test
description: Run unit tests for frp_easy. Use after build passes, before verify_all.
allowed-tools: Bash, PowerShell, Read
---

# Test skill — frp_easy (Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy))

## Steps

1. Detect package manager (see build skill).
2. Run:
   ```
   <pm> test
   ```
3. Compare test count to `scripts/baseline.json`:
   - If test count decreased → **FAIL**. Tests were deleted; investigate.
   - If test count increased → OK; the qa-tester agent will update baseline on delivery.

## Pass criteria

- All tests pass (exit 0).
- Test count >= baseline.

## Flake handling

If a test passes 9/10 runs, treat as **flaky**, file as MAJOR defect, do not retry forever.
