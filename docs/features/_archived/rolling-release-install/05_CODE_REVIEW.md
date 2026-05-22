# 05 Code Review — T-013 rolling-release-install

> Harness 流水线 stage 5 产出。Code Reviewer 独立核对 git 工作区实际改动（未 commit）。
> 上游：01/02/03（APPROVED WITH CONDITIONS）/04（含 1 处声明的 DESIGN DRIFT）。

## Verdict：APPROVED（BLOCKER 0 / MAJOR 0 / MINOR 1 / NIT 2）

15 条完成闸门 AC（AC-1..AC-15）全部满足；AC-16/AC-17 为交付后人工验证、非闸门。12 BC 全部覆盖。唯一 DESIGN DRIFT 经核验合法。红线全部遵守。

## 评审文件
`.github/workflows/release.yml`、`scripts/install.sh`、`scripts/install.ps1`、`docs/DEPLOYMENT.md`、`README.md`、`docs/dev-map.md`（设计已声明的顺带更新）。

## DESIGN DRIFT 合法性核查
Developer 把 BC-7/BC-8 从设计 §4.4 首选方案（`clean_release_attachments`）改为 §8 R-3 退化方案（`git tag -f` + `gh release delete-asset`）。**合法**：
- (a) 退化路径由设计 §4.4 末段 + §8 R-1/R-3 + §14 要点 2 明确预留，YAML 示例是设计作者亲笔。
- (b) 实现正确，基于 04 给出的源码级实证：softprops/action-gh-release@v2.6.2，`src/github.ts` L321-327/L541-579，`grep clean_release_attachments` NOT FOUND。
- 已按红线 1 显式声明。不因"drift"标签本身判 CHANGES REQUIRED。

## release.yml 改造核查
- `on:` 双触发（push.branches: [main] + push.tags: ['v*']）✅
- `concurrency.group` 含 `${{ github.ref }}` ✅ — main 与 v* 落不同并发组，不互相取消（保护 BC-12）
- main/tag 路径 `if` 互斥且穷尽 ✅
- `Move rolling tag` step 独占 tag 处理；`Publish rolling release` 不传 `target_commitish`，二者不重复（Gate 条件 2 满足）✅
- `Purge old rolling assets` step 首次无 release 时 `gh release view` 失败被 `if` 吞掉，不报错 ✅

## install 脚本核查
- API 端点 `releases/latest` → `releases/tags/rolling`，install.sh/install.ps1 两脚本逐字一致 ✅
- 不依赖 jq 的 grep/sed/ConvertFrom-Json 解析继续成立（两端点返回结构同为单 release 对象）✅
- BC-1/2/3/4 失败分流保留；404 文案改为"滚动发布尚未生成" ✅

## `rolling` 字面量三处一致
- workflow `tag_name: rolling` ✅
- install 脚本 API `releases/tags/rolling`（复数）✅
- DEPLOYMENT.md 网页地址 `releases/tag/rolling`（单数）✅ — 单/复数区分正确

## 红线核查
改动限于 5 个授权文件 + dev-map.md（设计 §13 声明）；未碰 `.harness/`、`.claude/`、`CLAUDE.md`、`copilot-instructions.md`、`build.sh`、`package.sh`、`verify_all.*`、归档文档；checkout 未加 `fetch-depth: 0`。

## Gate 5 条条件核查
1. 实证 action 行为 ✅（04 给出 v2.6.2 commit hash + 源码行号）
2. 退化方案 tag 处理不重复 ✅
3. 手动补静态校验 ⚠️ 部分 — actionlint(AC-1)✅ / `bash -n`(AC-6)✅ / pwsh 解析(AC-8)✅ / shellcheck(AC-7) 环境缺失，以 bash -n + 人工走查替代 → MINOR-1
4. AC-16/17 交付后人工验证 — PM 在 07 写明
5. 改动范围 + rolling 三处一致 ✅

## Findings

### BLOCKER / MAJOR
无。

### MINOR
- **M-1**：Gate 条件 3③ 要求贴出 `shellcheck scripts/install.sh` 证据覆盖 AC-7，但开发环境未装 shellcheck，Developer 以"`bash -n` 通过 + 人工走查（本次改动纯字面量替换、无新语法、改动前版本已过 shellcheck）"替代。论证充分合理，不阻塞。**建议 QA 在具备 shellcheck 的环境补跑一次闭环 AC-7；若 QA 环境同样缺失，沿用人工走查论证并由 PM 在 07 记录。**

### NIT（无需改）
- N-1 `Purge old rolling assets` 的 `[ -n "$a" ]` 空值保护属冗余防御。
- N-2 两个新 step 未设 step 级 `timeout-minutes`，job 级 15 分钟已足够。

## 给 PM / QA 的说明
- M-1 是唯一遗留项，QA 视环境补 shellcheck 证据。
- QA 复核 AC-15（verify_all PASS ≥ 19）。
- AC-16/AC-17 真实 CI/联网正确性需用户交付后首次 push main 实测，PM 在 07 显著位置写明。
