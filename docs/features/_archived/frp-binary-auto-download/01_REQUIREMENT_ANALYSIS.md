# 01 · 需求分析 — T-014 · frp-binary-auto-download

> 模式：`full` · 编写：requirement-analyst · 日期：2026-05-22 · PM 自治模式
> 上游输入（只读）：`docs/features/frp-binary-auto-download/INPUT.md`（用户原话 + PM 7 项决策）
> 上游历史：T-002 zero-config-quickstart（`docs/features/_archived/zero-config-quickstart/`，downloader 来源）
> 决策原则（PM 授权全权决策）：① 用户体验好 > ② 符合软件工程标准 > ③ 长期易使用易维护

---

## 1. Goal（目标）

将 frp 二进制（frpc/frps）从 git 仓库内置改为运行时从 fatedier/frp 官方 Release 下载**最新版**，并让 frp_easy 的"更新方式"对用户可发现。

---

## 2. In-scope behaviors（本期必须实现的行为）

> 每条均为可观察、可测试的行为。技术实现细节（目录占位方式、首启是否自动触发下载、GitHub API 调用细节）由 Solution Architect 决定。

### 2.A 块：frp 二进制不再内置、改为运行时下载最新版

1. **FR-1**：从 git 仓库移除内置的 frp 可执行文件 `frp_linux/frpc`、`frp_linux/frps`、`frp_win/frpc.exe`、`frp_win/frps.exe`。移除后 `git ls-files` 不再列出这四个文件。

2. **FR-2**：`frp_linux/` 与 `frp_win/` 两个目录在仓库中保留（作为 downloader 的下载落地目标 + binloc 的定位锚点）。目录在仓库中的占位方式（`.gitkeep` 占位文件 / `.gitignore` 忽略下载产物 / frp 自带的 `LICENSE`、`frpc.toml`、`frps.toml` 三个文件去留）由 Architect 决定（见开放问题 OQ-1）。

3. **FR-3**：`scripts/package.sh` 不再把 frp 二进制打进发布包——移除对 `frp_linux/`、`frp_win/` 整目录的 `cp` 打包逻辑（L228-244 区间）。

4. **FR-4**：`scripts/package.sh` 移除对 frp 子二进制存在性的前置检查（L115-131 的 `frp_linux/frpc`、`frp_linux/frps`、`frp_win/frpc.exe`、`frp_win/frps.exe` 缺失即 `exit 1` 的校验块）。移除后 package.sh 在仓库无 frp 二进制时仍能成功打包。

5. **FR-5**：`internal/downloader` 由"下载写死的固定版本"改为"下载 fatedier/frp 的最新 Release"。具体地：
   - 移除 `FRPVersion = "0.68.1"` 这一写死版本常量对下载 URL 的硬约束。
   - 下载前先经 GitHub Release API 解析出 fatedier/frp 当前 latest release 的版本号（tag）。
   - 依据解析出的版本号 + 当前平台/架构（Windows amd64 → `.zip`；Linux amd64 → `.tar.gz`）匹配对应 Release 资产，构造下载 URL。

6. **FR-6**：downloader 在解析 latest release 失败时按失败路径降级，不得 panic、不得让 frp_easy 进程崩溃。须覆盖至少以下失败分支，每个分支产生面向用户的中文错误消息：
   - GitHub API 限流（HTTP 403，响应体为合法 JSON）。
   - GitHub API 网络不可达 / 超时。
   - latest release 中找不到匹配当前平台/架构的资产（资产命名变更场景）。
   失败后 `Status(kind)` 返回 `{ "status": "failed", "error": "<中文消息>" }`。

7. **FR-7**：下载过程中断（连接中断、HTTP 非 2xx、解压失败）时，沿用 T-002 既有失败处理：清理临时文件、状态置 `failed`、已存在的有效旧二进制不被破坏（atomic rename 语义保持，见 T-002 NF-S1）。

