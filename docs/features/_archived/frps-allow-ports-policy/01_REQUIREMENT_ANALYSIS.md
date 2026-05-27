# 01 — Requirement Analysis · T-040 frps-allow-ports-policy

> Stage 1 / 7。需求分析师阶段。把"管理 frpc 端口开放"高层意图收敛到 frp 上游 `allowPorts` 语义 + 项目落点。

## 1. 业务目标

用户在 **批次 `frps-monitor-and-mgmt-suite`** 中明确要求"frps 能管理 frpc 的端口开放"。对照 FRP 上游语义，该能力 = frps.toml 中的 `allowPorts` 配置段：

> 限制 frpc 申请的"远程端口"必须落在白名单范围内；frpc 在 `RegisterProxy` 时若 remotePort 不在白名单 → frps 直接拒绝并回 client 错误 `remote port out of range`。

业务价值：

- **安全收敛**：服务端运维人员可以把"远端口暴露面"硬约束到一个明确范围（如 6000-7999、22 单端口），杜绝 client 端误开高危端口（如 0-1024 系统端口、3389 RDP）
- **多用户场景**：单台 frps 给多组 frpc 共享时，可按段划分（如 6000-6999 给 A 团队、7000-7999 给 B 团队，配合多 client 的端口段约定）
- **零信任配套**：与 `authToken`（已有）+ TLS（frp 默认）+ Dashboard 限 127.0.0.1（已有）形成完整 server-side 守门链路

## 2. 范围（IN / OUT）

### IN（本任务交付）

| 编号 | 功能 | 落点 |
|---|---|---|
| FR-1 | 后端数据结构 `FrpsAllowPortRange{Start, End, Single int}` + `FrpsRenderInput.AllowPorts []FrpsAllowPortRange` | `internal/frpconf/render.go` |
| FR-2 | `RenderFrps()` 按 frp 上游 `[[allowPorts]]` 数组段 TOML 字面渲染（每个 range 一个 block，含 `start`+`end` 或 `single`） | `internal/frpconf/render.go` |
| FR-3 | 后端校验：每个端口 ∈ [1,65535]；Start ≤ End；Start/End/Single 互斥（Single != 0 时 Start/End 必须 0，反之亦然）；range 之间不允许重叠 | `internal/frpconf/render.go` + `internal/httpapi/handlers_server.go` |
| FR-4 | `FrpsConfig.AllowPorts []AllowPortRange` JSON schema 新增；GET/PUT `/api/v1/server` 持久化到既有 KV `frps.config`；改完触发 `applyConfigBestEffort("frps")` | `internal/httpapi/handlers_server.go` |
| FR-5 | 前端 `AllowPortsEditor.vue` 独立组件：列表 + "添加范围" / "添加单端口" 按钮 + 每行 start/end 或 single 输入 + 删除按钮 + 实时校验红色错误提示 | `web/src/components/AllowPortsEditor.vue` |
| FR-6 | `Server.vue` 嵌入 AllowPortsEditor，加"端口策略"段；保存按钮调既有 `apiPutServer()` 把 allowPorts 一并传 | `web/src/pages/Server.vue` |
| FR-7 | 顶部说明文案："留空 = 允许所有端口；配置后只允许列出范围被 frpc 申请。改动需要 frps 重启生效（自动）。" | `Server.vue` 端口策略段头部 |
| FR-8 | 单向数据流（继承 insight L13 / T-032 范式）：父侧 ref 写种子 + 子组件 setup 读一次 + defineExpose `getAllowPortsInput()`；**不引入新 v-model 桥** | `Server.vue` ↔ `AllowPortsEditor.vue` |
| FR-9 | OpenAPI `FrpsConfig` schema 新增 `allowPorts` 字段 | `openapi.yaml` |
| FR-10 | dev-map.md 新增 `AllowPortsEditor.vue` 行 + `Server.vue` 行更新 | `docs/dev-map.md` |

### OUT（本任务不做）

- `[[denyPorts]]` 段（frp 上游对称能力，用户未提及；YAGNI，后续如需加 T-XXX）
- `allowSubdomains` / `subDomainHost`（http/https 域名守门，与端口策略正交，归 frps 域名管理任务）
- frpc 侧 reload 通知（frps 重启时既有 frpc 会重连重申请；frpc 端无 UI 影响）
- 端口策略历史 / 审计日志（用户未提，单独任务）

## 3. 输入产物

- 用户高层需求：`frps 服务端 ... 管理 frpc 的端口开放等`
- 批次 PLAN：`docs/batches/frps-monitor-and-mgmt-suite/BATCH_PLAN.md` T-040 行（已固化"端口策略"分拆理由）
- T-039 交付：共享 `RenderFrps()` 路径 + `handlers_server.go::putServer` 路径，无冲突字段
- frp 上游文档（fatedier/frp doc/configure_format.md `[[allowPorts]]` 段）：
  ```toml
  [[allowPorts]]
  start = 2000
  end = 3000

  [[allowPorts]]
  single = 3001

  [[allowPorts]]
  start = 3003
  end = 4000
  ```

