---
task: hardening-pass-audit
task_id: T-007
stage: 02_solution_design
mode: full
date: 2026-05-19
author: solution-architect
status: READY
---

# 02 解决方案设计 — T-007 hardening-pass-audit

## 设计摘要

本任务属典型 **加固扫除（hardening pass）**：9 个聚焦修复点分布在三个分区
（后端 Go / 前端 Vue / 文档），无新增模块、无新增依赖、无 DB schema 变更、无新增
HTTP 路由。**整体架构无变化**；改动均为局部加固。

- **后端 (dev-backend)**：`internal/frpconf` 与 `cmd/frp-easy/main.go` 收紧文件权限；
  `internal/httpapi` 新增 `SecurityHeaders` 中间件；`internal/logtail.ReadFrom` 提升
  单次读取上限到 2 MiB（当前为 1 MiB，但 REQ 要求 2 MiB —— 见 AC-4 节）；
  `internal/procmgr/manager.go` `Start()` 重构为 defer-unlock；`openapi.yaml`
  追加 `/proxies` POST/PUT 的 409 响应（IS-6 同步）。
- **DB 分区 (dev-db)**：`internal/storage` 新增 sentinel `ErrDuplicateName`，
  `UpsertProxy` 在 sqlite UNIQUE 冲突（仅 `proxies.name`）时返回它。**无迁移文件**，
  schema 不动。
- **前端 (dev-frontend)**：`Dashboard.vue` 错误显示加 `word-break`；`Proxies.vue`
  删除时清 firewallPorts + 加空状态；`ProxyForm.vue` 类型切换重置时按 type 互斥
  双向清理（修 `useProxyForm.ts` 的 `handleTypeChange` + 加 watch）。

外部契约（HTTP API 路径 / 请求体 schema / DB schema / CLI flag）全部不变。
唯一新增的可观察契约：`POST/PUT /api/v1/proxies` 在 name 冲突时返回 **409
CONFLICT**（原来是 422 + `code: CONFLICT`），是非破坏性扩展。

## 模块改动地图

| 文件 | 类型 | 改动说明 | 估行数 | 分区 |
|---|---|---|---|---|
| `internal/frpconf/render.go` | edit | `AtomicWrite` 在临时文件 / 目标文件上加 `Chmod(0o600)` | +12 / -0 | dev-backend |
| `internal/frpconf/render_test.go` | edit/new | 新增权限位单元测试（POSIX 跳过 Windows） | +60 / -0 | dev-backend |
| `cmd/frp-easy/main.go` | edit | `ui.log` OpenFile mode 0o644 → 0o600 | +1 / -1 | dev-backend |
| `internal/procmgr/manager.go` | edit | (a) `supervise` 中 frpc/frps.log OpenFile mode 改 0o600；(b) `Start()` 重构 defer-unlock | +25 / -25 | dev-backend |
| `internal/procmgr/manager_test.go` | edit | 补 race + idempotent / 早返回测试 | +40 / -0 | dev-backend |
| `internal/httpapi/middleware.go` | edit | 新增 `SecurityHeaders` 中间件函数 | +20 / -0 | dev-backend |
| `internal/httpapi/middleware_test.go` | new | `SecurityHeaders` 单元测试 | +80 / -0 | dev-backend |
| `internal/httpapi/router.go` | edit | 在 chi `Group` 最外层 `r.Use(SecurityHeaders())`；同时为顶层 `/api/v1/health` 与 SPA fallback 显式套上（见 AC-3 设计） | +8 / -0 | dev-backend |
| `internal/logtail/tail.go` | edit | `ReadFrom` 的 `maxReadPerCall` 常量提升 1 MiB → 2 MiB；导出为包级 `MaxReadBytes` 或新增 `ReadFromN(path, off, maxBytes)` —— 见 AC-4 决策 | +6 / -2 | dev-backend |
| `internal/logtail/tail_test.go` | edit | 2 MiB 切片边界测试 | +50 / -0 | dev-backend |
| `internal/httpapi/handlers_logs.go` | edit (optional) | 仅在 AC-4 选 `ReadFromN(...)` 方案时改 1 行 | +1 / -1 | dev-backend |
| `internal/httpapi/handlers_logs_test.go` | edit | 5 MiB 测试日志 + 多次轮询断言 | +90 / -0 | dev-backend |
| `internal/storage/errors.go` | new | 新文件，存放 `ErrDuplicateName` sentinel（也可放 store.go 现有 sentinel 段，见决策） | +12 / -0 | dev-db |
| `internal/storage/proxies.go` | edit | `UpsertProxy` INSERT/UPDATE 错误路径检测 UNIQUE 冲突，仅 `proxies.name` 列 → 包装 `ErrDuplicateName` | +20 / -2 | dev-db |
| `internal/storage/proxies_test.go` | edit/new | 新增 (a) name 冲突 → ErrDuplicateName；(b) (type,remotePort) 冲突 → 非 ErrDuplicateName | +50 / -0 | dev-db |
| `internal/httpapi/handlers_proxies.go` | edit | `mapProxyWriteError` 提前判断 `errors.Is(err, storage.ErrDuplicateName)` → 409 + CONFLICT + 中文 + `field:"name"` | +6 / -0 | dev-backend |
| `internal/httpapi/handlers_proxies_test.go`（或集成测试） | edit | 新增"同名 POST 第二次 → 409"测试 | +40 / -0 | dev-backend |
| `openapi.yaml` | edit | `/proxies` POST 与 PUT 的 `responses` 新增 `'409'` 条目 | +14 / -0 | dev-backend |
| `web/src/pages/Dashboard.vue` | edit | 两处错误 `<div>` style 加 `word-break: break-word` | +2 / -2 | dev-frontend |
| `web/src/pages/Proxies.vue` | edit | (a) `handleDeleteConfirm` 成功后清 firewallPorts；(b) `n-data-table` 加空状态文案 | +10 / -0 | dev-frontend |
| `web/src/components/ProxyForm.vue` | edit (small) | 新增 `watch(() => form.type, ...)` 触发清理（防 select 之外的更新路径） | +6 / -0 | dev-frontend |
| `web/src/composables/useProxyForm.ts` | edit | `handleTypeChange` 按目标 type 互斥重置（不一刀切） | +10 / -3 | dev-frontend |
| `web/src/components/__tests__/ProxyForm.test.ts`（或扩展现有） | edit/new | 类型切换字段重置组件单测 | +60 / -0 | dev-frontend |
| `docs/dev-map.md` | （不动） | 无模块结构变化 | 0 | — |

合计：后端 ~480 行变化、DB ~80 行、前端 ~90 行；纯加固，无新依赖。

## 详细设计

### AC-1（IS-1）frpconf 临时文件 / 输出文件权限

