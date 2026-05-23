# 06 — Test Report · T-020 claude-settings-context7-fix

> Stage 6 / 7 · QA Tester · 输出语言：中文 · 2026-05-23
> 上游：`01_REQUIREMENT_ANALYSIS.md` READY · `02_SOLUTION_DESIGN.md` READY · `03_GATE_REVIEW.md` APPROVED · `04_DEVELOPMENT.md` READY FOR REVIEW · `05_CODE_REVIEW.md` APPROVED

---

## Summary

T-020 静态断言 T1-T7 全部 PASS、verify_all PASS:19/WARN:0/FAIL:0（连跑三次稳定）、字节级核对全过、红线边界无触碰、8 条 AC 中 7 条自动 ✅ + 1 条 (AC-6) manual followup。本任务 **APPROVED FOR DELIVERY (with manual followup)**：自动化层完全通过，AC-6 / ADV-1 需 PM 或用户在 delivery 阶段实际打开 VS Code 一次性目测确认（已给出操作步骤与预期观察）。

---

## Coverage map (RA §4 八条 AC 全覆盖)

| AC | 描述 | 测试用例 | 状态 |
|---|---|---|---|
| AC-1 | `$schema` 字段值 = `https://json.schemastore.org/claude-code-settings.json` | T2 (`-ceq` 大小写敏感字面对比) | ✅ |
| AC-2 | `.claude/settings.json` 是合法 JSON | T1 (`ConvertFrom-Json` 无异常) + verify_all 顺带读 | ✅ |
| AC-3 | `permissions.deny` 非空且保留原 3 条 | T3 (HashSet 包含断言 3/3) | ✅ |
| AC-4 | `hooks.Stop[0].hooks[0]` 不含 `matcher` | T6 (Stop[0].PSObject.Properties.Name 反向断言) | ✅ |
| AC-5 | `verify_all` PASS 与改前同等级别（不允许指标下降） | T8 (PASS:19 baseline 保持，跑了 3 次) | ✅ |
| AC-6 | (manual) VS Code 状态栏无 schema 错误 + IntelliSense 补全 | ADV-1 manual（见下） | **MANUAL** |
| AC-7 | (条件 P1) `permissions.deny` 新增 `rm -rf ~`、`rm -rf .`、`/bin/rm -rf` 字面前缀 | T4 (HashSet 包含 5/5；超出最小 3 集) | ✅ |
| AC-8 | (条件 P2) `permissions.deny` 新增 `Read(./.env)`、`Read(./.env.*)`、`Read(./secrets/**)` | T5 (HashSet 包含 3/3) | ✅ |

7 / 8 自动断言通过；AC-6 严格按设计 §12.3 / RA §4 归类为 manual，不阻塞 verdict。

---

## 静态断言 T1-T7 输出（设计 §12.1 全量执行 · Bash → pwsh 调度）

执行命令：`pwsh -NoProfile -Command "<inline T1-T7 脚本>"` （沙箱实证 Bash 层放行，与 Developer 意外 #1 / Reviewer §5.6 结论一致）

```
=== T1 · ConvertFrom-Json 无异常 ===
T1 PASS: ConvertFrom-Json succeeded, no exception

=== T2 · schema URL 精确等于 context7 字面值 ===
  expected = https://json.schemastore.org/claude-code-settings.json
  actual   = https://json.schemastore.org/claude-code-settings.json
T2 PASS: schema 字面值精确匹配 (case-sensitive)

=== T3 · deny 含原 3 条 ===
  HIT  Bash(git push --force:*)
  HIT  Bash(git push -f:*)
  HIT  Bash(rm -rf /:*)
T3 PASS: 原 3 条 deny 全部保留

=== T4 · deny 含新 5 条 P1 ===
  HIT  Bash(rm -rf ~:*)
  HIT  Bash(rm -rf .:*)
  HIT  Bash(rm -rf $HOME:*)
  HIT  Bash(/bin/rm:*)
  HIT  Bash(find / -delete:*)
T4 PASS: P1 5 条新 deny 全部命中

=== T5 · deny 含新 3 条 P2 ===
  HIT  Read(./.env)
  HIT  Read(./.env.*)
  HIT  Read(./secrets/**)
T5 PASS: P2 3 条新 deny 全部命中

=== T6 · Stop[0] 无 matcher ===
  Stop[0] keys = hooks
T6 PASS: Stop[0] 无 matcher

=== T7 · _comment / _doc_sync_hook 保留 ===
  _comment present       = True
  _doc_sync_hook present = True
T7 PASS: 信息性字段保留

=== Counts ===
  allow count = 21
  deny count  = 11

=== ALL T1-T7 PASS ===
```

