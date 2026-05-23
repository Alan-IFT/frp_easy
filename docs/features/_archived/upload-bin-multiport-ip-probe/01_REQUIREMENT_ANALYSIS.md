# T-018 upload-bin-multiport-ip-probe — 需求分析

> **作者**：Requirement Analyst
> **日期**：2026-05-23
> **模式**：full（7-stage）
> **关联看板**：`docs/tasks.md` T-018
> **分区**：A=二进制上传入口 / B=公网 IP 探测扩展 / C=多端口转发与端口能力

## 0. 目标（Goal）

在已交付的 frp_easy 之上，新增三项 Web UI 体验增强：(A) 允许用户在浏览器直接上传 frpc/frps 二进制作为"一键下载"失败时的兜底通道；(B) 公网 IP 探测加入大陆友好源以提升国内 VM 检测成功率；(C) 端口转发新增批量端口、常用端口预设与可用性探测能力。三项均不破坏既有数据 schema，不引入新依赖，作为同一发布交付。

---

## A. 二进制上传入口

### A.1 In-scope 行为（FR-A）

- **FR-A.1**：在已显示"一键下载 frpc / frps"按钮的 **AppLayout 顶部 banner**（与现有"一键下载"按钮组并列）新增"手动上传"次级入口；二者并存，用户可任选其一。Wizard.vue 当前**无**下载入口，本任务**不**在 Wizard 新增上传/下载按钮。
- **FR-A.2**：上传通过 `POST /api/v1/system/upload-bin` 接收 `multipart/form-data`，字段 `kind` ∈ {`frpc`,`frps`}、字段 `file` 为单个二进制文件；单次请求只接受一个二进制（不接收压缩包，不接收多文件）。[PM-DECIDED：拒收压缩包，理由：减少解压相关攻击面与平台无关解包分歧；下载链路才需要解压因为只能拿到 release 资产；用户手动上传完全可以本地解压后只传 binary。]
- **FR-A.3**：后端在落盘前校验：(a) 文件大小 ≤ 64 MiB（[PM-DECIDED]，FRP 1.x 各平台单 binary 实测 ~20 MiB，留 3× 安全余量）；(b) 文件头魔数与运行平台一致（Linux=ELF `\x7fELF`、Windows=PE `MZ` 头并校验偏移 0x3C 的 PE\0\0 签名、macOS=Mach-O `\xfe\xed\xfa\xce/\xcf` 或反序）。
- **FR-A.4**：[PM-DECIDED]（响应 R-1）仅接受与运行平台一致的二进制：Linux 实例只接受 ELF，Windows 实例只接受 PE。跨平台上传返回 422 + 中文消息 "上传的二进制平台不匹配（本机=linux，文件=windows）"。
- **FR-A.5**：上传成功后落盘路径与"一键下载"完全一致（复用 `downloader.Manager.resolveParams` 同款路径）：Linux 落 `<root>/frp_linux/frpc|frps`、Windows 落 `<root>/frp_win/frpc.exe|frps.exe`。
- **FR-A.6**：落盘走"临时文件 → 原子 rename → Linux chmod 0o755"三步，与现有下载链路一致；Windows 上若 rename 失败先 `os.Remove` 再重试（沿用现有 downloader 模式）。
- **FR-A.7**：上传过程中若同 kind 的下载正在进行（`Downloader.Status(kind).Status == "downloading"`），返回 409 `PROC_BUSY` + "下载进行中，请稍后再上传或取消下载"。不强制反向（下载方早已检查上传后的 binary 已存在不阻塞 —— 因为下载也是覆盖）。
- **FR-A.8**：成功后响应 200 JSON `{ok:true, kind, sha256, size, path}`（path 仅返回相对 root 的子路径，不暴露绝对路径，沿用 NF-S 既有口径）。
- **FR-A.9**：前端在 UI 显式标注两路径差异：下载按钮 tooltip = "从 GitHub Releases 自动拉取最新版（境内可能失败）"；上传按钮 tooltip = "本地选择已下载好的 frpc/frps 二进制（适合 GitHub 不可达时使用）"。

### A.2 Out-of-scope（本期不做）

- 上传时不做 GPG/cosign 签名校验。
- 不支持上传压缩包（.tar.gz/.zip）；用户应解压后只上传 binary 本身。
- 不做 binary 的 frp 版本号探测（不调用 `frpc -v`），版本展示沿用现有"未知/由 frpc -v 子进程在启动时报告"机制。
- 不做断点续传、分片上传；64 MiB 内单次完成。
- 不做 macOS 平台支持（沿用项目现状）。
- 不做"已上传后清除"按钮（删除二进制可手动 rm，不属于本期 UX 范围）。

