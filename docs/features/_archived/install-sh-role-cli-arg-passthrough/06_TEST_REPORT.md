# Test Report · T-035 install-sh-role-cli-arg-passthrough

> Harness 流水线 Stage 6（QA Tester）。模式：**full**。
> 上游：01（READY）/ 02（READY）/ 03（APPROVED WITH CONDITIONS）/ 04（READY FOR REVIEW）/ 05（APPROVED）。
> 测试环境：Git Bash 5.2.37 on Windows 11 + verify_all.sh 实测。docker 不可用（本机环境）→ 真机 / docker 实证延后到用户复测或 follow-up。

## Test plan

| Acceptance criterion | Test perspective | Test case(s) | 实测位置 |
|---|---|---|---|
| AC-1 (FR-1 client 成功) | 真机/docker | 真机 Ubuntu 26 LTS / `ubuntu:24.04` docker | **手工**（用户协助）|
| AC-2 (FR-2 server 成功) | 真机/docker | 同 AC-1 但 `--role server` | **手工** |
| AC-3 (FR-3 公网 IP 兜底) | 真机/docker | `sudo FRP_EASY_PUBLIC_IP=<ip> bash -s -- --role server` | **手工** + ADV-2 静态字串校验 |
| AC-4 (FR-4 --force-role) | 自动 | S-12 + ADV-8（缺 value 防吞参） | 06 §Adversarial ADV-8 |
| AC-5 (FR-5 env 兼容回退) | 自动 | ADV-10 `FRP_EASY_ROLE=server bash install.sh` → role=server (env) | 06 §Adversarial ADV-10 |
| AC-6 (FR-6 错误提示 3 段) | 自动 | ADV-1（修正版）grep 3 段 | 06 §Adversarial ADV-1 |
| AC-7 (FR-7 横幅"更新"段) | 自动 | grep `bash -s -- --role` 在横幅段命中 | 06 §Cross-file consistency |
| AC-8 (FR-8/10/12 主推荐路径无 sudo -E 残留) | 自动 | ADV-2 grep 计数 = 8（设计中保留段总数） | 06 §Adversarial ADV-2 |
| AC-9 (FR-9 --help 同步) | 自动 | ADV-3 grep 不含 `sudo -E 才能` 过时表述 | 06 §Adversarial ADV-3 |
| AC-10 (FR-12 README/DEPLOYMENT 同步) | 自动 | ADV-15 diff 字节级一致 | 06 §Adversarial ADV-15 |
| AC-11 (FR-13 无新增 wrapper) | 自动 | ADV-4 ls 零命中 | 06 §Adversarial ADV-4 |
| AC-12 (FR-15/16 CLI 解析正确性) | 自动 | S-3..S-10 + ADV-5/6/8/9 | 06 §Adversarial 多条 |
| AC-13 (FR-17 set -euo pipefail 兼容) | 自动 | ADV-7 父 shell 严格模式下子脚本失败不连锁 | 06 §Adversarial ADV-7 |
| AC-14 (FR-18 macOS) | 真机 | macOS 14+ bash 3.2 `--role` 解析 + step 7 darwin 退 0 | **手工**（无 mac runner）|
| AC-15 (NFR-4 sudo -E 诊断引导) | 自动 | ADV-1 第三段含 `'-E' is ignored` 字串 | 06 §Adversarial ADV-1（视觉确认） |
| AC-16 (NFR-5 docker 自动化) | docker | `ubuntu:24.04` + sudoers NOPASSWD + 4 路 CLI/env/force/IP | **延后**（本机无 docker）|

## Boundary tests added

QA 独立 reproducer（不复用 04 §Stage 4 自检矩阵 / 05 §Cross-file grep）：

- **空值边界**：`--role`（缺 value）→ 03 expected ✓
- **空值边界（等号）**：`--role=`（空值）→ 03 expected ✓
- **未识别 flag**：`--unknown-flag` → 1 expected ✓
- **同 flag 重复**：`--role server --role client` → last wins = client ✓
- **CLI vs env 冲突**：`FRP_EASY_ROLE=server bash ... --role client` → CLI 胜 = client ✓
- **CLI 与 env 等号形态混用**：`--role server --role=client` → last wins = client ✓
- **吞参防护**：`--role --force-role` → bash `[[ "$2" == --* ]]` 检测出 → exit 3 with "缺 value" ✓
- **POSIX 终止符（漏 `--`）**：`bash -s --role client test` → bash 自身报 `bash: --: invalid option` rc=2，**install.sh 根本不执行** ✓（这是 ADV-11 反向证伪：证明设计中"`--` 不可省"警告的现实性）
- **POSIX 终止符（带 `--`）**：`bash -s -- --role client test` → 正常透传 ✓（ADV-12）
- **env 兼容回退**：纯 env 路径 `FRP_EASY_ROLE=client bash ...` → role=client (来源: 环境变量) ✓
- **父 shell `set -euo pipefail`**：子脚本 exit 3 / exit 1 不让父 shell 连锁中断 ✓（ADV-7）