#### 现状
`internal/frpconf/render.go::AtomicWrite`（line 256–289）：
```go
tmp, err := os.CreateTemp(dir, ".frpconf-*.tmp")
// ... tmp.Write / tmp.Sync / tmp.Close
os.Rename(tmpName, path)
```
`os.CreateTemp` 在 POSIX 下用 `0o600` 调 `open(...)`，但**实际生效权限 = 0o600 & ~umask**。
当 umask = `0o022` 时实际是 `0o600` 不受影响（已是 owner-only）；但 umask = `0o002`、
`0` 等场景仍可能让 group/other 拿到读权限。已渲染的目标 toml 含 `frps_token` 明文，
需要严格 owner-only。

#### 目标
- 临时文件创建后**立即** `tmp.Chmod(0o600)`，关闭 umask 影响。
- `os.Rename` 之后再对最终路径 `os.Chmod(path, 0o600)`，关闭"目标文件已存在被覆盖
  但保留原权限位"等 corner case。
- POSIX 平台测试断言权限 = 0o600；Windows 平台 `os.Chmod` 仅控制 read-only bit，
  无安全意义，但**保留代码**以保持单一代码路径；测试 `runtime.GOOS == "windows"` 时
  跳过权限断言。

#### 改动点（render.go）
伪代码（增量）：
```go
func AtomicWrite(path string, content []byte) error {
    // ... 已有
    tmp, err := os.CreateTemp(dir, ".frpconf-*.tmp")
    if err != nil { return ... }
    tmpName := tmp.Name()

    // 【新增】立即收紧临时文件权限（POSIX 立即生效；Windows no-op for non-readonly bit）
    if chmodErr := os.Chmod(tmpName, 0o600); chmodErr != nil {
        _ = tmp.Close()
        _ = os.Remove(tmpName)
        return fmt.Errorf("frpconf.AtomicWrite chmod tmp: %w", chmodErr)
    }
    // ... 已有：tmp.Write / tmp.Sync / tmp.Close / Rename
    if err := os.Rename(tmpName, path); err != nil { ... }

    // 【新增】rename 后再 chmod 目标（防止目标文件已存在时被继承的旧权限）
    if chmodErr := os.Chmod(path, 0o600); chmodErr != nil {
        // 不回滚 rename（已成功）；记 warn 但保留写入成果
        // 注意：本包无 logger，错误向上冒泡由调用方处理
        return fmt.Errorf("frpconf.AtomicWrite chmod final: %w", chmodErr)
    }
    return nil
}
```

#### 跨平台考量
- **Windows**：`os.Chmod` 仅当 mode & 0o200 == 0 时设置 ReadOnly attr；其余位被忽略。
  当前调用 `0o600` 含 owner write，**不会**误把目标文件标 readonly，安全。
- **POSIX**：`os.Chmod` 直接 `chmod(2)`，权限位精确生效。
- **Q-1 决策**：选 **(a) Chmod 路径**（见 Open Questions 决议 Q-1）。理由：改动小、
  代码可读、`os.CreateTemp` 在 modernc.org/sqlite 已经验证可移植；理论窗口期 < 1ms
  且攻击者需在该窗口内完成 stat + open，对单机本地 UI 是可忽略风险。

#### 测试要点（render_test.go）
- `TestAtomicWritePermissions`：写一个 toml → `os.Stat` → `Mode().Perm() == 0o600`
  （POSIX 平台）；Windows 跳过。
- `TestAtomicWriteOverwriteExistingPermissions`：预先 `os.WriteFile(path, ..., 0o666)`，
  AtomicWrite 后断言 perm 已收紧到 0o600。
- `TestAtomicWriteCleansTempOnError`：现有"不残留 tmp" 行为不退化（通过让 dir
  只读触发 rename 失败 + 检查无残留 `.frpconf-*.tmp` 文件）。

### AC-2（IS-2）日志文件权限

#### 现状
- `cmd/frp-easy/main.go:93`：`os.OpenFile(uiLogPath, ..., 0o644)`。
- `internal/procmgr/manager.go:378`（`supervise` 函数）：`os.OpenFile(logPath, ..., 0o644)`
  用于 frpc/frps 子进程 stdout/stderr tee。**该文件由 UI 进程打开 / 写入**，归属
  本任务范围（REQ B-2 已明确）。
- frpc/frps 自己通过 toml `log.to` 写出的同名文件（首次创建权限由上游 FRP 决定）**不在范围**。

#### 目标
两处 `0o644` 改为 `0o600`。

#### 改动点
1. `cmd/frp-easy/main.go:93`：
   ```go
   logFile, _ := os.OpenFile(uiLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
   ```
2. `internal/procmgr/manager.go:378`：
   ```go
   logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
   ```

注意：`OpenFile` 的 mode 参数仅在**文件不存在 + O_CREATE**时生效；如果文件已存在
（用户从老版本升级），权限不会回收。**追加调用** `os.Chmod(path, 0o600)`：

```go
// procmgr.supervise
if logPath != "" {
    logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
    if err == nil {
        _ = os.Chmod(logPath, 0o600) // 升级路径：把老 0o644 文件强制收紧
        defer logFile.Close()
    }
}
```

`main.go` 中同理：
```go
logFile, _ := os.OpenFile(uiLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
if logFile != nil {
    _ = os.Chmod(uiLogPath, 0o600)
}
```

#### 跨平台考量
- Windows：同 AC-1。`0o600` 中 owner write 不会触发 ReadOnly attr。
- POSIX：immediate take effect。

#### 测试要点
- `procmgr_test.go` 增 `TestSuperviseLogFilePerms`：mock 一个 sleep 二进制（或用
  现有测试 binary fixture），Start 后断言 log 路径 perm == 0o600（POSIX）。
- `cmd/frp-easy/` 无传统单测入口；改为**代码审查 + 集成运行**验证（AC-2.1 是
  人工 `ls -l`）。

### AC-3（IS-3）SecurityHeaders 中间件

#### 现状
`internal/httpapi/router.go` 中间件链：`ReadyGate → Recover → RequestID → Logger → CORS`。
当前无 nosniff / X-Frame-Options / Referrer-Policy 任何头。

#### 目标
新增 `SecurityHeaders()` 中间件，**对所有响应**写出：
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`

不加 CSP（SPA 内联可能挂；需单独评估，列入"延后"）。
不加 HSTS（HTTP-only 部署，HSTS 会卡 IP 直连访问）。

#### 改动点（middleware.go）
```go
// SecurityHeaders 给所有响应注入最低基线安全头。
// 对静态资源、API、错误响应一视同仁。
func SecurityHeaders() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            h := w.Header()
            h.Set("X-Content-Type-Options", "nosniff")
            h.Set("X-Frame-Options", "DENY")
            h.Set("Referrer-Policy", "no-referrer")
            next.ServeHTTP(w, r)
        })
    }
}
```

#### 挂载位置（router.go）
**Q-2 决策**：选 **(a) 在 chi 顶层**，覆盖**包括 `/api/v1/health` 与 SPA fallback**。
当前 router.go 结构是：
```go
r := chi.NewRouter()
r.Get("/api/v1/health", h.health)            // 顶层，无中间件
r.Group(func(r chi.Router) {                  // 中间件 group
    r.Use(ReadyGate(...)) ; r.Use(Recover ...) ; ...
})
r.NotFound(assets.Handler().ServeHTTP)
r.Get("/", assets.Handler().ServeHTTP)
```

由于 `/health` 在顶层注册（无 r.Use），不能让 SecurityHeaders 仅挂在 Group 内。
两种实现选择：
- **(a1)**：把 `SecurityHeaders` 用 `r.Use(SecurityHeaders())` 挂在顶层 chi router；
  chi 的全局 `Use` 也覆盖直接 `r.Get(...)` 注册的路由。
- **(a2)**：用包装手法 `r.Use(SecurityHeaders())` + 重新组织代码先 Use 再 Get。

**选 (a1)**：chi `r.Use(...)` 必须在路由注册之前调用，且影响所有后续路由（包括顶层）。
具体编排：
```go
r := chi.NewRouter()
r.Use(SecurityHeaders())                      // 【新增】最先注入

