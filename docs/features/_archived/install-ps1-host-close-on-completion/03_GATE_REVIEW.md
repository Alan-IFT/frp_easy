# 03 — Gate Review · T-031 install-ps1-host-close-on-completion

> Harness 流水线 Stage 3（Gate Reviewer）。模式：**full**。
> 上游：01_REQUIREMENT_ANALYSIS.md (Verdict=READY) + 02_SOLUTION_DESIGN.md (Verdict=READY FOR GATE REVIEW)。
> 输出契约：8 维审计 + A1-A3/B4-B8/C9-C11/D12-D14/E15-E16 独立结论 + Developer-facing notes + Verdict。

---

## §1 8 维审计速览

| # | 维度 | 结论 | 一句话 |
|---|---|---|---|
| 1 | Requirement completeness | PASS | 01 §3 FR-1~FR-10 全部可测试、无模糊；OQ 全部 [PM-resolved]。|
| 2 | Design completeness | PASS | 02 §3 给 10 条 FR + 12 条 AC 每条都有落地点 / 闸门 / mini repro。|
| 3 | Reuse correctness | PASS | 02 §3.2.1 引用 install.ps1 L389-L391 真实存在；T-026 `& {}` 包裹 + 失败横幅完全沿用；install-service.ps1 OQ-5 a 不动符合 `about_Scopes` 文档。|
| 4 | Risk coverage | PASS | 02 §4 RISK-1~5 (RA) + RISK-A~F (新) 共 11 条覆盖 -NoExit 入口 / WT setting / grep 误报 / scope 写全局 / baseline drift / PS5.1 替代字串。|
| 5 | Migration safety | PASS | 零 schema / 零 DB / 零 feature flag；改 README "对外合约"已用 E.10 闸门防回归 + "如你看到旧入口"段缓解过渡期。|
| 6 | Boundary handling | WARN | 见 E15（E.8 grep `(?m)` multiline 标志缺失）+ E16（sh `echo -e` 反斜杠解释风险），均不破红线但影响 dev 实施稳健性。|
| 7 | Test feasibility | PASS | 12 条 AC + AC-额外 全有"执行命令 / 期望输出 / 自动 或 手工"标签；ADV 5 条含正向+反向。|
| 8 | Out-of-scope clarity | PASS | 01 §6 OOS-1~7 + 02 §1.3 红线 + §3.2.6 / §3.2.7 显式"不改"清单防 over-build。|

---

## §2 审计点逐条结论

### A. 一致性与完备

#### A1. 01 §3 FR 全部在 02 §3 找到落地点？特别 FR-1 / FR-2 / FR-3 红线？

**PASS**。

- FR-1（推荐入口宿主不关）→ 02 §3.2.3 README 改 `pwsh -NoExit -Command "..."`（位置 1 L290-L307）+ §3.2.1 `$global:LASTEXITCODE = 0` 巩固 T-026 子作用域。
- FR-2（18 处 exit N 失败路径下宿主不退）→ 02 §3.2.1 不改 18 处 exit N + T-026 子作用域 + L398-L402 中文横幅原样保留（install.ps1 实读已确认 L398-L402 在）。
- FR-3（不引入 Read-Host/pause/ReadKey/Wait-Event 阻塞）→ 02 §3.2.4/§3.2.5 新增 E.8 静态闸门，pattern `Read-Host|\[Console\]::ReadKey|^\s*pause\s*$|Wait-Event` 直接命中 throw FAIL。
- FR-4 / FR-5 / FR-6 / FR-7 / FR-8 / FR-9 / FR-10 各有 §3.2.X 对应；FR-10 README diff 明示，FR-8 E.9 闸门兜底，FR-9 OQ-5 a + AC-11 mini repro。

#### A2. 01 §5 AC 全部在 02 §5 找到验证策略？标签合理？

**PASS**。