### A.3 边界条件（Boundary）

- **B-A.1 空文件**：`file` 字段为 0 字节 → 422 "上传文件为空"。
- **B-A.2 缺字段**：缺 `kind` 或 `file` → 422 "缺少字段：kind / file"。
- **B-A.3 非法 kind**：`kind` ∉ {`frpc`,`frps`} → 422 "kind 必须为 frpc 或 frps"。
- **B-A.4 超大文件**：> 64 MiB → 413 + "文件超过 64 MiB 上限"；用 `http.MaxBytesReader` 限流而非读完再判（防 OOM）。
- **B-A.5 错误魔数**：上传任意非可执行文件（如 .txt/.png）→ 422 + "不是合法的二进制文件（缺少 ELF/PE 文件头）"。
- **B-A.6 平台不匹配**：见 FR-A.4。
- **B-A.7 并发上传同 kind**：两个浏览器同时上传 frpc → 后到者收 409 `PROC_BUSY`，由进程级 `sync.Mutex`（按 kind 拆 2 把锁）拦截，不是 DB 锁。
- **B-A.8 与下载并发**：见 FR-A.7。
- **B-A.9 落盘失败**：临时文件创建失败 / rename 失败 → 500 + 中文具体原因；临时文件必须清理（defer Remove）。
- **B-A.10 鉴权与 CSRF**：未登录或缺 CSRF token → 401 / 403；上传是写接口，必须走 `SessionAuth` + `CSRF` 中间件（与 `POST /system/download-bin` 同档）。
- **B-A.11 root 目录不存在**：先 `MkdirAll(targetDir, 0o755)` 兜底（沿用 downloader 模式）。
- **B-A.12 frpc/frps 正在运行**：[PM-DECIDED] 上传**允许覆盖**正在运行的 binary，但响应里附加 `advisory: "上传成功；如需立即生效请到运行控制重启 frpc/frps"`。理由：Linux 下文件 inode 仍被运行进程持有，覆盖文件不会影响运行；Windows 下则可能 rename 失败（被锁），此时 errno 信息直接透传给用户，由用户先停进程再重试 —— 不在本接口做"先停进程"自动联动（避免越权）。

### A.4 非功能性需求（NF-A）

- **NF-A.1 性能**：上传 64 MiB 在本地 loopback 应在 < 5 秒内完成；服务端流式写入临时文件，不全文入内存。
- **NF-A.2 安全**：严格的 `multipart` 字段白名单（只读 `kind`/`file`，忽略其它）；不接受任意路径名 —— 文件名只用于日志，落盘路径由后端固定。
- **NF-A.3 可观测**：成功/失败均写 `slog.Info/Error` + `kind`、`size`、`sha256`、`elapsed_ms`；失败必须给用户可读中文（不能只 500）。
- **NF-A.4 审计**：sha256 写入响应与日志，便于用户事后核对（"我上传的真是 fatedier 官方版本吗"）。

### A.5 验收标准（AC-A）

- **AC-A.1** Linux 主机上传 0.5KB 的 `.txt` → 422 + "不是合法的二进制文件"。
- **AC-A.2** Linux 主机上传 Windows .exe → 422 + "平台不匹配"。
- **AC-A.3** Linux 主机上传合法 frpc ELF → 200 + `{ok:true, kind:"frpc", sha256:"...", size:N}`，落盘 `<root>/frp_linux/frpc` 且 mode = 0o755。
- **AC-A.4** Windows 主机上传合法 frpc.exe → 200，落盘 `<root>/frp_win/frpc.exe`。
- **AC-A.5** 上传 70 MiB 文件 → 413 + "文件超过 64 MiB 上限"，且服务端内存峰值未超过 25 MiB（用 MaxBytesReader 验证）。
- **AC-A.6** 上传中并发再起一次同 kind 上传 → 第二个收 409 PROC_BUSY。
- **AC-A.7** 上传请求未带 CSRF token → 403。
- **AC-A.8** 未登录上传 → 401。
- **AC-A.9** 前端 AppLayout 顶部 banner 同时展示下载与上传两按钮（与 FR-A.1 一致），hover tooltip 文案区分两种用途。
- **AC-A.10** 上传成功后 `GET /api/v1/system/ready` 的 `binMissing` 字段对应 kind 移除。

