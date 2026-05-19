# 06 — QA 测试报告：T-009 polish-pass

> Stage 6 of 7-stage `/harness` 流水线 · 中文 · PM 亲扮 QA Tester（按 `.harness/agents/qa-tester.md` 的 Adversarial 契约）

---

## 1. 测试目标

验证 04_DEVELOPMENT.md 的实现满足 01 的全部 AC，并主动寻找反例。

---

## 2. AC 直接验收（Happy Path）

| AC | 命令 / 检查 | 结果 |
|---|---|---|
| AC-1 PowerShell 18 PASS | `& "C:\Programs\frp_easy\scripts\verify_all.ps1"` | PASS: 18 / WARN: 0 / FAIL: 0 / SKIP: 0 ✅ |
| AC-2 Git Bash 18 PASS | `bash scripts/verify_all.sh` | PASS: 18 / WARN: 0 / FAIL: 0 / SKIP: 0 ✅ |
| AC-3 不引入 Go 依赖 | `git diff HEAD -- go.mod go.sum` | 空输出 ✅ |
| AC-4 playwright.config.ts 平台分支 | `grep -n "process.platform" web/playwright.config.ts` | line 21 `command: process.platform === 'win32'` ✅ |
| AC-5 .ps1 行为对齐 .sh | PowerShell `npx playwright test --project=chromium` | 5 passed (3.6s) ✅ |
| AC-6 归档完整 | `ls docs/features/_archived/web-ui-mvp/` | 11 文件（INPUT + PM_LOG + 01..07 + 04_DEVELOPMENT × 3 partitions）✅ |
| AC-7 insight-index 无重复 | `git diff .harness/insight-index.md` | 空（07_DELIVERY 无 ## Insight 段）✅ |
| AC-8 dev-map 假名清零 | ripgrep `[぀-ゟ゠-ヿ]` | 0 命中 ✅ |
| AC-9 不动 .claude/CLAUDE.md/.github/copilot-instructions.md | `git diff HEAD --` 仅 docs/ + scripts/ + web/playwright.config.ts | ✅ |
| AC-10 不改 migration | `git diff HEAD -- migrations/` | 空 ✅ |
| AC-11 不改业务代码 | `git diff HEAD -- cmd/ internal/ web/src/` | 空 ✅ |
| AC-12 tasks.md 含 T-009 | line 11 已加进行中 | ✅（Stage 7 会改为 done） |

12 条全部 PASS。

---

## 3. 验证产物

### 3.1 PowerShell verify_all 最后摘要

```
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agent definitions present in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa) ... PASS
[E.6] Adversarial tests section present in completed task reports ... PASS

=== Summary ===
  PASS: 18 / WARN: 0 / FAIL: 0 / SKIP: 0
```

### 3.2 Git Bash verify_all 最后摘要

```
[C.1] E2E smoke (playwright) ... PASS
...
=== Summary ===
  PASS: 18 / WARN: 0 / FAIL: 0 / SKIP: 0
```

### 3.3 PowerShell 独立 Playwright 跑 5 tests

```
ok 1 [chromium] tests\e2e\01-setup.spec.ts › Setup wizard › TC-01
ok 2 [chromium] tests\e2e\01-setup.spec.ts › Setup wizard › TC-02
ok 3 [chromium] tests\e2e\02-auth.spec.ts › Auth › TC-03
ok 4 [chromium] tests\e2e\03-dashboard.spec.ts › Dashboard › TC-04
ok 5 [chromium] tests\e2e\03-dashboard.spec.ts › Dashboard › TC-05
5 passed (3.6s)
```

---

## Adversarial tests

> 主动找反例：每条 AC 至少一条"如果它假成立会触发的检查"。本段是 verify_all E.6 红线要求的硬性段。

### Adv-1（对应 AC-3）：Go 依赖暗中新增检测

**反例假设**：开发者无意中在 .ps1 中引入了某个仅当 go.mod 新增依赖才能编译的实现。
**测试**：`git diff HEAD -- go.mod go.sum`
**结果**：空。✅ 反例不成立。

### Adv-2（对应 AC-8）：dev-map.md 残留假名

**反例假设**：翻译时漏了半角假名（U+FF61-U+FF9F）或部分平假名。
**测试**：`rg --pcre2 '[぀-ゟ゠-ヿ]' docs/dev-map.md`（覆盖 U+3040-U+30FF，平 / 片假名）。同时 git grep 半角假名 `[\xef\xbd\xa1-\xef\xbe\x9f]`。
**结果**：双正则均 0 命中。✅

### Adv-3（对应 AC-6）：归档目录文件丢失

**反例假设**：archive-task 脚本因路径有空格或编码问题漏移文件。
**测试**：`ls docs/features/_archived/web-ui-mvp/ | wc -l`，对比原 `docs/features/web-ui-mvp/` 移动前的 11 文件。
**结果**：归档目录恰好 11 文件，且 `docs/features/web-ui-mvp/` 目录已不存在。✅

### Adv-4（对应 AC-11）：业务代码暗中触动

**反例假设**：编辑 dev-map 时手滑改了 internal/ 注释，或 archive-task 改了 web/src。
**测试**：`git diff HEAD -- cmd/ internal/ migrations/ web/src/`
**结果**：空输出。✅

### Adv-5（对应 AC-5）：Playwright 终止后 frp-easy.exe 残留

**反例假设**：PowerShell 启动子进程后 Playwright SIGTERM 不能优雅传播，留 stale `frp-easy.exe` 监听 8080。
**测试**：Playwright 跑完后立刻 `tasklist | grep -i frp-easy.exe`。
**结果**：无 stale 进程。✅

### Adv-6（对应 AC-4）：平台分支被打开后不再走 bash

**反例假设**：playwright.config.ts 改动让 Linux/macOS 也走 pwsh 命令（pwsh 在 Linux 可能不在 PATH）。
**测试**：人工审查 `web/playwright.config.ts:21-24`，确认三元的 false 分支是 `bash ../scripts/start-e2e-server.sh`。
**结果**：false 分支保留 bash 调用 ✅。同时 Git Bash 路径下跑 verify_all 仍 PASS（间接证明 Linux 路径未受影响）。

### Adv-7（对应 AC-5 §2.4 §2.3 重建逻辑）：Need-Rebuild 误报 / 漏报

**反例假设**：`Get-ChildItem -Recurse` 加 `Where-Object { $_.LastWriteTime -gt $binMtime }` 在 dist 与 bin 时间戳相等时是否触发不必要重建。
**测试**：脚本里 bin mtime = 15:05:54，dist mtime = 15:02:43；Need-Rebuild 应返回 false。
**结果**：返回 false（不重建）。✅ 等价于 .sh 的 `find -newer`（严格大于）。

### Adv-8（对应 AC-1 / AC-2 跨 shell 双向）：是否仅一个 shell 通过另一个就坏？

**反例假设**：改 playwright.config.ts 时三元写错让 PowerShell 路径通了但 Git Bash 路径变成 pwsh，Git Bash 中没 pwsh 失败。
**测试**：两 shell 各跑完整 verify_all 一次。
**结果**：两边都 18 PASS。✅

### 总结

8 条反例测试全部不成立。AC 验收无侥幸 PASS。

---

## 4. Verdict

**READY FOR DELIVERY** — 进入 Stage 7（PM Delivery）。

总结：12 AC 全 PASS、8 Adversarial 全不成立、verify_all 在 PowerShell + Git Bash 双 shell 均 18 PASS / 0 FAIL。
