# BATCH_PLAN — frps-monitor-and-mgmt-suite

> 2026-05-27 创建。用户高层目标：**实现 frps 服务端能力 —— 查看所有 frpc 在线状态 / 连接状态、并可管理 frpc 的端口开放等**。
>
> 用户授权 AI 全权决策（设计 + 实现 + commit + push），决策原则：用户体验好 / 符合软件工程标准 / 长期易使用易维护。

## 决策摘要（AI 视角）

调研发现项目现有能力：
- `internal/frpcadmin/` 已是 frpc admin API HTTP 客户端（可作 frps admin API 客户端的设计模板）
- `internal/frpconf/RenderFrps()` 已渲染 frps.toml（含 dashboard 字段），但 `allowPorts` 未支持
- `internal/httpapi/` 仅有 `/api/v1/server` 配置 CRUD，无运行时查询
- 前端无"server 端看 client 集群"入口；Proxies.vue 只有配置态，无运行态

**核心缺口**：frps 上游有完整 admin HTTP API（`/api/serverinfo`、`/api/proxies?type=...`、`/api/traffic/:name` 等），项目尚未消费。

**分拆理由**（每个任务都符合"独立可交付 + 易回滚 + 单测可守"）：
1. 后端基础（frpsadmin 包 + REST API + dashboard 自动渲染）必须先做，UI 才有数据源
2. 端口策略（allowPorts）逻辑上属于"frps 端管 frpc"，是用户需求的一半，独立交付
3. 监控页是用户体感最强的功能，独立 UI feature，避免与既有 Proxies.vue 改动耦合
4. 把运行态合并到既有 Proxies.vue 是高 ROI 体验升级（"配置即所见"），单独做避免污染 T-041 监控页设计

## Baseline 状态（2026-05-27 batch 启动时）

- `pwsh scripts/verify_all.ps1`：**31 PASS / 1 FAIL / 0 WARN / 0 SKIP**
  - C.1 E2E smoke (playwright) **FAIL — 长期已知环境耦合问题**（insight L21 / L34；T-033 缓解；T-036 / T-038 已豁免运行）

**回归判定豁免规则**：本批次 baseline 已 FAIL=1，标准 batch skill "refuse on FAIL" 规则在此豁免，理由：
- C.1 FAIL 是已记录的长期环境问题（Playwright `reuseExistingServer` + 既有 frp-easy 进程占 7800 端口）
- 本批次所有任务的改动域（frpsadmin 包 / httpapi handlers / 新 Vue 页面 / Proxies.vue 扩展）与 C.1 路径**零重合**
- 每任务跑完 verify_all 后，**FAIL 数 > 1 即停批**（任何新增 FAIL 都是回归）
- 与 `post-t032-followup` batch 豁免姿势同源

## 任务表

| ID | Slug | Goal | Mode | Depends on | Status |
|---|---|---|---|---|---|
| T-039 | frpsadmin-server-runtime-api | 新增 `internal/frpsadmin/` 包（HTTP basic auth 客户端封装 frps admin API：ServerInfo / Proxies(byType) / Traffic / ProxyDetail）；新增 `GET /api/v1/server/runtime/info` `GET /api/v1/server/runtime/proxies` `GET /api/v1/server/runtime/traffic/{name}` 三条 REST 路由；改 `RenderFrps()` 让 dashboard 凭据若空则自动生成稳定值（避免用户手动配置缺失就用不上监控）；同步 openapi.yaml；后端 unit test + handler test 覆盖。 | full | — | pending |
| T-040 | frps-allow-ports-policy | 改 `RenderFrps()` 支持 `allowPorts` 字段（[]PortRange，渲染为 `[[allowPorts]] start=X end=Y` 或 `single=N`）；`PUT /api/v1/server` 入参 schema 新增 `allowPorts`；Server.vue 加端口策略编辑器（范围列表 + 单端口列表 + 添加/删除按钮）；后端 unit test 覆盖渲染；前端 Vitest 守门表单交互。 | full | T-039（共享 RenderFrps 改动路径，避免双任务并行改同一文件） | pending |
| T-041 | server-monitor-page-ui | 新增 `web/src/pages/ServerMonitor.vue`（路由 `/server/monitor`，导航菜单加入口）；展示 ServerInfo（版本 / 运行时长 / 总连接数 / 总流量）+ Proxies 表格（按 type 分组：name / 在线状态 / 当前连接数 / 累计流量 in/out / lastStartTime）；5s 轮询自动刷新（可暂停）；loading / empty / error 三态完备；调 T-039 新 API；Vitest mount + setProps 覆盖三态切换。 | full | T-039 | pending |
| T-042 | proxy-runtime-status-merge | Proxies.vue 在既有"配置态"表格基础上叠加 runtime 列（运行中绿点 / 未运行灰点 / error 红点 + tooltip 流量 in/out + 最近错误信息）；调 T-039 `/server/runtime/proxies`；5s 轮询；继承 T-032 单向数据流范式（不引入新 v-model 桥）；继承 T-037 删除面 grep 守门；Vitest 守门 runtime 列 render 边界。 | full | T-039 + T-041（共享 useServerRuntime composable 提取） | pending |

**Topo order**：T-039 → T-040 → T-041 → T-042（sequential；T-040 与 T-041 在 T-039 完成后可逻辑并行，但 batch v0.19.0 sequential，按"先后端收尾，再 UI"自然顺序排）

## 决策原则映射（用户传达 → 任务实施）

| 用户原则 | T-039 | T-040 | T-041 | T-042 |
|---|---|---|---|---|
| 用户体验好 | dashboard 自动生成凭据，零配置即用 | 端口策略可视化编辑，不需要手写 toml | 5s 实时刷新 + 三态完备 + 可暂停 | 配置态/运行态单视图聚合，省去切页 |
| 软件工程标准 | unit test + handler test + openapi.yaml 同步 | 后端单测渲染 + 前端 Vitest 守门 | Vitest mount × 3 态 + adversarial 反向构造 | 继承既有范式 + 反向构造 |
| 长期易使用易维护 | 与 frpcadmin 包同款 API design 镜像 | last-wins merge + 边界明确 | composable 抽离 polling 逻辑，可复用 | useServerRuntime composable + 单向数据流 |

## 本批次的 strong-signal 停止条件

- 任一 task pm-orchestrator 返回 `FAILED` verdict
- 任一 task 跑完后 verify_all **FAIL 数 > 1**（超过 baseline）
- `.harness/intervention.md` 出现 `STOP` 关键字
- 安全 hook 拦截 destructive Bash 调用

## 提交 / 推送策略（用户授权）

- 每个 task 完成后由该 task 的 pm-orchestrator 在 Stage 7 Delivery 提交（commit message 遵循既有 conventional commit 风格：`feat(T-NN): slug — 简述`）
- 批次结束（或停批）后，batch orchestrator 写 BATCH_REPORT.md 单独 commit，并 `git push origin main`
