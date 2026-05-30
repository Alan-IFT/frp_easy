# Test Report — T-054 archive-task-sh-regex-align

> Stage 6 / QA Tester · mode: full

## Test plan

| Acceptance criterion | Test case(s) | File / 方式 |
|---|---|---|
| AC-1 awk 模式为容错版 | 源码静态核对 `archive-task.sh:53` | Read 实证（见下） |
| AC-2 上方注释含 .ps1 对齐 + insight 引用 | 源码静态核对 `archive-task.sh:47-50` | Read 实证 |
| AC-3 §N 前缀 fixture 修复后命中 / 修复前 0 命中 | AT-1 临时 fixture + 独立 awk 双模式对照 | `## Adversarial tests` AT-1 |
| AC-4 裸 `## Insight` 不回归 | AT-2 临时 fixture + 新模式 awk | AT-2 |
| AC-5 `## Files changed` 不误命中 | AT-3 临时 fixture + 新模式 awk | AT-3 |
| AC-6 verify_all PASS FAIL=0 测试数不降 | `bash scripts/verify_all.sh` | orchestrator 硬闸门（见 §verify_all） |
| AC-7 改动面 = 1 文件 | `git diff --stat` | orchestrator 实跑 |

## 源码静态核实（AC-1 / AC-2 / 改动面）

`scripts/archive-task.sh` 改后片段（已 Read 实证 L47-53）：

```
47	    # Extract '## Insight' section bullets.
48	    # 标题正则容忍单 token 前缀（如 '## §8 Insight' / '## 8. Insight'），与 archive-task.ps1:48
49	    # 的 ^##\s+(?:[^\s\n]+\s+)?Insights?\s*$ 对齐——偿还 insight-index 记录的"双实现不对称"债
50	    # （T-035：T-028 仅修了 .ps1，.sh 仍踩 §N 前缀坑）。POSIX ERE 等价：([^[:space:]]+[[:space:]]+)? = 可选前缀 token。
53	    done < <(awk '/^##[[:space:]]+([^[:space:]]+[[:space:]]+)?Insights?[[:space:]]*$/{flag=1; next} /^##[[:space:]]/{flag=0} flag && /^[[:space:]]*-[[:space:]]/' "$delivery_file" || true)
```

AC-1 ✅（容错模式落地）、AC-2 ✅（注释 3 行含 .ps1 对齐 + T-035 insight 引用）。同行 awk 尾部 `{flag=1; next} /^##[[:space:]]/{flag=0} flag && /^[[:space:]]*-[[:space:]]/` 与改前字节一致（AC-7 改动面局部化）。

## Boundary tests added

本任务为 verify_all 闸门相邻工具改动，不新增自动化测试到套件（01 OOS-5 / GR Q4：不增减测试、不 bump baseline）。边界覆盖通过下方独立 awk 反向证伪 fixture 实现：裸标题 / §N 前缀 / N. 数字前缀 / 复数 / 非 Insight 标题 / 行尾空白六类。

## Adversarial tests

> 反向证伪契约（insight L46/L30）：构造临时 fixture，对**修复前**与**修复后**两个 awk 模式对照跑，证明修复扩大命中集合且不回归、不误命中、不污染真实 `.harness/insight-index.md`。
>
> **执行说明**：本 QA 角色在当前 SDK 派发上下文**无 Bash / PowerShell 工具**（insight L14 role-collapse；agent frontmatter `Bash` 为理论上界，运行时被裁剪）。下列 reproducer 命令为 QA 独立从 AC 编写（非抄 04 的测试），由 **batch orchestrator 在真 bash 下实跑**作为硬证据。命令直接对临时 fixture 跑 `awk`，**不调用** `archive-task.sh` 主流程的 append/move，**绝不** append 真实 index。预期输出由 POSIX ERE 语义确定性推导。

### Reproducer 脚本（QA 编写，orchestrator 实跑）

```bash
#!/usr/bin/env bash
set -u
TMP="$(mktemp -d)"

# Fixture A: §N 前缀标题
printf '# Delivery Summary\n## §9 Insight\n- prefixed fact A · evidence: T-054\n## Files changed\n- x\n' > "$TMP/A.md"
# Fixture B: 裸标题
printf '# Delivery Summary\n## Insight\n- bare fact B · evidence: T-054\n## Next steps\n- none\n' > "$TMP/B.md"
# Fixture C: 无 Insight 段
printf '# Delivery Summary\n## Files changed\n- x\n## Next steps\n- none\n' > "$TMP/C.md"
# Fixture D: '## 8.' 数字前缀
printf '## 8. Insight\n- numbered fact D · evidence: T-054\n' > "$TMP/D.md"
# Fixture E: 复数裸标题
printf '## Insights\n- plural fact E · evidence: T-054\n' > "$TMP/E.md"

OLD='/^##[[:space:]]+Insights?[[:space:]]*$/{flag=1; next} /^##[[:space:]]/{flag=0} flag && /^[[:space:]]*-[[:space:]]/'
NEW='/^##[[:space:]]+([^[:space:]]+[[:space:]]+)?Insights?[[:space:]]*$/{flag=1; next} /^##[[:space:]]/{flag=0} flag && /^[[:space:]]*-[[:space:]]/'

echo "## OLD (pre-fix) ##"
for f in A B C D E; do printf '%s: ' "$f"; awk "$OLD" "$TMP/$f.md" | grep -c . ; done
echo "## NEW (post-fix) ##"
for f in A B C D E; do printf '%s: ' "$f"; awk "$NEW" "$TMP/$f.md" | grep -c . ; done

rm -rf "$TMP"
```

