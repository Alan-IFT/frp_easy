# 04 DEVELOPMENT · T-018 upload-bin-multiport-ip-probe

> 合并 dev-backend + dev-frontend 两个分区的开发报告。
> 日期：2026-05-23

## 1. 范围与分区

按 02 §12 Partition assignment：

- **dev-backend**：A 上传后端 / B 公网 IP 后端 / C.1 批量后端 / C.3 探测后端 / 路由 + OpenAPI
- **dev-frontend**：A 上传 UI / C.1 批量 UI / C.2 预设 / C.3 探测 UI / 类型契约 / 折叠分组
- **dev-db**：不参与（零 migration）

派发顺序：dev-backend → dev-frontend（前端依赖后端 API + 类型契约）。

## 2. 新增 / 修改文件清单

### 后端（10 新 + 9 改）

**新增：**
- `internal/portrange/portrange.go` — `Parse(expr, maxCount)` 共享解析器（sentinel ErrEmpty/ErrBadSyntax/ErrPortOutOfRange/ErrRangeReversed + 类型 DuplicateError/TooManyError/BadSyntaxError）
- `internal/portrange/portrange_test.go` — 23 个 table-driven 用例（覆盖 11 必测点 + 12 边界）
- `internal/downloader/install.go` — `(*Manager).Install(kind, src, maxBytes)`，共享原子 rename + chmod + Windows fallback；ErrFileTooLarge sentinel；`maxBytes ≤ 0` 表示不限（下载链路走此分支，Q1）
- `internal/downloader/install_test.go` — 8 用例（HappyPath / TooLarge / AtMaxBytes / Unlimited / BadKind / UnsupportedGOOS / WindowsFallback / ReaderError）
- `internal/storage/proxies_batch_test.go` — 8 用例含 **B-5** ConcurrentWithUpsertProxy 并发用例 + **B-8** modernc UNIQUE 文本捕获 adversarial 用例
- `internal/httpapi/handlers_upload_test.go` — 11 用例
- `internal/httpapi/handlers_system_publicip_test.go` — 10 用例（env 短路 / first-wins / all-fail / non-IP / UA 头 / HTML 私有段过滤 / IPv6 advisory / 空 sources / ip.cn 两 shape / bilibili）
- `internal/httpapi/handlers_batch_test.go` — 12 用例
- `internal/httpapi/port_probe_test.go` — 11 用例

**修改：**
- `internal/downloader/downloader.go` — `doDownload` 的 step 3+4 改调 `Install`（零行为变更重构）
- `internal/storage/store.go` — 新增 `ErrDuplicateTcpRemote` sentinel
- `internal/storage/proxies.go` — 新增 `isDuplicateTcpRemoteError()` + `UpsertProxiesTx(ctx, ps)`（整段持 `s.mu`，**B-5 修订**）
- `internal/httpapi/handlers_system.go` — 重写 `fetchPublicIP` 为并发 5 源 + `FRP_EASY_PUBLIC_IP` env 短路（**B-1** Go 端首次引入）+ HTML/JSON 多 parser；新增 `uploadBin` / `validateBinaryHeader` / `probePorts` / `probeOnePort`；`PublicIPResponse` 增 `Source` 字段
- `internal/httpapi/handlers_proxies.go` — 新增 `batchProxies` / `humanizePortRangeErr` / `writeBatchProxiesError` + 常量 `BatchProxiesMaxCount=32`
- `internal/httpapi/router.go` — 注册 3 条新路由（system/upload-bin、system/probe-ports、proxies/batch），全在受保护组
- `openapi.yaml` — 新增 5 个 schema（UploadBinResponse / BatchProxiesRequest / BatchProxiesResponse / PortProbeRequest / PortProbeResult / PortProbeResponse）+ 3 个 path + PublicIPResponse.source
- `docs/dev-map.md` — 加 portrange 模块 + 路由数同步

### 前端（11 新 + 4 改）

