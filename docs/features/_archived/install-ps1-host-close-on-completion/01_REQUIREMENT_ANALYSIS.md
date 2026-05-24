# 01 Requirement Analysis — T-031 install-ps1-host-close-on-completion

> 角色：Requirement Analyst | 模式：full | 上游：`PM_LOG.md` + 用户原报告
> 红线：本文不做技术选型 / 不做实现决策（架构师产出）。仅明确"什么是对、什么是错、用什么验证"。
> 用户原报告：Win11 PowerShell 7 终端通过 `irm | iex` 形态运行 `scripts/install.ps1`，"到后期会自动关闭终端"，无法观察第 8 步打印的访问地址 / 公网 IP 探测结果 / 服务注册结果横幅。
> 用户授权完全自主决策（OQ 默认 `[PM-resolved]`）。

---

## §0 目标（Goal）

一句话：让任意被 `README.md` "Windows 一键安装"段所推荐的入口在 `scripts/install.ps1` 走到 step 8/8 完成后（或任何 `exit N` 失败路径触发后），承载该脚本的 PowerShell 宿主进程**不被关闭**，使用户能完整看到第 8 步的访问地址、公网 IP 探测结果、服务注册结果横幅，以及失败时的中文红色诊断横幅。

---

## §1 现状与触发场景（穷举入口 + 宿主关闭判定）

### 1.1 当前实现摘要

`scripts/install.ps1`（commit 046bdcc，T-026 交付）：

- 主体放入 `& { param([switch]$Help) ... } @PSBoundParameters` 子作用域包裹（L46 / L392）。
- 子作用域内含 9 处 `exit N`（行号 144 / 153 / 158 / 166 / 186 / 191 / 196 / 206 / 213 / 250 / 256 / 263 / 303 / 312 / 391，含末尾成功路径 `exit 0`）。
- 子作用域**外**有失败横幅 `if ($LASTEXITCODE -ne 0) { Write-Host ❌... }`（L398-L402）。
- step 7 调用磁盘脚本 `& $svc`（`install-service.ps1`），后者本身含 9 处 `exit N`（L97 / 104 / 113 / 142 / 147 / 153 / 169 / 179 / 204），其中 L204 为成功路径 `exit 0`。

### 1.2 入口穷举与宿主关闭判定

> 引用：MS 官方 `about_Invoke-Expression`：_"Expressions are evaluated and run in the **current scope**."_
> `about_Operators` "Call operator `&`"：_"The call operator executes in a **child scope**."_
> `about_Language_Keywords` "exit"：_"Causes PowerShell to exit a script **or a PowerShell instance**. … When you run `pwsh.exe -File <path to a script>` and the script file terminates with an `exit` command, the exit code is set to the numeric argument used with the `exit` command."_

