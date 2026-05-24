# 01 — 需求分析 · T-026 install-ps1-iex-bom-and-host-exit-fix

> Harness 流水线 Stage 1（Requirement Analyst）。模式：**full**。
> 上游输入：`docs/features/install-ps1-iex-bom-and-host-exit-fix/PM_LOG.md` §"任务起源 / PM 根因初判"（PM 转述，只读）；
> 用户报告时间 2026-05-23；
> PM 已确认 E1（BOM → ParserError）+ E2（`exit N` 在 iex 形态杀宿主）双根因，本 RA 直接采信。
> 本文档只描述"做什么 / 怎么验"，不做技术选型 / 实现决议（那是 Architect 在 02 的事）。

---

## §1 用户故事 / 价值陈述

**作为** Windows 10/11 终端用户（管理员身份打开 PowerShell 5.1 或 7.x、zh-CN 默认 code page 936），
**我希望** 执行单条命令 `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex` 时：
1. 终端不出现任何首部解析错误噪声（不应有 `is not recognized as a name of a cmdlet` 类红字），
2. 脚本运行结束（成功或失败）后宿主 PowerShell 会话**仍然存活**，让我能继续阅读最后输出与执行 `sc query frp-easy` 等验证命令，
**以便** 我能确认安装到底是成功还是失败，而不是看着突然关闭的窗口怀疑系统状态。

---

## §2 背景

本任务是 install.ps1 一键安装路径在 T-021、T-024 之后的**第三次修复**。前两次共同关心 iex 管道形态的解析与执行模型，但都未触及本任务两个根因：

| 任务 | 关注点 | 与本任务关系 |
|---|---|---|
| T-021 encoding-ps51-bom | 给所有 `scripts/*.ps1` 加 UTF-8 BOM，修 PS5.1+zh-CN **磁盘形态**的中文 GBK 误解码 | 直接制造了本任务 E1：BOM 在 iex 管道形态下经 `Invoke-RestMethod` 解码为 U+FEFF 字符进入字符串，让 `iex` parser 把首字符当 cmdlet 名 |
| T-024 install-ps1-iex-cmdletbinding-fix | 删 `[CmdletBinding()]` 修 iex 形态 `Unexpected attribute 'CmdletBinding'` ParserError | 同一文件（install.ps1）同形态（iex）的解析层修复；本任务是它发现的 ParserError 之后**另一组**未覆盖的 ParserError + 一个执行模型问题 |

本任务的范围是**仅** `scripts/install.ps1` 这一个 iex 入口文件；`scripts/install-service.ps1` 与 `scripts/uninstall-service.ps1` **不在范围**（它们永远是磁盘形态、被 `frp-easy.exe` 内部的 PowerShell 子进程调起，BOM 必须保留以让 T-021 修的 PS5.1+zh-CN 磁盘形态正常）。

---

## §3 功能需求（FR）

### §3.1 解析层（消除 ParserError）

- **FR-1**：`scripts/install.ps1` 通过 `irm <url> | iex` 接收并执行时，stdout/stderr 中**不出现**含 `'﻿#' is not recognized` 字样的 ParserError。
- **FR-2**：`scripts/install.ps1` 通过 `irm <url> | iex` 接收并执行时，stdout/stderr 中**不出现**含 `'param' is not recognized` 字样的 ParserError。
- **FR-3**：`scripts/install.ps1` 通过 `irm <url> | iex` 接收并执行时，stdout/stderr 中**不出现**任何首部解析层（pre-`==> [1/8]`）红字错误（防御 BOM 之外其他不可见前缀字符的潜在变体）。

### §3.2 执行模型层（不杀宿主）

- **FR-4**：`scripts/install.ps1` 通过 `irm <url> | iex` 形态运行时，**脚本正常完成（8 步全跑通）**后，宿主 PowerShell 会话仍存活；执行 `$LASTEXITCODE` 能读到 0、执行 `sc query frp-easy` 能返回结果。
- **FR-5**：`scripts/install.ps1` 通过 `irm <url> | iex` 形态运行时，**脚本主动失败**（任何 `exit 1` / `exit 2` 路径，如非管理员、非 amd64、GitHub API 403/404、下载失败、`install-service.ps1` 退出非零等）后，宿主 PowerShell 会话仍存活；用户能看到 stderr 上的中文报错并能继续输入命令排障。
- **FR-6**：脚本失败时**用户必须能感知失败**（而不是被"窗口保活"反向掩盖错误）—— 即至少满足以下任一可观测信号：
  - (a) stderr 上仍有清晰的中文 `Write-Error` 错误行；
  - (b) `$LASTEXITCODE` 或某显式标识变量被设置为非零；
  - (c) 最后一行 stdout 明确印 "❌ 安装失败" / "安装中断" / "安装未完成" 等中文断言（具体文案由 Architect 在 02 选定）。