---

## B. 公网 IP 探测扩展

### B.1 In-scope 行为（FR-B）

- **FR-B.1**：在现有 `fetchPublicIP` 中追加 ≥2 个大陆友好源候选。[PM-DECIDED 候选清单]：
  1. `https://ip.cn/api/index?ip=&type=0` (JSON，字段 `ip`)
  2. `https://api.live.bilibili.com/ip_service/v1/ip_service/get_ip_addr` (JSON，字段 `data.addr`)
  3. `https://www.ip.cn/` (HTML 兜底，正则提取首个 IPv4)
  实现至少取前 2 个，第 3 个 HTML 源作为 enabled-by-default 的兜底。
- **FR-B.2**：保留现有 ipify / my-ip.io 两个国际源（境外用户继续可用），不删除。
- **FR-B.3**：探测改为**并发**：所有候选源同时发起 GET，首个返回合法 IP 的胜出；其它请求 ctx.cancel 取消。总预算保持 3s（NF-P1 不变）。
- **FR-B.4**：每个源返回的字符串都必须经 `net.ParseIP` 校验为合法 IPv4 或 IPv6 才被接受；中间页/广告页污染（非 IP 文本）→ 跳过该源、继续等其它源。
- **FR-B.5**：HTML 源用 `regexp.MustCompile(`(\d{1,3}\.){3}\d{1,3}`)` 抽取首个候选，再过 `net.ParseIP`；不解析 DOM。
- **FR-B.6**：保留 5 min 进程内缓存与 `FRP_EASY_PUBLIC_IP` 环境变量短路（沿用 T-017 insight 第 40 条）。环境变量优先级最高，命中则不发任何 HTTP。
- **FR-B.7**：响应体在已有 `{ip, error, advisory}` 之上**新增可选** `source` 字段，标识胜出源（例 `"ip.cn"`）。前端可不展示；测试与运维诊断使用。
- **FR-B.8**：所有外部 HTTP 请求设 `User-Agent: frp_easy`（沿用 T-014 insight：GitHub API 也踩过此坑）。

### B.2 Out-of-scope（本期不做）

- 不引入第三方 HTTP 客户端库；继续用 stdlib `http.DefaultClient` 配 ctx。
- 不做 IP 归属地解析（运营商/地理位置）。
- 不做 DNS-over-HTTPS。
- 不做"用户自定义 IP 探测源"配置项。
- 前端不改 UI 布局；如需展示 `source` 仅作 tooltip。

### B.3 边界条件（Boundary）

- **B-B.1 全部源失败**：返回现有 `error: "检测超时，请手动查询"` 兼容前端横幅逻辑，不破坏 T-017 既定 UI。
- **B-B.2 部分源返回非法 IP**：单源失败不污染最终结果，由 IP 校验拦截。
- **B-B.3 HTML 源返回页面巨大**：每源用 `io.LimitReader(body, 256<<10)` 限制 256 KiB（HTML 源），JSON 源 32 KiB。
- **B-B.4 第三方源返回 IPv6**：合法 → 沿用现有 IPv6 `advisory` 提示（"frpc serverAddr 填写时请加方括号"）。
- **B-B.5 国内源域名被劫持/中间证书错误**：单源 TLS 失败 → 跳过 + 写 debug 日志，不上升为整体失败。
- **B-B.6 总 3s 预算用尽**：所有 in-flight 请求 ctx.cancel，返回 error。
- **B-B.7 进程内并发 2 个客户端同时查 IP**：缓存非原子读写但已加 `ipCache.mu` 锁，且偶发"双重 fetch"是可接受的（注释已说明）。
- **B-B.8 `FRP_EASY_PUBLIC_IP` 设了非法值**：[PM-DECIDED] 仍透传该值不做 ParseIP 校验，因为这是用户显式覆盖通道（运维可能用它注入"占位 IP"用于测试）。在响应里加 `source: "env"` 让用户能看出来自环境变量。

### B.4 非功能性需求（NF-B）

- **NF-B.1 性能**：并发探测使前端 IP 显示首屏时间从串行最坏 3s 降至 ~1s（首个成功响应时间）。
- **NF-B.2 网络外发**：每次 cache miss 最多 5 个并发 HTTPS 请求（2 国际 + 3 国内）；缓存命中时 0 次。
- **NF-B.3 兼容性**：不破坏现有 `PublicIPResponse` 契约（`ip` / `error` / `advisory` 字段保持，新 `source` 字段为可选）。

