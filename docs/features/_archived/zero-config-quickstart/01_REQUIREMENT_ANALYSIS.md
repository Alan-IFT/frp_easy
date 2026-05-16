# 01 · 需求分析 — T-002 · zero-config-quickstart

> 模式：`full` · 编写：requirement-analyst · 日期：2026-05-16 · PM 自治模式
> 上游输入（只读）：`docs/features/zero-config-quickstart/PM_LOG.md` + PM 派发描述
> 上游历史：T-001 web-ui-mvp (`docs/features/web-ui-mvp/01_REQUIREMENT_ANALYSIS.md`、`02_SOLUTION_DESIGN.md`、`07_DELIVERY.md`)
> 决策原则（沿用 T-001）：① 用户体验 > ② 软件工程规范 > ③ 长期可维护性

---

## 1. Goal（目标）

消除 frp_easy 首次上手的四处摩擦点（frp 二进制缺失、无部署引导、无公网 IP 辅助、无防火墙提示），使用户在 `git clone` 后仅需在浏览器里完成"选择角色 → 填写公网 IP → 点击启动"即可让 frpc 或 frps 正常运行。

---

## 2. In-scope behaviors（本期必须实现的行为）

> 以下每条均为可观察、可测试的行为。技术实现细节由 Architect 决定。

### 2.1 FRP 二进制自动下载

1. **B-1**：当 `GET /api/v1/system/ready` 返回 `binMissing` 非空时，UI 在现有缺失提示 banner 上额外显示每个缺失二进制的"一键下载"按钮（frpc、frps 各自独立）。

2. **B-2**：点击"一键下载"后，后端立即响应 202，在后台异步完成下载与安装（解压到对应平台目录 `frp_win/` 或 `frp_linux/`）。UI 显示进度条（0–100%），按钮在进行中禁用。进度通过轮询接口获取。

3. **B-3**：下载安装成功后，无需重启 frp_easy 进程，`GET /api/v1/system/ready` 立即返回对应 kind 不再出现在 `binMissing` 中，对应模式开关变为可用状态。

4. **B-4**：下载失败（网络不可达、HTTP 非 2xx、解压错误）时，UI 显示具体失败原因，并提供指向 FRP GitHub Releases 页面的手动下载链接。已存在的有效二进制文件不被覆盖。

5. **B-5**：当对应平台的二进制文件已存在时，"一键下载"按钮不显示（二进制升级不在本期 in-scope）。

### 2.2 部署角色向导（Wizard）

6. **B-6**：首次完成 `/setup`（密码创建成功）后，若后端检测到当前状态为"未配置任何角色"（frpc 与 frps 均未启用且均无已保存的连接配置），则 UI 自动跳转到新增的 `/wizard` 页面而非 `/dashboard`。

7. **B-7**：`/wizard` 页面展示三种角色选项：
   - "仅配置 frpc（我需要穿透到 frps 服务器）"
   - "仅配置 frps（我的机器有公网 IP）"
   - "两者都配置（同一台机器兼做服务端和客户端）"

   三选一，选择后显示对应的最小配置表单。

8. **B-8**：选择"frpc"角色后，wizard 展示 frpc 最小配置表单：
   - `serverAddr`（必填，IP 或主机名）
   - `serverPort`（默认 7000，可选修改）
   - `auth.token`（可选）

   点击"完成配置"后通过现有 `PUT /api/v1/client` + `PUT /api/v1/mode` 保存配置（mode.frpc.enabled=true），跳转到 `/dashboard`。

9. **B-9**：选择"frps"角色后，wizard 展示 frps 最小配置表单：
   - `bindPort`（默认 7000，可选修改）
   - `auth.token`（可选，留空表示不启用 token 鉴权）

   点击"完成配置"后通过现有 `PUT /api/v1/server` + `PUT /api/v1/mode` 保存配置（mode.frps.enabled=true），跳转到 `/dashboard`。

10. **B-10**：选择"两者都配置"角色后，wizard 顺序展示 frps 配置表单（第一步）和 frpc 配置表单（第二步），分别保存。完成后设置 mode.frpc.enabled=true 且 mode.frps.enabled=true。

11. **B-11**：`/wizard` 页面有"跳过，直接进入"入口，点击后直接跳转到 `/dashboard`，不保存任何配置。wizard 被跳过或完成后，后端持久化"wizard 已处理"状态；后续登录不再自动展示 wizard（即 wizard 仅在满足 B-6 条件时出现一次）。

