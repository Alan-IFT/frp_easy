# T-006 需求分析：Playwright E2E 烟雾测试

**任务 ID**：T-006  
**Slug**：e2e-smoke-tests  
**日期**：2026-05-16  
**阶段**：req

---

## 1. 目标

为 frp_easy 添加 Playwright E2E 烟雾测试，覆盖 4 条关键用户路径（初次设置、登录、仪表盘可见、退出登录），使 `scripts/verify_all.sh` C.1 检查从 SKIP 变为 PASS。

---

## 2. 范围内行为

### 2.1 文件布局

1. `web/playwright.config.ts` 文件存在（verify_all.sh C.1 的触发条件）。
2. 测试文件位于 `web/tests/e2e/` 目录，扩展名为 `.spec.ts`。
3. `web/package.json` 的 `devDependencies` 新增 `@playwright/test`（版本 ^1.44.0 或更新）。
4. `web/package-lock.json` 同步更新，`npm install --frozen-lockfile` 在 `web/` 目录内执行成功。

### 2.2 verify_all.sh 修改

5. `scripts/verify_all.sh` C.1 节的检测条件扩展为同时检查 `web/playwright.config.ts` 和 `web/playwright.config.js`（在现有的根目录检查之外追加）。
6. 当 playwright config 位于 `web/` 时，C.1 节先 `pushd "$ROOT/web"`，在该目录内重新计算包管理器（`pkgmgr`），执行 `$PM exec playwright test --project=chromium`，执行完毕后 `popd`。
7. `scripts/verify_all.sh --quick` 行为不变：C.1 步骤完全跳过，不输出任何结果行。

### 2.3 后端服务器自动启动

8. `web/playwright.config.ts` 通过 `webServer` 配置自动启动后端实例，不需要用户提前手动操作。
9. webServer 启动命令必须使用独立的临时数据目录（不读写开发环境的 `frp_easy.toml` 或项目级 data 目录）。
10. `internal/assets/dist/` 目录不存在时，webServer 命令启动失败，C.1 结果为 FAIL（不 SKIP）；这是期望行为，提示开发者先执行 `npm run build`。
11. `playwright.config.ts` 的 `reuseExistingServer` 字段：CI 环境（`process.env.CI` 为真时）设为 `false`，本地开发时设为 `!process.env.CI`（即 `true`，允许复用已运行的服务器）。
12. webServer `timeout` 不少于 60000 毫秒（60 秒），给 Go 进程启动留足时间。

### 2.4 测试配置

13. `playwright.config.ts` 的 `projects` 数组只包含 `chromium`（与 verify_all.sh C.1 命令一致）。
14. `playwright.config.ts` 的 `workers` 设为 1（顺序执行，避免多 worker 共享同一后端实例导致状态污染）。
15. 测试执行顺序为 TC-01 → TC-02 → TC-03 → TC-04 → TC-05，共享同一后端实例，TC-02 的 setup 调用产生的管理员账号被 TC-03/04/05 复用。

### 2.5 测试场景

#### TC-01：未初始化时自动跳转 /setup

16. 前置条件：后端使用空数据库启动（无管理员账号）。
17. 操作：浏览器访问 `http://localhost:{port}/`。
18. 验证：当前 URL path 等于 `/setup`。

#### TC-02：setup 表单提交成功并离开 /setup

19. 前置条件：同 TC-01（空数据库）。
20. 操作：在 /setup 页面，通过 `placeholder="admin"` 定位用户名输入框，填入 `e2eadmin`；通过 `placeholder="至少12位，含字母和数字"` 定位密码框，填入 `E2eTestPass1!`；通过 `placeholder="再次输入密码"` 定位确认密码框，填入相同值；点击文本为 `完成初始化` 的按钮。
21. 验证：URL path 不再是 `/setup`（允许 `/dashboard` 或 `/wizard`，两者均视为通过）。
22. 验证：页面不包含文本 `初始化失败`。

#### TC-03：login 表单提交成功并离开 /login

