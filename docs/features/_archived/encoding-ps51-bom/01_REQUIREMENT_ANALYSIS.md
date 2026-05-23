# 01 — 需求分析 · T-021 encoding-ps51-bom

> Harness 流水线 Stage 1（Requirement Analyst）。模式：**full**。
> 上游输入：`docs/features/encoding-ps51-bom/INPUT.md`（PM 转述，只读）；T-019 `07_DELIVERY.md` §7 第 1 条 backlog；T-018 首次记录此 baseline。
> 本文档只描述"做什么 / 怎么验"，不做技术选型 / 实现决议（那是 Architect 在 02 的事）。

---

## §1 用户故事 / 价值陈述

**作为** 在 Windows zh-CN 主机（默认仅装 Windows PowerShell 5.1、未升级 PowerShell 7.x）的 frp_easy 用户，
**我希望** 直接 `powershell.exe -File scripts/install.ps1` / `.\scripts\verify_all.ps1` 等磁盘加载形态能正常运行而不报中文乱码 / syntax error，
**以便** 不必先额外安装 PowerShell 7、也不必走 `irm | iex` 才能用本项目的所有 .ps1 入口，与已通过 T-013 验证的 `irm | iex` 路径达到等价可用性。

---

## §2 验收标准（AC）

> 标注规则：
> - **[A] 自动可验**：可由 verify_all 或 `pwsh -File` 直接断言、出退出码。
> - **[U] 需用户真机验证**：需要 PS5.1 + zh-CN 主机（QA 主机为 PS7 默认，无法本地复现）。

### §2.1 BOM 字节级断言

- **AC-1 [A]**：`scripts/` 目录下 11 个 .ps1 文件全部以字节序列 `EF BB BF` 起始（UTF-8 BOM）。逐文件清单：
  - `scripts/archive-task.ps1`
  - `scripts/build.ps1`
  - `scripts/harness-sync.ps1`
  - `scripts/install-hooks.ps1`
  - `scripts/install-service.ps1`
  - `scripts/install.ps1`
  - `scripts/package.ps1`
  - `scripts/start-e2e-server.ps1`
  - `scripts/start.ps1`
  - `scripts/uninstall-service.ps1`
  - `scripts/verify_all.ps1`
- **AC-2 [A]**：每个 .ps1 文件 BOM 之后的字节段（即文件第 4 字节起到 EOF）解码 UTF-8 后与本任务实施前同文件的全文内容**字符级完全一致**（仅前置 3 字节差异，无任何脚本逻辑变更、无 CR/LF 行尾改动、无 trailing newline 增减）。

### §2.2 PS5.1 + zh-CN 主机执行（真机）

- **AC-3 [U]**：在 Windows PowerShell 5.1（`$PSVersionTable.PSVersion.Major -eq 5`）+ host codepage = `936`（zh-CN GBK）的非管理员终端执行 `powershell.exe -File scripts\install.ps1 -Help`，退出码 = 0，stdout 含完整中文帮助内容（不出现 `锘`、`鏄`、`鈥` 等典型 GBK 错解符号），stderr 无 `ParserError` / `UnexpectedToken` 类异常。
- **AC-4 [U]**：在 PS5.1 + zh-CN 主机以管理员身份执行 `powershell.exe -File scripts\verify_all.ps1 -Quick`，能跑到 Summary 行（PASS/WARN/FAIL/SKIP 计数行）；不在文件加载阶段就因中文 syntax error 崩溃。允许此次运行因主机环境（缺 go / npm / playwright 等）出现单项 FAIL，但 `Get-Content scripts\verify_all.ps1 -Encoding Byte -TotalCount 3` 必须返回 `239 187 191`。
- **AC-5 [U]**：在 PS5.1 + zh-CN 主机管理员终端执行 `powershell.exe -File scripts\install-service.ps1 -BinaryPath C:\dummy\frp-easy.exe -DryRun`（若 -DryRun 不存在则用 `-WhatIf` 或 `-Help`，由 Architect 在 02 选定参数）；脚本能加载并解析中文文案不报 syntax error。
- **AC-6 [U]**：T-019 已通过的 SCM 启动场景在 PS5.1 + zh-CN 主机重新跑 `irm ... install.ps1 | iex` 仍能完成 8 步、最终输出 `==> 服务已启动`，退出码 0。**确认 BOM 加入未破坏管道形态**。