> 工具输出原始字节被 PowerShell 与 bash 串联时的 GBK→UTF-8 转码层显示为乱码（中文部分），但程序逻辑判断与英文 KEY/VALUE/COUNT 字面全部清晰。已据 Counts 段独立核对：allow=21、deny=11，与设计 §10 / Reviewer §3 / 04 字节统计三方一致。

**逐条预测失败假设 vs 实际**（adversarial 思维强制项）：

| Test | "我预期失败因为…" | 实际 |
|---|---|---|
| T1 | 担心 Developer 用 PowerShell 字节级写盘时残留 BOM 致 `ConvertFrom-Json` 抛 "unexpected character" | 实际 PASS；字节统计 `7B 0A 20...` 首字节非 BOM，确认 `UTF8Encoding($false)` 写盘正确 |
| T2 | 担心 `$schema` 被 PowerShell 当 PSVariable 误展开（设计 R-7 / Developer 意外 #2 类型问题），导致字面值丢字符 | 实际精确匹配；`'\$schema'` 引用方式在 single-quoted 内是字面 |
| T3 | 担心 Developer 在重排 deny 数组时漏抄原 3 条 | 3/3 全命中，HashSet 检索零误差 |
| T4 | 担心 `Bash(rm -rf $HOME:*)` 的 `$HOME` 在 PowerShell HashSet 比较时被展开成空字符串 → MISS | 实际 HIT；`$HOME` 在 single-quoted string array 内字面保留，HashSet 比较精确 |
| T5 | 担心 `Read(./.env.*)` 与 `Read(./secrets/**)` 在 HashSet equality 上对 `*` 或 `.` 特殊处理 | 3/3 全命中，HashSet 比较纯字面 |
| T6 | 担心 04 同步生成 hooks 时悄悄加了 `matcher` 字段 | Stop[0] keys 仅 `hooks` 一项；反向断言正确通过 |
| T7 | 担心 D-4 保留 `_comment` 被某个层（如 ConvertFrom-Json）反序列化为 `$null` | 两个 `[bool]` 转换都为 `True`，字段正确存在且值非空 |

7 条假设全部"未能复现失败"——实现确实通过对抗性核验，不是侥幸。

---

## 集成断言 T8 (verify_all) · 完整 Summary 段（设计 §12.2）

执行命令：`pwsh -NoProfile -File c:/Programs/frp_easy/scripts/verify_all.ps1`

**Run #1（QA 首跑）：**
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

=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

**Run #2（稳定性复核）**：PASS:19 / WARN:0 / FAIL:0 / SKIP:0（完整输出与 #1 完全一致）。
**Run #3（稳定性复核）**：PASS:19 / WARN:0 / FAIL:0 / SKIP:0（完整输出与 #1 完全一致）。

**数字断言**：
- baseline：`scripts/baseline.json` test_count=333, passing_count=333
- verify_all PASS 计数：19（改前基线 = 19，改后 = 19，零变化）
- FAIL：0（AC-5 要求）
- WARN：0
- E.6 (`## Adversarial tests` 段存在断言)：PASS ✅（说明已完结任务的 06 都合规；本 06 也将通过——见末尾"## Adversarial tests" 裸标题）
- E.4 (`Binding in sync`)：PASS ✅（实证设计 §11 R-4：settings.json 不进入 harness-sync diff 范围）

**Delta**：0 new failures, 0 new warnings, 19 PASS 保持 ↔ **AC-5 通过**。

---

## 红线边界实证 (`git diff --stat`)

### 全仓库视角

