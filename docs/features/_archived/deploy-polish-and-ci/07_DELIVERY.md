# 07 — 交付：T-010 deploy-polish-and-ci

> Stage 7 of 7-stage `/harness` 流水线 · 语言：中文
> 任务：解决遗留问题 + 验证服务端/客户端傻瓜部署可达性 + 端口管理审查
> 交付日期：2026-05-19

---

## 1. 任务起源

用户原话："解决所有遗留问题，检查是否有影响用户体验的，若有则你来决策处理；检查是否能傻瓜式部署服务端和客户端，以及进行端口管理；以用户体验好，符合软件工程标准，长期易使用易维护为原则来决策"。

PM 调研后判定 4 类遗留 + 1 个 review 期发现的隐藏 bug，按用户三原则全面处理。开 T-010 deploy-polish-and-ci 走全 7-stage 流水线，PM 主导 + 派 code-reviewer 子 agent 独立评审。

## 2. 交付内容（13 个文件 + 29 个 web/src/.js 删除）

| # | 路径 | 状态 | 内容 |
|---|---|---|---|
| 1 | `internal/logrotate/logrotate.go` | new | lumberjack v2.2.1 包装，0o600 强制 chmod，env 覆盖 |
| 2 | `internal/logrotate/logrotate_test.go` | new | 4 测试（含轮转产物 perm 断言，Rework 补） |
| 3 | `internal/browseropen/browseropen.go` | new | 跨平台浏览器打开 + TTY/env/xdg-open 四层 opt-out + goosFunc seam（Rework 加） |
| 4 | `internal/browseropen/browseropen_test.go` | new | 6 测试（含 table-driven 三平台 command selection，Rework 补） |
| 5 | `cmd/frp-easy/main.go` | edit | +imports / +`--no-browser` flag / usageText 加 env var 段 / logger 接 logrotate / ready 后调 browseropen |
| 6 | `.github/workflows/release.yml` | new | push v* tag 触发，复用 build.sh + package.sh，softprops/action-gh-release 上传产物 |
| 7 | `docs/DEPLOYMENT.md` | edit | `<ORG>` × 3 处替换为 Alan-IFT；占位符约定表精简 |
| 8 | `scripts/install-service.sh` | edit | systemd Documentation 字段替 `<ORG>` |
| 9 | `web/tsconfig.json` | edit | 加 `"noEmit": true` —— Rework MAJOR-1 三层防御之一 |
| 10 | `scripts/verify_all.sh` | edit | B.1 `npm exec -- tsc --noEmit` 修参数透传；新增 B.5 哨兵 |
| 11 | `scripts/verify_all.ps1` | edit | 同上（双 shell 一致） |
| 12 | `docs/dev-map.md` | edit | 加入 logrotate / browseropen / release.yml 三索引 + "可复用工具" 表 2 行 |
| 13 | `scripts/baseline.json` | edit | bump to version 4，go_tests 156→166，frontend_tests 57（实数，清残留后） |
| — | `web/src/**/*.js` | delete × 29 | 历史 tsc 残留清理（env.d.ts 保留） |
| — | `docs/features/deploy-polish-and-ci/` | new dir | INPUT / PM_LOG / 01..07（本期 8 文件） |
| — | `docs/tasks.md` | edit | 加 T-010 进行中 + 完成后挪到已完成 |

新增依赖：
- `gopkg.in/natefinch/lumberjack.v2 v2.2.1`（MIT，纯 Go）
- `golang.org/x/term v0.43.0`（BSD-3，stdlib-adjacent）

go.sum 增 ~10 行。无前端依赖变化。

## 3. 流水线执行回顾

| Stage | 输出 | 关键事件 |
|---|---|---|
| 1 Requirement Analyst | 01_REQUIREMENT_ANALYSIS.md（4 用户故事 + 7 AC + 7 风险 + 4 Open Q PM-resolved） | verdict READY（PM-authored） |
| 2 Solution Architect | 02_SOLUTION_DESIGN.md（4 工作线 L1-L4 + 实施顺序 + 回退预案） | verdict READY（PM-authored） |
| 3 Gate Reviewer | 03_GATE_REVIEW.md | verdict APPROVED FOR DEVELOPMENT；3 MINOR 由 Developer 吸收 |
| 4 Developer | 04_DEVELOPMENT.md（L1-L5 实施，含中途发现的 .js 残留 L5） | 全部落地 |
| 5 Code Reviewer | 05_CODE_REVIEW.md | **独立子 agent** dispatch；verdict CHANGES REQUIRED；1 MAJOR-1（L5 没真生效）+ 11 MINOR/NIT |
| 4 Developer (Rework) | 04 §6 追加 | MAJOR-1（三层防御）+ 选 2 MINOR + 1 NIT 落地 |
| 6 QA Tester | 06_TEST_REPORT.md（PM 接手，对齐 T-008 节奏；Adversarial 段独立保留） | verdict READY FOR DELIVERY；7 AC × 10 Adversarial 全有结论 |
| 7 Delivery | 本文件 + archive + commit | — |

