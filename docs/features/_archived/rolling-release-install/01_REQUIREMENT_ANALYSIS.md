# 01 — 需求分析：T-013 rolling-release-install

> Stage 1 of 7-stage `/harness` 流水线。
> 任务 ID：**T-013** · Slug：`rolling-release-install` · 模式：full · 语言：中文
> 上游：用户原始诉求 + PM 已完成的 5 项预决策（见 `INPUT.md`）。
> 本文档为下游 Solution Architect 的唯一输入。

---

## 1. 目标

把 frp_easy 的发版从"维护者手动打 `v*` tag 触发版本化 GitHub Release"改造为"每次 push 到 `main` 分支自动编译并刷新一个固定的滚动发布（rolling release）"，使 `install.sh` / `install.ps1` 永远能下到与 `main` 同步的预编译二进制，维护者零手动发版动作、终端用户零工具链。

---

## 2. In-scope 行为（本期必做，可验证）

> 全部产出（脚本注释、报错、help、文档、workflow 注释）使用中文。

### A 块 — 改造 `.github/workflows/release.yml`

1. `release.yml` 的 `on:` 触发条件在保留原 `push: tags: ['v*']` 的基础上，新增 `push: branches: [main]` 触发。两条触发路径共存，互不破坏。
2. 由 `push: branches: [main]` 触发时，workflow 复用现有 `scripts/build.sh --all` 与 `scripts/package.sh --windows --skip-build` 构建并打包 Linux + Windows 发布产物（与 `v*` 路径同一构建逻辑，不复刻）。
3. 由 `push: branches: [main]` 触发时，workflow 创建或更新一个**固定标签**的 GitHub Release（滚动发布）。该固定标签在 workflow 中以字面量定义（标签名由 Architect 在 `02_SOLUTION_DESIGN.md` 选定，见开放问题 OQ-2）。
4. 滚动发布每次刷新时，**替换**其上一次的全部资产（`*.tar.gz` + `*.zip`），使滚动发布的资产始终对应最新一次 `main` 构建，不残留旧资产。
5. 当固定标签已存在且指向旧 commit 时，workflow 把该标签更新为指向本次触发的 `main` HEAD commit（不因标签已存在而失败）。
6. 由 `push: tags: ['v*']` 触发时，workflow 行为与改造前一致：创建以去 `v` 前缀版本号命名的版本化 Release，不触碰滚动发布。
7. workflow 顶部注释更新：说明两种触发路径（`main` push → 滚动发布、`v*` tag → 版本化发布）各自的行为与产物去向。

### B 块 — 同步 `install.sh` / `install.ps1` 的 release 查询逻辑

8. `install.sh` 与 `install.ps1` 的 GitHub API 查询端点从 `releases/latest` 调整为与滚动发布方案匹配的端点（`releases/tags/<固定标签>` 或 `releases/latest`，由 Architect 据 OQ-1 裁决结果定稿）。两脚本所用端点保持一致。
9. 调整后的查询逻辑保留 T-012 既有的失败分流能力：网络层失败、HTTP 403 限流、HTTP 404（目标 release 不存在）、缺当前平台资产，四类失败各自给出中文 stderr 报错并以非 0 退出码退出（详见第 4 节边界条件），不静默失败。
10. 调整后，`install.sh` / `install.ps1` 仍按"匹配 `frp-easy-*-<os>-<arch>.{tar.gz,zip}` 文件名"解析资产下载链接；除查询端点 URL 与必要的 prerelease 处理外，下载/校验/解压/服务注册/升级语义等步骤不变。
11. 两脚本顶部注释中描述 API 查询行为的文字同步更新为滚动发布语义。

### C 块 — 文档更新

12. `docs/DEPLOYMENT.md` 中与发布/安装相关的说明（A.1 手动下载地址 `releases/latest`、B.6 打包说明、路径 A 一键安装段）更新为与滚动发布方案一致：明确"一键安装下载的是与 `main` 同步的滚动发布预编译包"，并把手动下载地址调整为滚动发布对应的稳定 URL。
13. `README.md` 中涉及发布的说明（当前 L45「GitHub Actions CI（T-010）：push `v*` tag 自动构建并上传 GitHub Releases 资产」）更新为同时描述 `main` push → 滚动发布、`v*` tag → 版本化发布两条路径。
14. `release.yml` 顶部的"发版步骤（人工）"注释更新：说明常规情形下维护者无需任何手动发版动作（push `main` 即自动刷新滚动发布），`v*` tag 为可选的正式版本化发布手段。

---

## 3. Out-of-scope（本期明确不做）