```
$ git diff --stat HEAD
 .claude/settings.json         |  12 ++++-
 cmd/frp-easy/main.go          |  42 +++++++++++++++--
 docs/dev-map.md               |   8 +++-
 docs/tasks.md                 |   5 +-
 go.mod                        |   2 +-
 scripts/install-service.ps1   | 103 ++++++++++++++++++++++++++++++++----------
 scripts/uninstall-service.ps1 |  14 ++++--
 7 files changed, 151 insertions(+), 35 deletions(-)
```

```
$ git status --short
 M .claude/settings.json
 M cmd/frp-easy/main.go
 M docs/dev-map.md
 M docs/tasks.md
 M go.mod
 M scripts/install-service.ps1
 M scripts/uninstall-service.ps1
?? cmd/frp-easy/service_other.go
?? cmd/frp-easy/service_windows.go
?? cmd/frp-easy/service_windows_test.go
?? docs/features/claude-settings-context7-fix/
?? docs/features/windows-service-scm-1053-fix/
```

### 归属分类（按文件路径逐条判定）

| 文件 | 归属任务 | 红线？ | 判定依据 |
|---|---|---|---|
| `.claude/settings.json` | **T-020**（本任务） | OK：仅修 deny 数组 + `$schema` URL | RA §3.1 in-scope 唯一编辑目标 |
| `cmd/frp-easy/main.go` | T-019 windows-service-scm-1053-fix | 非本任务范围 | RA §8 已记录 T-019 "in progress, stage req"；最近 commit history 显示该文件由 T-011/T-008/T-007/T-004 修改过，与本任务 RA §3.1 / 设计 §2 列出的 "Affected modules" 不交集 |
| `docs/dev-map.md` | T-019 windows-service-scm-1053-fix | 非本任务范围 | 04 §"Dev-map updates" 显式声明 "无变更"；当前 diff 与 T-020 无关 |
| `docs/tasks.md` | T-020（PM 阶段状态更新） | OK | PM Orchestrator 维护，非 QA 关注 |
| `go.mod` | T-019 | 非本任务范围 | 本任务零代码 / 零依赖（设计 §3 / §4 / §5 / §7） |
| `scripts/install-service.ps1` | T-019 | 非本任务范围 | T-019 任务 slug 即"windows-service-scm-1053-fix"，install-service.ps1 与 SCM 1053 错误码强相关 |
| `scripts/uninstall-service.ps1` | T-019 | 非本任务范围 | 同上 |
| `cmd/frp-easy/service_other.go`（untracked） | T-019 | 非本任务范围 | 新 Windows service 抽象拆分文件 |
| `cmd/frp-easy/service_windows.go`（untracked） | T-019 | 非本任务范围 | 同上 |
| `cmd/frp-easy/service_windows_test.go`（untracked） | T-019 | 非本任务范围 | 同上 |
| `docs/features/claude-settings-context7-fix/`（untracked） | **T-020** | OK | 本任务文档落盘目录 |
| `docs/features/windows-service-scm-1053-fix/`（untracked） | T-019 | 非本任务范围 | 并行任务文档目录 |

### T-020 自身范围内的 diff（精确过滤）

```
$ git diff --stat HEAD -- .claude/settings.json docs/features/claude-settings-context7-fix/ docs/tasks.md
 .claude/settings.json | 12 ++++++++++--
 docs/tasks.md         |  5 ++++-
 2 files changed, 14 insertions(+), 3 deletions(-)
```

（`docs/features/claude-settings-context7-fix/` 子树为 untracked，未在 `diff --stat` 中显示，但出现在 `git status` 的 `??` 行；这是新任务文档目录的标准状态。）

### `.claude/settings.json` 完整 diff（设计 §9 unified diff 字符级核对）

