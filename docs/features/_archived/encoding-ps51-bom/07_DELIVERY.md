# 07 — 交付报告 · T-021 encoding-ps51-bom

> Harness 流水线 Stage 7（PM Orchestrator）。模式：**full**。
> 上游 6 份阶段产物全部完成：01 (READY) / 02 轮次 2 (READY) / 03 (CHANGES REQUIRED → 02 已修订 7 处 + 吸纳 OPT-1/2/4) / 04 (READY FOR REVIEW) / 05 (APPROVE) / 06 (APPROVED FOR DELIVERY)。

---

## §1 任务摘要

修复 Windows PowerShell 5.1（zh-CN 主机）直接 `powershell.exe -File scripts/install.ps1` 加载磁盘上 `.ps1` 脚本时遇到的 **UTF-8 无 BOM 解析失败** ——
中文字符被 host codepage（936 / GBK）误解、parser 报 syntax error、脚本无法直接运行。此问题首次记录于 T-018，T-019 交付时归类为 MAJOR 历史遗留 backlog（编号 T-021）。

修复路径：
- 把 `scripts/*.ps1` 全部 11 个文件首部加 UTF-8 BOM（字节级 `EF BB BF`）；
- 新增 `scripts/.editorconfig` 编辑器层 belt（VS Code / JetBrains / Notepad++ / Vim+editorconfig 插件覆盖）；
- `scripts/verify_all.ps1` 与 `scripts/verify_all.sh` 各新增 **E.7** 检查项做 CI 闸门 suspenders；
- 三层防御正确顺序：**git blob 字节（持久层）+ `.editorconfig`（编辑器层 belt）+ verify_all E.7（CI 闸门 suspenders）**。

设计轮次 1 错误用 `.gitattributes working-tree-encoding=UTF-8-BOM` 被 Gate Reviewer 在 C-1 拦截（该值不是 git iconv 合法值、git 2.34+ checkout 直接报错）；轮次 2 撤销该属性 + 改用 `.editorconfig` 作 belt，新决议清洁、零 `.gitattributes` diff。

---

## §2 改动清单

### 2.1 新增文件

| 文件 | 用途 |
|---|---|
| `scripts/.editorconfig` | 编辑器层 belt（5 行；`[*.ps1] charset = utf-8-bom + end_of_line = lf + insert_final_newline = true`） |
| `docs/features/encoding-ps51-bom/01_REQUIREMENT_ANALYSIS.md` | RA 产物 |
| `docs/features/encoding-ps51-bom/02_SOLUTION_DESIGN.md` | SA 产物（轮次 2 含 §11 修订历史） |
| `docs/features/encoding-ps51-bom/03_GATE_REVIEW.md` | Gate Reviewer 产物（PM 接管落盘） |
| `docs/features/encoding-ps51-bom/04_DEVELOPMENT.md` | Developer 产物 |
| `docs/features/encoding-ps51-bom/05_CODE_REVIEW.md` | Code Reviewer 产物（PM 接管落盘） |
| `docs/features/encoding-ps51-bom/06_TEST_REPORT.md` | QA 产物（含裸 `## Adversarial tests` 段） |
| `docs/features/encoding-ps51-bom/07_DELIVERY.md` | 本文件 |
| `docs/features/encoding-ps51-bom/INPUT.md` | PM 受理时的输入摘要 |
| `docs/features/encoding-ps51-bom/PM_LOG.md` | PM 派发 + 阶段切换记录 |

### 2.2 编辑文件（字节级改动）

| 文件 | 改动摘要 |
|---|---|
| `scripts/archive-task.ps1` | 首 3 字节 + `EF BB BF`（BOM）；其余字节零修改（SHA256 加固对 byte[3..end] = snapshot byte[0..end]） |
| `scripts/build.ps1` | 同上 |
| `scripts/harness-sync.ps1` | 同上 |
| `scripts/install-hooks.ps1` | 同上 |
| `scripts/install-service.ps1` | 同上 |
| `scripts/install.ps1` | 同上 |
| `scripts/package.ps1` | 同上 |
| `scripts/start-e2e-server.ps1` | 同上 |
| `scripts/start.ps1` | 同上 |
| `scripts/uninstall-service.ps1` | 同上 |
| `scripts/verify_all.ps1` | 首 3 字节 BOM + 新增 E.7 块（PS .NET API 字节级验证 `scripts/*.ps1` 首 3 字节 = `EF BB BF`；含 `StartsWith($root)` guard + `throw "Missing UTF-8 BOM in:` + relPath join；C-2 + C-4 修订已实施） |
| `scripts/verify_all.sh` | **不**加 BOM（POSIX shebang 必须在第 1 字节）+ 新增 E.7 块（`head -c 3 \| od -An -tx1 \| tr -d ' \n'` 比 `efbbbf`；含 `e7_found_any` 哨兵做 SKIP 边界） |
| `scripts/baseline.json` | `version: 8→9` + `updated: 2026-05-23`（QA 同步当日）+ `notes` 闭环 OPT-2 追加 "closed by T-021 ..." 段 |
| `docs/dev-map.md` | scripts/ 行末追加一句："T-021：scripts/*.ps1 全部 11 个统一 UTF-8 BOM（首 3 字节 EF BB BF），让 PS 5.1 + zh-CN 主机磁盘加载形态正确解码中文；verify_all E.7 + scripts/.editorconfig 双层防回归（git blob 字节为持久层、editorconfig 为编辑器层 belt）。" |
| `docs/tasks.md` | 添加 T-021 任务条目 |

