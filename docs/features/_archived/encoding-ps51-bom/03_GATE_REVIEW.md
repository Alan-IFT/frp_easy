# 03 — Gate Review · T-021 encoding-ps51-bom

> Harness 流水线 Stage 3（Gate Reviewer）。模式：**full**。
> 上游：`docs/features/encoding-ps51-bom/01_REQUIREMENT_ANALYSIS.md`（Verdict = READY）、`docs/features/encoding-ps51-bom/02_SOLUTION_DESIGN.md`（Verdict = READY）。
> 本文件由 Gate Reviewer 独立审查产出；本 agent frontmatter `tools: Read, Glob, Grep`（无 Write），由 PM 接管落盘（insight L42 已知模式）。

---

## §1 评审维度

### D1 需求完整性 — **PASS**

AC-1 ~ AC-18（共 18 条；RA §9 误称 19 条，实际编号止于 AC-18）全部可机械验证：10 条 [A] 自动 + 5 条 [U] 真机 + 3 条文档/格式（AC-16/17/18）；NFR-1 ~ NFR-8 每条都绑定 verify_all 或 §3 步骤；I-1 ~ I-5 全部由 RA 提供候选 + 推荐方向、留给 SA 决议。`[A]` / `[U]` 标记规范、降级模式与 T-019 一致。

### D2 设计可行性 — **WARN**

§3 步骤 1~11 + §9 全部 8 项决议落地；§7.1 ~ §7.4 测试策略覆盖自动/真机/dogfood/adversarial 四层；§5.3 R-4 已独立审计仓库内"第一行字节" grep 用法（Reviewer 复核 = 0 命中，§2 验证段证实）。
**WARN 点**：§2.3 `.gitattributes` 的 `working-tree-encoding=UTF-8-BOM` 决议存在事实层面的不确定性（见 C-1 必须项）；§3 步骤 2 / §3 步骤 5 的 ReadAllText 编码假设有边角风险（见 C-2）。

### D3 风险覆盖 — **WARN**

R-1 ~ R-10 覆盖了编辑器去 BOM、archive-task dogfood、CI 差异、grep 第一行、self-block、sh/ps1 同步漂移、git < 2.10 兼容、二进制 detection、stage 7 PM 调用、baseline.json 解析共 10 条。
**漏列风险**（见 §2 必须/建议项）：
- R-11（漏）：`working-tree-encoding=UTF-8-BOM` 不是 git 官方 iconv 支持的合法值（C-1）。
- R-12（漏）：步骤 2 `ReadAllText(..., UTF8Encoding(false))` 对 noBOM 现状脚本 OK，但若其中**任一文件已被某历史 commit 误存为 GBK** 则字符级回归（步骤 4）才会发现，缺前置 sanity（C-3 建议）。
- R-13（漏）：`install-hooks.ps1` 第 35 行嵌入 `#!/bin/sh` 字面字符串到 `$hookContent` here-string，并通过 `Set-Content` 写入 `.git/hooks/pre-commit`；该 .sh 钩子文件**不应**带 BOM（POSIX 解释器拒），而源 install-hooks.ps1 本身被加 BOM 后，PowerShell 解释器读字符串字面 `'#!/bin/sh'` 时 BOM 已被解释器吞、不会进入字符串 —— 但 Reviewer 已独立核对此点为安全（OPT-1 留档）。

### D4 决议落地 — **PASS**

RA §6 给 SA 的 8 项前置决议清单全部覆盖：

| 编号 | RA 推荐 | SA §9 决议 | 一致性 |
|---|---|---|---|
| I-1 | A 全加 | A 全加 | 一致 |
| I-2 | A 严格 | A 严格 | 一致 |
| I-3 | B `.editorconfig` | **C `.gitattributes`**（替换 RA 推荐） | 已替换 + 给四点理由，**但理由 #2 事实存疑**，见 C-1 |
| I-5 | B 必跑 dogfood | B 必跑 | 一致 |
| §6-4 实现工具 | 留待选 | `.NET WriteAllText + UTF8Encoding($true)` | 已落定 |
| §6-5 step 编号 | 无偏好 | E.7 | 已落定 |
| §6-6 baseline.json | 留待核 | 不改 test_count，只改 notes/version/updated | 已基于 schema 实读决议（§2.5） |
| §6-7 irm \| iex BOM 吞咽 | 留待解释 | `iex` parser 把 `[char]0xFEFF` 当 whitespace 忽略 + §7.1 AC-10 mock 测试 | 已落定 |
| §6-8 降级对账 | 留待复核 | §7.2 5 项真机清单 | 已落定，待 GR 复核 = PASS |

