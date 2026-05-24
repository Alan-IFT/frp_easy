# 03 — 闸门评审 · T-035 install-sh-role-cli-arg-passthrough

> Harness 流水线 Stage 3（Gate Reviewer）。模式：**full**。
> 上游：01_REQUIREMENT_ANALYSIS.md（READY）+ 02_SOLUTION_DESIGN.md（READY）。
> 本文档的产出 = Verdict + 8 维度审计 + 开发期高概率问题预答。**不**对上游做任何修改。

---

## §1 8-维度审计

| # | 维度 | 结论 | 一句话理由 |
|---|---|---|---|
| 1 | Requirement completeness | **PASS** | 01 §3 FR-1..18 全部测试可验证；OQ 全部 `[PM-resolved]` 收敛；AC 1-16 与 FR 一一对应。 |
| 2 | Design completeness | **PASS** | 02 §3 拆解到行级；§9 AC-coverage 表覆盖 01 全部 16 条 AC。 |
| 3 | Reuse correctness | **PASS** | 02 §4.1 Reuse audit 引用的 6 个现有代码点全部实测存在（L38-93、L101-107、L400、L508-596、L42-85、stderr 中文 idiom）。 |
| 4 | Risk coverage | **WARN** | 02 §6 R-1..R-12 覆盖了主要风险；但 GR 独立实测发现 2 个未覆盖的风险（F-1 / F-4，见 §2）。 |
| 5 | Migration safety | **PASS** | 02 §7 无 schema 变更；§7.3 兼容矩阵显式列出"修复前 vs 修复后"所有用户场景结果；rollback 可走 commit SHA 锚定。 |
| 6 | Boundary handling | **WARN** | 02 §3.1 case 解析对 `--role` 缺 value / 等号空值 / 未识别 flag 都设计了；但 §3.5 `sudo VAR=val cmd` 形态在严格 sudoers `env_check` 下的边界未审视（F-2，见 §2）。 |
| 7 | Test feasibility | **WARN** | AC-1..AC-15 可执行；AC-14 macOS 无 CI runner 标手工——既有 T-025 / T-031 同款做法；AC-16 docker 自动化提供 NFR-5 兜底。但 AC-12 中"`--role server --role client`"行为未在 02 §3.1 显式定义（F-3，见 §2）。 |
| 8 | Out-of-scope clarity | **PASS** | 01 §6 OOS-1..8 明确；02 §8 扩展了"不抽 RECOMMENDED_INSTALL_CMD 常量""不动 install-service.sh"等设计层面的 OOS。Developer 不会跑偏。 |

---

## §2 Findings（每条点名上游责任章节）

### F-1（WARN，归 02 §6 风险登记缺漏）

**事实**：02 §3.7 README 新字串 + §3.8 DEPLOYMENT 新字串 + §3.5 install.sh 横幅字串都依赖 `bash -s -- --role <value>` 形态。GR 在 Git Bash bash 5.2.37 上实测验证：

```text
# 正确（有 --）：
$ echo 'echo "args=$@"' | bash -s -- --role server test
args=--role server test       (rc=0)

# 错误（漏 --）：
$ echo 'echo "args=$@"' | bash -s --role server test
bash: --: invalid option      (rc=2)
（脚本根本不执行）
```

**含义**：用户复制粘贴时若漏 `--`，bash 直接 rc=2 失败，install.sh 根本不进入，错误信息来自 bash 自身（英文 + GNU long options 列表），与本项目"中文可复制粘贴错误"风格完全不一致——用户大概率不知道是哪里错了。

02 R-2 已提此风险并写"README + 注释中显式写明 `--` 不可省"，但 02 §3.7 / §3.8 / §3.4 / §3.6 的文案 diff 中**没有落实**这条警告——README 仅给字串本身，未加"`bash -s -- --role` 中的 `--` 不可省，省了 bash 会 invalid option"提示。

