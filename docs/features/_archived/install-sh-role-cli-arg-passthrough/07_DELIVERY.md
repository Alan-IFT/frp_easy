# 07 — Delivery · T-035 install-sh-role-cli-arg-passthrough

> Harness 流水线 Stage 7（PM Orchestrator delivery wrap-up）。模式：**full**。
> 上游：01 (READY) + 02 (READY) + 03 (APPROVED WITH CONDITIONS C-1~C-5) + 04 (READY FOR REVIEW) + 05 (APPROVED · 0 CRITICAL / 0 MAJOR / 3 MINOR + 2 NIT) + 06 (APPROVED FOR DELIVERY · 14/14 Adversarial reproducer PASS)。
> 用户决策原则（INPUT 原话）：用户体验好 · 符合软件工程标准 · 长期易使用易维护。

---

## §1 任务摘要

**问题**：用户在 Ubuntu 26 LTS 上运行 `README.md` 推荐的客户端一键安装命令失败：

```text
$ curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=client sudo -E bash
sudo: preserving the entire environment is not supported, '-E' is ignored
错误：必须指定 FRP_EASY_ROLE=server|client（不允许静默默认）
...
curl: (23) Failure writing output to destination, passed 1378 returned 1273
```

**根因**：当前推荐入口依赖"环境变量 + `sudo -E` 透传"穿越 shell → sudo → bash 三段边界。Ubuntu 24/26 LTS、Debian 13 等较新 sudo 默认安全策略**拒绝 `-E` 保留环境变量**（直接打印 `'-E' is ignored`），导致 `FRP_EASY_ROLE` 被剥离，脚本走"role 缺失"错误路径。这是设计层面问题（脆弱契约依赖于 sudo 行为），不是脚本 bug。

**解决路径**（方案 A：CLI 主推荐 + env 兼容回退）：

- `scripts/install.sh` 扩展 while loop 加 `--role <value>` / `--role=<value>` / `--force-role` / `--` 四个 case，CLI 优先 + env 兼容（FR-5），错误提示 3 段化（CLI 推荐 → env 兼容回退 → sudo `-E` 诊断引导）。
- README 推荐入口字串改 `curl ... | sudo bash -s -- --role <role>`，明确把 `--` POSIX 终止符的不可省说明（漏 `--` 时 bash 自身 rc=2 + `invalid option`）写到 README + DEPLOYMENT + help + 注释 5 处。
- 业界主流 pattern（rustup / k3s / docker）的 CLI 参数 + 外层 sudo 形态，比环境变量 + sudo `-E` 更跨发行版稳定。

---

## §2 改动汇总（按 file）