r.Get("/api/v1/health", h.health)             // 头会写出
r.Group(func(r chi.Router) {
    r.Use(ReadyGate(d.Ready))
    // ... 其余照旧
})
r.NotFound(assets.Handler().ServeHTTP)        // 头会写出
r.Get("/", assets.Handler().ServeHTTP)        // 头会写出
```

chi 的 `Router.Use` 在初始化阶段把全局 middleware 加入栈，对所有后续注册的路由
（包括 `Get`、`Group`、`NotFound`、`MethodNotAllowed`）生效，这是 chi 文档保证的行为。

#### 跨平台考量
无。

#### 测试要点（middleware_test.go，新文件）
- `TestSecurityHeadersOnHealth`：GET /api/v1/health 响应含三头精确值。
- `TestSecurityHeadersOnAPI`：GET /api/v1/system/ready（无需登录）含三头。
- `TestSecurityHeadersOnNotFound`：GET /nonexistent（走 SPA fallback / NotFound）含三头。
- `TestSecurityHeadersOnError`：POST /api/v1/proxies 不带 cookie → 401 响应仍含三头。
- 现有 `httpapi_test.go` / `qa_ac_test.go` 不退化（断言响应体 / status code，对头不敏感）。

### AC-4（IS-4）日志读取大小上限

#### 现状
`internal/logtail/tail.go::ReadFrom`（line 89-123）**已经有上限**：
```go
const maxReadPerCall = 1 << 20  // 1 MiB
want := size - startAt
if want > maxReadPerCall { want = maxReadPerCall }
```

**重要发现**：此值是 1 MiB，但 REQ AC-4.1 明确要求 **2 MiB**。

#### 目标
将上限提升到 **2 MiB（2 × 1024 × 1024 = 2097152 字节）**，与 REQ AC-4 完全对齐；
将常量提升为**包级导出**（或者通过 handler 层 `LimitReader` 限制），便于测试。

#### 决策：在 logtail 层调整常量
**Q-? 实现路径**（REQ 未明确，SA 决定）：
- **方案 A（推荐）**：直接把 `maxReadPerCall` 改为 `MaxReadBytes = 2 << 20`，**导出**
  为包级 const。测试导入 `logtail.MaxReadBytes` 用于切片大小断言。改动 1 行常量、
  1 行导出名。Handler 不变。
- **方案 B**：新增 `ReadFromN(path, off, maxBytes int64)` API，保留 `ReadFrom` 调用
  内部 ReadFromN(path, off, MaxReadBytes)；handler 显式传 `2 << 20`。改动稍多但
  灵活性更高。
- **方案 C**：handler 用 `io.LimitReader` 包裹 ReadFrom 结果（不推荐，因为
  ReadFrom 已经把所有数据读进内存，limit 只能截字符串）。

**选方案 A**。理由：
1. 单点改动最小；
2. 现有 ReadFrom 已经做了文件层 seek + 部分读取，是真正的 IO 上限不是后置截断；
3. 包级 const 导出后测试可以直接断言长度上限，未来调整也只动一处；
4. 不影响 handler 签名 / 响应字段。

#### 改动点（tail.go）
```go
// MaxReadBytes 是 ReadFrom 单次最多返回的字节数；超过部分留到下次调用。
// 保护服务器免于读取超大日志时一次性占用过多内存 / HTTP body 过大。
// 当前 = 2 MiB；客户端按 nextOffset 继续轮询即可还原完整流。
const MaxReadBytes = 2 << 20 // 2 MiB

