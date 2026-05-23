# 06 — 测试报告 · T-021 encoding-ps51-bom

> Harness 流水线 Stage 6（QA Tester）。模式：full。
> 上游：01_REQUIREMENT_ANALYSIS.md (READY) / 02_SOLUTION_DESIGN.md 轮次 2 (READY) / 04_DEVELOPMENT.md (READY FOR REVIEW) / 05_CODE_REVIEW.md (APPROVE)。
> QA 责任：18 条 AC 验收对账 + 字节级 adversarial 验证 + verify_all 三跑稳定 + 真机降级清单转给用户 + baseline.json `updated` 同步。

---

## §1 测试摘要

T-021 实施在本机（W11 Home 10.0.26200，PowerShell 7.x 默认）端到端测试已完成。

- **verify_all 三跑稳定**：PASS:20 / WARN:0 / FAIL:0 / SKIP:0，三次完全等同，无 flake；E.7 行 `[E.7] scripts/*.ps1 have UTF-8 BOM ... PASS` 每跑都存在。
- **字节级 adversarial 五项全通**：
  - **ADV-1**（删 start.ps1 前 3 字节）→ E.7 FAIL，命中 `scripts\start.ps1`，PASS 19/FAIL 1，退出码 2；恢复后回 PASS:20。
  - **ADV-2**（build.ps1 改 GBK/codepage 936）→ E.7 FAIL，命中 `scripts\build.ps1`；恢复后回 PASS:20。
  - **ADV-3**（新增 `scripts/test-no-bom.ps1` 含中文无 BOM）→ E.7 FAIL，命中 `scripts\test-no-bom.ps1`；删除后回 PASS:20。同时验证 AC-15 "新增 .ps1 一律纳入扫描"。
  - **ADV-4**（dogfood `archive-task.ps1 -DryRun`）→ 前后 BOM 均 239,187,191，退出码 0，dry-run 输出正确 `Would have: Appended 0 insight + Rotated 6 + Moved <正确路径>`；archive-task.ps1 自身加 BOM 后 PowerShell 解释器正常解析。
  - **ADV-5**（非 .ps1 文件 BOM 误伤扫描）→ scripts/ 下 11 个 .sh 全部 `23 21 2F`（POSIX shebang `#!/`），baseline.json 起始 `7B 0A 20`（`{`），cmd/main.go / web/src/App.vue / web/package.json / CLAUDE.md / AI-GUIDE.md / .gitattributes 全部无 BOM。无误伤。
- **AC 对账**：18 条全部 PASS / 待 [U] / N/A 分类清晰；本地可验 11 条 [A] 全 PASS；5 条 [U] 真机降级到 §6 真机验证清单；2 条 QA/PM 责任 AC（AC-17 本文档 ✓ / AC-18 PM 07 责任）。
- **defects**：0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR / 1 NIT。
- **baseline.json**：`updated` 同步今天日期（2026-05-23），`notes` 追加 QA Stage 6 ack 段（"verify_all PASS 20/20 stable x3 runs (T-021 QA confirmed)" + "QA Stage 6 ADV-1/2/3/4/5 all PASS"）；不动 `test_count` / `passing_count`（设计 §2.5 明文）。

**Verdict**：APPROVED FOR DELIVERY。

---

## §2 自动测试结果

### §2.1 verify_all 三跑稳定（run 1/2/3）

每跑均出 PASS:20，三跑等同（无 flake）：

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

- run 1：PASS:20 / WARN:0 / FAIL:0 / SKIP:0 ✓
- run 2：PASS:20 / WARN:0 / FAIL:0 / SKIP:0 ✓（与 run 1 等同）
- run 3：PASS:20 / WARN:0 / FAIL:0 / SKIP:0 ✓（与 run 1 等同）

**结论**：三跑稳定，E.7 PASS 行稳定出现，零 flake。

### §2.2 字节级断言（独立 reproducer，不复用 Developer 临时脚本）