---

## §2 必须修改项（CHANGES REQUIRED）

### C-1【必须】§2.3 + §9 I-3 决议：`working-tree-encoding=UTF-8-BOM` 不是 git 官方合法值

**问题**：
SA §2.3 决议 `.gitattributes` 追加 `*.ps1 working-tree-encoding=UTF-8-BOM eol=lf`，并在 §9 I-3 称"git 官方支持的 BOM 锁定属性（参见 git-attributes(5)）"。
但 `git-attributes(5)` man page 对 `working-tree-encoding` 的官方文档实际只支持 **iconv 兼容的字符编码标签**（如 `UTF-16`、`UTF-16LE`、`UTF-16BE`、`UTF-32`、`Shift_JIS`、`GB18030`、`Big5` 等），且**明文规定** "use UTF-8 as the internal representation"——指定 `UTF-8` 等于啥都没干（git 内部就是 UTF-8）。
`UTF-8-BOM` **不是** iconv 标准编码名（iconv 用 `UTF-8` 并通过 signature 处理 BOM；POSIX iconv -l 输出无此别名）。git 见到不识别的编码值时行为是：(a) 旧版本静默忽略；(b) git ≥ 2.34 在 checkout 时调用 iconv 失败 → 报错 `error: failed to encode 'scripts/install.ps1' from UTF-8 to UTF-8-BOM` → checkout 中断；(c) 中间版本表现不可预测。
结论：此属性**不能**起到 SA 声称的"checkout 时按指定编码写工作区"防回归作用；最坏情况下让用户 `git pull` 直接报错。

**RA / SA 责任**：02_SOLUTION_DESIGN.md §2.3 + §9 I-3 + §5.2 表"Git checkout（含 working-tree-encoding 属性）" + §6 R-7 + §10 O-1 共 5 处需重写。

**SA 必须**给出三选一回滚方案：
- **方案 A**：撤销 `.gitattributes` 改动，单靠 verify_all E.7 + 本任务字节级写 BOM（git blob 内已是 BOM 字节）防回归；记入 §6 新风险 R-11"编辑器层无 belt，仅 suspenders"，可接受（PR 入 main 前 verify_all 拦下）。**Reviewer 倾向方案 A**。
- **方案 B**：改用 `.editorconfig` 在 `scripts/` 加 `[*.ps1]` + `charset = utf-8-bom`（RA §I-3 原推荐方案 B）。覆盖编辑器面比 working-tree-encoding 真实可用；新增一个 `scripts/.editorconfig` 文件。
- **方案 C**：两者皆加：`.editorconfig` + 删 working-tree-encoding 行只保留 `*.ps1 text eol=lf`（无 working-tree-encoding 属性时 git 走默认文本拷贝、字节级保留 BOM）。

**不修则 REJECTED**：用户在新克隆 / `git pull` 时可能直接命中 git checkout 报错，T-021 上线即破产。

---

### C-2【必须】§3 步骤 2 `ReadAllText` 编码假设需显式 fallback 防 GBK 误读

**问题**：
§3 步骤 2 用 `[System.IO.File]::ReadAllText($path, [System.Text.UTF8Encoding]::new($false))` 读原文件。`UTF8Encoding($false)` 仅控制 BOM 探测的"写时不输出"，**读时仍按 UTF-8 严格解码**；若 .ps1 文件中存在任一字节序列不是合法 UTF-8（如某行历史误用 GBK 0xD6 0xD0 = "中"），ReadAllText 会用 `�` 替换字符或抛 `DecoderFallbackException`，让步骤 4 字符级回归对比"看似通过"但内容已 silently 损坏。
.NET `UTF8Encoding($false, $false)`（第二参数 = throwOnInvalidBytes）默认 false = 静默替换；第二参数 = true 才抛异常。

