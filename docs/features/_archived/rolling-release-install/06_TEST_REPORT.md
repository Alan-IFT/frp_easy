# 06 测试报告 — T-013 rolling-release-install

> Harness 7-stage 流水线 stage 6 产出。语言：中文。
> 上游：01（READY FOR DESIGN）、02（READY）、03（APPROVED WITH CONDITIONS，5 条开发期条件）、04（含 1 处合法 DESIGN DRIFT）、05（APPROVED，1 MINOR）。
> QA 独立核对 git 工作区实际改动（未 commit）。本任务无 .go/.ts 产品代码改动，无可加入自动化套件的新单元测试；验证以静态校验、对抗性推演、独立复现 install 脚本分流逻辑为主。

## 结论摘要

- **verify_all：PASS 19 / WARN 0 / FAIL 0**（PowerShell + Git Bash 双 shell 一致），满足 AC-15（PASS ≥ 基线 19、不新增 WARN/FAIL）。
- **shellcheck 已在本环境实跑成功**：经 `winget install koalaman.shellcheck` 装上 shellcheck 0.11.0，`shellcheck scripts/install.sh` 退出码 0、0 告警 —— **Code Review M-1 / Gate 条件 3③ 闭环**，不再依赖人工走查替代。
- 4 项静态校验全部通过：YAML 解析（含 actionlint）/ `bash -n` / `shellcheck` / pwsh 解析。
- 对抗性验证：5 项关键证伪点全部「实现存活」，无新缺陷。
- DESIGN DRIFT（BC-7/BC-8 退化方案）复核合法。

---

## Test plan（验收标准 → 验证手段映射）

| 验收标准 | 验证手段 | 文件 | 结果 |
|---|---|---|---|
| AC-1 release.yml 合法 YAML/workflow | `python yaml.safe_load` + `actionlint` | `.github/workflows/release.yml` | PASS |
| AC-2 `on:` 同含 push.branches(main) + push.tags(v*) | YAML 解析后核查 `on` 键 | release.yml | PASS |
| AC-3 main push 创建/更新固定标签滚动发布步骤、标签字面量 | grep + 源码走查 | release.yml L94-109 `tag_name: rolling` | PASS |
| AC-4 BC-7 标签移动 + BC-8 旧资产替换逻辑可定位 | 源码走查 | release.yml L72-92（Move rolling tag / Purge old rolling assets） | PASS |
| AC-5 v* 路径版本化 Release 逻辑保留、行为同 T-010 | 源码对比 + if 互斥推演 | release.yml L111-126 | PASS |
| AC-6 `bash -n scripts/install.sh` 通过 | `bash -n` | install.sh | PASS |
| AC-7 install.sh 过 shellcheck 无 error | `shellcheck`（实跑，0.11.0） | install.sh | **PASS（实跑闭环）** |
| AC-8 install.ps1 可被 PowerShell 解析 | `[Parser]::ParseFile` + `[ScriptBlock]::Create` | install.ps1 | PASS |
| AC-9 两脚本端点已调整且逐字一致 | grep 提取 URL 比对 | install.sh L26 / install.ps1 L31 | PASS |
| AC-10 BC-1..4 中文 stderr + 非 0 退出可定位 | grep + 独立复现分流逻辑 | install.sh / install.ps1 | PASS |
| AC-11 `set -euo pipefail` / `$ErrorActionPreference="Stop"` 保留 | grep + diff 走查 | install.sh L23 / install.ps1 L28 | PASS |
| AC-12 DEPLOYMENT.md 发布说明改为滚动发布语义、下载地址稳定 URL | 目视 + grep | DEPLOYMENT.md L70-72 / L90-91 | PASS |
| AC-13 README 同时描述两条发布路径 | 目视 + grep | README.md L45 | PASS |
| AC-14 release.yml 顶部注释更新、述两路径 + 常规无需手动发版 | 目视 | release.yml L1-18 | PASS |
| AC-15 verify_all PASS ≥ 19、无新增 WARN/FAIL | 跑 verify_all（双 shell） | — | PASS |
| AC-16 push main 后 CI 真实产出滚动发布 | 【集成/人工 — 交付后验证，非闸门】 | — | 未验证（设计 R-6/PM 决策 5） |
| AC-17 滚动发布产生后真实联网一键安装跑通 | 【集成/人工 — 交付后验证，非闸门】 | — | 未验证（同上） |

15 条完成闸门 AC（AC-1..AC-15）全部 PASS。AC-16/AC-17 为交付后人工验证、非完成闸门，QA 不在流水线内触发真实 CI / 真实 push main。

## 4 项静态校验证据（Gate 条件 3 / Code Review M-1 闭环）

