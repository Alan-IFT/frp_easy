# Code Review · T-035 install-sh-role-cli-arg-passthrough

> Harness 流水线 Stage 5（Code Reviewer）。模式：**full**。
> 上游：01（READY）/ 02（READY）/ 03（APPROVED WITH CONDITIONS）/ 04（READY FOR REVIEW）。
> 独立视角：Reviewer 不读 04 §"Stage 4 内自检矩阵"再复述，而是从空白页角度重做代码 + 文档逐行 + 跨文件一致性 + 边界/安全/性能/可维护性维度审视。

## Files reviewed

- `scripts/install.sh`（修改前 → 修改后逐行 diff，重点 L1-L200 步骤 0~0.5，L289-L292 步骤 1 root 检查，L484-L491 步骤 6.5 force role 路径，L615 / L659 / L674 步骤 8 横幅）
- `README.md` L51-L103（一键安装段）+ L96（公网 IP 兜底命令）
- `docs/DEPLOYMENT.md` L36-L94（A.0 一键安装段）
- 02_SOLUTION_DESIGN.md §3 全部小节（逐项核对代码落地）
- 03_GATE_REVIEW.md §5 全部 conditions（C-1 ~ C-5）

无单元测试或 spec 文件——本任务范围是 bash 脚本 + Markdown 文档，无 Go/Vue 代码变动；测试覆盖由 QA Stage 6 在真机/docker 实证完成（设计上已在 01 NFR-5 + 03 C-3 锁定）。

## Findings

### CRITICAL

无。

### MAJOR

无。

### MINOR

- [LOGIC] `scripts/install.sh:120` `[[ $# -lt 2 || "$2" == --* ]]` 在 `set -euo pipefail` 下当 `$# < 2` 时不会因 `$2` 未定义触发 unbound variable error —— bash `[[ ]]` 内 `||` 是**短路评估**（实测 bash 5.2.37：`bash -c 'set -u; [[ 1 -lt 2 || "$xyz" == foo ]] && echo ok'` → `ok`，不报错）。设计依赖此语言语义；但**Reviewer 建议**附一行注释明确"依赖 bash `[[ ]]` 短路评估，set -u 安全"，便于未来 maintainer 不误改成 `[ ... || ... ]`（外部 test 不短路 +`set -u` 下会失败）。本项不阻塞，但 Stage 7 PM/Developer 可选择即兴补注释。
- [MAINT] `scripts/install.sh:185` 错误提示"兼容用法"段只列了 1 条 `client` 示例命令，而上方"推荐用法（CLI 形态）"段列了 server + client 两条。读者代入"我是 server 用户" 时只能从 CLI 段拷贝，从 env 段需要心算把 `client` 换成 `server`——这是设计意图（降级显示）还是疏漏？建议在文档（README L67 已有兼容回退说明）下方注释一行"兼容路径单条示意；server / client 自行替换"或补成 2 条。MINOR 不阻塞，可由 QA 在 06 反馈用户体感后决定。
- [MAINT] `scripts/install.sh:291` step 1 root 检查的"推荐用法"行：`curl -fsSL <url> | sudo bash -s -- --role ${ROLE}`，`<url>` 是字面占位符不是真链接。从用户视角，他们刚好通过管道用了脚本，理应已经知道 url；但失败现场（id != 0）多半是用户**没**用一键命令而是**手动**下载然后 `bash install.sh` 在非 root 下跑——这时 `<url>` 占位符对用户**确实**有教育意义（指引正确的一键命令）。Reviewer 不主张改，但记录此设计选择。

### NIT