- 02 §5 矩阵覆盖 AC-1~AC-12 + AC-额外；自动 [A] / 手工 [U] 标签明确（AC-1/2/3/4/6/7/8/11/12 = 手工 + 真机 PS7；AC-5/9/10/额外 = 自动 verify_all）。
- 标签合理性：AC-1（真机交互式 PS7 复测）标手工，符合 insight L44 "QA 不能用 pwsh -File mock 证伪宿主存活"红线；NFR-5 与之对齐。
- 唯一注意：AC-11 标 "手工 [U] mini repro (5 分钟可执行); 可选 [A]"——dev 实施时如选 [A] 路径，需把 `scripts/.t031-mini-repro.ps1` 临时脚本归类到 E.7 BOM 白/黑名单，否则触发 E.7c WARN。建议 mini repro 不落到 `scripts/` 而落 `docs/features/install-ps1-host-close-on-completion/` 内（与本任务文档同目录），就不触发 E.7c。

#### A3. 01 §6 OOS 是否在 02 中被尊重？

**PASS**。

- OOS-7（不在脚本内"兼容"(b)/(d)/(e) -Command 跑完即退）→ 02 §1.3 红线表 + §3.2.2 注释"脚本侧无法逆转（除非引入 Read-Host 类阻塞，破 FR-3 红线）"显式承认。
- OOS-3（卸载脚本不动）→ 02 §3.2 范围不含 uninstall-service.ps1；§11 partition 表无该文件。
- OOS-5（Wait-ServiceRunning 30s 不调整）→ 02 §3.2.6 install-service.ps1 字节零变；OQ-5 a 决议显式锁。

### B. 技术可行性

#### B4. 02 §3.2.1 `$global:LASTEXITCODE = 0` 跨 scope 写全局可行？

**PASS**。

- PowerShell `about_Scopes`：`$global:VAR` 是显式 scope 修饰符，**任何 scope** 内对 `$global:VAR` 赋值都会写入根 scope。`& { ... }` scriptblock 是 child scope，但 `$global:LASTEXITCODE = 0` 一行直接打穿到根 scope。
- 02 §4 RISK-D 缓解动作给了一行 mini repro：`pwsh -NoProfile -Command "& { $global:LASTEXITCODE = 0 }; $LASTEXITCODE"` → 期望输出 `0`。可在 04 步骤 9 跑前 5 秒验证。
- 但 nuance：`$LASTEXITCODE` 是**自动变量**，PowerShell 在每条 native cmd 退出后会**重写**它。若 `} @PSBoundParameters` 这行**之后**到 `if ($LASTEXITCODE -ne 0)` 之间不再有任何 native 调用，`$global:LASTEXITCODE = 0` 的写入会保持；当前 install.ps1 子作用域结束 → 紧跟 L398 if，路径上无 native 调用，安全。

#### B5. README `pwsh -NoExit -Command "..."` 三场景下能阻止窗口关闭？

**PASS**（设计已在 02 §1.2 / §2.1 引用 MS `about_PowerShell_Exe` 文档"-NoExit: Don't exit after running startup commands"）。

- 01 §1.2 表 (b)/(d)/(e) + 02 §2.1 评估明确：`pwsh -NoExit` 让 pwsh 在 -Command 跑完后**不退出**进入交互式 prompt → 进程不退 → 承载窗口（cmd / Win+R 启动的 pwsh 自身窗口 / Windows Terminal tab）保留。
- 三场景细节：
  - cmd 启动 `pwsh -NoExit -Command "..."`：cmd 窗口承载的子进程是 pwsh，pwsh 不退 → cmd 子进程不退 → cmd 窗口保留。
  - Win+R `pwsh -NoExit -Command "..."`：Run 框直接启动 pwsh.exe，窗口主进程就是 pwsh 自身 → -NoExit 让 pwsh 不退 → 窗口保留。
  - Windows Terminal：默认 setting "close on exit"=auto/graceful 即"进程 exit code = 0 时关 tab"，但 -NoExit 让进程不 exit → tab 不关。
