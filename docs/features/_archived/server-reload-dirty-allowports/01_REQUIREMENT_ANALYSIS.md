# 01 需求分析 — T-060 server-reload-dirty-allowports

> 角色：Requirement Analyst · 模式：full · 语言：中文

## 1. 目标（一句话）

让 `web/src/pages/Server.vue`「重新加载」按钮的脏检测（dirty detection）覆盖 AllowPortsEditor 端口策略子组件的编辑状态，消除"只改了端口策略 → 点重新加载 → 无确认弹窗 → 静默丢弃端口编辑"的数据丢失路径。

## 2. 范围内行为（可测试，无"也许/应该"）

1. 当用户**仅修改了端口策略**（AllowPortsEditor 内增删行或改端口值，未动 6 个标量字段）后点击「重新加载」，系统**弹出确认弹窗**（复用现有 ConfirmDialog），不静默重载。
2. 当用户**既未改标量字段也未改端口策略**后点击「重新加载」，系统**不弹确认弹窗**，直接重新加载（无打扰回归）。
3. 当用户**修改了任一标量字段**（bindPort / authToken / dashboardEnabled / dashboardPort / dashboardUser / dashboardPass）后点击「重新加载」，系统**弹出确认弹窗**（已有行为，不得回归）。
4. 脏检测对端口策略的比较基于"加载时存的规范化快照"与"当前编辑器输出的规范化值"的字符串相等比较；二者用同一规范化函数，保证未改动时（round-trip identity）判定为非脏。
5. 规范化函数 `normalizeAllowPorts(ranges)` 将每个 range 映射成稳定字符串（`single → 's:N'`、`range → 'r:A-B'`）后用稳定分隔符 join，结果对端口策略的**顺序敏感**（用户调整行顺序视为脏）、对 single/range 形态敏感。
6. 加载成功时，在存 6 标量快照（`loadedSnapshot`）的同时，存一份端口策略规范化快照（`loadedAllowPortsSnapshot = normalizeAllowPorts(cfg.allowPorts ?? [])`）。
7. 每次成功加载后，端口策略快照刷新；重载完成后再次点击「重新加载」（未改动）判定为非脏。
8. 更新 `Server.vue` 原 L156-158 注释，说明 allowPorts 已纳入脏检测，移除"已知局限"措辞。

## 3. 范围外（本次明确不做）

1. 不修改 `AllowPortsEditor.vue` 组件本身（不新增 emit、不引入 v-model 桥、不改 setup 单向数据流读取范式）。
2. 不修改 `Client.vue`（frpc 客户端无 allowPorts 端口策略字段）。
3. 不改动后端、store、路由、API 层、`utils`、其它页面。
4. 不抽取 `normalizeAllowPorts` 到独立 util 文件（保持在 Server.vue 内联，避免扩散；若未来 Client 也需要可再议）—— 除非该决策影响 SFC < 200 行红线，则由 Architect 在 02 决策。
5. 不改变保存（handleSave）逻辑、不改变端口策略校验（hasValidationError）逻辑。

## 4. 边界条件（null / 空 / 最大 / 并发 / 错误路径）

- **空策略两侧**：`cfg.allowPorts` 为 `undefined`/`[]` 且编辑器无行 → 双侧 normalize 均为空串 → 非脏。
- **single 行未填值**：`getAllowPortsInput()` 对 `single===null` 行返回 `{single: 0}`；normalize 需对 `single:0` 产生确定字符串（如 `'s:0'`），与"用户加了一行但没填"对应。若加载快照无此行（空）则比较为脏（用户确实加了行）→ 符合"改了端口策略"语义。
- **range 行未填值**：`getAllowPortsInput()` 对未填行返回 `{start:0,end:0}`；normalize 产生 `'r:0-0'`，加载快照若无此行则为脏。
- **round-trip identity**：对合法的加载值（如 `[{single:8080}]` / `[{start:1000,end:2000}]`），编辑器 seed→row→output 应为 identity，双侧 normalize 相等 → 非脏（这是范围内行为 2/4 的前提，必须由测试锁死）。
- **顺序变化**：`[{single:1},{single:2}]` 重排为 `[{single:2},{single:1}]` → normalize 字符串不同 → 脏（顺序敏感是有意决策，避免漏判）。
- **single vs range 同端口**：`[{single:8080}]` 与 `[{start:8080,end:8080}]` normalize 后不同（`'s:8080'` vs `'r:8080-8080'`）→ 视为脏。这与编辑器形态区分一致（用户切了形态）。
- **错误态**：加载失败（loadError 非 null）时表单与编辑器均不渲染（T-047 三态），「重新加载」按钮不可达，本任务行为不在该路径触发。
- **编辑器 ref 未挂载**：`allowPortsEditorRef.value` 可能为 null（loading/error 态），`isDirty()` 中 `?.` 兜底 + `?? []` 退化为空策略比较，不抛错。

