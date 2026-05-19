---
task: hardening-pass-audit
task_id: T-007
stage: 01_requirement_analysis
mode: full
date: 2026-05-19
author: requirement-analyst
status: READY
---

# 01 需求分析 — T-007 hardening-pass-audit

## 任务摘要

针对 frp_easy 已交付系统的三方并行审计（安全 / 后端代码质量 / 前端 UX）结果，PM 已挑选 9 项聚焦的强化修复点：4 项后端安全、2 项后端质量、3 项前端 UX。本任务**不引入新功能**，目标是在不改变外部契约（HTTP API schema、DB schema、CLI flag）的前提下消除明确缺陷，提升单机部署的最低安全水位、错误反馈完整性与维护性。

## 范围内（in scope）

### IS-1 — frpconf AtomicWrite 临时文件权限收紧到 0o600

- **What**：`internal/frpconf/render.go` 的 `AtomicWrite` 在 `os.CreateTemp(dir, ".frpconf-*.tmp")` 后，立即把临时文件权限降到 0o600（owner 读/写，其他用户无访问）。同样的紧权限要求传播到 rename 后的目标文件（如果当前实现未显式设置目标 mode，则在 rename 后或 close 前 chmod 0o600）。
- **Why**：当前默认权限受 umask 影响（典型 0o600/0o644/0o666），在多用户主机上同主机其它本地用户可读取临时窗口期内含 `frps_token` 的 frpc.toml / frps.toml 配置渲染中间产物。临时窗口虽短，但对常驻多用户主机仍是可观察的可重现窗口。
- **User-visible behavior**：无 UI 变化。POSIX 主机上 `ls -l` 临时文件 / 最终 toml 文件显示 `-rw-------`。Windows ACL 不受 Go chmod 直接影响（Go 仅控制 user-read/write 位映射），本项不要求改变 Windows ACL 行为；以 POSIX 系统为主要验证目标。
- **关联 insight**：`insight-index.md` 2026-05-16 条目（Windows os.Rename 不能覆盖已存在文件）—— 本项必须保留 AtomicWrite 现有"先写临时→Rename"的不变量；权限收紧仅在写入路径内插入，不引入新的 Remove+Rename 序列。

### IS-2 — UI 日志文件 ui.log 权限收紧到 0o600

- **What**：`cmd/frp-easy/main.go` 中 `os.OpenFile(uiLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)` 的 mode 改为 `0o600`。范围仅限 UI 进程自己控制能写权限的日志文件（ui.log）；frpc.log / frps.log 由 procmgr.supervise 内部 OpenFile 创建的日志同步改为 0o600（见 `internal/procmgr/manager.go` `supervise` 函数中 `os.OpenFile(logPath, ..., 0o644)`）。**子进程 frpc/frps 自身写日志的行为不在本项范围**——子进程通过 toml 中 `log.to` 自行打开文件，权限由上游 FRP 控制，本任务不修补上游。
- **Why**：ui.log 含 request body redact 后的结构化日志、frpc/frps 错误回环行；frpc.log / frps.log 含连接日志。多用户主机上需限制为 owner-only。
- **User-visible behavior**：无 UI 变化。POSIX 主机上 `ls -l <log>` 显示 `-rw-------`。

### IS-3 — 新增 SecurityHeaders 中间件

