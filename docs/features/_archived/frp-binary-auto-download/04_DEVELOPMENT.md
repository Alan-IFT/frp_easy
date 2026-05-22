# 04 · 开发记录 — T-014 · frp-binary-auto-download

> 模式：`full` · 编写：developer · 日期：2026-05-22 · PM 自治模式
> 上游（只读）：`01_REQUIREMENT_ANALYSIS.md`（READY）、`02_SOLUTION_DESIGN.md`（READY）、`03_GATE_REVIEW.md`（APPROVED FOR DEVELOPMENT）

## Summary

按设计 §3 实现两块：A 块把 frp 二进制（frpc/frps）从 git 内置 + 随发布包分发改为运行时由 `internal/downloader` 从 fatedier/frp 官方 Release **下载最新版**（GitHub Release API 解析 latest tag + 按平台后缀匹配资产）；B 块修复 install 脚本升级路径不再误删用户已下载的 frp 二进制，并在安装成功输出新增「更新」说明。verify_all 全程 PASS 19/19，无新增 WARN/FAIL。

## Files changed

### 删除（git rm，AC-1）
- `frp_linux/frpc`、`frp_linux/frps`、`frp_linux/frpc.toml`、`frp_linux/frps.toml` — 移除 git 索引
- `frp_win/frpc.exe`、`frp_win/frps.exe`、`frp_win/frpc.toml`、`frp_win/frps.toml` — 移除 git 索引
- 共 8 个文件。`frp_linux/LICENSE`、`frp_win/LICENSE` **保留不动**，作为目录占位锚点（AC-2）。

### 新增
- 无新增源文件（downloader 改造在既有文件内；测试在既有 `downloader_test.go` 内扩展）。

### 改动
- `.gitignore` — 新增根锚定规则忽略 4 个 frp 二进制文件名 + downloader 临时文件（`.dl-archive-*.tmp` / `.dl-bin-*.tmp` × 2 目录，覆盖 Gate F-2）。前导 `/` 精确锚定，不误伤 `LICENSE`。
- `internal/downloader/downloader.go` — 移除 `FRPVersion` 常量；`Manager` 新增 `apiBaseURL` 测试注入 seam；新增 `ghRelease` 结构体 + `resolveLatestAsset()` 函数（GitHub Release API 调用、先判状态码后解析 JSON、`io.LimitReader` 1 MiB 防御、按 `_linux_amd64.tar.gz`/`_windows_amd64.zip` 后缀匹配、403/网络/非200/资产未匹配各有中文降级）；`resolveParams()` 去掉 URL 构造、签名改为返回 4 个值；`doDownload()` 在 `resolveParams` 之后、`MkdirAll` 之前插入 latest 解析。解压函数（`extractFromTarGz`/`extractFromZip`）、进度追踪、原子安装、Zip Slip 防御全部不动。
- `internal/downloader/downloader_test.go` — 移除 `FRPVersion` 引用改 `frpTestVersion` 局部字面量；6 个既有测试改用 `apiBaseURL` seam + `newFRPServer` helper（同时响应 API 端点与资产路径）；新增 4 个 latest 解析测试。
- `scripts/package.sh` — 删 frp 子二进制前置检查块；`build_package` 删 linux/windows 两处 frp 整目录 `cp -a` + `chmod`；头注释退出码描述同步。
- `scripts/package.ps1`（**Gate F-1**）— 删 frp 子二进制前置检查的 Windows/Linux 两个 `foreach` 块；`Build-Package` 删 windows/linux 两行 `Copy-Item ... frp_win/frp_linux`；头注释同步。
- `scripts/install.sh` — 升级分支删 `if [[ -d "$EXTRACTED/frp_linux" ]]` 的 `rm -rf`+`cp` 块（OQ-4）；成功 banner 在「常用命令」与「卸载」之间新增「更新」段。
- `scripts/install.ps1` — 升级分支 `foreach ($sub in @("frp_win","scripts"))` 改为 `@("scripts")`（OQ-4）；成功 banner 新增「更新」段。
- `NOTICE` — 改写「关于随附的第三方二进制」→「关于运行时下载的第三方二进制」，移除全部「随附」表述。
- `README.md` — 亮点段、目录树注释、许可证注意段更新；一键安装小节新增「如何更新」。
- `docs/DEPLOYMENT.md` — A.0 一键安装小节新增「如何更新」；F.5 节增「首次使用需下载一次」说明、「在线」改为常规路径语气。
- `docs/dev-map.md` — `frp_win/`/`frp_linux/` 目录描述更新；downloader 表格行补 T-014 说明。
- `scripts/baseline.json` — version 5→6；go_tests 167→171；test_count 224→228；passing_count 219→223。

## 13 闸门 AC 逐条落实