| 不做项 | 理由 |
|---|---|
| 本任务自动 push `main` / 自动打 tag / 自动触发真实发布 | PM 决策 5：交付后由用户自行 push `main` 触发首次滚动发布。本流水线不代行对外发布动作。 |
| 改写 `scripts/build.sh` / `scripts/package.sh` 的构建/打包逻辑 | 滚动发布复用其既有契约，只在 workflow 编排层加一条触发路径与发布逻辑。 |
| 改写 `install-service.{sh,ps1}` / `uninstall-service.{sh,ps1}` | 不涉及；`install.{sh,ps1}` 仍按既有契约调用它们。 |
| ARM / RISC-V 多架构、macOS 专用包 | 与 T-012 一致，发布产物仍仅 amd64（linux + windows）；macOS 走源码或降级路径。 |
| 下载包的 GPG / 数字签名校验 | 与 T-012 一致，本期仅校验非空、可解压。 |
| 滚动发布的资产保留多份历史 / 版本回滚机制 | 滚动发布按定义只持有"最新一次 main 构建"，历史版本由 `v*` tag 版本化发布承担，本期不做。 |
| 把滚动发布或一键安装接入 `verify_all` 做真实联网/真实 CI 测试 | 真实 CI 运行依赖 push `main` 与外网，不适合进 `verify_all` 闸门；`verify_all` 仅做静态检查（见第 5 节 AC 分类）。 |
| 引入 GitHub token 认证以提高 API 限流额度 | 与 T-012 一致，安装脚本仍为未认证匿名请求；限流走友好降级。 |
| 改动 PR / 非 `main` 分支的 CI 行为 | 本任务只新增 `main` 分支 push 触发，不在 PR 或其他分支上触发发布。 |

---

## 4. 边界条件与失败场景

> BC-1…BC-6 为 `install.sh` / `install.ps1` 侧；BC-7…BC-12 为 `release.yml` workflow 侧。
> 安装脚本侧所有报错为中文、写 stderr、退出码非 0（与 T-012 一致）。

| 编号 | 场景 | 必须的行为 |
|---|---|---|
| **BC-1** | 无网络 / DNS 失败 / GitHub API 不可达 | 安装脚本检测到 API 拉取失败，打印"无法访问 GitHub（请检查网络或代理）"，退出码非 0。 |
| **BC-2** | GitHub API 返回 403 限流 | 安装脚本识别为限流，打印中文限流提示并指向手动下载备选，退出码非 0；不把限流响应当正常 JSON 解析。 |
| **BC-3** | 目标 release 不存在（API 返回 404） | 滚动发布尚未首次产生时（用户尚未 push `main`），安装脚本查询固定标签或 latest 得 404，打印中文报错（"滚动发布尚未生成，请等待维护者首次 push 或改用源码构建"），退出码非 0。 |
| **BC-4** | 目标 release 存在但缺当前平台资产 | 打印"该发布未包含当前平台（`<os>-<arch>`）的发布包"，退出码非 0。 |
| **BC-5** | 滚动发布被标记为 prerelease，而安装脚本查询 `releases/latest` 端点 | `releases/latest` 端点只返回非 prerelease、非 draft 的最新 release——此组合会导致安装脚本取不到滚动发布（404 或取到错误的旧版本化 release）。Architect 必须在 `02_SOLUTION_DESIGN.md` 中令"滚动发布是否 prerelease"与"安装脚本查询端点"两个决策**自洽**，使本场景不发生（见 OQ-1）。 |
| **BC-6** | 同时存在版本化 `v*` Release 与滚动发布，安装脚本需取到正确目标 | 安装脚本必须确定性地取到方案选定的目标 release（滚动发布或最新版本化发布），不因两者并存而取错。 |
| **BC-7** | 固定标签已存在且指向旧 commit | workflow 把固定标签强制更新为指向本次 `main` HEAD，并刷新 Release 内容与资产；不因"标签已存在"而构建失败或创建重复 Release。 |
| **BC-8** | 滚动发布的旧资产仍挂在 Release 上 | 本次刷新时旧 `*.tar.gz` / `*.zip` 资产被清除/覆盖，刷新后 Release 仅含本次构建产物，不混入上一次的过时资产。 |
| **BC-9** | 短时间内连续多次 push `main`（并发 / 排队的 workflow run） | 多个 workflow run 各自刷新同一滚动发布；不得因并发产生重复 Release 或永久损坏的标签状态。最终一致性可接受（最后完成的 run 决定最终资产）；竞态下的具体行为由 Architect 在设计中说明（见 OQ-3）。 |
| **BC-10** | `releases/latest` 与 prerelease 标记的交互 | 若滚动发布设为 prerelease：`releases/latest` 会跳过它、返回更早的版本化 Release（或 404）。若滚动发布设为正式 release：它会成为 `releases/latest`，而后续真正的 `v*` 版本化发布若晚于它则又顶替它。此交互必须在设计中被显式处理，安装脚本的查询端点选择须与之一致（关联 BC-5 / OQ-1）。 |
| **BC-11** | workflow 构建/打包步骤失败 | workflow run 以失败状态结束；失败时不发布残缺资产、不把滚动发布刷新成半成品（构建成功后才发布，沿用现有 `softprops/action-gh-release` 的步骤次序）。 |
| **BC-12** | 用户既有的旧版本化 `v*` 发布流程 | 改造后 push `v*` tag 仍产出版本化 Release，行为与 T-010 一致；本任务不破坏该既有能力。 |

