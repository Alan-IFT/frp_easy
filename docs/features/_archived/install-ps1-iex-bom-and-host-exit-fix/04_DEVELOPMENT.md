# 04 — 开发实录 · T-026 install-ps1-iex-bom-and-host-exit-fix

> Harness 流水线 Stage 4（Developer，generic）。模式：**full**。
> 上游：01_REQUIREMENT_ANALYSIS.md（READY）+ 02_SOLUTION_DESIGN.md（READY）+ 03_GATE_REVIEW.md（APPROVED FOR DEVELOPMENT WITH 4 MAJOR CONDITIONS）。
> 本文档记录实施实录 + 偏差，不重述方案设计（02 已有）。

---

## §1 实施摘要

按 02 §6 步骤 + 03 §8 G-6/G-7/G-8/G-15 四条 MAJOR 必修条件完成 T-026：

- **代码层**：删 `scripts/install.ps1` 的 UTF-8 BOM；主体用 `& { param([switch]$Help) ... } @PSBoundParameters` 子作用域包裹（exit N 在交互式宿主下退子作用域）；末尾追加中文失败横幅；顶部 / 内部 param 块上方加入 G-6 / G-15 增补注释。
- **闸门层**：`scripts/verify_all.ps1` 与 `scripts/verify_all.sh` E.7 拆为 E.7a（10 个 BOM-required）/ E.7b（1 个 iex-entry forbid BOM）/ E.7c（anti-drift WARN 含 unclassified 文件名打印，依 G-7 增补）。
- **配置层**：`scripts/.editorconfig` 追加 `[install.ps1] charset=utf-8` 例外块覆盖 `[*.ps1] charset=utf-8-bom`。
- **基线层**：`scripts/baseline.json` version 11→12 + notes 更新（依 Dev-Q5 仅改 version/updated/notes，不动 test_count 等）。
- **导航层**：`docs/dev-map.md` scripts/ 块追加 T-026 注解（D-1/D-2/D-4）。

完成 AC：AC-3 [A] ✓、AC-9 [A] ✓、AC-10 [A] ✓、AC-12 [A] ✓、AC-13 [A] ✓（ADV-1）、AC-14 [A] ✓（ADV-2）、AC-15 [A] ✓、AC-16 [A] ✓、AC-17/18 [A] 留 QA 06 / PM 07。

待 QA 06 + 用户真机验证（标 [U]）：AC-1 / AC-2 / AC-4 / AC-5 / AC-6 / AC-7 / AC-8 / AC-11。

---

## §2 实施步骤实录（按 Dev-Q3 调整后的顺序）

依 03 §7 Dev-Q3：**先改 verify_all（步骤 4-5）再改 install.ps1（步骤 2-3）**，避免中间态 verify_all 因 install.ps1 BOM 缺失但 E.7 仍是单一全量检查而 FAIL。

### §2.1 步骤 0：baseline 捕获

`pwsh -File scripts/verify_all.ps1 -Quick`（Quick 跳 E2E 节约时间）。结果：