**新增：**
- `web/src/composables/usePortPresets.ts` — `PORT_PRESETS` 含 SSH/RDP/HTTP/HTTPS/MySQL/PG/Redis/Mongo/SMB/VNC
- `web/src/composables/useProxyGrouping.ts` — **B-12 修订正则** `^(.+)-(\d{1,5})$` + compressPorts 端口区间压缩
- `web/src/components/UploadBinButton.vue` — 64 MiB 前端预校验 + NProgress 进度条 + useMessage 错误提示
- 5 个 vitest spec：`usePortPresets.spec.ts` / `useProxyGrouping.spec.ts` / `api/system.spec.ts` / `api/proxies.spec.ts` / `components/UploadBinButton.spec.ts` — 共 39 个新单测

**修改：**
- `web/src/types.ts` — 加 UploadBinResponse / BatchProxiesRequest+Response / PortProbeRequest+Result+Response；PublicIPResponse 加 `source?`
- `web/src/api/system.ts` — 加 `apiUploadBin`（**B-2**: 不显式设 Content-Type；**B-6**: 字段顺序无关）+ `apiProbePorts`
- `web/src/api/proxies.ts` — 加 `apiBatchCreateProxies`（字段 `portsExpr`）
- `web/src/stores/proxies.ts` — 加 `batchCreate` action
- `web/src/components/AppLayout.vue` — banner 内并列追加 UploadBinButton（**B-4** 仅 AppLayout，不动 Wizard）
- `web/src/components/ProxyForm.vue` — 加预设 Tag / 单端口探测按钮 / 批量模式开关 / portsExpr 输入（http/https 自动锁定 batch）
- `web/src/pages/Proxies.vue` — 折叠分组渲染（组行/单行混合 + 展开明细）+ batchCreate 提交分支
- `docs/dev-map.md` — 同步前端新模块

## 3. AC 覆盖矩阵

| AC | 模块 | 验证 |
|---|---|---|
| AC-A.1~A.10 | A 上传 | handlers_upload_test.go 11 用例 + install_test.go 落盘；AC-A.5（413 + 内存峰值）OversizeBody 验 413，内存峰值留 QA Adversarial |
| AC-B.1~B.8 | B 公网 IP | handlers_system_publicip_test.go 10 用例（含 env 短路、UA、HTML 污染过滤） |
| AC-C.1.1~1.8 | C.1 批量 | handlers_batch_test.go 12 用例 + storage proxies_batch_test.go 8 用例 |
| AC-C.2 | C.2 预设 | usePortPresets.spec.ts 6 用例 |
| AC-C.3.1~3.8 | C.3 探测 | port_probe_test.go 11 用例 |
| AC-折叠分组 | C.1 UI | useProxyGrouping.spec.ts 19 用例（覆盖 B-12 五种正则用例） |

## 4. Gate Review 修订点全部落地

| Finding | 落地位置 |
|---|---|
| B-1 FRP_EASY_PUBLIC_IP Go 端首次引入 | `handlers_system.go` `fetchPublicIP` 入口短路 |
| B-2 axios 不显式 Content-Type | `web/src/api/system.ts` apiUploadBin（spec 断言无 headers 字段） |
| B-3 ProcMgr.Status 单返回值 | `handlers_system.go` uploadBin handler |
| B-4 仅 AppLayout banner 挂载 | AppLayout.vue + 不动 Wizard.vue |
| B-5 UpsertProxiesTx 持 s.mu | `internal/storage/proxies.go` + ConcurrentWithUpsertProxy 并发单测 |
| B-6 ParseMultipartForm + FormValue/FormFile | `handlers_system.go` uploadBin handler |
| B-7 保持并发 5 源 + 5min 缓存 | `handlers_system.go` fetchPublicIP（文档化 R-14） |
| B-8 实证 modernc 复合 UNIQUE | TestIsDuplicateTcpRemoteError_FromRealDriver 捕获真实 err.Error() |
| B-9 dual-stack wildcard `:N` | probeOnePort 用 `net.Listen("tcp", ":<port>")` |
| B-10 反代 client_max_body_size | 留 QA Adversarial 实测（已在 02 §A.4） |
| B-11 仅 MZ 即接受 PE | validateBinaryHeader 注释 + 实现一致 |
| B-12 折叠正则 greedy `(.+)-(\d{1,5})$` | useProxyGrouping.ts + 19 spec 用例 |
| Q1 Install maxBytes ≤ 0 不限 | install.go 注释 + Unlimited 单测 |
| Q2 BatchProxiesResponse 对象版 | openapi.yaml + 后端响应 + 前端类型一致 |