## 5. 验收标准（可验证）

- AC-1：仅改端口策略后 `isDirty()` 返回 true，`handleReloadClick()` 置 `reloadConfirmShow=true` 且此刻不调用 `apiGetServer`。（测试断言 apiGet 调用计数不变）
- AC-2：未改任何内容时 `isDirty()` 返回 false，`handleReloadClick()` 不弹确认且 `apiGetServer` 被再调用一次（直接重载）。
- AC-3：改标量字段仍 `isDirty()=true` 且弹确认（无回归）—— 复用 T-058 既有断言。
- AC-4：`normalizeAllowPorts` 稳定性单测：single / range / 顺序 / 空 各产生确定且互不相同的预期字符串。
- AC-5：round-trip：加载 `[{single:8080},{start:1000,end:2000}]` 后未改动 → `isDirty()=false`。
- AC-6：脏 + 确认（confirmReload）→ 重载覆盖回真实值且 `isDirty()` 归零（端口策略快照随之刷新）。
- AC-7：`scripts/verify_all` PASS（前端 vitest 全绿 + eslint 无错 + B.4 测试数不降）。
- AC-8：`scripts/baseline.json` 的 `frontend_tests` 与 `test_count` 同步 bump，等于新增测试数。
- AC-9：`Server.vue` SFC `<script setup>` 纯逻辑行数仍 < 200 行（红线）。
- AC-10：06_TEST_REPORT.md 含**裸** `## Adversarial tests` 段（无 §N / 数字前缀），含一条"只删一行端口 → 脏检测捕获 → 确认出现"的反向证伪。

## 6. 非功能需求（仅列实质性）

- 兼容性：不破坏 e2e 烟雾测试（01-setup / 02-auth / 03-dashboard）。已确认 03-dashboard 仅断言 `frps（服务端）` 仪表盘卡片标题文本，不进入 Server 配置页编辑流（详见 02/04）。
- 安全：无新增数据流出口；端口策略值不经新路径序列化。
- 一致性：保留 AllowPortsEditor 单向数据流范式（insight L13）；测试断言禁用 naive-ui 组件名查询（insight L45）。

## 7. 相关历史任务

- **T-058 frontend-interaction-polish**（`docs/features/frontend-interaction-polish/`，pending archive）：引入 Server/Client「重置→重新加载」+ dirty 防误丢的标量快照机制；其 (B) 决策 D2 明确**刻意不覆盖 AllowPortsEditor**，记为"已知局限：仅 allowPorts 改动退化为直接重载"。本任务 = 补齐该局限。**不重新设计标量 dirty 机制，仅扩展。**
- **T-040 frps-allow-ports-policy**（`docs/features/frps-allow-ports-policy/`）：AllowPortsEditor 组件 + 单向数据流 + getAllowPortsInput/hasValidationError 暴露契约的来源。
- **T-047 frontend-honest-states**：Server.vue 加载/错误三态（loadError/loading）—— 错误态下表单与编辑器均不渲染，本任务行为不在该路径触发。
- **T-057 binary-missing-onboarding-ux**：踩坑 naive-ui 组件名查询不可靠，沉淀 insight L45（断言用 DOM/文本/getExposed）。

## 8. 给用户的待澄清问题

无。本任务为 T-058 (B) 已知局限的精确补齐，技术上下文（PM dispatch）已逐处核实，无歧义。

## 9. 裁决

`READY` —— 无待澄清问题，可进入 Stage 2（Solution Architect）。
