# Test Report — T-005 docs-and-api-schema

**任务**：T-005  
**测试日期**：2026-05-16  
**QA Tester**：qa-tester agent

---

## Test plan

| 验收标准 | 测试用例 | 验证方法 | 结果 |
|---|---|---|---|
| AC-A1：README 不含 "TD-1 ～ TD-8" | `grep -c "TD-1 ～ TD-8" README.md` → 0 | CLI grep | PASS |
| AC-A2：README 不将"向导路由守卫漏洞"列为当前问题 | `grep -c "向导路由守卫漏洞" README.md` → 0 | CLI grep | PASS |
| AC-A3：README 不将"verify_all 前端检查路径"列为当前问题 | `grep -c "verify_all 前端检查路径" README.md` → 0 | CLI grep | PASS |
| AC-A4：project-status.html TD-1～TD-7 已修复标记 ≥ 7 | `grep -c "已修复" docs/project-status.html` → 8 | CLI grep | PASS |
| AC-A5：project-status.html 含数字 119 | `grep -c "119" docs/project-status.html` → 3 | CLI grep | PASS |
| AC-A6：project-status.html 不含 "ParseIPFromJSON" | `grep -c "ParseIPFromJSON" docs/project-status.html` → 0 | CLI grep | PASS |
| AC-B1：openapi.yaml 存在时 D.1 = PASS | `bash scripts/verify_all.sh --quick 2>&1 \| grep "D\.1"` 含 PASS | 运行脚本 | PASS |
| AC-B2：openapi.yaml 不存在时 D.1 = WARN | 临时重命名后运行 → WARN；恢复文件 | 运行脚本 | PASS |
| AC-B3：verify_all.sh D.1 块有 go.mod 注释 | `grep "go\.mod\|前置条件" scripts/verify_all.sh` 在 D.1 块内有 2 行注释 | 代码 + CLI grep | PASS |
| AC-B4：verify_all.ps1 D.1 块有 go.mod 检测逻辑 | `grep "go\.mod\|前置条件" scripts/verify_all.ps1` 在 D.1 块内有 2 行注释 + 检测语句 | 代码 + CLI grep | PASS |
| AC-C1：openapi.yaml 存在于根目录 | `test -f openapi.yaml && echo PASS` | CLI | PASS |
| AC-C2：operationId 数 = 28 | `grep -c "operationId:" openapi.yaml` → 28 | CLI grep | PASS |
| AC-C3：YAML 语法有效 | `npx -y js-yaml openapi.yaml` 无错误退出，输出完整 JSON | CLI js-yaml | PASS |
| AC-C4：/auth/login requestBody 含 username 字段 | `grep -A5 "LoginRequest:" openapi.yaml \| grep username` → 匹配 | CLI grep | PASS |
| AC-C5：/auth/me 200 response 含 username 字段 | `grep -A5 "MeResponse:" openapi.yaml \| grep username` → 匹配 | CLI grep | PASS |
| AC-C6：go.mod 未修改 | `git diff go.mod` 无输出 | git diff | PASS |
| AC-VERIFY：verify_all FAIL: 0 | `bash scripts/verify_all.sh --quick 2>&1 \| grep "FAIL:"` → `FAIL: 0` | 运行脚本 | PASS |

---

## Boundary tests added

本任务（T-005）为文档 + 配置更新，未引入新 Go 代码，无需新增单元测试。边界覆盖通过以下方式完成：

- **openapi.yaml 不存在时**：AC-B2 临时重命名验证 → D.1 WARN（非 SKIP、非 FAIL）
- **go.mod 不存在时**：verify_all.sh D.1 逻辑分支 `[[ ! -f go.mod ]]` 输出 SKIP（代码审查 + B/AC 一致性确认）
- **重命名后恢复**：AC-B2 测试完成后确认 openapi.yaml 已恢复，后续所有测试未受影响
- **并发稳定性**：verify_all 运行 3 次，结果完全一致（PASS: 17, WARN: 0, FAIL: 0, SKIP: 0 for --quick）

