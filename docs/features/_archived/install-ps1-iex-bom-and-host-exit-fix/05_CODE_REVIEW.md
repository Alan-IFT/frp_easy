# 05 — Code Review · T-026 install-ps1-iex-bom-and-host-exit-fix

> Harness 流水线 Stage 5（Code Reviewer）。模式：**full**。
> 上游：01_REQUIREMENT_ANALYSIS.md（READY）+ 02_SOLUTION_DESIGN.md（READY）+ 03_GATE_REVIEW.md（APPROVED WITH 4 MAJOR CONDITIONS）+ 04_DEVELOPMENT.md（READY FOR REVIEW）。
> Reviewer 工具集仅 Read/Glob/Grep（insight L41/L48 已知模式）——本文由 PM 代落盘，Reviewer 在消息体给完整内容。

---

## §1 总评

**整体高质量**。Developer 按 02 §6 12 步全程执行 + 03 §8 4 条 MAJOR 必修条件（G-6 / G-7 / G-8 / G-15）**全部增补落实**，并且每条增补在代码里有显式 "依 03 §8 G-N 增补" 注释锚点便于未来溯源。verify_all Quick 19 → 21（baseline 干净）= E.7 拆 a/b/c 净 +2，无新 FAIL/WARN。install-service.ps1 / uninstall-service.ps1 字节零变（PM 已 spot-check 二次确认）。AC-3/9/10/12/13/14/15/16 全部 [A] 类直接闭环；[U] 类合理地交 QA 06 / 用户真机。

**核心修复方案落地正确**：install.ps1 首字节确为 `0x23 (#)`、无 BOM、CR=0、size=18184、line=402；主体 `& { param([switch]$Help) ... } @PSBoundParameters` 包裹完整（`& {` 在 L46、`} @PSBoundParameters` 在 L392，包裹内 16 处 `exit N` 与改前数量一致）；末尾失败横幅在包裹外（L398-L402）；scripts/.editorconfig 加 `[install.ps1]` 例外块覆盖 `[*.ps1]`；verify_all.ps1 / verify_all.sh 双端 E.7a/b/c 拆分 + G-7 unclassified 文件名打印；baseline.json 仅改 version/updated/notes（test_count 等保持 342）；dev-map.md scripts/ 块追加 T-026 一行注解。

**仅有少量 MINOR / NIT 偏离**（详见 §6）：(a) 02 §3 表"L23-L25 注释"行原本要求"删除 T-024 留下的旧 `[CmdletBinding()]` 注释"，Developer 实际保留并叠加 T-026 注释（保留历史上下文，无功能影响）；(b) 02 §1 提及顶层除 `param` 外还应有 `$ErrorActionPreference="Stop"`，Developer 将 Stop 放在内层 scriptblock 第一句（与 02 §4.1 "After" 伪码一致，无功能影响）；(c) 04 §3.1 揭示"`& { exit N }` 仅在交互式 PowerShell console host 下保护宿主，脚本宿主下仍杀进程"这个 nuance 已在 04 errata 记录但**未触发任何代码改动**——这点是 02 D-2 论断的边界条件细化，Reviewer 同意 Developer 的处理（用户真实使用场景仍是交互式宿主，FR-4/5 满足）。**没有 CRITICAL/MAJOR**。

**裁决：APPROVED**。建议 PM 派 QA 06 真机覆盖 [U] AC。

---

## §2 文件清单（已审视）

- `c:\Programs\frp_easy\scripts\install.ps1` （402 行，18184 B，BOM=False，CR=0）
- `c:\Programs\frp_easy\scripts\verify_all.ps1` （15378 B；E.7 拆 E.7a/b/c）
- `c:\Programs\frp_easy\scripts\verify_all.sh` （13758 B；同款拆分）
- `c:\Programs\frp_easy\scripts\.editorconfig` （15 行；追加 `[install.ps1]` 例外块）
- `c:\Programs\frp_easy\scripts\baseline.json` （12 行；version 11→12，notes 更新）
- `c:\Programs\frp_easy\docs\dev-map.md` （scripts/ 块第 29 行追加 T-026 注解）

对照 04 §7 "Files changed" 清单：**6/6 全审**。未审 install-service.ps1 / uninstall-service.ps1（FR-9 / AC-10 约束字节零变，PM 已 spot-check 二次确认；Reviewer 不重复，但通过文件大小 / BOM=True / CR=0 字节级特征已记录）。

