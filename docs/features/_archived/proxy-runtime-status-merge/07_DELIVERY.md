# 07_DELIVERY — T-042 proxy-runtime-status-merge

> Stage 7 / PM Orchestrator · 2026-05-28
>
> 批次：`frps-monitor-and-mgmt-suite` 第 4/4 个（最后任务）

## Delivery Summary

- **Task**: T-042 proxy-runtime-status-merge — Proxies.vue 叠加 runtime 列（运行状态 + 流量）单视图聚合配置态/运行态；抽 utils/format.ts + utils/proxyStatus.ts 与 ServerMonitor 共享
- **Mode**: full (7-stage)
- **Stages traversed**:
  - Stage 1 RA · 2026-05-28T00:35Z · `01_REQUIREMENT_ANALYSIS.md`
  - Stage 2 SA · 2026-05-28T00:40Z · `02_SOLUTION_DESIGN.md`
  - Stage 3 GR · 2026-05-28T00:45Z · `03_GATE_REVIEW.md` (APPROVED FOR DEVELOPMENT)
  - Stage 4 Dev · 2026-05-28T01:00Z · `04_DEVELOPMENT.md` (READY FOR REVIEW)
  - Stage 5 CR · 2026-05-28T01:10Z · `05_CODE_REVIEW.md` (APPROVED)
  - Stage 6 QA · 2026-05-28T01:20Z · `06_TEST_REPORT.md` (PASS)
  - Stage 7 Delivery · 2026-05-28T01:25Z · 本文件
- **Rollbacks**: 0
- **Final verify_all result**: PASS=31 / FAIL=1 (预期，与 baseline 持平；C.1 e2e 已知豁免)。实测由 batch orchestrator 在批末执行（insight L23 PM 派发上下文工具裁剪）。
- **Baseline changes**:
  - 新增 vitest 用例 +52（utils + Proxies + qa adversarial 4 文件合计）
  - 既有 spec 零修改
  - 后端 Go 测试零触碰
- **Files changed**: 9
  - NEW: `web/src/utils/format.ts`
  - NEW: `web/src/utils/proxyStatus.ts`
  - NEW: `web/src/utils/__tests__/format.spec.ts`
  - NEW: `web/src/utils/__tests__/proxyStatus.spec.ts`
  - NEW: `web/src/pages/__tests__/Proxies.spec.ts`
  - NEW: `web/src/pages/__tests__/qa_t042_adversarial.spec.ts`
  - MODIFIED: `web/src/pages/Proxies.vue` (+194 / -24)
  - MODIFIED: `web/src/pages/ServerMonitor.vue` (+5 / -22)
  - MODIFIED: `docs/dev-map.md` (+3 / -1)
- **Outstanding risks**:
  - Proxies.vue 纯逻辑行数 ~198 临界（< 200 红线）；下次再加 runtime 相关 setup 应抽 `useProxyRuntimeMerge.ts` composable
  - 双 polling 实例（ServerMonitor + Proxies 各一份）对 frps 是 2× 请求量；当前可接受，若未来用户数据爆发需重写 useServerRuntime 为单例 + ref count
- **Next steps for user**:
  - 用户打开 Proxies 配置页即刻能看到每条 proxy 的运行态（绿/灰/红点 + 流量），无需切到 ServerMonitor 页
  - frps 未运行时 Proxies 页配置 CRUD 仍正常工作（监控列灰点降级）
  - 批末由 batch orchestrator 跑 `pwsh scripts/verify_all.ps1` + 单 commit + 整批 push

## Insight

- 2026-05-28 · "配置态 ⊗ 运行态"叠加 UI 范式：当 UI 同时展示两份不同来源数据集合时，**必须用 Map 摊平 runtime 数据 + 以配置态作主表左外连接渲染**（runtime 缺则降级"离线"），切忌反向（以 runtime 为主表）。本任务 runtimeMap computed + columns 内 `runtimeMap.value.get(row.name)` 是范本；反例：以 runtime 为主表会让"用户刚创建但 frps 尚未感知"的 proxy 在配置页消失（用户感知 = bug）。AC-4 / AC-5 反向构造矩阵专门守门这两侧。任何"配置/状态混合表格"复刻此模式 · evidence: T-042 Proxies.vue runtimeMap + Proxies.spec.ts AC-4 / AC-5 用例
- 2026-05-28 · UI utils 抽取的"字节级搬运 + 一处新增防御"模式：从既有 SFC 抽 utils 时严格遵守"先按 inline 实现 1:1 搬运 → 在 utils 提交后再以单独提示在文件顶端注释 + 单测显式覆盖方式新增防御行为"（如本任务在 utils/format.ts 顶端注释 + AC-7 单测覆盖 `formatBytes(-1) → '—'` 新增负数防御），让 reviewer 可一眼区分"搬运"与"行为变更"，避免回归。反例：抽取时静默加防御 → 既有 spec 不覆盖 → 未来若有依赖该 bug 行为的下游会断 · evidence: T-042 utils/format.ts 头注释 + format.spec.ts "负数（防御）→ '—'" 用例
- 2026-05-28 · "降级"列的 UI 表达必须满足"独立判定 + 不挂主功能"双约束：本任务 runtimeUnavailable computed 只看 `runtime.proxies.value === null && runtime.error.value !== null` 不耦合配置态 store；frps 不可用时整列灰点，但 Proxies CRUD 走的 store.fetchProxies / createProxy / updateProxy / deleteProxy 通路完全不接触 runtime ref，保证配置功能零回归。这是与 T-038 boot-autostart "尾巴 retry 不阻塞 ready gate" 同款异步降级范式（叠加层失败不影响主面） · evidence: T-042 Proxies.vue runtimeUnavailable + Proxies.spec.ts AC-6 双用例（降级 + listMock 仍调用）

— end —