func ReadFrom(path string, offset int64) ([]byte, int64, error) {
    // ... 同前
    want := size - startAt
    if want > int64(MaxReadBytes) {
        want = int64(MaxReadBytes)
    }
    // ... 同前
}
```

#### 跨平台考量
- 测试时构造 5 MiB 临时文件 →`t.TempDir()` 隔离 + `os.WriteFile` 写 5 MiB 0xAA 即可。
  Windows / POSIX 均 < 100ms。
- 5 MiB 测试数据可在测试包用全局 helper 生成一次（`testdata/` 不存盘，运行时
  `t.TempDir()` 写）。

#### 测试要点
**tail_test.go**：
- `TestReadFrom_ChunkBoundary`：5 MiB 文件，offset=0 → len==2 MiB & next==2 MiB；
  offset=2 MiB → len==2 MiB & next==4 MiB；offset=4 MiB → len==1 MiB & next==5 MiB；
  offset=5 MiB → len==0 & next==5 MiB（已到 EOF）。
- `TestReadFrom_SmallFile`：100 KiB 文件 offset=0 → 一次返回全部，next==100 KiB。
- `TestReadFrom_OffsetBeyondEOF`：offset=999999 在 1KiB 文件上 → next==0 且 data 为空
  （行为已有；不退化）。

**handlers_logs_test.go**（集成）：
- `TestLogsIncremental_LargeFile`：通过 HTTP 端点连续 3 次请求覆盖 5 MiB 文件，
  断言每次 `LogsIncrementalResponse.Data` 长度 ≤ 2 MiB 且 `NextOffset` 单调递增。

### AC-5（IS-5）procmgr.Start defer-unlock 重构

#### 现状（manager.go 172-242）
当前 Start 函数体内有 **6 处** 显式 `m.mu.Unlock()`：
- L181（`StateStarting/StateRunning` 早返回，info 已取）
- L184（`StateStopping` 早返回带错误）
- L189（binPath 失败）
- L194（cfgPath 失败）
- L199（mkdir log 失败）
- L216（cmd.Start 失败，写完 ps.info 后）
- L230（成功路径，构造 info 后让 supervisor 接管）

每条都正确，但分支多 → 维护风险高（缺一条就死锁；新增分支容易漏）。

#### 目标
单一 `defer m.mu.Unlock()`；所有早返回不再手动 unlock；需要 emit StatusEvent 的
代码段在持锁期间快照数据到局部变量，**unlock 之后再 emit**（保留"不在持锁期间 emit"
的不变量，避免 subscribers 慢消费者死锁）。

`waitUntilStable` 这种长耗时调用必须 unlock 后执行（现状已如此）。

#### 改动点（manager.go::Start）— 完整重写骨架
```go
func (m *Manager) Start(kind string) (ProcessInfo, error) {
    if err := validateKind(kind); err != nil {
        return ProcessInfo{}, err
    }

    // 第一段：持锁，做"决策 + cmd.Start"。
    // 关键不变量：解锁前不调 emit、不调 waitUntilStable。
    var (
        infoToEmit ProcessInfo
        shouldEmit bool
        emitOnExit bool  // cmd.Start 失败需要 emit error 状态
        startCmd   *exec.Cmd
        startCtx   context.Context
        startDone  chan struct{}
        stdoutPipe io.ReadCloser
        stderrPipe io.ReadCloser
        logPath    string
        startErr   error
    )

    func() {
        m.mu.Lock()
        defer m.mu.Unlock()

        ps := m.processes[kind]
        switch ps.info.State {
        case StateStarting, StateRunning:
            infoToEmit = ps.info // 用作返回值，但不 emit
            return                 // idempotent 早返回
        case StateStopping:
            infoToEmit = ps.info
            startErr = fmt.Errorf("procmgr.Start(%s): currently stopping", kind)
            return
        }

        binPath, err := m.binPathFor(kind)
        if err != nil {
            infoToEmit = ps.info
            startErr = err
            return
        }
        cfgPath, ok := m.configPaths[kind]
        if !ok || cfgPath == "" {
            infoToEmit = ps.info
            startErr = fmt.Errorf("procmgr.Start(%s): no config path configured", kind)
            return
        }
        logPath = m.logFiles[kind]
        if logPath != "" {
            if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
                infoToEmit = ps.info
                startErr = fmt.Errorf("procmgr.Start(%s) mkdir log: %w", kind, err)
                return
            }
        }

        cmd := exec.Command(binPath, "-c", cfgPath)
        cmd.Dir = filepath.Dir(binPath)
        applyPlatformAttrs(cmd)
        stdoutPipe, _ = cmd.StdoutPipe()
        stderrPipe, _ = cmd.StderrPipe()

        if err := cmd.Start(); err != nil {
            ps.info.State = StateError
            ps.info.LastErr = err.Error()
            ps.info.ChangedAt = time.Now().UTC()
            infoToEmit = ps.info
            shouldEmit = true
            emitOnExit = true
            startErr = fmt.Errorf("procmgr.Start(%s): %w", kind, err)
            return
        }

        ctx, cancel := context.WithCancel(context.Background())
        doneCh := make(chan struct{})
        ps.cmd = cmd
        ps.cancel = cancel
        ps.doneCh = doneCh
        ps.info.State = StateStarting
        ps.info.PID = cmd.Process.Pid
        ps.info.LastErr = ""
        ps.info.ChangedAt = time.Now().UTC()
        infoToEmit = ps.info
        shouldEmit = true
        startCmd = cmd
        startCtx = ctx
        startDone = doneCh
    }()

    // 第二段：解锁后再执行。
    if shouldEmit {
        m.emit(StatusEvent{Kind: kind, Info: infoToEmit})
    }
    if startErr != nil {
        // emitOnExit 用于区分"cmd.Start 失败"（已 emit 一次）；其它早返回不 emit。
        return infoToEmit, startErr
    }
    // 成功路径：启动 supervisor + 等稳定。
    go m.supervise(startCtx, kind, startCmd, stdoutPipe, stderrPipe, logPath, startDone)
    if waitErr := m.waitUntilStable(kind, 3*time.Second); waitErr != nil {
        return m.Status(kind), waitErr
    }
    return m.Status(kind), nil
}
```

**逐分支映射**（response to R-1）：

| 原分支（手动 unlock 处） | 新模式行为 |
|---|---|
| L178-182 idempotent | 第一段 IIFE 内 `case StateStarting/StateRunning: infoToEmit=...; return` → 第二段 `shouldEmit=false`, `startErr=nil` → 返回 `infoToEmit, nil` |
| L183-186 StateStopping | 第一段 `infoToEmit=...; startErr=...; return` → 第二段 `shouldEmit=false` 直接 `return infoToEmit, startErr` |
| L187-191 binPath err | 同上 |
| L192-196 cfgPath err | 同上 |
| L198-201 mkdir err | 同上 |
| L211-219 cmd.Start err | 第一段 `shouldEmit=true; emitOnExit=true; startErr=...; return` → 第二段 emit 一次 → `return infoToEmit, startErr` |
| L220-242 成功路径 | 第一段 `shouldEmit=true; startCmd=...; startErr=nil` → 第二段 emit + go supervise + waitUntilStable |

行为契约外部不变：idempotent 返回值、emit 次数（每条原路径 emit 一次或零次，不变）、
waitUntilStable 调用顺序、错误返回值类型，全保。

#### 跨平台考量
无。`applyPlatformAttrs` 已在 manager_{windows,unix}.go 处理。

#### 测试要点
- `go test ./internal/procmgr/... -race`：开 race detector，新结构无新数据竞争。
- 现有所有测试（idempotent Start、Start→Stop→Start、二进制缺失、cfgPath 缺失）
  **不修改测试代码**通过。这是 AC-5.3 的硬要求。
- 新增：`TestStart_DeferUnlock_NoLeakOnPanic`：mock 一个会 panic 的 `binPathFor`，
  验证锁能被 defer 释放（非必需，但 race-detector 友好）。

### AC-6（IS-6）storage.ErrDuplicateName + handler 409

#### 现状
- `internal/storage/store.go` 顶部已有 `ErrCorruptReset / ErrVersionConflict / ErrNotFound`。
- `internal/storage/proxies.go::UpsertProxy` INSERT/UPDATE 错误直接 `fmt.Errorf("...: %w", err)`，
  不区分 UNIQUE 冲突。
- `internal/httpapi/handlers_proxies.go::mapProxyWriteError`（L237-262）用
  `strings.Contains(low, "unique")` 文本匹配 → 422 + CONFLICT + 模糊文案
  `"字段冲突：可能 name 重复或 (type,remotePort) 冲突"`。

#### 目标
- 在 storage 层加 `ErrDuplicateName` sentinel。
- `UpsertProxy` 仅当 sqlite UNIQUE 冲突且冲突列是 `proxies.name` 时返回 `ErrDuplicateName`；
  `(type,remote_port)` 冲突保持原样（继续走 422 路径）。
- handler `mapProxyWriteError` 用 `errors.Is(err, storage.ErrDuplicateName)` 拦截 →
  **409 Conflict** + `CodeConflict` + `"代理名称已存在，请改用其它名称"` + `field:"name"`。

#### sentinel 放在哪
**决策**：新建 `internal/storage/errors.go`，集中存放包级 sentinel（包括把
`ErrCorruptReset / ErrVersionConflict / ErrNotFound` 也迁过来）。一步到位避免 store.go
膨胀。**但**为最小化 diff、降低回归风险，本任务选**最小变更**：
- 在 `store.go` 现有 sentinel 段（L35-47）追加 `ErrDuplicateName`；不动其它。
- 注释里指明"未来 sentinel 多了再拆 errors.go"。

如果 Gate Reviewer 偏好新文件，可以接受 — 但默认本任务在 store.go 加 4 行：
```go
// ErrDuplicateName 表示 UpsertProxy 时 proxies.name UNIQUE 约束冲突。
// 调用方应据此返回 409 Conflict 而非 422。
ErrDuplicateName = errors.New("storage: duplicate proxy name")
```

#### UNIQUE 冲突识别策略（REQ B-5 / Q）
modernc.org/sqlite 的 `*sqlite.Error`（在 `modernc.org/sqlite/lib`）支持 `Code()` /
`ExtendedCode()`。SQLITE_CONSTRAINT_UNIQUE = 2067。但当前项目代码**未引入 sqlite 子包**
（store.go 只 `_ "modernc.org/sqlite"` blank import）。引入子包会增加耦合。

**决策**：**用 `strings.Contains` 文本匹配作为可移植 fallback**。modernc.org/sqlite 的
UNIQUE 冲突错误文本格式：
```
constraint failed: UNIQUE constraint failed: proxies.name (2067)
```
**精确判断**：错误字符串同时包含 `"UNIQUE constraint failed"` 和 `"proxies.name"`
（不区分大小写不必要，sqlite 输出固定）。这能 **唯一** 区分 name UNIQUE 与
(type,remote_port) UNIQUE（后者错误文本含 `proxies.type`、`proxies.remote_port` 或
索引名 `idx_proxies_tcp_remote`）。

如果未来 sqlite 驱动版本变更错误文本，测试会立即捕获（AC-6.1）。

#### 改动点（proxies.go::UpsertProxy）
在 INSERT 与 UPDATE 两个 `s.db.ExecContext(...)` 调用之后立即检测：
```go
// 在 INSERT 路径：
res, err := s.db.ExecContext(ctx, insertQ, ...)
if err != nil {
    if isDuplicateNameError(err) {
        return ErrDuplicateName
    }
    return fmt.Errorf("storage.UpsertProxy insert: %w", err)
}

