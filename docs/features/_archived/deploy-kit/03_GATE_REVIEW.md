# 03 — Gate Review：T-008 deploy-kit

> Stage 3 of 7-stage `/harness` 流水线
> 上游：01_REQUIREMENT_ANALYSIS.md（已批准）+ 02_SOLUTION_DESIGN.md（待评审）
> 角色：Gate Reviewer（独立验证，永不盲信上游）

---

## 1. 审阅摘要

设计文档（`02_SOLUTION_DESIGN.md`）整体质量高、详尽、可执行。十二章结构完整，覆盖架构、模块、契约、目录、字段、风险、AC 映射、实施顺序与 partition 分配。关键设计决策正确（标准库 `flag`、分平台 shell 不强行共用、systemd unit 内联生成避免双真相源、Windows .cmd 包装锁 cwd、原子写 unit 文件）。

**独立验证结果**：8 项核心断言中 6 项 PASS，2 项发现问题（主要是 partition 边界书面上不完全覆盖、`AC-22 grep` 校验命令在 PowerShell 上不可移植）。整体可进入开发，但建议先解决 **MAJOR-1（partition 文字依据）**，避免 Developer 在第 1 分钟就报 `BLOCKED ON PARTITION` 让 PM 二次澄清。

---

## 2. 逐 AC 可实现性核验（24 条）

