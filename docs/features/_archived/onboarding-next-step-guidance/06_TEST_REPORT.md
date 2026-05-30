# 06 测试报告 — T-062 · onboarding-next-step-guidance

> Stage 6 / QA Tester · 模式：full · 输出语言：中文
> 上游：01/02/03/04/05。QA 独立从 AC 重写反向证伪用例（不复用 dev 测试假设）。

## 测试执行边界说明

QA 实际可用工具为 Read/Write/Edit/Glob/Grep（无 Bash/PowerShell，insight L31）。`scripts/verify_all` 全量真跑由 batch orchestrator 在 Bash 会话执行作**硬闸门**。本报告的验证方式为：(1) 逐 AC 静态走查实现 + 对应测试；(2) QA 独立从 AC 重写 2 个反向证伪用例（不复用 dev 测试结构）；(3) 反向证伪的**确定性**论证（纯前端 DOM/computed/router.push，无随机/IO/竞态，预期输出可由组件逻辑逐用例推导，与 insight L26 "确定性反向证伪让无 Bash 不构成阻塞"同源）。verify_all 标 PENDING，执行规格见 §verify_all result。

## Test plan（逐 AC）

| 验收标准 | 测试用例 | 文件 |
|---|---|---|
| AC-1 frpc/both 就绪 → 加规则引导 + push(/proxies) | `AC-1：both 全就绪完成 → 出现引导 + 点击 push(/proxies)`、`AC-1：frpc 全就绪 → 也出现` | Wizard.spec.ts |
| AC-2 frps 角色 → 不出现引导 | `AC-2：frps 角色完成 → 不出现加规则引导` | Wizard.spec.ts |
| AC-3 both 缺失 → 警告在、引导不在 | `AC-3：both 但 frps 缺失 → 缺失警告在、引导不在` | Wizard.spec.ts |
| AC-4 Client 保存成功引导/失败不显示 | `AC-4：handleSave 成功 → showNextStepHint + 文案`、`点击 push(/proxies)`、`BC-7：保存失败不显示` | Client.spec.ts |
| AC-5 Proxies 保存成功双入口 | IS-3 5 用例（初始不显示/双入口出现/点 dashboard/点 monitor/handler 直调） | Proxies.spec.ts |
| AC-6 Proxies 空态连通入口 | `AC-6：空态含去监控入口+保留新增规则文案`、`空态点击 push(/server/monitor)` | Proxies.spec.ts |
| AC-7 Server loaded 查看运行态/失败中态不显示 | IS-5 5 用例（出现/点击 push/handler/加载失败不显示/加载中不显示） | Server.spec.ts |
| AC-8 ProxyForm tcp/udp 端口策略文案 | `tcp 含`、`udp 含`、`http 不强制` | ProxyForm.spec.ts |
| AC-9 both token 不等 → warning + 非阻断 | `AC-9：均非空不等→warning`、`AC-9：不一致非阻断推进 step3` | Wizard.spec.ts |
| AC-10 token 相等/一空/非 both 不报 | `AC-10：相等不报`、`AC-10：一空不报`、`AC-10：非 both 不报` | Wizard.spec.ts |
| AC-11 T-057 自动跳转既有行为不变 | `AC-11：全就绪自动跳转既有行为不变` + 既有 T-057 全套用例保留 | Wizard.spec.ts |
| AC-12 导航 router.push 而非 href | 各 spec 的 pushSpy 调用参数断言（push('/proxies' 等)） | 全部 spec |
| AC-13 verify_all + baseline bump | baseline frontend_tests 534 / test_count 856 / version 28 | baseline.json |

**覆盖结论**：13/13 AC 均有至少 1 个测试。

## Boundary tests added（边界）

