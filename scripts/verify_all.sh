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

    # B.3 — only if test script exists. 捕获输出供 B.4 计数（NO_COLOR 去 ANSI 便于解析）。
    fe_have=-1
    if ! grep -q '"test"' package.json; then
        step "B.3" "Unit tests pass" "SKIP"
    else
        test_out=$(NO_COLOR=1 $PM test 2>&1); test_rc=$?
        if [[ $test_rc -eq 0 ]]; then
            step "B.3" "Unit tests pass" "PASS"
        else
            step "B.3" "Unit tests pass" "FAIL"
        fi
        fe_have=$(printf '%s\n' "$test_out" | grep -oE 'Tests[[:space:]]+[0-9]+ passed' | grep -oE '[0-9]+' | tail -1)
        [[ -z "$fe_have" ]] && fe_have=-1
    fi

    # B.4 — Test count >= baseline（真计数：Go 顶层测试 + 前端 vitest 用例）。
    # 任一低于 baseline.json 即 FAIL —— 杜绝"静默删测试 / role-play QA 漏跑"让红树溜过
    # （T-043/T-044 修复的根因）。Go 用 `go test -list` 计数（不执行，仅列举顶层 Test*），
    # 前端复用 B.3 捕获的 vitest "Tests N passed"。insight L26 双实现对账 + L30 反向证伪守门。
    bl="$ROOT/scripts/baseline.json"
    if [[ ! -f "$bl" ]] || grep -qE '"test_count":[[:space:]]*0' "$bl"; then
        step "B.4" "Test count >= baseline" "SKIP"
    else
        go_want=$(grep -oE '"go_tests":[[:space:]]*[0-9]+' "$bl" | grep -oE '[0-9]+' | head -1)
        fe_want=$(grep -oE '"frontend_tests":[[:space:]]*[0-9]+' "$bl" | grep -oE '[0-9]+' | head -1)
        go_have=$( cd "$ROOT" && go test -list '.*' ./... 2>/dev/null | grep -cE '^(Test|Example|Benchmark|Fuzz)' )
        b4fail=false; b4detail=""
        if [[ -n "$go_want" && "$go_have" -lt "$go_want" ]]; then b4fail=true; b4detail="Go $go_have < baseline $go_want. "; fi
        if [[ "$fe_have" -ge 0 && -n "$fe_want" && "$fe_have" -lt "$fe_want" ]]; then b4fail=true; b4detail="${b4detail}frontend $fe_have < baseline $fe_want."; fi
        if $b4fail; then
            step "B.4" "Test count >= baseline" "FAIL" "$b4detail"
        else
            step "B.4" "Test count >= baseline" "PASS"
        fi
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

# --- G. Reviewer dispatch protocol (T-034) ---
# G.1 / G.2 守门 sub-agent 工具白名单 frontmatter 在 SDK 派发上下文可能被裁剪现象。
# 仅静态守门"契约段在源码里存在"，不试图测运行时派发（不在静态闸门可达范围）。

# G.1 — reviewer agents declare PM_FALLBACK_WRITE sentinel
if [[ ! -d .harness/agents ]]; then
    step "G.1" "Reviewer agents declare PM_FALLBACK_WRITE sentinel" "SKIP"
else
    g1_missing=""
    for t in .harness/agents/gate-reviewer.md .harness/agents/code-reviewer.md; do
        if [[ ! -f "$t" ]]; then
            g1_missing="$g1_missing\n$t (MISSING)"
            continue
        fi
        if ! grep -qE 'MODE:\s*PM_FALLBACK_WRITE' "$t"; then
            g1_missing="$g1_missing\n$t (no PM_FALLBACK_WRITE sentinel)"
        fi
    done
    if [[ -z "$g1_missing" ]]; then
        step "G.1" "Reviewer agents declare PM_FALLBACK_WRITE sentinel" "PASS"
    else
        step "G.1" "Reviewer agents declare PM_FALLBACK_WRITE sentinel" "FAIL" "$(echo -e $g1_missing)"
    fi
fi

# G.2 — PM Orchestrator declares Reviewer dispatch protocol
g2_file=".harness/agents/pm-orchestrator.md"
if [[ ! -f "$g2_file" ]]; then
    step "G.2" "PM Orchestrator declares Reviewer dispatch protocol" "SKIP"
else
    g2_problems=""
    if ! grep -qE 'Reviewer\s+dispatch\s+protocol' "$g2_file"; then
        g2_problems="$g2_problems missing 'Reviewer dispatch protocol' heading;"
    fi
    if ! grep -qE 'MODE:\s*PM_FALLBACK_WRITE' "$g2_file"; then
        g2_problems="$g2_problems missing PM_FALLBACK_WRITE sentinel reference;"
    fi
    if [[ -z "$g2_problems" ]]; then
        step "G.2" "PM Orchestrator declares Reviewer dispatch protocol" "PASS"
    else
        step "G.2" "PM Orchestrator declares Reviewer dispatch protocol" "FAIL" "$g2_problems"
    fi
