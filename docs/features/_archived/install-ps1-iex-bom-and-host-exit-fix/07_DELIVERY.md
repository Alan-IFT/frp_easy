# 07 — Delivery · T-026 install-ps1-iex-bom-and-host-exit-fix

> Harness 流水线 Stage 7（PM Orchestrator 自写）。模式：**full**。
> 上游：01-06 全 READY；Gate Review **APPROVED FOR DEVELOPMENT**；Code Review **APPROVED**；QA **APPROVED FOR DELIVERY**。
> 交付日期：2026-05-24。

---

## §1 任务摘要

修复 `irm ... | iex` 一键安装路径下两条 P0 缺陷：

- **E1 BOM 引发 ParserError**：T-021 给 `scripts/install.ps1` 加的 UTF-8 BOM（用于 PS5.1+zh-CN 磁盘形态解析中文）在 `irm | iex` 形态下被 `irm` 解码进字符串成 U+FEFF 字符，`iex` 解析器把 `<U+FEFF>#` 当 cmdlet 名 → 报 `'﻿#' is not recognized`；后续 `param()` 不再处于"脚本第一句"位置 → 报 `'param' is not recognized`。两条 ParserError 非终止性，脚本继续执行至 [5/8]。
- **E2 `exit N` 杀宿主**：`iex` 在父 runspace 执行，install.ps1 主体 16 处 `exit N` 直接终止用户的 PowerShell 宿主窗口 → 用户报告"步骤 8/9 终端关闭，无法验证安装结果"。

**修复方案**（02 §2 决议 D-1..D-7）：
- **D-1**：删除 install.ps1 BOM；接受 PS5.1+zh-CN **磁盘形态**中文乱码（OOS-9，install.ps1 主要使用形态是 iex，非磁盘）。`install-service.ps1` / `uninstall-service.ps1` BOM **字节零变**（磁盘形态调用，需 BOM）。
- **D-2**：install.ps1 主体用 `& { param([switch]$Help) ... } @PSBoundParameters` 子作用域包裹；交互式 PowerShell 宿主下 `exit N` 仅退子作用域、不杀宿主。
- **D-3**：包裹外 `if ($LASTEXITCODE -ne 0)` 触发中文失败横幅，弥补 `exit N` 不再杀宿主后的"失败可观测"需求。
- **D-4 / D-5**：`scripts/verify_all.ps1` + `.sh` 把 E.7 拆为 E.7a（10 个 BOM-required .ps1 白名单）+ E.7b（install.ps1 黑名单禁 BOM）+ E.7c（名单外 .ps1 WARN，依 G-7 增补 unclassified 文件名打印）。

**改动统计**：6 个文件改、~80 行净增（含注释 + 失败横幅 + verify_all 拆分）；verify_all step 数 20 → 22。

---

## §2 交付指标

| 指标 | 改前 | 改后 | 备注 |
|---|---|---|---|
| install.ps1 size | 16024 B | 18184 B | +2160 B（包裹 + 注释 + 横幅） |
| install.ps1 BOM | True | **False** | E1 根因消除 |
| install.ps1 行数 | 372 | 402 | +30 行 |
| install-service.ps1 BOM | True | True ✓ | 字节零变（AC-10） |
| uninstall-service.ps1 BOM | True | True ✓ | 字节零变（AC-10） |
| verify_all step 数 | 20 (full) | 22 (full) | E.7 → E.7a/b/c 拆 |
| baseline.json version | 11 | 12 | 仅 version/updated/notes 变更 |
| AC PASS 比例 | — | 10 [A] PASS / 7 [U] 待真机 / 1 [Pending] | 0 FAIL |
| Adversarial 跑通 | — | 6/6 PASS | ADV-A..ADV-F |

---

## §3 verify_all 闸门结果

| 阶段 | 命令 | 结果 |
|---|---|---|
| Developer Quick（04 §5.2） | `pwsh -NoProfile -File scripts/verify_all.ps1 --quick` | PASS=21 / WARN=0 / FAIL=0（vs baseline=19 → +2） |
| QA Full（06 §2） | `pwsh -NoProfile -File scripts/verify_all.ps1`（stash T-027 wave-front 后） | **PASS=22 / WARN=0 / FAIL=0 / SKIP=0** |
| PM 最终闸门 | 见 §6 PM 独立闸门 | （隔离 T-027 wave-front 后） |