**SA 必须**：
- 步骤 2 改为 `new System.Text.UTF8Encoding($false, $true)`（`encoderShouldEmitUTF8Identifier=$false`、`throwOnInvalidBytes=$true`），让任何非法 UTF-8 字节立即抛异常，Developer 立即可见。
- 或在步骤 4 增加"读字节级 hash（SHA256）"对比而非 `-ne` 字符串对比：`(Get-FileHash $orig -Algorithm SHA256).Hash` vs `(Get-FileHash $new -Algorithm SHA256).Hash[3..end]`（跳过 BOM 3 字节后比较剩余字节 hash），从字节层证明零修改。
- 二选一 + 明文记入 §3 步骤 2 / §3 步骤 4 / NFR-1 关联验证段。

**不修则 APPROVED WITH CONDITIONS**：现网 11 个 .ps1 当前全是合法 UTF-8（INPUT.md 标记 noBOM ASCII / noBOM zh 都是合法 UTF-8 子集），实际概率低；但 D6 缺失任何一道字节级 sanity 是设计漏洞。

---

### C-3【必须】§7.1 AC-10 的 `iex` BOM 吞咽 mock 测试在 PS5.1 vs PS7 行为不同需明确

**问题**：
§7.1 AC-10 行写 `[char]0xFEFF + 'echo hello' | iex`，作为机制层证据。但：
- 在 PS7.x（QA 主机）`iex` 接收字符串时 `[char]0xFEFF`（U+FEFF ZERO WIDTH NO-BREAK SPACE）会被 parser 当 whitespace 容忍，输出 `hello`。
- 在 PS5.1 行为**不保证**相同 —— PS5.1 parser 对 U+FEFF 的处理在某些 build 上会当 `unexpected token` 抛 ParserError；这正是本任务起源问题"PS5.1 解析中文字符的脆弱性"的近亲。
- 而 `irm | iex` 真实路径下，`Invoke-RestMethod` 返回 `string` 时 PS HTTP 客户端**已经** BOM-aware 剥离了 BOM（实测 PS7 / PS5.1 一致），所以 `iex` 拿到的字符串前根本没有 `[char]0xFEFF` —— mock 测试和真实路径**断开**。

**SA 必须**：
- 将 §7.1 AC-10 改为：(a) [A] 项断言"`Invoke-RestMethod` 模拟（用 `Test-Path file://` URL 或本地 `http.server` mock）后字符串首字符 ≠ U+FEFF"；或 (b) 直接降为 [U] 真机由用户 AC-9 复跑时一并观察（与 AC-9 合并）。
- §9 I-6/7 第 7 项"irm | iex BOM 吞咽机制"的解释需修订为"Invoke-RestMethod 客户端层剥 BOM、不是 iex parser 层吞 BOM"，否则把后续维护者引向错误的 mental model。

**不修则 APPROVED WITH CONDITIONS**：AC-10 现状 mock 在 QA 主机会 PASS，但**不证明**真实 irm 路径下 BOM 被正确处理；属"测试给假信心"问题。

---

### C-4【必须】§3 步骤 5 / 步骤 6 加 E.7 块进 verify_all 之前必须先验证 E.7 PS 块语义与 sh 块语义**等价 FAIL**

**问题**：
§2.2 给出 PS 块与 sh 块两套伪码。比较：
- PS 块：`throw "Missing UTF-8 BOM in:`n$($missing -join "`n")"` → 走 `Step` 的 try/catch → FAIL 输出 `[E.7] ... FAIL` + DarkRed 详情行。
- sh 块：`step "E.7" "..." "FAIL" "$(echo -e $e7_missing)"` → 走 `step()` 函数 → 输出 `[E.7] ... FAIL` + `      $detail`。
两个 FAIL 路径的**退出码语义一致**（均 `errors++` → 最终 exit 2），**输出格式不一致但可接受**。
但 §2.2 PS 块用 `$root.Length + 1` 截取相对路径，而 verify_all.ps1 中 `$root = (Get-Location).Path`（行 26），若 verify_all 不是从仓库根目录调用、`$missing += $f.FullName.Substring($root.Length + 1)` 会 `ArgumentOutOfRangeException`（startIndex > length）。