8. **FR-8**：当 `frp_linux/`（或 `frp_win/`）下已存在旧版 frp 二进制时再次触发下载，新下载的最新版**覆盖**旧版二进制（保持 T-002 既有 atomic rename 覆盖语义；Windows 走 Remove-then-Rename 兜底）。

9. **FR-9**：`internal/binloc` 与 frp_easy 启动逻辑对"frp 二进制缺失"的处理保持现状语义不变：
   - frp_easy 进程启动**不**被 frp 二进制缺失阻塞，UI 仍可正常打开。
   - `binloc.Missing()` 在二进制缺失时返回缺失 kind 列表。
   - UI 检测到缺失时展示下载横幅（T-002 既有行为）。
   - 启动 frpc/frps 时若二进制缺失返回明确错误（T-002 既有行为）。

10. **FR-10**：改写仓库根 `NOTICE` 文件：删除"`frp_linux/`/`frp_win/` 下随附 frpc/frps 二进制"的表述，改为说明 frp 二进制在运行时从上游 fatedier/frp 下载、遵循其 Apache-2.0 许可证（frp_easy 自身仍为 MIT）。

11. **FR-11**：同步更新文档以反映"frp 二进制不再内置、运行时下载"：
    - `README`：说明 frp 二进制在首次使用时自动下载，离线时按文档手动放置。
    - `docs/DEPLOYMENT.md`：调整 F.5 节（frp 子二进制缺失）措辞，把"一键下载"描述为常规首启路径而非补救路径；保留离线手动放置说明。
    - `docs/dev-map.md`：更新 `frp_linux/`、`frp_win/` 目录的描述（不再是"内置二进制"，改为"运行时下载落地目录"）。

### 2.B 块：更新功能可发现性

12. **FR-12**：`scripts/install.sh` 安装成功输出（L285-300 的成功 banner）新增"更新"小节，明确：重新运行同一条一键安装命令即可升级到最新版，升级过程保留 `frp_easy.toml` 与 `.frp_easy/`（配置与数据）。

13. **FR-13**：`scripts/install.ps1` 安装成功输出（L237-253 的成功 banner）新增同等内容的"更新"小节，措辞与 install.sh 对齐（Windows 路径/命令形态）。

14. **FR-14**：`docs/DEPLOYMENT.md` 一键安装小节 + `README` 各补一句"如何更新"说明，与 FR-12/FR-13 表述一致（重跑一键安装命令即升级，配置数据保留）。

---

## 3. Out-of-scope（本期明确不做）

| 编号 | 项 | 依据 |
|---|---|---|
| O-1 | 下载二进制的 SHA-256 / 签名校验 | 沿用 T-002 O-2，HTTPS 提供传输安全；校验留后续任务 |
| O-2 | arm64 / Apple Silicon 平台支持 | 沿用 T-002 O-3，仅 amd64 |
| O-3 | 让用户在 UI / 配置中指定/锁定 frp 版本 | 用户明确要"最新版"；版本锁定增加配置复杂度，不在本期 |
| O-4 | frp 版本与 frp_easy 渲染的 TOML schema 不兼容时的自动迁移/适配 | 真实长期风险，见第 8 节 R-2；本期仅记录风险，不实现迁移逻辑 |
| O-5 | frp_easy 自身二进制的自动更新（in-app self-update） | B 块只做"更新方式可发现"（重跑一键安装），不引入应用内自更新机制 |
| O-6 | 首启自动触发下载（免点横幅）的实现 | 触发方式是设计决策，委托 Architect（OQ-2）；本期需求只约束"不阻塞启动、离线可正常启动" |
| O-7 | 离线安装包内预置 frp 二进制的选项 | PM 决策：离线降级为文档化手动放置，不再提供预置包 |
| O-8 | macOS 平台的 frp 下载 | 沿用 T-002，downloader 仅支持 windows/linux amd64 |

---

## 4. Boundary conditions（边界条件）

### 4.1 下载 / latest 解析边界

