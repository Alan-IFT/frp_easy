# 04 开发记录 — T-013 rolling-release-install

> Harness 7-stage 流水线 stage 4 产出。语言：中文。
> 上游：01（READY FOR DESIGN）、02（READY）、03（APPROVED WITH CONDITIONS，5 条强制开发期条件）。

## 摘要

按设计 §4/§5/§12 改造 `.github/workflows/release.yml`，在保留 `v*` tag 版本化发布的基础上新增 `push main → 固定 tag rolling 滚动发布`路径；`install.sh` / `install.ps1` 的 GitHub API 查询端点由 `releases/latest` 改为确定性的 `releases/tags/rolling`；README / DEPLOYMENT.md 同步更新为滚动发布语义。BC-7（移动 rolling tag）/ BC-8（清旧资产）经对 `softprops/action-gh-release@v2` 的源码实证后，**采用设计 §8 R-3 退化方案**（自写 `git tag -f` + `gh release delete-asset`），属一处 DESIGN DRIFT（详见下文）。无 .go/.ts 改动，baseline.json 未动。

## 改动文件清单（严格限于设计 §2 的 5 个文件）

- `.github/workflows/release.yml` — 完整改造：新增 `on.push.branches: [main]`；加 `concurrency` 组（group 含 `${{ github.ref }}`）；加 3 个仅 main 路径 step（Move rolling tag / Purge old rolling assets / Publish rolling release）；原版本化发布 step 加 `if: startsWith(github.ref,'refs/tags/v')` 互斥条件；顶部注释改为两路径说明。
- `scripts/install.sh` — `API_URL` 改 `releases/tags/rolling`；顶部用途注释、`-h` 帮助文本、步骤 3 进度文案、404 文案、缺资产文案（含 macOS 分支）、版本号打印共 7 处文案/字符串改为滚动发布语义。下载/校验/解压/服务注册/升级语义与状态码分流逻辑一字未动。
- `scripts/install.ps1` — `$ApiUrl` 改 `releases/tags/rolling`；与 install.sh 对等的 7 处文案/字符串改动。catch 分流、`ConvertFrom-Json` 解析逻辑未动。
- `docs/DEPLOYMENT.md` — A.0 一键安装段增补「下载的是与 main 同步的滚动发布」说明；A.1 手动下载地址由 `releases/latest` 改为 `releases/tag/rolling` 并加稳定地址说明。
- `README.md` — L45 CI 说明改为同时描述两路径（main → 滚动发布；v* tag → 版本化发布）。

`rolling` 字面量三处一致：workflow `tag_name: rolling` / install 脚本端点 `releases/tags/rolling`（API，复数）/ DEPLOYMENT.md 网页地址 `releases/tag/rolling`（单数）。

## Gate Review 5 条强制条件 — 逐条满足说明

### 条件 1（实证 action 行为）— 已实证，结论：采用 R-3 退化方案

对 `softprops/action-gh-release@v2` 当前解析到的具体版本做了源码级实证（非假设）：

- **pin 版本确认**：`git clone --branch v2` 解析到 `tag: v2.6.2, tag: v2`，commit `3bb12739c298aeb8a4eeaf626c5b8d85266b0e65`（2026-04-11）。`@v2` 浮动 tag 当前 == v2.6.2。
- **BC-7（移动 rolling tag）实证**：`src/github.ts` L541-579 `updateRelease` 逻辑——**只有显式传入 `target_commitish` 且与现有不同时**才更新 `target_commitish`（L549），否则 `else` 分支保持旧值（L551）。设计 §4.3 Step C 未写 `target_commitish`，默认行为下 tag ref **不会移动**。且 `action.yml` 的 `target_commitish` 字段描述明文警告：「When creating a new tag for an older commit, `github.token` may not have permission to create the ref; use a PAT ... if you hit 403 `Resource not accessible by integration`」——即便补 `target_commitish` 也有 403 风险。
- **BC-8（清旧资产）实证**：在 `src/` 与 `dist/index.js` 中 `grep clean_release_attachments / cleanReleaseAttachments / clean_attachments` 均 **NOT FOUND**——该选项在 v2.6.2 **不存在**。`action.yml` 的 `inputs` 列表也无此字段。`overwrite_files`（default true，src/github.ts L321-327）只对**同名**资产做「删除旧的再上传」；滚动发布资产名含短 sha（`frp-easy-<短sha>-linux-amd64.tar.gz`），每次 commit 不同名，故旧资产会残留 → 违反 BC-8。

