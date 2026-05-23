# 03 Gate Review — T-027 download-cancel-and-upload-decouple

> 本文件由 PM 代为落盘（gate-reviewer sub-agent 工具集无 Write/Edit，复现 insight L41/L44 长期未落地建议；本批 reviewer 强烈建议在 `.harness/agents/gate-reviewer.md` frontmatter 加 `Write` 工具）。

## Verdict

**APPROVED FOR DEVELOPMENT**

设计完备、对实际源码核实匹配（Manager 结构 / state map / mu / setFailed / Install / uploadBin 锁逻辑 / openapi 现状全部 verbatim 一致），关键风险（R-1 stdlib ctx 解阻塞、R-2 cancel-vs-success race、R-3 轮询回弹、R-4 tmp 残留）均有论证 + 测试覆盖，对 T-025 决策（Client.Timeout=0）与三层契约（Go enum / OpenAPI / TS union）一致性明示落地。存在 4 项 SHOULD-FIX + 6 项 NIT 需 Developer 在 04 明确回应，但无 BLOCKER；APPROVE 偏好成立。

## 发现清单

| ID | 严重度 | 主题 | 描述 | 修复建议 |
|---|---|---|---|---|
| F-1 | SHOULD-FIX | dialog→tooltip 对 FR-9 的偏离 | §4.5 把 01 FR-9 的"上传 confirm dialog（三按钮：取消并上传/仅上传/放弃）"收紧为"tooltip + disabled"。01 U-4 用户故事核心诉求是"一步取消"。Architect 给出的理由（mainstream 风格 / 高级用户嫌烦）合理但没有覆盖低频用户：他们看见 disabled 上传按钮可能直接放弃。Architect 自己在 §4.5 末尾标"若 PM 要求严格按 01，回退到 dialog ~60 行 Vue" —— 让 Developer 在 04 显式选择并记录 DESIGN DRIFT。 | 04 显式给出最终选择并按红线 §1 标 `DESIGN DRIFT`。tooltip 方案保留亦可，但 04 必须把"不引入 dialog"作为显式决策点写下来。 |
| F-2 | SHOULD-FIX | FR-4 中 400 vs codebase 一致 422 偏离需 04 同步修 01 缺口 | OQ-4 把 "kind 非法" 从 01 FR-4 的 400 改为 422（与 uploadBin/downloadBin/probePorts 一致）。01 §5.2 AC-http-cancel-bad-kind-400 卡 400；Developer 实施时必须把 AC 断言改 422，但 01 的 AC 是 reviewer/QA 闸门契约，不能由 Developer 静默改。 | 04 在"DESIGN DRIFT"段写明该 follow-on 变更，06 测试报告显式说明。 |
| F-3 | SHOULD-FIX | Cancel 等待 3s 超时的兜底失态 | §2.3 Cancel 在 3s 轮询超时仅记日志返回 nil，但 FR-7 / §1.2 不变量 5 要求"Cancel 同步返回时 state 必须已是 canceled"。极端场景下 HTTP 200 返回但 state 仍 downloading，调用方 upload-bin 立即追打仍可能 409，破坏"零等待时间窗"。**不允许只记日志返 nil**。 | 04 实施 Cancel 时在 3s 超时分支增加防御：(a) 选项 A：拿锁强写 canceled（破坏单调 guard 但保 FR-7 不变量）；(b) 选项 B：返回新 sentinel `ErrCancelTimeout` 让 HTTP 层返 504 让前端重试。任一选项需在 04 落地。 |
| F-4 | SHOULD-FIX | resolveLatestAsset 阶段 cancel 无法响应（NFR-1 边界场景） | OQ-1 决策 resolveLatestAsset 不 ctx 化。但用户在 API 阶段卡 60s 的边界场景下点取消，Cancel 调用会卡 60s 等 apiClient 超时，违反 NFR-1（≤3s）。Architect 明示"5 行代码就能改回" → 应该改回。 | 04 给 resolveLatestAsset 加 ctx 参数，`http.NewRequest` 改 `http.NewRequestWithContext(ctx, ...)`。doDownload 调用方传入。加单测 AC-cancel-during-resolve-asset。 |
| F-5 | NIT | binTmp 兜底 defer 与既有显式 Remove 共存的语义说明 | §2.5 提议在 `binTmpPath` 创建后立刻加 `defer os.Remove(binTmpPath)`，line 257 / 273 既有的主动 Remove 保留。这没问题（ENOENT 可忽略），但 04 实施时应在注释里说一句"双层清理：defer 兜底 cancel 路径 + 主动 Remove 加速正常路径释放"。 | 04 实施时加 3 行注释说明。 |
| F-6 | NIT | setFailed 缺单调 guard 的潜在 race 未关闭 | OQ-3 / R-6 显示 setFailed 没有 `if Status == downloading` guard。本任务新增 err 分支内先判 ctx.Err() 走 setCanceled，让 setFailed 的调用点变多。若某 err 分支漏掉 ctx 重检，setFailed 会盖 canceled。5 行代码 + 防御纵深。 | 04 NIT 加 guard 或显式拒绝并解释理由。 |
| F-7 | NIT | `idleState()` 工厂函数侧不显式列举 canceled | `web/src/stores/downloader.ts:11` 的 `idleState()` 工厂返回 `{status:'idle', progress:0}` 是窄类型。建议 04 在 store 改动时把 `idleState(): DownloadState` 显式标注让 union 类型流入。 | 04 显式标注返回类型。 |
| F-8 | NIT | E2E mock GitHub 在 T-006 框架是否可用未核验 | §5.5 假定 T-006 框架可注入 mock GitHub。Architect 没引证 T-006 是否提供此 hook。 | 04 实施前快速 grep T-006 e2e 是否有 mock seam；没有则降级为 Vitest 集成测试，并在 06 标该 AC 由集成测试满足。 |
| F-9 | NIT | UploadBinButton.vue 现有 `:disabled="uploading"` 与新增 `siblingDownloading` 合并 | §4.5 给出 `:disabled="uploading || siblingDownloading"`，但当前 line 9 是 `:disabled="uploading"`。04 实施时 verbatim 替换，不引入 trailing 空白漂移。 | 04 实施时 verbatim 替换。 |
| F-10 | NIT | L42 双路径注释未落到具体行 | §10 提醒 L42 应用到 Manager 字段 `cancels` 与 Cancel 方法 doc，但样板代码块没带 02 路径引用。 | 04 实施时按 §10 在 `cancels` 字段与 `Cancel` 方法 doc 各加双路径注释行。 |