---

## §3 AC 逐条核查表（01 §5 全部 18 条 AC）

| AC | 类型 | 实现位置 | 状态 | 备注 |
|---|---|---|---|---|
| **AC-1** | [M][U] | install.ps1 删 BOM；首字节 = `0x23` | [U] | PS5.1+zh-CN 真机由 QA 06 / 用户验；Developer 步骤 10 PS7 mock 已正向 |
| **AC-2** | [M][U] | 同上 | [U] | PS7 上 04 §2.10 已通；PS5.1 留 QA |
| **AC-3** | [A] | install.ps1 首 3 字节 = `23 20 69`，verify_all E.7b PASS | PASS | 04 §2.5 字节级断言 stdout 已录 |
| **AC-4** | [M][U] | install.ps1 L46-L392 `& { ... } @PSBoundParameters` 包裹 16 处 `exit N` | [U] | 04 §3.1 揭示自动化场景与真机场景行为差异；用户真机交互式宿主下 FR-4/5 满足；标 [U] 合理 |
| **AC-5** | [M][U] | install.ps1 L298-L313 `& $svc` + `$LASTEXITCODE` 透传逻辑保持原样，被 `& { ... }` 包裹 | [U] | 同 AC-4 |
| **AC-6** | [U] | 端到端，需用户真机 8/8 + sc query frp-easy | [U] | 留 QA 06 |
| **AC-7** | [M] | install.ps1 L398-L402 失败横幅 + 既有 stderr Write-Error；04 §9 第 3 条警告自动化无法精确验，需真机 | [U] | 横幅代码实现 ✓；触发可观测留真机 |
| **AC-8** | [U] | install.ps1 L109-L145 -Help 分支保留；顶层 + 内层双层 `param([switch]$Help)` + `@PSBoundParameters` splat | [U] | 04 §2.11 PS7 端 ExitCode=0 + Help 显示 ✓；PS5.1+zh-CN 留用户（D-1 接受中文乱码） |
| **AC-9** | [A] | 04 §2.11 `pwsh -NoProfile -File scripts/install.ps1 -Help` → ExitCode=0 + Help 输出 | PASS | 已实证 |
| **AC-10** | [A] | install-service.ps1 / uninstall-service.ps1 首 3 字节 = `EF BB BF`；git diff --stat 空输出（04 §2.12 已断言；PM 二次 spot-check：BOM=True、size 9708/3993、CR=0）| PASS | 字节零变 |
| **AC-11** | [U] | 端到端，需 PS5.1+zh-CN 真机 | [U] | 留用户真机 |
| **AC-12** | [A] | verify_all.ps1 L288/L305 含 "install.ps1" + "BOM" + "iex-entry" 命名；verify_all.sh L301/L317 同款 | PASS | grep 友好命名达成 |
| **AC-13** | [A] | 04 §2.10 ADV-1：BOM 加回 install.ps1 → E.7b FAIL；错误信息含 "install.ps1" 与 "MUST NOT have UTF-8 BOM" | PASS | 负向自检通过 |
| **AC-14** | [A] | 04 §2.10 ADV-2：删 install-service.ps1 BOM → E.7a FAIL；E.7a step 名含 "BOM-required scripts/*.ps1 have UTF-8 BOM" 保留 T-021 对其余 10 个 .ps1 的覆盖 | PASS | T-021 防回归覆盖未丢失 |
| **AC-15** | [A] | 04 §5.2 verify_all PASS Quick 19→21（full 模式预期 20→22），baseline.json version 11→12 + notes 同步 | PASS | 计数不下降（实际 +2） |
| **AC-16** | [A] | docs/dev-map.md L29 追加 T-026 注解（与 L28 T-021 注解并列） | PASS | 文档同步 |
| **AC-17** | [A] | 留 QA 06 `## Adversarial tests` 裸标题 | [Pending QA] | 由 QA 06 落实，Code Review 阶段无需验 |
| **AC-18** | [A] | 留 PM 07 `## Insight` 裸标题 | [Pending PM] | 由 PM 07 落实 |

**核查结论**：8 条 [A] 全 PASS；8 条 [U] / [M] 类合理标延后；AC-17/AC-18 是 stage 6/7 责任，本阶段无遗漏。**无 missing criterion = 0 CRITICAL**。

---

## §4 BC 逐条核查表（01 §4 全部 12 条）