23. 前置条件：后端数据库中已存在管理员账号（由 TC-02 创建）；当前未登录（session 未设置）。
24. 操作：访问 `/login`；通过 `placeholder="admin"` 定位用户名框，填入 `e2eadmin`；通过 `placeholder="密码"` 定位密码框，填入 `E2eTestPass1!`；点击文本为 `登录` 的按钮。
25. 验证：URL path 为 `/dashboard` 或 `/wizard`（路由守卫决定，两者均视为通过）。
26. 验证：页面不包含文本 `用户名或密码错误`。

#### TC-04：dashboard 关键元素可见

27. 前置条件：已完成 TC-03 的登录操作，当前处于已登录状态。
28. 前置操作（绕过向导干扰）：若当前 URL path 为 `/wizard`，通过 API 调用完成向导（先 GET `/api/v1/auth/csrf` 获取 CSRF token，再 POST `/api/v1/wizard/complete` 携带 `X-CSRF-Token` 请求头），然后导航至 `/dashboard`。
29. 验证：页面包含文本 `仪表盘`（n-page-header 的 title 属性）。
30. 验证：页面包含文本 `frpc（客户端）`（第一张进程卡片标题）。
31. 验证：页面包含文本 `frps（服务端）`（第二张进程卡片标题）。

#### TC-05：退出登录跳转 /login

32. 前置条件：已登录并处于 `/dashboard` 页面（TC-04 完成后的状态）。
33. 操作：点击文本为 `退出登录` 的按钮（位于 AppLayout header，n-button）。
34. 验证：URL path 变为 `/login`。
35. 验证：在当前 page 上访问 `/dashboard`，URL 重定向至 `/login`（证明 session 已清除）。

---

## 3. 范围外

- 不向任何 Vue 组件添加 `data-testid` 属性（使用现有 placeholder 文本和按钮文字作为选择器）。
- 不测试向导（/wizard）流程内部步骤（T-002 已交付）。
- 不测试 proxy CRUD、日志页面、服务端配置、客户端配置、设置页面。
- 不添加 Firefox 或 WebKit 测试 profile（verify_all.sh 仅使用 `--project=chromium`）。
- 不配置截图、视频录制或 trace 采集（可后续单独任务追加）。
- 不修改任何 Go 源代码。
- 不修改任何 Vue 组件源代码（仅新增测试文件和配置文件）。
- 不测试登录失败、密码不符合规则等错误路径（超出烟雾测试范围）。
- 不添加 Playwright HTML reporter 配置（使用默认输出）。

---

## 4. 边界条件

| 场景 | 期望行为 |
|---|---|
| `internal/assets/dist/` 不存在 | webServer 启动失败，C.1 = FAIL（不 SKIP），提示需先 `npm run build` |
| verify_all.sh `--quick` 标志 | C.1 完全不执行，整体结果不受影响 |
| 测试端口被占且 `reuseExistingServer: false` | Playwright 报错退出，C.1 = FAIL |
| TC-02 之后直接重复执行 POST /api/v1/setup | 后端返回 409（管理员已存在），fixture 必须处理此错误而不中断测试 |
| TC-03 登录后路由跳转 /wizard | TC-04 前置操作通过 wizard/complete API 完成向导，测试继续 |
| CSRF token 获取失败 | TC-04 前置操作失败，测试报错（不静默忽略） |
| webServer 60 秒内未就绪 | Playwright 超时，C.1 = FAIL |
| 多次运行（幂等性） | 每次运行均使用新的临时数据目录（CI），或复用已有服务器（本地，reuseExistingServer=true） |

---

## 5. 验收标准

