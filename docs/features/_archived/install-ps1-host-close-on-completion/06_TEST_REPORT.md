# 06 — Test Report · T-031 install-ps1-host-close-on-completion

> Harness 流水线 Stage 6（QA Tester）。模式：**full**。
> 上游：01 (Verdict=READY) + 02 (Verdict=READY FOR GATE REVIEW) + 03 (APPROVED W/ Conditions C-1~C-4) + 04 (Verdict=READY FOR CODE REVIEW) + 05 (Verdict=APPROVED · 0 CRITICAL / 0 MAJOR / 1 MINOR maintainability nit)。
> 角色契约：`.harness/agents/qa-tester.md`。

---

## §1 自动化 / 静态 AC 复跑结果

### 1.1 PowerShell verify_all（Win 主路径）

命令：`pwsh -NoProfile -File scripts/verify_all.ps1 -Quick`

```
[E.6] Adversarial tests section present in completed task reports ... PASS
[E.7a] BOM-required scripts/*.ps1 have UTF-8 BOM ... PASS
[E.7b] iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM ... PASS
[E.7c] All scripts/*.ps1 classified in E.7a or E.7b (anti-drift) ... PASS
[E.8] install.ps1 / install-service.ps1 forbid interactive blockers (FR-3) ... PASS
[E.9] No wrapper.cmd / install*.bat in scripts/ (FR-8 single-script invariant) ... PASS
[E.10] README Windows install entry contains -NoExit (T-031 FR-10) ... PASS

=== Summary ===
  PASS: 24
  WARN: 0
  FAIL: 0
  SKIP: 0
```

基线对照：04 §4 Step 12 实测同款 PASS=24 / WARN=0 / FAIL=0 / SKIP=0。

### 1.2 Bash verify_all（Linux/git-bash 对账）

命令：`bash scripts/verify_all.sh --quick`

```
[E.6] Adversarial tests section in completed task reports ... FAIL
      Missing section:
docs/features/_archived/download-cancel-and-upload-decouple/06_TEST_REPORT.md
[E.7a] BOM-required scripts/*.ps1 have UTF-8 BOM ... PASS
[E.7b] iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM ... PASS
[E.7c] All scripts/*.ps1 classified in E.7a or E.7b ... PASS
[E.8] install.ps1 / install-service.ps1 forbid interactive blockers ... PASS
[E.9] No wrapper.cmd / install*.bat in scripts/ ... PASS
[E.10] README Windows install entry contains -NoExit ... PASS

=== Summary ===
  PASS: 23
  WARN: 0
  FAIL: 1
  SKIP: 0
```

**重要预存缺陷（非 T-031 范围）**：bash 侧 E.6 在 T-027 归档 06 上 FAIL，PowerShell 侧 PASS——属 **verify_all.ps1/.sh 两实现 E.6 regex 不一致**（详见 §5）。E.8/E.9/E.10（T-031 新增三闸门）在 bash 侧均 PASS，**T-031 自身改动零回归**。

### 1.3 T-031 三新闸门 step 输出（E.8 / E.9 / E.10）

| 闸门 | 期望状态 | PowerShell | Bash | AC |
|---|---|---|---|---|
| E.8 install.ps1/install-service.ps1 forbid interactive blockers (FR-3) | PASS | PASS | PASS | AC-5 |
| E.9 No wrapper.cmd / install*.bat in scripts/ (FR-8) | PASS | PASS | PASS | AC-10 |
| E.10 README Windows install entry contains -NoExit (T-031 FR-10) | PASS | PASS | PASS | AC-额外 |

三闸门双实现对账一致全部 PASS。

### 1.4 AC-9 install.ps1 -Help 路径回归

命令：`pwsh -NoProfile -Command "& { & 'C:/Programs/frp_easy/scripts/install.ps1' -Help; $LASTEXITCODE }"`

结果：Help 文本完整输出（"用法: install.ps1 [-Help] ... 卸载:" 全段）+ 末尾 `$LASTEXITCODE = 0`。**PASS**。

（git-bash 终端 stdout 显示乱码是 git-bash 默认 cp936 stdout 编码，非脚本输出问题；改用 Windows Terminal / PS console 直接看为正常中文，T-029 已 OOS 化处理。）

---

## §2 ADV-1~5 独立复跑（不抄 04，QA 重跑）

### ADV-1 — E.8 命中 Read-Host