### B.5 验收标准（AC-B）

- **AC-B.1** 国际源全部 mock 失败、`ip.cn` mock 返回 `1.2.3.4` → 响应 `{ip:"1.2.3.4", source:"ip.cn"}`。
- **AC-B.2** 所有 5 源 mock 失败 → 响应 `{error:"检测超时，请手动查询"}`，前端横幅不破。
- **AC-B.3** 任一源返回 `not-an-ip` → 跳过、走其它源。
- **AC-B.4** 设 `FRP_EASY_PUBLIC_IP=10.0.0.5` → 响应 `{ip:"10.0.0.5", source:"env"}`，且无任何 HTTP 外发（用 httptest 拦截器计数验证）。
- **AC-B.5** 5 min 内第二次请求命中缓存 → 返回首次结果，无 HTTP 外发。
- **AC-B.6** 并发探测的总耗时 < 2s（即便慢源 3s 超时，快源 < 1s 也已返回）。
- **AC-B.7** HTML 源返回 1 MiB 大页面 → 读取在 256 KiB 截断，仍能正确抽取 IP。
- **AC-B.8** 所有出站请求都带 `User-Agent: frp_easy` 头（mock 服务器断言）。

---

## C. 多端口转发与端口能力

### C.1 In-scope 行为（FR-C）

#### C-1 批量端口创建

- **FR-C.1.1**：`ProxyForm` 新增"端口模式"选择：`single`（默认，与现有一致）/ `batch`（新增）。仅当 `type ∈ {tcp, udp}` 时可选 batch（http/https 走域名，无意义）。
- **FR-C.1.2**：batch 模式下，"本地端口" + "远程端口" 输入框替换为单个"端口表达式"输入框，语法：
  - 范围：`6000-6010`（含两端，两端为正整数 1-65535，左 ≤ 右）
  - 列表：`22,80,443`（逗号分隔，无空格容忍 = 空格被 trim）
  - 混合：`6000-6010,7000,8000-8010`
  - localPort 与 remotePort 必须用**相同**表达式（即 1:1 端口一致映射）；不支持"本地范围映射到远程范围偏移"。理由：避免用户表达不一致导致的难调试错误。
- **FR-C.1.3**：批量条目数上限 **32**（PM-DECIDED）；超过返回 422 "单次批量端口数超过 32 上限"。
- **FR-C.1.4**：每条 storage.Proxy 的 `name` 自动派生为 `<basename>-<port>`，例 basename=`web`、端口 `6000,6001` → 生成 `web-6000`、`web-6001`。basename 沿用现有 `^[A-Za-z0-9_-]{1,64}$` 校验，但需为派生后名称留 6 字符余量（实际限制 basename ≤ 58）。
- **FR-C.1.5**：[PM-DECIDED 响应 R-3]：批量创建在**单事务**内执行；任一条违反约束（name 重复、(type,remote_port) 重复、total > 200 等）→ 全部回滚，响应 422 + JSON 数组列出每条冲突的 `{port, reason}`。
- **FR-C.1.6**：新增端点 `POST /api/v1/proxies/batch`（不复用 `POST /api/v1/proxies`），请求体 `{type, localIP, portsExpr, basename, enabled}`；响应 201 + `{created, items}`（对象，含创建条数 `created` 与新建条目数组 `items: [ProxyResponse...]`）。
- **FR-C.1.7**：现有 200 条总上限叠加生效（即当前 190 条时批量 + 15 条 → 422 "代理规则已达上限"）。
- **FR-C.1.8**：批量成功后触发一次 `applyConfigBestEffort("frpc")`，不在每条之间触发（避免 32 次 frpc reload）。

#### C-2 常用端口预设

- **FR-C.2.1**：`ProxyForm` 在 `localPort` 字段旁加"常用预设"下拉/Tag 列表；点击 Tag 自动填入端口号。**前端 hardcode**，不入 DB。
- **FR-C.2.2**：预设清单（PM-DECIDED）：
  - SSH 22 / RDP 3389 / VNC 5900
  - HTTP 80 / HTTPS 443 / HTTP-Alt 8080
  - MySQL 3306 / PostgreSQL 5432 / Redis 6379 / MongoDB 27017
  - SMB 445 / FTP 21