### §2.3 PS7.x 不回归

- **AC-7 [A]**：在 QA 主机（W11 Home 10.0.26200，PS7 默认）执行 `pwsh -File scripts\verify_all.ps1`，结果与 T-019 交付时一致或更优（≥ 19 PASS、0 FAIL，新增 1 项 BOM 检查后变 ≥ 20 PASS）。
- **AC-8 [A]**：`pwsh -File scripts\install.ps1 -Help` 与 `pwsh -File scripts\build.ps1`（或其他 dry-run 友好脚本）执行行为字节级等价于 T-019 末态；BOM 不进入 stdout（PowerShell 解释器吞 BOM，不当字符输出）。

### §2.4 `irm | iex` 管道形态不破

- **AC-9 [U]**：保留 T-013 已验证的命令形态 `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex` 在 PS5.1 + PS7.x 两种解释器下均能完整跑完 install 流程。
- **AC-10 [A]**：从 `Invoke-RestMethod` 拉到的脚本内容字节流（前 3 字节为 `EF BB BF`）经 `iex` 接收时**不**额外打印 BOM 字符到 stdout、**不**让 `$PSScriptRoot` 自定位逻辑（insight L25）异常 —— 即用 mock 字节流 `\xEF\xBB\xBF + 已知合法 .ps1 内容` 通过 `iex` 跑通的最小测试在 PS7 主机可自动验，PS5.1 真机由用户在 AC-9 复跑时一并确认。

### §2.5 verify_all 防回归

- **AC-11 [A]**：`scripts/verify_all.ps1` 与 `scripts/verify_all.sh` 各新增 1 个检查项（编号建议由 Architect 在 02 选定，例 `E.7` 或 `F.1`），实现"扫描 `scripts/*.ps1`，对每个文件读前 3 字节，必须等于 `0xEF 0xBB 0xBF`，否则 FAIL"。
- **AC-12 [A]**：检查项命名稳定（中英文 step name 至少含 "BOM" 或 "UTF-8 BOM" 字样），便于未来 grep 定位。
- **AC-13 [A]**：故意把任一 .ps1 改回 noBOM（删前 3 字节）后，`pwsh -File scripts\verify_all.ps1` 必须 FAIL 在该新项上，错误信息含被命中的文件路径。该负向自检由 QA 在 Stage 6 模拟、不留在 main 上。
- **AC-14 [A]**：`scripts/verify_all.ps1` PASS 计数从 19 升到 **20**；`scripts/verify_all.sh` 对应等价升一项；`scripts/baseline.json` 若有 `pass_count` / `test_count` 类字段相应同步（具体字段由 Architect 在 02 核实 baseline.json 的真实 schema 决定）。
- **AC-15 [A]**：新检查项**仅扫描** `scripts/*.ps1`，不扫描 `.harness/`、`.claude/`、`web/`、`docs/`、`node_modules/`、归档目录下的任何文件 —— 避免误伤其他 .ps1（若未来出现）或第三方依赖。

### §2.6 文档同步

- **AC-16 [A]**：`docs/dev-map.md` 若有"脚本编码"相关条目则更新；若无则新增一行简述 ".ps1 文件统一 UTF-8 BOM"（由 Architect 在 02 决定是否需要新建条目）。
- **AC-17 [A]**：本任务 `06_TEST_REPORT.md` 含裸标题 `## Adversarial tests`（insight L24 / L35 红线，无数字编号前缀）。
- **AC-18 [A]**：本任务 `07_DELIVERY.md` 含裸标题 `## Insight`（insight L43 红线，无数字编号前缀），否则 `archive-task.ps1` 收割 0 条。