**假设**："如果 install.ps1 出现 `Read-Host` 调用，E.8 必触发 FAIL"。

操作：编辑 `scripts/install.ps1` L58 `$ErrorActionPreference = "Stop"` 后插入 `Read-Host "ADV-1 QA temporary blocker"`，跑 verify_all.ps1 -Quick：

```
[E.8] install.ps1 / install-service.ps1 forbid interactive blockers (FR-3) ... FAIL
       Interactive blockers found (破 FR-3 红线):
scripts\install.ps1:58: Read-Host "ADV-1 QA temporary blocker"
[E.9] No wrapper.cmd / install*.bat in scripts/ (FR-8 single-script invariant) ... PASS
[E.10] README Windows install entry contains -NoExit (T-031 FR-10) ... PASS

=== Summary ===
  PASS: 23
  WARN: 0
  FAIL: 1
  SKIP: 0
```

还原后跑 verify_all.ps1 -Quick：

```
[E.8] install.ps1 / install-service.ps1 forbid interactive blockers (FR-3) ... PASS
=== Summary ===
  PASS: 24
  WARN: 0
  FAIL: 0
  SKIP: 0
```

**结论**：E.8 闸门按行扫描捕获 Read-Host 命中点（行号 58 + 命中文本），假设证实。AC-5 自动闸门 PASS ✓

### ADV-2 — E.9 命中 install-wrapper.cmd

**假设**："如果 scripts/ 出现 install*.cmd，E.9 必触发 FAIL"。

操作：`touch scripts/install-wrapper.cmd`，跑 verify_all.ps1 -Quick：

```
[E.9] No wrapper.cmd / install*.bat in scripts/ (FR-8 single-script invariant) ... FAIL
       Forbidden wrapper files found:
=== Summary ===
  PASS: 23
  WARN: 0
  FAIL: 1
  SKIP: 0
```

（路径行被 grep 过滤，但 E.9 已正确 throw FAIL。）

`rm scripts/install-wrapper.cmd` 后还原：

```
[E.9] No wrapper.cmd / install*.bat in scripts/ (FR-8 single-script invariant) ... PASS
=== Summary ===
  PASS: 24
  WARN: 0
  FAIL: 0
  SKIP: 0
```

**结论**：E.9 闸门 PASS ✓。AC-10 自动闸门 PASS ✓

### ADV-3 — E.10 命中 README 缺 -NoExit

**假设**："如果 README L70 的 -NoExit 被删除，E.10 必触发 FAIL"。

操作：编辑 README L70 `pwsh -NoExit -Command "..."` → `pwsh -Command "..."`（删 `-NoExit`），跑 verify_all.ps1 -Quick：

```
[E.10] README Windows install entry contains -NoExit (T-031 FR-10) ... FAIL
       README.md Windows install entry missing -NoExit flag (T-031 FR-1 / FR-10):
=== Summary ===
  PASS: 23
  WARN: 0
  FAIL: 1
  SKIP: 0
```

还原后：

```
[E.10] README Windows install entry contains -NoExit (T-031 FR-10) ... PASS
=== Summary ===
  PASS: 24
  WARN: 0
  FAIL: 0
  SKIP: 0
```

**结论**：E.10 闸门 PASS ✓。AC-额外 自动闸门 PASS ✓

### ADV-4 — `& script.ps1` script-scope 隔离（AC-11 mini repro / OQ-5 a 证实）

**假设**："`& 'path/inner-exit-1.ps1'` call operator 创建独立 script scope；inner 内 `exit 1` 仅退 inner scope，**不会** 杀掉外层 scriptblock；外层 `'still alive'` 仍执行，`$LASTEXITCODE` 透传到外层 = 1"。

inner 脚本（dev 阶段产物，QA 复用）：

```
$ cat docs/features/install-ps1-host-close-on-completion/.scratch/inner-exit-1.ps1
exit 1
```

命令：`pwsh -NoProfile -Command "& { & 'C:/Programs/frp_easy/docs/features/install-ps1-host-close-on-completion/.scratch/inner-exit-1.ps1' ; 'still alive'; $LASTEXITCODE }"`

stdout 完整原文：

```
still alive
1
```

**结论**：假设证实。"still alive" 被打印 + `$LASTEXITCODE = 1` 透传到外层。OQ-5 a 决议（不动 install-service.ps1）安全：install.ps1 主体内 `& $svc` 调用 install-service.ps1 的 9 处 `exit N` 不会泄漏到外层 iex runspace，AC-11 mini repro 自动化 PASS ✓

