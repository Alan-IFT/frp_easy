# T-018 upload-bin-multiport-ip-probe — 解决方案设计

> **作者**：Solution Architect
> **日期**：2026-05-23
> **模式**：full（7-stage）
> **关联**：`docs/features/upload-bin-multiport-ip-probe/01_REQUIREMENT_ANALYSIS.md`
> **分区**：fullstack（dev-frontend / dev-backend；dev-db 不参与）

---

## 1. 总览（Architecture Summary）

T-018 在 frp_easy 单二进制 + Web UI 之上叠加三组体验增强，**不改 DB schema、不引入新依赖**：

- **A：二进制上传入口**。新增 `POST /api/v1/system/upload-bin`，浏览器 multipart 上传 frpc/frps，落盘路径与下载链路一致。后端把"原子 rename + chmod + Windows fallback"代码抽到新文件 `internal/downloader/install.go`，让下载（T-014）与上传共享同一落盘实现，避免两条路径走偏。
- **B：公网 IP 大陆友好源**。改造 `fetchPublicIP` 从串行 2 源 → 并发 5 源（ipify / my-ip.io / ip.cn / bilibili IP 服务 / `https://www.ip.cn/` HTML 兜底），保留 5 min 缓存 + `FRP_EASY_PUBLIC_IP` 短路。响应新增可选 `source` 字段，便于诊断。
- **C：多端口转发 + 预设 + 端口探测**。新增 `POST /api/v1/proxies/batch`（端口表达式展开为多条 Proxy，单事务）+ `POST /api/v1/system/port-probe`（本机端口可用性探测）。前端在 `ProxyForm.vue` 加端口模式切换、常用预设 Tag、探测按钮；新增 `internal/portrange/` 共享 helper 包。

**改动估算**：
- 新增文件：6 个（Go 3 / TS-Vue 3）
- 修改文件：8 个（Go 4 / TS-Vue 3 / openapi.yaml 1）
- 0 新依赖、0 migration

**与历史任务衔接**：
- 沿用 **T-002** 的 `downloader.resolveParams` 路径解析与原子 rename 模式（A 把"原子 rename 段"抽到共享 `Install`）。
- 沿用 **T-007** 的 `AtomicWrite` 双 chmod 模式 + UNIQUE 错误文本解析（C.1 批量插入复用 `isDuplicateNameError`）。
- 沿用 **T-014** 的 GitHub API User-Agent 习惯（B 的所有出站 HTTP 都带 UA）。
- 沿用 **T-017** 的 `FRP_EASY_PUBLIC_IP` 短路 + 失败横幅（B 不破坏既有 UI；只扩源）。

---

## 2. 模块 A：二进制上传

### A.1 API 设计

**新端点**：`POST /api/v1/system/upload-bin`

- **Content-Type**：`multipart/form-data`
- **字段白名单**：仅读 `kind`（`frpc` / `frps`）与 `file`（单个 binary），其它字段忽略（NF-A.2）。
- **鉴权**：SessionAuth + CSRF（与 `POST /system/download-bin` 同档；挂在 router.go 的受保护分组）。
- **响应 200**：
  ```json
  {
    "ok": true,
    "kind": "frpc",
    "sha256": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
    "size": 21048576,
    "path": "frp_linux/frpc",
    "advisory": "上传成功；如需立即生效请到运行控制重启 frpc/frps"
  }
  ```
  - `path` 为相对仓库 root 的子路径（NF-S 已有口径）。
  - `advisory` 字段在 frpc/frps 子进程**当前正运行**时才返回（procmgr 查询）；否则该字段省略。
- **错误码**：
  | 状态 | code | 触发 |
  |---|---|---|
  | 400 | VALIDATION_FAILED | 请求非 multipart |
  | 401 | UNAUTHENTICATED | 未登录 |
  | 403 | CSRF_FAILED | 缺 CSRF token |
  | 409 | PROC_BUSY | 同 kind 下载进行中 / 同 kind 上传进行中 |
  | 413 | VALIDATION_FAILED | 文件 > 64 MiB |
  | 422 | VALIDATION_FAILED | 缺字段 / 空文件 / kind 非法 / 文件头非法 / 平台不匹配 |
  | 500 | INTERNAL | 落盘失败、临时文件失败 |

### A.2 后端实现

**关键决策：抽出共享 `Install` helper**。

新文件 `internal/downloader/install.go`：

```go
// Install writes src to the canonical bin path for kind on the current GOOS,
// atomically: temp file → rename → chmod 0o755 (Linux only).
// Total bytes copied = size (caller is responsible for limiting).
// Returns (sha256Hex, bytesWritten, finalPath, error).
//
// maxBytes <= 0 表示不限大小（下载链路走此分支，因 archive 解压后 binary 大小不可预知，
// 由调用方在外层 LimitReader / size check 控制；upload 链路必须传 > 0 的明确上限）。
// maxBytes > 0 时：超过该字节数 → 返回 ErrFileTooLarge sentinel（handler 转 413），
//                  目标文件不写入（临时文件 defer Remove）。
//
// Reused by:
//   - downloader.doDownload (after archive extraction, T-002 / T-014) — 传 maxBytes = -1
//   - handlers_system.uploadBin (T-018, this task) — 传 maxBytes = uploadBinMaxBytes (64 MiB)
func (m *Manager) Install(kind string, src io.Reader, maxBytes int64) (sha256Hex string, written int64, finalPath string, err error)
```

**Install 内部步骤**（提炼自现有 downloader.go L194-251 + 加 sha256）：

1. `resolveParams(kind, goos)` 拿 `targetDir/targetPath`（沿用现有方法，唯一区别：上传路径不需要 `archiveExt/entryName`，仅前两个返回值）。
2. `os.MkdirAll(targetDir, 0o755)`（fail-closed）。
3. `tmp, _ := os.CreateTemp(targetDir, ".upload-bin-*.tmp")`；defer Remove 临时文件（成功 rename 后此 Remove 无副作用）。
4. `hasher := sha256.New()`；`tee := io.TeeReader(src, hasher)`；`written, _ = io.Copy(tmp, io.LimitReader(tee, maxBytes+1))`。
5. 若 `written > maxBytes` → 返回 `errFileTooLarge` sentinel（handler 转 413）。
6. `tmp.Sync()` + `tmp.Close()`。
7. **原子 rename + Windows fallback**（直接复制现有 L229-244 行为）：
   - `os.Rename(tmpPath, targetPath)`
   - Windows 失败 → `os.Remove(targetPath)`（忽略 ErrNotExist）→ retry rename
8. Linux: `os.Chmod(targetPath, 0o755)`（沿用 L247，best-effort warn）。
9. 返回 `(hex.EncodeToString(hasher.Sum(nil)), written, targetPath, nil)`。

> 提炼后 `doDownload` 的"step 3 + step 4"（L224-251）改为调 `Install(kind, openedBinTmp, maxBytes=-1)`，沿用旧行为；这是**纯重构**，零行为变更，单测 `downloader_test.go` 保证回归。

**新 handler** `handlers_system.uploadBin`（在现有 `handlers_system.go` 追加）：

```go
const uploadBinMaxBytes = 64 << 20 // 64 MiB (PM-DECIDED)

var (
    uploadLockFrpc sync.Mutex
    uploadLockFrps sync.Mutex
)

func (h *handlers) uploadBin(w http.ResponseWriter, r *http.Request) {
    // B-6 修订：放弃 MultipartReader 流式 + "客户端 append 顺序" 假设，
    //   改用 ParseMultipartForm + FormValue + FormFile 模式 —— 不依赖客户端字段顺序，
    //   且 multipart 库会把 file 落到磁盘临时文件（受 maxMemory 控制，超部分自动 spill），
    //   不会全文入内存，故仍然安全。
    const maxBodyBytes = uploadBinMaxBytes + (1 << 20)        // +1 MiB 容 multipart 包头
    const parseMemory = int64(8 << 20)                          // 8 MiB 走内存，剩余 spill 到 tmp

    // 1. MaxBytesReader 在 ParseMultipartForm **之前**裹一层（防 OOM；超大请求直接 413）
    r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

    // 2. 解析 multipart（顺序无关，自动磁盘 spill）
    if err := r.ParseMultipartForm(parseMemory); err != nil {
        // MaxBytesReader 触发时 err 是 *http.MaxBytesError；与"非 multipart"区分
        if errors.As(err, new(*http.MaxBytesError)) {
            writeError(w, 413, CodeValidationFailed, "文件超过 64 MiB 上限", "file") ; return
        }
        writeError(w, 400, CodeValidationFailed, "请求不是合法的 multipart/form-data", "") ; return
    }

    // 3. 读字段（顺序无关）
    kind := strings.TrimSpace(r.FormValue("kind"))
    if kind != "frpc" && kind != "frps" {
        writeError(w, 422, CodeValidationFailed, "kind 必须为 frpc 或 frps", "kind") ; return
    }
    file, fh, ferr := r.FormFile("file")
    if ferr != nil {
        writeError(w, 422, CodeValidationFailed, "缺少字段：file", "file") ; return
    }
    defer file.Close()
    if fh.Size == 0 {
        writeError(w, 422, CodeValidationFailed, "上传文件为空", "file") ; return
    }
    if fh.Size > int64(uploadBinMaxBytes) {
        writeError(w, 413, CodeValidationFailed, "文件超过 64 MiB 上限", "file") ; return
    }

    // 4. 与下载/上传互斥
    lk := pickUploadLock(kind)
    if !lk.TryLock() {
        writeError(w, 409, CodeProcBusy, "上传进行中，请稍后重试", "") ; return
    }
    defer lk.Unlock()
    if st, _ := h.deps.Downloader.Status(kind); st.Status == downloader.StatusDownloading {
        writeError(w, 409, CodeProcBusy, "下载进行中，请稍后再上传或取消下载", "") ; return
    }

    // 5. peek 文件头校验（必须在落盘前；用 bufio.Reader.Peek 不消费流）
    br := bufio.NewReaderSize(file, 4096)
    head, _ := br.Peek(64)  // PE/ELF 头都在前 64 字节内
    if len(head) == 0 {
        writeError(w, 422, CodeValidationFailed, "上传文件为空", "file") ; return
    }
    if err := validateBinaryHeader(head, runtime.GOOS); err != nil {
        writeError(w, 422, CodeValidationFailed, err.Error(), "file") ; return
    }

    // 6. 走 downloader.Install
    sha, n, finalPath, err := h.deps.Downloader.Install(kind, br, uploadBinMaxBytes)
    if errors.Is(err, downloader.ErrFileTooLarge) {
        writeError(w, 413, CodeValidationFailed, "文件超过 64 MiB 上限", "file") ; return
    }
    if err != nil { writeError(w, 500, ...) ; return }

    // 7. 检查 frpc/frps 是否在跑（决定是否返回 advisory）
    // B-3 修订：procmgr 实际签名是 `func (m *Manager) Status(kind string) ProcessInfo`（单返回值），
    //   `State` 类型是 `procmgr.State`（string alias）、应用 `procmgr.StateRunning` 常量比对
    advisory := ""
    if h.deps.ProcMgr != nil {
        info := h.deps.ProcMgr.Status(kind)
        if info.State == procmgr.StateRunning {
            advisory = "上传成功；如需立即生效请到运行控制重启 " + kind
        }
    }

    // 8. 日志 + 响应
    h.deps.Logger.Info("upload-bin success", "kind", kind, "size", n, "sha256", sha, "elapsed_ms", ...)
    writeJSON(w, 200, UploadBinResponse{Ok: true, Kind: kind, SHA256: sha, Size: n,
        Path: relativeFromRoot(finalPath, h.deps.Locator.Root()), Advisory: advisory})
}
```

