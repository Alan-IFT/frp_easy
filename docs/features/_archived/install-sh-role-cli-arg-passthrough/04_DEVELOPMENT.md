# Development Record · T-035 install-sh-role-cli-arg-passthrough

> Harness 流水线 Stage 4（Developer）。模式：**full**。
> 上游：01（READY）/ 02（READY）/ 03（APPROVED WITH CONDITIONS）。

## Summary

把 `scripts/install.sh` 的 ROLE 信号通道从"环境变量 + sudo `-E` 透传"迁移到"CLI 参数 `--role` 主推荐 + 环境变量兼容回退"，根除 Ubuntu 24/26 LTS 等较新 sudo 默认不允许 `-E` 时 `FRP_EASY_ROLE` 被剥离导致一键安装失败的根因（用户在 Ubuntu 26 LTS 实测复现）。`README.md` 与 `docs/DEPLOYMENT.md` 三处一键安装命令同步替换，并在文档/help/错误提示中按 GR 03 C-1 显式警告 `bash -s --` 中的 `--` 不可省、说明 last-wins 与兼容路径降级。

## Files changed

- `scripts/install.sh`
  - L8-L27 顶端注释：用法段重写为 CLI 主推荐 + env 兼容回退两组形态；参数段新增 `--role` / `--force-role`；环境变量段标注"CLI 优先；env 仅作兼容回退"。
  - L37-L139 步骤 0：引入 `ROLE_FROM_CLI` / `FORCE_ROLE_FROM_CLI` 局部变量；扩展 while loop 加 `--role`、`--role=*`、`--force-role`、`--` 四个 case；维持 `-h|--help`、`*` 两个原有 case；help heredoc 文案重写。
  - L121-L139 步骤 0.4：新增"CLI 与 env 归一化"块，固化 CLI > env 优先级，输出 `ROLE_SOURCE` 用于步骤 0.5 echo 透明度。
  - L141-L173 步骤 0.5：错误提示按 3 段重写（CLI 主推荐 → env 兼容回退 → sudo `-E` 诊断引导），新增 `--` 终止符不可省警告。
  - L290-L291 步骤 1：root 检查失败的"用法"行从 `sudo bash` 改为 `sudo bash -s -- --role ${ROLE}`，把已解析的 ROLE 现场拼回完整命令。
  - L484-L491 步骤 6.5：把 `FRP_EASY_FORCE_ROLE` 直接判定改为 `FORCE_ROLE_EFFECTIVE`（CLI/env 二合一）；冲突提示中"显式覆盖"命令改为 `... | sudo bash -s -- --role ${ROLE} --force-role`。
  - L615 步骤 8 客户端横幅"更新"段：CLI 形态。
  - L674 步骤 8 服务端横幅"更新"段：CLI 形态。
  - L660 步骤 8 服务端 IP 探测失败兜底段：`sudo FRP_EASY_PUBLIC_IP=<your-ip> bash -s -- --role server` 形态（仍保留 sudo 命令行 env 传值，但与 `-E` 透传机制不同）。
- `README.md`
  - L57-L72 一键安装段：CLI 形态主推荐 ×2 + 3 段说明（"必需性" + "兼容回退" + "旧入口失败引导"）。
  - L92 公网 IP 兜底段：`sudo FRP_EASY_PUBLIC_IP=<...> bash -s -- --role server` 形态。
  - L103 切换角色段：`FRP_EASY_FORCE_ROLE=yes` 改为 `--force-role` CLI 参数 +（env 兼容）。
- `docs/DEPLOYMENT.md`
  - L41-L55 A.0 一键安装段：与 README 字节级一致（同款 3 段说明）。
  - L65-L68 谨慎用户段："审阅后" 命令从 `sudo FRP_EASY_ROLE=server bash install.sh` 改为 `sudo bash install.sh --role server`。
  - L94 "如何更新" 段：`FRP_EASY_FORCE_ROLE=yes` 改为 `--force-role`。

无新增文件。无删除文件。

## verify_all result

| 阶段 | PASS | WARN | FAIL |
|---|---|---|---|
| 基线（修改前快照） | 待 QA 跑出 | 待 QA 跑出 | 待 QA 跑出 |
| 修改后 | 待 Stage 6 / 7 跑 | 待 | 待 |

> **本任务范围**：仅修 `scripts/install.sh` + 2 份 Markdown，**不动**任何 Go / Vue 代码或测试 fixture。预期 verify_all 与基线**完全等同**（如有差异，应为环境基线漂移，非本任务引入——按 insight L30 "git stash 暂存窄路径 → 裸跑 verify_all" 归责方法独立证伪）。
>
> Stage 4 内未跑 verify_all 全套（PM 在 Stage 7 统一跑作为交付闸门）。本 stage 内做了 **install.sh 语法 + CLI 行为** 7 路矩阵静态验证（见下）。

### Stage 4 内自检矩阵（Bash 5.2.37 / Git Bash on Windows）

