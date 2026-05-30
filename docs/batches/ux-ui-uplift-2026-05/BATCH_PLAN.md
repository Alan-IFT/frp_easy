# BATCH_PLAN — ux-ui-uplift-2026-05

> 2026-05-30 创建。用户高层目标（与 project-optimization-2026-05 / ux-be-refinement-2026-05 同款指令的**第三轮**）：**优化项目、提升后端与前端 UI、提升用户体验**，决策原则：**用户体验好 / 符合软件工程标准 / 长期易使用易维护**。
>
> 用户授权 AI 全权决策（设计 + 实现 + commit + push），只看结果是否符合需求。范围：聚焦高价值，**拒绝为干净代码库制造 churn**（over-engineering 本身违反"软件工程标准"原则）。

## 决策摘要（AI 视角）

启动前做了 3 维度独立证据审计（前端 UI/视觉/a11y/响应式 · 前端 UX 流程/信息架构 · 后端 Go/可维护性）+ 真实 `verify_all` 全量基线。

**关键结论：项目当前已处于优秀状态（基线 PASS 32 / WARN 0 / FAIL 0，822 测试，含 e2e）。** 前两轮（同一天）已做透**正确性 / 卫生 / 交互逻辑**维度——红基线恢复、闸门加固、全面补测、诚实三态、破坏性二次确认、dirty 防误丢、错误 sentinel 化、500 兜底脱敏、删死代码。

因此本轮**刻意把焦点放在前两轮欠覆盖、且正是本次用户措辞（"提升前端 UI / 用户体验"）所指向的维度**：视觉设计质量、可访问性、响应式布局、正向 onboarding 引导。后端只纳入两条有书面证据的"对称漏项"。所有候选项均**有 `文件:行号` 证据支撑**，无一是为凑数而造。

**故意不做（避免 churn / 维持前两轮的刻意决策）**：
- router↔openapi 静态闸门：预防性、当前路由集 100% 一致，前两轮已明确"新增路由频率上升时再加"，本轮无新增路由 → 维持 backlog。
- "重新运行向导"入口：向导是首用脚手架，配置页可直接改，重入需求弱。
- 术语微调（运行态语境的 "proxy" vs 配置侧"代理规则"）：已基本分层统一，纯洁癖。
- 启动后 2s 轮询的"即时 pollStatus"补偿：当前 toast 诚实反映返回态，纯 polish。
- 复制按钮"已复制 ✓" emoji 装饰统一：message 已播报，视觉冗余，非缺陷。
- mapProcErr 之外更激进的 procmgr 错误体系重构：超范围。

## Baseline 状态（2026-05-30 batch 启动时）

- `bash scripts/verify_all.sh`（全量含 e2e）：**PASS 32 / WARN 0 / FAIL 0 / SKIP 0**。
- baseline.json：version=27，test_count=822（go_tests=322 + frontend_tests=500）。
- HEAD：`10262ef`（main，clean）。
- **回归判定**：任一任务跑完后 verify_all 出现**新 FAIL（超过启动基线）即停批**。

## 执行模型决策（沿用前两批硬教训）

带红交付的根因是 **verify_all 闸门被角色扮演而非真跑**（insight L31/L46）。本批次 **batch orchestrator（拥有 Bash）在每个任务后真正运行 `scripts/verify_all`** 作为硬闸门，绝不依赖角色扮演的 QA。pm-orchestrator 子 agent 产出 7 阶段文档 + 代码改动；**最终验证一律由 orchestrator 真跑**。每个任务 verify PASS 后由 orchestrator 提交（conventional commit），批次结束统一 `git push origin main`（触发 release.yml 滚动发布）。

> 注：用户 PowerShell 有 deny 规则（不绕过）；`.ps1` verify 路径与 `.sh` 严格对称，本会话以 `bash scripts/verify_all.sh` 全量真验。`-race` 因本机无 C 编译器（cgo）跑不了，后端并发任务仅静态核验。

## 任务表

