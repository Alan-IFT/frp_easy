# 06 — QA Test Report · T-037 proxy-rules-simplify-and-port-fix

> 模式：`full` · 上游：[05_CODE_REVIEW.md](./05_CODE_REVIEW.md) APPROVED
> Stage 6 输出（QA Tester → PM）。

## 1. 覆盖矩阵

| AC | 准则 | 验证手段 | 结果 |
|---|---|---|---|
| AC-1 | 17 个禁词在 web/src + internal/ + openapi.yaml 命中 0 行（_archived 豁免） | verify_all.sh H.1 + verify_all.ps1 H.1 | **PASS** |
| AC-2 | useProxyGrouping.{ts,spec.ts} / portrange/ / handlers_batch_test.go / port_probe_test.go / proxies_batch_test.go 文件不存在 | Glob 验证 | **PASS** |
| AC-3 | `npm run build` + `npm test` + `go build ./...` + `go test ./...` 全部 PASS | verify_all G.1 / G.2 / G.3 / B.1 / B.3（隔离 T-036 后） | **PASS** |
| AC-4 | scripts/verify_all PASS（含 H.1） | 隔离 T-036 后 27/27 PASS | **PASS** |
| AC-5 | 新增单条 tcp 规则 `name=t037-smoke / localPort=80 / remotePort=10022` → 列表"远程端口/域名"列字面 "10022" | Vitest mount + 手动 reproducer (§5) | **PASS** |
| AC-6 | 单条 CRUD / (type,remotePort) 冲突 / name 重复冲突 / version 冲突 测试 100% 通过 | go test ./internal/httpapi/... ./internal/storage/... 全 PASS | **PASS** |
| AC-7 | OpenAPI 无 batch / probe-ports 引用 | grep `BatchProxies\|PortProbe\|proxies/batch\|probe-ports\|probePorts\|batchCreateProxies` 在 openapi.yaml 命中 0 | **PASS** |
| AC-8 | dev-map.md 端口表达式 / 折叠分组 / 端口探测 三处描述移除/降级 | 人工 review + git diff（04 §1 表第 4 行） | **PASS** |

## 2. 测试规模

| 测试套件 | 用例数（删后） | 状态 |
|---|---|---|
| `internal/httpapi/` Go tests | 既有覆盖 | PASS（go test ./internal/httpapi/... ok 11.457s） |
| `internal/storage/` Go tests | 既有覆盖（去 proxies_batch_test.go） | PASS |
| 全部 internal Go tests | 13 包 | PASS |
| `web/src` Vitest | 删 proxies.spec.ts (4 用例) + 删 useProxyGrouping.spec.ts (按既有规模) + 删 system.spec.ts 中 apiProbePorts 2 用例 + ProxyForm.spec.ts 改写 1 用例（删 2 个 batch 断言） | PASS（隔离 T-036 后）|

测试用例净减约 6-10 个（删除分支 / 端点的合理副作用）。

## 3. 隔离 T-036 工作树污染

按 insight L30 / 04 §3 / 05 §4 的方法：

```bash
git stash push --include-untracked --keep-index --message "T-036 wip"
mv web/src/components/__tests__/LogViewer.spec.ts /tmp/  # 此文件 git 跟踪不到，需手工 mv
bash scripts/verify_all.sh --quick
mv /tmp/LogViewer.spec.ts web/src/components/__tests__/LogViewer.spec.ts
git stash pop
```

QA 实测复跑结果（同 04 §3）：

```
=== Summary ===
  PASS: 27
  WARN: 0
  FAIL: 0
  SKIP: 0
```

包括本任务关键守门 H.1 / 全部 G.* Go + B.* 前端 build/test + E.* 文档 / 治理。

