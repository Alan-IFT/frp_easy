# 05 CODE REVIEW · T-018 upload-bin-multiport-ip-probe

> Reviewer：Code Reviewer
> 日期：2026-05-23

## 1. 评审范围

T-018 三大分区落地审计：A 二进制上传 / B 公网 IP 多源 / C 多端口+预设+探测；核 B-1~B-12 + Q1/Q2 修订是否真进代码、前后端契约是否一致、需求/设计偏离、测试质量、安全与工程纪律。

## 2. 检查矩阵

| 检查项 | 结果 | 证据 |
|---|---|---|
| **B-1** `FRP_EASY_PUBLIC_IP` Go 端短路 | ✅ | `handlers_system.go:257` + spec L23-46 |
| **B-2** axios 不显式 Content-Type | ✅ | `api/system.ts:42-53` 无 headers；spec L47 断言 `headers` undefined |
| **B-3** ProcMgr.Status 单返回值 | ✅ | `handlers_system.go:499` |
| **B-4** 上传挂 AppLayout 不在 Wizard | ✅ | `AppLayout.vue:37`；Wizard 零命中 |
| **B-5** UpsertProxiesTx 持 s.mu | ✅ | `proxies.go:402-404`；并发用例 `proxies_batch_test.go:204-268` |
| **B-6** ParseMultipartForm + FormValue/FormFile | ✅ | `handlers_system.go:420-457`；file-first 顺序断言 PASS |
| **B-8** modernc 复合 UNIQUE 实证 | ✅ | `proxies_batch_test.go:161-199` 直接 raw INSERT 捕获 |
| **B-9** probeOnePort dual-stack `:N` | ✅ | `handlers_system.go:659` `":%d"` |
| **B-11** PE 仅 MZ 无 0x3C | ✅ | `handlers_system.go:539` `head[0]=='M' && head[1]=='Z'` |
| **B-12** 折叠正则 greedy | ✅ | `useProxyGrouping.ts:24` `/^(.+)-(\d{1,5})$/` |
| **Q1** Install maxBytes≤0 不限 | ✅ | `install.go:80-82` + Unlimited 测 |
| **Q2** BatchProxiesResponse 对象版 | ✅ | OpenAPI/Go/TS 三处一致 |
| 上传字段 `size` 前后端一致 | ❌ | **P0-1** |
| 批量字段 `basename` 前后端一致 | ❌ | **P0-2** |
| port-probe 上限 64（FR-C.3.4） | ❌ | **P1-1** |
| 错误中文化 | ✅ | 全部用户可见消息中文 |
| 无硬编码 secret | ✅ | grep 通过 |
| 无 TODO/FIXME 残留 | ✅ | 零命中 |
| 无 .js 残留 | ✅ | T-010 insight 满足 |
| 无新依赖 | ✅ | go.mod / package.json 未变 |
| 无 cgo | ✅ | grep 零命中 |
| 写接口 SessionAuth + CSRF | ✅ | `router.go:89-128` |

## 3. 发现

### P0-1 [CONTRACT-DRIFT] 上传响应字段 `size` vs `sizeBytes` 前后端不一致

- **后端**：`handlers_system.go:379` `Size int64 \`json:"size"\``；OpenAPI 也是 `size`。
- **前端类型**：`web/src/types.ts:124` `sizeBytes: number`。
- **前端组件**：`UploadBinButton.vue:85,91` `res.sizeBytes` / `formatBytes(res.sizeBytes)` → 生产环境会显示 `已上传 frpc（NaN undefined）`。
- **测试为何不抓**：前端 spec mock 直接返回 `{ sizeBytes: 1024 }`（与前端类型对齐，但与后端真实响应不对齐）。
- **建议**：`web/src/types.ts` + UploadBinButton.vue + spec mock 全改 `size`（最小改动方向）。

### P0-2 [CONTRACT-DRIFT] 批量字段 `basename` vs `namePrefix` 前后端不一致 — 生产环境批量按钮 100% 422

- **后端**：`handlers_proxies.go:245-251` `Basename string \`json:"basename"\``；OpenAPI `required: [basename, type, portsExpr]`。
- **前端**：`web/src/types.ts:137` 与 `Proxies.vue:165` 都用 `namePrefix`。
- **影响**：JSON 解码得 `req.Basename=""` → `batchBasenameRE.MatchString("")` 失败 → 422 "basename 非法"。**生产环境前端"批量新增"100% 失败**。
- **测试为何不抓**：后端测试用 `{"basename":"web"}` 路径正确；前端 spec 只测 mock client 被调用并透传 `namePrefix`，**未验后端**。契约缝隙没人测。
- **建议**：前端三处全改 `basename` 与后端 / OpenAPI 一致。

### P1-1 [REQ-DEVIATION] port-probe 单次上限 32 而 FR-C.3.4 / AC-C.3.5 要求 64

- **需求**：01 FR-C.3.4 "端口列表上限 **64**（PM-DECIDED）"；AC-C.3.5 "ports 含 **65** 项 → 422"。
- **实现**：`handlers_system.go:581` `portProbeMaxCount = 32`；OpenAPI `maxItems: 32`；测试用 33 触发 422。
- **影响**：与 PM 决策违背；用户构造 50 端口请求会被 422。
- **建议**：`portProbeMaxCount = 64`；测试 33→65；openapi.yaml 32→64。