12. **B-12**：wizard 表单字段校验与现有 `/client`、`/server` 页面一致（serverAddr 非空、端口 1–65535），校验失败给字段级错误提示，不允许提交。

### 2.3 公网 IP 自动检测

13. **B-13**：`/server`（frps 配置）页面新增"检测公网 IP"按钮。点击后调用后端检测接口，将结果以 advisory banner 形式展示："检测到公网 IP：{ip}，可将此地址告知 frpc 用户"，banner 含"复制"按钮。

14. **B-14**：后端检测公网 IP 的接口：超时时限 3 秒，超时或网络不可达时返回 HTTP 200 + `{ "error": "检测超时，请手动查询" }`，不返回 4xx/5xx（前端无需特殊错误处理路径）。检测结果在进程内缓存，缓存有效期 5 分钟，避免重复调用外部服务。

15. **B-15**：检测到的公网 IP **不**自动填入任何表单字段。仅为 advisory 展示，用户需手动复制粘贴。原因：检测结果可能与实际对外 IP 不符（多 NIC、NAT、CDN 等场景），必须由用户确认。

### 2.4 防火墙端口提示

16. **B-16**：保存 frps 配置（`PUT /api/v1/server` 成功）后，`/server` 页面在表单下方显示一个可折叠的代码块，内容为针对当前 `bindPort` 的 Linux 防火墙开放命令：
    ```
    sudo ufw allow {bindPort}/tcp
    sudo iptables -I INPUT -p tcp --dport {bindPort} -j ACCEPT
    ```
    代码块含"复制全部"按钮。

17. **B-17**：新建或修改 tcp 类型代理规则（`POST`/`PUT /api/v1/proxies`）成功后，`/proxies` 页面针对该规则显示一个可折叠的代码块，内容为 frps 服务器侧需执行的命令：
    ```
    # 在 frps 服务器上执行（开放 remotePort）：
    sudo ufw allow {remotePort}/tcp
    sudo iptables -I INPUT -p tcp --dport {remotePort} -j ACCEPT
    ```

18. **B-18**：新建或修改 udp 类型代理规则成功后，同 B-17 逻辑，但命令中协议改为 `udp`：
    ```
    sudo ufw allow {remotePort}/udp
    sudo iptables -I INPUT -p udp --dport {remotePort} -j ACCEPT
    ```

19. **B-19**：防火墙提示代码块为**可收起**设计，初始展开；用户点击收起后，该次 UI 交互的临时展示状态消失（刷新页面后如仍有未解除的端口，再次保存才会重新出现）。提示状态**不**持久化到后端。

20. **B-20**：http(s) 类型代理规则不生成防火墙提示（http/https 代理依赖 frps 的 vhostHTTPPort/vhostHTTPSPort，此处提示由 frps 配置页统一处理，不在单条代理层展示）。

---

## 3. Out-of-scope（本期明确不做）

| 编号 | 项 | 依据 |
|---|---|---|
| O-1 | frp 二进制版本升级（已存在二进制的更新） | T-001 O-9 延续；升级逻辑复杂（校验哈希、迁移配置兼容），单独任务处理 |
| O-2 | 下载二进制时的 SHA-256 校验 | HTTPS 已提供传输安全，MVP 阶段可接受；校验功能留下一任务 |
| O-3 | arm64 / Apple Silicon 平台支持 | T-001 NF-C2 仅要求 x64；arm64 另立任务 |
| O-4 | wizard 完成后自动启动进程 | wizard 保存配置；启动操作仍由用户在 /dashboard 手动触发（避免自动启动报错时用户困惑） |
| O-5 | frpc 客户端配置页面的公网 IP 检测 | frpc 部署在 NAT 内网的情形下，本机公网 IP 无助于 frpc 配置（只需 frps 服务器的 IP）；frpc 页面不加检测按钮 |
| O-6 | frps vhostHTTPPort / vhostHTTPSPort 的防火墙提示 | http/https 代理高级配置不在 T-001 基础 frps 表单内；配置字段未暴露时不生成提示 |
| O-7 | Windows 防火墙命令提示（netsh） | 用户场景明确：frps 仅部署在 Ubuntu 22+，frpc 客户端不需要开放入站端口 |
| O-8 | 向导多语言 | 沿用 T-001 O-8，仅中文 |
| O-9 | 配置导入 / 导出 | T-001 O-13 延续 |
| O-10 | wizard 的"返回上一步"功能 | "两者都配置"角色的两步表单仅支持向前；返回需重新进入 /wizard |