| BC | 实现承接 | 状态 |
|---|---|---|
| **BC-1** PS5.1 + zh-CN | 删 BOM + `& { ... }` 包裹 + 失败横幅；磁盘形态接受中文乱码（D-1） | [U] 真机 |
| **BC-2** PS7 + 任意 cp | 04 §2.10 / §2.11 PS7 端实证通过 | PASS |
| **BC-3** 非管理员 exit 1 | install.ps1 L151-L154 `Write-Error` + `exit 1` 在 `& { }` 内 | [U] 真机；自动化模拟 ADV-5 留 QA 06 |
| **BC-4** 非 amd64 | install.ps1 L164-L167 | 同上 |
| **BC-5** 403/404 | install.ps1 L186-L196 | 同上 |
| **BC-6** 下载失败 | install.ps1 L242 | 同上 |
| **BC-7** 解压失败 | install.ps1 L250/L257/L263 | 同上 |
| **BC-8** install-service.ps1 透传 | install.ps1 L308-L313 `$LASTEXITCODE = 0` reset + `& $svc` + 透传 `exit $LASTEXITCODE`；被 `& { }` 包裹 | 留 QA 06 ADV-5 |
| **BC-9** 成功 exit 0 | install.ps1 L391 在 `& { }` 内 | [U] 真机 AC-6/AC-11 |
| **BC-10** tmpDir 清理 | install.ps1 L314-L316 `try { ... } finally { Remove-Item -Recurse -Force $tmpDir.FullName -ErrorAction SilentlyContinue }` 完整保留在 `& { }` 内；PS 在 scriptblock 内 `exit N` 走 finally（验 04 §7 Dev-Q2） | PASS（逻辑层）；留 QA ADV mock 触发 exit 后 `Test-Path $tmpDir` = $false |
| **BC-11** 不可见前缀防御 | E.7b 仅断言 `EF BB BF`；U+200B / U+00A0 等其他不可见字符未覆盖（02 §2 D-5 已决议不选 (b) "前 N 字节纯 ASCII"以避免误报；接受此 trade-off） | 接受（02 D-5 明示） |
| **BC-12** `$ErrorActionPreference="Stop"` 传播 | install.ps1 L51 内层 scriptblock 第一句设 Stop；02 §7 R-2 已分析 child-scope shadow / read-through 语义；不影响 try/catch | PASS |

**BC 全覆盖；无遗漏**。

---

## §5 03 §8 4 条 MAJOR 必修条件增补复核

### G-6 [PASS] `install.ps1` 双层 `param([switch]$Help)` 上方注释 `-Verbose`/`-Debug` 不支持

- 顶层注释 install.ps1 L33-L34、内层注释 L41-L42 均含 "本脚本仅支持 -Help 参数；-Verbose/-Debug 等 cmdlet common parameter 需要 [CmdletBinding()]，因 T-024 / insight L36 已不可加，故不支持（依 03 §8 G-6 增补）"。
- **措辞合规**：明确引用 T-024 + insight L36，并标注"03 §8 G-6 增补"溯源。**PASS**。

### G-7 [PASS] verify_all.ps1 E.7c WARN 分支真打印 unclassified 文件名

- PS 端 verify_all.ps1 L329-L335：`Write-Host -ForegroundColor Yellow "       unclassified: $($unclassified -join ', ')"; return $false`。
- sh 端 verify_all.sh L329-L335：`echo "    unclassified:$(echo -e $e7c_unclassified)"` 在 step WARN 之**前**调用。
- 04 §2.10 ADV-3 已实证：`unclassified: fake.ps1` + `WARN` 显示。**PASS**。

### G-8 [PASS] 04 在 §"adversarial 冒烟 / mock 步骤" 段显式注明"必须在 PS7 主机跑"

- 04 §2.10 段开头 + §4.3 重申同款 G-8 增补。**PASS**。

### G-15 [PASS] install.ps1 注释"未来加新顶层参数必须同步内部 scriptblock param"

- install.ps1 L43-L45：`未来在 install.ps1 加新顶层参数时，必须同步在内部 scriptblock param 块加同名同类型参数（@PSBoundParameters splatting 要求 hashtable key 与内层 param 严格对应；否则报"找不到接受实际参数的位置参数"或静默错位）（依 03 §8 G-15 增补）。`
- 位置：紧贴 `& {` 上方，是 splatting idiom 的紧邻 context，未来开发者必读位置。**PASS**。