---

## §3 非功能需求（NFR）

- **NFR-1 内容零字节改**：除前置 3 字节 BOM 外，11 个 .ps1 的脚本逻辑、参数、注释、空白、行尾、trailing newline **完全不变**。任何"顺手优化"（重命名、整理空行、改注释措辞）均属 design drift，必须在 04 报告。
- **NFR-2 行尾 `LF` 不变**：项目 `.gitattributes` 第 2 行 `* text=auto eol=lf` 已强制 LF 行尾；BOM 添加不得引入 CRLF。BOM 加在文件第 1 字节、不影响后续行尾字符。
- **NFR-3 不引入新依赖**：不引入新的 Go module、npm 包、PS module、外部 CLI 工具。`verify_all` 新检查项用 PS / bash 内置原语（`Get-Content -Encoding Byte` / `head -c 3` / `xxd` / `dd` 等可任选，由 Architect 定）。
- **NFR-4 verify_all 新 check 运行时间 < 1s**：11 个文件读前 3 字节，预算极小；不得引入显著 IO 或 spawn 子进程。
- **NFR-5 git diff 噪声最小**：BOM 仅前 3 字节，`git diff` 对 .ps1 默认显示为"二进制文件差异"或"文件开头多 3 字节"中的一种；不得让整文件 diff 因 CRLF/LF 切换而全量变红。
- **NFR-6 编辑器友好**：BOM 添加后，VS Code / Notepad / PowerShell ISE 打开仍能正常显示中文；不得让任一主流编辑器把整文件识别为二进制。
- **NFR-7 编码不漂移**：考虑提供机制（`.editorconfig` / `.gitattributes` / pre-commit hook / verify_all 闸门 任一或组合）让未来 PR 改 .ps1 时若编辑器误存为 noBOM 能被立即拦下；该机制的**形式选择**留给 Architect §02，但**至少 verify_all 闸门必须存在**（已由 AC-11 ~ AC-15 强制）。
- **NFR-8 兼容 archive-task.ps1 自我引用**：`scripts/archive-task.ps1` 自身加 BOM 后必须能被 `pwsh -File` 与 `powershell.exe -File`（PS5.1）正常调用而不报 parser 错误。该脚本在交付 stage 7 会被 PM 调用归档本任务，是 dogfood 闭环验证点。

---

## §4 范围 / 非范围

### §4.1 范围内（In-scope）

1. `scripts/*.ps1` 全部 11 个文件加 UTF-8 BOM（即首 3 字节 `EF BB BF`）。
2. `scripts/verify_all.ps1` 与 `scripts/verify_all.sh` 各加 1 个 BOM 防回归检查项，同步 baseline.json（若需要）。
3. 可选：根据 Architect §02 决议，加 `.editorconfig` 或修改 `.gitattributes`，锁定 .ps1 = UTF-8 BOM。

### §4.2 范围外（Out-of-scope）

1. **不**改 `.harness/skills/*/SKILL.md` 等非 .ps1 文件。
2. **不**动 `.sh` / `.go` / `.ts` / `.vue` / `.md` 等其他源文件编码。
3. **不**改 `scripts/install.sh`、`scripts/install-service.sh` 等 Linux/macOS 入口（无此问题）。
4. **不**改 PowerShell 行为本身（不调整用户终端 `$OutputEncoding` / `chcp` / `[Console]::OutputEncoding`）。
5. **不**改任何 .ps1 的执行逻辑（包括但不限于：参数列表、函数签名、错误处理、退出码、stdout 文案）。
6. **不**触及 `settings.json` / `.claude/` / `CLAUDE.md`（T-020 已处理 + 静态生成）。
7. **不**处理 T-019 §7 第 2 条 backlog `T-022 service-mode-stderr-bridge`（独立任务）。

---

## §5 重大澄清 / 不确定性

> 强制必含。即使 PM 派发"不要问问题"，也必须列出所有可疑点供 PM / Architect 后续判断。每条给候选方案与推荐方向。