---

## 5. 验收标准（AC）

> 验收方式分两类并明确标注：
> **【静态/自动】** 可在 `verify_all` 环境或本地无网络下机械验证；
> **【集成/人工 — 交付后验证，非完成闸门】** 需真实 push `main` 触发 CI 或真实联网，由用户在交付后执行，不作为任务完成闸门。

| ID | 描述 | 验证方式 |
|---|---|---|
| **AC-1** | `.github/workflows/release.yml` 为合法 YAML（可被 YAML 解析器/`actionlint` 解析无语法错误） | 【静态/自动】 |
| **AC-2** | `release.yml` 的 `on:` 同时含 `push.branches`（含 `main`）与 `push.tags`（含 `v*`）两条触发路径 | 【静态/自动】grep / YAML 走查 |
| **AC-3** | `release.yml` 中存在"由 `main` push 触发时创建/更新固定标签滚动发布"的步骤，且固定标签名为字面量、可在源码中定位 | 【静态/自动】源码走查 + grep |
| **AC-4** | `release.yml` 中"标签已存在指向旧 commit 时更新标签"（BC-7）与"旧资产被替换"（BC-8）的逻辑可在源码中定位 | 【静态/自动】Code Review 走查 |
| **AC-5** | `release.yml` 中 `v*` tag 触发路径产出版本化 Release 的逻辑保留，行为与 T-010 改造前一致 | 【静态/自动】源码对比走查 |
| **AC-6** | `bash -n scripts/install.sh` 通过（无语法错误） | 【静态/自动】 |
| **AC-7** | `scripts/install.sh` 通过 `shellcheck`，无 error 级告警 | 【静态/自动】 |
| **AC-8** | `pwsh -NoProfile -Command "$null = [ScriptBlock]::Create((Get-Content -Raw scripts/install.ps1))"` 解析通过 | 【静态/自动】 |
| **AC-9** | `install.sh` 与 `install.ps1` 的 GitHub API 查询端点已调整为滚动发布方案选定的端点，且两脚本端点一致、与 `02_SOLUTION_DESIGN.md` 裁决一致 | 【静态/自动】grep + 源码走查 |
| **AC-10** | BC-1…BC-4 四类失败分支在 `install.sh` / `install.ps1` 源码中均可定位到对应中文 stderr 报错与非 0 退出，文案语义与第 4 节一致 | 【静态/自动】Code Review 逐条走查 |
| **AC-11** | `install.sh` 仍含 `set -euo pipefail`；`install.ps1` 仍含 `$ErrorActionPreference = "Stop"`；下载/校验/解压/服务注册/升级语义步骤未被破坏 | 【静态/自动】grep + 源码对比 |
| **AC-12** | `docs/DEPLOYMENT.md` 中发布/安装相关说明已更新为滚动发布语义，手动下载地址为滚动发布对应稳定 URL，不再隐含"需手动 v* tag 发版才有产物" | 【静态/自动】目视 + grep |
| **AC-13** | `README.md` 涉及发布的说明已同时描述 `main` push → 滚动发布、`v*` tag → 版本化发布两条路径 | 【静态/自动】目视 + grep |
| **AC-14** | `release.yml` 顶部注释已更新，描述两种触发路径与产物去向，并说明常规无需手动发版 | 【静态/自动】目视 |
| **AC-15** | 引入本任务全部改动后 `scripts/verify_all` 输出 PASS 数不低于当前基线（当前 19），WARN/FAIL 不新增 | 【静态/自动】执行 `scripts/verify_all` |
| **AC-16** | 用户 push `main` 后，GitHub Actions 真实跑通并产出/刷新滚动发布，Release 资产含 `frp-easy-*-linux-amd64.tar.gz` 与 `frp-easy-*-windows-amd64.zip` | 【集成/人工 — 交付后验证，非完成闸门】依赖真实 push `main`，由用户执行。 |
| **AC-17** | 滚动发布产生后，在联网标准 Linux 执行 `curl -fsSL <install.sh raw-url> \| sudo bash` 能下到预编译包并完成安装；Windows 侧 `irm <install.ps1 raw-url> \| iex` 同理 | 【集成/人工 — 交付后验证，非完成闸门】依赖 AC-16 已发生。 |

