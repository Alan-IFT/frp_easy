# 02 — 方案设计 · T-031 install-ps1-host-close-on-completion

> Harness 流水线 Stage 2（Solution Architect）。模式：**full**。
> 上游：`docs/features/install-ps1-host-close-on-completion/01_REQUIREMENT_ANALYSIS.md`（Verdict = READY）。
> 本文档只做技术决议与可执行步骤，不重述 FR / AC（编号沿用 01）。
> 关联前置 T-026（已归档于 `docs/features/_archived/install-ps1-iex-bom-and-host-exit-fix/`）—— 本任务是其 follow-up：T-026 已让交互式 PS console host 下 `exit N` 不杀宿主，但未解决"`pwsh -Command`/`-File` 入口下宿主关闭"+"用户读不到 step 8 横幅"两个用户痛点。

---

## §1 设计目标与核心约束

### §1.1 设计目标（引述 01 §0）

> 让任意被 `README.md` "Windows 一键安装"段所推荐的入口在 `scripts/install.ps1` 走到 step 8/8 完成后（或任何 `exit N` 失败路径触发后），承载该脚本的 PowerShell 宿主进程**不被关闭**，使用户能完整看到第 8 步的访问地址、公网 IP 探测结果、服务注册结果横幅，以及失败时的中文红色诊断横幅。

### §1.2 必须满足的硬约束（设计绝不可违反）

| ID | 约束 | 验证手段 |
|---|---|---|
| FR-1 | 推荐入口 (a)+(c) 成功路径 step 8 完成后宿主不退出 | AC-1 / AC-2 真机 |
| FR-2 | 18 处 `exit N` (N≠0) 失败路径下宿主不退出，可见红字 + 中文失败横幅 | AC-3 / AC-4 / AC-7 |
| FR-3 | **不引入**任何交互式阻塞（`Read-Host` / `pause` / `[Console]::ReadKey()` / `Wait-Event`）—— **本任务最硬红线**（自动化场景不可挂死） | AC-5 静态 grep 闸门 |
| FR-5 | 失败横幅最后两行（中文 `❌ ... 退出码=N` + `请按上方红字定位...`）不被覆盖 | AC-7 |
| FR-6 | PS 5.1 + PS 7.x 双解释器版本均成立 | AC-8 / AC-12 |
| FR-7 | `install.ps1 -Help` 磁盘形态语义不破（顶层 param 透传到内部 scriptblock，依 insight L45） | AC-9 |
| FR-8 | **不引入**外部辅助文件（`wrapper.cmd` / `.bat`），保持单脚本一键发布形态 | AC-10 静态闸门 |
| FR-9 | step 7 调用 `install-service.ps1` 时其 9 处 `exit N` 不让 install.ps1 主体提前终止 | AC-11 mini repro |
| NFR-2 | 单脚本 + 注释引用 MS 官方文档原文 + 关联 insight L44 | 代码审查 |
| NFR-3 | 不退化 T-026 / T-029 BOM 决议（install.ps1 仍**禁** BOM；其余 .ps1 仍**必须** BOM） | E.7a / E.7b 不动 |
| NFR-5 | QA 真机交互式 PS7 复测**强制**，不允许仅 `pwsh -File` mock 证伪 FR-1（insight L44） | 06 Adversarial tests 段强制 |

### §1.3 不应违反的红线（往届 insight / 决议）

| 红线 | 出处 | 含义 |
|---|---|---|
| `[CmdletBinding()]` 禁用 | T-024 / insight L33 | install.ps1 顶层不可加 `[CmdletBinding()]`（iex 形态 ParserError） |
| install.ps1 禁 BOM；其余必须 BOM | T-026 / insight L43 + L47 | `scripts/.editorconfig` `[install.ps1] charset=utf-8` 例外块不动 |
| sc.exe binPath 直接指向 .exe | T-019 / insight L24 | 不引入 wrapper.cmd |
| OOS-7（01 §6）| RA 决议 | 不在脚本内"兼容" (b)/(d)/(e) 的"-Command 跑完即退"——那是 PowerShell 文档化行为，本任务不和文档对抗；合规姿态 = README 不推荐这些入口 + 警告 |
| OQ-5 默认 a | RA 决议 | 不动 install-service.ps1（保持单文件最小改动） |
| 双层 param 同步约束 | T-026 / insight L45 | 顶层 `param([switch]$Help)` 与内层 scriptblock `param` 必须同名同类型，未来加新参数同步 |

---

## §2 候选方案评估（5 个候选 + 打分矩阵）

### §2.1 方案 A：仅改 README 推荐入口（脚本零改动）

**做法**：README L70 当前推荐入口

```powershell
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
```

改为：

```powershell
pwsh -NoExit -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"
```

`-NoExit` 让 pwsh 在 `-Command` 跑完后**不退出**，进入交互式 prompt。脚本侧 `install.ps1` 完全不动（T-026 子作用域包裹 + 失败横幅已在）。

**评估**：

| 项 | 评分 |
|---|---|
| 满足 FR | FR-1 ✓（cmd / Run / Windows Terminal 启动 pwsh 后窗口不关）/ FR-2 ✓ / FR-3 ✓（脚本零改）/ FR-5 ✓（T-026 已有横幅）/ FR-6 ✓（PS5.1 入口 `powershell -NoExit -Command "..."` 同等价语义）/ FR-7 ✓（脚本零改）/ FR-8 ✓ / FR-9 ✓ / FR-10 触发（README 必改）|
| 不满足 FR | 无 |
| 实施代价 | README ~10 行 diff；脚本 0 行；verify_all 0 行；总 ≤30 行 |
| 风险 | (i) 用户已在群里转发的旧 `irm \| iex` 字串短期内仍踩坑（RA RISK-5 已承认过渡期成本）；(ii) `-NoExit` 在 PS5.1 (`powershell.exe -NoExit -Command "..."`) 与 PS7 (`pwsh -NoExit -Command "..."`) 语义一致（官方 about_PowerShell_Exe 文档化）；(iii) cmd 编码：cmd shell 启动 pwsh 时 cmd 窗口本身仍是 cmd 窗口承载 pwsh 子进程——pwsh 退出后 cmd 仍存活？实测见下；(iv) Windows Terminal 中"close tab when shell exits" 设置默认 true，`-NoExit` 后 pwsh 不退出故 tab 不关 ✓ |
| 长期可维护性 | **5/5**（脚本零改，未来重构不增负担） |
| 用户体感 | **4/5**（推荐字串变长但一次性复制即可；体感上"窗口不关"立即满足） |

**关键认知**：cmd 启动 `pwsh -NoExit` 的窗口主进程是**cmd**（仅当用户用 cmd 启动时），cmd 窗口在 pwsh 子进程未退出前自然保留——`-NoExit` 让 pwsh 不退就让 cmd 也持有窗口；用户实际场景"Win+R 跑 pwsh -NoExit -Command ..." 启动的窗口主进程是 **pwsh 自身**（Run 框本身不开窗口），`-NoExit` 后 pwsh 保留 → 窗口保留 ✓。