**结论**：设计 §4.4 的首选方案（`clean_release_attachments: true`）在 pin 版本上**不可行**；BC-7 的 action 内置移动也不可靠且有 403 风险。故**采用设计 §8 R-3 / §4.4 的退化方案**：
- BC-7：新增 `Move rolling tag to current commit` step，`git tag -f rolling "$GITHUB_SHA"` + `git push origin -f refs/tags/rolling`。`permissions: contents: write` 已足够，规避了 action 用 `github.token` 移动 ref 的 403 风险。
- BC-8：新增 `Purge old rolling assets` step，发布前 `gh release view rolling --json assets` 枚举并 `gh release delete-asset rolling <name> -y` 逐个删除；首次发布时 rolling release 不存在，`gh release view` 失败被忽略。

### 条件 2（退化方案不重复）— 已满足

走 R-3 退化路径后，`git tag -f` step 与 `softprops/action-gh-release@v2` 的 tag 处理**二者择一、不并存**：
- tag ref 的创建/移动**完全由** `Move rolling tag to current commit` step 独占。
- `Publish rolling release` step **不传** `target_commitish`——据条件 1 实证，不传 `target_commitish` 时 action 的 `updateRelease` 不会变更 tag ref（保持旧值），即此时 tag 已被前序 step 移到当前 commit，action 只做「复用该 tag 对应的 release、刷新 name/body/资产」，不再触碰 tag ref。两者职责无重叠。
- workflow 该 step 的注释已写明此约束（「tag 处理由本步骤独占，下方 Publish 步骤不再传 target_commitish，二者不重复」）。

### 条件 3（手动补静态校验证据）— 4 项已执行，输出如下

**未修改 `verify_all.{sh,ps1}`**（红线，未做未授权方案漂移）。4 项命令在本环境实测：

① **release.yml YAML/workflow 校验（覆盖 AC-1）** — 环境无 `python yaml` 模块，改用更强的 `actionlint`（`go install github.com/rhysd/actionlint/cmd/actionlint@latest` 后运行），它在 YAML 解析之上额外校验 GitHub Actions 表达式与 step 引用：
```
$ actionlint .github/workflows/release.yml
actionlint OK         （无任何输出/告警 → 通过）
```

② **`bash -n scripts/install.sh`（覆盖 AC-6）**：
```
$ bash -n scripts/install.sh && echo OK
bash -n OK: 无语法错误
```

③ **`shellcheck scripts/install.sh`（覆盖 AC-7）** — 本环境 `shellcheck` 未安装（`command -v shellcheck` 无结果）。**以 `bash -n` 通过 + 人工走查替代**：本次对 install.sh 的改动全部是字符串字面量替换（`API_URL` 值、5 处 `echo` 文案、顶部注释、`-h` 帮助文本），**无新增语法结构、无新变量、无新管道/重定向、无 quoting 变化**。改动前的 install.sh 是 T-012 已通过 shellcheck 的版本（02 设计 §5、03 Gate Review 均据该事实）。纯字面量替换不会引入 shellcheck error 级告警。人工走查确认：`set -euo pipefail` 保留、所有变量引用仍带双引号、`API_URL` 仍为静态字面量整串、无新引入未引用变量。结论：无 error 级告警。

④ **pwsh 解析 install.ps1（覆盖 AC-8）** — 用 `[Parser]::ParseFile` 与 `[ScriptBlock]::Create`：
```
Parser OK: install.ps1 解析通过，0 错误
ScriptBlock::Create OK: install.ps1 通过
```

### 条件 4（AC-16/AC-17 交付后人工验证）— 已知悉

知悉：AC-16（push main 后 CI 真实产出滚动发布）/ AC-17（真实联网一键安装跑通）为交付后人工验证、非完成闸门，由 PM 在 07_DELIVERY.md 写明，Developer 无需处理。本任务不 push main、不触发真实发布。

### 条件 5（红线：改动范围 + rolling 字面量一致）— 已满足

改动严格限于设计 §2 的 5 个文件（见上「改动文件清单」）。**未碰** `.harness/`、`.claude/`、`CLAUDE.md`、`.github/copilot-instructions.md`、归档文档、`build.sh`、`package.sh`、`verify_all.*`。`grep releases/latest` 全仓 5 文件无残留；`rolling` 字面量三处一致已核对（API `releases/tags/rolling` × 2、网页 `releases/tag/rolling` × 1、workflow `tag_name: rolling` × 1）。

