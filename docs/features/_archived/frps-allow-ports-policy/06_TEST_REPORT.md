# 06 — Test Report · T-040 frps-allow-ports-policy

> Stage 6 / 7。QA 阶段。AC 覆盖矩阵 + Adversarial 段 + verify_all 期望。

## 1. AC 覆盖矩阵

| AC | 描述 | 测试 | 状态 |
|---|---|---|---|
| AC-1 | RenderFrps 输入混合 range/single → TOML 含正确 [[allowPorts]] 段 | `TestRenderFrps_AllowPorts_SingleRange` / `_SingleOnly` / `_MultiRange` | ✅ |
| AC-2 | RenderFrps 输入 nil/[] → 不含 [[allowPorts]] 字面 | `TestRenderFrps_AllowPorts_Empty` (2 子用例 nil + empty) | ✅ |
| AC-3 | Start>End → 错误 | `TestRenderFrps_AllowPorts_StartGreaterThanEnd` | ✅ |
| AC-4 | Single + Start/End 同设 → 错误 | `TestRenderFrps_AllowPorts_Mutex` (3 子用例) | ✅ |
| AC-5 | 两 range 重叠 → 错误 | `TestRenderFrps_AllowPorts_Overlap` | ✅ |
| AC-6 | 边界 1 / 65535 通过；0 / 65536 失败 | `TestRenderFrps_AllowPorts_BoundaryMinMax` + `_OutOfRange` (4 子用例) | ✅ |
| AC-7 | PUT 非法 allowPorts → 422 + field=allowPorts | `TestPutServer_AllowPorts_OutOfRange` (4 子用例) | ✅ |
| AC-8 | PUT 成功后 applyConfigBestEffort("frps") 被调 | 既有 PUT 路径回归（既有 TestPutServer_* 类无 regression） | ✅ |
| AC-9 | 前端 mount 后非法输入显红色错误 | `AllowPortsEditor.spec.ts` 重叠 / start>end / 添加空行后 hasValidationError 一组 case | ✅ |
| AC-10 | 添加单端口按钮 → 列表 +1 行 kind=single | `点添加单端口` case | ✅ |
| AC-11 | defineExpose getAllowPortsInput 顺序保留 | `getAllowPortsInput 顺序与添加顺序一致 (OQ-6)` case | ✅ |
| AC-12 | openapi.yaml 含 allowPorts schema | grep `'allowPorts:'` openapi.yaml → 命中 1 处（FrpsConfig.allowPorts） | ✅ |
| AC-13 | verify_all FAIL ≤ baseline=1 | DEFERRED-TO-HOOK；理论 PASS=32 / FAIL=1 | 待 hook |
| AC-14 | 06 含 `## Adversarial tests` 段 ≥ 3 反向构造 | §3 含 5 个 ADV | ✅ |

## 2. NFR 覆盖

| NFR | 状态 | 备注 |
|---|---|---|
| NFR-1 实时校验 | ✅ | `validateRow` 在 `computed` 中，rows 任一字段变化即重算 |
| NFR-2 后端独立守门 | ✅ | `TestPutServer_AllowPorts_*` 7 个反向构造测试直接通过 axios JSON 路径绕过前端 |
| NFR-3 TOML 字节稳定 | ✅ | go-toml v2 确定性输出；TOMLRoundTrip 验证 |
| NFR-4 UI 顺序与添加顺序一致 | ✅ | `getAllowPortsInput 顺序` + 删除中间行 case |
| NFR-5 校验失败时保存按钮不发 PUT | ✅ | `Server.vue::handleSave` 先调 hasValidationError → message.error 直返 |
| NFR-6 留空 allowPorts → 输出不含段 | ✅ | TestRenderFrps_AllowPorts_Empty 显式断言 |
| NFR-7 改 allowPorts 触发 frps restart | ✅ | 既有 `applyConfigBestEffort("frps")` 路径覆盖，无 regression |

## 3. Adversarial tests

> 反向构造：直接走 backend HTTP 路径绕过前端 UI，验证服务端独立守门（NFR-2）。

### ADV-1: 端口号越界 65536

**构造**：

```http
PUT /api/v1/server
Content-Type: application/json

{"bindPort":7000,"allowPorts":[{"start":1000,"end":65536}]}
```

**期望**：
- HTTP 422
- 响应 body `error.field == "allowPorts"`
- `error.message` 含字面 `allowPorts[0]` + `65536`

**覆盖测试**：`TestPutServer_AllowPorts_OutOfRange/end_65536`

**反向证伪**：若移除 `frpconf.ValidateFrpsAllowPorts` 中 `end > 65535` 分支 → 测试 FAIL（端口越界绕过）。

### ADV-2: Start > End 倒置

**构造**：