**影响**：用户体感降级；过渡期用户报告增加。

**归属**：02 §3.7 / §3.8 / §3.4 / §3.6 应在 README / DEPLOYMENT / `--help` / 注释 4 处中至少 1 处加显式警告。Developer 实施时应附带处理。

### F-2（WARN，归 02 §6 / §3.5 风险覆盖不全）

**事实**：02 §3.5 公网 IP 兜底新字串：

```bash
curl ... | sudo FRP_EASY_PUBLIC_IP=<your-ip> bash -s -- --role server
```

`sudo VAR=val cmd` 形态在 `sudoers(5)` 默认配置下的语义：
- 若 `VAR` 在 `env_check` 白名单中（且 value 通过无 shell 元字符校验）→ 允许设置
- 若 `VAR` 不在 `env_check` 列表 → **受白名单限制**，sudo 可能拒绝（具体取决于发行版默认 `sudoers` 文件）

`FRP_EASY_PUBLIC_IP` 显然不是 sudoers 默认 `env_check` 中的标准变量（`LANG` / `LC_*` / `TERM` 等）。Ubuntu 24/26 LTS 默认 `/etc/sudoers` 是否拒绝 `FRP_EASY_PUBLIC_IP=<ip>` 形态需 QA 实测——02 §6 R-7 仅笼统提到"docker 与真机一致性"，未具体到此字串。

**含义**：本任务**主推荐字串**（普通 install）解决了 `-E` 失败；但**兜底字串**可能仍受 sudoers 限制。

**影响**：兜底场景（用户报"公网 IP 探测失败"）下仍可能再次失败，需用户走"先 export 再 sudo"或类似变通——破坏 NFR-1 "一条命令即装"。

**归属**：02 §3.5 应至少补一条 QA 实证要求（AC-3 已含但需明确"在严格 sudoers 默认配置下复现"）；或考虑兜底路径加 `--public-ip` CLI flag（与 01 OQ-7 b 的"OOS"决策有冲突，但 GR 不主张方案变更——仅 flag 给 PM）。

**优先建议**：Developer 实施时不动 §3.5 兜底字串（保持 OQ-7 b 默认 OOS），QA 在 AC-3 中显式跑 Ubuntu 26 LTS 真机或 `ubuntu:24.04` docker 复现并记录结果；若实测失败，则后续新开任务（T-036）增 `--public-ip`。

### F-3（WARN，归 02 §3.1 行为未定义）

**事实**：02 §3.1 case 解析未显式说明用户输入 `--role server --role client` 时的行为。按 bash case 顺序 shift 逻辑，最后一次出现的 `--role` 值生效（"last wins"）。这与 02 R-3 "help 总是赢" 不同——R-3 是 help 与 role 混用；本 finding 是同款 flag 重复出现。

**含义**：与 GNU long option 主流"last wins"语义一致；但 02 / 01 未明示。AC-12 列了此用例但缺乏期望值定义。

**归属**：02 §3.1 应在注释中明确"`--role X --role Y` 后者生效"；developer 实施时一条注释解决；AC-12 期望值 = "client（last wins）"。

### F-4（WARN，归 02 §4.2 partition 分配描述失真）

**事实**：02 §4.2 写"项目使用单 Developer 模式（`.harness/agents/dev-*.md` 仅有 `developer.md`、无 frontend/backend 分区 agent）"。GR 实测：

```
.harness\agents\dev-db.md
.harness\agents\dev-backend.md
.harness\agents\dev-frontend.md
```

事实是项目**有** dev-db / dev-backend / dev-frontend 三个分区 agent。本任务改的是 `scripts/install.sh` + 两份 Markdown 文档，**不属于** frontend / backend / db 任一分区——属于 "运维/脚本"（dev-ops 类，项目未设此分区）。

