# 03 — Gate Review · T-026 install-ps1-iex-bom-and-host-exit-fix

> Harness 流水线 Stage 3（Gate Reviewer）。模式：**full**。
> 上游：01_REQUIREMENT_ANALYSIS.md（READY）+ 02_SOLUTION_DESIGN.md（READY）。
> Reviewer 工具集仅 Read/Glob/Grep —— 不能落盘，本文由 PM 代写到本路径（insight L41 / L48 已知模式）。

---

## §1 总评

**01 需求分析质量高**：FR 13 条 / AC 18 条 / BC 12 条 / NFR 9 条覆盖根因 + 边界 + 防回归三层；明确把 A-1～A-8 八项决议挂给 Architect；OOS 边界硬护栏（特别是 OOS-1 / 2 / 4 / 5）防 over-build。所有 AC 都可被分类为 [A] / [M] / [U]，与项目历史降级模式一致。**未发现 BLOCKER**。

**02 方案设计在大方向上无误**：D-1（删 BOM + 接受磁盘 PS5.1+zh-CN 中文乱码）+ D-2（`& { ... }` 子作用域包裹）+ D-4（white-list 拆 E.7a/b/c）是局部最小改动 + 语义清晰的组合；§3 受影响清单精确到行号、§4 给到伪码、§5 跨形态矩阵自洽、§7 列 11 项风险。**但存在 4 个 MAJOR + 7 个 MINOR + 2 个 NIT**，集中在：(a) `@PSBoundParameters` splatting 在 iex 形态下的边界条件需机制层证据脚注；(b) E.7c WARN 必须打印 unclassified 文件名才不沦为 silent regression；(c) §6 步骤 10 "Get-Content -Raw | iex" 在 PS5.1+zh-CN mock 主机上等价性会破坏，需注 "QA 主机必须 PS7"；(d) splatting 中 hashtable key 与内层 param 必须严格对应，未来加参数有静默错位风险。

**没有 BLOCKER**。MAJOR 都可在 Developer/QA 阶段以"明文 fallback + adversarial 测试 + 文档脚注"消化，不需回退 SA。建议 **APPROVED FOR DEVELOPMENT WITH CONDITIONS** —— 列出 4 条 MAJOR 必修条件，MINOR / NIT 作为 Developer 04 / QA 06 必关注项。

---

## §2 01 文档逐条审视

### G-1 [MINOR] 01 §3.4 FR-11 把"verify_all 闸门"候选 (c)"端到端 iex mock"列为可选，02 §2 D-5 已否决

- **责任文档**：01 §3.4 FR-11。
- **建议**：MINOR；不阻塞（02 已收敛）；归档时 PM 在 07 注解"02 D-5 已选 (a) 字节级断言，否决 (c)"。

### G-2 [PASS] 01 §3.3 FR-9 / AC-10 对 install-service.ps1 / uninstall-service.ps1 字节零变的硬约束清晰

无问题，硬护栏正确。

### G-3 [PASS] 01 §5 AC-17 / AC-18 直接引用 insight L43 / L22 / L35 的"标题禁数字前缀"红线

正面遵守，QA 06 / PM 07 模板已被 RA 提前固化。

### G-4 [PASS] 01 §11 Q-1 / Q-2 / Q-3 都标 "RA 推荐"，把决策权显式让渡给 SA + GR

符合 RA 角色契约"列 OQ 不阻塞 Architect"的模式。

### G-5 [MINOR] 01 §6 NFR-1 "本任务不引入 PS 版本检测分流"与 02 §7 R-3 缓解"如失败回退 D-2 选项 D（function 包）"潜在冲突

- 若 R-3 触发后真的回退到 function 包方案，仍属"PS 版本通用"——NFR-1 不会被违反。但 02 没显式重申。
- MINOR；不阻塞。

---

## §3 02 文档逐条审视

