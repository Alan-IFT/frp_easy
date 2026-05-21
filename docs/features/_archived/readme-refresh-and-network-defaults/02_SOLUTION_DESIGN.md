# 02 解决方案设计 — T-011 readme-refresh-and-network-defaults

> Harness 流水线 stage 2 产出。模式：**full**（7-stage）。
> 上游：`01_REQUIREMENT_ANALYSIS.md`（verdict = `READY FOR DESIGN`，7 FR 组 / 5 NF / 24 AC）。
> 本设计是纯文档刷新 + 网络默认值变更 + 启动安全提示重构，**不新增功能模块、不改 API 形状、不改数据库 schema**。
> 单 Developer 分区（改动横跨 Go 代码 + 前端 config + 脚本 + 文档，逻辑高度耦合，不宜拆分）。

---

## 1. 方案概述

T-011 在系统层面做三件事，互相独立但同期交付：

1. **网络默认值变更（代码 + 测试）**：`internal/appconf` 的出厂默认值 `UIPort 8080 → 7800`、`UIBindAddr 127.0.0.1 → 0.0.0.0`。仅影响**新建配置文件**与**缺省字段补默认**两条路径；`Load()` 的"用户显式值优先"语义保持不变（NF-2）。
2. **启动安全提示重构（代码）**：`cmd/frp-easy/main.go` 把原 NF-S4 的"偏离 127.0.0.1 即 WARN"逻辑（行 135-139）重写为面向新默认值的、中性建设性的中文安全提示。浏览器自动打开的 URL 改写逻辑（行 266-276）经核对在新默认值下仍正确，保留不动。
3. **全量文档刷新**：`README.md` 重写为标准开源结构；`project-status.html`、`architecture.html` 深度刷新到 T-010 实际状态；其余 `docs/` 非归档文档逐一审计同步端口/绑定地址。

不引入任何新依赖。不新增 Go 包、不新增前端组件、不新增 spec 文档（PM 裁决 Q-3）。`config.go` 的 `Validate()` 已用 `net.ParseIP` 接受 `0.0.0.0`，无需改校验。

---

## 2. 受影响文件清单（精确到函数 / 行为）

> 行号基于本设计撰写时（commit `c3f080c`）的快照。Developer 实现时若行号已偏移，以**函数名 + 现状文本**为准定位。

### 2.1 Go 代码 — 端口变更（FR-4）

#### `internal/appconf/config.go`

| 位置 | 现状 | 目标 | AC |
|---|---|---|---|
| 包头 doc-comment 端口表（行 11，`本 UI 服务（HTTP）` 行） | `8080` | `7800` | AC-14 |
| `Default()` 行 49 | `UIPort: 8080` | `UIPort: 7800` | AC-13/14 |
| `Load()` 缺省回填 行 98-100 | `if cfg.UIPort == 0 { cfg.UIPort = 8080 }` | `... = 7800` | AC-14 |

> 注意：`config_test.go` 的 `TestDoc_PortTablePresent`（行 101-113）断言 `config.go` doc-comment 必须含 `{"8080","7400","7500","7000"}` 四个端口数字。改 doc-comment 端口表后，该测试断言数组必须同步改为 `{"7800","7400","7500","7000"}`（见 2.3）。

#### `internal/appconf/config.go` — 绑定地址变更（FR-5）

| 位置 | 现状 | 目标 |
|---|---|---|
| `Default()` 上方 doc-comment 行 44-45 | "默认仅监听 127.0.0.1（NF-S4）。修改 UIBindAddr 为 0.0.0.0 等公网地址时，main.go 会在 stderr 打 WARN。" | 重写为：「默认监听 `0.0.0.0`（所有网卡），便于从其他设备访问 Web UI（frp_easy 本质是远程内网穿透管理工具）。仅需本机访问时可把 `UIBindAddr` 改为 `127.0.0.1`。绑定对外地址时 main.go 会在 stderr 打印一条安全提示。」 |
| `Default()` 行 48 | `UIBindAddr: "127.0.0.1"` | `UIBindAddr: "0.0.0.0"` |
| `Load()` 缺省回填 行 95-97 | `if cfg.UIBindAddr == "" { cfg.UIBindAddr = "127.0.0.1" }` | `... = "0.0.0.0"` |

> **不改**：`Validate()`（行 114-138，`net.ParseIP("0.0.0.0")` 已通过）、`ListenAddr()`（行 141-143）、`Load()` 的解析/写文件主流程（行 61-111 除上述两处回填外）。NF-2 的"用户显式值优先"靠的就是"仅在字段为空/0 时回填"——该条件判断本身不动，只改回填的目标值。

#### `cmd/frp-easy/main.go`

| 位置 | 现状 | 目标 | AC |
|---|---|---|---|
| `usageText` 常量行 68 | `UI 默认地址        http://127.0.0.1:8080` | `... http://127.0.0.1:7800` | AC-10 |
| 包头 doc-comment 行 16 `【NF-S4】` 那行 | `【NF-S4】UIBindAddr != "127.0.0.1" 时 stderr 打 WARN。` | 重写为：`【安全提示】UIBindAddr 为对外地址（0.0.0.0/::）时 stderr 打印安全提示，引导尽快完成 setup。` | — |
| NF-S4 WARN 块 行 134-139 | 见下方 §3 | 重构为新安全提示，见 §3 | AC-18 |
| 端口被占用提示 行 237-239 | `UIPort = %d ... cfg.UIPort+1` | **不改**（相对逻辑，变更后自动建议 7801，FR-4.3） | — |
| 浏览器自动打开 行 266-276 | `if cfg.UIBindAddr == "0.0.0.0" \|\| "::" { openURL = http://127.0.0.1:%d }` | **不改**（经核对：新默认 0.0.0.0 下首启即走此改写分支，行为正确，FR-5.3） | AC-19 |

> `main.go` 行 232-251 的 `ListenAddr()` / `net.Listen` / `frp_easy UI 已启动：http://%s` 不动——`%s` 渲染的是 `addr`（如 `0.0.0.0:7800`），是真实监听地址，正确。

### 2.2 测试 — 端口与绑定地址断言（FR-4 / FR-5）