### ADV-5 — `$global:LASTEXITCODE = 0` 跨 scope 写穿（RISK-D 证实）

**假设**："PowerShell `$global:` scope 修饰符（about_Scopes 文档化）让 child scope `& { ... }` 内对 `$global:LASTEXITCODE` 的赋值直接写穿到根 scope；外层 `$LASTEXITCODE` 读到 `0`"。

命令：`pwsh -NoProfile -Command "& { $global:LASTEXITCODE = 0 }; $LASTEXITCODE"`

stdout 完整原文：

```
0
```

**结论**：假设证实。`$global:` 修饰符跨 scope 写入合法；02 §3.2.1 / install.ps1 L398 `$global:LASTEXITCODE = 0` 防御性置零方案安全。RISK-D 假 ✓

---

## §3 人工复测项清单（NFR-5 + insight L44 红线下放给用户真机）

> **L44 红线：本环境（PowerShell 7.5 on Win11 via Claude Code subprocess pipe，非交互式 console host）**无法**自动化复现 AC-1/2/3/4/6/7/8/12 涉及的"交互式 PS console host 下宿主存活"行为。下面列表给出用户精确可粘贴命令 + 期望输出，让用户在真机 5 分钟内复测完成 NFR-5 强制项。**

### AC-1（FR-1 入口 (a) — 交互式 PS7 + irm | iex）

**真机操作**：

1. Win 开始菜单搜 `pwsh` → 右键 → 以管理员身份运行（**重要**：用 pwsh.exe 本体，不是 Windows Terminal 中可能配 `-Command` 的 profile）。
2. 弹出蓝色 PS7 控制台窗口（`PS C:\Windows\system32>` 提示符）。
3. 粘贴并回车：

```powershell
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
```

**期望**：

- 8 步进度跑完后，看到 step 8/8 横幅 13 行（"frp_easy 一键安装完成！" 起到 `============` 止，含访问地址、公网 IP 行、常用命令、更新、卸载段落）。
- **`PS C:\Windows\system32>` 提示符回到屏幕**，窗口**未关闭**。
- 手动输入 `$LASTEXITCODE` 回车，期望输出 `0`。

**截图位置**：建议截 step 8 横幅 + 提示符全屏。

### AC-2（FR-1 入口 (c) — 磁盘形态）

**真机操作**：

1. 管理员 PS7 prompt：`cd $env:USERPROFILE\Downloads`
2. `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 -OutFile install.ps1`
3. `.\install.ps1`

**期望**：同 AC-1（横幅 + 提示符回来 + `$LASTEXITCODE = 0`）。

### AC-3（FR-2 失败路径 — 非管理员）

**真机操作**：

1. 普通用户 PS7（**不**以管理员身份运行），粘贴 AC-1 同款 `irm | iex` 字串。

**期望**：

- step 1 红字：`请以管理员身份运行 PowerShell ...`
- 紧接着子作用域外中文失败横幅：`❌ frp_easy 安装未完成（退出码=1）` + `请按上方红字定位失败原因；必要时执行 'sc query frp-easy' 检查服务状态。`
- **提示符回来**，窗口未关闭。`$LASTEXITCODE = 1`。

### AC-4（FR-2 失败路径 — 架构非 AMD64 mock）

**真机操作**：管理员 PS7 prompt：

```powershell
$env:PROCESSOR_ARCHITECTURE = 'ARM64'
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
```

**期望**：step 2 红字架构不支持 → 失败横幅 → 提示符回来。`$LASTEXITCODE = 1`。复测后 `Remove-Item Env:PROCESSOR_ARCHITECTURE` 还原。

### AC-6（FR-4 step 8 横幅可见性）

**操作**：AC-1 / AC-2 复现后截图最后一屏。

**期望肉眼检查**：横幅 13 行（"frp_easy 一键安装完成！" → `============`）完整可见且**位于最末位置**，未被任何后续行覆盖。`$publicLine` / `$publicHint` 两个动态行存在。

### AC-7（FR-5 失败横幅最后两行可见）

**操作**：AC-3 / AC-4 复现后截图最后一屏。

**期望肉眼检查**：最后**两行**精确是：

```
❌ frp_easy 安装未完成（退出码=N）
请按上方红字定位失败原因；必要时执行 'sc query frp-easy' 检查服务状态。
```