| AC | 设计位置 | 核验 | 备注 |
|---|---|---|---|
| AC-1 | §3.1 / §3.2 / §4 | YES | 输出路径、命名约定齐全 |
| AC-2 | §3.1 表 + §10 | YES | 继承 build.sh L19 兜底；行为已验证 |
| AC-3 | §4 目录布局 | YES | 7 项内容枚举到位，LICENSE 缺失 WARN 跳过明确 |
| AC-4 | §3.1 前置校验 / §4 末尾 | YES | exit 1 |
| AC-5 | §4 FR-1.6 + `Compress-Archive -Force` | YES | 幂等 |
| AC-6 | §3.3 + §5.1 + §3.5 | YES | Type=simple + enable --now / sc.exe start |
| AC-7 | §3.4 + §3.6 | YES | 卸载链路完整 |
| AC-8 | §3.3 / §3.5 幂等分支 | YES | "刷新现有 unit" 显式覆盖 |
| AC-9 | §3.4 / §3.6 + 收尾中文提示 | YES | 不动数据目录 |
| AC-10 | §3.3 `--user` + getent / §3.5 `-DisplayName` | YES | 参数透传清晰 |
| AC-11 | §6.3 flag 解析块 | YES | `return nil` → main 退出码 0 |
| AC-12 | §6.2 `usageText` 常量 | YES | 中文用法/flag/配置/UI/退出码 5 项全包含 |
| AC-13 | §6.3 注释 | YES | 早于 envOr/Load |
| AC-14 | §6.3 错误分支 | **NEEDS_CLARIFY** | 见 MINOR-1（flag.ErrHelp 分流） |
| AC-15 | §8 大纲 3 个 H2 | YES | |
| AC-16 | §8 占位符约定 | YES | `<INSTALL_DIR>` / `<VERSION>` / `<ORG>` |
| AC-17 | §8 决策表 | YES | 3 行表头 |
| AC-18 | §8 F.1–F.5 | YES | 5 场景 + 日志位置 |
| AC-19 | §7 表"插入"行 | YES | |
| AC-20 | §7 表 L119–L158 搬迁 | YES | |
| AC-21 | §7 表 L161–L174 下沉 | YES | |
| AC-22 | §7 校验方法 | **NEEDS_CLARIFY** | 见 MINOR-2（`comm` POSIX-only） |
| AC-23 | §10 注释 | YES | verify_all 18 项无一项扫 scripts/*.sh、docs/*.md |
| AC-24 | §3.1 退出码 | YES | |

**小计**：22 YES / 2 NEEDS_CLARIFY / 0 NO。

---

## 3. 设计独立验证（8 点）

### 3.1 Partition 文字依据

读 `.harness/agents/dev-backend.md` L13–L23 owned paths：列举式（非 glob `scripts/**`），逐字未含 `scripts/package.{sh,ps1}`、`scripts/install-service.*`、`scripts/uninstall-service.*`、`docs/DEPLOYMENT.md`、`README.md`。**MAJOR-1**。

### 3.2 行号断言核验

| 断言 | 实测 | 结论 |
|---|---|---|
| build.sh L19 echo dev 兜底 | 行号一致 | PASS |
| main.go L48 `var Version = "0.1.0"` | 一致 | PASS |
| main.go L50 `func main()` | 一致 | PASS |
| main.go L57 `func run() error {` | 一致 | PASS |
| main.go L58 `// 1. appconf` | 一致 | PASS |
| main.go 未 import flag | 一致；io/errors/fmt 已 import | PASS |

### 3.3 AC 表覆盖

§10 表覆盖 AC-1…AC-24 全部，PASS。

### 3.4 Open Question PM-resolved

01 §8 共 10 条均带 `**PM-resolved**` 标注，PASS。

### 3.5 Windows .cmd 包装 stop 信号

未实测 `sc.exe stop` 是否能优雅停 cmd.exe 包装的 frp-easy.exe 子进程。**MINOR-3**：先按设计实施，QA 加对抗用例；若 fail 走 DESIGN DRIFT 回退 `--config` flag 方案。

### 3.6 verify_all 风险

逐项核 18 项：A.1 secrets scan 正则 `(api_key|secret|password|token)[\s]*[:=][\s]*["'][^"']{8,}["']`，**MINOR-4**：`frp_easy.toml.example` / `README.txt` / `usageText` 中不得出现 `password = "12345678"` 这类 8+ 字符引号串。设计 §10 AC-23 注释"无新 import"不准确（新增 stdlib `flag`），应小修。

### 3.7 实现顺序

§11 五批顺序合理：① main.go flag → ② 服务脚本 → ③ 打包脚本 → ④ DEPLOYMENT.md → ⑤ README，每步前置依赖前一步产出。PASS。

### 3.8 package.sh 应跑 `frp-easy --version` sanity check

设计未列出，**MINOR-5**：建议在前置校验追加 `bin/frp-easy --version >/dev/null` 捕获 ldflags 失效 / 二进制损坏。

---

## 4. 问题分级

### CRITICAL

无。

### MAJOR

- **MAJOR-1 · Partition 文字依据**：`.harness/agents/dev-backend.md` owned paths 不显式包含 `scripts/package.*` / `scripts/install-service.*` / `scripts/uninstall-service.*` / `docs/DEPLOYMENT.md` / `README.md`。PM 必须在 Stage 4 派发文里**显式授权**，或修订 agent.md（前者轻量）。

### MINOR

- **MINOR-1 · `flag.ErrHelp` 未分流**：Developer 实施时需显式 `if errors.Is(err, flag.ErrHelp) { fmt.Fprint(os.Stdout, usageText); return nil }`，再处理"真未知 flag"分支退出码 2。
- **MINOR-2 · AC-22 `comm` 校验不可移植**：改为"`git diff` 人工对照 §7 表"的 SOP。
- **MINOR-3 · Windows .cmd binPath stop 信号**：QA 必须加对抗用例实测。
- **MINOR-4 · A.1 secrets scan 误中风险**：`frp_easy.toml.example` / `README.txt` / `usageText` 避免 `password = "12345678"` 这类 8+ 字符引号串。设计 §10 AC-23 注释应改为"新 import 仅标准库 flag，不引入 go.mod 依赖"。
- **MINOR-5 · package.sh sanity check 缺失**：建议追加 `bin/frp-easy --version` 调用。

---

## 5. 开发期高概率提问预判

| 预测提问 | 预答 |
|---|---|
| dev-backend owned paths 没列 README.md / package.sh，是否要 BLOCKED ON PARTITION？ | 否 — PM 已在派发文中显式授权这 6 个路径归本任务 dev-backend |
| `flag.Parse` 返回 `flag.ErrHelp` 怎么处理？ | `errors.Is(err, flag.ErrHelp)` 分流走 usageText + return nil |
| 仓库根无 LICENSE，打包脚本 WARN 还是 FAIL？ | WARN 跳过（Open Question 7 PM-resolved a） |
| Windows `sc.exe stop` 不能优雅停 .cmd 子进程？ | 先按 §5.2 实施；QA 实测 fail 走 DESIGN DRIFT 引入 `--config` |
| `git describe` 返回 `dev`，AC-2 还过吗？ | 过 — AC-2 接受 dev fallback（R-5 已记录） |

---

## 6. 8 维度审计

| # | 维度 | 结果 | 说明 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | 24 AC + US-1~4 + FR-1~6 完整对齐 |
| 2 | 设计完整性 | PASS | 12 章覆盖全部 FR/NFR/AC |
| 3 | 复用正确性 | PASS | 行号断言全部命中 |
| 4 | 风险覆盖 | WARN | MINOR-3 + MINOR-4 真实风险未列出 |
| 5 | 迁移安全 | PASS | 无 DB schema 改动；卸载脚本不动数据目录 |
| 6 | 边界处理 | WARN | MINOR-1（ErrHelp）+ MINOR-3（stop 信号）两处边界未显式处理 |
| 7 | 测试可行性 | PASS | AC-23 已确认 verify_all 不受影响 |
| 8 | Out-of-scope 清晰 | PASS | §2.2 + §7.1 边界清晰 |

---

## 7. Verdict

**APPROVED FOR DEVELOPMENT**（条件式）

PM 派发 Stage 4 时必须做以下两步：

1. **显式授权** dev-backend 触达：`scripts/{package,install-service,uninstall-service}.{sh,ps1}`、`README.md`、`docs/DEPLOYMENT.md`、`docs/dev-map.md`。同步在 PM_LOG 追加 `2026-05-19 partition-override：…`。
2. **转达 4 项 MINOR 建议**让 Developer 主动落地或在 04_DEVELOPMENT.md 标 DESIGN DRIFT：
   - MINOR-1：`flag.ErrHelp` 显式分流
   - MINOR-3：Windows stop 实测（交 QA）
   - MINOR-4：避免 8+ 字符密码字面量误中 A.1 + §10 AC-23 注释小修
   - MINOR-5：package.sh / package.ps1 加 `frp-easy --version` sanity check
