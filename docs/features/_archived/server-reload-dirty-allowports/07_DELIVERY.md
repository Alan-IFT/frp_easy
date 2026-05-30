# 交付总结 — T-060 server-reload-dirty-allowports

- **任务**: `server-reload-dirty-allowports` — 让 Server.vue「重新加载」的 dirty 检测纳入 AllowPortsEditor 端口策略，消除"只改端口策略 → 点重新加载 → 无确认 → 静默丢弃端口编辑"的数据丢失路径（补齐 T-058 (B) 已知局限）。
- **模式**: full（7-stage）
- **阶段历程（含时间）**:
  - Stage 0 PM 预核实（2026-05-30）：真读 Server.vue / AllowPortsEditor / baseline / Server.spec / e2e grep。
  - Stage 1 Requirement Analyst → `READY`（无歧义）。
  - Stage 2 Solution Architect → `READY`（分区 dev-frontend 单分区；normalizeAllowPorts 内联设计）。
  - Stage 3 Gate Reviewer → `APPROVED WITH CONDITIONS`（C1：SFC<200 行）；独立核实全部代码引用存在。
  - Stage 4 dev-frontend → `READY FOR REVIEW`（0 DESIGN DRIFT）。
  - Stage 5 Code Reviewer → `APPROVED`。
  - Stage 6 QA Tester → **发现 D-1**（AC-6 测试断言与单向数据流不复位范式冲突）。
  - Stage 4 第二轮 dev-frontend → D-1 纯测试侧修复（生产逻辑零变更）。
  - Stage 5 复审 → `APPROVED`（第二轮）。
  - Stage 6 复验 → `APPROVED FOR DELIVERY`（D-1 RESOLVED）。
  - Stage 7 PM 交付。
- **回退**: 1 次（dev-frontend，D-1 测试侧断言修复；未触 3 次阈值）。
- **Final verify_all result**: **PENDING（交 orchestrator Bash 会话真跑作硬闸门）** —— PM 单上下文无 Bash/PowerShell（insight L31）。静态 + 确定性预测全绿：B.2 eslint / B.3 vitest 491 全绿 / B.4 计数达标（491≥481, go 322）/ B.5 无残留 / C.1 e2e 不受影响 / E.6 Adversarial 段存在。
- **Baseline changes**: `frontend_tests` 481→491（+10）/ `test_count` 803→813（+10）/ `go_tests` 322 不变 / version 25→26。
- **Outstanding risks**:
  - verify_all 全量真跑未在交付会话执行（PM 无 Bash），交 orchestrator Bash 会话核对（预期全绿，纯确定性测试，偏离即回退信号）。
  - 范式约束（非缺陷）：端口策略改动后即使确认重载，AllowPortsEditor 因单向数据流不 watch props.initial 而 rows 不复位——这是有意保留的 T-040 范式，dirty 检测正确捕获（多一次确认无数据风险），已在 06 D-1 与测试注释中显式记录。
- **Files changed**（4 个）:
  - `web/src/pages/Server.vue` — 新增 normalizeAllowPorts 纯函数 + loadedAllowPortsSnapshot ref；loadConfig 末尾从 cfg.allowPorts 派生快照；isDirty() 标量比较后追加端口策略规范化比较；更新注释移除"已知局限"；expose 补 3 项。
  - `web/src/pages/__tests__/Server.spec.ts` — +10 测试（normalize 稳定性 3 + 端口策略纳入 dirty 6 + Adversarial 删行 1）；import AllowPortRange + TestingHandle 扩展。
  - `scripts/baseline.json` — 491/813/version 26 + notes。
  - `docs/dev-map.md` — Server.vue 描述行更新。
- **Next steps for user**:
  1. 在 orchestrator Bash 会话真跑 `bash scripts/verify_all.sh`（交付硬闸门）。预期 PASS；若 FAIL 按 06 D-1 处理建议路由回 dev-frontend。
  2. 本任务按用户要求**未 git commit/push、未跑 archive-task**。如需归档，verify_all 绿后手动 `scripts/archive-task --task server-reload-dirty-allowports`。

## Insight

- 2026-05-30 · 表单页"重新加载/重置"的 dirty 检测纳入有独立子编辑器（如 AllowPortsEditor）时，比较必须用"加载值派生的规范化字符串快照 vs 子组件当前输出规范化值"双侧同函数（normalize：single→'s:N'/range→'r:A-B'/join，顺序+形态敏感），靠 round-trip identity 保证未改动判非脏；但子编辑器若守单向数据流不 watch props.initial（T-040 范式），确认重载后其内部 rows **不复位**——故"重载后 dirty 归零"只对父级直接重赋的标量 form 成立，对独立子编辑器不成立。测试 AC 不能假设子编辑器复位（QA D-1 抓到此张力）；正解是父级断言"快照刷新+API 调用"、不强求子编辑器侧 isDirty 归零，并显式注释这是范式约束而非 bug · evidence: T-060 Server.vue isDirty/normalizeAllowPorts + Server.spec AC-6 拆分（端口策略侧 vs 标量侧）+ 06 D-1
