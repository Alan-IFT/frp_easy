# 02 · 解决方案设计 — T-014 · frp-binary-auto-download

> 模式：`full` · 编写：solution-architect · 日期：2026-05-22 · PM 自治模式
> 上游（只读）：`docs/features/frp-binary-auto-download/01_REQUIREMENT_ANALYSIS.md`（verdict=READY）、`INPUT.md`
> 设计依据：T-002 `docs/features/_archived/zero-config-quickstart/02_SOLUTION_DESIGN.md` §3.1（downloader 原始设计）
> 决策原则：① 用户体验好 > ② 符合软件工程标准 > ③ 长期易使用易维护

---

## 1. 架构概述（Architecture summary）

本任务把 frp 二进制（frpc/frps）从「git 仓库内置 + 随发布包分发」改为「运行时按需从 fatedier/frp 官方 Release 下载**最新版**」。系统级变化集中在三层：

- **数据/资产层**：`git rm` 四个 frp 可执行文件 + 两份上游 `frpc.toml`/`frps.toml` 样例；`.gitignore` 忽略下载落地产物；`frp_linux/`、`frp_win/` 目录用各自保留的上游 `LICENSE` 作为占位锚点（OQ-1 候选 B）。
- **Go 后端层（`internal/downloader`）**：下载 URL 不再用写死的 `FRPVersion` 常量构造，改为先调 GitHub Release API（`/repos/fatedier/frp/releases/latest`）解析最新 tag，再据 tag + 平台/架构匹配资产。新增一个**前置解析步骤** `resolveLatestVersion()`，对 API 限流(403)/网络失败/资产未匹配三类失败降级为既有 `failed` 状态 + 中文错误消息，不引第三方库。downloader 既有的异步 goroutine、进度追踪、原子安装、Zip Slip 防御全部保留。
- **脚本/文档层**：`package.sh` 移除 frp 二进制打包逻辑与前置检查；`install.sh`/`install.ps1` 修复升级路径不再 `rm -rf frp_linux/`（OQ-4），并在成功 banner 新增「更新」小节；`NOTICE`/README/DEPLOYMENT.md/dev-map.md 同步改写。

**首启策略（OQ-2）裁定**：采用**候选 A（不自动触发下载）**，沿用 T-002 横幅点击触发。理由见 §3.2。启动流程不被下载阻塞、离线可正常启动这两条硬约束在现有架构下天然满足（downloader 仅在 HTTP handler 收到 `POST /download-bin` 时才 `Start`），无需新增任何启动期逻辑。

---

## 2. 受影响模块（Affected modules）

| 文件 | 类型 | 改动摘要 |
|---|---|---|
| `frp_linux/frpc`、`frp_linux/frps` | 删 | `git rm` |
| `frp_win/frpc.exe`、`frp_win/frps.exe` | 删 | `git rm` |
| `frp_linux/frpc.toml`、`frp_linux/frps.toml` | 删 | `git rm`（OQ-1 候选 B） |
| `frp_win/frpc.toml`、`frp_win/frps.toml` | 删 | `git rm`（OQ-1 候选 B） |
| `frp_linux/LICENSE`、`frp_win/LICENSE` | 保留 | 不动，作为目录占位锚点 |
| `.gitignore` | 改 | 新增忽略 frp 下载产物规则 |
| `internal/downloader/downloader.go` | 改 | 移除 `FRPVersion`，新增 latest 解析步骤 + 资产匹配 + 降级 |
| `internal/downloader/downloader_test.go` | 改 | 测试随 `resolveParams` 签名变化调整 + 新增 latest 解析 4 分支测试 |
| `scripts/package.sh` | 改 | 移除 frp 二进制前置检查（L115-131）与打包（L231-241 区） |
| `scripts/install.sh` | 改 | 升级路径不删 `frp_linux/`（L228-231）；成功 banner 增「更新」段 |
| `scripts/install.ps1` | 改 | 升级路径不删 `frp_win\`（L183-190）；成功 banner 增「更新」段 |
| `NOTICE` | 改 | 改写「随附」→「运行时下载」 |
| `README.md` | 改 | §许可证注意段 + 亮点段 + 目录树注释 + 新增「如何更新」 |
| `docs/DEPLOYMENT.md` | 改 | F.5 节措辞 + A.0 一键安装小节增「更新」 |
| `docs/dev-map.md` | 改 | `frp_linux/`/`frp_win/` 目录描述更新 |

**不改**（红线）：`.harness/`、`.claude/`、`CLAUDE.md`、`.github/copilot-instructions.md`、`docs/features/_archived/`。

**确认不受影响**：`internal/binloc/binloc.go`（语义不变，见 §3.5）、`cmd/frp-easy/main.go`（启动序列不变，见 §3.2）、`internal/httpapi/handlers_system.go`（download-bin/download-status handler 不变）、`internal/httpapi/router.go`。

---

## 3. 详细设计

### 3.1 OQ-1 落地：git 资产清理 + 目录占位

**裁定：采纳候选 B。**

- **删除（`git rm`）**，确切 6 个文件：
  - `frp_linux/frpc`、`frp_linux/frps`、`frp_linux/frpc.toml`、`frp_linux/frps.toml`
  - `frp_win/frpc.exe`、`frp_win/frps.exe`、`frp_win/frpc.toml`、`frp_win/frps.toml`
  - 共 8 个文件（4 个可执行 + 4 个 toml 样例）。
- **保留（不动）**：`frp_linux/LICENSE`、`frp_win/LICENSE` —— 这两个上游 Apache-2.0 许可证文本既满足归属义务，又**天然充当目录占位**：`git ls-files frp_linux/` / `frp_win/` 各仍列出 1 个被跟踪文件，满足 **AC-2**，无需额外 `.gitkeep`。
- **`frpc.toml`/`frps.toml` 删除理由**：frp_easy 由 `internal/frpconf` 渲染 runtime TOML（写到 `<dataDir>/runtime/frpc.toml`），上游样例从不被读取、只会与 frp_easy 自己的配置混淆（dev-map.md「要避免的模式」已明示「不要把 frp_easy.toml 和 frpc.toml/frps.toml 搞混」）。

**`.gitignore` 新增规则**（追加到现有 `# frp_easy runtime data` 段之后）：