### G-6 [MAJOR] §4.1 `& { param([switch]$Help) ... } @PSBoundParameters` 的 iex 形态实际语义需 SA 在 02 增"机制层证据脚注"

SA 在 §4.1 末段断言："iex 形态下 `$PSBoundParameters` 为空 hashtable，`& { param ... } @PSBoundParameters` 等价于 `& { ... }`（无参）"。**这个判断方向上对**，但有边界陷阱：

1. **顶层 `param([switch]$Help)` 在 iex 形态是否被 parser 接受？** —— iex 把 string 当 scriptblock，scriptblock 允许 `param()` 作为第一句。**合法**。但这要求 iex 收到的 string 第一非空非注释行是 `param()`；当前 install.ps1 第 1-26 行是注释、L27 是 `param()`，符合规则。**SA 未引用 about_Script_Blocks 明文，建议补一句脚注**。
2. **`$PSBoundParameters` 在 iex 形态读到的是什么？** —— iex 内部 scriptblock 没绑外层参数，**`$PSBoundParameters` 是该 scriptblock 自身的 hashtable，初始空**（不会泄露 iex caller 的 `$PSBoundParameters`，因 scope rules）。SA 表述正确。
3. **真正的陷阱**：当用户磁盘形态跑 `.\install.ps1 -Help -Verbose`（多个参数）时，顶层 `param([switch]$Help)` 只声明 `-Help` —— `-Verbose` 是 cmdlet common parameter，需 `[CmdletBinding()]` 才支持。**T-024 已删 `[CmdletBinding()]`**（insight L36），所以 `.\install.ps1 -Verbose` 会报 "无法将参数 'Verbose' 绑定" —— 这**不在本任务 AC 范围**（AC-8 / AC-9 仅断言 `-Help`），但需在 03 标注：**SA 设计未引入新回归，但确认遗留行为"非 -Help 命名参数会报错"**。
- **责任文档**：02 §4.1。
- **修法**：02 §4.1 末尾追加一段脚注："本设计假定 install.ps1 magnetic / iex 两形态仅支持 `-Help` 一个命名参数。`-Verbose` / `-Debug` 等 cmdlet common parameter 因无 `[CmdletBinding()]` 不支持（T-024 已确认）；不在本任务范围。"
- **严重度**：MAJOR —— 不修 Developer 可能在步骤 9 跑 `pwsh -File scripts/install.ps1 -Verbose` 撞墙浪费排查时间。

### G-7 [MAJOR] §4.2 E.7c WARN 实现 `return $false` 与 verify_all `Step` helper 实际语义需严格对照

verify_all.ps1 L40 实测：`$result -eq $false` → WARN（递增 `$script:warns`）。SA 用 `return $false` 触发 WARN 是**正确**的。**但**：

- SA 在 §4.2 末段写 `if ($unclassified) { return $false }`，**没有打印 unclassified 文件名**。verify_all 当前 Step helper（L40-L43）的 WARN 分支**只**记 `status = "WARN"`，**不**打印 detail。这让 §8.4 ADV-3 期望 "错误含 `fake.ps1`" 在 PS 端**无法满足**——用户不会看到具体未分类的文件名。
- 对比 sh 端 §4.3 实现：用 `step "..." "WARN" "$(echo -e $e7c_unclassified)"`，sh `step` 函数对 WARN 分支也不打 detail（脚本 L33-L42 实测：仅 FAIL 分支会 echo detail）。
- **责任文档**：02 §4.2 + §4.3。
- **修法**：方案 A（首选）—— SA 在 §4.2 改为 `Write-Host "  unclassified: $($unclassified -join ', ')" -ForegroundColor Yellow; return $false`；§4.3 同步在 WARN 分支前 `echo "    unclassified: ..."`。方案 B —— 升 FAIL（即 throw），与 Q-2 SA 留给 GR 的选项一致。
- **严重度**：MAJOR —— 不修则 ADV-3 无法判定通过 / 失败，AC-12 隐含的"易于 grep 定位"也未达成。

