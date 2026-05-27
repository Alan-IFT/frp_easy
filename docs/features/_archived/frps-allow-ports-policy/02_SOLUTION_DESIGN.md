# 02 — Solution Design · T-040 frps-allow-ports-policy

> Stage 2 / 7。架构师阶段。把 01 的 14 个 AC 映射到具体代码改动点 + 测试集 + 校验逻辑实现细节。

## 1. 设计目标

- 端口策略前后端双层守门（前端 UX + 后端硬约束）
- 复用既有路径（KV `frps.config`、`applyConfigBestEffort`、单向数据流范式）
- 不引入新依赖、不引入新 verify_all step（无删除面、无破坏性 grep 需求）

## 2. 总览（数据流）

```
┌─ Browser ──────────────────────────┐         ┌─ frp-easy backend ────────────────┐
│ Server.vue                          │  PUT    │ handlers_server.go::putServer     │
│   ├─ form (ref) — bindPort/...      │ ──────► │   ├─ FrpsConfig.AllowPorts        │
│   └─ AllowPortsEditor (ref)         │  JSON   │   ├─ validateAllowPorts()         │
│       ├─ props.initial (read once)  │         │   ├─ KVSet("frps.config")         │
│       └─ defineExpose               │         │   └─ applyConfigBestEffort("frps")│
│           .getAllowPortsInput()     │         │       └─ renderAndApplyFrps       │
└─────────────────────────────────────┘         │           └─ frpconf.RenderFrps   │
                                                │               └─ TOML w/ [[allowPorts]] │
                                                └───────────────────────────────────┘
                                                            │
                                                            ▼
                                                      frps restart (auto)
```

## 3. 后端 — `internal/frpconf/render.go`

### 3.1 新增类型

```go
// FrpsAllowPortRange 是单条 [[allowPorts]] 数组段的渲染输入。
//
// 互斥规则：
//   - Single != 0：Start 与 End 必须 0；TOML 渲染 `single = N`。
//   - Single == 0：Start ∈ [1,65535] 且 End ∈ [1,65535] 且 Start ≤ End；TOML 渲染 `start = X\nend = Y`。
//   - 三者全 0：非法（空 entry）。
type FrpsAllowPortRange struct {
    Start  int
    End    int
    Single int
}

// 内部序列化结构：用 int + omitempty。frp 上游不接受 single=0；
// 用 int + omitempty 让 0 值字段在 TOML 输出时省略，符合"Single != 0 时不写 start/end"语义。
type frpsAllowPort struct {
    Start  int `toml:"start,omitempty"`
    End    int `toml:"end,omitempty"`
    Single int `toml:"single,omitempty"`
}
```

### 3.2 `frpsRoot` struct 增字段（在末尾）

```go
type frpsRoot struct {
    BindPort   int             `toml:"bindPort"`
    Log        *frpLog         `toml:"log,omitempty"`
    Auth       *frpAuth        `toml:"auth,omitempty"`
    WebServer  *frpWebServer   `toml:"webServer,omitempty"`
    AllowPorts []frpsAllowPort `toml:"allowPorts,omitempty"` // T-040 新增；放最后让 TOML 输出符合"表段 → 数组段"惯例
}
```

### 3.3 `FrpsRenderInput` 增字段

```go
type FrpsRenderInput struct {
    BindPort         int
    AuthMethod       string
    AuthToken        string
    DashboardEnabled bool
    DashboardAddr    string
    DashboardPort    int
    DashboardUser    string
    DashboardPass    string
    LogPath          string
    LogLevel         string
    LogMaxDays       int
    AllowPorts       []FrpsAllowPortRange // T-040 新增；nil 或 空 = 不渲染 [[allowPorts]] 段
}
```

### 3.4 `RenderFrps()` 新增逻辑

在既有 dashboard/log 渲染之后追加：

```go
if len(in.AllowPorts) > 0 {
    if err := ValidateFrpsAllowPorts(in.AllowPorts); err != nil {
        return nil, err
    }
    out := make([]frpsAllowPort, 0, len(in.AllowPorts))
    for _, r := range in.AllowPorts {
        if r.Single != 0 {
            out = append(out, frpsAllowPort{Single: r.Single})
        } else {
            out = append(out, frpsAllowPort{Start: r.Start, End: r.End})
        }
    }
    root.AllowPorts = out
}
```