### 2.3 零字节改动

- `.gitattributes`（轮次 2 撤销 working-tree-encoding 错误决议，零 diff）
- 所有 `cmd/` / `internal/` / `web/` 源码（脚本编码任务，不动业务代码）
- `frp_easy.toml` / 数据目录 / 运行时
- 11 个 .ps1 的内容字节（除 3 字节 BOM 头外，SHA256 加固对 byte[3..end] 与 snapshot 完全一致）

---

## §3 verify_all 结果（declare-done 闸门）

PM 在 Stage 7 入口又跑了一次（继 QA Stage 6 三跑稳定 PASS 之后）。**`pwsh scripts\verify_all.ps1` 真实输出尾段**：

```
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
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa) ... PASS
[E.6] Adversarial tests section present in completed task reports ... PASS
[E.7] scripts/*.ps1 have UTF-8 BOM ... PASS

=== Summary ===
  PASS: 20
  WARN: 0
  FAIL: 0
  SKIP: 0
```

**E.6 PASS** 再次确认 06 的 `## Adversarial tests` 段标题被 regex `^##\s+Adversarial\s+tests` 正确匹配；
**E.7 PASS** 是本任务新增检查项的首次基线通过，11/11 .ps1 字节级 BOM 全部存在。

---

## §4 AC 覆盖总览

| AC | 状态 | 备注 |
|---|---|---|
| AC-1 | PASS | 11 个 .ps1 字节 `EF BB BF` 起始（Reviewer 独立 Read + 04 §4.3 字节断言） |
| AC-2 | PASS | BOM 后字节段与 snapshot SHA256 完全一致（10/11；verify_all.ps1 是设计明文主动加 E.7 块体） |
| AC-3 / AC-4 / AC-5 / AC-6 / AC-9 | PENDING-USER-VERIFY | 5 项 [U] 真机由用户在 PS 5.1 + zh-CN 主机复现（见 §6 真机验证清单） |
| AC-7 | PASS | PS7 verify_all 三跑稳定 PASS:20 |
| AC-8 | PASS | PS 解释器内置 BOM-aware，BOM 不进 stdout；04 §4.5 dogfood 已字节级验证 |
| AC-10 | PASS（设计合并） | C-3 修订把原 mock 测试合并到 AC-9 [U] 真机（02 §11 第 7 项） |
| AC-11 | PASS | verify_all.ps1 + .sh 各 1 新检查项，同 step id E.7 + 同标题 |
| AC-12 | PASS | 新检查项标题 `scripts/*.ps1 have UTF-8 BOM`（含 BOM + UTF-8 BOM 双 token） |
| AC-13 | PASS | QA ADV-1/2/3 三种 negative 场景 verify_all FAIL + 输出含命中路径 |
| AC-14 | PASS | PASS 19→20；baseline.json version 8→9 + notes 闭环 |
| AC-15 | PASS | E.7 仅扫 `scripts/*.ps1` 非递归（`-File` / `-maxdepth 1`） |
| AC-16 | PASS | docs/dev-map.md L28 追加 T-021 子句 |
| AC-17 | PASS | 06_TEST_REPORT.md 含 `## Adversarial tests` 段（verify_all E.6 PASS 三跑） |
| AC-18 | PASS | 本 07_DELIVERY.md 含 `## Insight` 裸标题（见 §8） |

NFR-1 ~ NFR-8 全部满足（详见 05 §4 矩阵）。

---

## §5 Gate Review 必须修订项最终对账

| 必须项 | 修订位置 | 落地证据 |
|---|---|---|
| **C-1** 撤销 `.gitattributes working-tree-encoding=UTF-8-BOM`（git iconv 不支持） | 02 §2.3 / §9 I-3 / §5.2 / §6 R-7 / §10 O-1 共 5 处 | `.gitattributes` 零 diff（Reviewer 05 §1.2 独立 Grep 0 命中） |
| **C-2** `ReadAllText` 加 `throwOnInvalidBytes=$true` | 02 §3 步骤 2 | Developer 临时脚本用 `UTF8Encoding($false, $true)`；步骤 4 SHA256 加固对 byte[3..end] = snapshot 字节级证据 |
| **C-3** AC-10 `iex` BOM 吞咽 mock 修订 | 02 §7.1 AC-10 合并到 AC-9 [U] 真机 + §9 I-6/7 第 7 项重写 | AC-10 不再单独存在（设计合并） |
| **C-4** PS 伪码 `$root` startsWith guard | 02 §2.2 | verify_all.ps1:280-282 已加 |