---

## 4. Boundary conditions（边界条件）

### 4.1 自动下载边界

- **网络不可达**：下载接口在后台 goroutine 超时（下载超时 60 秒），超时后状态接口返回 `{ "status": "failed", "error": "下载超时" }`。
- **磁盘空间不足**：解压失败时报 OS 错误，状态接口透传错误消息；已写出的临时文件清理后返回 failed。
- **重复触发**：同一 kind 正在下载期间再次调用下载接口返回 409 + 错误码 `PROC_BUSY`（复用现有错误码）。
- **并发下载**：frpc 与 frps 的下载互不影响，可同时进行。

### 4.2 向导边界

- **已有配置的用户**：若 frpc 或 frps 任一已有保存的配置（`kv.frpc.serverConn` 或 `kv.frps.config` 非空）或任一模式已启用，则 B-6 条件不满足，不自动跳转 wizard。
- **wizard 期间后端宕机**：表单提交失败时显示错误，留在 wizard 页面，用户可重试或跳过。
- **serverAddr 输入空字符串**：提交时前端校验报错，不调用后端。
- **wizard 完成后清空 kv 数据**：若用户在 wizard 完成后手动清除 DB，B-6 的"未配置"判断将再次成立，但 wizard 已处理标志已持久化，不会重新展示。

### 4.3 公网 IP 检测边界

- **IPv6 地址**：检测到 IPv6 地址（如 `2001:db8::1`）时原样展示，不过滤；文案提示"IPv6 地址，frpc serverAddr 填写时请加方括号 [2001:db8::1]"。
- **私有 IP 返回**：部分网络环境的检测服务可能返回内网 IP；不做过滤，展示原值，由用户判断。
- **缓存失效**：若检测后网络 IP 发生变化（如 DHCP 重新分配），缓存过期（5 分钟）前不更新；用户可手动再次点击刷新。

### 4.4 防火墙提示边界

- **同一端口多次保存**：每次 PUT /api/v1/server 成功后提示均重新展示（状态不跨提交持久化，见 B-19）。
- **bindPort = 0 或非法**：后端校验失败返回 422，表单不提交成功，提示不显示。
- **删除代理规则后**：删除操作不显示防火墙关闭端口的提示（关闭端口属于高权限操作，UI 不代劳，用户自行判断）。
- **remotePort 与 bindPort 相同**：提示分别展示，均为独立代码块。

---

## 5. Acceptance criteria（验收准则 — 每条可机器/人工验证）

> 编号 AC-1 ~ AC-18。验证方法中涉及"已有二进制"时以现有 `frp_win/` 或 `frp_linux/` 目录为基准；涉及"缺失二进制"时以临时改名可执行文件为模拟手段。

