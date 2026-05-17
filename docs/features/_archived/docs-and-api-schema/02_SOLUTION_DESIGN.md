# 技术方案 — T-005 docs-and-api-schema

**任务 ID**：T-005
**Slug**：docs-and-api-schema
**方案日期**：2026-05-16
**上游需求**：`docs/features/docs-and-api-schema/01_REQUIREMENT_ANALYSIS.md`（Verdict: READY）

---

## 1. 架构摘要

T-005 是纯文档 / 配置层变更，不触及任何 Go 或 Vue 生产代码。变更分三条独立轨：（A）在两个已有文档文件中更新文字和数字以反映 T-004 的交付结果；（B）在两个已有脚本文件中把 D.1 的前置条件判断替换为检测 `go.mod`，使当前 Go 项目结构能触发 PASS/WARN 而不是永久 SKIP；（C）在项目根目录新建静态 `openapi.yaml`，覆盖 `internal/httpapi/router.go` 中的全部 28 条路由，字段名和类型直接取自各 `handlers_*.go` 文件中已声明的 Go 结构体。三条轨没有相互依赖，可并行，但同一 Developer 实施时按 A → B → C 顺序最为安全（C 依赖 B：openapi.yaml 创建后 B 的修改才能使 D.1 输出 PASS）。

---

## 2. 受影响模块

| 文件 | 类型 | 变更性质 |
|---|---|---|
| `README.md` | 文档 | 编辑（"技术债与优化建议"章节改写） |
| `docs/project-status.html` | 文档 | 编辑（§4 数字、§5 TD 状态、§6 OPT 状态、§7 移除两行） |
| `scripts/verify_all.sh` | Shell 脚本 | 编辑（D.1 块前置条件替换） |
| `scripts/verify_all.ps1` | PowerShell 脚本 | 编辑（D.1 块前置条件替换） |
| `openapi.yaml` | YAML 文档 | 新建（项目根目录） |

**不修改**：`internal/httpapi/` 任何 .go 文件、`web/src/` 任何文件、`migrations/` 任何文件、`go.mod`。

---

## 3. 模块分解

### 子任务 A：文档更新

无新模块。对两个已有文件做局部编辑，详见第 5 节 API 合同 / 实现描述。

### 子任务 B：verify_all 脚本修复

无新模块。对两个已有脚本文件做局部替换，详见第 5 节。

### 子任务 C：openapi.yaml

**新文件**：`openapi.yaml`（项目根目录）

- **职责**：静态描述全部 28 条 REST API 路由的 HTTP 方法、路径、认证要求、请求/响应 schema，供外部工具（Redocly、Swagger Editor、Postman）导入。
- **公开 API**：文件本身即接口，无运行时代码依赖。
- **不引入任何新的 Go 依赖**：纯 YAML 文本，不修改 `go.mod`。

---

## 4. 数据模型变更

**无**。不新建表、不修改 schema、不新增迁移文件。

---

## 5. 实现描述（含 API 合同）

### 5-A：README.md 改写

**定位**：文件末尾的 `## 技术债与优化建议` 章节（第 208-213 行）。

**当前文字**（需替换的完整段落）：

```
项目当前存在若干已知技术债（TD-1 ～ TD-8）和优化建议（OPT-1 ～ OPT-9），涵盖向导路由守卫漏洞、verify_all 前端检查路径、版本注入标准化等议题。

完整清单（含影响级别和优先级）请查阅：[docs/project-status.html](docs/project-status.html)
```

**替换后文字**：

```markdown
**T-004 清偿状态（2026-05-16）**

原有 8 条技术债中，TD-1～TD-7 已由 T-004 全部清偿；TD-8（SQLite 单连接并发限制）保留，
适合当前 MVP 规模（≤200 条代理规则）。

原有 9 条优化建议中，OPT-1～OPT-8 已由 T-004 全部实现；OPT-9（OpenAPI schema）已在
T-005 中完成，`openapi.yaml` 位于项目根目录。

**当前已知技术债（1 条）**

- **TD-8**：SQLite 单连接并发限制（`SetMaxOpenConns(1)` 串行化所有读写）。适合 MVP
  低并发场景，多 Tab 高频轮询时（logs/proc/status 均 2 秒轮询）可能产生队列等待。
  如需更高并发，可迁移至连接池 + WAL 模式。

完整历史清单（含影响级别和优先级）请查阅：[docs/project-status.html](docs/project-status.html)
```

**验收检查**：

- `grep -c "TD-1 ～ TD-8" README.md` → 0
- `grep -c "向导路由守卫漏洞" README.md` → 0
- `grep -c "verify_all 前端检查路径" README.md` → 0