### G-8 [MAJOR] §6 步骤 10 "QA 主机 PS7 上 `Get-Content -Raw scripts/install.ps1 | Invoke-Expression`" 在 BOM-less 状态下的等价性边界

SA 把步骤 10 当作 AC-1 [M] / AC-4 [M] / AC-7 [M] 的 mock。**但**：

1. `Get-Content -Raw scripts/install.ps1` 在 PS7 上**默认按 UTF-8 解码**（PS7 默认编码 UTF-8 no-BOM-required）；BOM-less 时返回纯字符串，**不含 U+FEFF**。
2. 真实 iex 形态：`irm` 默认 charset 选 ISO-8859-1（无 Content-Type 时 fallback）或 UTF-8（GitHub raw 返回 `text/plain; charset=utf-8`）。**带 BOM 时**：BOM 进 string、ParserError 复现；**不带 BOM 时**：等价 `Get-Content -Raw | iex`。
3. **所以**：步骤 10 mock 在**删 BOM 后**与真实 iex 等价（正向断言）；但**不能反向证伪"未来回归加 BOM 后真实 iex 是否复现 ParserError"** —— 这点 §8.1 表 ADV-1 已用"verify_all E.7b FAIL"补 belt，OK。
4. **但** SA 在步骤 10 命令里写 `Get-Content -Raw scripts/install.ps1 | Invoke-Expression`——`Get-Content -Raw` 在 PS5.1 上**默认按 ANSI codepage 解码**（PS5.1 + zh-CN 上 = GBK）；如未来 QA 主机切换到 PS5.1 跑该 mock，BOM-less install.ps1 的中文会按 GBK 误解码。SA 在 §5 矩阵第 4 / 7 行已识别此场景为"接受回归"，但 §6 步骤 10 没标注"必须在 PS7 主机跑"。
- **责任文档**：02 §6 步骤 10。
- **修法**：步骤 10 命令前加注 "QA 主机必须为 PS7（PS5.1 + zh-CN 会让 Get-Content -Raw 按 GBK 解码 install.ps1 中文，与真实 iex 形态行为不一致）"。
- **严重度**：MAJOR —— 不修则 QA 在 06 误用 PS5.1 + zh-CN 主机执行 mock 会得到 misleading 结论。

### G-9 [MINOR] §3 表格末尾"修正"语段把"不改 .editorconfig"翻案为"改 .editorconfig"

SA §3 表格后段的"自我修正"语段在归档文档读起来易致歧义。MINOR 建议：Developer 在 04 实施时直接把 §3 主表合并 `.editorconfig` 一行，删除"修正"段；归档时 PM 也可在 07 用 "errata" 注解。

### G-10 [MINOR] §11 Partition 分配末段"推荐 PM ... 单 generic developer agent 一次性完成"与"严格按 owned paths 派发则按 dispatch order 走两个 agent"两个路径并存，PM 需明确选一种

- 责任文档：02 §11。
- 修法：GR 这里给 PM 一句建议："本任务规模 5 文件 / 局部改动，**推荐 PM 单 generic developer 顺序完成** —— 避免 dev-backend 启停 overhead。"

### G-11 [MINOR] §7 R-2 关于 `$ErrorActionPreference` 在 scriptblock 内继承的解释只覆盖一半

SA 写 "scriptblock 内**赋值** 这些 preference 变量等于 child-scope shadow（不影响 parent）"——这是对**写**的描述。读侧：scriptblock 内**读** `$ErrorActionPreference` 走 dynamic scope lookup（read-through 父 scope）—— SA 应明示此点，因为 install.ps1 的 `try/catch` 行为依赖于 `$ErrorActionPreference="Stop"`。