### §3.3 兼容性（不回归既有路径）

- **FR-7**：`scripts/install.ps1 -Help` 通过**磁盘形态** `.\install.ps1 -Help` 或 `powershell.exe -File .\install.ps1 -Help` 执行时，仍能正确显示中文帮助内容、退出码 0、stderr 无 ParserError。覆盖 PS5.1 + zh-CN 与 PS7.x 两个解释器。
- **FR-8**：`scripts/install.ps1` 通过磁盘形态执行**完整安装流程**（`.\install.ps1`，无 `-Help`）时，原 T-013 / T-014 / T-021 已通过的所有行为（GitHub API 查询、zip 下载校验、解压、升级语义、调 `install-service.ps1`、最终 8/8 输出）保持等价或更优；不引入新失败路径。
- **FR-9**：`scripts/install-service.ps1` 与 `scripts/uninstall-service.ps1` 在 git 仓库内的**字节内容完全不变**（含首 3 字节 BOM = `EF BB BF`）。git diff 对这两个文件必须为空。
- **FR-10**：`scripts/install.ps1` 内部对 `install-service.ps1` 的调用（当前在第 280 行 `$svc = Join-Path $InstallDir "scripts\install-service.ps1"; & $svc`）保持工作：被调脚本以**子进程**形式启动，`$LASTEXITCODE` 透传逻辑（当前 285-293 行）继续生效。

### §3.4 verify_all 闸门（防回归）

- **FR-11**：`scripts/verify_all.{ps1,sh}` 新增至少 1 个检查项断言"`scripts/install.ps1` 在 iex 形态下不会因首部 BOM / 不可见前缀字符触发 ParserError"。具体实现机制由 Architect 决定，候选含但不限于：
  - (a) 字节级断言 `install.ps1` 首 3 字节**不等于** `EF BB BF`；
  - (b) 字节级断言 `install.ps1` 顶部 N 字节为纯 ASCII（无任何 BOM / U+FEFF / U+200B 等不可见前缀字符）；
  - (c) 调用 `Get-Content -Raw install.ps1 | iex -ErrorAction Stop` 做端到端冒烟（需在 mock 网络与 mock sc.exe 环境下执行，复杂度较高）。
- **FR-12**：FR-11 新增的检查项**不波及** `scripts/install-service.ps1` 与 `scripts/uninstall-service.ps1` —— 这两个文件的 BOM 检查（T-021 AC-1）必须保留通过。即 verify_all 必须能区分"install.ps1（iex 入口、禁 BOM）"与"install-service.ps1/ uninstall-service.ps1（磁盘形态、要 BOM）"两类。
- **FR-13**：T-021 现有的 `scripts/*.ps1` 全量 BOM 检查项必须被**降级或拆分**，以容纳 install.ps1 的反向规则。具体降级 / 拆分方式由 Architect 在 02 决定（候选：(a) 改全量检查为白名单驱动；(b) 把 install.ps1 加入显式例外列表；(c) 重写为"非 iex-entry 必须 BOM、iex-entry 禁 BOM"分类检查）。

---

## §4 边界条件 / 错误路径