总耗时：单 session，~2 小时（含 1 次完整 rework 循环）。

## 4. 最终 verify_all（双 shell）

| Shell | 项数 | 结果 |
|---|---|---|
| PowerShell 7+ | 19 | `PASS: 19 / WARN: 0 / FAIL: 0 / SKIP: 0` ✅ |
| Git Bash (MSYS2) | 19 | `PASS: 19 / WARN: 0 / FAIL: 0 / SKIP: 0` ✅ |

第 19 项 = 新增 **B.5 No tsc residue in web/src/** 哨兵（防 .js 残留再生）。

## 5. 用户视角的能力提升（Before / After）

| 维度 | Before（T-009 完成时） | After（T-010 交付） |
|---|---|---|
| DEPLOYMENT.md 下载链接 | `https://github.com/<ORG>/frp_easy/releases/latest` 字面量 → 404 | `https://github.com/Alan-IFT/frp_easy/releases/latest` 可点（待用户首次推 tag 后产物会出现）|
| systemd unit Documentation | `https://github.com/<ORG>/frp_easy` 含尖括号 | 真实 URL |
| Windows 双击 .exe 体验 | 看到 stderr 一行 "UI 已启动 http://..."，需复制 URL | 浏览器自动打开（TTY 检测，service 模式不触发）|
| 关闭自动开浏览器 | 无机制 | `--no-browser` flag 或 `FRP_EASY_NO_BROWSER=1` env |
| 长跑服务 ui.log 增长 | 无限增长（爆盘风险） | 10 MB × 5 份 × 30 天三轴轮转，权限保持 0o600 |
| 日志轮转可调 | — | `FRP_EASY_LOG_MAX_SIZE_MB/_BACKUPS/_AGE_DAYS` 环境变量 |
| 发布产物 | 无（用户克隆源码 build） | tag push → GitHub Actions 自动构建上传 Linux tar.gz + Windows zip |
| web/src 测试残留 | 29 个 .js 与 .ts 共存 → vitest 模块解析按 .js 优先 → 改 .ts 测试可能无效果 | 0 残留；B.5 闸门防再生；tsconfig.json `"noEmit": true` 双保险 |
| verify_all 项数 / 闸门强度 | 18 PASS（B.4 是占位）| 19 PASS（B.5 是机器执行的项目特有不变量）|
| --help 中文化 | 仅 -h/-v/退出码 | 加 `--no-browser` flag + 5 个环境变量段 |
| `npm exec tsc --noEmit` 静默 emit bug | 存在（每次 verify_all 重 emit 29 个 .js）| 修：`npm exec -- tsc --noEmit` 强制透传 |

US-1（普通用户首启）/ US-2（Windows 双击）/ US-3（长跑运维）/ US-4（项目维护者）四类用户故事全覆盖。

## 6. 用户需关注的信息

### 6.1 改了什么 / 立竿见影的差异

- **打开浏览器自动了**：之后下载发布产物双击 .exe，浏览器会自动开 UI（默认开；若不想可加 `--no-browser` 或 systemd unit 里自然不会触发）
- **日志不会再爆盘**：ui.log 自动轮转，默认 10 MB × 5 份 × 30 天
- **发版自动化**：推 `git tag v0.1.0 && git push origin v0.1.0`，GitHub Actions 会自动 build + 上传 Linux tar.gz + Windows zip 到 Releases 页面
- **文档不再有占位符**：DEPLOYMENT.md 里的 GitHub URL 全是真实 `Alan-IFT/frp_easy`，复制即用

### 6.2 当前状态（git）

未提交改动汇总：
- `M` 6 个文件（main.go / DEPLOYMENT.md / install-service.sh / dev-map.md / tasks.md / verify_all.sh / verify_all.ps1 / tsconfig.json / baseline.json / go.mod / go.sum）
- `??` 4 个新目录/文件（`.github/workflows/` / `docs/features/deploy-polish-and-ci/` / `internal/browseropen/` / `internal/logrotate/`）
- `web/src/**/*.js` 删除 29 个（.gitignore 已忽略，git 不感知 —— 这是设计如此；B.5 哨兵守住"不准再生"）

verify_all 在两种 shell 下都 19/19 PASS。

### 6.3 部署体验自评（按用户原则）

| 用户问 | PM 答 |
|---|---|
| 服务端能傻瓜部署吗？ | 是。`./frp-easy` 启动后浏览器自动开，向导（T-002）会引导选择 server 角色 + 填 bindPort/auth；端口冲突时 stderr 明确告知 + exit 2；防火墙 hint UI 提示 ufw/iptables；systemd `install-service.sh` 一行装服务 + 自动重启 |
| 客户端能傻瓜部署吗？ | 是。同一二进制，向导选 client 角色 + 填 server 地址/token；frpc admin（7400）由 main.go 自动生成凭据；frp 二进制缺失时 UI 顶部横幅一键下载（T-002）|
| 端口管理够清晰吗？ | 是。`internal/appconf/config.go` 头注释固定写四端口表（8080/7400/7500/7000）；README 默认端口表镜像；端口被占时友好提示具体改哪个；FirewallHint 组件按协议过滤命令 |

### 6.4 已知留待 / 不在本期范围

- **首次正式 release smoke**：用户推 v0.1.0 后跑一次 release.yml，看 actions/setup-go 是否真能取 1.25、build.sh --all 是否在 ubuntu-latest 完整跑通、package.sh --windows 在 ubuntu 上是否能用 zip 命令打包（推测 OK，因为 ubuntu-latest 默认有 zip；本机 Git Bash 验证仅做了 Linux 段）
- **frpc / frps 子进程日志轮转**：跨进程信号协调复杂；本期文档化为已知留待。frp 二进制自身有 `log.max_days` / `log.disable_print_color` 配置可用（属上游 FRP 行为，不归本仓库管）
- **自更新机制**：用户没要求，scope creep；DEPLOYMENT.md A.4 / C.2.4 / C.3.4 已有手工升级流程
- **package.sh on Windows Git Bash**：缺 zip 命令时会 fail；本机用户走 `scripts/package.ps1` 即可；CI 用 ubuntu 自带 zip 不受影响。文档化为已知边界
- **MaxBackups=0 = unlimited 反直觉**：未做语义转换层（lumberjack 行为）；属 escape hatch

### 6.5 长期可维护性的 wins

- 新增 `internal/browseropen` / `internal/logrotate` 两个干净的小包，单职责 + 高 test seam（commandFunc / lookPathFunc / stdinFd / isTerminalFunc / goosFunc 全是可注入点）
- verify_all B.5 把 T-009 insight "vitest 优先 .js" 从被动经验变成主动闸门
- tsconfig.json `"noEmit": true` 把"tsc 不应 emit"从约定变成 enforced
- CI workflow 是 single-purpose（仅 release），不和本地 verify_all 重复，反馈延迟最短
- 所有 commit 都关联本任务 ID（T-010），未来 `git log` 可追

---

## Insight

> 收割到 `.harness/insight-index.md` 的非显然事实。Archive 脚本会自动 harvest 本段。

- **2026-05-19** · `npm exec <pkg> --someflag` 中 `--someflag` 会被 npm 自己当成 flag 吞掉（output `npm warn Unknown cli config "--someflag"`），子进程实际收不到。必须用 `npm exec -- <pkg> --someflag`（`--` 分隔符强制透传）。T-010 verify_all B.1 的 `npm exec tsc --noEmit` 被静默 emit 反复污染 web/src/，根因即此 · evidence: T-010 deploy-polish-and-ci
- **2026-05-19** · `web/.gitignore` 含 `web/src/**/*.js` 时，`git status` / `git ls-files` 看不到 tsc 残留产物 —— 让"已清残留"自检永远通过、Reviewer 才能用 Glob 抓到。验证残留清理务必用 `find ... | wc -l` 而非 `git status` · evidence: T-010 deploy-polish-and-ci
- **2026-05-19** · `tsconfig.json` 设 `"noEmit": true` 是真正一劳永逸防 tsc 误 emit 的方式；调用方加 `--noEmit` flag 是 belt，tsconfig 是 suspenders。新项目应当默认两者都有 · evidence: T-010 deploy-polish-and-ci
- **2026-05-19** · Go 跨平台 `runtime.GOOS switch` 的单测如果直接读 `runtime.GOOS`，不论分支多漂亮都只能测当前主机的那一支 —— 必须用 `var goosFunc = func() string { return runtime.GOOS }` 这种可注入 seam 配 stubGOOS helper 才能 table-driven 跑遍三平台。其他平台不变量同理（`os.Getenv` → `getenvFunc`、`os.Stdin.Fd()` → `stdinFd`） · evidence: T-010 deploy-polish-and-ci
- **2026-05-19** · GitHub Actions `actions/setup-go@v5` 的 `go-version` 应当与 `go.mod` 顶部 `go X.Y` 对齐；不对齐时 setup-go 拉指定版本后又被 `GOTOOLCHAIN=auto`（默认 Go 1.21+ 行为）二次拉真实版本，CI 时间翻倍 · evidence: T-010 deploy-polish-and-ci