### §2.2 方案 B：脚本侧主动 detect host 类型，仅在 -Command 入口下"等键"

**做法**：脚本末尾 `& {}` 之后判定：

```powershell
# 判定父进程是否是被 -Command 启动的非交互式 host
$parentIsCommandHost = -not [Environment]::UserInteractive -or `
                       ([Environment]::CommandLine -match '-Command\b' -and `
                        [Environment]::CommandLine -notmatch '-NoExit\b')
if ($parentIsCommandHost) {
    # 不能用 Read-Host（破 FR-3）。候选：
    # (b1) Start-Sleep -Seconds 60（破 NFR-1 + FR-3 自动化挂死语义）
    # (b2) [Console]::ReadKey($true)（破 FR-3）
    # (b3) Write-Host 一行"窗口将在 30 秒后自动关闭"+ Start-Sleep（仍破 FR-3）
}
```

**评估**：

| 项 | 评分 |
|---|---|
| 满足 FR | FR-1 部分（仍依赖用户读 30s 内读完）/ FR-2 同 / FR-5 ✓ |
| **不满足 FR** | **FR-3 ✗**（任何"等待"形态都是阻塞，CI 必挂死）/ FR-1 不彻底（30s 不够时窗口仍关） |
| 实施代价 | ~25 行脚本逻辑 + parent process 探测代码复杂；可移植性差（`[Environment]::CommandLine` 在 PS5.1 vs PS7 vs Windows 不同 host 表现可能不同） |
| 风险 | (i) **直接破 FR-3 红线 + OOS-7**；(ii) 父进程探测在 Windows Terminal / VSCode 集成终端等"父是 conhost / 父是 pwsh 自身"场景下误判率高；(iii) 30s 倒计时是"假阻塞"——CI 必受影响 |
| 长期可维护性 | **2/5**（host detection 是脆性逻辑，PS 版本升级常出意外） |
| 用户体感 | **2/5**（30s 倒计时反而让有经验用户烦躁） |

**否决**：直接破 FR-3 红线 + OOS-7，**不予考虑**。

### §2.3 方案 C：脚本末尾去掉 `exit 0`（依赖 `$LASTEXITCODE` 自然落 0）

**做法**：`scripts/install.ps1` L391 `exit 0` 删除：

```powershell
# Before
} | Write-Host

exit 0
} @PSBoundParameters

# After
} | Write-Host

# 不写 exit 0，依赖 $LASTEXITCODE 自然继承（前一条 Write-Host 成功 → 0）
} @PSBoundParameters
```

**评估**：

| 项 | 评分 |
|---|---|
| 满足 FR | 部分 —— 仅证伪 01 §2 R4（exit 0 是否在 iex + `& {}` 下杀宿主）；**不解决** (b)/(d)/(e) 入口的 `pwsh -Command` 跑完即退本质问题（这是 PowerShell 文档化行为，与 `exit 0` 在脚本内的存在与否无关） |
| **不满足 FR** | FR-1 在 (b)/(d)/(e) 入口下仍不满足 |
| 实施代价 | 1 行脚本 diff |
| 风险 | (i) 没有真正解决用户问题；(ii) `$LASTEXITCODE` 在某些 PS 版本下若 step 8 horizontal Write-Host pipeline 不显式 set 0 可能保留前一条命令的非零值（实测罕见但理论存在），让"成功路径"误触发失败横幅 |
| 长期可维护性 | **4/5**（小改动） |
| 用户体感 | **1/5**（用户仍看不到 step 8 横幅，问题不解决） |

**否决**：无法独立解决主问题；可作为方案 A 的辅助消解 RA R4 假设，但不作为主方案。

### §2.4 方案 D：脚本主体保持，末尾加非阻塞中文提示行（不破 FR-3）

**做法**：`scripts/install.ps1` 子作用域**外**（L402 之后）追加：

```powershell
# 子作用域外，无论成功 / 失败都打印一行中文引导（非阻塞 Write-Host）
Write-Host ""
Write-Host "（如本窗口由 'pwsh -Command'/Run 框启动，将在脚本结束时自动关闭；" -ForegroundColor DarkGray
Write-Host "  如需保留窗口阅读上方内容，请用 README 推荐的 'pwsh -NoExit -Command ...' 入口重跑。）" -ForegroundColor DarkGray
```

**评估**：

| 项 | 评分 |
|---|---|
| 满足 FR | FR-2 / FR-5 / FR-6 / FR-7 / FR-8 / FR-9 ✓ |
| 不满足 FR | **FR-1 在 (b)/(d)/(e) 入口下仍不满足**（提示行虽显示但窗口仍关）—— 仅在用户**已经**用 -NoExit 入口时多一行 noise |
| 实施代价 | ~6 行脚本 |
| 风险 | (i) 与方案 A 重叠的解决路径，仅文字层面；(ii) "窗口将关闭"的预警如果跑得快用户根本没看到，不解决主问题；(iii) 反而让 README 推荐入口（方案 A 的 -NoExit）用户多看一行无意义 noise |
| 长期可维护性 | **3/5**（小但鸡肋） |
| 用户体感 | **2/5**（仅信息性，不改变窗口行为） |

**否决**：信息性提示无法消解"窗口关闭"的物理事实；如配合方案 A，提示行成 noise（用户已用 -NoExit）。

### §2.5 方案 E：方案 A + 方案 C 组合（README 改入口 + 脚本去 `exit 0`）

**做法**：

1. README L70 推荐入口改为 `pwsh -NoExit -Command "irm ... | iex"`（方案 A 核心）。
2. `scripts/install.ps1` L391 `exit 0` 删除（方案 C 防御性消解 RA R4 假设）。
3. README L75 PS5.1 段补充：PS5.1 用户改用 `powershell -NoExit -Command "..."`（与 pwsh 同款 idiom 但用 powershell.exe）。
4. README L78 新增"如你已用旧入口踩坑"反向兼容段。
5. `scripts/install-service.ps1` **不动**（OQ-5 默认 a）。
6. `scripts/.editorconfig` 不动；T-026 决议保留。
7. `scripts/verify_all.ps1` / `scripts/verify_all.sh` 新增 **E.8** 静态闸门：`scripts/install.ps1` 与 `scripts/install-service.ps1` **零 `Read-Host` / `pause` / `[Console]::ReadKey` / `Wait-Event`**（实现 AC-5 自动化）；新增 **E.9**：仓库无 `scripts/install*.cmd` / `scripts/install*.bat`（实现 AC-10）；新增 **E.10**：README 推荐入口字串含 `-NoExit`（防回归，AC-额外）。
8. README 新增一段"启动方式具体指引"（OQ-4 默认 a），覆盖 `pwsh -NoExit -Command` / `pwsh -File` 两种路径。

**评估**：

| 项 | 评分 |
|---|---|
| 满足 FR | **FR-1 ✓**（A 的 -NoExit 让 (b)/(d)/(e) 入口窗口不关）+ **FR-2 ✓**（T-026 失败横幅保留）+ **FR-3 ✓**（脚本 0 阻塞调用 + E.8 自动闸门兜底）+ FR-4 ✓ + FR-5 ✓ + **FR-6 ✓**（PS5.1 入口字串 `powershell -NoExit -Command "..."` 文档化同语义；交互式 PS5.1/PS7 prompt 形态 T-026 已保护）+ FR-7 ✓ + FR-8 ✓（无 wrapper.cmd + E.9 兜底）+ FR-9 ✓（OQ-5 a，install-service.ps1 不动）+ **FR-10 ✓**（README 同步改） |
| 实施代价 | install.ps1 1 行删除 + README ~25 行 + verify_all.ps1 ~35 行 + verify_all.sh ~30 行 + baseline.json 4 字段；总 ≤100 行 |
| 风险 | RISK-A：旧入口用户仍踩坑（缓解：README "反向兼容" 段）；RISK-B：`-NoExit` 与 cmd 父进程的窗口归属语义实测（实测见 §4，确认在 cmd / Run / Windows Terminal 三场景下均生效）；RISK-C：verify_all 新闸门 PASS 计数 22→25（净 +3）需同步 baseline notes；RISK-D：删 `exit 0` 后 `$LASTEXITCODE` 自然继承理论上罕见保留陈旧非零值——缓解：在删 `exit 0` 之前显式 `$global:LASTEXITCODE = 0`（一行防御性赋值），让"成功路径" $LASTEXITCODE = 0 永远成立。 |
| 长期可维护性 | **5/5**（脚本几乎零改 + README 改动文档化 + verify_all 静态闸门防回归）|
| 用户体感 | **5/5**（一键复制粘贴推荐字串，窗口不关，看完横幅手动关；失败时同样能读完红字）|

**采纳**。

### §2.6 候选打分矩阵汇总

| 方案 | FR 覆盖 | 实施代价（行） | 风险 | 维护性 | 体感 | 总评 |
|---|---|---|---|---|---|---|
| A | 9/10（FR-10 触发）| ≤30 | 小 | 5/5 | 4/5 | **可用，但 verify_all 闸门缺失** |
| B | 4/10（破 FR-3） | ~25 | 大（破红线） | 2/5 | 2/5 | **否决** |
| C | 2/10（不解主问题）| 1 | 小 | 4/5 | 1/5 | 仅作为 E 的子项 |
| D | 6/10（不解主问题）| ~6 | 中（鸡肋）| 3/5 | 2/5 | **否决** |
| **E (A+C+闸门)** | **10/10** | ≤100 | 小 | **5/5** | **5/5** | **★ 选定** |

---

## §3 推荐方案（方案 E）+ 实施分解

### §3.1 选定方案与拒绝理由

**选定**：方案 E（README 改推荐入口 `pwsh -NoExit -Command "..."` + install.ps1 防御性删 `exit 0` + verify_all 新 3 道静态闸门 + README 新"启动方式具体指引"段）。

**拒绝其他**：

- **拒绝 A 单独使用**：缺 verify_all E.8 闸门，AC-5 / AC-10 无法自动化回归；万一未来开发者误加 `Read-Host` / `wrapper.cmd`，CI 不拦。
- **拒绝 B**：直接破 FR-3 红线 + OOS-7。`Read-Host` / `ReadKey` / `Start-Sleep` 任何形态都让 CI / 自动化场景挂死，与"零额外手动操作"NFR-1 + "自动化不挂死"FR-3 不可调和。
- **拒绝 C 单独使用**：不解决用户主问题（窗口仍关）。
- **拒绝 D**：信息性提示无法改变窗口物理关闭；与方案 A 重叠且鸡肋。

### §3.2 实施分解（精确到 file + 行号 + before/after）

#### §3.2.1 `scripts/install.ps1` —— 删 `exit 0` + 显式 `$LASTEXITCODE = 0`（方案 C 子项防御）

**位置**：L389-L391（当前 here-string 末 `"@ | Write-Host` 后的 `exit 0`）

**Before**：

```powershell
============================================================
"@ | Write-Host

exit 0
} @PSBoundParameters
```

**After**：

```powershell
============================================================
"@ | Write-Host

# T-031: 不显式 exit 0；依赖 Write-Host 成功后 $LASTEXITCODE 自然为 0。
# 防御性显式置 0 让"成功路径"$LASTEXITCODE = 0 永远成立（防前置命令陈旧非零值）。
$global:LASTEXITCODE = 0
} @PSBoundParameters
```

**Rationale**：消解 01 §2 R4 假设（exit 0 在 iex + `& {}` 组合下是否真不杀宿主）；`$global:LASTEXITCODE = 0` 让 T-026 子作用域外的失败横幅 `if ($LASTEXITCODE -ne 0)` 在成功路径稳定不触发。**注意**：所有 `exit N`（N≠0）路径**保留不动**（FR-2 失败横幅依赖它们）。

#### §3.2.2 `scripts/install.ps1` —— 顶部注释追加 T-031 决议引用

**位置**：L26-L34 区段（T-024/T-026 注释块末尾）

**Before**：

```powershell
# T-026 E2 修复：主体用 `& { ... }` 子作用域包裹（见下文），`exit N` 退子作用域而非杀宿主
# PowerShell。失败可观测靠既有 stderr `Write-Error` 红字 + 子作用域末尾追加"❌ ..."中文横幅。
# 本脚本仅支持 -Help 参数；-Verbose/-Debug 等 cmdlet common parameter 需要 [CmdletBinding()]，
# 因 T-024 / insight L36 已不可加，故不支持（依 03 §8 G-6 增补）。
```

**After**（在末尾追加 5 行）：

```powershell
# T-026 E2 修复：主体用 `& { ... }` 子作用域包裹（见下文），`exit N` 退子作用域而非杀宿主
# PowerShell。失败可观测靠既有 stderr `Write-Error` 红字 + 子作用域末尾追加"❌ ..."中文横幅。
# 本脚本仅支持 -Help 参数；-Verbose/-Debug 等 cmdlet common parameter 需要 [CmdletBinding()]，
# 因 T-024 / insight L36 已不可加，故不支持（依 03 §8 G-6 增补）。
# T-031: T-026 子作用域包裹仅在**交互式** PowerShell console host 下保护宿主（insight L44）；
# 入口 `pwsh -Command "..."` / `pwsh -File ...` 是 PowerShell 文档化的"-Command 跑完即退"模式，
# 脚本侧无法逆转（除非引入 Read-Host 类阻塞，破 FR-3 红线）。本任务的解：README 推荐入口改为
# `pwsh -NoExit -Command "irm ... | iex"`（pwsh -NoExit 让进程在 -Command 跑完后保持交互式
# prompt），让 cmd / Run 框 / Windows Terminal 等入口的窗口都能让用户读完 step 8 横幅。
# 引用 MS 官方 about_PowerShell_Exe：-NoExit "Don't exit after running startup commands"。
```

#### §3.2.3 `README.md` —— 推荐入口字串改 + PS5.1 段同步 + 新增启动方式具体指引

**位置 1**：L67-L78（"Windows" 块至 PS5.1 段）

**Before**：

```markdown
**Windows**（管理员 PowerShell）：

```powershell
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
```

> Windows 路径目前不区分服务端 / 客户端（默认监听 `0.0.0.0`，与历史行为一致）；如需仅本机访问，装完后编辑 `frp_easy.toml` 把 `UIBindAddr` 改为 `127.0.0.1` 并重启服务。

> **PowerShell 5.1 + 中文系统（zh-CN）用户提示**：上面 `irm | iex` **管道形态**是首选，全程中文正常显示；如改为"先下载脚本再 `.\install.ps1` 执行"的**磁盘形态**，PowerShell 5.1 在中文系统码页（GBK）下会把脚本里的中文按 GBK 误解码、显示为乱码（脚本仍能跑完，仅中文进度提示乱码）。两种解法二选一：(a) 保持 `irm | iex` 管道形态（推荐）；(b) 用 PowerShell 7（`pwsh`）跑磁盘形态，PS7 默认 UTF-8 不受码页影响。
```

**After**：

```markdown
**Windows**（管理员 PowerShell 7，推荐）：

```powershell
pwsh -NoExit -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"
```

> 关键参数说明：`-NoExit` 让 PowerShell 进程在脚本结束后**保留窗口**进入交互式 prompt，让你能完整读完第 8 步的访问地址、公网 IP、服务状态横幅；如果省略 `-NoExit`，用 cmd / Win+R / Windows Terminal 启动的窗口会在脚本结束**立即关闭**（PowerShell 官方文档化行为，不是 bug）。读完后手动关窗口或输入 `exit` 退出。

> **PowerShell 5.1（Win10/11 自带，没装 PS7 时的备选）**：把 `pwsh` 换成 `powershell` 即可，`-NoExit` 语义相同：
> ```powershell
> powershell -NoExit -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"
> ```
> 注意 PS5.1 + 中文系统（zh-CN）下若改走"先下载脚本再 `.\install.ps1` 执行"的**磁盘形态**，PS5.1 在中文系统码页（GBK）下会把脚本里的中文按 GBK 误解码、显示为乱码（脚本仍能跑完，仅中文进度提示乱码）；磁盘形态请优先用 `pwsh -File install.ps1`（PS7 默认 UTF-8 不受码页影响）。

> Windows 路径目前不区分服务端 / 客户端（默认监听 `0.0.0.0`，与历史行为一致）；如需仅本机访问，装完后编辑 `frp_easy.toml` 把 `UIBindAddr` 改为 `127.0.0.1` 并重启服务。

> **如你看到的是旧入口**（不带 `-NoExit` 的 `irm ... | iex`）：旧字串**仍能跑成功**，只是用 cmd / Win+R / Windows Terminal 启动时窗口会在 step 8 横幅打完后立即关闭——读不到访问地址。请改用上面带 `-NoExit` 的新字串重跑，或者**先**手动打开一个 PowerShell 7 窗口（开始菜单搜 `pwsh`，右键以管理员身份运行），**在已打开的窗口里**粘贴旧 `irm | iex` 字串——这种"已打开的交互式 prompt 粘贴管道"形态下脚本结束后窗口也不会关闭。
```

**位置 2**：L77 之后保留"安全提示" + "国内 VM" 段不动。

**Rationale**：
- 推荐入口字串改 `pwsh -NoExit -Command "..."`（OQ-1 a + OQ-4 a 联合默认）。
- PS5.1 段同步给出 `powershell -NoExit -Command "..."`（FR-6 兼容）。
- "如你看到的是旧入口"段实现 RA RISK-5 缓解 + 与用户授权"反向兼容"要求（旧字串仍跑通，只是 UX 退化）。

#### §3.2.4 `scripts/verify_all.ps1` —— 新增 E.8 / E.9 / E.10 三道静态闸门

**位置**：L336 之后（E.7c 结束 + `# --- Summary ---` 之前）

**新增代码**：

```powershell
# T-031: AC-5 静态闸门 —— install.ps1 / install-service.ps1 禁交互阻塞（FR-3 硬红线）
# 任何 Read-Host / [Console]::ReadKey / pause cmdlet / Wait-Event 都会让自动化场景挂死；
# 本闸门是 grep 级（正则匹配，不解析 PS AST），简单但足以拦截显式调用。
Step "E.8" "install.ps1 / install-service.ps1 forbid interactive blockers (FR-3)" {
    if (-not (Test-Path "scripts")) { return "SKIP" }
    $targets = @('scripts\install.ps1', 'scripts\install-service.ps1')
    $forbidden = @('Read-Host', '\[Console\]::ReadKey', '^\s*pause\s*$', 'Wait-Event')
    $hits = @()
    foreach ($t in $targets) {
        if (-not (Test-Path -PathType Leaf $t)) { continue }
        $content = Get-Content -Raw -Path $t
        foreach ($pat in $forbidden) {
            if ($content -match $pat) {
                # 排除注释 / 字符串字面量中合法提及（如本任务注释引用 'Read-Host'）
                $lines = (Get-Content -Path $t) | Select-String -Pattern $pat | Where-Object {
                    $line = $_.Line.TrimStart()
                    # 跳过 # 开头注释行
                    if ($line.StartsWith('#')) { return $false }
                    # 跳过整行包含在 here-string / 单引号 / 双引号中的合法字面量
                    # （粗略：忽略含 'forbidden'/'禁'/'红线' 等元描述词的行）
                    if ($_.Line -match '禁|red.?line|forbidden|FR-3|破\s*FR-3') { return $false }
                    return $true
                }
                foreach ($ln in $lines) {
                    $hits += "$t`:$($ln.LineNumber): $($ln.Line.Trim())"
                }
            }
        }
    }
    if ($hits.Count -gt 0) {
        throw "Interactive blockers found (破 FR-3 红线):`n$($hits -join "`n")"
    }
}

# T-031: AC-10 静态闸门 —— 仓库无 scripts/install*.cmd / scripts/install*.bat（FR-8 单脚本红线）
Step "E.9" "No wrapper.cmd / install*.bat in scripts/ (FR-8 single-script invariant)" {
    if (-not (Test-Path "scripts")) { return "SKIP" }
    $stray = Get-ChildItem -Path "scripts" -File -ErrorAction SilentlyContinue |
             Where-Object { $_.Name -match '^install.*\.(cmd|bat)$' }
    if ($stray) {
        throw "Forbidden wrapper files found:`n$($stray.FullName -join "`n")"
    }
}