**文件头校验函数**：

```go
// validateBinaryHeader：仅接受与运行平台一致的二进制（FR-A.4）。
// linux: ELF magic = 0x7F 'E' 'L' 'F' (4 bytes)
// windows: PE magic = 'M' 'Z' (2 bytes)
//
// B-11 修订：**仅 MZ 即接受 PE**。不再做 offset 0x3C 处的 PE\0\0 二次校验
//   （原方案 peek 64 字节，0x3C 偏移本身只占 4 字节、足够读 peOff，但 PE\0\0
//    签名可能落在 64 字节之外、peek 不到）。设计上接受 MZ 后落盘即可；若
//    后续 procmgr 启动该 binary 失败（非真正可执行），由 procmgr 的 `lastErr`
//    暴露给前端 —— 错误已有显示通道，不需要在校验层重复设关。
func validateBinaryHeader(head []byte, goos string) error {
    if len(head) < 4 {
        return errors.New("不是合法的二进制文件（文件过短）")
    }
    isELF := head[0] == 0x7F && head[1] == 'E' && head[2] == 'L' && head[3] == 'F'
    isPE  := len(head) >= 2 && head[0] == 'M' && head[1] == 'Z'
    switch goos {
    case "linux":
        if !isELF {
            if isPE { return errors.New("上传的二进制平台不匹配（本机=linux，文件=windows）") }
            return errors.New("不是合法的二进制文件（缺少 ELF 文件头）")
        }
    case "windows":
        if !isPE {
            if isELF { return errors.New("上传的二进制平台不匹配（本机=windows，文件=linux）") }
            return errors.New("不是合法的二进制文件（缺少 PE 文件头）")
        }
    default:
        return errors.New("不支持的操作系统")
    }
    return nil
}
```

**path 公开方式**：响应里的 `path` 调用 `filepath.Rel(locator.Root(), finalPath)` + `filepath.ToSlash`，保证返回 `frp_linux/frpc` 这种相对路径（NF-S 既有口径）。

### A.3 前端实现

**新组件** `web/src/components/UploadBinButton.vue`：

- props：`kind: 'frpc' | 'frps'`
- 内部用 `<input type="file" accept=".exe" ref="fileInput" hidden>` + `<n-button @click="trigger">`。
- 选定文件后 → `apiUploadBin(kind, file, onProgress)`。
- 用 `axios` 的 `onUploadProgress` 显示百分比（NProgressBar 或 inline NProgress）。
- 错误分类提示：
  - 413 → `"文件超过 64 MiB 上限（请确认上传的是单 binary 而不是 .tar.gz / .zip）"`
  - 422 + field=file/平台不匹配 → 透传后端中文消息
  - 409 PROC_BUSY → `"下载或上传正在进行，请稍后重试"`
- 成功后 NMessage success，触发 `emit('uploaded', {sha256, size})` 让父组件刷新 `system/ready`。

**API client** 在 `web/src/api/system.ts` 追加：

```ts
export async function apiUploadBin(
  kind: 'frpc' | 'frps',
  file: File,
  onProgress?: (pct: number) => void,
): Promise<UploadBinResponse> {
  const fd = new FormData()
  fd.append('kind', kind)
  fd.append('file', file)  // 必须在 kind 之后 append（与后端 MultipartReader 流式假设一致）
  const res = await apiClient.post<UploadBinResponse>('/api/v1/system/upload-bin', fd, {
    // B-2 修订：**禁止**显式设置 Content-Type
    //   留给 axios 自动加 `multipart/form-data; boundary=...`
    //   显式写 'multipart/form-data' 会丢 boundary，服务端 MultipartReader 直接 400
    onUploadProgress: (e) => {
      if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100))
    },
    timeout: 120_000, // 64MiB on 慢链路最长 120s
  })
  return res.data
}
```

**挂载点（B-4 修订收紧）**：实测 grep 确认 `web/src/pages/Wizard.vue` 当前**没有**任何下载入口
（`frpc / frps / handleDownload / downloaderStore` 均零命中）；现有"一键下载 frpc / frps"
按钮**仅在** `web/src/components/AppLayout.vue` 顶部 binMissing banner 内（L21-29）。
本任务**仅在** AppLayout.vue banner 的下载按钮组**并列**追加 `<UploadBinButton :kind="..." @uploaded="refreshReady">`：

- 下载按钮 tooltip：「从 GitHub Releases 自动拉取最新版（境内可能失败）」
- 上传按钮 tooltip：「本地选择已下载好的 frpc/frps 二进制（适合 GitHub 不可达时使用）」

**显式排除**：Wizard.vue 当前无下载入口，本任务**不**在 Wizard 新增上传/下载按钮
（避免凭空引入新 UX 入口；如未来需要可在后续任务追加）。Partition 表已同步移除 Wizard.vue 编辑项。

### A.4 测试点

- **单测**（`downloader/install_test.go`）：
  - `Install_HappyPath`：write 1 KiB content，验证 sha256 一致、目标文件存在、Linux 下 mode = 0o755。
  - `Install_TooLarge`：write `maxBytes+1` 字节，验证返回 `ErrFileTooLarge` 且目标文件不存在。
  - `Install_WindowsFallback`：在 Linux runner 用 `goos="windows"` 注入 + 预先 `os.WriteFile(targetPath, ...)`，验证 rename 走 Remove+Rename 路径（用 `runtime.GOOS == "windows"` 短路意味着这一段必须用现有 `goosFunc` seam 注入，否则 Linux 跑不到 fallback；T-010 insight 已有此模式）。
- **单测**（`httpapi/handlers_upload_test.go`）：
  - `UploadBin_BadKind` → 422
  - `UploadBin_TxtAsELF` → 422 + "缺少 ELF 文件头"
  - `UploadBin_PEonLinux` → 422 + "平台不匹配"
  - `UploadBin_Success` → 200 + 返回正确 sha256
  - `UploadBin_TooLarge` → 413（用 io.LimitReader 模拟超大请求，断言内存不爆）
  - `UploadBin_NoCSRF` → 403
  - `UploadBin_Unauth` → 401
- **AC 映射**：覆盖 AC-A.1 ~ AC-A.10。
- **Adversarial（B-10 修订新增）**：QA 阶段在 `06_TEST_REPORT.md` 的 `## Adversarial tests` 段
  落一条 `nginx_client_max_body_size`：用 docker-compose 起 nginx 反代（默认 1 MiB body 上限），
  上传 25 MiB binary，断言得到反代层的清晰 413/4xx 错误而非超时；若 QA 环境不便起 nginx，
  则在 `docs/DEPLOYMENT.md` 文档化为 known limitation（已在 R-1 / R-16 提示）并标"未实测"。

---

## 3. 模块 B：公网 IP 大陆友好源

### B.1 后端改造

**改造目标文件**：`internal/httpapi/handlers_system.go` 的 `fetchPublicIP`。

**新候选源数组**（FR-B.1）：

```go
type ipSource struct {
    name    string
    url     string
    parser  func([]byte) (string, error)   // JSON / HTML 不同 parser
    maxBody int64                            // body 读取上限
}

var ipSources = []ipSource{
    // 国际源（沿用，兼容境外用户）
    {"ipify",    "https://api.ipify.org?format=json",          parseIPFromJSONField, 32 << 10},
    {"my-ip.io", "https://api.my-ip.io/json",                  parseIPFromJSONField, 32 << 10},
    // 大陆友好源（PM-DECIDED 候选清单）
    {"ip.cn",    "https://ip.cn/api/index?ip=&type=0",         parseIPCnJSON,        32 << 10},
    {"bilibili", "https://api.live.bilibili.com/ip_service/v1/ip_service/get_ip_addr",
                                                                parseBilibiliJSON,    32 << 10},
    {"ip.cn-html","https://www.ip.cn/",                        parseFirstIPv4FromHTML, 256 << 10},
}
```

**并发探测（FR-B.3）**：