---

## Adversarial tests (REQUIRED, one per acceptance criterion)

### 假设 1：D.1 条件修改是否影响其他 verify_all 检查项

**假设方向（预测会失败）**：D.1 的前置条件从 `src/apps/packages` 改为 `go.mod`，如果代码引用了 Go 包 section 的逻辑，可能导致 G.1/G.2/G.3 或 B.1~B.4 出现意外 SKIP 或 FAIL。

**独立验证**：运行不带 `--quick` 的完整 verify_all，观察全部检查项输出。

```
bash scripts/verify_all.sh 2>&1
```

**实际输出**：

```
[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO/FIXME budget ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[C.1] E2E smoke (playwright) ... SKIP
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agents in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md ... PASS
[E.6] Adversarial tests section in completed task reports ... PASS

=== Summary ===
  PASS: 17
  WARN: 0
  FAIL: 0
  SKIP: 1
```

**结论**：D.1 修改未影响任何其他检查项。G.1/G.2/G.3 仍用独立 `go.mod` 检测（与 D.1 相同，但互相独立）；B.1~B.4 用 `web/package.json` 检测，完全不受 D.1 影响。假设**被证伪——实现通过**。

---

### 假设 2：openapi.yaml DownloadState.status 是否使用正确值（success/failed，非 done/error）

**假设方向（预测会失败）**：04_DEVELOPMENT.md 记录"设计文档草稿写 done/error，实际 Go 常量为 success/failed"，如果 openapi.yaml 使用了设计草稿的错误值（done/error），则与 Go 实现不一致，属于 CRITICAL 缺陷。

**独立验证**：

步骤 1 — 从 Go 源码获取权威值：
```
grep -n "StatusSuccess\|StatusFailed\|StatusDone\|StatusError" internal/downloader/downloader.go
```

输出：
```
34:	StatusSuccess     = "success"
35:	StatusFailed      = "failed"
```

步骤 2 — 检查 openapi.yaml DownloadState.status 描述：
```
grep -A8 "DownloadState:" openapi.yaml | grep "status\|idle\|success\|failed\|done\|error"
```

输出：
```
      required: [status, progress]
        status:
          description: "idle | downloading | success | failed"
```

**结论**：openapi.yaml 使用 `success | failed`，与 Go 源码 `StatusSuccess = "success"` 和 `StatusFailed = "failed"` 完全一致。假设**被证伪——实现通过**。

---

### 假设 3：project-status.html §7 是否仍包含 "ParseIPFromJSON" 的待处理条目

**假设方向（预测会失败）**：如果开发者只更新了 §7 的文字，但忘记同时清除 §5/§6 中的 ParseIPFromJSON 相关"未处理"标记（或 §7 仍有 table row），则 AC-A6 和 AC-A7 均应 FAIL。

**独立验证**：

步骤 1 — 全局搜索 ParseIPFromJSON：
```
grep -n "ParseIPFromJSON\|OPT-6\|IP 解析\|§7\|sec-7" docs/project-status.html
```

输出：
```
238:      <li><a href="#sec-7">§7 已知后续事项</a></li>
411:            <td><strong>IP 解析重复代码</strong>：...
526:            <td><strong>OPT-6</strong></td>
527:            <td><strong>统一 IP 解析实现</strong></td>
557:    <!-- §7 已知后续事项 -->
558:    <section class="section" id="sec-7">
559:      <h2>§7 已知后续事项</h2>
560:      <p>T-004 已清偿全部已知后续事项（OPT-2 向导路由守卫已修复，OPT-6 IP 解析重复代码已统一）。当前无待处理后续事项。</p>
```

步骤 2 — 确认 §5 中 ParseIPFromJSON 相关的 TD-2 行有已修复标记：
```
grep -c "已修复" docs/project-status.html
```
输出：`8`（TD-1～TD-7 共 7 行"已修复"，加上"已修复"概述段落共 8 处）