| # | 准则 | 验证方法 |
|---|---|---|
| **AC-1** | 当 frpc 二进制缺失时，`GET /api/v1/system/ready` 返回 `binMissing: ["frpc"]`，前端 banner 显示"frpc 二进制缺失"且包含"一键下载"按钮 | 临时改名 `frp_linux/frpc`，浏览器打开 /dashboard，观察 banner 含下载按钮；`curl /api/v1/system/ready` 验证响应字段 |
| **AC-2** | 点击"一键下载 frpc"后，后端立即返回 202，随后轮询状态接口可得 `{ "status": "downloading", "progress": N }`（N 从 0 递增至 100） | 在网络可达环境下执行，F12 Network 观察初始 202 响应 + 后续轮询响应包含 progress 字段 |
| **AC-3** | 下载安装成功后（不重启 frp_easy 进程），`GET /api/v1/system/ready` 返回的 `binMissing` 不再包含 frpc；对应模式开关变为可点击状态 | 下载完成后立即 `curl /api/v1/system/ready`，对比下载前后 binMissing 值 |
| **AC-4** | 在网络不可达环境下触发下载，轮询接口最终返回 `{ "status": "failed", "error": "..." }`，UI 显示错误消息 + FRP GitHub Releases 页面链接；原有效二进制（若存在）未被改动 | 断网后触发下载，等待超时，验证 UI 错误展示及 MD5 校验原文件未变 |
| **AC-5** | 当 frp_linux/frpc 文件已存在时，`GET /api/v1/system/ready` 返回 `binMissing: []`，对应"一键下载"按钮不渲染到页面 | 正常环境下访问 /dashboard，检查 DOM 中无下载按钮 |
| **AC-6** | 全新数据库（首次 /setup 完成后，无任何 frpc/frps 配置），前端跳转到 /wizard 而非 /dashboard | 删除 `.frp_easy/data.db`，启动，完成 /setup，观察 URL 跳转到 /wizard |
| **AC-7** | /wizard 页面必须先选择角色才可进行下一步；未选角色直接点击"下一步"或"完成配置"无效并给提示 | 在 /wizard 页面不选角色直接点击提交，观察无导航且出现提示文案 |
| **AC-8** | 选择"frpc"角色 → 填写合法 serverAddr（如 `1.2.3.4`）→ 点击"完成配置"后，`GET /api/v1/client` 返回 serverAddr=1.2.3.4，`GET /api/v1/mode` 返回 `{ "frpc": true, "frps": false }`，浏览器跳转到 /dashboard | `curl /api/v1/client` + `curl /api/v1/mode` 在 wizard 完成后验证 |
| **AC-9** | 选择"frps"角色 → 接受默认 bindPort 7000 → 点击"完成配置"后，`GET /api/v1/server` 返回 bindPort=7000，`GET /api/v1/mode` 返回 `{ "frpc": false, "frps": true }`，浏览器跳转到 /dashboard | `curl /api/v1/server` + `curl /api/v1/mode` 验证 |
| **AC-10** | 点击"跳过，直接进入"后跳转到 /dashboard 不保存任何配置；之后重新登录（创建新 session）不再自动跳转 /wizard | 跳过后 `curl /api/v1/client` 验证 serverAddr 为空；退出登录、重新登录后观察 URL 落在 /dashboard |
| **AC-11** | /wizard 中"frpc"表单提交时 serverAddr 为空 → 返回 HTTP 422 + 字段级错误 `{ "error": { "code": "VALIDATION_FAILED", "field": "serverAddr" } }`，浏览器保持在 wizard 页面 | `curl -X PUT /api/v1/client -d '{"serverAddr":"",...}'` 验证响应 |
| **AC-12** | /server 页面点击"检测公网 IP"后，若网络可达，advisory banner 显示检测到的 IP 地址（格式为有效 IPv4 或 IPv6），且该 IP 未被填入 bindPort 等表单字段 | 在有公网访问的机器上测试；F12 Network 观察 `GET /api/v1/system/public-ip` 响应；检查 bindPort 输入框值未被改变 |
| **AC-13** | `GET /api/v1/system/public-ip` 在网络不可达时，3 秒内返回 HTTP 200 + `{ "error": "检测超时，请手动查询" }`，不返回 4xx/5xx | 断网后 `curl /api/v1/system/public-ip`，计时验证 ≤3s 响应 + 验证 HTTP 200 + 检查 error 字段 |
| **AC-14** | 保存 frps bindPort=7001 后，/server 页面出现代码块，包含文本 `sudo ufw allow 7001/tcp` 和 `sudo iptables -I INPUT -p tcp --dport 7001 -j ACCEPT`；代码块含"复制全部"按钮 | `PUT /api/v1/server` 提交 bindPort=7001 后检查页面 DOM 包含以上命令字符串 |
| **AC-15** | 新建 tcp 代理（remotePort=6001）后，/proxies 页面对该规则显示代码块，包含文本 `sudo ufw allow 6001/tcp` 和 `sudo iptables -I INPUT -p tcp --dport 6001 -j ACCEPT`；代码块包含注释"在 frps 服务器上执行" | 提交 tcp 代理后检查页面 DOM |
| **AC-16** | 新建 udp 代理（remotePort=5001）后，/proxies 页面代码块包含 `sudo ufw allow 5001/udp` 和 `sudo iptables -I INPUT -p udp --dport 5001 -j ACCEPT`（协议为 udp，不是 tcp） | 提交 udp 代理后检查页面 DOM |
| **AC-17** | http / https 类型代理提交成功后，/proxies 页面对该规则**不**显示任何防火墙代码块 | 提交 http 类型代理后检查页面 DOM 无防火墙提示元素 |
| **AC-18** | T-001 原有 15 条验收标准（AC-1 ~ AC-15 of T-001）全部保持通过 | 运行 `go test ./...` + `npm run test` 验证；已有测试数量不低于 146（T-001 baseline） |

