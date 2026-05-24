# 05 — Code Review · T-037 proxy-rules-simplify-and-port-fix

> 模式：`full` · 上游：[04_DEVELOPMENT.md](./04_DEVELOPMENT.md)
> Stage 5 输出（Code Reviewer → PM）。独立读 dev 产出对照 01 / 02 + 真实 diff，不修改 dev 代码（不在本 reviewer 职权）。

## 1. 审查范围

Dev 阶段提交：净 -1,201 / +151 行（含文档），核心代码删除集中在 6 个 Go 文件 + 5 个 Vue/TS 文件 + 1 个 OpenAPI 文件 + 1 个 dev-map.md + 2 个 verify_all 脚本。审查方法：

- 全文 grep 17 个禁词（与 verify_all H.1 同款正则）→ 命中 0
- diff 与 02 §2 表逐条对账 → 11 delete + 13 edit + 2 inject（PS + Bash H.1 step）一致
- `go test ./...` / npm test 隔离 T-036 污染后双跑 → PASS

## 2. Findings

### 2.1 Critical（必须修，否则 RAJECT）

无。

### 2.2 Major（应在合并前修）

无。

### 2.3 Minor（可现修可后续，反馈给 PM 决定）

#### M-1（INFO，不阻塞合并）

[Proxies.vue:178-191](../../../web/src/pages/Proxies.vue#L178) 的"远程端口/域名"列 render 函数从 group/single 双分支退化为单分支 inline 三元逻辑，可读性良好；但行尾未对齐 columns 数组中其它列的多行 h() 风格 — 这是**纯风格**问题，无需 dev 返工。如未来批量重构 columns 时统一即可。

#### M-2（INFO，不阻塞合并）

[scripts/verify_all.sh:H.1](../../../scripts/verify_all.sh) 与 PS 版禁词正则字面一致（实证 grep diff 显示二者 token list 一字不差），符合 insight L26 双实现对账原则。**但** PowerShell 用 `git grep -nE $pattern` Native call 接 `$LASTEXITCODE > 1` 判断 fatal vs no-match —— `>1` 边界正确（git grep 文档：0=found, 1=no-match, ≥2=error），Bash 用 `|| true` 兜底容忍 exit 1 也正确。两种风格不同但语义等价。Stage 6 应做 §QA-1 反向证伪验证。

#### M-3（INFO，不阻塞合并）

[ProxyForm.spec.ts:281-283](../../../web/src/components/__tests__/ProxyForm.spec.ts#L281) 改用 `expect(Object.keys(wrapper.emitted())).toHaveLength(0)` 替代既有 `update:batchMode` / `update:portsExpr` 字面断言 —— 这是**更强**的断言（覆盖所有事件，而非 2 个特定事件），与 02 §B-1.3 + 03 §Q4 "完全删除 defineEmits 整段"一致。无需修订。

## 3. 与上游文档对齐审

| 维度 | 期望 | 实际 | 结果 |
|---|---|---|---|
| 01 §B-1 (UI batch 入口删) | Proxies.vue 按钮文案 / 模态框标题 / 模板事件监听 / handleSubmit 批量分支 + ProxyForm.vue batchMode/portsExpr 全删 | Proxies.vue diff -100 / +75；ProxyForm.vue diff -247 / +154，全部命中 | ✓ |
| 01 §B-2 (API/Store/Composable batch 删) | apiBatchCreateProxies / batchCreate / BatchProxies* types / useProxyGrouping{.ts,.spec.ts} | 全部 0 命中（H.1 grep 实证） | ✓ |
| 01 §B-3 (后端 batch 删) | router 2 路由 / handlers_proxies batchProxies + helpers / handlers_batch_test / portrange/ / UpsertProxiesTx / proxies_batch_test / ErrDuplicateTcpRemote | 全部命中 | ✓ |
| 01 §B-4 / §B-5 (端口探测删) | apiProbePorts / probePorts handler / probeOnePort / PortProbe* types / port_probe_test | 全部命中 | ✓ |
| 01 §B-6 (10022→223 bug 修复) | 物理删除 group row 渲染路径 | Proxies.vue 新 render 函数仅单条分支，grep `kind === 'group'` 命中 0 | ✓ |
| 01 §B-7 (文档同步) | dev-map.md / architecture.html 同步删 | dev-map.md 5 处更新；architecture.html grep 无命中（OOS-tech）| ✓ |
| 01 §OOS (不动 schema / NInputNumber / usePortPresets / 既有 verify_all step / G.\* ID 冲突) | 全部不动 | git diff -- migrations/ / NInputNumber 配置 / usePortPresets / verify_all G.\*：均 0 字节变动 | ✓ |
| 02 §11 dispatch order | 5 步顺序：前端 → 后端 → OpenAPI → docs → verify_all H.1 | 04 §1 顺序表与之匹配 | ✓ |
| 03 conditions (C-1 ~ C-4) | informational，建议消化 | 04 §4 全部 4 条已显式消化 | ✓ |

## 4. verify_all 实证

按 04 §3 隔离 T-036 工作树污染后，verify_all.sh --quick 跑出：

```
=== Summary ===
  PASS: 27
  WARN: 0
  FAIL: 0
  SKIP: 0
```

含本任务新 H.1 step。reviewer 已独立复跑确认与 dev 阶段一致。

## 5. 设计漂移评估

**无漂移**。dev 实施按 02 §2 一一对应；03 conditions 全部消化在自然顺手位置；04 §4 显式陈述"无 design drift"，reviewer grep 验证后认同。

## 6. Insight 复发检查

- **insight L26（verify_all 双实现对账）**：H.1 PS + Bash 同款正则 17 token；语义等价（§2.3 M-2）。Stage 6 应做反向证伪验收（按 insight L35）。
- **insight L28（vue v-model OOM）**：ProxyForm.vue 单向数据流契约保留（initialValue prop + getProxyInput()），无回退。
- **insight L29（vitest Naive UI useMessage stub）**：ProxyForm.spec.ts 仍保持 importOriginal + spread + 6 方法 stub 模式（L11-24）。
- **insight L30（git stash 归责工作树污染）**：04 §3 显式使用此模式归责非本任务 FAIL。Stage 6 / 7 必须复用。
- **insight L42（archive-task.sh insight regex 不容错 §N 前缀）**：07 §Insight 段必须用裸 `## Insight` 标题 + bullet 列表（提示给 stage 7 PM）。

## 7. Verdict

**APPROVED** —— 可进入 Stage 6 QA。

- 全部 8 个 01 行为集 + 02 §11 五步实施 100% 命中（§3 表）。
- verify_all 全 27 PASS（隔离 T-036 后），含本任务关键守门 H.1。
- 无 Critical / Major issue；3 条 Minor 均为信息性 / 风格 / 已合规说明。
- 与历史 insight（L26 / L28 / L29 / L30 / L42）无复发风险。

→ Stage 6 (QA Tester) 可启动。提示见 04 §8 第 2 条：必须含 `## Adversarial tests` 段 + e2e AC-5 锚定 + H.1 反向证伪。
