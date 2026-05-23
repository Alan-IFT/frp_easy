#!/usr/bin/env bash
# verify_all.sh — Fullstack project total verification
# Generated for frp_easy (Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)) on 2026-05-16
#
# Usage:
#   ./scripts/verify_all.sh          # full run
#   ./scripts/verify_all.sh --quick  # skip e2e
#
# Exit codes:
#   0  PASS
#   1  WARN
#   2  FAIL
# SKIPs do not affect exit code (a check is SKIP when its prerequisites
# are absent, e.g. no package.json yet).
#
# CUSTOMIZE: edit the command lines below to match your actual stack.

set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

QUICK=false
for arg in "$@"; do
    case $arg in
        --quick) QUICK=true ;;
    esac
done

errors=0
warns=0
skips=0
declare -a report

step() {
    local id="$1" name="$2" status="$3" detail="${4:-}"
    case "$status" in
        PASS) echo "[$id] $name ... PASS" ;;
        SKIP) echo "[$id] $name ... SKIP"; ((skips++)) ;;
        WARN) echo "[$id] $name ... WARN"; ((warns++)) ;;
        FAIL) echo "[$id] $name ... FAIL"; [[ -n "$detail" ]] && echo "      $detail"; ((errors++)) ;;
    esac
    report+=("$id|$name|$status")
}

pkgmgr() {
    if [[ -f pnpm-lock.yaml ]]; then echo pnpm
    elif [[ -f yarn.lock ]]; then echo yarn
    else echo npm
    fi
}

echo "=== verify_all (fullstack) ==="
echo "Project: frp_easy"
echo "Stack:   Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)"
echo ""

# --- A. Static checks (require git) ---
if [[ ! -d .git ]]; then
    step "A.1" "No hardcoded secrets" "SKIP"
    step "A.2" "No .env files committed" "SKIP"
    step "A.3" "TODO/FIXME budget" "SKIP"
else
    # A.1 — secrets scan
    if git grep -E "(api[_-]?key|secret|password|token)[[:space:]]*[:=][[:space:]]*[\"'][^\"']{8,}[\"']" \
        -- ':!*.md' ':!scripts/verify_all*' ':!.harness/*' &>/dev/null; then
        step "A.1" "No hardcoded secrets" "FAIL" "see git grep results above"
    else
        step "A.1" "No hardcoded secrets" "PASS"
    fi

    # A.2 — no .env committed
    env_committed=$(git ls-files '*.env' '.env*' 2>/dev/null | grep -vE 'example|sample' || true)
    if [[ -n "$env_committed" ]]; then
        step "A.2" "No .env files committed" "FAIL" "$env_committed"
    else
        step "A.2" "No .env files committed" "PASS"
    fi

    # A.3 — TODO/FIXME budget
    todo_count=$(git grep -ic "TODO\|FIXME" -- '*.ts' '*.tsx' '*.js' '*.jsx' '*.py' '*.go' 2>/dev/null | awk -F: '{s+=$2} END {print s+0}')
    if (( todo_count > 20 )); then
        step "A.3" "TODO/FIXME budget" "WARN" "$todo_count items (limit 20)"
    else
        step "A.3" "TODO/FIXME budget" "PASS"
    fi
fi

# --- G. Go checks (require go.mod) ---
if [[ ! -f go.mod ]]; then
    step "G.1" "go vet" "SKIP"
    step "G.2" "go test ./..." "SKIP"
    step "G.3" "go build ./cmd/frp-easy" "SKIP"
else
    # G.1
    PKGS=$(go list ./... 2>/dev/null | grep -v node_modules | tr '\n' ' ')
    if [[ -z "$PKGS" ]]; then
        step "G.1" "go vet" "SKIP"
    elif go vet $PKGS 2>/tmp/go_vet_out; then
        step "G.1" "go vet" "PASS"
    else
        step "G.1" "go vet" "FAIL" "$(cat /tmp/go_vet_out)"
    fi

    # G.2
    if [[ -z "$PKGS" ]]; then
        step "G.2" "go test ./..." "SKIP"
    elif go test $PKGS 2>/tmp/go_test_out; then
        step "G.2" "go test ./..." "PASS"
    else
        step "G.2" "go test ./..." "FAIL" "$(cat /tmp/go_test_out)"
    fi

    # G.3
    if CGO_ENABLED=0 go build -o /tmp/frp-easy-verify ./cmd/frp-easy 2>/tmp/go_build_out; then
        step "G.3" "go build ./cmd/frp-easy" "PASS"
        rm -f /tmp/frp-easy-verify
    else
        step "G.3" "go build ./cmd/frp-easy" "FAIL" "$(cat /tmp/go_build_out)"
    fi
fi

# --- B. Build / test (require web/package.json) ---
if [[ ! -f web/package.json ]]; then
    step "B.1" "Install / typecheck" "SKIP"
    step "B.2" "Lint" "SKIP"
    step "B.3" "Unit tests pass" "SKIP"
    step "B.4" "Test count >= baseline" "SKIP"