---

## 6. 非功能性需求（NFR）

- **NFR-1 维护者零手动发版** — 常规情形下维护者把代码推到 `main` 即触发滚动发布刷新，无需打 tag、无需进 GitHub UI 操作。
- **NFR-2 用户零工具链** — 终端用户的安装路径不要求预装 Go / Node / npm / git；一键安装下载的始终是预编译二进制。
- **NFR-3 确定性** — 安装脚本对"要下载哪个 release"的解析必须确定（不依赖 release 列表的隐式排序），在滚动发布与版本化发布并存时不取错（BC-6）。
- **NFR-4 CI 成本** — 新增的 `main` push 触发会让每次 `main` 提交都跑一次约 2–3 分钟的构建。本期接受此成本（公共仓库 Actions 免费额度内）；不在 PR / 其他分支上触发，避免额外消耗。
- **NFR-5 兼容性** — 改造后 `v*` tag 版本化发布能力完整保留，给将来正式发版留口子；安装脚本除 API 端点外的行为对用户无感。
- **NFR-6 语言** — 所有产出（workflow 注释、脚本注释/报错/help、文档）使用中文。

---

## 7. 相关任务

| 任务 | 关系 | 关键文件 |
|---|---|---|
| **T-012 `one-click-install-and-mit-license`** | 本任务直接前序。创建了 `scripts/install.sh` / `install.ps1`，其 GitHub API 查询端点 `releases/latest` 正是本任务要调整的对象。需求文档：`docs/features/_archived/one-click-install-and-mit-license/01_REQUIREMENT_ANALYSIS.md`。其 8.1 风险段已预见"仓库当前无任何已发布 release，install 取 latest 会 404"——本任务正是为消除该前置缺失而生。 |
| **T-010 `deploy-polish-and-ci`** | 创建了 `.github/workflows/release.yml`（push `v*` tag 触发版本化发布）。本任务在该文件上叠加 `main` 分支触发与滚动发布逻辑，复用其 `build.sh` / `package.sh` 编排与资产命名约定。文档：`docs/features/_archived/deploy-polish-and-ci/`。 |
| **T-008 `deploy-kit`** | 建立 `install-service.{sh,ps1}` 与 `docs/DEPLOYMENT.md` 三路径结构；本任务不改动服务脚本，仅更新 DEPLOYMENT.md 发布相关说明。 |
| **T-002 `zero-config-quickstart`** | frp 子二进制的 GitHub Releases 自动下载机制；与本任务无关、不改动。 |

相关 insight（`.harness/insight-index.md`）：
- `GitHub API 未认证请求的限流响应（HTTP 403）响应体是合法 JSON；查询 release 必须"先判 HTTP 状态码、后解析 JSON"，且查询步骤不能用 curl -f` —— 本任务调整查询端点时必须保留该既有分流模式。
- `curl|bash` / `irm|iex` 管道形态下脚本无磁盘路径，禁用 `$0`/`$PSScriptRoot` 自定位 —— 本任务不新增自定位需求，沿用约束即可。
- `actions/setup-go@v5` 的 `go-version` 应与 `go.mod` 顶部对齐 —— `release.yml` 现有 `go-version: '1.25'` 已对齐，新增 `main` 触发路径复用同一 job 时保持不变。

---

## 8. 风险与给 PM/用户的开放问题

### 8.1 风险（已识别，缓解写入 in-scope/边界，无需用户裁决）

| 风险 | 影响 | 缓解 |
|---|---|---|
| `releases/latest` 端点只返回非 prerelease、非 draft 的 release | 若滚动发布设为 prerelease 而安装脚本仍查 `releases/latest`，则安装脚本永远取不到滚动发布 | BC-5 / BC-10 显式点明；OQ-1 要求 Architect 令"prerelease 标记"与"查询端点"自洽。 |
| 每次 `main` push 都跑构建 | CI 时间/额度消耗上升 | NFR-4：本期接受，仅 `main` 触发、不在 PR 触发；公共仓库免费额度内。 |
| 短时间连续 push `main` 引发并发 workflow run 刷新同一滚动发布 | 竞态下资产可能短暂不一致 | BC-9：最终一致性可接受；竞态行为由 Architect 在设计中说明（OQ-3）。 |
| 滚动发布作为 `releases/latest` 时会与未来 `v*` 版本化发布争抢"latest"语义 | 安装脚本取到的目标可能在两者间漂移 | BC-6 / BC-10 / NFR-3：要求安装脚本查询确定性目标；倾向查固定标签端点（见 OQ-1）。 |
| 本任务不真实触发 CI | AC-16 / AC-17 无法在本流水线内验证 | PM 决策 5 已定：交付后由用户 push `main` 实测；AC-16/AC-17 标注为"交付后验证，非完成闸门"，PM 在 `07_DELIVERY.md` 写明实测前提。 |