- **BC-1 PS5.1 + zh-CN code page 936**：用户主机 `chcp` 输出 936，`$PSVersionTable.PSVersion.Major -eq 5`。FR-1 ~ FR-6 必须在此环境通过。
- **BC-2 PS7.x + 任意 code page**：用户主机 `$PSVersionTable.PSVersion.Major -ge 7`。FR-1 ~ FR-6 必须在此环境通过（PS7 用 UTF-8 默认 code page，BOM 与中文表现可能与 PS5.1 不同）。
- **BC-3 非管理员**：脚本第 130 行检测非管理员后走 `exit 1`。在 iex 形态下，FR-5 要求宿主存活；用户应看到中文 `Write-Error` 提示后能继续输入命令。
- **BC-4 非 amd64 架构**：脚本第 144 行检测后 `exit 1`。同 BC-3 要求。
- **BC-5 GitHub API 403 限流 / 404 滚动发布未生成**：脚本第 167-176 行各自 `exit 1`。同 BC-3 要求。
- **BC-6 网络下载失败**：脚本第 219-223 行 `exit 1`。同 BC-3 要求。
- **BC-7 zip 解压失败 / 结构异常**：脚本第 234-244 行 `exit 1`。同 BC-3 要求。
- **BC-8 `install-service.ps1` 失败**：脚本第 290-293 行 `exit $LASTEXITCODE`（透传，通常为 2）。在 iex 形态下宿主必须存活，用户必须能看到上方 `install-service.ps1` 输出的中文错误。
- **BC-9 成功完成（8/8）**：脚本第 371 行 `exit 0`。在 iex 形态下宿主必须存活，用户能继续读 stdout 安装结果横幅、能输 `sc query frp-easy`。
- **BC-10 嵌套 try/finally 中的清理**：脚本第 295 行 `Remove-Item -Recurse -Force $tmpDir.FullName -ErrorAction SilentlyContinue` 必须仍能在所有退出路径（含错误退出）执行；本任务不允许引入"修宿主存活但跳过 tmpDir 清理"的回归。
- **BC-11 不可见前缀字符变体**：即使删 BOM 后，仍需防御未来有人误粘贴 U+FEFF / U+200B / U+00A0（NBSP）等不可见字符到脚本首字符位置；FR-3 与 FR-11 候选 (b) 形成长期防御。
- **BC-12 `$ErrorActionPreference="Stop"` 与 throw 的传播**：脚本第 31 行设置 Stop。Architect 若选 throw / return 替代 exit 的方案，必须保证 Stop 不让宿主 PowerShell 自杀（throw 在 iex 上下文行为与 exit 不同，需 Architect 在 02 验证）。

---

## §5 验收准则（AC）

> 标注规则：
> - **[A] 自动可验**：可由 verify_all / `pwsh -File` / 字节断言直接产出退出码。
> - **[U] 需用户真机验证**：需要用户在 PS5.1 + zh-CN 真机或公网拉 raw 内容跑 iex（QA 主机为 PS7 默认，无 PS5.1 + zh-CN）。
> - **[M] mock 可验**：可在 QA 主机用 mock（`Get-Content -Raw scripts/install.ps1 | iex`、mock GitHub API / sc.exe）模拟 iex 形态执行。

### §5.1 ParserError 消除

- **AC-1 [M] [U]**：在 PS5.1 + zh-CN 主机执行 `irm <raw_url> | iex`，stdout/stderr 全文 `Select-String 'is not recognized'` 命中 0 行。QA 主机可用 `Get-Content -Raw scripts/install.ps1 | iex` mock 形式预跑，结论一致。
- **AC-2 [M] [U]**：在 PS7.x 主机执行 `irm <raw_url> | iex`，stdout/stderr 全文 `Select-String 'ParserError'` 命中 0 行。
- **AC-3 [A]**：`scripts/install.ps1` 文件首字节读取（`Get-Content -Raw -Encoding Byte -TotalCount 8 scripts/install.ps1`）不以 `0xEF 0xBB 0xBF` 三字节开头；首 8 字节全部 ≤ `0x7E`（纯 ASCII 可打印范围），无任何 U+FEFF / U+200B 等不可见字符的 UTF-8 多字节编码起始字节。

### §5.2 宿主存活

- **AC-4 [M] [U]**：触发任一 `exit 1` 路径（候选最易触发：非管理员执行 → 走 FR-5/BC-3），iex 形态下宿主 PowerShell 会话**不退出**；事后能在同一会话执行 `$LASTEXITCODE` 与 `Get-Date` 返回有效结果。
- **AC-5 [M] [U]**：触发 `install-service.ps1` 失败（mock：故意把 `$svc` 路径改成不存在文件让其 `exit 1`），iex 形态下宿主存活，用户能读到中文 `Write-Error` 错误行。
- **AC-6 [U]**：成功跑完 `[8/8] 安装完成。` 后，iex 形态下宿主存活，能继续输 `sc query frp-easy` 拿到服务状态行。
- **AC-7 [M]**：失败路径下用户能感知失败 —— 满足 FR-6 (a)(b)(c) 中至少一条（Architect 在 02 选定具体形式，QA 在 06 按 02 选定形式断言）。

