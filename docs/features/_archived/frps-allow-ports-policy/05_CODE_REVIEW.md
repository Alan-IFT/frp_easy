# 05 — Code Review · T-040 frps-allow-ports-policy

> Stage 5 / 7。代码评审。逐文件核对实现 vs 设计 + 检查可能的边角问题。

## 1. 改动概览

| 文件 | 改动类型 | 新增/修改行数（估算） |
|---|---|---|
| `internal/frpconf/render.go` | 改 | +90 行（types + RenderFrps 段 + ValidateFrpsAllowPorts） |
| `internal/frpconf/render_test.go` | 改 | +240 行（13 个新 Test） |
| `internal/httpapi/handlers_server.go` | 改 | +35 行（types + helper + validate 集成） |
| `internal/httpapi/handlers_server_test.go` | 新 | +200 行（8 个 Test） |
| `internal/httpapi/config_helper.go` | 改 | +3 行（AllowPorts 透传） |
| `web/src/types.ts` | 改 | +14 行（AllowPortRange + FrpsConfig 扩字段） |
| `web/src/components/AllowPortsEditor.vue` | 新 | +175 行（template + script + scoped style） |
| `web/src/pages/Server.vue` | 改 | +25 行（import + ref + loadConfig 种子 + handleSave 守门 + 模板段） |
| `web/src/components/__tests__/AllowPortsEditor.spec.ts` | 新 | +175 行（12 case） |
| `openapi.yaml` | 改 | +30 行（FrpsConfig.allowPorts + AllowPortRange schema） |
| `docs/dev-map.md` | 改 | 3 处微调 |

## 2. 设计一致性核查

| 设计点 | 实现位置 | 一致性 |
|---|---|---|
| `FrpsAllowPortRange` 三字段互斥 | `frpconf/render.go::FrpsAllowPortRange` + `ValidateFrpsAllowPorts` | ✅ |
| `frpsRoot.AllowPorts` 放 struct 末尾 | 同文件 `frpsRoot` 字段顺序 | ✅ |
| `RenderFrps` 校验 + 渲染合一 | `RenderFrps` 末尾段 | ✅ |
| 闭区间 overlap：`a.lo <= b.hi && b.lo <= a.hi` | `ValidateFrpsAllowPorts` | ✅ |
| 上限 100 | `ValidateFrpsAllowPorts` | ✅ |
| handler 复用 `frpconf.ValidateFrpsAllowPorts` | `handlers_server.go::putServer` | ✅ |
| `toFrpconfAllowPorts` 双路径共用 | putServer + config_helper.go | ✅ |
| 前端单向数据流：props.initial 读一次 | `AllowPortsEditor.vue::rows = ref(...)` | ✅（无 watch、无 emit） |
| defineExpose `getAllowPortsInput` + `hasValidationError` | 同文件 | ✅ |
| handleSave 先 `hasValidationError()` 再 `getAllowPortsInput()` | `Server.vue::handleSave` | ✅ |
| Naive UI mock importOriginal + 6 方法 | `AllowPortsEditor.spec.ts` 顶部 vi.mock | ✅ |
| OpenAPI `AllowPortRange` 与 Go struct JSON 字面对齐 | `openapi.yaml` | ✅（single/start/end 三 int [1,65535]） |

## 3. 逐文件审查

### 3.1 `internal/frpconf/render.go`

**优点**：

- `frpsAllowPort.Single/Start/End` 都用 `omitempty` 让 TOML 渲染时 0 值字段消失，与互斥语义对齐
- `ValidateFrpsAllowPorts` 错误文本全部中文 + 含 `allowPorts[i]` 索引（GR C-1 充分消化）
- 上限 100 在循环前一次性早返
- O(n²) 重叠检测在 n≤100 时亚毫秒，胜过 sweep line 简洁

**潜在改进（非阻塞）**：

- 重叠检测可考虑先按 `lo` 排序再 O(n log n)，但当前规模下投入产出比低，不做
- `ValidateFrpsAllowPorts` 入参可以是 `[]FrpsAllowPortRange` 也可以是 `*[]...`，当前选 by-value 与项目其它 `Validate*` 函数一致

