# 01 — 需求分析：T-010 deploy-polish-and-ci

> Stage 1 of 7-stage `/harness` 流水线 · 中文 · PM-authored（清理性任务）

---

## 1. 任务目标

把 T-008/T-009 留下的"几乎傻瓜部署"推进到**真正闭环**：用户从仓库主页一路点到首启完成无空气墙、长跑服务不爆盘、日常运维更省心。

## 2. 用户故事

### US-1（普通用户首启）
作为非开发者，**当**我打开 README/DEPLOYMENT 文档**时**，
**我希望**所有 URL 与命令都能直接复制运行（不要让我替换占位符），
**这样**我不会卡在第一步。

### US-2（Windows 双击用户）
作为下载发布产物的 Windows 用户，**当**我双击 frp-easy.exe **时**，
**我希望**浏览器自动打开 UI 地址（而不只是 stderr 一行小字），
**这样**我不需要复制 URL。

### US-3（长跑运维）
作为把 frp-easy 注册成 systemd / Windows Service 的运维，**当**服务连续运行数月**时**，
**我希望**ui.log 不会无限增长占满磁盘，
**这样**主机不会因为日志爆盘出故障。

### US-4（项目维护者）
作为项目作者，**当**我打 git tag 推送**时**，
**我希望**Linux + Windows 发布产物自动构建并上传到 GitHub Releases，
**这样**普通用户能真的下到东西（解除 DEPLOYMENT.md A.1 的"空发布页"困境）。

## 3. 验收标准（AC）

### AC-1 占位符消除
- `git grep '<ORG>'` 在 `docs/**` 与 `scripts/**` 下应**零命中**（除 _archived/ 历史文档不动）。
- 替换为真实 GitHub owner `Alan-IFT`。

### AC-2 浏览器自动打开
- `frp-easy` 在交互式终端（`os.Stdin` 是 TTY 且 `FRP_EASY_NO_BROWSER` 环境变量未设置）下，启动成功后自动调用平台默认浏览器打开 UI 地址。
- 提供 `--no-browser` flag 与 `FRP_EASY_NO_BROWSER` 环境变量两种关闭手段。
- 非交互式启动（systemd / Windows Service / `nohup`）**必须**不打开浏览器（避免污染 server 环境）。
- 失败（无浏览器 / xdg-open 缺失）不影响主流程，仅打 WARN。
- `--help` 输出更新，新增 flag 与环境变量说明。

### AC-3 日志轮转
- ui.log（应用日志）按 size 轮转：默认单文件上限 10 MB，保留 5 个历史，最长 30 天。
- 通过环境变量可调（`FRP_EASY_LOG_MAX_SIZE_MB` / `FRP_EASY_LOG_MAX_BACKUPS` / `FRP_EASY_LOG_MAX_AGE_DAYS`）。
- 轮转产物权限保持 0o600（不放宽）。
- 子进程日志（frpc.log/frps.log）本期**不轮转**（属于 frp 进程行为，跨进程协调复杂；纳入 §6 已知留待）。

### AC-4 CI 发布工作流
- 新增 `.github/workflows/release.yml`，触发条件：`push: tags: ['v*']`。
- 构建产物：Linux amd64 `tar.gz` + Windows amd64 `zip`，文件名与 DEPLOYMENT.md A.1 约定一致（`frp-easy-<VERSION>-<os>-amd64.<ext>`）。
- 构建复用现有 `scripts/package.sh`（不重复定义构建逻辑）。
- 工作流上传到 GitHub Releases，自动用 tag 名（去前缀 `v`）作 release name。
- 工作流首跑前用户需手动 push `v0.1.0` tag；DEPLOYMENT.md / README 已含足够上下文（不新增"如何发版"段落，避免膨胀）。

### AC-5 verify_all 不退化
- 18 项 PASS 维持；不引入 FAIL/WARN。