fi

# --- H. T-037 deletion surface guard ---
# T-037 删除了"批量代理 / 端口探测 / 折叠分组"三类辅助能力。本闸门防止未来静默回退。
# 禁词列表覆盖前端 / 后端 / OpenAPI 三层；归档 (docs/features/_archived/*) 豁免。
# 双实现对账（insight L26）：与 verify_all.ps1 H.1 行为一致——按行 grep + 同款禁词表。
if [[ ! -d .git ]]; then
    step "H.1" "T-037 deletion surface clean (no batch/probe/grouping residue)" "SKIP"
else
    h1_pattern='\b(batchMode|portsExpr|apiBatchCreate|batchProxies|UpsertProxiesTx|apiProbePorts|probePorts|probeOnePort|useProxyGrouping|groupProxiesByPrefix|BatchProxiesRequest|BatchProxiesResponse|PortProbeRequest|PortProbeResult|PortProbeResponse|ErrDuplicateTcpRemote|isDuplicateTcpRemoteError|internal/portrange)\b'
    h1_hits=$(git grep -nE "$h1_pattern" -- 'web/src/**' 'internal/**' 'openapi.yaml' \
        ':(exclude)docs/features/_archived/**' ':(exclude).harness/**' 2>/dev/null || true)
    if [[ -z "$h1_hits" ]]; then
        step "H.1" "T-037 deletion surface clean (no batch/probe/grouping residue)" "PASS"
    else
        step "H.1" "T-037 deletion surface clean (no batch/probe/grouping residue)" "FAIL" "$h1_hits"
    fi
fi

# --- I. T-038 boot-autostart-hardening static gates ---
# 守门 4 处与"开机自启硬保证"相关的字面契约，未来若有人改回旧形态会立即 FAIL。
# 锚字串 [boot-autostart-fix] 在 README / ServiceStatusCard.vue / install-service.sh --help
# 三处呈现，与 install-service.sh / .ps1 自检失败诊断段共享同款锚——dev/QA/用户都能 grep。
# 双实现对账（insight L26）：与 verify_all.ps1 I.x 行为一致——按行 grep。

# I.1 install-service.sh unit 模板含 network-online.target（实测主因 #1 守门）
i1_hits=$(grep -c 'network-online.target' scripts/install-service.sh 2>/dev/null || echo 0)
if (( i1_hits >= 2 )); then
    step "I.1" "install-service.sh unit references network-online.target (Wants+After)" "PASS"
else
    step "I.1" "install-service.sh unit references network-online.target (Wants+After)" "FAIL" "expected >=2 hits, got $i1_hits"
fi

# I.2 render.go 渲染 frpc.toml 含 LoginFailExit 字段（实测主因 #2 守门）
if grep -q 'LoginFailExit' internal/frpconf/render.go 2>/dev/null; then
    step "I.2" "frpconf/render.go has LoginFailExit field" "PASS"
else
    step "I.2" "frpconf/render.go has LoginFailExit field" "FAIL" "字段被删除会让 frpc 在首次登录失败时立即 exit"
fi

# I.3 README + ServiceStatusCard.vue + install-service.sh 三处含 [boot-autostart-fix] 锚字串
i3_missing=""
for f in README.md scripts/install-service.sh web/src/components/ServiceStatusCard.vue; do
    if [[ ! -f "$f" ]] || ! grep -q '\[boot-autostart-fix\]' "$f"; then
        i3_missing+="$f "
    fi
done
if [[ -z "$i3_missing" ]]; then
    step "I.3" "[boot-autostart-fix] anchor present in README + install-service.sh + ServiceStatusCard.vue" "PASS"
else
    step "I.3" "[boot-autostart-fix] anchor present in README + install-service.sh + ServiceStatusCard.vue" "FAIL" "missing: $i3_missing"
fi

# I.4 main.go 含 retryRestoreLoop 函数 + retryBackoff 序列（实测主因 #3 守门）
i4_problems=""
grep -q 'retryRestoreLoop' cmd/frp-easy/main.go 2>/dev/null || i4_problems+="retryRestoreLoop missing "
grep -q 'retryBackoff' cmd/frp-easy/main.go 2>/dev/null || i4_problems+="retryBackoff missing "
if [[ -z "$i4_problems" ]]; then
    step "I.4" "main.go has retryRestoreLoop + retryBackoff" "PASS"
else
    step "I.4" "main.go has retryRestoreLoop + retryBackoff" "FAIL" "$i4_problems"
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