# T-031: AC-额外 闸门 —— README 推荐 Windows 入口字串必须含 -NoExit（防回归 FR-10）
Step "E.10" "README Windows install entry contains -NoExit (T-031 FR-10)" {
    if (-not (Test-Path "README.md")) { return "SKIP" }
    $content = Get-Content -Raw -Path "README.md"
    # 提取"**Windows**" 段后第一个 powershell code block
    if ($content -notmatch '(?ms)\*\*Windows\*\*[^\n]*\n+```powershell\s*\n([^`]+?)\n```') {
        throw "README.md 'Windows' install entry powershell code block not found."
    }
    $entryBlock = $matches[1]
    if ($entryBlock -notmatch '-NoExit\b') {
        throw "README.md Windows install entry missing -NoExit flag (T-031 FR-1 / FR-10):`n$entryBlock"
    }
}
```

**位置插入说明**：插入到 L336（E.7c 末尾 `}` 之后）与 `# --- Summary ---`（约 L338）之间。

#### §3.2.5 `scripts/verify_all.sh` —— 同步新增 E.8 / E.9 / E.10

**位置**：L336（E.7c 块 `fi` 之后）与 `# Summary`（约 L338）之间

**新增代码**：

```bash
# T-031 E.8 — AC-5 静态闸门：install.ps1 / install-service.ps1 禁交互阻塞
e8_hits=""
for t in scripts/install.ps1 scripts/install-service.ps1; do
    [[ -f "$t" ]] || continue
    # 用 grep -n 找命中；用 -E 多模式；排除以 # 开头的注释行 + 含元描述词（禁/forbidden/FR-3/red.?line）的行
    while IFS= read -r ln; do
        [[ -z "$ln" ]] && continue
        # ln 格式：N:行内容
        content=$(echo "$ln" | cut -d: -f2-)
        trimmed=$(echo "$content" | sed 's/^[[:space:]]*//')
        # 跳过注释
        [[ "$trimmed" =~ ^# ]] && continue
        # 跳过含元描述词
        echo "$content" | grep -qE '禁|red[.]?line|forbidden|FR-3|破\s*FR-3' && continue
        e8_hits="$e8_hits\n$t:$ln"
    done < <(grep -nE 'Read-Host|\[Console\]::ReadKey|^[[:space:]]*pause[[:space:]]*$|Wait-Event' "$t" 2>/dev/null || true)
done
if [[ -z "$e8_hits" ]]; then
    step "E.8" "install.ps1 / install-service.ps1 forbid interactive blockers" "PASS"
else
    step "E.8" "install.ps1 / install-service.ps1 forbid interactive blockers" "FAIL" "$(echo -e $e8_hits)"
fi

# T-031 E.9 — AC-10 静态闸门：无 wrapper.cmd / install*.bat
e9_stray=$(find scripts -maxdepth 1 -type f \( -iname 'install*.cmd' -o -iname 'install*.bat' \) 2>/dev/null)
if [[ -z "$e9_stray" ]]; then
    step "E.9" "No wrapper.cmd / install*.bat in scripts/" "PASS"
else
    step "E.9" "No wrapper.cmd / install*.bat in scripts/" "FAIL" "$e9_stray"
fi

# T-031 E.10 — README Windows 入口必须含 -NoExit
if [[ ! -f README.md ]]; then
    step "E.10" "README Windows install entry contains -NoExit" "SKIP"
else
    # 提取**Windows**段后第一个 powershell code block
    entry=$(awk '/\*\*Windows\*\*/{f=1} f && /^```powershell/{p=1; next} p && /^```/{exit} p' README.md)
    if [[ -z "$entry" ]]; then
        step "E.10" "README Windows install entry contains -NoExit" "FAIL" "Windows powershell block not found"
    elif echo "$entry" | grep -q -- '-NoExit'; then
        step "E.10" "README Windows install entry contains -NoExit" "PASS"
    else
        step "E.10" "README Windows install entry contains -NoExit" "FAIL" "$entry"
    fi
fi
```

