# 03 — Gate Review · T-039 frpsadmin-server-runtime-api

> Stage 3 / 7。守门 01 + 02 是否够 developer 启动 stage 4，不替 SA 改设计。

## 1. 验收摘要

- 01_REQUIREMENT_ANALYSIS.md：FR/NFR/AC/范围/关联任务齐备，决策点 PM-DECIDED 显式记录 → ✅
- 02_SOLUTION_DESIGN.md：包结构 / 接口签名 / 错误模型 / KV 字面 / 测试矩阵 / Adversarial 计划齐备，design drift 显式标记 → ✅

## 2. 决策矩阵

| 维度 | 评估 | 是否阻塞 |
|---|---|---|
| 范围明确 | FR-1 ~ FR-5 共 25 条，分层清晰 | 否 |
| 接口契约 | 4 个 sentinel error + 4 个公开方法签名 + 4 条 HTTP 路由 | 否 |
| 错误码覆盖 | 200 / 401 → 502 / 404 / 503 / 502 4 条映射规则明示 | 否 |
| 测试可证伪 | 4 个 ADV 反向证伪用例覆盖每个关键决策（401 分支 / envelope / autogen fallback / KV key 字面） | 否 |
| 与既有架构兼容 | 与 frpcadmin 对称镜像、复用 KV / SessionAuth / writeError | 否 |
| 不引入新依赖 | NFR-1 确认 | 否 |
| Design drift 已识别 | 02 §3.4 显式标记 RA FR-3.3 "DashboardEnabled=false → 自动翻 true" 已调整为"尊重用户禁用意图" | 否（已合理化） |

## 3. 关注点（非阻塞条件）

### C-1（WARN）：handler 503 文案要测一下长度上限

- 02 §3.2 友好文案如 `"frps dashboard 未启用。请到 Server 设置页打开 'Dashboard' 开关并保存（frp_easy 会自动生成凭据并应用配置）。"` 约 70 中文字符；R-4 说 ≤ 100。
- developer 在 stage 4 实施时按字符数自核；如超就改短，无需回 SA 改设计。

### C-2（WARN）：`serverRuntimeProxies` 全 fatal 路径的实现要清晰

- 02 §3.2 写的"全 fatal 才整体 5xx"实现里再次循环调一次 `c.Proxies(...)` 取 sentinel，**性能上等于多调一次上游**。
- 推荐 developer 在 stage 4 改成"loop 内记下第一个 fatal err"，避免重调。这是实现细节优化，不改契约。

### C-3（WARN）：`config_helper.go` 的 fallback 应同步出现在 PUT /server handler

- 02 §3.4 只改 `renderAndApplyFrps`。但 `putServer` 在 `applyConfigBestEffort` 内异步调 `renderAndApplyFrps`——已天然覆盖。
- developer 验证一次链路（在 stage 4 调试时）即可。

### C-4（WARN）：openapi.yaml 新 schema 命名与既有风格保持一致

- 既有 schema 是 `PascalCase`（如 `FrpsConfig` / `SystemReady`）；新增 `ServerRuntimeInfo` / `ServerRuntimeProxiesResponse` 符合规范。
- developer 在 stage 4 注意大小写一致即可。

### C-5（INFO）：design drift 已合理化，不需 RA 改文档

- 02 §3.4 关于 FR-3.3 的调整（"DashboardEnabled=false 不自动翻 true"）已显式标记 + 论证。
- 该 drift 与 RA D-1（"用户体验好"）兼容——尊重用户禁用意图本就是 D-1 隐含原则。
- 不需 stage 1 回滚。

## 4. PM 派发 stage 4 时的提示

- developer 按 02 §7 实现顺序 1~9 推进，每个 step 跑对应 `go test` 局部验证。
- 顺手消化 C-1 / C-2 / C-3 / C-4 4 条 WARN（与 T-035 / T-038 的同款"developer 顺手消化 GR conditions"风格一致，insight L29）。
- Stage 4 结束跑 `pwsh scripts/verify_all.ps1`，目标 PASS ≥ 32, FAIL = 1（baseline 不退化）。

## 5. Verdict

**APPROVED FOR DEVELOPMENT**

Conditions（WARN，不阻塞）：C-1（文案长度）/ C-2（全 fatal 路径性能）/ C-3（fallback 链路验证）/ C-4（schema 命名一致）。Developer 在 stage 4 顺手消化。
