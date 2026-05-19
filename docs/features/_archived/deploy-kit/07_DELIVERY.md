# 07 — 交付：T-008 deploy-kit

> Stage 7 of 7-stage `/harness` 流水线 · 语言：中文
> 任务：把 frp_easy 从"需要 Go+Node 自己 build"升级为"傻瓜下载 + 解压 + 跑"的成熟可部署项目
> 交付日期：2026-05-19

---

## 1. 任务起源

用户问"当前项目是否可以直接写个部署文档就够用，最好傻瓜式"。

PM 评估后判定**仅文档不够**：当前缺口是构建门槛（Go 1.22+ + Node 18+）+ 无预打包发布产物 + 无系统服务化 + 无 `--version` 自检。按用户授权"以用户体验好、符合软件工程标准、长期易使用易维护为原则"开启 T-008 deploy-kit 全 7-stage 流水线。

---

## 2. 交付内容（10 个文件）

| # | 路径 | 状态 | 责任 |
|---|---|---|---|
| 1 | `cmd/frp-easy/main.go` | edit | 新增标准库 `flag` 解析（`--version` / `-v` / `--help` / `-h`），早于 appconf.Load |
| 2 | `scripts/package.sh` | new | Linux/macOS 打包脚本（含 sanity check 与 LICENSE WARN 降级） |
| 3 | `scripts/package.ps1` | new | Windows 打包脚本（同上） |
| 4 | `scripts/install-service.sh` | new | Linux systemd unit 安装（双重 chmod 原子写 + trap 清理 + `ExecStart="${BINARY}"` 双引号包路径） |
| 5 | `scripts/uninstall-service.sh` | new | Linux 卸载（disable --now + reset-failed + 数据目录保留） |
| 6 | `scripts/install-service.ps1` | new | Windows Service 安装（管理员检测 + wrapper.cmd 锁 cwd + 轮询 stop） |
| 7 | `scripts/uninstall-service.ps1` | new | Windows 卸载（轮询 stop + 删 wrapper.cmd + 数据目录保留） |
| 8 | `docs/DEPLOYMENT.md` | new | 部署权威文档：3 路径 + 决策表 + 升级 + 故障排查 F.1–F.5 |
| 9 | `README.md` | edit | 顶部加快速开始/部署 + 删冗余 + 开发模式下沉 |
| 10 | `docs/dev-map.md` | edit | 索引追加 |

无新 Go 依赖（仅标准库 `flag`），无 storage / migration / 前端改动。

---

## 3. 流水线执行回顾

| Stage | 输出 | 关键事件 |
|---|---|---|
| 1 Requirement Analyst | 01_REQUIREMENT_ANALYSIS.md（9 章 + 24 AC + 10 Open Questions 全 PM-resolved） | verdict READY |
| 2 Solution Architect | 02_SOLUTION_DESIGN.md（12 章 + 5 批实现顺序 + 12 风险） | verdict READY FOR GATE REVIEW |
| 3 Gate Reviewer | 03_GATE_REVIEW.md | verdict APPROVED FOR DEVELOPMENT（条件式）— 1 MAJOR-1 partition 文字依据（PM 在派发时显式授权解决）+ 4 项 MINOR 建议 |
| 4 Developer (dev-backend) | 04_DEVELOPMENT.md | 10 文件落地，verify_all 18 PASS |
| 5 Code Reviewer | 05_CODE_REVIEW.md | verdict CHANGES REQUIRED — 1 MAJOR-1（systemd unit 含空格路径解析失败）+ 5 MINOR |
| 4 Developer (rework) | 04_DEVELOPMENT.md（追加 Rework 段） | MAJOR-1 + 5 MINOR 全修，verify_all 仍 18 PASS |
| 6 QA Tester | 06_TEST_REPORT.md（PM 接手撰写）| QA 子 agent 因 API 额度耗尽未交付；PM 亲跑 19 条本机 AC + 6 条 Adversarial + verify_all；verdict READY FOR DELIVERY |
| 7 Delivery | 本文件 + archive + commit | — |

总耗时：单 session，~2 小时。

---

## 4. 最终 verify_all

```
=== Summary ===
  PASS: 18
  WARN: 0
  FAIL: 0
  SKIP: 0
```

详细 18 项见 06_TEST_REPORT.md §4。

---

## 5. 用户视角的能力提升（Before / After）