| # | 入口 | 是否被 README 推荐 | 顶层 `exit N` 行为（当前实现下） | 宿主关闭? |
|---|---|---|---|---|
| (a) | 交互式 PS7 prompt + `irm ... \| iex`（已开窗口里粘贴） | **是**（README 唯一推荐入口） | `iex` 在父 runspace 当前 scope 解析脚本字符串；脚本顶层是 `& {...} @PSBoundParameters`，主体的 `exit N` 仅退子 scope（call operator 创建 child scope），随后 `$LASTEXITCODE` 被设置，宿主 prompt 回到 PS7 提示符 | **否（按设计）** —— L44 已记录 |
| (b) | `pwsh -Command "irm ... \| iex"`（一行启动模式，启动一个新 PS7 进程跑命令） | **未在 README 出现，但用户复制黏贴时常见** | `-Command` 跑完后 PS7 进程默认退出（与 `-NoExit` 互斥）；即便 `& {}` 包裹保护了脚本 `exit`，**外层 `pwsh -Command` 本身完成即关进程**，承载它的 cmd.exe / Windows Terminal tab / Run 框窗口会闭合 | **是（cmd shell 关 / Windows Terminal tab 关）** |
| (c) | `pwsh -File C:\path\install.ps1`（磁盘形态，README 备选"谨慎用户先下后审"路径） | **是**（README 备选） | MS 官方文档明确："When you run `pwsh.exe -File <path>` and the script file terminates with an `exit` command, the exit code is set …" — 即 `-File` 形态下顶层 `exit N` 退出**整个 pwsh 进程**；`& {}` 包裹后子作用域的 `exit` 也透传到顶层 `$LASTEXITCODE`，但 `-File` 模式脚本结束后 pwsh 进程仍按"脚本退出码 = 进程退出码"自然退出 | **是** |
| (d) | `cmd /c pwsh -Command "irm ... \| iex"`（双壳） | 未推荐，但用户群里常见 | pwsh `-Command` 跑完 → pwsh 退出 → `cmd /c` 跑完 → cmd 退出 → 宿主 cmd 窗口关 | **是** |
| (e) | Windows Run 框 `Win+R` 直接跑 `pwsh -Command "irm ... \| iex"` | 未推荐 | 与 (b) 同；Run 框启动的临时 PS7 窗口在 `-Command` 完成即关 | **是** |
| (f) | VS Code / Windows Terminal 集成 profile 启动行为 | 视用户配置 | 集成终端默认是交互式 host，行为同 (a)；若启动行配 `-Command` 则同 (b) | 取决于 profile |

### 1.3 用户实际场景定位

用户报告"Win11 PowerShell 7 终端"+"`irm | iex`"+"到后期会自动关闭终端"。按 §1.2 矩阵：

- **若**用户在已打开的 PS7 prompt 中粘贴 `irm ... | iex` → 入口 (a) → 按 L44 设计**不应该**关闭。**但用户报告关闭了** → 表明 L44 设计未在用户机器上生效或有未覆盖的边角。
- **若**用户在 Windows Terminal / Run 框 / cmd 里启动了一行 `pwsh -Command "irm ... | iex"` → 入口 (b)/(e)/(d) → 关闭符合 PowerShell 文档化行为，**但仍需让用户能看到 step 8 输出**（用户体验红线）。

### 1.4 待证伪的环境假设

> RA 不能直接登陆用户机器。下列假设须在 §2 根因矩阵的"如何证伪"列里给出可执行验证。

- A1：用户用的"PS7 终端"实际是 Windows Terminal 中的 PowerShell 7 默认 profile（即入口 (a) 等价物，应被 L44 保护）。
- A2：用户用的"PS7 终端"实际是 Windows Terminal 中的某个**已配 `-Command`** profile（即入口 (b)，L44 不保护）。
- A3：用户用的"PS7 终端"实际是 VSCode 集成终端 PowerShell profile（行为与 (a) 等价，应被 L44 保护，**除非** VSCode 在脚本结束后自动关闭 panel —— VSCode `terminal.integrated.shellIntegration.decorationsEnabled` 等不影响进程存活，但用户可能误把"output 窗口被新窗口顶下去"当成关闭）。

---

## §2 根因假设矩阵（候选根因 + 证据强度 + 证伪步骤）

> RA 不做选型，只列候选 + 给架构师证伪入口。