## 5. DESIGN-DRIFT

无重大漂移。

**一处对齐说明**（dev-backend 报告）：批量请求体字段名最终落地为 `portsExpr`（非任务消息中曾提到的 `portSpec`/`localPortSpec`），按 02 §C.1.2 / FR-C.1.2 "1:1 一致映射"权威设计。前端 `apiBatchCreateProxies` 与 spec 测试也锁定此字段名，前后端一致。

## 6. 测试结果

### 后端 `go test ./...`
```
ok  github.com/frp-easy/frp-easy/internal/appconf      0.669s
ok  github.com/frp-easy/frp-easy/internal/assets       1.778s
ok  github.com/frp-easy/frp-easy/internal/auth         1.137s
ok  github.com/frp-easy/frp-easy/internal/binloc       0.626s
ok  github.com/frp-easy/frp-easy/internal/downloader   1.978s
ok  github.com/frp-easy/frp-easy/internal/httpapi     10.307s
ok  github.com/frp-easy/frp-easy/internal/portrange    0.742s
ok  github.com/frp-easy/frp-easy/internal/storage      2.266s
（共 14 包全部 PASS；go build ./... 与 go vet ./... 均无输出）
```

### 前端 `npm run test --run`
- vitest 新增 39 用例（60 → 99 总数）
- `npm run build` PASS

### dev-frontend 自跑 verify_all
- **PASS:19 / WARN:0 / FAIL:0**（与 baseline 一致）

## 7. 工程纪律

- 零新增依赖（Go modules / npm）
- 零 migration
- 零 cgo
- 零硬编码 secret
- 所有写接口走 SessionAuth + CSRF
- 端口探测仅本机（B-9 dual-stack wildcard），无远程 host 字段，防滥用
- 中文化错误消息全覆盖

## 8. 待 Stage 5+ 的 follow-up

- Code Reviewer：核 B-1~B-12 修订是否真落入代码（不只是文档）
- QA Tester：必须写 `## Adversarial tests` 英文标题（insight-index L31），覆盖：
  - 大文件上传 413（反代场景为 B-10 known limitation 记录）
  - portsExpr 跨度 > 32 / 起 > 止 / 非法字符
  - 公网 IP 全 5 源 timeout
  - 端口探测竞态（先探可用，立即被占）
  - 批量创建中途冲突，验事务全部回滚

## 9. Code Review 后续修复（2026-05-23）

Code Review 发现 3 个 blocker，已修复（详见 `05_CODE_REVIEW.md` "修复确认"小节）：

- **P0-1 contract drift**：upload 响应字段 `sizeBytes`→`size`（前端 4 文件 7 处）
- **P0-2 contract drift**：batch 请求字段 `namePrefix`→`basename`（前端 3 文件 5 处）
- **P1-1 req deviation**：port-probe 上限 32→64（后端 + openapi.yaml）

同步清理 P2-1（死代码）、P2-3（测试改名）、P2-5（末尾空行）。

后端 `go test ./...` PASS；前端 `npm run test --run` 96/96 PASS；`npm run build` PASS。前后端字段名现与 OpenAPI 契约完全一致。

**契约一致性教训** —— 写入本任务的 Insight 候选（07 阶段决定是否归档到 insight-index）：

> 前端 TS 接口与后端 Go struct 的 JSON 字段名漂移在双方 mock 测试都 PASS 时无法被捕获。补救：spec 测试或 OpenAPI codegen 应建立"契约一锤定音"机制（如 openapi-typescript），而非两边各自从 OpenAPI 抄一遍。本任务暂用 manual sync + Code Review 兜底。