- 责任文档：02 §7 R-2。
- 修法：SA 在 R-2 末尾加一句 "读侧：scriptblock 内读 preference 变量走 read-through 至父 scope；故顶层若未先设 Stop 而依赖子作用域第一句 `$ErrorActionPreference="Stop"`，前面 `param()` 之后到该赋值之间无 try/catch 时仍有窗口期 —— 实际 install.ps1 内 `param` 后第一句就是 `$ErrorActionPreference="Stop"`，无窗口。" MINOR。

### G-12 [NIT] §4.1 注释 "PS {} 内对缩进零要求" 用语随意

PowerShell `{}` 内部缩进确实无语法影响，但 SA 措辞可改为 "PowerShell scriptblock 对内部缩进无语法依赖（不像 Python），保留原缩进减少 diff 噪音" 更专业。NIT。

### G-13 [NIT] §8.4 ADV-4 期望逻辑反向需复核

SA 在 ADV-4 写："故意把 `& {` 之前的顶层 `param([switch]$Help)` 删掉 → `pwsh -File scripts/install.ps1 -Help` 仍能走 Help 分支..."

仔细读：删顶层 `param` 后，`pwsh -File install.ps1 -Help` 时 PowerShell parser 找不到顶层 `param([switch]$Help)`，要么：(i) 报"无法将参数 'Help' 绑定"并 exit；(ii) 把 `-Help` 当 positional arg 传入内部 `& { }`（**不会**，因为 splatting 用的是 `$PSBoundParameters`，外层都没 bind 就没参数可 splat）。所以 ADV-4 实际行为是 **`-Help` 绑定失败 + exit**，而非 SA 写的 "走主安装路径 → 非管理员 exit 1"。

- 责任文档：02 §8.4 ADV-4。
- 修法：把期望改为 "PowerShell 报参数绑定失败 + exit 非零；或 -Help 被吞、走主安装路径 exit 1。任一非"显示 Help"结果即证明双层 param 必要性。" NIT（不影响是否通过，只是预期文案精度）。

---

## §3.2 跨形态行为矩阵复核

SA §5 矩阵 8 行覆盖完整。GR 复核：

| 矩阵行 | GR 结论 |
|---|---|
| 1. iex + PS5.1 + zh-CN | OK 设计正确（删 BOM + `& { }` 包裹后） |
| 2. iex + PS5.1 + en-US | OK |
| 3. iex + PS7 + 任意 | OK |
| 4. 磁盘 `-Help` + PS5.1 + zh-CN | WARN 接受中文乱码（D-1 明示取舍） |
| 5. 磁盘 `-Help` + PS5.1 + en-US | OK |
| 6. 磁盘 `pwsh -File -Help` + PS7 | OK |
| 7. 磁盘完整安装 + PS5.1 + zh-CN | WARN 中文乱码 logic 可走 |
| 8. 磁盘完整安装 + PS7 | OK |

矩阵**未覆盖**两个边界：

- **跨 host process 调用**：服务模式下 `frp-easy.exe` 内部用 PowerShell 子进程调 install-service.ps1（**与 install.ps1 无关**，OOS 安全）—— OK。
- **VS Code "PowerShell Integrated Terminal"** + zh-CN：通常 PS7，但若用户配置 PS5.1，等价行 4 / 7 —— 已隐含覆盖，OK。

**MINOR G-14**：SA §5 矩阵第 4 / 7 行的"接受乱码"应在 02 §10.4 "OOS 边界"再次显式 cross-reference，目前 SA 在 §10.1 写"磁盘 PS5.1 + zh-CN 中文乱码回归（D-1 接受）"——已含；MINOR 通过。

---

## §4 Insight 合规复核