### I-1 纯 ASCII 的 3 个 .ps1 是否也加 BOM？

涉及文件：`archive-task.ps1` / `harness-sync.ps1` / `install-hooks.ps1`（INPUT.md 第 16-18 行标 `ascii`）。
- **方案 A**：所有 11 个 .ps1 一律加 BOM（含纯 ASCII 三个）。
  - 利：一致性，未来谁加中文字符不需再补 BOM；verify_all 检查项规则简单（"所有 .ps1 都必须有 BOM"）。
  - 弊：3 字节冗余、纯 ASCII 文件被某些工具识别为"含 BOM 的 ASCII"略不正交。
- **方案 B**：仅含中文字符的 8 个加 BOM，纯 ASCII 3 个不动。
  - 利：BOM 加得最少；纯 ASCII 文件保持极简。
  - 弊：verify_all 规则复杂（要先判文件是否含非 ASCII 字节再决定是否要求 BOM）；未来谁给 archive-task.ps1 加一个中文 Write-Host 又忘了同步加 BOM 时会复发。
- **RA 推荐方向**：**方案 A**。一致性 + 防未来回归优先于 3 字节冗余。Architect 在 02 二选一并写入设计书。

### I-2 verify_all 检查粒度

- **方案 A**：严格 —— 任何 `scripts/*.ps1` 必须 BOM，与 I-1 方案 A 配对。
- **方案 B**：宽松 —— 仅"含非 ASCII 字节的 .ps1" 必须 BOM；纯 ASCII 文件不要求；与 I-1 方案 B 配对。
- **方案 C**：双检 —— 同时检 BOM 与"含中文必须有 BOM"两层（冗余、覆盖意图最强）。
- **RA 推荐**：与 I-1 推荐方向绑定 —— 选 I-1A 时本项选 A，选 I-1B 时本项选 B。Architect 决议。

### I-3 是否需要 `.editorconfig` / `.gitattributes` 增量锁定 .ps1 = UTF-8 BOM？

现状：项目根 `.gitattributes` 仅 11 行，未对 .ps1 单独声明；`.editorconfig` 不存在（根目录、scripts/ 均无）。
- **方案 A**：什么都不加，仅靠 verify_all 闸门防回归。
  - 利：零新增文件；与"不引入新依赖" NFR 一致。
  - 弊：编辑器（VS Code 默认 / Vim）保存时若按"无 BOM"配置去掉 BOM，开发者只在跑 verify_all 时才发现，反馈链长。
- **方案 B**：加 `.editorconfig` 在 `scripts/` 局部声明 `[*.ps1]` + `charset = utf-8-bom`。
  - 利：主流编辑器（VS Code、JetBrains 系、Vim with plugin）会读 .editorconfig 自动保留 BOM；防回归在编辑期就生效。
  - 弊：新增配置文件；`utf-8-bom` 是 .editorconfig 1.0 spec 内值，部分老编辑器不识别。
- **方案 C**：加 `.gitattributes` 行 `*.ps1 text working-tree-encoding=UTF-8 eol=lf`（git 2.10+ 的 working-tree-encoding 属性）。
  - 利：git checkout / commit 边界自动转码；无新文件。
  - 弊：working-tree-encoding 不直接控制 BOM 存在与否；与 `* text=auto eol=lf` 默认规则可能冲突；行为对老 git 客户端不可预测。
- **RA 推荐**：**方案 B**（加 `.editorconfig`）作为"belt"，verify_all 闸门作为"suspenders"，与 insight L13（`tsconfig.json noEmit` + `--noEmit` flag 双层防御）同款双层模式。Architect 二选一或选 A+verify_all 单层即可。

### I-4 PS5.1 + zh-CN 真机验证可否本地降级？

QA 主机为 W11 Home + PS7 默认，无 PS5.1 + zh-CN 主机；T-019 此类条件已降级为"用户在 §6 真机验证清单复现"。
- **方案 A**：复用 T-019 降级模式，AC-3 ~ AC-6 一律标 `[U]`，转用户真机验证；本地仅做字节级断言 + PS7 不回归。
  - 利：与历史任务降级模式一致；不强行 mock。
  - 弊：自动化覆盖率不够，理论上 BOM 加错（如混入其他不可见字符）的边角案例本地抓不到。