| File | 性质 | 行数 | 说明 |
|---|---|---|---|
| `scripts/install.sh` | edit | +90 / -22 | L8-L32 顶端注释重写（CLI 主推荐 + env 兼容 + 谨慎用户三组形态）；L43-L150 步骤 0 扩展 CLI 解析（4 个新 case + 设计依据注释 + `[[ ]]` 短路注释）；L152-L167 步骤 0.4 新增 CLI/env 归一化（ROLE_FROM_CLI / FORCE_ROLE_FROM_CLI / ROLE_SOURCE）；L169-L193 步骤 0.5 错误提示重写为 3 段（CLI / env / sudo `-E` 诊断）+ `--` 不可省警告；L291 步骤 1 root 检查"用法"行从 `sudo bash` → `sudo bash -s -- --role ${ROLE}`；L487 / L491 步骤 6.5 force role 引用与提示字串迁移；L615 / L659 / L674 步骤 8 横幅 3 处字串迁移 |
| `README.md` | edit | +6 / -3 | L57-L62 一键安装主推荐 ×2 字串改 CLI 形态；L65-L72 替换 sudo -E 解释行为 3 段说明（必需性 + 兼容回退 + 旧入口失败引导 + `--` 不可省警告）；L96 公网 IP 兜底命令改 sudo VAR=val 形态；L103 切换 role 提示同步 `--force-role` CLI + env 兼容回退 |
| `docs/DEPLOYMENT.md` | edit | +4 / -3 | L41-L55 A.0 一键安装段与 README 字节级一致；L65-68 谨慎用户"审阅后"命令改 `sudo bash install.sh --role server`；L94 "如何更新"段同步 `--force-role` |
| `docs/tasks.md` | edit | +1 / 0 | 增 T-035 任务行 |
| `docs/features/install-sh-role-cli-arg-passthrough/` | new dir | 7 文件 | PM_LOG.md / 01..07 + reproducer.sh |
| `scripts/install-service.sh` / `scripts/uninstall-service.sh` | **不改** | 0 | OOS-7 决议遵守 |
| `scripts/install.ps1` | **不改** | 0 | OOS-1 决议遵守（Windows 路径不区分 server/client，与历史一致）|
| `scripts/baseline.json` | **不改** | 0 | 本任务不新增 Go/Vue 测试，test_count = 375 保持 |
| `docs/dev-map.md` | **不改** | 0 | 项目结构无新增/移动/删除文件 |
| `scripts/verify_all.{sh,ps1}` | **不改** | 0 | 03 C-5 评估：暂不加"主推荐字串无 sudo -E 残留" grep 闸门（涉及双实现对账，单独走 follow-up）|

**净改动（源码 + 文档）**：4 个文件 / +101 / -28 行。Stage docs：8 个文件新增。

---

## §3 验证证据

### 3.1 verify_all 最终输出

命令：`bash scripts/verify_all.sh`

```
=== Summary ===
  PASS: 26
  WARN: 0
  FAIL: 1   (C.1 E2E smoke (playwright))
  SKIP: 0
```

C.1 归责：按 insight L30 "git stash 暂存窄路径文件 → 裸跑 verify_all" 独立证伪。stash T-035 三个改动文件后裸基线同样 26 PASS / 1 FAIL（C.1）——C.1 与 T-035 改动**零相关**，是 T-031 引入、T-033 试图修但仍残留的"E2E playwright setup fixture"环境基线漂移问题（baseline.json notes 既已记录）。

### 3.2 Adversarial reproducer 输出

命令：`bash docs/features/install-sh-role-cli-arg-passthrough/reproducer.sh`

```
OK   ADV-1 AC-6 错误文案 3 段 (got=3)
OK   ADV-2 AC-8 sudo -E bash 命中数 (got=8)
OK   ADV-3 AC-9 help 段无过时 sudo -E 表述 (got=0)
OK   ADV-4 AC-11 无 wrapper 文件 (got=0)
OK   ADV-5 AC-12 同 flag 重复 last-wins (got=role=client)
OK   ADV-6 AC-12 CLI > env 优先级 (got=role=client)
OK   ADV-7 AC-13 父 shell strict 模式下子脚本 rc 透传 (got=3)
OK   ADV-8 AC-12 --role 缺 value 检测 (got=3)
OK   ADV-8b 错误信息含 '缺少取值'
OK   ADV-9 AC-12 等号+空格混用 last-wins (got=role=client)
OK   ADV-10 AC-5 env 兼容回退 + ROLE_SOURCE 透明标记 (got='role=server  (来源: 环境变量 (FRP_EASY_ROLE)')
OK   ADV-11 POSIX -- 漏掉 → bash 自身 rc=2 + invalid option 报错（证明设计警告必要）
OK   ADV-12 POSIX -- 带上 → bash 完整透传
OK   ADV-15 AC-10 README+DEPLOYMENT 主推荐字串字节一致

===== Summary: 14 pass / 0 fail =====
```

### 3.3 关键 AC 覆盖