**4/4 必修条件全部 PASS**。

---

## §6 逐文件审视

### §6.1 `scripts/install.ps1`

| # | 类型 | 严重度 | 位置 | 描述 |
|---|---|---|---|---|
| C-1 | LOGIC | OK | L46-L392 | `& { param([switch]$Help) ... } @PSBoundParameters` 包裹完整闭合；包裹内 16 处 `exit N` 数量与改前一致（grep 验证）；包裹外仅顶层 `param` + L398-L402 失败横幅。**结构正确**。 |
| C-2 | LOGIC | OK | L51 | `$ErrorActionPreference = "Stop"` 在内层 scriptblock 第一句，与 02 §4.1 "After" 伪码一致；scope rule 上是 child-scope shadow，不污染父 scope。 |
| C-3 | LOGIC | OK | L314-L316 | `try { ... } finally { Remove-Item ... $tmpDir.FullName -ErrorAction SilentlyContinue }` 完整保留在 `& { }` 内；BC-10 / 02 Dev-Q2 论断"scriptblock 内 exit 会走 finally"成立。 |
| C-4 | LOGIC | OK | L398-L402 | 失败横幅在 `& { }` 退出后 `if ($LASTEXITCODE -ne 0)` 触发；红字（`-ForegroundColor Red`）但**走 stdout 不是 stderr**——RA FR-6(a)"stderr 错误行"由既有 `Write-Error` 满足；FR-6(c)"明确中文失败横幅"由本 if 块满足；组合达 D-3 (a)+(c)。 |
| C-5 | DESIGN | MINOR | L23-L34 | 02 §3 表"L23-L25 注释"行的设计指示是"**删除** T-024 留下的旧 `[CmdletBinding()]` 注释（已无意义）"。Developer 实际**保留并扩展**（L24-26 仍是 T-024 老段，L27-34 是 T-026 新段）。**功能无影响**，且历史溯源更完整；归类 MINOR drift（实际不阻塞）。 |
| C-6 | SECURITY | OK | 全文 | install.ps1 内 `Invoke-WebRequest` / `Invoke-RestMethod` 调用全部走 HTTPS（api.github.com / 用户 raw URL）；URL 字面量未变；ipify / ifconfig / icanhazip 候选公网 IP 探测也是 HTTPS。无 SQL / shell 注入面。 |
| C-7 | MAINT | OK | L23-L34 | 顶部注释段精简而完备：T-024 + T-026 E1 + T-026 E2 + G-6 + 配对闸门 + 配对 .editorconfig 一目了然。符合"WHY 非显然时写注释"项目规则。 |
| C-8 | LOGIC | NIT | L46 | 设计 02 §4.1 给出 idiom 是 `& {` 单独一行；Developer 实施确为 `& {`（L46）+ `param( ... )` 下移到 L47-L49——结构略不同于 02 §4.1 "After" 伪码（伪码 param 与 `& {` 在同一逻辑块内），但 PS 语法等价（scriptblock 起始 `{` 后第一句 `param()` 即可）。NIT 文体。 |
| C-9 | LOGIC | NIT | L37-L46 | 顶层 `param([switch]$Help)` 之后到 `& {` 之间夹了 8 行注释（L39-L45）。PS 语法允许 param 与后续语句间插入注释；不影响 splatting。NIT。 |
| C-10 | LOGIC | OK | L308 | `$LASTEXITCODE = 0` 显式 reset 防御陈旧值（C 组 §C.3 沿用 T-019/T-025 沉淀），保留原样。 |

**install.ps1 结论**：0 CRITICAL / 0 MAJOR / 1 MINOR (C-5) / 2 NIT (C-8, C-9)。**通过**。

### §6.2 `scripts/verify_all.ps1`

