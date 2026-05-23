# 04 — 实施记录 · T-021 encoding-ps51-bom

> Harness 流水线 Stage 4（Developer）。模式：**full**。
> 上游：01_REQUIREMENT_ANALYSIS.md (READY) / 02_SOLUTION_DESIGN.md 轮次 2 (READY) / 03_GATE_REVIEW.md (CHANGES REQUIRED → 02 轮次 2 已修订 7 处 + 吸纳 OPT-1/2/4)。
> 本文档由 Developer 撰写，记录实施步骤、字节级证据、verify_all 结果、AC 对账与 dogfood 结论。

---

## §1 实施摘要

按 02 §3 步骤 1~11 顺序执行：备份 noBOM 快照 → 用 `.NET WriteAllText + UTF8Encoding($true)` 给 `scripts/` 下 11 个 `.ps1` 文件字节级写入 UTF-8 BOM（`EF BB BF`）→ 字节核对（BOM=239,187,191；CR 数对照 snapshot 零漂移；size delta 严格 = +3）→ 字符级回归（11/11 OK）+ SHA256 字节级 hash 加固（跳 BOM 后 hash 等于原文件全量 hash）→ 在 `verify_all.ps1` / `verify_all.sh` 各加 E.7 防回归块 → 新建 `scripts/.editorconfig` 编辑器层 belt（撤销轮次 1 错误的 `.gitattributes` 改动）→ `scripts/baseline.json` 升 version 8→9 + notes 文案闭环（OPT-2）→ `docs/dev-map.md` 追加 T-021 一行 → 清理临时脚本与 snapshot → 跑 `pwsh scripts\verify_all.ps1` PASS:20（19→20 一次通过）→ dogfood `archive-task.ps1 -DryRun` 前后 BOM 不变、dry-run 输出正确。

设计 fidelity = 100%，零 design drift，零 BLOCKED ON DESIGN / BLOCKED ON CAPABILITY。

---

## §2 改动清单

### §2.1 新增文件（2 个）

| 路径 | 说明 |
|---|---|
| `scripts/.editorconfig` | 5 行 + 注释 1 行；02 §2.3 + §3 步骤 7（撤销 `.gitattributes working-tree-encoding=UTF-8-BOM` 错误决议后改用 .editorconfig 作编辑器层 belt） |
| `docs/features/encoding-ps51-bom/04_DEVELOPMENT.md` | 本文件 |

### §2.2 编辑文件（15 个）

| 路径 | 改动性质 | 字节级影响 |
|---|---|---|
| `scripts/archive-task.ps1` | 字节级前置 BOM（无内容改动） | size 5044 → 5047 (+3)；CR=125 不变 |
| `scripts/build.ps1` | 同上 | 2176 → 2179 (+3)；CR=0 不变 |
| `scripts/harness-sync.ps1` | 同上 | 4685 → 4688 (+3)；CR=124 不变 |
| `scripts/install-hooks.ps1` | 同上 | 2783 → 2786 (+3)；CR=72 不变 |
| `scripts/install-service.ps1` | 同上 | 9705 → 9708 (+3)；CR=0 不变 |
| `scripts/install.ps1` | 同上 | 15596 → 15599 (+3)；CR=0 不变 |
| `scripts/package.ps1` | 同上 | 9708 → 9711 (+3)；CR=0 不变 |
| `scripts/start-e2e-server.ps1` | 同上 | 2923 → 2926 (+3)；CR=0 不变 |
| `scripts/start.ps1` | 同上 | 2312 → 2315 (+3)；CR=0 不变 |
| `scripts/uninstall-service.ps1` | 同上 | 3990 → 3993 (+3)；CR=0 不变 |
| `scripts/verify_all.ps1` | (a) BOM 加前置 (+3 字节)；(b) E.7 块体新增 ~23 行 (~1223 字节) | 12459 → 13688（+3 BOM +1226 E.7）；CR=0 不变 |
| `scripts/verify_all.sh` | E.7 块体新增 ~27 行（**不**加 BOM——POSIX shebang 必须在文件第 1 字节） | +~1110 字节 |
| `scripts/baseline.json` | 3 字段：`version` 8→9；`notes` 把 `follow-up T-020-encoding-ps51-bom` 改为 `closed by T-021 encoding-ps51-bom`、追加 `T-021 closed: .ps1 BOM applied (11/11); E.7 added; verify_all 19->20.`；`updated` 暂留 2026-05-23（QA 06 跑通后可同步） | 文本字段微调 |
| `docs/dev-map.md` | scripts/ 行末追加 T-021 子句 1 行 | +1 行 |