```go
func fetchPublicIP(ctx context.Context) ipResult {
    // FR-B.6: env override short-circuit
    // **NEW（B-1 修订）**：`FRP_EASY_PUBLIC_IP` 此前仅在 `scripts/install.sh` / `scripts/install.ps1`
    // 安装期读取（T-017），Go 端 grep 零命中。本任务**首次**在 Go 后端引入该 env 短路，
    // 把覆盖通道从"安装期"扩展到"运行期"。优先级最高，命中则不发任何 HTTP。
    if v := strings.TrimSpace(os.Getenv("FRP_EASY_PUBLIC_IP")); v != "" {
        r := buildIPResult(v)
        r.Source = "env"   // FR-B.7
        return r
    }

    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    type winner struct { ip, source string }
    ch := make(chan winner, len(ipSources))
    var wg sync.WaitGroup
    for _, src := range ipSources {
        wg.Add(1)
        go func(s ipSource) {
            defer wg.Done()
            ip, err := fetchIPFromSource(ctx, s)
            if err == nil && ip != "" {
                select { case ch <- winner{ip, s.name}: default: }
            }
        }(src)
    }
    go func() { wg.Wait(); close(ch) }()

    select {
    case w, ok := <-ch:
        if ok {
            cancel()   // 取消其它在飞请求
            r := buildIPResult(w.ip)
            r.Source = w.source
            return r
        }
    case <-ctx.Done():
    }
    // 全部失败或超时
    return ipResult{ErrMsg: "检测超时，请手动查询"}
}
```

**HTML parser**（FR-B.5）：

```go
var ipv4RE = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

func parseFirstIPv4FromHTML(data []byte) (string, error) {
    matches := ipv4RE.FindAll(data, -1)
    for _, m := range matches {
        if ip := net.ParseIP(string(m)); ip != nil && ip.To4() != nil {
            // 排除本地保留段（防 HTML 里嵌广告 192.168.x.x 污染）
            if !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
                return string(m), nil
            }
        }
    }
    return "", errors.New("HTML 中未提取到合法公网 IPv4")
}
```

**ip.cn / bilibili JSON parser**：

```go
// ip.cn: {"code":0,"data":{"ip":"1.2.3.4",...}} 也可能直接顶层 ip
type ipCnResp struct {
    IP   string `json:"ip"`
    Data struct{ IP string `json:"ip"` } `json:"data"`
}

// bilibili: {"code":0,"data":{"addr":"1.2.3.4",...}}
type biliResp struct {
    Data struct{ Addr string `json:"addr"` } `json:"data"`
}
```

**所有请求带 UA**（FR-B.8 + T-014 insight L37）：

```go
func fetchIPFromSource(ctx context.Context, s ipSource) (string, error) {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
    req.Header.Set("User-Agent", "frp_easy")
    req.Header.Set("Accept", "application/json,text/html")
    resp, err := http.DefaultClient.Do(req)
    if err != nil { return "", err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return "", fmt.Errorf("HTTP %d", resp.StatusCode) }
    data, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBody))
    if err != nil { return "", err }
    ip, err := s.parser(data)
    if err != nil { return "", err }
    if net.ParseIP(ip) == nil { return "", errors.New("非合法 IP") }  // FR-B.4
    return ip, nil
}
```

**响应体扩展**：

```go
type PublicIPResponse struct {
    IP       string `json:"ip,omitempty"`
    Error    string `json:"error,omitempty"`
    Advisory string `json:"advisory,omitempty"`
    Source   string `json:"source,omitempty"`   // FR-B.7 新增可选
}
```

注意：`ipResult` 内部结构同样加 `Source string`；`respondWithIPResult` 透传。

### B.2 测试点

- **单测**（`handlers_system_publicip_test.go` 新增）：
  - `PublicIP_EnvOverride` → AC-B.4：设 `FRP_EASY_PUBLIC_IP=10.0.0.5`，断言响应 `{ip:"10.0.0.5", source:"env"}` 且 `http.DefaultClient` 计数为 0（用 RoundTripper mock 验证）。
  - `PublicIP_ParallelFirstWins` → 起 5 个 httptest.NewServer，最快的延迟 50ms、其余 500ms+；断言总耗时 < 200ms 且 source 是最快那个。
  - `PublicIP_HTMLPolluted` → AC-B.7：mock 返回 1 MiB HTML，body 头部含 `192.168.1.1`、200KB 处含 `8.8.8.8`，断言抽取到 `8.8.8.8`（私有段被跳过）。
  - `PublicIP_AllFail` → AC-B.2：所有 mock 502，断言 `{error:"检测超时，请手动查询"}`。
  - `PublicIP_NonIPText` → AC-B.3：单源返回 `"not-an-ip"`，断言跳过 + 用其它源。
  - `PublicIP_IPv6` → AC-B.4 IPv6 兼容：mock 返回 `2001:db8::1`，断言 advisory 含方括号提示。
  - `PublicIP_UAHeader` → AC-B.8：httptest handler 内 assert `r.UserAgent() == "frp_easy"`。

> 测试必须用 httptest 注入；现有 `ipSources` 是包级常量需要测试 seam。方案：把 `ipSources` 改为 `var`（可测试时替换），或者改 `fetchPublicIP` 接受 `sources []ipSource` 参数，在 handler 内再绑生产清单。**采用后者**（接受 sources 参数 → handler 层默认传 `defaultIPSources`），符合 T-010 insight L29 的"可注入 seam"模式。

### B.3 UI 不动

`PublicIpDetector.vue` 不改；新 `source` 字段如未展示则前端忽略（兼容契约）。如 UI 想展示可在 NAlert subtitle 显示 `"（来源：ip.cn）"`，但本期非强制 —— 留给后续 polish。

---

## 4. 模块 C：多端口转发 + 预设 + 探测

### C.1 批量端口创建

#### C.1.1 端点设计

`POST /api/v1/proxies/batch`

**请求体**：
```json
{
  "basename": "web",
  "type": "tcp",
  "localIP": "127.0.0.1",
  "portsExpr": "6000-6010,7000",
  "enabled": true
}
```

- `basename`：派生 name 的前缀，`^[A-Za-z0-9_-]{1,58}$`（留 6 字符给 `-65535` 后缀）。
- `type`：`tcp` / `udp` 二选一（http/https 走域名，batch 不适用，FR-C.1.1）。
- `portsExpr`：端口表达式（详见 portrange 包）。
- `localIP`：可选，缺省 `127.0.0.1`。
- `enabled`：可选，缺省 `true`。

> **关键决策**：localPort 与 remotePort 必须 1:1 映射，**不**支持单独的 localPortSpec（FR-C.1.2 / R-6）；用户表达"本地 6000 → 远程 7000"得用单条 POST，而非 batch。

**响应 201**：
```json
{
  "created": 12,
  "items": [
    { "id": 17, "name": "web-6000", "type": "tcp", "localIP": "127.0.0.1",
      "localPort": 6000, "remotePort": 6000, "enabled": true, "version": 1,
      "updatedAt": "2026-05-23T..." },
    ...
  ]
}
```

**错误**：
| 状态 | code | 触发 |
|---|---|---|
| 422 | VALIDATION_FAILED | portsExpr 非法 / basename 非法 / 端口 > 32 / 总数 > 200 |
| 409 | CONFLICT | name 派生后冲突（含已存在）或 (type,remote_port) 冲突；返回明细 |
| 500 | INTERNAL | 事务失败 |

422 / 409 响应体携带冲突明细（PM-DECIDED R-3）：
```json
{
  "error": {
    "code": "CONFLICT",
    "message": "批量创建因 2 条冲突回滚",
    "conflicts": [
      { "port": 6003, "reason": "name 已存在: web-6003" },
      { "port": 6004, "reason": "(tcp, 6004) 已存在" }
    ]
  }
}
```

> 这是对现有 `ErrorBody` 的扩展（追加可选 `conflicts` 字段），不破坏既有契约。

#### C.1.2 portrange 共享包

**新包** `internal/portrange/portrange.go`：

```go
// Package portrange 解析端口表达式（"6000-6010,7000,8000-8002"）。
// 仅用 stdlib；不引入新依赖。
package portrange

// Parse 解析端口表达式，返回去重后的有序端口数组。
// 限制：单次最多 maxCount 个端口；端口 ∈ [1, 65535]。
//
// 语法：
//   token := <int>                    // 单端口
//          | <int> "-" <int>          // 闭区间，左 ≤ 右
//   expr := token ("," token)*        // 逗号分隔，空格被 trim
//
// 错误：
//   - ErrEmpty             : 表达式空
//   - ErrBadSyntax         : 含非法字符 / 段为空
//   - ErrPortOutOfRange    : 任一端口 < 1 或 > 65535
//   - ErrRangeReversed     : 左 > 右（如 "6010-6000"）
//   - ErrDuplicate(port)   : 展开后含重复（带具体 port 值用于 422 消息）
//   - ErrTooMany(count, max): 总数 > maxCount
func Parse(expr string, maxCount int) ([]int, error)
```

**为什么独立成包**：portrange 解析在 handler 与（未来）CLI/import 都可能用；放 `internal/httpapi/` 会被 httpapi 包紧耦合。独立包便于单测 + 复用。

#### C.1.3 storage 层：事务版批量插入

**修改** `internal/storage/proxies.go`，新增方法：

```go
// UpsertProxiesTx 在单事务内插入 ps 数组。任一失败全部回滚。
// 限制：仅支持新建（要求 p.ID == 0）；调用方保证。
// 总条数上限校验放在 handler 层（需要先 count 现有数）。
//
// 返回值：
//   - 全部成功：(insertedIDs, nil)，p.ID/p.Version/p.UpdatedAt 已回填到 ps 切片。
//   - 任一冲突：(nil, conflictErr)，事务回滚，DB 完全不变。
//
// 冲突类型映射：
//   - name UNIQUE → 调用 isDuplicateNameError 后包成 ErrDuplicateName（沿用 T-007）
//   - (type, remote_port) 部分唯一索引 → 包成 ErrDuplicateTcpRemote（新 sentinel）
func (s *Store) UpsertProxiesTx(ctx context.Context, ps []*Proxy) ([]int64, error)
```

**新 sentinel**：`ErrDuplicateTcpRemote = errors.New("storage: duplicate (type, remote_port)")`