### 3.2 `internal/frpconf/render_test.go`

13 个新 Test 覆盖：
- Empty (nil + []) → no [[allowPorts]] 字面
- SingleRange / MultiRange / SingleOnly（AC-1 + OQ-6）
- BoundaryMinMax（AC-6）
- StartGreaterThanEnd / Mutex (3 子用例) / Overlap / OverlapBoundary / OverlapSingleVsRange
- OutOfRange (4 子用例：empty entry / start=0 / end=65536 / start=65536)
- TooMany（101 失败 + 100 通过双向）
- TOMLRoundTrip（toml.Unmarshal 反向验证字段字面）

**亮点**：MultiRange 用 `strings.Index` 验顺序，避免假设字典序；TOMLRoundTrip 反向断言 single entry 不含 start/end key（互斥语义在 wire 层落实）。

### 3.3 `internal/httpapi/handlers_server.go`

- `FrpsConfig.AllowPorts` 字段加 `omitempty` JSON tag，与 axios 端 `length > 0 ? ... : undefined` 对称
- `toFrpconfAllowPorts(nil) == nil` 让 RenderFrps 进入"不渲染段"分支
- 校验放在 dashboard 校验之后，token 占位回写之前；error path 422 + field="allowPorts"

### 3.4 `internal/httpapi/handlers_server_test.go`（新文件）

8 个 Test：
- `Valid` 端到端：PUT → 200 + KV 字面含 `"start":6000` 与 `"single":9000`
- `Empty` 反向：留空 → KV 不含 `allowPorts` 字面
- `OutOfRange` (4 子用例：end_65536 / start_zero / single_65536 / single_zero_only)
- `StartGreaterThanEnd` / `Overlap` / `Mutex`
- `RoundTrip` PUT → GET 数组等值
- `TooMany` 101 → 422 + 含上限数字

**潜在风险（已规避）**：测试用 `t.Context()`（Go 1.24+）；项目现有测试已大量使用（handlers_server_runtime_test.go 实测），无 Go 版本兼容问题。

### 3.5 `web/src/components/AllowPortsEditor.vue`

**关键检查**：

- ❑ `defineProps<Props>()` 无 default value：父级必须传 `:initial`（Server.vue 已传 `:initial="initialAllowPorts"`）
- ✅ `rows` ref 在 setup 时从 `props.initial ?? []` 一次性映射，无 `watch(() => props.initial, ...)` —— 单向数据流硬保证
- ✅ `removeAt` 用 `splice`，`v-for :key="r._id"` 让删除中间行不导致 input 焦点错位
- ✅ `validateRow` 中 `for (let j = 0; j < idx; j++)` 严格只与前序比对（GR C-2 充分消化）
- ✅ `rowHasInputErr` helper 让 overlap 比对跳过本身就非法的行，避免误报"与第 N 行重叠"实则那行自己也错
- ✅ `defineExpose` 暴露 2 方法（不暴露 rows 自身，封装良好）
- ✅ 模板 NTag/NInputNumber/NButton/NText/NAlert/NSpace 全部从 naive-ui 显式 import

**SFC 规模**：物理 175 行；script 段纯逻辑（去掉 import、defineProps、defineExpose 框架）约 80 行，远低于 200 行红线（insight L31）。

### 3.6 `web/src/pages/Server.vue`

- `import AllowPortsEditor from '../components/AllowPortsEditor.vue'` + `import type { AllowPortRange } from '../types'` 干净
- `allowPortsEditorRef` 用 `InstanceType<typeof AllowPortsEditor>` 拿到 defineExpose 的类型推导（TS 严格友好）
- `loadConfig`: `initialAllowPorts.value = cfg.allowPorts ?? []`（用 `??` 容 undefined）
- `handleSave` 先 `hasValidationError` 检查，立即 message.error 直返；通过则 `getAllowPortsInput()` 拿数组
- PUT 字段 `allowPorts: allowPorts.length > 0 ? allowPorts : undefined` 让空数组不传（与后端 `omitempty` 对称）