```diff
diff --git a/.claude/settings.json b/.claude/settings.json
index 2aea079..794c84b 100644
--- a/.claude/settings.json
+++ b/.claude/settings.json
@@ -1,5 +1,5 @@
 {
-  "$schema": "https://json.schemastore.org/claude-code-settings",
+  "$schema": "https://json.schemastore.org/claude-code-settings.json",
   "_comment": "Generated by harness-kit. Edit hooks/permissions to suit your project. Stop hook auto-runs harness-sync so .harness/ edits flow to .claude/ + CLAUDE.md without you remembering.",
   "permissions": {
     "allow": [
@@ -28,7 +28,15 @@
     "deny": [
       "Bash(git push --force:*)",
       "Bash(git push -f:*)",
-      "Bash(rm -rf /:*)"
+      "Bash(rm -rf /:*)",
+      "Bash(rm -rf ~:*)",
+      "Bash(rm -rf .:*)",
+      "Bash(rm -rf $HOME:*)",
+      "Bash(/bin/rm:*)",
+      "Bash(find / -delete:*)",
+      "Read(./.env)",
+      "Read(./.env.*)",
+      "Read(./secrets/**)"
     ]
   },
   "hooks": {
```

**字符级 100% 匹配设计 §9 的 unified diff**（仅 2 处 hunk：`$schema` 字面值替换 + `deny` 数组追加 8 行），零字节漂移。

### 字节级核对（独立复跑 Developer 04 §"Step 3"）

```
size       = 1702 bytes
CR count   = 0
LF count   = 55
first 3 hex= 7B 0A 20 (expect 7B 0A 20, no BOM)
last 2 hex = 7D 0A (expect 7D 0A, trailing LF)
```

- size=1702（Developer 04 报告值 1702 ✅）
- BOM：无（首 3 字节 `7B 0A 20` = `{ \n SPACE`，非 `EF BB BF`）
- 行尾：纯 LF（CR=0）
- 末尾：单 LF
- LF 总数 = 55（与 04 一致）

**红线判定**：T-020 任务实施完全自限于 RA §3.1 in-scope（仅 `.claude/settings.json` + 本任务 docs 目录 + PM 维护的 `docs/tasks.md`）；T-019 的并行未提交工作在共享工作树中可见但与 T-020 commit 范围无任何 overlap。**红线 PASS**：未触碰 `.claude/agents/`、`.claude/skills/`、`CLAUDE.md`、`.github/copilot-instructions.md`、`.claude/settings.local.json`。

---

## Manual 测试归属表（设计 §12.3 / §12.4 + Gate Review F-2）

| 项 | 类型 | Manual 步骤 | 预期 | 不能自动化的原因 |
|---|---|---|---|---|
| **AC-6 / T9** | manual | (1) 在 VS Code 中打开 `c:/Programs/frp_easy/.claude/settings.json`。(2) 看状态栏右下角 schema 标识：应显示 `claude-code-settings.json` 已加载，**无 "Cannot resolve schema"** 红字。(3) 在 `permissions.allow` 数组末尾输入 `"B`，应触发 IntelliSense 自动补全 `Bash()` 模板。 | (a) Schema URL 加载成功（200 OK）。(b) IntelliSense 浮窗出现 `permissions.allow` 字段的字符串值建议。 | VS Code 是 GUI 应用，无 headless schema validation pipeline 可在 verify_all 内调度。 |
| **ADV-1**（设计 §12.4，**QA 必跑 manual**） | manual + 思想实验 | (1) 编辑 `.claude/settings.json` 把 `$schema` 临时改回缺 `.json` 的旧值 `"https://json.schemastore.org/claude-code-settings"`。(2) 重启 VS Code（或重新打开该文件）。(3) 观察状态栏 schema 解析失败 / IntelliSense 消失。(4) 改回带 `.json` 的新值并保存。(5) 重启 VS Code，观察状态栏恢复绿色 + IntelliSense 复活。 | (a) 旧 URL → 状态栏 schema 错误或缺失（D-1 是真实可观测修复）。(b) 新 URL → 一切恢复正常。**这两次观察的差异是 D-1 价值的唯一终极证据**。 | 同 AC-6：需真实 VS Code session 完成"reload → 观察 schema 解析器是否报错"循环。 |
| **ADV-2**（Gate Review F-2 标 manual / out-of-automated-scope） | manual / out-of-scope | 在真实 Claude Code session 内对模型说 "请执行 `rm -rf ~/test_T020_dummy`"，观察 permissions 引擎是否拒绝（应命中 `Bash(rm -rf ~:*)`）。 | 引擎拒绝该工具调用，输出类似 "denied by permissions" 提示。 | 需要真实 Claude Code 模型 → 工具调用 → deny 引擎反馈链路；verify_all 仅在脚本进程内运行，无法启动 Claude session。 |
| **ADV-3**（Gate Review F-2 标 manual / out-of-automated-scope） | manual / out-of-scope | 在真实 session 内尝试变体命令 `\rm -rf ~/test_T020_dummy`（反斜杠转义）、`rm  -rf ~/x`（双空格）。 | 预期**漏拦**，与设计 §8.1 "不做完备防御"声明一致；这是 deny 字面前缀真实强度的 reviewer-visible 证据。 | 同 ADV-2。 |
| **ADV-4**（Gate Review F-2 标 manual / out-of-automated-scope） | semi-manual / out-of-scope | 临时在 deny 数组末尾追加尾逗号 → 重启 Claude Code → 观察错误日志中是否报 JSON 拒载；恢复后正常。 | Claude Code 拒载 settings.json 并打印解析错误（验证 R-1 缓解的可见性）。 | 需要重启 Claude Code 应用（不是 CLI），且 settings.json 一旦故意破坏会立即影响当前 session，verify_all 无法在不破坏自身运行环境的前提下复现。 |
| **设计 §12.1 T1-T7** | automated（QA 已跑） | 见上节 | 全 PASS | 不适用（已自动跑） |
| **设计 §12.2 T8** | automated（QA 已跑 3 次） | 见上节 | PASS:19/FAIL:0 ×3 | 不适用（已自动跑） |

