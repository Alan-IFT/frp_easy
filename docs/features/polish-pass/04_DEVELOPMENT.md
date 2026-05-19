# 04 — 开发：T-009 polish-pass

> Stage 4 of 7-stage `/harness` 流水线 · 中文 · PM 亲做（清理任务）

---

## 1. 实施回顾（按 02 §5 顺序）

| 步 | 动作 | 文件 | 结果 |
|---|---|---|---|
| 1 | 新增 `scripts/start-e2e-server.ps1`（PowerShell 7+；UTF-8 无 BOM TOML；自动重建；`& $BIN` 直接 invoke） | `scripts/start-e2e-server.ps1` (new) | 文件 67 行，含 `#requires -Version 7` |
| 2 | playwright.config.ts 加平台分支 `process.platform === 'win32' ? pwsh : bash` | `web/playwright.config.ts` | 加 2 行注释 + 3 行三元表达式 |
| 3 | dev-map.md 日文 → 中文（按 02 §4.2 翻译表，3 段 Edit 替换） | `docs/dev-map.md` | 第 50–96 行；grep 日文假名 0 命中 |
| 3.5 | dev-map.md `scripts/` 索引补 `start-e2e-server.{sh,ps1}` | `docs/dev-map.md` | line 20 单点替换 |
| 4 | `bash scripts/archive-task.sh --task web-ui-mvp` | filesystem move | 11 个文件移到 `_archived/web-ui-mvp/`；07_DELIVERY 无 `## Insight` 段，insight-index.md 未变 |
| 5 | tasks.md 已正确指向 `_archived/web-ui-mvp/`（PM 检查发现 T-001 行此前就写了 archived 路径，无需改）；只清理误加的空行 | `docs/tasks.md` | done |
| 6 | 两 shell 验证 verify_all | — | （进入 Stage 6 QA） |

## 2. 关键实现要点

### 2.1 start-e2e-server.ps1 三处与 .sh 不同的细节

1. **go 解析**：先尝试硬编码 `C:\Program Files\Go\bin\go.exe`（项目其他脚本同款），失败再 `Get-Command go`。.sh 直接 `go build` 因为 Git Bash PATH 已包含。
2. **TOML 编码**：用 `[System.IO.File]::WriteAllText` + `UTF8Encoding(false)` 强制无 BOM，避免 BurntSushi/toml 拒绝。.sh 用 heredoc 自然无 BOM 不存在此问题。
3. **路径分隔符**：DataDir / LogDir 用 `replace '\\' '/'`，TOML 字符串里 Windows 反斜杠会被当转义符；改成正斜杠 Go 在 Windows 上同样能用。

### 2.2 playwright.config.ts 改动 diff

```diff
+  // Windows 上用 pwsh 调用 .ps1（PowerShell 自带 `bash` 解析到 WSL shim 会失败）；
+  // 其他平台沿用 bash 调用 .sh。两个脚本行为等价（详见 docs/features/_archived/polish-pass/02_SOLUTION_DESIGN.md §2）。
   webServer: {
-    command: 'bash ../scripts/start-e2e-server.sh',
+    command: process.platform === 'win32'
+      ? 'pwsh -NoProfile -ExecutionPolicy Bypass -File ../scripts/start-e2e-server.ps1'
+      : 'bash ../scripts/start-e2e-server.sh',
     url: 'http://127.0.0.1:8080/api/v1/health',
     ...
```

### 2.3 Gate Review 3 项 MINOR 吸收情况

| MINOR | 吸收 |
|---|---|
| M-1 `#requires -Version 7` | ✅ 文件第 1 行 |
| M-2 playwright.config.ts 顶部注释 | ✅ 2 行说明 |
| M-3 诊断行加颜色 | ⚪ 不采纳——`[Console]::Error.WriteLine` 进 stderr 已被 Playwright 标黄；多加 ForegroundColor 会破坏在管道下的输出 |

---

## 3. 独立验证结果

### 3.1 .ps1 单独运行（不经 Playwright）

```
[e2e-server] binary up-to-date, skipping build
[e2e-server] using E2E_TMP: C:\Users\yangx\AppData\Local\Temp\frp-easy-e2e-1596262ac739496e988f28a24823c084
... locator resolved
... http listening 127.0.0.1:8080
... ready gate opened
```
`Invoke-RestMethod /api/v1/health` → `{"status":"ok","version":"43ad919-dirty"}` ✅

### 3.2 PowerShell 中 `npx playwright test`

```
ok 1 [chromium] TC-01
ok 2 [chromium] TC-02
ok 3 [chromium] TC-03
ok 4 [chromium] TC-04
ok 5 [chromium] TC-05
5 passed (3.6s)
```
✅

### 3.3 Git Bash 中保持不变（回归保护）

将由 QA 在 Stage 6 双 shell 验证。

---

## 4. 没改的文件（白名单核对）

| 区域 | 状态 |
|---|---|
| `cmd/**` | 未触 ✅ |
| `internal/**` | 未触 ✅ |
| `migrations/**` | 未触 ✅ |
| `web/src/**` | 未触 ✅ |
| `.claude/**` | 未触 ✅ |
| `CLAUDE.md` | 未触 ✅ |
| `.github/copilot-instructions.md` | 未触 ✅ |
| `.harness/rules/**` `.harness/agents/**` | 未触 ✅ |

---

## 5. Verdict

**READY FOR CODE REVIEW** — 进入 Stage 5。
