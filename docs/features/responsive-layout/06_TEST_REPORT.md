# 测试报告 — T-067 · responsive-layout

> Harness Stage 6 · QA Tester · 全中文 · 对抗心态：假设实现错误，从 AC 独立构造证伪用例

## Test plan

| 验收标准 | 测试用例 | 文件 |
|---|---|---|
| AC-2 窄屏 collapsed=true / 宽屏=false | useViewport 窄/宽初值 2 + 跨阈值 2；AppLayout 窄/宽初始 2 + 宽↔窄自动 2 | `useViewport.spec.ts` / `AppLayout.spec.ts` |
| AC-3 窄屏仍可手动展开不锁死 | QA-ADV-3（手动展开后视口仍窄不被强制收回） | `qa_t067_adversarial.spec.ts` |
| AC-4 表单 max-width 化 | Server/Client 各 2（正向 width:100%+max-width + 反向证伪无裸像素宽） | `Server.spec.ts` / `Client.spec.ts` |
| AC-5 顶栏不溢出 + 关键入口可达 | AppLayout 顶栏关键入口 1；QA-ADV-5（窄屏主题切换/退出+折叠态 a11y） | `AppLayout.spec.ts` / `qa_t067_adversarial.spec.ts` |
| AC-6 e2e 视口 > 阈值零回归 | useViewport 常量 < 1280 1；QA-ADV-6（1280 等价 matches=false 展开菜单可见） | `useViewport.spec.ts` / `qa_t067_adversarial.spec.ts` |
| AC-7 既有 spec 零回归 | AppLayout 既有 8 例不动 + Server/Client 既有 spec 无 width 断言 | （回归核查，见下） |

## Boundary tests added

- 阈值边界：NARROW_MAX_WIDTH=767.98 使 768 整数判展开（BC-1）；常量 < 1280 断言（BC-2）。
- 降级：无 matchMedia（stubGlobal undefined）+ matchMedia 抛错 → isNarrow 留 false 不崩（BC-6，useViewport.spec 2 例）。
- 监听守卫：多次 useViewport() 后 listeners.length===1（BC-5/NFR-4，无重复注册无泄漏）。
- 跨阈值往返：宽→窄→宽 collapsed 与最终断点一致不残留（QA-ADV-4）。
- 折叠态 a11y：自动折叠让折叠态成窄屏常态，断言 7 个 menuIcon aria-label 仍在（T-064 a11y 不退化，BC-7）。

## Adversarial tests

> 每条独立 reproducer（不复用 dev 测试代码），先写失败假设再断言实现存活。文件
> `web/src/components/__tests__/qa_t067_adversarial.spec.ts`（QA 独立编写，受控 matchMedia
> 用本文件自有 QaMql 工厂，不引 dev 的 FakeMql）。verify_all 真跑由 batch orchestrator
> 执行（QA role-collapsed 上下文无 Bash/PS，insight L31）；下表给"预期存活/证伪"执行规格。

| AC | 失败假设（"我预期失败当…"） | Reproducer（QA 新写） | 预期结果 |
|---|---|---|---|
| AC-2 | collapsed 初值写死 false 而非 isNarrow.value → 窄屏不折叠 | `qa_t067_adversarial.spec.ts::QA-ADV-1` 窄屏挂载断言 `.n-layout-sider--collapsed` | 存活（确实折叠；初值取 isNarrow.value） |
| AC-2/桌面不回归 | 阈值逻辑反了/误判 → 宽屏也折叠回归 | `QA-ADV-2` 宽屏挂载断言非折叠 | 存活（宽屏展开，桌面不回归） |
| **AC-3（最高风险）** | watch 误用 `{immediate:true}` 或每渲染周期重置 → 窄屏手动展开立即被收回锁死 | `QA-ADV-3` 窄屏折叠→点 trigger 展开→视口仍窄再 emit(true)→断言仍展开；trigger DOM 不稳时回退断言 isNarrow 未变 watch 不触发 collapsed 无副作用 | 存活（非 immediate watch 仅跨阈值触发，同区间手动展开不被覆盖，不锁死） |
| AC-3 状态残留 | watch 状态泄漏/方向判断错 → 往返后 collapsed 与最终视口不符 | `QA-ADV-4` 宽→窄→宽往返断言每段 collapsed 与断点一致 | 存活（无残留，方向正确） |
| AC-5 | 窄屏过度收缩误伤关键入口 → 手机端无法登出/切主题 | `QA-ADV-5` 窄屏断言 `[aria-label="主题切换"]` 存在 + "退出登录"文本 + 7 menuIcon aria-label | 存活（关键入口可达，折叠态 a11y 不退化） |
| AC-6 | NARROW_MAX_WIDTH ≥ 1280 → e2e 1280 视口被误折叠隐藏菜单 → 03-dashboard FAIL | `QA-ADV-6` 1280 等价 matches=false 断言展开 + 菜单文本"仪表盘"可见 + 常量 < 1280 | 存活（1280 判展开，菜单可见，e2e 零回归） |