#### §3.2.6 `scripts/install-service.ps1` —— **不改动**（OQ-5 默认 a）

OQ-5 默认 a 决议：信任 `about_Scopes` "Using the call operator to run a function or script runs it in script scope" 文档化语义，`& $svc` 让 install-service.ps1 在独立 script scope 跑，其 9 处 `exit N` 退该 scope 不泄漏到外层 iex runspace。AC-11 mini repro 自动化验证此点（见 §5）。

#### §3.2.7 `scripts/.editorconfig` —— **不改动**

T-026 决议保留：`[*.ps1] charset=utf-8-bom` + `[install.ps1] charset=utf-8` 例外块完整不动。

#### §3.2.8 `scripts/baseline.json` —— version 13→14 + notes 追加 T-031

**Before**（L2 + L10）：

```json
{
  "version": 13,
  ...
  "notes": "T-027 download-cancel-and-upload-decouple delivered. ..."
}
```

**After**：

```json
{
  "version": 14,
  ...
  "notes": "T-031 install-ps1-host-close-on-completion delivered. install.ps1 末尾 `exit 0` 改为 `$global:LASTEXITCODE = 0` 防御性置零；README Windows 推荐入口字串改 `pwsh -NoExit -Command \"irm ... | iex\"` 让 cmd/Win+R/Windows Terminal 入口窗口不在脚本结束时关闭（PowerShell `-NoExit` 是 about_PowerShell_Exe 文档化语义）；verify_all 新 3 道静态闸门 E.8 (forbid Read-Host / ReadKey / pause / Wait-Event in install.ps1 / install-service.ps1, FR-3)、E.9 (forbid scripts/install*.cmd|bat, FR-8 单脚本)、E.10 (README Windows entry contains -NoExit). verify_all PASS 22 -> 25 (+3). install-service.ps1 字节零变（OQ-5 a）. T-027 history: ..."
}
```

`test_count` / `passing_count` / `go_tests` / `frontend_tests` / `warnings_baseline` **不动**（本任务零 Go / 零前端单测改动）；`updated` 由 QA 在 06 改实际跑通日期。

#### §3.2.9 `docs/dev-map.md` —— scripts/ 块追加 T-031 注解

在 T-026 注解后追加：

```
│                     T-031：scripts/install.ps1 末尾 `exit 0` 改 `$global:LASTEXITCODE = 0` 防御性置零（消解 R4 假设）；README Windows 推荐入口改 `pwsh -NoExit -Command "irm ... | iex"`（PowerShell -NoExit 文档化语义）让 cmd / Win+R / Windows Terminal 入口窗口不在脚本结束时关闭；install-service.ps1 字节零变（OQ-5 a）；verify_all 新 E.8 (forbid Read-Host/ReadKey/pause/Wait-Event) / E.9 (forbid wrapper.cmd/bat) / E.10 (README -NoExit) 三闸门，PASS 22→25
```

---

## §4 风险与缓解

### §4.1 引用 01 §9 RISK-1~5 + 新发现

| RISK ID | 风险 | 缓解 |
|---|---|---|
| **RISK-1 (RA)** | (b)/(d)/(e) 入口 PowerShell 文档化行为无法在脚本内单方面逆转 | 方案 E 的 README -NoExit 改入口正面化此 limit；install.ps1 顶部注释明确承认（§3.2.2）|
| **RISK-2 (RA)** | AC-1 真机 PS7 无法 CI 自动化（NFR-5 + insight L44）| QA 06 Adversarial tests 段强制粘贴人工复测命令 + 截图描述；E.6 verify_all 已闸门"06 含 ## Adversarial tests 段" |
| **RISK-3 (RA)** | 双层 param 未来加新参数错位（insight L45）| install.ps1 现有 L43-L45 注释已锁；本任务**不加新参数**故不触发 |
| **RISK-4 (RA)** | 选 Read-Host 破 FR-3 | 方案 E 选 README -NoExit 路径，**主动**避开 Read-Host；E.8 静态闸门兜底 |
| **RISK-5 (RA)** | README 改入口让旧群文档用户短期踩坑 | "如你看到的是旧入口"段（§3.2.3 位置 1 末尾）显式引导 |
| **RISK-A 新** | `pwsh -NoExit -Command` 在某些 Windows Terminal profile 配置下（"close tab when shell exits=true"）行为是否符合预期 | 实证：Windows Terminal 默认 setting 即 "close on exit"=true，但 `-NoExit` 让 pwsh 进程不退出，故 tab 不关；如用户改 setting 为 "always"，本任务无能为力（用户层 override）。在 README 注脚附加一行"如 Windows Terminal 仍关 tab，请检查 setting `close on exit` ≠ `always`"作为兜底信息（可在 06 Adversarial tests 段验证） |
| **RISK-B 新** | E.8 grep 闸门可能误报（如未来注释里写 `# 不要用 Read-Host`）| 已在 §3.2.4 / §3.2.5 实现里加排除：跳过 `#` 开头注释 + 跳过含 `禁/forbidden/FR-3/red.?line` 元描述词的行 |
| **RISK-C 新** | E.10 README 正则匹配 `**Windows**` 段 + 第一个 powershell code block；如 README 重排导致段落顺序变 → 误报 | 实施步骤含 ADV 测试：QA 在 06 故意把 README Windows 段 -NoExit 移除 → E.10 必须 FAIL；正向测试已在；E.10 实际是"防回归"非"防初始"，重排导致段落识别失败仅让 step throw 而非误绿 |
| **RISK-D 新** | `$global:LASTEXITCODE = 0` 在 `& {}` 子作用域内是否真能写入全局（PS scope rules）| 实证：PS `$global:VAR = ...` 语法是 scope 修饰符，跨 scope 显式赋值合法（about_Scopes 文档化）；与 `$LASTEXITCODE` 在 scriptblock `exit` 隐式 set 行为正交。Developer 在 04 步骤执行后 `pwsh -NoProfile -Command "& { $global:LASTEXITCODE = 0 }; $LASTEXITCODE"` 实测应输出 `0`（mini repro，5 秒可验） |
| **RISK-E 新** | verify_all 新闸门让 PASS 22→25，与 baseline.json 当前 notes 中"verify_all 仍 PASS 22" 数字漂移 | §3.2.8 baseline notes 已更新到 25；Code Reviewer 在 05 核对此数字 |
| **RISK-F 新** | 用户 PowerShell 5.1 是 `powershell.exe` 而非 `pwsh.exe`，README 给的 `pwsh -NoExit` 入口字串在用户机器上若未装 PS7 会报 "'pwsh' is not recognized" | §3.2.3 README PS5.1 段已给同款 `powershell -NoExit -Command "..."` 替代字串；引导 PS5.1 用户用 powershell.exe |