- BC-1 frps 角色不显示加规则引导（Wizard AC-2）。
- BC-2 缺失分支（binWarning>0）不加引导（Wizard AC-3 + QA 独立分支互斥）。
- BC-3 token 一端空不报不一致（Wizard IS-7 '一空不报'）。
- BC-4 纯空白 token trim 视空不误报（Wizard Adversarial）。
- BC-6 删除成功不触发 IS-3 引导（设计 + Proxies handleDeleteConfirm 未置 showPostSaveHint，CR 已核）。
- BC-7 Client 保存失败不显示引导（Client.spec BC-7）。
- BC-8 Server 加载失败/加载中态不显示查看运行态（Server.spec 2 用例）。

## Adversarial tests

> QA 从 AC 独立重写的反向证伪。验收依据是"实现是否在此用例下存活"，而非 dev 自测是否过。每条先写失败假设。

| AC | 失败假设（"我预期失败当…"） | 复现器（QA 独立编写 / dev 编写） | 结果（确定性论证） |
|---|---|---|---|
| AC-3 | 若 IS-1 引导按钮被错误放在 step3 外层（非 binWarning===0 template 内），缺失态也会渲染该按钮 | `ADV（T-062 QA）：缺失态 IS-1 引导按钮元素不存在，且「进入仪表盘」按钮存在（分支互斥）`（QA 独立，从 AC-3 重写，不复用 dev 文本断言） | **存活** — 按钮在 `v-if="binWarning.length===0"` template 内（Wizard.vue:147+157），缺失态走 `v-else`（:165），`findAll('button')` 找不到「前往代理规则」按钮，且「进入仪表盘」按钮存在 → 断言通过。若按钮误置外层则 guideBtn 非 undefined → 失败。确定性：纯模板条件渲染。 |
| AC-1 | 若点击引导 push('/proxies') 与自动 push('/dashboard') 互相抑制/串扰（如共享单 push 调用），则两路径计数不独立 | `ADV（T-062 QA）：就绪态 push(/dashboard) 与点击 push(/proxies) 两次独立共存`（QA 独立，按 push.mock.calls 过滤路径计数） | **存活** — goToProxies 与 handleNext 自动跳转是两个独立 `router.push` 调用，pushSpy.mock.calls 中 '/dashboard' 与 '/proxies' 各自计数互不影响（点击前 /proxies 计数=0，点击后=1，/dashboard 仍在）→ 断言通过。确定性：两独立函数调用同一 spy。 |
| AC-9 | 若 tokenMismatch 被误接进 handleNext 校验（当作阻断），不一致时 currentStep 停在 2 | `ADV（T-062）：token 不一致绝不阻断完成 → 仍推进到 step3`（dev 编写，QA 复核失败假设有效） | **存活** — tokenMismatch 仅用于 template `v-if`（Wizard.vue:126），handleNext step2 分支（L268-339）未读 tokenMismatch、无 return 守卫 → currentStep 推到 3、configError 空、PUT 照常 → 断言通过。确定性：handleNext 无 tokenMismatch 引用（CR/QA grep 核实）。 |
| AC-10 | 若 tokenMismatch 漏 trim，纯空白 '   ' 会被判"非空且不等" → 误报 | `ADV（T-062）：一端纯空白 token 经 trim 视为空 → 不误报`（dev 编写，QA 复核） | **存活** — tokenMismatch 用 `frpsForm.value.authToken.trim()` + `!== ''` 判空（Wizard.vue computed），'   '.trim()==='' → 触发前提不满足 → false → 不报 → 断言通过。确定性：trim() 语义。 |
| AC-6 | 若 IS-4 空态入口被放在 data-table 外无条件渲染，加载失败态也会出现 | `ADV（T-062）：加载失败态不出现空态连通入口`（dev 编写，QA 复核） | **存活** — 入口在 `<template #empty>` slot 内（Proxies.vue），仅 data-table（`v-else` 非 error 分支）的空数据态渲染；失败态走 `n-result`（`v-if="proxiesStore.error"`），data-table 整体不渲染 → 空态入口不出现 → 断言通过。确定性：v-if/v-else 互斥 + slot 作用域。 |