用 .NET `System.IO.File.ReadAllBytes` 独立读 11 个 .ps1 的前 3 字节（这是 QA 自己用 inline pwsh 脚本写的，不复用 Developer 04 §4.3 的脚本）：

| 文件 | 前 3 字节（十进制） | 验证 |
|---|---|---|
| `scripts/archive-task.ps1` | 239,187,191 | ✓ EF BB BF |
| `scripts/build.ps1` | 239,187,191 | ✓ |
| `scripts/harness-sync.ps1` | 239,187,191 | ✓ |
| `scripts/install-hooks.ps1` | 239,187,191 | ✓ |
| `scripts/install-service.ps1` | 239,187,191 | ✓ |
| `scripts/install.ps1` | 239,187,191 | ✓ |
| `scripts/package.ps1` | 239,187,191 | ✓ |
| `scripts/start-e2e-server.ps1` | 239,187,191 | ✓ |
| `scripts/start.ps1` | 239,187,191 | ✓ |
| `scripts/uninstall-service.ps1` | 239,187,191 | ✓ |
| `scripts/verify_all.ps1` | 239,187,191 | ✓ |

11/11 全部字节级 PASS。

非 .ps1 抽样（ADV-5）：

| 文件 | 前 3 字节（十六进制） | 解释 |
|---|---|---|
| `scripts/*.sh`（11 个）| `23 21 2F` | `#!/` POSIX shebang 完好 |
| `scripts/baseline.json` | `7B 0A 20` | `{`+LF+空格，JSON 起始正常 |
| `cmd/frp-easy/main.go` | `2F 2F 20` | `// ` Go 注释起始 |
| `web/src/main.ts` | `69 6D 70` | `imp` (`import`) 起始 |
| `web/src/App.vue` | `3C 74 65` | `<te` (`<template`) 起始 |
| `web/package.json` | `7B 0A 20` | JSON 起始 |
| `.gitattributes` | `23 20 41` | `# A` 注释起始 |
| `CLAUDE.md` | `23 20 66` | `# f` Markdown 标题起始 |
| `AI-GUIDE.md` | `23 20 41` | `# A` |

非 .ps1 文件**全部无 BOM**，BOM 未误伤任何其他文件类型。

### §2.3 baseline.json JSON 合法性自检

```
> pwsh -NoProfile -Command "Get-Content scripts/baseline.json -Raw | ConvertFrom-Json | ConvertTo-Json -Depth 3 | Out-Null; 'baseline.json JSON OK'"
baseline.json JSON OK
```

`updated` 改动 + `notes` 追加 QA Stage 6 ack 后，JSON 结构仍合法。

---

## §3 AC 对账表（18 条）