| # | 候选根因 | 证据强度 | 证伪步骤（可执行）|
|---|---|---|---|
| R1 | `& {} @PSBoundParameters` 子作用域包裹在用户的真实入口（不是 (a) 而是 (b)）下**不能保护宿主** —— L44 文字明确"仅在交互式 console host 下保护"，`pwsh -Command` / `pwsh -File` 启动的非交互或 host-bound 调用下 `exit` 仍杀进程。 | **强**（L44 明文 + MS `about_Language_Keywords` 官方文档） | 1) 在 Win11 交互式 PS7 prompt 复现：开 PS7 → 粘贴 `irm <raw> \| iex` → 跑完观察 prompt 是否仍在；2) 在 cmd 跑 `pwsh -Command "irm <raw> \| iex"` 观察 cmd 窗口是否关。两次结果若 (1)=不关 / (2)=关，则 R1 = 真。 |
| R2 | `install-service.ps1` 是磁盘脚本，从 `install.ps1` 主体内通过 `& $svc` 调用；磁盘 .ps1 内 `exit N` 的作用域归属——按 `about_Scopes` "Using the call operator to run a function or script runs it in script scope"，`& $svc` 让磁盘脚本以 script scope 跑，**脚本内 `exit N` 退出该脚本 scope**，不该泄漏到外层的 `iex` runspace；但若 PS 版本对 "magic script `exit` 跨 scope" 行为差异，可能在某些版本/某些 host 下导致 `exit` 透传到 pwsh.exe 进程层。 | **中**（理论上 `& script.ps1` 创建独立 scope；但实测变量行为复杂，需真机验证） | 1) 构造 mini repro：`pwsh -NoExit -c "& { & 'C:\tmp\inner-exit.ps1' ; 'still alive' }"`，其中 `inner-exit.ps1` 内只一行 `exit 1`；2) 观察 "still alive" 是否打印。若打印 → R2 假；若不打印且窗口关 → R2 真。 |
| R3 | step 7 `& $svc` 调用 install-service.ps1 因新版本 `Wait-ServiceRunning` 30s 轮询超时未达 → 走 `Write-Error + exit 2`，且此处的 `Write-Error` 在 `$ErrorActionPreference="Stop"` 下抛 terminating error，加之 install.ps1 主体也设 `$ErrorActionPreference="Stop"`，未被 try/catch 拦截的 terminating error 是否触发"runspace stop"语义被宿主解释为关窗。 | **弱**（terminating error 不直接关 host；但若被 `-Command` 模式承载则按入口 (b) 关） | 1) 临时让 `install-service.ps1` `Wait-ServiceRunning` 超时 5s 而非 30s，观察是否仍能复现"窗口关"；2) 加临时 `try {& $svc} catch { Write-Host "caught: $_" }` 包裹，观察异常是否被吞，是否仍关。 |
| R4 | 第 8 步成功路径的最后 `exit 0`（install.ps1 L391）在子作用域内执行，**按 L44 设计**应只退子作用域；但 PS7 实现的 `iex` 与 `& {}` 组合在某些 build/版本下 `exit 0` 行为差异（如 PS7.4 vs PS7.5）导致仍杀宿主。 | **弱**（无证据指向 PS7 实现差异；L44 当时验证基于 PS7.x 主线） | 1) 把 L391 `exit 0` 改为不写 `exit` 直接走完 scriptblock（依赖 `$LASTEXITCODE` 自然落 0），复测 PS7 + iex 形态；2) 若仍关 → R4 假；若不关 → R4 真。 |
| R5 | 中文失败横幅是 `Write-Host -ForegroundColor Red`，而 step 8 成功路径的访问地址表是 here-string 通过管道 `\| Write-Host`，**在 PS7 + iex + 父 runspace 当前 scope** 下若任何一处触发了 host 的 `[Console]::ReadKey()` / `Press any key` / `pause` 类阻塞……（核查后**不存在**这类 idiom） | **极弱**（grep 已确认 install.ps1 无 `pause` / `Read-Host` / `ReadKey`） | 已通过静态 grep 证伪：`Grep "Read-Host\|pause\|ReadKey" scripts/install.ps1` 无命中。R5 = 假。 |
| R6 | 入口 (b)/(d)/(e) 的 `pwsh -Command "..."` 是 PowerShell 文档化的"跑完即退"模式，本根因**不是 bug 而是文档化行为** —— 真正的修复方向是"修复 README 推荐入口（让 README 不推荐这些自杀型一行启动）"或"修脚本，让脚本内主动 detect 父进程是 `-Command`-launched 时阻塞等待用户按键"。 | **强**（MS 官方文档；R1 / R6 是同一现象的两面） | 同 R1 证伪步骤。 |

### 2.1 RA 倾向（非裁决，供架构师参考）