```gitignore
# frp 二进制运行时下载落地产物（T-014：不再内置，运行时从 fatedier/frp 下载）
# frp_linux/ 与 frp_win/ 目录保留作下载落地目标，仅 LICENSE 被跟踪
/frp_linux/frpc
/frp_linux/frps
/frp_win/frpc.exe
/frp_win/frps.exe
# downloader 的临时文件（os.CreateTemp 前缀，见 downloader.go）
/frp_linux/.dl-*.tmp
/frp_win/.dl-*.tmp
```

> 注意：用根锚定（前导 `/`）精确忽略这 4 个文件名，不写 `frp_linux/*` 通配，以免误伤 `LICENSE`。`.dl-*.tmp` 规则覆盖下载中断残留的临时文件（虽然 downloader 正常路径会 `os.Remove`，但崩溃场景可能残留，忽略它们避免污染 `git status`）。

### 3.2 OQ-2 落地：首启与下载触发策略

**裁定：采纳候选 A —— 不在首启自动触发下载，沿用 T-002 横幅点击触发。**

决策依据（原则排序 ① UX > ② 工程标准 > ③ 易维护）：

1. **硬约束天然满足、零启动期风险**：现有 `main.go` 启动序列（L182-263）中，`downloader.New(loc.Root(), logger)` 只构造 Manager，**从不**在启动期调 `Start()`。下载唯一入口是 HTTP handler `downloadBin`（`handlers_system.go` L107）。因此「启动不被下载阻塞」「离线可正常启动」在候选 A 下是**结构性保证**，无需任何新代码、无新增 race/panic 面。候选 B 需在启动序列注入后台 goroutine，引入新失败面，违反「downloader 尽量小改面」的任务约束。
2. **UX 不打折**：T-002 已实现的横幅（`AppLayout.vue` 下载按钮 + `stores/downloader.ts` 1s 轮询进度条）在 `binloc.Missing()` 非空时**自动出现**，用户打开 UI 即见、一键即下，无「需要自己去找」的发现成本。候选 B 的「免点」收益边际很小，却要承担「用户没看 UI 时静默消耗带宽」「下载失败无人看到横幅」等问题。
3. **与 B 块「可发现性」一致**：本任务 B 块的核心诉求是「让更新方式可被发现」，而非「全自动」。横幅本身就是可发现性载体；O-6 也明确「首启自动触发的实现」不在本期。

**结论**：`main.go`、`internal/httpapi/*`、前端 store/组件**全部不改**。下载触发流程保持 T-002 现状：

```
浏览器打开 UI
  → GET /api/v1/system/ready 返回 binMissing:["frpc","frps"]（loc.Missing()）
  → 前端 stores/app.ts 检测 binMissing 非空
  → AppLayout.vue 渲染顶部下载横幅
  → 用户点「下载」→ POST /api/v1/system/download-bin {kind}
  → handlers_system.downloadBin → Downloader.Start(kind) → 异步 goroutine
  → 前端 stores/downloader.ts 每 1s GET /download-status/{kind} 刷新进度条
  → 下载+解压+原子安装完成 → 横幅消失（下次 Missing() 为空）
```

### 3.3 OQ-4 落地：install 脚本升级路径修复

**裁定：采纳候选 A —— 升级保留用户运行时已下载的 frp 二进制。**

#### 3.3.1 问题根因（关键正确性点）

T-014 之后发布包**不再包含** `frp_linux/`、`frp_win/` 目录。但 `install.sh` 现有升级逻辑（L228-231）：

```bash
if [[ -d "$EXTRACTED/frp_linux" ]]; then
    rm -rf "$INSTALL_DIR/frp_linux"      # ← 危险：清掉用户已下载的 frpc/frps
    cp -a "$EXTRACTED/frp_linux" "$INSTALL_DIR/"
fi
```

RA 的 R-3 推测「`[[ -d "$EXTRACTED/frp_linux" ]]` 守卫为假 → 整块跳过 → 不会误删」。**这个推测在当前发布包结构下成立**（发布包无 `frp_linux/`，守卫为假）。但它依赖一个**脆弱的隐式契约**：「发布包恰好不含该目录」。一旦未来 `package.sh` 因任何原因（回归、误改、调试）又把空 `frp_linux/` 目录打进包，守卫立刻变真，`rm -rf "$INSTALL_DIR/frp_linux"` 就会清掉用户下载的二进制。**正确做法是显式删除这段逻辑**，让「保留 frp 二进制」成为脚本的**显式契约**而非隐式副作用。install.ps1 的 `frp_win` 同理（L183-190 的 `foreach ($sub in @("frp_win","scripts"))`）。

#### 3.3.2 install.sh 修复（升级分支，当前 L226-241）

升级分支改为**显式白名单**，`frp_linux/` 从白名单中**移除**（即升级时完全不触碰它）：

```bash
# 白名单逐项覆盖：绝不触碰 frp_easy.toml、.frp_easy/，以及用户运行时下载的 frp_linux/。
cp -a "$EXTRACTED/frp-easy" "$INSTALL_DIR/frp-easy"
# T-014：升级不再覆盖/删除 frp_linux/ —— 发布包已不含 frp 二进制，
# frp_linux/ 下的 frpc/frps 由用户经 UI 横幅按需下载，升级须原样保留。
if [[ -d "$EXTRACTED/scripts" ]]; then
    rm -rf "$INSTALL_DIR/scripts"
    cp -a "$EXTRACTED/scripts" "$INSTALL_DIR/"
fi
for f in README.txt VERSION LICENSE frp_easy.toml.example; do
    if [[ -e "$EXTRACTED/$f" ]]; then
        cp -a "$EXTRACTED/$f" "$INSTALL_DIR/$f"
    fi
done
chmod 0755 "$INSTALL_DIR/frp-easy" 2>/dev/null || true
```

