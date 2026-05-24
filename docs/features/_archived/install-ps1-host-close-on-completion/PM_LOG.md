# PM Log — T-031 install-ps1-host-close-on-completion

## 任务
用户在 Win11 PowerShell 7 终端运行一键安装脚本（`scripts/install.ps1`，通过 `irm | iex` 形态加载）到后期会自动关闭终端，导致无法观察到实际安装结果。

## 决策原则（用户给定）
1. 用户体验好
2. 符合软件工程标准
3. 长期易使用易维护

## 模式
full 7-stage（fix 类任务有明确验收点："PS7 + iex 形态执行 install.ps1 完成后宿主终端不关闭，用户能看到第 8 步打印的完整结果")。

## Insight Pre-check
- L33 (T-033 ParserError): `[CmdletBinding()]` 在 iex 不允许 — 当前已删
- L37 (T-026 BOM 容忍度): iex 形态禁 BOM — 当前已正确
- **L44 (T-026 核心 nuance)**: `& { ... } @PSBoundParameters` 子作用域包裹**仅**在交互式 console host 下保护宿主，`pwsh -File <script>.ps1` 脚本宿主下 `exit N` 仍杀进程
- L45 (T-026 splatting 配对): 内外 param 块必须一致
- L48 (T-026 archive-task 标题): `## Insight` 必须裸标题

## Context7 / Microsoft 官方文档（已收集，传给 RA）
1. `about_Invoke-Expression`: "Expressions are evaluated and run in the **current scope**."
2. `about_Language_Keywords` (exit): "Causes PowerShell to exit a script **or a PowerShell instance**." — 二义性是杀宿主与否的根因
3. `about_Scopes`: "Using the call operator to run a function or script runs it in script scope." — `& script.ps1` 创建 script scope

## 阶段路由
- [x] Stage 0: PM 拉历史 + 收集 context7 文档
- [x] Stage 0: 创建任务条目 + 目录
- [ ] Stage 1: Requirement Analyst → 01_REQUIREMENT_ANALYSIS.md
- [ ] Stage 2: Solution Architect → 02_SOLUTION_DESIGN.md
- [ ] Stage 3: Gate Reviewer → 03_GATE_REVIEW.md
- [ ] Stage 4: Developer → 04_DEVELOPMENT.md
- [ ] Stage 5: Code Reviewer → 05_CODE_REVIEW.md
- [ ] Stage 6: QA Tester → 06_TEST_REPORT.md（含 `## Adversarial tests` 段）
- [ ] Stage 7: PM 写 07_DELIVERY.md + verify_all + archive-task + commit + push