**B-8 修订（关键字以单测断言为准）**：modernc sqlite 在**复合 UNIQUE 索引**报错时的
具体文本（如 `UNIQUE constraint failed: proxies.type, proxies.remote_port`）
**在本项目历史任务中未实证过**（insight-index L16 的样例是单列 `proxies.name`）。
Developer 在实现 `isDuplicateTcpRemote` 前**必须**先写一条 storage adversarial 单测：
故意 insert `(tcp, 6000)` 两次，捕获 `err.Error()` 打印出来，**以真实文本为准**
确定 `strings.Contains` 的关键字串。**不要**直接照搬 `isDuplicateNameError` 的模板。

**B-5 修订（并发约束）**：`UpsertProxiesTx` **必须**整段持 `s.mu.Lock()` —— 在 `BeginTx`
**之前** Lock，`Commit/Rollback` **之后** Unlock。理由：
- `internal/storage/store.go` `SetMaxOpenConns(1)` → sqlite 单连接，所有写依赖 `s.mu` 串行化；
- 现有 `UpsertProxy/DeleteProxy` 都持 `s.mu`；若 `UpsertProxiesTx` 不持锁，事务内 INSERT
  与并行 `UpsertProxy` 共享物理连接 → `database is locked` 概率虽低但语义错误；
- 与现有 `UpsertProxy` 保持同一串行化约束、零行为差异。

伪代码骨架：
```go
func (s *Store) UpsertProxiesTx(ctx context.Context, ps []*Proxy) ([]int64, error) {
    s.mu.Lock()              // B-5：BeginTx 前先 Lock
    defer s.mu.Unlock()      // Commit/Rollback 后自然 Unlock
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil { return nil, err }
    defer tx.Rollback()      // 提前 Commit 后 Rollback 是 no-op
    // ... 逐条 INSERT；任一失败 return → 自动 Rollback ...
    if err := tx.Commit(); err != nil { return nil, err }
    return ids, nil
}
```

**新增 storage 并发单测**（必测）：起两个 goroutine，A 跑 `UpsertProxy`、B 跑 `UpsertProxiesTx`
（互相端口/name 不冲突），断言两者都成功且无 `database is locked` error。该单测固化"两路径共享 `s.mu` 串行化"的契约，未来重构若退化为并行也会立即 fail。

事务失败 → defer 自动 `tx.Rollback`。

**Handler 实现** `handlers_proxies.batchProxies`：

```go
func (h *handlers) batchProxies(w http.ResponseWriter, r *http.Request) {
    var req BatchProxiesRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, 400, ...) ; return
    }

    // 1. basename / type 校验
    if !batchBasenameRE.MatchString(req.Basename) {   // ^[A-Za-z0-9_-]{1,58}$
        writeError(w, 422, CodeValidationFailed, "basename 非法或过长（≤58 字符）", "basename") ; return
    }
    if req.Type != "tcp" && req.Type != "udp" {
        writeError(w, 422, ..., "batch 仅支持 tcp/udp", "type") ; return
    }

    // 2. 端口表达式解析
    ports, err := portrange.Parse(req.PortsExpr, 32)   // FR-C.1.3 上限 32
    if err != nil {
        writeError(w, 422, ..., humanizePortRangeErr(err), "portsExpr") ; return
    }

    // 3. 总数上限（叠加）
    existing, _ := h.deps.Store.ListProxies(r.Context())
    if len(existing) + len(ports) > 200 {
        writeError(w, 422, ..., "代理规则已达上限（200 条），请删除部分规则后重试", "") ; return
    }

    // 4. 构造 storage.Proxy 数组
    ps := make([]*storage.Proxy, 0, len(ports))
    for _, p := range ports {
        rp := p
        ps = append(ps, &storage.Proxy{
            Name: fmt.Sprintf("%s-%d", req.Basename, p),
            Type: req.Type,
            LocalIP: defaultLocalIP(req.LocalIP),
            LocalPort: p,
            RemotePort: &rp,
            Enabled: deref(req.Enabled, true),
        })
    }

    // 5. 单事务插入
    ids, err := h.deps.Store.UpsertProxiesTx(r.Context(), ps)
    if err != nil {
        writeBatchConflict(w, err, ps)   // 422/409 + conflicts 明细
        return
    }

    // 6. 一次性 reload frpc（FR-C.1.8，不每条都触发）
    h.applyConfigBestEffort(r.Context(), "frpc")

    // 7. 响应
    out := make([]ProxyResponse, len(ps))
    for i, p := range ps { out[i] = toResponse(*p) }
    writeJSON(w, 201, BatchProxiesResponse{Created: len(ids), Items: out})
}
```

#### C.1.4 前端

`ProxyForm.vue` 扩展：

- 新增 `portMode: 'single' | 'batch'`（仅 type=tcp/udp 时显示切换器；http/https 强制 single）。
- batch 模式：隐藏 localPort / remotePort，显示单个"端口表达式"`<n-input>` + 帮助文本"如 `6000-6010,7000`，最多 32 个端口"。
- batch 模式：name 字段变为 "基础名称（basename）"，hint "实际名称会派生为 basename-端口号"。
- 提交时根据 `portMode` 分流：single 走 `apiCreateProxy`，batch 走 `apiBatchCreateProxies`。

`Proxies.vue` 列表层视图增强（**关键决策：纯前端折叠，DB 还是独立行**）：

- **B-12 修订**：按 `name` 的 `^(.+)-(\d{1,5})$` 模式 group（greedy `.+` 取最后一段连字符前
  作为 basename，最后一段必须是 1-5 位数字作为 port）。相同 basename 且 port 是数字的
  折叠成"组"，列表展示一行 `web-* (12 条)`，可展开看 12 条详情。
- 旧正则 `^([^-]+)-(\d+)$` 会把 `my-web-6000` 错切为 basename=`my`、port=`web-6000`；
  新正则把它正确切为 basename=`my-web`、port=`6000`。
- 前端单测必须覆盖：`web-6000`（普通）、`my-web-6000`（basename 含 `-`）、`a-b-c-22`
  （basename 多 `-`）、`web-notaport`（非数字尾段，不折叠保持单条）、`abc`（无 `-`，不折叠）。
- 折叠开关默认开；用户可点关闭恢复扁平列表。
- 实现：computed property 把 `proxiesStore.proxies` 转成 `displayRows`，每行可能是 `{kind:'single', proxy}` 或 `{kind:'group', basename, members: Proxy[]}`。

不引入新表 / 不引入新字段（KISS）。

`web/src/api/proxies.ts` 追加：

```ts
export interface BatchProxiesRequest {
  basename: string
  type: 'tcp' | 'udp'
  localIP?: string
  portsExpr: string
  enabled?: boolean
}
export interface BatchProxiesResponse {
  created: number
  items: Proxy[]
}
export async function apiBatchCreateProxies(req: BatchProxiesRequest): Promise<BatchProxiesResponse> {
  const res = await apiClient.post<BatchProxiesResponse>('/api/v1/proxies/batch', req)
  return res.data
}
```

`web/src/types.ts` 追加 `BatchProxiesRequest` / `BatchProxiesResponse`。

#### C.1.5 测试点

**Go 单测** `portrange_test.go`（table-driven）：

| 输入 | maxCount | 期望 |
|---|---|---|
| `""` | 32 | ErrEmpty |
| `"abc"` | 32 | ErrBadSyntax |
| `"6000-"` | 32 | ErrBadSyntax |
| `"6000-7000"` | 32 | ErrTooMany(1001, 32) |
| `"6010-6000"` | 32 | ErrRangeReversed |
| `"0,80"` | 32 | ErrPortOutOfRange |
| `"70000"` | 32 | ErrPortOutOfRange |
| `"80,80"` | 32 | ErrDuplicate(80) |
| `"6000-6005,6003"` | 32 | ErrDuplicate(6003) |
| `"  22 , 80, 443 "` | 32 | [22,80,443] |
| `"6000-6010,7000"` | 32 | [6000..6010,7000] |

**Storage 单测** `proxies_batch_test.go`：
- `UpsertProxiesTx_HappyPath`：5 条全成功，返回 5 个 ID。
- `UpsertProxiesTx_RollbackOnNameDup`：第 3 条 name 与 DB 已有冲突 → 返回 ErrDuplicateName，DB 行数不变。
- `UpsertProxiesTx_RollbackOnTcpDup`：第 3 条 (tcp, 6003) 与 DB 已有冲突 → ErrDuplicateTcpRemote，DB 行数不变。
- **`UpsertProxiesTx_ConcurrentWithUpsertProxy`（B-5 修订新增）**：goroutine A 跑 `UpsertProxy`、
  goroutine B 跑 `UpsertProxiesTx`（端口/name 互不冲突），断言两者均 nil error 且 DB 最终
  行数为两路径之和；不出现 `database is locked`。固化 `s.mu` 串行化契约。

**Handler 单测** `handlers_batch_test.go`：覆盖 AC-C.1.1 ~ AC-C.1.8 全部。

### C.2 常用端口预设

**新文件** `web/src/composables/usePortPresets.ts`（按需求 NF-C.3 集中导出）：

```ts
export interface PortPreset {
  label: string       // "SSH 22"
  port: number
  category: 'remote' | 'db' | 'web' | 'file'
  tagType: 'info' | 'success' | 'warning' | 'default'
  hint?: string        // "MySQL 通常用 TCP"
}

export const PORT_PRESETS: PortPreset[] = [
  { label: 'SSH 22',         port: 22,    category: 'remote', tagType: 'info' },
  { label: 'RDP 3389',       port: 3389,  category: 'remote', tagType: 'info' },
  { label: 'VNC 5900',       port: 5900,  category: 'remote', tagType: 'info' },
  { label: 'HTTP 80',        port: 80,    category: 'web',    tagType: 'success' },
  { label: 'HTTPS 443',      port: 443,   category: 'web',    tagType: 'success' },
  { label: 'HTTP-Alt 8080',  port: 8080,  category: 'web',    tagType: 'success' },
  { label: 'MySQL 3306',     port: 3306,  category: 'db',     tagType: 'warning', hint: 'MySQL 通常用 TCP' },
  { label: 'PostgreSQL 5432',port: 5432,  category: 'db',     tagType: 'warning' },
  { label: 'Redis 6379',     port: 6379,  category: 'db',     tagType: 'warning' },
  { label: 'MongoDB 27017',  port: 27017, category: 'db',     tagType: 'warning' },
  { label: 'SMB 445',        port: 445,   category: 'file',   tagType: 'default' },
  { label: 'FTP 21',         port: 21,    category: 'file',   tagType: 'default' },
]

export function usePortPresets() {
  return { presets: PORT_PRESETS }
}
```