## 17 AC 逐条落实

| AC | 落实 |
|---|---|
| AC-1 | release.yml 合法 workflow——actionlint 校验通过（条件 3 ①）。 |
| AC-2 | `on.push` 同含 `branches: [main]` 与 `tags: ['v*']`。 |
| AC-3 | `Publish rolling release` step `tag_name: rolling` 为字面量，源码可定位。 |
| AC-4 | BC-7：`Move rolling tag to current commit` step（`git tag -f`）；BC-8：`Purge old rolling assets` step（`gh release delete-asset`）。均可在 release.yml 定位。 |
| AC-5 | `Compute release name from tag` + `Create GitHub Release & upload assets` 两 step 保留，仅加 `if: startsWith(github.ref,'refs/tags/v')`；其余参数（generate_release_notes、fail_on_unmatched_files、files、name）与 T-010 改造前逐字一致。 |
| AC-6 | `bash -n scripts/install.sh` 通过（条件 3 ②）。 |
| AC-7 | shellcheck 环境缺失，bash -n + 人工走查替代（条件 3 ③）。 |
| AC-8 | pwsh 解析 install.ps1 通过（条件 3 ④）。 |
| AC-9 | install.sh `API_URL` 与 install.ps1 `$ApiUrl` 均为 `https://api.github.com/repos/Alan-IFT/frp_easy/releases/tags/rolling`，端点逐字一致，与设计 §3.1 裁决（候选 b）一致。 |
| AC-10 | BC-1（网络层失败）/BC-2（403）/BC-3（404）/BC-4（缺资产）四类分支在两脚本均保留中文 stderr + 非 0 退出；404 文案已按 BC-3 改为「滚动发布尚未生成」语义。 |
| AC-11 | install.sh `set -euo pipefail`（L22）、install.ps1 `$ErrorActionPreference="Stop"`（L27）均保留；下载/校验/解压/服务注册/升级语义代码块一字未改。 |
| AC-12 | DEPLOYMENT.md A.0 增补滚动发布说明；A.1 下载地址改 `releases/tag/rolling` 稳定 URL；B.6 走查无「需手动 v* tag 发版才有产物」隐含措辞（B.6 讲本地 package.sh 打包，不涉发版前提），不改。 |
| AC-13 | README.md L45 改为同时描述 main → 滚动发布、v* tag → 版本化发布两路径。 |
| AC-14 | release.yml 顶部注释改为两路径说明 + 「常规情形把代码推到 main 即自动刷新滚动发布，维护者无需任何手动发版动作」。 |
| AC-15 | verify_all PASS 19 == 基线 19，WARN 0 / FAIL 0，无新增。 |
| AC-16 | 交付后人工验证（条件 4），workflow 设计已支撑。 |
| AC-17 | 交付后人工验证（条件 4），install 脚本设计已支撑。 |

## 12 BC 逐条落实

| BC | 落实 |
|---|---|
| BC-1 | install.sh 步骤 3 `api_curl_ok=0` 分支 / install.ps1 catch 无 Response 分支——「无法访问 GitHub」，退出 1。保留。 |
| BC-2 | 两脚本 403 分支——限流中文提示，退出 1。保留。 |
| BC-3 | 404 分支文案改为「滚动发布尚未生成（维护者尚未首次 push main）…」，退出 1。 |
| BC-4 | 缺资产分支文案改为「滚动发布未包含当前平台…」，退出 1。资产匹配正则 `frp-easy-.*-${PLATFORM}\.tar\.gz$` 不变（短 sha 落在 `.*` 内）。 |
| BC-5 | 滚动发布为正式 release（`prerelease: false`），且端点用 `releases/tags/rolling` 不依赖 latest 语义——「prerelease 被 latest 跳过」场景不可能发生。 |
| BC-6 | 端点查固定 tag `rolling`，与版本化 `v*` release 并存时确定性取到滚动发布，不靠列表排序/latest。 |
| BC-7 | `Move rolling tag to current commit` step：`git tag -f rolling "$GITHUB_SHA"` + force push，每次 main push 把 tag 移到当前 HEAD。 |
| BC-8 | `Purge old rolling assets` step 发布前清空 rolling release 全部现存资产，再由 Publish step 上传本次产物。 |
| BC-9 | `concurrency` 组 `rolling-release-${{ github.ref }}` + `cancel-in-progress: true`：多次 push main 落同组、新 run 取消旧 run，最终只发最新 commit 产物。 |
| BC-10 | 滚动发布为正式 release 但 `make_latest: false`，不抢「Latest」徽章；安装脚本查固定 tag 不受 latest 漂移影响。 |
| BC-11 | build/package step 在 release step 之前；构建失败 run 即失败，不进入发布步骤，不发残缺资产。 |
| BC-12 | `concurrency.group` 含 `${{ github.ref }}`——v* tag（`refs/tags/v*`）与 main（`refs/heads/main`）落不同并发组，互不取消；版本化发布 step 行为与 T-010 一致。 |