### §5.3 不回归既有路径

- **AC-8 [U]**：PS5.1 + zh-CN 磁盘形态 `.\install.ps1 -Help` 退出码 = 0，stdout 含完整中文帮助（不出现 `锘`、`鏄` 等 GBK 错解符号），stderr 无 ParserError。
- **AC-9 [A]**：PS7.x 磁盘形态 `pwsh -File scripts/install.ps1 -Help` 退出码 = 0，stdout 含完整中文帮助，stderr 无 ParserError。
- **AC-10 [A]**：`git diff --stat` 范围限定到 `scripts/install-service.ps1` 与 `scripts/uninstall-service.ps1` 必须返回空（字节级未改）。等价地：`Get-Content -Raw -Encoding Byte -TotalCount 3 scripts/install-service.ps1` = `[239,187,191]`，对 `uninstall-service.ps1` 同款。
- **AC-11 [U]**：PS5.1 + zh-CN 主机完整跑 `irm <raw_url> | iex` 从 `[1/8]` 到 `[8/8]`，最终输出"frp_easy 一键安装完成！"横幅，`sc query frp-easy` 显示 `STATE : 4 RUNNING`。等价于 FR-4 + FR-7 + FR-8 + FR-9 + FR-10 端到端 dogfood。

### §5.4 verify_all 闸门

- **AC-12 [A]**：`scripts/verify_all.ps1` 与 `scripts/verify_all.sh` 各包含 1 个新检查项实现 FR-11，命名含 "install.ps1" 与 "iex" 或 "BOM" 字样，便于未来 grep 定位。
- **AC-13 [A]**：负向自检：临时把 `scripts/install.ps1` 加回 UTF-8 BOM（前置 3 字节 `EF BB BF`），跑 `pwsh -File scripts/verify_all.ps1` 必须在 FR-11 新检查项 FAIL，错误信息含 `install.ps1` 文件路径与"iex"或"BOM"字样。负向验证由 QA 在 06 模拟、不落 main。
- **AC-14 [A]**：T-021 现有 BOM 全量检查必须 PASS（对 `install-service.ps1` / `uninstall-service.ps1` 等其余 .ps1 文件），即 FR-13 的拆分 / 降级必须保留对其余文件的覆盖。
- **AC-15 [A]**：`scripts/verify_all.ps1` 与 `scripts/verify_all.sh` 总 PASS 计数相对 T-021 末态**不下降**（持平或新增 FR-11 后升 1）；`scripts/baseline.json` 同步更新（具体字段由 Architect 在 02 核实 baseline.json 的真实 schema）。

### §5.5 文档同步

- **AC-16 [A]**：`docs/dev-map.md` 若有"install.ps1 编码"或"iex 入口"相关条目则更新；若无则新增一行简述"install.ps1 因 iex 入口禁 BOM；其余 .ps1 因磁盘形态加载需 BOM"（由 Architect 在 02 决定是否新建条目）。
- **AC-17 [A]**：本任务 `06_TEST_REPORT.md` 含裸标题 `## Adversarial tests`（insight L22 / L35 红线，无数字编号前缀）。
- **AC-18 [A]**：本任务 `07_DELIVERY.md` 含裸标题 `## Insight`（insight L43 红线，无数字编号前缀），否则 `archive-task.ps1` 收割 0 条。

---

## §6 非功能需求（NFR）