### 3.5 新增导出函数 `ValidateFrpsAllowPorts`

```go
// ValidateFrpsAllowPorts 校验 allowPorts 数组：
//   - 每个 entry 必须满足互斥规则（§3.1）；
//   - 每个端口必须在 [1, 65535]；
//   - Start ≤ End；
//   - 多个 entry 之间不允许重叠（闭区间语义，[1,10] 与 [10,20] 算重叠）；
//   - 数组长度 ≤ 100（OQ-1）。
//
// 返回首个错误，错误文本中含 entry 序号 + 字段，方便 UI 定位。
//
// 暴露为导出函数让 httpapi handler 调用，避免后端 PUT 时绕过 frpconf 包再校验一遍。
func ValidateFrpsAllowPorts(in []FrpsAllowPortRange) error {
    if len(in) > 100 {
        return fmt.Errorf("frpconf: allowPorts 最多 100 条（当前 %d）", len(in))
    }
    // expanded 区间用于检测 overlap：每个 entry 归一化为 [lo, hi]
    type span struct{ lo, hi, idx int }
    spans := make([]span, 0, len(in))
    for i, r := range in {
        // 互斥
        if r.Single != 0 && (r.Start != 0 || r.End != 0) {
            return fmt.Errorf("frpconf: allowPorts[%d] single 与 start/end 互斥", i)
        }
        if r.Single == 0 && r.Start == 0 && r.End == 0 {
            return fmt.Errorf("frpconf: allowPorts[%d] 必须设 single 或 start+end", i)
        }
        var lo, hi int
        if r.Single != 0 {
            if r.Single < 1 || r.Single > 65535 {
                return fmt.Errorf("frpconf: allowPorts[%d] single=%d 超出 [1,65535]", i, r.Single)
            }
            lo, hi = r.Single, r.Single
        } else {
            if r.Start < 1 || r.Start > 65535 {
                return fmt.Errorf("frpconf: allowPorts[%d] start=%d 超出 [1,65535]", i, r.Start)
            }
            if r.End < 1 || r.End > 65535 {
                return fmt.Errorf("frpconf: allowPorts[%d] end=%d 超出 [1,65535]", i, r.End)
            }
            if r.Start > r.End {
                return fmt.Errorf("frpconf: allowPorts[%d] start=%d > end=%d", i, r.Start, r.End)
            }
            lo, hi = r.Start, r.End
        }
        spans = append(spans, span{lo, hi, i})
    }
    // overlap 检测：O(n²) on n≤100；够用且语义清晰
    for i := 0; i < len(spans); i++ {
        for j := i + 1; j < len(spans); j++ {
            a, b := spans[i], spans[j]
            if a.lo <= b.hi && b.lo <= a.hi {
                return fmt.Errorf("frpconf: allowPorts[%d] 与 allowPorts[%d] 区间重叠", a.idx, b.idx)
            }
        }
    }
    return nil
}
```

### 3.6 边界 / 决策矩阵

| 输入 | 期望 | 理由 |
|---|---|---|
| `nil` | 渲染时不写 `[[allowPorts]]` | NFR-6 / AC-2；保持"留空 = 全开"语义 |
| `[]` | 同上 | go nil slice 与空 slice 在 `len()` 上等价 |
| `[{Single:80}]` | `[[allowPorts]]\n  single = 80\n` | 单端口最简形态 |
| `[{Start:6000,End:7000}]` | `[[allowPorts]]\n  start = 6000\n  end = 7000\n` | 范围最简形态 |
| `[{Start:80,End:80}]` | 渲染为 start/end 形态（不自动转为 single） | 用户填什么渲染什么，UX 一致性 |
| `[{Start:6000,End:7000},{Single:9000}]` | 两 block 按用户顺序输出 | OQ-6 顺序保留 |
| `[{Start:80,End:80,Single:80}]` | error: mutually exclusive | AC-4 |
| `[{Start:1000,End:2000},{Start:1500,End:2500}]` | error: overlapping | AC-5 |
| `[{Start:1000,End:2000},{Start:2000,End:3000}]` | error: overlapping（闭区间 2000 同属两段） | OQ-2 |
| `[{Single:1}]` / `[{Single:65535}]` | 成功 | AC-6 边界 |
| `[{Start:0,End:100}]` / `[{End:65536}]` | error: 超出范围 | AC-6 反向 |