```
① AC-1  release.yml YAML 解析
   $ python -c "import yaml; d=yaml.safe_load(open('.github/workflows/release.yml'))"
   YAML safe_load OK; top keys: ['name', True('on'), 'permissions', 'concurrency', 'jobs']
   on: {'push': {'branches': ['main'], 'tags': ['v*']}}
   $ actionlint .github/workflows/release.yml
   actionlint exit code: 0   → 0 告警（YAML 之上额外校验 Actions 表达式/step 引用）

② AC-6  $ bash -n scripts/install.sh
   bash -n OK: 无语法错误

③ AC-7  $ shellcheck scripts/install.sh        ← 本环境实跑（winget 装 shellcheck 0.11.0）
   shellcheck exit code: 0   → 0 告警（M-1 闭环，无需人工走查替代）

④ AC-8  $ [Parser]::ParseFile(install.ps1)
   Parser OK: install.ps1 解析通过，0 错误
   ScriptBlock::Create OK
```

> 说明 M-1：05 Code Review 的唯一 MINOR 是「开发环境无 shellcheck，AC-7 以 bash -n + 人工走查替代，建议 QA 在具备 shellcheck 的环境补跑」。QA 已用 `winget install --id koalaman.shellcheck` 在本 Windows 环境装上 shellcheck 0.11.0 并实跑 `shellcheck scripts/install.sh`，退出码 0、零告警。**M-1 已闭环，AC-7 由实跑通道覆盖，不再依赖人工走查论证。**

## Boundary tests added（边界条件验证）

本任务为 workflow/脚本/文档改动，无产品代码模块，未向 Go/Vitest 套件新增自动化用例（baseline 不变）。边界条件以「独立复现 install 脚本分流逻辑 + 模拟响应体」方式验证：

- BC-2 限流：模拟 GitHub 403 限流响应体（合法 JSON，含 `message` 无 `tag_name`）→ 验证脚本「先判状态码、后解析 JSON」，限流不被当 200 处理。
- BC-3 滚动发布不存在：模拟 404 响应体 → 走「滚动发布尚未生成」分支。
- BC-4 缺当前平台资产：模拟 200 但 assets 只有 windows、当前平台 linux-amd64 → ASSET_URL 解析为空、走缺资产分支。
- BC-6 短 sha 版本号：模拟 rolling release 真实 JSON（资产名 `frp-easy-a1b2c3d-linux-amd64.tar.gz`）→ `frp-easy-.*-${PLATFORM}\.tar\.gz$` 正则中 `.*` 通配短 sha，确定性取到资产。
- 跨 shell 一致性：verify_all 在 PowerShell 与 Git Bash 两 shell 下均 PASS 19。

## Adversarial tests（对抗性测试 / 每条关键点一个证伪假设）

判定依据是「实现是否在该证伪测试下存活」，而非开发者自己的论证是否成立。CI workflow 无法在流水线内真实触发（AC-16/AC-17 为交付后人工项），故对抗点聚焦于可静态/可推演证伪的关键风险。

| # | 假设（"我预期失败当…"） | 复现器（QA 独立编写） | 结果（含工具输出） |
|---|---|---|---|
| A-1 | concurrency.group 不含 `${{ github.ref }}` → main 与 v* 发布落同组互相取消，破坏 BC-12 | `python /tmp/adv_if.py`（NEW，QA 编写，推演两 ref 的并发组） | **存活** — 实际 `group: rolling-release-${{ github.ref }}`；推演输出 `main='rolling-release-refs/heads/main'` vs `v*='rolling-release-refs/tags/v0.1.0'` → 不同组、不互相取消，BC-12 保护成立 |
| A-2 | main 路径与 v* 路径的 `if` 条件不互斥 → 某 ref 同时触发两路径，资产/Release 混乱 | `python /tmp/adv_if.py`（NEW，枚举 4 种 ref 推演 RUN/SKIP step） | **存活** — `ref=refs/heads/main` 仅跑 Move/Purge/Publish rolling 三 step；`ref=refs/tags/v0.1.0` 仅跑 Compute/Create versioned 两 step；枚举 `refs/heads/main`/`refs/tags/v0.1.0`/`refs/tags/vrolling`/`refs/heads/rolling` 无任何 ref 同时满足 `==refs/heads/main` 与 `startsWith(refs/tags/v)` → 「无 — 两条件互斥成立」 |
| A-3 | `Move rolling tag` 与 `Publish rolling release` 都处理 `rolling` tag → 重复处理、tag ref 状态冲突（Gate 条件 2） | 源码走查 + softprops/action-gh-release@v2 行为（04 已实证 v2.6.2 仅在显式 `target_commitish` 时移 ref） | **存活** — `Move rolling tag` step 用 `git tag -f` + force push 独占 tag 处理；`Publish rolling release` step 未传 `target_commitish`，据 v2.6.2 源码实证此时 action 不变更 tag ref。二者职责无重叠，不重复 |
| A-4 | install.sh 与 install.ps1 的 API 端点不一致，或写成网页地址 `releases/tag/rolling`（单数）而非 API `releases/tags/rolling`（复数） | `grep -oE 'https://api[^"]+'` 提取两脚本 URL 逐字比对（NEW） | **存活** — 两脚本均为 `https://api.github.com/repos/Alan-IFT/frp_easy/releases/tags/rolling`（API 复数 tags），逐字一致；DEPLOYMENT.md 网页地址为 `releases/tag/rolling`（单数）—— 单/复数区分正确，全仓 `grep releases/latest` 无残留 |
| A-5 | 改端点后 BC-1/2/3/4 失败分流文案/退出码失效，或把 403 限流响应体当正常 JSON 解析 | `bash /tmp/adv_bc.sh`（NEW，QA 独立复现脚本步骤 3/4 分流逻辑，喂入模拟 403/404/缺资产响应体） | **存活** — 403→`PASS: 识别为 403 限流, 不解析 JSON`；404→`PASS: 走 BC-3 滚动发布尚未生成`；200 缺 linux 资产→`PASS: 走 BC-4 缺资产分支`；rolling tag_name→`VERSION=rolling`（仅进度打印用） |