### AC-6 新增单元测试
- logrotate（lumberjack 接入）至少 1 个测试：写满触发轮转、permission 保持。
- browseropen 至少 2 个测试：interactive TTY 命中 / 非 TTY 不命中（mock 化）；platform branch 至少桩到位（Linux/Windows/Darwin path 选择）。
- 不引入 E2E 改动（Playwright 测试不变）。

### AC-7 文档同步
- `docs/dev-map.md` 索引追加：browseropen 子包、logrotate 子包、`.github/workflows/release.yml`。
- README 默认端口表 / 部署决策表不变（无需修改）。

## 4. 范围与边界

**在范围内**：
- 文档/脚本占位符替换
- main.go 启动后自动开浏览器逻辑（标准库 `os/exec` + 平台 switch）
- ui.log 轮转（引入 `gopkg.in/natefinch/lumberjack.v2`，仅 1 个轻量依赖）
- 单个 GitHub Actions workflow 文件

**不在范围内**：
- frpc/frps 子进程日志轮转（跨进程行为，列入已知留待）
- 自更新机制（用户没要求，scope creep）
- Web UI 自身改动（不需要）
- 数据库 schema 变更（无需要）
- E2E 测试新增

## 5. 风险

| ID | 风险 | 缓解 |
|---|---|---|
| R-1 | 自动开浏览器在 systemd unit 下可能误触发（journalctl 重定向 stdin） | 用 `golang.org/x/term.IsTerminal(int(os.Stdin.Fd()))` 检测；service 模式 stdin 通常非 TTY，天然被排除。systemd_invocation_id 环境变量作 secondary 检测。 |
| R-2 | 引入 lumberjack 新依赖 | lumberjack 是 Go 生态标准日志轮转库（k8s/Prometheus 在用）；MIT license；纯 Go 无 cgo；接受。 |
| R-3 | xdg-open / cmd /c start / open 三平台调用差异 | 分别用 `exec.Command` 包装；失败 silent + WARN 日志，不阻塞主流程。 |
| R-4 | CI workflow 第一次跑会失败（无 Linux runner 经验、scripts/package.sh 在 GitHub-hosted runner 是否能跑 npm + go build） | release.yml 用 ubuntu-latest 跑两个目标（Linux native + Windows cross-compile via `GOOS=windows`），复用 `scripts/build.sh --all`；workflow 不在本期 verify_all 闸门内，user 推 tag 时由 GitHub 验证。 |
| R-5 | 占位符替换可能误中 _archived/ 历史文档 | 显式排除 `docs/features/_archived/`；只动当前文档。 |
| R-6 | lumberjack 重命名旧 ui.log 时 Windows 文件锁 | lumberjack 用 close + rename 模式，自身处理 Windows 锁；社区已验证（k8s on Windows）。 |
| R-7 | NF-S4 安全：自动开浏览器 + 0.0.0.0 绑定 → 暴露给本地浏览器一切 | 主流程不变；浏览器开的是 cfg.ListenAddr，0.0.0.0 时显示 0.0.0.0:8080（浏览器自动解析为 localhost），WARN 日志已有，不引入新风险。 |

## 6. 开放问题（PM 已决）

- **Q-1**：浏览器自动打开默认开还是关？→ **默认开**（TTY 启动），目标用户群是 Windows 普通用户。`--no-browser` 与 `FRP_EASY_NO_BROWSER` 是 opt-out。
- **Q-2**：lumberjack vs 自写轮转？→ **lumberjack**。自写轮转踩 Windows 文件锁 + 并发 fsync 坑，重造轮子违背"长期易维护"。
- **Q-3**：CI 同时发 macOS？→ **不发**。`scripts/build.sh` 当前只支持 linux/windows；macOS 用户走路径 B（源码构建）。等用户群有 mac 需求再加。
- **Q-4**：是否换 git remote origin URL 把 `<ORG>` 真的查出来？→ **不**，已查（`Alan-IFT`），直接硬编码进文档/脚本。`<ORG>` 占位符模式本身是反 UX 的（要求用户做替换）。