---

### 5-B：project-status.html 更新

变更点逐一列出（均为局部文字替换，不改 CSS 和页面结构）：

#### §4 测试基线（`<section id="sec-4">`）

1. **截至提示**：将 `截至 T-002 交付（commit 1138694）：` 改为 `截至 T-004 交付（commit c71077e）：`。

2. **大数字卡片（baseline-grid）**：

   | 卡片 | 旧值 | 新值 |
   |---|---|---|
   | Go 测试 | `117` | `119` |
   | 前端测试 | `45` | `45`（不变） |
   | 合计测试用例 | `162` | `164` |

3. **verify_all 状态表格**：

   | 行 | 旧值 | 新值 | 说明 |
   |---|---|---|---|
   | PASS | badge 内数字 `12` | `16` | T-004 修复 B.1-B.4 |
   | SKIP | badge 内数字 `6` | `2` | B.1-B.4 转 PASS；D.1 仍 SKIP 待 T-005 |
   | SKIP 说明文字 | `B.1–B.4（前端检查）/ C.1（E2E）/ D.1（OpenAPI）—— 见 TD-3` | `C.1（E2E）/ D.1（OpenAPI schema，T-005 完成后消除）` | |

4. **版本节点历史表格**：在 `T-002 结束（当前）` 行之后追加：

   ```html
   <tr><td>T-004 结束（当前）</td><td>119</td><td>45</td><td>164</td></tr>
   <tr><td>增量（T-002→T-004）</td><td>+2</td><td>0</td><td>+2</td></tr>
   ```

   同时把 `T-002 结束（当前）` 改为 `T-002 结束`。

#### §5 技术债清单（`<section id="sec-5">`）

1. **说明文字**：将 `以下为已确认的 8 条技术债，本版本仅文档化，不修复。` 改为 `以下为已确认的 8 条技术债；TD-1～TD-7 已由 T-004 清偿，TD-8 保留。`

2. **TD-1～TD-7 每行**：在 `<td>来源…</td>` 后追加一列（或在描述列末尾追加），标注 `已修复` 状态。建议在每行最右侧 `<td>` 内增加：

   ```html
   <span class="badge badge-green">已修复</span>（T-004）
   ```

   注意：表格当前没有"状态"列，可以把修复标记直接追加到"来源"列的内容后面，如：
   ```html
   <td>T-002 delivery 已知问题 <span class="badge badge-green">已修复</span>（T-004）</td>
   ```

3. **TD-8 行**：保持原样，不追加修复标记。

**验收**：`grep -c "已修复" docs/project-status.html` ≥ 7。

#### §6 优化建议清单（`<section id="sec-6">`）

1. **说明文字**：将 `本版本仅文档化，不实现。` 改为 `OPT-1～OPT-8 已由 T-004 实现；OPT-9 已由 T-005 实现。`

2. **OPT-1～OPT-8 每行**：在"说明"列末尾追加：
   ```html
   <span class="badge badge-green">已实现</span>（T-004）
   ```

3. **OPT-9 行**：追加：
   ```html
   <span class="badge badge-green">已实现</span>（T-005）
   ```

   （T-005 完成后才能追加；如果在 T-005 开发阶段写 HTML，先标记"处理中"或推迟到 QA 验收后更新。需求分析 §4 boundary conditions 允许 QA 在 06_TEST_REPORT.md 中记录，不要求在本任务中预填。设计决策：OPT-9 状态标记留给 Developer 在 T-005 验收通过后同步更新。）

#### §7 已知后续事项（`<section id="sec-7">`）

1. 移除整个 `<tr>` 行（含两个 `<td>`）：向导路由守卫漏洞（OPT-2）。
2. 移除整个 `<tr>` 行：ParseIPFromJSON 重复（OPT-6）。

移除后 §7 表格仅剩 0 行（tbody 为空，表格仍保留 thead）。可选：在 `<p>` 提示文字改为 `T-004 已清偿全部已知后续事项。`

**验收**：`grep -c "ParseIPFromJSON" docs/project-status.html` → 0。

---

### 5-C：verify_all D.1 条件替换

#### verify_all.sh（第 184-191 行）

**当前代码**：

```bash
# --- D. Schema (require source code) ---
if [[ ! -d src && ! -d apps && ! -d packages ]]; then
    step "D.1" "OpenAPI / tRPC schema present" "SKIP"
elif [[ -f openapi.yaml || -f openapi.json || -d src/server/trpc ]]; then
    step "D.1" "OpenAPI / tRPC schema present" "PASS"
else
    step "D.1" "OpenAPI / tRPC schema present" "WARN" "no API schema found"
fi
```