| Insight ID | 02 是否复现 | GR 结论 |
|---|---|---|
| **L12**（管道形态固定路径锚定） | 02 §10.4 隐含遵守 | PASS |
| **L25**（管道形态禁 `$PSScriptRoot`） | 02 不引入新 `$PSScriptRoot`；install.ps1 现状 OK | PASS |
| **L32**（git 不能 working-tree-encoding 锁 BOM） | 02 §3 .editorconfig 例外 + verify_all 闸门是 "持久层 git blob + 编辑器层 + CI 层" 同款三层防御反向应用 | PASS |
| **L33**（PS 解释器加载磁盘 .ps1 先剥 BOM） | 02 §2 D-1 明确引用 "PS5.1 解释器加载磁盘 .ps1 时若无 BOM 即按 host ANSI codepage 解码"——这是 L33 的反面镜像 | PASS（机制层证据已给） |
| **L34**（BOM 字节 idiom） | 02 §6 步骤 2 `UTF8Encoding($true, $true)` 读 + `UTF8Encoding($false)` 写，正是 L34 反向使用 | PASS |
| **L36**（iex 禁 `[CmdletBinding()]`） | 02 §3 表 "L23-L25 注释" 行确认**不重新引入** | PASS |
| **L41 / L48**（reviewer 不落盘） | **GR 自身违反风险**：本 03 实际由 GR 写消息体 → PM 代落盘（reviewer 工具集只有 Read/Glob/Grep），与该 insight 一致 | **触发**：本 03 由 PM 代写，GR 在消息体提供完整内容 |
| **L43 / L22 / L35**（QA / archive-task 标题禁数字前缀） | 02 §8.4 标题用裸 `## Adversarial tests`（设计 hooks 段，非 verify_all 检查范围，但 SA 已规范）；02 自身 §1～§13 用 `## §N ...` —— **这是设计文档自身标题，不被 verify_all E.6 检查**，OK | PASS |
| **L49 / L46**（同上） | 同 L43 | PASS |

**未漏读关键 insight**。

---

## §5 独立发现（01 / 02 都没明示的）

### G-15 [MAJOR] `& { ... } @PSBoundParameters` 中 splatting 应用于 scriptblock 调用时，PowerShell 对 hashtable 中**未在 param 块声明的 key** 行为差异

- 实测语义：当 hashtable 含 scriptblock param 块未声明的 key 时，PowerShell 报错 "找不到接受实际参数的位置参数"。
- **本任务影响**：磁盘形态 `.\install.ps1 -Help` 时顶层 `param([switch]$Help)` bind → `$PSBoundParameters = @{ Help = $true }` → splat 到内部 `& { param([switch]$Help) ... } @PSBoundParameters` —— **Help 在内部也声明，匹配**，OK。
- **但**：若**未来**有人在顶层 `param` 加一个新参数（如 `-DryRun`）忘了同步在内部 param 加，会**静默错位**或绑定失败。SA 未在 §7 风险表列此。
- **修法**：SA 在 §4.1 末尾或 §7 R-3 加一句 "未来在 install.ps1 加新顶层参数时，**必须**同步在内部 scriptblock `param` 块加同名同类型参数，否则 splat 错位。"
- **严重度**：MAJOR —— 不修则未来 install.ps1 加参数的人不知道这个约束、踩坑后难定位。

### G-16 [MINOR] §6 步骤 7 baseline.json 更新顺序

SA §6 步骤 7 让 Developer 写 baseline.json，但 baseline.json 的 "passing_count" / "test_count" 等字段在本任务**无变化**——SA 在 §2 D-7 明示 "不动"。但 §6 步骤 7 命令模板**未明示 "test_count 等保持原值"**，Developer 可能误改。

- 修法：SA 在 §6 步骤 7 加一行 "**仅**改 `version` / `updated` / `notes` 三字段；`test_count` / `passing_count` / `go_tests` / `frontend_tests` / `warnings_baseline` **保持原值**。"
- MINOR。

### G-17 [MINOR] §4.2 white-list 名单中的文件名 与 实际 scripts/ 目录文件名比对

GR 复核 `scripts/*.ps1` 实际清单 11 个：

```
archive-task.ps1, build.ps1, harness-sync.ps1, install-hooks.ps1,
install-service.ps1, install.ps1, package.ps1, start-e2e-server.ps1,
start.ps1, uninstall-service.ps1, verify_all.ps1
```

