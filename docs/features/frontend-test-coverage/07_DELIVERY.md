# 07_DELIVERY — T-051 frontend-test-coverage

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05 · dev-frontend 子 agent 实现，orchestrator 真跑 verify_all 闸门

## 需求

补齐前端 store/composable/api 的覆盖缺口（测试审计 B-1/B-2/B-3/B-5），尤其 `api/client.ts`（所有请求公共层）与 `useProxyForm`（T-037 "同列名不同语义" bug 高发区）。

## 改动（+84 前端测试，342→426；7 个新 spec，0 源码改动）

- **B-1** `stores/__tests__/proxies.spec.ts`（12）+ `wizard.spec.ts`（7）：proxies CRUD 本地数组维护（push/findIndex 命中·未命中/filter）+ loading + T-047 error ref（apiError 透传 + 保留旧列表，普通 Error fallback）；wizard checkWizard/completeWizard 状态流转。
- **B-2** `composables/__tests__/useProxyForm.spec.ts`（17）：watch(type) 切换清残留字段（tcp↔http）、handleTypeChange、toProxyInput 按 isTcpUdp 两路输出。
- **B-3** `statusUtils.spec.ts`（12，全 ProcessState 枚举）+ `components/__tests__/useLogLevelFilter.spec.ts`（8，补齐 log composable 簇最后一个）+ `useServiceStatus.spec.ts`（10，setup-host 挂载触发 onMounted）。
- **B-5** `api/__tests__/client.spec.ts`（18）：用 `apiClient.defaults.adapter` 合成响应走完真实拦截器链 —— 200 解 JSON、CSRF 注入、4xx/5xx 抛错 + extractErrorMessage 取后端 message、401 跳转（非/login→跳、已在/login·/setup 不跳）、extractApiError/extractErrorMessage 的 axios vs 普通 Error 分支（T-043 契约）。

## 验证

- `npx vitest run`：**426 passed**（39 files）。`vue-tsc --noEmit` / `eslint .` 净。
- orchestrator 真跑 `bash scripts/verify_all.sh`（完整含 e2e）：**PASS 32 / WARN 0 / FAIL 0**。baseline.json v23（frontend_tests 426 / test_count 734）。

## Adversarial tests

- useProxyForm：tcp→http 必清 remotePort、http→tcp 必清 customDomains（反向证伪"残留脏字段提交"）。
- client.ts：401 在 /login·/setup 不跳转（反向证伪重定向循环）；结构化 apiError 透传 message vs 普通 Error 走 fallback（T-043 契约双侧）。

## 发现的观察（报告不修）

1. `stores/proxies.ts` 的 create/update/delete 不更新 `error` ref（T-047 有意分工：error 仅覆盖列表加载，CRUD 失败靠异常上抛给页面 message）。已在测试注释固化该契约。
2. `composables/useServiceStatus.ts` 用 `e instanceof Error ? e.message : '加载失败'` 而非项目约定的 `extractErrorMessage` —— 结构化 axios 错误拿不到后端 `response.data.error.message`。该端点有 5s 探测兜底、契约上"不会 5xx"，现实影响小；属与 T-043 全项目错误处理约定的小偏差，建议未来一并对齐（与 T-048 A4 修 PublicIpDetector 同类）。按现状测了两条分支，未改行为。

## Insight

- `api/client.ts` 这类请求公共层用 `apiClient.defaults.adapter` 合成响应即可走完真实拦截器链（CSRF 注入 / 401 跳转 / 错误解包），afterEach 还原 adapter + CSRF getter 零泄漏 —— 不必 mock 整个 axios，测的是真实拦截器行为。
- 同簇 composable 覆盖要对账（log/ 下 5 个 composable 此前漏了 useLogLevelFilter 一个）；"同簇漏一两个"是覆盖不均的典型信号，补齐时按目录清点。
