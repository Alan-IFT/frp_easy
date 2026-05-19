# 01 — 需求分析：T-009 polish-pass

> Stage 1 of 7-stage `/harness` 流水线 · 中文 · PM 亲撰（清理任务，省派遣开销）

---

## 1. 任务背景

T-008 deploy-kit 提交 message 声明 "verify_all PASS:18"，但事后扫描发现：
- 在 PowerShell 7+ 下跑 `scripts/verify_all.ps1`，**C.1 E2E smoke (playwright) FAIL**，最终 17 PASS + 1 FAIL。
- 在 Git Bash 下跑 `bash scripts/verify_all.sh`，仍是 18 PASS。

两个 shell 行为不一致，违反 "verify_all PASS 是声明完成的硬闸门" 红线。
同时扫描出 2 项较低优先级的过程性遗留（T-001 未归档、dev-map 日文注释）。

用户授权 PM 一并清理。

---

## 2. 用户故事（PM 自构）

- **US-1（Windows + PowerShell 开发者）**：我在 PowerShell 跑 `.\scripts\verify_all.ps1`，所有 18 项都应该 PASS，不需要切换到 Git Bash。
- **US-2（项目维护者）**：tasks.md 与 docs/features/_archived/ 应保持一致，DELIVERED 任务的文档应已归档。
- **US-3（新加入贡献者）**：dev-map.md 是项目入口导航，所有解释应使用同一种语言（项目规定中文）。

---

## 3. 验收标准（AC）

### 3.1 主目标：Playwright cross-shell parity

| AC | 描述 |
|---|---|
| AC-1 | `.\scripts\verify_all.ps1`（PowerShell 7+）执行后输出 `PASS: 18 / WARN: 0 / FAIL: 0 / SKIP: 0`。 |
| AC-2 | `bash scripts/verify_all.sh`（Git Bash）执行后输出 `PASS: 18 / WARN: 0 / FAIL: 0 / SKIP: 0`。 |
| AC-3 | 修改不引入新 Go 依赖；不改业务代码（cmd / internal / migrations / web/src）。 |
| AC-4 | playwright.config.ts 的 webServer 跨平台启动：Linux/macOS 走 `bash scripts/start-e2e-server.sh`，Windows 走 `pwsh -File scripts\start-e2e-server.ps1`（或等价物，详 02 设计）。 |
| AC-5 | start-e2e-server.ps1 行为对齐 .sh 版：自动构建 + 临时 DataDir 隔离 + FRP_EASY_CONFIG 注入 + Playwright 可通过 SIGTERM 终止。 |

### 3.2 次目标：归档与文档清理

| AC | 描述 |
|---|---|
| AC-6 | `docs/features/web-ui-mvp/` 已通过 `scripts/archive-task.sh --task web-ui-mvp` 移到 `_archived/`。 |
| AC-7 | `.harness/insight-index.md` 不因本次归档新增重复条目（archive-task 自动 harvest 时若 07_DELIVERY.md 无 `## Insight` 段则不写入）。 |
| AC-8 | `docs/dev-map.md` 中 web/ 子树所有日文注释翻译为中文，技术含义保持完全等价（dev-frontend 区域命名 `main.ts` / `App.vue` 等等不变）。 |

### 3.3 红线（不可违反）

| AC | 描述 |
|---|---|
| AC-9 | 不修改 `.claude/`、`CLAUDE.md`、`.github/copilot-instructions.md` 这三类生成 / 静态文件。 |
| AC-10 | 不修改已合并的 migration 文件 `migrations/0001_init.*.sql`。 |
| AC-11 | 不更改任何业务代码模块（`internal/*` `cmd/*` `web/src/*`）。 |
| AC-12 | 任务结束时 `docs/tasks.md` 更新 T-009 为 `done` 并加入"已完成"表。 |

---

## 4. 范围（在 / 不在）

### 在范围
- `playwright.config.ts`（webServer 跨平台）
- 新增 `scripts/start-e2e-server.ps1`（对应已有的 .sh）
- `docs/dev-map.md`（日文 → 中文）
- `docs/tasks.md`（归档 + 状态）
- `docs/features/web-ui-mvp/` → `_archived/web-ui-mvp/`
- 本任务 `docs/features/polish-pass/` 7 阶段文档

### 不在范围
- 不重写 Go 业务代码
- 不改 frontend src（即使 dev-map 改了，引用的代码不动）
- 不处理 deploy-kit 的 "release-smoke 5 AC"（仅发布前关注，docs/DEPLOYMENT.md 已留入口）
- 不动 `.harness/rules/*.md`、`.harness/agents/*.md`、`.claude/`

---

## 5. 隐含约束（环境）

- 用户机：Windows 11 + PowerShell 7+ + Git Bash（MSYS2）+ Go 1.22+ + Node 18+
- `bash.exe` 在 PowerShell 默认 PATH 中解析到 WSL shim（`C:\Users\yangx\AppData\Local\Microsoft\WindowsApps\bash.exe`），不是 Git Bash。
- 不能假设用户已安装 WSL 发行版。
- PowerShell 启动脚本必须用 pwsh 7+ 兼容写法（项目其他 .ps1 已是这个标准）。

---

## 6. 风险与未决项

| # | 风险 | 缓解 |
|---|---|---|
| R-1 | start-e2e-server.ps1 的临时目录清理在 Playwright SIGKILL 时会泄漏 | 用 `$env:TMP\frp-easy-e2e-<pid>` + Playwright reuseExistingServer 复用规则，泄漏可接受（每次 PID 不同）；或注册 `trap { cleanup } ... ` 模式 |
| R-2 | go build 在 PowerShell 下产物路径是 `bin\frp-easy.exe` 还是无后缀 | 沿用 .sh 版同样的"`-o bin/frp-easy` 后探测 `.exe` 后缀" 模式 |
| R-3 | Playwright Node 子进程在 Windows 上对 .ps1 文件的执行策略可能阻挡 | 用 `pwsh -ExecutionPolicy Bypass -File <path>` 绕过 |
| R-4 | 项目内已有 Powershell + Git Bash 双脚本配对惯例（package.sh/ps1, install-service.sh/ps1）—— 新增 start-e2e-server.ps1 必须对齐这个惯例 | 设计阶段确认 |

无未决问题（Open Questions），PM 已按用户授权全部预决策。

---

## 7. Verdict

**READY FOR DESIGN** — 进入 Stage 2 Solution Architect。