**替换后代码**：

```bash
# --- D. Schema (require source code) ---
# 前置条件改为检测 go.mod：本项目为 Go 项目，无 src/apps/packages 目录，
# 原条件导致 D.1 永久 SKIP（TD-3）；以 go.mod 存在作为"已有源码"判据。
if [[ ! -f go.mod ]]; then
    step "D.1" "OpenAPI / tRPC schema present" "SKIP"
elif [[ -f openapi.yaml || -f openapi.json ]]; then
    step "D.1" "OpenAPI / tRPC schema present" "PASS"
else
    step "D.1" "OpenAPI / tRPC schema present" "WARN" "no API schema found"
fi
```

**注意**：移除了对 `src/server/trpc` 目录的检测（本项目不使用 tRPC），PASS 条件只保留文件存在性检查。

#### verify_all.ps1（第 165-171 行）

**当前代码**：

```powershell
# --- D. Schema / contract (require source code) ---
Step "D.1" "OpenAPI / tRPC schema present" {
    # SKIP if no source code yet (empty project just initialized)
    if (-not ((Test-Path "src") -or (Test-Path "apps") -or (Test-Path "packages"))) { return "SKIP" }
    $found = (Test-Path "openapi.yaml") -or (Test-Path "openapi.json") -or (Test-Path "src/server/trpc")
    if (-not $found) { return $false } # WARN, not FAIL
}
```

**替换后代码**：

```powershell
# --- D. Schema / contract (require source code) ---
Step "D.1" "OpenAPI / tRPC schema present" {
    # 前置条件改为检测 go.mod：本项目为 Go 项目，无 src/apps/packages 目录，
    # 原条件导致 D.1 永久 SKIP（TD-3）；以 go.mod 存在作为"已有源码"判据。
    if (-not (Test-Path "go.mod")) { return "SKIP" }
    $found = (Test-Path "openapi.yaml") -or (Test-Path "openapi.json")
    if (-not $found) { return $false } # WARN, not FAIL
}
```

**行为验证**（需求 B-2、B-3）：

| 场景 | go.mod 存在 | openapi.yaml 存在 | D.1 输出 |
|---|---|---|---|
| 正常运行（T-005 后） | YES | YES | PASS |
| openapi.yaml 被移除 | YES | NO | WARN |
| 非 Go 项目（无 go.mod） | NO | any | SKIP |

---

### 5-D：openapi.yaml 完整内容

以下为 Developer 应当在项目根目录直接写入的完整文件内容。字段名和类型均来自对 `internal/httpapi/handlers_*.go` 的逐文件核对，不得修改 Go 代码以迁就 schema。

