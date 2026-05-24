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

# E.7a/b/c — T-026: 拆分 T-021 的全量 BOM 检查为 white-list 驱动，容纳 install.ps1 反向规则。
# install.ps1 是 irm | iex 入口；BOM 会被 Invoke-RestMethod 解码为 U+FEFF 进入字符串触发
# ParserError。其余 10 个 .ps1 仍是磁盘形态调用，PS5.1 + zh-CN 主机无 BOM 时会按 host
# ANSI codepage (GBK) 误解码中文，必须保留 BOM。
PS1_REQUIRE_BOM=(archive-task.ps1 build.ps1 harness-sync.ps1 install-hooks.ps1 \
    install-service.ps1 package.ps1 start-e2e-server.ps1 start.ps1 \
    uninstall-service.ps1 verify_all.ps1)
PS1_FORBID_BOM=(install.ps1)

if [[ ! -d scripts ]]; then
    step "E.7a" "BOM-required scripts/*.ps1 have UTF-8 BOM" "SKIP"
    step "E.7b" "iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM" "SKIP"
    step "E.7c" "All scripts/*.ps1 classified in E.7a or E.7b" "SKIP"
else
    # E.7a
    e7a_missing=""
    for name in "${PS1_REQUIRE_BOM[@]}"; do
        f="scripts/$name"
        if [[ ! -f "$f" ]]; then e7a_missing="$e7a_missing\n$name (MISSING)"; continue; fi
        first3=$(head -c 3 "$f" 2>/dev/null | od -An -tx1 | tr -d ' \n')
        if [[ "$first3" != "efbbbf" ]]; then e7a_missing="$e7a_missing\n$name"; fi
    done
    if [[ -z "$e7a_missing" ]]; then
        step "E.7a" "BOM-required scripts/*.ps1 have UTF-8 BOM" "PASS"
    else
        step "E.7a" "BOM-required scripts/*.ps1 have UTF-8 BOM" "FAIL" "$(echo -e $e7a_missing)"
    fi

    # E.7b
    e7b_wrong=""
    for name in "${PS1_FORBID_BOM[@]}"; do
        f="scripts/$name"
        if [[ ! -f "$f" ]]; then e7b_wrong="$e7b_wrong\n$name (MISSING)"; continue; fi
        first3=$(head -c 3 "$f" 2>/dev/null | od -An -tx1 | tr -d ' \n')
        if [[ "$first3" == "efbbbf" ]]; then e7b_wrong="$e7b_wrong\n$name"; fi
    done
    if [[ -z "$e7b_wrong" ]]; then
        step "E.7b" "iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM" "PASS"
    else
        step "E.7b" "iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM" "FAIL" "$(echo -e $e7b_wrong)"
    fi

    # E.7c — 防漏白名单：未在两表的 .ps1 触发 WARN，提醒维护者归类
    known=" ${PS1_REQUIRE_BOM[*]} ${PS1_FORBID_BOM[*]} "
    e7c_unclassified=""
    while IFS= read -r f; do
        base=$(basename "$f")
        if [[ "$known" != *" $base "* ]]; then
            e7c_unclassified="$e7c_unclassified\n$base"
        fi
    done < <(find scripts -maxdepth 1 -name '*.ps1' -type f 2>/dev/null)
    if [[ -z "$e7c_unclassified" ]]; then
        step "E.7c" "All scripts/*.ps1 classified in E.7a or E.7b" "PASS"
    else
        # G-7 必修条件：WARN 分支显式打印 unclassified 文件名（依 03 §8 G-7 增补）
        echo "    unclassified:$(echo -e $e7c_unclassified)"
        step "E.7c" "All scripts/*.ps1 classified in E.7a or E.7b" "WARN" "$(echo -e $e7c_unclassified)"
    fi
fi

# T-031 E.8 — AC-5 静态闸门：install.ps1 / install-service.ps1 禁交互阻塞（FR-3 硬红线）
# grep -nE 天然按行扫描，'^' / '$' 按行匹配（无需 PCRE multiline 标志）。
# 跳过 # 开头注释行 + 含元描述词（禁/forbidden/FR-3/red.?line）的合法字面量行。
e8_hits=""
for t in scripts/install.ps1 scripts/install-service.ps1; do
    [[ -f "$t" ]] || continue
    while IFS= read -r ln; do
        [[ -z "$ln" ]] && continue
        content=$(echo "$ln" | cut -d: -f2-)
        trimmed=$(echo "$content" | sed 's/^[[:space:]]*//')
        # 跳过注释
        [[ "$trimmed" =~ ^# ]] && continue
        # 跳过含元描述词
        echo "$content" | grep -qE '禁|red\.?line|forbidden|FR-3|破\s*FR-3' && continue
        e8_hits="$e8_hits\n$t:$ln"
    done < <(grep -nE 'Read-Host|\[Console\]::ReadKey|^[[:space:]]*pause[[:space:]]*$|Wait-Event' "$t" 2>/dev/null || true)
done
if [[ -z "$e8_hits" ]]; then
    step "E.8" "install.ps1 / install-service.ps1 forbid interactive blockers" "PASS"
else
    step "E.8" "install.ps1 / install-service.ps1 forbid interactive blockers" "FAIL" "$(echo -e $e8_hits)"
fi

# T-031 E.9 — AC-10 静态闸门：无 wrapper.cmd / install*.bat
e9_stray=$(find scripts -maxdepth 1 -type f \( -iname 'install*.cmd' -o -iname 'install*.bat' \) 2>/dev/null)
if [[ -z "$e9_stray" ]]; then
    step "E.9" "No wrapper.cmd / install*.bat in scripts/" "PASS"
else
    step "E.9" "No wrapper.cmd / install*.bat in scripts/" "FAIL" "$e9_stray"
fi

# T-031 E.10 — README Windows 入口必须含 -NoExit
if [[ ! -f README.md ]]; then
    step "E.10" "README Windows install entry contains -NoExit" "SKIP"
else
    entry=$(awk '/\*\*Windows\*\*/{f=1} f && /^```powershell/{p=1; next} p && /^```/{exit} p' README.md)
    if [[ -z "$entry" ]]; then
        step "E.10" "README Windows install entry contains -NoExit" "FAIL" "Windows powershell block not found"
    elif echo "$entry" | grep -q -- '-NoExit'; then
        step "E.10" "README Windows install entry contains -NoExit" "PASS"
    else
        step "E.10" "README Windows install entry contains -NoExit" "FAIL" "$entry"
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