- **FR-C.2.3**：预设仅影响输入填充；不强制 type 联动（例如选 MySQL 不自动改 type 为 tcp，但 UI 给出 hint "MySQL 通常用 TCP"）。
- **FR-C.2.4**：batch 模式下，预设变为多选 → 一次填入逗号表达式。

#### C-3 端口可用性探测

- **FR-C.3.1**：新增端点 `POST /api/v1/system/port-probe`，请求体 `{ports: [int...]}`，响应 `{results: [{port, available: bool, reason?: string}]}`。
- **FR-C.3.2**：仅对本机 `127.0.0.1` + `0.0.0.0` 探测：`net.Listen("tcp", "0.0.0.0:<port>")` 试绑定，成功立即 Close → `available:true`；失败 → `available:false, reason:"端口已被占用或受限"`。
- **FR-C.3.3**：[PM-DECIDED 响应 R-2] 端口 < 1024（特权端口）：不真正尝试 Listen（避免在 root 启动下伪绑定误删占位）；直接返回 `{port, available:false, reason:"特权端口（<1024）需以 root/Administrator 启动 frp_easy 才能绑定"}`，前端展示该 reason 即可。
- **FR-C.3.4**：端口列表上限 **64**（PM-DECIDED），超过 → 422 "单次最多探测 64 个端口"。
- **FR-C.3.5**：每个端口探测有 200ms timeout；总体限 5s。
- **FR-C.3.6**：探测仅探 TCP；UDP 探测语义不可靠（Listen 总成功），本期不做。
- **FR-C.3.7**：[PM-DECIDED 安全] 不接受 host 参数 —— 防止接口被用作端口扫描工具扫描任意远程主机。后端硬编码 `0.0.0.0`。
- **FR-C.3.8**：UI：在 `ProxyForm` 的 localPort 字段右侧加"探测可用性"按钮，单端口模式探单端口、批量模式逐个探。结果以 Tag 形式贴在输入框下方（绿=可用，红=占用）。

### C.2 Out-of-scope（本期不做）

- 不实现 FRP 上游的 portsRange 模板语法（项目用结构化 TOML 渲染，绕开）。
- 不做"按 IP 段批量"（远程端口由公网控制，本地批量足够）。
- 不做端口冲突的实时（subscribe）通知，仅按需探测。
- 不做端口探测的结果缓存（每次请求都现探，因为占用状态随时变）。
- 不做 UDP 端口探测、不做 IPv6 端口探测。
- 不改 DB schema（每条端口仍是独立 Proxy 行；批量只是"后端 fan-out"语法糖）。
- 不做"已存在 30 条 web-* 规则，全部一键删除"的反向批量删除（按需后续任务）。

### C.3 边界条件（Boundary）

- **B-C.1** portsExpr 为空 → 422 "端口表达式必填"。
- **B-C.2** portsExpr 非法语法（含字母、负数、`6000-`、`-6000`、左 > 右如 `6010-6000`） → 422 + 具体出错 token。
- **B-C.3** portsExpr 包含端口 0 或 > 65535 → 422 "端口必须在 1-65535 之间"。
- **B-C.4** portsExpr 展开后含重复端口（如 `80,80` 或 `6000-6005,6003`） → 422 "端口表达式含重复项：6003"。
- **B-C.5** basename 派生后某条与 DB 已有 name 冲突 → 422 + 列出冲突项；事务整体回滚。
- **B-C.6** 批量中含与 DB 既有 (type,remote_port) 冲突 → 同上回滚。
- **B-C.7** 单事务批量需在 storage 层提供 `UpsertProxiesTx`（事务版批量插入），不能只在 handler 层 N 次调用现有 UpsertProxy（那样无法回滚）。
- **B-C.8** port-probe 请求体 `ports: []` 空数组 → 200 + `results: []`（合法空批）。
- **B-C.9** port-probe ports 含重复 → 去重后探测，结果按去重后顺序返回。
- **B-C.10** port-probe 端口已被 frp_easy 自身的 :8088 占用 → 仍返回 `available:false`（不特判，因为用户可能就是要诊断这点）。

### C.4 非功能性需求（NF-C）