- **AC-1**：文件 `web/playwright.config.ts` 存在于仓库中（git tracked）。
- **AC-2**：文件 `web/tests/e2e/*.spec.ts` 至少存在 1 个文件，包含 TC-01 至 TC-05 的测试用例。
- **AC-3**：`web/package.json` devDependencies 含 `@playwright/test`；`web/package-lock.json` 与之同步。
- **AC-4**：在预先构建过前端（`internal/assets/dist/` 存在）的环境中，执行 `cd web && npm exec playwright test --project=chromium`，TC-01 至 TC-05 全部通过，退出码为 0。
- **AC-5**：在 `internal/assets/dist/` 不存在时，执行 `scripts/verify_all.sh`，C.1 结果为 FAIL（而非 SKIP）。
- **AC-6**：执行 `scripts/verify_all.sh`（前端已构建），C.1 显示 PASS，整体退出码为 0（无 FAIL、无 WARN）。
- **AC-7**：执行 `scripts/verify_all.sh --quick`，C.1 步骤不出现在输出中，脚本正常完成。
- **AC-8**：执行 `cd web && npm install --frozen-lockfile` 成功（lockfile 已提交且未过期）。

---

## 6. 非功能需求

- **执行时间**：TC-01 至 TC-05 完整执行（含 webServer 启动）不超过 120 秒。
- **CI 兼容**：headless Chromium 模式，不依赖 xvfb 或任何 display server；在 `process.env.CI=true` 环境下不尝试复用已有服务器。
- **数据隔离**：webServer 使用的临时数据目录在测试结束后可被清理，不影响项目根目录下的开发数据。
- **选择器稳定性**：所有 Playwright 选择器使用用户可见的文本（placeholder、按钮文字、标题文字），不使用位置索引或 CSS 类名。

---

## 7. 关联历史任务

| 任务 | 关联点 |
|---|---|
| T-001 web-ui-mvp（`docs/features/_archived/web-ui-mvp/`） | 建立了 Setup/Login/Dashboard 页面实现；本任务在其交付结果上添加 E2E 测试 |
| T-002 zero-config-quickstart（`docs/features/_archived/zero-config-quickstart/`） | 实现了向导流程（/wizard）及路由守卫向导检查；本任务 TC-04 需绕过向导干扰 |
| T-004 tech-debt-cleanup（`docs/features/_archived/tech-debt-cleanup/`） | 修复路由守卫等技术债；本任务路由跳转预期建立在修复后行为之上 |

**关键代码引用（读实现确认选择器）**：
- Setup.vue 表单选择器：placeholder `admin` / `至少12位，含字母和数字` / `再次输入密码`，提交按钮文字 `完成初始化`
- Login.vue 表单选择器：placeholder `admin` / `密码`，提交按钮文字 `登录`
- AppLayout.vue 退出按钮：文字 `退出登录`（`<n-button size="small" @click="handleLogout">退出登录</n-button>`）
- Dashboard.vue 可验证文本：`仪表盘`（n-page-header title）、`frpc（客户端）`、`frps（服务端）`（n-card title）
- router.ts 守卫逻辑：未初始化跳 /setup；已登录访问 /login 或 /setup 跳 /dashboard；向导未完成时访问 /dashboard 跳 /wizard
- handlers_setup.go：POST /api/v1/setup 成功后自动创建 session，返回 `Set-Cookie: frp_easy_sid`；已初始化时返回 409
- router.go：GET /api/v1/auth/csrf 和 POST /api/v1/wizard/complete 均在 SessionAuth 保护下，需要 X-CSRF-Token

---

## 8. 开放问题

无。PM 已授权 Requirement Analyst 关闭全部技术决策点，以下决策已在本文档中锁定：

| 决策点 | 已选方案 |
|---|---|
| playwright.config.ts 位置 | `web/playwright.config.ts` |
| 测试文件目录 | `web/tests/e2e/` |
| verify_all.sh 修改方式 | 扩展 C.1 的文件检测条件 + pushd 到 web/ |
| 后端启动方式 | playwright webServer 配置自动启动 |
| 数据隔离方式 | 独立临时数据目录（环境变量传入） |
| 测试并发模型 | workers=1，顺序执行，单服务器实例 |
| 向导干扰处理 | TC-04 通过 API 调用 wizard/complete 绕过 |
| 浏览器范围 | 仅 Chromium |

---

## 9. 结论

**READY**