- **What**：在 `internal/httpapi/` 中新增一个中间件函数（命名 `SecurityHeaders`），对**所有**响应（包括 /api/v1/health、SPA 静态资源、错误响应）写出以下三个响应头：
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Referrer-Policy: no-referrer`
- 该中间件挂在 chi 路由链上，位置满足"对全部出站响应生效"（包括 /api/v1/health 与 SPA fallback）。
- **Why**：当前 HTTP 响应无任何安全相关头，浏览器在异常 content-type 或被嵌入 iframe 等场景下行为不受约束。三个头是零配置、零外部依赖的最低基线。
- **User-visible behavior**：浏览器开发者工具 Network 面板任一响应均含三个 header；功能行为不变（不会拒绝任何请求，不会改变状态码）。

### IS-4 — 日志读取端点单次响应字节上限

- **What**：`internal/httpapi/handlers_logs.go` 中 `GET /api/v1/logs/{kind}?offset=...` 增量读路径，调用 `logtail.ReadFrom(path, off)` 后或在读时，限制**单次响应**的 `data` 字段字节长度上限为 **2 MiB（2 × 1024 × 1024 = 2097152 字节）**。当文件相对 offset 的剩余字节超过上限时：
  - 仅返回前 2 MiB 字节；
  - `nextOffset` 设为 `offset + 实际返回字节数`（使客户端可在下次请求接续读取）；
  - 响应仍走原 `LogsIncrementalResponse` 结构（不引入新字段、不改 schema）。
- 上限定义为**包内常量**（如 `const maxLogChunk = 2 * 1024 * 1024`），便于测试覆盖与未来调整。
- 仅作用于 `?offset=` 增量模式；`?lines=N` 模式已经有 N≤5000 上限，本任务不动。
- **Why**：当前 ReadFrom 对单次响应字节无上限，恶意客户端 `offset=0` 对一个 100 MiB 日志可让进程一次性把全部内容读入内存 + 序列化进 HTTP body，DoS 风险显式且廉价。
- **User-visible behavior**：客户端轮询日志时一次最多收到 2 MiB；前端按 `nextOffset` 继续轮询即可还原完整日志流。前端代码若已经按 `nextOffset` 轮询则无需修改（参见现 Logs 页面实现）。

### IS-5 — procmgr.Start() 统一 defer unlock

- **What**：`internal/procmgr/manager.go` 的 `Start(kind)` 函数当前混用"手动 `m.mu.Unlock()` + 早返回"与"持锁中再操作"两种模式（如 lines 176-217 区间多次显式 unlock）。重构为单一 `defer m.mu.Unlock()` 模式：
  - 入口加锁后立即 `defer Unlock`。
  - 所有早返回（不变更状态）不再手动 unlock。
  - 需要 emit StatusEvent 的代码段：在持锁期间快照所需数据到局部变量，**unlock 之后**再 emit（保留现行不在持锁期间 emit 的不变量，避免与 subscribers 慢消费者死锁）。
  - 等待 `waitUntilStable` 这种长耗时操作必须在 unlock 之后调用（已经如此；保持）。
- 行为契约（外部可观察）不变：idempotent 启动、starting→running 转换的时序、emit 顺序、错误返回值类型、与 Stop/Restart 的交互全部维持现状。
- **Why**：当前函数有 6 处 `m.mu.Unlock()` 调用且分散在不同分支，未来修改时容易遗漏一处导致死锁。`defer` 单点模式可消除该类错误。
- **User-visible behavior**：无外部行为变化。单元测试结果不变。

### IS-6 — storage 层 ErrDuplicateName sentinel + handler 返 409

- **What**：
  - 在 `internal/storage` 包中（推荐放在 `store.go` 已有 sentinel 段，与 `ErrNotFound` / `ErrVersionConflict` 并列）新增导出错误 `var ErrDuplicateName = errors.New("storage: duplicate proxy name")`。
  - 修改 `internal/storage/proxies.go` `UpsertProxy` 内 SQL INSERT/UPDATE 报错路径：当底层 sqlite 错误信息匹配 UNIQUE 约束 + `name` 字段时（**仅** name 唯一冲突，不含 (type,remotePort) 冲突），返回 `ErrDuplicateName` 而非原始包装错误。
  - `internal/httpapi/handlers_proxies.go` `mapProxyWriteError` 检测到 `ErrDuplicateName` → 返回 HTTP **409 Conflict** + `CodeConflict` + 中文消息 `"代理名称已存在，请改用其它名称"` + `field: "name"`。
  - 现有的 (type,remotePort) 冲突仍走原有 UNIQUE/constraint 文本分支返回 422。
- **Why**：当前 UNIQUE 冲突由 handler 通过 `strings.Contains(low, "unique")` 文本匹配捕获并返 422 + `字段冲突：可能 name 重复或 (type,remotePort) 冲突`。文案模糊（"可能"），且语义上 name 冲突更适合 409 Conflict（资源唯一性冲突）。引入 sentinel 后契约清晰，未来 sqlite 驱动变更错误文本也不破坏映射。
- **User-visible behavior**：前端创建 / 更新代理规则时，name 与已有规则冲突 → 响应 409 + 友好中文消息 `"代理名称已存在，请改用其它名称"`，`field: "name"`。
- **API schema 影响评估**：返回新的 409 状态码——`docs/spec/openapi.yaml` 中 `/proxies` POST/PUT 的 responses 段需要新增 `409` 条目。在 Open Questions 中明确（Q-3）。

### IS-7 — Dashboard 错误信息默认完整显示

- **What**：`web/src/pages/Dashboard.vue` frpc / frps 错误面板中：
  - 移除"查看完整日志 →"链接的**前置条件**——错误文本本身默认就完整展示在 NAlert 卡片内（当前已经使用 `white-space: pre-wrap`，但 NAlert 默认对长文本不做高度限制，看上去已经能展示；要求确认 `word-break: break-word` 也加上以避免超长单词不换行）。
  - 卡片整体不引入新的折叠/截断逻辑；如果 `lastErr` 内含换行（如 ringBuffer JoinTail 用 `" | "` 连接的多行 tail），每段以原样 wrap 展示。
  - "查看完整日志 →"链接保留（作为辅助导航），但用户**不必**点击它才能看到错误内容。
- **Why**：审计发现 lastErr 包含"exit: %v | tail: <最多 20 行>"，对诊断必要。用户应在 Dashboard 卡片即可看到全部，而非二次跳转。
- **User-visible behavior**：进程错误时，错误卡片完整显示 `lastErr` 全文（即使包含 1000 字符级长度）。链接仍可点击跳转。

### IS-8 — Proxies 页删除后清理 firewallPorts + 空状态占位

- **What**：`web/src/pages/Proxies.vue`：
  - **(a)** 在 `handleDeleteConfirm` 删除成功后，把 `firewallPorts.value = []` 清空（同时把 `firewallProto.value` 重置为初始值 `'both'`）。FirewallHint 组件因 ports 为空而自动隐藏，不留下已被删除规则对应的防火墙提示。
  - **(b)** `n-data-table` 添加空状态占位文案 `"暂无代理规则，点击右上角添加规则开始配置"`。实现方式按 naive-ui 的 NDataTable 推荐插槽（`#empty` slot 或 `data-table` 的 empty 配置），由 SA 决定具体 API。
