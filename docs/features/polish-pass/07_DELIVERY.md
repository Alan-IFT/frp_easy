# 07 — 交付：T-009 polish-pass

> Stage 7 of 7-stage `/harness` 流水线 · 中文
> 任务：解决遗留问题、修跨 shell verify_all 不一致、清理混杂语言文档、归档遗漏的 T-001
> 交付日期：2026-05-19

---

## 1. 任务起源

用户请求："解决所有遗留问题，检查是否有影响用户体验的，若有则你来决策处理；以用户体验好，符合软件工程标准，长期易使用易维护为原则来决策"。

PM 扫描后发现 3 类遗留：

1. **P1 [UX 影响]** PowerShell 中 `scripts/verify_all.ps1` 第 C.1 (E2E Playwright) FAIL —— 根因 `web/playwright.config.ts` webServer 调用 `bash`，PowerShell 解析到 WSL shim 失败。Windows 用户在默认 shell 下永远 17 PASS + 1 FAIL，违反"声明完成前必须 PASS"红线。
2. **P3 [工程标准]** T-001 web-ui-mvp 任务文档显示 DELIVERED 但 `docs/features/web-ui-mvp/` 未归档；违反"DELIVERED 即归档"约定。
3. **P3 [可维护性]** `docs/dev-map.md` 第 50-96 行注释含日文假名（早期任务遗留），与 CLAUDE.md "中文" 约定不一致。

按 UX 优先 + 工程标准 + 可维护性原则，PM 自主决策一次性处理。开启 T-009 polish-pass 全 7-stage 流水线（清理任务，PM 亲做不派子 agent）。

---

## 2. 交付内容（6 个文件 + 1 个移动）

| # | 路径 | 状态 | 行数 |
|---|---|---|---|
| 1 | `scripts/start-e2e-server.ps1` | new | 69 |
| 2 | `web/playwright.config.ts` | edit | +6 / -1 |
| 3 | `docs/dev-map.md` | edit（日文 → 中文，三段；scripts/ 索引补 .ps1） | ~32 注释行 |
| 4 | `docs/tasks.md` | edit（T-009 写入"已完成"） | +3 |
| 5 | `docs/features/web-ui-mvp/*` → `docs/features/_archived/web-ui-mvp/*` | move（脚本） | 11 文件 |
| 6 | `docs/features/polish-pass/{INPUT,PM_LOG,01..07}.md` | new（流水线产出） | 9 文件 |

零业务代码改动（cmd/internal/migrations/web/src 未触）。零新依赖（无 Go / npm 包变化）。

---

## 3. 流水线执行回顾

| Stage | 输出 | 关键事件 |
|---|---|---|
| 1 Requirement Analyst | 01_REQUIREMENT_ANALYSIS.md（3 用户故事 + 12 AC + 5 风险） | verdict READY |
| 2 Solution Architect | 02_SOLUTION_DESIGN.md（3 线 L1/L2/L3 互不依赖 + 行为等价表 + 翻译表 32 行） | verdict READY FOR GATE REVIEW |
| 3 Gate Reviewer | 03_GATE_REVIEW.md | verdict APPROVED FOR DEVELOPMENT；3 MINOR 由 Developer 吸收（M-1 #requires 加；M-2 注释加；M-3 不采纳） |
| 4 Developer | 04_DEVELOPMENT.md（3 步实施 + 独立验证 .ps1 → Playwright 5 通过） | 6 文件改动落地 |
| 5 Code Reviewer | 05_CODE_REVIEW.md | verdict APPROVED FOR QA；MAJOR/CRITICAL 0；MINOR 2 项均不阻塞 |
| 6 QA Tester | 06_TEST_REPORT.md（12 AC + 8 Adversarial） | verdict READY FOR DELIVERY；PowerShell 18 PASS + Git Bash 18 PASS 双向通过 |
| 7 Delivery | 本文件 + archive + commit | — |

总耗时：单 session，~1 小时（清理性任务，无设计争议）。

---

## 4. 最终 verify_all（双 shell）