// 在 UPDATE 路径（tx.ExecContext 之后）：
if _, err := tx.ExecContext(ctx, updQ, ...); err != nil {
    _ = tx.Rollback()
    if isDuplicateNameError(err) {
        return ErrDuplicateName
    }
    return fmt.Errorf("storage.UpsertProxy update: %w", err)
}
```

新增辅助（同 proxies.go 或 errors.go）：
```go
func isDuplicateNameError(err error) bool {
    if err == nil {
        return false
    }
    s := err.Error()
    // sqlite 驱动文本格式："UNIQUE constraint failed: proxies.name"
    return strings.Contains(s, "UNIQUE constraint failed") &&
        strings.Contains(s, "proxies.name")
}
```

#### 改动点（handlers_proxies.go::mapProxyWriteError）
在 `errors.Is(err, storage.ErrVersionConflict)` 分支后、`strings.Contains(low, "unique")`
分支前插入：
```go
if errors.Is(err, storage.ErrDuplicateName) {
    writeError(w, http.StatusConflict, CodeConflict,
        "代理名称已存在，请改用其它名称", "name")
    return
}
```

保留原 `strings.Contains(low, "unique")` 分支不动 —— 它现在只命中
`(type,remote_port)` 冲突场景，文案仍是"字段冲突..."，但 `field` 检测逻辑里
`if strings.Contains(low, "remote_port") { field = "remotePort" }` 会正确选中
`remotePort` 字段。可以**进一步优化**文案为 `"(type, remotePort) 组合已被占用"`，
但本任务不强制（REQ 仅要求 name 冲突的语义化）。

#### OpenAPI 同步（Q-3 决策：在本任务范围内更新）
**决策选 (a)**：在本任务内同步更新 `openapi.yaml`。
- 给 `/api/v1/proxies` POST（L631-668）`responses` 追加：
  ```yaml
  '409':
    description: 代理名称冲突（CONFLICT）
    content:
      application/json:
        schema:
          $ref: '#/components/schemas/ErrorBody'
  ```
- 给 `/api/v1/proxies/{id}` PUT 已有 `'409'`（版本冲突）但 description 是"版本冲突"。
  改为 `"版本冲突或名称冲突（CONFLICT）"`（合并描述），或追加一条 example。**选合并描述**
  更简洁：
  ```yaml
  '409':
    description: 冲突（CONFLICT，版本或代理名称冲突）
  ```

理由：insight-index 2026-05-16 第 3 条明确"openapi 以 Go 常量为权威"。让实现与
schema 同步发版更稳，且本任务即可完成。

#### 测试要点
**storage/proxies_test.go**：
- `TestUpsertProxy_DuplicateName`：插 name="abc" tcp,8080 → 成功；再插 name="abc"
  tcp,8081 → `errors.Is(err, ErrDuplicateName) == true`。
- `TestUpsertProxy_DuplicateTypeRemotePort`：插 name="abc" tcp,8080 → 成功；再插
  name="xyz" tcp,8080 → `errors.Is(err, ErrDuplicateName) == false`，错误文本仍含
  `"UNIQUE constraint failed"` 但不含 `"proxies.name"`。

**httpapi/handlers_proxies_test.go** 或 `httpapi_test.go`：
- `TestCreateProxy_DuplicateNameReturns409`：连续两次 POST `/api/v1/proxies` 同 name →
  第二次 status==409，body.error.code=="CONFLICT", message=="代理名称已存在，请改用其它名称",
  field=="name"。
- 现有"(type,remotePort) 冲突" 测试如果存在则验证不退化（仍是 422 + CONFLICT）；
  若不存在则补一条。

### AC-7（IS-7）Dashboard 错误完整显示

#### 现状（Dashboard.vue L45-56, L115-126）
两处错误面板：
```vue
<n-alert v-if="..." type="error" style="margin-top: 12px">
  <div style="font-family: monospace; font-size: 12px; white-space: pre-wrap">
    {{ procStore.frpcInfo.lastErr }}
  </div>
  <n-button text tag="a" href="/logs/frpc" style="margin-top: 4px">
    查看完整日志 →
  </n-button>
</n-alert>
```

`lastErr` 来自 `procmgr.supervise` 的 `fmt.Sprintf("exit: %v | tail: %s", waitErr, tail.JoinTail())`。
tail.JoinTail() 用 `" | "` 把末 20 行连接成一行，可能很长（每行 200 字符 × 20 = 4 KiB）。

现有 style 已含 `white-space: pre-wrap`，**长行不换行**问题来自缺少 `word-break`
（长 token 如长 stacktrace path 会撑爆容器）。审计报告"被截断"实际是溢出隐藏。

#### 目标
- style 加 `word-break: break-word`（或等价 `overflow-wrap: anywhere`）。
- 不引入新的折叠/截断逻辑；NAlert 内部高度 auto，可展开任意长度。
- 保留"查看完整日志 →"链接（辅助导航）。

#### 改动点
两处 `<div>` style 修改：
```vue
<div style="font-family: monospace; font-size: 12px; white-space: pre-wrap; word-break: break-word">
```

#### 跨平台考量
浏览器兼容：`word-break: break-word` 在所有主流浏览器（Chrome / Firefox / Safari /
Edge）均支持；`overflow-wrap: anywhere` 同样可用，二者择一。**选 `word-break: break-word`**
（与 REQ AC-7.2 字面一致）。

#### 测试要点
- Vitest 组件测试不易模拟 NAlert 内部渲染；改为**人工/E2E**：
  - 模拟 lastErr = 1000 字符无空格字符串 → 视觉断言换行（人工）。
  - Playwright 烟雾测试不要求覆盖（T-006 基线不退化即可）。
- 代码审查（AC-7.2）：grep `word-break.*break-word` 在 Dashboard.vue 中出现 ≥ 2 次。

### AC-8（IS-8）Proxies 删除清理 + 空状态

#### 现状（Proxies.vue）
- `handleDeleteConfirm`（L113-123）成功后只 `message.success` 并清 `deletingProxy.value`；
  **未清** `firewallPorts.value` —— 用户删了一个 TCP 规则，下方 FirewallHint 仍展示
  该规则的端口提示。
- `n-data-table`（L9-14）无 `:empty-text` 或 `#empty` slot；列表为空时显示 naive-ui
  默认 "No Data"（英文）。