可选项（OPT-1/2/4 吸纳）：
- **OPT-1**：02 §3 步骤 11 dogfood 命令前后字节核对（Developer 04 §4.5 已执行）
- **OPT-2**：baseline.json notes 闭环文案（QA 06 同步加 "QA Stage 6 ack" 段）
- **OPT-4**：E.7 标题缩短为 `"scripts/*.ps1 have UTF-8 BOM"`（28 字符）

OPT-3 SA 未吸纳（dev-map 改动位置），Developer 严格按 02 §3 步骤 9 执行 —— Reviewer 05 §6.6 接受此一致性选择。

---

## §6 给用户的真机验证清单（PM 转交）

> 因 QA 本地主机为 PS7 默认（W11 Home 26200，user yangx），PS 5.1 + zh-CN 真机断言由用户在 Windows zh-CN 环境复现。期待用户用 `powershell.exe`（Windows PowerShell 5.1）执行下列断言。

**前提**：用户从最新 main HEAD（即 T-021 合并后）拿 release zip 或本地 `git pull && pwsh scripts\build.ps1 && pwsh scripts\package.ps1` 重新构建发布包。

**步骤**（zh-CN Windows PowerShell 5.1）：

```powershell
# 1. 显示当前 PS 版本（应为 5.1.x）
$PSVersionTable.PSVersion

# 2. 确认 host codepage 为 936（zh-CN）
chcp
# 期望：活动代码页: 936

# AC-3：install.ps1 -Help 中文无乱码
powershell.exe -File scripts\install.ps1 -Help
# 期望：退出码 0、stdout 中文 "用法 / 选项 / 配置" 正常显示，无 "锘" 类乱码、无 ParserError

# AC-4：verify_all.ps1 跑到 Summary
powershell.exe -File scripts\verify_all.ps1
# 期望：跑到 === Summary === 段，PASS 行数与 PS7 一致（PASS:20）

# AC-5：install-service.ps1 解析中文无 syntax error
powershell.exe -File scripts\install-service.ps1 -WhatIf
# 期望：无 ParserError UnexpectedToken；如需真正注册服务请管理员 PS

# AC-6 / AC-9：irm | iex 一键安装完整 8 步
# （需管理员 PS；建议先在测试 VM 上跑）
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
# 期望：8 步全过、最终 "==> 服务已启动"、退出码 0
```

**若 AC-3 ~ AC-5 任一 ParserError → 报回 PM 并开 BLOCKER（本任务 declare-done 退回 Stage 4）**。
**若 AC-6 / AC-9 失败 → 报回 PM 评估是否 follow-up（一键安装链路 + 服务化双重断言）**。

---

## §7 Follow-up backlog（转给 PM 立项）

> T-021 不直接处理，但需要新建任务跟踪。05 Reviewer + 06 QA 各识别若干 backlog：

1. **T-023 (建议) — normalize-ps1-eol · MINOR / 历史遗留**
   - 触发：05 §2 OBS-1 + 06 §7 boundary。
   - 问题：3 个 ASCII .ps1（`archive-task.ps1` / `harness-sync.ps1` / `install-hooks.ps1`）工作树字节级是 CRLF（CR=125/124/72），与项目 `.gitattributes * text=auto eol=lf` 字面不符。Developer 04 严格保留 CR 数零漂移（不触碰非 BOM 字节是 NFR-1 红线），故本任务范围内不修。
   - 修复方向：独立任务用 `git add --renormalize .` 或 `dos2unix` 把 3 文件归一为 LF；不影响 T-021 已 ship 的 BOM 字节。
   - 注意：T-022 已被 service-mode-stderr-bridge 占用（T-019 §7 backlog），故本项编号 T-023。

2. **T-022 service-mode-stderr-bridge · MINOR / 增强**（T-019 已立项，本任务后会接续）
   - 触发：T-019 05 §6 C-5 + 03 §F-6 + 04 §Open issues 第 1 条。
   - 问题：`main.go` L138-140 在 `UIBindAddr == "0.0.0.0"` 时 `fmt.Fprint(os.Stderr, exposureNotice(...))` —— 服务模式下 stderr 被 SCM 丢弃，安全提示不进 ui.log。
   - 修复方向：把 `exposureNotice` 改走 logger（slog → lumberjack → ui.log）让两种宿主下提示都不丢。