**结论**：`ParseIPFromJSON` 字符串不在任何"待处理"上下文中出现（§7 已是空表格并附文字说明），TD-2（IP 解析重复代码）在 §5 中标注"已修复（T-004）"，OPT-6 在 §6 中标注"已实现（T-004）"，§7 中已无 table 行。假设**被证伪——实现通过**。

---

### 假设 4：README 技术债章节是否仍列出待解决问题

**假设方向（预测会失败）**：如果开发者仅新增了"已清偿"行但未移除原有的"待解决"示例文字，README 可能仍存在旧的问题描述（如"向导路由守卫漏洞"、"ParseIPFromJSON 重复"），造成信息混淆。

**独立验证**：

步骤 1 — 提取完整技术债章节：
```
grep -A30 "技术债与优化建议" README.md
```

输出：
```
## 技术债与优化建议

| 批次 | 状态 | 内容 |
|---|---|---|
| TD-1 ～ TD-7 | ✅ T-004 已清偿 | 向导路由守卫、verify_all 前端门禁、slog 双写、版本注入、ParseIPFromJSON 统一、/health 端点、TOML 预检 |
| TD-8 | 保留（正确设计） | SQLite 单连接——这是 SQLite 的推荐用法，非债务 |
| OPT-1 ～ OPT-8 | ✅ T-004 已实现 | 见上方 TD 清单 |
| OPT-9 | ✅ T-005 已实现 | OpenAPI 3.0.3 schema（`openapi.yaml`，28 条路由） |
```

步骤 2 — 搜索"当前已知问题"或类似无修复状态的表述：
```
grep -c "向导路由守卫漏洞\|版本注入\|verify_all 前端检查路径" README.md
```
输出：`0`

**结论**：README 技术债章节已改为简洁表格，仅含 TD-1～TD-7（已清偿）、TD-8（保留正确设计）、OPT-1～OPT-9（均已实现），不再列出任何待解决示例。假设**被证伪——实现通过**。

---

## verify_all result

### 完整运行（含 E2E）

```
=== verify_all (fullstack) ===
Project: frp_easy
Stack:   Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)

[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO/FIXME budget ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[C.1] E2E smoke (playwright) ... SKIP
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agents in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md ... PASS
[E.6] Adversarial tests section in completed task reports ... PASS

=== Summary ===
  PASS: 17
  WARN: 0
  FAIL: 0
  SKIP: 1
```

### --quick 模式

```
=== Summary ===
  PASS: 17
  WARN: 0
  FAIL: 0
  SKIP: 0
```

| 指标 | 变化 |
|---|---|
| Total tests | 162 → 164（T-004 QA 未及时更新 baseline，本次补正） |
| Go tests | 117 → 119 |
| Frontend tests | 45（无变化） |
| Pass | 17 |
| Fail | 0 |
| Warn | 0 |
| Skip | 1（C.1 E2E，需 T-006 Playwright） |
| New tests added（T-005 本任务） | 0（纯文档/配置变更） |
| Baseline updated | yes（补正 T-004 未更新部分：go_tests 117→119，test_count 162→164） |

---

## Defects found

无 BLOCKER、CRITICAL、MAJOR、MINOR 缺陷。

**辅助发现（非缺陷，仅记录）**：`scripts/baseline.json` 中 `go_tests: 117`、`test_count: 162` 未反映 T-004 实际交付结果（T-004 QA 报告确认"总数从 117 升至 119"但未更新 baseline.json）。已在本次 QA 阶段补正至 119/164。

---

## Stability

verify_all `--quick` 连续运行 3 次，结果完全一致（PASS: 17, WARN: 0, FAIL: 0, SKIP: 0），无 flaky 检查项。

---

## Verdict

**APPROVED FOR DELIVERY**

全部 17 条验收标准（AC-A1～AC-A6、AC-B1～AC-B4、AC-C1～AC-C6、AC-VERIFY）均通过，verify_all FAIL: 0，无缺陷，稳定性良好。