- RISK-A 已识别 WT setting "close on exit"=always 的极少数极端用户会 override → 02 §4 RISK-A 缓解给出"README 注脚附加 'WT setting check' 兜底信息（在 ADV-5 验证）"。

#### B6. E.8 / E.9 / E.10 闸门实现是否漏 / 误报？

**WARN**（**关键 bug**，见 E15 / E16）。

- E.8 在 install-service.ps1 误报风险：grep pattern `Wait-Event`（精确字面量，含连字符）**不会**匹配 install-service.ps1 内的 `Wait-ServiceStopped` / `Wait-ServiceMarkedDeleteCleared` / `Wait-ServiceRunning`（这些函数名是 `Wait-Service*` 不含 `Event`）。已实读 install-service.ps1 用同款 pattern grep 结果"No matches found"——验证 OK，**E.8 不会误报这些 Wait-* 函数**。
- E.8 在 install.ps1 当前命中：实读用 pattern grep → "No matches found"。新代码加入后只要不引入这 4 模式即 PASS。
- **但 E.15 揭示了 pattern 本身有 bug**：`$content -match '^\s*pause\s*$'` 在 `Get-Content -Raw` 拿到的整文件单字符串上不会按行匹配 `^` / `$`（需要 `(?m)` multiline 标志）。这是个潜在漏报 bug：未来若有人加一行裸 `pause`，E.8 不会命中，FR-3 红线悄悄失守。详见 E15。
- E.9 实现 `Get-ChildItem -Path "scripts" -File | Where-Object { $_.Name -match '^install.*\.(cmd|bat)$' }` 边界 OK：仅 scripts/ maxdepth 1（不递归到子目录）；模式 `^install.*\.(cmd|bat)$` 含 `^` 锚定，不会误匹配 `not-install-wrapper.cmd`。
- E.10 实现 multiline regex `(?ms)\*\*Windows\*\*[^\n]*\n+` + 代码块匹配正确 `(?ms)` 标志；正则中 `[^backtick]+?` 排除反引号让 lazy 匹配在第一个三反引号处停。但**风险**：README §"Windows" 段后还有"PowerShell 5.1"段也含 powershell code block，正则 lazy 匹配只取第一个 → 取的是首字 Windows 段，正确。不过若未来重排顺序导致 PS5.1 段在 Windows 段**之前**，E.10 会误抓 PS5.1 段（仍含 -NoExit 故不会假 FAIL，但语义漂移）。02 §6.1 已锁"README 推荐入口字串作为对外合约"，运维侧约束 OK。

#### B7. baseline.json PASS 数字 22→25 是否准确？

**WARN（数字漂移）**。

- 已实读 `scripts/verify_all.ps1` 数 Step 调用 = **22**（grep `^Step\s+"|^\s+Step\s+"` 计数）。Step 详细：A.1/A.2/A.3 (3) + G.1/G.2/G.3 (3) + B.1/B.2/B.3/B.4/B.5 (5) + C.1 (1, full only) + D.1 (1) + E.1/E.2/E.3/E.4/E.5/E.6/E.7a/E.7b/E.7c (9) = 22 个 Step。
- Quick 模式去 C.1 = 21；Full = 22。
- baseline.json 当前 notes 写 "verify_all 仍 PASS 22"。但**这只是 notes 字串里的旧基线值**，与实际 verify_all 跑通后的 PASS 数无关——`verify_all` 内 `pass = ($report | Where-Object status -eq "PASS").Count` 是动态算，依赖每个 Step 实际状态（PASS vs SKIP vs WARN）。
- 02 §5.1 设计写"Quick 22→25 / Full 23→26"——基线"22"和"23"是设计者凭印象写的数字。实测 Quick = 21 (D.1 WARN 不算 PASS / openapi.yaml 实存在故 D.1 应 PASS 看 README 写"根目录 openapi.yaml"——实读未确认；G.X 也依赖 Go env 是否可用)。
- **结论**：dev 实施后必须以**实跑 verify_all 输出的 PASS 数**为准写 baseline.json notes，而不是照搬设计的 "22→25"。若 dev 实跑 PASS=21，新增 3 个则 24；若实跑 22 则 25。建议 04 步骤 9 跑后用实际数字 + "(基线 21/22 + 3)" 双标。

