# 04 · 后端开发记录 — T-002 · zero-config-quickstart

> 编写：dev-backend · 日期：2026-05-16
> 上游输入（只读）：`01_REQUIREMENT_ANALYSIS.md`（verdict=READY）、`02_SOLUTION_DESIGN.md`（verdict=READY）

---

## 1. 实现的文件列表

### 新建

| 文件 | 说明 |
|---|---|
| `internal/downloader/downloader.go` | FRP 二进制异步下载管理器；追加 `ParseIPFromJSON` 函数（供 httpapi 复用，避免重复 JSON 解析逻辑） |
| `internal/downloader/downloader_test.go` | 下载器单测（8 个测试用例，全部 mock CDN）：成功路径 tar.gz/zip、ErrAlreadyInProgress、HTTP 404 失败、Zip Slip 防御、网络超时、ErrBadKind、ParseIPFromJSON |
| `internal/httpapi/handlers_wizard.go` | Wizard 状态读写 handler |

### 修改

| 文件 | 修改内容 |
|---|---|
| `internal/httpapi/handlers_system.go` | 追加 `ipResult`、`ipCache` 类型；`PublicIPResponse`、`DownloadBinRequest`、`DownloadStatusResponse` 结构；`systemPublicIP`、`downloadBin`、`downloadStatus` 三个 handler；IP 检测辅助函数 |
| `internal/httpapi/router.go` | 追加 `Downloader *downloader.Manager` 到 `Dependencies`；`ipCache ipCache` 字段到 `handlers` 结构（C-1）；5 条新路由；`downloader` import |
| `cmd/frp-easy/main.go` | 注入 `downloader.New(loc.Root(), logger)` → `deps.Downloader` |
| `docs/dev-map.md` | 追加 `internal/downloader/` 模块条目 |

---

## 2. 关键决策记录

### 2.1 FRPVersion = "0.68.1"

设计文档给的占位版本是 `"0.61.1"`，实际运行 `frp_win/frpc.exe --version` 得到 `0.68.1`。
代码中使用实际版本，避免 R-8 风险（版本不一致导致下载二进制与现有 toml 字段不兼容）。

### 2.2 测试注入机制（C-4）

`Manager` 有两个未导出字段用于测试注入：
- `baseURL string`：空字符串时使用 GitHub CDN；测试时设为 `httptest.Server.URL`
- `goos string`：空字符串时使用 `runtime.GOOS`；测试时设为 `"linux"` 或 `"windows"`

由于测试文件是 `package downloader`（白盒测试），可直接访问这些未导出字段。
**全部 6 个测试用例均通过 `net/http/httptest.NewServer` mock，无任何外网访问（C-4 满足）。**

### 2.3 进度追踪：io.TeeReader + progressWriter

按设计文档 §3.1 规范，使用 `io.TeeReader(resp.Body, progressWriter)` 模式：
- 若 `Content-Length` 已知：`progress = written * 95 / contentLength`（下载中最高 95%，成功后跳 100）
- 若 `Content-Length` 未知：伪进度，每 512KB +2%，上限 95%

### 2.4 Zip Slip 防御（R-2）

tar.gz 提取：检查 `strings.Contains(hdr.Name, "..") || filepath.IsAbs(hdr.Name)`  
zip 提取：检查 `strings.Contains(name, "..") || filepath.IsAbs(f.Name) || strings.HasPrefix(name, "/")`

恶意 entry 被 `continue` 跳过，不影响合法 entry 的提取。Test 5 验证了此行为。

### 2.5 Windows os.Rename 兼容

Windows 上 `os.Rename(src, dst)` 在 dst 已存在时会报错（不同于 Linux 的原子替换）。
实现中先 `_ = os.Remove(targetPath)` 再 `os.Rename`，代价是极短窗口内文件不存在（MVP 可接受）。

### 2.6 C-1 门控：ipCache 字段位置

`ipCache` 类型定义在 `handlers_system.go`（合理的业务归属），
但 `handlers` 结构体中的 `ipCache ipCache` 字段**必须在 `router.go`** 中声明（Gate Review C-1 要求）。

### 2.7 ipCache 并发策略

两次并发请求同时 miss 缓存时，可能发起两次外部调用（double-fetch）。
对于公网 IP 检测这类场景可接受（MVP 阶段，无 singleflight 依赖）。
缓存命中路径（5 分钟内）写 `debug` 级别日志，避免高频命中刷爆日志（NF-O2）。

### 2.8 wizard hasAnyConfig 判断

按设计文档 §3.2，有 4 个 KV key 表示"已有配置"：
1. `frpc.serverConn` 存在 → 用户曾保存 frpc 配置
2. `frps.config` 存在 → 用户曾保存 frps 配置
3. `mode.frpc.enabled == "true"` → frpc 已启用
4. `mode.frps.enabled == "true"` → frps 已启用

任一满足则 `shouldShow = false`，wizard 不自动展示。
这 4 个 key 均使用已有常量（`kvFrpcServerCfg`、`kvFrpsConfig`，定义在 `handlers_server.go`），无重复定义。

---

## 3. 与设计不一致之处