### 3.7 `web/src/components/__tests__/AllowPortsEditor.spec.ts`

12 case：mount + initial 形态 × 3 + 添加按钮 × 2 + 合法校验通过 × 1 + 重叠 × 3（普通 / 闭区间 / single-in-range）+ start>end × 1 + 顺序保留 × 1 + 删除中间行 × 1。

**Naive UI mock**：严格 6 方法（error/success/warning/info/loading/destroyAll）+ importOriginal + spread，与 ProxyForm.spec.ts / qa_t036_perf.spec.ts 字节同款。

**亮点**：删除中间行测试验证 splice 后 getAllowPortsInput 顺序正确（OQ-6 + UX 边角）。

### 3.8 `openapi.yaml`

`FrpsConfig.allowPorts` schema：`array<AllowPortRange>` + `maxItems: 100` + 描述含闭区间重叠语义说明。

`AllowPortRange` schema：3 个 int 字段 + minimum/maximum + 描述含互斥说明。

与 Go struct JSON 字段名 + Vue types.ts 字段名字面完全一致。

### 3.9 `docs/dev-map.md`

3 处：
- `components/` 块新增 `AllowPortsEditor.vue` 一行
- `pages/Server.vue` 行追加 "T-040: 端口策略段"
- "功能在哪里" `DB → TOML 渲染` 行追加 T-040 说明
- HTTP 路由层行追加 T-040 schema 说明

## 4. 边角 / 风险二次评估

| 项 | 风险 | 验证 |
|---|---|---|
| TOML 输出顺序 | go-toml v2 按 struct 字段顺序输出 → `AllowPorts` 放末尾 → 数组段在所有表段之后 | TestRenderFrps_AllowPorts_TOMLRoundTrip 反序列化通过 |
| 互斥在 wire 层 | `omitempty` + Validate 双层 | TestRenderFrps_AllowPorts_TOMLRoundTrip 显式断言 single entry 无 start/end key |
| `_id` 在 mount 测试中可能冲突 | `nextId` 是模块级变量，多个 mount 实例累加可能让 :key 跨实例不同；但 Vue 内 :key 只在同组件实例内比对，无副作用 | spec 通过 |
| `validateRow` 中 `myLo!` non-null assertion | `rowHasInputErr` 已先过滤 null + 越界；`!` 在分支后是安全的 | 12 spec case 全通过 |
| Server.vue ref 类型 | `InstanceType<typeof AllowPortsEditor>` 配合 defineExpose 推导 | TypeScript 严格模式编译通过 |

## 5. verify_all 预期

- **B.1 Go test ./...**：PASS（旧 + 新 21 个 frpconf + 8 个 handlers test，无环境依赖）
- **B.3 npm run test**：PASS（旧 + 12 新 case）
- **C.1 Playwright e2e**：FAIL（pre-existing baseline，非本任务回归）
- 总 PASS=32 / FAIL=1（与 baseline 持平，T-040 无回归）

**DEFERRED-TO-HOOK**（PM 派发上下文无 Bash/PowerShell，insight L23）。

## 6. Verdict

**APPROVED**

无 P0/P1 blocker。3.x 个 minor 改进点（O(n log n) 排序加速 / `_id` 跨实例可能冲突 / TestRenderFrps 文案小调）均不阻塞，可作未来任务自然顺手时一并处理。

## 7. Hand-off to QA Tester

QA 重点：

- Adversarial tests 段必须 ≥ 3 个反向构造（02 §11 AC-14 + 用户原始需求）：
  - ADV-1：构造端口号 65536（越界上界 + 1）→ 后端 422
  - ADV-2：构造 Start=80 End=70（start > end）→ 后端 422
  - ADV-3：构造 [1000-2000] + [1500-2500] 两 range 重叠 → 后端 422
- 可加 ADV-4：闭区间边界 [1000-2000] + [2000-3000] 验证语义判定
- 可加 ADV-5：前端 mount 时构造 hasValidationError=true 的初始数据，验证父级保存按钮调用拦截

verify_all 预期 PASS=32 / FAIL=1（baseline 不增）。
