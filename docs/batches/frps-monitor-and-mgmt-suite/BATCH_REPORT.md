# BATCH_REPORT — frps-monitor-and-mgmt-suite

> 2026-05-28 收尾。clean completion，无 stop signal 触发。

## 批次结果

**4/4 任务全部 DELIVERED**

| Task | Slug | Verdict | Commit | 归档路径 |
|---|---|---|---|---|
| T-039 | frpsadmin-server-runtime-api | DELIVERED | `ecc49b9` | `docs/features/_archived/frpsadmin-server-runtime-api/` |
| T-040 | frps-allow-ports-policy | DELIVERED | `68da3a1` | `docs/features/_archived/frps-allow-ports-policy/` |
| T-041 | server-monitor-page-ui | DELIVERED | `37dca96` | `docs/features/_archived/server-monitor-page-ui/` |
| T-042 | proxy-runtime-status-merge | DELIVERED | `52a8cda` | `docs/features/_archived/proxy-runtime-status-merge/` |

## 聚合统计

- **任务**: 4 done / 0 failed / 0 blocked / 0 skipped
- **总 files changed**: ~61（去重后）
- **总 LOC**: 生产 ~1500 / 测试 ~1800 / 文档 ~9000
- **测试新增**: 后端 ~37 unit/handler test + 前端 ~98 vitest 用例
- **挂起期**: 2026-05-27 ~ 2026-05-28（wall time ~2 天，含 PM 派发上下文 deferred-hook 接力）
- **基线 verify_all**: PASS=31 / FAIL=1（C.1 e2e Playwright 长期已知问题，insight L21/L34 记录）
- **最终 verify_all**: PASS=31 / FAIL=1（持平 baseline，**零回归**）
- **stop_reason**: 无（自然完成）
- **rollback 触发**: 1 次软 rollback（T-041 06_TEST_REPORT 标题 `## 3. Adversarial tests` 触发 verify_all E.6 FAIL → batch orchestrator 在 commit 前一行 Edit 修复 + 收割成新 insight，未触发 strong-signal stop）

## 用户原始需求 vs 交付

| 用户原话 | 交付的能力 | 涉及任务 |
|---|---|---|
| 实现 frps 服务端 | frpsadmin 包 + 4 条 REST API + dashboard 凭据 autogen | T-039 |
| 查看所有 frpc 在线状态 | ServerMonitor 页 ServerInfo 卡片（版本/运行时长/总客户端连接数）| T-041 |
| 连接状态 | ServerMonitor proxy 表格（按 type 分组 / status 三色 dot / 当前连接数 / 累计流量 / lastStartTime / lastCloseTime）+ Proxies 页运行态列叠加（配置/运行单视图） | T-041 + T-042 |
| 管理 frpc 的端口开放 | Server 设置页 AllowPortsEditor（range + single + 实时校验 + 上限 100 + 后端 ValidateFrpsAllowPorts 二层守门） | T-040 |
| 用户体验好 | 5s 自动轮询 + visibility 暂停 + 三态完备 + 友好错误文案 + 配置降级（监控不可用时 CRUD 仍正常） | 全部 |
| 软件工程标准 | unit + handler + mount + adversarial × 4 任务 / openapi.yaml 同步 / dev-map 同步 / 每 task git commit conventional 格式 | 全部 |
| 长期易使用易维护 | 单向数据流（T-032 范式）/ utils 抽取 DRY（T-042）/ composable 复用（T-041 → T-042 直接 import）/ SFC 行数自检 / sentinel error 分类 | 全部 |
| AI 决策 + commit + push | 整批由 batch orchestrator 决策 4-task 分拆 + 每 task 单 commit + 末尾批量 push | 本批次 |

## 关键 insight 收割（写入 `.harness/insight-index.md`）

本批次共收割 13 条 insight：T-039 +2 / T-040 +3 / T-041 +5（含修复案 1 条）/ T-042 +3。其中最值得未来 PM 警惕的：