| AC | 落实 | 验证 |
|---|---|---|
| AC-1 git 不再跟踪 4 个 frp 可执行文件 | `git rm` 8 文件 | `git ls-files frp_linux/ frp_win/` 仅余两个 `LICENSE` |
| AC-2 `frp_linux/`、`frp_win/` 目录仍存在 | 保留两个 `LICENSE` 占位 | `git ls-files` 各列 1 个被跟踪文件 |
| AC-3 package 无 frp 二进制打包逻辑 | 删 package.sh/.ps1 前置检查 + 整目录拷贝 | grep 无 `frp_linux/.`/`Copy-Item frp_win`；发布包 tar 列表无 `frp_win/` |
| AC-4 package 无 frp 二进制时打包成功 | 删前置检查 → 无 exit 1 | 实跑 `package.ps1 -Windows -SkipBuild` exit 0，staging 7 文件 |
| AC-5 downloader 不用写死版本号构造 URL | 移除 `FRPVersion` 常量；URL 来自 `resolveLatestAsset` | grep `internal/` 无 `FRPVersion` 常量（仅注释提及历史名） |
| AC-6 downloader 测试覆盖 latest 解析 4 分支 | 新增 `TestResolveLatest_Success`/`RateLimited403`/`AssetNotMatched`/`NetworkFailure` | `go test ./internal/downloader/...` 12 测试全绿 |
| AC-7 binloc 既有测试不失败 | binloc 未改 | `go test ./internal/binloc/...` 5 测试全绿 |
| AC-8 NOTICE 去「随附」含「运行时下载」+「Apache-2.0」 | 全文改写 | `grep -c 随附 NOTICE` = 0；含 `fatedier/frp` + `Apache` |
| AC-9 install.sh 成功输出含「更新」小节 | banner 新增段 | grep `更新：` 命中；含「保留」+ frp_easy.toml/.frp_easy/ |
| AC-10 install.ps1 成功输出含「更新」小节 | banner 新增段 | grep `更新：` 命中 |
| AC-11 README/DEPLOYMENT/dev-map 已更新 | 三文件逐处改 | 见上「改动」清单 |
| AC-12 install.sh/.ps1 语法正确 | — | `bash -n` OK；PowerShell `Parser.ParseFile` 无错误 |
| AC-13 verify_all 全绿 pass_count ≥ 19 | — | verify_all PASS 19/19，WARN 0 / FAIL 0 |

## Gate F-1（package.ps1）落实说明

Gate Review F-1 指出 `package.ps1` 是 `package.sh` 的 Windows 对等脚本，含同款 frp 二进制前置检查与打包逻辑，设计 §3.6/§9 只对 package.sh 给了精确行号。按 F-1 评审补出的确切改动点全部落实：
- 删除 frp 子二进制前置检查的 `if ($Windows)` 与 `if ($Linux)` 两个 `foreach` 块（原 L80-98 区），替换为说明注释；
- 删除 `Build-Package` 中 windows 分支的 `Copy-Item ... frp_win` 与 linux 分支的 `Copy-Item ... frp_linux` 两行（原 L204、L210）；
- 头注释退出码描述同步为「1 前置缺失（bin\frp-easy.exe）」+ 新增 T-014 说明行；
- staging 文件数断言阈值 `-lt 6` 不变（实测 windows staging 7 文件 ≥ 6，OQ-3 候选 A 成立）。
实跑 `pwsh -File scripts/package.ps1 -Windows -SkipBuild` 退出码 0，证明仓库无 frp 二进制时 Windows 打包成功，FR-4 满足。

## Gate F-2 落实

`.gitignore` 临时文件 glob 同时覆盖 downloader 实际两个临时前缀 `.dl-archive-*.tmp` 与 `.dl-bin-*.tmp`（对应 `os.CreateTemp` 的 `.dl-archive-*.tmp` / `.dl-bin-*.tmp`），且 `frp_linux/`、`frp_win/` 两目录各写一对，未写窄。

## verify_all result

- Baseline（改动前）：PASS 19 / WARN 0 / FAIL 0 / SKIP 0
- After changes：PASS 19 / WARN 0 / FAIL 0 / SKIP 0
- Delta：0 new failures，baseline 保持。Go 测试函数 167→171（+4 个 latest 解析测试）；C.1 Playwright e2e 仍 PASS（R-1 确认：移除内置 frp 二进制不影响 e2e）。

## R-2 风险记录（供 PM 收割进 07_DELIVERY.md 的 ## Insight）

**R-2（高 · 长期）**：downloader 改为下载 fatedier/frp 的 latest release 后，frp 版本不再受控。当前 `internal/frpconf/render.go` 渲染的 frpc.toml/frps.toml 字段对齐 frp 上游 camelCase TOML schema；未来 frp 发布大版本若变更 TOML schema（字段重命名/移除/语义改变），frp_easy 渲染的配置可能被新版 frpc/frps 拒绝，导致子进程启动失败。本期按设计 O-4 **不实现**版本适配。建议后续任务（「frp 版本锁定 / 兼容矩阵」）跟进。一句话 Insight 建议：「downloader 下载 frp latest，frp 大版本 TOML schema 变更会破坏 internal/frpconf 渲染兼容性，本期未做版本适配（O-4）」。