#### `internal/appconf/config_test.go`

| 行 | 现状 | 目标 | AC |
|---|---|---|---|
| 18 | `cfg.UIBindAddr != "127.0.0.1" \|\| cfg.UIPort != 8080` | `... != "0.0.0.0" \|\| ... != 7800` | AC-13/17 |
| 71 | `{"default", AppConfig{UIBindAddr: "127.0.0.1", UIPort: 8080, ...}, true}` | `UIPort: 7800`（绑定地址保留 `127.0.0.1` 即可，此用例只验 Validate 通过；改 8080→7800 更贴合新默认，二选一均可，建议改 7800） | AC-13 |
| 74 | `{"empty bind", AppConfig{..., UIPort: 8080, ...}` | `UIPort: 7800` | — |
| 75 | `{"bind with port", AppConfig{UIBindAddr: "127.0.0.1:8080", UIPort: 8080, ...}` | 此用例验"带端口的 bind 被拒"——`127.0.0.1:8080` 字面量是**测试夹具**，保留 `8080` 不影响语义；`UIPort: 8080 → 7800`。**注意**：`UIBindAddr: "127.0.0.1:8080"` 这个串里的 8080 不必改（它测的是"host:port 格式被拒绝"的逻辑，端口数字无关紧要），但为全仓库 grep 一致性，可一并改为 `"127.0.0.1:7800"` —— Developer 自行决定，二者皆正确。**推荐改**，避免 AC-10/NF-5 grep 噪音。 |
| 76 | `{"empty datadir", AppConfig{..., UIPort: 8080, ...}` | `UIPort: 7800` | — |
| 90-91 | `TestListenAddr`：`AppConfig{UIBindAddr: "127.0.0.1", UIPort: 8080}`；`a != "127.0.0.1:8080"` | `UIPort: 7800`；`a != "127.0.0.1:7800"` | AC-13 |
| 96 | `c2 := ... UIPort: 8080`；`strings.Contains(a, "8080")` | `UIPort: 7800`；`strings.Contains(a, "7800")` | AC-13 |
| 108 | `for _, must := range []string{"8080", "7400", "7500", "7000"}` | `[]string{"7800", "7400", "7500", "7000"}` | AC-13 |

> **建议新增（可选，强化 NF-2 / AC-20）**：`config_test.go` 现有 `TestLoad_RoundTrip`（行 26-47）已用显式 `UIBindAddr = "0.0.0.0"` / `UIPort = 9090` 验证用户值不被覆盖。AC-20 要求验证"显式写 `127.0.0.1` 经 Load 后仍是 `127.0.0.1`"。现有 RoundTrip 用的是 0.0.0.0，**没有覆盖"用户显式写回环地址 ≠ 新默认值"这条**。Developer **应新增一个测试用例** `TestLoad_ExplicitLoopbackNotOverwritten`：写 `UIBindAddr = "127.0.0.1"` + `UIPort = 8080` 的 toml，`Load()` 后断言 `cfg.UIBindAddr == "127.0.0.1"` 且 `cfg.UIPort == 8080`。这条新测试直接对应 AC-20，且满足 NF-1「测试数只升不降」。

#### `internal/browseropen/browseropen_test.go`

| 行 | 现状 | 目标 | AC |
|---|---|---|---|
| 116 | `_ = Open("http://127.0.0.1:8080")` | `_ = Open("http://127.0.0.1:7800")` | AC-16 边缘 |
| 127 | `if last != "http://127.0.0.1:8080"` | `if last != "http://127.0.0.1:7800"` | — |

> 这是测试常量，两处必须同步改（一处是输入、一处是断言），否则测试失败。

### 2.3 前端配置（FR-4）

| 文件 | 行 | 现状 | 目标 | AC |
|---|---|---|---|---|
| `web/vite.config.ts` | 8 | `server: { proxy: { '/api': 'http://127.0.0.1:8080' } }` | `... 'http://127.0.0.1:7800'` | AC-16 |
| `web/playwright.config.ts` | 9 | `baseURL: 'http://localhost:8080'` | `baseURL: 'http://localhost:7800'` | AC-16 |
| `web/playwright.config.ts` | 24 | `url: 'http://127.0.0.1:8080/api/v1/health'` | `... :7800/api/v1/health` | AC-16 |

> 这两处与 `start-e2e-server.{sh,ps1}` 生成的 `UIPort` 必须一致，否则 E2E（verify_all C.1）的健康检查会连不上服务。设计上三者捆绑：playwright `webServer.url` 的端口 = e2e server 脚本写入的 `UIPort`。

### 2.4 脚本（FR-4）

| 文件 | 行 | 现状 | 目标 |
|---|---|---|---|
| `scripts/start.sh` | 4 | `# 开发模式：Go API (port 8080) + Vite dev (port 5173) ...` | `port 7800` |
| `scripts/start.ps1` | 3 | 同上 | `port 7800` |
| `scripts/start-e2e-server.sh` | 53 | `UIPort     = 8080`（生成的 TOML） | `UIPort     = 7800` |
| `scripts/start-e2e-server.ps1` | 59 | `UIPort     = 8080`（生成的 TOML） | `UIPort     = 7800` |
| `scripts/package.sh` | 145 | `UIPort     = 8080`（打包 README/默认配置） | `UIPort     = 7800` |
| `scripts/package.sh` | 168, 181, 200, 213 | `http://127.0.0.1:8080` / `监听 127.0.0.1:8080` 文案 | `127.0.0.1:7800` / `0.0.0.0:7800`（见下注） |
| `scripts/package.ps1` | 111 | `UIPort     = 8080` | `UIPort     = 7800` |
| `scripts/package.ps1` | 133, 146, 168, 181 | `http://127.0.0.1:8080` / `监听 127.0.0.1:8080` 文案 | 见下注 |

> **package.{sh,ps1} 文案区分**：
> - "看到 stderr 提示 ... 后浏览器打开该地址"——这里描述的是**用户访问 URL**，应写 `http://127.0.0.1:7800`（浏览器访问回环地址正确，与 main.go 改写逻辑一致）。
> - "默认配置：UI 监听 127.0.0.1:8080"——这里描述的是**监听地址**，应改为 `UI 监听 0.0.0.0:7800`（绑定地址变了）。
> - Developer 须按语义区分这两类文案，不要无脑全替换为同一串。