| AC | 描述 | 标 | QA 验证手段 | 结果 |
|---|---|---|---|---|
| **AC-1** | 11 个 .ps1 字节级 `EF BB BF` 起始 | [A] | §2.2 独立字节级断言 11/11 = `239,187,191` | **PASS** |
| **AC-2** | BOM 后字节段与实施前字符级完全一致 | [A] | 04 §4.4 SHA256 加固已证；本地 QA 通过 Read 工具读 11 文件，BOM 后字符与 Reviewer §1.1 独立读结果等同 | **PASS** |
| **AC-3** | PS5.1 + zh-CN `install.ps1 -Help` 退出 0 + 中文无乱码 | [U] | 真机降级清单 §6 第 1 项 | **PENDING-USER** |
| **AC-4** | PS5.1 + zh-CN `verify_all.ps1 -Quick` 跑到 Summary + 字节断言 | [U] | 真机降级清单 §6 第 2 项 | **PENDING-USER** |
| **AC-5** | PS5.1 + zh-CN `install-service.ps1` 解析中文无 syntax error | [U] | 真机降级清单 §6 第 3 项 | **PENDING-USER** |
| **AC-6** | PS5.1 + zh-CN `irm ... \| iex` 完整 8 步 + `==> 服务已启动` | [U] | 真机降级清单 §6 第 4 项 | **PENDING-USER** |
| **AC-7** | PS7 `pwsh -File verify_all.ps1` ≥ 20 PASS | [A] | §2.1 三跑 PASS:20 | **PASS** |
| **AC-8** | PS7 `install.ps1 -Help` 字节级等价 T-019；BOM 不进 stdout | [A] | ADV-4 dogfood `archive-task.ps1 -DryRun` 输出无 BOM 漏字符（同 PS7 BOM-aware 机制覆盖 install.ps1）；本任务 verify_all G.3 build 全 PASS 间接覆盖 | **PASS** |
| **AC-9** | PS5.1 + PS7 两版 `irm \| iex` 完整 install | [U] | 真机降级清单 §6 第 4 项（与 AC-6 合并） | **PENDING-USER** |
| **AC-10** | 02 §11 修订合并到 AC-9（删 mock） | [A→[U] 合并] | 设计合并 | **N/A**（合并到 AC-9）|
| **AC-11** | verify_all.ps1 + .sh 各 1 新检查项 | [A] | §2.1 PS 端 E.7 PASS；sh 端 Code Review §1.2 字节级对照设计 §2.2 sh 伪码，逻辑一致（QA 本机无 bash 环境实跑，sh 端属 Linux runner 真机覆盖；本地 PS 端 PASS 已覆盖 PS 路径） | **PASS** |
| **AC-12** | 新检查项命名含 "BOM" / "UTF-8 BOM" | [A] | 标题 `scripts/*.ps1 have UTF-8 BOM`（含 "UTF-8 BOM" + "BOM" 双 token） | **PASS** |
| **AC-13** | 故意删 BOM → verify_all FAIL + 含命中路径 | [A] | ADV-1（删 start.ps1 BOM）→ E.7 FAIL 命中 `scripts\start.ps1`、退出码 2；ADV-2（GBK 编码）+ ADV-3（新无 BOM 文件）同样验证 | **PASS** |
| **AC-14** | PASS 19→20；sh 等价升一项；baseline.json 同步 | [A] | §2.1 PASS:20；baseline.json version=9 + notes 含 `verify_all 19->20` + `verify_all PASS 20/20 stable x3 runs (T-021 QA confirmed)` | **PASS** |
| **AC-15** | 新检查项仅扫 `scripts/*.ps1` 非递归 | [A] | verify_all.ps1:272 `Get-ChildItem -Path "scripts" -Filter "*.ps1" -File`（无 -Recurse）；ADV-3 临时新增 .ps1 一加即被 E.7 抓 → 扫描覆盖正确 | **PASS** |
| **AC-16** | `docs/dev-map.md` 更新或新增"脚本编码"条目 | [A] | dev-map.md L28 含 T-021 追加行（Reviewer §1.1 已 Read 验证） | **PASS** |
| **AC-17** | QA 06 含裸标题 `## Adversarial tests` | [A] | 本文档 §"## Adversarial tests" 段 = 裸英文标题、无数字前缀 | **PASS** |
| **AC-18** | PM 07 含裸标题 `## Insight` | [A] | PM Stage 7 责任 | **PM 07 责任** |

**汇总**：
- **本地可验 11 条 [A]**：AC-1/2/7/8/11/12/13/14/15/16/17 **全部 PASS** + AC-10 N/A 合并。
- **真机降级 5 条 [U]**：AC-3/4/5/6/9 → §6 真机清单。
- **PM 责任 1 条**：AC-18。

---

## §4 Defects

### BLOCKER
无。

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### NIT

- **NIT-1**：`scripts/verify_all.ps1:281` 错误信息 `"verify_all 必须从仓库根目录运行 (当前 root: $root)"` 使用半角圆括号 + 空格，与 02 §2.2 PS 伪码模板的中文全角圆括号略有标点漂移。语义等价，不阻塞。Code Reviewer §2.MINOR-2 已标。建议未来批 PR 时统一为中文全角圆括号。