- **NFR-1 PS 版本支持矩阵**：PS 5.1（Windows 默认）+ PS 7.x（用户主动升级）两个解释器都必须满足 FR-1 ~ FR-10。本任务**不引入** PS 版本检测分流逻辑（不在 install.ps1 内根据 `$PSVersionTable.PSVersion.Major` 走不同路径），让脚本对两个版本同款行为。
- **NFR-2 跨 code page**：zh-CN（cp936 / GBK）+ en-US（cp1252）+ UTF-8（cp65001）三种宿主 code page 下，FR-1 ~ FR-10 必须满足。
- **NFR-3 失败可观测**：FR-5 + FR-6 联合 —— 不允许"修了宿主存活但让用户无法感知失败"的反向回归。任何 silent-success-on-failure 模式（如 `exit` 改 `return` 但不打错误行）禁止。
- **NFR-4 内容变更最小化**：本任务仅修 `scripts/install.ps1` 一个文件 + `scripts/verify_all.{ps1,sh}` + 可能的 `scripts/baseline.json` + 可能的 `docs/dev-map.md`。**不允许**顺手优化（重命名变量、整理注释、改文案）—— 任何这类变更属 design drift，必须在 04 报告。
- **NFR-5 不引入新依赖**：不引入新的 PS module / npm 包 / Go module / 外部 CLI 工具。FR-11 的检查项用 PS / bash 内置原语（`Get-Content -Encoding Byte` / `head -c 3` / `xxd` / `dd` 等可任选）。
- **NFR-6 verify_all 新检查运行时间 < 1s**：单文件字节读 + 路径比对预算极小。
- **NFR-7 行尾 LF 不变**：项目 `.gitattributes` 第 2 行 `* text=auto eol=lf` 已强制 LF；删 BOM 不得引入 CRLF。
- **NFR-8 git diff 噪声最小**：删 BOM 是前 3 字节变化；不得让 install.ps1 整文件 diff 因 CRLF/LF 切换而全量变红。
- **NFR-9 archive-task.ps1 自我引用 dogfood**：T-026 stage 7 PM 调 `pwsh -File scripts/archive-task.ps1 -Task install-ps1-iex-bom-and-host-exit-fix`；archive-task.ps1 本身**仍有** BOM（不在本任务范围内），与本任务的 install.ps1 删 BOM 不冲突 —— 因 archive-task.ps1 是磁盘形态调用、L33 已证 PS5.1/7 解释器对磁盘 BOM 透明。

---

## §7 范围

### §7.1 In-scope

1. 修改 `scripts/install.ps1` 让其在 iex 形态下不报 ParserError、不杀宿主（具体手段由 Architect 决定，候选见 §9）。
2. 修改 `scripts/verify_all.{ps1,sh}` 加 FR-11 闸门，调整 FR-13 现有 BOM 全量检查的拆分 / 降级。
3. 可能同步 `scripts/baseline.json`（依 verify_all 计数变化而定）。
4. 可能新增 `docs/dev-map.md` 一行（AC-16 留口子）。

### §7.2 Out-of-scope

1. **不**改 `scripts/install-service.ps1` / `scripts/uninstall-service.ps1` 任何字节（FR-9 / AC-10 硬约束）。
2. **不**改其他 `scripts/*.ps1`（archive-task / build / harness-sync / install-hooks / package / start-e2e-server / start / verify_all 主体逻辑）的字节内容（FR-11 / FR-12 / FR-13 仅允许 verify_all 加新 step，主体逻辑不动）。
3. **不**改 install.sh / install-service.sh（Linux 入口、无此问题）。
4. **不**引入 PS 版本检测在 install.ps1 内分流（NFR-1）。
5. **不**改 `$ErrorActionPreference="Stop"` 的全局策略（除非 Architect 在 02 证明必须改且 PM 批准；目前 RA 视为 OOS）。
6. **不**处理 T-025 / T-022 / T-018 等其他 backlog。
7. **不**触及 `.harness/` / `.claude/` / `CLAUDE.md` / `settings.json`。
8. **不**修改 `frp-easy.exe` Go 代码 / Vue Web UI 代码。
9. **不**改用户文档（README / docs/DEPLOYMENT.md）的一键安装命令本身 —— 命令 `irm ... | iex` 不变。

---

## §8 风险

### §8.1 删 BOM 对 install.ps1 在 PS5.1 + zh-CN **磁盘形态** 的回归