**确切改动**：删除 L228-231 整个 `if [[ -d "$EXTRACTED/frp_linux" ]]` 块；把 L226 注释中的「绝不触碰 frp_easy.toml 与 .frp_easy/」补成包含「以及 frp_linux/」；新增一行说明注释。

> **升级覆盖清单（升级后语义）**：
> - **覆盖**：`frp-easy`（主二进制）、`scripts/`、`README.txt`、`VERSION`、`LICENSE`、`frp_easy.toml.example`
> - **保留（绝不触碰）**：`frp_easy.toml`（用户配置）、`.frp_easy/`（数据/日志）、**`frp_linux/`（用户下载的 frpc/frps）**

#### 3.3.3 install.ps1 修复（升级分支，当前 L181-196）

`foreach ($sub in @("frp_win", "scripts"))` 改为只含 `scripts`：

```powershell
# 白名单逐项覆盖：绝不触碰 frp_easy.toml、.frp_easy\，以及用户运行时下载的 frp_win\。
Copy-Item -Force (Join-Path $extracted.FullName "frp-easy.exe") (Join-Path $InstallDir "frp-easy.exe")
# T-014：升级不再覆盖/删除 frp_win\ —— 发布包已不含 frp 二进制，
# frp_win\ 下的 frpc.exe/frps.exe 由用户经 UI 横幅按需下载，升级须原样保留。
foreach ($sub in @("scripts")) {
    $src = Join-Path $extracted.FullName $sub
    if (Test-Path $src) {
        $dst = Join-Path $InstallDir $sub
        if (Test-Path $dst) { Remove-Item -Recurse -Force $dst }
        Copy-Item -Recurse -Force $src $dst
    }
}
foreach ($f in @("README.txt", "VERSION", "LICENSE", "frp_easy.toml.example")) { ... }  # 不变
```

**确切改动**：把 L183 的 `@("frp_win", "scripts")` 改为 `@("scripts")`；L181 注释补「以及 frp_win\」；新增说明注释。

> 全新安装分支（install.sh L242-247 / install.ps1 L197-203）**不改**：全新安装时 `INSTALL_DIR` 下本来就没有 frp 二进制，`cp -a "$EXTRACTED/."` / `Copy-Item *` 把发布包内容整体拷过去即可（发布包无 frp_linux/ 也无妨）。

### 3.4 downloader 改造：下载 fatedier/frp 最新版

#### 3.4.1 改造目标与约束

- 移除 `FRPVersion = "0.68.1"` 对下载 URL 的硬约束（AC-5）。
- 下载前经 GitHub Release API 解析 latest tag，据此构造资产 URL（FR-5）。
- 仅用 Go 标准库（`net/http` + `encoding/json` 已 import），**不引第三方**。
- 保持异步 goroutine + 进度追踪 + 原子安装 + Zip Slip 防御不变（小改面）。
- 失败降级：API 限流(403)、网络失败、资产未匹配三分支，各产中文错误，置 `failed`，不 panic（FR-6）。

#### 3.4.2 frp 上游 Release 资产命名规则（已核对）

fatedier/frp 的 GitHub Release（如 tag `v0.68.1`）资产命名为：

| 平台 | 资产文件名 | 归档格式 | 内部条目 |
|---|---|---|---|
| linux amd64 | `frp_<X.Y.Z>_linux_amd64.tar.gz` | `.tar.gz` | `frp_<X.Y.Z>_linux_amd64/frpc`、`.../frps` |
| windows amd64 | `frp_<X.Y.Z>_windows_amd64.zip` | `.zip` | `frp_<X.Y.Z>_windows_amd64/frpc.exe`、`.../frps.exe` |

关键点（与 frp_easy 自己的 `frp-easy-<version>-<os>-amd64.<ext>` 命名**不同**，勿混）：

1. frp 的 tag 形如 `v0.68.1`（带前导 `v`）；资产文件名里的版本号**不带** `v`（`0.68.1`）。解析逻辑须 `strings.TrimPrefix(tag, "v")` 得到纯版本号。
2. 归档内二进制位于一级子目录 `frp_<ver>_<os>_amd64/` 下。现有 `extractFromTarGz`/`extractFromZip` 用 `filepath.Base(entry) == entryName` 匹配（只看 basename，不管前缀目录），**已天然兼容**该子目录结构 —— `entryName` 仍是 `frpc`/`frps`/`frpc.exe`/`frps.exe`，无需改解压函数。
3. 资产匹配**不应**硬拼文件名，而应在 API 响应的 `assets[]` 数组里按**后缀**匹配（`_linux_amd64.tar.gz` / `_windows_amd64.zip` 结尾），这样对资产命名的小变动更鲁棒，且能精确实现「资产未匹配 → failed」分支。

#### 3.4.3 GitHub Release API 调用

- **端点**：`GET https://api.github.com/repos/fatedier/frp/releases/latest`
- **响应**（只取需要的字段）：
  ```json
  { "tag_name": "v0.68.1",
    "assets": [ { "name": "frp_0.68.1_linux_amd64.tar.gz",
                  "browser_download_url": "https://github.com/.../frp_0.68.1_linux_amd64.tar.gz" }, ... ] }
  ```
- **请求头**：设 `Accept: application/vnd.github+json`、`User-Agent: frp_easy`（GitHub API 要求 UA，缺失会 403）。
- **限流处理**：未认证请求 60 次/时/IP，限流时返回 **HTTP 403** 且响应体是合法 JSON（insight-index 已记录此事实，install.sh 同款处理）。**必须先判 HTTP 状态码、再决定是否解析 JSON**：
  - `403` → 中文错误「GitHub API 触发限流（未认证请求 60 次/小时/IP），请稍后重试，或按文档手动下载 frp 二进制」
  - 网络层错误（`client.Do` 返回 err）→「无法访问 GitHub（请检查网络或代理）」
  - 其它非 200 → 「查询 frp 最新版本失败：HTTP <code>」
