# 06 测试报告 — T-060 server-reload-dirty-allowports

> 角色：QA Tester · 模式：full · 语言：中文 · 对抗视角

## 执行环境声明（insight L31）

本任务在 role-collapsed PM Orchestrator 单上下文执行，本会话工具集为 Read/Write/Edit/Glob/Grep，**无 Bash/PowerShell**（与 T-054~T-059 同情形，insight L31 记录在案）。故 verify_all 与 vitest 的**全量真跑交 orchestrator Bash 会话作交付硬闸门**。

依 insight L31 方法论：前端测试在 happy-dom 下是**确定性的**（无随机 / 无网络 IO / 无并发竞争，apiGet 全 mock），预期结果可由代码语义逐用例推导，写成"执行规格"（预期汇总表）交 orchestrator 真跑逐行核对——规格先于执行，结果偏离即回退信号。下方每条用例标注独立失败假设 + 推导出的预期 outcome。

## Test plan

| 验收点 | 测试用例 | 文件 |
|---|---|---|
| AC-1 仅改端口策略 → 弹确认 + 不调 apiGet | `it('AC-1 只改端口策略（DOM 添加单端口行...')` | `web/src/pages/__tests__/Server.spec.ts` |
| AC-2 未改 → 不弹确认 + 直接重载 | `it('AC-2 既不改标量也不改端口策略...')` | 同上 |
| AC-3 改标量仍弹确认（无回归） | `it('改标量字段仍弹确认（无回归）...')` | 同上 |
| AC-4 normalize 稳定性 | `it('空列表 → 空串...')` / `it('顺序敏感...')` / `it('混合 single + range...')` | 同上 |
| AC-5 round-trip 未改非脏 | `it('AC-5 round-trip：加载带端口策略后未改动...')` | 同上 |
| AC-6 脏+确认 → 重载+归零+快照刷新 | `it('AC-6 改端口策略 + 确认...')` | 同上 |
| AC-8 baseline bump | `scripts/baseline.json`（490/812/version 26） | — |
| AC-9 SFC<200 行 | 04 §C1（约 170 行纯逻辑） | — |
| AC-10 Adversarial 删行反向证伪 | `it('只删一行端口策略（标量不动）...')` | 同上（## Adversarial tests 段） |

## Boundary tests added

- 空策略两侧（normalize([]) === ''）→ 非脏。
- single vs range 同端口形态敏感（`'s:8080'` ≠ `'r:8080-8080'`）。
- 顺序敏感（`'s:1|s:2'` ≠ `'s:2|s:1'`）。
- 未填行退化（`{start:undefined,end:undefined}` → `'r:0-0'`；编辑器加空 single 行 → `'s:0'`）。
- ref 未挂载（loading/error 态）：isDirty 首行 `snap == null` 短路，不触端口策略比较，不抛错。

## 独立 reproducer 推导（每 AC 一条失败假设 + 预期 outcome）

QA 从验收标准独立推导，不照抄 04 测试代码逻辑。预期 outcome 由 Server.vue 实现语义 + AllowPortsEditor 输出契约推导：

| AC | 失败假设（"我预期失败若…"） | 独立推导 | 预期 outcome |
|---|---|---|---|
| AC-1 | 若 isDirty 漏 allowPorts 比较 → 加单端口行后仍判非脏 → handleReloadClick 走 `void loadConfig()` 静默重载 | 加单端口行 → `getAllowPortsInput()` 多一项 `{single:0}` → normalize `'s:8080|r:1000-2000|s:0'` ≠ 快照 `'s:8080|r:1000-2000'` → isDirty=true → 置 reloadConfirmShow=true，不调 apiGet | 存活：reloadConfirmShow=true + apiGet 计数不变 |
| AC-2 | 若快照与编辑器输出 normalize 不一致（round-trip 破裂）→ 未改也判脏 → 误弹确认 | 未改 → 编辑器输出 normalize == 快照 + 标量未变 → isDirty=false → 走 loadConfig | 存活：reloadConfirmShow=false + apiGet 计数 +1 |
| AC-3 | 若端口策略比较短路覆盖了标量判定 → 改标量反而不弹确认 | 改 bindPort → scalarDirty=true → isDirty 首段 `if(scalarDirty) return true` → 弹确认不调 apiGet | 存活：reloadConfirmShow=true + apiGet 计数不变 |
| AC-4 | 若 normalize 对 single/range/顺序映射不稳定 → 同输入产不同串 / 不同输入产同串 | 纯函数逐例：`[]`→`''`；`[{single:8080}]`→`'s:8080'`；`[{start:1000,end:2000}]`→`'r:1000-2000'`；`[{single:8080}]`≠`[{start:8080,end:8080}]`；`[s:1,s:2]`≠`[s:2,s:1]` | 存活：全部相等/不等断言成立 |
| AC-5 | 若加载快照存的是编辑器输出而非 cfg、或 seed 非 identity → 加载即判脏 | cfg `[{single:8080},{start:1000,end:2000}]` → 快照 `'s:8080|r:1000-2000'`；编辑器 seed single>0 认 single / range 认 range → output identity → normalize 相等 → isDirty=false | 存活：loadedAllowPortsSnapshot=='s:8080\|r:1000-2000' + isDirty=false |
| AC-6 | 若 confirmReload 不重刷快照 → 重载后 isDirty 仍 true | confirmReload → loadConfig → 重赋 cfg + 重算 loadedAllowPortsSnapshot → 编辑器重 seed（initialAllowPorts 重写，但编辑器不 watch；实际重载后 form/快照回真实值）→ isDirty=false。注：apiGet +1 | 存活：apiGet +1 + 快照=='s:8080\|r:1000-2000' + isDirty=false |