T-021 加 BOM 的原意是修 PS5.1 + zh-CN 磁盘形态加载无 BOM .ps1 时中文 GBK 误解码。本任务删 install.ps1 BOM 后，**理论上**该文件在 PS5.1 + zh-CN 磁盘形态会复现 T-021 之前的乱码问题。但实证证据：
- T-021 修复前的现象是 install.ps1 第 152 行 `Write-Host "==> [3/8] 查询 GitHub 滚动发布..."` 等中文字符串在 PS5.1 + zh-CN 磁盘形态被 GBK 误解码，输出乱码或解析失败。
- README / docs/DEPLOYMENT.md 也提到磁盘形态 `.\install.ps1` 是备选用法（虽推荐是 iex）。
- **风险 R-1**：删 BOM 后，少数用户走"先 `irm -OutFile install.ps1` 再 `.\install.ps1`"路径，在 PS5.1 + zh-CN 主机上可能回归到 T-021 之前的乱码。需要 Architect 在 02 给出**机制层判断**：PS5.1 加载无 BOM 含中文的 install.ps1 是否一定乱码？mitigation 候选：
  - (a) 接受此回归（理由：iex 是主推荐路径，磁盘形态只是备选；README 加注解）；
  - (b) install.ps1 全文改为纯 ASCII（中文 Write-Host 改英文 + 中文用 Unicode 转义 `[char]0x4e2d`）—— 变动大；
  - (c) 让 install.ps1 的中文从外部 resource 加载 —— iex 形态拉不到外部资源，不可行；
  - (d) 其他机制（Architect 探索）。
- 本 RA 把此风险**显式列出**，由 Architect 在 02 选定 mitigation 并由 PM 在 03 Gate 复核。

### §8.2 "宿主不关闭"反向引起新问题

如果 FR-4 / FR-5 的实现是"全部 `exit N` 改 `return` / `throw`"，但忘了同时确保失败时打错误行，**用户可能在窗口存活后误以为安装成功**（看到提示符回到 `PS>`，没注意上方红字）。
- **风险 R-2**：FR-6 是这个风险的直接缓解 —— 必须让失败可观测。Architect 在 02 必须设计明确的"失败横幅"或"失败标识变量"，QA 在 06 必须断言其存在。
- **风险 R-3**：`$LASTEXITCODE` 在 iex 形态下行为不确定 —— iex 是 in-process 执行，不像启动子进程那样自动设置 `$LASTEXITCODE`。Architect 必须在 02 验证 `$LASTEXITCODE` 在 iex 形态下的行为，决定是用它还是用另设变量。

### §8.3 `install-service.ps1` 调用层退出码透传

install.ps1 第 280-293 行通过 `& $svc` 子进程方式调用 `install-service.ps1`，并 `if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }` 透传。
- **风险 R-4**：如果 Architect 选"把所有 `exit N` 改 `throw`/`return`"，必须明确 install-service.ps1 的失败如何在 iex 上下文传播。`& $svc` 是子进程，子进程 `exit N` 正常设置父进程 `$LASTEXITCODE`，不会杀父 host —— 这部分**理论上**不需改。但需 Architect 验证。

### §8.4 跨形态测试覆盖不全

QA 主机为 PS7 + en-US（W11 Home 10.0.26200），无 PS5.1 + zh-CN 真机。AC-1 / AC-2 / AC-4 / AC-5 / AC-8 / AC-11 部分标 [U]。
- **风险 R-5**：自动化测试只能覆盖 PS7 + en-US；PS5.1 + zh-CN 路径靠用户真机验证。Architect 在 02 必须给出 mock 策略让 [M] 项尽量自动化。

### §8.5 verify_all 现有 BOM 检查兼容性

T-021 已加的 BOM 检查（覆盖全部 `scripts/*.ps1`）如不拆分，**本任务删 install.ps1 BOM 后 verify_all 必失败**。
- **风险 R-6**：FR-13 必须实现；Architect 在 02 必须给出具体拆分 / 降级方案。这不是"可选优化"，是"必做"。

---

## §9 给 Solution Architect 的提示（02 前置决议清单）

Architect 在 `02_SOLUTION_DESIGN.md` 必须给出明确决议的项：

1. **A-1 解决 E1（BOM ParserError）的手段**：候选含但不限于
   - (a) 单纯删 `install.ps1` 的 BOM（前 3 字节）；
   - (b) 在文件第一行加 `[char]0xFEFF` 处理代码（不可行 —— ParserError 已在第一字节发生，等执行到这行已晚）；
   - (c) 让 install.ps1 改为"自我重新解码" —— 文件第一行用 ASCII 写 `$raw = $MyInvocation.MyCommand.ScriptBlock; iex ($raw -replace '﻿', '')` 类机制（复杂、可能不稳）；
   - (d) 改用其他可移植机制（Architect 探索）。
   - **RA 默认指向**：(a) 删 BOM —— 最直接、与 §8.1 风险绑定 mitigation。