- **不能用 `-f` 等价语义**：`http.Client` 本身不会因 4xx 返回 err（与 curl 不同），天然满足「先拿到状态码」。

#### 3.4.4 代码改动（`internal/downloader/downloader.go`）

**(a) 移除 `FRPVersion` 常量**（L26-28）。新增一个 API base 注入 seam（与既有 `baseURL`/`goos` 同款，供测试用）：

```go
// Manager 结构体新增字段（紧邻既有 baseURL、goos）：
apiBaseURL string // 空 = 使用 https://api.github.com；测试注入 httptest server
```

**(b) 新增 release API 响应结构体 + latest 解析函数**：

```go
// ghRelease 是 GitHub Release API 响应的最小子集。
type ghRelease struct {
    TagName string `json:"tag_name"`
    Assets  []struct {
        Name        string `json:"name"`
        DownloadURL string `json:"browser_download_url"`
    } `json:"assets"`
}

// resolveLatestAsset 查询 fatedier/frp 最新 release，返回匹配 goos 的资产下载 URL 与版本号。
// 失败时返回的 error 已是面向用户的中文消息（直接进 setFailed）。
func (m *Manager) resolveLatestAsset(goos string) (downloadURL, version string, err error) {
    apiBase := m.apiBaseURL
    if apiBase == "" {
        apiBase = "https://api.github.com"
    }
    url := apiBase + "/repos/fatedier/frp/releases/latest"

    req, _ := http.NewRequest(http.MethodGet, url, nil)
    req.Header.Set("Accept", "application/vnd.github+json")
    req.Header.Set("User-Agent", "frp_easy")

    resp, err := m.client.Do(req)
    if err != nil {
        return "", "", fmt.Errorf("无法访问 GitHub（请检查网络或代理）: %v", err)
    }
    defer resp.Body.Close()

    // 先判状态码、后解析 JSON（限流 403 响应体也是合法 JSON）。
    switch resp.StatusCode {
    case http.StatusOK:
        // 继续
    case http.StatusForbidden:
        return "", "", fmt.Errorf("GitHub API 触发限流（未认证请求 60 次/小时/IP），请稍后重试，或按文档手动下载 frp 二进制")
    default:
        return "", "", fmt.Errorf("查询 frp 最新版本失败：HTTP %d", resp.StatusCode)
    }

    body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB 上限，防御超大响应
    if err != nil {
        return "", "", fmt.Errorf("读取 GitHub 响应失败: %v", err)
    }
    var rel ghRelease
    if err := json.Unmarshal(body, &rel); err != nil {
        return "", "", fmt.Errorf("解析 GitHub 响应失败: %v", err)
    }
    if rel.TagName == "" {
        return "", "", fmt.Errorf("GitHub 响应缺少版本号字段")
    }

    // 按平台后缀匹配资产。
    var suffix string
    switch goos {
    case "linux":
        suffix = "_linux_amd64.tar.gz"
    case "windows":
        suffix = "_windows_amd64.zip"
    default:
        return "", "", ErrUnsupportedOS
    }
    for _, a := range rel.Assets {
        if strings.HasSuffix(a.Name, suffix) && a.DownloadURL != "" {
            return a.DownloadURL, rel.TagName, nil
        }
    }
    return "", "", fmt.Errorf("未找到匹配当前平台的 frp 资产（%s），请按文档手动下载", suffix)
}
```

> **HTTPS 校验（NF-S1）**：`apiBaseURL` 为空时默认 `https://api.github.com`；`browser_download_url` 由 GitHub 返回，恒为 `https://github.com/...`。无需额外断言（与 T-002 一致：信任 GitHub 域）。测试注入 `httptest` 时是 `http://127.0.0.1`，属测试 seam，合理。

**(c) 改造 `resolveParams`**：去掉 URL 构造（URL 现在来自 `resolveLatestAsset`），只保留路径/格式/条目名映射。新签名：

```go
// resolveParams 计算路径与格式参数（不含 downloadURL — 后者由 resolveLatestAsset 提供）。
func (m *Manager) resolveParams(kind, goos string) (targetDir, targetPath, archiveExt, entryName string, err error)
```

linux 分支：`targetDir=<root>/frp_linux`、`archiveExt=".tar.gz"`、`entryName="frpc"|"frps"`、`targetPath=targetDir/entryName`。
windows 分支：`targetDir=<root>/frp_win`、`archiveExt=".zip"`、`entryName="frpc.exe"|"frps.exe"`、`targetPath=targetDir/entryName`。
default：`err = ErrUnsupportedOS`。

**(d) 改造 `doDownload`**（L122-130 区）：在 `resolveParams` 之后、`MkdirAll` 之前插入 latest 解析：

```go
func (m *Manager) doDownload(kind, goos string) {
    startTime := time.Now()

    targetDir, targetPath, archiveExt, entryName, err := m.resolveParams(kind, goos)
    if err != nil {
        m.setFailed(kind, err.Error())
        return
    }

    // T-014：解析 fatedier/frp 最新 release 资产 URL。
    downloadURL, version, err := m.resolveLatestAsset(goos)
    if err != nil {
        m.setFailed(kind, err.Error())  // err 已是中文消息
        return
    }
    m.logger.Info("resolved latest frp release",
        "kind", kind, "goos", goos, "version", version, "url", downloadURL)

    // ... 之后逻辑（MkdirAll / 下载 / 解压 / 原子 rename）完全不变 ...
}
```

下载循环、`progressWriter`、`extractFromTarGz`/`extractFromZip`、原子 rename（含 Windows Remove-then-Rename 兜底）、`setProgress`/`setFailed`、`ParseIPFromJSON` **全部不动**。FR-7（中断清理）、FR-8（覆盖式更新原子语义）由既有逻辑保证。

#### 3.4.5 失败/降级分支汇总（FR-6 / 边界 4.1）

