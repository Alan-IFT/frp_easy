# 03 Gate Review — T-013 rolling-release-install

> Harness 流水线 stage 3 产出。Gate Reviewer 独立验证（已逐行核对 verify_all.sh、build.sh、package.sh、install.sh、install.ps1）。
> 上游：01（READY FOR DESIGN）、02（READY）。模式：full。

## 1. 八维审计

| # | 维度 | 结论 |
|---|---|---|
| 1 | 需求完整性 | PASS — 14 In-scope / 12 BC / 17 AC 可定位；BC-5/BC-10 把 prerelease 与端点自洽性点为硬约束 |
| 2 | 设计完整性 | PASS — §4 YAML 结构级改造、§5 脚本逐行改动、§11 AC 覆盖映射，OQ-1/2/3 明确裁决 |
| 3 | 复用正确性 | PASS — build.sh L19、package.sh L66-69/L220、install 脚本行号经实文件核对属实 |
| 4 | 风险覆盖 | WARN — 漏了 verify_all 不校验 workflow YAML（F-1） |
| 5 | 迁移安全 | PASS — 无 schema/无持久数据；git revert + 手删 rolling tag 可回滚 |
| 6 | 边界处理 | PASS — BC-1..12 全覆盖；concurrency 组含 github.ref 避免误杀 v* |
| 7 | 测试可行性 | WARN — AC-1/6/7/8 标"静态/自动"但 verify_all 无对应步骤（F-1/F-2） |
| 8 | 范围边界 | PASS — 10 项边界明确 |

6 PASS / 2 WARN / 0 FAIL。

## 2. Findings

- **F-1（WARN，最重要）**：`verify_all.sh` 经逐行核对**完全不校验 `.github/workflows/*.yml`**，不跑 actionlint / bash -n / shellcheck / pwsh 解析。AC-1/AC-6/AC-7/AC-8 标"静态/自动"名实不符；AC-15（verify_all PASS≥19）通过 ≠ workflow 正确。→ 条件 3。
- **F-2（WARN）**：AC-6/7/8（bash -n / shellcheck / pwsh 解析）同样无 verify_all 自动通道，需手动跑。→ 条件 3。
- **F-3（WARN）**：设计对 `softprops/action-gh-release@v2` 的 BC-7（移动 rolling tag）、BC-8（清旧资产）行为是断言而非实证。但备有确定可行的退化方案（gh release delete-asset / git tag -f），不阻塞。→ 条件 1。
- **F-4（WARN）**：R-3 退化的 `git tag -f` step 与 action 自身 tag 处理可能语义重复，需 Developer 明确二者择一。→ 条件 2。
- **F-5（WARN）**：Step C 未写 target_commitish；首次创建 tag 落在触发 commit 正确，后续移动见 F-3。
- **F-6（INFO）**：insight-index 三条相关条目与设计一致，无冲突。

## 3. Verdict

**APPROVED WITH CONDITIONS** —— 6 PASS / 2 WARN / 0 FAIL，无 FAIL、无需回退上游。开发期必须满足以下 5 条条件（Code Reviewer / QA 逐条核验）：

1. **（F-3/F-1）** Developer 不得"假设 action 行为成立"就交付。必须对所 pin 版本的 BC-7（tag 移动）、BC-8（旧资产清理）行为做实证（查 action.yml/文档），或直接采用设计 §4.4 / §8 R-3 退化方案。最终选型写入 04_DEVELOPMENT.md。
2. **（F-4）** 若走 R-3 退化路径，须明确 `git tag -f` step 与 action 的 tag 处理不重复 —— 二者择一，不并存。
3. **（F-1/F-2）** 因 verify_all 不校验 workflow YAML / 不跑 bash -n/shellcheck/pwsh，Developer 必须在 04_DEVELOPMENT.md 手动执行并贴出证据：① release.yml 的 YAML 解析（actionlint 或 python yaml.safe_load）覆盖 AC-1；② `bash -n scripts/install.sh` 覆盖 AC-6；③ `shellcheck scripts/install.sh` 覆盖 AC-7；④ pwsh 解析 install.ps1 覆盖 AC-8。**不得为补检查而修改 verify_all.sh**（属未授权方案漂移）。
4. **（Q5）** PM 须在 07_DELIVERY.md 显著位置写明：AC-16/AC-17 为交付后人工验证、未在流水线内验证；workflow 真实正确性需用户首次 push main 后确认。
5. **（红线）** 改动严格限于设计 §2 的 5 个文件（release.yml、install.sh、install.ps1、DEPLOYMENT.md、README.md）。不碰 `.harness/`、`.claude/`、`CLAUDE.md`、`.github/copilot-instructions.md`、归档文档、build.sh、package.sh、verify_all.*。`rolling` 字面量三处一致（workflow `tag_name: rolling`、install 脚本端点 `releases/tags/rolling`、DEPLOYMENT.md 网页地址 `releases/tag/rolling`）。
