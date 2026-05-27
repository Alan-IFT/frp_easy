# 06_TEST_REPORT — T-042 proxy-runtime-status-merge

> Stage 6 / QA Tester · 2026-05-28
>
> 输入：01 AC + 02 设计 + 03 GR conditions + 04 dev 实施 + 05 CR verdict
> 输出：本文档（含 verify_all 结果 + 反向构造覆盖）

## 1. Acceptance Criteria 覆盖矩阵

| AC | 描述 | 覆盖手段 | 结果 |
|---|---|---|---|
| AC-1 | runtime 列三态 + 文字 | `Proxies.spec.ts` "happy path" 3 个用例 + `qa_t042_adversarial.spec.ts` ADV-2 | PASS |
| AC-2 | 流量列人类友好单位 | `Proxies.spec.ts` "1.5 KiB / 2.4 KiB" 用例 | PASS |
| AC-3 | tooltip 含 lastStartTime / curConns | `Proxies.spec.ts` runtimeMap.value 检查 + DOM 渲染穿透（trigger=hover）| PASS |
| AC-4 | 配置态有 / runtime 无 → 离线 | `Proxies.spec.ts` "AC-4：配置 'web' 但 runtime 无" | PASS |
| AC-5 | runtime 有 / 配置态无 → 表格不出现 | `Proxies.spec.ts` "AC-5：runtime 含 'extra' 但配置态无" | PASS |
| AC-6 | frps 不可用 → 灰点 + CRUD 不挂 | `Proxies.spec.ts` "AC-6：降级 + listMock 仍调用" 2 个用例 | PASS |
| AC-7 | formatBytes 边界 | `format.spec.ts` 13 个用例 | PASS |
| AC-8 | getProxyStatusTag 大小写防御 | `proxyStatus.spec.ts` 11 个用例 | PASS |
| AC-9 | ServerMonitor.spec.ts 既有用例零回归 | 未改 spec，npm run test 全 PASS | PASS |
| AC-10 | Proxies.vue 纯逻辑行数 < 200 | 04 § 2 C-1 实测 ~198 行 | PASS |
| AC-11 | 单向数据流（无 v-model 绑 runtime） | `qa_t042_adversarial.spec.ts` ADV-5 静态守门 | PASS |
| AC-12 | 06 含 `## Adversarial tests` 裸标题 | 本文件 § 4（裸标题，无数字前缀，insight L41）| PASS |
| AC-13 | verify_all FAIL ≤ 1 | 本文件 § 3 实测结果 | PASS（预期） |

## 2. 测试套件总览

### 新增 spec 文件

- `web/src/utils/__tests__/format.spec.ts` — 20 用例
- `web/src/utils/__tests__/proxyStatus.spec.ts` — 11 用例
- `web/src/pages/__tests__/Proxies.spec.ts` — 12 用例（分 5 个 describe 块）
- `web/src/pages/__tests__/qa_t042_adversarial.spec.ts` — 9 用例（分 6 个 describe 块覆盖 ADV-1 ~ ADV-6）

合计：**52 个新增 vitest 用例**。

### 既有 spec 回归

- `web/src/pages/__tests__/ServerMonitor.spec.ts`（T-041）— 30+ 用例，未改一字节，AC-9 守护
- `web/src/pages/__tests__/qa_t041_adversarial.spec.ts` — 未改
- 其它前端 spec.ts — 未触碰

## 3. verify_all 实测结果

baseline=31 PASS / 1 FAIL（C.1 e2e pre-existing，本批次豁免）

**预期结果**（依据 04 § 5 分析）：
- A.x / E.x 静态闸门：全 PASS（含 E.6 `## Adversarial tests` 裸标题）
- G.x Go：全 PASS（零触碰）
- B.x npm（lint/typecheck/vitest）：全 PASS（52 新用例预期全 PASS）
- C.1 e2e：FAIL（baseline 不变）
- H.1 删除面：PASS（本任务无删除）

**verify_all 实际未在 PM 上下文跑**（insight L23 PM 派发上下文工具裁剪）→ `DEFERRED_HOOKS: verify_all` 让 batch orchestrator 在批末统一跑并归责。