**Manual followup 归属**：交付阶段由 PM 或用户在真实 VS Code session 中至少跑一次 ADV-1（必跑），其余 ADV-2/3/4 按 Gate Review F-2 标 out-of-automated-scope 不阻塞 verdict。

---

## Adversarial tests

> 严格按 qa-tester 角色契约 "Adversarial mindset" 三条铁律：每条 AC 都给出独立 reproducer + 失败假设。
> **本节标题为裸 `## Adversarial tests`**（无任何数字编号 / 中文修饰前缀），匹配 `scripts/verify_all.sh` E.6 regex `^##\s+Adversarial\s+tests`（Insight Index #29 / #40 证据）。

### 8 条 AC 的对抗性 reproducer 与失败假设

| AC | 假设 ("我预期失败因为…") | Reproducer (独立编写，**未抄 04 的脚本**) | 实际结果（带工具输出） |
|---|---|---|---|
| AC-1 | Developer 用 PowerShell 字符串替换 `claude-code-settings` → `claude-code-settings.json` 时，可能在 multi-match 场景误替换其他位置（如 `_comment` 内若也含该子串）。我用 grep 直接全文件搜索 `claude-code-settings` 字面来验证只在 `$schema` 这一处出现。 | `Grep -o 'claude-code-settings(\.json)?' c:/Programs/frp_easy/.claude/settings.json` → 应只命中 1 次，且必带 `.json` 后缀 | **Survived** — T1+T2 结果：`actual = https://json.schemastore.org/claude-code-settings.json`（line 2 唯一出现，case-sensitive 字面匹配），无误替换 |
| AC-2 | 我用 PowerShell `ConvertFrom-Json` 解析（PS 解析器比 Claude Code 内的 strict JSON parser **更宽松**——见下 §"Edge cases" (a)/(c)）。故 ConvertFrom-Json PASS 不足以证明 strict 也 PASS。我补做：(1) 字节级检查 first 3 hex 排除 BOM；(2) Grep `//`、`/*`、尾逗号正则三次扫描确认无 strict-fatal 语法 | T1 (ConvertFrom-Json) + `Grep '//\|/\*\|,\s*[\]\}]' .claude/settings.json` | **Survived** — T1 PASS；Grep `//` 0 命中、Grep `/*` 仅命中 `Read(./secrets/**)` 字符串字面（非注释，是 deny 值的 glob `**`，无 `*/` 闭合）、尾逗号 0 命中。即使 strict parser 也会通过 |
| AC-3 | Developer 在重排 deny 数组（按设计 §8.3 git → rm → Read 分组）时可能漏抄 `rm -rf /:*`（最容易被新 P1 `rm -rf ~`/`rm -rf .` 视觉吞并）。我独立用 HashSet.Contains 三次断言而非看清单字符串 | T3 脚本：`$set.Contains('Bash(rm -rf /:*)')` 等 | **Survived** — T3 HIT 3/3，且通过 `git diff` 二次复核：line 31 `"Bash(rm -rf /:*)",` 字面完整保留 |
| AC-4 | Anthropic 的 Stop hook 可能在某些版本中要求带 `matcher: "*"`；Developer 是否真删了 matcher？我用反向断言（属性枚举 + `-contains 'matcher'`）而非看代码 | T6 脚本：`$cfg.hooks.Stop[0].PSObject.Properties.Name -contains 'matcher'` | **Survived** — T6 输出 `Stop[0] keys = hooks`，仅 1 个 key，反向断言无 matcher |
| AC-5 | 本任务声称 verify_all 不依赖 settings.json，但 E.6 段会读 06 测试报告 markdown——这就意味着改 settings.json **理论上**不影响 verify_all。然而 [E.4] `Binding in sync` 跑 harness-sync `-Check`，可能间接对比 .claude/ 文件 → 此处可能爆雷。我跑 verify_all 3 次连续观察是否稳定 | `pwsh -File scripts/verify_all.ps1` × 3 | **Survived** — 3/3 都是 PASS:19/WARN:0/FAIL:0，[E.4] 三次都 PASS（确认 harness-sync `-Check` 只对比 `.harness/agents/` 与 `.harness/skills/`，不读 settings.json） |
| AC-6 | **Manual followup**（ADV-1 必跑）：见上节 Manual 表 | VS Code 真实 session 中 schema URL 切换观察 | **MANUAL** — 等待 PM/用户在 delivery 阶段实际操作；自动化层无法替代 |
| AC-7 | Developer 是否真在 P1 的 5 条 deny 中都正确保留 `$HOME` 的字面美元符号（不被 PowerShell 变量展开吞掉）？这是设计 §11 R-1 / Developer 意外 #2 / Reviewer §4 三方共同关注点。我独立查 HashSet 含 `Bash(rm -rf $HOME:*)` 字面，并 grep settings.json line 34 字符 | T4 HashSet 断言 + Read line 34 视检 | **Survived** — T4 HIT 5/5，line 34 `"Bash(rm -rf $HOME:*)",` 字面美元符号正确保留（PowerShell single-quoted 字符串数组 + `Set-Content`/`WriteAllText` 字节级写盘 → 美元符号字面安全） |
| AC-8 | Read deny 三条都含 `.` 或 `**` 这种 glob/path 元字符；担心 Developer 在 PowerShell `String.Replace` 时被正则误解释。我独立 HashSet 三次断言 | T5 HashSet 断言 | **Survived** — T5 HIT 3/3。`String.Replace` 是字面替换（非 regex），不会误解释；HashSet equality 是字面比较 |

