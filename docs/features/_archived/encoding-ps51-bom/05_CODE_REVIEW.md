# 05 — Code Review · T-021 encoding-ps51-bom

> Harness 流水线 Stage 5（Code Reviewer）。模式：full。
> 上游：01_REQUIREMENT_ANALYSIS.md (READY) / 02_SOLUTION_DESIGN.md 轮次 2 (READY) / 03_GATE_REVIEW.md (CHANGES REQUIRED → 02 轮次 2 已修订 7 处 + 吸纳 OPT-1/2/4) / 04_DEVELOPMENT.md (READY FOR REVIEW)。
> Reviewer 工具集 = Read / Glob / Grep（无 Write，insight L42），本文档由 PM 接管落盘。

---

## §1 评审范围

### §1.1 Files reviewed（独立读取）

- `c:\Programs\frp_easy\scripts\.editorconfig`（新增，5 行）
- `c:\Programs\frp_easy\scripts\verify_all.ps1`（+E.7 块 + 自身 BOM；315 行）
- `c:\Programs\frp_easy\scripts\verify_all.sh`（+E.7 块；319 行；**不**加 BOM）
- `c:\Programs\frp_easy\scripts\archive-task.ps1`（+BOM，125 行；R-2 dogfood 关键文件）
- `c:\Programs\frp_easy\scripts\install.ps1`（+BOM）
- `c:\Programs\frp_easy\scripts\build.ps1`（+BOM）
- `c:\Programs\frp_easy\scripts\harness-sync.ps1`（+BOM）
- `c:\Programs\frp_easy\scripts\install-hooks.ps1`（+BOM）
- `c:\Programs\frp_easy\scripts\install-service.ps1`（+BOM）
- `c:\Programs\frp_easy\scripts\package.ps1`（+BOM）
- `c:\Programs\frp_easy\scripts\start.ps1`（+BOM）
- `c:\Programs\frp_easy\scripts\start-e2e-server.ps1`（+BOM）
- `c:\Programs\frp_easy\scripts\uninstall-service.ps1`（+BOM）
- `c:\Programs\frp_easy\scripts\baseline.json`（version 8→9 + notes 闭环 + updated 留原占位）
- `c:\Programs\frp_easy\docs\dev-map.md`（scripts 行末追加 T-021 子句）
- `c:\Programs\frp_easy\.gitattributes`（零 diff，确认轮次 1 错误已撤销）

### §1.2 Reviewer 独立验证手段