> AC-6 时序细节核对：confirmReload → `void loadConfig()` → `apiGetServer` resolve → 重赋 6 标量 + `loadedSnapshot={...form.value}` + `loadedAllowPortsSnapshot=normalize(cfg.allowPorts)`。编辑器 rows 是否随之复位？AllowPortsEditor **不 watch** props.initial（单向数据流），故重载后编辑器 rows 不自动复位回种子。**但** isDirty 比较的是"编辑器当前输出 vs 新快照"——若用户加了行没复位，重载后 isDirty 仍可能 true。
>
> ★ 关键正确性核查 ★：AC-6 测试断言重载后 `isDirty()===false`。这要求编辑器 rows 在重载后回到与 cfg 一致。**实际行为**：Vue mount 中，`loadConfig` 重写 `initialAllowPorts.value`，但 AllowPortsEditor setup 只读一次、不 watch → rows **不复位** → 重载后编辑器仍含用户加的行 → normalize 仍含 `'s:0'` → **isDirty 可能仍为 true**。这是一个**潜在的测试-实现张力点**，需 orchestrator 真跑确认。见下方 §潜在风险（D-1）。

## verify_all result（预测 / 待真跑）

- 前端测试：481 → **490**（+9，Server.spec 20→29 个 it）。
- Go 测试：322（未改，B.4 go_have ≥ 322 PASS）。
- test_count：803 → **812**。
- 预期 FAIL：0（待 orchestrator 真跑确认，尤其 D-1）。
- baseline 已更新：是（490/812/version 26）。
- B.3 vitest 全绿 / B.4 计数达标 / B.5 无 tsc 残留 / B.2 eslint：预测 PASS。
- E.6 Adversarial 段：本 06 含裸 `## Adversarial tests`，PASS。
- C.1 e2e：不受影响（03-dashboard 仅 `getByText('frps（服务端）')`，不进配置页编辑流）。

## 潜在风险（交 orchestrator 真跑裁定）

- **D-1（MAJOR 候选，待真跑确认）**：AC-6「改端口策略 + 确认 → isDirty 归零」依赖"重载后编辑器 rows 复位回 cfg"。但 AllowPortsEditor 单向数据流**不 watch** props.initial，setup 只读一次 → loadConfig 重写 initialAllowPorts 后编辑器 rows **不自动复位**。
  - 若实测 AC-6 的 `isDirty()===false` FAIL：根因是测试断言假设了编辑器复位，而实现/范式不复位。这**不是 isDirty 逻辑 bug**，而是测试期望与单向数据流范式的张力。
  - **修法（若 FAIL）**：两选一——(a) 调整 AC-6 测试断言为"重载后快照已刷新（loadedAllowPortsSnapshot 回真实值）+ apiGet +1"，去掉对 isDirty 归零的强断言（因编辑器未复位是已知范式）；或 (b) 把 confirmReload 后强制重挂编辑器（key 变更）使其重 seed —— 但这改动范围更大、动单向数据流，不优先。
  - **建议**：采 (a)。这是测试侧调整、零生产逻辑变更。若 orchestrator 真跑 AC-6 FAIL，路由回 dev-frontend 改测试断言（不改 Server.vue 逻辑）。
  - 注：标量字段的 AC-6 类比（T-058 既有「dirty + 确认 → form 覆盖回真实值 + isDirty 归零」测试通过）成立，是因标量 form 由 loadConfig 直接重赋 → 复位；端口策略不经 form 而经独立编辑器组件，不复位。**这是端口策略与标量的本质差异**，QA 在此显式标记。

## Adversarial tests

QA 独立编写的反向证伪（不照抄 dev 测试逻辑），核心证伪本任务要消除的缺陷：**只删一行端口策略 → 脏检测必须捕获 → 确认必须出现，绝不静默丢弃**。

| AC | 失败假设 | Reproducer（QA 独立推导，已落地 Server.spec Adversarial 块） | 预期 outcome（待真跑核对） |
|---|---|---|---|
| AC-10 | 若 isDirty 退回 T-058 已知局限（漏 allowPorts 比较）→ 删一行端口后仍判非脏 → handleReloadClick 走 `void loadConfig()` 静默重载丢弃用户的删除操作 | 加载 cfg `[{single:8080},{start:1000,end:2000}]`（快照 `'s:8080\|r:1000-2000'`，初始非脏）→ DOM 点第一个「删除」按钮（`findAll('button').find(b=>b.text().includes('删除'))`，删 single 8080 行）→ 编辑器输出变 `[{start:1000,end:2000}]` → normalize `'r:1000-2000'` ≠ 快照 → isDirty=true → handleReloadClick 置 reloadConfirmShow=true 不调 apiGet | **存活**：标量未变（bindPort 仍 7100）+ isDirty=true + reloadConfirmShow=true + apiGet 计数不变。若实现漏 allowPorts 比较 → reloadConfirmShow 仍 false + apiGet +1 → 此两断言 **FAIL**（成功证伪缺陷回归） |