## 4. 非功能需求（NFR）

| 编号 | NFR | 验收方式 |
|---|---|---|
| NFR-1 | 前端校验必须实时反馈（onChange，不等 blur），错误用红色文字 + Naive UI `n-form-item` 状态联动 | Vitest mount 触发 input 事件断言错误文案 |
| NFR-2 | 后端校验必须独立守门（不能依赖前端）—— 后端单测必须有"前端绕过场景"反向构造测 | handlers_server PUT 校验测试 |
| NFR-3 | TOML 渲染必须字节稳定（同输入同输出，避免 hash mismatch） | `go-toml/v2` Marshal 本身确定性，验证 round-trip 解 / 解后等值 |
| NFR-4 | UI 列表渲染顺序与用户添加顺序一致（不重排），方便用户对照保存 | Vitest mount 验序 |
| NFR-5 | 校验失败时保存按钮**不**调 API（前端拦截），仅当全部校验通过才走 PUT | Vitest mount 模拟"添加非法 → 点保存"断言 apiPutServer mock 0 次 |
| NFR-6 | 留空 allowPorts 时 TOML 输出**不包含** `[[allowPorts]]` 段（保持原 frps.toml 字节级语义 = 允许所有端口） | go test 反向断言 `strings.Contains(s, "[[allowPorts]]") == false` |
| NFR-7 | 改 allowPorts 后必须触发 frps restart（frps 无 reload；既有 `applyConfigBestEffort("frps")` 路径） | handler 测试断言 restart 被调（既有路径，回归覆盖即可） |

## 5. 约束 & 风险

### 约束

- **frp 上游 1.x 系列字面契约**：`[[allowPorts]]` 数组段，字段 `start`/`end`/`single`，integer。不可改名（frp parser 强字面）。
- **互斥规则**：frp 上游允许同一 block 同时含 `start+end` 或 `single` 之一，**不允许同时含两者**。前后端必须双层守门。
- **重叠语义**：frp parser 不主动拒绝重叠 range（last-wins 加入 allow set）；项目侧前后端必须主动拒绝，理由：用户重叠语义模糊，且会让 UI 列表展示与实际生效不一致（治理可读性）。
- **TOML 数组段嵌套**：`[[allowPorts]]` 必须在 `[webServer]` / `[auth]` 等表段之后（TOML 规范：表段不能出现在数组段后），需在 `frpsRoot` struct 中保证字段顺序（go-toml 按 struct 字段定义顺序输出）。
- **单向数据流约束**（insight L13）：禁止用 `defineModel` 或 `v-model:allowPorts` 桥接，必须用 props 单向 + defineExpose `getAllowPortsInput()` 拉取。

### 风险

| ID | 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|---|
| R-1 | 重叠校验逻辑错（如 [1,10] vs [10,20] 算不算重叠） | 中 | 中 | 02 §3 决策矩阵明确"端口包含语义 = 闭区间 [start, end]"；end 等 start 算重叠（10 同时属于两段）。单测覆盖边界用例。 |
| R-2 | 用户填了 Single=80 同时又填 Start=80 End=80 | 中 | 低 | 校验：Single != 0 → Start/End 必须 == 0，反之亦然。前端通过"添加范围" / "添加单端口"两按钮分流让用户不可能同时填，后端再做 belt-and-suspenders 校验。 |
| R-3 | TOML 渲染中 `[[allowPorts]]` 出现在 `[log]` / `[webServer]` 之前 → frp parser 接受但 TOML 规范不推荐 | 低 | 低 | go-toml/v2 默认按 struct field 顺序输出；把 AllowPorts 放 frpsRoot struct 最后，自然在所有表段之后。 |
| R-4 | 前端 Vitest mount AllowPortsEditor 漏 stub useMessage 导致 render 失败 | 中 | 低 | insight L9/L14 已规定 importOriginal + 6 方法 stub；spec 文件直接复用 ProxyForm.spec.ts 模式。 |
| R-5 | 用户 PUT 时提供超大 allowPorts 数组（如 1000 条）导致 KV 序列化膨胀 / TOML 渲染慢 | 低 | 低 | 加 200 条上限校验（与 proxies 上限同款；frps 单实例典型场景 ≤ 50 段）。 |
| R-6 | 与 T-041 / T-042 后续任务共享 Server.vue → 改动域冲突 | 低 | 中 | T-041 加新页面（不动 Server.vue），T-042 改 Proxies.vue（不动 Server.vue），无冲突。 |

## 6. 验收条件（AC）