| AC | 验证手段 | 状态 |
|---|---|---|
| AC-1 / AC-2 / AC-3 / AC-14 / AC-16 | 真机 / docker 实证 | **延后**（设计已锁手工或留 follow-up；用户即将复测）|
| AC-4 / AC-5 / AC-6 / AC-7 / AC-8 / AC-9 / AC-10 / AC-11 / AC-12 / AC-13 / AC-15 | 自动 reproducer + verify_all | **全 PASS** |

---

## §4 03 Conditions 闭环

| Condition | 状态 |
|---|---|
| C-1（`--` 不可省警告 ≥2 处） | ✅ 落实 5 处：install.sh L14 注释 + L71-72 help 段 + L182 错误提示 + README.md L67 + DEPLOYMENT.md L48 |
| C-2（`--role X --role Y` last-wins 注释） | ✅ install.sh L46-L50 设计依据块明示；reproducer ADV-5 + ADV-9 实测 |
| C-3（QA 真机/docker sudo VAR=val 兜底字串实证） | ⏳ 留用户复测（无 docker 本机；06 §C-3 决策清晰：失败则触发 T-036 加 `--public-ip`）|
| C-4（02 §4.2 partition 字面失实纠正） | ✅ 04 "Design drift" 段一行话纠正：实际跨分区由 developer 主 agent 一次性处理 |
| C-5（verify_all 增量闸门"主推荐无 sudo -E 残留"） | ⏸ 本任务暂不加（06 §C-5 决策：双实现对账复杂，单独走 follow-up；当前 review checklist + reproducer 覆盖已充分）|

---

## §5 兼容性矩阵（实测/推断）

| 用户场景 | 修复前行为 | 修复后行为 |
|---|---|---|
| Ubuntu 22 / 旧 RHEL + 旧 env 入口 | 成功 | **成功**（FR-5 env 兼容路径 + ADV-10 实证） |
| Ubuntu 22 / 旧 RHEL + 新 CLI 入口 | N/A | **成功**（ADV-5/6/9 实证） |
| Ubuntu 24/26 LTS + 旧 env 入口 | **失败**（用户报告） | 仍失败，**但**错误提示 3 段精准引导用户改 CLI 形态（ADV-1 实证 stderr 包含 sudo `-E` 诊断引导）|
| Ubuntu 24/26 LTS + 新 CLI 入口 | N/A | **成功**（设计目标 FR-1 / FR-2 ✓；用户即将真机复测）|
| --role bogus / --role= 空值 / --unknown-flag | N/A | exit 3 / 3 / 1，中文错误（ADV-7 / 8 / 06 §S-3..S-6） |
| --help | 0 退 | **0 退**，新 help 段（ADV-3 实证无过时 sudo -E 表述）|

---

## §6 风险与跟进

### §6.1 已知短期风险（02 §6 / 03 / 06 §C-3 已记）

1. **过渡期**：老群文档 / 老 SO 答案 / 老博客里的旧 env 入口仍在传播，用户在 Ubuntu 24/26 上复用会失败——错误提示 3 段精准引导是最强缓解（命中"sudo `'-E' is ignored`"后用户**就在错误现场**拿到新命令文案）。
2. **公网 IP 兜底字串**：`sudo FRP_EASY_PUBLIC_IP=<ip> bash -s -- --role server` 形态在严格 sudoers `env_check` 下行为未在本任务内自动化实证；若用户实测失败 → 触发 T-036 加 `--public-ip` CLI flag。

### §6.2 后续 follow-up 任务建议

| 建议 ID | 标题 | 触发条件 | 优先级 |
|---|---|---|---|
| T-036 | install-sh-public-ip-cli-arg | 用户在 Ubuntu 24/26 LTS 上跑公网 IP 兜底命令失败（sudo VAR=val 被严格 sudoers 拒）| 触发再做 |
| T-037 | install-sh-verify-grep-gate | 加 verify_all step "主推荐字串无 sudo -E bash 残留" 的双实现对账（insight L26）| 任意 trivial 批次 |

---

## §7 Insight

