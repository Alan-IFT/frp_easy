# 03_GATE_REVIEW — T-042 proxy-runtime-status-merge

> Stage 3 / Gate Reviewer · 2026-05-28
>
> 输入：01_REQUIREMENT_ANALYSIS.md + 02_SOLUTION_DESIGN.md
> 输出：本文档（verdict 在文末）

## 1. 一致性核验（01 ↔ 02）

| 01 项 | 02 实现 | 一致 |
|---|---|---|
| AC-1 ~ AC-3 runtime 列三态 + tooltip | § 3.3 columns 追加 + NTooltip 嵌套 | ✓ |
| AC-4 / AC-5 反向构造 | § 3.5 Proxies.spec.ts + adversarial.spec.ts | ✓ |
| AC-6 降级 + CRUD 不挂 | § 3.3 `runtimeUnavailable` computed + CRUD 路径未触碰 | ✓ |
| AC-7 / AC-8 utils 边界 | § 3.5 format.spec.ts / proxyStatus.spec.ts | ✓ |
| AC-9 ServerMonitor 既有 spec 不破坏 | § 3.4 仅 import 替换；defineExpose 指针不变 | ✓ |
| AC-10 SFC 行数 | § 7 A-2 预估 ~138 行 < 200 | ✓（待 dev 实测） |
| AC-11 单向数据流 | § 4 数据流图无反向 + § 3.5 ADV-5 grep 守门 | ✓ |
| AC-12 / AC-13 verify_all + 标题硬约束 | 由 QA stage 6 守护 | ✓ |

## 2. 决策原则映射核验

| 原则 | 体现 | 评分 |
|---|---|---|
| 用户体验好 | 单视图聚合 + 降级 CRUD 不挂 | A |
| 软件工程标准 | DRY 抽 utils + 边界全覆盖 + 反向构造 | A |
| 长期易维护 | 单向数据流 + utils 复用 + 既有列零改 | A |

## 3. 假设审查（02 § 7）

| 假设 | 02 论证强度 | GR 接受 |
|---|---|---|
| A-1 双 mount 并存 polling 不互扰 | 静态分析 closure 局部变量（已论证） | ✓ |
| A-2 SFC < 200 行 | 估算 ~138 行 | ✓（要求 dev 实测确认并在 04 § 中记录） |
| A-3 无既有 Proxies.spec.ts | Glob 实测 | ✓ |
| A-4 utils 抽不破坏 ServerMonitor spec | 函数引用同源 + 行为字节一致 | ✓ |

## 4. 风险评估

| 风险 | 缓解强度 | GR 接受 |
|---|---|---|
| Proxies.vue 行数膨胀 | utils 抽减负 + 预估 | ✓ |
| ServerMonitor 行为漂移 | 字节级搬运 + 既有 spec 不改 | ✓ |
| polling 双 instance 资源 | 5s × 2 通道 = 接受范围 | ✓ |

## 5. Conditions（dev 应主动消化，非阻塞）

- **C-1**：dev 在 04 § 中**实测**记录 Proxies.vue script 段纯逻辑行数（去 import / 注释 / interface），证实 < 200。若 ≥ 200，必须当即拆分（候选：runtime 相关 setup 抽 `useProxyRuntimeMerge.ts` composable）。
- **C-2**：dev 必须在 utils/format.ts 顶端注释明示 "T-041 字节级搬运 + T-042 新增负数防御"，让维护期一眼追溯来源。
- **C-3**：dev 必须在 utils/proxyStatus.ts 顶端注释明示 "空字符串归 offline 的语义合并决策"（02 § 3.2 决策点）。
- **C-4**：dev 在改 ServerMonitor.vue 时只允许 (a) 删除 inline 函数定义 (b) 改 columns 状态 render (c) 添加 import。**禁** 改：template / NCard 结构 / activeType / firstLoading 等其它 setup ref。
- **C-5**：QA 06 `## Adversarial tests` **必须**裸标题（insight L41）。PM 在写 06 时硬约束。
- **C-6**：QA 反向构造至少覆盖 ADV-1 ~ ADV-5（02 § 3.5）+ AC-4 ~ AC-6。

## 6. 闸门判定

- 需求清晰度：A
- 设计完备度：A
- 风险覆盖：A
- 与既有范式一致性：A（继承 T-032 / T-037 / T-041）

## Verdict

**APPROVED FOR DEVELOPMENT**

Conditions C-1 ~ C-6 应被 developer / qa 在 04 / 06 中主动消化，不阻塞 stage 4 启动。

— end —