### 8.2 开放问题（留给下游裁决，非阻塞用户）

> 以下 3 项均属**设计决策**，按角色契约 RA 不做技术选型，交 Solution Architect 在 `02_SOLUTION_DESIGN.md` 定稿；PM 在 INPUT.md 决策 2 已明确"滚动发布的设计由 Solution Architect 定稿"。因此本任务**无需用户裁决的开放问题**，下列为传递给 Architect 的设计待定项。

1. **【OQ-1 → Architect】滚动发布是否标记为 prerelease，以及安装脚本查询哪个端点。**
   候选 (a)：滚动发布设为**正式 release**（非 prerelease）+ 安装脚本继续查 `releases/latest`——简单，但未来 push `v*` 后 `latest` 会被版本化发布顶替，且滚动发布会出现在 Releases 页"latest"徽章上。
   候选 (b)：滚动发布设为**正式 release** + 安装脚本改查 `releases/tags/<固定标签>`——确定性强，安装脚本永远锁定滚动发布，不受 `v*` 发布干扰（INPUT.md 决策 2 倾向此项）。
   候选 (c)：滚动发布设为 **prerelease** + 安装脚本查 `releases/tags/<固定标签>`——滚动发布不抢占 `latest` 徽章，但**必须**配合查固定标签端点（查 `releases/latest` 会失败，即 BC-5）。
   约束：(a) 与 prerelease 标记互斥；安装脚本端点选择必须与 prerelease 决策自洽。

2. **【OQ-2 → Architect】滚动发布的固定标签名与 Release 标题。**
   候选 (a)：标签 `rolling` / 标题"最新构建（rolling）"。
   候选 (b)：标签 `latest` / 标题"最新版本"。
   候选 (c)：标签 `nightly` / 标题"每日构建"——但本方案是"每次 push"而非"每日定时"，`nightly` 语义不精确。
   约束：标签名不得与 `v*` 版本化 tag 命名空间冲突。

3. **【OQ-3 → Architect】并发 `main` push 时多个 workflow run 刷新同一滚动发布的竞态处理。**
   候选 (a)：不做特殊处理，接受"最后完成的 run 决定最终资产"的最终一致性（BC-9 默认可接受语义）。
   候选 (b)：在 workflow 中加 `concurrency` 组（如 `group: rolling-release, cancel-in-progress: true`），让新 run 取消进行中的旧 run，保证只有最新提交的产物被发布。
   约束：无论选哪项，最终滚动发布资产必须与某次真实 `main` 构建一致，不得是两次构建混合的损坏状态。

---

## 9. Verdict

**READY FOR DESIGN** — 无需用户裁决的开放问题。第 2 节 14 条 In-scope 行为、第 4 节 12 条边界条件、第 5 节 17 条 AC 均不依赖任何未决的用户问题。第 8.2 节 3 项开放问题（OQ-1/OQ-2/OQ-3）均属技术设计决策，按角色契约与 PM INPUT.md 决策 2 交由 Solution Architect 在 `02_SOLUTION_DESIGN.md` 定稿，不构成对用户的阻塞。

> 下游说明：
> - Architect 不得推翻 PM 在 `INPUT.md` 中的 5 项预决策。
> - Architect 必须在 `02_SOLUTION_DESIGN.md` 中对 OQ-1/OQ-2/OQ-3 给出明确裁决，并保证 OQ-1 的"prerelease 标记"与"安装脚本查询端点"两个决策自洽（BC-5 / BC-10 为硬约束）。
> - 本任务**不自动 push `main`、不打 tag、不触发真实发布**；AC-16/AC-17 为"交付后人工验证、非完成闸门"，PM 在 `07_DELIVERY.md` 注明"需用户首次 push `main` 触发滚动发布后方可实测"。任务完成闸门为静态/自动 AC（AC-1…AC-15）。
> - 资产命名约定沿用 T-010：`frp-easy-<VERSION>-<os>-amd64.{tar.gz,zip}`；安装脚本资产匹配正则不变。