> **e2e server 脚本绑定地址**：`start-e2e-server.{sh,ps1}` 生成的 TOML 未显式写 `UIBindAddr`，则走新默认 `0.0.0.0`。Playwright 健康检查访问 `127.0.0.1:7800`——`0.0.0.0` 监听同时接受 `127.0.0.1` 入站连接，**功能正确**。但若希望 E2E 环境保持回环隔离，**可选**在 e2e server 脚本生成的 TOML 中显式补 `UIBindAddr = "127.0.0.1"`（避免 E2E 测试期间对外开放端口）。**推荐补**——E2E 是 CI/本地测试场景，无对外访问需求，显式回环更安全且消除安全提示对 stderr 的干扰。Developer 实现时若补此行，须同步确认脚本里没有别处依赖默认绑定值。

### 2.5 文档（FR-1 / FR-2 / FR-3 / FR-3b）

详见 §6 文档刷新清单与 §7 过时点清单。

---

## 3. 安全提示重构 — 具体文案与逻辑（FR-5.2，重点）

### 3.1 设计决策：仅在对外绑定时打印（不始终打印）

**方案**：保留"条件触发"，但反转条件并重写文案。
- 触发条件改为：`UIBindAddr` 为 `0.0.0.0` 或 `::`（对外可达地址）时打印；为 `127.0.0.1` / `::1` / `localhost` 时**不打印**（FR-5.2(b)）。
- **理由**：始终打印一行访问信息会和 main.go 行 251 已有的 `frp_easy UI 已启动：http://%s` 重复（那行已经告诉用户监听地址）。安全提示的价值在于"对外暴露"这个特定风险——只在该风险存在时提示，信息密度最高、不产生噪音。仅本机绑定的用户不需要看到任何安全相关文字。这符合 FR-5.2(b) 的硬性要求（回环绑定时不打印）。
- 新默认 `0.0.0.0` 下，绝大多数首启用户**会**看到这条提示——这正是设计意图：默认对外，必须让用户知情并引导其尽快 setup。

### 3.2 触发判定

复用现有判断结构（main.go 行 135 的 `if` 条件），把"≠ 回环 → WARN"反转为"是对外地址 → 提示"。建议判定写法（伪代码，Developer 实现）：

```go
// 安全提示：UI 绑定对外地址时，引导用户尽快完成 setup。
isExposed := cfg.UIBindAddr == "0.0.0.0" || cfg.UIBindAddr == "::"
if isExposed {
    fmt.Fprint(os.Stderr, exposureNoticeText(cfg.UIPort, cfgPath))
}
```

- 用 `== "0.0.0.0" || == "::"` 正向枚举对外地址，**不要**用 `!= "127.0.0.1" && ...` 的反向排除——反向排除会把用户自填的具体局域网 IP（如 `192.168.1.10`）也算作"对外"。按 FR-5.2(a) 字面要求，提示只需覆盖 `0.0.0.0` / `::` 两个 unspecified 地址。用户显式填具体 IP 属高级用法，他清楚自己在做什么，不打提示符合最小惊扰原则。这与 FR-5.2(b) 一致（回环不打），也不违反 (a)（(a) 只要求 0.0.0.0/:: 时打）。
- 该提示是 stderr 单条文本，不阻塞、不改退出码（FR-5.2(c)）——`fmt.Fprint` 后正常往下走，不 `os.Exit`。

### 3.3 建议文案（中性、建设性，三要素齐全）

将文案抽为常量或小函数。建议措辞如下（Developer 可微调用词，但三要素 ①对外可达事实 ②尽快 setup ③如何改回回环 必须齐全，且不得用恐吓性措辞）：

```
提示：frp_easy UI 当前监听 0.0.0.0:<PORT>，局域网/公网内的设备均可访问本管理界面。
  · 请尽快用浏览器打开 UI 完成 setup 向导，创建管理员账号（完成 setup 前界面无密码保护）。
  · frp_easy 已内置认证加固：argon2id 密码哈希、会话 Cookie、CSRF 防护、登录失败限流。
  · 如仅需本机访问，可编辑 <CONFIG_PATH>，将 UIBindAddr 改为 "127.0.0.1" 后重启。
```

- `<PORT>` = `cfg.UIPort`；`<CONFIG_PATH>` = `cfgPath` 变量（main.go 行 129，通常 `frp_easy.toml`）。
- 用"提示："不用"WARN:"——`WARN:` 在日志语境里是异常级别，新默认值下每次启动都触发，用 WARN 会让用户误以为配置出错。"提示："中性。
- 第二要素显式点出"完成 setup 前界面无密码保护"——这是暴露窗口最危险的点（NF-3 关注的 setup 前暴露面），但用陈述句而非威胁句。
- 第三条主动给出关闭对外访问的精确操作（编辑哪个文件、改哪个字段、改成什么值），是"建设性"的体现。
- 多行文本：建议用反引号原始字符串常量 + `fmt.Sprintf` 注入 `PORT` / `CONFIG_PATH`，或 `fmt.Fprintf` 直接格式化。末尾带换行。
- **verify_all A.1 secrets scan 注意**（insight-index 2026-05-19 evidence T-008）：此文案不含任何"引号包裹的 8+ 字符疑似密钥串"，安全。Developer 写文案时勿引入形如 `password = "xxxxxxxx"` 的样例字面量。

### 3.4 浏览器自动打开（FR-5.3，确认保留）

main.go 行 266-276 现有逻辑：`browseropen.ShouldOpen` 为真时，若 `UIBindAddr` 是 `0.0.0.0` / `::` 则把打开 URL 从监听地址改写为 `http://127.0.0.1:<port>`。

**核对结论**：新默认 `0.0.0.0` 下，首启用户必然走入这个改写分支，`openURL` 被正确改写为 `http://127.0.0.1:7800`，浏览器可正常打开（浏览器无法访问 `0.0.0.0` unspecified 地址）。**逻辑无需任何改动，保留原样**。AC-19 专项验证此路径。

---