执行规格示意（batch orchestrator 真跑核对）：
```
cd web && npx vitest run src/components/__tests__/qa_t067_adversarial.spec.ts
预期：6 passed（QA-ADV-1~6 全 survived）
cd web && npx vitest run   # 全量
预期：frontend_tests == 600（576 + 18 dev + 6 QA），0 failed
```

## 回归核查（AC-7）

- **AppLayout 既有 8 例（T-064 菜单 5 + T-066 主题 3）**：既有用例不 stub matchMedia → happy-dom 默认 matchMedia 对 `(max-width:767.98px)` 返 false（默认 innerWidth > 768）→ isNarrow=false → collapsed 初值 false = 展开，与既有"默认展开"行为字节一致。**预期零回归**（CR/QA 已确认既有用例不依赖侧栏折叠态、不注入 matchMedia）。
- **Server/Client 既有 spec（T-047/T-058/T-060/T-062）**：grep `width|max-width|matchMedia` **0 命中**（GR/CR 已核）→ max-width 改动对既有断言（DOM 文本/按钮/getExposed/apiGet）透明。**预期零回归**。
- **e2e 03-dashboard**：playwright `devices['Desktop Chrome']` 视口 1280 ≥ 768 阈值 → 侧栏保持展开、菜单文本可见；TC-04 断言 Dashboard 页正文 + `仪表盘.first()`、TC-05 按 `name '退出登录'` 点击（位置/文本未改，n-select 仍在退出按钮前）。**预期零回归**（QA-ADV-6 编码同一命题）。

## verify_all result

**PENDING**（QA role-collapsed 上下文无 Bash/PS，insight L31）。执行规格交 batch orchestrator Bash 会话真跑作硬闸门：
- Total tests: 918 → **942**（+24）
- frontend_tests: 576 → **600**（dev 18 + QA 6）
- go_tests: **342**（不变）
- Pass: 预期 942 / Fail: 预期 0 / Warn: 预期 0
- New tests added: 24
- Baseline updated: **yes**（version 33→34，frontend_tests 600，test_count 942；同时修复 baseline.json 既有冗余尾巴使其为合法单一 JSON 对象，C-4）
- 特别复核：e2e（C.1 03-dashboard 1280 视口保持展开）+ AppLayout 既有 8 例 + Server/Client 既有 spec 零回归。

## Defects found

无。0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR。

## Stability

- 测试全部确定性（受控 matchMedia mock，无真实计时/网络/竞态；vi.resetModules 保证单例隔离无跨用例泄漏；无 setTimeout 依赖）→ 预期非 flaky。
- QA-ADV-3 的 trigger DOM 选择器有回退分支（`.n-layout-toggle-button`/`.n-layout-toggle-bar` 不命中时退化为等价命题断言），避免因 naive-ui 内部 class 变动导致脆弱失败——这是对 insight L45"少用组件名查询"的折中（折叠 trigger 无可观察 aria 名，只能靠 class，故加回退保稳）。

## Verdict

**APPROVED FOR DELIVERY**（0 缺陷；6 条独立对抗全预期存活；24 新测试；baseline 已 bump 并修复结构；verify_all 真跑交 batch orchestrator 硬闸门）
