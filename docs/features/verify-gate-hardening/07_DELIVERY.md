# 07_DELIVERY — T-044 verify-gate-hardening

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05

## 需求

堵死让 T-038~T-042 红树（39 个前端测试失败 + E.6 红）通过 verify_all 假报 PASS 交付的闸门洞，让验证闸门长期不可被静默绕过；并恢复 E.6 绿。

## 根因（三洞叠加）

1. **`.ps1` B.3 是瞎的**：`& $pkgMgr test 2>&1 | Out-Null` 从不检查 `$LASTEXITCODE`，vitest 退出码非 0 也走 Step 的"无异常即 PASS"分支。上批次跑 `pwsh verify_all.ps1` 报"31 PASS / 1 FAIL"漏掉 39 个前端失败的**直接根因**。（`.sh` B.3 靠 `elif $PM test` 的退出码判定，本来就有效。）
2. **B.4「测试数 ≥ 基线」双实现都是空操作**：`.ps1` 只有 `# CUSTOMIZE` 注释从不比较；`.sh` 只要 `test_count != 0` 就 PASS。删测试 / 测试变少永不被发现。
3. **baseline.json 自 T-036 起过期**：写 451/265/186，实际 Go 285 + 前端 297 = 582。即便 B.4 修好也比错。

## 方案与改动

- **`.ps1` B.3**：`NO_COLOR=1` 跑测试并 `Out-String` 捕获 → 取 `$LASTEXITCODE`，非 0 即 `throw`（真失败）；顺带正则 `Tests\s+(\d+)\s+passed` 采集前端用例数存 `$script:feTestCount` 供 B.4。
- **B.4 双实现真计数**（`.ps1` + `.sh` 对称，insight L26）：
  - Go 顶层测试数 = `go test -list '.*' ./...`（仅列举不执行，cheap）过滤 `^(Test|Example|Benchmark|Fuzz)` 计数。
  - 前端用例数 = B.3 采集的 vitest "Tests N passed"。
  - 任一低于 baseline.json 对应字段即 FAIL（带 "Go X < baseline Y" 明细）。
- **baseline.json** 重置到真实计数（version 17；go_tests=285 / frontend_tests=297 / test_count=582）+ 详尽 notes 记录三洞与修复，并明确"后续每个加测试的任务必须同步 bump"。
- **E.6**：T-038/T-039/T-040 三个归档报告本就有完整 `## Adversarial tests` 段（ADV-A~F 等），但标题带数字前缀（`## 4. / ## 3.`）被 E.6 严格裸标题正则 `^##\s+Adversarial\s+tests\s*$` 拒绝（正是 insight L40 预言的姊妹陷阱）。归一化为裸标题 —— 内容零改动，只去前缀。

## 验证

- `bash scripts/verify_all.sh --quick`：**PASS 31 / WARN 0 / FAIL 0**（B.3/B.4/E.6 全绿，基线从批次启动时 FAIL 2 恢复）。
- `.ps1` 因用户 PowerShell deny 规则无法在本会话运行；改动与 `.sh` 严格对称、逐行复核（`@(...)` 包裹 Where-Object 防单结果 .Count 不稳；`$script:feTestCount` 脚本作用域跨 B.3/B.4）。

## Adversarial tests

- **B.4 反向证伪（insight L30）**：临时把 baseline 抬到 go_tests=99999 / frontend_tests=88888 → `verify_all.sh --quick` 的 B.4 精确 FAIL 并打印 "Go 285 < baseline 99999. frontend 297 < baseline 88888." → 恢复后 PASS。证明 B.4 真计数、非空操作、非假阳性。
- **E.6 反向证伪**：归一化前 3 报告标题带前缀 → E.6 FAIL；去前缀 → PASS（与修复前后实测一致）。
- `.ps1` B.3 退出码逻辑：`if ($code -ne 0) { throw }` 是无歧义的小改；旧版无此判定故假报 PASS（已由本任务的根因分析反推证明）。

## Insight

- verify_all 双实现必须**逐桩对账**：`.sh` 的 B.3 有效但 `.ps1` 的 B.3 瞎了，导致跑哪个脚本结果不同 —— 这是 insight L26"双实现对账原则"被违反的真实代价（红树交付）。任何 verify_all 改动必须同时改 .ps1 + .sh 并各自反向证伪。
- 静态计数闸门要真比较，"读了 baseline 但不比较"等于没有闸门。Go 用 `go test -list` 顶层计数（稳定，无子测试膨胀）、前端复用测试运行输出的 "Tests N passed"（`NO_COLOR=1` 去 ANSI 便于正则），是低成本可维护的双语言计数范式。
- PowerShell Step 模式下"native 命令失败"必须显式 `throw`（查 `$LASTEXITCODE`），不能依赖管道；`& cmd | Out-Null` 会吞掉退出码让失败静默通过。
- `## Adversarial tests`（及 `## Insight`）标题禁带任何前缀（数字编号 `## N.`、`§N` 均不行），E.6/archive 正则严格锚定裸标题。PM 写 06 模板时应硬约束（insight L40 第三次复现：T-038/039/040 三连犯）。