2. **A-2 解决 E2（`exit N` 杀宿主）的手段**：候选含但不限于
   - (a) **整段 `& { ... }` 子作用域包裹**：把现有 install.ps1 主体放在 `& { ... }` 内，`exit N` 只退子作用域不杀宿主；改动局部，原 `exit N` 逻辑不变；
   - (b) **所有 `exit N` 改 `throw`**：配合最外层 try/catch 转中文错误行；改动散落多处；
   - (c) **所有 `exit N` 改 `return`** + 顶层 if-else 分流：改动散落多处；
   - (d) **包成 function**：把主体放进 `function Invoke-Install { ... }` 然后调一次，`exit N` 改 `return`；
   - (e) 其他机制（Architect 探索）。
   - **RA 不预设方向**，但要求 Architect 在 02 明确分析每个候选对 BC-1 ~ BC-12 的覆盖。

3. **A-3 FR-6 失败可观测的具体形式**：从 (a) stderr 错误行 / (b) `$LASTEXITCODE` 类标识变量 / (c) 明确中文失败横幅 三选一或组合，Architect 在 02 选定并让 QA 在 06 按选定形式断言。

4. **A-4 FR-11 verify_all 闸门的具体实现**：从字节级断言 / 端到端 iex mock 等候选选定。

5. **A-5 FR-13 BOM 全量检查的拆分 / 降级方式**：从白名单 / 例外列表 / 分类检查 三候选选定。

6. **A-6 §8.1 R-1 mitigation 选择**：接受回归 / 改全 ASCII / 其他机制 三候选选定，PM 在 03 Gate 复核是否可接受。

7. **A-7 Mock 策略**：QA 主机 PS7 + en-US，如何 mock PS5.1 + zh-CN 行为？或明确放弃 mock、全标 [U] 用户真机验证。

8. **A-8 baseline.json schema**：Architect 在 02 实际打开 `scripts/baseline.json` 核实其 schema，决定 AC-15 是否需要改字段值。

---

## §10 关联历史任务

| 任务 | 关联点 | 关键文件路径 |
|---|---|---|
| T-021 encoding-ps51-bom | 给 `scripts/*.ps1` 全量加 BOM 修 PS5.1+zh-CN 磁盘形态；本任务 E1 根因 | `docs/features/_archived/encoding-ps51-bom/01_REQUIREMENT_ANALYSIS.md` / `02_SOLUTION_DESIGN.md` |
| T-024 install-ps1-iex-cmdletbinding-fix | 同一文件（install.ps1）同形态（iex）删 `[CmdletBinding()]` 修 ParserError；本任务 E1 ParserError 是它发现的之后另一组 | `docs/tasks.md` 第 18 行（trivial 直接修复，无阶段文档）+ commit `dd83eba` |
| T-022 service-mode-stderr-bridge | 同期 trivial 修复，无强关联（独立模块），但 PM_LOG 思路同款 | `docs/tasks.md` 第 20 行 |
| T-013 rolling-release-install | 首次设计 `irm \| iex` 一键安装路径，建立 PS5.1 + PS7 双解释器目标 | `docs/features/_archived/rolling-release-install/02_SOLUTION_DESIGN.md` |
| T-019 windows-service-scm-1053-fix | install-service.ps1 当前形态由本任务建立；本任务 OOS 第 1 条保护它 | `docs/features/_archived/windows-service-scm-1053-fix/02_SOLUTION_DESIGN.md` |
| T-017 install-role-and-public-ip | install.ps1 的 Get-PublicIPv4 函数 + 公网 IP 探测路径由本任务引入；本任务必须保留其字节级行为不变 | `docs/features/_archived/install-role-and-public-ip/02_SOLUTION_DESIGN.md` |
| T-014 frp-binary-auto-download | install.ps1 升级语义"不覆盖 frp_win\"由本任务引入；本任务必须保留 | `docs/features/_archived/frp-binary-auto-download/02_SOLUTION_DESIGN.md` |