**ProxyForm.vue 集成**：

- localPort 输入框下方放 `<n-space>` + `<n-tag size="small" v-for="p in presets" :type="p.tagType" @click="applyPreset(p)" style="cursor:pointer">{{ p.label }}</n-tag>`。
- single 模式：点 Tag → `form.localPort = p.port`，同时如 type=tcp/udp 且 remotePort 为空 → 也填 remotePort（一致映射默认）。
- batch 模式：点 Tag → 若 `form.portsExpr` 非空则追加 `,p.port`，否则填 `p.port`。
- 不强制改 type；显示 `<n-text depth=3>{{ p.hint }}</n-text>` 作为副提示（如 MySQL → "通常用 TCP"）。

### C.3 端口可用性探测

#### C.3.1 端点

`POST /api/v1/system/port-probe`

**请求体**：`{ "ports": [22, 80, 6000] }`

**响应 200**：
```json
{
  "results": [
    { "port": 22,   "available": false, "reason": "特权端口（<1024）需以 root/Administrator 启动 frp_easy 才能绑定" },
    { "port": 80,   "available": false, "reason": "端口已被占用或受限" },
    { "port": 6000, "available": true }
  ]
}
```

**错误**：
| 状态 | code | 触发 |
|---|---|---|
| 401 / 403 | UNAUTHENTICATED / CSRF_FAILED | 未登录 / 缺 CSRF |
| 422 | VALIDATION_FAILED | 端口列表 > 64 / 端口 < 1 或 > 65535 |

#### C.3.2 后端实现

新 handler `handlers_system.portProbe`：

```go
const portProbeMaxCount = 64
const portProbeTimeout = 200 * time.Millisecond

type portProbeReq struct {
    Ports []int `json:"ports"`
}
type portProbeResult struct {
    Port      int    `json:"port"`
    Available bool   `json:"available"`
    Reason    string `json:"reason,omitempty"`
}
type portProbeResp struct {
    Results []portProbeResult `json:"results"`
}

func (h *handlers) portProbe(w http.ResponseWriter, r *http.Request) {
    var req portProbeReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, 400, ...) ; return
    }

    // 1. 去重（保持原顺序）
    seen := make(map[int]bool, len(req.Ports))
    ports := make([]int, 0, len(req.Ports))
    for _, p := range req.Ports {
        if !seen[p] {
            seen[p] = true
            ports = append(ports, p)
        }
    }

    // 2. 上限校验
    if len(ports) > portProbeMaxCount {
        writeError(w, 422, ..., "单次最多探测 64 个端口", "ports") ; return
    }
    for _, p := range ports {
        if p < 1 || p > 65535 {
            writeError(w, 422, ..., "端口必须在 1-65535 之间", "ports") ; return
        }
    }

    // 3. 探测（并发，每端口 200ms timeout）
    results := make([]portProbeResult, len(ports))
    var wg sync.WaitGroup
    for i, p := range ports {
        wg.Add(1)
        go func(i, port int) {
            defer wg.Done()
            results[i] = probeOnePort(port)
        }(i, p)
    }
    wg.Wait()
    writeJSON(w, 200, portProbeResp{Results: results})
}

func probeOnePort(port int) portProbeResult {
    if port < 1024 {
        return portProbeResult{Port: port, Available: false,
            Reason: "特权端口（<1024）需以 root/Administrator 启动 frp_easy 才能绑定"}
    }
    // B-9 修订：用 `":N"`（dual-stack wildcard）而非 `"0.0.0.0:N"`（IPv4-only）。
    //   理由：Windows 上 `0.0.0.0:N` 仅探 IPv4，若端口被 IPv6-only 监听者占用 →
    //   误判为 available。`":N"` 让 Go runtime 走 dual-stack（v4mapped），
    //   与 frp 实际绑定行为（默认监听所有协议族）更接近。
    //   Linux 行为一致（`tcp` network 即双栈），无回归。
    ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        return portProbeResult{Port: port, Available: false,
            Reason: "端口已被占用或受限"}
    }
    _ = ln.Close()
    return portProbeResult{Port: port, Available: true}
}
```

> **安全**：硬编码本机 wildcard 监听（`":<port>"`，即所有协议族 + 所有接口），
> **不接受 host 参数**（FR-C.3.7 / R-8 / AC-C.3.7）。这等价于 0.0.0.0 + :: 双栈通配，
> 不暴露对任意远程主机的扫描能力。

#### C.3.3 前端

`ProxyForm.vue` localPort 字段右侧加 `<n-button @click="probe">探测可用性</n-button>`：

- single 模式：POST `[form.localPort]`，结果以 `<n-tag>` 贴在按钮下：绿色"可用"或红色 reason。
- batch 模式：先用 `portrange.Parse` 客户端展开（避免无效请求），上限 32 仍生效，POST 全部端口；逐个 Tag 渲染（最多 32 个，UI 可承受）。
- 列表页 `Proxies.vue`：每行加"远程端口冲突预检"小按钮（仅 type=tcp/udp 且有 remotePort 时），POST `[row.remotePort]`，结果用 `useMessage` 提示。

**新 API client** `web/src/api/system.ts`：

```ts
export interface PortProbeResult {
  port: number
  available: boolean
  reason?: string
}
export async function apiProbePorts(ports: number[]): Promise<PortProbeResult[]> {
  const res = await apiClient.post<{ results: PortProbeResult[] }>(
    '/api/v1/system/port-probe', { ports }
  )
  return res.data.results
}
```

#### C.3.4 测试点

- **单测** `port_probe_test.go`：
  - `Probe_Privileged`：探 80 → available=false, reason 含 "特权端口"（不实际 Listen，所以单测可在无权限环境跑）。
  - `Probe_Available`：先 `ln, _ := net.Listen("tcp", ":0")`，拿 `port := ln.Addr().(*net.TCPAddr).Port`，`ln.Close()` 立即释放，再探这个 port → available=true。
  - `Probe_Occupied`：先 Listen 一个高 port 不 Close，再探它 → available=false。
  - `Probe_OutOfRange`：ports=[70000] → 422。
  - `Probe_TooMany`：65 个端口 → 422。
  - `Probe_Dedup`：[80,80,80] → results 长度 1。
  - `Probe_HostFieldIgnored`：请求体加 `host:"8.8.8.8"`，断言后端忽略（探本机）。
  - `Probe_NoCSRF` → 403；`Probe_Unauth` → 401。

---

## 5. API 契约（openapi.yaml 增量）

> 以下 snippet 为草稿；Developer 阶段直接落地到 `openapi.yaml` 对应位置（`components.schemas` + `paths`）。

### 5.1 新 schemas

```yaml
    UploadBinResponse:
      type: object
      required: [ok, kind, sha256, size, path]
      properties:
        ok: { type: boolean }
        kind: { type: string, enum: [frpc, frps] }
        sha256: { type: string, description: "上传文件的 SHA-256 (hex)" }
        size: { type: integer, format: int64 }
        path: { type: string, description: "相对仓库 root 的相对路径，如 frp_linux/frpc" }
        advisory: { type: string, description: "若同 kind 子进程正在运行，提示重启" }

    BatchProxiesRequest:
      type: object
      required: [basename, type, portsExpr]
      properties:
        basename: { type: string, description: "派生 name 前缀，^[A-Za-z0-9_-]{1,58}$" }
        type: { type: string, enum: [tcp, udp] }
        localIP: { type: string, description: "默认 127.0.0.1" }
        portsExpr: { type: string, example: "6000-6010,7000", description: "端口表达式，最多 32 个端口" }
        enabled: { type: boolean, description: "默认 true" }

    BatchProxiesResponse:
      type: object
      required: [created, items]
      properties:
        created: { type: integer }
        items:
          type: array
          items: { $ref: '#/components/schemas/ProxyResponse' }

    BatchConflict:
      type: object
      properties:
        port: { type: integer }
        reason: { type: string }

    PortProbeRequest:
      type: object
      required: [ports]
      properties:
        ports:
          type: array
          maxItems: 64
          items: { type: integer, minimum: 1, maximum: 65535 }

    PortProbeResponse:
      type: object
      required: [results]
      properties:
        results:
          type: array
          items:
            type: object
            required: [port, available]
            properties:
              port: { type: integer }
              available: { type: boolean }
              reason: { type: string }

    # 扩展现有 ErrorDetail（追加可选 conflicts；不破坏既有契约）
    ErrorDetail:
      type: object
      required: [code, message]
      properties:
        code: { type: string }
        message: { type: string }
        field: { type: string }
        conflicts:
          type: array
          description: "批量操作冲突明细（仅 batch 端点）"
          items: { $ref: '#/components/schemas/BatchConflict' }

    # 扩展 PublicIPResponse（追加可选 source）
    PublicIPResponse:
      type: object
      properties:
        ip: { type: string }
        error: { type: string }
        advisory: { type: string }
        source:
          type: string
          description: "胜出源标识，如 ipify / ip.cn / env（5 min 缓存命中时仍是首次胜出源）"
```

### 5.2 新 paths

