# BATCH_PLAN — ux-be-refinement-2026-05

> 2026-05-30 创建。用户高层目标（与 project-optimization-2026-05 同款指令的**第二轮**）：**优化后端 + 前端 UI 交互**，决策原则：**用户体验好 / 符合软件工程标准 / 长期易使用易维护**。
>
> 用户授权 AI 全权决策（设计 + 实现 + commit + push）。范围：聚焦高价值，**拒绝为干净代码库制造 churn**。

## 决策摘要（AI 视角）

启动前做了 3 维度证据审计（后端 Go 健康 / 前端 UX / 契约与可维护性）+ 真实 `verify_all` 全量基线。

**关键结论：项目当前已处于优秀状态（基线 PASS 32 / WARN 0 / FAIL 0，734 测试，含 e2e）。三份审计一致认定"无 P0 救火项"。** 上一批次（project-optimization-2026-05，同一天完成）已恢复红基线 + 加固闸门 + 全面补测 + 诚实三态 + 删死代码。因此本轮**刻意收敛**为少量"改了有明确价值"的项，并显式记录**故意不做**的洁癖/过度工程项。

**故意不做（避免 over-engineering 违反"软件工程标准"原则）**：
- router↔openapi 静态闸门（当前 30 path 100% 一致，实现略脆、收益中等）→ backlog。
- `useServiceStatus` 改 `extractErrorMessage`（该端点设计上不返结构化 5xx，纯洁癖无行为收益）。
- `proxies` store CRUD 不写 error ref（经核实是**正确的有意分工**：CRUD 用瞬时 toast，fetch 用持久页面态；改了反引入回归）。
- FirewallHint 补 Windows 防火墙命令（中等成本、前端难可靠知运行平台）→ backlog。
- 窄屏顶栏横幅退让 / 预设点击反馈（边缘场景，P2）。
- `frp_easy.toml` "默认值不一致"（经核实是 .gitignore 的本地运行时产物，不进契约）。

## Baseline 状态（2026-05-30 batch 启动时）

- `bash scripts/verify_all.sh`（全量含 e2e）：**PASS 32 / WARN 0 / FAIL 0 / SKIP 0**。
- baseline.json：test_count=734（go_tests=308 + frontend_tests=426）。
- **回归判定**：任一任务跑完后 verify_all 出现**新 FAIL（超过启动基线）即停批**。

## 执行模型决策（沿用上一批次硬教训）

上上批次带红交付的根因是 **verify_all 闸门被角色扮演而非真跑**（insight L14 role-collapse、L46 三洞叠加）。本批次 **batch orchestrator（拥有 Bash）在每个任务后真正运行 `scripts/verify_all`** 作为硬闸门，绝不依赖角色扮演的 QA。pm-orchestrator 子 agent 产出 7 阶段文档 + 代码改动；**最终验证一律由 orchestrator 真跑**。每个任务 verify PASS 后由 orchestrator 提交（conventional commit），批次结束统一 `git push origin main`。

## 任务表

| ID | Slug | Goal | Mode | Depends on | Status |
|---|---|---|---|---|---|
| T-054 | archive-task-sh-regex-align | 修 `scripts/archive-task.sh` 的 awk Insight 标题正则不容错前缀缺陷（insight L23 长期债），对齐 `.ps1` 的 `^##\s+(?:[^\s\n]+\s+)?Insights?\s*$` 容错版；按 insight L26/L46 双实现对账 + 反向证伪（造 `## §N Insight` 前缀标题 → harvest 命中 → 裸标题仍命中）。 | full | — | done |
| T-055 | backend-api-hygiene | 后端 API 卫生两项：(1) frps 运行态代理端点 `{type}` 用 `frpsProxyTypes` 白名单校验、`{name}` 经 `url.PathEscape` 编码（堵 `internal/frpsadmin/client.go` doGet 纯字符串拼 path 的注入缺口）；(2) handler 停止向前端透传内部错误细节（procStop / mapProxyWriteError 兜底 / downloadBin default 改固定中文文案，细节只进 logger；保留 uploadBin errno 透传——B-A.12 有意决策）。补对应测试。 | full | T-054 | done |
| T-056 | proc-stop-destructive-confirm | Dashboard 的 frps 停止/重启加二次确认（复用 `ConfirmDialog`，文案点明"将中断当前所有穿透连接"），与"删除代理规则"的破坏性确认标准对齐；frpc 停止可选同款。不给"启动"加确认（非破坏性）。补组件测试。 | full | T-055 | done |
| T-057 | binary-missing-onboarding-ux | 首次使用体验两项：(1) Dashboard 二进制缺失提示从"拷文件重启"硬核说明改为指向/复用 AppLayout 顶栏既有的一键下载 + 手动上传入口（信息架构一致）；(2) Wizard 完成前校验所选角色对应二进制是否存在，缺失则在向导内提示并给入口，避免"配好了却跑不起来"困惑。补测试。 | full | T-056 | done |
| T-058 | frontend-interaction-polish | 前端交互一致性小修打包：(1) Server/Client「重置」文案改"重新加载"并在表单 dirty 时二次确认（避免静默丢弃未保存编辑）；(2) FirewallHint / PublicIpDetector 剪贴板写入失败不再静默——抄 LogViewer 的 execCommand fallback + `message` 提示（内网 http 部署命中率高）；(3) 清理 Wizard.vue v-if/v-else 两分支文案相同的死分支。补测试。 | full | T-057 | pending |

> 注：T-054 置首因 batch 收尾要跑 `archive-task` 收割 11+5 个已完成任务的 insight，正则修复必须先落地以防 harvest 丢失（虽已核实现存 11 个标题均裸 `## Insight`，仍偿还长期债）。其余 4 个任务彼此独立，按"后端 → 前端破坏性确认 → 前端引导 → 前端打磨"价值/风险递减排序。

**Topo order**：T-054 → T-055 → T-056 → T-057 → T-058（sequential）。

## 决策原则映射

| 任务 | 用户体验好 | 软件工程标准 | 长期易使用易维护 |
|---|---|---|---|
| T-054 | — | 双实现对账（消 .sh/.ps1 不对称） | 偿还 insight L23 债；解锁干净 harvest |
| T-055 | 不向用户暴露 SQL/errno 黑话 | 输入校验 + 错误响应一致性 | 不依赖驱动错误文本 |
| T-056 | 破坏性操作有确认，避免误停断连 | 站内确认标准统一 | 复用既有 ConfirmDialog |
| T-057 | 缺二进制时给可操作入口而非弯路 | 信息架构一致 | 向导即时校验降首用挫败 |
| T-058 | 复制有反馈、不静默丢编辑 | 错误处理模式统一 | 抄既有正确范式 |

## strong-signal 停止条件

- 任一任务跑完后 verify_all 出现**新 FAIL（超过该任务启动时基线）**。
- 任一 pm-orchestrator 返回 FAILED verdict / 同 stage 3 次回退。
- `.harness/intervention.md` 出现 STOP。
- 安全 hook 拦截 destructive Bash 调用。

## 提交 / 推送策略（用户授权）

- 每个任务 verify_all 真跑 PASS 后由 orchestrator 提交到 main（`fix/feat/refactor/docs(T-NN): slug — 简述`）。
- 批次结束：归档全部已完成任务（prior 11 + 本轮 5）收割 insight → 触发 insight-index 旋转回 ≤30 行 → 写 BATCH_REPORT.md → `git push origin main`（触发 release.yml 滚动发布）。