### 反向证伪用例与预期

| AC | Hypothesis（"I expect …"） | Reproducer | Expected outcome（确定性推导） |
|---|---|---|---|
| AT-1 / AC-3 | 期望旧模式对 `## §9 Insight`（Fixture A）漏命中=0，新模式命中=1（证明修复确实生效，非假阳性） | 上述脚本 Fixture A，OLD vs NEW | OLD A: `0` ；NEW A: `1`。OLD 因 `^##[[:space:]]+Insights?` 要求 `##`+空白后直接 `Insight`，`§9 ` 卡死→flag 永不置 1→0 bullet；NEW 的 `([^[:space:]]+[[:space:]]+)?` 吸收 `§9 `→命中→收 1 bullet。**差异即修复证据**。 |
| AT-2 / AC-4 | 期望新模式对裸 `## Insight`（Fixture B）仍命中=1（不回归） | Fixture B，NEW | NEW B: `1`（可选组取 0 次退化为旧式，裸标题恒命中）。OLD B 同为 `1`（对照确认两模式对裸标题一致，证明超集关系）。 |
| AT-3 / AC-5 | 期望新模式对 `## Files changed`（Fixture C，无 Insight 段）命中=0（不误命中/防假阳性） | Fixture C，NEW | NEW C: `0`。`changed` 不匹配 `Insights?[[:space:]]*$`，flag 不置 1；BC-6 守门通过。 |
| AT-4 / BC-3 扩展 | 期望新模式对 `## 8. Insight`（Fixture D，数字点前缀）命中=1 | Fixture D，NEW | NEW D: `1`（`8. ` 是单 token `8.` + 空白，被可选组吸收）。OLD D: `0`（同 AT-1 漏命中）。 |
| AT-5 / BC-2 | 期望新模式对复数 `## Insights`（Fixture E）命中=1（复数不回归） | Fixture E，NEW | NEW E: `1`（`Insights?` 的 `s?` 命中复数）。OLD E: `1`（复数本就被旧模式支持，确认不回归）。 |

**预期汇总（orchestrator 实跑应得）**：
```
## OLD (pre-fix) ##
A: 0   B: 1   C: 0   D: 0   E: 1
## NEW (post-fix) ##
A: 1   B: 1   C: 0   D: 1   E: 1
```
关键差异列（A、D 从 0→1）= 修复生效证据；B、E 保持 1 = 不回归；C 保持 0 = 不误命中。**不污染真实 index**：脚本只对 `$TMP` 下 fixture 跑 `awk`，全程不触 `.harness/insight-index.md`、不调 `archive-task.sh`。

> 注：若 orchestrator 偏好用脚本自身验证，可改跑 `bash scripts/archive-task.sh --task <临时-fixture-slug> --dry-run`（脚本支持 `--dry-run`，仅打印不写盘），但需先把 fixture 放进临时 `docs/features/<临时-slug>/07_DELIVERY.md`，验毕删除。上述直接 awk 法更轻、零副作用，推荐。

## verify_all result

- **本机角色化上下文无 Bash 工具**，无法本地跑。由 batch orchestrator 独立真跑 `bash scripts/verify_all.sh` 作硬闸门。
- 预期：PASS，FAIL=0，go_tests/frontend_tests/test_count 与批次基线（32/0/0）一致——本任务不增减测试、不改 verify_all / baseline.json。
- E.6 自检：本报告 `## Adversarial tests` 为裸标题（无 `## 3.` / `§N` 前缀），符合 `verify_all.sh:287` `^##\s+Adversarial\s+tests` 锚定（insight L40）。

## Defects found

无。

## Stability

- 反向证伪为纯确定性 awk 文本匹配，无随机性、无 I/O 竞争、无 flake 面。fixture 内容固定→输出恒定。orchestrator 实跑应一次稳定复现预期汇总表。

## Verdict

`APPROVED FOR DELIVERY`

（AC-6 verify_all 硬闸门 + AT 反向证伪实跑由 batch orchestrator 独立执行；本报告提供 reproducer 命令 + 确定性预期作为执行规格。若 orchestrator 实跑结果与上述汇总表不符，应回退 developer 而非接受。）