```yaml
openapi: "3.0.3"
info:
  title: frp_easy REST API
  version: "0.1.0"
  description: |
    frp_easy Web UI 的 REST API。
    认证方式：Cookie（frp_easy_sid）。
    写操作（POST/PUT/DELETE，在受保护分组中）额外需要 X-CSRF-Token 请求头，
    该令牌由 GET /api/v1/auth/csrf 获取。

servers:
  - url: http://127.0.0.1:8080
    description: 默认本地服务器

security:
  - cookieAuth: []

components:
  securitySchemes:
    cookieAuth:
      type: apiKey
      in: cookie
      name: frp_easy_sid
      description: 登录后由服务端写入的 session cookie
    csrfToken:
      type: apiKey
      in: header
      name: X-CSRF-Token
      description: 由 GET /api/v1/auth/csrf 获取，写操作必须携带

  schemas:

    ErrorDetail:
      type: object
      required: [code, message]
      properties:
        code:
          type: string
          description: 错误码，见 internal/httpapi/errors.go
          example: VALIDATION_FAILED
        message:
          type: string
          description: 人类可读的错误描述
        field:
          type: string
          description: 出错的字段名（可选）

    ErrorBody:
      type: object
      required: [error]
      properties:
        error:
          $ref: '#/components/schemas/ErrorDetail'

    OkResponse:
      type: object
      required: [ok]
      properties:
        ok:
          type: boolean

    HealthResponse:
      type: object
      required: [status, version]
      properties:
        status:
          type: string
          example: ok
        version:
          type: string
          example: "0.1.0"

    SystemReady:
      type: object
      required: [initialized, binMissing, version]
      properties:
        initialized:
          type: boolean
          description: 管理员账号是否已设置
        binMissing:
          type: array
          items:
            type: string
          description: 缺失的 frp 二进制列表（空表示全部就绪）
        version:
          type: string
          example: "0.1.0"

    SetupRequest:
      type: object
      required: [username, password]
      properties:
        username:
          type: string
        password:
          type: string

    LoginRequest:
      type: object
      required: [username, password]
      properties:
        username:
          type: string
        password:
          type: string

    LoginResponse:
      type: object
      required: [ok]
      properties:
        ok:
          type: boolean

    MeResponse:
      type: object
      required: [username]
      properties:
        username:
          type: string

    CSRFResponse:
      type: object
      required: [csrfToken]
      properties:
        csrfToken:
          type: string

    ChangePasswordRequest:
      type: object
      required: [oldPassword, newPassword]
      properties:
        oldPassword:
          type: string
        newPassword:
          type: string

    ModeState:
      type: object
      required: [frpc, frps]
      properties:
        frpc:
          type: boolean
          description: frpc 进程是否应处于运行状态
        frps:
          type: boolean
          description: frps 进程是否应处于运行状态

    ProxyInput:
      type: object
      required: [name, type, localPort]
      properties:
        name:
          type: string
        type:
          type: string
          description: "tcp | udp | http | https"
        localIP:
          type: string
          description: 本地监听 IP，默认 127.0.0.1
        localPort:
          type: integer
        remotePort:
          type: integer
          description: tcp/udp 必填；http/https 禁填
        customDomains:
          type: array
          items:
            type: string
          description: http/https 必填（≥1 项）；tcp/udp 禁填
        enabled:
          type: boolean
          description: 默认 true
        version:
          type: integer
          format: int64
          description: PUT 时必填（乐观锁）

    ProxyResponse:
      type: object
      required: [id, name, type, localIP, localPort, enabled, version, updatedAt]
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
        type:
          type: string
        localIP:
          type: string
        localPort:
          type: integer
        remotePort:
          type: integer
          description: tcp/udp 类型才有此字段
        customDomains:
          type: array
          items:
            type: string
          description: http/https 类型才有此字段
        enabled:
          type: boolean
        version:
          type: integer
          format: int64
        updatedAt:
          type: string
          format: date-time
          example: "2026-05-16T10:00:00Z"

    FrpsConfig:
      type: object
      required: [bindPort]
      properties:
        bindPort:
          type: integer
          description: frps 监听端口，默认 7000
        authMethod:
          type: string
          description: "token | oidc | 空字符串"
        authToken:
          type: string
          description: GET 时脱敏为 ***（除非 ?reveal=1）
        dashboardEnabled:
          type: boolean
        dashboardAddr:
          type: string
        dashboardPort:
          type: integer
        dashboardUser:
          type: string
        dashboardPass:
          type: string

    FrpcServerConn:
      type: object
      required: [serverAddr, serverPort]
      properties:
        serverAddr:
          type: string
          description: frps 服务器地址
        serverPort:
          type: integer
          description: frps 服务器端口
        authMethod:
          type: string
        authToken:
          type: string
          description: GET 时脱敏为 ***（除非 ?reveal=1）

    ProcessInfo:
      type: object
      required: [kind, state, pid, changedAt]
      properties:
        kind:
          type: string
          description: "frpc | frps"
        state:
          type: string
          enum: [stopped, starting, running, stopping, error]
        pid:
          type: integer
          description: 进程 PID，未运行时为 0
        lastErr:
          type: string
          description: 最后一次错误信息（可选）
        changedAt:
          type: string
          format: date-time

    ProcStatusResponse:
      type: object
      required: [frpc, frps]
      properties:
        frpc:
          $ref: '#/components/schemas/ProcessInfo'
        frps:
          $ref: '#/components/schemas/ProcessInfo'

    LogsTailResponse:
      type: object
      required: [lines]
      properties:
        lines:
          type: array
          items:
            type: string
          description: 尾部 N 行日志（默认 500，最多 5000）

    LogsIncrementalResponse:
      type: object
      required: [data, nextOffset]
      properties:
        data:
          type: string
          description: 自上次 offset 起的新增日志文本
        nextOffset:
          type: integer
          format: int64
          description: 下次轮询使用的 offset 值

    PublicIPResponse:
      type: object
      properties:
        ip:
          type: string
          description: 检测到的公网 IP（成功时）
        error:
          type: string
          description: 错误描述（超时时）
        advisory:
          type: string
          description: IPv6 使用提示（IP 为 IPv6 时）

    DownloadBinRequest:
      type: object
      required: [kind]
      properties:
        kind:
          type: string
          description: "frpc | frps"

    DownloadState:
      type: object
      required: [status, progress]
      properties:
        status:
          type: string
          description: "idle | downloading | done | error"
        progress:
          type: integer
          description: 下载进度 0-100
        error:
          type: string
          description: 错误信息（status=error 时）

    WizardStatus:
      type: object
      required: [handled, shouldShow]
      properties:
        handled:
          type: boolean
          description: wizard 是否已被处理（完成或跳过）
        shouldShow:
          type: boolean
          description: 是否应当显示向导（!handled && !hasAnyConfig）

paths:

  /api/v1/health:
    get:
      summary: 存活探针（轻量，不经过 ReadyGate）
      operationId: getHealth
      security: []
      tags: [system]
      responses:
        '200':
          description: 服务存活
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HealthResponse'

  /api/v1/system/ready:
    get:
      summary: 就绪检查（返回初始化状态和缺失二进制列表）
      operationId: getSystemReady
      security: []
      tags: [system]
      responses:
        '200':
          description: 就绪状态
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/SystemReady'
        '503':
          description: 服务未就绪（ReadyGate 拦截）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/setup:
    post:
      summary: 初始化管理员账号（只能调用一次）
      operationId: setup
      security: []
      tags: [auth]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/SetupRequest'
      responses:
        '200':
          description: 初始化成功，同时写入 session cookie
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OkResponse'
        '400':
          description: 请求体非法 JSON
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '409':
          description: 已初始化（ALREADY_INITIALIZED）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: 用户名或密码校验失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/auth/login:
    post:
      summary: 登录
      operationId: login
      security: []
      tags: [auth]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/LoginRequest'
      responses:
        '200':
          description: 登录成功，写入 frp_easy_sid cookie
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LoginResponse'
        '400':
          description: 请求体非法 JSON
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 用户名或密码错误，或未初始化
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '429':
          description: 登录尝试过多（RATE_LIMITED），响应头含 Retry-After
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/auth/logout:
    post:
      summary: 登出（清除 session）
      operationId: logout
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [auth]
      responses:
        '200':
          description: 登出成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OkResponse'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/auth/password:
    post:
      summary: 修改密码
      operationId: changePassword
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [auth]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ChangePasswordRequest'
      responses:
        '200':
          description: 修改成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OkResponse'
        '400':
          description: 请求体非法 JSON
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录或旧密码错误
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: 新密码不符合规则
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/auth/me:
    get:
      summary: 获取当前登录用户信息
      operationId: getMe
      tags: [auth]
      responses:
        '200':
          description: 当前用户
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MeResponse'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/auth/csrf:
    get:
      summary: 获取 CSRF token（同时设置 X-CSRF-Token 响应头）
      operationId: getCSRF
      tags: [auth]
      responses:
        '200':
          description: CSRF token
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CSRFResponse'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/mode:
    get:
      summary: 获取 frpc/frps 模式开关状态
      operationId: getMode
      tags: [mode]
      responses:
        '200':
          description: 模式状态
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ModeState'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
    put:
      summary: 设置 frpc/frps 模式开关（变更后立即启停进程）
      operationId: putMode
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [mode]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ModeState'
      responses:
        '200':
          description: 更新后的模式状态
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ModeState'
        '400':
          description: 请求体非法 JSON
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/proxies:
    get:
      summary: 获取全部代理规则列表
      operationId: listProxies
      tags: [proxies]
      responses:
        '200':
          description: 代理规则数组
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/ProxyResponse'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
    post:
      summary: 创建代理规则
      operationId: createProxy
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [proxies]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ProxyInput'
      responses:
        '201':
          description: 创建成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ProxyResponse'
        '400':
          description: 请求体非法 JSON
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: 校验失败（字段不合法、达到 200 条上限等）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/proxies/{id}:
    put:
      summary: 更新代理规则（乐观锁，需携带 version 字段）
      operationId: updateProxy
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [proxies]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
            format: int64
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ProxyInput'
      responses:
        '200':
          description: 更新后的代理规则
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ProxyResponse'
        '400':
          description: id 非法或请求体非法 JSON
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '404':
          description: 规则不存在
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '409':
          description: 版本冲突（CONFLICT）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: 校验失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
    delete:
      summary: 删除代理规则
      operationId: deleteProxy
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [proxies]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
            format: int64
      responses:
        '200':
          description: 删除成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OkResponse'
        '400':
          description: id 非法
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '404':
          description: 规则不存在
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/server:
    get:
      summary: 获取 frps 服务端配置（token 默认脱敏）
      operationId: getServer
      tags: [config]
      parameters:
        - name: reveal
          in: query
          required: false
          schema:
            type: string
            enum: ["1"]
          description: 传 reveal=1 可获取原始 token（不脱敏）
      responses:
        '200':
          description: frps 配置
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FrpsConfig'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
    put:
      summary: 保存 frps 服务端配置
      operationId: putServer
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [config]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/FrpsConfig'
      responses:
        '200':
          description: 保存后的配置（token 脱敏）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FrpsConfig'
        '400':
          description: 请求体非法 JSON
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: 端口号非法
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/client:
    get:
      summary: 获取 frpc 客户端连接配置（token 默认脱敏）
      operationId: getClient
      tags: [config]
      parameters:
        - name: reveal
          in: query
          required: false
          schema:
            type: string
            enum: ["1"]
          description: 传 reveal=1 可获取原始 token
      responses:
        '200':
          description: frpc 客户端配置
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FrpcServerConn'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
    put:
      summary: 保存 frpc 客户端连接配置
      operationId: putClient
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [config]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/FrpcServerConn'
      responses:
        '200':
          description: 保存后的配置（token 脱敏）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FrpcServerConn'
        '400':
          description: 请求体非法 JSON
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: serverAddr 为空或 serverPort 非法
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/proc/{kind}/start:
    post:
      summary: 启动 frpc 或 frps 进程
      operationId: procStart
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [proc]
      parameters:
        - name: kind
          in: path
          required: true
          schema:
            type: string
            enum: [frpc, frps]
      responses:
        '200':
          description: 启动后的进程信息
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ProcessInfo'
        '400':
          description: kind 非法
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '409':
          description: 进程已在运行（PROC_BUSY）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: 二进制缺失（BIN_MISSING）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/proc/{kind}/stop:
    post:
      summary: 停止 frpc 或 frps 进程
      operationId: procStop
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [proc]
      parameters:
        - name: kind
          in: path
          required: true
          schema:
            type: string
            enum: [frpc, frps]
      responses:
        '200':
          description: 停止后的进程信息
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ProcessInfo'
        '400':
          description: kind 非法
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '500':
          description: 内部错误
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/proc/{kind}/restart:
    post:
      summary: 重启 frpc 或 frps 进程
      operationId: procRestart
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [proc]
      parameters:
        - name: kind
          in: path
          required: true
          schema:
            type: string
            enum: [frpc, frps]
      responses:
        '200':
          description: 重启后的进程信息
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ProcessInfo'
        '400':
          description: kind 非法
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '409':
          description: 进程正在启停（PROC_BUSY）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: 二进制缺失（BIN_MISSING）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/proc/status:
    get:
      summary: 获取 frpc 和 frps 进程状态快照
      operationId: procStatus
      tags: [proc]
      responses:
        '200':
          description: 两个进程的状态
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ProcStatusResponse'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/logs/{kind}:
    get:
      summary: 读取 frpc 或 frps 日志
      operationId: getLogs
      tags: [logs]
      parameters:
        - name: kind
          in: path
          required: true
          schema:
            type: string
            enum: [frpc, frps]
        - name: lines
          in: query
          required: false
          schema:
            type: integer
            default: 500
            minimum: 1
            maximum: 5000
          description: 返回末尾 N 行（不与 offset 同用）
        - name: offset
          in: query
          required: false
          schema:
            type: integer
            format: int64
          description: 增量读取起始字节偏移；提供时忽略 lines 参数，返回增量格式响应
      responses:
        '200':
          description: |
            不携带 offset 时返回 LogsTailResponse；
            携带 offset 时返回 LogsIncrementalResponse。
          content:
            application/json:
              schema:
                oneOf:
                  - $ref: '#/components/schemas/LogsTailResponse'
                  - $ref: '#/components/schemas/LogsIncrementalResponse'
        '400':
          description: kind 非法
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '404':
          description: 未配置日志路径
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/system/public-ip:
    get:
      summary: 检测当前出口公网 IP（最多等待 3 秒，始终返回 200）
      operationId: getPublicIP
      tags: [system]
      responses:
        '200':
          description: 公网 IP 或超时错误（HTTP 状态码始终为 200）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PublicIPResponse'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/system/download-bin:
    post:
      summary: 触发异步下载 frpc 或 frps 二进制
      operationId: downloadBin
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [system]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DownloadBinRequest'
      responses:
        '202':
          description: 下载已开始（异步）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OkResponse'
        '400':
          description: 请求体非法 JSON
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '409':
          description: 下载已在进行中（PROC_BUSY）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: kind 非法（必须为 frpc 或 frps）
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '503':
          description: 下载器未初始化或不支持当前 OS
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/system/download-status/{kind}:
    get:
      summary: 查询 frpc 或 frps 下载进度
      operationId: getDownloadStatus
      tags: [system]
      parameters:
        - name: kind
          in: path
          required: true
          schema:
            type: string
            enum: [frpc, frps]
      responses:
        '200':
          description: 下载状态
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DownloadState'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '404':
          description: kind 非法
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/wizard/status:
    get:
      summary: 查询部署向导状态
      operationId: getWizardStatus
      tags: [wizard]
      responses:
        '200':
          description: 向导状态
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/WizardStatus'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'

  /api/v1/wizard/complete:
    post:
      summary: 标记向导已完成或跳过
      operationId: wizardComplete
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [wizard]
      responses:
        '200':
          description: 标记成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OkResponse'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '500':
          description: 内部存储错误
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
```