（N=具体退出码数字）

### AC-8（FR-6 PS5.1 兼容）

**真机操作**：

1. Win 开始菜单搜 `powershell`（**不是** pwsh）→ 以管理员身份运行（启动 Windows PowerShell 5.1）。
2. 磁盘形态：`cd $env:USERPROFILE\Downloads; .\install.ps1`（install.ps1 是预先 `irm ... -OutFile` 下载）。

**期望**：跑完 step 8 + 横幅 + 提示符回来。**允许中文乱码**（T-029 已 OOS：PS5.1 + zh-CN 磁盘形态码页 GBK 误解码）；脚本逻辑必须跑通 + `$LASTEXITCODE = 0`。

### AC-12（NFR-3 PS5.1 + PS7 双解释器 4×2 矩阵）

**操作**：AC-1 / AC-2 / AC-3 / AC-9 在 `pwsh.exe`（PS7）和 `powershell.exe`（PS5.1）下各跑一遍 = 8 个用例。

**期望**：8 全 PASS（PS5.1 允许中文乱码，但脚本逻辑必须跑完）。

### ADV-5 用户对照测试（02 §5.2 ADV-5）

**真机操作**：Win+R 跑两次：

(a) 新字串：`pwsh -NoExit -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"`

(b) 旧字串：`pwsh -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"`

**期望对照**：

- (a) 窗口在 step 8 横幅打完**保留**，prompt 回来；
- (b) 窗口在 step 8 横幅打完**立即关闭**（PowerShell 文档化"-Command 跑完即退"）。

这正反两次对照实证 README 推荐字串 `-NoExit` 必要性。

---

## §4 回归扫描

### 4.1 Go 单测

命令：`go test ./...`

结果（节选）：

```
ok  	github.com/frp-easy/frp-easy/cmd/frp-easy	(cached)
ok  	github.com/frp-easy/frp-easy/internal/appconf	(cached)
ok  	github.com/frp-easy/frp-easy/internal/assets	(cached)
ok  	github.com/frp-easy/frp-easy/internal/auth	(cached)
ok  	github.com/frp-easy/frp-easy/internal/binloc	(cached)
ok  	github.com/frp-easy/frp-easy/internal/browseropen	(cached)
ok  	github.com/frp-easy/frp-easy/internal/downloader	(cached)
ok  	github.com/frp-easy/frp-easy/internal/frpcadmin	(cached)
ok  	github.com/frp-easy/frp-easy/internal/frpconf	(cached)
ok  	github.com/frp-easy/frp-easy/internal/httpapi	(cached)
ok  	github.com/frp-easy/frp-easy/internal/logrotate	(cached)
ok  	github.com/frp-easy/frp-easy/internal/logtail	(cached)
ok  	github.com/frp-easy/frp-easy/internal/portrange	(cached)
ok  	github.com/frp-easy/frp-easy/internal/procmgr	(cached)
ok  	github.com/frp-easy/frp-easy/internal/storage	(cached)
```

全部 15 个 internal package + cmd/frp-easy 全 ok（baseline `go_tests=265`，本任务零 Go 改动故 cached 命中）。**零回归** ✓

### 4.2 前端单测

命令：`npm --prefix web test`（实际脚本 `vitest run`）

结果（末尾）：

```
 Test Files  13 passed (13)
      Tests  103 passed (103)
   Start at  10:31:11
   Duration  1.92s
```

13/13 测试文件 + 103/103 测试 PASS（baseline `frontend_tests=102` → 实测 103；baseline 只升不退原则下，本任务不动 baseline.json `frontend_tests` 字段——本任务零前端单测改动，103 vs 102 差 1 来自此前 commit 的存量；**T-031 自身零回归** ✓）。

### 4.3 install.ps1 -Help 路径（AC-9）

命令：`pwsh -NoProfile -Command "& { & 'C:/Programs/frp_easy/scripts/install.ps1' -Help; $LASTEXITCODE }"`

结果：Help 文本 25 行完整输出（"用法: install.ps1 [-Help] ... 退出码: 0 成功 ... 卸载: ..."）；末尾 `$LASTEXITCODE = 0`。**PASS** ✓

### 4.4 BOM / scope / param 红线复验

