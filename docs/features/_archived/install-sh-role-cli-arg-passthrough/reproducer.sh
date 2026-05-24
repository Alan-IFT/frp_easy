#!/usr/bin/env bash
# T-035 Adversarial reproducer — 独立 + 可重跑 + 实测 evidence。
# 用法：bash docs/features/install-sh-role-cli-arg-passthrough/reproducer.sh
# 期望：所有 ADV-* 行打印 OK；任何 FAIL → 退非 0 + 哪条 hypothesis 复活。
#
# 设计：每条 ADV 对应 06_TEST_REPORT.md §Adversarial tests 表中一行；
#      静态可验证项在本机一次跑完。
#      docker / 真机项（AC-1 / AC-2 / AC-3 / AC-14 / AC-16）不在此文件覆盖。

set -uo pipefail
cd "$(dirname "$0")/../../.."

PASS=0
FAIL=0
report() {
    local name="$1" got="$2" want="$3"
    if [[ "$got" == "$want" ]]; then
        echo "OK   $name (got=$got)"
        PASS=$((PASS + 1))
    else
        echo "FAIL $name (got=$got, want=$want)"
        FAIL=$((FAIL + 1))
    fi
}

# ADV-1: AC-6 错误文案 3 段
got=$(bash scripts/install.sh --role bogus 2>&1 | grep -cE '推荐用法|兼容用法|诊断：')
report "ADV-1 AC-6 错误文案 3 段" "$got" "3"

# ADV-2: AC-8 sudo -E bash 残留全部位于保留段
got=$(grep -cE 'sudo -E bash' scripts/install.sh README.md docs/DEPLOYMENT.md | awk -F: '{s+=$2} END {print s}')
report "ADV-2 AC-8 sudo -E bash 命中数" "$got" "8"

# ADV-3: AC-9 help 段无过时表述
got=$(bash scripts/install.sh --help 2>&1 | grep -cE '透传环境变量|sudo -E 才能|需 -E')
report "ADV-3 AC-9 help 段无过时 sudo -E 表述" "$got" "0"

# ADV-4: AC-11 无新增 wrapper
got=$(ls scripts/install*.cmd scripts/install*.bat scripts/install-wrapper* 2>/dev/null | wc -l | tr -d ' ')
report "ADV-4 AC-11 无 wrapper 文件" "$got" "0"

# ADV-5: AC-12 last wins (--role server --role client)
got=$(bash scripts/install.sh --role server --role client 2>&1 | grep -oE 'role=[a-z]+' | head -1)
report "ADV-5 AC-12 同 flag 重复 last-wins" "$got" "role=client"

# ADV-6: AC-12 CLI 优先 env
got=$(FRP_EASY_ROLE=server bash scripts/install.sh --role client 2>&1 | grep -oE 'role=[a-z]+' | head -1)
report "ADV-6 AC-12 CLI > env 优先级" "$got" "role=client"

# ADV-7: AC-13 父 shell set -euo pipefail 不连锁中断
( set -euo pipefail; bash scripts/install.sh --role 2>&1 >/dev/null )
got="$?"
report "ADV-7 AC-13 父 shell strict 模式下子脚本 rc 透传" "$got" "3"

# ADV-8: AC-12 --role 吞参防护
out=$(bash scripts/install.sh --role --force-role 2>&1)
got="$?"
report "ADV-8 AC-12 --role 缺 value 检测" "$got" "3"
if echo "$out" | grep -q '缺少取值'; then
    echo "OK   ADV-8b 错误信息含 '缺少取值'"
    PASS=$((PASS + 1))
else
    echo "FAIL ADV-8b 错误信息缺 '缺少取值' 字段"
    FAIL=$((FAIL + 1))
fi

# ADV-9: AC-12 等号 + 空格混用 last-wins
got=$(bash scripts/install.sh --role server --role=client 2>&1 | grep -oE 'role=[a-z]+' | head -1)
report "ADV-9 AC-12 等号+空格混用 last-wins" "$got" "role=client"

# ADV-10: AC-5 env 兼容回退 + 来源标记
got=$(FRP_EASY_ROLE=server bash scripts/install.sh 2>&1 | grep -oE 'role=server\s+\(来源: 环境变量[^\)]*\)' | head -1)
if [[ -n "$got" ]]; then
    echo "OK   ADV-10 AC-5 env 兼容回退 + ROLE_SOURCE 透明标记 (got='$got')"
    PASS=$((PASS + 1))
else
    echo "FAIL ADV-10 AC-5 env 兼容回退缺 (来源: 环境变量) 标记"
    FAIL=$((FAIL + 1))
fi

# ADV-11: POSIX `--` 必须显式 — 反向证伪
out=$(echo 'echo "args=$@"' | bash -s --role client test 2>&1)
got="$?"
if [[ "$got" == "2" ]] && echo "$out" | grep -q 'invalid option'; then
    echo "OK   ADV-11 POSIX -- 漏掉 → bash 自身 rc=2 + invalid option 报错（证明设计警告必要）"
    PASS=$((PASS + 1))
else
    echo "FAIL ADV-11 漏 -- 行为意外（got rc=$got, 期望 rc=2 + invalid option）"
    FAIL=$((FAIL + 1))
fi

# ADV-12: POSIX -- 带上时 bash 完整透传
out=$(echo 'echo "args=$@"' | bash -s -- --role client test 2>&1)
got="$?"
if [[ "$got" == "0" ]] && [[ "$out" == "args=--role client test" ]]; then
    echo "OK   ADV-12 POSIX -- 带上 → bash 完整透传"
    PASS=$((PASS + 1))
else
    echo "FAIL ADV-12 带 -- 透传意外（got rc=$got, out='$out'）"
    FAIL=$((FAIL + 1))
fi

# ADV-15: README + DEPLOYMENT 主推荐字串字节级一致
ra=$(grep 'sudo bash -s -- --role server$' README.md)
da=$(grep 'sudo bash -s -- --role server$' docs/DEPLOYMENT.md)
if [[ "$ra" == "$da" ]]; then
    echo "OK   ADV-15 AC-10 README+DEPLOYMENT 主推荐字串字节一致"
    PASS=$((PASS + 1))
else
    echo "FAIL ADV-15 AC-10 跨文件 drift: README='$ra' vs DEPLOYMENT='$da'"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "===== Summary: $PASS pass / $FAIL fail ====="
[[ "$FAIL" -eq 0 ]] && exit 0 || exit 1