- **首次启动离线**：无网络时 frp_easy 进程正常启动，UI 可打开；frpc/frps 因二进制缺失不可启动，UI 展示缺失横幅；点击下载因网络不可达进入 `failed` 状态并显示错误。
- **GitHub API 限流（HTTP 403）**：未认证请求被限流时响应体为合法 JSON；实现须"先判 HTTP 状态码、再决定是否解析 JSON"，限流时给出明确的中文提示（区别于一般网络错误）。
- **latest release 资产命名变更**：若 fatedier/frp 未来改变 Release 资产命名规则，导致按当前规则匹配不到当前平台/架构的资产 → 进入 `failed`，错误消息提示"未找到匹配当前平台的 frp 资产，请按文档手动下载"。
- **下载中断**：连接中断 / HTTP 非 2xx / 解压失败 → 清理临时文件，状态置 `failed`，旧二进制（若有）不受影响。
- **已存在旧版二进制**：再次下载用最新版覆盖旧版（FR-8）。
- **并发下载**：frpc 与 frps 下载互不影响（T-002 既有）；同一 kind 下载进行中再次触发返回 `ErrAlreadyInProgress`（T-002 既有）。
- **不支持的平台**：非 windows/linux 或非 amd64 → 沿用 T-002 `ErrUnsupportedOS`。

### 4.2 package.sh 边界

- **仓库无 frp 二进制时打包**：FR-3/FR-4 后，package.sh 在 `frp_linux/`、`frp_win/` 无二进制时仍 `exit 0` 成功；产物不含 frp 二进制。
- **staging 文件数下限断言**：package.sh L260 有 `file_count < 6 → exit 1` 的健全性断言；移除 frp 二进制目录后须确认 staging 仍 ≥ 6 个文件，否则该断言阈值需 Architect/Developer 调整（见开放问题 OQ-3）。

### 4.3 install 脚本边界

- **install.sh 复制 `frp_linux/` 逻辑**：install.sh L228-231 在升级路径中 `cp` 发布包内的 `frp_linux/`；FR-3 后发布包不含该目录 → 现有 `[[ -d "$EXTRACTED/frp_linux" ]]` 守卫使该 `cp` 自然跳过，行为不破坏（需 Developer 验证升级路径下不误删用户已下载的 `frp_linux/` 二进制——见 R-3）。
- **升级保留语义**：install 脚本"更新"说明涉及的保留对象为 `frp_easy.toml` 与 `.frp_easy/`；用户运行时已下载的 frp 二进制位于 `frp_linux/`/`frp_win/`，升级是否保留这些已下载二进制属设计决策（OQ-4）。

### 4.4 文档边界

- **NOTICE 措辞**：改写后不得遗留"随附"/"vendored"/"开箱即用预置二进制"的旧表述。
- **DEPLOYMENT.md F.5**：保留离线手动放置说明（这是"不内置"决策的可接受代价，PM 决策 4）。

---

## 5. Acceptance criteria（验收准则）

> 分两类。**【静态/自动可验】= 完成闸门**（必须在 declare-done 前全绿）；**【需真实联网验证】= 交付后人工验证**，非完成闸门。

### 5.1 【静态/自动可验】—— 完成闸门