---

## 6. Non-functional requirements（非功能需求）

### 6.1 性能

- **NF-P1**：`GET /api/v1/system/public-ip` 接口在外部服务可达时响应 P95 ≤ 3 秒（超时硬上限）。
- **NF-P2**：下载进度轮询接口响应 P95 ≤ 200ms（纯内存读取，无 IO）。
- **NF-P3**：wizard 表单提交（PUT /api/v1/client 或 PUT /api/v1/server）P95 ≤ 1 秒（与现有写接口一致）。

### 6.2 安全

- **NF-S1**：下载的 frp 二进制文件写入时使用临时文件 + atomic rename（与现有 `frpconf.AtomicWrite` 规范一致），避免写到一半的文件被 binloc 误用。
- **NF-S2**：下载二进制文件的源 URL 必须使用 HTTPS；不允许 HTTP 下载（防止中间人替换）。
- **NF-S3**：`/api/v1/system/download-bin`、`/api/v1/system/download-status/{kind}`、`/api/v1/system/public-ip` 均需 session 鉴权（受保护端点，同现有受保护接口规范）。
- **NF-S4**：wizard 端点（若新增）遵循现有 CSRF 防护规范（写接口带 X-CSRF-Token）。

### 6.3 可用性 / UX

- **NF-U1**：所有新增 UI 文案使用中文，与 T-001 NF-U1 一致。
- **NF-U2**：防火墙提示代码块中的命令内容可一键复制（clipboard API），复制成功后按钮文字变"已复制 ✓"持续 2 秒。
- **NF-U3**：wizard 页面支持键盘操作（Tab 切换选项、Enter 确认）。
- **NF-U4**：下载进度条每次轮询更新间隔 ≤ 1 秒，进度条动画平滑无跳变。

### 6.4 跨平台 / 兼容

- **NF-C1**：自动下载功能支持 Windows 11 x64（下载 .zip，解压到 `frp_win/`）和 Ubuntu 22+ x64（下载 .tar.gz，解压到 `frp_linux/`），两平台行为一致。
- **NF-C2**：防火墙提示命令固定输出 Linux（ufw + iptables）格式，与后端运行平台无关（因 frps 部署场景固定为 Ubuntu 22+）。
- **NF-C3**：T-001 NF-C1、NF-C2、NF-C3 继续满足。

### 6.5 可观测 / 可维护

- **NF-O1**：二进制下载的全过程（开始、进度、完成/失败）均写结构化日志（level=info/error），含下载 URL、已下载字节数、耗时。
- **NF-O2**：公网 IP 检测结果（含检测耗时、使用的外部服务）写 level=debug 日志，不写 level=info（避免频繁缓存命中刷爆日志）。

---

## 7. Related tasks（关联任务）

- **T-001 · web-ui-mvp**（`docs/features/web-ui-mvp/`）— 本任务在 T-001 基础上扩展：
  - T-001 AC-13 定义了"二进制缺失 → banner + 禁用开关"行为；T-002 在其基础上增加"一键下载"选项（不破坏 AC-13）。
  - T-001 `02_SOLUTION_DESIGN.md` §6.5 决定了 `frp_win/` `frp_linux/` 保留在 git 中，风险 R-5 注明"T-002 再引入 git-lfs"——本期需求不引入 git-lfs，但实现下载功能作为 R-5 的替代缓解方案。
  - T-001 `02_SOLUTION_DESIGN.md` §5.2 给出了 `GET /api/v1/system/ready` 的响应结构（含 `binMissing`），T-002 沿用此结构扩展。
  - T-001 `02_SOLUTION_DESIGN.md` §3.8 `internal/binloc` 的 `Missing()` 接口是本期自动下载触发条件的判据来源。

---

## 8. Open questions + PM 决策（自治模式 — 全部自答）

### Q-1：自动下载是否验证 SHA-256 校验和？

- 候选 A：不验证，依赖 HTTPS 传输安全。
- 候选 B：从 GitHub Releases 下载对应 `.sha256` 文件并校验。

**[PM 决策] 选 A**。依据：原则 ③ 避免实现复杂度；HTTPS 已提供传输安全，校验和仅防止静态托管被篡改场景，不适用于 GitHub 官方 Releases。校验功能留后续任务。