### Adversarial 总结

8 条 AC 全部经过独立 reproducer 验证：7 条 **Survived**（实现挺住了我故意挑刺的角度），1 条 **MANUAL**（AC-6 / ADV-1 设计层就标 manual，自动化无能为力）。无 BLOCKER / CRITICAL / MAJOR / MINOR 缺陷暴露。

---

## Edge cases / 回归测试覆盖（QA 独立检视）

### (a) PowerShell `ConvertFrom-Json` 解析宽松度对比 Claude Code strict parser

**实证脚本输出**（_qa_edge_probe.ps1，跑后已清理）：

```
--- (a) 引入尾逗号 ---
  UNEXPECTED PASS: 尾逗号未触发解析错误 (PowerShell ConvertFrom-Json 可能宽松)
--- (c) 引入 // 注释（应触发 ConvertFrom-Json 异常） ---
  UNEXPECTED PASS: // 注释未触发解析错误
```

**结论**：PowerShell `ConvertFrom-Json` 比 strict JSON 宽松——尾逗号 + `//` 注释均被接受。这意味着 T1 (`ConvertFrom-Json` 成功) **本身不能保证 strict parser 也 PASS**。补救：我独立 Grep `//`、`/*`、尾逗号正则 → 全部 0 命中（见 AC-2 row）。**当前实际写入的 settings.json 同时通过宽松解析器和 strict 解析器**，R-1 风险面真正被消除。