## 4. 复用审计（Reuse Audit）

| 需求 | 现有代码 | 文件路径 | 决策 |
|---|---|---|---|
| 配置默认值 | `Default()` / `Load()` 缺省回填 | `internal/appconf/config.go:46-111` | 改默认值字面量，复用全部读写/校验逻辑 |
| 绑定地址校验 | `Validate()`（`net.ParseIP`） | `internal/appconf/config.go:114-138` | 复用，`0.0.0.0` 已通过校验，不改 |
| 用户值优先（NF-2） | `Load()` 的 `if cfg.X == 空 { 回填 }` 模式 | `internal/appconf/config.go:95-106` | 复用此条件结构，仅改回填目标值 |
| 启动期 stderr 提示 | NF-S4 WARN 块 | `cmd/frp-easy/main.go:134-139` | 重构（反转条件 + 重写文案），不新增机制 |
| 对外绑定 → 浏览器 URL 改写 | `if UIBindAddr == 0.0.0.0/:: 改写` | `cmd/frp-easy/main.go:266-276` | 复用，原样保留（FR-5.3 已验证正确） |
| 端口占用友好提示 | `isAddrInUse` + stderr 文案 | `cmd/frp-easy/main.go:236-241, 309-327` | 复用，`cfg.UIPort+1` 相对逻辑自动适配 7800→7801 |
| 端口表真相源 | doc-comment 端口表 | `internal/appconf/config.go:7-17` | 复用为 README/architecture.html 端口表的引用源（FR-1.1 §6、Q-3 裁决） |
| 测试基线数字 | `baseline.json` | `scripts/baseline.json`（Go 166 / 前端 57 / 合计 223 / passing 218） | 文档刷新时以此为唯一真相源（FR-2.1 / FR-3b.1） |
| project-status.html / architecture.html | 已存在的独立 HTML | `docs/project-status.html`、`docs/architecture.html` | 原地刷新，**不新建** HTML（PM 决策 4：避免重复文档） |

**新模块**：无。本任务不新增任何 Go 包、前端组件、HTML 文件、spec 文档（PM 裁决 Q-3：现有载体已足够）。**不新增 `LICENSE` 文件**（PM 裁决 Q-2）。

---

## 5. 流程（绑定/端口变更后的启动序列）

```
frp-easy 启动
  │
  ├─ flag 解析（--version/--help 早于 Load，不受影响）
  │
  ├─ appconf.Load("frp_easy.toml")
  │     ├─ 文件不存在 → 写 Default(){UIBindAddr:"0.0.0.0", UIPort:7800,...} → 返回
  │     └─ 文件存在 → Unmarshal → 缺省字段回填(空→0.0.0.0 / 0→7800) → Validate
  │           └─ 用户显式写了值 → 保留用户值（NF-2，AC-20）
  │
  ├─ 【安全提示】UIBindAddr ∈ {0.0.0.0, ::} ?
  │     ├─ 是 → stderr 打印 §3.3 三要素安全提示（不阻塞）
  │     └─ 否（回环/具体IP）→ 跳过
  │
  ├─ ... storage / logger / procmgr / httpapi 不变 ...
  │
  ├─ net.Listen("tcp", "0.0.0.0:7800")
  │     └─ 端口占用 → stderr "端口 7800 已被占用 ... UIPort = 7801" → exit 2
  │
  ├─ stderr "frp_easy UI 已启动：http://0.0.0.0:7800"
  │
  └─ browseropen.ShouldOpen ?
        └─ 是 → openURL = (UIBindAddr∈{0.0.0.0,::} ? "http://127.0.0.1:7800" : "http://0.0.0.0:7800")
              → 浏览器打开 http://127.0.0.1:7800   ← FR-5.3 / AC-19
```

无新增请求路径、无 API 形状变更、无数据库迁移。

---

## 6. 文档刷新清单（FR-1 / FR-2 / FR-3 / FR-3b）

### 6.1 `README.md` 全量重写（FR-1，AC-1/2/3/4/4b）

按 FR-1.1 固定章节顺序重写。当前 README（135 行）结构杂乱（"功能列表"只到 T-002、"技术债"章节不属于面向用户的 README），整体重写而非增量修补。

**目标章节顺序**：
1. 项目名称 + 一句话简介
2. 项目简介（这是什么 / 解决什么问题，2-5 句）—— frp 是优秀的内网穿透工具但需手写 toml；frp_easy 提供 Web UI 可视化管理 frpc/frps，单二进制部署，零配置上手。
3. 功能亮点 —— **必须覆盖 T-001~T-010**（见下"功能亮点覆盖矩阵"）
4. 快速开始（最短上手路径）—— 下载发布包 → 运行 → 浏览器开 `http://127.0.0.1:7800` → setup 向导
5. 配置说明（`frp_easy.toml` 四字段表）—— `UIBindAddr` 默认值改为 `0.0.0.0`、`UIPort` 改为 `7800`；示例 toml 同步
6. 默认端口表 —— 四行 `7800 / 7400 / 7500 / 7000`（AC-4）
7. 文档导航 —— 指向 DEPLOYMENT.md / project-status.html / architecture.html / dev-map.md / openapi.yaml
8. 开发模式（面向贡献者）—— `scripts/start.sh`；Go API `http://127.0.0.1:7800`、Vite `:5173`
9. 目录结构速览 —— 现有目录树补 `internal/browseropen/`、`internal/logrotate/` 两行
10. 许可证 —— **如实写**"开源许可证待项目维护者确定"；注明 `frp_linux/`、`frp_win/` 下随附 frp 二进制属上游 `fatedier/frp`、遵循 Apache-2.0；**不创建 LICENSE 文件**（AC-4b）

**功能亮点覆盖矩阵**（FR-1.2 / AC-3，README §3 必须可定位以下每一项）：