- **Why**：当前删除后，残留的 `firewallPorts` 仍指向已删除规则的端口，对用户呈现误导性提示。空状态文案缺失导致首次使用者面对空表无引导。
- **User-visible behavior**：
  - (a) 删除任意 TCP/UDP 规则后，先前展示的 FirewallHint 立即消失。
  - (b) 列表为空时表格区域显示中文引导文案。

### IS-9 — ProxyForm 类型切换时重置不适用字段

- **What**：`web/src/components/ProxyForm.vue` / `web/src/composables/useProxyForm.ts`：
  - 当前 `handleTypeChange` 已经将 `remotePort` 与 `customDomains` 置空。但**未处理**从 HTTP 切回 TCP 时：
    - `remotePort` 不会自动给一个合理初始值（仍为 null，用户需要手动填一遍）。
  - 要求修改后：每次 `type` 变化（任意方向）：
    - **TCP / UDP** ← 切入：`customDomains = []`；`remotePort` 重置为 null（或合理默认值，由 SA 决定，本 REQ 只要求"用户上次填的 HTTP 域名不残留在隐藏字段中"）。
    - **HTTP / HTTPS** ← 切入：`remotePort = null`；`customDomains` 重置为 `[]`。
  - "重置"的语义：表单 visual state 中**隐藏的字段值**必须被清空，避免提交时上送遗留值导致后端 422。
- **Why**：当前 `handleTypeChange` 已部分清理，但审计指出某些路径下（包括 watch 触发未走 select 事件的场景）字段可能残留。统一行为 + 在 watch type 变化时也触发清理，避免 422。
- **User-visible behavior**：用户在表单中切换 type，hidden form item 内部值不再保留前一类型的数据；提交时不会因隐藏字段未清空导致 422。

## 范围外（out of scope）

PM 已明确延后到独立任务（PM_LOG §"延后"）：