**路由计数验证**（Developer 实施后自查）：

| HTTP 方法 | 路径数 | 路由数 |
|---|---|---|
| GET | 15 路径 | 15 |
| POST | 8 路径 | 8 |
| PUT | 4 路径 | 4 |
| DELETE | 1 路径 | 1 |
| **合计** | **24 唯一路径** | **28 路由** |

---

## 6. 请求流程

```
浏览器/工具客户端
│
├─ GET /api/v1/health          → 直通 handler（无任何中间件）→ HealthResponse
│
├─ GET /api/v1/system/ready    → ReadyGate → handler → SystemReady
├─ POST /api/v1/setup          → ReadyGate → handler（公开）→ SetupResponse
├─ POST /api/v1/auth/login     → ReadyGate → RateLimiter → handler（公开）→ LoginResponse
│
└─ [受保护端点]
   → ReadyGate → Recover → RequestID → Logger → CORS
   → SessionAuth（验证 frp_easy_sid cookie）
   → CSRF（写方法验证 X-CSRF-Token 头）
   → handler → 业务响应

verify_all D.1 逻辑（修复后）：
  go.mod 不存在 → SKIP
  go.mod 存在 && openapi.yaml 存在 → PASS
  go.mod 存在 && openapi.yaml 不存在 → WARN（exit 1）
```