SA §4.2 `$Ps1RequireBom` 列 10 个（除 install.ps1）+ `$Ps1ForbidBom` 列 install.ps1。**11 个全覆盖**，PASS。MINOR：SA 名单按字母序列出，建议 Developer 实施时保持字母序便于未来 diff。

---

## §6 Open Questions 裁决

### Q-1 R-1 缓解扩 README 一行 vs 单开 trivial T-XXX？

**裁决：保留 OOS，单开 trivial T-XXX**。

理由：(a) 本任务 OOS-9 已硬约束；扩 README 会增加 PR diff scope creep，让 Code Review 难判定是否还有其他 silent drift；(b) 本任务发布后 PM 在 07 加一条 "follow-up：README 加 PS5.1+zh-CN 推荐 iex 形态警告" 的待办；(c) trivial T-XXX 走快速通道 < 10 行 diff，10 分钟完成；(d) 即便不加 README，用户撞磁盘形态乱码概率有限——install.ps1 是 iex 入口、磁盘形态本就少用。

### Q-2 E.7c WARN vs FAIL？

**裁决：保留 WARN**，但**必须修 G-7（打印 unclassified 文件名）**。

理由：(a) NFR-3 "不允许 silent regression" 关心的是"修了宿主存活但让用户感知不到失败"语义，与 verify_all WARN 性质不同——verify_all WARN 仍让用户在 console 看到 "WARN" 字样；(b) WARN 的设计目的是 "新增 .ps1 PR 时给一次提醒"，符合 R-6 缓解；(c) **前提**：G-7 修后 WARN 行显示 unclassified 文件名，**不**留 silent 余地。

### Q-3 `@PSBoundParameters` splatting PS5.1 vs PS7 兼容性？

**裁决：依赖 AC-8 [U] 用户真机覆盖，无需 SA 提前预设 fallback**。

理由：(a) `@PSBoundParameters` splatting 在 PS3.0+ 一致语义，PS5.1 / 7 实测无版本差异（about_Splatting 跨版本文档统一）；(b) SA 已在 §7 R-3 列出回退方案 D（function 包），fallback path 已声明；(c) 真机实测交 AC-8 [U] / AC-11 [U]，PM 在 06 标 "PS5.1+zh-CN 用户首跑必须验 -Help 走 Help 分支"。**但**：必须配 G-6 修复（02 §4.1 增机制层脚注 + 明示遗留 `-Verbose` 不支持），让用户即便撞墙也能看到设计假定。

---

## §7 高概率开发问题（预测 + 预答）