1. **Version 字段 int64 → string 改造**（影响 OpenAPI / 前端 type / 兼容性 → 独立任务）
2. **前端文案集中化 i18n**（大型重构，需单独 ROI 评估）
3. **日志页虚拟滚动**（前端 feat，独立设计）
4. **Wizard 多步骤进度反馈优化**（产品决策）
5. **frpc admin 凭据加密存储**（深度安全工程：需要 keystore / DPAPI / libsecret 桥接，独立评估）
6. **基于部署模式自动启用 Secure cookie**（需 ADR 描述部署模式探测策略）

本任务**额外**不做（PM 未授权扩张）：

- frpc.log / frps.log 由子进程 toml 内 `log.to` 自己写出的部分（上游 FRP 行为，不修补）。
- Windows ACL 调整（Go chmod 在 Windows 仅控制 read-only 位，本任务以 POSIX 系统为权限收紧主要目标）。
- HTTPS / TLS / CSP / HSTS 等其它响应头（本任务仅三个最低基线头）。
- 日志读取速率限流（仅做单次大小上限，不做 QPS）。
- 审计中其他低优先发现（除非已在 IS-1..9 中显式列出）。

## 验收标准（AC）

每条 AC 必须独立可验证（自动测试或人工可观察）。

### AC-1（IS-1）frpconf 临时文件 / 输出文件权限

- **AC-1.1**（单元测试）：调用 `frpconf.AtomicWrite("/tmp/xxx/frpc.toml", []byte("..."))` 后，目标文件 `os.Stat` 返回的 `Mode().Perm() & 0o077 == 0`（POSIX 平台；Windows 平台跳过该断言）。
- **AC-1.2**（单元测试）：在 AtomicWrite 执行期间，临时文件（`.frpconf-*.tmp`）若被 stat 到，其 `Mode().Perm() & 0o077 == 0`。可通过在 mock io.Writer 中 sleep + 并发 stat 实现，或用 `t.Cleanup` 检查目录内残留文件权限。
- **AC-1.3**（回归）：`go test ./internal/frpconf/...` 全部 PASS；原有 AtomicWrite "tempfile 不残留" 行为不退化。

### AC-2（IS-2）ui.log / frpc.log / frps.log 权限

- **AC-2.1**（启动后人工验证）：POSIX 平台启动 `frp-easy`，触发一次 frpc Start 后，`ls -l <logDir>/ui.log <logDir>/frpc.log <logDir>/frps.log` 三者权限位末 6 位均为 `0o600`（如 `-rw-------`）。
- **AC-2.2**（代码审查）：`grep -nE "0o644|0644" cmd/frp-easy/main.go internal/procmgr/manager.go` 在日志 OpenFile 调用处不再出现 `0o644`；ui.log / frpc.log / frps.log 三个调用点统一为 `0o600`。
- **AC-2.3**（回归）：`go test ./internal/procmgr/... ./cmd/frp-easy/...`（若 cmd 有可测试入口）PASS；procmgr 现有进程启动 / 停止测试不退化。

### AC-3（IS-3）SecurityHeaders 中间件

- **AC-3.1**（单元测试）：新增 `internal/httpapi/middleware_test.go`（或同包测试文件）覆盖：
  - 任意 GET `/api/v1/health` 响应包含三个头，值精确等于 `nosniff` / `DENY` / `no-referrer`。
  - GET SPA fallback（例如 `/`）响应同样包含三个头。
  - POST `/api/v1/...` 错误路径（如 401/403/422/500）响应仍包含三个头。
- **AC-3.2**（代码审查）：`SecurityHeaders` 中间件挂载位置在 `router.go` 中明确，覆盖所有路由（包括 `/api/v1/health` 与 SPA NotFound）。
- **AC-3.3**（回归）：`scripts/verify_all` PASS；现有 `httpapi_test.go` / `qa_ac_test.go` 不退化（响应体不变，仅多了头）。

### AC-4（IS-4）日志增量响应字节上限 2 MiB

- **AC-4.1**（单元测试）：在 `internal/httpapi/handlers_logs_test.go`（新增或扩展现有测试）中：
  - 构造一个 5 MiB 测试日志文件，调用 `GET /api/v1/logs/frpc?offset=0` → 响应 `Data` 长度 == 2 MiB，`NextOffset == 2 MiB`。
  - 紧接着 `GET ?offset=2097152` → 响应 `Data` 长度 == 2 MiB，`NextOffset == 4 MiB`。
  - 再请求 `?offset=4194304` → 响应 `Data` 长度 == 1 MiB（剩余），`NextOffset == 5 MiB`。