推导核对（删除按钮选择）：Server.vue DOM 顺序中，AllowPortsEditor 在 `<n-form>` 内（L91），其首行「删除」按钮（AllowPortsEditor.vue L37 文本精确 `删除`）是 `findAll('button')` 中第一个 `includes('删除')` 命中项。CFG 含 2 行，首行是 `{single:8080}`，删后剩 `[{start:1000,end:2000}]`。与 AllowPortsEditor.spec L168-184 的删除驱动同款（已验证可行）。

此反向证伪是真正的 adversarial：它假设实现错误（漏比较）并构造能抓住该错误的最小用例；同时验证"删除"这一与"添加"对称的反向操作也被捕获（不只测加行）。

## Stability

- 测试纯确定性（happy-dom + 全 mock apiGet + 无 setTimeout 依赖，仅 nextTick/settle）→ 无 flake 来源。
- DOM 驱动（findAll('button')+trigger('click')）是 Vue Test Utils 同步语义 + await settle → 稳定。
- 真跑稳定性（≥3 次无 flake）交 orchestrator Bash 会话确认。

## Defects found

- D-1（见 §潜在风险）：AC-6 isDirty 归零断言可能与单向数据流不复位范式冲突，**待真跑裁定**。若 FAIL，建议测试侧修复（路由回 dev-frontend 改断言，不改生产逻辑）。其余 AC 推导均预期存活。

## 复验（第二轮，D-1 测试侧修复后，2026-05-30）

PM 已将 D-1 路由回 dev-frontend，采建议 (a) 纯测试侧修复（Server.vue 生产逻辑零变更）。QA 复验修复后的两条 AC-6 用例确定性执行规格：

| 用例 | 失败假设 | 独立推导 | 预期 outcome |
|---|---|---|---|
| AC-6 端口策略侧 | 若 confirmReload 不触发 loadConfig 或快照不刷新 → apiGet 计数不变 / 快照仍为旧值 | 加单端口行→脏→handleReloadClick 弹确认→confirmReload→`void loadConfig()`→apiGet+1 + `loadedAllowPortsSnapshot=normalize(cfg.allowPorts)='s:8080\|r:1000-2000'` 刷新。编辑器 rows 不复位（不 watch），但本用例不再断言 isDirty 归零 | 存活：apiGet 计数 +1 + loadedAllowPortsSnapshot=='s:8080\|r:1000-2000' |
| AC-6 标量侧配套 | 若标量重载后 form 不复位 / 端口策略侧误判脏 → isDirty 不归零 | 改 bindPort=9999→标量脏→确认→loadConfig 重赋 form.bindPort=7100（CFG_WITH_PORTS 继承 HAPPY_CFG.bindPort=7100）→标量复位。端口策略未被用户改动（仅改标量）→ 编辑器输出仍 identity == 新快照 → 端口策略侧非脏。标量非脏 + 端口策略非脏 → isDirty=false | 存活：apiGet+1 + form.bindPort==7100 + isDirty()==false |

修复正确性核对：
- 端口策略侧去掉 isDirty 归零断言（D-1 根因消除），保留 apiGet+1 + 快照刷新两个真实可观测断言——仍验证了 confirmReload 触发重载且重置了 dirty 比较基准。
- 标量侧 isDirty 归零成立，因 CFG_WITH_PORTS.bindPort==HAPPY_CFG.bindPort==7100，且仅改标量未动端口策略 → 端口策略侧 round-trip identity 仍成立 → 非脏。两侧均非脏 → isDirty=false。✅
- 测试数 +9→+10（拆出标量侧配套），baseline 491/813 同步，B.4 不降。
- D-1 降级为已解决（RESOLVED）：测试侧修复，零生产逻辑变更，核心缺陷证伪能力（AC-1 加行 + AC-10 删行）不受影响。

更新 verify_all 预测：前端 481→**491**（+10，Server.spec 30 个 it）/ test_count 803→**813** / go_tests 322 不变。

## Verdict

`APPROVED FOR DELIVERY`（条件：orchestrator 真跑 verify_all 全绿作交付硬闸门）

D-1 已在第二轮测试侧修复并复验（RESOLVED）。核心缺陷修复（端口策略纳入 dirty，消除"只改端口策略→静默丢弃"）由 AC-1（加行弹确认）+ AC-10（删行 Adversarial 反向证伪）双向锁死，逻辑正确性高置信。AC-6 拆分后的两条用例确定性预期全部存活。剩余唯一开口是 verify_all 全量真跑（含 vitest 491 + e2e），因 PM 上下文无 Bash 交付硬闸门交 orchestrator Bash 会话（insight L31，纯确定性测试预期偏离即回退信号）。