- [STYLE] `README.md:67` + `README.md:69` 两段都包含 `... | FRP_EASY_ROLE=client sudo -E bash` 字串，语义部分重叠（前者"兼容回退说明"+后者"旧入口失败引导"）。可考虑合并为单段三行（说明 + 字串 + 失败引导）减少视觉重复；当前分开亦清晰。
- [STYLE] `scripts/install.sh:160` 当 `ROLE_FROM_CLI` 为空且 `FRP_EASY_ROLE` 也未设时，`ROLE_SOURCE="环境变量 (FRP_EASY_ROLE)"` 但实际 ROLE 空。后续 L176 校验立即走 exit 3 不会打印 L193 `role=... (来源: ...)` 行，故不构成 user-facing bug。但变量值"逻辑不真"在严格代码风格下可改为 `if [[ -n "$ROLE" ]]; then ROLE_SOURCE="环境变量 (FRP_EASY_ROLE)"; else ROLE_SOURCE="(未设置)"; fi`——纯洁癖。

## Requirement coverage check（对应 01 §5 AC）

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 | `scripts/install.sh:115-126` (`--role` case) + L155-161（ROLE 归一化 → CLI 优先）；用户真机/docker 实证延后到 QA Stage 6 | ⚠️ 静态实施 ✓；真机实证 → QA |
| AC-2 | 同 AC-1，server role 同款解析 | ⚠️ 同 AC-1 |
| AC-3 | `scripts/install.sh:659`（步骤 8 IP 探测失败兜底字串）+ README L96 + DEPLOYMENT L54 | ⚠️ 静态字串 ✓；sudo `VAR=val cmd` 在严格 sudoers 下行为 → QA AC-16 |
| AC-4 | `scripts/install.sh:136-138`（`--force-role` case）+ L163-167（FORCE_ROLE_EFFECTIVE 归一化）+ L487 引用切换；冲突提示 L491 给完整 `--force-role` CLI 形态 | ✅ 静态 |
| AC-5 | `scripts/install.sh:155-161` env 兼容回退分支；S-11 矩阵实测 | ✅ |
| AC-6 | `scripts/install.sh:176-192` 3 段错误文案（CLI / env / sudo `-E` 诊断）+ `--` 不可省警告 | ✅ |
| AC-7 | `scripts/install.sh:615`（client 横幅）+ `scripts/install.sh:674`（server 横幅） | ✅ |
| AC-8 | 实测 `grep -nE 'sudo -E bash' scripts/install.sh README.md docs/DEPLOYMENT.md` 命中 7 处，全部位于设计中"兼容回退"段（详见 04 §"Stage 4 内静态闸门"）；无主推荐路径污染 | ✅ |
| AC-9 | `scripts/install.sh:57-112` help 段重写；包含 `--role` / `--force-role` / `--` 警告 / 兼容用法 | ✅ |
| AC-10 | `README.md:59,62,96` + `docs/DEPLOYMENT.md:44,47,54` 字符串字节级一致；grep 跨文件确认 | ✅ |
| AC-11 | 无新增 wrapper 文件；`ls scripts/install*.cmd scripts/install*.bat` 实测零命中 | ✅ |
| AC-12 | Stage 4 S-3..S-10 矩阵在本地实测覆盖（`--role` / `--role=` / `--unknown-flag` / last-wins / CLI vs env 优先） | ✅ |
| AC-13 | `scripts/install.sh:36` `set -euo pipefail` 不变；Reviewer 静态验证 case 分支均不破坏严格模式（含 MINOR L120 短路安全） | ✅ |
| AC-14 | macOS bash 3.2 兼容性：`[[ "$2" == --* ]]` 与 `${1#--role=}` 在 bash 3.2 均合法语法；具体实证 → QA 手工 | ⚠️ 静态 ✓；真机 → QA |
| AC-15 | `scripts/install.sh:187-188` 诊断指引段（"如果你刚才看到 sudo 输出 '-E' is ignored ..."） | ✅ |
| AC-16 | docker `ubuntu:24.04` / `ubuntu:26.04` 自动化实证 → QA Stage 6 | ⚠️ → QA |

## Design fidelity check（对应 02 §3）