| ID | 验收 | 方式 |
|---|---|---|
| AC-1 | `RenderFrps` 输入 AllowPorts=[{Start:6000,End:7000}, {Single:9000}] → 输出 TOML 含 `[[allowPorts]]\n  start = 6000\n  end = 7000\n` 与 `[[allowPorts]]\n  single = 9000\n` 各一段 | go test |
| AC-2 | `RenderFrps` 输入 AllowPorts=nil 或 [] → 输出 TOML **不含** `[[allowPorts]]` 字面（NFR-6） | go test |
| AC-3 | `RenderFrps` 输入 Start=80, End=70 → 返回错误 `start > end` | go test |
| AC-4 | `RenderFrps` 输入 Start=80, End=80, Single=80（三者同设）→ 返回错误 `single and start/end mutually exclusive` | go test |
| AC-5 | `RenderFrps` 输入 [{Start:1000,End:2000}, {Start:1500,End:2500}] → 返回错误 `overlapping ranges` | go test |
| AC-6 | `RenderFrps` 输入边界 Start=1, End=65535 → 成功；Start=0 或 End=65536 → 失败 | go test |
| AC-7 | `PUT /api/v1/server` 入参 allowPorts 含非法值（如 End=65536）→ 422 + 字段名定位 | handler test |
| AC-8 | `PUT /api/v1/server` 成功后 `applyConfigBestEffort("frps")` 被调用 | handler test（既有路径回归） |
| AC-9 | 前端 AllowPortsEditor mount 后点 "添加范围" → 列表 +1 行；填非法数值（如 End=65536）→ 实时显红 + 父级保存按钮被禁用或拦截 | Vitest mount |
| AC-10 | 前端 AllowPortsEditor mount 后点 "添加单端口" → 列表 +1 行 single 输入 | Vitest mount |
| AC-11 | 前端 defineExpose `getAllowPortsInput()` 返回当前列表（顺序保留），父级 Server.vue 调用此方法填入 PUT 请求 | Vitest mount |
| AC-12 | `openapi.yaml` 中 `FrpsConfig` schema 含 `allowPorts: array<AllowPortRange>` | grep + 手动核 |
| AC-13 | `pwsh scripts/verify_all.ps1` baseline FAIL=1（C.1 e2e）不增（≤1） | verify_all 实跑 |
| AC-14 | 06_TEST_REPORT.md 含 `## Adversarial tests` 段，至少 3 个反向构造用例（端口越界 / start>end / 重叠） | grep 06 文件 |

## 7. 与历史任务的关系

| 任务 | 关系 | 必读章节 |
|---|---|---|
| T-039 frpsadmin-server-runtime-api | 同批次前驱；共享 `RenderFrps` + `handlers_server.go` 改动路径 | 02 §3.4 / 04 §"Design drift"（dashboard 凭据 fallback 范式） |
| T-032 proxy-form-vmodel-oom-fix | 同款"父子单向数据流"范式 | 02 §7 决策矩阵 / 04 §3（defineExpose `toXxx()` pattern） |
| T-018 upload-bin-multiport-ip-probe | 同款"端口列表"概念，但本任务是 server-side 策略，非 client-side 探测 | 仅参考 `usePortPresets` 端口常量风格 |
| T-037 proxy-rules-simplify-and-port-fix | UI 简化范式 + 删除面 grep 守门；本任务**不**删除任何旧能力，故不引入新 verify_all step | — |

## 8. Open questions（PM 决策）

| ID | 问题 | PM 决策（理由） |
|---|---|---|
| OQ-1 | 是否限上限数量（如最多 N 个 allowPorts entry）？ | **限 100 条**（与 proxies 200 上限低一个量级；端口策略典型场景 ≤ 50；防 KV 膨胀） |
| OQ-2 | 重叠的语义边界（[1,10] vs [10,20] 算重叠吗）？ | **算重叠**（10 同时属两段；闭区间语义；前端 + 后端一致） |
| OQ-3 | 单端口和范围在 UI 是分两个 list 还是一个 list 两种 row 类型？ | **一个 list，row 类型字段区分**（用户视角"端口策略"是统一概念，分两列割裂体验；row 内通过 `single` vs `start+end` 是否填来区分） |
| OQ-4 | 保存时校验失败应该提示哪条？ | **第一条非法 entry 的字段定位 + 红色提示文字**（Naive UI `n-form-item validation-status='error' feedback='...'`） |
| OQ-5 | 是否提供"清空所有"按钮？ | **不提供**（用户可通过删除按钮逐条删；YAGNI；防误操作） |
| OQ-6 | TOML 渲染时是否保留用户的顺序？ | **保留**（用户视角的可读性）；序列化按用户添加顺序写 |

## 9. Hand-off to Architect

下一阶段：**solution-architect** 写 `02_SOLUTION_DESIGN.md`，重点：

- §3 后端：`FrpsAllowPortRange` struct + `RenderFrps` 渲染逻辑 + 校验函数
- §4 后端 handler：`FrpsConfig.AllowPorts` schema + 校验集成
- §5 前端：`AllowPortsEditor.vue` 组件设计 + Server.vue 集成路径 + 单向数据流契约
- §6 单测 + Vitest 覆盖矩阵
- §7 partition assignment（本任务 single developer mode，不分区）
- §8 OpenAPI schema diff

**Verdict**: READY FOR ARCHITECT.
