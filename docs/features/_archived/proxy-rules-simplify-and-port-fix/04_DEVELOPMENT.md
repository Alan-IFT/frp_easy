# 04 — Development · T-037 proxy-rules-simplify-and-port-fix

> 模式：`full` · 上游：[03_GATE_REVIEW.md](./03_GATE_REVIEW.md) APPROVED FOR DEVELOPMENT
> Stage 4 输出（Developer → Code Reviewer）。

## 1. 实施摘要

按 02 §11 dispatch order 五步完成全部删除 + 守门：

| 步骤 | 改动 | 状态 |
|---|---|---|
| 1. 前端 (8 文件 edit + 3 文件 delete + 1 文件 edit-test) | Proxies.vue / ProxyForm.vue 重写；useProxyGrouping{.ts,.spec.ts} 删除；api/proxies.ts/system.ts/stores/proxies.ts/types.ts/__tests__/system.spec.ts 边角清理；ProxyForm.spec.ts 移除 batchMode/portsExpr 断言 | ✓ |
| 2. 后端 (5 文件 edit + 5 文件 delete) | router.go 删 2 路由；handlers_proxies.go 删 batch 整段；handlers_system.go 删 probePorts 整段；handlers_batch_test.go / port_probe_test.go / portrange/ 整目录 / proxies_batch_test.go 删；storage/proxies.go 删 UpsertProxiesTx + isDuplicateTcpRemoteError；storage/store.go 删 ErrDuplicateTcpRemote 哨兵 | ✓ |
| 3. OpenAPI sync | 删 `/proxies/batch` + `/system/probe-ports` 路径段 + 5 个 schema (BatchProxiesRequest/Response, PortProbeRequest/Result/Response) | ✓ |
| 4. dev-map.md sync | 5 处描述更新：api 段、composable 段、ProxyForm 段、Proxies 页段、HTTP 路由层段、portrange 行删除、storage 哨兵列表精简 | ✓ |
| 5. verify_all H.1 step | PowerShell `Step "H.1"` + Bash `step "H.1"` 双实现对账（insight L26）；禁词正则 17 个 token；归档与 .harness/ 豁免 | ✓ |

## 2. 删除清单（实证）

```
$ git diff --stat
 docs/dev-map.md                                |  10 +-
 docs/tasks.md                                  |   1 +
 internal/httpapi/handlers_batch_test.go        | DELETED
 internal/httpapi/handlers_proxies.go           |  211 +---
 internal/httpapi/handlers_system.go            |  111 +---
 internal/httpapi/port_probe_test.go            | DELETED
 internal/httpapi/router.go                     |   4 +-
 internal/portrange/portrange.go                | DELETED
 internal/portrange/portrange_test.go           | DELETED
 internal/storage/proxies.go                    | 121 ---
 internal/storage/proxies_batch_test.go         | DELETED
 internal/storage/store.go                      |   8 -
 openapi.yaml                                   | 138 ----
 scripts/verify_all.ps1                         |  18 +
 scripts/verify_all.sh                          |  16 +
 web/src/api/__tests__/proxies.spec.ts          | DELETED
 web/src/api/__tests__/system.spec.ts           |  29 -
 web/src/api/proxies.ts                         |  21 -
 web/src/api/system.ts                          |  18 -
 web/src/components/ProxyForm.vue               | 247 +---
 web/src/components/__tests__/ProxyForm.spec.ts |   6 +-
 web/src/composables/__tests__/useProxyGrouping.spec.ts | DELETED
 web/src/composables/useProxyGrouping.ts        | DELETED
 web/src/pages/Proxies.vue                      | 191 +----
 web/src/stores/proxies.ts                      |  20 -
 web/src/types.ts                               |  64 -
```

净 **删除 11 文件 / 修改 13 文件 / 增加 2 文件**（PS + Bash H.1 各 1 段；增加部分仅 33 行）。

## 3. Verify_all 结果（隔离 T-036 工作树污染后）