- 证据指向：**R1 + R6 是首要根因复合**（用户极可能用 cmd / Run 框 / Windows Terminal 自定义 profile 启动一行 `pwsh -Command`，触发 PowerShell 文档化的"`-Command` 跑完即退"）。
- R4 的"`exit 0` 在 iex + `& {}` 下是否真不杀宿主"必须**在真机交互式 PS7 prompt 复测**，因为 QA 自动化不能用 `pwsh -File` 替代（L44 明记）。
- R2 / R3 是低概率次因，但架构师方案应将其纳入"防御性消除"（即修主路径时顺带让 step 7 调用语义更安全）。

---

## §3 功能需求（FR）

每条 FR 都是测试可验证的，无"应该 / 大概"等模糊词。

| ID | 需求 | 必/可 |
|---|---|---|
| FR-1 | 在 README 当前推荐的"Windows 一键安装"任意入口（§1.2 表中 (a)、(c) 两类被 README 明文推荐的入口）成功路径完成后（step 8 横幅打印后），承载该脚本运行的 PowerShell 宿主进程**不退出**；宿主窗口不关闭；用户能完整读到访问地址、公网 IP 行、常用命令、更新、卸载段落。 | 必 |
| FR-2 | 同样的入口在任意一个 install.ps1 / install-service.ps1 内的失败路径（9 + 9 = 18 处 `exit N`，N≠0）触发后，承载宿主进程**不退出**；用户能看到上方的英文/中文红字错误 + 子作用域末尾追加的中文失败横幅（`❌ frp_easy 安装未完成（退出码=N）`）。 | 必 |
| FR-3 | 修复**不引入**任何交互式阻塞（`Read-Host`、`pause`、`[Console]::ReadKey()`、`Wait-Event` 等），不能让安装在自动化场景（CI / 远程脚本）下挂死等键。 | 必 |
| FR-4 | step 8 成功横幅的全部 13 行内容（"frp_easy 一键安装完成！" 起到 "============" 止，含 `$publicLine` / `$publicHint` 两个动态行）必须在宿主关闭前对人类用户可见，且**位于其他输出之后**（最后一屏可读）。 | 必 |
| FR-5 | 失败路径下，**最后两行**必须是 install.ps1 子作用域外的中文失败横幅（L399-L401 `❌ ... 退出码=N` + `请按上方红字定位失败原因`），不允许被后续输出覆盖。 | 必 |
| FR-6 | 修复方案对 PS5.1 + PS7.x 两个解释器版本均成立（PS5.1 仍是 zh-CN Win10/11 自带 Windows PowerShell，磁盘形态用户群无法回避）。 | 必 |
| FR-7 | 修复**不破坏** `install.ps1 -Help` 磁盘形态语义（顶层 `param([switch]$Help)` 透传到内部 scriptblock；splat 配对见 L45）。 | 必 |
| FR-8 | 修复**不引入**外部辅助文件（如 `wrapper.cmd` / `start-installer.bat`），保持单脚本一键发布形态（NFR-2 易维护原则）。 | 可 |
| FR-9 | step 7 调用 `install-service.ps1` 时其内 9 处 `exit N` 不得让 install.ps1 主体提前终止（按 `about_Scopes`，`& $svc` 创建 script scope，`exit` 退该 scope；架构师方案须验证）。 | 必 |
| FR-10 | README 当前推荐入口字符串若被本次修复要求更改（例如改为推荐 `pwsh -NoExit -Command "..."` 而非裸 `irm \| iex`），README 必须**同步**更新；本任务交付件含 README diff。 | 条件必（仅当架构师方案要求改 README 时） |

---

## §4 非功能需求（NFR）