| # | 类型 | 严重度 | 位置 | 描述 |
|---|---|---|---|---|
| V1-1 | LOGIC | OK | L272-L286 | `$Ps1RequireBom`（10 个，字母序）+ `$Ps1ForbidBom`（install.ps1 1 个）= 11 个 == 实际 `scripts/*.ps1` 全集（grep 验证）。 |
| V1-2 | LOGIC | OK | L288-L303 | E.7a 字节级判定 `EF BB BF`；MISSING 文件归 `$missing` 给出"`(MISSING)`"后缀；throw 含完整列表换行展开。 |
| V1-3 | LOGIC | OK | L305-L322 | E.7b 反向判定；throw 文案含 "iex-entry .ps1 MUST NOT have UTF-8 BOM (BOM -> U+FEFF -> ParserError in iex form)" 友好；对应 AC-13 ADV-1 已实证。 |
| V1-4 | LOGIC | OK | L324-L336 | E.7c 增补 G-7：`Write-Host` + Yellow + `unclassified: ...` 列表 + `return $false`（→ WARN）。注释明示 "WARN 而非 FAIL：提醒维护者归类、不阻塞 CI"。 |
| V1-5 | LOGIC | OK | L329 | `@($actual | Where-Object ...)` 强制数组化防御 PS 单元素返回标量。**Developer 加固超出 02 伪码**，正向。 |
| V1-6 | MAINT | OK | L268-L271 | 注释段引用 "T-026" + 解释 white-list 拆分理由 + "PS5.1 + zh-CN 主机无 BOM 时按 host ANSI codepage (GBK) 误解码中文"机制；维护者可读性强。 |

**verify_all.ps1 结论**：0 CRITICAL / 0 MAJOR / 0 MINOR。**通过**。

### §6.3 `scripts/verify_all.sh`

| # | 类型 | 严重度 | 位置 | 描述 |
|---|---|---|---|---|
| V2-1 | LOGIC | OK | L282-L285 | `PS1_REQUIRE_BOM` / `PS1_FORBID_BOM` 数组内容与 PS 端一致（11 个文件全覆盖）。 |
| V2-2 | LOGIC | OK | L293-L304 | E.7a `head -c 3 + od -An -tx1 + tr -d ' \n'` POSIX 字节判定，与 PS 端逻辑等价；MISSING 文件用 `(MISSING)` 后缀。 |
| V2-3 | LOGIC | OK | L306-L318 | E.7b 反向判定，文案与 PS 端等价。 |
| V2-4 | LOGIC | OK | L320-L335 | E.7c G-7 增补：`echo "    unclassified:$(echo -e $e7c_unclassified)"` 在 step WARN 之**前**调用，绕过 sh `step` 函数 WARN 分支不打 detail 的设计——逻辑正确。 |
| V2-5 | NIT | NIT | L296/L310/L325 | `e7a_missing="$e7a_missing\n$name"` 字面 `\n`，靠 `echo -e $e7a_missing`（FAIL 路径）解释；可读性上 `printf '%s\n%s' ...` 更稳，但当前实现可用。NIT。 |
| V2-6 | MAINT | OK | L278-L281 | 注释段引用 "T-026" + 同款机制解释；与 PS 端同步。 |

**verify_all.sh 结论**：0 CRITICAL / 0 MAJOR / 0 MINOR / 1 NIT (V2-5)。**通过**。

### §6.4 `scripts/.editorconfig`

| # | 类型 | 严重度 | 位置 | 描述 |
|---|---|---|---|---|
| E-1 | LOGIC | OK | L7-L14 | `[install.ps1]` block 在 `[*.ps1]` 之后，EditorConfig spec "more specific section overrides" 保证后者覆盖前者；`charset = utf-8` 覆盖 `utf-8-bom`，`end_of_line = lf` 与 `insert_final_newline = true` 与前者一致（冗余但显式）。 |
| E-2 | MAINT | OK | L7-L10 | 注释段说明 T-026 / iex / ParserError 因果链 + "后置 section 覆盖前置" 规则 + 锚定范围说明。维护者可读性强。 |

**.editorconfig 结论**：0 CRITICAL / 0 MAJOR / 0 MINOR / 0 NIT。**通过**。

### §6.5 `scripts/baseline.json`

| # | 类型 | 严重度 | 位置 | 描述 |
|---|---|---|---|---|
| BL-1 | LOGIC | OK | L2 | `version: 12`（11 → 12 升 1）符合 02 §6 步骤 7 + 03 Dev-Q5。 |
| BL-2 | LOGIC | OK | L5-L8 | `test_count: 342` / `passing_count: 342` / `go_tests: 246` / `frontend_tests: 96` / `warnings_baseline: 0` **保持原值**——符合 02 D-7 + 03 Dev-Q5"仅改 version/updated/notes"。 |
| BL-3 | LOGIC | OK | L4 | `updated: "2026-05-23"` 与今日（2026-05-23）一致。 |
| BL-4 | LOGIC | OK | L10 | `notes` 字段：T-026 闭环描述完整；保留 T-025/T-022/T-021/T-019/T-018 历史链不丢。 |