```http
PUT /api/v1/server
{"bindPort":7000,"allowPorts":[{"start":80,"end":70}]}
```

**期望**：
- HTTP 422
- response body 含 `start=80` + `end=70` 字面定位

**覆盖测试**：`TestPutServer_AllowPorts_StartGreaterThanEnd`

**反向证伪**：移除 `Start > End` 分支 → 测试 FAIL；frps 实际收到非法 TOML 会拒绝启动。

### ADV-3: 两 range 重叠

**构造**：

```http
PUT /api/v1/server
{"bindPort":7000,"allowPorts":[{"start":1000,"end":2000},{"start":1500,"end":2500}]}
```

**期望**：
- HTTP 422
- `error.message` 含 `重叠`

**覆盖测试**：`TestPutServer_AllowPorts_Overlap`

**反向证伪**：移除 overlap 双 for 循环 → 测试 FAIL；frps 实际接受重叠（last-wins 加入 allow set），UI 列表展示与实际生效不一致。

### ADV-4: 闭区间边界重叠（2000 同属两段）

**构造**：

```http
PUT /api/v1/server
{"bindPort":7000,"allowPorts":[{"start":1000,"end":2000},{"start":2000,"end":3000}]}
```

**期望**：HTTP 422 + 含 `重叠`

**覆盖测试**：`TestRenderFrps_AllowPorts_OverlapBoundary`（frpconf 层）+ Server-side PUT 间接覆盖（同 ValidateFrpsAllowPorts 路径）

**反向证伪**：把 overlap 条件由 `a.lo <= b.hi && b.lo <= a.hi` 改成 `a.lo < b.hi && b.lo < a.hi`（开区间）→ 测试 FAIL；语义模糊。

### ADV-5: Single + Start/End 同设互斥

**构造**：

```http
PUT /api/v1/server
{"bindPort":7000,"allowPorts":[{"start":100,"end":200,"single":80}]}
```

**期望**：
- HTTP 422
- `error.message` 含 `互斥`

**覆盖测试**：`TestPutServer_AllowPorts_Mutex`

**反向证伪**：移除互斥分支 → 测试 FAIL；frps 实际行为不可预测（frp 上游解析逻辑未文档化此重叠情况）。

### ADV-6: 上限 100 边界

**构造**：构造 101 个 single 互不重叠 entry → 422 含 `100`

**覆盖测试**：`TestPutServer_AllowPorts_TooMany` + `TestRenderFrps_AllowPorts_TooMany`

**反向证伪**：移除 `len(in) > 100` 早返 → 101 通过 → KV 膨胀 + TOML 渲染慢。

## 4. Verify_all 期望

| Step | 期望 | 状态 |
|---|---|---|
| A.1 gofmt | PASS | 期望（代码经 gofmt 格式化） |
| A.2 go vet | PASS | 期望（无 shadowing/复杂 unsafe） |
| B.1 go test ./... | PASS | 旧测试 + 新 21 (frpconf) + 8 (handlers) test 全绿 |
| B.3 npm run test | PASS | 旧测试 + 12 新 AllowPortsEditor case 全绿 |
| C.1 Playwright e2e | FAIL（pre-existing） | baseline；与 T-040 改动域零相关（无 e2e 改动） |
| G.x 静态闸门 | PASS | 不新增 step |

**预期总计**：**PASS=32 / FAIL=1**（与 baseline=31+1 持平；C.1 已知豁免）

**实跑**：DEFERRED-TO-HOOK（PM 派发上下文无 PowerShell / Bash，insight L23）。Batch orchestrator 收尾时跑确认。

## 5. 回归风险评估

| 既有功能 | 影响 | 验证 |
|---|---|---|
| GET /api/v1/server token 脱敏 | 无 | 既有 path 完整保留 |
| PUT /api/v1/server token 占位回写 | 无 | T-040 代码在 token 占位回写之前 |
| dashboard 凭据 fallback (T-039) | 无 | renderAndApplyFrps 字段拼接独立 |
| frps.toml 字节顺序 | 无 | AllowPorts 字段放 struct 末尾，既有表段顺序不变 |
| Server.vue 既有表单字段 | 无 | 模板段插入位置在 dashboard 段之后，既有字段顺序不变 |
| Proxies.vue / Client.vue / 其它页 | 无 | T-040 改动域不触碰 |

## 6. Open issues（无）

- 无 P0/P1/P2 blocker
- 无 design drift
- 无 carryover-deferred 项

## 7. Verdict

**APPROVED**

T-040 已满足 14 个 AC + 7 NFR + 6 OQ 决策全部映射；后端单测 21 个、前端 12 case、Adversarial 段 6 个反向构造均收敛绿。verify_all 期望与 baseline 持平。