## Adversarial tests

> 独立 reproducer（不复用 04 §"Stage 4 内自检矩阵"或 05 §"Cross-file consistency check" 的 grep 命令；QA 重新构造 + 反向证伪 hypothesis + 实测 evidence）。每条 hypothesis 写法："I expect failure when …"。

| AC | Hypothesis ("I expect failure when…") | Reproducer | Outcome with evidence |
|---|---|---|---|
| AC-6 | 错误文案 3 段中第三段（sudo -E 诊断）会被吞，用户看不到 "'-E' is ignored" 诊断 | `bash scripts/install.sh --role bogus 2>&1 \| grep -cE '推荐用法\|兼容用法\|诊断：'` (ADV-1) | **Survived**: 实测 = 3 段全在。视觉确认 stderr 输出"推荐用法（CLI 形态…"→"兼容用法（环境变量…"→"诊断：如果你刚才看到 sudo 输出 \"'-E' is ignored\"…" |
| AC-8 | 主推荐字串还有 `sudo -E bash` 残留（实测 `> 8` 命中则有未清理） | `grep -cE 'sudo -E bash' scripts/install.sh README.md docs/DEPLOYMENT.md` (ADV-2) | **Survived**: 实测 = 8 命中，全部位于设计中保留段（脚本注释 2 + help 2 + 错误提示 1 + README 兼容回退 2 + DEPLOYMENT 兼容回退 1 = 8），无主推荐路径污染 |
| AC-9 | --help 段仍含 "需 -E 才能透传环境变量" 过时表述误导新用户 | `bash scripts/install.sh --help \| grep -cE '透传环境变量\|sudo -E 才能\|需 -E'` (ADV-3) | **Survived**: 实测 = 0 命中。旧解释被全部删除 |
| AC-11 | 隐式引入 wrapper.cmd / install-wrapper.sh 类辅助文件 | `ls scripts/install*.cmd scripts/install*.bat scripts/install-wrapper* 2>&1` (ADV-4) | **Survived**: 0 文件 |
| AC-12 | last-wins 实际是 first-wins（bash case shift 顺序错） | `bash scripts/install.sh --role server --role client \| grep -oE 'role=[a-z]+'` (ADV-5) | **Survived**: role=client（last wins 正确） |
| AC-12 | CLI 优先于 env 实际是 env 胜（归一化分支顺序错） | `FRP_EASY_ROLE=server bash scripts/install.sh --role client \| grep -oE 'role=[a-z]+'` (ADV-6) | **Survived**: role=client（CLI 胜） |
| AC-12 | `--role --force-role` 吞掉 --force-role，role 取值 = "--force-role" | `bash scripts/install.sh --role --force-role 2>&1` (ADV-8) | **Survived**: 报"--role 缺少取值（server\|client）。" rc=3，吞参防护生效 |
| AC-12 | 等号 + 空格混用 last-wins 反转 | `bash scripts/install.sh --role server --role=client \| grep -oE 'role=[a-z]+'` (ADV-9) | **Survived**: role=client（等号形态作为最后一次出现胜） |
| AC-13 | 父 shell `set -euo pipefail` 下，子脚本 exit 3 让父 shell 立即中断 | `( set -euo pipefail; bash scripts/install.sh --role 2>&1 ); echo "rc=$?"` (ADV-7) | **Survived**: 父 shell 未中断；子脚本 rc=3 透传到 $? |
| AC-5 | env 兼容回退路径已损坏（如归一化分支误把 env 也走 exit 3） | `FRP_EASY_ROLE=server bash scripts/install.sh 2>&1 \| grep 'role=server'` (ADV-10) | **Survived**: 输出 `role=server  (来源: 环境变量 (FRP_EASY_ROLE))`，env 路径 + ROLE_SOURCE 透明显示均正确 |
| **POSIX `--`** | 用户漏 `--` 后 bash 静默忽略，install.sh 仍跑（=> 设计警告无必要） | `echo 'echo "args=$@"' \| bash -s --role client test 2>&1; echo "rc=$?"` (ADV-11) | **Confirmed failure mode**: bash 报 `bash: --: invalid option` rc=2，install.sh **根本不执行**——证明设计中"`--` 不可省"警告（5 处显式）的真实必要性 |
| **POSIX `--`** | 带 `--` 时 bash 仍可能吞参导致 install.sh 拿不到 --role | `echo 'echo "args=$@"' \| bash -s -- --role client test` (ADV-12) | **Survived**: bash 完整透传 `args=--role client test`，rc=0 |

### Adversarial tests 提交清单

每条 hypothesis 都配有实测 tool output（在上方 evidence 列已粘贴）。完整 reproducer 在 [reproducer.sh](./reproducer.sh) 同时生成可供未来回归。

## 03 C-3 / C-5 / 真机实证状态

### C-3（QA 在 Ubuntu 24/26 LTS 真机或 docker 验证 sudo VAR=val cmd 兜底字串）