| Shell | 结果 |
|---|---|
| PowerShell 7+ | `PASS: 18 / WARN: 0 / FAIL: 0 / SKIP: 0` ✅ |
| Git Bash (MSYS2) | `PASS: 18 / WARN: 0 / FAIL: 0 / SKIP: 0` ✅ |

---

## 5. 用户视角的能力提升（Before / After）

| 维度 | Before（T-008 完成时实测） | After（T-009 交付） |
|---|---|---|
| Windows + PowerShell verify_all | 17 PASS + 1 FAIL（C.1 E2E） | 18 PASS ✅ |
| Windows + Git Bash verify_all | 18 PASS | 18 PASS（保持） |
| Linux / macOS verify_all | 18 PASS | 18 PASS（保持） |
| `docs/dev-map.md` 注释语言 | 中 + 日文假名混杂 | 全中文 |
| 任务归档约定 | T-001 文档目录路径写归档但实际未移 | tasks.md 与磁盘 100% 一致 |

US-1 / US-2 / US-3 三类用户故事全覆盖。

---

## 6. 用户需关注的信息

### 6.1 改了什么

- 加了一个 PowerShell 启动脚本（`scripts/start-e2e-server.ps1`）和对应的 playwright config 平台分支，让 Windows + PowerShell 用户能直接跑 verify_all 全绿，不再需要切换 Git Bash。
- 翻译了 dev-map 中 50 行残留的日文注释为中文。
- 归档了 T-001 文档到 `_archived/`。

### 6.2 当前状态

- `git status` 显示 17 项变更（13 修改/删除 + 3 新增目录），按"零业务代码"准则只触动文档、脚本、playwright config。
- verify_all 在两种 shell 下都通过；E2E 5 个测试都过。
- 没有暂存任何 .env / 凭据；A.1 secrets 扫描清白。

### 6.3 后续可能的话题（PM 评估，不在本次范围）

- **deploy-kit 中 "release-smoke 5 AC"**：服务安装脚本（install-service.{sh,ps1}）只做了静态验证；真正 `systemctl is-active` / `sc query` 应在首次正式 release 时跑一遍。docs/DEPLOYMENT.md 已列入口。
- **CI 集成**：项目暂无 GitHub Actions / 任何 CI 配置。如果将来上 CI，verify_all.{sh,ps1} 两个入口都可以直接跑，无需为 PowerShell shell 额外适配。
- **dev-map 与代码漂移检测**：本次手工同步；若想自动化，可加一个 scripts/check-devmap.sh 验证文件存在性（不阻塞，按需上）。

---

## Insight

> 收割到 `.harness/insight-index.md` 的非显然事实。Archive 脚本会自动 harvest 本段。

- **2026-05-19** · Windows PowerShell 的 `bash` 命令默认解析到 `C:\Users\<user>\AppData\Local\Microsoft\WindowsApps\bash.exe`（WSL shim），即使 Git Bash 已安装；用户未装 Linux 发行版时返回乱码错误。Playwright `webServer.command: 'bash ...'` 在 PowerShell 调用链下会失败。修复模式：`process.platform === 'win32' ? 'pwsh ... -File .ps1' : 'bash .sh'` 双脚本配对 · evidence: T-009 polish-pass
- **2026-05-19** · PowerShell 写 TOML 配置文件必须用 `[System.IO.File]::WriteAllText($path, $content, [System.Text.UTF8Encoding]::new($false))` 强制无 BOM；`Out-File -Encoding utf8` 在 Windows PowerShell 5.x 会写 BOM 让 BurntSushi/toml 解析失败（PS7 默认无 BOM 但项目支持 PS5 时仍要显式） · evidence: T-009 polish-pass
- **2026-05-19** · TOML 字符串中 Windows 反斜杠路径会被当转义；写脚本生成的临时 TOML 配置时 `-replace '\\' '/'` 把所有反斜杠换正斜杠是最简单的方式，Go 在 Windows 上同样接受正斜杠路径 · evidence: T-009 polish-pass
