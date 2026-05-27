# 03 — Gate Review · T-040 frps-allow-ports-policy

> Stage 3 / 7。闸门评审。在动手前对 01 + 02 做最后一次冷静审视。

## 1. 输入

- `01_REQUIREMENT_ANALYSIS.md` v1（14 AC + 7 NFR + 6 OQ）
- `02_SOLUTION_DESIGN.md` v1（数据结构 + 校验逻辑 + UI 契约 + 测试矩阵）
- 项目 insight L9/L13/L14/L23/L29/L31/L41/L43

## 2. 审查矩阵（按 02 章节）

| § | 关键点 | 结论 |
|---|---|---|
| §3.1 | `FrpsAllowPortRange` 三字段互斥设计 | ✅ 与 frp 上游字面一致；Single != 0 + Start/End == 0 是清晰契约 |
| §3.2 | `frpsRoot.AllowPorts` 放 struct 末尾 | ✅ go-toml 按 struct 字段顺序输出，表段在前数组段在后，符合 TOML 规范 |
| §3.5 | `ValidateFrpsAllowPorts` overlap O(n²) on n≤100 | ✅ 100×100=10000 比对，亚毫秒；语义清晰胜过 sweep line（CO-1 简化原则） |
| §3.5 | 闭区间重叠 `lo <= b.hi && b.lo <= a.hi` | ✅ OQ-2 决策一致 |
| §3.6 | 决策矩阵 11 行边界 | ✅ 覆盖 AC-1~6 |
| §4.2 | handler 校验复用 `frpconf.ValidateFrpsAllowPorts` | ✅ 避免双实现漂移（与 ValidatePort 同款做法） |
| §4.4 | last-wins 整体替换 vs T-039 per-field | ✅ 决策合理；allowPorts 是用户编辑的整体快照，per-field 不适用 |
| §5.1 | defineExpose 双方法 `getAllowPortsInput` + `hasValidationError` | ✅ T-032 范式延伸；hasValidationError 让父级保存前能拒收 |
| §5.3 | 前端 `validateRow` 镜像后端 §3.5 | ✅ 错误文案中文友好；逻辑分支一致 |
| §6.3 | `handleSave` 中校验失败 message.error 直返 | ✅ NFR-5 落实 |
| §7.1 | 11 个 backend test | ✅ AC-1~6 + OQ-1/2/6 全覆盖 |
| §7.3 | 8 个 frontend test，Naive UI mock 严格 6 方法 | ✅ insight L9/L14 落实 |
| §8 | OpenAPI maxItems=100 + AllowPortRange schema | ✅ 与 §3.5 / §5.1 一致 |
| §10 | Single developer mode | ✅ 无 dev-* partition 文件，默认 generic developer |

## 3. 风险二次评估

| R | 原评估 | GR 复评 | 行动 |
|---|---|---|---|
| R-1 | 中/中 | 中/低 | 02 §3.6 边界矩阵充分；测试已覆盖 |
| R-2 | 中/低 | 低/低 | 前端两按钮分流物理隔离 + 后端 belt 兜底 |
| R-3 | 低/低 | 低/低 | go-toml 顺序确定 |
| R-4 | 中/低 | 低/低 | 7.3 spec 直接复用 ProxyForm.spec.ts mock 块 |
| R-5 | 低/低 | 低/低 | 100 条上限多层落实 |
| R-6 | 低/中 | 极低/极低 | 改动域确认无重叠 |

## 4. 命中 / 未命中 insight 检查

| Insight | 是否命中 | 体现位置 |
|---|---|---|
| L9 NMessageProvider 在 App.vue | ✅ | 02 §7.3 mock useMessage |
| L13 v-model 桥 + composable 新对象 OOM | ✅ | 02 §5.1 单向数据流 + defineExpose |
| L14 importOriginal + 6 方法 stub | ✅ | 02 §7.3 |
| L23 PM 派发上下文工具裁剪 | ✅ | PM_LOG 已注明角色 collapse |
| L26 verify_all 新 step 必须 ADV 4 次 | N/A | 本任务不增 step |
| L29 UI 列名 / DB 字段名同名时字面引用 | ✅ | 02 §3.4 单一渲染源 |
| L31 SFC 200 行按纯逻辑判 | ✅ | 02 §5.5 估算 110 物理 / 50 纯逻辑 |
| L41 per-field fallback | ✅ | 02 §4.4 明确说明数组场景不适用 + 理由 |
| L43/L48/L49 07 ## Insight 裸标题 + bullet | 待 stage 7 落实 | — |

## 5. Conditions（必须在 stage 4 消化）

| ID | Severity | Finding | 期望行动 |
|---|---|---|---|
| C-1 | INFO | 02 §3.5 错误文案均为中文，但 frp 上游 / TOML 渲染错误是英文。建议在错误文本中**仅**用中文（用户面向），并在错误后保留位置索引方便定位 | dev 落实时确认错误文本中文 + 含 `allowPorts[i]` 字面 |
| C-2 | INFO | 02 §5.3 前端 overlap 检测会双向比对（i 行 与 j 行 + j 行 与 i 行），同一对错误显示两次（每行各一条）。可考虑改为"只对索引小的行显示"消除重复 | dev 可微调（不强制）；当前体验影响低 |
| C-3 | WARN | 02 §4.2 校验只在 PUT 时跑；GET 返回 KV 中持久化的 allowPorts，若历史数据曾绕过校验写入（虽然路径上做不到），GET 不重校验也不报警 | dev 加注释说明 "GET 信任 KV 持久化数据"；不强制重校验（与既有 dashboard 字段同款语义） |
| C-4 | INFO | 02 §3.4 渲染时只有 `len(in.AllowPorts) > 0` 才 ValidateFrpsAllowPorts；空数组 / nil 跳过校验 | 符合 NFR-6；INFO 留痕，不阻塞 |

无 P0/P1 blocker。

## 6. 决策

**APPROVED FOR DEVELOPMENT**

- C-1~C-4 均为 INFO/WARN，可在 dev 自然顺手时一并消化（insight L25 范式）
- 设计已经满足 14 个 AC + 7 NFR 的全部映射
- 风险评估收敛到"极低/低"级别
- 测试矩阵覆盖完备

## 7. Hand-off to Developer

Developer 重点：

1. 后端先动（frpconf → handlers_server），跑 `go test ./internal/frpconf/... ./internal/httpapi/...` 全绿
2. 再前端（types.ts → AllowPortsEditor → Server.vue 集成 → spec），跑 `npm run test -- AllowPortsEditor` 全绿
3. 同步 openapi.yaml + dev-map.md
4. 跑 `pwsh scripts/verify_all.ps1` 确认 FAIL ≤ 1（baseline）
5. 提交单 commit `feat(T-040): frps-allow-ports-policy — 简述`
6. 完成时在 04_DEVELOPMENT.md 显式回答 C-1~C-4 的消化情况