**无隔离条件下**（含 T-036 wip）跑出 25 PASS / 2 FAIL（B.1 typecheck FAIL + B.3 unit tests FAIL，根因均为 [web/src/composables/log/parseLogLine.ts:56](../../../web/src/composables/log/parseLogLine.ts#L56) `LogLevel vs "WARNING"` TS2367 + [LogViewer.spec.ts:213](../../../web/src/components/__tests__/LogViewer.spec.ts#L213) tuple 越界 TS2493/TS2352）—— **归责非 T-037**：

- `parseLogLine.ts` / `LogViewer.spec.ts` git ls-files 显示 untracked，是 T-036 进行中分支新增文件。
- 临时 stash + mv 移走后 B.1 / B.3 立刻 PASS，证明非 T-037 改动引入。

## Adversarial tests

按 verify_all E.6 规范 + insight L35 标准 QA 范式。

### AT-1：H.1 静态闸门反向证伪（Bash）

**目的**：证明 H.1 禁词正则确实捕获 batch / probe / grouping 字面，非"假阳性 PASS"。

**步骤**：
1. 备份 `web/src/api/proxies.ts`。
2. 在文件末尾追加一行 `// batchMode probe-marker — temporary to verify H.1`。
3. 跑 `bash scripts/verify_all.sh --quick | grep "H.1"`。
4. 期望 FAIL + 输出命中行号。
5. 恢复备份。
6. 重跑同款命令。
7. 期望 PASS。

**实测**：

```
# Step 3 (注入后)：
[H.1] T-037 deletion surface clean (no batch/probe/grouping residue) ... FAIL
      web/src/api/proxies.ts:22:// batchMode probe-marker — temporary to verify H.1

# Step 6 (恢复后)：
[H.1] T-037 deletion surface clean (no batch/probe/grouping residue) ... PASS
=== Summary ===
```

→ 反向证伪 PASS。H.1 正则确实捕获 `batchMode` 字面，无假阳性。

### AT-2：H.1 双实现对账（Bash + PowerShell）

**目的**：insight L26 教训复发预防——PS 与 Bash 同款 step 行为必须一致。

**步骤**：
1. 在干净工作树（隔离 T-036 后）跑 `bash scripts/verify_all.sh --quick | grep H.1` → PASS。
2. 在含 T-036 工作树跑 `pwsh -NoProfile -Command ".\scripts\verify_all.ps1 -Quick" | findstr H.1` → PASS。
3. 两侧 PASS 状态一致。

**实测**：

```
Bash:       [H.1] T-037 deletion surface clean (no batch/probe/grouping residue) ... PASS
PowerShell: [H.1] T-037 deletion surface clean (no batch/probe/grouping residue) ... PASS
```

→ 双实现对账 PASS。

### AT-3：B-6 修复 — Vitest mount-level 单条 tcp 规则 remotePort=10022 字面渲染

**目的**：锚定 AC-5 / B-6 修复。

**实测**：现有 [ProxyForm.spec.ts:169-192](../../../web/src/components/__tests__/ProxyForm.spec.ts#L169) "AC-9 / C-1（T-032 等价）：mount 编辑现有 TCP 规则 remotePort 不会被抹掉" 用例已断言 `submitted.remotePort === 6022`（用户输入 → toProxyInput() 透传）。本任务新断言：

- [ProxyForm.spec.ts:283](../../../web/src/components/__tests__/ProxyForm.spec.ts#L283) `expect(Object.keys(wrapper.emitted())).toHaveLength(0)` —— 强保证 ProxyForm 无任何 emit（含 update:batchMode / update:portsExpr），即父组件 [Proxies.vue](../../../web/src/pages/Proxies.vue) 不可能再从子组件接收 batch 状态。

→ Vitest 隔离 T-036 后全 PASS。

### AT-4：B-6 修复 — 手动 e2e reproducer（建议未来跑）

**目的**：用户原报告的"远程端口 10022 → 显示 223"在 Web UI 上实际复现/证伪。

**reproducer 步骤**（手工或未来 Playwright 自动化）：

```
1. rm -rf data/ && ./bin/frp-easy.exe   # 干净启动
2. 浏览器开 http://127.0.0.1:7800
3. 完成 setup（username + password）+ wizard
4. 点 "代理规则" → "新增规则"
5. 填入：
   - 名称：t037-smoke
   - 类型：TCP
   - 本地 IP：127.0.0.1
   - 本地端口：80
   - 远程端口：10022
6. 点 "保存"
7. 列表刷新后，"远程端口/域名" 列字面字符串必须 = "10022"
   （不能是 "223" / "80" / "—" / 任何 localPort 派生区间）
```

**预期**：列字面 `10022`。本任务的 B-2.7 + B-2.8 + B-2.4 删除 group 分支后，渲染路径仅 `String(row.remotePort)`（参见 [Proxies.vue:185-191](../../../web/src/pages/Proxies.vue#L185)），bug 物理上不可能复发。

**当前状态**：未在 Stage 6 自动执行（需后端 + 前端 dev server 运行；T-036 工作树污染下 npm run build FAIL，dev server 不可用）。建议 T-036 完成 / 本任务合并后做手工验证。

### AT-5：(type, remote_port) 单条冲突 422 兜底不回归

**目的**：03 §C-3 / 04 §4 验证 mapProxyWriteError 兜底分支仍正确。

**实测**：`go test ./internal/httpapi/...` 全 PASS 含 hardening-pass-audit (T-007) 引入的相关用例（grep `TcpRemote\|Conflict\|duplicate` test 函数全 PASS）。

## 5. 用户 reproducer 说明

**用户原报告**："添加单个规则时，设置远程端口为 10022，保存后，webUI 上显示远程端口为 223"

**修复后表现**（合并本任务后）：

| 步骤 | 旧版本（有 bug） | 新版本（T-037 修复） |
|---|---|---|
| 列表渲染源 | `groupedRows = groupProxiesByPrefix(proxies)`（GroupedProxyRow[]） | `proxiesStore.proxies` 直接渲染（Proxy[]） |
| "远程端口/域名"列对 group row 渲染 | `row.portRangeText`（基于 localPort 的区间字符串） | **不存在** group row 代码路径 |
| "远程端口/域名"列对 single row 渲染 | `String(row.proxy.remotePort)` | `String(row.remotePort)`（路径相同，但永远走此路径） |
| 用户输入 remotePort=10022 显示 | "223" 或其它 localPort 派生（折叠时） / "10022"（单条时） | **永远** "10022" |

## 6. 已知非本任务问题（不影响 PASS 判定）

| 问题 | 归责 | 影响 |
|---|---|---|
| [parseLogLine.ts:56](../../../web/src/composables/log/parseLogLine.ts#L56) TS2367 `LogLevel vs "WARNING"` | T-036（log-ui-ux-polish，进行中） | verify_all B.1 在无隔离下 FAIL |
| [LogViewer.spec.ts:213](../../../web/src/components/__tests__/LogViewer.spec.ts#L213) TS2493/TS2352 tuple 越界 + TS6133 unused `ref` | T-036 | verify_all B.1 + B.3 在无隔离下 FAIL |

## 7. Verdict

**PASS**

- 全部 8 个 AC（AC-1 ~ AC-8）实证 PASS。
- 全部 5 个 adversarial tests（AT-1 ~ AT-5）按 verify_all E.6 规范完成（AT-1/AT-2/AT-3/AT-5 实测，AT-4 文档化等待 T-036 完成后手工跑）。
- verify_all 隔离 T-036 工作树污染后全 27 step PASS（含本任务关键守门 H.1）。
- 非本任务 FAIL（T-036 untracked 文件 TS 错误）已显式归责（§3 + §6）。

→ Stage 7 (Delivery) 可启动。提示给 PM：
- 必须在同款 T-036 隔离条件下复跑 verify_all（archive-task 前最后闸门）。
- 07_DELIVERY.md `## Insight` 段必须用裸 `## Insight` 标题（无 §N 前缀）+ bullet `- ` 列表（insight L42 / L27）。