---

## 7. 复用审计

| 需求 | 现有代码 | 文件路径 | 决策 |
|---|---|---|---|
| API 错误响应结构 | `ErrorBody`, `ErrorDetail` | `internal/httpapi/errors.go` | openapi.yaml 中 ErrorBody/ErrorDetail schema 与之严格对齐 |
| 认证请求/响应类型 | `LoginRequest`, `MeResponse`, `CSRFResponse`, `ChangePasswordRequest` | `internal/httpapi/handlers_auth.go` | 直接映射为 openapi.yaml schema |
| 代理规则类型 | `ProxyInput`, `ProxyResponse` | `internal/httpapi/handlers_proxies.go` | 直接映射 |
| 服务端/客户端配置类型 | `FrpsConfig`, `FrpcServerConn` | `internal/httpapi/handlers_server.go` | 直接映射 |
| 进程状态类型 | `ProcessInfo`, State 枚举常量 | `internal/procmgr/manager.go` | 枚举值：stopped/starting/running/stopping/error |
| 日志响应类型 | `LogsTailResponse`, `LogsIncrementalResponse` | `internal/httpapi/handlers_logs.go` | 直接映射，用 oneOf |
| 公网 IP 响应 | `PublicIPResponse` | `internal/httpapi/handlers_system.go` | 直接映射 |
| 下载状态类型 | `DownloadState` | `internal/downloader/downloader.go` | 直接映射 |
| 向导状态类型 | `WizardStatus` | `internal/httpapi/handlers_wizard.go` | 直接映射 |
| 系统就绪类型 | `SystemReady` | `internal/httpapi/handlers_system.go` | 直接映射 |
| 下载请求类型 | `DownloadBinRequest` | `internal/httpapi/handlers_system.go` | 直接映射 |
| 模式类型 | `ModeState` | `internal/httpapi/handlers_mode.go` | 直接映射 |
| 路由权威源 | `New()` 函数 | `internal/httpapi/router.go` | 路由清单、认证分组均以此为准 |
| D.1 PASS 检测路径 | `[[ -f openapi.yaml ]]` | `scripts/verify_all.sh` 第 187 行 | 文件放根目录保持与现有脚本一致 |