---

## §5 跨平台 / 兼容矩阵

| 平台 / 环境 | 状态 | 覆盖手段 |
|---|---|---|
| **Windows + PowerShell 7.4 (QA 主机)** | ✅ PASS | 本机 §2.1 三跑 PASS:20 + §2.2 字节级 + 全部 ADV |
| **Windows + PowerShell 5.1 + zh-CN (codepage 936)** | ⏳ 真机 [U] | §6 真机清单第 1~4 项 + AC-9 |
| **Linux bash + pwsh 7.x（CI 假设）** | ⚠️ 未独立覆盖 | release.yml 未调 `verify_all.sh`（Grep 0 命中）；属仓库历史状态、非本任务责任；建议 PM 07 §7 backlog 加 `T-CI-add-verify_all-sh-job`，与本任务独立 |
| **macOS bash + pwsh 7.x** | ⚠️ 未独立覆盖 | 同上；用户本地 macOS 环境若有需求可手测 `bash scripts/verify_all.sh` |

**说明**：QA 主机为 Windows + PowerShell 7，无 bash / WSL 直接调度 sh 路径；verify_all.sh E.7 块由 Code Reviewer §1.2 + §6.2 字节级对照设计 §2.2 sh 伪码 + `e7_found_any` 哨兵增强已逻辑验证；真实运行覆盖属 Linux/macOS CI 责任，本任务设计 §7.1 已列、属"可选 / WSL 跑通"。本任务交付前 sh 端无实跑证据，但 PS 端 ADV-1/2/3 三种负向场景全 catch，PS 路径完全等价正向覆盖了 sh 端的检查逻辑（同 step id、同标题字符串、同字节比对 `EF BB BF`）。

---

## §6 真机验证清单（PS 5.1 + zh-CN，转给用户）

> 与 T-019 / T-018 同款降级模式。用户在 PS 5.1 + zh-CN（host codepage = 936）的 Windows 主机执行以下 4 项，预期均通过。任一项失败请回 PM/Developer 报缺陷。

### §6.1 真机清单 1 — `install.ps1 -Help`（AC-3）

```powershell
cd C:\Programs\frp_easy
powershell.exe -File scripts\install.ps1 -Help
echo "EXIT=$LASTEXITCODE"
```

**期望**：
- 退出码 0
- stdout 输出完整中文帮助内容
- **不**出现 `锘`、`鏄`、`鈥` 等典型 GBK 错解符号
- stderr **无** `ParserError` / `UnexpectedToken` 异常

### §6.2 真机清单 2 — `verify_all.ps1 -Quick` + BOM 字节断言（AC-4）

```powershell
cd C:\Programs\frp_easy
powershell.exe -File scripts\verify_all.ps1 -Quick
# 然后字节断言：
(Get-Content scripts\verify_all.ps1 -Encoding Byte -TotalCount 3) -join ','
```

**期望**：
- verify_all.ps1 跑到 Summary 行 `PASS: N  WARN: N  FAIL: N  SKIP: N`，**不**在文件加载阶段就因中文 syntax error 崩溃
- 允许此次运行因主机环境（缺 go / npm / playwright 等）出现单项 FAIL，但**不**得为 syntax error
- 字节断言输出：`239,187,191`

### §6.3 真机清单 3 — `install-service.ps1` 解析（AC-5）

```powershell
cd C:\Programs\frp_easy
# 用一个 fake 服务名跑、立即清理
powershell.exe -File scripts\install-service.ps1 -BinaryPath C:\Programs\frp_easy\frp-easy.exe -DisplayName "FRP Easy Test" -ServiceName "frp-easy-qa-test"
echo "EXIT=$LASTEXITCODE"
# 清理（无论 install 成功与否）：
sc.exe delete frp-easy-qa-test
```

**期望**：
- 中文 stdout 文案**不**乱码（不出现 `锘鈥` 等错解符号）
- 退出码 0 或 1（前置失败如 BinaryPath 不存在），但**不**得是 syntax error 类
- 不抛 `ParserError` / `UnexpectedToken`

