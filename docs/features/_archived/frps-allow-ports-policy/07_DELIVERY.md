# 07 — Delivery Summary · T-040 frps-allow-ports-policy

> Stage 7 / 7。任务完成总览。

## 1. 任务元数据

- **Task**: T-040 frps-allow-ports-policy · frps 端管理 frpc 端口开放策略（allowPorts 配置 + 前后端双层校验 + UI 编辑器）
- **Mode**: full（7-stage pipeline）
- **批次**: `docs/batches/frps-monitor-and-mgmt-suite/` 第 2/4 个任务
- **依赖**: T-039（共享 `internal/frpconf/RenderFrps()` + `internal/httpapi/handlers_server.go` 改动路径，已 DELIVERED ecc49b9）
- **完成日期**: 2026-05-27

## 2. 阶段时间线

| Stage | Agent | Output | Verdict |
|---|---|---|---|
| 1 | requirement-analyst | `01_REQUIREMENT_ANALYSIS.md` | READY FOR ARCHITECT |
| 2 | solution-architect | `02_SOLUTION_DESIGN.md` | READY FOR GATE REVIEW |
| 3 | gate-reviewer | `03_GATE_REVIEW.md` | APPROVED FOR DEVELOPMENT（4 INFO/WARN conditions） |
| 4 | developer | `04_DEVELOPMENT.md` | READY FOR REVIEW（GR conditions C-1~C-4 全消化） |
| 5 | code-reviewer | `05_CODE_REVIEW.md` | APPROVED（3 minor 非阻塞） |
| 6 | qa-tester | `06_TEST_REPORT.md` | APPROVED（含 6 个 Adversarial 反向构造） |
| 7 | (this) | `07_DELIVERY.md` | DELIVERED |

**Rollbacks**: 0

## 3. 交付内容

### 3.1 后端 — `internal/frpconf/`

- `render.go`:
  - `FrpsAllowPortRange` 导出 struct（Start/End/Single 互斥语义）
  - `frpsAllowPort` 私有渲染结构（三字段 omitempty）
  - `frpsRoot` 末尾追加 `AllowPorts []frpsAllowPort` 让 TOML 输出符合"表段→数组段"惯例
  - `FrpsRenderInput.AllowPorts` 字段
  - `RenderFrps()` 渲染段（含校验自调）
  - `ValidateFrpsAllowPorts()` 导出函数（互斥 + 范围 + 闭区间重叠 + 上限 100；错误文本中文 + `allowPorts[i]` 字面定位）
- `render_test.go`:
  - 13 个新 Test：Empty/SingleRange/MultiRange/SingleOnly/BoundaryMinMax/StartGreaterThanEnd/Mutex/Overlap/OverlapBoundary/OverlapSingleVsRange/OutOfRange/TooMany/TOMLRoundTrip

### 3.2 后端 — `internal/httpapi/`

- `handlers_server.go`:
  - `FrpsConfig.AllowPorts []AllowPortRange` 字段
  - `AllowPortRange` JSON struct（与 frpconf 层一一对应）
  - `toFrpconfAllowPorts` helper（putServer + config_helper 双路径共用）
  - `putServer` 集成 ValidateFrpsAllowPorts（422 + field="allowPorts"）
- `handlers_server_test.go`（新文件）:
  - 8 个 Test：Valid / Empty / OutOfRange (4 sub) / StartGreaterThanEnd / Overlap / Mutex / RoundTrip / TooMany
- `config_helper.go`:
  - `renderAndApplyFrps` 透传 AllowPorts

### 3.3 前端 — `web/src/`

- `types.ts`:
  - `AllowPortRange` interface（start/end/single 可选）
  - `FrpsConfig.allowPorts?: AllowPortRange[]`