| 维度 | Before（T-007 完成时） | After（T-008 交付） |
|---|---|---|
| 首启门槛 | 装 Go 1.22+ + Node 18+ + git clone + 自行 build | 下载 zip → 解压 → `./frp-easy` |
| 部署文档 | README 把"安装/开发/更新/dev-mode"混排，非开发者难找入口 | `docs/DEPLOYMENT.md` 三路径决策表 + 复制即用命令 + 5 场景故障排查 |
| 生产运行 | 手工 nohup / screen 自托管 | 一条命令 systemd / Windows Service 安装；失败自动重启；干净卸载 |
| 版本核对 | 没有 `--version` flag | `./frp-easy --version` + 包内 VERSION 文件双重核对 |
| 跨平台 | 无预打包 | `frp-easy-<ver>-linux-amd64.tar.gz` + `frp-easy-<ver>-windows-amd64.zip` 各 ~19 MB |
| 含空格 / 中文路径 | 无保证 | systemd unit 双引号包路径 + Windows wrapper.cmd `-Encoding Default` 兼容 |

US-1（傻瓜用户）/ US-2（开发者）/ US-3（运维）/ US-4（升级用户）四类用户故事全覆盖。

---

## 6. 已知留待 release-smoke 验证项（不阻塞交付）

5 条系统服务 AC（AC-6/7/8/10）做了静态验证（脚本模板正确、bash -n 通过、非 root 拒绝路径生效），实际 `systemctl is-active` / `sc query` 行为**未在用户开发主机上跑**（避免污染主机系统服务列表）。建议在首次 release 时各跑一次 Linux + Windows 主机 smoke：

```bash
# Linux
sudo ./scripts/install-service.sh && systemctl is-active frp-easy
sudo ./scripts/uninstall-service.sh && systemctl status frp-easy

# Windows（管理员 PowerShell）
.\scripts\install-service.ps1 ; sc query frp-easy
.\scripts\uninstall-service.ps1 ; sc query frp-easy
```

MINOR-R3 实测项：Windows `sc.exe stop` 信号是否能优雅传播到 wrapper.cmd 下的 frp-easy.exe 子进程。Stage 4 rework 通过 `Wait-ServiceStopped` 轮询 5 秒缓解；release smoke 时需专项确认（停服后任务管理器中 frp-easy.exe 进程已退出）。

---

## Insight

> 收割到 `.harness/insight-index.md` 的非显然事实。Archive 脚本会自动 harvest 本段。

- **2026-05-19** · systemd unit 中 `ExecStart=${PATH}` 与 `WorkingDirectory=${PATH}` 必须用双引号包路径（`ExecStart="${PATH}"`，systemd 5.0+ 语法），否则路径含空格时 systemd 按空格分参导致启动失败。Code Review MAJOR-1 直接证据 · evidence: T-008 deploy-kit
- **2026-05-19** · Windows Service 通过 sc.exe 创建时，binPath 指向 wrapper.cmd 包装而非 .exe 本身可锁定 cwd（`cd /d "$InstallDir" && "$BinaryPath"`），但 `Set-Content -Encoding ASCII` 写 .cmd 会让中文路径乱码，需 `-Encoding Default`（host codepage） · evidence: T-008 deploy-kit
- **2026-05-19** · Go stdlib `flag.NewFlagSet(name, flag.ContinueOnError)` + `fs.SetOutput(io.Discard)` 是中文化 / 自定义错误输出的标准范式；显式 `errors.Is(err, flag.ErrHelp)` 分流 `-help` 单 dash 形式仍可触发（非死代码），与已注册 `-h`/`--help` BoolVar 不冲突 · evidence: T-008 deploy-kit
- **2026-05-19** · verify_all A.1 secrets scan 正则 `(api_key|secret|password|token)[\s]*[:=][\s]*["'][^"']{8,}["']` 会误中文档/脚本内的样例字面量；写 `frp_easy.toml.example` 之类时只列字段名 = 默认值，避免任何 8+ 字符引号串 · evidence: T-008 deploy-kit
- **2026-05-19** · `sudo` 调用 bash 脚本时 `id -un` 返回 root；要拿到真实调用者用 `${SUDO_USER:-$(id -un)}` 优先 `$SUDO_USER` 才符合 "默认 user = 当前调用者" 的意图 · evidence: T-008 deploy-kit