| 场景 | 触发点 | `Status(kind)` 返回 |
|---|---|---|
| API 网络不可达/超时 | `client.Do` 返回 err | `failed` · "无法访问 GitHub（请检查网络或代理）: ..." |
| API 限流 403 | `resp.StatusCode == 403` | `failed` · "GitHub API 触发限流..." |
| API 其它非 200 | `default` 分支 | `failed` · "查询 frp 最新版本失败：HTTP <code>" |
| 响应非法 JSON / 缺 tag | `json.Unmarshal` 失败 / `TagName==""` | `failed` · "解析 GitHub 响应失败..." / "缺少版本号字段" |
| 资产命名变更未匹配 | `assets[]` 循环无命中 | `failed` · "未找到匹配当前平台的 frp 资产（<suffix>），请按文档手动下载" |
| 下载中断/HTTP 非2xx/解压失败 | 既有逻辑（L161-219） | `failed` · 既有中文消息（旧二进制不受影响） |
| 不支持平台 | `Start` 早判 `ErrUnsupportedOS` | `Start` 返回 err，不进 goroutine |

### 3.5 binloc 与启动逻辑（FR-9，确认不变）

`internal/binloc/binloc.go` **不改**：它按 `runtime.GOOS` 在 `frp_linux/`/`frp_win/` 用 `os.Stat` 探测 frpc/frps，缺失返回 `ErrBinMissing`，`Missing()` 返回缺失 kind 列表。移除内置二进制后，**开发环境**下 `Missing()` 会从「空」变为「["frpc","frps"]」——这正是期望行为（触发横幅）。

`cmd/frp-easy/main.go` **不改**：`autoRestoreProcs`（L389）已对 `loc.Missing()` 中的 kind「记 warn 不报错」（L388 注释「二进制缺失则记 warn 不报错」）。`main.go` L184 已把 `loc.Missing()` 写进结构化日志。启动序列对二进制缺失零阻塞，离线启动 UI 可访问 —— 硬约束满足。

### 3.6 package.sh 改造

**(a) 移除 frp 子二进制前置检查**（确切删除 L115-131 整块）：

```bash
# 校验 frp 子二进制完整性
if $DO_LINUX; then
    for f in frp_linux/frpc frp_linux/frps; do ... exit 1 ... done
fi
if $DO_WINDOWS; then
    for f in frp_win/frpc.exe frp_win/frps.exe; do ... exit 1 ... done
fi
```

整段删除。删除后 package.sh 在仓库无 frp 二进制时不再 `exit 1`（满足 FR-4 / AC-4）。

**(b) 移除 frp 二进制打包**（`build_package` 内）：

- linux 分支删除 L231-233 三行：
  ```bash
  mkdir -p "$top/frp_linux"
  cp -a "$ROOT/frp_linux/." "$top/frp_linux/"
  chmod 0755 "$top/frp_linux/frpc" "$top/frp_linux/frps"
  ```
- windows 分支删除 L240-241 两行：
  ```bash
  mkdir -p "$top/frp_win"
  cp -a "$ROOT/frp_win/." "$top/frp_win/"
  ```

> 删除后发布包 staging 不再含 `frp_linux/`、`frp_win/`（满足 FR-3 / AC-3）。注意 `cp -a "$ROOT/frp_linux/."` 会连带把 `LICENSE` 拷进发布包 —— 删除该行后发布包不再含 frp 的 LICENSE，但发布包仍含**仓库根 LICENSE**（frp_easy 自身 MIT，L251-255 逻辑不变），合规无损。

**(c) 文件头注释更新**：L13 退出码说明把「frp_linux/frpc / frp_linux/frps 等」前置缺失从 `exit 1` 描述里去掉，改为「1 前置缺失（bin/frp-easy）」。

**(d) OQ-3 文件数断言阈值**：移除 frp 二进制后，linux staging 仍含 `frp-easy`、`README.txt`、`VERSION`、`LICENSE`、`frp_easy.toml.example`、`scripts/install-service.sh`、`scripts/uninstall-service.sh` = **7 个文件** ≥ 6。**结论：阈值 `< 6` 不变**（OQ-3 候选 A）。Developer 须在实际跑 `package.sh` 后用 `find "$top" -type f | wc -l` 核对确为 7；若实测 < 6 才按候选 B 下调。设计方向：**阈值保持 6，不动**。

### 3.7 NOTICE 改写（FR-10）

全文改写「关于随附的第三方二进制」段，去掉「随附」「开箱即用预置」表述：

```
frp_easy
Copyright (c) 2026 Alan_IFT

本项目（frp_easy）自身的源代码与脚本采用 MIT 许可证，详见同目录下的 LICENSE 文件。

== 关于运行时下载的第三方二进制 ==

frp_easy 在运行时按需从上游开源项目 fatedier/frp 的官方 GitHub Release
下载 frpc、frps 可执行文件（首次使用时由 UI 触发下载）：

  项目地址：https://github.com/fatedier/frp
  许可证：  Apache License 2.0（Apache-2.0）

这些 frp 二进制不属于 frp_easy，未随本仓库或发布包分发；它们由用户在运行时
直接从 fatedier/frp 上游获取，版权与许可证归上游 fatedier/frp 项目所有，
遵循 Apache-2.0，与 frp_easy 自身的 MIT 许可证相互独立、互不影响。

Apache-2.0 许可证全文见：https://www.apache.org/licenses/LICENSE-2.0
```

> AC-8 校验：grep 无「随附」、含「fatedier/frp」+「Apache」。上文满足（「随附」字样已全部移除，标题与正文均改为「运行时下载」）。

### 3.8 文档更新（FR-11 / FR-14）