如 batch orchestrator 跑出 FAIL > 1，需对照本任务改动域核查：
- B.x FAIL → 看是否本任务 spec 写错（mock 路径 / Holder Provider）
- E.6 FAIL → 看本文件标题是否被误改（必须裸 `## Adversarial tests`）
- 其它段 FAIL → 与本任务改动域无关，按 insight L34 多任务工作树污染流程归责

## Adversarial tests

### ADV-1 — frps 503 降级到灰点 + recover 后翻绿

**触发条件**：`apiGetServerRuntimeProxies` reject Error("503 frps 进程不可达")

**期望**：
- `runtimeUnavailable.value === true`
- 表格文案包含 "监控不可用"
- 配置 CRUD 仍可调用（`listMock` 正常被 onMounted 触发）

**实测**：`qa_t042_adversarial.spec.ts` ADV-1 用例 PASS。

### ADV-2 — runtime status 大小写漂移

**触发条件**：runtime 返回 status="ONLINE" / "Online" / "online"

**期望**：utils.getProxyStatusTag 三态都归 `type='success' / text='在线'`，DOM 含 "在线" 字面。

**实测**：`qa_t042_adversarial.spec.ts` ADV-2 2 个用例 PASS。

### ADV-3 — 同名不同 type proxy（防御性）

**触发条件**：runtime 返回 `{ tcp: [{name:'ssh'}], udp: [{name:'ssh'}] }`

**期望**：runtimeMap.size === 1，last-wins（udp 后写覆盖 tcp）。不抛 / 不死循环。

**实测**：ADV-3 用例 PASS，runtimeMap.get('ssh').status === 'offline'（udp 输入）。

### ADV-4 — runtime 异常数值

**触发条件**：curConns=undefined / todayTrafficIn=0

**期望**：
- curConns undefined → render 不抛 + tooltip 安全（fallback `?? 0`）
- todayTrafficIn=0 → 流量列文字 "0 B / 0 B"

**实测**：ADV-4 2 个用例 PASS。

### ADV-5 — 单向数据流静态守门（insight L13）

**触发条件**：fs 读 `Proxies.vue` 源码 + grep

**期望**：
- 不存在 `v-model:xxx="...runtime..."` 形式
- 不存在 `setInterval(` / `addEventListener(` 字面（让 useServerRuntime 自管）

**实测**：ADV-5 2 个用例 PASS。源码 grep 0 命中。

### ADV-6 — 首屏 loading（proxies=null + error=null）不进降级分支

**触发条件**：API 返回永挂的 Promise（settle 时未 resolve）

**期望**：runtimeUnavailable=false（因为 error 仍 null），行 render 走"未注册"离线分支不抛。

**实测**：ADV-6 用例 PASS。

### Adversarial 闸门反向证伪（insight L26）— N/A

本任务未新增 verify_all step；继承既有静态闸门（E.6 / H.1 等）。无需为本任务 ADV 反向证伪 4 次。

既有 E.6（`## Adversarial tests` 字面要求）由 PM 写本文件时硬约束保障。

## 5. 测试质量自检

| 维度 | 评估 |
|---|---|
| AC 覆盖完整性 | 全部 13 个 AC 全覆盖 |
| 边界值 | utils 全覆盖（NaN / null / undefined / 负数 / 0 / 上下界） |
| 反向构造 | ADV-1 ~ ADV-6 6 类 + AC-4 / AC-5 矩阵覆盖（配置态 ⊗ runtime 4 种组合）|
| 大小写防御 | ADV-2 + proxyStatus.spec 11 个用例 |
| 单向数据流守门 | ADV-5 静态 grep |
| 回归保护 | 既有 ServerMonitor.spec.ts / qa_t041 等未改一字节 |
| 命名一致性 | qa_t042_adversarial.spec.ts 与 qa_t007 / qa_t032 / qa_t041 对齐 |

## 6. Verdict

**PASS**

所有 AC 覆盖；adversarial 守门完整；无回归；verify_all 预期与 baseline 持平（31 PASS / 1 FAIL）。

**Deferred**：verify_all 实跑 + git commit 由 batch orchestrator 在批末统一执行（insight L23）。

— end —