### Q-2：向导是强制走完还是可跳过？

- 候选 A：强制完成，不可跳过，否则 /dashboard 功能受限。
- 候选 B：可跳过，跳过后直接进入 /dashboard（功能不受限）。

**[PM 决策] 选 B**。依据：原则 ①（不阻断已熟悉系统的用户）；原则 ②（T-001 AC-1～AC-15 全部通过的前提是现有 /dashboard 流程不被强制拦截）。wizard 为增强体验，不替代现有页面。

### Q-3：检测公网 IP 使用哪些外部服务？

- 候选 A：单一服务（如 https://api.ipify.org）。
- 候选 B：主备双服务，主超时后自动切换备用。

**[PM 决策] 选 B**。依据：原则 ①（单点故障降低用户体验）。实现细节（具体 URL、超时策略）由 Architect 决定；需求侧要求"检测超时 ≤ 3 秒"且不阻塞页面交互。

### Q-4：wizard 完成后是否自动启动进程？

- 候选 A：自动启动（保存配置 + 立即 Start）。
- 候选 B：仅保存配置，启动由用户在 /dashboard 手动触发。

**[PM 决策] 选 B**。依据：原则 ②（自动启动如遇二进制缺失或端口占用会静默失败，用户更困惑）；原则 ①（/dashboard 的启停按钮提供明确的操作反馈，用户点击启动后才有 5 秒确认窗口，体验更佳）。

### Q-5：wizard 处理状态（已完成/已跳过）如何判断"已处理"？

- 候选 A：完成和跳过均标记，之后不再自动出现。
- 候选 B：仅完成标记，跳过后每次进入 /setup 流程仍重新展示。

**[PM 决策] 选 A**。依据：原则 ①（跳过是用户有意选择，重复弹出会造成干扰）。

### Q-6：防火墙提示是否覆盖 frps dashboard webServer 端口？

- 候选 A：仅覆盖 bindPort（frpc 连入 frps 的主端口）。
- 候选 B：同时覆盖 webServer.port（frps dashboard 端口）。

**[PM 决策] 选 B**。依据：原则 ①（frps dashboard 开启后端口同样需要在防火墙放通，漏提示会让用户困惑为何 dashboard 不可访问）。实现上：若 frps 配置中 dashboard 为开启状态，`PUT /api/v1/server` 成功后需同时展示 bindPort 和 webServer.port 两个端口的提示代码块。（注意：此条目追加到 B-16 的实现范围中，但不新增单独 B 条目 —— Architect 阶段落地。）

### Q-7：自动下载支持 arm64 吗？

- 候选 A：仅 amd64。
- 候选 B：amd64 + arm64。

**[PM 决策] 选 A**。依据：T-001 NF-C2 明确仅要求 x64（即 amd64），arm64 单独任务处理。

### Q-8：下载超时时间？

- 候选 A：30 秒（适合快速网络）。
- 候选 B：60 秒（适合慢速网络）。
- 候选 C：用户可配置。

**[PM 决策] 选 B**。依据：原则 ①（frp 二进制约 15–25 MB，低速网络（1 Mbps）需 ≈120s，60s 为折中；慢速环境下用户会看到进度条持续更新，不会误以为卡住）。C 选项增加配置复杂度，不值得。

> 综上：**0 条 BLOCKED ON USER**。所有技术选型疑问（具体外部 IP 服务 URL、下载 URL 模板、进度轮询 API 结构）已显式委托 Architect 决定。

---

## 9. Verdict

**READY**

- 全部开放问题已在 PM 自治模式下完成决策，无 BLOCKED ON USER 项。
- 20 条 In-scope 行为条目、18 条可验证 AC、8 条 NFR，均满足可测试要求。
- 与 T-001 的接口边界已明确（复用现有 PUT /api/v1/client、PUT /api/v1/server、PUT /api/v1/mode、GET /api/v1/system/ready；新增下载与 IP 检测端点由 Architect 设计契约）。
- 下一阶段：**Solution Architect**（`02_SOLUTION_DESIGN.md`）。重点设计：
  1. 新增 API 端点契约（下载触发、进度轮询、IP 检测）。
  2. 下载实现（并发模型、临时文件处理、解压逻辑）。
  3. wizard 持久化字段（KV key 或新迁移）。
  4. 公网 IP 检测缓存实现。
  5. 前端新增路由 `/wizard` + 组件分区分配。