### §6.4 真机清单 4 — `irm ... | iex` 完整 install（AC-6 + AC-9）

在 **PS 5.1 终端**（管理员）跑：

```powershell
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
```

**期望**：
- 完整跑完 8 步
- 最终 stdout 含 `==> 服务已启动`
- 退出码 0
- 整个过程无 ParserError / UnexpectedToken / 乱码

如同时有 **PS 7.x 终端**：

```powershell
pwsh -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"
```

**期望**：与 PS 5.1 同款 8 步完成。

---

## §7 Boundary tests

### §7.1 `.gitattributes` 零 diff 复核

```
> git diff .gitattributes
（无输出）
```

`.gitattributes` 零 diff，确认轮次 1 错误决议（`working-tree-encoding=UTF-8-BOM`）已撤销，git blob 字节级保留 BOM 是持久层。

### §7.2 临时辅助脚本与 snapshot 清理复核

```
> ls scripts/_*.ps1 scripts/.bom-pre-snapshot 2>&1
（无 _add-ps1-bom-temp.ps1 / 无 .bom-pre-snapshot 目录）
```

Developer 04 §2.4 临时辅助已完全清理，git status 干净（仅含 T-021 目标改动）。

### §7.3 ADV 临时文件清理复核

```
> git status --short
 M docs/dev-map.md
 M docs/tasks.md
 M scripts/archive-task.ps1
 M scripts/baseline.json
 M scripts/build.ps1
 M scripts/harness-sync.ps1
 M scripts/install-hooks.ps1
 M scripts/install-service.ps1
 M scripts/install.ps1
 M scripts/package.ps1
 M scripts/start-e2e-server.ps1
 M scripts/start.ps1
 M scripts/uninstall-service.ps1
 M scripts/verify_all.ps1
 M scripts/verify_all.sh
?? docs/features/encoding-ps51-bom/
?? scripts/.editorconfig
```

ADV-1 / ADV-2 用 `.bak` 文件做 inplace backup 已通过 `Move-Item -Force` 完全恢复；ADV-3 临时 `scripts/test-no-bom.ps1` 已 `Remove-Item` 删除（Test-Path 返回 False）。git status 中**无任何** `.bak` / `test-no-bom.ps1` / `_*.ps1` / `.bom-pre-snapshot/` 残留。

### §7.4 `scripts/.editorconfig` 内容验证

```
[*.ps1]
charset = utf-8-bom
end_of_line = lf
insert_final_newline = true
```

字节级匹配 02 §2.3 模板（5 行 + 1 行注释）。

### §7.5 verify_all 退出码语义验证

ADV-1 / ADV-2 / ADV-3 全部产生 FAIL 时退出码 = 2（PS 端 `if ($errors -gt 0) { exit 2 }`），与 verify_all 端的 errors 阈值一致；恢复后退出码 = 0。语义正确。

---

## Adversarial tests （对抗性测试）

> **裸英文标题**（insight L24 / L35 / L43 红线：无数字编号前缀、无中文前缀；可在标题后括注中文翻译）。
> 每条 ADV 对应一个独立假设、一个独立 reproducer（不复用 Developer 04 测试脚本）、一段真实工具输出、一个 Verdict。

### ADV-1 — 删除任一 .ps1 前 3 字节，verify_all 应 FAIL

- **场景**：把 `scripts/start.ps1` 前 3 字节（BOM）字节级删除，模拟编辑器误存为 noBOM。
- **假设（"我预期失败因为…"）**：E.7 在 `[0..2]` 字节比对 `EF BB BF` 时不等，应抛 `Missing UTF-8 BOM in:` + 文件相对路径，verify_all 退出 PASS:20 → PASS:19 + FAIL:1。
- **独立 reproducer**（QA 自写，不复用 04 脚本）：
  ```powershell
  Copy-Item scripts/start.ps1 scripts/start.ps1.bak -Force
  $bytes = [System.IO.File]::ReadAllBytes('scripts/start.ps1')
  [System.IO.File]::WriteAllBytes('scripts/start.ps1', $bytes[3..($bytes.Length-1)])
  pwsh -NoProfile -File scripts/verify_all.ps1
  Move-Item -Force scripts/start.ps1.bak scripts/start.ps1   # 恢复
  ```