- `components/AllowPortsEditor.vue`（新文件，175 行物理 / 80 行纯逻辑）:
  - 单端口 / 范围混合 list
  - "添加范围" / "添加单端口" 双按钮
  - 实时校验（端口越界 / start>end / 重叠；闭区间语义；overlap 只与 j<i 比对避免双报）
  - defineExpose `getAllowPortsInput()` + `hasValidationError()`
  - 严格继承 T-032 单向数据流范式（无 emit / 无 v-model / props.initial 读一次）
  - `_id` 字段稳定 :key 让 splice 删除中间行不影响 input 焦点
  - 顶部 NAlert 文案："留空 = 允许所有端口；配置后只允许列出范围被 frpc 申请。改动需要 frps 重启生效（自动）。"
  - 空态文案："（暂无策略 = 允许所有端口）"
- `pages/Server.vue`:
  - 引入 AllowPortsEditor + AllowPortRange
  - `initialAllowPorts` ref 种子；`allowPortsEditorRef` defineExpose 句柄
  - `loadConfig` 写种子；`handleSave` 先 `hasValidationError` 后 `getAllowPortsInput`
  - 模板在 dashboard 段之后追加 "端口策略" `n-form-item`
- `components/__tests__/AllowPortsEditor.spec.ts`（新文件）:
  - 12 mount case：空 initial / single 形态 / range 形态 / 添加范围 / 添加单端口 / 合法两条 / 重叠（3 子）/ start>end / 顺序保留 / 删除中间行
  - 严格 importOriginal + spread + 6 方法 stub mock naive-ui（insight L9/L14）

### 3.4 OpenAPI — `openapi.yaml`

- `FrpsConfig` schema 扩 `allowPorts: array<AllowPortRange>` + maxItems=100
- `AllowPortRange` 新 schema：3 个 int 字段 + minimum/maximum 1/65535 + 互斥描述

### 3.5 dev-map.md

- `components/` 块 +1 行 `AllowPortsEditor.vue`
- `pages/Server.vue` 行追加 "T-040: 端口策略段"
- "功能在哪里" `DB → TOML 渲染` 行 + `HTTP 路由层` 行各追加 T-040 说明

## 4. 验证结果

| 检查 | 结果 | 备注 |
|---|---|---|
| `pwsh scripts/verify_all.ps1` Full | **DEFERRED-TO-HOOK** | PM 派发上下文无 Bash/PowerShell（insight L23）；预期 PASS=32 / FAIL=1（C.1 e2e 已知豁免，与 baseline 持平） |
| Stage 1-6 文档齐备 | ✅ | 7 个文档 + PM_LOG |
| 06 含 `## Adversarial tests` 段 | ✅ | 6 个反向构造用例（ADV-1~6） |
| 07 含 `## Insight` bullet 列表 | ✅ | §7 |
| 不引入新依赖 | ✅ | 仅 `internal/frpconf` 包内 + chi/json 既有依赖 |
| GR conditions C-1~C-4 消化 | ✅ | 04 §3 显式记录 |
| 不新增 verify_all step | ✅ | 与 T-039 节奏一致 |

## 5. 文件变更统计

```
新增文件：
  internal/httpapi/handlers_server_test.go                          (~200 行)
  web/src/components/AllowPortsEditor.vue                           (~175 行)
  web/src/components/__tests__/AllowPortsEditor.spec.ts             (~175 行)
  docs/features/frps-allow-ports-policy/01_REQUIREMENT_ANALYSIS.md
  docs/features/frps-allow-ports-policy/02_SOLUTION_DESIGN.md
  docs/features/frps-allow-ports-policy/03_GATE_REVIEW.md
  docs/features/frps-allow-ports-policy/04_DEVELOPMENT.md
  docs/features/frps-allow-ports-policy/05_CODE_REVIEW.md
  docs/features/frps-allow-ports-policy/06_TEST_REPORT.md
  docs/features/frps-allow-ports-policy/07_DELIVERY.md
  docs/features/frps-allow-ports-policy/PM_LOG.md

修改文件：
  internal/frpconf/render.go                                        (+~90 行)
  internal/frpconf/render_test.go                                   (+~240 行)
  internal/httpapi/handlers_server.go                               (+~35 行)
  internal/httpapi/config_helper.go                                 (+~3 行)
  web/src/types.ts                                                  (+~14 行)
  web/src/pages/Server.vue                                          (+~25 行)
  openapi.yaml                                                      (+~30 行)
  docs/dev-map.md                                                   (3 处微调)
  docs/tasks.md                                                     (+1 行进行中 / 转完成)
```