## 4. 后端 — `internal/httpapi/handlers_server.go`

### 4.1 类型扩展

```go
type FrpsConfig struct {
    BindPort         int                  `json:"bindPort"`
    AuthMethod       string               `json:"authMethod,omitempty"`
    AuthToken        string               `json:"authToken,omitempty"`
    DashboardEnabled bool                 `json:"dashboardEnabled,omitempty"`
    DashboardAddr    string               `json:"dashboardAddr,omitempty"`
    DashboardPort    int                  `json:"dashboardPort,omitempty"`
    DashboardUser    string               `json:"dashboardUser,omitempty"`
    DashboardPass    string               `json:"dashboardPass,omitempty"`
    AllowPorts       []AllowPortRange     `json:"allowPorts,omitempty"` // T-040
}

// AllowPortRange 是 PUT/GET /api/v1/server 中 allowPorts 数组项的 JSON schema。
// 与 frpconf.FrpsAllowPortRange 一一对应（用 omitempty 让"未填"序列化为 0）。
type AllowPortRange struct {
    Start  int `json:"start,omitempty"`
    End    int `json:"end,omitempty"`
    Single int `json:"single,omitempty"`
}
```

### 4.2 `putServer` 校验集成

在既有 BindPort / DashboardPort 校验之后追加：

```go
if len(cfg.AllowPorts) > 0 {
    fcfg := make([]frpconf.FrpsAllowPortRange, len(cfg.AllowPorts))
    for i, r := range cfg.AllowPorts {
        fcfg[i] = frpconf.FrpsAllowPortRange{Start: r.Start, End: r.End, Single: r.Single}
    }
    if err := frpconf.ValidateFrpsAllowPorts(fcfg); err != nil {
        writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), "allowPorts")
        return
    }
}
```

### 4.3 `renderAndApplyFrps` 集成

在既有 cfg unmarshal 之后，构造 `FrpsRenderInput` 时拼上：

```go
in := frpconf.FrpsRenderInput{
    // ... 既有字段 ...
    AllowPorts: convertAllowPorts(cfg.AllowPorts), // 新增 helper
}
```

### 4.4 决策：last-wins 整体替换

allowPorts 是数组字段，**不**适用 T-039 的 per-field fallback 范式（insight L41 是 per-struct 替换的反模式，针对 user/pass 两字段；本任务每次 PUT 整体替换数组语义，是合理契约）。前端每次保存把当前完整数组 PUT 回，后端整存。

### 4.5 GET 行为

GET 直接返回 KV 中持久化的 `allowPorts`（与 dashboard 字段同款），不做脱敏（allowPorts 无敏感语义）。

## 5. 前端 — `web/src/components/AllowPortsEditor.vue`（新文件）

### 5.1 组件契约

| Prop / Method | 类型 | 用途 |
|---|---|---|
| props `initial: AllowPortRange[]` | 数组 | 父级保存的初始值；setup 时读一次，后续父级不再下推（单向数据流） |
| defineExpose `getAllowPortsInput(): AllowPortRange[]` | 方法 | 父级保存按钮调用，返回当前 list（按用户顺序） |
| defineExpose `hasValidationError(): boolean` | 方法 | 父级保存按钮调用，true 时不发 PUT |

**不**有 emit / v-model（继承 insight L13）。

### 5.2 内部状态