```yaml
  /api/v1/system/upload-bin:
    post:
      tags: [system]
      summary: 上传 frpc/frps 二进制（一键下载兜底）
      security:
        - cookieAuth: []
          csrfToken: []
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              required: [kind, file]
              properties:
                kind: { type: string, enum: [frpc, frps] }
                file: { type: string, format: binary }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema: { $ref: '#/components/schemas/UploadBinResponse' }
        '413':
          description: 文件超过 64 MiB
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorBody' }
        '422':
          description: 字段非法 / 文件头非法 / 平台不匹配
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorBody' }
        '409':
          description: 下载或上传进行中
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorBody' }

  /api/v1/proxies/batch:
    post:
      tags: [proxies]
      summary: 批量创建代理规则（单事务，全成或全败）
      security:
        - cookieAuth: []
          csrfToken: []
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/BatchProxiesRequest' }
      responses:
        '201':
          description: 全部创建成功
          content:
            application/json:
              schema: { $ref: '#/components/schemas/BatchProxiesResponse' }
        '422':
          description: 表达式非法 / 上限超限
        '409':
          description: name 或 (type,remote_port) 冲突，事务已回滚
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ErrorBody' }

  /api/v1/system/port-probe:
    post:
      tags: [system]
      summary: 探测本机 0.0.0.0 端口可用性（仅 TCP）
      security:
        - cookieAuth: []
          csrfToken: []
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/PortProbeRequest' }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema: { $ref: '#/components/schemas/PortProbeResponse' }
        '422':
          description: 端口数量超 64 / 端口越界
```

---

## 6. 数据模型变更

**无 migration**。所有改动在应用层 / 文件系统层完成（CC-1）：

- A：仅动文件系统（`frp_linux/frpc` 等），不动 DB。
- B：仅动进程内 cache + 环境变量短路，不动 DB。
- C：批量端口在 storage 层只是事务包装，每条 Proxy 仍是独立 row（schema 不变）；端口探测不写 DB。

> 这是 `dev-db` 分区**不参与本任务**的根因。

---

## 7. 请求流（Sequence / Flow）

### 7.1 上传二进制（A）

```
浏览器                   chi router           SessionAuth/CSRF      uploadBin handler        downloader.Install      filesystem
  │  POST /upload-bin       │                      │                       │                          │                    │
  │  multipart {kind,file}  │                      │                       │                          │                    │
  ├────────────────────────►│                      │                       │                          │                    │
  │                         ├─► Recover/RequestID/Logger ─────────────────►│                          │                    │
  │                         │                      │  ✓ cookie + X-CSRF    │                          │                    │
  │                         │                      ├──────────────────────►│                          │                    │
  │                         │                      │                       │ MaxBytesReader(64+1 MiB) │                    │
  │                         │                      │                       │ MultipartReader 流式读   │                    │
  │                         │                      │                       │ 取 kind→ uploadLock.Try  │                    │
  │                         │                      │                       │ check Downloader.Status  │                    │
  │                         │                      │                       │ bufio Peek 64B → 校验头  │                    │
  │                         │                      │                       │ Install(kind, br, max) ──►│                    │
  │                         │                      │                       │                          │ CreateTemp + Copy │
  │                         │                      │                       │                          │ + sha256 tee      │
  │                         │                      │                       │                          │ Rename atomic     │
  │                         │                      │                       │                          │ chmod 0o755       │
  │                         │                      │                       │ ProcMgr.GetStatus ──────►│ 拿 advisory       │
  │ ◄───────── 200 {sha256, path, ...}             │                       │                          │                    │
```

### 7.2 批量端口（C.1）

```
浏览器              batchProxies handler         portrange.Parse        Store.UpsertProxiesTx       applyConfig
  │ POST batch         │                              │                          │                       │
  ├───────────────────►│ validate basename/type       │                          │                       │
  │                    ├─────────────────────────────►│ → []int (≤32)            │                       │
  │                    │ count existing + 200 上限    │                          │                       │
  │                    │ build []Proxy                │                          │                       │
  │                    ├────────────────────────────────────────────────────────►│ BEGIN                 │
  │                    │                              │                          │ for each: INSERT      │
  │                    │                              │                          │  ↳ any UNIQUE fail    │
  │                    │                              │                          │      → ROLLBACK + err │
  │                    │                              │                          │ COMMIT                │
  │                    │ 若 err 含 conflicts 明细     │                          │                       │
  │                    │ 否则 applyConfigBestEffort ─────────────────────────────────────────────────────►│ 1 次 frpc reload
  │ ◄─── 201 {created:12, items:[...]} ──────────────│                          │                       │
```

### 7.3 公网 IP 并发（B）

```
client → systemPublicIP → cache miss → fetchPublicIP(ctx 3s)
   ├── go ipify        ──┐
   ├── go my-ip.io     ──┤
   ├── go ip.cn        ──┼──► select { case w := <-ch: cancel() others; return w
   ├── go bilibili     ──┤                  case <-ctx.Done(): return ErrMsg }
   └── go ip.cn-html   ──┘
```

---

## 8. Reuse Audit

| 需要 | 已有 | 文件 | 决策 |
|---|---|---|---|
| 原子 rename + Windows fallback | `downloader.doDownload` 内联段 | `internal/downloader/downloader.go:229-244` | **抽出**为 `Install` 共享方法（A 模块），downloader 自己也改调 Install（纯重构） |
| Linux chmod 0o755 | 同上 | `internal/downloader/downloader.go:247-251` | 同上一并抽出 |
| 路径解析 | `resolveParams` | `internal/downloader/downloader.go:266-296` | A 复用原方法（去掉 archiveExt/entryName 返回值的需求 — handler 只读前两个） |
| GitHub UA + 状态码先判 | `resolveLatestAsset` | `internal/downloader/downloader.go:315-373` | B 沿用同款 UA `frp_easy` |
| 公网 IP 5min 缓存 | `ipCache` + `systemPublicIP` | `internal/httpapi/handlers_system.go:55-104` | B 完全保留，只改 `fetchPublicIP` 内部 |
| IPv6 advisory | `buildIPResult` | `internal/httpapi/handlers_system.go:204-210` | B 复用 |
| 运行期 `FRP_EASY_PUBLIC_IP` env 短路（Go 端） | （**无**——仅 scripts/install.sh / install.ps1 有） | — | **NEW**（B-1）：本任务在 `fetchPublicIP` 首次落地，把短路从安装期扩展到运行期 |
| ParseIP from JSON | `downloader.ParseIPFromJSON` | `internal/downloader/downloader.go:508-517` | B 复用（ipify / my-ip.io 已用） |
| 错误码 / writeError / writeJSON | `errors.go` | `internal/httpapi/errors.go` | A/C 全部复用 |
| SessionAuth / CSRF / Recover / SecurityHeaders 中间件 | `middleware.go` + `router.go` | `internal/httpapi/router.go:75-124` | 新端点挂在已有受保护分组，零中间件改动 |
| Proxy 字段校验 | `validateProxyInput` + `ValidatePort` etc. | `internal/httpapi/handlers_proxies.go` + `validate.go` | C.1 batch 复用（每条派生后仍走相同校验） |
| name UNIQUE 错误识别 | `isDuplicateNameError` + `ErrDuplicateName` | `internal/storage/proxies.go:329-336` | C.1 batch 事务复用；新加同款 `isDuplicateTcpRemote` 兄弟函数（按 T-007 insight L11 文本格式 `proxies.type, proxies.remote_port`） |
| frpc reload 触发 | `applyConfigBestEffort` | `internal/httpapi/handlers_proxies.go:145-159` | C.1 batch 在事务成功后调一次（FR-C.1.8） |
| axios 客户端 + CSRF 拦截 | `apiClient` | `web/src/api/client.ts` | A/C 全部前端 API 走它 |
| ProxyForm 已有联动逻辑 | `useProxyForm` | `web/src/composables/useProxyForm.ts` | C 在它之上加 `portMode` 状态 |
| naive-ui NTag / NInput / NSpace / NTooltip | （依赖） | `web/src/main.ts` 已注册 | A/C 复用，零新依赖 |
| 端口探测 | （无） | — | 新 helper 必要 |
| 端口表达式解析 | （无） | — | 新包 `internal/portrange/` 必要 |
| sha256 hash | stdlib `crypto/sha256` | — | A 直接用 |
| multipart 解析 | stdlib `mime/multipart` | — | A 直接用（不引入第三方） |

> **新依赖：0**。

---

## 9. 风险与缓解