#### B8. AC-11 mini repro 设计能证伪 R2？

**PASS**。

- 02 §5 AC-11 + §5.2 ADV-4 给的命令 `pwsh -NoExit -Command "& { & 'C:\tmp\inner-exit-1.ps1' ; 'still alive' }"`（inner = `exit 1`），期望 "still alive" 被打印 + `$LASTEXITCODE = 1`。
- 这个 repro 直接对应 01 §2 R2 假设"`& script.ps1` 创建独立 scope 让 exit N 退该 scope 不泄漏"。
- 真证伪要点：`& 'C:\tmp\inner-exit-1.ps1'` 用 call operator 调磁盘 .ps1，按 `about_Scopes` 应进 script scope，`exit 1` 仅退该 scope，外层 `'still alive'` 应执行。
- 边界：必须把 mini repro 跑在**真机 PS7 host**（不能是 `pwsh -File` mock 形态），否则 -NoExit 本身就让外层 prompt 不退，与 inner 是否泄漏 exit 无关。设计已用 `pwsh -NoExit -Command` 包裹整段，是正确选择（让外层 prompt 持续以便看到 `'still alive'` + 后续 `$LASTEXITCODE` 探查）。

### C. 风险与缓解充分

#### C9. 02 §4 RISK-A~F 缓解动作具体可执行？

**PASS**。

- RISK-A（WT setting "close on exit=always"）→ "README 注脚附加 'WT setting check' 兜底信息（ADV-5 验证）"——具体（README 加一行 + ADV-5 测试）。
- RISK-B（E.8 grep 误报）→ "已在 §3.2.4/§3.2.5 实现里加排除：跳过 # 开头注释 + 跳过含 禁/forbidden/FR-3/red.?line 元描述词的行"——具体且已在伪代码里实现。
- RISK-C（E.10 README 重排导致段落识别失败）→ "实际是'防回归'非'防初始'，重排导致 step throw 而非误绿"——具体（fail-safe 方向）。
- RISK-D（$global:LASTEXITCODE = 0 写全局可行性）→ "Developer 在 04 步骤执行后 mini repro 实测应输出 0（5 秒可验）"——具体可执行。
- RISK-E（baseline PASS 数字漂移）→ "§3.2.8 baseline notes 已更新到 25；Code Reviewer 在 05 核对此数字"——具体（但参见 B7 / E15 修正建议：以实跑数字为准）。
- RISK-F（用户机器无 pwsh，PS5.1 替代字串）→ "§3.2.3 README PS5.1 段已给 `powershell -NoExit -Command "..."` 替代"——具体。

非"将来注意"空话，全部可执行。

#### C10. README 入口字串变"对外合约"是否需要 E.10 闸门？

**PASS**。

- 02 §6.1 把 README 入口字串显式声明为"对外合约"，与代码改动等价对待。
- E.10 闸门防回归价值高：未来任何 PR 改 README "Windows" 段去掉 -NoExit（如"为了简化"或"误删"）→ E.10 throw FAIL → CI 拦。
- 比 "README test 已足"更强：README 没有专门测试套件（无 ADV / E2E），单靠人审 PR 看 README diff 容易漏，E.10 静态正则是稳定的程序化兜底。
- 类比 E.6（adversarial tests section regex）/ E.7a/b（BOM 白黑名单）—— 同款 "文档/字节级合约的程序化闸门" 模式，与项目既有惯例一致。