- **NF-C.1 安全**：port-probe 接口受 `SessionAuth` + `CSRF` 保护（写动作 + 探测能力均敏感）。
- **NF-C.2 性能**：批量 32 条全部新建在 < 200ms（单事务 SQLite）；port-probe 64 端口在 < 2s。
- **NF-C.3 可维护**：预设清单在前端集中导出（如 `web/src/data/portPresets.ts`），便于后续追加；不散在多个组件。
- **NF-C.4 向后兼容**：现有 `POST /api/v1/proxies` 单条接口语义不变（不破坏脚本/curl 用户）。

### C.5 验收标准（AC-C）

#### C-1 批量
- **AC-C.1.1** POST batch `{type:"tcp", basename:"web", portsExpr:"6000-6002"}` → 201 + 3 条响应；DB 内 `web-6000`、`web-6001`、`web-6002` 各一条。
- **AC-C.1.2** portsExpr=`6000-6010,7000` → 12 条创建成功。
- **AC-C.1.3** portsExpr=`abc` → 422 + "端口表达式语法错误"。
- **AC-C.1.4** portsExpr 展开 = 35 条 → 422 + "单次批量端口数超过 32 上限"。
- **AC-C.1.5** DB 现有 198 条，批量 5 条 → 422 + "代理规则已达上限"，**且 DB 行数仍为 198**（验证事务回滚）。
- **AC-C.1.6** basename=`web`，DB 已有 `web-6000`，批量含 `6000` → 422 + 冲突项明细 `[{port:6000, reason:"name 已存在"}]`，DB 不变。
- **AC-C.1.7** basename=`ab`、portsExpr=`6000-6005` → name 全部派生为 `ab-6000` 等，6 字符余量校验通过。
- **AC-C.1.8** basename 超 58 字符 → 422 "basename 过长"。

#### C-2 预设
- **AC-C.2.1** 前端 ProxyForm 渲染出"SSH 22 / MySQL 3306 / ..."Tag 列表，点击后 localPort 输入框值变为对应数字。
- **AC-C.2.2** 选预设后用户仍可手动改 localPort，无强制。
- **AC-C.2.3** batch 模式下点 SSH + MySQL 两 Tag → portsExpr 变为 `22,3306`。

#### C-3 探测
- **AC-C.3.1** POST port-probe `{ports:[80,9999]}` → 在 80 已被系统占用、9999 未占用的环境下，响应 `[{port:80,available:false,...},{port:9999,available:true}]`。
- **AC-C.3.2** ports=[22] 在非 root 下 → `{port:22, available:false, reason:"特权端口..."}`（不真探）。
- **AC-C.3.3** ports=[] → 200 + `results:[]`。
- **AC-C.3.4** ports=[70000] → 422 + "端口必须在 1-65535 之间"。
- **AC-C.3.5** ports 含 65 项 → 422 + "单次最多探测 64 个端口"。
- **AC-C.3.6** ports=[80,80,80] → 探一次、results 长度 1（去重）。
- **AC-C.3.7** 请求体 `{ports:[80], host:"8.8.8.8"}` → host 字段被忽略，依然探本机 80；接口不接受 host 参数（无回归路径）。
- **AC-C.3.8** 未登录 → 401；未带 CSRF token → 403。
- **AC-C.3.9** 前端 ProxyForm "探测可用性" 按钮单端口模式点击 → 在 200ms 内显示绿/红 Tag。

---

## 跨模块约束

- **CC-1 数据库 schema 不变**：A/B/C 三模块均不需要新 migration。批量端口在 storage 层只是事务包装；上传二进制只动文件系统；IP 探测只动 in-memory cache。
- **CC-2 不引入新依赖**：全部用 Go stdlib + 现有 chi/sqlite 驱动 + 现有 Vue/naive-ui。multipart 用 `r.ParseMultipartForm` + `MaxBytesReader`；并发探测用 `sync.WaitGroup` + `context`；端口探测用 `net.Listen`。
- **CC-3 中间件**：所有新写接口（A 的 upload-bin、C 的 batch、C 的 port-probe）走 `SessionAuth` + `CSRF` 中间件，与现有写接口同档。
- **CC-4 国际化**：所有用户可见错误消息**中文**（沿用项目规则）。
- **CC-5 verify_all 必须 PASS**：含 A.1 secrets scan、B.1 frontend type-check、C.1 backend test、E.6 Adversarial tests 段（英文标题）。
- **CC-6 不破坏现有契约**：`PublicIPResponse` 字段保持；`POST /api/v1/proxies` 单条仍工作；`POST /api/v1/system/download-bin` 不变。
- **CC-7 OpenAPI 同步**：新增 3 个 endpoint 必须更新 `docs/spec/openapi.yaml`（沿用 T-005 insight：字段以 Go 常量为权威）。
- **CC-8 E2E 烟测**：现有 Playwright 烟测不被破坏；本期不强制加 E2E（QA 决定，建议加 1-2 个 happy path）。