| 任务 | 亮点 | 来源 |
|---|---|---|
| T-001 | setup 向导 / 进程控制 / 代理 CRUD / DB→TOML 管道 / 日志查看 / 认证安全 | 现 README §T-001 |
| T-002 | frp 二进制自动下载 / 部署向导 / 公网 IP 检测 / 防火墙提示 | 现 README §T-002 |
| T-003 | README + 健康报告（project-status.html）首版 | tasks.md |
| T-005 | OpenAPI 3.0.3 schema（`openapi.yaml`） | tasks.md |
| T-006 | E2E 烟雾测试（Playwright） | tasks.md |
| T-007 | 安全加固（ui.log 0600 / 对抗性测试） | tasks.md |
| T-008 | 部署套件（`scripts/package.{sh,ps1}` / DEPLOYMENT.md / `--help` / systemd / Windows Service） | tasks.md |
| T-009 | 跨 shell parity（PowerShell + Git Bash） | tasks.md |
| T-010 | 浏览器自动打开 / 日志轮转（size+age+count）/ GitHub Actions CI | tasks.md |

> Developer 实现时以 `docs/tasks.md` 与现有 `docs/project-status.html` §2 为功能描述真相源，不要凭记忆写。AC-3 只要求 T-006 之后能力（E2E / 部署套件 / 浏览器自动打开 / 日志轮转 / CI）至少各 1 处可定位——上表已满足。

**README 安全说明**（FR-5.4）：§5 配置说明或 §6 端口表附近，须写明：默认 `0.0.0.0` 的取舍理由（远程穿透管理工具天然需跨设备访问）、已内置认证加固（argon2id + session + CSRF + 限流）、仅需本机访问时改 `UIBindAddr = "127.0.0.1"` 的操作。删除现 README 行 97-99 那段"默认仅监听 127.0.0.1"的旧安全警告，改写为新表述。

### 6.2 `docs/project-status.html` 深度刷新（FR-2，AC-5/6/7/8/9）

| 区块 | 现状（行） | 目标 |
|---|---|---|
| 头部 `更新日期`（行 223） | `2026-05-16` | 本任务交付日期（Developer 用交付当日，约 2026-05-21） |
| §4 引导句（行 353） | `截至 T-005 交付（2026-05-16）` | `截至 T-010 交付` |
| §4 测试数字（行 356/364 等） | Go `119` / 合计 `164` | Go `166` / 前端 `57` / 合计 `223`（读 `baseline.json`，AC-6） |
| §4 版本节点演进表（行 387-388） | 末行停在 `T-005 结束` | 追加 T-006~T-010 行，末行数字 = baseline.json |
| §4 verify_all 项表 | 旧值（17 项） | `19` 项 PASS（AC-8，依据 commit `d22d0d8`） |
| §2 已交付功能 | 停在 T-005 | 补 T-005 OpenAPI / T-006 E2E / T-007 安全加固 / T-008 部署套件 / T-009 跨shell parity / T-010 浏览器打开+日志轮转+CI（AC-7） |
| §3 架构模块表 | 缺新模块 | 补 `internal/browseropen`、`internal/logrotate`（AC-7）；建议同时确认 `internal/downloader` 在表内 |
| §5 技术债 / §6 优化建议 / §7 已知后续 | 停在 T-005，§7 写"无后续事项" | 更新到 T-010 实际状态；§7 加 T-011 网络默认值变更的取舍说明（默认 0.0.0.0 的安全权衡） |

> FR-2.6 / AC-9：保持纯内联 HTML/CSS，无外链。刷新时**不要**引入 `<link>` / `<script src>` 外部资源。

### 6.3 `docs/architecture.html` 深度刷新（FR-3b，AC-12b/12c）

| 区块 | 现状（行） | 目标 |
|---|---|---|
| 开发模式卡片（行 650） | `go run ./cmd/frp-easy（后端，端口 8080）` | `端口 7800` |
| 开发模式卡片（行 652） | `Vite proxy：/api → http://127.0.0.1:8080` | `http://127.0.0.1:7800` |
| 测试覆盖 — Go（行 668） | `101 个测试用例，覆盖全部 14 个 AC` | `166 个测试用例`（baseline.json）；删除/更新"14 个 AC"写死表述——改为不写死数字的描述（如"覆盖核心 AC 与对抗性场景"），FR-3b.1 |
| 测试覆盖 — Vitest（行 672） | `45 个测试用例` | `57 个测试用例`（baseline.json） |
| 架构 / 模块描述 | 缺 T-002~T-010 新模块 | 补 `internal/downloader`、`internal/browseropen`、`internal/logrotate`（AC-12b） |
| 头部更新日期 | architecture.html 头部 `<h1>` / `subtitle`（行 145-146）**当前无显式日期标识** | FR-3b.4 要求"头部更新日期/版本标识（若有）更新"——当前无日期元素。Developer **应在 subtitle 下方补一行更新日期**（与 project-status.html 的 `.meta` 行风格一致），写本任务交付日期，使 AC-12b "头部更新日期不再是 2026-05-16" 可验证。**注意**：architecture.html 全文件 grep 无 `2026-05-16`——AC-12b 该子句实为"补一个正确的日期"。 |

> FR-3b.5 / AC-12c：保持纯内联，无外链。

> **architecture.html 端口/绑定地址全文核对**：本设计已 grep 确认 architecture.html 的 `8080` 仅出现在行 650/652（均为 UI 服务端口，须改）。Developer 仍须通读全文，确认无遗漏的 `8080` / `127.0.0.1` 默认绑定表述（如有架构图文字描述监听地址）。

### 6.4 其余 docs 审计（FR-3，AC-10/11）

| 文件 | 须改位置 | 说明 |
|---|---|---|
| `docs/DEPLOYMENT.md` | 行 77/80/191/205/392/399/404/412/418/423 的 `8080` | 区分语义：访问 URL 改 `127.0.0.1:7800`；端口占用示例文案改 `7800 / 7801`；`ss -ltnp 'sport = :8080'` / `lsof -iTCP:8080` / `Get-NetTCPConnection -LocalPort 8080` 等诊断命令改 `7800`；行 427"绑定到非 127.0.0.1"的排障表述须按新默认值复核（默认已是 0.0.0.0，排障语境要相应调整）。行 77 的"frp_easy UI 已启动：http://127.0.0.1:8080"是示例输出，须改为 `http://0.0.0.0:7800`（实际监听地址打印格式）。 |
| `openapi.yaml` | 行 12 `servers[].url: http://127.0.0.1:8080` | 改为 `http://127.0.0.1:7800`（AC-11） |
| `docs/dev-map.md` | grep 结果：无 `8080` / `127.0.0.1` | 通读核对模块结构是否需补 browseropen/logrotate（若 dev-map 模块清单过时则一并补）；无端口相关改动 |
| `docs/workflow.md` | 待 Developer grep 核对 | 7-agent 流程文档，预期无端口/绑定相关内容；Developer grep `8080` 确认即可 |
| `docs/spec/README.md` | 待 Developer grep 核对 | PM 裁决 Q-3 不新增 spec；通读核对是否有端口过时表述 |

