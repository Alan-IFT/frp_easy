# 02 方案设计 — T-060 server-reload-dirty-allowports

> 角色：Solution Architect · 模式：full · 语言：中文

## 1. 架构摘要

纯前端单文件改动。在 `Server.vue` 内为脏检测补一条端口策略比较：加载时存一份端口策略的"规范化字符串快照"，`isDirty()` 末尾追加"当前编辑器输出规范化值 vs 加载快照"的字符串不等比较。引入内联纯函数 `normalizeAllowPorts(ranges)` 把 `AllowPortRange[]` 映射为稳定字符串以消除 JSON key 顺序/格式歧义。不动 AllowPortsEditor、不动后端、不引入 v-model 桥，保留 T-040 单向数据流范式。

## 2. 受影响模块

| 文件 | 改动 |
|---|---|
| `web/src/pages/Server.vue` | 编辑：新增 `normalizeAllowPorts` 纯函数 + `loadedAllowPortsSnapshot` ref；`loadConfig` 存快照；`isDirty()` 追加 allowPorts 比较；更新 L156-158 注释；`defineExpose.__testing` 补出 `normalizeAllowPorts` + `loadedAllowPortsSnapshot` 供测试 |
| `web/src/pages/__tests__/Server.spec.ts` | 编辑：新增端口策略 dirty 测试 + normalize 单测 + 反向证伪 |
| `scripts/baseline.json` | 编辑：`frontend_tests` + `test_count` bump |
| `docs/dev-map.md` | 编辑：若有 Server.vue 描述则微调（确认后定） |

## 3. 模块分解（新增逻辑）

新增**内联纯函数**（非独立模块，留在 Server.vue `<script setup>` 内）：

```ts
// normalizeAllowPorts：把端口策略列表映射成稳定字符串，消除 JSON key 顺序/格式歧义。
// single 行 → 's:N'；range 行 → 'r:A-B'；按用户顺序 join('|')。
// 顺序敏感（重排视为脏）；形态敏感（single vs range 同端口视为脏）。
function normalizeAllowPorts(ranges: AllowPortRange[]): string {
  return ranges
    .map((r) => {
      // AllowPortRange 只可能是 {single} 或 {start,end}（getAllowPortsInput 的两种产出形态，
      // 及后端返回的 cfg.allowPorts 同构）。判定与 AllowPortsEditor seed 规则对齐：
      // single 是 number 即认作单端口；否则范围。
      if (typeof r.single === 'number') {
        return `s:${r.single}`
      }
      return `r:${r.start ?? 0}-${r.end ?? 0}`
    })
    .join('|')
}
```

> 决策：`normalizeAllowPorts` **内联** Server.vue，不抽到 `utils/`。理由：(a) 仅 Server.vue 使用（Client.vue 无 allowPorts，范围外 OOS-2）；(b) 抽取会新增文件 + 改 import 扩散；(c) 不影响 SFC < 200 行红线（见风险 R3 实测）。若未来出现第二个消费者再抽取。与需求 OOS-4 一致。

> 形态判定细节（与 AllowPortsEditor 对齐，关键正确性点）：
> - `cfg.allowPorts`（后端加载值）：合法 single 行为 `{single: N}`（N>0），合法 range 行为 `{start, end}`。
> - `getAllowPortsInput()`（编辑器输出）：single 行返 `{single: r.single ?? 0}`（**始终带 single 键**），range 行返 `{start, end}`（**不带 single 键**）。
> - 故判定 `typeof r.single === 'number'` 对两侧一致：编辑器 single 行 single 必为 number（含 0）；range 行无 single 键（`undefined`）；后端 single 行带 number、range 行无 single 键。**双侧用同一函数 → 同形态产同字符串 → round-trip identity 成立**。

## 4. 数据模型变更

无。纯前端，无 schema/migration/DB。

## 5. API 契约

无新增/变更。`apiGetServer` / `apiPutServer` 契约不变。

## 6. 时序 / 流程

```
loadConfig() 成功
  ├─ 赋值 6 标量字段
  ├─ initialAllowPorts.value = cfg.allowPorts ?? []   （AllowPortsEditor setup 读种子，单向）
  ├─ loadedSnapshot.value = { ...form.value }          （T-058 标量快照，保留）
  └─ loadedAllowPortsSnapshot.value =                  （★新增★）
       normalizeAllowPorts(cfg.allowPorts ?? [])

用户点击「重新加载」→ handleReloadClick()
  └─ isDirty() ?
       ├─ 标量浅比较任一不等 → dirty（T-058 已有）
       └─ ★新增★ normalizeAllowPorts(allowPortsEditorRef.value?.getAllowPortsInput() ?? [])
                  !== loadedAllowPortsSnapshot.value → dirty
       ├─ dirty=true  → reloadConfirmShow.value = true（弹确认，不重载）
       └─ dirty=false → void loadConfig()（直接重载）
```

关键不变量：未改动时双侧 normalize 字符串相等（round-trip identity，由 AC-5 测试锁死）；改动端口策略时不等（AC-1）。

## 7. Reuse audit（必填）