**含义**：02 §4.2 字面失实；但**实际影响为零**：
1. insight L31-L34 / L38 已记录 PM 派发上下文 SDK 工具裁剪让 7-stage 角色 collapse 到 PM 自演——分区分给谁实际都是 PM 跑
2. 即便真派发，按"运维脚本归 dev-backend"最近邻原则可路由

**归属**：02 §4.2 的描述失实**不阻塞**实施；Developer 阶段在 04_DEVELOPMENT.md 中可以一行话纠正"实际跨 dev-backend / 文档维护边界，归 developer 主 agent 一次性提交"。GR 不要求架构师改 02。

### F-5（INFO，归 02 §6 R-7 缓解充分性提示）

**事实**：02 §6 R-7 "docker ubuntu:24.04/26.04 sudoers 与 LTS 真机行为可能不同"。GR 同意此风险存在，且用户已提供 Ubuntu 26 LTS 真机失败前态作直接证据——本任务 modification（CLI 形态）的**正向证伪**仍需在用户真机或 docker 实证。QA 已在 AC-16 加 docker 自动化；建议 QA 在 06 中**同时**给 docker 复现 + 文字描述 + 期望用户真机复测的请求。

**含义**：F-5 是 INFO（不阻塞）；缓解充分性由 QA 在 06 决定。

---

## §3 高概率开发期问题（预答）

### Q-1：`--role` 与 `--role=` 两种形态的 case 顺序？

**Pre-answer**：必须**先** `--role=*` 后 `--role`（02 §3.1 已显式排序）。bash case 按文件顺序匹配，`--role` glob 不会匹配 `--role=server`（无尾通配），但显式排序更安全。

### Q-2：FORCE_ROLE_EFFECTIVE 变量何时初始化？

**Pre-answer**：在 while loop 解析完毕后、§3.3 步骤 6.5 校验前。即 install.sh L93（while loop 结束）之后、L100 ROLE 校验之前的位置。Developer 实施时建议把 `FORCE_ROLE_EFFECTIVE="..."` 与 ROLE 合并放在一个"参数解析后归一化" 注释块下。

### Q-3：02 §3.6 顶端注释 vs §3.4 help 段 vs §3.7 README 三处文案是否完全一致？

**Pre-answer**：必须完全一致（字串相同位置可微调换行 / 缩进，但**命令本身**字节级一致）。Developer 在实施时用 grep 锚定：

```
grep -nE 'bash -s -- --role' scripts/install.sh README.md docs/DEPLOYMENT.md
```

应命中至少 8 次（脚本 5 处：注释 2 + help 4 + 横幅 client 1 + 横幅 server 1 + 错误提示 2；README 3 处；DEPLOYMENT 3 处）。逐一核对一致。

### Q-4：bash `--` 终止符是否要在 README 字串中加注释解释？

**Pre-answer**：按 F-1 finding：README + DEPLOYMENT 文档需要在字串下方加一行"`bash -s --` 中的 `--` 是 POSIX 参数终止符，省去后 bash 会把 `--role` 当自己的非法选项报错——不可省"。Developer 实施时补此一行。

### Q-5：兼容回退路径 `FRP_EASY_ROLE=server sudo -E bash` 是否还在 help / README 中显式列出？

**Pre-answer**：是。01 OQ-2 b 默认（保留兼容回退路径），02 §3.2 + §3.4 + §3.7 + §3.8 都明示。Developer 在实施时**不要删除** env 形态文案，只把它**降级**为 "兼容用法（环境变量形态，仅在 sudo 允许 env 透传的发行版上有效）" 副推荐——这样既保护老用户 / 老文档 / 老 SO 答案，又通过排序 + 标签清晰引导新用户走 CLI 形态。

### Q-6：CI 滚动发布与 raw.githubusercontent.com 同步窗口？