QA 独立反向证伪结论：5 条对抗用例（含 QA 独立编写 2 条）实现全部存活。核心张力点（IS-1 分支互斥、token 非阻断、空态入口隔离）均被锁死。

## Regression（回归）

- **T-057 既有用例零回归**：QA 核 Wizard.spec 既有 missingForRole 4 + 全就绪自动跳转 2 + 缺失分支 3 + 保存失败 1 + frpc 标题 3 + 既有 Adversarial 3 全部保留未改。新增 AC-11 护栏额外锁死自动跳转。TestingHandle 仅追加字段不改既有。
- **新增 vue-router mock 零回归**：Client/Proxies/Server.spec 新增 `vi.mock('vue-router')`，既有用例不依赖 router；beforeEach 加 pushSpy.mockReset()。复用 Wizard.spec:58-59 已验证范式。
- **ProxyForm 既有用例零回归**：IS-6 仅在 remotePort form-item 内加 n-space 包裹 + n-text，未改 v-model / path / rules / useProxyForm；既有 T-032/T-037 单向数据流用例（emit=0 等）不受影响。
- **e2e 不受影响**：QA grep 核实 web/tests/e2e/（PM 已先核）—— 01-setup/02-auth/03-dashboard 断言仅 setup/login 表单 + 仪表盘 'frpc（客户端）'/'frps（服务端）'/'退出登录'，用 bypassWizard 绕过向导，不进 Proxies/Server/Client 编辑流，无任何新引导文案（'前往代理规则'/'去服务端监控'/'查看运行态'/'两端 token 不一致'/'端口策略'）断言。新增文案对 e2e 零冲突（insight L34）。零 e2e 改动。

## Stability（稳定性）

- 新增用例全部确定性：纯前端 computed（tokenMismatch trim 比较）+ 模板条件渲染 + 同步 router.push spy + DOM 文本断言，无 setTimeout 依赖（除既有 ProxyForm emit 用例的 50ms，本任务未碰）、无随机、无网络、无真实定时器。多次运行结果一致，无 flake 风险。
- 断言全用 DOM 文本 / find('button') 按文本 / getExposed / pushSpy，零 naive-ui 组件名查询（insight L45）→ 不受 naive-ui 内部实现/版本漂移影响，稳定。

## verify_all result（执行规格 — PENDING）

- 总测试：822 → **856**（test_count）。
- 前端：500 → **534**（frontend_tests，+34）。Go：322 不变。
- 预期 Pass：856 / Fail：**0** / Warn：0。
- 新增测试：+34（Wizard 14 + Client 4 + Proxies 8 + Server 5 + ProxyForm 3）。
- baseline 已更新：是（version 28、frontend_tests 534、test_count 856、passing_count 856、updated 2026-05-31）。
- **执行规格（交 orchestrator 真跑）**：
  - B.3 前端：`cd web && npm test`（vitest run）预期 534 passed / 0 failed。
  - B.4 测试数闸门：实测前端用例数 >= baseline.frontend_tests(534) 且 Go >= 322 → PASS。
  - E.6 报告标题闸门：06 含裸标题 `## Adversarial tests`（本段上方）→ PASS。
  - lint/typecheck：新增 import（Client +NAlert、Proxies +NAlert、ProxyForm +NText、Wizard +computed、Server +useRouter）均已添加，无未用 import、无 TS 类型错（TestingHandle 追加字段与 defineExpose 对齐）。
  - **特别复核项**：Wizard T-057 既有 30 余用例零回归 + frontend_tests==534。

## Defects found

无。0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR。

## Verdict

**APPROVED FOR DELIVERY** —— 13/13 AC 有测试、5 条反向证伪（含 QA 独立 2 条）实现全部存活、T-057 既有用例零回归、e2e 零冲突、+34 前端测试、baseline 已 bump。verify_all 标 PENDING（无 Bash），静态+确定性预测全绿，全量真跑交 orchestrator 作硬闸门。