#### 目标
- (a) `handleDeleteConfirm` 删除成功后 `firewallPorts.value = []`、
  `firewallProto.value = 'both'`（重置为初始值）。FirewallHint 因 props.ports==[]
  自动隐藏（已在保存 http/https 路径用过 = []，组件已支持）。
- (b) 添加空状态文案。

#### Q-4 决策：空状态文案
**REQ 倾向 (b)** —— 统一现有 "新增规则" 按钮文案。**最终文案**：

```
"暂无代理规则，点击右上角「新增规则」开始配置"
```

字数与原 REQ AC-8.3 接近，但用 「」 中文引号包裹按钮文本，与项目其它中文文案
（如 "确定要删除规则「...」吗？" 在 L45 已有）的引号风格一致。

#### 改动点
1. `handleDeleteConfirm` 末尾追加（**在 try 成功路径内**，不在 catch 内 —— 删除失败
   不清空提示）：
```ts
async function handleDeleteConfirm() {
  if (!deletingProxy.value) return
  try {
    await proxiesStore.deleteProxy(deletingProxy.value.id)
    message.success('规则已删除')
    // 【新增】清理可能残留的防火墙提示
    firewallPorts.value = []
    firewallProto.value = 'both'
  } catch (e) {
    message.error(extractErrorMessage(e, '删除失败'))
  } finally {
    deletingProxy.value = null
  }
}
```

2. `n-data-table` 模板：
```vue
<n-data-table
  :columns="columns"
  :data="proxiesStore.proxies"
  :loading="proxiesStore.loading"
  style="margin-top: 16px"
>
  <template #empty>
    <n-empty description="暂无代理规则，点击右上角「新增规则」开始配置" />
  </template>
</n-data-table>
```

naive-ui `NDataTable` 支持 `#empty` 作用域插槽（NDataTable 公开 slot 名 `empty`），
内嵌 `NEmpty` 组件（已在 Proxies.vue 上下文中可用，本就属 naive-ui）。
需要 import：
```ts
import { ..., NDataTable, NEmpty } from 'naive-ui'
```
（NDataTable 已 import；只补 NEmpty）

#### 测试要点
- 现有 Proxies 集成测试（如存在）不退化。
- 新增 Vitest（可选）：mock `useProxiesStore().proxies = []` → mount Proxies → 
  断言文本 "暂无代理规则" 出现在 DOM。
- E2E 烟雾测试不要求覆盖。

### AC-9（IS-9）ProxyForm 类型切换重置

#### 现状（useProxyForm.ts L32-35）
```ts
function handleTypeChange() {
  form.value.remotePort = null
  form.value.customDomains = []
}
```

每次切换 type **都**清两个字段 → 实际上从 TCP → HTTP 时 `remotePort` 应保持 null
（正确），但**反向**也清 customDomains（无意义但无害）。问题在于：
- `handleTypeChange` 仅在 `n-select @update:value` 触发（ProxyForm.vue L23）；
- 如果某条路径通过 `v-model` 直接更新 `form.type`（未来扩展或 syncFromInput 写入
  initial.type），不会触发 handler。
- 现有 `syncFromInput`（L54-63）就直接写入 `form.value.type = val.type`，不经
  handler。但 syncFromInput 是父→子单向同步，本应保持父端字段不被清，所以
  **不能在 syncFromInput 内调 handler**。

#### 目标
- handler 按目标 type 做**有方向的**重置（语义化更清晰）。
- 加 `watch(() => form.value.type, ...)` 作为兜底，但**避免与 syncFromInput 的
  reentrancy**：watch 触发清理时不能反向写回父组件被父组件再 sync 回来形成循环。

#### Reentrancy 防护设计
策略：watch 触发时仅修改本地 `form.value.remotePort` / `form.value.customDomains`，
不直接调 `emit('update:modelValue')`。下游 watch `form` (deep) 会发出一次 emit，
父组件 syncFromInput 写回时 `form.value.type` 不变 → watch type 不再触发 → 无循环。

唯一边界：父组件初次 mount 时 syncFromInput 设置 form.value.type，会触发本 watch
执行一次清理 —— 但 initial 数据已经是合法（不会同时有 remotePort 和 customDomains
矛盾），清理 no-op 或仅清空一个不应在的字段，是**期望行为**。

#### 改动点（useProxyForm.ts）
```ts
function handleTypeChange(newType?: ProxyFormType) {
  // 兼容 select 不传参数的旧调用：用当前 form.value.type
  const t = newType ?? form.value.type
  if (t === 'tcp' || t === 'udp') {
    // tcp/udp 不接受 customDomains；remotePort 由用户填
    form.value.customDomains = []
    // remotePort 保留 null（用户上次填的 HTTP 域名不在此字段；remotePort 由用户填新值）
  } else if (t === 'http' || t === 'https') {
    // http/https 不接受 remotePort
    form.value.remotePort = null
  }
}
```

新增 watch（在 useProxyForm 返回之前）：
```ts
// 兜底：任何路径修改 form.type 时同步清理互斥字段。
// 注意：syncFromInput 直接写 form.value.type 也会触发此 watch；初次 mount 时执行一次
// no-op 或冗余清理（已是 null/[]）安全无副作用。
watch(() => form.value.type, (newType, oldType) => {
  if (newType === oldType) return
  handleTypeChange(newType)
})
```

#### ProxyForm.vue 模板侧（无变化）
现有 `@update:value="handleTypeChange"` 保留即可（select 一次 → handler 一次；
watch 也会触发一次，幂等无副作用，因为两次都清同样字段为同样值）。

#### 测试要点（components/__tests__/ProxyForm.test.ts）
- `TestTypeSwitch_HttpToTcpClearsCustomDomains`：mount ProxyForm 初始 type=http,
  customDomains=['example.com'] → 调用 toggle type='tcp' → 断言 form.customDomains==[]。
- `TestTypeSwitch_TcpToHttpClearsRemotePort`：初始 tcp+remotePort=8080 → 切 type=http
  → 断言 form.remotePort==null。