**SA 必须**：在 §2.2 PS 伪码加 guard：
```powershell
$relPath = if ($f.FullName.StartsWith($root)) { $f.FullName.Substring($root.Length + 1) } else { $f.FullName }
```
或显式断言 `if (-not $f.FullName.StartsWith($root)) { throw "verify_all must run from repo root" }`。

**不修则 APPROVED WITH CONDITIONS**：QA 主机历来从 `c:\Programs\frp_easy` 跑 verify_all，命中概率低，但 CI / 其他开发者环境从子目录调用即崩。

---

## §3 可选改进项

### OPT-1【建议】§3 步骤 11 dogfood 命令补充字节核对

当前命令 `pwsh -File scripts\archive-task.ps1 -Task encoding-ps51-bom -DryRun`。
Reviewer 实读 `archive-task.ps1` L17~L21 确认 `-DryRun` 参数存在、L109~L114 确认 dry-run 输出正确；与 SA §3 步骤 11 一致。
建议在 dogfood 命令前后加一行字节核对：`(Get-Content scripts\archive-task.ps1 -Encoding Byte -TotalCount 3) -join ','` 应 = `239,187,191`，证明 dogfood 是真在已加 BOM 版本上跑。

### OPT-2【建议】§2.5 baseline.json `notes` 文案精修

SA §2.5 决议 notes 改 `"verify_all PASS 19/19 stable x3 runs."` → `"verify_all PASS 20/20 stable x3 runs (T-021 added E.7 ps1 BOM check)."`，但当前 baseline.json L10 实际文案已含 T-019 大段说明（"... AC-18 PS 5.1 zh-CN disk-load .ps1 parse FAIL is T-018 historical regression (not T-019 induced); follow-up T-020-encoding-ps51-bom. ..."）。建议追加一句"T-021 closed: .ps1 BOM applied (11/11); E.7 added; verify_all 19→20."，并把 `follow-up T-020-encoding-ps51-bom` 这句改为 `closed by T-021 encoding-ps51-bom`，保持 narrative 闭环。

### OPT-3【建议】§3 步骤 9 dev-map.md 改动位置

Reviewer Grep `docs\dev-map.md` L23~L27 确认 `scripts/` 行存在，但**已含历史 T-008 / T-013 / T-006 / T-009 等多任务追加子句**，行已较长。建议改"追加到 L25 末尾"为"在 L27 后另起一行 'T-021：scripts/*.ps1 统一 UTF-8 BOM ...'"，避免单行过长难读。SA 可保留原方案，OPT 仅供参考。

### OPT-4【建议】E.7 命名缩短

SA §10 O-5 自评 E.7 标题 60 字符过长（"All scripts/\*.ps1 start with UTF-8 BOM (EF BB BF)"）；Reviewer 同意 O-5 自评，建议简化为 `"scripts/*.ps1 have UTF-8 BOM"`（28 字符）。与 E.6 `"Adversarial tests section present in completed task reports"`（57 字符）量级相当但更精简。

---

## §4 已知风险但接受

PM 在派发 Developer 前必须 ack 以下降级 / best-effort 项：