> **强制兜底（NF-5 / AC-10）**：Developer 完成所有改动后，须执行全仓库 grep（排除 `docs/features/_archived/`）确认：除 FR-4.4 枚举的 FRP 代理端口/测试夹具外，无 `8080` 作 UI 服务端口、无 `127.0.0.1` 作默认绑定地址的表述。建议命令见 §10。

---

## 7. 过时点清单留档（FR-3.2 / AC-12 — 文件 → 行号 → 现状 → 目标）

> 本表即 FR-3.2 要求的"过时点清单"留档，AC-12 据此验证。Developer 实现后此表可直接转入 `07_DELIVERY.md`。

| # | 文件 | 行号 | 现状 | 目标 |
|---|---|---|---|---|
| 1 | `internal/appconf/config.go` | 11 | doc-comment 端口表 `8080` | `7800` |
| 2 | `internal/appconf/config.go` | 44-45 | "默认仅监听 127.0.0.1（NF-S4）" | 重写为默认 0.0.0.0 表述 |
| 3 | `internal/appconf/config.go` | 48 | `UIBindAddr: "127.0.0.1"` | `"0.0.0.0"` |
| 4 | `internal/appconf/config.go` | 49 | `UIPort: 8080` | `7800` |
| 5 | `internal/appconf/config.go` | 96 | `cfg.UIBindAddr = "127.0.0.1"` | `"0.0.0.0"` |
| 6 | `internal/appconf/config.go` | 99 | `cfg.UIPort = 8080` | `7800` |
| 7 | `cmd/frp-easy/main.go` | 16 | 包头 `【NF-S4】... WARN` | 重写为安全提示说明 |
| 8 | `cmd/frp-easy/main.go` | 68 | `UI 默认地址 http://127.0.0.1:8080` | `:7800` |
| 9 | `cmd/frp-easy/main.go` | 134-139 | NF-S4 WARN 块 | 重构为 §3.3 安全提示 |
| 10 | `internal/appconf/config_test.go` | 18,71,74,75,76,90,91,96,108 | `8080` 断言 / 端口集合 | `7800`（详见 §2.2） |
| 11 | `internal/browseropen/browseropen_test.go` | 116,127 | `http://127.0.0.1:8080` | `:7800` |
| 12 | `web/vite.config.ts` | 8 | proxy `:8080` | `:7800` |
| 13 | `web/playwright.config.ts` | 9,24 | baseURL / webServer.url `:8080` | `:7800` |
| 14 | `scripts/start.sh` | 4 | `port 8080` | `port 7800` |
| 15 | `scripts/start.ps1` | 3 | `port 8080` | `port 7800` |
| 16 | `scripts/start-e2e-server.sh` | 53 | `UIPort = 8080` | `7800`（+ 可选补 `UIBindAddr = "127.0.0.1"`） |
| 17 | `scripts/start-e2e-server.ps1` | 59 | `UIPort = 8080` | `7800`（+ 可选补 `UIBindAddr`） |
| 18 | `scripts/package.sh` | 145,168,181,200,213 | `UIPort = 8080` / `127.0.0.1:8080` 文案 | `7800`，按语义区分 URL/监听地址 |
| 19 | `scripts/package.ps1` | 111,133,146,168,181 | 同上 | 同上 |
| 20 | `README.md` | 全文重写 | 结构模板化、功能停在 T-002、端口 8080、绑定 127.0.0.1 | FR-1 标准开源结构、覆盖 T-001~T-010、端口 7800、绑定 0.0.0.0 |
| 21 | `docs/project-status.html` | 223,353,356,364,387-388,§2/§3/§5/§6/§7 | 更新日期/测试数/模块表/功能清单停在 T-005 | 刷新到 T-010（详见 §6.2） |
| 22 | `docs/architecture.html` | 650,652,668,672,模块描述,头部 | 端口 8080、Go 101/前端 45、缺新模块、无更新日期 | 详见 §6.3 |
| 23 | `docs/DEPLOYMENT.md` | 77,80,191,205,392,399,404,412,418,423,427 | `8080` / `127.0.0.1` 排障表述 | 详见 §6.4 |
| 24 | `openapi.yaml` | 12 | `servers[].url: ...:8080` | `:7800` |

---

## 8. 风险分析