| ID | 类别 | 需求 |
|---|---|---|
| NFR-1 | 用户体感 | 用户从启动脚本到看完 step 8 横幅，**全过程无需任何额外手动操作**（不需要打开新窗口 / 不需要按回车 / 不需要手动 scroll）。失败时同样无需额外操作。|
| NFR-2 | 可维护性 | **不引入** wrapper.cmd / `Start-Process -NoExit` / 注册表改 `AutoRun` / 全局 PSReadLine 配置等"黑魔法"侧通道；修复必须在 `scripts/install.ps1` 单文件内可解释（最多附带 `install-service.ps1` 同形改动）。注释需引用 MS 官方文档原文（已在 §1.2 引用）+ 关联 insight L44。|
| NFR-3 | 兼容性 | PS 5.1（Windows PowerShell，Win10/11 自带，zh-CN 默认 GB18030 解释器）+ PS 7.x（PowerShell Core，独立安装）两个版本下 §3 全部 FR 成立。不退化 T-026/T-029 已修复的 BOM 行为。 |
| NFR-4 | 失败可观测 | 18 处 `exit N` 任何一处触发，最终用户可见的最后一屏必须包含：（1）触发 `exit` 的中文/英文红字 `Write-Error` 行；（2）子作用域外的中文失败横幅 `❌ frp_easy 安装未完成（退出码=N）`；（3）退出码数字真值（如 1 / 2）。|
| NFR-5 | 自动化测试边界 | QA 测试**必须**包含"真机交互式 PS7 host 复测"步骤；不允许仅靠 `pwsh -File` mock 证明 FR-1（L44 明记此 mock 不可靠）。QA 自动化跑 `pwsh -File` 的部分用于回归 FR-2 / FR-7 等"非宿主存活"维度。|
| NFR-6 | 软件工程标准 | 任何"用户可能用 (b)/(d)/(e) 入口"的现实必须在脚本注释 + README 中以中文显式承认 + 给出对应指引（不可装作"用户应该只用 (a)"）。|

---

## §5 验收准则（AC，每条 FR ≥ 1 个可在 5 分钟内执行的验证序列）

> 每条 AC 给出"执行命令 + 期望输出 + 自动 / 手工 标签"。

### AC-1（验 FR-1，入口 a，真机交互式 PS7）

**操作**：在 Win11 真机打开 PowerShell 7（开始菜单搜 "pwsh" 启动；不是 Windows Terminal 中可能配 `-Command` 的 profile，而是直接 pwsh.exe），管理员身份。粘贴：

```powershell
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
```

**期望**：脚本跑完 step 8/8，打印完整横幅，**PS7 prompt（`PS C:\Users\xxx>`）回来**，窗口未关闭。手动执行 `$LASTEXITCODE` 应输出 `0`。**手工**。

### AC-2（验 FR-1，入口 c，磁盘形态）

**操作**：管理员身份 PS7 prompt 中，cd 到任意目录，执行：

```powershell
.\install.ps1
```

（install.ps1 是预先 `irm -OutFile install.ps1` 下载的版本）

**期望**：脚本跑完 step 8/8，打印完整横幅，**PS7 prompt 回来**，窗口未关闭。`$LASTEXITCODE` = 0。**手工**。

### AC-3（验 FR-2，失败路径 - 非管理员）

**操作**：以**普通用户**身份打开 PS7，运行 `irm ... | iex`（入口 a）。

**期望**：step 1 报红 `请以管理员身份运行 PowerShell ...`，**紧接着**显示中文失败横幅 `❌ frp_easy 安装未完成（退出码=1）`，**PS7 prompt 回来**，窗口未关闭。`$LASTEXITCODE` = 1。**手工**。

### AC-4（验 FR-2，失败路径 - 架构非 AMD64 mock）

**操作**：管理员 PS7 prompt，临时设 `$env:PROCESSOR_ARCHITECTURE='ARM64'` 后跑 `irm ... | iex`。

**期望**：step 2 报红架构不支持，紧接着中文失败横幅，prompt 回来。`$LASTEXITCODE` = 1。**手工**。

### AC-5（验 FR-3，无交互阻塞）

**操作**：在 install.ps1 / install-service.ps1 静态 grep：

```powershell
Select-String -Path scripts\install.ps1, scripts\install-service.ps1 -Pattern 'Read-Host|^\s*pause\s*$|\[Console\]::ReadKey|Wait-Event'
```