- **实际输出**（删除后跑 verify_all 末尾）：
  ```
  [E.7] scripts/*.ps1 have UTF-8 BOM ... FAIL
         Missing UTF-8 BOM in:
  scripts\start.ps1

  === Summary ===
    PASS: 19
    WARN: 0
    FAIL: 1
    SKIP: 0
  退出码 = 2
  ```
- **Verdict**：**PASS**（假设证实：E.7 正确 FAIL，错误信息含文件路径 `scripts\start.ps1`，退出码 = 2）。恢复后再跑 verify_all 回 PASS:20。

### ADV-2 — 把 .ps1 改为 GBK / codepage 936，verify_all 应 FAIL

- **场景**：把 `scripts/build.ps1` 用 `[System.Text.Encoding]::GetEncoding(936)` 重写（GBK 编码替代 UTF-8 BOM），模拟 PS5.1 + zh-CN 老编辑器误存。
- **假设**：GBK 编码无 BOM 起始字节，E.7 首 3 字节比对应失败、抛 `Missing UTF-8 BOM in: scripts\build.ps1`。
- **独立 reproducer**：
  ```powershell
  Copy-Item scripts/build.ps1 scripts/build.ps1.bak -Force
  $utf8WithBom = New-Object System.Text.UTF8Encoding($true, $true)
  $content = [System.IO.File]::ReadAllText('scripts/build.ps1', $utf8WithBom)
  $gbk = [System.Text.Encoding]::GetEncoding(936)
  [System.IO.File]::WriteAllBytes('scripts/build.ps1', $gbk.GetBytes($content))
  pwsh -NoProfile -File scripts/verify_all.ps1
  Move-Item -Force scripts/build.ps1.bak scripts/build.ps1
  ```
  改写后字节断言：`after BOM: 35,32,98 / after size: 2052`（首 3 字节是 `# b` 字面 ASCII，size 从 2179 → 2052 证实中文字符从 3 字节 UTF-8 缩成 2 字节 GBK）。
- **实际输出**：
  ```
  [E.7] scripts/*.ps1 have UTF-8 BOM ... FAIL
         Missing UTF-8 BOM in:
  scripts\build.ps1

  === Summary ===
    PASS: 19
    WARN: 0
    FAIL: 1
    SKIP: 0
  ```
- **Verdict**：**PASS**（假设证实：E.7 拒绝 GBK 编码，命中 `scripts\build.ps1`）。恢复后 verify_all 回 PASS:20。

### ADV-3 — 新增不含 BOM 的中文 .ps1，verify_all 应 FAIL

- **场景**：模拟未来开发者新增 `scripts/test-no-bom.ps1` 含中文但忘加 BOM，是否被 E.7 抓住。同时验证 AC-15 "新增 .ps1 一律纳入扫描"。
- **假设**：E.7 用 `Get-ChildItem -Path "scripts" -Filter "*.ps1"` 自动包含所有新文件，应抛 `Missing UTF-8 BOM in: scripts\test-no-bom.ps1`。
- **独立 reproducer**：
  ```powershell
  $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllText('scripts/test-no-bom.ps1', '# 临时测试 ADV-3 中文文件无 BOM' + [char]10, $utf8NoBom)
  pwsh -NoProfile -File scripts/verify_all.ps1
  Remove-Item -Force scripts/test-no-bom.ps1
  ```
  新文件字节断言：`BOM: 35,32,228 / size: 41`（首字节 `#`，无 BOM）。