**Insight 索引相关条目**：L12（管道形态路径锚定）、L17（PowerShell 写 TOML 必须无 BOM —— 反向参照）、L25（iex 形态禁 `$PSScriptRoot`，install.ps1 已合规）、L32（git 不能用 working-tree-encoding 锁 BOM）、L33（PS 解释器加载磁盘 .ps1 先剥 BOM —— 本任务证明此**仅限磁盘形态**，iex 形态 BOM 由 irm 解码成 U+FEFF 进入字符串）、L34（BOM 字节级 idiom）、L36（T-024 同形态 `[CmdletBinding()]` 修复）、L43（archive-task insight 标题禁数字前缀）。

---

## §11 给用户的待澄清问题（Open Questions）

> 本节按 RA 角色契约第 23-24 行强制必填。每条给至少 2 个候选答案。
> **本任务用户原始报告已被 PM 充分确认根因（E1 + E2），下列问题均属"设计偏好 / mitigation 取舍"层面而非根因层 ambiguity**。RA 推荐 PM 不阻塞 Architect、把这些问题作为 02 决议的候选交给 Architect 自决并在 03 Gate 由 Reviewer 复核。

### Q-1 §8.1 R-1 磁盘形态回归是否可接受？

删 install.ps1 BOM 后，少数用户走"先 `irm -OutFile install.ps1` 再 `.\install.ps1`"路径，在 PS5.1 + zh-CN 主机上**可能**回归到 T-021 之前的中文乱码。

- (a) **接受此回归**：iex 是主推荐路径；README 加注解"PS5.1 + zh-CN 主机请用 iex 形态"；
- (b) **不接受**：install.ps1 全文改纯 ASCII（中文 Write-Host 改英文 + 中文用 `[char]0x4e2d` Unicode 转义）；
- (c) **延后决定**：Architect 在 02 给机制层判断（PS5.1 加载无 BOM 含中文的 .ps1 究竟会乱码到什么程度），用户根据严重程度再决定。

**RA 推荐**：(c) 延后到 02 由 Architect 用机制层证据判断。

### Q-2 FR-6 失败可观测的形式偏好

用户希望失败时如何被告知？

- (a) **stderr 红字错误行就够了**（与当前 `Write-Error` 一致，最小变更）；
- (b) **必须有显式中文失败横幅**（如最后一行 stdout 印 "❌ 安装失败，请按上方错误排障"）；
- (c) **必须设置全局变量 `$global:FRP_EASY_INSTALL_FAILED = $true`** 让脚本结束后用户能 `if ($FRP_EASY_INSTALL_FAILED) { ... }` 编程感知。

**RA 推荐**：(a) + (b) 组合 —— 最低成本 + 用户体验提升；(c) 过度工程。

### Q-3 install.ps1 主体是否允许重排结构？

A-2 候选 (a)（`& { ... }` 包裹）几乎零结构改动；(b)/(c)/(d) 涉及散落多处 `exit N` 替换。用户对结构改动幅度的容忍度？

- (a) **优先最小改动**（推荐 A-2 (a)）；
- (b) **接受较大重构**（接受 A-2 (b)/(c)/(d)，换取更"PowerShell 习语"的代码）；
- (c) **不在意**：让 Architect 按工程权衡自选。

**RA 推荐**：(c) Architect 自选，PM 在 03 复核。

---

## §12 Verdict

**READY**

理由：
- 用户原始报告已被 PM 充分确认 E1（BOM ParserError）+ E2（`exit N` 杀宿主）双根因，事实层无 ambiguity；
- §11 列出的 3 个 Open Question **全部是设计偏好 / mitigation 取舍层**而非需求阻塞层 —— Architect 在 02 二选一或三选一即可推进，不需要用户先回答；
- FR 13 条、AC 18 条、BC 12 条均可机械验证（自动 [A] + mock [M] + 用户真机 [U] 分层标注，与 T-021 / T-019 已建立的降级模式一致）；
- 范围（In-scope / Out-of-scope）明确，特别是 OOS-1 / OOS-2 / OOS-4 / OOS-5 形成硬护栏；
- 风险（R-1 ~ R-6）逐条列出 mitigation 候选与 owner（Architect 在 02 / PM 在 03）；
- 无超出 §7 范围的隐藏需求。

**如 PM 判断 §11 Q-1 / Q-2 / Q-3 中任一条必须先让用户回答再开 02**，则把对应条改为 BLOCKER 并退回用户；否则直接派发 SA。RA 默认建议不阻塞、直接派 SA，Architect 在 02 自决并由 03 Gate Reviewer 复核。