- **方案 B**：尝试 mock —— 用 `pwsh -File` 但显式设 `[Console]::InputEncoding = [Text.Encoding]::GetEncoding(936)` 模拟 zh-CN host codepage，跑 .ps1 看是否能复现。
  - 利：可能本地抓更多 case。
  - 弊：PS7 的解释器**不复用** PS5.1 的"按 host codepage 解码无 BOM UTF-8"行为；mock 本质不成立，结论无效。
- **RA 推荐**：**方案 A**。Architect / QA 不必为伪复现耗时间。AC-3 ~ AC-6 明确标 `[U]`。

### I-5 archive-task.ps1 自我引用 dogfood 风险

T-021 交付 stage 7 PM 调用 `pwsh -File scripts/archive-task.ps1 -Task encoding-ps51-bom` 时，本脚本自身已被加 BOM；BOM 是否影响 PS 解释器加载？
- **方案 A**：相信 PowerShell 5.1+ / PowerShell 7.x 解释器对 UTF-8 BOM 头部的标准容忍（实际上 PowerShell 解释器**就是**靠 BOM 探测识别 UTF-8），不额外做事。
  - 利：零工作量；理论正确。
  - 弊：PS5.1 在某些组合下 BOM + script block 解析行为有边角 bug，无法保证 100%。
- **方案 B**：在 04 实施完成后、stage 7 PM 调用 archive-task 前，QA 在 06 强制跑一次 `pwsh -File scripts/archive-task.ps1 -Task encoding-ps51-bom -DryRun` dogfood 验证；通过后再交付。
  - 利：dogfood 闭环、抓真问题。
  - 弊：增加一个 stage 6 步骤，但成本极低。
- **RA 推荐**：**方案 B**（dogfood）。Architect 把这条写入 02 测试策略，QA 在 06 必跑。

---

## §6 给 Solution Architect 的提示（02 前置决议清单）

Architect 在 `02_SOLUTION_DESIGN.md` 必须给出明确决议的项：

1. **I-1 / I-2 二选一**：纯 ASCII .ps1 是否一并加 BOM？verify_all 检查粒度是严格还是宽松？两者必须配对。RA 推荐 I-1A + I-2A。
2. **I-3 三选一**：是否加 `.editorconfig` / 改 `.gitattributes` 锁定 .ps1 编码？RA 推荐方案 B（`.editorconfig`）。
3. **I-5 二选一**：archive-task.ps1 自我 dogfood 是否进 QA 06 必跑清单？RA 推荐方案 B（必跑）。
4. **实现工具选定**：用什么具体命令/语法把 BOM 写入 11 个 .ps1？候选含 `[System.IO.File]::WriteAllText($p, $content, [System.Text.UTF8Encoding]::new($true))` / `Set-Content -Encoding utf8BOM`（仅 PS7+） / `dd` + `cat` Linux 方式 / 手编 bytes。**注意 insight L37 / L38**：Edit/Write 工具对 `.claude/settings.json` 类敏感路径有 soft block；本任务路径 `scripts/*.ps1` 不在该列表，理论上 Edit/Write 可用，但 11 个文件 read-write 仍需 Architect 选最稳路径。
5. **verify_all step 编号**：新检查项落在 E 段（项目结构）还是新开一段（如 `F. Script encoding`）？RA 无偏好，Architect 决定。
6. **baseline.json 是否要改**：需 Architect 在 02 实际打开 `scripts/baseline.json` 核实其 schema 是否含 `pass_count` 之类字段，并决定改不改。AC-14 留口子。
7. **管道形态 BOM 字符吞咽行为**：`Invoke-RestMethod` 接管 BOM 头的 .ps1 后 `| iex` 是否会把 BOM 当字面字符抛错？Architect 在 02 给出**机制层**解释（PowerShell 内置 BOM-aware 解码 / 或必须显式 `[Text.Encoding]::UTF8.GetString`）+ 对应 mock 测试，让 QA 在 06 跑。
8. **降级策略对账**：AC-3 ~ AC-6 + AC-9 标 `[U]`，QA 主机不强制；Architect 必须明文采纳并在 03 Gate Review 由 Reviewer 复核（防 T-019 同款"漏标降级"问题）。