---

## 8. 风险分析

| 风险 | 可能性 | 影响 | 缓解措施 |
|---|---|---|---|
| **R-1：openapi.yaml YAML 语法错误导致 Redocly lint 失败** | 中（手写 YAML 易出缩进错误） | AC-C3 不通过 | Developer 在提交前本地运行 `npx @redocly/cli lint openapi.yaml`；设计文档提供完整 YAML 减少手写出错 |
| **R-2：路由数量/方法与 router.go 不一致** | 低（路由清单已在需求中固定） | AC-C2 不通过 | Developer 对照 `internal/httpapi/router.go` 第 65-116 行逐条核对；本设计文档提供的路由计数表可用于自查 |
| **R-3：verify_all.ps1 语法在 PowerShell 5.1 下行为异常** | 低（改动极小，只替换三行） | AC-B4 不通过 | 替换后用 `pwsh scripts/verify_all.ps1 --quick` 验证；改动不引入新 PS 语法特性 |
| **R-4：project-status.html 编辑破坏 HTML 结构** | 低（只追加标记和数字） | 页面渲染异常 | 编辑后在浏览器打开目测；用 `grep -c "已修复"` 验证追加数量 |
| **R-5：openapi.yaml 中 oneOf（logs 端点）被 Redocly 报 WARN 以上** | 低 | AC-C3 可能不通过 | Redocly 3.x 支持 `oneOf`；若有问题，可退为仅定义 tail 模式响应并在 description 中注释 incremental 模式 |
| **R-6：test count 数字填写错误（不是 119）** | 低（数字明确来自需求） | AC-A5 不通过 | 直接搜索替换 `117` → `119`，确认文件中 `162` → `164` |

