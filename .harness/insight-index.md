# Insight Index — frp_easy

> 项目踩坑学到的跨任务真相。≤30 行。
> 设计/实现任务开始时读；只在证据支持的意外之后写。
> 规则见 `.harness/rules/05-insight-index.md`。

<!-- 追加新 insight 写下面，一行一条。格式：
-->
- 关键文件里的死代码比普通死代码危害更大：procmgr 的发布订阅让维护者误以为"状态推送已接通"，实则 5 处 emit 广播给空列表。删除型清理必须配 grep 全仓确认零生产消费 + go build/vet 兜底悬挂引用。
- `var _ = pkg.Symbol` 形式的"导入保活 hack"是反模式：它假装某 import 有用，实际掩盖了"当前无用"，并让 goimports/linter 失效。需要时直接加回 import 即可，不该预先保活。
- 删除死代码的死测试导致 go_tests 计数下降，与 B.4 的"测试数只升不降"张力：正解是 PM 显式批准 + baseline.json notes 记录例外（区别于"为过测删活测试"的红线违规）。B.4 仍守住"意外/静默下降"。
- "读时不删过期行、靠后台周期清理"是 session 存储的标准范式，但**周期清理任务必须真的被启动序列拉起**，否则 GetSession 的"不删"优化会让表无界增长。清理 loop 必须随根 ctx 取消（SIGTERM/stopCh）以免 goroutine 泄漏，并把间隔设为包级 var 便于测试注入短间隔 / 长间隔。
- 请求关联 ID 必须用 crypto/rand 而非时间戳：reqID 的唯一价值是日志关联，时间戳在并发下碰撞。项目已有 `auth.GenerateCSRFToken`/`randToken` 的 crypto/rand 范式，middleware 直接用 `crypto/rand`+`hex` 即可，无需引入 auth 依赖。
- 有副作用的代码（子进程 spawn、平台探测、boot 自恢复）也能测：(1) 平台分支抽纯函数 + `t.Setenv` 注入；(2) 真 spawn 用编译独立 helper 程序（当被测代码硬编码自定义 flag、标准 `TestHelperProcess` 不可用时）；(3) 慢 spawn 测试 `testing.Short()` 门控。
- 同步点禁用固定 `time.Sleep`（脆弱），用 poll-until-condition + deadline。
- 加测试常顺带暴露 bug：A-3 写 canceled 用例时发现 retryRestoreLoop 的 canceled-persist 用错 ctx —— "为可测性细看代码"本身就是发现缺陷的高效路径。
- "默认值表单"在加载失败时是 UX 反模式：用户会把空表单/默认值当成"当前真实配置"进而误操作覆盖。正解是**失败态根本不渲染表单**（而非渲染表单+弹 toast）。三态 `v-if/else-if/else` 写成互斥分支 + 断言"error 时 loading 必 false"锁死，避免 loading+error 同显。
- store 的 fetch 失败应**保留旧数据 + 暴露 error ref**，由页面据 error 区分"加载失败"与"空"，而非 `void fetchX()` 吞 promise 让失败静默退化成空态。
- 有状态控件（开关）获取失败必须显式呈现失败态（disabled+tooltip+重试），静默停在默认值 = UI 撒谎。
- 同一数据的展示格式必须跨页统一：`formatTime` 三份实现（裸返回 / 两种 toLocaleString）让用户在监控页见裸 ISO、仪表盘见本地化 —— 抽到单一 util 并全局复用。本地化时间的测试断言必须时区稳定（断言"含年份/非裸 ISO" + 同引擎对齐），不能 hardcode 期望字符串。
- 浅色主题下严禁硬编码 `rgba(255,255,255,*)` 文字色（不可读）；用 `n-text` 的 `depth`/`type` 语义色或 `useThemeVars()` 变量，随主题自适应。
- SPA 内导航必须 `router.push`，`href`/`tag=a` 触发整页刷新丢 Pinia 状态 + 重跑路由守卫。
- `api/client.ts` 这类请求公共层用 `apiClient.defaults.adapter` 合成响应即可走完真实拦截器链（CSRF 注入 / 401 跳转 / 错误解包），afterEach 还原 adapter + CSRF getter 零泄漏 —— 不必 mock 整个 axios，测的是真实拦截器行为。
- 同簇 composable 覆盖要对账（log/ 下 5 个 composable 此前漏了 useLogLevelFilter 一个）；"同簇漏一两个"是覆盖不均的典型信号，补齐时按目录清点。
- "契约文件描述全部路由"的声明（README/openapi）必须有机制守门，否则新路由（service-status）漏登、声明变假却无人发现。理想是 verify_all 加一道"router.go 路由集 == openapi paths 集"的静态闸门（本任务未做，建议未来 T 候选）。
- 文档自相矛盾（dev-map 树 vs prose）的根因是"树是手画快照、prose 增量更新"两条更新路径不同步。修法是树只放结构、增删数字一律指向单一权威（router.go 行），避免在两处各维护一份会漂移的计数。
- 大型冻结文档（project-status/architecture.html）与其重生成，不如加显眼时效声明 + 指向持续更新的 dev-map/tasks —— 低成本止损"半年前快照被当现状"。
- 错误/取消路径上的"最终状态持久化"必须用 **detached context**（`context.Background()` + 自带超时），不能复用触发该路径的已取消 ctx —— 否则这条"我被取消了"的记录本身会被取消连累，永远写不出去。这是 ctx 取消语义的常见陷阱：取消应停止"进行中的工作"，但不应阻止"记录我已停止"这一收尾动作。
- 真机场景下该 canceled 写仍与进程 shutdown 的 store.Close 存在竞态（best-effort）；对"上次自恢复结果"这类观测字段可接受，若要强保证需在 run() 用 waitgroup 等 retry goroutine 收尾后再 Close（本任务未做，超出范围）。
- 2026-05-30 · `archive-task.sh` 的 Insight 收割 awk 正则在 T-054 补齐单 token 前缀容错（`/^##[[:space:]]+([^[:space:]]+[[:space:]]+)?Insights?[[:space:]]*$/`），与 `archive-task.ps1:48` 对齐——T-028 遗留半年的双实现不对称债清零；二者前缀容忍集合现完全相同（裸/§N/N. 前缀命中，双 token 与非 Insight 标题不命中），未来改任一实现须同步另一实现并逐片段对账（awk POSIX ERE `([^[:space:]]+[[:space:]]+)?` ≡ .NET `(?:[^\s\n]+\s+)?`，因 awk 单行 record 内 `[^[:space:]]`≡`[^\s\n]` 且 awk 不暴露捕获组）· evidence: T-054 scripts/archive-task.sh L47-53 + archive-task.ps1:48 + 06 AT-1/AT-4 反向证伪汇总表
- 2026-05-30 · "harvest 工具自身的正则改动"无法在 role-collapsed PM 上下文（无 Bash）自验，但反向证伪的**确定性**让这不构成阻塞：纯文本 awk 匹配无随机/IO/竞争，预期输出可由 POSIX ERE 语义逐 fixture 推导并写成"执行规格"（OLD vs NEW 命中数汇总表）交 orchestrator 真跑核对——比"跑一次拿日志"更可审计（规格先于执行，结果偏离即回退信号）· evidence: T-054 06 §Adversarial tests 预期汇总表 + Bash 工具实测不可用（No such tool available: Bash）
- 2026-05-30 · frps 运行态代理 handler 的 `{type}`/`{name}` path 参数注入防御正确分层是"client 层 `url.PathEscape` 作根防御（即使 handler 漏校验也安全）+ handler 层白名单校验前移到构造 client 之前（非法输入零成本早返、不触上游）"双层；测试断言上游收到的 path 必须用 `r.RequestURI`（请求行原文逐字节保留）而非 `r.URL.EscapedPath()`（后者在 RawPath 与 Path 默认转义不同如含 `%2F` 时虽返 RawPath 但语义上是"可被规范化重算"的，不如 RequestURI 确定）· evidence: T-055 frpsadmin/client.go Proxies/ProxyDetail/Traffic + client_test.go::TestProxyDetail_PathEscape 表驱动 6 子例 + handlers_server_runtime.go::serverRuntimeProxyDetail 校验前移
- 2026-05-30 · 后端 500 兜底"固定面向用户文案 + 原始 error 进 logger"的统一 helper（`writeInternalError`）解决了一个隐性可测性问题：当 500 兜底由具体类型（`*procmgr.Manager` / `*downloader.Manager`，其方法仅返 sentinel 且 handler 已前置校验）守护时，该分支在黑盒 HTTP 测试中事实上不可达——抽纯 helper 直测（`httptest.NewRecorder` + 捕获型 `slog` buffer 断言"响应不含 leak 子串 + cause 进日志"）是零生产行为变更、零依赖、零扩散的最小可测路径，优于为测试引入 mock 接口（会扩散到 5+ 调用点）· evidence: T-055 handlers_proc.go::writeInternalError + handlers_hygiene_test.go::TestWriteInternalError_FixedMessage_NoLeak / TestMapProxyWriteError_Fallback_NoLeak
- 2026-05-30 · 给 Dashboard 破坏性按钮加二次确认时，e2e 烟雾测试（03-dashboard TC-04/TC-05）只断言文案可见 + 退出登录、**不点击停止/重启按钮**，故此类"破坏性按钮加确认"UI 改动对 e2e 零影响——评 e2e 回归风险应先 grep e2e spec 确认是否真点击该按钮，多数烟雾测试不点破坏性按钮，无需改 e2e；只有当 e2e 实际点击该按钮时才需在用例内补"先点确认"步骤 · evidence: web/tests/e2e/03-dashboard.spec.ts TC-04/TC-05 + Dashboard.vue requestStop/requestRestart
- 2026-05-30 · 顶级路由页面（不嵌 AppLayout 的 /wizard /login /setup）无法依赖 AppLayout 顶栏的全局横幅/入口（缺失横幅、版本、登出），任何"引导用户去用顶栏入口"的提示在这些页面必须就地复刻或显式说明顶栏在别处——frp_easy 的 Wizard 完成态因此需自带二进制缺失警告而非指望顶栏横幅。规则：判断"某全局 UI 在某页是否可见"先查 router.ts 该路由是否为 `/`（AppLayout）的 children，平级顶级路由一律不可见 · evidence: T-057 router.ts:10 /wizard 与 / 平级 + Wizard.vue step3 自带 warning alert
- 2026-05-30 · "保存配置"与"运行就绪"是两个正交关注点，向导/表单完成流必须分别给信号：配置 PUT 成功 ≠ 进程能启动（二进制可能缺失）。把二者耦合成单一 success toast + 自动跳转会制造"配好了却跑不起来"的首用挫败。正确范式是完成保存后 `await fetchReady()` 刷新就绪态、按所选角色求缺失交集（per-role 而非整体 length>0，避免无关缺失误报），缺失则不自动跳走、就地警告 + 手动入口；不缺失维持原自动跳转（保后向兼容 + e2e 不受影响）。binWarning 用 ref 定格快照而非 computed，避免完成后后台状态变化抹掉已展示的警告 · evidence: T-057 Wizard.vue:322-334 完成分支 + missingForRole 交集 + Adversarial 定格快照证伪
- 2026-05-30 · 前端"剪贴板复制"在内网 http（非安全上下文）部署下 `navigator.clipboard.writeText` 必 reject——任何复制按钮都必须配 `document.execCommand('copy')` + 临时 textarea fallback，且 fallback 失败要 `message.error` 不能静默 `catch {}`；项目已验证范本是 LogViewer.vue::onCopy（try clipboard → success；catch → textarea+execCommand → ok?success:error；finally removeChild），三处复制点（LogViewer/FirewallHint/PublicIpDetector）应统一此模式。测试模拟须 `Object.defineProperty(navigator,'clipboard',{value:{writeText:mock}})` + 显式装 `document.execCommand`（happy-dom 默认无），断言走 useMessage 单例 spy（`vi.mock('naive-ui')` 工厂内单例 + 导出 `__messageSpies` 取回，否则每次 `useMessage()` 返回新对象无法断言）· evidence: T-058 FirewallHint.spec/PublicIpDetector.spec + LogViewer.vue:147-171 范式
- 2026-05-30 · 表单页"重置/重新加载"按钮防误丢未保存编辑的低成本范式：加载成功时 `loadedSnapshot = { ...form.value }` 存标量字段快照，点击时 `isDirty()`（逐字段浅比较）则弹 ConfirmDialog 确认才 reload、不 dirty 直接 reload（不打扰）；文案"重置"应改"重新加载"避免用户误以为重置为默认值。dirty 检测**刻意不覆盖**有独立增删行显式操作的子编辑器（如 AllowPortsEditor，单向数据流 insight L13 + 纳入会扩散），用户对其改动感知强、误丢风险低，可接受局限 · evidence: T-058 Server.vue/Client.vue isDirty + handleReloadClick + 各 spec dirty/取消/确认 apiGet 调用计数证伪