1. **`## Adversarial tests` 标题禁数字前缀** —— 与 `## Insight` 同款规则（L41）。T-041 实测因 `## 3. Adversarial tests` 触发 verify_all E.6 FAIL，batch orchestrator 一行 Edit 修回，收割成新 insight。未来 PM 写 06_TEST_REPORT.md 模板应硬约束。
2. **集合类配置（allowPorts）走 last-wins 整体替换 vs 单字段（user/pass）走 per-field fallback** —— 两种合并范式不可混（T-040 vs T-039 形成对比）。
3. **frps admin API 响应 `{"proxies":[...]}` envelope 必须在 client 层 unwrap** 让 handler 层拿到扁平数组（T-039）。
4. **三态 UI 必须写成布尔代数式互斥矩阵** 避免"loading + error 同时显示"尴尬态（T-041）。
5. **后端"集合"白名单的"边界重叠"语义必须前后端双层校验 + 测试硬编码**，仅靠文档约定会让 UI/实际生效不一致（T-040）。
6. **n-tabs activeKey 用 `Object.keys()` 顺序会因 polling 响应顺序漂移导致 tab 闪烁** —— 前端 hardcode 显示顺序数组 + Set 过滤（T-041）。

完整 insight 见 `.harness/insight-index.md`（已 rotate 13 条到 insight-history.md 保持 ≤30 行预算）。

## 用户需要关注的事项

### ✅ 已完成（无需用户操作）
- 4 个功能已上线 main 分支，4 个独立 commit
- verify_all PASS=31 / FAIL=1（与 batch 启动时 baseline 完全一致）
- 所有 stage 文档已归档到 `docs/features/_archived/`
- 13 条 insight 已 harvest 到 `.harness/insight-index.md`
- batch 工件（PLAN / LOG / 本 REPORT）保留在 `docs/batches/frps-monitor-and-mgmt-suite/`
- 最终 git push origin main 由 batch orchestrator 在本 report commit 后执行

### ⚠️ 已知 baseline 问题（不属于本批次）
- **C.1 E2E smoke (Playwright)**：长期 FAIL，根因是本机 7800 端口被既有 frp-easy 进程占用 → Playwright `reuseExistingServer` 复用已初始化后端触发 fixture fail-fast（insight L21/L34/T-033/T-036 多次记录）。本批次完全未碰 e2e / Playwright / Go 后端 e2e 路径，零相关。
- **如要在本机让 C.1 PASS**：kill 占用 7800 端口的既有 frp-easy 进程 + 删除 `data.db` 让 Playwright 启动一个 fresh 后端实例（详见 `docs/features/_archived/e2e-setup-spec-flake-fix/`）。CI 环境永远 fresh server，不复现此问题。

### 💡 用户验收建议（可选）
- 跑 `pwsh scripts/start.ps1`（或 `go run ./cmd/frp-easy`）启动应用，浏览器访问：
  - **`/server`** → 看新的"端口策略"段（Server.vue 底部，T-040 交付）
  - **`/server/monitor`** → 新的服务端监控页（T-041 交付）
  - **`/proxies`** → 既有列表加了"运行状态 + 流量"两列（T-042 交付）
- 如需测试 allowPorts 效果：填一条 [10000-10100] → 保存 → 在 Proxies 页尝试加远程端口 9999 → 应被 frps 拒绝（frpc.log 会显示 port not allowed）
- 如需测试 ServerMonitor：先在 Server 页保 frps 配置（dashboard 凭据若未填会自动生成）→ Apply → 跑起 frps → 进 `/server/monitor` 应看到 ServerInfo + proxy 表格

### 📋 后续可选的小优化（YAGNI，未做）
- frps 历史流量持久化（metrics 表）—— 当前 ServerMonitor 只展示瞬时值，不存历史；如未来需要"过去 24h 流量曲线"再开任务
- 在 ServerMonitor 加"强制踢出 client"按钮（frps admin API 不提供此能力，需 frps 重启实现）—— 当前不做避免破坏稳定连接
- allowPorts UI 加"导入 / 导出"按钮 —— 简单需求暂不做