**期望**：**零命中**。**自动**（verify_all 可纳入）。

### AC-6（验 FR-4，step 8 横幅可见性）

**操作**：AC-1 / AC-2 复现后，截图最后一屏。

**期望**：横幅 13 行（"frp_easy 一键安装完成！" 起到 "============" 止）完整可见且在最末位置，未被任何后续行覆盖。**手工**。

### AC-7（验 FR-5，失败横幅可见性）

**操作**：AC-3 / AC-4 复现后，截图最后一屏。

**期望**：最后两行是 `❌ frp_easy 安装未完成（退出码=N）` + `请按上方红字定位失败原因；必要时执行 'sc query frp-easy' 检查服务状态。`，含 N=具体数字。**手工**。

### AC-8（验 FR-6，PS5.1 兼容）

**操作**：Win11 中开 `powershell.exe`（PS5.1）以管理员身份，磁盘形态跑 `.\install.ps1`。

**期望**：步骤都跑完 + step 8 横幅 + prompt 回来。允许中文乱码（T-029 已记录 OOS）。`$LASTEXITCODE` = 0。**手工**。

### AC-9（验 FR-7，-Help 仍工作）

**操作**：

```powershell
.\install.ps1 -Help     # 磁盘形态
echo $LASTEXITCODE      # 期望 0
```

**期望**：打印 Help 文本，退出码 0，prompt 回来。**自动**（可在 verify_all 加 `pwsh -File scripts\install.ps1 -Help`）。

### AC-10（验 FR-8，无外部辅助文件）

**操作**：

```powershell
Get-ChildItem scripts\install*.cmd, scripts\install*.bat -ErrorAction SilentlyContinue
```

**期望**：零命中（除非架构师方案明确决定引入并由 RA 重新评估）。**自动**。

### AC-11（验 FR-9，install-service.ps1 内 `exit N` 不泄漏到 install.ps1 之外）

**操作**：构造 mini repro `& { & 'C:\tmp\inner-exit-1.ps1' ; 'still alive in outer' }`，其中 inner 脚本只一行 `exit 1`。

**期望**：`still alive in outer` 被打印。`$LASTEXITCODE` 在外层为 1。**自动**（PS host scope 语义回归）。

### AC-12（验 NFR-3，PS5.1 + PS7 双版本回归）

**操作**：AC-1 / AC-2 / AC-3 / AC-9 在 PS5.1（`powershell.exe`）和 PS7（`pwsh.exe`）下各跑一遍。

**期望**：4 × 2 = 8 个用例全 PASS。**手工**。

---

## §6 Out of Scope（明确不做）

| OOS-ID | 项 | 理由 |
|---|---|---|
| OOS-1 | GUI 安装器（.msi / Inno Setup / Squirrel） | NFR-2 易维护原则 + 单脚本一键发布形态 |
| OOS-2 | UAC 自动提权（脚本启动时检测非 admin → 自动 `Start-Process -Verb RunAs`） | 与 T-026 / 原 install.ps1 设计明确要求"用户手动开管理员 PS"一致；自动提权会跨 host 边界使本任务 FR-1 更难保证 |
| OOS-3 | 卸载脚本 `uninstall-service.ps1` 的同款修复 | 卸载脚本不在 README 推荐入口；用户报告未涉及。如未来发现同款问题，新开任务。|
| OOS-4 | Web UI 启动后浏览器自动打开 | 与本任务正交 |
| OOS-5 | install-service.ps1 内 `Wait-ServiceRunning` 30s 超时常量调整 | 不在本任务范围；如有调整需求新开任务 |
| OOS-6 | 改 `apiClient` / 后端 download 等运行时代码 | 本任务仅修 install.ps1（必要时 + install-service.ps1 + README） |
| OOS-7 | "兼容 (b)/(d)/(e) 入口让 cmd 窗口不关" | 入口 (b)/(d)/(e) 是 PowerShell 文档化的"-Command 跑完即退"行为；若架构师选择"在脚本末尾加 `Read-Host`" 类方案会违反 FR-3，故 OOS。允许的合规姿态：README 不推荐这些入口 + 在入口字串处加中文警告。|