**baseline.json 结论**：0 CRITICAL / 0 MAJOR / 0 MINOR / 0 NIT。**通过**。

### §6.6 `docs/dev-map.md`

| # | 类型 | 严重度 | 位置 | 描述 |
|---|---|---|---|---|
| D-1 | DOC | OK | L29 | T-026 注解一行追加在 T-021 注解后，覆盖：iex 入口禁 BOM 因果链、E.7 拆 a/b/c、.editorconfig 例外块。文风与 L28 T-021 同款。 |
| D-2 | DOC | OK | L29 | 文案"主体 `& { ... } @PSBoundParameters` 子作用域包裹让 `exit N` 在交互式宿主下退子作用域不杀宿主"——**已经反映 04 §3.1 揭示的"交互式宿主"nuance**，正向。 |

**dev-map.md 结论**：0 CRITICAL / 0 MAJOR / 0 MINOR / 0 NIT。**通过**。

---

## §7 设计落地完整性（02 §6 步骤 1-12 对账）

| 02 §6 步骤 | 04 实施记录 | 代码对照 | 状态 |
|---|---|---|---|
| 步骤 1 备份字节快照 | 04 §2.1（隐含；步骤 0 baseline 跑） | scripts/.t026-snapshot 已自行清理 | OK |
| 步骤 2 删 install.ps1 BOM | 04 §2.5 子步骤 c | install.ps1 首 3 字节 = `23 20 69` | PASS |
| 步骤 3 `& { ... }` 包裹 + 横幅 | 04 §2.5 子步骤 a/b | install.ps1 L46/L392/L398-L402 | PASS |
| 步骤 4 verify_all.ps1 E.7 拆 | 04 §2.2 | verify_all.ps1 L268-L336 | PASS |
| 步骤 5 verify_all.sh E.7 拆 | 04 §2.3 | verify_all.sh L278-L336 | PASS |
| 步骤 6 .editorconfig 例外 | 04 §2.6 | .editorconfig L7-L14 | PASS |
| 步骤 7 baseline.json | 04 §2.7 | baseline.json L2/L4/L10 | PASS |
| 步骤 8 dev-map.md | 04 §2.8 | dev-map.md L29 | PASS |
| 步骤 9 跑 verify_all | 04 §2.9 / §5.2 | Quick 19→21 / Full 预期 20→22 | PASS |
| 步骤 10 PS7 动态冒烟 | 04 §2.10（ADV-1/2/3 已自测；ADV-4/5 留 QA） | 04 §3.1 揭示 nuance + 02 D-2 论断需细化 | PASS（含 errata） |
| 步骤 11 -Help 验证 | 04 §2.11 | ExitCode=0 + Help 显示 | PASS |
| 步骤 12 字节零变 install-service / uninstall-service | 04 §2.12 + PM spot-check | `EF BB BF` 头 + size 9708/3993 + CR=0 | PASS |

**12/12 全 PASS**。

---

## §8 设计漂移检查（design fidelity）

| 02 设计 | 实际实现 | 漂移度 | 备注 |
|---|---|---|---|
| 02 §3 "L23-L25 注释" 要求**删** T-024 旧注释 | install.ps1 L24-L26 仍保留 T-024 注释并叠加 T-026 注释 | **MINOR DRIFT** (C-5) | 历史溯源更完整；功能无影响；建议保留实施 |
| 02 §1 "顶层 `param` + `$ErrorActionPreference="Stop"` 之外的所有逻辑都在 `& { ... }` 内" | 实际 Stop 在内层 scriptblock 第一句（L51），不在外层 | **零漂移**（02 §4.1 "After" 伪码也是把 Stop 放在内层；02 §1 是概览，§4.1 是细节伪码） | 内层 Stop 是 child-scope shadow，更干净；OK |
| 02 §4.1 idiom `& { param([switch]$Help) ... } @PSBoundParameters` | 实际 L46-L392 完全等价 | 零漂移 | PASS |
| 02 §4.2 / §4.3 E.7 拆 a/b/c + WARN 用 `return $false` | 实际 PS 端 L329 / sh 端 L329 等价 | 零漂移 | PASS |
| 02 §4.4 `.editorconfig` `[install.ps1]` block | 实际 L11-L14 等价 | 零漂移 | PASS |
| 02 §2 D-2 论断"`exit N` 在 scriptblock 内不终止 host runspace" | 04 §3.1 揭示仅在交互式 PowerShell console host 下成立，脚本宿主下仍杀进程 | **机制层 nuance**（不是漂移，是 02 论断的边界条件细化） | 用户真实场景 = 交互式宿主，FR-4/5 仍满足；errata 已记录 |