| ID | 风险 | 缓解 |
|---|---|---|
| R-1 | 误改 FRP 业务代理端口（FR-4.4 枚举的 `8080`：`storage_test.go:397` LocalPort、`qa_t007_adversarial_test.go:47` rp、`httpapi/qa_ac_test.go:480` rp、`web/.../qa_t007_adversarial.spec.ts`、`ProxyForm.spec.ts`），破坏代理测试语义 | §2 改动清单**不含**这些文件；Developer 严禁对其执行替换。AC-15 专项 grep 校验这批仍为 8080。建议 Developer 用**逐文件定点编辑**而非全仓库 sed 替换。 |
| R-2 | `Load()` 默认值变更意外改写用户既有 `frp_easy.toml` | 仅改回填**目标值**，不动 `if cfg.X == 空/0` 的条件判断本身（§2.1）；新增 `TestLoad_ExplicitLoopbackNotOverwritten`（§2.2）直接验 AC-20。 |
| R-3 | 安全提示文案触发条件写成反向排除（`!= 127.0.0.1`），导致用户填具体局域网 IP 也打提示，或漏掉 `::` | §3.2 明确用正向枚举 `== "0.0.0.0" \|\| == "::"`；FR-5.2(b) 回环不打 + AC-18 验证。 |
| R-4 | `8080` 在 DEPLOYMENT.md / architecture.html 分散且多语义（访问 URL vs 监听地址 vs 诊断命令端口），易遗漏或语义错替 | §6.4 / §2.4 已逐处标注语义；§10 给出 grep 兜底命令；AC-10 grep 全仓库兜底。 |
| R-5 | E2E（verify_all C.1）：playwright `webServer.url` 端口与 e2e server 脚本 `UIPort` 不一致 → 健康检查超时失败 | §2.3/§2.4 把三处（vite proxy / playwright url / e2e server 脚本 UIPort）捆绑为 7800；建议 e2e server 脚本补 `UIBindAddr = "127.0.0.1"` 消除安全提示对 stderr 的干扰。 |
| R-6 | 文档刷新时测试数字与 `baseline.json` / 实际不一致（手工 HTML 易抄错） | §6.2/§6.3 锚定 `baseline.json`（Go 166 / 前端 57 / 合计 223）为唯一真相源；AC-6/AC-12b grep 校验。设计**不写死**数字，Developer 实现时读 `baseline.json`。 |
| R-7 | 跨平台：PowerShell 写 e2e server TOML 时 BOM / 反斜杠路径问题（insight 2026-05-19） | 本任务**不新增** TOML 写入逻辑——`start-e2e-server.ps1` 已有的写法保持不动，仅改其中 `UIPort` 字面量数字。若 Developer 选择补 `UIBindAddr` 行，须沿用脚本现有的写文件方式，不引入新写法。NF-4 要求 verify_all 在双 shell PASS。 |
| R-8 | architecture.html 无现有日期元素，FR-3b.4 / AC-12b 的"头部更新日期"无锚点 | §6.3 已明确：Developer 须**新增**一行更新日期元素（仿 project-status.html `.meta` 风格），不是改既有日期。 |

---

## 9. 迁移 / 兼容性

- **无数据库迁移**：本任务不碰 schema、不碰 `migrations/`。
- **配置文件兼容（NF-2，关键）**：
  - 老用户已有 `frp_easy.toml` 且**显式**写了 `UIPort = 8080` / `UIBindAddr = "127.0.0.1"` → `Load()` 的回填条件 `== 0` / `== ""` 不成立 → 用户值保留不变。升级 frp-easy 二进制后行为不变，用户**不会**感知端口/绑定变化。
  - 老用户已有 `frp_easy.toml` 但**缺**某字段 → 该字段回填为新默认（7800 / 0.0.0.0）。
  - 全新环境无 `frp_easy.toml` → 写入新默认值文件。
- **文档须说明（FR-5.4，README + DEPLOYMENT「升级」章节）**：升级老用户默认值不变——只有新建配置或缺字段才用新默认；想用新默认的老用户须手动改自己的 `frp_easy.toml` 或删除该文件让其重建。
- **无 feature flag**：默认值变更是确定性的，无需开关。
- **回滚**：若需回滚，`git revert` 本任务 commit 即可——纯文本/字面量改动，无状态迁移、无不可逆操作。
- **commit（FR-6）**：所有改动由流水线 commit，message 遵循 `00-core.md`（祈使语气、首行 ≤72 字符、正文解释 why、标注 `T-011`）。建议按逻辑分组 commit（如：① 网络默认值代码+测试 ② 文档刷新），或单 commit 一并提交——由 Developer/PM 决定，本设计不强制。

---

## 10. 给 Developer 的实现注意事项

1. **先清前端 .js 残留**（insight 2026-05-19，evidence T-009/T-010）：本任务改 `web/vite.config.ts` / `web/playwright.config.ts` 是 `.ts`，理论上不受 `.js` 残留影响，但开始前仍建议 `find web/src -type f \( -name '*.js' -o -name '*.js.map' \) -delete` 保证干净。
2. **改 config.go 后必同步 config_test.go**：`TestDoc_PortTablePresent`（行 108）的端口集合断言 `{"8080",...}` 与 doc-comment 端口表绑定，一改一改，否则单测红。
3. **新增 AC-20 测试**：按 §2.2 新增 `TestLoad_ExplicitLoopbackNotOverwritten`，满足 NF-1「测试数只升不降」并直接覆盖 AC-20。完成后 `baseline.json` 的 `go_tests` 须 +1（166→167），`test_count` 同步 +1——Developer 须更新 `baseline.json` 并在 `06_TEST_REPORT` 说明。
4. **FRP 代理端口禁改清单**（R-1）：`internal/storage/storage_test.go`、`internal/storage/qa_t007_adversarial_test.go`、`internal/httpapi/qa_ac_test.go`、`web/src/components/__tests__/qa_t007_adversarial.spec.ts`、`web/src/components/__tests__/ProxyForm.spec.ts` 中的 `8080` 是 `remotePort` / `LocalPort` 业务夹具，**一个都不许动**。改动前对照本清单。
5. **go:embed 重建**（insight 2026-05-17）：改了 `web/` 下文件后，若要本地跑 E2E，需重新 `npm run build` 让 `internal/assets/dist/` 快照更新（`start-e2e-server` 脚本的时间戳检查会驱动重建）。本任务实际只改 vite/playwright config，不改 `web/src/`，dist 内容不变——但 verify_all 流程仍会触发构建，正常。
6. **文案语义区分**（R-4）：DEPLOYMENT.md / package 脚本里的 `127.0.0.1:8080` 有三种语义——浏览器访问 URL（→ `127.0.0.1:7800`）、监听地址描述（→ `0.0.0.0:7800`）、诊断命令端口（→ `7800`）。不要无脑全局替换，逐处按语义判断。
7. **文档数字读 baseline.json**：project-status.html / architecture.html 的测试数字以 `scripts/baseline.json` 实时值为准（注意第 3 点会让 go_tests 变 167）。verify_all PASS 项数以实际运行结果为准（需求写 19，Developer 须实跑确认）。
8. **架构 HTML 补日期元素**（R-8）：architecture.html 当前无更新日期，须新增。
9. **grep 兜底命令**（NF-5 / AC-10，完成后自检）：
   ```bash
   # 应只剩 FR-4.4 枚举的 FRP 代理端口夹具命中
   grep -rn "8080" --include="*.go" --include="*.ts" --include="*.md" \
     --include="*.html" --include="*.yaml" --include="*.sh" --include="*.ps1" \
     . | grep -v "docs/features/_archived/"
   ```
   PowerShell 下用 `Select-String -Path ... -Pattern 8080`。预期命中仅：`storage_test.go` / `qa_t007_adversarial_test.go` / `qa_ac_test.go` / `qa_t007_adversarial.spec.ts` / `ProxyForm.spec.ts`（FR-4.4 白名单）。其余任何命中都是遗漏。