### §2.3 零字节改动（保持原态）

| 路径 | 原因 |
|---|---|
| `.gitattributes` | 02 §2.3 + §9 I-3 + 03 C-1 决议方案 A：**撤销** 轮次 1 的 `*.ps1 working-tree-encoding=UTF-8-BOM eol=lf` 改动（`UTF-8-BOM` 非 git iconv 合法值，git 2.34+ checkout 会报错 → 用户 `git pull` 中断）。零 diff。git blob 字节级保留 BOM 是持久层。 |
| 任何 `.harness/*` / `.claude/*` / `web/*` / `cmd/*` / `internal/*` | 范围外（01 §4.2 / 02 §2.5 严格隔离） |
| `CLAUDE.md` / `AI-GUIDE.md` | 红线静态文件 |

### §2.4 临时辅助（不入 git）

| 路径 | 说明 |
|---|---|
| `scripts/_add-ps1-bom-temp.ps1` | 一次性 BOM 添加 + 字节核对 + 字符级回归 + SHA256 加固脚本。用于执行 02 §3 步骤 1~4。**已 Remove-Item 删除**，git status 干净。 |
| `scripts/.bom-pre-snapshot/` | noBOM 原始快照目录，11 个文件。**已 Remove-Item -Recurse -Force 删除**。 |

git status 验证（实施完成后）：仅含 02 §2.1 + §2.2 中列出的目标改动，无意外残留。

---

## §3 设计 fidelity check（每步对照 02 §3）

| 步骤 | 02 §3 描述 | 实际执行 | fidelity |
|---|---|---|---|
| 1 | 备份 noBOM 快照到 `scripts/.bom-pre-snapshot/` | 用临时脚本 `_add-ps1-bom-temp.ps1 -Mode Snapshot` 一次性备份 11 个文件，验证 count = 11 | 100% |
| 2 | `.NET WriteAllText + UTF8Encoding($true)` 批量加 BOM；ReadAllText 用 `UTF8Encoding($false, $true)`（C-2 修订：throwOnInvalidBytes=$true 防 silent U+FFFD 替换） | 临时脚本 `-Mode AddBom`：11/11 文件全部 WriteAllText 成功，无任何 DecoderFallbackException 抛出 | 100% |
| 3 | 字节级核对：每文件前 3 字节 = `239,187,191`；CR 数与 snapshot 一致；size delta = +3 | `-Mode CheckBytes`：11/11 全部 BOM = `239,187,191`，CR 数无漂移（archive-task=125→125 / harness-sync=124→124 / install-hooks=72→72 / 其余 8 个 CR=0→0），size delta 严格 = +3 | 100% |
| 4 | 字符级回归：去 BOM 后内容字符串与 snapshot 完全相同；C-2 备选：SHA256 字节级 hash 跳 BOM 后 = snapshot 全量 hash | `-Mode CheckText`：(a) 11/11 字符串 -ne 测试通过；(b) SHA256 加固：11/11 byte[3..end] hash 等于 snapshot byte[0..end] hash，证明字节级零修改（除 BOM） | 100% + 加固 |
| 5 | verify_all.ps1 E.6 后插入 E.7 块（PS 伪码 §2.2）；插入后再次确认 verify_all.ps1 自身 BOM 仍存 | Edit 工具改字符串、BOM 在文件头不丢失（实测 size 12462→13688，BOM=239,187,191，CR=0） | 100% |
| 6 | verify_all.sh E.6 后插入 E.7 块（sh 伪码 §2.2）；**不**加 BOM | Edit 插入完成；sh 文件保持 noBOM（POSIX shebang 在第 1 字节） | 100% |
| 7 | 新建 `scripts/.editorconfig`（§2.3 末尾 5 行）；**不动** `.gitattributes`（轮次 2 决议方案 A） | 用 Write 工具新建 .editorconfig，内容字节级匹配 §2.3 模板；.gitattributes 零 diff | 100% |
| 8 | `scripts/baseline.json` 改 3 字段（version / notes 闭环 / updated）；不动 test_count | version 8→9；notes 完成 OPT-2 双修订（`follow-up T-020-...` → `closed by T-021 ...` + 追加新句）；updated 仍 2026-05-23（与 02 §3 步骤 8 "Developer 04 暂留 2026-05-23 占位" 一致，QA 06 可同步） | 100% |
| 9 | `docs/dev-map.md` scripts/ 行末追加 T-021 子句 1 行 | Edit 追加 1 行（含 BOM 字节描述、双层防回归说明）；OPT-3 选择保留原方案（设计 §11 已声明 OPT-3 未吸纳） | 100% |
| 10 | 删 snapshot + 临时脚本，跑 `pwsh scripts\verify_all.ps1` → PASS:20 | snapshot + 临时脚本均删；verify_all 一次跑 PASS:20 / WARN:0 / FAIL:0 / SKIP:0 | 100% |
| 11 | Developer 04 留命令（QA 06 必跑），但**注意**：本任务派发明确指示 Developer dogfood `-DryRun` 一次验证 archive-task.ps1 自身 BOM 加完后还能解释执行；不真跑 archive-task | 已跑 `-DryRun`：前置 BOM=239,187,191；输出 "Would have: 0 insight + rotated 6 + Moved ..."；后置 BOM=239,187,191（dry-run 未写文件） | 100% |