---

## §4 4 条 MAJOR 必修条件落实（03 §8）

| 条件 | 实施 | 验证 |
|---|---|---|
| **G-6**：双层 `param([switch]$Help)` 上方注释 `-Verbose`/`-Debug` 不支持 | install.ps1 L33-L34 + L41-L42 各一份注释 | Code Review 05 §5 / §6.1 C-7 PASS |
| **G-7**：verify_all.ps1 / .sh E.7c WARN 分支打印 unclassified 文件名 | PS L329 `Write-Host -ForegroundColor Yellow`；sh L325 `echo "    unclassified:..."` 在 step 之前 | QA 06 ADV-C 实证 stdout 含 `unclassified: fake.ps1` |
| **G-8**：04 注明步骤 10 mock 必须 PS7 主机跑 | 04 §2.10 段开头 + §4.3 重申 | Code Review 05 §5 G-8 PASS |
| **G-15**：install.ps1 注释"未来加新顶层参数必须同步内部 scriptblock param" | install.ps1 L43-L45 紧贴 `& {` 上方 | Code Review 05 §5 G-15 PASS |

**4/4 全落地 + 代码 + 注释 + 实证三层闭环**。

---

## §5 待用户真机验证（[U] AC 清单）

7 条 AC 需 PS5.1+zh-CN 真机覆盖（QA 主机 PS7+en-US 无法 substitute）：

- **AC-1 / AC-2**：`irm <raw_url> | iex` 在 PS5.1+zh-CN 下首部无 ParserError
- **AC-4 / AC-5**：iex 形态错误退出 / 成功退出，宿主 PowerShell 窗口仍存活
- **AC-6**：完整安装 8/8 → 宿主存活 → 用户能继续输 `sc query frp-easy`
- **AC-7**：触发失败时中文失败横幅在宿主窗口可见（红字）
- **AC-8 / AC-11**：磁盘形态 `.\install.ps1 -Help` 在 PS5.1+zh-CN 下显示中文帮助 + 退出码 0

QA 06 §6 已打包 6 组具体命令 + 期望输出，请用户按 §6 清单跑一次真机冒烟。**预期通过后**本任务全 PASS。

---

## §6 PM 独立闸门（verify_all 复跑）

为避免 T-027 wave-front 干扰，PM 在 archive-task 前跑独立 verify_all：先 stash T-027 wave-front（10 modified + 5 untracked，含 Go + 前端 + 新 spec），跑 verify_all，再 pop 恢复。结果见任务结尾 stash/run/pop 序列日志（07 文档完成后追加）。

---

## §7 归档与 follow-up

### 归档动作

1. 跑 `scripts/archive-task --task install-ps1-iex-bom-and-host-exit-fix`
2. 移动 `docs/features/install-ps1-iex-bom-and-host-exit-fix/` → `docs/features/_archived/install-ps1-iex-bom-and-host-exit-fix/`
3. 收割本文 §8 `## Insight` 段到 `.harness/insight-index.md`
4. 更新 `docs/tasks.md` T-026 stage → done、加 DELIVERED 行

### Follow-up（不阻塞本任务，作为 PM 备忘）

1. **T-XXX-A（trivial，低优）**：README 加"PS5.1+zh-CN 推荐 iex 形态"警告（03 Q-1 裁决保留 OOS-9，单开任务处理）。
2. **T-XXX-B（trivial，低优）**：05 §12 关注点 4：失败横幅中 `❌` emoji 在 PS5.1 cp936 console 显示乱码，可考虑改 `[ERROR]` 字面或保留（用户 PS7 可正常显示）。
3. **05 §13 C-5 errata**：02 §3 表说"删 T-024 旧注释"但 Developer 保留并扩展。Code Reviewer 复核认定保留更优；本 errata 已在 05 §13 记录，归档时随阶段文档保留。

---

## Insight

> 以下条目由 archive-task 收割到 `.harness/insight-index.md`。**回顾教训**：PM 初版误写 `## §8 Insight`（数字前缀）导致 archive-task regex 不命中、0 条收割（insight L43/L49 第 N 次复发），手工修复 + 手工追加 5 条到 insight-index.md。修法已沉淀到 §8 末新 insight"PM Stage 7 标题红线复审"。