### P1-2 [TEST-WEAK] B-12 折叠正则覆盖合格但不极致（NIT）

折叠正则 5 种修订要求全在，端口越界 2 条已加；未覆盖 `name="-6000"` 与 `name="web-"` 边界。**不阻塞**。

### P1-3 [LOGIC] `fetchPublicIP` ctx Done 与最后成功几乎同时的微小竞态

`handlers_system.go:296-307` 在 ctx Done 后直接返回 ErrMsg，可能丢弃同瞬已 push 到 ch 的 winner。**罕见**，可加非阻塞 select 二次救一下。**MINOR**。

### P2-1 [MAINT] `handlers_batch_test.go:139` 残留死代码 `if err := store.UpsertProxy(t.Context(), nil); err != nil`

注释自陈"跳过"，但 statement 仍执行。**MINOR**。

### P2-2 [MAINT] TestUploadBin_ConcurrentSameKind 不强求 409 出现

`handlers_upload_test.go:391-394` 注释"不强求 409（依赖时序），仅作 advisory（log）"。AC-A.6 锁路径存在但无硬证据。**MINOR**。

### P2-3 [DOC] `TestProbePorts_HostIgnored` 命名误导

测试构造 raw body 含 `host` 字段，但 Request 结构体无该字段，JSON 解码后忽略。建议改名 `TestProbePorts_ExtraFieldsIgnored`。**NIT**。

### P2-4 [SECURITY-INFO] uploadBin 落盘失败 errno 透传

`handlers_system.go:492` `"落盘失败: " + err.Error()` 可能泄露绝对路径。仅认证用户可见，**MINOR**。

### P2-5 [STYLE] handlers_system.go 末尾多余空行

**NIT**。

## 4. 测试覆盖核查

| AC | 状态 |
|---|---|
| AC-A.1~A.10 | ✅（A.5 内存峰值留 QA Adversarial；A.6 弱断言 P2-2） |
| AC-B.1~8 | ✅ 完整 10 用例 |
| AC-C.1.1~1.8 | 后端测 PASS，**但 P0-2 让前端 e2e 失败** |
| AC-C.2 预设 | ✅ |
| AC-C.3.5 | ❌ P1-1（33 vs 65） |
| AC-C.3.6/7/8 | ✅ |
| AC 折叠 5 用例 | ✅ |

## 5. 与 04 的偏差

- 04 §5 自报"DESIGN-DRIFT 无" — 但未自察 `size↔sizeBytes` 与 `basename↔namePrefix`。开发组的契约一致性盲点。
- 04 §3 AC-C.1.x 声称全覆盖 — 真实情况：后端测 PASS 但前端 namePrefix 错位让 happy path 永不到达。
- 04 §3 AC-C.3.5 声称覆盖 — 实际上限 32 不是 64。

## 6. Verdict

**CHANGES REQUIRED**

**Blockers：2 P0 + 1 P1（共 3 个）**

- **P0-1**：upload 响应 `size` vs `sizeBytes` 前后端漂移
- **P0-2**：batch 请求 `basename` vs `namePrefix` 前后端漂移
- **P1-1**：port-probe 上限 32 vs 需求 64

**修复清单（回 Developer）**：
1. `web/src/types.ts` + `UploadBinButton.vue` + `api/__tests__/system.spec.ts` + `components/__tests__/UploadBinButton.spec.ts` — `sizeBytes` → `size`
2. `web/src/types.ts` + `pages/Proxies.vue:165` + `api/__tests__/proxies.spec.ts` — `namePrefix` → `basename`
3. `handlers_system.go:581` `portProbeMaxCount = 64`；`port_probe_test.go:103` 33→65；`openapi.yaml` 32→64 同步
4. （可选 P2 清理）

修复后无需重走前序阶段，可直接二次 Code Review 或进 Stage 6 QA。

---

## 修复确认 · 2026-05-23

| Blocker | 状态 | 实测 |
|---|---|---|
| P0-1 `sizeBytes`→`size` | ✅ FIXED | `web/src/types.ts` / `UploadBinButton.vue` / system.spec.ts / UploadBinButton.spec.ts 共 4 文件 7 处；`npm run test --run` 96/96 用例 PASS |
| P0-2 `namePrefix`→`basename` | ✅ FIXED | `web/src/types.ts` / `Proxies.vue:165` / proxies.spec.ts 共 3 文件 5 处；全仓 grep 无残留 |
| P1-1 port-probe 32→64 | ✅ FIXED | `handlers_system.go` `portProbeMaxCount=64`；`port_probe_test.go` 33→65；`openapi.yaml` 32→64；后端测试 PASS |
| P2-1 死代码 `UpsertProxy(nil)` | ✅ FIXED | handlers_batch_test.go L139/L217 删除 |
| P2-3 测试改名 | ✅ FIXED | `TestProbePorts_HostIgnored`→`TestProbePorts_ExtraFieldsIgnored` |
| P2-5 末尾空行 | ✅ FIXED | handlers_system.go |

**二次 Verdict**：**APPROVED FOR QA**。所有 P0/P1 修复，可选 P2 也一并清理。