| ID | Slug | Goal | Mode | Depends on | Status |
|---|---|---|---|---|---|
| T-062 | onboarding-next-step-guidance | 补正向 onboarding 引导与跨页连通，消除"配好了不知道下一步"断点：(1) Wizard 完成页（frpc/both 角色）+ Client.vue 保存成功后追加"下一步：前往『代理规则』添加转发规则 →"链接按钮（`router.push`，非 href）；(2) Proxies 保存成功/空态引导去 Dashboard 启动 / ServerMonitor 看运行态；(3) Server 配置页加"查看运行态 →"链接到 `/server/monitor`，与 ServerMonitor→Server 形成双向连通；(4) ProxyForm 远程端口字段加 help"需在服务端『端口策略』允许范围内"（纯文案，不做跨页校验联动）；(5) Wizard `both` 模式两个 token 都非空且不相等时给非阻断 warning（token 不一致是 frp 连不上头号原因）。纯文案 + `router.push` + 一处条件比较，不碰后端/store/数据流。 | full | — | in-progress |
| T-063 | loginfail-kv-purge | 关闭 `loginfail.<ip>` 限流 KV 永久滞留（T-046 修了 sessions 表无界增长，未修此对称面；有跨多轮书面 backlog 证据）。storage 层加按前缀清理过期能力（如 `PurgeKVByPrefixOlderThan` 或 `RateLimiter.PurgeExpired`，所有 SQL 留 storage 层），挂到既有 `purgeSessionsLoop` 同一 ticker（不新增 goroutine），过期判定语义与 ratelimit 窗口一致。补 Go 测试（`t.TempDir` 隔离）。 | full | — | pending |
| T-064 | menu-icons-and-a11y | 修侧边栏菜单图标 + 补 a11y：(1) 消除 `⚙` 字形被"服务端配置"与"设置"重复使用（折叠态同形→误点），并给每个 menu item 无障碍名（aria-label/title）；(2) 全屏日志滚动容器加 `tabindex`/`role` 让键盘可滚（LogList.vue）；(3) 复制按钮瞬时态加 `aria-live`/`role=status` 供屏幕阅读器播报。**偏好零新依赖**（不引入 @vicons 等重型图标库，除非 7-stage 评审判定明显更优）。补组件测试。 | full | — | pending |
| T-065 | mapprocerr-sentinel-hygiene | 把 `handlers_proc.go:mapProcErr` 的脆弱 `strings.Contains`（stopping/starting/running 文本匹配 → 409 PROC_BUSY）收口为 procmgr sentinel（`ErrBusy`），与 T-059 `mapProxyWriteError` / `binloc.ErrBinMissing` 范式一致；500 路径改走 `writeInternalError`（不泄露内部 error 文本，与 T-055 一致）。保留 uploadBin errno 透传（B-A.12 有意决策不动）。开工前 grep 出所有断言旧文本的测试纳入同分区（insight L35），断言更新需 PM_LOG 显式批准（红线 3 例外、有意改变行为），用例数不降。 | full | — | pending |
| T-066 | dark-theme-support | 加生产暗色主题，激活已投入一半的主题感知基建（日志子系统 `useThemeVars` + 暗色测试已就绪，但全站只能浅色）：(1) App.vue `useOsTheme()` 跟随系统 + 顶栏手动切换按钮 + localStorage 持久化偏好；(2) 把散落硬编码 hex（Login/Setup/Wizard 背景、`#888` 文字、品牌绿 `#18a058`、ServiceStatusCard 橙）改成主题 token / `useThemeVars` / CSS 变量（insight L16 浅色禁硬编码同源）；(3) 暗色下逐页可读性核验。补测试（参考 LogViewer.spec 用 `darkTheme` 范式）。 | full | T-064 | pending |
| T-067 | responsive-layout | 让应用外壳在窄屏/移动端可用（手机查看进程状态/重启穿透是真实场景）：(1) 侧边栏按断点自动折叠（`useBreakpoint`）；(2) 顶栏横幅（品牌+版本+二进制缺失横幅+用户名+退出）窄屏不溢出、换行得当；(3) 表单固定像素宽（Server 360px / Client 300px 等）改 `max-width` 不横向溢出。内容栅格已响应式（`responsive="screen"`）不动。补测试/视觉核验。 | full | T-066 | pending |

**Topo order**：T-062 → T-063 → T-064 → T-065 → T-066 → T-067（sequential）。

> 排序逻辑：按"价值×安全"递减前置，并尊重 AppLayout 分层（图标结构 T-064 → 主题着色 T-066 → 响应式布局 T-067，三者均碰 AppLayout，分层推进避免概念混杂）。后端两项（T-063/T-065）彼此独立、与前端零冲突，插在前端任务之间。T-066 依赖 T-064（主题切换按钮落在 AppLayout 顶栏、与图标修复同区），T-067 依赖 T-066（响应式新样式应使用主题 token 而非硬编码）。

## 决策原则映射

| 任务 | 用户体验好 | 软件工程标准 | 长期易使用易维护 |
|---|---|---|---|
| T-062 | 消除"配好不知下一步"迷茫、token 不一致预警 | 跨页连通而非死路、SPA 用 router.push | 纯文案/导航，零数据流耦合 |
| T-063 | —（后台卫生） | 资源生命周期对称（sessions + loginfail 都清理） | 偿还跨轮书面 backlog；复用既有 ticker |
| T-064 | 折叠态不再两个齿轮误点、键盘可达 | a11y 无障碍名 + 键盘语义 | 零新依赖、与既有 a11y 范式对齐 |
| T-065 | 不向用户暴露内部 error 黑话 | 收口脆弱文本匹配为 sentinel | 不依赖 procmgr 内部文本、与 T-059 对称 |
| T-066 | 暗色主题（运维高频诉求） | 颜色统一走主题 token | 激活已有投资而非新造、消硬编码债 |
| T-067 | 移动端可用 | 响应式外壳补齐 | max-width 替固定宽，随主题/视口自适应 |

## strong-signal 停止条件

- 任一任务跑完后 verify_all 出现**新 FAIL（超过该任务启动时基线）**。
- 任一 pm-orchestrator 返回 FAILED verdict / 同 stage 3 次回退。
- `.harness/intervention.md` 出现 STOP。
- 安全 hook 拦截 destructive Bash 调用。

## 提交 / 推送策略（用户授权）

- 每个任务 verify_all 真跑 PASS 后由 orchestrator 提交到 main（`fix/feat/refactor/docs(T-NN): slug — 简述`）。
- 批次结束：归档全部本轮已完成任务收割 insight → 维持 insight-index ≤30 行 → 写 BATCH_REPORT.md → `git push origin main`。