**`README.md`**：
- L223-225「许可证」注意段：把「`frp_linux/` 与 `frp_win/` 目录下随附的 frp 二进制」改为「frp_easy 在运行时按需从上游 `fatedier/frp` 下载 frpc/frps 二进制」。
- L32 亮点「frp 二进制自动下载」：保留，措辞从「检测到缺失时」微调为「首次使用时检测到 frpc/frps 缺失，UI 顶部弹横幅，一键从 GitHub Releases 下载**最新版**」。
- L208-209 目录树注释：`frp_win/` `frp_linux/` 从「Windows/Linux FRP 二进制（vendored）」改为「frp 二进制运行时下载落地目录（不内置）」。
- **新增「如何更新」**（FR-14）：在「快速开始」或「许可证」前合适位置加一小节，与 install 脚本表述一致：「重新运行同一条一键安装命令即可升级到最新版；升级保留 `frp_easy.toml`、`.frp_easy/` 数据，以及已下载的 frp 二进制。」

**`docs/DEPLOYMENT.md`**：
- **F.5 节**（L539-558）：标题与症状保留；把「在线：UI 顶部横幅点『一键下载』」从「补救路径」语气改为「常规首启路径」语气 —— 例如开头加一句「frp 二进制不随 frp_easy 内置，首次使用时需下载一次（之后持久保留）」。L557「在线」项措辞改为「常规方式：打开 UI，顶部横幅点下载，frp_easy 从 fatedier/frp 自动下载最新版 frpc/frps」。L558「离线」手动放置说明**保留不动**（边界 4.4）。
- **A.0 一键安装小节**（L36-82）：新增一段「如何更新」（FR-14）：「重新运行上方同一条一键安装命令即可升级。升级会停服→覆盖主二进制与脚本→重注册服务，并完整保留 `frp_easy.toml`、`.frp_easy/` 数据目录，以及 `frp_linux/`/`frp_win/` 下你已下载的 frp 二进制。」

**`docs/dev-map.md`**：
- L31-32 目录树：`frp_win/` `frp_linux/` 从「Windows/Linux FRP 二进制（vendored，git 保留）」改为「frp 二进制运行时下载落地目录（T-014：不再内置，仅 LICENSE 占位；downloader 下载落地于此）」。
- L115「FRP 二进制定位」与 L116「FRP 二进制自动下载（T-002）」表格行：downloader 行补一句「T-014：改为下载 fatedier/frp 最新 release（GitHub API 解析 latest tag）」。
- L161「要避免的模式」：无需改（仍准确）。

---

## 4. 复用审计（Reuse audit）

| 需求 | 已有代码 | 文件路径 | 决策 |
|---|---|---|---|
| 异步下载 + 进度追踪 + 原子安装 | `Manager.doDownload` / `progressWriter` | `internal/downloader/downloader.go` | 复用，不动 |
| tar.gz / zip 解压 + Zip Slip 防御 | `extractFromTarGz` / `extractFromZip` | `internal/downloader/downloader.go` | 复用，不动（basename 匹配天然兼容 frp 子目录结构） |
| Windows 原子 rename 兜底 | `doDownload` L226-241 | `internal/downloader/downloader.go` | 复用（insight 2026-05-16：先 Rename 失败再 Remove+Rename） |
| 测试注入 seam（baseURL/goos） | `Manager.baseURL` / `goos` 字段 | `internal/downloader/downloader.go` | 复用模式，新增同款 `apiBaseURL` |
| HTTP 客户端 | `Manager.client`（60s 超时） | `internal/downloader/downloader.go` | 复用（API 查询与下载共用同一 client） |
| JSON 解析 | `encoding/json`（已 import） | `internal/downloader/downloader.go` | 复用标准库，不引第三方 |
| 二进制缺失探测 | `binloc.Missing()` | `internal/binloc/binloc.go` | 复用，不动 |
| 下载触发 API | `downloadBin` / `downloadStatus` handler | `internal/httpapi/handlers_system.go` | 复用，不动 |
| GitHub API 403 先判状态码后解析 | install.sh 步骤 3 模式 + insight-index | `scripts/install.sh` L138-155 | 复用同款模式到 downloader |
| 升级白名单覆盖模式 | install.sh/ps1 升级分支 | `scripts/install.{sh,ps1}` | 改造（从白名单移除 frp 目录） |
| GitHub Release latest API 概念 | install.sh 查的是 frp_easy 自己的 `releases/tags/rolling` | `scripts/install.sh` L26 | 参考但端点不同（本任务查 `fatedier/frp` 的 `releases/latest`） |

---

## 5. 风险分析（Risk analysis）

| # | 风险 | 等级 | 缓解 |
|---|---|---|---|
| **R-1** | 移除内置二进制后 verify_all C.1 Playwright e2e 若断言「无缺失横幅」或依赖二进制存在则 FAIL | 中 | RA 已核：`verify_all` 全文无 `frp_linux`/`frpc` 引用；e2e fixture（`web/tests/e2e/`）走 setup/login/dashboard 流程，不启动 frpc/frps、不断言横幅。**Developer 必须**改动后跑完整 `scripts/verify_all` 确认 C.1 通过；若 e2e 因此 FAIL，按红线 2 经 PM 提阻塞，**不得删测试**。 |
| **R-2** | **下载 latest 使 frp 版本不再受控**：未来 frp 大版本可能改 TOML schema，`internal/frpconf` 渲染的字段被新版 frp 拒绝 → frpc/frps 启动失败 | **高 · 长期** | 本期**不实现**版本适配（O-4）。当前 `internal/frpconf/render.go` 字段对齐 frp camelCase TOML 上游；frp 0.52+ schema 已较稳定，短期风险可控。**强制要求**：Developer 在 `07_DELIVERY.md` 的 `## Insight` 段记录此风险，供后续任务（「frp 版本锁定 / 兼容矩阵」）参考；建议 insight 文字含「downloader 下载 latest，frp 大版本 TOML schema 变更会破坏 frpconf 渲染兼容性」。 |
| **R-3** | install 升级路径误删用户已下载的 frp 二进制 | 中 | §3.3 已**显式修复**：升级白名单移除 `frp_linux/`/`frp_win/`，不再依赖「发布包恰好不含该目录」的脆弱隐式契约。Developer 须验证升级后 `frp_linux/frpc`（若用户先前下载过）仍在。 |
| **R-4** | GitHub API 未认证限流（60 次/时/IP）—— 多次点下载横幅可能触发 403 | 中 | downloader 限流分支已产明确中文提示（§3.4.5），区别于一般网络错误（NF-R1）；用户体验上「稍后重试」可接受。本期不引入 token 认证（增加配置复杂度，且未认证额度对单用户足够）。 |
| **R-5** | 资产匹配用后缀 `_linux_amd64.tar.gz`；若 frp 改用其它压缩格式或架构后缀 | 低 | 后缀匹配比硬拼文件名鲁棒；未命中时进 `failed` 分支并提示「按文档手动下载」（边界 4.1），不崩溃。frp 多年命名稳定。 |
| **R-6** | downloader 测试现状用 `FRPVersion` 常量构造归档内条目路径（`downloader_test.go` L98/L139） | 中 | 移除 `FRPVersion` 后这两处编译失败。Developer 须把测试里的归档条目路径改为字面量（如 `"frp_0.68.1_linux_amd64/frpc"`）或局部常量；并新增 `apiBaseURL` 注入的 latest 解析测试（见 §6 AC-6）。 |
| **R-7** | API 查询与文件下载共用 60s 超时的同一 `http.Client`；API 查询慢会占用时间预算 | 低 | API 响应通常 <1s；60s 对「查询 + 下载」串行总和足够。无需拆分 client。 |