```ts
import { ref, computed, defineProps, defineExpose } from 'vue'

interface Props { initial: AllowPortRange[] }
const props = defineProps<Props>()

interface Row { kind: 'range' | 'single'; start: number | null; end: number | null; single: number | null }

// setup 时读一次 initial（单向），转 internal Row[]
const rows = ref<Row[]>(props.initial.map(r => r.single
  ? { kind: 'single', start: null, end: null, single: r.single }
  : { kind: 'range', start: r.start ?? null, end: r.end ?? null, single: null }
))

function addRange() { rows.value.push({ kind: 'range', start: null, end: null, single: null }) }
function addSingle() { rows.value.push({ kind: 'single', start: null, end: null, single: null }) }
function removeAt(i: number) { rows.value.splice(i, 1) }

// 校验：computed 每行的 error 文本（null 表 OK）
const rowErrors = computed<(string | null)[]>(() => rows.value.map((r, i) => validateRow(r, i, rows.value)))
const hasError = computed(() => rowErrors.value.some(e => e !== null))

defineExpose({
  getAllowPortsInput(): AllowPortRange[] {
    return rows.value.map(r => r.kind === 'single'
      ? { single: r.single ?? 0 }
      : { start: r.start ?? 0, end: r.end ?? 0 })
  },
  hasValidationError(): boolean { return hasError.value },
})
```

### 5.3 校验函数 `validateRow`（前端镜像后端）

```ts
function validateRow(r: Row, idx: number, all: Row[]): string | null {
  if (r.kind === 'single') {
    if (r.single === null) return '请填写端口'
    if (r.single < 1 || r.single > 65535) return '端口范围 1-65535'
  } else {
    if (r.start === null || r.end === null) return '请填写起止端口'
    if (r.start < 1 || r.start > 65535) return '起始端口 1-65535'
    if (r.end < 1 || r.end > 65535) return '结束端口 1-65535'
    if (r.start > r.end) return '起始端口必须 ≤ 结束端口'
  }
  // overlap 检测：与 idx 之前的行比对（避免双向重报）
  const myLo = r.kind === 'single' ? r.single! : r.start!
  const myHi = r.kind === 'single' ? r.single! : r.end!
  for (let j = 0; j < all.length; j++) {
    if (j === idx) continue
    const o = all[j]
    const oLo = o.kind === 'single' ? o.single : o.start
    const oHi = o.kind === 'single' ? o.single : o.end
    if (oLo === null || oHi === null) continue
    if (myLo <= oHi && oLo <= myHi) return `与第 ${j + 1} 行区间重叠`
  }
  return null
}
```

### 5.4 模板（Naive UI 组件）

```vue
<template>
  <div class="allow-ports-editor">
    <n-alert type="info" :show-icon="false" style="margin-bottom: 12px">
      留空 = 允许所有端口；配置后只允许列出范围被 frpc 申请。改动需要 frps 重启生效（自动）。
    </n-alert>
    <n-space vertical :size="8">
      <div v-for="(r, i) in rows" :key="i" class="row">
        <n-tag :type="r.kind === 'single' ? 'info' : 'success'" size="small">
          {{ r.kind === 'single' ? '单端口' : '范围' }}
        </n-tag>
        <template v-if="r.kind === 'single'">
          <n-input-number v-model:value="r.single" :min="1" :max="65535" placeholder="端口" style="width: 140px" />
        </template>
        <template v-else>
          <n-input-number v-model:value="r.start" :min="1" :max="65535" placeholder="起始" style="width: 120px" />
          <span>-</span>
          <n-input-number v-model:value="r.end" :min="1" :max="65535" placeholder="结束" style="width: 120px" />
        </template>
        <n-button size="small" tertiary type="error" @click="removeAt(i)">删除</n-button>
        <n-text v-if="rowErrors[i]" type="error" style="margin-left: 8px">{{ rowErrors[i] }}</n-text>
      </div>
      <n-space>
        <n-button size="small" @click="addRange">添加范围</n-button>
        <n-button size="small" @click="addSingle">添加单端口</n-button>
      </n-space>
    </n-space>
  </div>
</template>
```

### 5.5 SFC 规模估算（NFR insight L31）

- script 段纯逻辑行数：~50 行（Row 定义 / addRange/addSingle/removeAt / validateRow / defineExpose）
- 模板：~30 行
- 总物理：~110 行；远低于 200 行红线。

## 6. 前端 — `web/src/pages/Server.vue` 改动

### 6.1 导入 + ref

```ts
import AllowPortsEditor from '../components/AllowPortsEditor.vue'
import type { AllowPortRange } from '../types'

const allowPortsEditorRef = ref<InstanceType<typeof AllowPortsEditor> | null>(null)
const initialAllowPorts = ref<AllowPortRange[]>([])  // 种子，从 loadConfig 写入
```