| ID | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R-1 | 反代 / 浏览器 HTTP/1.1 默认体上限 < 64 MiB（如部分国产反代默认 10 MiB） | 上传 64 MiB 走反代会被截断 | 文档（`docs/DEPLOYMENT.md` 追加）显式提示反代配置 `client_max_body_size 80m`；前端 413 错误后端透传 + 前端 hint "如使用反向代理，请确认反代允许 64 MiB 上传体" |
| R-2 | ip.cn / bilibili 等大陆源反爬虫（UA / 频率限制） | 单源失败 | 已设 UA=`frp_easy`；5 min 进程缓存压低频次；任一源失败不影响整体（并发取胜出者） |
| R-3 | portsExpr 解析的边界（`""`、`"-"`、`"6000-"`、`"abc"`、范围跨度 > 32、负号、超大端口、混合空格） | 用户体验崩 / 注入风险 | 单独的 `internal/portrange/` 包 + 11 条 table-driven 单测；handler 转 422 + 字段名（FR-C.1.3） |
| R-4 | 探测竞争：先探可用 → 用户填入 → 保存间隙端口被其它进程占走 | 用户以为可用、保存后启动 frpc 失败 | UI 文案明确"探测仅供参考，启动后才是真实绑定"；FRP 启动失败已有 `lastErr` 显示通道 |
| R-5 | 批量创建中第 N 条冲突 → 事务回滚 | 必须保证 BEGIN/ROLLBACK 真的回滚（不是逐条 INSERT 后失败) | `UpsertProxiesTx` 显式 `BeginTx` + 任一 INSERT err 立即 `tx.Rollback()`；单测 `UpsertProxiesTx_RollbackOnNameDup` 在事务后断言 `ListProxies` 行数不变 |
| R-6 | 上传时同 kind 子进程正在运行（Linux 文件被持有）→ rename 行为 | Linux: rename 成功但运行进程仍用旧 inode（隐性"未生效"） | 已有 advisory 字段提示用户重启；前端在 NMessage 一并展示 |
| R-7 | 上传时 Windows 文件被运行进程锁 → rename 失败 | 上传 500 | 已有 Windows fallback (`os.Remove` 再 `Rename`)；如 Remove 也失败则 500 + 把 errno 透传给用户（"binary 被运行进程锁，请先停 frpc/frps 再上传"） |
| R-8 | port-probe 接口被滥用做端口扫描其它主机 | 安全 | 硬编码 `0.0.0.0:<port>`、不接受 host 字段；上限 64；走 SessionAuth + CSRF（必须登录用户才能调） |
| R-9 | HTML IP 抽取被广告内嵌 IP 污染（如 `192.168.1.1` 出现在页面） | 探测结果错误 | parser 强制 `IsPrivate()/IsLoopback()` 排除；仅取第一个**公网** IPv4；单测 `PublicIP_HTMLPolluted` 验证 |
| R-10 | T-018 修改 `fetchPublicIP` 把 `ipSources` 改成可注入 seam → 测试侵入 | 设计 vs 测试代码污染生产 | 采用最小侵入：`fetchPublicIP(ctx, sources []ipSource)`，包级 `defaultIPSources`；handler 调时传 `defaultIPSources`，测试调时传 mock |
| R-11 | `bufio.Reader.Peek(64)` 可能阻塞慢上游 | 上传卡顿 | peek 后立即转 io.Copy；MaxBytesReader 限流的 timeout 由 axios 端 120s 兜底；handler 不设独立 timeout（与现有写接口一致） |
| R-12 | C.1 批量在前端展开预览，但后端 portsExpr 解析与前端不一致 | UX 错配 | 后端 portrange 是权威源；前端 batch 提交时**不**展开（直接发 portsExpr），后端展开后返回 items 数组让前端渲染列表；前端只做"预览数量"提示（用 portrange 同款规则的 TS 移植，仅 UI hint，不参与提交） |
| R-13 | systemd 进程权限不足时探 80 之类特权端口的 Listen 调用 | 误判为可用（实际 EACCES 但 Go 会返 err，所以是 unavailable，OK） | < 1024 不真探（FR-C.3.3）；≥ 1024 真探时 Listen err 就是 unavailable，语义对 |
| R-14 （B-7） | cache miss 时 5 源并发探测对外部站点的出站流量 | 每次 cache miss 最多 ~600 KiB 流量（HTML 源 ip.cn 单次 256 KiB + 4 个 JSON 源各 ≤ 32 KiB）；首个成功后 `cancel()` 触发其它 in-flight 取消，但 HTML 源可能已开始读 body | **选择"保持并发 5 源"而非"分波探测"**：理由 (a) 5 min 进程内缓存已有效抑制频率（用户首屏一次 + 5min 内不再发）；(b) 分波探测（先国内 2 源、3s 后触发国际 3 源）会把首屏耗时从 ~1s 推到 ~3-4s，破坏 NF-B.1；(c) 600 KiB ≈ 单张图片大小，可接受。文档化该流量上限为已知行为 |
| R-15 （B-9） | Windows 上 `net.Listen("tcp", "0.0.0.0:N")` 仅探 IPv4 wildcard | 若被 IPv6-only 监听者占用 → 误判为 available | **修订**：改 `net.Listen("tcp", ":N")`（dual-stack wildcard），详见 §C.3.2 注释 |
| R-16 （B-10） | 反代 `client_max_body_size` 默认 1 MiB 时上传 64 MiB | 用户上传得到反代层 413（错误来源不清晰） | QA 阶段在 06_TEST_REPORT 的 Adversarial 段加 nginx docker-compose 实测；或文档化为 known limitation（见 §11.6 测试策略） |

---

## 10. 迁移 / 回滚计划（Migration / Rollout）

### 10.1 部署顺序

本任务**无 DB schema 改动 / 无配置文件改动**，单二进制发布即生效。

1. **代码合并** → CI 自动构建 → 滚动发布走 T-013 的 `rolling` tag。
2. 用户升级 = 替换 frp-easy 二进制 + systemd restart（或 Web 控制重启）。
3. **零数据迁移**：
   - 已有 frpc/frps binary 路径不变（上传只是另一种写入方式）；
   - 已有 proxies 行不变（batch 只是新建多条）；
   - 已有公网 IP 缓存被进程重启清空（自然失效）。

### 10.2 向后兼容

- `POST /api/v1/proxies` 单条接口语义不变（CC-6 / NF-C.4）。
- `GET /api/v1/system/public-ip` 响应新增 `source` 字段，旧客户端忽略。
- `POST /api/v1/system/download-bin` 不变。
- 新错误体 `conflicts` 字段为可选；旧客户端解析现有 `code/message/field` 仍工作。

### 10.3 回滚

- 二进制层面：回退到上一个滚动 tag，所有新端点 404（前端 fallback 到旧路径）。
- 数据层面：无迁移 → 无回滚步骤。
- 唯一需要关心的：若用户在新版本期间用 batch 创建了 50 条 web-* 规则，回退到旧版本后这些规则仍在 DB（旧版本读它们没问题，因为 schema 没变）；只是少了批量删除的入口（本期 out-of-scope，没影响）。

### 10.4 Feature flag

不引入 feature flag。三模块均为加性能力，旧 UI 路径仍工作。

---

## 11. Out-of-scope（设计边界）

本设计**不包含**以下能力（参 01 §A.2 / B.2 / C.2）：

- 上传压缩包解压、GPG 签名、binary 版本探测（不调 `frpc -v`）、断点续传、macOS 支持、"已上传后清除"按钮。
- IP 归属地解析、DNS-over-HTTPS、用户自定义探测源、第三方 HTTP 客户端库。
- 本地→远程端口偏移映射（`6000-6010:7000-7010`）、portsRange FRP 上游模板语法、按 IP 段批量、批量删除、UDP/IPv6 端口探测。
- 上传/创建后的"自动停启子进程"联动（保持单一职责，advisory 提示由用户手动操作）。
- IP 源胜出者在前端 UI 的可视化（响应字段已就绪，UI 展示由后续 polish）。
- `Proxies.vue` 列表批量删除按 prefix（本期仅折叠显示）。

---

## 12. Partition Assignment（fullstack 强制）

| 子模块 / 文件 | 分区 | 新 / 编辑 | 依赖 |
|---|---|---|---|
| `internal/downloader/install.go` | **dev-backend** | 新 | — |
| `internal/downloader/downloader.go` | dev-backend | 编辑（doDownload 改调 Install；零行为变更重构） | install.go |
| `internal/downloader/install_test.go` | dev-backend | 新 | install.go |
| `internal/httpapi/handlers_system.go` | dev-backend | 编辑（追加 uploadBin、portProbe；重写 fetchPublicIP） | install.go |
| `internal/httpapi/handlers_upload_test.go` | dev-backend | 新 | handlers_system.go |
| `internal/httpapi/handlers_system_publicip_test.go` | dev-backend | 新 | handlers_system.go |
| `internal/httpapi/port_probe_test.go` | dev-backend | 新 | handlers_system.go |
| `internal/portrange/portrange.go` | **dev-backend** | 新 | — |
| `internal/portrange/portrange_test.go` | dev-backend | 新 | portrange.go |
| `internal/httpapi/handlers_proxies.go` | dev-backend | 编辑（追加 batchProxies） | portrange + storage |
| `internal/httpapi/handlers_batch_test.go` | dev-backend | 新 | handlers_proxies.go |
| `internal/storage/proxies.go` | dev-backend | 编辑（追加 UpsertProxiesTx + isDuplicateTcpRemote + ErrDuplicateTcpRemote 哨兵） | — |
| `internal/storage/proxies_batch_test.go` | dev-backend | 新 | proxies.go |
| `internal/httpapi/router.go` | dev-backend | 编辑（注册 3 个新路由 + 1 个 batch 路由到受保护分组） | 上述 handler |
| `internal/httpapi/errors.go` | dev-backend | 编辑（ErrorDetail 追加可选 Conflicts 字段） | — |
| `openapi.yaml` | dev-backend | 编辑（追加 §5 全部 snippet） | — |
| `web/src/types.ts` | **dev-frontend** | 编辑（追加 UploadBinResponse / BatchProxiesRequest / BatchProxiesResponse / PortProbeResult / PublicIPResponse.source / DownloadState 无关；ErrorDetail.conflicts） | 依赖 openapi |
| `web/src/api/system.ts` | dev-frontend | 编辑（追加 apiUploadBin / apiProbePorts） | types.ts |
| `web/src/api/proxies.ts` | dev-frontend | 编辑（追加 apiBatchCreateProxies） | types.ts |
| `web/src/components/UploadBinButton.vue` | dev-frontend | 新 | apiUploadBin |
| `web/src/components/AppLayout.vue` | dev-frontend | 编辑（在 binMissing banner 现有下载按钮旁挂 UploadBinButton + tooltip） | UploadBinButton |
| ~~`web/src/pages/Wizard.vue`~~ | ~~dev-frontend~~ | **B-4 修订移除**：Wizard 当前无下载入口，本任务不挂上传 | — |
| `web/src/composables/usePortPresets.ts` | **dev-frontend** | 新 | — |
| `web/src/composables/useProxyForm.ts` | dev-frontend | 编辑（增加 portMode 状态 + batch toProxyInput 分流） | — |
| `web/src/components/ProxyForm.vue` | dev-frontend | 编辑（portMode 切换 + 预设 Tag + 探测按钮） | usePortPresets / apiProbePorts / useProxyForm |
| `web/src/pages/Proxies.vue` | dev-frontend | 编辑（折叠分组显示 + 行级远程端口预检按钮 + batch 提交分流） | ProxyForm / apiBatchCreateProxies / apiProbePorts |
| `web/src/components/__tests__/ProxyForm.test.ts` | dev-frontend | 编辑（增加 portMode / 预设 / 探测测试） | ProxyForm.vue |