- **AC-4.2**（边界）：文件总大小 < 2 MiB 时行为不变（一次返回全部）。文件不存在 / offset 超过文件大小时行为与现状一致（空响应）。
- **AC-4.3**（代码审查）：`maxLogChunk` 常量定义在 `handlers_logs.go` 中或包级常量，便于测试导入与未来调整。

### AC-5（IS-5）procmgr.Start defer unlock 重构

- **AC-5.1**（代码审查）：`internal/procmgr/manager.go` `Start(kind)` 函数体内 `m.mu.Unlock()` 显式调用次数 == 0（仅有入口处 `m.mu.Lock()` + 一处 `defer m.mu.Unlock()`）。
- **AC-5.2**（回归）：`go test ./internal/procmgr/... -race` PASS（启用 race detector，验证无新引入数据竞争 / 死锁）。
- **AC-5.3**（行为等价）：现有所有 procmgr 测试不修改测试代码即通过。包括 idempotent Start、Start→Stop→Start 序列、二进制缺失返错等场景。

### AC-6（IS-6）ErrDuplicateName + 409

- **AC-6.1**（storage 层单元测试）：`internal/storage/proxies_test.go`（或现有 test 文件）新增用例：插入 name="abc" tcp 规则成功后，再插入同名规则 → 返回的 error `errors.Is(err, storage.ErrDuplicateName)` 为 true。
- **AC-6.2**（storage 层单元测试）：(type=tcp, remotePort=8080) 冲突的另一插入 → `errors.Is(err, storage.ErrDuplicateName)` 为 **false**（必须区分 name 冲突与 (type,remotePort) 冲突）。
- **AC-6.3**（handler 层测试）：`internal/httpapi/handlers_proxies_test.go` 或集成测试中，连续两次 POST `/api/v1/proxies` 同名 → 第二次响应 HTTP **409**，body 含 `code: "CONFLICT"`，`message: "代理名称已存在，请改用其它名称"`，`field: "name"`。
- **AC-6.4**（OpenAPI schema）：见 Open Question Q-3 确认是否同步更新 `docs/spec/openapi.yaml`。

### AC-7（IS-7）Dashboard 错误完整显示

- **AC-7.1**（人工 / E2E）：模拟 frpc Start 失败（如 binary 缺失或 toml 损坏），Dashboard 卡片错误面板内 lastErr 全文可见，无 ellipsis / "..." / "查看完整日志才能看全" 的行为。
- **AC-7.2**（代码审查）：`Dashboard.vue` 错误展示节点 style 含 `white-space: pre-wrap` **与** `word-break: break-word`（或等价 `overflow-wrap: anywhere`）。
- **AC-7.3**（E2E 不退化）：现有 Playwright 烟雾测试（T-006 交付）仍 PASS。

### AC-8（IS-8）Proxies 删除清理 + 空状态

- **AC-8.1**（人工 / 单测）：在 Proxies 页面添加 TCP 规则 → 保存后看到 FirewallHint → 删除该规则 → FirewallHint 立即消失（DOM 中不再渲染或返回空）。
- **AC-8.2**（代码审查）：`Proxies.vue` `handleDeleteConfirm` 成功路径 setState 中包含 `firewallPorts.value = []`（或语义等价的清空操作）。
- **AC-8.3**（人工 / 单测）：列表为空时，表格区域显示文案 `"暂无代理规则，点击右上角添加规则开始配置"`（精确等于此字符串）。
- **AC-8.4**（E2E 不退化）：T-006 烟雾测试 PASS。

### AC-9（IS-9）ProxyForm 类型切换清理

- **AC-9.1**（人工 / 组件单测）：
  - 新建规则 → 选 HTTP → 在 customDomains 输入 `example.com` → 切到 TCP → 表单内部状态 `customDomains` 为 `[]`（hidden 状态下用 DevTools 或暴露 ref 验证）。
  - 紧接着填 `remotePort=8080` 提交 → 后端收到的 payload 不含 `customDomains` 字段（或为空数组）→ 不返回 422。