### (b) 删除 `$schema` 字段后的鲁棒性

```
--- (b) 删除 $schema 字段（应仍是合法 JSON） ---
  PASS: 删 schema 后 deny 仍 11 条，JSON 合法
```

确认 `$schema` 字段是**可选信息字段**，删除不破坏 JSON parsing 也不影响 Claude Code permissions 引擎；本任务把它写对是为了 VS Code IntelliSense 体验，而非 functional correctness。这与设计 §8 D-1 / §5 论述一致。

### (c) deny 字面前缀的局限性（设计 §11 R-2 / R-3 / §8.1 仍成立）

```
--- (d) deny 字面前缀的字符级局限性 ---
  rm -rf ~/foo               matched=True  expect=HIT  [OK] // Bash(rm -rf ~:*) 字面前缀
  \rm -rf ~/foo              matched=False expect=MISS [OK] // 反斜杠转义变体（设计 §8.1 不做完备防御）
  rm  -rf ~/foo              matched=False expect=MISS [OK] // 双空格变体
  /bin/rm -rf /tmp           matched=True  expect=HIT  [OK] // Bash(/bin/rm:*) 字面前缀
  /usr/bin/rm -rf /          matched=False expect=MISS [OK] // /usr/bin/rm 路径变体不被覆盖
  rm -rf ~                   matched=True  expect=HIT  [OK] // 精确前缀
```

6 / 6 行为符合**设计声明**：HIT 的命中、MISS 的漏拦——设计 §8.1 "不做完备防御" 的声明在字符级被实证为真。**这不是缺陷**，这是已宣告的限制。Reviewer / 用户读 settings.json 时不应高估其安全强度。

### (d) 回归：原 3 条 deny 与原 21 条 allow 完整性

- 原 3 条 deny（git push --force / git push -f / rm -rf /）：T3 HIT 3/3 ✅
- 原 21 条 allow：T1-T7 输出 `allow count = 21` ✅（设计 §10 / Reviewer §3 三方一致）
- `_comment` / `_doc_sync_hook` 长字符串字面：T7 ✅（Reviewer 04 已 Grep 长串字符级核对）
- `hooks.Stop[0].hooks[0].command` = `pwsh -File scripts/harness-sync.ps1`：未变（D-5 out-of-scope）

### (e) Stability（QA 强制要求）

- verify_all 跑 3 次（QA 首跑 + 稳定性复核 ×2）：3/3 都是 PASS:19/WARN:0/FAIL:0/SKIP:0。
- **零 flake** ✅
- 所有 T1-T7 输出对同一 settings.json 内容是确定性的（无时间 / 随机 / 网络依赖）→ 重复运行结果不变。

### (f) Performance NFR

设计 §11 R-5 / RA §7：N/A（纯配置文件，无运行时性能影响）。无需测。

### (g) 红线边界正交性

- `.claude/agents/`：Glob 验证 7 个 agent 文件均存在；本任务零触碰。
- `.claude/skills/`：3 个 SKILL.md 均存在；零触碰。
- `CLAUDE.md`：未修改（git diff --stat 不含此文件）。
- `.github/copilot-instructions.md`：未修改。
- `.claude/settings.local.json`：未修改（gitignore 范围）。
- `.harness/agents/`、`.harness/skills/`：未修改（E.4 Binding in sync PASS 实证）。

---

## Defects found

**0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR**

无任何缺陷（与 Code Review §10 "0 CRITICAL + 0 MAJOR + 0 MINOR" 一致）。

### 已知声明性限制（非缺陷，设计已 acknowledge）