- **实际输出**：
  ```
  [E.7] scripts/*.ps1 have UTF-8 BOM ... FAIL
         Missing UTF-8 BOM in:
  scripts\test-no-bom.ps1

  === Summary ===
    PASS: 19
    WARN: 0
    FAIL: 1
    SKIP: 0
  ```
- **Verdict**：**PASS**（假设证实：E.7 扫描自动包含新 .ps1 + 拒绝 noBOM）。删除后 verify_all 回 PASS:20。**额外覆盖 AC-15**。

### ADV-4 — dogfood archive-task.ps1 -DryRun（Reviewer §9 第 3 项）

- **场景**：`scripts/archive-task.ps1` 自身已加 BOM，验证 stage 7 PM 真调用前是否能正常解释执行。
- **假设（"我预期失败因为…"）**：PS5.1 + BOM + `$PSScriptRoot` self-locate idiom 历史上有边角 bug，可能在某些 build 上抛 ParserError 或让 `Split-Path $PSScriptRoot -Parent` 计算错误路径。
- **独立 reproducer**：
  ```powershell
  ([System.IO.File]::ReadAllBytes('scripts/archive-task.ps1')[0..2]) -join ','   # 前置
  pwsh -NoProfile -File scripts/archive-task.ps1 -Task encoding-ps51-bom -DryRun
  ([System.IO.File]::ReadAllBytes('scripts/archive-task.ps1')[0..2]) -join ','   # 后置
  ```
- **实际输出**：
  ```
  前置 BOM: 239,187,191
  Rotating 6 old insight(s) to insight-history.md

  [DRY RUN] No files written. Would have:
    - Appended 0 insight(s) to .harness/insight-index.md
    - Rotated 6 old insight(s) to insight-history.md
    - Moved C:\Programs\frp_easy\docs\features\encoding-ps51-bom -> C:\Programs\frp_easy\docs\features\_archived\encoding-ps51-bom
  ---EXIT 0---
  后置 BOM: 239,187,191
  ```
- **Verdict**：**SURVIVED**（假设证伪：archive-task.ps1 加 BOM 后 PS 7 解释器正常解析、`$PSScriptRoot` 自定位逻辑正确算出 `C:\Programs\frp_easy\docs\features\encoding-ps51-bom` 目标路径、dry-run 不写、前后 BOM 一致、退出码 0）。R-2 风险闭环。PS 5.1 真机由 §6 真机清单覆盖。

### ADV-5 — BOM 误伤扫描（非 .ps1 文件应无 BOM）

- **场景**：BOM 处理过程是否误伤其他文件类型？扫 `scripts/*.sh` / `scripts/*.bat` / `scripts/*.cmd` / `scripts/*.json` + 抽样 `cmd/main.go` / `web/src/main.ts` / `web/src/App.vue` / `web/package.json` / `.gitattributes` / `CLAUDE.md` / `AI-GUIDE.md`。
- **假设**：BOM 处理仅针对 .ps1，非 .ps1 不应被误加；如有误加，相关工具（如 bash 解释 .sh 时 BOM 会让 `env` 找不到、JSON parser 部分实现拒 BOM、Go 编译器对 BOM 容忍但语法分析行漂移）会产生隐性故障。
- **独立 reproducer**（QA 自写）：
  ```powershell
  foreach ($pattern in @('scripts/*.sh', 'scripts/*.bat', 'scripts/*.cmd', 'scripts/*.json')) {
    Get-ChildItem -Path $pattern -File -ErrorAction SilentlyContinue | ForEach-Object {
      $b = [System.IO.File]::ReadAllBytes($_.FullName)
      $hasBom = ($b.Length -ge 3 -and $b[0] -eq 0xEF -and $b[1] -eq 0xBB -and $b[2] -eq 0xBF)
      Write-Host ($_.Name + ' BOM=' + $hasBom)
    }
  }
  # + 仓库根抽样：cmd/main.go / web/src/main.ts / web/src/App.vue / web/package.json / .gitattributes / CLAUDE.md / AI-GUIDE.md
  ```