- **AC-9.2**（人工 / 组件单测）：反向场景 — 选 TCP 填 `remotePort=8080` → 切到 HTTP → `remotePort` 为 null（不会作为隐藏值上送，避免后端 422 "http/https 不接受 remotePort"）。
- **AC-9.3**（代码审查）：`useProxyForm.ts` 或 `ProxyForm.vue` 在 `watch(() => form.type, ...)` 或 `handleTypeChange` 内部对**两个**互斥字段均做了重置；不依赖单一事件链。

## 边界 / 假设

### 边界

- **B-1**：本任务全部修改保持向后兼容：HTTP API URL / 请求体 schema / DB schema / CLI flag 全部不变。**唯一**新增状态码 409（IS-6）属于 OpenAPI responses 集合扩展，非 breaking。
- **B-2**：IS-1 / IS-2 的权限收紧仅作用于 frp_easy 进程自己 OpenFile / CreateTemp 的文件。上游 frpc / frps 通过 toml `log.to` 写出的同名 frpc.log / frps.log 文件**首次创建**时可能仍受其上游默认权限影响（视上游版本而定）。验收以"frp_easy 自己创建/打开"为准；若上游覆盖了已存在文件的权限属上游 bug，不在本任务修复范围。
- **B-3**：IS-4 的 2 MiB 阈值是常量；如果某次单行日志超过 2 MiB（极端罕见），仍按字节切分，可能在响应中切到 UTF-8 字符中间。前端解析 `data` 时按字节流处理（当前 LogsIncrementalResponse `Data` 为 JSON 字符串字段 → JSON 编码会对非法 UTF-8 转义），不会导致 panic 或非法 JSON。
- **B-4**：IS-5 的"行为等价"判定基准是现有测试集合 + 现有 emit 顺序契约。若发现现有测试覆盖不足以保证等价，由 Developer 在 04 中补充测试用例，但**不修改外部契约**。
- **B-5**：IS-6 仅识别 `name` 字段的 UNIQUE 冲突。识别方式由 SA 决定（推荐：sqlite 错误文本同时含 `unique` 与 `proxies.name`；或捕获 sqlite3 错误码 `SQLITE_CONSTRAINT_UNIQUE` 后解析 column 名）；本 REQ 不规定具体实现。

### 假设

- **A-1**：审计输入对源代码行号 / 文件路径的引用准确（已通过 RA 阶段抽样读源代码验证：`render.go` AtomicWrite、`main.go` ui.log、`handlers_logs.go` ReadFrom、`manager.go` Start、`proxies.go` UpsertProxy、`Dashboard.vue` 错误面板、`Proxies.vue` 删除、`ProxyForm.vue`/`useProxyForm.ts` 类型切换均存在并与描述一致）。
- **A-2**：T-005 已交付 OpenAPI schema 与 Go 常量对齐（见 insight-index 2026-05-16 第 3 条）；IS-6 的 409 新增需要回填 `docs/spec/openapi.yaml`，工作量小。

## 风险

- **R-1（中）**：IS-5 procmgr Start 重构存在引入回归风险。缓解：要求 `-race` 测试在 AC-5.2 通过，并要求 SA 在 02 中给出"逐分支映射"说明（每个原 unlock 点 → 新模式下走哪个 return），Gate Reviewer 据此 review。
- **R-2（低）**：IS-4 单元测试需要 5 MiB 临时文件，CI 上磁盘/时间开销可接受（NVMe 一次写 5 MiB < 100ms）。无需特殊处理。
- **R-3（低）**：IS-3 三个安全头看似无害，但若未来引入 OAuth/SSO 跳转或 iframe 嵌入需求，`X-Frame-Options: DENY` 会成为障碍。当前 frp_easy 是单机本地 UI，DENY 与产品定位一致；如未来变化属"独立任务范围调整"，不在本任务考虑。
- **R-4（低）**：IS-7 当前实现已使用 `white-space: pre-wrap`，问题在于审计描述的"被截断"可能是另一个因素（如父容器固定高度 + overflow:hidden）。验证时需要实际触发长 lastErr 场景。缓解：AC-7.1 是人工/E2E 验证，能直接看到效果；如 SA 发现根因是父容器，应在 02 中纠正实现路径。
- **R-5（低）**：IS-9 类型切换重置可能与现有表单 v-model 双向绑定的 watch 链形成无限循环。缓解：SA 设计时明确"是否在 watch 内修改 form.value 同源字段"的 reentrancy 防护。