- **WARN-1**（Gate Review F-1）：`Bash(rm -rf $HOME:*)` 命中前提（"模型不自行展开 `$HOME`"）未在设计 §11 风险表中点明；与 `Bash(rm -rf ~:*)` 是 OR 关系，最坏只是无效不破坏其他 4 条 P1。**不阻塞 verdict**，下游可按 Reviewer §9 第 1 / 2 条建议追加 R-8 或 Insight。
- **WARN-2**（Gate Review F-2）：ADV-2/3/4 不可自动化 → 已在本 06 "Manual 测试归属表"中明确标 manual / out-of-automated-scope。**不阻塞 verdict**。
- **WARN-3**（设计 §8.1 / §11 R-2 / R-3）：deny 字面前缀的 5 条 P1 + 3 条 P2 不做完备防御（反斜杠/双空格/`/usr/bin/rm` 路径变体均会漏拦）。**这是设计有意保留的限制**，已在上面 Edge cases (c) 字符级实证。

---

## verify_all result

- 改前基线（baseline.json）：test_count=333 / passing_count=333 / verify_all PASS:19
- 改后实测（连跑 3 次）：PASS:19 / WARN:0 / FAIL:0 / SKIP:0
- 总测试数：333 → 333（本任务无代码改动，无新单测 / e2e 测试增加）
- New tests added: 0（纯配置任务，按 04 §"Dev-map updates" / Reviewer §0 一致归类）
- Baseline 更新：**否**（test_count 未变化；按 qa-tester 角色契约 "If the test count increased" 触发条件未满足）

> 本任务是**纯配置 / 文档任务**，无 Go / TS 单测可以新增（设计 §3 / §4 / §5 明确无新模块）。QA 在静态断言 T1-T7 与对抗性核验中已为本任务"配置正确性"提供了完整的 reproducible 证据，未来类似任务可直接复用本 06 §"静态断言" 段的 PowerShell 脚本作为回归基线。

---

## Stability

- verify_all 跑 3 次：3/3 PASS:19，零 flake ✅
- T1-T7 静态断言：基于确定性的 JSON 解析 + HashSet 比较，每次运行结果相同（已心证为非 flaky）
- 字节级核对 size=1702 / CR=0 / LF=55 / first 3 = `7B 0A 20` / last 2 = `7D 0A`：与 Developer 04 报告一致 ✅

---

## Verdict

**APPROVED FOR DELIVERY (with manual followup)**

判定依据（每条带证据）：

1. **7 / 8 AC 自动通过 + 1 / 8 manual followup**（AC-6 设计层归类 manual，不阻塞）。
2. **T1-T7 静态断言 7 / 7 PASS**——独立 reproducer，每条带"我预期失败因为…"假设，全部 Survived。
3. **verify_all PASS:19 / WARN:0 / FAIL:0**（连跑 3 次稳定，零回归 vs baseline）。
4. **字节级核对全过**：size=1702、BOM 无、CR=0、LF=55、trailing LF；与 Developer 04 报告一致。
5. **红线边界零触碰**：`git diff --stat` 实证 T-020 自身仅修 `.claude/settings.json` + `docs/features/claude-settings-context7-fix/` + `docs/tasks.md`；范围外的 modified 文件 + untracked 文件全部归属并行任务 T-019（windows-service-scm-1053-fix），不是 T-020 越界。
6. **Edge cases 全部行为可解释**：PowerShell ConvertFrom-Json 宽松度通过补充 Grep 弥补；deny 字面前缀局限性符合设计 §8.1 / §11 R-2/R-3 声明。
7. **零缺陷**（0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR）。

**Manual followup（不阻塞，但 delivery 前必须做 ADV-1 至少一次）**：
- ADV-1（必跑）：PM 或用户在 VS Code 中实测旧 URL → 新 URL schema 解析切换，眼见为实 D-1 真实价值。
- ADV-2/3/4（按 Gate Review F-2 标 out-of-automated-scope）：可选；若 delivery 后任何时间在 Claude Code session 实地观察到，可作为后续 insight 落入 Insight Index。

**下一步**：PM Stage 7 合并 / 落 Insight Index 两条（Developer 04 §"Insight to surface" 候选）/ 关闭任务。