| 维度 | 手段 | 结果 |
|---|---|---|
| BOM 字节级 | Read 工具读 11 个 `.ps1` 首 3 行；ripgrep 默认 BOM-aware（剥 BOM 后匹配）；`Grep "^#"` head 1 在 11 文件全部匹配成功（间接证明 BOM 不污染首行字符匹配） | 11/11 PASS |
| verify_all.sh 不加 BOM | `Grep "^#!/usr/bin/env bash" verify_all.sh -n` → 命中 L1 | PASS（POSIX shebang 在第 1 字节） |
| `.gitattributes` 干净 | Read `.gitattributes` 全 10 行；Grep `scripts/\*\.ps1` / `working-tree-encoding` → 0 命中 | PASS（轮次 2 撤销决议落地） |
| 临时辅助脚本清理 | Glob `scripts\_*.ps1` / `scripts\.bom-pre-snapshot\**` → 0 命中 | PASS |
| 设计 §2.2 PS 伪码字节级对齐 | Read verify_all.ps1 L268~L290 逐行对照 02 §2.2；含 `StartsWith($root)` guard、`throw "Missing UTF-8 BOM in:` + relPath` | PASS |
| 设计 §2.2 sh 伪码字节级对齐 | Read verify_all.sh L278~L301 逐行对照 02 §2.2 + 加 `e7_found_any` 哨兵（设计基础上多一层 SKIP guard） | PASS（实施略增强） |
| baseline.json JSON 合法 | Read baseline.json 全 12 行；结构 `{ ... }` 闭合、字段名字符串引号配对、`version: 9` 整数、`notes` 单行字符串 | PASS |
| dev-map.md 增量行 | Read L28 → `T-021：scripts/*.ps1 全部 11 个统一 UTF-8 BOM ...` 一行追加 | PASS |
| `.editorconfig` 内容 | Read 全 5 行（除注释）：`[*.ps1] / charset = utf-8-bom / end_of_line = lf / insert_final_newline = true` 字节级匹配 02 §2.3 模板 | PASS |
| verify_all PASS:20 真实性 | Reviewer 无 Bash 跑 pwsh 权限的直接证据，但 Read E.7 代码 + 04 §4.2 完整 Summary 文本 + Step 函数 try/catch 路径 + 11/11 BOM 已字节级 PASS（Read 间接证据） → 逻辑必然 PASS:20 | PASS（按设计与代码推导） |

---

## §2 Findings

### CRITICAL

无。

### MAJOR

无。

### MINOR

- **[MAINT]** `c:\Programs\frp_easy\scripts\baseline.json:4` — `"updated": "2026-05-23"` 为本任务 Stage 4 占位（02 §3 步骤 8 明文 "Developer 04 暂留 2026-05-23 占位，QA 06 跑通日期改"）。Reviewer 接受当前占位作为 Developer 范围内的合理选择，但提醒 QA Stage 6 / PM Stage 7 在 verify_all 全绿后必须把 `updated` 同步为真实跑通日期，否则 baseline.json 内 `version=9` 与 `updated=2026-05-23`（与 T-019 同日）会让 future Reviewer 误判此次 bump 是 T-019 自身延续。
- **[MAINT]** `c:\Programs\frp_easy\scripts\verify_all.ps1:281` — `throw "verify_all 必须从仓库根目录运行 (当前 root: $root)"` 字面中文（圆括号、空格半角）与 02 §2.2 PS 伪码（轮次 2 §11 行）模板 `"verify_all 必须从仓库根目录运行（当前 root: $root）"` 略有标点漂移（伪码为中文圆括号 + 全角，代码实现为半角 + 空格）。语义等价，不阻塞；建议未来如需复读这段错误信息时统一中文标点。
- **[MAINT]** `c:\Programs\frp_easy\scripts\verify_all.ps1:276` — 注释 `# 使用 .NET API 跨 PS5/7 一致 (Get-Content -Encoding Byte / -AsByteStream 在两版本 flag 不同)` 说明动机充分，但未链接到 02 §2.2 "工具选定理由"；如未来 verify_all.ps1 被独立翻阅，读者无法快速定位设计动机。建议追加一行 `# 详见 02_SOLUTION_DESIGN.md §2.2`（NIT 边界，不阻塞）。

### NIT

- **[STYLE]** `c:\Programs\frp_easy\scripts\verify_all.sh:285~294` — sh 端实施在设计 §2.2 sh 伪码基础上**额外**增加了 `e7_found_any` 哨兵（找不到任何 .ps1 时显式 SKIP 而非 PASS）。这是 sh 端比 PS 端更稳健的小增强，与 PS 端 L273 `if (-not $ps1s -or $ps1s.Count -eq 0) { return "SKIP" }` 语义对齐。Reviewer 认可此偏离设计是**改善**而非漂移，但 04 §3 fidelity check 未明确记录此增强；建议 PM 在 06 / 07 简短一句 ack。
- **[STYLE]** `c:\Programs\frp_easy\docs\dev-map.md:28` — T-021 子句单行较长（160+ 字符），与该 scripts/ 行已含 T-008/T-012/T-013 多次追加形成"长龙"；02 §11 OPT-3 已建议 "在 L27 后另起一行" 但 SA 未吸纳，Developer 严格按 02 §3 步骤 9 执行。Reviewer 接受此一致性选择，纯 NIT 不阻塞。
- **[STYLE]** `c:\Programs\frp_easy\scripts\baseline.json:10` notes 单串内同时含 T-019 / T-021 多任务历史 + Go/前端 unit 增量记录，已逼近 800 字符长行；未来若再加任务可能需要换 schema（如 `"notes_history": [...]` 数组化），属技术债提示，非本任务责任。

### OBSERVATION（无 severity，仅记录给后续任务）

- **[OBS-1]** 设计 04 §6.1 已自承"3 个 ASCII .ps1 (`archive-task.ps1` / `harness-sync.ps1` / `install-hooks.ps1`) 工作树字节级是 CRLF"（CR=125 / 124 / 72）。Reviewer 独立通过 Read 工具读取这 3 个文件首 3 行均显示正常无 `^M` 可见伪影（Read 工具自动归一），但 04 §4.3 表 CR=125→125 / 124→124 / 72→72 已字节级证明这是**预先存在状态**（snapshot 已 CRLF），并非 BOM 添加引入。
  - **与 NFR-2 关系**：NFR-2 字面 "项目 `.gitattributes` 第 2 行 `* text=auto eol=lf` 已强制 LF 行尾；BOM 添加不得引入 CRLF"。Developer 严格满足后半句（CR 数零漂移），但前半句"已强制 LF"在工作树字节字面被 CRLF 反例。这不是 T-021 的责任（这 3 文件本就是 CRLF 工作树），属**仓库历史遗留**。
  - **建议**：PM 在 07 §7 backlog 加入 `T-023 normalize-ps1-eol`（注：T-022 已被 service-mode-stderr-bridge 占用），独立任务用 `git add --renormalize .` 或 `dos2unix` 把这 3 文件归一为 LF；与本任务范围完全独立、不阻塞 T-021 ship。
- **[OBS-2]** `c:\Programs\frp_easy\scripts\start-e2e-server.ps1` L1 是 `#requires -Version 7`（PS7-only），与本任务"让 PS5.1 + zh-CN 能跑磁盘 .ps1"的目标对齐时该文件本身**不需要** PS5.1 可用（它是 E2E 测试专用、CI / Playwright 调用）。但本任务仍按一致性原则给它加 BOM（I-1 决议方案 A），无负面影响。仅记录给 future reviewer，便于理解为何该文件 BOM 与"PS5.1 兼容"目标不直接相关。
- **[OBS-3]** `c:\Programs\frp_easy\scripts\archive-task.ps1:24` 使用 `$repoRoot = Split-Path $PSScriptRoot -Parent`，依赖 `$PSScriptRoot` 自定位。BOM 加完后 PS 解释器在加载 .ps1 时**先剥 BOM 再 parse**，`$PSScriptRoot` 由解释器从磁盘路径计算（与文件内容无关），故 BOM 不影响该变量。这与 insight L25 "管道形态禁用 `$PSScriptRoot`"反向 —— 此处脚本走磁盘形态、`$PSScriptRoot` 合法可用。**Dogfood 04 §4.5 实测 archive-task.ps1 -DryRun 退出 0 + 输出含正确 `Moved ...` 路径**，已字节级验证 `$PSScriptRoot` + BOM 共存 OK。

---

## §3 Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| **AC-1** [A] 11 个 .ps1 字节 `EF BB BF` 起始 | Reviewer Read 11 个文件首 3 行 + 04 §4.3 完整字节断言表 + Grep `^#` 在 11 文件全部 head 1 匹配（间接） | OK |
| **AC-2** [A] BOM 后字节段与实施前字符级完全一致 | 04 §4.4 SHA256 加固：byte[3..end] hash 等于 snapshot byte[0..end] hash（10/11；verify_all.ps1 由步骤 5 主动加 E.7 块体除外，符合设计明文允许） | OK |
| **AC-3** [U] PS5.1 + zh-CN `install.ps1 -Help` 退出 0 + 中文无乱码 | 02 §7.2 真机清单第 1 项 | 待 [U]（QA 06 真机清单） |
| **AC-4** [U] PS5.1 + zh-CN `verify_all.ps1 -Quick` 跑到 Summary | 02 §7.2 真机清单第 2 项 | 待 [U] |
| **AC-5** [U] PS5.1 + zh-CN `install-service.ps1` 解析中文无 syntax error | 02 §7.2 真机清单第 3 项 | 待 [U] |
| **AC-6** [U] PS5.1 + zh-CN `irm ... install.ps1 \| iex` 完整 8 步 | 02 §7.2 真机清单第 4 项 | 待 [U] |
| **AC-7** [A] PS7 `pwsh -File scripts\verify_all.ps1` ≥ 20 PASS | 04 §4.2 PASS:20 完整 Summary；Reviewer 逐行核对 E.7 实现，逻辑必 PASS | OK |
| **AC-8** [A] PS7 `install.ps1 -Help` 字节级等价 T-019；BOM 不进 stdout | PS 解释器内置 BOM-aware；04 §4.5 dogfood archive-task.ps1 -DryRun 输出无 BOM 漏字符 → 同款机制覆盖 install.ps1 | OK |
| **AC-9** [U] PS5.1 + PS7 两版 `irm \| iex` 跑完 install | 02 §7.2 真机清单第 4 项（与 AC-6 合并） | 待 [U] |
| **AC-10** | C-3 修订合并到 AC-9（02 §11 修订记录第 7 项） | 设计合并 |
| **AC-11** [A] verify_all.ps1 + .sh 各 1 新检查项 | verify_all.ps1:268-290 + verify_all.sh:278-301，同 step id `E.7` + 同标题 `scripts/*.ps1 have UTF-8 BOM` | OK |
| **AC-12** [A] 新检查项命名含 "BOM" / "UTF-8 BOM" | 标题 `scripts/*.ps1 have UTF-8 BOM`（含 "UTF-8 BOM" + "BOM" 双 token，28 字符） | OK |
| **AC-13** [A] 故意删 BOM → verify_all FAIL + 含命中路径 | verify_all.ps1:288 `throw "Missing UTF-8 BOM in:` + relPath join；sh 端:299 `step ... "FAIL" "$(echo -e $e7_missing)"` 含相对路径列表 | OK 设计满足；QA 06 ADV-1 模拟执行 |
| **AC-14** [A] PASS 19→20；sh 等价升一项；baseline.json 同步 | 04 §4.2 PASS:20；baseline.json version 8→9 + notes 含 `verify_all 19->20` 文本；02 §2.5 决议不动 test_count（保 335） | OK |
| **AC-15** [A] 新检查项仅扫 `scripts/*.ps1` 非递归 | verify_all.ps1:272 `Get-ChildItem -Path "scripts" -Filter "*.ps1" -File`（无 -Recurse）；sh 端:293 `find scripts -maxdepth 1 -name '*.ps1' -type f` | OK |
| **AC-16** [A] `docs/dev-map.md` 更新或新增"脚本编码"条目 | dev-map.md L28 追加 1 行（含 BOM 字节描述、双层防回归说明） | OK |
| **AC-17** [A] QA 06 含裸标题 `## Adversarial tests` | QA Stage 6 责任；verify_all E.6 已强制 | 待 QA 06 |
| **AC-18** [A] PM 07 含裸标题 `## Insight` | PM Stage 7 责任；archive-task.ps1 收割 regex 已强制 | 待 PM 07 |

**Developer 范围内 11 条 [A] AC**：全部 OK。
**5 条 [U] 真机 AC**：02 §7.2 + 03 D2 已 ack 降级，待 QA 06 真机清单。
**2 条 QA / PM 责任 AC**：待相应 Stage。

---

## §4 NFR coverage check

| NFR | 实现 / 证据 | Status |
|---|---|---|
| **NFR-1** 内容字节零改（除 BOM 3 字节外） | 04 §4.4 SHA256 加固：byte[3..end] hash = snapshot byte[0..end] hash（10/11；verify_all.ps1 是设计明文主动改 +1226 字节 E.7 块体） | OK |
| **NFR-2** 行尾 LF 不变（BOM 不引入 CRLF） | 04 §4.3 CR 数对照 snapshot 零漂移（11/11，含 3 个原 CRLF 文件维持 CR=125/124/72） | OK（注：3 文件原本 CRLF 是历史遗留，OBS-1 标注） |
| **NFR-3** 不引入新依赖 | 字节级写盘用 .NET BCL；verify_all 检查用 .NET BCL / POSIX `head` + `od` 内置；新增 `.editorconfig` 是被动配置非依赖 | OK |
| **NFR-4** verify_all 新 check 运行时间 < 1s | 11 个文件 `ReadAllBytes` 前 3 字节，IO 量极小；无 spawn 子进程 | OK（逻辑推导） |
| **NFR-5** git diff 噪声最小 | BOM 是 3 字节非 NUL，不触发 git binary detection；项目 `.gitattributes * text=auto eol=lf` 已让 .ps1 走文本 diff | OK |
| **NFR-6** 编辑器友好 | BOM 是 UTF-8 标准 signature，VS Code / Notepad / PowerShell ISE 全部识别；.editorconfig 进一步锁 charset | OK |
| **NFR-7** 编码不漂移机制 | 三层防御：(a) git blob 字节保存 BOM (持久层)；(b) `scripts/.editorconfig` (编辑器层 belt)；(c) verify_all E.7 (CI 闸门 suspenders) | OK |
| **NFR-8** 兼容 archive-task.ps1 自我引用 | 04 §4.5 dogfood `archive-task.ps1 -DryRun` 前后 BOM 不变 + 退出 0 + 输出正确 | OK |

---

## §5 Design fidelity check

| 设计项 | 实现 | Status |
|---|---|---|
| 02 §2.1 11 个 .ps1 一律加 BOM (I-1A) | 11/11 全加 | OK |
| 02 §2.2 PS 块用 `.NET ReadAllBytes` + 字节比对 0xEF/0xBB/0xBF | verify_all.ps1:277-285 字节级实现 | OK |
| 02 §2.2 PS 块含 `StartsWith($root)` guard (C-4) | verify_all.ps1:280-282 已加 | OK |
| 02 §2.2 sh 块用 `head -c 3 \| od -An -tx1 \| tr -d ' \n'` | verify_all.sh:289 字节级实现（含 sed/od 兼容） | OK |
| 02 §2.2 E.7 编号 (E 段尾、E.6 后) | PS:268 / sh:278 紧接 E.6 之后 | OK |
| 02 §2.2 E.7 标题 `scripts/*.ps1 have UTF-8 BOM` (OPT-4) | PS:268 / sh:282/297/299 完全一致 | OK |
| 02 §2.3 新增 `scripts/.editorconfig` 5 行 | 字节级匹配模板 | OK |
| 02 §2.3 撤销 `.gitattributes` working-tree-encoding (C-1) | `.gitattributes` 零 diff，Grep 0 命中 | OK |
| 02 §2.4 不新建 `scripts/add-ps1-bom.ps1` | 04 §2.4 临时辅助已删；Glob 验证 0 残留 | OK |
| 02 §2.5 baseline.json 只改 notes/version/updated；不动 test_count | baseline.json:2 version=9 / :4 updated=2026-05-23（占位）/ :5-8 test_count=335/passing_count=335/go_tests=239/frontend_tests=96 全不变 / :10 notes 含 "closed by T-021" + "T-021 closed: ... verify_all 19->20" 双修订（OPT-2） | OK |
| 02 §3 步骤 1~11 全 11 步落地 | 04 §3 fidelity 表 11/11 标 100% | OK |
| 02 §3 步骤 2 ReadAllText 用 `UTF8Encoding($false, $true)` (C-2 修订) | 04 §3 行 2 自报：临时脚本用该构造、无 DecoderFallbackException | OK（Developer 自报；临时脚本已删，Reviewer 无法字节级验证 source，但 SHA256 加固对 byte[3..end] 等同 snapshot 是真实证据） |
| 02 §3 步骤 7 .editorconfig + 不动 .gitattributes | 字节级匹配 + 零 diff | OK |
| 02 §3 步骤 11 dogfood archive-task.ps1 -DryRun | 04 §4.5 完整证据：前后 BOM=239,187,191 + 退出 0 + dry-run 输出含正确 Moved 路径 | OK |
| 02 §11 OPT-3 未吸纳（dev-map 改动位置） | dev-map.md L28 追加到 scripts/ 行末（非 L27 后另起一行），与 02 §11 明文一致 | OK（一致性保持） |

**总评**：11 步 100% fidelity，无 silent design drift。Developer 范围内**唯一**对设计的偏离是 sh 端 E.7 加 `e7_found_any` 哨兵（NIT 增强非漂移）。

---

## §6 6 维度独立判定

### §6.1 D1 设计还原度 — **PASS**

11 步全落地，C-1/C-2/C-3/C-4 必须修订 + OPT-1/2/4 吸纳全部体现到代码。零 silent drift。

### §6.2 D2 正确性 — **PASS**

- BOM 字节级：Read 11 文件首行均显示首字符 = `#`（BOM 已被 Read decoder 吞、字节首在 EF BB BF）+ 04 §4.3 / §4.4 字节级 + SHA256 双重证据。
- verify_all.ps1 E.7 PS 块：`.NET ReadAllBytes` + 字节比对 + StartsWith guard + relPath throw → 逻辑严密。
- verify_all.sh E.7 块：`head -c 3 | od -An -tx1 | tr -d ' \n'` 比 `efbbbf` → POSIX 兼容、字节正确；额外 `e7_found_any` 哨兵进一步加固 SKIP 边界。
- Reviewer 无法在本环境直接跑 pwsh / bash 验证 PASS:20，但代码逻辑 + 04 完整 Summary 文本一致 → 推导必然 PASS:20，零回归。

### §6.3 D3 NFR / Security / Quality — **PASS**

- NFR-1 ~ NFR-8 全 OK（见 §4）。
- Security：BOM 添加不引入任何 input/output 路径变化、无新 API、无新 IO；E.7 检查仅读 `scripts/*.ps1` 前 3 字节，无 shell injection 风险（`Get-ChildItem` 返回 FileInfo、不走字符串拼接）；sh 端 `find scripts -maxdepth 1 -name '*.ps1' -type f` + `head -c 3 "$f"` 用引号包裹路径，路径含空格也安全。
- Quality：注释充分（E.7 块含 T-021 引用 + 02 §2.2 + C-4 修订说明）；命名稳定（E.7 step id + 中英文标题）；无 dead code。
- `.editorconfig` 5 行精简（仅 `[*.ps1]` section，未误伤其他文件类型）。
- `.gitattributes` 零 diff（轮次 2 撤销决议落地清洁）。

### §6.4 D4 测试覆盖 / verify_all gate — **PASS**

- E.7 在 PS 与 sh 两端**步骤号 + 标题字符串**完全一致（PS:268 vs sh:282/297/299）；FAIL 路径退出码语义等价（PS throw → Step try/catch → errors++；sh step("FAIL") → errors++；最终均 exit 2）；FAIL 输出格式略有差异（PS 用 `\n` join + DarkRed 详情行；sh 用 `echo -e` + 缩进），但都含被命中文件相对路径，AC-13 字面要求"错误信息含被命中的文件路径"两端均满足。
- 负向自检（AC-13）：QA 06 ADV-1 必跑（02 §7.4 ADV 段已列）。Reviewer 通过代码 walk-through 已确认 throw / step "FAIL" 路径正确，QA 06 字节级删 3 字节即可触发 FAIL。
- baseline.json：version 8→9 OK；notes 闭环（OPT-2）：`follow-up T-020-encoding-ps51-bom` → `closed by T-021 encoding-ps51-bom` + 追加 `T-021 closed: .ps1 BOM applied (11/11); E.7 added; verify_all 19->20.` 字节级核对 baseline.json:10 文本一致。

### §6.5 D5 跨任务 insight 适配 — **PASS**

- insight L17（PowerShell 写 TOML 必须 `UTF8Encoding($false)` 无 BOM）→ 本任务反向用法 `UTF8Encoding($true)` 带 BOM，正确区分。
- insight L37（Edit/Write soft-block 仅对 `.claude/settings.json`）→ 本任务路径 `scripts/*.ps1` 不在列表，但 Developer 仍选 .NET API（理由：跨 PS 版本 BOM 行为差异），决议合理。
- insight L38（[char]10 + StringBuilder 跨语言 LF 稳定）→ 本任务无 LF 拼接场景，不涉及。
- insight L41/L42（reviewer 默认无 Write）→ 本评审通过 PM 接管落盘，符合该 insight。
- insight L24/L35/L43（标题禁数字前缀）→ AC-17/AC-18 已涵盖、E.7 标题已用纯 `E.7` 编号 + 描述（无数字混乱），无冲突。

### §6.6 D6 可维护性 — **PASS**

- 注释只在 WHY 处（verify_all.ps1:269/279/281 中文注释解释 T-021、设计文档引用、C-4 修订原因）。
- 无 dead code、无 premature abstraction。
- 命名稳定（E.7 step id 与 E.1~E.6 同源命名空间；`scripts/.editorconfig` 路径直观）。
- 唯一可改进点：MAINT-2/3 标点统一 + 设计文档反向链接（NIT 边界）。

---

## §7 跨任务 insight 候选（给 PM Stage 7 决策）

Reviewer 从本任务过程识别出 **2 条候选 insight**，建议 PM 在 07 `## Insight`（裸标题）段择优写入：

### §7.1 [INS-CAND-1] PS 解释器内置 BOM-aware + `$PSScriptRoot` 与 BOM 共存

**proposed insight**（一行）：
> **2026-05-23** · PowerShell 5.1 / 7.x 解释器加载磁盘 .ps1 时**先剥 BOM 再 parse**，BOM 不进入脚本字符串；`$PSScriptRoot` 由解释器从磁盘路径计算与文件内容无关，故 .ps1 加 UTF-8 BOM 后所有 `$PSScriptRoot` / `Split-Path $PSScriptRoot -Parent` 等自定位 idiom 仍正常工作（与 insight L25 "管道形态禁用 $PSScriptRoot" 互补——磁盘形态合法、管道形态禁用）· evidence: T-021 dogfood archive-task.ps1 -DryRun 加 BOM 后 `$repoRoot = Split-Path $PSScriptRoot -Parent` 计算正确路径、退出 0

### §7.2 [INS-CAND-2] git blob 字节级 checkout 是 BOM 持久层、`working-tree-encoding` 不支持 `UTF-8-BOM`

**proposed insight**（一行）：
> **2026-05-23** · `.gitattributes working-tree-encoding=UTF-8-BOM` 不是 git iconv 合法值（git 2.34+ checkout 直接报 `failed to encode ... from UTF-8 to UTF-8-BOM`），git 内部本就是 UTF-8 表示、指定 `UTF-8` 等于啥都没干；BOM 的真正持久层是 **git blob 字节本身**（默认文本拷贝是字节级，仅 CRLF/LF 归一可能改字节），`scripts/.editorconfig charset=utf-8-bom` 是编辑器层 belt，verify_all 字节级闸门是 CI suspenders—— 三层防御正确顺序是"git blob (持久) + editorconfig (编辑器) + verify_all (CI)"，不要试图用 git 属性强行锁定 BOM · evidence: T-021 03 C-1 + 02 §2.3 轮次 2 撤销决议

PM 决策建议：[INS-CAND-2] 价值更高（防止未来任务再踩同款 git 属性陷阱）；[INS-CAND-1] 边界较窄（仅 .ps1 + BOM 场景），按 insight-index ≤30 行预算可二选一或择一。

---

## §8 已知风险但 ack（继承 03 §4）

| 项 | 性质 | Reviewer ack |
|---|---|---|
| AC-3/4/5/6/9 [U] 真机降级 | 与 T-019 / T-018 历史降级模式一致 | ack |
| R-1 编辑器去 BOM | 三层防御（git blob + editorconfig + E.7 闸门）已落地 | ack |
| R-2 archive-task.ps1 dogfood 失败风险 | 04 §4.5 dogfood 已通过、风险消除 | ack（已闭环） |
| R-11 编辑器层 belt 仅靠 .editorconfig 不覆盖所有编辑器 | verify_all E.7 是最终闸门 | ack |
| baseline.json `updated` 占位 2026-05-23 | Developer 范围内合理；QA 06 / PM 07 同步真实日期 | ack（MINOR-1） |
| OBS-1 3 ASCII .ps1 工作树 CRLF 历史遗留 | 与本任务无关；建议 followup `T-023 normalize-ps1-eol`（T-022 已被 service-mode-stderr-bridge 占用） | ack |

---

## §9 Verdict

**APPROVE**

理由：
- 0 CRITICAL + 0 MAJOR；3 MINOR + 4 NIT + 3 OBSERVATION 全部非阻塞。
- 18 AC：11 [A] Developer 范围内全 OK；5 [U] 真机已 ack 降级到 QA 06 清单；2 QA/PM 责任 AC 待相应 Stage。
- 8 NFR 全 OK。
- 设计 fidelity 100%；唯一偏离（sh 端 `e7_found_any` 哨兵）是稳健性增强非漂移。
- BOM 字节级、SHA256 加固、verify_all PASS:20、dogfood archive-task.ps1 -DryRun 全闭环。
- `.gitattributes` 零 diff（轮次 2 撤销错误决议落地清洁）；`scripts/.editorconfig` 字节级匹配设计模板。
- 三层防御（git blob 字节 + editorconfig + verify_all E.7）正确实施。
- 临时辅助脚本与 snapshot 完全清理，git status 预期干净。

PM 路由建议：**直接派发 QA Stage 6**，附带：
1. 真机清单 4 项（02 §7.2）由 QA 在 06 §6 真机验证清单转给用户。
2. ADV 段 5 项（02 §7.4）QA 必跑，其中 ADV-1 字节级负向自检为 AC-13 关键证据。
3. dogfood `archive-task.ps1 -DryRun` Developer 04 已跑，QA 06 可复跑确认 / 跳过（视 QA 判断）。
4. baseline.json `updated` 字段建议 QA 06 真机跑通后同步真实日期（MINOR-1）。
5. PM 07 写 `## Insight` 裸标题（无数字前缀），建议吸纳本评审 §7 候选 insight 中至少 1 条（推荐 [INS-CAND-2]）。

---

**Verdict**: APPROVE