---

## 与历史任务的关联

| 任务 | 关系 | 复用 |
|---|---|---|
| T-002 zero-config-quickstart | 下载/公网 IP 基础设施的原始引入 | A 的落盘路径 = T-002 定义；B 的缓存与超时框架 = T-002 |
| T-005 docs-and-api-schema | OpenAPI 契约维护 | CC-7 同步规则 |
| T-007 hardening-pass-audit | UNIQUE 错误文本格式 + AtomicWrite 双 chmod | A 的落盘逻辑沿用；C 的批量冲突 422 文案沿用 |
| T-014 frp-binary-auto-download | 下载器查 latest release + UA 头 | A 与下载器路径一致；B 的 UA 头沿用 |
| T-017 install-role-and-public-ip | 公网 IP 在国内全失败的 insight + 手动覆盖通道 | B 的 `FRP_EASY_PUBLIC_IP` 短路 + 横幅文案沿用 |

历史 insight 命中（必须遵循）：
- `insight-index.md` L10 (Windows os.Rename)：A 的落盘逻辑必须 fallback Remove+Rename
- L17 (AtomicWrite 双 chmod)：A 的 chmod 必须在 rename 后再做一次
- L37 (User-Agent for GitHub API)：B 的所有 HTTP 客户端设 UA
- L40 (公网 IP 国内全失败)：B 必须保留 `FRP_EASY_PUBLIC_IP` 短路 + 横幅

---

## 风险 / 决策点

| ID | 决策点 | 决策 | 理由 |
|---|---|---|---|
| R-1 | 上传跨平台二进制是否允许？ | [PM-DECIDED] 拒绝，仅接受与运行平台一致 | 避免误用、跨平台 binary 直接报错比静默落盘安全 |
| R-2 | 端口探测是否需要 sudo？ | [PM-DECIDED] 仅探 ≥1024；< 1024 直接返回特权端口提示 | 不在低权限下伪绑定误判；不要求用户升权 |
| R-3 | 批量创建中 1 条冲突，回滚 vs 部分成功？ | [PM-DECIDED] 单事务全成功或全失败 | 部分成功语义不清，回滚 + 明细更可调试 |
| R-4 | 上传二进制是否做 SHA256 白名单（"仅允许已知官方 hash"）？ | [PM-DECIDED] 不做白名单，仅展示 sha256 供用户核对 | 用户上传场景多样（如打补丁版），白名单会卡正常路径；展示 sha 已满足审计 |
| R-5 | IP 探测的国内源是否需要可配置？ | [PM-DECIDED] 不开放配置，硬编码 5 源 | 配置项增加复杂度；当前 5 源覆盖国内外足够；后续如需可再开放 |
| R-6 | 批量端口的 portsExpr 是否支持本地→远程偏移映射（如 `6000-6010:7000-7010`）？ | [PM-DECIDED] 不支持，本地与远程必须用相同表达式 1:1 映射 | 偏移语义易混淆、错配端口隐蔽；用户真要这种可后续任务 |
| R-7 | 上传时是否自动停止 frpc/frps 进程？ | [PM-DECIDED] 不联动，仅在响应里给 advisory 提示用户去重启 | 上传 API 不应越权动子进程；运行控制是独立面板 |
| R-8 | port-probe 是否允许 host 参数？ | [PM-DECIDED] 拒绝，硬编码 `0.0.0.0` | 防止接口被用作扫描工具；OWASP 友好 |
| R-9 | 大陆友好源 ip.cn 等的 ToS 是否允许程序化调用？ | [PM-NOTED] 不做合规审计；与 ipify/my-ip.io 同档自承担风险 | 用户层面、轻量频次（5min 缓存）、与已有国际源风险等价 |
| R-10 | 端口预设清单未来扩张是否要进 DB？ | [PM-DECIDED] 本期 hardcode，未来若需用户自定义再进 DB | YAGNI；预设只有 ~12 个，前端常量足够 |

---

## 验证开放问题（Open Questions）

无。所有未定决策已在"风险 / 决策点"段以 [PM-DECIDED] 自填。

---

## Verdict

**READY** — 可进入 Stage 2（Solution Architect）。
