# PM_LOG — T-014 frp-binary-auto-download

任务模式：full（7-stage）。负责人：PM Orchestrator。

## 时间线

- 2026-05-22 · 用户要求：frp 二进制不再内置、改为运行时下载最新版；补更新提示。创建任务 T-014，stage=req。
- 2026-05-22 · PM 预决策见 INPUT.md（A 块 frp 二进制 unvendor + 下载最新版，B 块更新提示可发现性）。
- 2026-05-22 · 派发 Requirement Analyst。

## 阻塞记录

（无）

- 2026-05-22 · RA 完成，verdict=READY（14 FR / 13 闸门 AC + 3 交付后 MV / 8 out-of-scope）。4 个开放问题（均设计选型）PM 裁决：OQ-1 采纳候选 B（保留目录与 frp LICENSE、.gitignore 忽略二进制、删 toml 样例，细节给 Architect）；OQ-2 首启是否自动下载委托 Architect（硬约束：不阻塞启动、离线可启动）；OQ-3 package.sh 断言阈值 Developer 按实核对；OQ-4 采纳候选 A —— install 升级**必须保留**用户运行时已下载的 frp 二进制（关键，否则每次更新都会清掉 frpc/frps）。
- 2026-05-22 · stage=design，派发 Solution Architect。

## 阶段产出

- 01_REQUIREMENT_ANALYSIS.md — ✅ READY
- 02_SOLUTION_DESIGN.md — ✅ READY（OQ-2 不自动下载、沿用横幅，启动不阻塞为结构性保证；downloader 加 resolveLatestAsset；OQ-4 显式删升级 rm-rf 块）
- 03_GATE_REVIEW.md — ✅ APPROVED FOR DEVELOPMENT（8 维 7 PASS/1 WARN）
- 04_DEVELOPMENT.md — ✅ 完成（无 DESIGN DRIFT）
- 05_CODE_REVIEW.md — ✅ APPROVED（0 BLOCKER/0 MAJOR/2 MINOR/1 NIT）
- 06_TEST_REPORT.md — 进行中

补记：
- 2026-05-22 · Code Review APPROVED；PM 把 M-1（删死字段 baseURL）、M-2（DEPLOYMENT C.2.4/C.3.4 过时手动升级指令）路由回 Developer 修复，已完成，verify_all 仍 PASS 19。stage=test。

补记：
- 2026-05-22 · Developer 完成。git rm 8 个 frp 文件、downloader 改下载 latest、package.ps1 已同步（F-1）。verify_all PASS 19；baseline go_tests 167→171、test_count 224→228。Open issue：Manager.baseURL 字段重构后未用、保留待 Code Reviewer 评估。stage=review。

补记：
- 2026-05-22 · Gate Review 通过。F-1（package.ps1 须同步改造，确切行号已给）并入 Developer 派发。F-3（OQ-2 横幅一键下载、非启动即自动下）PM 将在交付时知会用户。stage=dev。

补记：
- 2026-05-22 · Architect 完成。单 Developer 分区。风险 R-2（frp latest 版本不受控、长期 TOML schema 兼容）记录待入 07 Insight。stage=gate。