---

## 6. AC 覆盖映射（13 条完成闸门 AC）

| AC | 设计覆盖 |
|---|---|
| **AC-1** git 不再跟踪 4 个 frp 可执行文件 | §3.1 `git rm` 4 个可执行文件 |
| **AC-2** `frp_linux/`、`frp_win/` 目录仍存在 | §3.1 保留 `frp_linux/LICENSE`、`frp_win/LICENSE` 作占位锚点，`git ls-files` 各列 1 个 |
| **AC-3** package.sh 无 frp 二进制打包逻辑 | §3.6 (b) 删 `cp -a "$ROOT/frp_linux/."`/`frp_win/.`；§3.6 (a) 删前置检查块 |
| **AC-4** package.sh 无 frp 二进制时打包成功 | §3.6 (a) 删除前置检查 → 无 `exit 1`；staging 不含 frp 二进制 |
| **AC-5** downloader 不用写死版本号构造 URL | §3.4.4 (a) 移除 `FRPVersion`；URL 来自 `resolveLatestAsset` 解析结果 |
| **AC-6** downloader 测试覆盖 latest 解析成功/403/资产未匹配/下载失败 4 分支 | §3.4 + §6 测试要求（见下）；`apiBaseURL` 注入 seam |
| **AC-7** binloc 既有测试不因移除内置二进制而失败 | §3.5 binloc 不改；`binloc_test.go` 用临时目录自造假二进制（RA 已核 L10-34） |
| **AC-8** NOTICE 去「随附」、含「运行时下载」+「Apache-2.0」 | §3.7 全文改写 |
| **AC-9** install.sh 成功输出含「更新」小节 | §7 install.sh banner 增段 |
| **AC-10** install.ps1 成功输出含「更新」小节 | §7 install.ps1 banner 增段 |
| **AC-11** README/DEPLOYMENT/dev-map 已更新 | §3.8 三文件逐处改动 |
| **AC-12** install.sh/ps1 语法正确 | 改动均为字符串/数组级，Developer 跑 `bash -n` + PowerShell 解析 |
| **AC-13** verify_all 全绿，pass_count ≥ 19 | 所有改动后 Developer 跑 `scripts/verify_all`；新增 downloader 测试只增不减 |

**AC-6 测试要求明细**（`internal/downloader/downloader_test.go` 新增/调整）：

1. **latest 解析成功**：`httptest` server 对 `/repos/fatedier/frp/releases/latest` 返回合法 JSON（含一个 `_linux_amd64.tar.gz` 资产，`browser_download_url` 指向同一 server 的归档路径）；对归档路径返回 `buildTarGz` 产物 → 断言 `Status` 为 `success`、二进制落地。
2. **API 限流 403**：server 对 latest 端点返回 `403` + 合法 JSON 体 → 断言 `failed`、`Error` 含「限流」。
3. **资产未匹配**：server 返回 200 + JSON，但 `assets[]` 无 `_linux_amd64.tar.gz` 后缀项 → 断言 `failed`、`Error` 含「未找到匹配」。
4. **API 网络失败 / 下载失败**：既有 `TestDownload_HTTP404_StatusFailed`、`TestDownload_NetworkTimeout_StatusFailed` 覆盖下载阶段失败；新增「latest API 网络失败」用关闭的 server 或不可达 `apiBaseURL` → 断言 `failed`。
5. **回归修复**：`TestDownload_TarGz_Success_Progress`、`TestDownload_Zip_Success` 等用了 `FRPVersion` 的测试，归档条目路径改字面量并配合新的 `apiBaseURL` mock。

> 测试用 `m.apiBaseURL = srv.URL` + `m.baseURL`（资产 server，可与 apiBase 同一 server 不同路径）+ `m.goos` 三个 seam 注入，全程 `httptest`，无真实网络（沿用 T-002 C-4）。

**MV-1/MV-2/MV-3**（交付后人工验证，非闸门）：联网下载验证、版本一致性、离线启动，由 QA/交付环节人工执行。

---

## 7. install 脚本「更新」小节（FR-12 / FR-13）

**install.sh** 成功 banner（L282-301 `cat <<EOF` 块）在「常用命令」与「卸载」之间新增：

```
更新：
  重新运行同一条一键安装命令即可升级到最新版：
    curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash
  升级会保留你的配置（frp_easy.toml）与数据（.frp_easy/），
  以及已下载的 frp 二进制（frp_linux/）。
```

**install.ps1** 成功 banner（L234-254 here-string 块）在「常用命令」与「卸载」之间新增：