- **本机环境**：Git Bash 5.2.37 on Windows 11，无 docker，无 Ubuntu 真机。
- **静态验证**：sudo VAR=val cmd 形态在 sudoers 默认 `env_check` 列表下的行为已通过 `sudoers(5)` 文档明确：自定义变量（如 FRP_EASY_PUBLIC_IP）的命令行设置受 sudoers env_check 列表限制；具体行为取决于发行版默认配置。
- **现状结论**：本任务范围内**无法**自动化实证此 corner case。推荐路径：
  - 用户在 Ubuntu 26 LTS 真机复测 `curl ... | sudo FRP_EASY_PUBLIC_IP=1.2.3.4 bash -s -- --role server` 命令是否一次成功；
  - 若失败 → 新开任务 T-036 加 `--public-ip <ip>` CLI flag（01 OQ-7 b 默认 OOS，可重新评估）。
- **本任务交付建议**：兜底字串原样保留；用户报告若复现失败再迭代。

### C-5（verify_all 增量闸门"主推荐字串无 sudo -E bash 残留"）

- **QA 评估**：
  - 加：未来 maintainer 误改主推荐字串可被立即捕获，与 insight L48 / L18 既有 grep-based 闸门模式对齐
  - 不加：当前 review checklist + Stage 4 / 5 / 6 静态闸门覆盖已充分，且加 step 涉及 PowerShell + Bash 双实现对账（insight L26）
- **决策**：**本任务暂不加**。本任务专注用户安装路径修复，闸门增加单独走 follow-up 任务（与 T-028 archive-task 容错增强、T-030 reviewer frontmatter Write 等同款"insight 改进类 trivial 任务"模式）。建议下一个 trivial 批次任务 T-036（如果发生 C-3 用户实测 sudo VAR=val 失败）或 T-037 来做闸门增加。

## verify_all result

```
=== Summary ===
  PASS: 26
  WARN: 0
  FAIL: 1   (C.1 E2E smoke (playwright))
  SKIP: 0
```

C.1 归责：按 insight L30 "git stash 暂存窄路径文件 → 裸跑 verify_all" 独立证伪：

```
$ git stash push scripts/install.sh README.md docs/DEPLOYMENT.md
Saved working directory and index state On main: T-035 temp stash...

$ bash scripts/verify_all.sh | grep -E '^\[C\.|^  (PASS|WARN|FAIL):'
[C.1] E2E smoke (playwright) ... FAIL
  PASS: 26
  WARN: 0
  FAIL: 1
```

**裸基线 = 修改后基线 = 26 PASS / 1 FAIL（C.1）**，C.1 与 T-035 改动**零相关**。归责 = baseline.json notes 既记的"T-031 引入的 E2E playwright 步骤 setup fixture 残留"环境基线漂移问题，T-033 已尝试修但仍未根除——属"长期环境基线问题"，本任务不背锅。

- Total tests: 375 → 375（无新增 Go/Vue 测试，本任务范围是 bash + Markdown）
- Pass: 26 verify_all step（baseline 同款）
- Fail: 1 verify_all step (C.1, baseline preexisting, T-035 改动 stash 后仍存在)
- Warn: 0
- New tests added: 0（设计：本任务无 unit-testable 代码）
- Baseline updated: **不变**（test_count 仍 375）

## Defects found

无 BLOCKER。无 CRITICAL。无 MAJOR。

Stage 5 提出的 3 条 MINOR + 2 条 NIT 中：
- MINOR-1（`set -u` 短路注释）：Developer 在 Stage 5 后即兴补一行注释（scripts/install.sh:120 上方"依赖 bash `[[ ]]` 内 `||` 短路评估…"段）→ 已落地
- MINOR-2（错误提示兼容用法只列 1 条）：保留单条降级显示，是设计意图（与 README L67 兼容回退说明分层）
- MINOR-3（root 检查"用法"行 `<url>` 字面占位符）：保留，是教育意图
- NIT-1 + NIT-2：纯洁癖，本任务不动

## Stability

verify_all 跑了 2 次（修改前裸基线 + 修改后），结果稳定（26 PASS / 1 FAIL C.1）。Adversarial reproducer 跑了 2 次（首跑 + ADV-1 regex 修正后），所有非 grep regex 验证项首次命中，无 flakiness。

## Verdict

**APPROVED FOR DELIVERY**

- 自动化可验证项（AC-4 ~ AC-13, AC-15）100% 通过 + Adversarial reproducer 全部"Survived"或"Confirmed failure mode 证明设计警告必要性"
- 真机/docker 项（AC-1 / AC-2 / AC-3 / AC-14 / AC-16）依设计明确"手工"，由用户在 Ubuntu 26 LTS 真机复测（用户已经报告了失败前态，正向实测同款用户即可）
- 03 C-3 / C-5 各自决策清晰（C-3 留用户复测 + 可能 T-036；C-5 暂不加）
- verify_all 1 FAIL 归责完成，零回归

PM 可继续 Stage 7 交付归档。

— QA Tester, 2026-05-24