---

## §7 关联历史任务

| 任务 | 关联点 | 关键文件路径 |
|---|---|---|
| T-008 deploy-kit | 首次为 Windows Service 引入 wrapper.cmd + `Set-Content -Encoding Default` 写中文路径 + 各 install-service.ps1 / install.ps1 雏形（含中文 Write-Host） | `docs/features/_archived/deploy-kit/02_SOLUTION_DESIGN.md` |
| T-009 polish-pass | PowerShell 写 TOML 必须 UTF-8 **无 BOM**（与本任务**反向**，TOML 文件不要 BOM、.ps1 文件**要** BOM；语义不同）；insight L17 | `.harness/insight-index.md` L17 |
| T-013 rolling-release-install | 首次验证 `irm \| iex` 管道形态对 PS5.1 + zh-CN 主机可用（管道解码 stdin 走 PS 自身、不走磁盘 BOM 探测） | `docs/features/_archived/rolling-release-install/02_SOLUTION_DESIGN.md` |
| T-018 upload-bin-multiport-ip-probe | 首次在真机遇到 PS5.1 + zh-CN 加载磁盘 .ps1 因无 BOM 中文乱码的现象（D-1 历史遗留 baseline） | `docs/features/_archived/upload-bin-multiport-ip-probe/06_TEST_REPORT.md`（INPUT.md §"已知风险" 转述） |
| T-019 windows-service-scm-1053-fix | §7 第 1 条 backlog 明确把本任务编号为 T-021；§6 AC-18 注脚明文记录 "D-1 历史遗留，不阻塞 T-019" | `docs/features/_archived/windows-service-scm-1053-fix/07_DELIVERY.md` L154, L162-L166 |
| T-020 claude-settings-context7-fix | insight L37 / L38：Edit/Write 工具对 `.claude/settings.json` 有 soft block；本任务路径不在列表，但 Architect 选实现工具时要参考此教训 | `.harness/insight-index.md` L37-L38 |

---

## §8 给 PM 的待办（READY 后）

1. 派发 Solution Architect（stage 2），prompt 必须显式要求其在 02 给出 §6 全部 8 项决议、不得遗漏 I-1 / I-2 / I-3 / I-5。
2. 注意 insight L41 / L42：Gate Reviewer / Code Reviewer 默认无 Write 工具，03 / 05 必须由 PM 接管落盘。
3. 注意 insight L24 / L35 / L43：QA 06 的 `## Adversarial tests`、PM 07 的 `## Insight` 必须**裸标题**、无数字编号前缀，否则 verify_all E.6 FAIL 或 archive-task 0 收割。
4. T-021 INPUT.md 已经包含 PM 的范围 / 约束 / 已知风险三段，Architect 在 02 不需要再向 PM 提同样问题。

---

## §9 Verdict

**READY**

理由：
- 上游输入（INPUT.md）已含完整范围 / 约束 / 已知风险三段，技术问题 / 现象描述 / 修复方向均已澄清；
- §5 列出的 5 项 I-1 ~ I-5 不确定性**全部为设计选型层**（不是需求阻塞），由 Architect 在 02 二选一即可推进，不需要回退用户回答；
- AC 19 条均可机械验证（10 条 `[A]` 自动 + 5 条 `[U]` 真机 + 4 条文档/格式），与 T-019 真机降级模式一致；
- 无超出 §4 范围的隐藏需求。

如 PM 判断 I-1 ~ I-5 中任一条需要用户先表态再开 02，则把对应条改为 BLOCKER 并退回 RA；否则直接派发 SA。