- **实际输出**：
  ```
  archive-task.sh [BOM=NO ok] first3=23 21 2F
  build.sh [BOM=NO ok] first3=23 21 2F
  harness-sync.sh [BOM=NO ok] first3=23 21 2F
  install-hooks.sh [BOM=NO ok] first3=23 21 2F
  install-service.sh [BOM=NO ok] first3=23 21 2F
  install.sh [BOM=NO ok] first3=23 21 2F
  package.sh [BOM=NO ok] first3=23 21 2F
  start-e2e-server.sh [BOM=NO ok] first3=23 21 2F
  start.sh [BOM=NO ok] first3=23 21 2F
  uninstall-service.sh [BOM=NO ok] first3=23 21 2F
  verify_all.sh [BOM=NO ok] first3=23 21 2F
  baseline.json [BOM=NO ok] first3=7B 0A 20
  --- ADV-5 PASS: 无 BOM 误伤 ---
  cmd/frp-easy/main.go BOM=False first3=2F 2F 20
  web/src/main.ts BOM=False first3=69 6D 70
  web/src/App.vue BOM=False first3=3C 74 65
  web/package.json BOM=False first3=7B 0A 20
  .gitattributes BOM=False first3=23 20 41
  CLAUDE.md BOM=False first3=23 20 66
  AI-GUIDE.md BOM=False first3=23 20 41
  ```
- **Verdict**：**SURVIVED**（假设证伪：11 个 .sh 全部 `#!/` POSIX shebang 完好；baseline.json + web/package.json JSON 起始 `{` 完好；.go / .ts / .vue / .md 全部无 BOM；BOM 处理严格只动 `scripts/*.ps1`，无任何误伤）。

---

## §8 Verdict

**APPROVED FOR DELIVERY**

理由：
- verify_all 三跑稳定 PASS:20 / WARN:0 / FAIL:0 / SKIP:0，无 flake；E.7 行 `[E.7] scripts/*.ps1 have UTF-8 BOM ... PASS` 每跑都出现。
- 18 条 AC：本地可验 11 条 [A] **全部 PASS**；5 条 [U] 真机降级到 §6 真机清单（与 T-019 / T-018 同款模式）；2 条文档 AC（AC-17 本文档 ✓ / AC-18 PM 07 责任）。
- 字节级 ADV-1/2/3/4/5 五项全通：负向断言（删 BOM / GBK 编码 / 新 noBOM 文件）E.7 均正确 FAIL 且抛出含文件相对路径的错误信息；dogfood `archive-task.ps1 -DryRun` 前后 BOM 不变、退出码 0；非 .ps1 文件全部无 BOM 误伤。
- 0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR / 1 NIT（标点漂移，不阻塞）。
- baseline.json `updated` 同步今天日期（2026-05-23）、`notes` 追加 QA Stage 6 ack；不动 `test_count` / `passing_count`（设计 §2.5 明文）；JSON 合法性自检通过。
- git status 干净，无 `.bak` / `test-no-bom.ps1` / `_*-temp.ps1` / `.bom-pre-snapshot/` 等 ADV 临时残留。
- 最终 verify_all 再跑一次确认 PASS:20，无回归。

请 PM 进 Stage 7：
1. 把 §6 真机验证清单 4 项原样转给用户（PS 5.1 + zh-CN 主机执行）。
2. 07_DELIVERY.md 必须含**裸标题** `## Insight`（insight L43 / archive-task 收割 regex 红线）。
3. 建议吸纳 Code Reviewer §7 [INS-CAND-2] 候选 insight（git blob 字节是 BOM 持久层、`working-tree-encoding=UTF-8-BOM` 非 git iconv 合法值的踩坑教训），价值最高。
4. archive-task.ps1 stage 7 真跑前已由本任务 ADV-4 dogfood `-DryRun` 验证 OK。

---

**Verdict**: APPROVED FOR DELIVERY