| # | 准则 | 验证方法 |
|---|---|---|
| **AC-1** | git 仓库不再跟踪四个 frp 可执行文件 | `git ls-files` 输出不含 `frp_linux/frpc`、`frp_linux/frps`、`frp_win/frpc.exe`、`frp_win/frps.exe` |
| **AC-2** | `frp_linux/` 与 `frp_win/` 目录在仓库中仍存在（占位方式由 Architect 定） | `git ls-files frp_linux/ frp_win/` 至少各列出 1 个被跟踪文件（占位文件或保留的 LICENSE 等） |
| **AC-3** | `scripts/package.sh` 不含 frp 二进制打包逻辑 | grep `package.sh` 无 `cp -a "$ROOT/frp_linux/.` 与 `cp -a "$ROOT/frp_win/.` 之类整目录拷贝；无 frp 子二进制存在性前置检查块 |
| **AC-4** | `package.sh` 在仓库无 frp 二进制时打包成功 | 删除/无 frp 二进制环境下运行 `bash scripts/package.sh --linux --skip-build`（或等效），退出码 0，产物 tar.gz 内不含 `frp_linux/frpc` |
| **AC-5** | `internal/downloader` 不再用写死版本号构造下载 URL | grep 确认 `FRPVersion` 常量已移除，或不再被下载 URL 构造引用；下载 URL 来自 latest release 解析结果 |
| **AC-6** | `internal/downloader` 包测试覆盖 latest 解析成功、API 限流(403)、资产未匹配、下载失败四个分支 | `go test ./internal/downloader/...` 全绿；测试用 `baseURL`/`goos` 注入 seam（T-002 既有 C-4 机制）模拟各场景 |
| **AC-7** | `internal/binloc` 既有测试不因移除内置二进制而失败 | `go test ./internal/binloc/...` 全绿（该包测试用临时目录自造假二进制，不依赖仓库内置文件——已核实 `binloc_test.go` L10-34） |
| **AC-8** | `NOTICE` 不再含"随附二进制"表述，含"运行时下载"+"Apache-2.0"表述 | grep `NOTICE`：无"随附"且含"fatedier/frp"+"Apache" |
| **AC-9** | `install.sh` 成功输出含"更新"小节 | grep `scripts/install.sh` 成功 banner 区段含"更新"关键词 + "保留"配置数据表述 |
| **AC-10** | `install.ps1` 成功输出含"更新"小节 | grep `scripts/install.ps1` 成功 banner 区段含等效内容 |
| **AC-11** | `README` / `docs/DEPLOYMENT.md` / `docs/dev-map.md` 均已按 FR-11/FR-14 更新 | grep 三文件：README + DEPLOYMENT 含"更新"说明；dev-map 中 `frp_linux`/`frp_win` 描述不再是"内置二进制" |
| **AC-12** | `install.sh` / `install.ps1` 语法正确 | `bash -n scripts/install.sh`；PowerShell 解析 install.ps1 无语法错误 |
| **AC-13** | `scripts/verify_all` 全绿，pass_count 不低于当前基线（19） | 运行 `scripts/verify_all`，PASS，测试数只升不降（红线 3） |

### 5.2 【需真实联网验证】—— 交付后人工验证（非完成闸门）

| # | 准则 | 验证方法 |
|---|---|---|
| **MV-1** | 在真实联网环境点击下载横幅，downloader 实际从 fatedier/frp 拉到 latest release 的 frpc/frps，解压安装后 `binloc.Missing()` 为空 | 联网机器上完整跑一次下载流程，对比下载前后 `GET /api/v1/system/ready` 的 `binMissing` |
| **MV-2** | 下载到的 frp 版本与 fatedier/frp GitHub 页面当前 latest tag 一致 | 下载完成后运行 `frpc --version`，与 GitHub latest release 页面比对 |
| **MV-3** | 离线环境下 frp_easy 进程正常启动、UI 可打开、缺失横幅正常展示 | 断网启动 frp_easy，验证 UI 可访问、横幅可见 |

---

## 6. Non-functional requirements（非功能需求）

- **NF-S1**：下载源 URL 必须 HTTPS（沿用 T-002 NF-S2）。GitHub Release API 查询同样走 HTTPS。
- **NF-S2**：下载的二进制写入沿用临时文件 + atomic rename（沿用 T-002 NF-S1），写到一半的文件不被 binloc 误用。
- **NF-O1**：latest release 解析与下载全过程写结构化日志（解析出的版本号、下载 URL、字节数、耗时、失败原因），沿用 T-002 NF-O1。
- **NF-C1**：下载功能支持 Windows 11 x64 与 Ubuntu 22+ x64，两平台行为一致（沿用 T-002 NF-C1）。
- **NF-R1**：GitHub API 未认证请求处理须容忍限流——查询失败时给用户可理解的中文降级提示，而非裸 HTTP 状态码。