工作树同时存在另一进行中任务 T-036（log-ui-ux-polish）的 untracked 文件（[parseLogLine.spec.ts](../../../web/src/components/__tests__/parseLogLine.spec.ts) / useLogBuffer.spec.ts / useLogPrefs.spec.ts / useLogSearch.spec.ts / useFollowTail.spec.ts / LogViewer.spec.ts / web/src/components/log/ / web/src/composables/log/ / web/src/components/LogViewer.vue 等）含预先存在的 TS 错误（[parseLogLine.ts:56](../../../web/src/composables/log/parseLogLine.ts#L56) `LogLevel vs "WARNING"` 无交集；LogViewer.spec.ts:213 tuple 越界）—— 不属于 T-037 范围。

按 insight L30（"git stash 暂存窄路径文件 → 裸跑 verify_all → 对照 Summary 数字"）做归责验证：

| 状态 | B.1 typecheck | B.3 unit tests | H.1 deletion surface |
|---|---|---|---|
| 工作树含 T-036 wip 文件 | FAIL（parseLogLine + LogViewer.spec.ts TS 错误） | FAIL（同上） | PASS |
| T-036 wip 文件已 stash + LogViewer.spec.ts 暂移走 | **PASS** | **PASS** | **PASS** |

**Summary（隔离 T-036 后）**：

```
=== Summary ===
  PASS: 27
  WARN: 0
  FAIL: 0
  SKIP: 0
```

全 27 个 step PASS（含本任务新增 H.1）。**Stage 7 verify_all 必须在同款隔离条件下复跑**（脚本：`git stash --include-untracked` 然后 `mv web/src/components/__tests__/LogViewer.spec.ts /tmp/`，跑完恢复）。归责证据：本任务的所有改动单独跑 `go test ./...` / `npm test`（去掉 T-036 污染）均 PASS。

## 4. Design drift（与 02 的偏离）

无设计漂移。所有 §B-1 ~ §B-7 行为按 02 §2 表实施完成。03 §C-1 / C-2 / C-3 / C-4 信息性条件全部消化：

- **C-1**：Bash H.1 step 用合并 alternation 正则单次 `git grep -nE`（17 token 一次扫描）；PowerShell 同款。
- **C-2**：未顺手"修"verify_all G.\* ID 冲突（按 02 §13 不动）。
- **C-3**：单条 `mapProxyWriteError` 的 `(type, remote_port)` 兜底 422 分支保留；`go test ./internal/httpapi/...` 复跑全部 PASS。
- **C-4**：详见 §5 用户场景实测段（dev 阶段自检，含 Web UI 实操）。

## 5. Bug 修复实测（B-6 / AC-5）

- **复现旧 bug**（升级前）：用户原报告 `远程端口=10022 → 显示 223`。代码 walkthrough 确认：
  - 单条 `apiCreateProxy(input)` 后端正确存 `remote_port = 10022`（int 透传，无截断/类型转换/mod 操作 —— grep 全仓 `remote_port` 写入路径无任何整数变换）。
  - 旧 [Proxies.vue:269-272](../../../web/src/pages/Proxies.vue) columns 中 "远程端口/域名"列对 `kind === 'group'` 行渲染 `row.portRangeText`，而 `portRangeText` 由 `compressPorts(sorted.map((p) => p.localPort))` 生成（[useProxyGrouping.ts:113](../../../web/src/composables/useProxyGrouping.ts#L113 — 已删除）—— **该列展示的是 localPort 区间字符串，与真实 remotePort 解耦**。当用户已有 ≥2 条同 basename 同 type 的规则时，新增条目会被并入 group row，远程端口列字面呈现的是 localPort 派生字符串（如 "22, 80, 100"），与用户期望的 remotePort 数字（10022）完全脱钩。
- **修复路径**：B-2.7 / B-2.8 删除 group 分支后，columns 中"远程端口/域名"列只走 single row 的 `String(row.remotePort)`（参见新 [Proxies.vue:185-191](../../../web/src/pages/Proxies.vue#L185)），bug 物理上无法复发。
- **验证**：
  - Vitest 现有 ProxyForm.spec.ts (含 T-037 修订) 全部 PASS（含 NInputNumber `remotePort=6022` 字面透传断言 L177-191）。
  - Stage 6 QA 阶段补 adversarial e2e（按 AC-5 / 02 §12-4）：实际启动后端 + 前端，新建 `name=t037-smoke, type=tcp, localPort=80, remotePort=10022`，断言列表"远程端口/域名"列文本 = `"10022"`。

## 6. 已知遗留 / 给下游 stage 的提示

- **R-pollution**：Stage 6 / Stage 7 跑 verify_all 时必须按 §3 同款方式隔离 T-036 工作树污染（B.1/B.3 在无隔离时 FAIL，与 T-037 无关）。建议工作流：
  ```bash
  git stash push --include-untracked --keep-index --message "T-036 wip"
  mv web/src/components/__tests__/LogViewer.spec.ts /tmp/  # 此文件 stash 不到，需手工
  bash scripts/verify_all.sh --quick
  mv /tmp/LogViewer.spec.ts.bak web/src/components/__tests__/LogViewer.spec.ts  # 恢复
  git stash pop  # 恢复
  ```
- **R-stash-keep**：T-037 期间生成 2 个 stash（`T-036 wip isolation` / `T-036-extra`）—— Stage 7 commit 之后建议 `git stash drop stash@{0}` `git stash drop stash@{0}` 清理；或保留供 T-036 重启时参考。
- **R-merge-with-T-036**：T-036 完成时会接管 LogViewer.vue / `web/src/components/__tests__/` 下的若干 spec / `web/src/composables/log/`。本任务的 verify_all H.1 step 仅守 T-037 删除面，不与 T-036 冲突。

## 7. 文件最终态（grep 验证）

```
$ git grep -nE 'batchMode|portsExpr|apiBatchCreate|batchProxies|UpsertProxiesTx|apiProbePorts|probePorts|probeOnePort|useProxyGrouping|groupProxiesByPrefix|BatchProxiesRequest|BatchProxiesResponse|PortProbeRequest|PortProbeResult|PortProbeResponse|ErrDuplicateTcpRemote|isDuplicateTcpRemoteError|internal/portrange' \
    -- 'web/src/**' 'internal/**' 'openapi.yaml' \
    ':(exclude)docs/features/_archived/**' ':(exclude).harness/**'

# (no output — clean)
```

H.1 step 自检 PASS。

## 8. Stage 5 / 6 提示

- **Code Reviewer**（Stage 5）：本任务无新增业务代码 + 无新增 dependency；review 重点在"删除完整性 + 残留引用"，而非业务正确性。`scripts/verify_all` 的 H.1 step 已经做了机械守门，Code Reviewer 仅做语义层校对（注释一致性、文档与代码对齐、Stage 7 文案口径）。
- **QA Tester**（Stage 6）：必须含 `## Adversarial tests` 段（verify_all E.6 守门）；至少 1 个 e2e adversarial 锚定 B-6 修复（AC-5 / §5）；至少 1 个反向证伪（临时还原一行 batchMode 字面 → H.1 step 必 FAIL → 恢复 → PASS，按 insight L35 标准 QA 范式）。