**Pre-answer**：02 §6 R-8 已答：README 推荐入口走 `raw.githubusercontent.com/.../main/scripts/install.sh` 直接拉 main，commit push 后 raw 即生效（无需等 CI）。Developer 实施时**不必**等 CI 滚动发布即可让新入口生效——但 release artifact 内的 install.sh 仍是 push 时的 main 快照，**下一次 CI 后**才更新。

### Q-7：`verify_all` 是否要新增 grep 闸门验证"主推荐字串无 `sudo -E bash` 残留"？

**Pre-answer**：建议加。AC-8 已列此 grep 命令，但 verify_all 现行未含。Developer / QA 协作：QA 在 06 中给出 verify_all 增量；若 PM 同意，并入本任务。**GR 不强制**——同款风险也可通过 review checklist 而非闸门控制。

### Q-8：错误提示中文行序顺序？

**Pre-answer**：02 §3.2 已固定 3 段顺序：CLI 推荐 → env 兼容 → sudo `-E` 诊断。理由：用户先看到主推荐（80% 用户问题解决），扫不到再看兼容（10%），再扫不到才看诊断（剩余 10%）。Developer 直接按 §3.2 字节级实施。

---

## §4 影响 / 不影响 verify_all 现行闸门检查

verify_all 现有 step 与本任务的预期关系：

| step | 检查项 | 本任务影响 |
|---|---|---|
| build / test | go build / go test / vitest / playwright | **不影响**（仅改 bash + Markdown）|
| E.4 中文 lint | install.sh 中文文案 | **不影响**（保持中文 idiom）|
| E.6 Adversarial tests 段 | QA 06 必须含 `## Adversarial tests` 段 | **必须遵守**（与既有 T-031 / T-027 / T-025 一致）|
| E.7a / E.7b BOM 黑白名单 | scripts/install.sh shebang + 无 BOM | **不影响**（install.sh 本就无 BOM）|
| E.10 install.ps1 推荐字串闸门（T-031） | install.ps1 PowerShell 推荐字串 `-NoExit` | **不影响**（本任务不动 install.ps1）|

无新增 verify_all 闸门强制要求，但 **Developer 可选**：考虑加一条 `grep -nE 'sudo[[:space:]]+-E[[:space:]]+bash' README.md docs/DEPLOYMENT.md scripts/install.sh` 在主推荐段不应命中的闸门——若加，需在 verify_all.sh + verify_all.ps1 两侧对称实现（insight L26 双实现对账）。Q-7 未强制。

---

## §5 Verdict

**APPROVED WITH CONDITIONS**

5 条 conditions（developer / QA 实施期附带处理，不阻塞 stage 4 启动）：

1. **C-1（必）**：Developer 在 README / DEPLOYMENT / `--help` / install.sh 顶端注释 4 处中至少 2 处**显式警告** `bash -s -- --role` 中的 `--` 不可省（F-1）。
2. **C-2（必）**：Developer 在 02 §3.1 实施时**加注释**说明 `--role X --role Y` 行为为"last wins"（F-3）；AC-12 期望值在 06 中明确。
3. **C-3（必）**：QA 在 AC-3 / AC-16 中**显式跑** Ubuntu 24/26 LTS 真机或 docker，验证 `sudo FRP_EASY_PUBLIC_IP=<ip> bash -s -- --role server` 兜底字串在严格 sudoers 默认配置下的实际行为；若失败，记 06 Adversarial tests + 提建议（可能触发 follow-up T-036 加 `--public-ip` CLI flag）（F-2）。
4. **C-4（建议）**：Developer 在 04_DEVELOPMENT.md 一行话纠正 02 §4.2 "单 developer 模式"描述失实——实际 `.harness/agents/dev-*.md` 存在 3 个分区 agent，本任务跨分区由 developer 主 agent 一次性处理（F-4）。
5. **C-5（可选）**：QA 在 06 中评估是否要加 verify_all 增量闸门"主推荐字串无 sudo -E bash 残留"，与 insight L26 双实现对账原则联动（Q-7）。

— Gate Reviewer, 2026-05-24