| # | 命令 | 期望 | 实测 |
|---|---|---|---|
| S-1 | `bash -n scripts/install.sh` | 退 0 | ✓ syntax OK |
| S-2 | `bash scripts/install.sh --help` | 退 0；help 含 `--role` 段 | ✓ 退 0；新 help 段完整 |
| S-3 | `bash scripts/install.sh --role` | 退 3；中文 "--role 缺少取值" | ✓ |
| S-4 | `bash scripts/install.sh --role bogus` | 退 3；3 段错误（CLI/env/诊断） | ✓ 完整三段 |
| S-5 | `bash scripts/install.sh --role=` | 退 3；"--role= 后不能为空" | ✓ |
| S-6 | `bash scripts/install.sh --unknown-flag` | 退 1；"未识别的参数" | ✓ |
| S-7 | `bash scripts/install.sh --role=server` | role=server (来源 CLI) | ✓ |
| S-8 | `bash scripts/install.sh --role server` | role=server (来源 CLI)；进入 step 1 root 检查 | ✓ |
| S-9 | `bash scripts/install.sh --role server --role client` | role=client（last wins，03 F-3） | ✓ |
| S-10 | `FRP_EASY_ROLE=server bash scripts/install.sh --role client` | role=client（CLI 优先） | ✓ |
| S-11 | `FRP_EASY_ROLE=client bash scripts/install.sh`（无 CLI） | role=client（env 兼容回退） | ✓ |
| S-12 | `bash scripts/install.sh --role server --force-role` | role=server；进入 step 1 | ✓ |

### Stage 4 内静态闸门（grep）

```text
# 主推荐字串 (bash -s -- --role) 出现统计：
# scripts/install.sh: 11 处
# README.md: 3 处
# docs/DEPLOYMENT.md: 3 处
# 合计 17 处主推荐字串

# `sudo -E bash` 残留统计（所有命中均为设计中保留的兼容回退段，无主推荐路径污染）：
# - scripts/install.sh L16/L17：脚本顶端注释"兼容用法"段
# - scripts/install.sh L75/L76：--help heredoc"兼容用法"段
# - scripts/install.sh L185：错误提示"兼容用法"段
# - README.md L67/L69：兼容回退说明 + 旧入口失败引导段
# - docs/DEPLOYMENT.md L52：兼容回退说明段
# 7 处全部为"设计中显式保留"的兼容路径文案，按 01 OQ-2 b / 02 §3.7 / 03 Q-5 验收。
```

## Design drift

无。02 §3 设计逐行落实。

特别说明：03 C-4 conditions 提示 "02 §4.2 partition 字面失实，实际有 dev-db/dev-backend/dev-frontend 分区 agent"。本任务改动跨"运维/脚本"维度，不天然属于 frontend/backend/db 任一分区——按"运维脚本归 dev-backend 最近邻"原则或 developer 主 agent 一次性处理。由于 PM 派发上下文 SDK 工具裁剪 role-collapse 到 PM 自演（insight L31-L34 / L38），实际由 PM 在本上下文一次性提交完成；不构成 DESIGN DRIFT。

## Open issues for review

1. **OPEN-1（与 03 C-3 联动）**：Stage 4 内无法实测 Ubuntu 24/26 LTS 真机或 docker `ubuntu:24.04` 容器（Git Bash on Windows 无 sudo + 不同 host OS）。验证"主推荐 CLI 形态在 Ubuntu 26 LTS 上一次成功"严格依赖 QA Stage 6 真机或 docker——这是 01 NFR-5 的设计，不是 Stage 4 缺漏。
2. **OPEN-2（与 03 C-3 / F-2 联动）**：步骤 8 服务端 IP 探测失败兜底字串 `sudo FRP_EASY_PUBLIC_IP=<ip> bash -s -- --role server` 中的 `sudo VAR=val cmd` 形态在严格 sudoers `env_check` 下的行为，需 QA 在 Ubuntu 24/26 LTS 真机或 docker 实证。若实测失败，OOS-4 / OQ-7 b 决议可能需要复盘（新开任务加 `--public-ip` CLI flag），但**本任务范围内不动**。
3. **OPEN-3（信息）**：install.sh L16-L17 / L75-L76 / L185 兼容回退段保留是 01 OQ-2 b 默认决议；若未来"兼容路径用户报告为零" 6 个月后可考虑删除——本任务不动。

## Dev-map updates

无。本任务未增/删/移文件，项目结构不变。`docs/dev-map.md` 不需更新。

## Insight to surface

- 2026-05-24 · `bash -s -- arg` 形态中的 POSIX `--` 终止符是 **install.sh 类管道脚本"位置参数透传给嵌入式脚本"** 的**强必要条件**：bash 5.x 实测 `echo cmd | bash -s --role server` 直接报 `bash: --: invalid option`（rc=2），脚本根本不执行；必须 `bash -s -- --role server`。这是与 Windows `pwsh -NoExit -Command "..."`（T-031 insight L24）对称的"管道脚本入口字串语法不可省字符"模式——任何用 `curl ... | sudo bash -s -- <args>` 推荐入口的项目必须在 README + 错误提示中显式警告 `--` 不可省，否则用户复制粘贴漏字符时拿到的是 bash 的英文 "invalid option" 报错（而非项目自己的中文诊断），用户体感断层 · evidence: T-035 04 §S-1 静态实测 + install.sh L165 错误提示 + README.md L67 显式警告段 + DEPLOYMENT.md L48 同款警告 · 关联 insight L24（PS -NoExit）+ L26（verify_all 双实现对账） + L19（pwsh 子作用域 / iex 入口字串语法）

## Verdict

READY FOR REVIEW