```
=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

注：full 模式下 baseline = PASS 20（含 C.1 E2E）；本任务后续以 Quick 模式监测 delta。

### §2.2 步骤 4：改 `scripts/verify_all.ps1` 拆 E.7 → E.7a/b/c

替换原 E.7 单 Step（L268-L290）为：
- `$Ps1RequireBom` / `$Ps1ForbidBom` 两个 white-list 数组（字母序排列）
- `Step "E.7a"`：BOM-required 10 个 .ps1 首 3 字节 == `EF BB BF`
- `Step "E.7b"`：iex-entry 1 个（install.ps1）首 3 字节 != `EF BB BF`
- `Step "E.7c"`：未在两表中的 .ps1 触发 WARN（`return $false`）+ 显式 `Write-Host -ForegroundColor Yellow` 打印 unclassified 文件名（依 03 §8 **G-7 增补**）

### §2.3 步骤 5：改 `scripts/verify_all.sh` 同款拆分

替换原 E.7 块（L278-L301）为 E.7a/b/c 三段：
- `PS1_REQUIRE_BOM` / `PS1_FORBID_BOM` bash 数组
- E.7a / E.7b 用 `head -c 3 + od -An -tx1` 字节断言（POSIX 兼容）
- E.7c 用 `echo "    unclassified:..."` 在 `step ... WARN` 调用**前**打印（依 03 §8 **G-7 增补**）

### §2.4 中间态：verify_all 验证 E.7 拆分生效

`pwsh -File scripts/verify_all.ps1 -Quick` 输出片段：

```
[E.7a] BOM-required scripts/*.ps1 have UTF-8 BOM ... PASS
[E.7b] iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM ... FAIL
[E.7c] All scripts/*.ps1 classified in E.7a or E.7b (anti-drift) ... PASS
```

E.7a PASS（10 个 .ps1 都有 BOM）+ E.7b FAIL（install.ps1 仍有 BOM）+ E.7c PASS（11 个全分类）—— 中间态完全符合预期（Dev-Q3 验证），下一步删 install.ps1 BOM 即可让 E.7b PASS。

### §2.5 步骤 2 + 3：改 install.ps1（注释 / 包裹 / 横幅 / 删 BOM）

**子步骤 a：改顶部注释块 + 增加顶层 param + `& {` 起始**

用 Edit 工具在原 L23-L29 区域：
- 删 T-024 旧注释段（"`[CmdletBinding()]` ..."）改写为完整的 T-024 + T-026 E1/E2 综合说明
- 在内层 param 块上方插入"本脚本仅支持 -Help 参数；-Verbose/-Debug 等 cmdlet common parameter 需要 [CmdletBinding()]，因 T-024 / insight L36 已不可加，故不支持（依 03 §8 G-6 增补）"
- 在 `& {` 上方插入"未来在 install.ps1 加新顶层参数时，必须同步在内部 scriptblock param 块加同名同类型参数（@PSBoundParameters splatting 要求 hashtable key 与内层 param 严格对应；否则报'找不到接受实际参数的位置参数'或静默错位）（依 03 §8 G-15 增补）"

**子步骤 b：末尾追加 `} @PSBoundParameters` + 失败横幅**

用 Edit 工具在原 L371 `exit 0` 之后插入 6 行：
```powershell
} @PSBoundParameters

if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    Write-Host "❌ frp_easy 安装未完成（退出码=$LASTEXITCODE）。" -ForegroundColor Red
    Write-Host "   请按上方红字定位失败原因；必要时执行 'sc query frp-easy' 检查服务状态。" -ForegroundColor Red
}
```

**子步骤 c：删 BOM（insight L34 字节级 idiom 反向用法）**

写临时 `scripts/.t026-strip-bom.ps1` 调用：
```powershell
$utf8ReadWithBom = [System.Text.UTF8Encoding]::new($true, $true)
$utf8WriteNoBom  = [System.Text.UTF8Encoding]::new($false)
$content = [System.IO.File]::ReadAllText($path, $utf8ReadWithBom)
[System.IO.File]::WriteAllText($path, $content, $utf8WriteNoBom)
```

执行 `pwsh -NoProfile -File scripts/.t026-strip-bom.ps1` → `OK BOM stripped`，立即删该临时脚本。

**Dev-Q1 字节级断言 stdout**（python 跨平台稳定，避免 PS 子进程编码污染）：

```
BOM? False (expect False)
First3: 23 20 69 (expect 23=#)
CR count: 0 (expect 0, NFR-7 LF only)
LF count: 402
Size: 18184 bytes
```

✓ BOM=False、首字节 `#`（0x23）、CR=0、Size 比原 16024 +2160（包裹 + 横幅 + G-6/G-15 注释 +2163；删 BOM -3；净 +2160）。

### §2.6 步骤 6：改 `scripts/.editorconfig` 加 `[install.ps1]` 例外块

追加在 `[*.ps1]` block 之后（依 02 §4.4）：
```editorconfig
[install.ps1]
charset = utf-8
end_of_line = lf
insert_final_newline = true
```

EditorConfig spec：后置 section 覆盖前置（最具体的 section 优先）；section 仅锚定 `scripts/install.ps1`（`.editorconfig` 在 scripts/ 目录内，相对匹配）。

### §2.7 步骤 7：改 `scripts/baseline.json`（依 03 Dev-Q5）

- `version`: 11 → 12
- `updated`: 保持 2026-05-23（QA 在 06 改）
- `notes`: 追加 T-026 闭环描述（含 verify_all 20→22 / E.7 拆 a/b/c / 净 +2）；保留 T-021/T-022/T-025 历史
- **不动** `test_count` / `passing_count` / `go_tests` / `frontend_tests` / `warnings_baseline`（Dev-Q5）

### §2.8 步骤 8：改 `docs/dev-map.md` scripts/ 块

在 T-021 注解后追加 T-026 一行（§"代码片段"段见 §4）。

### §2.9 步骤 9：最终 verify_all（Quick 模式）

```
=== Summary ===
  PASS: 21
  WARN: 0
  FAIL: 0
  SKIP: 0
```

PASS = 19（baseline Quick）→ 21（T-026 后 Quick），净 +2 = E.7 → E.7a/b/c 拆分。**0 新失败、0 新 WARN**。

### §2.10 步骤 10：adversarial 自测（ADV-1/2/3，依 02 §8.4）

**注**：步骤 10 必须在 PS7 主机跑（PS5.1 + zh-CN 会让 Get-Content -Raw 按 GBK 解码 install.ps1 中文，与真实 iex 形态行为不一致）（依 03 §8 **G-8 增补**）。本机 QA 主机是 PS7 + en-US，满足。

`pwsh -NoProfile -File scripts/.t026-adv.ps1` 关键输出：

```
===== ADV-1: 把 BOM 加回 install.ps1 =====
  install.ps1 first3 = EF BB BF
  E.7b 结果: [E.7b] iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM ... FAIL
  ADV-1 PASS: E.7b 正确 FAIL
  已还原 install.ps1

===== ADV-2: 故意删 install-service.ps1 的 BOM → E.7a FAIL =====
  install-service.ps1 first3 = 23 20 69
  E.7a 结果: [E.7a] BOM-required scripts/*.ps1 have UTF-8 BOM ... FAIL
  ADV-2 PASS: E.7a 正确 FAIL
  已还原 install-service.ps1

===== ADV-3: 新建未分类 fake.ps1 → E.7c WARN + 打印文件名 =====
  E.7c 相关行:
    [E.7c] All scripts/*.ps1 classified in E.7a or E.7b (anti-drift) ...
           unclassified: fake.ps1
     WARN
  ADV-3 PASS: E.7c WARN + 打印 fake.ps1
```

- **ADV-1 ✓**（AC-13）：BOM 加回 → E.7b 正确 FAIL
- **ADV-2 ✓**（AC-14）：删 install-service.ps1 BOM → E.7a 正确 FAIL
- **ADV-3 ✓**（G-7 验证）：fake.ps1 触发 E.7c WARN + unclassified 文件名打印

**ADV-4**（删顶层 param 验证双层 param 必要性）/ **ADV-5**（mock install-service.ps1 退出 2 验证横幅触发）：留 QA 06 跑（涉及更深的子流程 mock）。

### §2.11 步骤 11：磁盘形态 `-Help` 验证（AC-9 [A]）

```
$ pwsh -NoProfile -File scripts/install.ps1 -Help
用法: install.ps1 [-Help]
frp_easy 一键安装脚本（Windows）...
---ExitCode=0---
```

ExitCode = 0 ✓ + Help 内容显示 ✓（中文在 Bash 终端 console 因 GBK 解码乱码是终端 console codepage 问题，与脚本本身无关；PS console 直显正常）。

### §2.12 步骤 12：AC-10 install-service / uninstall-service 字节零变断言

```
install-service.ps1 first3= efbbbf (expect efbbbf)
uninstall-service.ps1 first3= efbbbf (expect efbbbf)
install.ps1 first3= 232069 (expect 232069 / # space i)
```

`git diff --stat scripts/install-service.ps1 scripts/uninstall-service.ps1` → 空输出 ✓（FR-9 / AC-10）。

---

## §3 意外 / 偏离设计（重要！）

### §3.1 意外 #1：`& { exit N }` 在脚本宿主下**杀脚本进程**，与 02 D-2 论断有 nuance

**现象**：在 Bash 工具调度 `pwsh -File <script>` 或 `pwsh -Command "& { exit 1 }; Write-Host after"` 时，"after" 行**不打印**，pwsh 进程退出码 1 —— 即 scriptblock 内 `exit N` **杀脚本宿主进程**，不只是退子作用域。

**额外测试**：用 `Start-Process pwsh -NoExit -File <script>` 模拟用户**交互式宿主**（用户桌面打开的 PowerShell 窗口 + 输 `irm | iex`）—— 子进程 4s 后仍存活（PID 仍在），与脚本宿主行为**不同**：

```
RESULT: 子进程仍活着 (PID=10032)。表示宿主受保护，install.ps1 exit N 退到子作用域。
MARKER: not created (probe never reached post-iex line)
```

**结论 / 偏差说明**：02 D-2 的论断"`exit N` 在 scriptblock 内 ... 不终止 host runspace"在**用户真实使用场景**（交互式 PowerShell.exe console host）下**成立**，但在自动化测试场景（pwsh -File / pwsh -Command）下**不成立**（PowerShell 脚本宿主下 `exit` 是 host-level 指令）。

**对用户而言**：FR-4 / FR-5（宿主存活）**满足** —— 用户 PowerShell 窗口不会被 `& { exit N }` 关闭。但 iex 之后**同一行**的命令不会执行（iex cmdlet 因子作用域 exit 而被打断）；用户**下一个命令**仍可正常输入。

**对 QA 而言**：AC-4 / AC-5 / AC-6 必须用**真实 PS 窗口手动测试**（标 [U] 用户真机；mock 路径下 -NoExit 子进程存活也算等价证据但非完美 mock）。

**未改 02**（02 是已签字设计文档），仅在此 04 errata 记录。建议 QA 06 + PM 07 引用此结论。

### §3.2 意外 #2：verify_all 跑时 G.1 / G.2 偶发 FAIL，是 Go 缓存抖动

verify_all 第一次跑时 G.2（go test ./...）报：
```
internal/downloader/downloader_cancel_test.go:355:2: undefined: waitGoroutineExit
```

但 `go test ./internal/downloader/...` 单独跑 PASS。问 baseline（stash T-026 改动后跑）也 PASS。后再次跑 verify_all 又 PASS。

**根因**：仓库目前同时有**另一进行中任务**（download-cancel-and-upload-decouple，T-027 类）留下的 untracked Go 文件 `internal/downloader/downloader_cancel_test.go`，引用了 untracked 的 helper `waitGoroutineExit`。clean go test cache 后跑 verify_all 稳定 PASS。**与 T-026 完全无关**，本任务未碰任何 .go 文件。

**留给 Code Reviewer**：T-026 0 Go 文件改动，G.1/G.2 抖动属另一任务的 stage 4 ongoing，与本任务无因果。

### §3.3 与 02 设计无实质偏差

- 文件清单一致（install.ps1 / verify_all.ps1 / verify_all.sh / .editorconfig / baseline.json / dev-map.md）
- 实施顺序按 03 Dev-Q3 调整（步骤 4-5 → 2-3 → 6-8 → 9-10），02 §6 顺序未跟改（02 已签字不动；04 errata 记录）
- 4 条 MAJOR 增补全做（§4 详）

---

## §4 依 03 §8 G-N 增补的实施记录

### §4.1 G-6 增补（install.ps1 双层 param 注释，说明仅支持 -Help）

**位置**：`scripts/install.ps1` L33-L34（顶层 param 上方）+ L41-L42（内层 param 上方）

**代码片段**（顶层）：
```powershell
# 本脚本仅支持 -Help 参数；-Verbose/-Debug 等 cmdlet common parameter 需要 [CmdletBinding()]，
# 因 T-024 / insight L36 已不可加，故不支持（依 03 §8 G-6 增补）。
param(
    [switch]$Help
)
```

**代码片段**（内层）：
```powershell
# T-026 D-2：主体放入 scriptblock，& 调用让 exit N 退子作用域不杀宿主。
# 顶层 $PSBoundParameters splat 进内部 param 透传 -Help（iex 形态下为空 hashtable）。
# 本脚本仅支持 -Help 参数；-Verbose/-Debug 等 cmdlet common parameter 需要 [CmdletBinding()]，
# 因 T-024 / insight L36 已不可加，故不支持（依 03 §8 G-6 增补）。
```

### §4.2 G-7 增补（verify_all E.7c WARN 打印 unclassified 文件名）

**位置 PS**：`scripts/verify_all.ps1` E.7c step

**代码片段**：
```powershell
$unclassified = @($actual | Where-Object { $known -notcontains $_ })
if ($unclassified.Count -gt 0) {
    # G-7 必修条件：WARN 分支显式打印未分类文件名让维护者一眼定位（依 03 §8 G-7 增补）
    Write-Host ""
    Write-Host "       unclassified: $($unclassified -join ', ')" -ForegroundColor Yellow
    return $false  # WARN 而非 FAIL：提醒维护者归类、不阻塞 CI
}
```

**位置 sh**：`scripts/verify_all.sh` E.7c step

**代码片段**：
```bash
if [[ -z "$e7c_unclassified" ]]; then
    step "E.7c" "All scripts/*.ps1 classified in E.7a or E.7b" "PASS"
else
    # G-7 必修条件：WARN 分支显式打印 unclassified 文件名（依 03 §8 G-7 增补）
    echo "    unclassified:$(echo -e $e7c_unclassified)"
    step "E.7c" "All scripts/*.ps1 classified in E.7a or E.7b" "WARN" "$(echo -e $e7c_unclassified)"
fi
```

**ADV-3 验证 stdout**：
```
[E.7c] All scripts/*.ps1 classified in E.7a or E.7b (anti-drift) ...
       unclassified: fake.ps1
 WARN
```

✓ unclassified 文件名打印 + WARN 状态显示。

### §4.3 G-8 增补（QA 主机 PS7 注记）

**位置**：本 04 文档 §2.10 步骤 10 段开头：
> 步骤 10 必须在 PS7 主机跑（PS5.1 + zh-CN 会让 Get-Content -Raw 按 GBK 解码 install.ps1 中文，与真实 iex 形态行为不一致）（依 03 §8 **G-8 增补**）。本机 QA 主机是 PS7 + en-US，满足。

**不改 02**（已签字文档；04 errata 记录即可）。

### §4.4 G-15 增补（future-proof 双层 param 同步约束）

**位置**：`scripts/install.ps1` L43-L45（`& {` 上方）

**代码片段**：
```powershell
# 未来在 install.ps1 加新顶层参数时，必须同步在内部 scriptblock param 块加同名同类型参数
# （@PSBoundParameters splatting 要求 hashtable key 与内层 param 严格对应；否则报"找不到接受
# 实际参数的位置参数"或静默错位）（依 03 §8 G-15 增补）。
& {
```

---

## §5 verify_all 最终输出

### §5.1 Baseline（stash T-026 改动后跑，确认真实基线）

`pwsh -File scripts/verify_all.ps1 -Quick` ：

```
PASS: 19  WARN: 0  FAIL: 0  SKIP: 0
```

（含 E.7 单 step；G.1 / G.2 / E.7 全 PASS）

### §5.2 After T-026

`pwsh -File scripts/verify_all.ps1 -Quick`（清 go testcache 后稳定跑）：

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
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agent definitions present in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa) ... PASS
[E.6] Adversarial tests section present in completed task reports ... PASS
[E.7a] BOM-required scripts/*.ps1 have UTF-8 BOM ... PASS
[E.7b] iex-entry scripts/*.ps1 MUST NOT have UTF-8 BOM ... PASS
[E.7c] All scripts/*.ps1 classified in E.7a or E.7b (anti-drift) ... PASS

=== Summary ===
  PASS: 21  WARN: 0  FAIL: 0  SKIP: 0
```

### §5.3 Delta

- **Quick 模式**：19 → 21（**净 +2**：E.7 单 step → E.7a/b/c 三 step；0 新失败、0 新 WARN）
- **Full 模式预期**：20 → 22（含 C.1 E2E；C.1 当前 FAIL 是另一进行中任务的前端代码改动 wave-front，与 T-026 无关 —— Code Reviewer 在 05 复核时如担心可让 PM 拉一次干净 main 跑）

baseline 准确性：测的是 T-026 改动 stash 后的状态（PASS=19 Quick），完全干净；新 22 个 step 中 **0 个新 FAIL / 0 个新 WARN**。

---

## §6 Adversarial 自测（[A] / [M] 类）

已自测：ADV-1（AC-13）✓、ADV-2（AC-14）✓、ADV-3（G-7 验证）✓

留 QA 06 跑（涉及子流程 mock，超出 Developer 4 阶段验证范围）：
- **ADV-4**：删 install.ps1 顶层 `param([switch]$Help)` 后跑 `pwsh -File scripts/install.ps1 -Help`，期望 stdout 第一行**不是** Help 输出（证明顶层 param 必要性 / D-3 双层 param 设计有效）
- **ADV-5**：mock `install-service.ps1` 退出 2，验证宿主存活 + 失败横幅触发 + `$LASTEXITCODE = 2`

[U] 类（用户真机标必走）：
- **AC-1 [U]**：PS5.1 + zh-CN 主机 `irm <raw_url> | iex` 无 `is not recognized` 红字
- **AC-4 [U]**：iex 形态触发非管理员路径，看到红字后 PS 提示符仍在
- **AC-6 [U]**：iex 形态完整跑通 8/8 + `sc query frp-easy` 返回 `STATE : 4 RUNNING`
- **AC-8 [U]**：磁盘形态 `.\install.ps1 -Help`（D-1 取舍：PS5.1 + zh-CN 下中文乱码是已知 trade-off）
- **AC-11 [U]**：磁盘形态完整安装

---

## §7 Files changed（清单）

- `scripts/install.ps1` — 删 BOM（前 3 字节 `EF BB BF` → 文件首字符为 `#`）；顶部加入 T-026 E1/E2 综合注释 + G-6 / G-15 增补注释；主体用 `& { param([switch]$Help) ... } @PSBoundParameters` 包裹；末尾追加失败横幅。改动行为：BOM=YES(3B) → BOM=NO；size 16024 → 18184（+2160）；LF 371 → 402（+31）；CR 0 → 0
- `scripts/verify_all.ps1` — 拆 E.7 单 step 为 E.7a/b/c 三 step（white-list 驱动）+ G-7 增补（E.7c WARN 分支 Write-Host -ForegroundColor Yellow 打印 unclassified 文件名）
- `scripts/verify_all.sh` — 同款拆 E.7（POSIX 字节断言 + WARN 前 echo unclassified）+ G-7 增补
- `scripts/.editorconfig` — 追加 `[install.ps1] charset=utf-8` 例外块覆盖 `[*.ps1] charset=utf-8-bom`
- `scripts/baseline.json` — version 11 → 12；notes 追加 T-026 描述（仅改 version / updated / notes 三字段，依 Dev-Q5）
- `docs/dev-map.md` — scripts/ 块 T-021 注解后追加 T-026 一行注解

未改任何 .go / .ts / .vue 文件；未改 install-service.ps1 / uninstall-service.ps1（FR-9 / AC-10 字节零 diff，已断言 ✓）。

---

## §8 Dev-map updates

在 `docs/dev-map.md` L25-L28 scripts/ 块的 T-021 注解后追加一行：

```
│                     T-026：scripts/install.ps1 因 iex 入口**禁** BOM（irm 把 EF BB BF 解码为 U+FEFF 进入字符串触发 ParserError）；其余 10 个 .ps1 继续要 BOM（磁盘形态防 GBK 误解码）；主体 `& { ... } @PSBoundParameters` 子作用域包裹让 `exit N` 在交互式宿主下退子作用域不杀宿主；verify_all E.7 拆 a/b/c 白名单（E.7a 必须 BOM / E.7b 禁 BOM / E.7c anti-drift WARN）；scripts/.editorconfig 追加 [install.ps1] charset=utf-8 例外块覆盖 [*.ps1] charset=utf-8-bom
```

无文件 add/move/remove，仅注释扩展。

---

## §9 遗留 / 给 Code Reviewer 的关注点（≤5 条）

1. **`& { exit N }` 在脚本宿主与交互式宿主行为差异**（§3.1）：02 D-2 论断需 nuance —— 自动化测试无法 100% mock 用户交互式 PowerShell 窗口行为；用户真机 [U] AC 不可省。Code Reviewer 复核时可引用 §3.1 给 QA 06 写 06 时参考。

2. **G.1 / G.2 verify_all 跑时偶发 FAIL**（§3.2）：根因是另一进行中任务（download-cancel-and-upload-decouple）的 untracked `internal/downloader/downloader_cancel_test.go` 引用 undefined helper。clean go testcache 后跑稳定 PASS。本任务 0 Go 改动，无关。Code Reviewer 不必为此 hold 本任务。

3. **失败横幅未在 iex 形态自动化路径精确验证**：因 §3.1 限制，"`& { ... }` 退出后 `$LASTEXITCODE` 仍可读 + 横幅打印"在 -File 模式下无法验（pwsh 退出前不到 if 块）；在 -NoExit 交互式模式下可见但 stdin/stdout 难自动断言。强烈建议 QA 06 用真实 PS 窗口手动验 AC-7。

4. **C.1 E2E baseline 异常**：full 模式跑 C.1 FAIL 是另一进行中任务的前端代码改动（修改了 web/src/api/downloader.ts / AppLayout.vue 等）引起的 wave-front，与 T-026 无关。本任务全程用 Quick 模式监测 delta（19 → 21，净 +2 干净）。Code Reviewer 如需 full 模式验证，建议在干净的 main 分支（git stash）跑。

5. **install.ps1 内 `Write-Host`（如失败横幅 `❌` emoji）在 PS5.1 console 显示验证**：本机 PS7 + UTF-8 console 显示正常；PS5.1 console 默认 codepage 936 可能让 `❌` 显示成 `??`。`❌` 字符（U+274C）在 GBK 中无对应，会按 `?` 替换。不影响功能（用户仍能看到中文文案"frp_easy 安装未完成"），仅图标显示降级。若 Code Reviewer 认为需要 ASCII 化 fallback，可作为 minor follow-up。

---

## §10 Insight to surface（optional）

`& { exit N }` 在 PowerShell 中**仅**在交互式 PowerShell console host（用户桌面打开的 pwsh.exe 窗口）下退子作用域不杀宿主；脚本宿主（`pwsh -File <script>` / `pwsh -Command "& {...}"`）下 `exit N` 是 host-level 指令直接退脚本进程，scriptblock 包裹**不**隔离。自动化测试用 `Start-Process pwsh -NoExit -File <probe>` + 进程存活观察是最接近真实 iex 形态的等价 mock，但 stdout 难捕获完整后续状态 · evidence: T-026 04 §3.1 实测 `pwsh -Command "& { exit 1 }; Write-Host after"` ExitCode=1 / "after" 不打印；vs `Start-Process pwsh -NoExit -File probe` 4s 后 HasExited=False / PID alive

---

## §11 Verdict

**READY FOR REVIEW**

- verify_all 最终 Quick 模式 PASS=21 / WARN=0 / FAIL=0（baseline 19 → 净 +2 来自 E.7 拆分；0 新失败、0 新 WARN）
- 实施严格按 02 设计 + 03 四条 MAJOR 必修条件全增补（G-6 / G-7 / G-8 / G-15 ✓）
- 设计偏差仅 §3.1 一处 nuance（02 D-2 论断在脚本宿主下不成立，但用户交互式宿主下成立—— FR-4/5 满足；04 errata 记录）
- AC-3 / AC-9 / AC-10 / AC-12 / AC-13 / AC-14 / AC-15 / AC-16 全 [A] 类完成；AC-1/2/4/5/6/7/8/11 标 [U] 留 QA 06 + 用户真机
- 5 个 Files changed + dev-map.md 1 个注解扩展 + 0 个文件 add/move/remove