10. **双 shell 验证**（NF-4 / AC-21）：`scripts/verify_all` 须在 PowerShell 与 Git Bash 下均 PASS，PASS 项数 ≥ 19，测试总数 ≥ 223（新增测试后为 224）。
11. **不创建 LICENSE 文件**（PM 裁决 Q-2）；**不新增 spec 文档**（Q-3）；**不动 `docs/features/_archived/`**。

---

## 11. 单 Developer 分区说明

本项目 `.harness/agents/` 下若存在 `dev-*.md` 分区 agent 则需分区表。本任务改动横跨 Go 代码（`internal/appconf`、`cmd/frp-easy`）、Go 测试、前端配置（`web/*.config.ts`）、shell/ps 脚本、文档（README / HTML / yaml）——五类文件**逻辑高度耦合**（端口数字必须全仓库一致、文档数字依赖代码状态），拆分区只会增加协调成本与不一致风险。**指定单 Developer 分区**承担全部改动，按 §2 清单顺序实现。

实现内部建议顺序（非强制）：
1. `internal/appconf/config.go` + `config_test.go`（含新增测试）— 改默认值，跑 `go test ./internal/appconf/...` 绿。
2. `cmd/frp-easy/main.go` — 安全提示重构 + usage 文本，`go build ./...` 绿。
3. `internal/browseropen/browseropen_test.go` — 测试常量。
4. `web/vite.config.ts` / `web/playwright.config.ts` + `scripts/*` — 前端配置与脚本端口。
5. 文档：README.md / project-status.html / architecture.html / DEPLOYMENT.md / openapi.yaml / dev-map 等。
6. grep 兜底自检 → `verify_all` 双 shell → 更新 `baseline.json`。

---

## 12. AC 覆盖映射（24 条）

| AC | 设计覆盖 |
|---|---|
| AC-1 | §6.1 README 十章节顺序 |
| AC-2 | §6.1 README 重写，URL `127.0.0.1:7800`，无 `8080`；§10.9 grep 兜底 |
| AC-3 | §6.1 功能亮点覆盖矩阵（T-006 后能力各 1 处） |
| AC-4 | §6.1 §6 端口表四行 `7800/7400/7500/7000` |
| AC-4b | §6.1 §10 许可证章节"待维护者确定" + frp 上游 Apache-2.0；§4/§10 不建 LICENSE |
| AC-5 | §6.2 project-status.html 头部更新日期 |
| AC-6 | §6.2 §4 测试基线 Go166/前端57/合计223，锚定 baseline.json |
| AC-7 | §6.2 §2 补 T-005~T-010、§3 补 browseropen/logrotate |
| AC-8 | §6.2 §4 verify_all 19 项 |
| AC-9 | §6.2 纯内联 HTML 无外链 |
| AC-10 | §6.4 + §10.9 grep 兜底，排除 _archived |
| AC-11 | §6.4 openapi.yaml `servers[].url: ...:7800` |
| AC-12 | §7 过时点清单（文件→行号→现状→目标）已留档于本文档 |
| AC-12b | §6.3 architecture.html Go166/前端57、补三模块、补更新日期 |
| AC-12c | §6.3 纯内联无外链 |
| AC-13 | §2.1 config.go Default()=7800；§2.2 config_test.go 全部断言改 7800 |
| AC-14 | §2.1 Default() + Load() 回填均 7800；§11 `go build` 绿 |
| AC-15 | §8 R-1 + §10.4 FRP 代理端口禁改清单；§2 不含这些文件 |
| AC-16 | §2.3 vite/playwright config + §2.4 e2e server 脚本均 7800 |
| AC-17 | §2.1 Default() UIBindAddr=0.0.0.0；§2.2 config_test.go 行18 断言 |
| AC-18 | §3 安全提示重构，三要素文案，回环不打 |
| AC-19 | §3.4 浏览器自动打开 URL 改写逻辑保留，首启开 `127.0.0.1:7800` |
| AC-20 | §2.2 新增 `TestLoad_ExplicitLoopbackNotOverwritten`；§9 NF-2 兼容 |
| AC-21 | §10.10 verify_all 双 shell PASS ≥19，测试 ≥223（新增后 224） |

**24 条 AC 全部有设计覆盖，无遗漏。**

---

## 13. 范围边界（本设计不覆盖）

- 不新增功能模块、不改 API 形状、不改数据库 schema、不改 verify_all 检查项本身。
- 不创建 `LICENSE` 文件（PM 裁决 Q-2）；不新增 `docs/spec/network-defaults.md`（Q-3）。
- 不修改 FRP 业务代理端口 / 测试夹具端口（FR-4.4 / §10.4 禁改清单）。
- 不修改 `docs/features/_archived/` 下任何归档文档。
- 不修改 frpc admin(7400) / frps dashboard(7500) / frps bindPort(7000) 端口。
- 不实拍 / 不引入二进制截图文件。
- `Validate()` 不改（已接受 0.0.0.0）；`Load()` 主流程、`ListenAddr()`、`isAddrInUse`、浏览器打开改写逻辑均不改。

---

## 14. Verdict

**READY**

01 需求 verdict = `READY FOR DESIGN`，无遗留开放问题。本设计已将 7 个 FR 组、5 个 NF、24 条 AC 全部映射到精确到文件/函数/行号的改动清单（§2、§6、§7），安全提示文案给出可直接采用的措辞与触发逻辑（§3），复用审计非空（§4），风险均带缓解（§8），兼容性与 commit 计划明确（§9）。无新增依赖、无新模块。单 Developer 分区。

下一步：Gate Reviewer 评审本设计；APPROVED 后转 Developer 实现（full 模式）。