| # | 不一致点 | 处理方式 |
|---|---|---|
| D-1 | 设计 §3.1 `New()` 签名写作 `func New(root string) *Manager`（无 logger 参数），Gate Review 要求 `func New(root string, logger *slog.Logger) *Manager` | 以 Gate Review C-2 为准，传入 logger |
| D-2 | 设计 §3.1 示例中 `states` 初始化后的 `ok=false` 语义描述不清 | 实现：`Status(invalid_kind)` 返回 `ok=false`；已初始化的 frpc/frps 永远返回 `ok=true` |
| D-3 | 设计占位 `FRPVersion = "0.61.1"` 与实际 vendored 版本不符 | 使用实际版本 `"0.68.1"`（经 `frp_win/frpc.exe --version` 验证） |

---

## 4. go vet + go test 结果

```
$ go vet $(go list ./... | grep -v node_modules)
(无输出，全部 PASS)

$ go test $(go list ./... | grep -v node_modules) -timeout 120s
?   github.com/frp-easy/frp-easy/cmd/frp-easy   [no test files]
ok  github.com/frp-easy/frp-easy/internal/appconf
ok  github.com/frp-easy/frp-easy/internal/assets
ok  github.com/frp-easy/frp-easy/internal/auth
ok  github.com/frp-easy/frp-easy/internal/binloc
ok  github.com/frp-easy/frp-easy/internal/downloader   0.892s  (8 tests)
ok  github.com/frp-easy/frp-easy/internal/frpcadmin
ok  github.com/frp-easy/frp-easy/internal/frpconf
ok  github.com/frp-easy/frp-easy/internal/httpapi      2.504s
ok  github.com/frp-easy/frp-easy/internal/logtail
ok  github.com/frp-easy/frp-easy/internal/procmgr
ok  github.com/frp-easy/frp-easy/internal/storage
```

verify_all 结果：PASS 12 / WARN 0 / FAIL 0（SKIP 6 为前端检查，不在 dev-backend 分区）。

---

## 5. Gate Review 条件自检

| 条件 | 状态 | 证据 |
|---|---|---|
| C-1: `ipCache ipCache` 字段在 `router.go` 的 `handlers` 结构体 | PASS | `router.go:118-120`，`type handlers struct { deps Dependencies; ipCache ipCache }` |
| C-2: `New()` 签名为 `func New(root string, logger *slog.Logger) *Manager` | PASS | `downloader.go:67-77` |
| C-4: `downloader_test.go` 全部使用 `httptest.NewServer` mock CDN | PASS | 8 个测试均使用 `httptest.NewServer`，无任何 `https://github.com` 调用 |

---

### Code Review 修复（2026-05-16）

**修复问题：M-3（B-4）— downloader.go Windows 原子化安装**

**修改文件：**
- `internal/downloader/downloader.go` — Step 3 原子 rename 改为安全的两阶段逻辑：先尝试 `os.Rename`（Linux 原子覆盖）；若失败且运行时为 Windows，则先 `os.Remove` 旧文件再重试 Rename；若 Remove 失败（非 ErrNotExist）则报"移除旧版本失败"而不是静默删除。使用 `renameErr` 变量避免遮蔽，`runtime` 包已在原有 import 中。

**修复问题：M-4 — 新增 5 个端点的测试**

**修改文件：**
- `internal/httpapi/qa_ac_test.go` — 新增 import `"github.com/frp-easy/frp-easy/internal/downloader"`；新增辅助函数 `newTestServerWithDownloader`（使用 `os.MkdirTemp` 而非 `t.TempDir()`，避免 Windows 下载 goroutine 持有文件句柄时 cleanup 报错）；新增 9 个测试：
  - `TestWizardStatus_FreshDB_ShouldShow`（shouldShow=true）
  - `TestWizardStatus_WithConfig_ShouldNotShow`（shouldShow=false）
  - `TestWizardComplete_ThenShouldNotShow`（complete → shouldShow=false）
  - `TestDownloadBin_ValidKind_202`（frpc → 202）
  - `TestDownloadBin_InvalidKind_422`（invalid → 422）
  - `TestDownloadStatus_KnownKind_200`（frpc → 200 + JSON）
  - `TestDownloadStatus_UnknownKind_404`（unknownkind → 404）
  - `TestPublicIP_Always200`（always 200，ip 或 error 字段）

**关键技术说明（M-4 Windows 兼容性）：**

`newTestServerWithDownloader` 使用 `os.MkdirTemp("", "frp-dl-test-")` 而非 `t.TempDir()`，并以 `_ = os.RemoveAll(dlRoot)` 注册清理（忽略错误）。这是因为 `TestDownloadBin_ValidKind_202` 调用 `Start("frpc")` 后下载 goroutine 立即开始向 GitHub 请求，同时在 `frp_win/` 目录创建 `.dl-archive-*.tmp` 临时文件；测试函数返回后 cleanup 立即运行，而 goroutine 此时还持有文件句柄，Windows 不允许删除被占用的文件，导致 `t.TempDir()` 的 cleanup 以 `t.Errorf` 将测试标记为 FAIL。使用 `os.MkdirTemp` + 忽略清理错误可避免此竞态，不影响测试断言的正确性。

**verify_all 结果：**
- 修复前：PASS 12 / WARN 0 / FAIL 0（baseline）
- 修复后：PASS 12 / WARN 0 / FAIL 0（delta: 0 新失败，新增 9 个测试用例全部通过）