```
更新：
  重新运行同一条一键安装命令即可升级到最新版：
    irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
  升级会保留你的配置（frp_easy.toml）与数据（.frp_easy\），
  以及已下载的 frp 二进制（frp_win\）。
```

> AC-9/AC-10 校验：banner 含「更新」+「保留」+配置/数据表述。措辞两脚本对齐（路径形态按平台：`/` vs `\`）。注意 here-string 内不出现引号包裹的 8+ 字符敏感串（insight 2026-05-19 verify_all A.1 secrets scan）—— 上述文本无此风险。

---

## 8. 分区分配（Partition assignment）

`.harness/agents/` 下存在 `dev-*.md` 分区 agent。但本任务改动**强耦合**：downloader 改造（Go 后端）、package/install 脚本、文档三类改动围绕同一行为变更，且无并行收益（脚本依赖 downloader 行为定义、文档描述两者）。

**裁定：单 Developer 模式**，不拆分区。理由：① 改动量中等且高度耦合；② downloader 是唯一代码改动，脚本/文档是配套；③ 拆分区的协调成本 > 收益。一名 Developer 按以下顺序实现：

1. **A 块代码**：`git rm` 8 个文件 → `.gitignore` → `internal/downloader/downloader.go` 改造 → `downloader_test.go` 调整 + 新增测试 → `go test ./internal/downloader/... ./internal/binloc/...` 绿。
2. **A 块脚本/文档**：`scripts/package.sh` → `NOTICE` → README/DEPLOYMENT/dev-map。
3. **B 块**：`install.sh`/`install.ps1` 升级路径修复 + banner「更新」段 → DEPLOYMENT A.0 + README「如何更新」。
4. **闸门**：`scripts/verify_all` PASS，pass_count ≥ 19。

---

## 9. 设计边界外（Out-of-scope clarifications）

- **不实现** frp 版本锁定 / UI 选版本（O-3）、SHA-256 校验（O-1）、arm64/macOS 下载（O-2/O-8）、frp TOML schema 兼容适配（O-4）、frp_easy 应用内自更新（O-5）、首启自动下载（O-6，本设计明确选候选 A 不做）、离线预置包（O-7）。
- 本设计**不改** `internal/binloc`、`cmd/frp-easy/main.go`、`internal/httpapi/*`、前端任何文件 —— 下载触发链路完全沿用 T-002。
- `package.ps1`（Windows 打包脚本）若也含 frp 二进制打包逻辑，Developer 须一并核对；本设计基于 `package.sh`，若 `package.ps1` 存在对等逻辑应同步移除（Developer 在实现时 grep 确认，属 §3.6 的 Windows 对等改动）。

---

## 10. 给 Developer 的实现注意事项

1. **`git rm` 用 `git rm`，不是 `rm`** —— 要让文件从 git 索引移除（AC-1 验证 `git ls-files`）。8 个文件：`frp_linux/{frpc,frps,frpc.toml,frps.toml}`、`frp_win/{frpc.exe,frps.exe,frpc.toml,frps.toml}`。`frp_linux/LICENSE`、`frp_win/LICENSE` **保留不动**。
2. **`.gitignore` 规则用前导 `/` 根锚定**，精确忽略 4 个文件名，不写 `frp_linux/*` 通配 —— 否则会误伤保留的 `LICENSE`，导致 AC-2 失败。
3. **`downloader_test.go` 会编译失败**（R-6）：移除 `FRPVersion` 后 L98/L139 等引用处须改字面量。新增测试用 `m.apiBaseURL` seam，与既有 `m.baseURL`/`m.goos` 同款。
4. **`extractFromTarGz`/`extractFromZip` 不要改** —— 它们用 `filepath.Base` 匹配，已兼容 frp 归档的 `frp_<ver>_<os>_amd64/` 子目录前缀。
5. **API 请求必须设 `User-Agent` 头** —— GitHub API 对无 UA 的请求返回 403，会被误判为限流。
6. **先判 HTTP 状态码再解析 JSON**（insight-index 已记录）：403 响应体也是合法 JSON，顺序错了会把限流误报成「网络正常但数据异常」。
7. **install 脚本升级路径**：严格按 §3.3 —— 删 `frp_linux/`/`frp_win/` 那段拷贝，不是改条件。验证：`bash -n scripts/install.sh`、PowerShell 解析 `install.ps1`（AC-12）。
8. **package.sh staging 文件数**：实现后用 `find "$top" -type f | wc -l` 实测核对 ≥ 6（预期 7），阈值 `< 6` 保持不变（OQ-3）。
9. **`07_DELIVERY.md` 必须记录 R-2 Insight**（高优先长期风险）—— 这是任务约束的硬要求。
10. **secrets scan**：install 脚本「更新」段、NOTICE 改写文本中不要出现引号包裹的 8+ 字符串（insight 2026-05-19 verify_all A.1）。当前设计文本已规避。
11. 改动后**必跑 `scripts/verify_all`**（红线 2 + AC-13），确认 PASS 且 pass_count ≥ 19；特别留意 C.1 Playwright e2e（R-1）。

---

## 11. Verdict

**READY**

- A 块 11 FR + B 块 3 FR 共 14 FR 全部有逐文件设计覆盖。
- 13 条完成闸门 AC（AC-1~AC-13）全部映射到具体设计点（§6）。
- 4 项开放问题已裁定：OQ-1 候选 B（保留 LICENSE 占位、删 frpc.toml/frps.toml）、OQ-2 候选 A（不自动下载，沿用横幅）、OQ-3 候选 A（阈值不变）、OQ-4 候选 A（升级保留已下载 frp 二进制）。
- 7 项风险（R-1~R-7）均附缓解；R-2 高优先长期风险已要求写入交付 Insight。
- 不引入任何第三方依赖（latest 解析全用 Go 标准库）。
- 无 BLOCKED 项；上游 `01_REQUIREMENT_ANALYSIS.md` verdict=READY。