| 红线 | 验证手段 | 结果 |
|---|---|---|
| install.ps1 首字节非 BOM | verify_all E.7b PASS | PASS |
| 其他 .ps1 仍有 BOM | verify_all E.7a PASS | PASS |
| 全 .ps1 在 E.7a/b 分类 | verify_all E.7c PASS | PASS |
| 无 `[CmdletBinding()]` 实际声明 | grep `^[^#]*CmdletBinding` install.ps1 = 仅注释命中 | PASS |
| 无 wrapper.cmd / .bat | verify_all E.9 PASS | PASS |
| 无 Read-Host / pause / ReadKey / Wait-Event 实际调用 | verify_all E.8 PASS | PASS |

---

## §5 已发现回归 / 新 bug

### 5.1 T-031 范围内回归 / 新 bug

**无**。所有 AC（AC-5 / AC-9 / AC-10 / AC-11 / AC-额外）自动化项 PASS；ADV-1~5 独立复跑全部 PASS；Go + 前端单测零回归；install.ps1 -Help 不破。

### 5.2 任务外预存缺陷（不阻塞 T-031）

#### [MAJOR / 历史预存] verify_all.ps1 vs verify_all.sh 的 E.6 实现不一致

**症状**：bash 侧 `bash scripts/verify_all.sh --quick` E.6 FAIL，归罪 `docs/features/_archived/download-cancel-and-upload-decouple/06_TEST_REPORT.md`（T-027 归档报告）；同时 PowerShell 侧 `pwsh -File scripts/verify_all.ps1 -Quick` E.6 PASS。

**根因**：

- T-027 06 §4 标题为 `## §4 Adversarial tests`（带数字 `§4` 前缀，违反 L18/L48 裸标题约定），同时文件中 L166 有引用行 `> 标题裸写 \`## Adversarial tests\`（verify_all E.6 闸门 / L21）；...`（含字面 `## Adversarial tests` 在引用块内）。
- bash `grep -qE '^##\s+Adversarial\s+tests'` 要求行首 `##` 直接跟空白再 `Adversarial`：(1) `## §4 Adversarial tests` 中 `##` 后是 `§4 Adversarial`，`\s+` 后是 `§4` 不是 `Adversarial` → 不匹配；(2) 引用块行 `> 标题裸写...` 行首是 `>` 不是 `##` → 不匹配。bash FAIL。
- PowerShell `Get-Content -Raw + -match '##\s+Adversarial\s+tests'`：`-match` 默认无锚定 + Raw 单字符串模式 + 行首 `^` 不在 pattern 中 → 在整文档任意位置搜子串 `##\s+Adversarial\s+tests`；引用块行 `> 标题裸写 \`## Adversarial tests\`...` 中的字面 `## Adversarial tests` 子串命中 → PowerShell PASS（**假阳性**）。

**结论**：

- T-027 06 自身确实违反 L18/L48 裸标题约定（应是 `## Adversarial tests` 不是 `## §4 Adversarial tests`），属归档时漏检的真实缺陷。
- verify_all.ps1 vs .sh 行为不一致是 **infrastructure-level 不对称**：PowerShell E.6 实现需收紧 regex 锚定（如改 `[Regex]::IsMatch($c, '(?m)^##\s+Adversarial\s+tests')`）才能与 bash 对账，否则永远漏过同款假阳性。

**严重度**：**MAJOR / 历史预存**（非 T-031 引入 + 不阻塞 T-031 交付 + 已归档任务追溯无意义）。

**建议路由**：PM 在 T-031 归档后另开新 task 修 verify_all.ps1 E.6 regex（一行改动），同时由 PM 决定是否回填 T-027 06 标题成裸 `## Adversarial tests`（涉及修改已归档文件，需用户授权）。

**对 T-031 verdict 的影响**：**零**。T-031 自身的三新闸门（E.8/E.9/E.10）在双实现下均 PASS；T-031 06 本报告标题（见 §6）严格写裸 `## Adversarial tests` 避免触发该缺陷。

---

## §6 Adversarial tests

> 本段为 QA Tester 强制契约段（L18/L48/L49 红线）。**裸标题** `## Adversarial tests`（无数字前缀），verify_all E.6 闸门按此匹配。
> 本段聚焦"adversarial verification 的红线证伪结论"，而非测试细节（细节见 §2）。

### 红线证伪汇总