### 6.2 `loadConfig` 改动

```ts
async function loadConfig(reveal = false) {
  try {
    const cfg = await apiGetServer(reveal)
    // ... 既有字段填充 ...
    initialAllowPorts.value = cfg.allowPorts ?? []
  } catch (e) { /* ... */ }
}
```

### 6.3 `handleSave` 改动

```ts
async function handleSave() {
  try { await formRef.value?.validate() } catch { return }

  // T-040：从 AllowPortsEditor 拉当前值 + 校验
  const allowPorts = allowPortsEditorRef.value?.getAllowPortsInput() ?? []
  if (allowPortsEditorRef.value?.hasValidationError()) {
    message.error('端口策略存在非法项，请修复后再保存')
    return
  }

  saving.value = true
  try {
    await apiPutServer({
      // ... 既有字段 ...
      allowPorts,
    })
    // ...
  }
}
```

### 6.4 模板新增段（在既有 dashboard 段之后）

```vue
<n-form-item label="端口策略" :show-feedback="false" style="margin-top: 16px">
  <allow-ports-editor :initial="initialAllowPorts" ref="allowPortsEditorRef" />
</n-form-item>
```

### 6.5 `web/src/types.ts` 新增

```ts
export interface AllowPortRange {
  start?: number
  end?: number
  single?: number
}

export interface FrpsConfig {
  // ... 既有字段 ...
  allowPorts?: AllowPortRange[]
}
```

## 7. 测试集

### 7.1 后端 — `internal/frpconf/render_test.go` 新增

| Test | 覆盖 AC |
|---|---|
| `TestRenderFrps_AllowPorts_Empty` | AC-2（nil + [] 不渲染段） |
| `TestRenderFrps_AllowPorts_SingleRange` | AC-1（[{6000-7000}]） |
| `TestRenderFrps_AllowPorts_MultiRange` | OQ-6 顺序保留（两 range + 一 single） |
| `TestRenderFrps_AllowPorts_SingleOnly` | AC-1（[{Single:9000}]） |
| `TestRenderFrps_AllowPorts_BoundaryMinMax` | AC-6（1 / 65535） |
| `TestRenderFrps_AllowPorts_StartGreaterThanEnd` | AC-3 |
| `TestRenderFrps_AllowPorts_Mutex` | AC-4（Single + Start 同设） |
| `TestRenderFrps_AllowPorts_Overlap` | AC-5（[1000-2000] vs [1500-2500]） |
| `TestRenderFrps_AllowPorts_OverlapBoundary` | OQ-2（[1000-2000] vs [2000-3000]） |
| `TestRenderFrps_AllowPorts_OutOfRange` | AC-6 反向（0 / 65536） |
| `TestRenderFrps_AllowPorts_TooMany` | OQ-1（101 条） |

### 7.2 后端 — `internal/httpapi/handlers_server_test.go`（新文件）

| Test | 覆盖 |
|---|---|
| `TestPutServer_AllowPorts_Valid` | 200 + KV 持久化字面 |
| `TestPutServer_AllowPorts_OutOfRange` | AC-7（422 + field="allowPorts"） |
| `TestPutServer_AllowPorts_StartGreaterThanEnd` | 422 |
| `TestPutServer_AllowPorts_Overlap` | 422 |
| `TestPutServer_AllowPorts_Mutex` | 422 |
| `TestGetServer_AllowPorts_RoundTrip` | PUT 后 GET 数组等值 |
| `TestPutServer_AllowPorts_Empty` | 200 + KV 字面无 allowPorts |

### 7.3 前端 — `web/src/components/__tests__/AllowPortsEditor.spec.ts`（新文件）

| Test | 覆盖 AC |
|---|---|
| `mount 空 initial → 0 行 + 两个添加按钮` | 渲染基础 |
| `点添加范围 → 列表 +1 行 kind=range` | AC-9 |
| `点添加单端口 → 列表 +1 行 kind=single` | AC-10 |
| `输入 end=65536 → 显红色错误文案 "结束端口 1-65535"` | AC-9 |
| `两行 [1000-2000] + [1500-2500] → 第二行显 "与第 1 行区间重叠"` | AC-9 + OQ-2 |
| `getAllowPortsInput() 返回顺序与添加顺序一致` | AC-11 + OQ-6 |
| `hasValidationError() 在有非法行时返 true` | NFR-5 |
| `initial=[{single:80}] → 第一行 kind=single value=80` | 单向数据流 |