else
    pushd "$ROOT/web" >/dev/null
    PM=$(pkgmgr)

    # B.1
    # 注意 `npm exec -- tsc --noEmit` 中 `--` 分隔符必需：
    # 没有它，npm 会把 --noEmit 当作 npm 自身的 flag 吞掉（npm warn Unknown cli config），
    # tsc 实际收不到 --noEmit。早期写成 `$PM exec tsc --noEmit` 让 tsc fallback 到
    # tsconfig.json 默认 emit，污染 web/src 写出 .js（T-010 修 tsconfig 加 noEmit:true
    # 是另一层防御，此处同步修正以让 typecheck 真的只 typecheck）。
    if $PM install --frozen-lockfile &>/dev/null && \
       { [[ ! -f tsconfig.json ]] || $PM exec -- tsc --noEmit &>/dev/null; }; then
        step "B.1" "Install / typecheck" "PASS"
    else
        step "B.1" "Install / typecheck" "FAIL"
    fi

    # B.2 — only if eslint config exists
    has_eslint=false
    for f in .eslintrc .eslintrc.js .eslintrc.cjs .eslintrc.json eslint.config.js eslint.config.mjs; do
        [[ -f "$f" ]] && has_eslint=true && break
    done
    if [[ "$has_eslint" == false ]]; then
        step "B.2" "Lint" "SKIP"
    elif $PM exec eslint . &>/dev/null; then
        step "B.2" "Lint" "PASS"
    else
        step "B.2" "Lint" "FAIL"
    fi

    # B.3 — only if test script exists
    if ! grep -q '"test"' package.json; then
        step "B.3" "Unit tests pass" "SKIP"
    elif $PM test &>/dev/null; then
        step "B.3" "Unit tests pass" "PASS"
    else
        step "B.3" "Unit tests pass" "FAIL"
    fi

    # B.4 — baseline check
    if [[ -f "$ROOT/scripts/baseline.json" ]] && grep -q '"test_count":\s*0' "$ROOT/scripts/baseline.json"; then
        step "B.4" "Test count >= baseline" "SKIP"
    else
        step "B.4" "Test count >= baseline" "PASS"
    fi

    # B.5 — anti-residue sentinel（T-010）：
    # tsc 早期未启用 noEmit 时会在 web/src/ 里写下 .js / .js.map 与 .ts 同名共存，
    # 让 vitest 模块解析按 .js 优先（insight-index 2026-05-19），改 .ts 测试看似无效果。
    # tsconfig.json 已加 "noEmit": true 但旧 tooling / IDE 仍可能误触；本步是闸门。
    # 命中即 FAIL；env.d.ts 是 Vite 类型声明，例外保留。
    residue=$(find src -type f \( -name '*.js' -o -name '*.js.map' \) -not -name 'env.d.ts' 2>/dev/null | head -10)
    if [[ -n "$residue" ]]; then
        step "B.5" "No tsc residue in web/src/" "FAIL" "found: $residue"
    else
        step "B.5" "No tsc residue in web/src/" "PASS"
    fi
    popd >/dev/null
fi

# --- C. E2E (require playwright config) ---
# 同时检测 web/playwright.config.ts（frp_easy 项目约定）和根目录配置
if [[ "$QUICK" == "true" ]]; then
    :  # skipped via flag, do not even emit step
elif [[ ! -f playwright.config.ts && ! -f playwright.config.js && \
        ! -f web/playwright.config.ts && ! -f web/playwright.config.js ]]; then
    step "C.1" "E2E smoke (playwright)" "SKIP"
else
    # frp_easy 约定：playwright config 位于 web/ 子目录
    if [[ -f web/playwright.config.ts || -f web/playwright.config.js ]]; then
        PLAYWRIGHT_DIR="$ROOT/web"
    else
        PLAYWRIGHT_DIR="$ROOT"
    fi
    pushd "$PLAYWRIGHT_DIR" >/dev/null
    PM=$(pkgmgr)   # 在 playwright 目录内调用，检测正确的 lockfile
    if $PM exec playwright test --project=chromium &>/dev/null; then
        step "C.1" "E2E smoke (playwright)" "PASS"
    else
        step "C.1" "E2E smoke (playwright)" "FAIL"
    fi
    popd >/dev/null
fi

# --- D. Schema (require source code) ---
# 前置条件改为检测 go.mod：本项目为 Go 项目，无 src/apps/packages 目录，
# 原条件导致 D.1 永久 SKIP（TD-3）；以 go.mod 存在作为"已有源码"判据。
if [[ ! -f go.mod ]]; then
    step "D.1" "OpenAPI / tRPC schema present" "SKIP"
elif [[ -f openapi.yaml || -f openapi.json ]]; then
    step "D.1" "OpenAPI / tRPC schema present" "PASS"
else
    step "D.1" "OpenAPI / tRPC schema present" "WARN" "no API schema found"
fi

# --- E. Project structure (Harness required) ---
[[ -f CLAUDE.md ]] && step "E.1" "CLAUDE.md present" "PASS" || step "E.1" "CLAUDE.md present" "FAIL"
[[ -f docs/workflow.md ]] && step "E.2" "workflow.md present" "PASS" || step "E.2" "workflow.md present" "FAIL"