补充对抗核查（非逐 AC，但属设计风险点）：

- **R-5 自触发循环**：`Move rolling tag` step 推 `rolling` tag，`on.push.tags` glob 为 `v*`，`rolling` 不匹配 `v*` → 不触发版本化路径；且 `GITHUB_TOKEN` 推的 ref GitHub 内建不再触发 workflow。双保险，无循环。**存活。**
- **BC-9 并发竞态**：`concurrency` 组 `rolling-release-${{ github.ref }}` + `cancel-in-progress: true`，多次 push main 落同组、新 run 取消旧 run，最终只发最新 commit 产物；A-1 已证 v* 不被误杀。设计语义成立。

## DESIGN DRIFT 复核

04 声明 1 处 DESIGN DRIFT：BC-7/BC-8 由设计 §4.4 首选方案（`clean_release_attachments: true`）改为 §8 R-3 退化方案（自写 `git tag -f` + `gh release delete-asset`）。

**复核结论：合法，与 05 Code Review 判定一致。**
- 退化路径由设计 §4.4 末段、§8 R-1/R-3、§14 要点 2 明确预留，YAML 示例为设计作者亲笔，落在设计授权的「核对后不支持则走退化方案并在 04 记录」路径内。
- 04 给出源码级实证（softprops/action-gh-release@v2 == v2.6.2，commit `3bb1273`；`src/github.ts` L321-327/L541-579；`grep clean_release_attachments` NOT FOUND）。QA 接受该实证为充分证据：`clean_release_attachments` 选项不存在属可被 grep 机械证伪的事实，无须 QA 重新 clone 复核。
- Gate 条件 2（退化 `git tag -f` 与 action tag 处理不重复）经对抗测试 A-3 验证成立。
- 已按红线 1 在 04 显式标注 `DESIGN DRIFT`，非静默漂移。

## verify_all result

- Total tests: 224 → 224（无变化；本任务无 .go/.ts 改动，未新增/删除自动化测试）
- verify_all 检查项：PASS 19 / WARN 0 / FAIL 0 / SKIP 0
- 基线对照：改动前 PASS 19 → 改动后 PASS 19，WARN/FAIL 不新增 → 满足 AC-15
- New tests added: 0（workflow/脚本/文档改动，无产品代码模块可挂自动化用例）
- Baseline updated: no（test_count 仍 224、passing_count 仍 219；baseline 只升不降，本任务无升项，保持不变）
- 跨 shell：PowerShell（`verify_all.ps1`）与 Git Bash（`verify_all.sh`）均 PASS 19 / WARN 0 / FAIL 0

## Defects found

无。BLOCKER 0 / CRITICAL 0 / MAJOR 0 / MINOR 0。

05 Code Review 的 MINOR M-1（shellcheck 未实跑）已由 QA 在本环境实跑 shellcheck 0.11.0 闭环，不再遗留。

## Stability

- verify_all 在两 shell（PowerShell + Git Bash）下各跑 1 次，结果一致 PASS 19 / WARN 0 / FAIL 0，无 flake。
- 对抗性复现脚本（adv_if.py / adv_bc.sh）为确定性逻辑推演，无随机性，无 flake 风险。
- 本任务无新增自动化测试，无新引入的 flaky 面。

## Verdict

**PASS** — 15 条完成闸门 AC（AC-1..AC-15）全部满足；12 BC 全覆盖；verify_all PASS 19 / WARN 0 / FAIL 0（双 shell）；4 项静态校验（含 shellcheck 实跑）全过；5 项对抗性证伪测试实现全部存活；DESIGN DRIFT 复核合法；0 缺陷。Code Review M-1 已由 shellcheck 实跑闭环。

AC-16/AC-17 为交付后人工验证、非完成闸门 —— 需用户首次 push `main` 触发滚动发布后实测，PM 须在 `07_DELIVERY.md` 显著位置写明该实测前提（Gate 条件 4）。