---

## 7. Related tasks（关联任务）

- **T-002 · zero-config-quickstart**（`docs/features/_archived/zero-config-quickstart/`）— 本任务直接改造 T-002 产物：
  - T-002 `01_REQUIREMENT_ANALYSIS.md` B-1~B-5 定义了 frp 二进制下载/横幅/进度机制；本任务复用该机制（PM 决策 3），仅把"固定版本"改为"latest"。
  - T-002 `01` O-1 当年把"二进制升级"列为 out-of-scope；本任务 FR-8 正式纳入"覆盖式更新"行为。
  - T-002 `02_SOLUTION_DESIGN.md` §3.1 是 `internal/downloader` 的设计依据；本任务在其基础上扩展 latest 解析步骤。
  - T-002 `01` 第 7 节引用了 T-001 `02` §6.5 风险 R-5（"frp_win/frp_linux 保留在 git，T-002 引入 git-lfs"）——本任务从 git 移除二进制，是 R-5 的最终解。
- **T-012 · one-click-install-and-mit-license**（`docs/features/_archived/one-click-install-and-mit-license/`）— 新增了 `NOTICE` 文件，本任务 FR-10 改写它。
- **T-013 · rolling-release-install**（`docs/features/_archived/rolling-release-install/`）— 新增/定型了 `install.sh`、`install.ps1` 一键安装；本任务 FR-12/FR-13 在其成功输出上扩展"更新"说明。insight-index 中关于 GitHub API 403 限流先判状态码、`curl|bash` 路径锚定的事实直接适用于本任务。

---

## 8. 风险 / 开放问题（Open questions for user / PM）

### 8.1 风险（需下游 agent 处理，已记录）

- **R-1（中）· 移除内置二进制对 verify_all 链路的影响**：`binloc` 单测用临时目录自造假二进制（`binloc_test.go` L10-34 已核实），**不受影响**；`scripts/verify_all` 全文 grep 无 `frp_linux`/`frpc`/`frps` 引用，verify_all 自身**不直接依赖**内置二进制。但 verify_all C.1 的 Playwright e2e 若有用例断言"无缺失横幅"或依赖二进制存在，移除后可能 FAIL。**Developer 必须**在改动后跑完整 `verify_all` 确认 C.1 通过；若 e2e 因此 FAIL，按红线 2 经 PM 提阻塞，不得删测试。
- **R-2（高 · 长期）· "下载最新版"使 frp 版本不再受控**：固定版本时 frp_easy 渲染的 frpc.toml/frps.toml 字段与 frp 版本是匹配的；改为下载 latest 后，未来 frp 发布大版本（如 schema 不兼容变更）可能导致 frp_easy 渲染的 TOML 字段被新版 frp 拒绝，frpc/frps 启动失败。本期不实现版本适配（O-4），但这是真实长期维护风险，需在 `07_DELIVERY.md` 的 Insight 段记录，供后续任务（如"frp 版本锁定/兼容矩阵"）参考。
- **R-3（中）· install.sh 升级路径误删已下载二进制**：install.sh L228-231 升级路径对 `frp_linux/` 执行 `rm -rf` 后再 `cp`；FR-3 后发布包不含 `frp_linux/`，`[[ -d "$EXTRACTED/frp_linux" ]]` 守卫为假使该块被跳过 → 不会误删。**Developer 须验证**此守卫确实阻止了 `rm -rf "$INSTALL_DIR/frp_linux"`，否则用户每次升级都要重新下载 frp 二进制（体验回退，违反原则 ①）。
- **R-4（低）· downloader 触发方式**：T-002 中下载由 UI 横幅点击触发；"首启自动下载"是否实现属设计决策，留给 Architect（见 OQ-2）。

### 8.2 开放问题（候选答案；PM 自治模式下由 PM 裁决，非阻塞用户）

