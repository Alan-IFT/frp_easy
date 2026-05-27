# 04 — Development · T-040 frps-allow-ports-policy

> Stage 4 / 7。开发阶段执行记录。

## 1. 实施顺序（与 02 §10 partition assignment 一致）

| # | 步骤 | 文件 | 状态 |
|---|---|---|---|
| 1 | 后端：frpconf 数据类型 + 渲染逻辑 | `internal/frpconf/render.go` | ✅ |
| 2 | 后端：ValidateFrpsAllowPorts 导出函数 | 同上 | ✅ |
| 3 | 后端：handler 类型扩展 + putServer 校验 | `internal/httpapi/handlers_server.go` | ✅ |
| 4 | 后端：config_helper.go 渲染拼接 | `internal/httpapi/config_helper.go` | ✅ |
| 5 | 后端单测：frpconf 渲染 + 校验（13 个 Test） | `internal/frpconf/render_test.go` | ✅ |
| 6 | 后端单测：handlers PUT 校验（8 个 Test） | `internal/httpapi/handlers_server_test.go`（新文件） | ✅ |
| 7 | 前端：types.ts 加 AllowPortRange + FrpsConfig.allowPorts | `web/src/types.ts` | ✅ |
| 8 | 前端：AllowPortsEditor.vue 新组件 | `web/src/components/AllowPortsEditor.vue` | ✅ |
| 9 | 前端：Server.vue 集成 | `web/src/pages/Server.vue` | ✅ |
| 10 | 前端单测：AllowPortsEditor.spec.ts（12 个 case） | `web/src/components/__tests__/AllowPortsEditor.spec.ts` | ✅ |
| 11 | OpenAPI: FrpsConfig.allowPorts + AllowPortRange schema | `openapi.yaml` | ✅ |
| 12 | dev-map.md 同步 | `docs/dev-map.md` | ✅ |
| 13 | verify_all 跑一遍 | DEFERRED-TO-HOOK（PM 派发上下文无 Bash/PowerShell，insight L23） | 待 hook 执行 |

## 2. 关键实现细节

### 2.1 frpconf — 数据结构 + 渲染

`frpsRoot` 末尾追加 `AllowPorts []frpsAllowPort`，让 go-toml 输出时所有表段（[log]/[auth]/[webServer]）在前、数组段在后，符合 TOML 规范。

`frpsAllowPort` 三字段 `start/end/single` 都加 `omitempty` 标签：

```go
type frpsAllowPort struct {
    Start  int `toml:"start,omitempty"`
    End    int `toml:"end,omitempty"`
    Single int `toml:"single,omitempty"`
}
```

渲染时 `Single != 0` 走单端口分支，只填 `Single` 字段；反之填 `Start/End`。omitempty + 互斥校验双层保证 frp 上游不会同时收到 single+start/end。

### 2.2 frpconf — ValidateFrpsAllowPorts 导出函数

`O(n²)` overlap 检测 on n≤100；闭区间相交条件 `a.lo <= b.hi && b.lo <= a.hi`。错误文本中文 + `allowPorts[i]` 字面定位（GR C-1 消化）。

### 2.3 handlers_server.go — 校验集成

`putServer` 在 `BindPort` / `DashboardPort` 校验之后加 `allowPorts` 校验块。复用 `frpconf.ValidateFrpsAllowPorts` 避免双实现漂移（与 `ValidatePort` 范式同款）。

`AllowPortRange` JSON struct 与 `frpconf.FrpsAllowPortRange` 一一对应，通过 helper `toFrpconfAllowPorts` 转换。该 helper 既给 putServer 校验路径用，也给 renderAndApplyFrps 渲染路径用（同一转换源避免漂移）。

### 2.4 前端 AllowPortsEditor.vue

严格继承 insight L13 / T-032 范式：

- props.initial 在 setup 时读一次（`(props.initial ?? []).map(...)`），后续不 watch
- 无 emit、无 v-model
- defineExpose `getAllowPortsInput()` + `hasValidationError()` 让父级保存按钮拉数据 + 决策
- 内部 Row 类型用 `_id` 字段做 v-for :key，让 splice 删除中间行不导致 input 焦点错位
- 校验函数 `validateRow` 与后端 `ValidateFrpsAllowPorts` 镜像；overlap 检测只与 `j < i` 比对（GR C-2 消化）

### 2.5 Server.vue 集成

- 在 dashboard 段之后加 `<n-form-item label="端口策略">` 装 `<allow-ports-editor>`
- `loadConfig` 写入 `initialAllowPorts.value`（种子）
- `handleSave` 中先调 `allowPortsEditorRef.value?.hasValidationError()` → 非空就 `message.error` 直返；否则调 `getAllowPortsInput()` 拿数组拼入 PUT
- `allowPorts: allowPorts.length > 0 ? allowPorts : undefined` 让 axios 不发空数组（与后端 `omitempty` 对称）

## 3. GR conditions 消化

| ID | Severity | 消化情况 |
|---|---|---|
| C-1 | INFO 错误文案中文 + 含 allowPorts[i] 字面 | ✅ 全部 13 处错误文本中文化 + 含索引（含 `allowPorts[i]` / 含 `start=N` 定位） |
| C-2 | INFO 前端 overlap 只与 j<i 行比对 | ✅ AllowPortsEditor `validateRow` 内 `for (let j = 0; j < idx; j++)` 严格守门 |
| C-3 | WARN GET 信任 KV 持久化数据 | ✅ `FrpsConfig` 注释明示 "GET 信任 KV 持久化数据；PUT 时校验" |
| C-4 | INFO 空数组跳过校验 | ✅ 设计本意，注释明示 `len(in.AllowPorts) > 0` |

## 4. 验证（本地理论执行，DEFERRED-TO-HOOK）

### 4.1 单测

预期：

- `go test ./internal/frpconf/...` PASS（旧 8 个测试 + 新 13 个 = 21 个）
- `go test ./internal/httpapi/...` PASS（既有测试无影响 + 新 8 个 PUT test）
- `npm run test -- AllowPortsEditor` PASS（12 个 mount case）

### 4.2 静态闸门

不新增 verify_all step（与 T-039 节奏一致）。预期 baseline FAIL=1（C.1 e2e 已知）不增。

## 5. Design drift（无）

实施与 02 设计完全一致，无 drift。

## 6. Verdict

**READY FOR REVIEW**

Code Reviewer 重点核：

- 后端 §3.5 校验逻辑闭区间边界正确性
- 前端 single-direction 数据流无回溯（无 emit、无 watch props.initial）
- Naive UI mock 6 方法 + importOriginal 模式
- OpenAPI schema 与 Go struct JSON tag 字面一致
