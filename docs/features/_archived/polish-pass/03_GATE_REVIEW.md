# 03 — Gate Review：T-009 polish-pass

> Stage 3 of 7-stage `/harness` 流水线 · 中文 · PM 亲撰扮演 Gate Reviewer 视角

---

## 1. Gate Reviewer 任务

独立审查 01_REQUIREMENT_ANALYSIS.md + 02_SOLUTION_DESIGN.md，判定是否准予开发。

---

## 2. 完整性核查（Requirements ↔ Design）

| AC | 设计中是否覆盖 | 备注 |
|---|---|---|
| AC-1 PowerShell 18 PASS | ✅ L1 全线 + Step 6 验证 | |
| AC-2 Git Bash 18 PASS | ✅ Step 6 验证；不动 .sh | |
| AC-3 不引入 Go 依赖 / 不改业务代码 | ✅ 02 §1 表格清单已限定文件 | |
| AC-4 playwright.config.ts 平台分支 | ✅ 02 §2.5 给出准确代码片段 | |
| AC-5 .ps1 行为对齐 .sh | ✅ 02 §2.4 行为等价表 | |
| AC-6 web-ui-mvp 归档 | ✅ L2 步 4 | |
| AC-7 insight-index 无重复 | ✅ T-001 07_DELIVERY.md 验证无 `## Insight` 段（Gate Reviewer 自查） | |
| AC-8 dev-map 日文 → 中文 | ✅ L3 §4.2 翻译表覆盖范围 | |
| AC-9 不动 .claude/ CLAUDE.md | ✅ 02 §1 范围表未含此类文件 | |
| AC-10 不改 migration | ✅ 范围内未列 | |
| AC-11 不改业务代码 | ✅ 范围内未列 | |
| AC-12 tasks.md 更新 done | ✅ L2 步 5 + L1 完成后由 PM Delivery 阶段补 | |

---

## 3. 可行性 / 风险评估

### 3.1 强项

- **修改面极小**：3 文件改 + 1 文件新增 + 1 脚本调用 + 1 任务看板状态更新。归档由现成脚本完成（已经过 7 次任务的实战）。
- **风险隔离**：L1/L2/L3 三条线互不依赖；任一回滚不影响其他。
- **可验证性强**：每条 AC 都有具体命令验证；最终硬闸门是 `verify_all` 在两个 shell 都 PASS。
- **不破坏现有路径**：.sh 不动，Linux/macOS/Git Bash 用户路径完整保留。

### 3.2 风险审查（按 02 §6）

| # | 评级 | Gate 备注 |
|---|---|---|
| R-1 pwsh 缺失 | 低 | 项目其他 .ps1 同前提（package.ps1 / install-service.ps1）；与现状一致 |
| R-2 ExecutionPolicy GPO | 低 | 开发者本机不应有；CI 上 GitHub Actions Windows runner 默认 Bypass |
| R-3 SIGTERM 优雅停 | 低 | Playwright 5s 超时 `taskkill /T /F` 强杀；偶发 stale 进程不阻断验证 |
| R-4 UTF-8 BOM | 中 | **Gate 直点**：设计已用 `[System.IO.File]::WriteAllText` + `UTF8Encoding(false)`，但仍要 Developer 实测 frp_easy.toml 解析成功，不能只信设计文字 |
| R-5 端口占用 | 不在范围 | OK |

### 3.3 反向审查（Devil's Advocate）

**Q1：能不能干脆放弃 PowerShell 路径，只支持 Git Bash？**
A：违反 US-1（Windows 开发者）。项目其他 .ps1 / .sh 双脚本配对一致表明项目主张"两个 shell 都是一等公民"。否定。

**Q2：能不能让 .sh 在 PowerShell 下用 `cmd /c bash ...` 跳过 WSL shim？**
A：依赖用户本机 Git Bash 路径在 PATH 中比 WSL shim 靠前，不可靠。否定。

**Q3：能不能引入 Node.js cross-platform wrapper（`cross-spawn` 等）？**
A：引入新 npm 依赖，违反"修改面最小"原则；项目当前未用此类库。否定。

设计方案 A（按 platform 路由）是最干净的选择。

### 3.4 未发现的 Gate 级阻塞

无 MAJOR / CRITICAL 级问题。

---

## 4. MINOR 建议（不阻塞）

| # | 建议 | 由谁处理 |
|---|---|---|
| M-1 | start-e2e-server.ps1 顶部加 `#requires -Version 7` 防止 Windows PowerShell 5.x 误跑 | Developer 实施时加 |
| M-2 | playwright.config.ts 顶部加一行注释 `// Windows: pwsh，其他: bash（双脚本路径）` 方便后续维护 | Developer 加 |
| M-3 | start-e2e-server.ps1 的诊断输出走 `Write-Host -ForegroundColor Cyan` 而不是普通 stderr，可在 Playwright 输出中更易识别 | 可选；不影响功能 |

---

## 5. Verdict

**APPROVED FOR DEVELOPMENT** — 无条件通过；3 项 MINOR 建议由 Developer 内联吸收。

进入 Stage 4。