missing_agent=""
for a in pm-orchestrator requirement-analyst solution-architect gate-reviewer developer code-reviewer qa-tester; do
    [[ -f ".harness/agents/$a.md" ]] || missing_agent="$missing_agent $a"
done
[[ -z "$missing_agent" ]] && step "E.3" "All 7 agents in .harness/agents/" "PASS" || step "E.3" "All 7 agents in .harness/agents/" "FAIL" "Missing:$missing_agent"

if [[ -f scripts/harness-sync.sh ]] && bash scripts/harness-sync.sh --check &>/dev/null; then
    step "E.4" "Binding in sync (.harness/ -> .claude/)" "PASS"
else
    step "E.4" "Binding in sync (.harness/ -> .claude/)" "FAIL" "Run scripts/harness-sync.sh"
fi

# E.5 — AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa)
if [[ ! -f AI-GUIDE.md || ! -d .harness/rules ]]; then
    step "E.5" "AI-GUIDE.md indexes every .harness/rules/*.md" "SKIP"
else
    e5_problems=""
    while IFS= read -r r; do
        name=$(basename "$r")
        if ! grep -qF ".harness/rules/$name" AI-GUIDE.md; then
            e5_problems="$e5_problems\nNot indexed: $name"
        fi
    done < <(find .harness/rules -maxdepth 1 -name '*.md' -type f)
    while IFS= read -r ref; do
        if [[ ! -f ".harness/rules/$ref" ]]; then
            e5_problems="$e5_problems\nReferences non-existent: .harness/rules/$ref"
        fi
    done < <(grep -oE '\.harness/rules/[0-9A-Za-z_\-]+\.md' AI-GUIDE.md | sed 's|\.harness/rules/||' | sort -u)
    if [[ -z "$e5_problems" ]]; then
        step "E.5" "AI-GUIDE.md indexes every .harness/rules/*.md" "PASS"
    else
        step "E.5" "AI-GUIDE.md indexes every .harness/rules/*.md" "FAIL" "$(echo -e $e5_problems)"
    fi
fi

# E.6 — Adversarial tests section required in every 06_TEST_REPORT.md
if [[ ! -d docs/features ]]; then
    step "E.6" "Adversarial tests section in completed task reports" "SKIP"
else
    bad_reports=""
    while IFS= read -r r; do
        if ! grep -qE '^##\s+Adversarial\s+tests' "$r"; then
            bad_reports="$bad_reports\n$r"
        fi
    done < <(find docs/features -name '06_TEST_REPORT.md' -type f 2>/dev/null)
    if [[ -z "$bad_reports" ]]; then
        step "E.6" "Adversarial tests section in completed task reports" "PASS"
    else
        step "E.6" "Adversarial tests section in completed task reports" "FAIL" "Missing section:$(echo -e $bad_reports)"
    fi
fi

# E.7 — All scripts/*.ps1 must start with UTF-8 BOM (EF BB BF)
# T-021: 防回归闸门 —— scripts/*.ps1 全部必须 EF BB BF 起始。
# 设计 02_SOLUTION_DESIGN.md §2.2 + §9 I-1/I-2 (全 11 个加 BOM, 严格粒度)。
if [[ ! -d scripts ]]; then
    step "E.7" "scripts/*.ps1 have UTF-8 BOM" "SKIP"
else
    e7_missing=""
    e7_found_any=false
    while IFS= read -r f; do
        e7_found_any=true
        # POSIX 字节级: head -c 3 + od -An -tx1; od 在 Alpine / 各 minimal 镜像默认存在 (xxd 不保证)
        first3=$(head -c 3 "$f" 2>/dev/null | od -An -tx1 | tr -d ' \n')
        if [[ "$first3" != "efbbbf" ]]; then
            e7_missing="$e7_missing\n$f"
        fi
    done < <(find scripts -maxdepth 1 -name '*.ps1' -type f 2>/dev/null)
    if [[ "$e7_found_any" == "false" ]]; then
        step "E.7" "scripts/*.ps1 have UTF-8 BOM" "SKIP"
    elif [[ -z "$e7_missing" ]]; then
        step "E.7" "scripts/*.ps1 have UTF-8 BOM" "PASS"
    else
        step "E.7" "scripts/*.ps1 have UTF-8 BOM" "FAIL" "$(echo -e $e7_missing)"
    fi
fi

# Summary
echo ""
echo "=== Summary ==="
pass_count=$(printf '%s\n' "${report[@]}" | grep -c PASS || true)
echo "  PASS: $pass_count"
echo "  WARN: $warns"
echo "  FAIL: $errors"
echo "  SKIP: $skips"

# History
ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
printf '{"timestamp":"%s","pass":%d,"warn":%d,"fail":%d,"skip":%d}\n' "$ts" "$pass_count" "$warns" "$errors" "$skips" >> scripts/verification_history.log

(( errors > 0 )) && exit 2
(( warns > 0 )) && exit 1
exit 0