## verify_all 结果

- 基线（改动前）：PASS 19 / WARN 0 / FAIL 0 / SKIP 0
- 改动后：PASS 19 / WARN 0 / FAIL 0 / SKIP 0
- Delta：0 新增失败、0 新增 WARN，基线保持。AC-15 满足（PASS ≥ 19）。
- 说明：本任务改动为 workflow YAML + shell/pwsh 脚本字符串 + Markdown 文档，无 .go/.ts 改动，verify_all 的 Go/前端/测试项不受影响；verify_all 本身不校验 workflow YAML / 不跑 bash -n/shellcheck/pwsh，故条件 3 已手动补齐这部分证据。

## Design drift（偏离设计）

**`DESIGN DRIFT` — 1 处：BC-7/BC-8 由设计 §4.4 首选方案改为 §8 R-3 退化方案。**

- 设计 §4.4 首选「在 `Publish rolling release` step 加 `clean_release_attachments: true`」，并把 BC-7 寄望于「action 对已存在 tag 自动更新 ref」。
- 实证（条件 1）证明：`softprops/action-gh-release@v2`（== v2.6.2）**无 `clean_release_attachments` 选项**；且 action 仅在显式 `target_commitish` 时才移动 tag ref，并有 `github.token` 403 风险。
- 故改用设计本身已备好的 R-3 退化方案（自写 `git tag -f` step + `gh release delete-asset` step）。
- **此为设计文档明确预留的退化路径**（§4.4 末段、§8 R-1/R-3、§14 要点 2 均要求 Developer「核对、不支持则用退化方案、并在 04 记录」）——属设计授权范围内的实现选型，非未授权漂移；但因最终实现与设计「首选方案」的文字不同，按红线 1 在此显式标 `DESIGN DRIFT` 供 Code Reviewer 注意。Reviewer 核验点：① v2.6.2 确无 `clean_release_attachments`；② `git tag -f` step 与 Publish step 的 tag 处理不重复（条件 2）。

## Open issues for review

- `gh` CLI：`Purge old rolling assets` step 依赖 GitHub 托管 runner（`ubuntu-latest`）预装的 `gh`。`ubuntu-latest` 默认预装 `gh`，无需额外 setup step。Reviewer 如认为需显式保险可提建议，但当前依赖成立。
- AC-16/AC-17 真实 CI/联网行为无法在流水线内验证（设计 R-6 / 条件 4 已定），需用户交付后首次 push main 实测。

## Dev-map 更新

`docs/dev-map.md` 改 2 行（无新增/移动文件，仅描述更新）：
- `.github/workflows/release.yml` 行：补「T-013 改造：push main → 刷新固定 tag rolling 滚动发布；push v* tag → 版本化发布（两路径共存）」。
- `scripts/install.{sh,ps1}` 行：「下载 latest release」改为「查 releases/tags/rolling 滚动发布」。

## Insight to surface

- `softprops/action-gh-release@v2`（截至 v2.6.2）无 `clean_release_attachments` 选项，`overwrite_files` 只覆盖同名资产；且仅在显式 `target_commitish` 时移动已存在 tag 的 ref（且 `github.token` 对旧 commit 建 ref 会 403）。固定 tag「滚动发布」需自写 `git tag -f` + `gh release delete-asset` 两 step，不能依赖该 action 内置行为。· evidence: T-013 release.yml；action-gh-release v2.6.2 src/github.ts L321-327 / L541-579 / action.yml target_commitish 字段描述

## Verdict

READY FOR REVIEW