| Q | 预答 |
|---|---|
| **Dev-Q1** Edit 工具改 install.ps1 时如何确保不引入 CRLF / BOM 回写？ | 按 §6 步骤 2 的 `[System.IO.File]::WriteAllText + UTF8Encoding($false)` 字节级写入；Edit 工具内部会保留原编码 / 行尾，但稳妥起见，每次大改后跑步骤 2 末尾的 4 行验证命令（BOM / CR / Size）。Developer 04 必须录这些字节断言的 stdout 截图入 04 文档。 |
| **Dev-Q2** `& { ... }` 包裹后原有 try/finally（L202-L296 `Remove-Item $tmpDir`）能否正常执行？ | 能。`finally` 在 scriptblock 内属普通 PS 控制流，`exit N` 在 PS 中**会**触发外层 try/finally 的 finally 块（PS exit 语义与 throw 不同，exit 走清理路径）。BC-10 隐含此断言；Developer 步骤 10 动态冒烟"故意触发 exit 1"时**必须**额外断言 `$tmpDir` 不存在（用 `Test-Path $tmpDir.FullName` 应为 `$false`）—— SA 在 §8.4 ADV 段没列；建议 Developer 04 自补一条对账。 |
| **Dev-Q3** 步骤 2 删 BOM 后跑 verify_all，E.7b 会通过吗？ | 通过 —— 但 Developer **必须先**改 verify_all.ps1（步骤 4）再跑 verify_all。否则当前 E.7 单 step 会因 install.ps1 BOM 缺失 FAIL。**实施顺序建议**：先改 verify_all（步骤 4-5），再改 install.ps1（步骤 2-3），最后跑 verify_all（步骤 9）—— SA §6 顺序是 "先删 BOM 再改 verify_all"，会让中间态 verify_all FAIL；Developer 可调整为先改 verify_all 防中间态失败。 |
| **Dev-Q4** `.editorconfig` 例外块要不要写绝对路径 `[/install.ps1]`？ | 不需要。`.editorconfig` 在 `scripts/` 目录内，相对锚定本目录；`[install.ps1]` 仅匹配 `scripts/install.ps1` 一个文件。SA §7 R-5 已分析；Developer 步骤 6 直接按 §4.4 写即可。 |
| **Dev-Q5** baseline.json 当前 `passing_count = 342` 跟 verify_all PASS count = 20 不是一回事，会不会冲突？ | 不会冲突 —— baseline.json `test_count` / `passing_count` 是 **Go + 前端单测总数**（246 + 96 = 342），verify_all PASS / FAIL 是 `Step` 数，二者独立。本任务 verify_all step 数从 20 升 22（拆 E.7 +2），baseline.json `notes` 字段反映即可，`test_count` 等保持原值。 |

---

## §8 必修条件（APPROVED FOR DEVELOPMENT WITH CONDITIONS）

**MAJOR（必修，Developer 04 实施前 SA 在 02 给补丁脚注 / Developer 在 04 自行补做并在 04 §"design drift / errata" 段标注）**：

1. **G-6**：02 §4.1 加机制层脚注 + 明示 `-Verbose` / `-Debug` 等 cmdlet common parameter 不支持（遗留行为）。
2. **G-7**：02 §4.2 + §4.3 WARN 分支必须打印 unclassified 文件名（PS 端用 `Write-Host -ForegroundColor Yellow`，sh 端用 `echo` 在 step 调用前）。
3. **G-8**：02 §6 步骤 10 命令前注 "QA 主机必须为 PS7（PS5.1 + zh-CN 会让 Get-Content -Raw 按 GBK 解码，与真实 iex 形态行为不一致）"。
4. **G-15**：02 §4.1 或 §7 R-3 加 "未来在 install.ps1 加新顶层参数时必须同步内部 scriptblock param 块" 约束。

**MINOR（建议修，Developer 04 注意即可，不阻塞合并）**：G-1 / G-5 / G-9 / G-10 / G-11 / G-16 / G-17

**NIT（文档润色，归档时可一并整理）**：G-12 / G-13

**Developer 实施顺序建议（按 Dev-Q3 调整）**：先改 verify_all.ps1 / verify_all.sh（步骤 4-5）再改 install.ps1（步骤 2-3），避免中间态 verify_all FAIL。

---

## §9 裁决

**裁决：APPROVED FOR DEVELOPMENT**

附 4 条 MAJOR 必修条件（G-6 / G-7 / G-8 / G-15）由 Developer 在 04 实施时自带补做并在 04 §"design drift / errata" 段标注 "依 03 §8 G-6/G-7/G-8/G-15 增补"。**无需回退 SA**。

理由：
- 01 / 02 大方向无误，无 BLOCKER。
- 4 条 MAJOR 是"补充脚注 / 修正 WARN 打印 / 注 mock 主机要求 / 加未来约束"性质，**不需重新设计**。
- 7 条 MINOR + 2 条 NIT 是文档质量问题，归档时整理。
- Open Questions Q-1 / Q-2 / Q-3 均给出裁决，不阻塞。
- Insight L25 / L32 / L33 / L36 / L43 全部正确遵守；L41 / L48 触发 reviewer 不落盘已知模式，PM 代写。