| 需求 | 既有代码 | 文件路径 | 决策 |
|---|---|---|---|
| 端口策略当前值读取 | `getAllowPortsInput()` | `web/src/components/AllowPortsEditor.vue` defineExpose | 复用现有暴露契约（不改组件） |
| 端口策略加载种子 | `initialAllowPorts` + `cfg.allowPorts` | `Server.vue` loadConfig L251 | 复用，额外从同一 `cfg.allowPorts` 派生快照 |
| 标量脏检测 + 确认弹窗 | `isDirty()` / `handleReloadClick()` / `confirmReload()` / ConfirmDialog | `Server.vue` L163-189 + L107-113 | 扩展 isDirty()，弹窗机制 0 改动 |
| 测试句柄读取 | `getExposed<T>` | `web/src/test-utils/exposed.ts` | 复用（禁 naive-ui 组件名查询，insight L45） |
| API 失败构造 | `apiError()` | `web/src/test-utils/apiError.ts` | 复用（Server.spec 已用） |
| 确认弹窗复用范式 | ConfirmDialog | `web/src/components/ConfirmDialog.vue`（T-056） | 复用，不改 |
| `AllowPortRange` 类型 | — | `web/src/types`（Server.vue 已 import） | 复用 |

## 8. 风险分析（≥3，每条配缓解）

- **R1 — round-trip 非 identity 导致"加载即脏"误报。** 若编辑器 seed→output 对某合法值不还原（如 single 行 single 被改写），未改动也会判脏，弹无意义确认（打扰回归）。
  - 缓解：双侧用**同一** `normalizeAllowPorts` 函数 + `typeof r.single === 'number'` 判定与 AllowPortsEditor seed 规则严格对齐（single>0 认 single）。AC-5 用 `[{single:8080},{start:1000,end:2000}]` 显式锁死 round-trip identity。注意：后端可能返回 `{single:0}` 这类边界值会被 seed 当 range（AllowPortsEditor L105 `r.single > 0` 才认 single），导致双侧形态分歧 → 见 R4。
- **R2 — normalize 顺序/形态敏感产生用户困惑。** 用户仅调整行顺序也判脏。
  - 缓解：这是**有意决策**（保守判脏优于漏判丢数据；漏判才是本任务要消除的缺陷）。需求 §4 显式声明顺序敏感。代价仅是多一次确认弹窗，无数据风险。
- **R3 — SFC 行数突破 200 行红线。** 新增函数 + ref + 注释扩 script。
  - 缓解：实测当前 `<script setup>`（L117-330）约 213 行含模板外注释，但红线计的是"SFC 纯逻辑 < 200 行"。新增约 12-15 行逻辑。dev-frontend 必须在 04 实测 script 逻辑行数；若逼近 200 行，优先精简注释而非拆文件（normalize 内联是最小改动）。**若确实超限 → DESIGN DRIFT 标记回退本设计重议抽 util。**
- **R4 — 边界值 `{single:0}` / 未填行的形态分歧。** 后端理论上不会返 `{single:0}`（合法策略 single>0），但 normalize 对 `typeof r.single==='number'` 判定会把 `{single:0}` 当 single 产 `'s:0'`，而 AllowPortsEditor seed 把 single≤0 当 range。
  - 缓解：实践中加载值不含 `{single:0}`（后端校验保证），此分歧不触发。编辑器输出侧未填 single 行返 `{single:0}` → normalize `'s:0'`，与加载快照（无此行）比较为脏 —— 正确语义（用户加了行）。无需特殊处理；测试 AC-4 覆盖空/single/range/顺序，不需覆盖 `{single:0}` 的人为加载值（非真实路径）。
- **R5 — e2e 回归。** 改 Server.vue dirty 逻辑可能影响 e2e。
  - 缓解：已 grep e2e（PM Stage 0）：03-dashboard 仅 `getByText('frps（服务端）')`（仪表盘卡片标题，非配置页），01-setup/02-auth 不进 Server。dirty 逻辑仅在用户进配置页改端口策略后点重新加载触发，e2e 无此路径。dev-frontend 在 04 复核确认。

## 9. 迁移 / 上线计划

无破坏性变更，无 migration，无 feature flag。行为变更纯增量：原"仅 allowPorts 改动 → 静默重载"变为"→ 弹确认"，是缺陷修复方向，向后兼容（用户预期"放弃修改前应被提醒"）。回滚 = 还原 Server.vue 与 spec 单文件 git 改动。

## 10. 范围外澄清（设计边界）

- 不设计 AllowPortsEditor 的 emit/v-model（OOS-1）。
- 不设计 Client.vue 改动（OOS-2，frpc 无 allowPorts）。
- 不设计后端/store/路由/util 改动（OOS-3）。
- 不设计 `normalizeAllowPorts` 抽 util（OOS-4，内联）。
- 不改 handleSave / hasValidationError（OOS-5）。

## Partition assignment

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `web/src/pages/Server.vue` | dev-frontend | edit（normalize + 快照 + isDirty 扩展 + 注释 + expose） | — |
| `web/src/pages/__tests__/Server.spec.ts` | dev-frontend | edit（新增 dirty/normalize/反向证伪测试） | depends on Server.vue |
| `scripts/baseline.json` | dev-frontend | edit（bump frontend_tests + test_count） | depends on 新增测试数 |
| `docs/dev-map.md` | dev-frontend | edit（如需，微调 Server.vue 描述） | — |

## Dispatch order

1. dev-frontend（唯一分区）

## Parallelism

None — 单分区，无并行。

## 11. 裁决

`READY` —— 设计完整、可由 dev-frontend 无需进一步设计决策直接实现。进入 Stage 3（Gate Reviewer）。