#### C11. 02 §9.2 候选 insight 2 条 evidence 占位 QA 能在 06 真实测产生？

**PASS**。

- Insight 1（PowerShell -NoExit 文档化 idiom）→ evidence "T-031 06 ADV-5 实测两入口字串对照截图 + PS5.1/PS7 双版本下 -NoExit 生效"。02 §5.2 ADV-5 已设计该测试，QA 截图即 evidence。
- Insight 2（`$global:LASTEXITCODE = 0` 跨 scope 写全局 idiom）→ evidence "T-031 04 mini repro 输出 0"。02 §4 RISK-D 给了完整 5 秒 mini repro，QA 在 06 粘 stdout 即 evidence。
- 两条都"易测可证"，符合 insight 写作"证据支持"红线。

### D. 红线

#### D12. 是否破任一红线？

**PASS**。

- FR-3（无 Read-Host/pause/ReadKey/Wait-Event）→ 02 全文 0 处引入；E.8 闸门兜底。
- OOS-7（不在脚本内兼容 (b)/(d)/(e) -Command 退）→ 02 §3.2.2 注释明文承认 limit，方案 E 走 README -NoExit 不破。
- `[CmdletBinding()]`（T-024 / insight L33）→ install.ps1 改动只在 L389-L391 + L26-L34 注释；不引入 `[CmdletBinding()]`（实读 install.ps1 L35 仅有裸 `param([switch]$Help)`）。
- T-026 BOM（install.ps1 禁 BOM / 其余必须 BOM）→ 02 §3.2.7 显式"`.editorconfig` 不改"；install.ps1 修改是文本内容不改首字节，BOM 状态不变。
- T-019 sc.exe binPath（不引入 wrapper.cmd）→ 02 §3.2.6 install-service.ps1 字节零变；E.9 闸门兜底拒 install*.cmd/bat。
- 双层 param 同步约束（insight L45）→ 02 §3.2.1 改动**不加新参数**，顶层 + 内层 param 同步状态不变；02 §1.3 红线表已锁。
- OQ-5 a（install-service.ps1 不动）→ 02 §3.2.6 显式遵守。

7 条红线全过。

#### D13. OQ-5 a 决议安全？AC-11 mini repro 若 FAIL 升级路径清晰？

**PASS**。

- OQ-5 a 安全性：依赖 PowerShell `about_Scopes` "Using the call operator to run a function or script runs it in script scope"——这是 PowerShell 官方文档化合约（自 v3.0 起稳定），install-service.ps1 通过 `& $svc` 调用本就跑在独立 script scope，其内 `exit N` 退该 scope 不泄漏。
- AC-11 mini repro 是对此合约的运行时验证（动态自检），与 02 §5 表中"失败回滚"列对齐："若 'still alive' 不打印 → R2 触发 → 升级 OQ-5 b（install-service.ps1 也加 `& {}` 包裹）"——升级路径清晰、单点决策、范围小。
- 注：升级到 OQ-5 b 时，install-service.ps1 必须从 BOM 白名单（E.7a）保留；改首字节会破 BOM 红线。02 设计未在 OQ-5 b 升级路径里明示这点，但 dev 真实施时会自然遇到 E.7a fail 提示。Trivial。

#### D14. README 改动是否破坏"安全提示" + "国内 VM" 段？

**PASS**。

- 已实读 README 当前 L77 "> 安全提示" + L79 "#### 国内 VM 公网 IP 探测兜底"段。
- 02 §3.2.3 "位置 2" 明确写"L77 之后保留'安全提示' + '国内 VM' 段不动"。
- 02 §3.2.3 改动范围 = L67-L78（Windows 块 + PS5.1 段），位置精确未跨进 L77 后的"> 安全提示"行。
- 但**风险**：dev 实施时若按"L67-L78 区段全替换"机械操作，可能误把 L77 "> 安全提示"行一并替换掉。建议 dev 实施时**先 diff before/after 行号范围**→ 替换 ≤ L77 之前的内容，L77 起的 "> 安全提示" + 后续段原样保留。详见 Developer-facing notes §3。