## Design drift (if any)

无 DESIGN DRIFT。所有改动严格按 02 设计 §3 落地：
- OQ-1 候选 B（保留 LICENSE 占位、删 8 文件）、OQ-2 候选 A（不自动下载、沿用横幅）、OQ-3 候选 A（阈值 6 不变）、OQ-4 候选 A（升级保留已下载二进制）全部按裁定实现。
- 一处实现细节说明（非 drift）：测试文件按 02 §6「测试用 `apiBaseURL` + `baseURL` + `goos` 三个 seam」的精神实现时，因下载 URL 现在完全来自 API 响应（`browser_download_url` 指向资产路径），改造后的测试不再单独注入 `baseURL`，而是用 `newFRPServer` helper 让同一 httptest server 既响应 API 端点又响应资产下载路径，API 响应里的 `browser_download_url` 直接指向该 server 的 `/archive` 路径。这与设计「全程 httptest、无真实网络」的约束一致，且 `apiBaseURL` seam 正是设计 §3.4.4(a) 新增的注入点。`baseURL` 字段仍保留在结构体中（未被本期测试使用，但无害，保留以免扩大改动面）。

## Open issues for review

- `Manager.baseURL` 字段在本次改造后不再被任何代码路径使用（下载 URL 全部来自 `resolveLatestAsset` 返回的 `browser_download_url`）。本期保留该字段以最小化改动面（移除它需同步删结构体字段，属可选清理）。Code Reviewer 可评估是否在本期一并清理；Developer 倾向保留，因移除是纯 cosmetic 且与本任务行为无关。

## Dev-map updates

- `frp_win/`、`frp_linux/` 目录树注释：`Windows/Linux FRP 二进制（vendored，git 保留）` → `frp 二进制运行时下载落地目录（T-014：不再内置，仅 LICENSE 占位；downloader 下载落地于此）`
- 「FRP 二进制自动下载」表格行标题 `（T-002）` → `（T-002 / T-014）`，备注补「T-014：改为下载 fatedier/frp 最新 release（GitHub API 解析 latest tag，`resolveLatestAsset`），不再用写死的 `FRPVersion`」。

## Insight to surface

frp 上游 Release 资产命名（`frp_<X.Y.Z>_linux_amd64.tar.gz`）与 frp_easy 自身发布包命名（`frp-easy-<version>-<os>-amd64.<ext>`）形态不同，且 frp 的 git tag 带前导 `v`（`v0.68.1`）而资产文件名内版本号不带 `v`；归档内二进制位于一级子目录 `frp_<ver>_<os>_amd64/` 下，现有 `extractFromTarGz/Zip` 用 `filepath.Base` 匹配 basename 天然兼容该子目录前缀，无需改解压逻辑 · evidence: internal/downloader/downloader.go resolveLatestAsset / extractFromTarGz

## Code Review 修复（M-1 / M-2）

> 日期：2026-05-22 · 由 `05_CODE_REVIEW.md`（APPROVED · MINOR 2）路由回 Developer 本期修复。仅改 2 文件，不动其它，未新增/删除测试，baseline.json 不动。

- **M-1 删除死字段 `Manager.baseURL`**：grep 确认 `internal/downloader` 包内（含 `downloader_test.go`）对 `baseURL` 零引用，下载 URL 全部来自 `resolveLatestAsset` 返回的 `browser_download_url`，测试注入走 `apiBaseURL`。删除 `downloader.go:58` 的字段声明 + 行尾注释；`New` 构造未对 `baseURL` 赋值，无需改动。此前「Open issues for review」中 Developer「保留以最小化改动面」的论据撤回——纯净删除，不触碰任何签名/调用方。`go build ./...` + `go test ./internal/downloader/...` 通过。
- **M-2 修正 `docs/DEPLOYMENT.md` C.2.4 / C.3.4 过时手动升级指令**：
  - C.2.4（Linux）：`tar xzf` 成员列表删去 `frp-easy-<新VERSION>-linux-amd64/frp_linux`（发布包已不含该目录，tar 对不存在成员会报错），仅解压 `frp-easy` 主二进制；补说明段，与 A.0「升级保留已下载二进制」对齐。
  - C.3.4（Windows）：删去 `Copy-Item -Recurse -Force ...\frp_win <INSTALL_DIR>\`（发布包已不含 `frp_win`，拷贝不存在源会报错），仅 `Copy-Item` 主二进制 `frp-easy.exe`；补同样说明段。
  - 两段修正后，手动升级与一键安装一致——不覆盖/删除用户运行时下载到 `frp_linux/`/`frp_win/` 的 frpc/frps。

**verify_all 复核（修复后）**：`pwsh -File scripts/verify_all.ps1` → PASS 19 / WARN 0 / FAIL 0 / SKIP 0，与基线一致，无新增 WARN/FAIL。

## Verdict

READY FOR REVIEW