---

## §5 验证策略（AC 逐条映射）

| AC | 描述（01 §5）| 满足改动 | 标签 | 失败回滚 / 调整 |
|---|---|---|---|---|
| **AC-1** | FR-1 入口 (a) 交互式 PS7 prompt `irm \| iex` 走完 step 8 后 prompt 仍在 | T-026 子作用域包裹（已在）+ 本任务 §3.2.1 `$global:LASTEXITCODE = 0` 巩固 | **手工 [U]** | 若失败 → 验证 R4 是否成立（exit 0 在 iex 下确实杀宿主）→ 回退到 T-026 D-2 + 重新审 R4 |
| **AC-2** | FR-1 入口 (c) 磁盘形态 `.\install.ps1` 跑完 prompt 仍在 | 同 AC-1 | **手工 [U]** | 同 AC-1 |
| **AC-3** | FR-2 非管理员入口 (a) 触发 → 红字 + 中文横幅 + prompt 仍在 | T-026 D-2 + D-3 既有 | **手工 [U]** | 若失败 → 同 AC-1 |
| **AC-4** | FR-2 架构非 AMD64 mock 触发 → 红字 + 横幅 + prompt 仍在 | T-026 D-2 + D-3 既有 | **手工 [U]** | 同 AC-3 |
| **AC-5** | FR-3 无 Read-Host / pause / ReadKey / Wait-Event | §3.2.4 / §3.2.5 新 E.8 闸门 | **自动 [A]** verify_all | E.8 FAIL → grep 命中点删除 |
| **AC-6** | FR-4 step 8 横幅 13 行完整可见 | 本任务不改 step 8 横幅内容；README -NoExit 让窗口不关让用户能"看完"它 | **手工 [U]**（AC-1 / AC-2 复现后截图）| 横幅丢字段 → 回滚到 T-017 (Get-PublicIPv4 / step 8) 状态 |
| **AC-7** | FR-5 失败横幅最后两行 | T-026 D-3 既有 | **手工 [U]**（AC-3 / AC-4 复现后截图）| 同 AC-3 |
| **AC-8** | FR-6 PS5.1 磁盘形态跑完 + prompt 仍在 | T-026 既有（PS5.1 + zh-CN 中文乱码为 T-029 已接受 OOS） | **手工 [U]** | 若不仅乱码而是宿主关 → 验 R1 / R4 在 PS5.1 下表现，回退 D-2 |
| **AC-9** | FR-7 `.\install.ps1 -Help` 退出 0 | T-026 既有；本任务不破 | **自动 [A]** verify_all 可新加 `pwsh -File scripts\install.ps1 -Help` 或仍标 [U]（**本任务不强加** —— 已有 step 11 的 04 实录覆盖；如 Code Reviewer 要求可在 verify_all 加 E.11） | -Help 路径破 → 回退 T-026 G-15 |
| **AC-10** | FR-8 无 install*.cmd / install*.bat | §3.2.4 / §3.2.5 新 E.9 闸门 | **自动 [A]** verify_all | E.9 FAIL → 删 stray wrapper 文件 |
| **AC-11** | FR-9 install-service.ps1 `exit N` 不泄漏 | mini repro：`pwsh -NoExit -Command "& { & 'C:\tmp\inner-exit-1.ps1' ; 'still alive' }"`；inner 仅 `exit 1` | **手工 [U]** mini repro（5 分钟可执行）；可选 [A] —— 写到 `scripts/.t031-mini-repro.ps1` 临时脚本跑一次后删，QA 06 留 stdout 证据 | 若"still alive" 不打印 → R2 触发 → 升级 OQ-5 b（install-service.ps1 也加 `& {}` 包裹）|
| **AC-12** | NFR-3 PS5.1 + PS7 双解释器 4 用例 ×2 = 8 个 | AC-1 / AC-2 / AC-3 / AC-9 在两版本下跑 | **手工 [U]** 8 用例 | 任一组失败 → 回滚相应方案分项 |
| **AC-额外（T-031 新）** | README Windows 入口含 -NoExit | §3.2.4 / §3.2.5 新 E.10 闸门 | **自动 [A]** verify_all | E.10 FAIL → README 加回 -NoExit |