| 项 | 来源 | 性质 | PM ack 含义 |
|---|---|---|---|
| **AC-3 / AC-4 / AC-5 / AC-6 / AC-9 [U] 真机降级** | RA §2.2 / RA §I-4 / SA §7.2 | 5 项真机 AC 由用户在 PS 5.1 + zh-CN 主机自跑；QA 主机不复现 | 与 T-019 / T-018 历史降级模式一致；PM 在 07 交付时高亮真机清单（SA §10 O-4） |
| **R-1 编辑器去 BOM** | SA §6 R-1 | 中概率；verify_all E.7 是最终闸门，编辑器层 belt 由 C-1 必须修改项重定（取代 working-tree-encoding） | PM 接受"PR 提交前漏跑 verify_all 的 dev 可能误推无 BOM 改动，CI 拦下"作为可接受反馈链 |
| **R-2 archive-task.ps1 dogfood 失败 → sub-fix** | SA §6 R-2 / I-5 | 极低概率；I-5 dogfood 在 QA 06 前置抓 bug；若失败则单独 followup（不阻塞其他 10 个 .ps1） | PM 接受"dogfood 失败 = QA 标 BLOCKER 回退到 SA 决议单独排除 archive-task.ps1" |
| **R-3 CI 跨平台 BOM 解析差异** | SA §6 R-3 | 极低；T-013 已验证 GitHub Actions ubuntu/windows runner .ps1 路径；AC-7 PS7 显式不回归 | PM 接受"若 CI 出 BOM 解析回归，回滚 PR 不计入本任务交付" |
| **R-7 git < 2.10 客户端不识别 working-tree-encoding** | SA §6 R-7 | 由 C-1 修订后此风险消失（如选方案 A 撤销该属性） | 与 C-1 联动 |

---

## §5 总裁决

**CHANGES REQUIRED**

修改清单（必须项不修则 REJECTED）：
- **C-1**【必须】撤销或替换 `.gitattributes` `working-tree-encoding=UTF-8-BOM` 决议（事实层错误，可能让 `git pull` 报错）。
- **C-2**【必须】§3 步骤 2 `ReadAllText` 加 `throwOnInvalidBytes=$true` 或步骤 4 改为字节级 SHA256 对比。
- **C-3**【必须】§7.1 AC-10 修订 `iex` BOM 吞咽机制描述 + 改为真实路径 mock 或合并到 AC-9 [U] 真机。
- **C-4**【必须】§2.2 PS 伪码加 `$root` startsWith guard，避免 `Substring` 越界。

建议项（不阻塞）：OPT-1 dogfood 字节核对、OPT-2 baseline.json notes 闭环文案、OPT-3 dev-map.md 改动位置、OPT-4 E.7 标题缩短。

PM 路由建议：**BLOCKED ON DESIGN** → 退回 Solution Architect 修订 02 的 §2.3 / §3 步骤 2 / §3 步骤 4 / §7.1 AC-10 / §9 I-3 / §9 I-6/7 第 7 项 / §2.2 PS 伪码共 7 处；修订后重跑 Stage 3 Gate Review（差异审查即可，不必全量重审）。

---

## §6 高概率开发问题（预答）

| Q | 预答 |
|---|---|
| Q1：Edit/Write 工具能否用来加 BOM？ | **不能**。Edit 字符串替换不改字节级 BOM 头；Write 在 PS7 / PS5 默认行为不一致（PS7 默认无 BOM、PS5 默认有 BOM）。**必须**用 `[System.IO.File]::WriteAllText + UTF8Encoding($true)`（SA §3 步骤 2 注释已明文，但 C-2 还要加 throwOnInvalidBytes 参数）。 |
| Q2：步骤 2 跑完后 git diff 会显示什么？ | 项目 `.gitattributes` 现有 `* text=auto eol=lf`、未对 .ps1 标 binary；git heuristic 看 NUL 字节判二进制，BOM 是 3 个非 NUL 字节，不触发 binary 显示；预期为文本 diff，每文件首行前出现 `<U+FEFF>` 字符标记 + 0 内容行变化。 |
| Q3：步骤 4 字符级回归发现 DRIFT 怎么办？ | 立即停下、回到步骤 2 调查。最可能根因：(a) ReadAllText 用 UTF8Encoding($false) 读时遇非法 UTF-8 字节静默替换为 U+FFFD（**C-2 必须修复后**会抛异常立即可见）；(b) 文件本身已损坏。**不要**手工 patch，必须从 `git show HEAD:scripts/xxx.ps1` 恢复后重做。 |
| Q4：E.7 在 verify_all 跑时位置？ | SA §2.2 决议放 E 段尾、紧接 E.6 之后、`# --- Summary ---` 之前；verify_all.ps1 当前 L254~L266 是 E.6 块，插入点 L267 之后、L268 `# --- Summary ---` 之前。verify_all.sh 当前 L262~L276 是 E.6 块，插入点 L277 之后、L279 `# Summary` 之前。 |
| Q5：步骤 11 dogfood 失败如何降级？ | 见 R-2 缓解。失败则**不**对 archive-task.ps1 加 BOM（剩余 10 个仍加），E.7 检查项调整为"排除 archive-task.ps1"，开 followup 任务单独调查 PS 解释器边角 bug。这是 R-2 唯一已批准的降级路径。 |