**dev-db 不参与本任务**（无 migration / schema 不变 / CC-1）。

### 12.1 Dispatch Order（派发顺序）

1. **dev-backend**（一次性派发所有 Go 改动；handler / storage / install / portrange / openapi / router）。
2. **dev-frontend**（依赖 dev-backend 完成后；前端消费已落地的 API + types）。

### 12.2 Parallelism

- dev-backend 内部并行度高：`install.go`、`portrange.go`、`storage/proxies.go` 三块互不依赖，dev-backend agent 可在单次会话内顺序完成。
- dev-frontend 必须**等待 dev-backend 完成**（API/types 是契约源）。
- 严格 serial：dev-backend → dev-frontend。

---

## 13. 与 insight-index.md 的契合

| insight 条目 | 在本设计的体现 |
|---|---|
| L10 Windows os.Rename 不能覆盖；先 Remove 再 Rename | A 的 `Install` 直接沿用 `doDownload` 现有代码段 |
| L17 AtomicWrite 双重 Chmod | A 的 Install 在 rename 后调 `os.Chmod(targetPath, 0o755)` 而非依赖 tmp 权限 |
| L16 modernc sqlite UNIQUE 文本格式 | C.1 `isDuplicateTcpRemote`：**关键字以单测断言为准**（B-8 修订）。复合 UNIQUE 报文文本在本项目未实证过，Developer 必须先用 adversarial 单测捕获真实 `err.Error()` 再写 `strings.Contains`，不直接照搬单列模板 |
| L28 Go runtime.GOOS 可注入 seam | A 的 Install 跨平台单测沿用现有 `goosFunc` seam（Windows fallback 在 Linux runner 可测） |
| L31 verify_all E.6 `## Adversarial tests` 英文标题 | QA 阶段提示（本设计无关，但 PM 派 QA 时不要踩） |
| L37 GitHub API 必须带 UA；先判状态码后解析 JSON | B 的所有出站 HTTP 都带 `User-Agent: frp_easy`；状态码非 200 直接 fail（不解析 body） |
| L40 国内 VM 公网 IP 三国际源易全失败；保留 env override | B 严格保留 `FRP_EASY_PUBLIC_IP` 短路；扩源不删旧源；失败横幅文案不动（CC-6） |
| L26 `npm exec -- <pkg>` 双 dash 透传 | 不直接相关（本任务不引入新 npm 工具调用） |
| L24 PowerShell BOM 写 TOML | 不相关（本任务不写 TOML） |

---

## 14. Verdict

**READY (v2，修订后重提)** — 可进入 Stage 3（Gate Review 二次）。

- 需求文档 verdict = READY（10 个 [PM-DECIDED] 完整覆盖；FR-A.1 / AC-A.9 / FR-C.1.6 已同步收紧）。
- 设计文档 v2：吸收首轮 GR 全部 P0/P1 + 全部 P2（B-1 ~ B-12）+ Q1 / Q2；新加 R-14 / R-15 / R-16 三条风险与缓解。
- 无 BLOCKED，无新依赖，无 DB migration，dev-db 不参与（已说明）。
- 所有大小 / 上限 / 超时数字均带依据（64 MiB ← FRP 实测 20 MiB × 3；32 端口、64 端口、200ms、3s ← 01 PM-DECIDED）。
- 复用审计非空，引用了 12+ 处现有代码路径，并明确标 `FRP_EASY_PUBLIC_IP` 运行期短路是 NEW。

---

## 修订记录 / Revision Log

### v2 (2026-05-23, 修订 P0/P1/P2 + Q1/Q2)

依据 `03_GATE_REVIEW.md`（首轮 GR，CHANGES REQUIRED）逐条吸收：

**P0**
- **B-1**：§3.B.1 `fetchPublicIP` 顶部注释明确 "**NEW**：Go 端首次引入 `FRP_EASY_PUBLIC_IP` 读取（之前仅 install.sh / install.ps1）；本任务把短路从安装期扩展到运行期"。§8 复用 audit 表新增一行"运行期 `FRP_EASY_PUBLIC_IP` env 短路（Go 端）"标 **NEW**。
- **B-2**：§A.3 `apiUploadBin` 删除 `headers: { 'Content-Type': 'multipart/form-data' }`，并加注释说明"留给 axios 自动加 boundary；显式写会丢 boundary 致服务端 400"。
- **B-3**：§A.2 步骤 7 改为 `info := h.deps.ProcMgr.Status(kind)` + `info.State == procmgr.StateRunning`（单返回值 + 常量比对），匹配真实 procmgr 签名。

**P1**
- **B-4**：§A.3 挂载点段落收紧为"**仅** AppLayout.vue banner 下载按钮组并列"，显式声明"Wizard.vue 当前无下载入口，本任务不新增"。§12 Partition 表中 Wizard.vue 编辑项标删除线 + 标注"B-4 修订移除"。同步 01 §FR-A.1 / AC-A.9。
- **B-5**：§C.1.3 增"并发约束"小节，明确 `UpsertProxiesTx` **整段持 `s.mu.Lock()`**（`BeginTx` 前 Lock、`Commit/Rollback` 后 Unlock），与 `UpsertProxy` 同串行化；附伪代码骨架。§C.1.5 storage 单测表追加 `UpsertProxiesTx_ConcurrentWithUpsertProxy` 并发用例（断言无 `database is locked`）。
- **B-6**：§A.2 `uploadBin` handler 重写：放弃 `MultipartReader` 流式 + 字段顺序假设，改 `ParseMultipartForm(8 MiB) + r.FormValue("kind") + r.FormFile("file")` 模式（multipart 库自动磁盘 spill 不爆内存）；`http.MaxBytesReader(w, r.Body, maxBodyBytes)` 仍在 `ParseMultipartForm` **之前**裹一层，并显式区分 `*http.MaxBytesError`（→ 413）与"非 multipart"（→ 400）。

**P2**
- **B-7**：§9 风险表新增 R-14，**选择"保持并发 5 源"而非"分波探测"**，理由三条（5min 缓存压频、分波破坏 NF-B.1 首屏 ~1s、600 KiB 可接受）。文档化流量上限为已知行为。
- **B-8**：§C.1.3 `isDuplicateTcpRemote` 段加显式约束 "Developer 必须先写 storage adversarial 单测捕获 modernc 复合 UNIQUE 真实 `err.Error()`、再据此写 `strings.Contains` 关键字；不直接照搬单列模板"。§13 insight 表 L16 状态由 △ 改为明确说明。
- **B-9**：§C.3.2 `probeOnePort` 改 `net.Listen("tcp", fmt.Sprintf(":%d", port))`（dual-stack wildcard，IPv4 + IPv6），并加注释解释 Windows 上 `0.0.0.0:N` 为 IPv4-only 的隐患。§C.3.2 "安全"小段同步措辞为"硬编码本机 wildcard 监听"。§9 新增 R-15。
- **B-10**：§A.4 测试点表追加 Adversarial 条目 `nginx_client_max_body_size`（docker-compose nginx 反代 1 MiB 上限上传 25 MiB），QA 阶段落到 `06_TEST_REPORT.md` 的 `## Adversarial tests` 段；若环境不便起 nginx 则文档化为 known limitation。§9 新增 R-16。
- **B-11**：§A.2 `validateBinaryHeader` 删除"PE\0\0 偏移 0x3C 校验"段（原本即注释为"不强校验"，文字自相矛盾），统一为"仅 MZ 即接受 PE；落盘后启动失败由 procmgr `lastErr` 暴露"，并在函数注释里写清这一设计选择。
- **B-12**：§C.1.4 列表折叠正则改为 `^(.+)-(\d{1,5})$`（greedy + 限定 1-5 位数字尾），举例 `my-web-6000` 正确切为 basename=`my-web`/port=`6000`；前端单测必须覆盖 `web-6000` / `my-web-6000` / `a-b-c-22` / `web-notaport` / `abc` 五种用例。
- **B-13**：无需补（HTML 源仅抓 IPv4 已在 R-9 / `parseFirstIPv4FromHTML` 实现；`errgroup` 未引入已在 CC-2 + §B.1 sync.WaitGroup 实现确认）。

**Q（PM 提问 / 设计内一致性）**
- **Q1**：§A.2 `Install` 函数注释加入 "`maxBytes <= 0` 表示不限大小（下载链路走此分支，因 archive 解压后 binary 大小不可预知）；upload 链路必须传 > 0 的明确上限"；同时给出两路径各自传入值（download = -1、upload = 64 MiB）。
- **Q2**：02 §5.1 OpenAPI `BatchProxiesResponse` 本就是对象版 `{ created, items }`（与 §C.1.1 响应 201 例子一致）；同步把 01 §FR-C.1.6 "响应 201 + `[ProxyResponse...]`"改写为"响应 201 + `{created, items}`（对象，含创建条数 `created` 与新建条目数组 `items`）"。无需走 04 DESIGN-DRIFT。

**01 文档同步修订**（仅 3 处文案，结构不变）
- 01 §FR-A.1：把"Wizard 与设置页两处"改为"AppLayout 顶部 banner（与现有'一键下载'按钮组并列）"，并加一句"Wizard.vue 当前无下载入口，本任务不在 Wizard 新增"。
- 01 §AC-A.9：同步从"Wizard 与设置页"改为"AppLayout 顶部 banner"。
- 01 §FR-C.1.6：响应 201 文案从 `[ProxyResponse...]` 改为 `{created, items}` 对象版。

**未变更的设计决策**（明示）
- §6 数据模型变更：仍为"无 migration"。
- §8 复用 audit：除新增 NEW 行外，其余 14 条复用项不变。
- §12 Partition Order：dev-backend → dev-frontend，严格 serial。
- §10 回滚策略不变。