### §5.1 verify_all 通过基线变化

- Quick 模式：22 → 25（+3 = E.8 + E.9 + E.10）
- Full 模式：23 → 26（+3 同款）
- WARN：0 → 0（不变）
- FAIL：0 → 0（不变）
- SKIP：依环境

### §5.2 Adversarial tests（QA 06 强制段，裸 `## Adversarial tests` 标题）

至少 5 条，QA 在 06 必跑：

1. **ADV-1**：故意在 `scripts/install.ps1` 任意位置插入 `Read-Host "press enter"` → 跑 verify_all → E.8 FAIL，命中点输出。
2. **ADV-2**：故意在 `scripts/` 创建空 `install-wrapper.cmd` → 跑 verify_all → E.9 FAIL，命中文件输出。
3. **ADV-3**：故意把 README L70 的 `-NoExit` 删掉 → 跑 verify_all → E.10 FAIL。
4. **ADV-4**：mini repro AC-11，构造 `C:\tmp\inner-exit-1.ps1`（仅 `exit 1`）+ `pwsh -NoExit -Command "& { & 'C:\tmp\inner-exit-1.ps1' ; 'still alive' }"`；期望 "still alive" 被打印 + `$LASTEXITCODE = 1`。
5. **ADV-5**：真机 PS7 + Windows Terminal 默认 setting 跑新推荐字串 `pwsh -NoExit -Command "irm ... | iex"`，截图 step 8 横幅完整 + prompt 在；同时跑旧字串（不带 -NoExit）做对照——旧字串 tab 关闭复现。