- `TestTypeSwitch_NoInfiniteLoop`：开启 vitest fake timer，连续切 type → assert
  watch 仅触发预期次数（≤ 2 次 per switch）。
- `TestToProxyInput_HiddenFieldsExcluded`：tcp 模式下即使 form.customDomains 历史
  残留（强行注入）也不出现在 `toProxyInput()` 输出（当前 toProxyInput 已经按 type
  分支选字段，行为不变）。

## 数据 / API 契约变化

### DB schema
**无变化**。proxies 表保持现状（迁移 0001 不动；任务期间不新增迁移文件）。

### HTTP API 路径 / 请求体
**无变化**。

### HTTP 状态码集合
**扩展**（非破坏性）：
- `POST /api/v1/proxies`：响应集合增加 `409 Conflict`（name 冲突时返回）。
  原有 422（其它字段冲突如 (type,remotePort)）保持。
- `PUT /api/v1/proxies/{id}`：响应集合的 `409` 含义扩展为"版本冲突 **或** 名称冲突"。

### 响应体 schema（ErrorBody）
**无变化**。`{error:{code,message,field}}` 结构保持。新增的 409 响应:
```json
{"error":{"code":"CONFLICT","message":"代理名称已存在，请改用其它名称","field":"name"}}
```
完全符合 ErrorBody。