**设计漂移结论**：1 MINOR DRIFT (C-5) + 1 机制层 nuance (02 D-2)。**无 MAJOR 漂移**。

---

## §9 insight 合规复核

| Insight ID | 02/04 复现 | 实际代码合规 | 结论 |
|---|---|---|---|
| **L25** 管道形态禁 `$PSScriptRoot` | install.ps1 grep `\$PSScriptRoot` / `\$MyInvocation`：**0 命中** | PASS | 无回归 |
| **L34** BOM 字节级 idiom | 04 §2.5 子步骤 c：`[System.Text.UTF8Encoding]::new($true, $true)` 读 + `[System.Text.UTF8Encoding]::new($false)` 写 | PASS | 反向应用正确 |
| **L36** iex 形态禁 `[CmdletBinding()]` | install.ps1 grep `CmdletBinding`：仅出现于注释 4 处（L24/L25/L33/L41），**无 attribute** | PASS | 未重新引入 |
| **L41 / L48** reviewer 不落盘 | 本 05 由 Reviewer 在消息体提供 → PM 代落盘 | PASS | 已遵守 |
| **L43 / L49** QA / archive 标题禁数字前缀 | 04 §6 标题 `## §6 Adversarial 自测...` 带数字前缀，但 04 文档不被 verify_all E.6 检查（E.6 只查 06_TEST_REPORT.md 的 `## Adversarial tests`）；本 05 自身章节用 `## §N` 也 OK（reviewer 文档不受 E.6 约束） | PASS | 真正受约束的是 06 / 07，留 QA / PM |
| **L33** PS 解释器加载磁盘 .ps1 先剥 BOM | 02 §2 D-1 反面镜像引用 | PASS | 机制层证据已给 |
| **L32** git 不能 working-tree-encoding 锁 BOM | 02 §3 .editorconfig 例外 + verify_all 闸门是 "持久层 git blob + 编辑器层 + CI 层" 三层防御反向应用 | PASS | 模式复用 |

**insight 全合规**。

---

## §10 回归风险评估

### §10.1 已 spot-check 的字节级零变

| 文件 | 期望 | 实测（PM 二次确认 + 我交叉核对） | 状态 |
|---|---|---|---|
| install-service.ps1 | BOM=True, size 9708, CR=0 | 同 | PASS |
| uninstall-service.ps1 | BOM=True, size 3993, CR=0 | 同 | PASS |
| install.ps1 | BOM=False, size 18184, CR=0, line=402 | 同 | PASS |
| verify_all.ps1 | BOM=True, size 15378, CR=0 | 同 | PASS |
| verify_all.sh | BOM=False, size 13758, CR=0 | 同 | PASS |

### §10.2 verify_all 其他 step 未被无关改动影响

E.1～E.6 / E.8+ 等其他 step 在 verify_all.ps1 / verify_all.sh 中**字符未动**（diff 仅 E.7 块）；G.1 / G.2 / G.3 / A.1-3 / B.1-5 / C.1 / D.1 全部沿用原逻辑。04 §3.2 / §9 第 2-4 条说明 G.1 / G.2 / C.1 偶发 FAIL 是另一进行中任务（download-cancel-and-upload-decouple / 前端代码 wave-front）造成，与 T-026 0 因果。**Reviewer 接受**该归因。

### §10.3 baseline.json `test_count` 等数值字段确实未动

仅 `version`（11→12）/ `updated`（2026-05-23 占位）/ `notes` 三字段变更。**符合 Dev-Q5**。

### §10.4 scope drift（是否顺手改无关代码）

grep 验证：本任务 0 .go 改动、0 .ts / .vue 改动、0 install.sh / install-service.sh 改动、0 .harness/ / .claude/ / CLAUDE.md 改动。**严格遵守 OOS**（02 §10.4）。

### §10.5 跨形态行为矩阵补充验证

02 §5 矩阵 8 行 + 03 §3.2 复核——本 Review 阶段无法触发用户真机；接受 [U] 标注；交 QA 06 + 用户。