### E. 工程基线

#### E15. E.8 `Get-Content -Raw` + `-match '^\s*pause\s*$'` multiline 标志 bug

**FAIL**（必须修，否则 FR-3 红线悄悄失守）。

- 实测语义：`Get-Content -Raw -Path $t` 返回整文件单字符串（含 LF/CRLF 换行符）；后续 `$content -match '^\s*pause\s*$'` 中 `-match` 是 `[regex]::Match` 默认无 multiline → `^` 仅匹配字符串**首位**、`$` 仅匹配字符串**末位**。
- 影响：若 install.ps1 / install-service.ps1 中某中间行写裸 `pause`（前后空格），`-match` **不会**命中。FR-3 红线悄失守。
- 修复（dev 实施时务必加）：把 `'^\s*pause\s*$'` 改为 `'(?m)^\s*pause\s*$'`（加 `(?m)` multiline 标志让 `^`/`$` 按行匹配）。
- 其他 3 个 pattern 不受影响：`Read-Host` / `\[Console\]::ReadKey` / `Wait-Event` 都是字符级匹配，不依赖 `^`/`$`。
- 同时影响后续 `Select-String -Pattern $pat`：Select-String 默认按行扫描，`^`/`$` 对它来说是"行首/行尾"语义，**不需要** `(?m)`。即 Select-String 内的 `^\s*pause\s*$` 命中 OK。所以**只有第一层 `$content -match $pat`** 需要修。
- 但 dev 别只改 Select-String 内的 pattern——`$content -match $pat` 是"先快筛是否含命中"的捷径；若它漏报，Select-String 那一步根本不会执行（条件分支被跳）。
- **追加建议**：把 `if ($content -match $pat) { ... Select-String ... }` 改为直接 `$lines = ... Select-String -Pattern $pat ...; if ($lines) { ... }`，去掉先 -match 再 Select-String 的两段式逻辑，单走 Select-String 一道（既能 multiline 又能拿行号）。

#### E16. E.8 bash `e8_hits="$e8_hits\n$t:$ln"` + `echo -e $e8_hits` 风险

**WARN**（不破红线但影响可读性 / 罕见路径下 mangling）。