### 响应头集合（新增）
所有响应（包括 SPA fallback / 错误 / 健康检查）**新增 3 个安全响应头**：
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`

属于"加增"而非"修改"，不破坏前端解析。

### openapi.yaml
按 Q-3 决策同步更新（见 AC-6 节）。

## 测试策略

按 AC 分类的测试类型与位置：

| AC | 测试类型 | 测试文件 | 关键断言 |
|---|---|---|---|
| AC-1 | Go 单元 | `internal/frpconf/render_test.go` | `Mode().Perm() == 0o600`（POSIX） |
| AC-2 | Go 单元 + 集成 | `internal/procmgr/manager_test.go` + 人工 `ls -l` | log 文件 perm |
| AC-3 | Go 单元 | `internal/httpapi/middleware_test.go`（新） | 三头精确值 + 覆盖 /health/SPA/错误 |
| AC-4 | Go 单元 | `internal/logtail/tail_test.go` + `handlers_logs_test.go` | 5 MiB 文件 3 次轮询切片 |
| AC-5 | Go 单元 + race | `internal/procmgr/manager_test.go` + `go test -race` | grep manager.go 中 `m.mu.Unlock()` == 0 次 |
| AC-6 | Go 单元 | `internal/storage/proxies_test.go` + `internal/httpapi/handlers_proxies_test.go` | sentinel 判断 + 409 响应 |
| AC-7 | 代码审查 + 人工 | `web/src/pages/Dashboard.vue` | grep `word-break.*break-word` |
| AC-8 | Vitest + 人工 | `web/src/pages/__tests__/Proxies.test.ts`（新或扩展） | 删除后 firewallPorts 清空 + 空状态文案 |
| AC-9 | Vitest 组件 | `web/src/components/__tests__/ProxyForm.test.ts` | 类型切换字段重置 |
| 回归 | E2E | `web/tests/e2e/*.spec.ts` | T-006 5 个 spec PASS |
| 全量 | `scripts/verify_all` | — | 必须 PASS |

`scripts/verify_all` 已经包含 Go 单测、Vitest、ESLint、Playwright；本任务无需新增
verify_all 步骤。

## Open Questions 决议

> PM 已授权 SA 自主决定（用户原话："你来决策即可"）。下列为最终决议，记入本设计。

### Q-1（IS-1 临时文件权限实现路径）
**决议**：选 **(a) `os.CreateTemp` + 立即 `tmp.Chmod(0o600)`**。

理由：
1. 改动最小，可读性高。
2. 单机本地 UI（绑 127.0.0.1）的攻击面下，"理论上微秒级窗口"风险可忽略。
3. (b) 自实现随机名 + O_EXCL 在 Windows 上 mode 位语义与 POSIX 不同，
   需要额外条件编译；ROI 低。
4. 测试可在 AtomicWrite 调用前/后用 stat 验证；窗口期内文件已被收紧。

### Q-2（IS-3 中间件挂载位置）
**决议**：选 **(a) 在 chi 顶层 `r.Use(SecurityHeaders())`**，所有路由（含
`/api/v1/health` 与 SPA fallback）一并覆盖。

理由：
1. health endpoint 也应有 nosniff（即便它返回 JSON）。
2. 单一 Use 调用，零特例代码。
3. chi 文档保证全局 Use 对后续注册（包括顶层 Get、Group、NotFound）生效。

### Q-3（IS-6 OpenAPI 同步）
**决议**：选 **(a) 在本任务内同步更新 `openapi.yaml`**。

理由：
1. insight-index 2026-05-16 第 3 条明确"openapi 以 Go 常量为权威"。
2. 实现与 schema 同步发版减少未来"docs drift" 任务。
3. 变更仅 2 处 yaml 节点（POST `/proxies` + PUT `/proxies/{id}`），diff < 20 行。

### Q-4（IS-8b 空状态文案）
**决议**：选 **(b) SA 微调**。最终文案：
```
暂无代理规则，点击右上角「新增规则」开始配置
```

理由：
1. 与现有 "新增规则" 按钮文本（Proxies.vue L6）一致。
2. 使用 「」 全角中文引号与本页 L45 风格统一。
3. 不强制 REQ 字面字符串（REQ Q-4 已开放给 SA）。

## 风险与缓解

| R | 严重度 | 风险描述 | 缓解 |
|---|---|---|---|
| R-1 | 中 | procmgr.Start defer-unlock 重构引入回归（死锁 / state 漂移 / emit 时序错） | (a) AC-5 节给出**逐分支映射**表（原 6 处 unlock → 新模式行为），Gate Reviewer 据此 review；(b) `-race` 测试强制 PASS；(c) 现有测试不修改测试代码即通过 |
| R-2 | 低 | 5 MiB 测试文件 CI 时间 / 磁盘开销 | NVMe 单次写 5 MiB < 100ms；testdata 用 `t.TempDir()` 隔离 |
| R-3 | 低 | SecurityHeaders `X-Frame-Options: DENY` 阻止未来 iframe / SSO 需求 | 本期 frp_easy 是单机本地 UI，DENY 与产品定位一致；如未来调整属独立任务 |
| R-4 | 低 | AC-7 错误显示"被截断"根因可能不是 word-break 而是父容器 overflow | 加 word-break 后人工触发长 lastErr 验证；若仍不可见，回到 02 文档纠正实现路径（可能需要 max-height: none / overflow: visible） |
| R-5 | 低 | AC-9 type 切换 watch 与 syncFromInput 形成循环 | AC-9 节明确 reentrancy 防护：watch 内不调 emit，依赖现有 form deep watch 自然发出；初次 mount 触发的 no-op 清理无害 |
| R-6 | 低 | AC-6 UNIQUE 错误文本匹配依赖 modernc.org/sqlite 版本 | 测试 AC-6.1 直接覆盖该判断；驱动升级时测试会立即捕获文本变化 |
| R-7 | 低 | AC-4 把 logtail.MaxReadBytes 提升到 2 MiB 增加单次内存峰值 | 2 MiB 在现代服务器内存里可忽略；多连接并发场景下仍受 chi 默认 max concurrent connections 限制；可接受 |
| R-8 | 低 | AC-2 升级路径：老用户 ui.log 已是 0o644 | 用 `os.Chmod(path, 0o600)` 在 OpenFile 之后无条件强制收紧；幂等安全 |
| R-9 | 低 | AC-3 三头与现有响应已有同名头冲突 | grep 全代码无此三头存在；Set（而非 Add）覆盖语义安全；CORS 中间件不写这三个头 |

## 实现顺序建议

按 PM 派发依赖顺序（dev-db → dev-backend → dev-frontend）+ 单分区内按风险升序：

### Step 1 — dev-db（先行）
1. **AC-6.A**：`internal/storage/store.go` 加 `ErrDuplicateName` sentinel + `proxies.go`
   `UpsertProxy` 错误识别。
2. **AC-6.B**：`proxies_test.go` 加 sentinel 单元测试。
3. 完成后 dev-db 写 04，PM 推 dev-backend。

### Step 2 — dev-backend（基础设施先于业务）
顺序：
1. **AC-1**：`internal/frpconf/render.go` AtomicWrite chmod + 测试（最低耦合）。
2. **AC-2**：`cmd/frp-easy/main.go` + `internal/procmgr/manager.go::supervise` 改 0o600。
3. **AC-3**：`internal/httpapi/middleware.go` 加 SecurityHeaders + router.go 挂载 + 测试。
4. **AC-4**：`internal/logtail/tail.go` 提升 MaxReadBytes 到 2 MiB + 测试。
5. **AC-5**：`internal/procmgr/manager.go::Start` defer-unlock 重构 + `-race` 测试（**最后**做，
   风险最高，先把别的稳了再动）。
6. **AC-6.C**：`internal/httpapi/handlers_proxies.go::mapProxyWriteError` 接 sentinel +
   集成测试。
7. **AC-6.D**：`openapi.yaml` 同步 409 响应。
8. 完成后 dev-backend 写 04，PM 推 dev-frontend。

### Step 3 — dev-frontend（消费后端契约）
1. **AC-7**：`Dashboard.vue` 两处 style 改 + 人工验证。
2. **AC-8**：`Proxies.vue` 删除清理 + 空状态。
3. **AC-9**：`useProxyForm.ts` handleTypeChange + watch 防护 + 组件单测。
4. 跑 Playwright 烟雾测试不退化。
5. 完成后 dev-frontend 写 04。

### Step 4 — Gate Review
GR 验证 9 个 AC、风险缓解、Open Questions 决议合理性、verify_all PASS。

## Partition assignment

按 `.harness/agents/dev-*.md` 的 owned paths：

| 文件 | 分区 | 新建 / 编辑 | 依赖（按分区） |
|---|---|---|---|
| `internal/storage/store.go` | dev-db | edit（加 sentinel） | — |
| `internal/storage/proxies.go` | dev-db | edit（识别 UNIQUE） | 依赖同文件 sentinel |
| `internal/storage/proxies_test.go` | dev-db | edit/new | — |
| `internal/frpconf/render.go` | dev-backend | edit（chmod） | — |
| `internal/frpconf/render_test.go` | dev-backend | edit/new | — |
| `cmd/frp-easy/main.go` | dev-backend | edit（log mode） | — |
| `internal/procmgr/manager.go` | dev-backend | edit（chmod + Start defer） | — |
| `internal/procmgr/manager_test.go` | dev-backend | edit | — |
| `internal/httpapi/middleware.go` | dev-backend | edit（加 SecurityHeaders） | — |
| `internal/httpapi/middleware_test.go` | dev-backend | new | 依赖 middleware.go |
| `internal/httpapi/router.go` | dev-backend | edit（挂载） | 依赖 middleware.go |
| `internal/httpapi/handlers_proxies.go` | dev-backend | edit（mapProxyWriteError） | 依赖 dev-db 的 ErrDuplicateName |
| `internal/httpapi/handlers_proxies_test.go` | dev-backend | edit/new | 依赖上行 |
| `internal/logtail/tail.go` | dev-backend | edit（MaxReadBytes） | — |
| `internal/logtail/tail_test.go` | dev-backend | edit | — |
| `internal/httpapi/handlers_logs_test.go` | dev-backend | edit | — |
| `openapi.yaml` | dev-backend | edit | 依赖 handlers_proxies.go 行为已落地 |
| `web/src/pages/Dashboard.vue` | dev-frontend | edit | — |
| `web/src/pages/Proxies.vue` | dev-frontend | edit | — |
| `web/src/components/ProxyForm.vue` | dev-frontend | edit | — |
| `web/src/composables/useProxyForm.ts` | dev-frontend | edit | — |
| `web/src/components/__tests__/ProxyForm.test.ts` | dev-frontend | edit/new | — |

### Dispatch order

1. **dev-db**（AC-6 后端依赖前置）
2. **dev-backend**（消费 sentinel + 加固后端全部）
3. **dev-frontend**（独立的 UX 修复，最后做即可）

### Parallelism

`dev-backend` 内部 AC-1/2/3/4 之间**无依赖**，可并行实施；AC-5 重构 procmgr 风险较
高，建议**最后做**避免影响其它 AC 的回归基线。

`dev-frontend` 与 `dev-backend` 在本任务**实际上无强耦合**（前端 AC-7/8/9 不依赖
后端 AC 落地后的新行为；前端代码改动不需要后端契约变化以外的事）—— 如果开发资源
充足，可以与 dev-backend 并行；但保守起见 PM 按 db→backend→frontend 顺序派发是
安全选择，便于 Gate Review 时基线清晰。

## Verdict

**READY**

- 9 项 AC 全部给出可实施方案，关键实现路径（Q-1、Q-2、Q-3、Q-4）已 SA 决策并写入。
- 无新依赖；无 schema 迁移；无新路由。
- 对 procmgr.Start 这一最高风险项给出**逐分支映射**（R-1 缓解）。
- 与 dev-map 一致，无结构性发现（无新模块、无新路径，dev-map.md 不需要更新）。
- 与 insight-index 一致：
  - 沿用 AtomicWrite 现有 rename 不变量（2026-05-16 第 1 条）；
  - openapi 与 Go 常量同步更新（2026-05-16 第 3 条）；
  - 不动 NMessageProvider / App.vue（2026-05-17 第 1 条无关）；
  - 前端无前端资源嵌入相关改动（2026-05-17 第 2 条无关）。
- 未发现需要标记新 INSIGHT 的"非显然"事实。AC-4 的"现状已经有 1 MiB 上限"是一处对
  REQ 假设的修正，但 REQ 明示要求 2 MiB，按要求实施即可；不构成新踩坑。

可移交 Gate Reviewer 复核；GR APPROVED 后按上述 Dispatch order 派发开发。