---

## §7 Open Questions（用户已授权自主决策，全部 `[PM-resolved]`）

### OQ-1：是否同步改 README 推荐入口语法？

**问题**：若架构师方案需要 README 推荐入口从 `irm ... | iex` 改为更安全的形态（如 `pwsh -NoExit -Command "irm ... | iex"` 或加 `; pause` 后缀），是否在本任务一并改？

**候选**：
- (a) 改 README 推荐入口同步本次修复
- (b) 仅修脚本，README 推荐入口不动（README 只在风险段落加警告）
- (c) 同时改 README + 加警告

**`[PM-resolved]` 默认 = (a)**：用户决策原则"用户体验好" + "符合软件工程标准"——若架构师认为需要改入口才能可靠满足 FR-1，README 必须同步（否则用户用旧入口仍会踩坑）。FR-10 已条件化此点。

### OQ-2：失败路径下是否打印"按任意键退出"提示？

**问题**：失败横幅之后是否加一行中文 `（请阅读上方报错后关闭此窗口）`？

**候选**：
- (a) 加
- (b) 不加（保持现有 2 行横幅）

**`[PM-resolved]` 默认 = (a)**：用户体感 NFR-1 倾向于显式告知。注意此为 `Write-Host` 提示行，**不是** `Read-Host` 阻塞（FR-3 红线不破）。

### OQ-3：QA 自动化是否将 AC-1（真机交互式 PS7 复测）标记为"必须人工"？

**问题**：L44 明记不能用 `pwsh -File` mock 证伪宿主存活；QA 自动化跑 `pwsh -File` 时如何让 verify_all 不误绿？

**候选**：
- (a) verify_all 仅跑 AC-5 / AC-9 / AC-10 / AC-11 类静态/自动化项；AC-1 / AC-2 / AC-3 / AC-4 / AC-6 / AC-7 / AC-8 / AC-12 强制"06_TEST_REPORT.md `## Adversarial tests` 段必须含人工复测证据（截图描述或命令输出粘贴）"
- (b) 写 PS5.1/PS7 真机自动化（需 windows-latest runner 上启动交互式 host，技术上复杂）

**`[PM-resolved]` 默认 = (a)**：与 L44 + T-026 既有做法一致；QA 在 `06_TEST_REPORT.md` 明确人工验证证据。

### OQ-4：是否将 README 入口 (a) 显式约束到"PS7 + Windows Terminal"或"PS7 直接 pwsh.exe"？

**问题**：README 当前只说"PowerShell 7"，不区分 Windows Terminal / VSCode / 裸 pwsh.exe；用户实测在 Windows Terminal 出问题（若 §1.4 假设 A2 成立）。

**候选**：
- (a) README 加章节"推荐启动方式"具体到截图
- (b) README 不动，仅在脚本注释里说明

**`[PM-resolved]` 默认 = (a)**：用户体感 NFR-1 + 软件工程 NFR-6 倾向于"用户应该看到清晰指引而非靠运气"。

### OQ-5：本任务是否引入对 install-service.ps1 的同款 `& {}` 包裹？

**问题**：install-service.ps1 当前未做子作用域包裹；按 FR-9 + §2 R2 假设，`& $svc` 调用本就把它隔离在独立 script scope，理论无需修；但若架构师方案需要"双层防御"……

**候选**：
- (a) 不动 install-service.ps1（信任 `about_Scopes` 文档化 script scope 隔离）
- (b) install-service.ps1 也做同款 `& {} @PSBoundParameters` 包裹（双层防御）

**`[PM-resolved]` 默认 = (a)**：NFR-2 易维护原则 + 单文件最小改动；若 AC-11 mini repro 证伪了 `about_Scopes` 隔离，再回头由架构师升级到 (b)。

---

## §8 关联历史任务（必读，不重复设计）