#### OQ-1：`frp_linux/`/`frp_win/` 目录在 git 中的占位方式 + frp 自带 `LICENSE`/`frpc.toml`/`frps.toml` 文件去留？

- 候选 A：放 `.gitkeep` 空文件占位；删除 frp 自带的 `LICENSE`/`frpc.toml`/`frps.toml`（这些随二进制一同从上游获取的文件也不再内置）。
- 候选 B：保留 frp 自带的 `LICENSE`（满足 Apache-2.0 的许可证随附义务）+ `.gitignore` 忽略下载落地的二进制；删除 `frpc.toml`/`frps.toml`（frp_easy 自己渲染配置，这两个上游样例无用）。
- 候选 C：保留全部三个文件 + `.gitignore` 忽略二进制。

> PM 决策建议：倾向 **候选 B**——二进制不再随附后 Apache-2.0 的许可证随附义务弱化（运行时下载的二进制由用户从上游直接获取），但保留 `LICENSE` 成本低且明确归属；`frpc.toml`/`frps.toml` 上游样例与 frp_easy 渲染流程无关、删除减少混淆。最终由 Architect 在 `02` 落地。

#### OQ-2：是否在首启自动触发下载（免用户点横幅）？

- 候选 A：不自动触发，沿用 T-002 横幅点击触发（最小改动）。
- 候选 B：首启检测到二进制缺失时自动后台触发下载，横幅同时展示进度（用户无需点击）。

> PM 决策建议：委托 Architect 在 `02` 定夺。硬约束（需求侧已固定，不可推翻）：①启动不可被下载阻塞；②离线时 frp_easy 仍正常启动。

#### OQ-3：package.sh staging `file_count < 6` 健全性断言阈值是否需调整？

- 候选 A：移除 frp 二进制目录后 staging 仍 ≥ 6 文件（frp-easy、README.txt、VERSION、LICENSE、frp_easy.toml.example、scripts/install-service.* 等），阈值不变。
- 候选 B：精确重算移除后的最小文件数，按实际下限调整阈值。

> PM 决策建议：由 Developer 在实现时按实际 staging 内容核对；若仍 ≥ 6 则不动（候选 A），否则按候选 B 调整。

#### OQ-4：install 升级时是否保留用户运行时已下载的 frp 二进制？

- 候选 A：保留——升级只覆盖 frp_easy 自身二进制与脚本，`frp_linux/`/`frp_win/` 内已下载的 frp 二进制原样保留（用户无需每次升级后重新下载，体验最佳）。
- 候选 B：清空——升级时一并清除，强制重新下载最新 frp（保证 frp 也跟随升级）。

> PM 决策建议：倾向 **候选 A**（原则 ①，避免每次升级后 frpc/frps 不可用）。frp 自身的更新由 UI 横幅"重新下载"按需触发，与 frp_easy 升级解耦。最终由 Architect 在 `02` 落地 install 脚本的升级白名单。

---

## 9. Verdict

**READY**

- A 块 11 条 FR + B 块 3 条 FR，共 **14 条 FR**；**13 条完成闸门 AC（AC-1~AC-13）** + 3 条交付后人工验证 MV；6 条 NFR。
- 4 项开放问题（OQ-1~OQ-4）均为**技术/设计选型**，已附候选答案与 PM 决策建议，按 PM 自治模式由 PM/Architect 裁决，**无 BLOCKED ON USER 项**。
- 4 项风险（R-1~R-4）已记录，明确指派下游 agent 处理；其中 R-2 为长期维护风险，要求写入交付 Insight。
- 下一阶段 **Solution Architect**（`02_SOLUTION_DESIGN.md`）重点：① latest release 解析的 API 调用与资产匹配实现；② downloader 的 `FRPVersion` 常量改造与失败降级分支；③ OQ-1~OQ-4 的落地决策；④ package.sh / install 脚本 / NOTICE / 文档的具体改动点清单。