## Open Questions

> 全部 Open Question 都属于"实现路径细节"，**不影响**本 RA 阶段 ready 判定（PM 决策原则授权 PM 在用户体验/工程标准/可维护性维度自主决策）。下列问题留给 SA 在 02 设计时自行决断并在 02 中明示选择；Gate Reviewer 验证选择合理。

- **Q-1（IS-1 实现路径）**：在 `os.CreateTemp` 之后立即 `tmp.Chmod(0o600)`，还是改用 `os.OpenFile(filepath.Join(dir, randomName), O_CREATE|O_EXCL|O_WRONLY, 0o600)` 自行生成随机名 + 严格 mode？
  - (a) Chmod 路径：改动小，但临时文件在 Chmod 调用前的微秒窗口内权限仍受 umask 影响（窗口短到几乎不可被利用，但理论上存在）。
  - (b) 自定义随机名 + O_EXCL：完全消除窗口，但需要小心 Windows 上 mode 位的语义差异 + 自实现随机名冲突重试。
  - **RA 倾向**：(a) 实用、改动可控；(b) 完美但工程复杂。SA 决定。

- **Q-2（IS-3 中间件挂载位置）**：`SecurityHeaders` 应挂在 `r.Use(...)` 链中的位置：
  - (a) 挂在 chi 顶层（与 `/api/v1/health` 平级），覆盖所有响应。
  - (b) 仅挂在 Group 内，但额外手动包裹 health endpoint。
  - **RA 倾向**：(a)（health endpoint 也应有 nosniff）。SA 决定。

- **Q-3（IS-6 OpenAPI 同步）**：`docs/spec/openapi.yaml` 是否需要在本任务内同步新增 `409` 响应条目？
  - (a) 在本任务范围内同步更新 openapi.yaml（保持 schema 与实现一致，符合 T-005 insight 第 3 条精神）。
  - (b) 留给独立的"docs 同步"任务（最小化本任务 PR diff）。
  - **RA 倾向**：(a)。insight-index 2026-05-16 第 3 条明确"openapi 以 Go 常量为权威"，让实现与 schema 同步发版更稳。SA 决定，并在 02 中明示。

- **Q-4（IS-8b 空状态文案精确性）**：`"暂无代理规则，点击右上角添加规则开始配置"` 是否一字不差？
  - (a) 一字不差（已在 IS-8 中作为字面字符串）。
  - (b) SA 微调以匹配现有页面已有的中文风格（"+ 添加规则" vs "+ 新增规则"等）。
  - **RA 倾向**：(b)。Proxies.vue 现有按钮文本是"新增规则"，建议空状态文案统一使用"新增规则"。**最终文案以 SA 在 02 中给出的为准**；本 REQ 在 AC-8.3 中精确字符串可由 SA 替换。

## 相关历史任务

- **T-001 web-ui-mvp**（`docs/features/_archived/web-ui-mvp/`）：HTTP API、中间件链、storage 层 sentinel `ErrNotFound` / `ErrVersionConflict` 的原始设计。本任务 IS-3 / IS-5 / IS-6 在其基础上扩展。
- **T-002 zero-config-quickstart**（`docs/features/_archived/zero-config-quickstart/`）：引入 wizard、downloader、自动启动模式。IS-7（Dashboard 错误显示）与 T-002 引入的"模式开关" UX 同处一页。
- **T-005 docs-and-api-schema**（`docs/features/_archived/docs-and-api-schema/`）：openapi.yaml 权威化。IS-6 新增 409 响应码需要遵循该任务建立的"以 Go 常量为权威"约定（见 Q-3）。
- **T-006 e2e-smoke-tests**（`docs/features/_archived/e2e-smoke-tests/`）：Playwright 烟雾测试基线。IS-7 / IS-8 / IS-9 的 AC 中均要求烟雾测试不退化。

## Verdict

**READY**

所有 9 项修复点的范围、行为变更、验收标准已明确。4 个 Open Questions 全部为"实现路径细节"，PM 已授权 SA 在用户体验/工程标准/可维护性维度自主决策；不阻塞 RA → SA 推进。