Naive UI mock 严格遵循 insight L9/L14：importOriginal + spread + 6 方法 stub。

### 7.4 不引入新 e2e（C.1 已知 baseline FAIL 豁免）

## 8. OpenAPI 同步

`openapi.yaml` 中 `FrpsConfig` schema 追加：

```yaml
allowPorts:
  type: array
  description: |
    端口策略白名单（T-040）。每个 entry 必须含 single 或 start+end（互斥）；
    所有端口 ∈ [1,65535]；start ≤ end；entry 之间不允许区间重叠（闭区间语义）；
    数组长度 ≤ 100；留空 = 允许所有端口。
  items:
    $ref: '#/components/schemas/AllowPortRange'
  maxItems: 100
```

新 schema：

```yaml
AllowPortRange:
  type: object
  description: 端口策略单条 entry（T-040）
  properties:
    single:
      type: integer
      minimum: 1
      maximum: 65535
      description: 单端口；与 start/end 互斥
    start:
      type: integer
      minimum: 1
      maximum: 65535
      description: 范围起始端口；与 single 互斥
    end:
      type: integer
      minimum: 1
      maximum: 65535
      description: 范围结束端口；start ≤ end
```

## 9. dev-map.md 同步

- "目录布局" 段 `components/` 块追加 `AllowPortsEditor.vue ← T-040: 端口策略编辑器（单端口 / 范围混合 list；defineExpose getAllowPortsInput）`
- "目录布局" 段 `pages/Server.vue` 行追加 "T-040: 新增 AllowPortsEditor 段"
- "功能在哪里" 段 `DB → TOML 渲染` 行追加 "T-040: RenderFrps 支持 allowPorts"

## 10. Partition assignment

**Single Developer mode**（项目无 `.harness/agents/dev-*.md`，使用 generic developer）。改动跨 backend (Go) + frontend (Vue/TS)，但单 agent 顺序处理：

1. backend: frpconf/render.go + validate
2. backend: handlers_server.go + types
3. backend: render_test.go + handlers_server_test.go
4. frontend: types.ts + AllowPortsEditor.vue + Server.vue 集成
5. frontend: AllowPortsEditor.spec.ts
6. openapi.yaml + dev-map.md
7. verify_all 跑一遍

## 11. Verify_all 影响

**不新增** verify_all step（与 T-039 节奏对齐，避免不必要 grep-based 闸门）。AC-13 守门：FAIL 数不超过 baseline=1。

## 12. 风险消化路径

| R | 风险 | 设计应对 |
|---|---|---|
| R-1 | 重叠语义 | §3.5 闭区间 `lo <= b.hi && b.lo <= a.hi`；§5.3 前端镜像 |
| R-2 | Single + Start 同设 | §3.5 + §4.2 双层校验 + §5.1 两按钮分流让 UI 不可能同时填 |
| R-3 | TOML 段顺序 | §3.2 frpsRoot AllowPorts 放最后 |
| R-4 | Vitest mount Naive UI | §7.3 严格 importOriginal + 6 方法 stub |
| R-5 | KV 膨胀 | §3.5 上限 100 + §8 OpenAPI maxItems=100 |
| R-6 | 与 T-041/T-042 冲突 | 改动域无重叠（T-041 新页面、T-042 改 Proxies.vue） |

## 13. Hand-off to Gate Reviewer

Gate Reviewer 重点核：

- §3 校验逻辑边界（重叠闭区间、互斥、上限）是否覆盖所有 OQ
- §4 后端是否实现"前端绕过场景"反向构造守门
- §5 前端是否严格继承 T-032 单向数据流范式（无 emit、无 v-model）
- §7 测试矩阵是否覆盖全部 14 个 AC
- §8 OpenAPI schema 与 §4 types 字面一致

**Verdict**: READY FOR GATE REVIEW.