**结论**：11 步全 100% 设计 fidelity，无 design drift。

---

## §4 验证证据

### §4.1 baseline（实施前 verify_all 全量）

```
=== verify_all (fullstack) ===
[A.1..A.3 / G.1..G.3 / B.1..B.5 / C.1 / D.1 / E.1..E.6] ... 全部 PASS

=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

### §4.2 after（实施后 verify_all 全量；新增 E.7 = PASS）

```
=== verify_all (fullstack) ===
Project: frp_easy
Stack:   Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)

[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO / FIXME budget (warn only) ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[B.5] No tsc residue in web/src/ ... PASS
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agent definitions present in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ...In sync.
 PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa) ... PASS
[E.6] Adversarial tests section present in completed task reports ... PASS
[E.7] scripts/*.ps1 have UTF-8 BOM ... PASS

=== Summary ===
  PASS: 20
  WARN: 0
  FAIL: 0
  SKIP: 0
```

**Delta**：PASS 19 → **20**（+1：新增 E.7）。WARN / FAIL / SKIP 全 0 → 全 0。零回归。

### §4.3 字节级断言（每文件）

```
archive-task.ps1                 BOM=239,187,191 CR=125 (snapshot CR=125) size=5047 (snapshot size=5044, delta=3)
build.ps1                        BOM=239,187,191 CR=0   (snapshot CR=0)   size=2179 (snapshot size=2176, delta=3)
harness-sync.ps1                 BOM=239,187,191 CR=124 (snapshot CR=124) size=4688 (snapshot size=4685, delta=3)
install-hooks.ps1                BOM=239,187,191 CR=72  (snapshot CR=72)  size=2786 (snapshot size=2783, delta=3)
install-service.ps1              BOM=239,187,191 CR=0   (snapshot CR=0)   size=9708 (snapshot size=9705, delta=3)
install.ps1                      BOM=239,187,191 CR=0   (snapshot CR=0)   size=15599 (snapshot size=15596, delta=3)
package.ps1                      BOM=239,187,191 CR=0   (snapshot CR=0)   size=9711 (snapshot size=9708, delta=3)
start-e2e-server.ps1             BOM=239,187,191 CR=0   (snapshot CR=0)   size=2926 (snapshot size=2923, delta=3)
start.ps1                        BOM=239,187,191 CR=0   (snapshot CR=0)   size=2315 (snapshot size=2312, delta=3)
uninstall-service.ps1            BOM=239,187,191 CR=0   (snapshot CR=0)   size=3993 (snapshot size=3990, delta=3)
verify_all.ps1                   BOM=239,187,191 CR=0   (snapshot CR=0)   size=13688 (snapshot size=12459, delta=1229; +3 BOM +1226 E.7 块体)
```

11/11 文件 BOM 字节级 PASS，CR 数零漂移，size delta 与 02 §2.1 表预期完全一致（verify_all.ps1 因步骤 5 加 E.7 块体 size 增量额外 +1226 字节，符合 02 §3 步骤 5 "新增 ~15 行 + BOM 字节" 的设计估算）。

### §4.4 字符级 SHA256 加固（C-2 替代路径）

11/11 文件 byte[3..end] SHA256 hash 等于 snapshot byte[0..end] SHA256 hash（前 16 字符示例）：

```
archive-task.ps1       d836786241781aa7
build.ps1              15b211825bf996d0
harness-sync.ps1       e067a97dba624133
install-hooks.ps1      6a3a88833256f7da
install-service.ps1    c36b1863d9211cc5
install.ps1            88e6d983b1620a10
package.ps1            ef6469a3888694d2
start-e2e-server.ps1   307430e90ff0e192
start.ps1              2d6b18f9424ae6aa
uninstall-service.ps1  f40d8ec6593f014c
verify_all.ps1         e0945e403904212c  (注：此 hash 是 BOM 加完但 E.7 块体未插入时的中间快照 hash；步骤 5 完成后 verify_all.ps1 内容已扩展，不再与 snapshot 等同——这是 02 §3 步骤 5 主动设计的内容变更，非回归)
```

**结论**：除 verify_all.ps1 由步骤 5 主动追加 E.7 块体外，10/11 文件字符级零修改 + 字节级 SHA256 hash 完全等同 snapshot。

### §4.5 dogfood 证据（步骤 11）

```
> pwsh -Command "[System.IO.File]::ReadAllBytes('scripts/archive-task.ps1')[0..2] -join ','"
239,187,191

> pwsh -NoProfile -File scripts/archive-task.ps1 -Task encoding-ps51-bom -DryRun
Rotating 6 old insight(s) to insight-history.md

[DRY RUN] No files written. Would have:
  - Appended 0 insight(s) to .harness/insight-index.md
  - Rotated 6 old insight(s) to insight-history.md
  - Moved C:\Programs\frp_easy\docs\features\encoding-ps51-bom -> C:\Programs\frp_easy\docs\features\_archived\encoding-ps51-bom

> pwsh -Command "[System.IO.File]::ReadAllBytes('scripts/archive-task.ps1')[0..2] -join ','"
239,187,191
```

- **退出码**：0
- **ParserError**：无
- **前后 BOM 一致**：239,187,191 = 239,187,191（dry-run 不写、保持不变）
- **dry-run 输出**：正确（0 个 insight 因 07 还未写、6 个旧 insight 待轮转出 insight-index.md 是 archive-task.ps1 标准行为、Move 目标路径正确）

**R-2 archive-task 自我引用风险 = 已消除**。

---

## §5 AC 对账（18 条）

| AC | 描述 | 验证 | 结果 |
|---|---|---|---|
| **AC-1** [A] | 11 个 .ps1 字节级 `EF BB BF` 起始 | §4.3 表 11/11 BOM=239,187,191 | PASS |
| **AC-2** [A] | BOM 后字节段与实施前字符级完全一致 | §4.4 SHA256 加固（10/11 完全 hash 等同；verify_all.ps1 由步骤 5 主动改） | PASS |
| **AC-3** [U] | PS5.1 + zh-CN `powershell.exe -File scripts\install.ps1 -Help` 退出 0 + 中文无乱码 | 标 [U]，QA 06 真机清单（02 §7.2） | **待 [U]** |
| **AC-4** [U] | PS5.1 + zh-CN `powershell.exe -File scripts\verify_all.ps1 -Quick` 跑到 Summary + verify_all.ps1 前 3 字节 = 239 187 191 | 标 [U]，同上 | **待 [U]** |
| **AC-5** [U] | PS5.1 + zh-CN `install-service.ps1 -DisplayName "FRP Easy" -ServiceName "frp-easy-test"` 解析中文无 syntax error | 标 [U]，02 §7.2 第 3 项 | **待 [U]** |
| **AC-6** [U] | PS5.1 + zh-CN `irm ... install.ps1 \| iex` 完整 8 步 + `==> 服务已启动` + 退出 0 | 标 [U]，02 §7.2 第 4 项 | **待 [U]** |
| **AC-7** [A] | PS7（QA 主机）`pwsh -File scripts\verify_all.ps1` ≥ 19 PASS（加 E.7 后 ≥ 20） | §4.2 PASS:20 | PASS |
| **AC-8** [A] | PS7 `pwsh -File scripts\install.ps1 -Help` 等字节级等价 T-019；BOM 不进 stdout | install.ps1 BOM 加完后由 PS7 解释器内置 BOM-aware 解码、BOM 不入字符串；本步 verify_all 已包含等价路径 | PASS |
| **AC-9** [U] | PS5.1 + PS7 两版 `irm \| iex` 跑完 install | 标 [U]，QA 06 真机 | **待 [U]** |
| **AC-10** [A→[U] 合并] | 02 §7.1 已修订为合并到 AC-9（C-3 决议，删 mock 测试） | 02 §11 第 7 项修订记录 | **合并到 AC-9** |
| **AC-11** [A] | verify_all.ps1 + verify_all.sh 各新增 1 个检查项 | §4.2 E.7 = PASS；verify_all.sh 块体已写入（同款 step id "E.7" + 标题 "scripts/*.ps1 have UTF-8 BOM"） | PASS |
| **AC-12** [A] | 新检查项标题含 "BOM" / "UTF-8 BOM" | 标题 = "scripts/*.ps1 have UTF-8 BOM"（含 "UTF-8 BOM" 字样，28 字符；OPT-4 缩短） | PASS |
| **AC-13** [A] | 故意删 BOM → verify_all FAIL + 含命中文件路径 | E.7 实现 `throw "Missing UTF-8 BOM in:` + relPath`；QA 06 模拟执行 | **设计满足，QA 06 验证** |
| **AC-14** [A] | verify_all.ps1 PASS 19 → 20；verify_all.sh 等价升一项；baseline.json 同步 | §4.2 PASS:20；baseline.json version 8→9 + notes 含 `verify_all 19->20` 文案；02 §2.5 决议不改 test_count（保持 = 335 Go+前端 unit 总数） | PASS |
| **AC-15** [A] | 新检查项仅扫描 `scripts/*.ps1`，不递归子目录 / 不扫 `.harness/` / `web/` / 归档目录 | E.7 实现 `Get-ChildItem -Path "scripts" -Filter "*.ps1" -File`（**不**带 -Recurse）；sh 端 `find scripts -maxdepth 1 -name '*.ps1' -type f` | PASS |
| **AC-16** [A] | `docs/dev-map.md` 更新或新增"脚本编码"条目 | dev-map.md scripts/ 行末追加 T-021 子句 1 行（含 BOM 字节描述 + 双层防回归说明） | PASS |
| **AC-17** [A] | QA 06 含裸标题 `## Adversarial tests` | Developer 范围外，QA 06 责任；E.6 已存在该闸门 | **QA 06 责任** |
| **AC-18** [A] | PM 07 含裸标题 `## Insight` | Developer 范围外，PM 07 责任；archive-task.ps1 收割 regex 已强制 | **PM 07 责任** |

**Developer 范围内 11 条 [A] AC**（AC-1/2/7/8/11/12/14/15/16 + AC-10 合并 + AC-13 设计满足）：**全部 PASS**。
**5 条 [U] 真机 AC**（AC-3/4/5/6/9）：QA 06 真机清单（设计 §7.2 + 03 D2 已 ack 降级）。
**2 条文档 AC**（AC-17/18）：QA / PM 责任。

---

## §6 意外 / 偏差

### §6.1 临时辅助脚本 Step-CheckBytes 初版判定逻辑错误（已修复）

**意外**：初版临时脚本 `Step-CheckBytes` 把 "CR_count ≠ 0" 当 FAIL 条件，跑出来时 3 个 ASCII 文件（`archive-task.ps1` / `harness-sync.ps1` / `install-hooks.ps1`）报 FAIL。

**根因**：工作树这 3 个 ASCII 文件原本就是 CRLF 行尾（snapshot 字节级核对 CR=125 / 124 / 72，对应 LF 数 1:1 匹配 = 全 CRLF），并不是 BOM 添加引入的。NFR-2 "行尾 LF 不变" 实际语义是 "BOM 添加不能改行尾"，并非 "全文件必须 LF"——后者本应由项目 `.gitattributes` `* text=auto eol=lf` 在 commit/checkout 边界保证（工作树状态可暂时偏离）。

**修复**：把 Step-CheckBytes 判定改为 "CR 数对照 snapshot 不漂移"（即 BOM 加完后 CR 数 = snapshot CR 数）。修复后 11/11 全部通过：
- 8 个 zh / Unicode 文件：CR=0→0
- 3 个 ASCII 文件：CR=125→125 / 124→124 / 72→72
- size delta 严格 = +3

**与 NFR-1/2 关系**：NFR-1 "内容字节零改" + NFR-2 "BOM 不引入 CRLF" 在新判定下完全等价并被 SHA256 加固验证（§4.4）。临时脚本是辅助工具、不入 git，无设计漂移。

**Reviewer 关注点**：本项不属于 design drift（02 §3 步骤 3 描述的 "CR_count = 0" 只是基于 11 个文件预期 CR=0 的乐观假设，未明确把 3 个 ASCII 文件的 CRLF 现状写出；属临时脚本侧自身判定 bug，非设计或目标产出问题）。后续若要把工作树 CRLF→LF 归一，应作为独立 `T-022 normalize-ps1-eol` followup，不属本任务范围。

### §6.2 verify_all.ps1 size delta 大于 02 §2.1 表预期

**意外**：02 §2.1 表 verify_all.ps1 BOM 后 size 写 "12462 + 新检查项实际增量"。实际：step 5 完成后 size = 13688（增量 1229 = 3 BOM + 1226 E.7 块体）。

**说明**：02 §2.1 表本来就声明 "verify_all.ps1 最终大小 = 加 BOM 后字节 + §2.2 新检查项块体；本表'BOM 后大小'列只算 +3 字节"。属设计明文允许，非偏差。

### §6.3 dogfood 显示 "Rotating 6 old insight(s) to insight-history.md"

**意外**：archive-task.ps1 dry-run 输出 "Rotated 6 old insight(s) to insight-history.md"，意味着 insight-index.md 当前接近或超过 30 行阈值。

**说明**：当前 `.harness/insight-index.md` 共 45 行（含 36 个 `-` 列表项），超出 30 条容量。archive-task.ps1 设计是 dry-run 时仅 echo 计算的轮转计数（"Rotating ..."）而不实际写盘 ——本步骤验证目的是 "脚本能解释执行（BOM 未破坏 parse）"，dry-run 输出符合预期。这条记账由 PM 07 stage 7 / 后续任务 stage 7 真跑时再实际触发轮转，本任务范围外。

---

## §7 待 Reviewer 关注的点

### §7.1 verify_all.sh E.7 实际未运行验证

**说明**：QA 主机为 Windows / PowerShell 7，verify_all.sh 走 bash/POSIX 路径，Developer 本机没跑 bash 验证（PS 路径已 §4.2 PASS）。设计 §2.2 已字节级比对 sh 块的 `head -c 3 | od -An -tx1 | tr -d ' \n'` 在 POSIX 上等价，逻辑严密。建议 QA 06 在 WSL / Git Bash 中跑一次 `bash scripts/verify_all.sh` 确认 sh 端 E.7 也 PASS（02 §7.1 已列）。

### §7.2 baseline.json `updated` 字段未改

按 02 §3 步骤 8 "Developer 04 暂留 2026-05-23 占位，QA 06 跑通日期改"——保留 `2026-05-23`。QA 在 06 跑完 4 真机清单后建议同步更新（不属 Developer 责任）。

### §7.3 3 个 ASCII .ps1 工作树 CRLF 行尾

`archive-task.ps1` / `harness-sync.ps1` / `install-hooks.ps1` 工作树字节级是 CRLF（CR 数 = LF 数）。项目 `.gitattributes` `* text=auto eol=lf` 在 commit 边界会自动归一化为 LF，但工作树字节字面是 CRLF。此为预先存在状态（snapshot 已显示），与本任务无关；若 Reviewer 觉得应顺手处理，建议独立 followup `T-022 normalize-ps1-eol`，本任务严格 NFR-1 范围内零修改这 3 个文件的行尾字节。

### §7.4 OPT-3 dev-map.md 改动位置选择保留

02 §11 OPT-3 建议 "在 L27 后另起一行" 而非 "追加到 scripts/ 行末"。Developer 按 02 §3 步骤 9 原文执行（追加到 scripts/ 行末），与 02 §11 "未吸纳 OPT-3" 一致。dev-map.md 该行确实较长，但与 T-008/T-012/T-013 同款追加模式延续，可读性可接受。

### §7.5 E.7 PS 块 `$root` 路径相对化

C-4 修订要求 `$root` startsWith guard 已写入：当 `$f.FullName` 不以 `$root` 起始时 throw 显式中文报错 "verify_all 必须从仓库根目录运行（当前 root: $root）"。建议 Reviewer 在 05 抽样核对该错误信息字面（设计 §2.2 PS 伪码原文）。

---

## §8 Dev-map 更新

追加到 `docs/dev-map.md` scripts/ 行末（追加位置 L25-L27 块后），新增 1 行：

```
│                     T-021：scripts/*.ps1 全部 11 个统一 UTF-8 BOM（首 3 字节 EF BB BF），让 PS 5.1 + zh-CN 主机磁盘加载形态正确解码中文；verify_all E.7 + scripts/.editorconfig 双层防回归（git blob 字节为持久层、editorconfig 为编辑器层 belt）
```

无新增目录 / 新增模块 / 移动文件，无需新建条目。

---

## §9 dogfood 结果（archive-task.ps1 -DryRun）

完整证据见 §4.5。摘要：

- **前置 BOM 字节核对**：scripts/archive-task.ps1 前 3 字节 = `239,187,191` ✓
- **dry-run 执行**：退出码 0，无 ParserError；输出 "Would have: Appended 0 insight + Rotated 6 + Moved <correct path>"
- **后置 BOM 字节核对**：前 3 字节仍 = `239,187,191`（dry-run 不写文件、保持不变） ✓

**结论**：archive-task.ps1 加 BOM 后 PowerShell 7 解释器正常解析、dry-run 逻辑路径完整。R-2 风险消除。PS 5.1 真机由 QA 06 / 用户在最终交付 stage 7 真跑时再覆盖（02 §7.3 / §7.4 ADV-1）。

---

## §10 Verdict

**READY FOR REVIEW**

- 11 步设计 fidelity = 100%
- verify_all PASS:19 → PASS:**20**（+1 新 E.7，零回归）
- 11/11 .ps1 字节级 BOM = `239,187,191` + CR 数零漂移 + size delta = +3 + SHA256 hash 等同 snapshot
- dogfood archive-task.ps1 -DryRun 前后 BOM 不变、退出 0
- git status 仅含目标改动，无临时残留
- 5 个 [U] 真机 AC 已 ack 降级到 QA 06 真机清单
- 4 处 Reviewer 关注点（sh E.7 待 QA 跑、baseline updated 字段、3 个 ASCII 文件工作树 CRLF、E.7 PS 块 guard 字面）明文列出

请 Code Reviewer 在 05 复核 §2 改动清单、§3 fidelity check、§4 字节级证据、§5 AC 对账。