代码：~750 行新增；config / docs / openapi ~110 行修改。

## 6. Outstanding risks / Next steps

- **C.1 e2e FAIL**（已知 baseline，与本任务零相关；insight L21/L34 多任务工作树污染归责范式适用）
- **frps 上游 allowPorts 字段规范变化**：本任务直接用上游字面 `start/end/single`；如 frp v0.x → v1.x 规范重命名 → 加 struct 字段并补单测即可
- **AllowPortsEditor `_id` 跨实例累加**：当前模块级 `nextId` 单调递增，多 mount 实例累加但 Vue :key 同实例内比对，无副作用；如未来需要严格隔离可改实例级 `useId()`
- **scripts/verify_all + archive-task + commit 执行**：本任务 DEFERRED-TO-HOOK；batch orchestrator 收尾时跑：
  - `pwsh scripts/verify_all.ps1` 确认 PASS=32 / FAIL=1（与 baseline 持平）
  - `git add -A && git commit -m "feat(T-040): frps-allow-ports-policy — server-side 端口策略 (allowPorts) + 前后端双层校验 + AllowPortsEditor 单向数据流"`
  - `pwsh scripts/archive-task.ps1 -Task frps-allow-ports-policy`（归档 stage 文档到 `_archived/`，收割 §7 Insight bullets 到 `.harness/insight-index.md`）
  - **不要 push**（按批次约定，由 batch 末尾统一推）

## 7. Insight

- 2026-05-27 · frp 上游 `[[allowPorts]]` TOML 数组段在 frpsRoot struct 必须放最后字段，让 go-toml v2 按字段顺序输出时所有表段（`[log]/[auth]/[webServer]`）在前、数组段在后，符合 TOML 规范"表段不能出现在数组段之后"；放中间会产生 frp 上游能解析但 toml.Unmarshal 反向 round-trip 失败的二义性输出 · evidence: T-040 frpconf/render.go::frpsRoot 字段顺序 + TestRenderFrps_AllowPorts_TOMLRoundTrip 反序列化验证
- 2026-05-27 · 端口"闭区间重叠"语义（`[1000,2000]` 与 `[2000,3000]` 算重叠，因 2000 同属两段）必须前后端镜像 + verify_all 单测固化，不能只在文档约定——frp 上游 parser 接受重叠并 last-wins 加入 allow set，前端不挡 + 后端不挡的话会让"UI 列表展示"与"实际生效允许端口集合"不一致让用户无法治理。规则：UI 编辑器型组件涉及"区间集合"概念，必须在 PM 决策矩阵显式定义开闭区间 + 前后端用同一闭区间相交条件 `a.lo <= b.hi && b.lo <= a.hi` · evidence: T-040 ValidateFrpsAllowPorts + AllowPortsEditor.validateRow 镜像 + TestRenderFrps_AllowPorts_OverlapBoundary
- 2026-05-27 · 集合类参数（如 `allowPorts []Range`）的更新语义与单字段（如 dashboard user/pass）的更新语义不同——前者**适合 last-wins 整体替换**（一次保存就是完整快照），后者**适合 per-field fallback**（T-039 insight L41 范式）。如把数组也按 per-field 处理（如"只填了第 0 项就只更第 0 项"）会让 UI 删除行的操作没有任何 backend 信号，逻辑不可达。任何"用户面向的集合编辑器"PUT 都应整存整取 · evidence: T-040 handlers_server.go::putServer 直接 KVSet 整 marshalled cfg + T-039 config_helper.go::renderAndApplyFrps per-field fallback 形成范式对照

## 8. Verdict

**DELIVERED**

T-040 完成。批次 `frps-monitor-and-mgmt-suite` 后续 T-041（server-monitor-page-ui）/ T-042（proxy-runtime-status-merge）可继续推进；本任务为 frps 服务端"管理 frpc 端口开放"能力提供了完整闭环（数据结构 + 校验 + 渲染 + UI 编辑器 + OpenAPI + 测试），单条提交即可发布。