| Design item | Implementation | Status |
|---|---|---|
| §3.1 CLI 参数解析骨架（`--role` + `--role=*` + `--force-role` + `--`）| `scripts/install.sh:115-144` | ✅ 字节级吻合 |
| §3.2 ROLE 校验新文案（3 段 + sudo `-E` 诊断）| `scripts/install.sh:176-192` | ✅ |
| §3.3 FORCE_ROLE_EFFECTIVE 合并 + L406 提示更新 | `scripts/install.sh:163-167` + L487 + L491 | ✅ |
| §3.4 help 段文案 | `scripts/install.sh:57-112` heredoc | ✅ |
| §3.5 step 8 横幅 3 处字串 | client `scripts/install.sh:615` + server `scripts/install.sh:674` + IP 兜底 `scripts/install.sh:659` | ✅ |
| §3.6 顶端注释 | `scripts/install.sh:8-32` | ✅ |
| §3.7 README 同步 | `README.md:57-72` + L96 + L103 | ✅ |
| §3.8 DEPLOYMENT 同步 | `docs/DEPLOYMENT.md:41-55` + L65-68 + L94 | ✅ |
| 03 C-1（`--` 不可省警告）| `README.md:67` + `docs/DEPLOYMENT.md:48` + `scripts/install.sh:14` + L71-72 + L182 共 5 处 | ✅ 远超 03 C-1 要求的"4 处中至少 2 处" |
| 03 C-2（last-wins 注释）| `scripts/install.sh:46` "`--role X --role Y` 行为 = "last wins"" | ✅ |
| 03 C-3（QA 真机/docker 实证）| Stage 6 未到，留给 QA | ⏳ Stage 6 处理 |
| 03 C-4（02 §4.2 partition 描述失实纠正）| `04_DEVELOPMENT.md` "Design drift" 段一行话纠正 | ✅ |
| 03 C-5（verify_all 增量闸门可选）| 由 QA 决定；本任务未引入 | ⏳ Stage 6 决定 |

## Cross-file consistency check（Reviewer 独立 grep）

跨文件主推荐 CLI 字符串实测命中：

```
scripts/install.sh:11,13,68,69,180,181,291,491,615,674   (10 处)
README.md:59,62                                          (2 处)
docs/DEPLOYMENT.md:44,47                                 (2 处)
合计 14 处主推荐 CLI 字串（均含 `sudo bash -s -- --role <role>`）
```

公网 IP 兜底字符串：`scripts/install.sh:659` + `README.md:96` + `docs/DEPLOYMENT.md:54` 三处字节级一致。

`sudo -E bash` 残留全部位于设计中保留的"兼容回退"段（共 7 处），无主推荐路径污染。

## 安全维度

- ✅ 无新增 `eval` / `exec` 类危险结构
- ✅ ROLE 取值仍严格匹配 `server` / `client`（L176-192 校验）
- ✅ `--role=$VAL` 等号形态用 `${1#--role=}` 参数展开，无 shell 注入
- ✅ `$2` 在缺 value 路径被短路保护，set -u 安全
- ✅ FORCE_ROLE_EFFECTIVE 取值严格 yes/no，无第三态
- ✅ ROLE_SOURCE 仅用于 stdout 进度行展示，无敏感信息泄漏

## 性能维度

不适用——本任务无热路径循环 / I/O / 内存分配变化。CLI 解析在 while 循环里至多遍历 6 个 token（用户合理输入），O(N) 与脚本启动成本相比可忽略。

## 维护性维度

- ✅ 注释 explain WHY（L44-50 注释块解释设计依据）
- ✅ 设计依据引 02 §3.1 / 03 Q-2 章节锚
- ✅ `--` 不可省警告在源码 + 注释 + help + README + DEPLOYMENT 总共 5 处显式
- ✅ 兼容路径与主推荐路径在文档中按"前者降级显示、后者首推"清晰分层
- ✅ MINOR `set -u` 短路注释建议（可选补）

## Verdict

**APPROVED**

无 CRITICAL / MAJOR；3 条 MINOR + 2 条 NIT 全部不阻塞 merge。Developer 可选在 Stage 7 PM 整合时即兴补 MINOR-1 短路注释（一行），不强制。03 C-3 / C-5 由 QA Stage 6 接手验证。

— Code Reviewer, 2026-05-24