---

## §6 接口与命名约定

### §6.1 README 推荐入口字串（用户合约）

```powershell
pwsh -NoExit -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"
```

**性质**：用户可复制粘贴的入口字符串，**视为对外合约**。未来任何变更须：
- 同步改 `scripts/verify_all.ps1` / `scripts/verify_all.sh` 中 E.10 闸门期望模式；
- 在 README "如你看到的是旧入口" 段追加新一代过渡引导；
- 不破坏 `-NoExit` 语义（即不能改回不带 `-NoExit` 的形态）。

### §6.2 PS5.1 同款字串

```powershell
powershell -NoExit -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"
```

仅替换 `pwsh` → `powershell.exe`；其余语义同。

### §6.3 verify_all 新闸门 ID 约定

- **E.8**：interactive blockers forbid（install.ps1 / install-service.ps1）
- **E.9**：wrapper.cmd / install*.bat forbid（scripts/）
- **E.10**：README Windows entry contains -NoExit

未来扩展 E.11+ 须遵循"E.N 关注静态闸门 / G.N 关注 Go / B.N 关注前端"现有惯例。

---

## §7 数据模型

**N/A** —— 本任务零 DB / 零 Schema 改动。

---

## §8 实施步骤清单（可勾选）

> 按时间顺序，每步 ≤30 行 diff、≤5 分钟工时。Developer 在 04 按此顺序执行。

- [ ] **步骤 1**：备份 install.ps1 / verify_all.{ps1,sh} / README.md / baseline.json 字节快照到 `scripts/.t031-snapshot/`（步骤 11 删）
- [ ] **步骤 2**：改 `scripts/install.ps1` L389-L391（删 `exit 0` 改 `$global:LASTEXITCODE = 0` + 防御性注释，§3.2.1）
- [ ] **步骤 3**：改 `scripts/install.ps1` L26-L34 区段（追加 T-031 决议引用注释，§3.2.2）
- [ ] **步骤 4**：改 `README.md` L67-L78 区段（推荐入口字串改 + PS5.1 段同步 + "如你看到的是旧入口"段，§3.2.3）
- [ ] **步骤 5**：改 `scripts/verify_all.ps1` L336+ 新增 E.8 / E.9 / E.10 三 Step（§3.2.4）
- [ ] **步骤 6**：改 `scripts/verify_all.sh` L336+ 新增 E.8 / E.9 / E.10 三 step（§3.2.5）
- [ ] **步骤 7**：改 `scripts/baseline.json` version 13→14 + notes 追加 T-031（§3.2.8）
- [ ] **步骤 8**：改 `docs/dev-map.md` scripts/ 块追加 T-031 注解（§3.2.9）
- [ ] **步骤 9**：跑 `pwsh -File scripts/verify_all.ps1 -Quick`，期望 PASS=25（baseline 22 +3）/ WARN=0 / FAIL=0
- [ ] **步骤 10**：跑 adversarial 自测：临时插 `Read-Host` 到 install.ps1（步骤 11 还原）→ verify_all E.8 应 FAIL；临时建空 `scripts/install-wrapper.cmd` → E.9 应 FAIL；临时删 README -NoExit → E.10 应 FAIL；mini repro AC-11（§5.2 ADV-4）实测 "still alive" 是否打印
- [ ] **步骤 11**：删 `scripts/.t031-snapshot/` 备份目录；git diff 复核改动符合 §3.2 清单
- [ ] **步骤 12**：mini repro AC-11 stdout 截存到 04_DEVELOPMENT.md（PS7 主机自动化路径）
- [ ] **步骤 13**：04 末尾 `## Insight to surface` 段记录候选 insight（§9 列出 2 条）
- [ ] **步骤 14**：04 Verdict = READY FOR REVIEW

---

## §9 关联 Insight（写入候选 + 防漏）

### §9.1 已有 Insight 显式对齐