---

## 9. 迁移 / 上线计划

所有变更均为向后兼容，无需数据迁移。

**顺序**：

1. 实施子任务 B（verify_all 脚本修复）。
2. 实施子任务 C（创建 openapi.yaml）。
3. 运行 `bash scripts/verify_all.sh --quick` 确认 D.1 输出 PASS。
4. 实施子任务 A（README.md 和 project-status.html 文档更新）。
5. 运行全部 AC 验收命令（见需求 §6）。

**回滚**：任一文件的 git diff 均可用 `git checkout -- <file>` 还原，不影响运行中二进制。

**功能开关**：无需特性标志，openapi.yaml 是静态文件，不影响服务启动。

---

## 10. 不在本设计范围内的说明

与需求 §4 Out-of-scope 对应：

- 不修改任何 Go handler 文件（`internal/httpapi/handlers_*.go`、`router.go`）。
- 不修改 Vue 前端代码（`web/src/`）。
- 不修改数据库迁移文件（`migrations/`）。
- 不添加 Swagger UI 服务端点（无 `/docs` 或 `/swagger` 路由）。
- 不修复 TD-8（SQLite 单连接），仅文档化现状。
- 不处理 T-006 E2E 测试。
- 不在 Go 二进制启动时验证 openapi.yaml 是否存在或合法（运行时完全不感知该文件）。
- 不为 openapi.yaml 字段添加枚举约束、长度限制或 example 示例（完整级别 C，本次只做标准级别 B）。
- `openapi.yaml` 版本号锁定为 `"3.0.3"`（不使用 3.1.0）：3.0.3 对 Redocly CLI 和主流工具兼容性更好，且无需处理 3.1.0 中 `nullable` 语法的迁移差异。

---

## 11. 分区分配

`.harness/agents/dev-backend.md`、`dev-frontend.md`、`dev-db.md` 均存在。T-005 的全部变更文件均属文档/脚本层，无 Vue 代码和数据库变更，统一分配给 `dev-backend`。

| 文件 | 分区 | 新建/编辑 | 说明 |
|---|---|---|---|
| `README.md` | dev-backend | 编辑（改写技术债章节） | 无分区依赖 |
| `docs/project-status.html` | dev-backend | 编辑（§4/§5/§6/§7） | 无分区依赖 |
| `scripts/verify_all.sh` | dev-backend | 编辑（D.1 块替换） | 无分区依赖 |
| `scripts/verify_all.ps1` | dev-backend | 编辑（D.1 块替换） | 无分区依赖 |
| `openapi.yaml` | dev-backend | 新建（项目根目录） | 依赖：verify_all 脚本已修复后方可验证 PASS |

### 派发顺序

1. **dev-backend**（全部子任务）

### 并行性

单分区，无并行。建议实施顺序：verify_all 修复（B）→ openapi.yaml（C）→ 文档（A），便于边改边用 verify_all 验证 D.1。

---

## 12. Verdict

**READY**

所有歧义在需求 v2 中已由 PM 关闭（openapi.yaml 位置 = 根目录，完整度 = 标准级 B）。设计完整，Developer 可直接按本文档实施无需进一步决策。
