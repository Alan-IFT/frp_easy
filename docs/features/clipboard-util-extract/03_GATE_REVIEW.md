# 03 闸门评审 — T-061 clipboard-util-extract

> 阶段 3 / Gate Reviewer · mode: full · 中文产出
> 独立核验：是否可以开始编码？

## 上游核验

- `01_REQUIREMENT_ANALYSIS.md` Verdict = **READY** ✓
- `02_SOLUTION_DESIGN.md` Verdict = **READY** ✓，含 §11 Partition assignment（dev-frontend 单分区）✓

## 引用代码独立核验（实读，非信任设计）

| 设计声明 | 核验结果 |
|---|---|
| `LogViewer.vue::onCopy` 在 L145-171，try clipboard → success；catch → textarea+execCommand | ✓ 实读确认（L145-171）。build text = `search.visibleLines.value.map((v) => v.parsed.raw).join('\n')`（L146）；message 用 L96 `useMessage()` |
| `FirewallHint.vue::copyText(text): Promise<boolean>` 同款 fallback，`copyCmd`/`copyAll` 依赖返回 ok | ✓ 实读确认（L81-128）。`copyCmd`/`copyAll` `if (await copyText(...))` 置短暂态 |
| `PublicIpDetector.vue::copyText(text): Promise<boolean>` 同款，`copyIp` 依赖 ok | ✓ 实读确认（L67-106） |
| 三处文案：成功 `'已复制到剪贴板'`、失败 `'复制失败：请手动选择文本复制'`、textarea fallback 逐字相同 | ✓ 三处比对逐字一致（aria-hidden + position:fixed + left:-9999px + select + execCommand('copy') + finally removeChild） |
| 既有 utils 范式 `format.ts`/`proxyStatus.ts` + `__tests__/` | ✓ 实读 `format.ts`（纯函数导出 + 顶部注释列共享方）；`utils/__tests__/format.spec.ts`、`proxyStatus.spec.ts` 存在 |
| dev-map「可复用工具」表（L157-179）格式 | ✓ 实读确认，format.ts/proxyStatus.ts 在 L173-174，4 列「需求/已有/文件/备注」 |
| **R-1 关键**：LogViewer.spec AC-6 只断言 `navigator.clipboard.writeText`，不断言 message / fallback DOM | ✓ 实读 `LogViewer.spec.ts:195-220`：AC-6 `Object.defineProperty(navigator,'clipboard',{value:{writeText:writeSpy}})` → `await t.onCopy()` → 断言 `writeSpy` 调用 + 拼接字符串内容；**未**断言 message、**未**断言 fallback textarea。抽取后 util 内部仍调同一被 mock 的 `navigator.clipboard.writeText` → 零回归。**R-1 顾虑（T-058 D1 当初规避抽取的核心理由）经核验不成立** |
| FirewallHint.spec / PublicIpDetector.spec mock 机制 | ✓ 实读：均 `Object.defineProperty(navigator,'clipboard',{value:{writeText:writeTextMock},configurable:true,writable:true})` + 显式装 `document.execCommand` + `vi.mock('naive-ui')` 单例 `__messageSpies`。抽取后 util 走同一 mock，组件层 message 断言不变 → 零回归 |

## 8 维审计

| # | 维度 | 结论 | 说明 |
|---|---|---|---|
| 1 | 需求完整性 | **PASS** | 8 条 in-scope 行为 + 6 条 BC 全可测；文案/文件集明确，无歧义词；OQ 为空 |
| 2 | 设计完整性 | **PASS** | §3 给出 util 公共 API + 伪代码；§6 给出三处组件逐字目标改造；覆盖全部 in-scope 行为 |
| 3 | 复用正确性 | **PASS** | 复用审计非空且经实读核验：三处内联 → 单 util；复刻 format.ts 范式（目录 + __tests__ + dev-map 登记）；无新依赖 |
| 4 | 风险覆盖 | **PASS** | R-1（LogViewer 回归，最关键）已被设计指认且我已独立核验其 spec 不断言被抽走的部分；R-2~R-6 覆盖文案漂移/误用 useMessage/测试脆弱/baseline/e2e，均带缓解 |
| 5 | 迁移安全 | **PASS** | 无 schema/API 变更；纯前端可观察行为不变；回滚 = git revert |
| 6 | 边界处理 | **PASS** | BC-1 空串 / BC-2 reject / BC-3 execCommand false / BC-4 抛错 / BC-5 textarea 清理 / BC-6 并发无共享状态，设计 §3 伪代码全覆盖 |
| 7 | 测试可行性 | **PASS** | AC-2 四条 util 用例 + textarea 残留断言均可由 `Object.defineProperty` + execCommand mock 验证（util 无 UI，断言纯布尔 + DOM 查询，不依赖 naive-ui 组件名，L45 风险天然不适用） |
| 8 | 范围外清晰度 | **PASS** | OOS-1~6 明确（不改可观察行为、不加新防御、不重构 text 拼接、不碰后端）；§10 设计边界呼应；over-build 风险低 |

## 开发期高概率问题（预答）

1. **Q: util 要不要加 `navigator.clipboard` 存在性预检 / SSR `typeof document` 守卫？**
   A: **不要**（OOS-5）。现状三处均直接 try `navigator.clipboard.writeText`，util 1:1 沿用即可；try/catch 已兜住 `clipboard` 为 undefined 的 TypeError（会走 fallback）。新防御属另一任务（L42）。

2. **Q: clipboard.spec 里 `document.execCommand` 在 happy-dom/jsdom 默认不存在，怎么测 fallback？**
   A: 显式装上 mock：`(document as unknown as { execCommand: typeof mock }).execCommand = mock`（与 FirewallHint.spec L84 / PublicIpDetector.spec L82 同款，insight L37）。`afterEach` 清 `document.body.innerHTML`，`beforeEach` reset mock。

3. **Q: FirewallHint/PublicIpDetector 的 `copyText` 用三元会不会改变 message 调用次数？**
   A: 不会。`message[ok?'success':'error'](ok?'...':'...')` 等价单次调用，与现状 if/else 各分支单次调用语义一致。CR 逐字比对。

4. **Q: baseline 该 bump 多少？**
   A: 净增 = `clipboard.spec.ts` 新增用例数。dev-frontend 落地后据实际用例数 bump `frontend_tests` + `test_count`（同增同量），不动 `go_tests`。CR 核对增量算术，orchestrator 真跑 B.4 闸门兜底。

5. **Q: 三组件既有 spec 要不要改？**
   A: 原则上**不需要**（行为不变，mock 机制命中 util 内部同一 API）。若 dev-frontend 发现某 spec 因实现细节（如 import 路径）需微调，必须保证断言语义不变 + 不删活测试；CR/QA 复审。LogViewer.spec 尤其零改动为佳。

## 裁决（Verdict）

**APPROVED** —— 8 维全 PASS，引用代码全部实读核验通过，R-1（抽取致 LogViewer 回归）这一历史顾虑经独立核验确认不成立（其 spec 不断言被抽走的部分）。开发可进行（Stage 4，dev-frontend）。

无条件附加。建议 dev-frontend 严格遵循 §6 三处逐字目标，并优先保证 LogViewer.spec 零改动。