## 评审记录（A-F 各章节）

### A. 完备性

- **A-1 ✅** 10 条 FR 全部对应可机器验证的 AC（单测 / HTTP / Vitest / E2E 五层），FR-7 由 §2.3 阻塞轮询 + AC-http-cancel-then-upload-200 一锤定音。
- **A-2 ⚠️** NFR-1（≤3s）有 §2.3 3s 上限实现但 F-3 / F-4 指出超时兜底失态 + resolveLatestAsset 阶段缺保障。其它 NFR 均有测试度量。
- **A-3 ✅** 状态机 5 态 + 转换图（§1.3）覆盖。
- **A-4 ✅** §3.4.3 三层共契字段名表格清晰，verbatim 防 L29 / L40 漂移。

### B. 可行性

- **B-1 ✅** §7 R-1 论证 stdlib `http.Transport` 在 ctx Done 时主动关 conn 是 Go 1.7+ 文档化契约。
- **B-2 ⚠️ → F-3** Cancel 3s 轮询超时仅记日志返 nil，破坏不变量 5。
- **B-3 ✅** §2.5 / §7 R-4 论证 defer 在 cancel→setCanceled→return 路径正常执行。
- **B-4 ✅** §2.2 末尾"ctx 重检 + 状态机单调 guard"两层防御充分。

### C. 风险

- **C-1 ✅** RA FR-1 与 Architect §2.3 一致。
- **C-2 ⚠️ → F-1** dialog→tooltip 是偏离，建议 04 显式 DESIGN DRIFT。
- **C-3 ✅** §3.4.3 三层共契字段引用 verbatim 准确（grep 核对一致）。
- **C-4 ✅** L42 双路径要求在 §10 明示。F-10 NIT 提醒落到具体行。

### D. 测试矩阵

- **D-1 ✅** §5.1-§5.5 逐条对账 22+ AC。
- **D-2 ✅** §5.1 表格首行 verbatim 引用 L40（math/rand 防 gzip 压塌）。
- **D-3 ✅** AC-cancel-success-noop / AC-cancel-after-failed-then-restart 显式覆盖。
- **D-4 ✅** §5.1 AC-cancel-mid-download "3s 内 assert Status==canceled" 是显式时间断言。

### E. 红线 / Insight 兼容

- **E-1 ✅** §1.2 不变量 2 / NFR-6 / OQ-1 多处明示 `downloadClient.Timeout=0` 不动。
- **E-2 ✅** §3.4.3 三层共契 + §11.8 PR 描述要求三层一致性自检 grep。
- **E-3 ✅** §10 列出 L42 双路径要求。
- **E-4 ✅** §10 显式提 L43 / L21 是 PM / QA 关切。
- **E-5 ✅** 无"先放上去再迭代"的偷工。

### F. 跨平台 / Service 模式

- **F-1 ✅** stdlib ctx cancel 是 OS-agnostic。
- **F-2 ✅** Cancel / setCanceled 路径全走 slog logger，service 模式安全。

## Approved-as-is 部分

- §2.1 `cancels map[string]context.CancelFunc` 字段由 `m.mu` 守护的注销模式。
- §2.3 Cancel 方法主体（除 F-3 提到的 3s 兜底外）。
- §2.4 `setCanceled` Info 级日志 + Status guard 的实现样板。
- §3.1 downloadCancel handler 体（除 F-2 留痕外）。
- §3.4 OpenAPI enum 扩展 + 新增 path block 全文。
- §4.1 / §4.2 / §4.3 前端 types / api / store 三处改动（F-7 NIT 不阻塞）。
- §4.4 AppLayout 5 状态按钮表 + canceled 状态用 warning + 文案 "已取消，点击重试"。
- §6 reuse audit 全表。
- §8 migration / rollback plan。

## 给 PM / Developer 的建议

1. **派发 Developer 时把 F-1 / F-2 / F-3 / F-4 四条 SHOULD-FIX 写到任务指令里**，要求在 04 显式回应（标 DESIGN DRIFT 或给出实施补丁）。
2. **F-3 建议倾向"选项 A：拿锁强写 canceled"路径**，FR-7 不变量优先级 > 状态机单调性的防御性 guard。
3. **F-4 改动量小（5 行），建议 04 直接做掉**。
4. **F-1 让 PM 拍板**：本评审不强求回退 dialog，但 04 必须显式 DESIGN DRIFT 并保留 follow-up 入口。
5. **L41 / L44 reviewer 落盘陷阱再复现**，**强烈建议**在 `.harness/agents/gate-reviewer.md` frontmatter 加 `Write` 工具。