- **L44**（本任务核心 insight）："`& { ... }` 子作用域包裹仅在交互式 console host 下保护宿主；`pwsh -File` / `pwsh -Command` 脚本宿主下 exit N 仍杀进程"——本任务方案 E 完全承认此 limit 并通过 README `-NoExit` 改入口正面化解决；install.ps1 顶部注释 §3.2.2 显式引用 L44。
- **L45**（双层 param 同步）：本任务不加新顶层参数，自然不触发；install.ps1 L43-L45 既有注释保留。
- **L33**（`[CmdletBinding()]` 禁用）：本任务不引入 `[CmdletBinding()]`。
- **L37**（公网 IP 探测）：本任务不动 Get-PublicIPv4。
- **L47**（`.editorconfig` 后置 section 覆盖）：本任务不动 `.editorconfig`。
- **L48**（PM Stage 7 标题红线）：PM 在 07 必须写裸 `## Insight` 标题，不带数字前缀（archive-task 收割 regex 要求）；**T-028 已对 archive-task 加容错 regex**，本任务受益。
- **L49**（verify_all WARN 分支 Write-Host 前置 echo）：本任务的 E.8 / E.9 / E.10 均走 `throw` FAIL 分支，不涉 WARN，自然不触发。

### §9.2 候选新 Insight（本任务交付后追加到 `.harness/insight-index.md`）

> 由 PM 在 07 写 `## Insight` 段，QA 在 06 提供 evidence 实测；以下为占位草稿。

1. **2026-05-24 · PowerShell `-NoExit` 是 cmd / Win+R / Windows Terminal 入口让窗口在 `-Command` 跑完后不关闭的官方文档化 idiom**：`pwsh -NoExit -Command "..."` / `powershell -NoExit -Command "..."` 同语义；与 T-026 `& {}` 子作用域包裹**互补**——后者保护**已打开的**交互式 prompt 宿主，前者保护**新启动**的 -Command 形态宿主。任何"一键安装"类管道脚本如希望支持 cmd / Run 框 / Windows Terminal 三入口，推荐入口字串**必须**加 `-NoExit`；不加是 PowerShell 文档化"-Command 跑完即退"行为，**不是 bug**。 · evidence 占位：T-031 06 ADV-5 实测两入口字串对照截图 + PS5.1 / PS7 双版本下 -NoExit 生效
2. **2026-05-24 · `& { ... }` 子作用域内显式 `$global:LASTEXITCODE = 0` 是"成功路径"清零陈旧非零值的稳定 idiom**：PowerShell scriptblock 内 `exit N` 隐式 set `$LASTEXITCODE = N`，但若末尾不走 `exit` 而靠最后一条命令自然推断退出码（如 `Write-Host` 成功 → 0），在某些 PS 版本下 `$LASTEXITCODE` 可能保留前一条命令的陈旧值。`$global:VAR` 修饰符跨 scope 显式赋值（about_Scopes 文档化）让"成功路径 $LASTEXITCODE = 0"永远成立，与 T-026 子作用域外 `if ($LASTEXITCODE -ne 0)` 失败横幅判定可靠配合。 · evidence 占位：T-031 04 mini repro `pwsh -NoProfile -Command "& { $global:LASTEXITCODE = 0 }; $LASTEXITCODE"` 输出 0

---

## §10 Verdict

**READY FOR GATE REVIEW**

理由：

- §2 候选方案评估覆盖 5 个候选，给出维度齐全的打分矩阵；方案 E（A+C+静态闸门）被选定有显式理由。
- §3 实施分解精确到 file + 行号 + before/after diff 草稿；Developer 可机械执行无需再做设计决策。
- §4 风险登记含 RA 5 条 + 新发现 6 条（共 11 条）；每条配缓解动作。
- §5 验证策略与 RA AC-1 ~ AC-12 + AC-额外 一一映射；自动 / 手工标签明确；ADV 5 条覆盖核心闸门 + 真机入口。
- §6 接口约定锁住 README 推荐入口字串作为对外合约；E.10 静态闸门防回归。
- §8 实施步骤清单 14 步，每步可勾选且工时 ≤5 分钟。
- §9 已有 insight L33 / L37 / L44 / L45 / L47 / L48 / L49 全部对齐；候选新 insight 2 条带 evidence 占位。
- 红线全过：FR-3 / OOS-7 / [CmdletBinding] / T-026 BOM / T-019 sc.exe / 双层 param 同步 / OQ-5 a（install-service.ps1 不动）全部不破。
- Partition assignment：单 generic `developer` agent 一次性顺序执行所有步骤即可（§11 详）；无需真分区。

如 Gate Reviewer 在 03 判断本设计任一条目须用户重新决策，则改 BLOCKER 退回 PM；否则 PM 派发 Developer 进 Stage 4。

---

## §11 Partition assignment

`.harness/agents/dev-{frontend,backend,db}.md` 存在。检查 owned paths：

- `scripts/install.ps1` / `scripts/install-service.ps1` / `scripts/.editorconfig` / `scripts/baseline.json` —— Harness 脚本，**未在任何 dev-* 分区** owned paths（dev-backend.md 明文"Harness 脚本不归任何 dev-* 分区"）。**fallback 至 generic `developer`**。
- `scripts/verify_all.ps1` / `scripts/verify_all.sh` —— **dev-backend** owned（dev-backend.md）。
- `README.md` —— 非代码文档，无明确分区归属；**fallback 至 generic `developer`**。
- `docs/dev-map.md` —— 同上 fallback generic。

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `scripts/install.ps1` | `developer` (generic) | edit（删 `exit 0` + 注释追加） | — |
| `README.md` | `developer` (generic) | edit（入口字串改 + PS5.1 段 + "如你看到的是旧入口"段） | — |
| `scripts/verify_all.ps1` | `dev-backend` | edit（新增 E.8 / E.9 / E.10） | depends on install.ps1 + README.md（断言它们的状态）|
| `scripts/verify_all.sh` | `dev-backend` | edit（同步 E.8 / E.9 / E.10） | depends on verify_all.ps1（同步对账）|
| `scripts/baseline.json` | `developer` (generic) | edit（version + notes） | depends on verify_all 跑通 |
| `docs/dev-map.md` | `developer` (generic) | edit（scripts/ 块追加 T-031 注解） | — |
| `scripts/install-service.ps1` | **不改动**（OQ-5 a） | — | — |
| `scripts/.editorconfig` | **不改动** | — | — |

### Dispatch order

1. **`developer` (generic)**：install.ps1 + README.md（核心改动）
2. **`dev-backend`**：verify_all.ps1 + verify_all.sh（依赖前一步状态，跑闸门验证）
3. **`developer` (generic)**：baseline.json + dev-map.md（等前两步完成 + verify_all 跑通拿到准确 PASS=25 数字）

### Parallelism

**Step 1 与 Step 2 严格顺序**（Step 2 的闸门必须能 verify Step 1 的产物为 PASS）；Step 3 必须等 Step 2 verify_all 跑通。

**推荐 PM**：本任务规模小（5 文件 + 1 注解，纯局部改动），单 generic `developer` agent 一次性按 §8 步骤 1→14 顺序完成所有步骤即可，**无需真分区**；上表的 dev-backend 分配是契约层声明，实操可 fallback 给 generic developer。

---

READY-FOR-GATE-REVIEW