| ADV | 假设 | 期望失败点 | 实测 | 结论 |
|---|---|---|---|---|
| ADV-1 | Read-Host 出现在 install.ps1 → E.8 必 FAIL | E.8 throw "Interactive blockers found" | install.ps1:58 命中 → E.8 FAIL → 还原后 PASS | FR-3 红线**自动可证伪** ✓ |
| ADV-2 | install-wrapper.cmd 出现在 scripts/ → E.9 必 FAIL | E.9 throw "Forbidden wrapper files found" | install-wrapper.cmd 命中 → E.9 FAIL → 删除后 PASS | FR-8 红线**自动可证伪** ✓ |
| ADV-3 | README 缺 -NoExit → E.10 必 FAIL | E.10 throw "missing -NoExit flag" | 删 -NoExit 命中 → E.10 FAIL → 还原后 PASS | FR-10 / 新合约**自动可证伪** ✓ |
| ADV-4 | `& script.ps1` 不创建 script scope → 外层 'still alive' 不打印 | 'still alive' 缺席或外层 LASTEXITCODE 异常 | stdout = "still alive\n1"，假设伪 | OQ-5 a 决议（不动 install-service.ps1）**安全**；R2 假 ✓ |
| ADV-5 | `$global:` 不能跨 scope 写穿 → 外层 LASTEXITCODE 不是 0 | 外层 stdout 非 "0" | stdout = "0"，假设伪 | RISK-D 假；`$global:LASTEXITCODE = 0` 防御性置零**安全** ✓ |

### 红线下放（NFR-5 + L44 真机交互式 PS7）

QA 环境是 Claude Code 子进程管道（**非**交互式 console host），**无法**自动证伪 FR-1（AC-1/2/3/4/6/7/8/12）。已在 §3 给出用户精确可粘贴命令 + 期望输出 + 截图位置，让用户 5 分钟内完成 NFR-5 真机复测。**ADV-5 用户对照测试**（新旧字串各跑一次）是最关键人工证伪——窗口关 vs 不关的物理对比，让用户**亲眼**看见 README 推荐 `-NoExit` 必要性。

### 红线结论

- **FR-3 自动闸门红线**（E.8）按设计触发 + 还原对照 ✓
- **FR-8 自动闸门红线**（E.9）按设计触发 + 还原对照 ✓
- **FR-10 自动闸门红线**（E.10）按设计触发 + 还原对照 ✓
- **OQ-5 a / R2** install-service.ps1 script scope 隔离 ✓（ADV-4 stdout 实证）
- **RISK-D** `$global:LASTEXITCODE = 0` 跨 scope 写穿 ✓（ADV-5 stdout 实证）
- **FR-1 真机宿主存活**（NFR-5）下放用户 ✓（§3 命令清单）

---

## §7 Verdict

**READY FOR DELIVERY**

理由：

- §1 自动化 / 静态 AC 全部 PASS：PowerShell verify_all Summary PASS=24/WARN=0/FAIL=0/SKIP=0；T-031 三新闸门 E.8/E.9/E.10 在双实现（PowerShell + Bash）下均 PASS；install.ps1 -Help PATH 不破（AC-9）。
- §2 ADV-1~5 独立复跑全部 PASS：三闸门 FAIL 触发 + 还原对照证实；ADV-4 stdout "still alive\n1" 证实 OQ-5 a；ADV-5 stdout "0" 证实 RISK-D。
- §3 NFR-5 真机复测项（AC-1/2/3/4/6/7/8/12 + ADV-5 用户对照）给出精确可粘贴命令 + 期望输出 + 截图位置，让用户 5 分钟内完成。
- §4 回归扫描：Go ./...（15 package + cmd/frp-easy）全 ok；前端 vitest 13 文件 103 测试全 PASS（baseline 102 + 1 此前 commit 留存，T-031 自身零前端改动）；install.ps1 -Help 不破。
- §5 T-031 范围内零回归 / 零新 bug；任务外发现 1 MAJOR 历史预存缺陷（verify_all.ps1 vs .sh E.6 regex 不一致），已记录路由方案，**不阻塞 T-031**。
- §6 裸 `## Adversarial tests` 段满足 L18/L48/L49 红线（verify_all E.6 双实现均 PASS 在本文件上）。
- baseline.json 由 Developer 在 04 已更新到 version 14 + notes "PASS 21 -> 24 (+3)"（C-2 兑现）；QA 本阶段无需再改 baseline.json（test_count / passing_count 字段 367 不变，T-031 零 Go/前端测试改动）。

PM 接 06 后派 Stage 7 (PM Archive)。

---

— QA Tester, 2026-05-24