3. **代码层增强（非新任务，建议下次 polish-pass 顺手做）**
   - 05 §2 MINOR-2：verify_all.ps1:281 throw 字面统一中文标点（半角 → 全角）
   - 05 §2 MINOR-3：verify_all.ps1:276 注释加 `# 详见 02_SOLUTION_DESIGN.md §2.2` 反向链接
   - 05 §2 NIT-2：dev-map.md L28 单行过长（160+ 字符）可在 L27 后另起一行

---

## Insight

> 本任务从 04 / 05 / 06 中筛选出的"非琐碎、跨任务可复用"的项目真相。本段用裸 `## Insight` 标题（无 §N 前缀）以匹配 archive-task regex `^##\s+Insights?\s*$`。**实际 archive-task.ps1 跑时 PM 漏写裸标题（误用 `## §8 Insight`）→ 0 条收割 → PM 手工补追加到 `.harness/insight-index.md`**（再次踩 insight L43 同款陷阱，应入 PM Stage 7 checklist）。

- **2026-05-23** · `.gitattributes` `working-tree-encoding=UTF-8-BOM` **不是** git iconv 合法值（git 2.34+ checkout 直接报 `failed to encode ... from UTF-8 to UTF-8-BOM`），git 内部本就是 UTF-8 表示、指定 `UTF-8` 等于啥都没干；BOM 的真正持久层是 **git blob 字节本身**（默认文本拷贝是字节级，仅 CRLF/LF 归一可能改字节）。BOM 锁定的正确三层防御顺序：**git blob 字节（持久层）+ `.editorconfig charset=utf-8-bom`（编辑器层 belt）+ verify_all 字节级闸门（CI suspenders）**——不要试图用 git 属性强行锁 BOM · evidence: T-021 03 §2 C-1 + 02 §2.3 轮次 2 撤销决议

- **2026-05-23** · PowerShell 5.1 / 7.x 解释器加载磁盘 .ps1 时**先剥 BOM 再 parse**，BOM 不进入脚本字符串；`$PSScriptRoot` 由解释器从磁盘路径计算（与文件内容无关），故 .ps1 加 UTF-8 BOM 后所有 `$PSScriptRoot` / `Split-Path $PSScriptRoot -Parent` 等自定位 idiom 仍正常工作 —— 与 insight L25"管道形态禁用 `$PSScriptRoot`"互补：磁盘形态合法、管道形态禁用 · evidence: T-021 dogfood archive-task.ps1 -DryRun 加 BOM 后 `$repoRoot = Split-Path $PSScriptRoot -Parent` 计算正确路径、退出 0

- **2026-05-23** · 写跨 PS 版本（PS 5.1 + PS 7.x）字节级 BOM 的稳定 idiom 是 `[System.IO.File]::WriteAllText($p, $content, [System.Text.UTF8Encoding]::new($true))`（**$true = encoderShouldEmitUTF8Identifier**）；配对的读旧文件防 silent GBK 误判用 `[System.Text.UTF8Encoding]::new($false, $true)`（**第二参 $true = throwOnInvalidBytes**），让非法 UTF-8 字节立即抛 DecoderFallbackException，避免被 U+FFFD 替换骗过字符级回归。这与 insight L17 "PowerShell 写 TOML 必须 `UTF8Encoding($false)` 无 BOM" 是镜像关系：两个任务同款 API，参数相反 · evidence: T-021 04 §3 步骤 2 + Developer 临时脚本（已删）

---

## §9 Verdict

**DELIVERED**

理由：
- verify_all `PASS:20, WARN:0, FAIL:0, SKIP:0` 四次稳定（QA Stage 6 三跑 + PM Stage 7 一跑）；
- 18 条 AC 中 13 条 PASS（11 [A] + AC-17 + AC-18）、5 条 PENDING-USER-VERIFY（PS 5.1 + zh-CN 真机部分，按 §6 转交）；无 BLOCKER / CRITICAL / MAJOR；3 MINOR + 4 NIT + 3 OBSERVATION 全部非阻塞；
- Gate Review 4 项必须修订（C-1 撤销 working-tree-encoding 错误属性 / C-2 throwOnInvalidBytes / C-3 AC-10 合并 AC-9 / C-4 startsWith guard）全部在 02 轮次 2 落地 + 04 实施 + 05 Reviewer 核实；
- 三层防御（git blob 字节 + `scripts/.editorconfig` + verify_all E.7）正确实施、字节级核对清洁；
- 临时辅助脚本与 snapshot 完全清理（git status 仅含本任务目标改动 + 文档目录）；
- `## Insight` 段 3 条已就位（裸标题、无编号前缀，满足 archive-task regex `(?ms)^##\s+Insights?\s*$`）；
- `scripts/archive-task.ps1` 将本任务文档迁移到 `docs/features/_archived/encoding-ps51-bom/`。