---

## §7 Reviewer 独立验证清单（证据）

- 读 01 / 02 / INPUT.md 全文，确认两个 Verdict = READY。
- 读 `.harness/agents/gate-reviewer.md` 角色契约，确认本 agent 工具集 = Read/Glob/Grep（无 Write），落盘需 PM 接管（insight L42）。
- 读 `.harness/insight-index.md` 全部 36 条，与设计交叉验证：
  - insight L17（PowerShell 写 TOML 无 BOM）→ SA §3 步骤 2 正确做反向用法（$true 带 BOM）。
  - insight L37（Edit/Write soft-block 仅对 `.claude/settings.json`）→ SA §3 步骤 2 正确判定 `scripts/*.ps1` 不在列表、但仍选 .NET API（理由：跨 PS 版本 BOM 行为差异），决议合理。
  - insight L38（PowerShell `[char]10` + StringBuilder 跨语言转义稳定）→ 本任务 BOM 字节级写入用 .NET API，与 L38 同源安全模式，无冲突。
  - insight L42（reviewer 默认无 Write）→ 本审查响应正文交还 PM，符合该 insight。
  - insight L24 / L35 / L43（标题禁数字前缀）→ AC-17 / AC-18 已涵盖，SA §7.4 ADV 段强调"裸标题"，无冲突。
- 读 `scripts/verify_all.ps1` L19~L266（含 E.1~E.6 全部 Step 块）+ `scripts/verify_all.sh` L18~L276，确认 E.7 插入点正确、Step / step 函数签名与 §2.2 伪码兼容。
- 读 `.gitattributes` 全 11 行，确认现有规则不与 BOM 冲突；但 SA §2.3 拟加的 `working-tree-encoding=UTF-8-BOM` 是 git iconv 不支持的值（**C-1 阻塞点**）。
- 读 `scripts/baseline.json` 全 12 行，确认 schema 含 version / created / updated / test_count / passing_count / go_tests / frontend_tests / warnings_baseline / notes 共 9 字段；SA §2.5 决议"只改 notes 文本 + version 升 + updated 改日期、不动 test_count"与 schema 实际匹配（test_count = Go+前端 unit 总数 = 335，与 verify_all 19 项检查无关）。
- 读 `scripts/install.ps1` L1~L10 + `scripts/archive-task.ps1` L1~L80 确认两个代表脚本的当前内容形态，与 INPUT.md 字节统计一致；archive-task.ps1 L17~L21 / L109~L114 确认 `-DryRun` 参数存在（SA §3 步骤 11 dogfood 命令可执行）。
- 读 `AI-GUIDE.md` L1~L80 确认规则索引、archive-task 用法描述（L71）与 SA 设计无冲突。
- Grep `scripts/*.{ps1,sh}` 第一行字节场景：`Select-Object -First N` 全部作用于数组截取、非文件首字节；`head -1` 在 start-e2e-server.sh L22 作用于 find 输出、非 .ps1 首字节；`install-hooks.ps1` L35 `#!/bin/sh` 是 here-string 字面写入 `.git/hooks/pre-commit`、与 .ps1 自身 BOM 无关。**R-4 风险面独立复核 = 0 命中**，SA §5.3 结论正确。
- Grep `working-tree-encoding|charset|utf-?8-?bom` 在仓库范围内，确认除 02_SOLUTION_DESIGN.md / 01_REQUIREMENT_ANALYSIS.md 外**无任何**其他现存 `.gitattributes` / `.editorconfig` 引用，C-1 替换决议无遗留兼容包袱。

---

**Verdict**：CHANGES REQUIRED