- **2026-05-24** · `irm | iex` 一键安装脚本对 UTF-8 BOM 的容忍度与磁盘形态相反：磁盘形态需 BOM 让 PS5.1+zh-CN 走 UTF-8 解码（insight L32-L33），但 iex 形态下 `irm` 把 BOM 解码进字符串成 U+FEFF 字符，`iex` 解析器把 `<U+FEFF>#` 当 cmdlet 名 → ParserError；后续 `param()` 不再处于"脚本第一句"位置 → 第二条 ParserError。两条 ParserError 非终止性，脚本继续执行（用户日志验证）—— 这让"看起来还跑了几步"误导排查方向。修复：单脚本承担两种加载形态时必须做出反向选择：iex 入口 .ps1 **禁** BOM（接受磁盘 PS5.1+zh-CN 中文乱码），磁盘 .ps1 **必须** BOM。verify_all 必须拆白名单 / 黑名单两个 step，名单外 WARN 强制维护者归类。证据：T-026 install.ps1 用户报告 + verify_all.ps1 L268-L336 E.7a/b/c 三段实现
- **2026-05-24** · `iex` 在父 runspace 执行的副作用：脚本顶层 `exit N` 直接终止用户的 PowerShell 宿主窗口（"步骤 8/9 终端关闭、无法验证安装结果"的根因）。修复 idiom：整段主体用 `& { param([switch]$Help) ... } @PSBoundParameters` 子作用域包裹，**交互式 PowerShell console host** 下 `exit N` 仅退子作用域、不杀宿主；包裹外 `if ($LASTEXITCODE -ne 0)` 触发中文失败横幅弥补"失败可观测"。**重要 nuance**（04 §3.1 揭示 + dev-map.md L29 反映）：此 idiom **仅在交互式 console host 下保护宿主**，`pwsh -File <script>.ps1` 脚本宿主下 `exit N` 仍杀进程——这与用户真实使用场景（交互式宿主跑 iex）一致，但 QA 自动化 mock 不能用 `pwsh -File` 证伪宿主存活，必须真机交互式宿主或 `Start-Process pwsh -NoExit`。证据：T-026 install.ps1 L46-L392 包裹 + 04 §3.1 实测 nuance
- **2026-05-24** · `& { param ... } @PSBoundParameters` splatting 配对约束：磁盘形态 `.\install.ps1 -Help` 时顶层 `param([switch]$Help)` bind → `$PSBoundParameters = @{ Help = $true }` → splat 到内部 `& { param([switch]$Help) ... }`——**Help 必须在内部 param 块也声明**，否则 PowerShell 报"找不到接受实际参数的位置参数"。未来给 install.ps1 加新顶层参数时**必须同步**在内部 scriptblock param 块加同名同类型参数，否则 splat 错位。这是 splatting 应用于 scriptblock 调用时 PowerShell 对 hashtable key 与 param 声明严格匹配的语义副作用，发布前 ADV-D 测试覆盖。证据：T-026 install.ps1 L43-L45 G-15 注释 + 06 ADV-D 实测 `-Help` 被吞证伪
- **2026-05-24** · `scripts/verify_all.ps1` `Step` helper 的 WARN 分支只记 `status="WARN"` 不打 detail（L40-L43）；要让 WARN 行显示具体未分类文件名，必须在 `Step` 调用**之前** `Write-Host -ForegroundColor Yellow "..."` 单独 echo。`scripts/verify_all.sh` 的 sh `step` 函数同款限制（仅 FAIL 分支会 echo detail），同样需要在 `step` 调用前 `echo "    unclassified: ..."`。这是 verify_all 闸门"WARN 而非 FAIL，但仍要让维护者立即看到具体问题"模式的通用 idiom；未来加新 WARN-emit step 时直接复用。证据：T-026 verify_all.ps1 L329-L335 / verify_all.sh L325-L335 G-7 增补 + 06 ADV-C 实证
- **2026-05-24** · `scripts/.editorconfig` 用 "more specific section overrides" 规则覆盖编辑器层 BOM 锁：先 `[*.ps1]` 设 `charset = utf-8-bom`，后 `[install.ps1]` 设 `charset = utf-8` 即可锚定单文件例外。这是 insight L32 "BOM 锁定三层防御（git blob + .editorconfig + verify_all）"的反向单点例外模式——为某文件**解锁** BOM 而非锁定。维护者在 IDE / 编辑器里改 install.ps1 不会被自动加回 BOM。证据：T-026 scripts/.editorconfig L7-L14