| Task | 关联点 |
|---|---|
| **T-026** `install-ps1-iex-bom-and-host-exit-fix`（2026-05-24，已 DELIVERED 归档于 `docs/features/_archived/install-ps1-iex-bom-and-host-exit-fix/`）| **直接前置**：上次解决"iex 形态 `exit` 杀宿主"问题，引入 `& {} @PSBoundParameters` 包裹。L44 nuance 明记此方案仅在交互式 console host 下保护，与本任务用户实测复现完全契合。架构师**必须**先读 T-026 `02_SOLUTION_DESIGN.md` / `04_DEVELOPMENT.md` / `06_TEST_REPORT.md`。|
| **T-024** `install-ps1-iex-cmdletbinding-fix` | 决定了 install.ps1 顶层不能用 `[CmdletBinding()]`（insight L33）。本任务不得退化此点。|
| **T-029** `readme-ps51-zhcn-disk-form-warning` | 决定了"接受 PS5.1+zh-CN 磁盘形态中文乱码 + README 引导用户首选 iex 形态或 PS7"。本任务的 FR-6 / AC-8 与此 OOS 共存。|
| **T-019** `windows-service-scm-1053-fix` | 决定了 sc.exe binPath 直接指向 .exe + install-service.ps1 用 `Wait-ServiceRunning` 30s 轮询。step 7 路径上的当前实现源自此任务。|
| **T-017** `install-role-and-public-ip` | 决定了 step 8 公网 IP 探测的 fallback 行为；本任务的 FR-4 step 8 横幅可见性约束包含 `$publicLine` / `$publicHint` 即源此任务。|
| insight L44 / L45 / L33 / L37 / L48 | §1.2 + §2 + §3 + §5 已逐条引用。|

---

## §9 风险登记（移交架构师）

| ID | 风险 | 缓解（建议方向，由架构师决定）|
|---|---|---|
| RISK-1 | 入口 (b)/(d)/(e) 是 PowerShell 文档化行为，**无法**在脚本内单方面"让父进程不退"——除非引入交互阻塞（破 FR-3）或改 README 推荐入口（OQ-1 默认 a）。 | OQ-1 默认让 README 改入口；同时脚本注释承认此 fundamental limit。|
| RISK-2 | AC-1 真机交互式 PS7 复测无法在 CI 自动化（OQ-3 默认 a 标手工）；若 QA 不跑或漏跑，FR-1 回归无法被自动捕获。 | QA 06 `## Adversarial tests` 段强制粘贴人工复测命令 + 输出；verify_all 加 `## Adversarial tests` 段存在性校验（已在 insight L18 / L48 模式中）。|
| RISK-3 | `& {} @PSBoundParameters` 内外 param 块未来加新顶层参数时易错位（insight L45）。 | 注释已存（install.ps1 L41-L45）；架构师改动时维持注释 + verify_all 加静态校验"内外 param 块名字段集相等"（如有架构师采纳）。|
| RISK-4 | 若架构师选择"加 `Read-Host` 等待用户按键" → 破 FR-3 → 不被允许。FR-3 红线锁定。| RA 已明确 FR-3 必约束 + OOS-7 显式拒绝。|
| RISK-5 | README 改推荐入口（OQ-1 a）会让"用户按旧群文档黏贴"短期内仍踩坑——属过渡期成本，非本任务能消除。 | README diff 中显式标注"如你看到的是旧入口请改用以下新入口"。|

---

## §10 Verdict

**READY**

所有 Open Questions 均按用户授权 `[PM-resolved]` 取默认。FR / NFR / AC / OOS 明确可验证。架构师可基于此进入 Stage 2（02_SOLUTION_DESIGN.md），优先证伪 §2 R1 / R2 / R4，选择实现方向（候选包括但不限于：让 install.ps1 在主体末尾保留无 `exit 0` 自然落出；在子作用域结束后追加 `[Console]::Title` / `Write-Host`-only 提示；README 改推荐入口为 `pwsh -NoExit -Command "..."`；等）。

— Requirement Analyst, 2026-05-24
