# Post-delivery followup · T-020 claude-settings-context7-fix

> 2026-05-23 · 用户在交付后跑 ADV-1 manual followup 时 VS Code 报错 "不允许属性 _doc_sync_hook"，捕获了 T-020 全流水线（RA / Architect / Gate Reviewer / Code Reviewer / PM 5 个 agent）一致判断错的事实。

## 问题

设计 02 §5 与 RA §5 I-1 的决议 D-4 "保留 `_comment` / `_doc_sync_hook`，因为 JSON Schema 默认 `additionalProperties: true` 不触发校验错误"——**实测不成立**。

## 根因

`curl -sL https://json.schemastore.org/claude-code-settings.json` 拉取 schema 源文件后 grep `additionalProperties`：

- **根级（schema line 256）**：`"additionalProperties": true` → 顶层 `_comment` 允许 ✅
- **`hooks` 子对象（schema line 1270）**：`"additionalProperties": false` → `_doc_sync_hook` 显式禁止 ❌

下划线前缀不构成豁免。

## 修复（trivial 直接修，无新阶段文档）

唯一改动：[.claude/settings.json:42-43](../../../.claude/settings.json#L42-L43) 删除 `_doc_sync_hook` 字段。`_comment` 顶层字段保留（根级 schema 允许）。

字节级核对：
- size 1702 → 1355（-347 = `_doc_sync_hook` 长度）
- CR=0 / LF=54（之前 55，删 1 行）
- 首字节 `7B`（无 BOM）/ 末字节 `7D 0A`（单 LF 结尾）
- `ConvertFrom-Json` 通过；`hooks` 现仅 `Stop` 一个 key；Stop[0] 仍无 matcher
- `_comment` / `permissions.allow`（21 条）/ `permissions.deny`（11 条）全部保留

verify_all：PASS:19 / WARN:0 / FAIL:0 / SKIP:0，零回归。

## 教训（已落入 [.harness/insight-index.md](../../../.harness/insight-index.md) 末行）

当设计基于 JSON Schema 时，**必须 curl 拉 schema 源文件并对每个对象层逐级 grep `additionalProperties`**，禁止按"JSON Schema 默认 true"的常识推测——schema 作者常在子对象覆盖。这是"全 reviewer 漏审"的标志性案例，价值高于本任务本身的 P0/P1/P2 修复。

ADV-1 manual followup 是这次捕获的唯一通道——自动化 7-stage 流水线无法验证 VS Code GUI 的 schema 校验输出。这也是 ADV 类对抗用例不可全部 ditch 给 verify_all 的另一证据。

## Insight 反思——为什么 5 个 agent 全漏

1. **RA**：决议 D-4 "保留" 写在 §5 信息性段，未要求 Architect 验证 schema 真实约束。
2. **Architect** 02 §5：明确写"该 schema 的 additionalProperties 默认为 true，故 `_comment` / `_doc_sync_hook` 不触发校验错误"——**完全是推测**，未用 WebFetch 或 curl 拉 schema 验证。
3. **Gate Reviewer** §1 维度 4 "Risk coverage" 评 WARN 但只点 `$HOME` 命中限制（F-1），未要求 Architect 拉 schema 实证。
4. **Code Reviewer** §5.4 "Security" / §5.6 "Maintainability" 仅复用设计 §5 结论，未独立验证。
5. **PM** 把"已查证 context7 settings 示例"等同于"已查证 schema 严格性"——但 context7 示例只展示推荐写法，不展示禁止什么。

补救已采取：insight-index 新条目要求"按 schema 设计时必须拉源文件 grep additionalProperties"。