**回归风险结论**：低；无 scope drift；字节零变约束达成。

---

## §11 性能 / 安全 / 维护性 维度

### 性能

- E.7a/b/c 三个 step 都是 O(N=11) 文件读 `[System.IO.File]::ReadAllBytes` 仅读前 ≥3 字节。**总开销 < 50ms**（远低于 NFR-6 < 1s 预算）。
- install.ps1 主体逻辑零改（仅缩进 + 包裹），完整安装流程 8/8 性能无回归。

### 安全

- 0 secrets / SQL injection / 命令注入面增减。
- URL 字面量未变；ipify / ifconfig / icanhazip / GitHub API 全 HTTPS。
- 失败横幅 `Write-Host -ForegroundColor Red` 不引入 escape 漏洞。
- `& $svc` 调用 `install-service.ps1`（解压后路径，受 InstallDir env 影响）—— 与改前同款语义，无新风险。

### 维护性

- 注释精简（仅 WHY），所有 G-N 增补都有"依 03 §8 G-N 增补"溯源锚点。
- 命名一致：`$Ps1RequireBom` / `$Ps1ForbidBom` PowerShell `$PascalCase` 风格与本仓库习惯一致。
- white-list 字母序便于未来 diff。

---

## §12 Reviewer 给 QA 06 的关注点

1. **AC-1 / AC-2 / AC-4 / AC-5 / AC-6 / AC-7 / AC-8 / AC-11 [U]**：必须用户真机 PS5.1 + zh-CN 跑 `irm <raw_url> | iex` 端到端。Developer 04 §3.1 揭示自动化 mock 边界——QA 不要试图在 `pwsh -File` 下证明 "宿主存活"（脚本宿主下 `exit N` 仍杀进程）；改用 `Start-Process pwsh -NoExit -File <probe>` 或直接交用户真机。
2. **ADV-4 / ADV-5**：Developer 04 §2.10 已自跑 ADV-1/2/3；ADV-4（删顶层 param 验证）+ ADV-5（mock install-service.ps1 退出 2）留 QA 06。
3. **AC-7 失败横幅**：自动化不易精确验，建议 QA 用 `Start-Process pwsh -NoExit -Command "..."` 触发非管理员或非 amd64 → 真实窗口读横幅。
4. **`X` emoji 在 PS5.1 cp936 console 显示**：04 §9 第 5 条标注；可考虑作为 minor follow-up（不阻塞本任务）。
5. **AC-17**：QA 06 `06_TEST_REPORT.md` 必须含**裸标题** `## Adversarial tests`（无数字前缀，否则 verify_all E.6 FAIL，insight L43）。

---

## §13 裁决

**APPROVED**

理由：
- **0 CRITICAL / 0 MAJOR**；1 MINOR (C-5 注释保留 vs 删除) + 3 NIT (C-8/C-9/V2-5) 均不阻塞合并。
- 02 §6 12 步全 PASS；03 §8 4 条 MAJOR 必修条件（G-6/G-7/G-8/G-15）全增补且代码 + 注释 + 实证三层落地。
- 18 条 AC 中 8 条 [A] 全 PASS，10 条 [U]/[M] 合理延后 QA 06 + 用户真机。
- 12 条 BC 全有承接点。
- install-service.ps1 / uninstall-service.ps1 字节零变（PM + Reviewer 双 spot-check）。
- 0 scope drift（无 Go / 前端 / sh 入口 / .harness / CLAUDE.md 误改）。
- insight L25 / L32 / L33 / L34 / L36 / L41 / L48 / L43 / L49 全合规。
- verify_all PASS Quick 19 → 21（净 +2 来自 E.7 拆分；0 新 FAIL / 0 新 WARN）。
- 04 §3.1 揭示的 `& { exit N }` nuance 已用 errata 透明记录 + dev-map.md L29 文案反映"交互式宿主"，不阻塞用户真实使用场景。

**MINOR 建议（不阻塞，归档时整理）**：
- **C-5**：02 §3 表说"删 T-024 旧注释"但实际保留并扩展。建议 PM 07 在归档 errata 段一并注解"经 Reviewer 复核：保留 T-024 注释扩展历史溯源价值优于删除，与设计原意 'no functional change' 一致"。

**下一步**：PM 派 QA 06 收割 [U] AC 真机验证（特别 ADV-4 / ADV-5 + AC-7 失败横幅可见性）。