- 实测语义：bash 字符串赋值里 `\n` 是字面 2 字符（反斜杠+n），不会展开为 LF；最终用 `echo -e` 把字面 `\n` 解释为 LF。
- 风险：若 `$ln`（grep -n 输出的 `行号:内容` 字串）中**内容部分**含反斜杠（PowerShell .ps1 文件路径用 `\` 常见），`echo -e` 会把这些反斜杠解释为转义序列，扭曲输出。
- 影响：仅在 FAIL 路径下的诊断字串显示扭曲，**不影响 PASS / FAIL 判定本身**。是 NIT 级别。
- 修复建议（dev 可选）：改用 `printf '%s\n' "$e8_hits"`（printf 不解释 `\n`）。
- 但与项目既有 verify_all.sh 同款 idiom（E.5 / E.6 / E.7a/b/c）一致——全项目惯例。如果 dev 严守"与既有惯例对齐"，可不改。

---

## §3 Developer-facing notes（落 04_DEVELOPMENT.md 时务必参考）

1. **【必须修】E15 multiline 标志**：02 §3.2.4 中 `$content -match $pat` 这一层对 `'^\s*pause\s*$'` pattern **必须**加 `(?m)` 前缀；否则裸 `pause` 行漏报，FR-3 悄失守。建议把 `$forbidden` 数组里的 `'^\s*pause\s*$'` 直接改为 `'(?m)^\s*pause\s*$'`，或者用 E15 建议的"单走 Select-String"路径。02 §3.2.5 bash 版用 `grep -nE` 按行扫描天然 multiline，**不受此 bug 影响**，无需修。

2. **【实施数字】B7 baseline PASS 数字**：以**实跑 verify_all 后的 Summary 输出**为准写 baseline.json notes，不要照搬 02 §5.1 的"22→25 / 23→26"。建议格式："verify_all PASS = <实测数字>（baseline <旧数字> + 3 = E.8/E.9/E.10）"。

3. **【边界保护】D14 README 改动范围**：02 §3.2.3 改 README L67-L78，但 L77 ">安全提示"行 + L79 "国内 VM" 段必须**原样保留**。dev 操作建议：用 `Edit` 工具按"old_string=L67-L75 既有 12 行 / new_string=新 22 行"做精确替换，**不要**用"删除 L67-L78 整段再插入"方式（会误删 L77 安全提示）。

4. **【AC-11 落地】**：mini repro 临时脚本（如 `inner-exit-1.ps1`）建议落在 `docs/features/install-ps1-host-close-on-completion/.scratch/` 或 `C:\tmp\` 系统临时目录，**不要**落 `scripts/`（避免触发 E.7c 未分类 WARN）。06 QA 留 stdout 证据后可删。

5. **【OQ-5 b 升级路径备忘】**：若 AC-11 实测 "still alive" 未打印（R2 真），需把 install-service.ps1 加 `& { param([string]$DisplayName="FRP Easy", [string]$ServiceName="frp-easy") ... } @PSBoundParameters` 包裹；包裹时**只**改 BOM 之后的内容，**不改首 3 字节 EF BB BF**（保 BOM 白名单 E.7a）。

6. **【已验证项】**：E.8 grep pattern `Wait-Event` 字面量 + install-service.ps1 含 `Wait-ServiceStopped` / `Wait-ServiceMarkedDeleteCleared` / `Wait-ServiceRunning` 共 3 个 `Wait-*` 函数，已实测 grep "No matches found"——**不会误报**。

7. **【已验证项】**：install.ps1 / install-service.ps1 当前**零命中**所有 4 个 forbidden pattern——E.8 实施后立即 PASS，无需 retrofit。

---

## §4 Verdict

**APPROVED WITH CONDITIONS**

理由：

- 8 维审计 7 PASS + 1 WARN（Boundary handling）。
- 关键 bug **E15**（E.8 `(?m)` 缺失）会让 FR-3 红线在裸 `pause` 场景悄失守 → 必须在 04 实施时修；非阻塞性（dev 加一行 `(?m)` 即过），不退回 02。
- E16 是 NIT 级（与项目既有 sh idiom 对齐可不改）。
- B7 / D14 是实施期注意事项，非设计缺陷。

**Conditions（Developer 在 04 必须满足，否则 Code Reviewer / Gate Reviewer 二审会 BLOCK）**：

1. **C-1**：02 §3.2.4 E.8 step 内 `'^\s*pause\s*$'` 模式**必须**带 `(?m)` multiline 前缀（或重构为"单走 Select-String"逻辑去掉先 -match 再 Select-String 的两段式）。详 E15。
2. **C-2**：baseline.json notes 中 "PASS N" 必须以**实跑 verify_all 后的 Summary 数字**为准，不照搬设计的 22→25。详 B7。
3. **C-3**：README L67-L78 改动**不得**误删 L77 ">安全提示"行 + L79 "国内 VM" 段；建议 Edit 工具精确字符串替换。详 D14。
4. **C-4**：AC-11 mini repro 临时脚本若产生**不得**落 `scripts/`（避免触发 E.7c WARN）；落任务目录或 `C:\tmp\`。详 Developer-facing notes §4。

满足以上 4 条 conditions 后，Developer 可执行 02 §8 步骤 1→14；PM 接 04 后派 Code Reviewer / QA。

---

— Gate Reviewer, 2026-05-24
