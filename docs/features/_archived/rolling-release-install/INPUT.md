# T-013 原始输入

> 用户原话与 PM 决策，PM Orchestrator 留档。

## 背景

T-012 交付了一键安装脚本（install.sh / install.ps1），但它依赖 GitHub Releases 下载发布包，而仓库尚无任何 Release。用户反馈不想维护 release，希望"下载仓库到本地、运行安装、注册 service、开机自启、启动项目"。

PM 指出"下载仓库 + 本地构建"会要求每个用户预装 Go+Node+npm 工具链，反而让用户使用更难，与用户"用户使用也简单"目标冲突。核心问题是"谁编译二进制"。PM 用 AskUserQuestion 给出三个方案，**用户选定：自动滚动发布**。

## 用户选定方案：自动滚动发布

改造 GitHub Actions：每次 push 到 `main` 分支，自动编译并更新一个固定的"最新版"发布（rolling release）。维护者此后无需手动打 tag / 发版；`install.sh` / `install.ps1` 始终下载这个滚动发布的预编译二进制。用户零工具链、秒装。

## 事实基线（PM 已核查）

- 现有 `.github/workflows/release.yml`（T-010 创建）：触发条件为 push `v*` tag；复用 `scripts/build.sh --all` + `scripts/package.sh`；用 `softprops/action-gh-release@v2` 创建以版本号命名的 Release，上传 `bin/release/*.tar.gz` + `*.zip`。
- 现有 `scripts/install.sh` / `install.ps1`（T-012 创建）：调 GitHub API `https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest` 取最新 release 资产。
- `.github/workflows/` 不在红线文件清单内（红线仅 `.github/copilot-instructions.md`），可编辑。

## PM 已定决策（用户授权全权决策，原则：用户体验好 / 符合软件工程标准 / 长期易维护）

1. **改造 `release.yml`**：新增 `push: branches: [main]` 触发，构建后创建/更新一个固定标签的滚动发布（rolling release）。保留原 `v*` tag 触发的版本化发布能力（零额外成本、给将来正式发版留口子）。
2. **滚动发布的设计**由 Solution Architect 定稿，但须解决一个已知坑：GitHub API 的 `releases/latest` 端点只返回**非 prerelease、非 draft** 的最新 release；若滚动发布标记为 prerelease 则 `releases/latest` 取不到。须明确滚动发布是否 prerelease、以及 install 脚本应查 `releases/latest` 还是 `releases/tags/<固定标签>`（倾向后者，确定性更强）。
3. **同步调整 `install.sh` / `install.ps1`** 的 API 查询逻辑以匹配滚动发布方案。
4. **更新 README / DEPLOYMENT.md** 中与发布/安装相关的说明。
5. 本任务不自动 push、不自动触发真实发布——交付后由用户自行 push main 触发首次滚动发布。

下游 agent 不得在未经 PM 的情况下推翻这些决策；有异议在 PM_LOG.md 提阻塞。