- 2026-05-24 · 一键安装管道脚本"environment variable + `sudo -E` 透传"模式是脆弱契约：依赖 (1) shell 解析 `VAR=val cmd` 语法、(2) sudo 不剥离自定义 env（受发行版 sudoers `Defaults env_reset` + `env_keep` 白名单控制）、(3) bash 接收，三者任一失败即败。Ubuntu 24/26 LTS、Debian 13 等较新 sudo 默认拒绝 `-E` 透传（打印 `sudo: '-E' is ignored`）让链路 2 断裂。**根治路径不是争论 sudoers 配置**而是改信号通道：CLI 参数 `--role <value>` 走 bash `-s -- <args>` 位置参数透传，与 sudo 安全策略完全解耦。这是 rustup / k3s / docker / nvm 等业界主流一键安装脚本的共同 pattern——env-based 是 90s 风格、CLI-based 是当前主流。改 README 推荐入口时务必同步 install.sh 顶端注释 + help heredoc + 横幅"更新"段 + 错误提示段 4 处文案，并保留 env 形态作"兼容回退"（删除会破坏老用户 / 老群文档存量）· evidence: T-035 用户实测 Ubuntu 26 LTS curl: (23) Failure writing output；改后 reproducer.sh 14/14 PASS（含 ADV-1 错误文案 3 段 + ADV-10 env 兼容回退 + ADV-5/6/9 CLI 解析正确性）
- 2026-05-24 · `bash -s -- <args>` 中的 POSIX `--` 终止符是**强必要条件**：bash 5.x 实测 `echo cmd | bash -s --role server` 直接报 `bash: --: invalid option`（rc=2），脚本根本不执行；必须 `bash -s -- --role server` 让 bash 停止解析自身 option。这与 Windows `pwsh -NoExit -Command "..."`（insight L24）对称："管道脚本入口字串语法不可省字符"是另一类"主流 idiom 看起来冗余但绝对必要"的 pitfall。任何用 `curl ... | sudo bash -s -- <args>` 推荐入口的项目**必须**在 README + 错误提示中显式警告 `--` 不可省，否则用户复制粘贴漏字符时拿到的是 bash 英文 `invalid option` 报错（而非项目自己的中文诊断），用户体感断层 · evidence: T-035 reproducer.sh ADV-11/12 反向证伪 + install.sh L14 + L71-72 + L182 + README.md L67 + DEPLOYMENT.md L48 共 5 处显式警告
- 2026-05-24 · GR 03 conditions 中"WARN / 建议"类 finding 不阻塞 stage 4 启动，但**应该被 developer 主动消化**（即兴补注释、补警告、纠正失实描述）：T-035 03 C-1（`--` 不可省警告 ≥2 处）→ 04 落实 5 处远超下限；03 C-2（last-wins 注释）→ 04 设计依据块明示；03 C-4（02 §4.2 partition 失实）→ 04 "Design drift" 段一行话纠正而非简单 ack 接受。**好的 developer 不是"满足 C-1 下限"而是"在自然顺手时一并消化所有 C-N"**，这让 stage 5 code review 处于"几乎无需改"的状态而非要求 fix 循环。与 T-027 / T-031 已见"GR conditions → Dev 即兴消化 → Reviewer APPROVED 一次"的同款节奏对齐 · evidence: T-035 03 §5 五条 conditions + 04 §"Design drift" + 05 Verdict APPROVED 一次过

---

## §8 Verdict

**DELIVERED**

verify_all 26 PASS / 0 WARN / 1 FAIL (C.1 预存在基线漂移，stash 法独立证伪)；reproducer.sh 14/14 PASS；5 条 GR conditions 4 闭环 + 1 暂不加（设计依据清晰）；用户原报告的 Ubuntu 26 LTS 失败前态预期由新 CLI 形态命令一次成功——待用户真机复测后归档闭环。

— PM Orchestrator, 2026-05-24
