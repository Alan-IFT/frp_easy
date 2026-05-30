# 02 方案设计 — T-054 archive-task-sh-regex-align

> Stage 2 / Solution Architect · mode: full

## 1. Architecture summary

单文件、单行级修复：把 `scripts/archive-task.sh` 中 awk 用于识别 `07_DELIVERY.md` 内 `## Insight` 段起始行的标题正则，从不容错前缀的 `/^##[[:space:]]+Insights?[[:space:]]*$/` 改为容错单 token 前缀的 `/^##[[:space:]]+([^[:space:]]+[[:space:]]+)?Insights?[[:space:]]*$/`，使其与 `archive-task.ps1` 的 .NET 正则 `^##\s+(?:[^\s\n]+\s+)?Insights?\s*$` 语义对齐。无数据模型、无 API、无新模块。

## 2. Affected modules

| 文件 | 角色 | 改动类型 |
|---|---|---|
| `scripts/archive-task.sh` | bash 收割脚本（PowerShell 版镜像） | edit：1 行正则 + 上方 1 行注释 |

无其它文件受影响。`scripts/archive-task.ps1` 为 read-only 对齐基准。

## 3. Module decomposition

无新模块。

## 4. Data model changes

无。

## 5. API contracts

无对外 API。脚本 CLI 契约（`--task <slug>` / `--dry-run`）不变。

## 6. Sequence / flow（改动点在收割流程中的位置）

```
archive-task.sh --task <slug>
  → 定位 docs/features/<slug>/07_DELIVERY.md
  → [改动点] awk 扫描：遇到匹配 Insight 标题正则的行 → flag=1
                       遇到下一个 ## 标题行       → flag=0
                       flag 且行以 '- ' 开头      → 收割该 bullet
  → 把收割到的 bullet append 到 .harness/insight-index.md（或 dry-run 仅打印）
  → mv task dir 到 _archived/
```

改动只影响"遇到匹配 Insight 标题的行 → flag=1"这一判定的**命中集合**：扩大为允许 0 或 1 个非空白前缀 token。其余流程（flag=0 终止、bullet 子正则、append、move）字节不变。

### 6.1 POSIX ERE 等价性论证（对账核心）

目标 awk 模式：`/^##[[:space:]]+([^[:space:]]+[[:space:]]+)?Insights?[[:space:]]*$/`

| 片段 | awk POSIX ERE | .ps1 .NET regex | 语义 |
|---|---|---|---|
| 行首 H2 | `^##[[:space:]]+` | `^##\s+` | `##` 后 ≥1 空白 |
| 可选前缀 | `([^[:space:]]+[[:space:]]+)?` | `(?:[^\s\n]+\s+)?` | 0 或 1 个「非空白 token + ≥1 空白」 |
| 关键词 | `Insights?` | `Insights?` | Insight 或 Insights |
| 行尾 | `[[:space:]]*$` | `\s*$` | 尾随任意空白后行尾 |

差异点与判定：
- awk 用 `[^[:space:]]` 表非空白、.NET 用 `[^\s\n]`。awk 是逐行处理（`$0` 天然不含换行），`[[:space:]]` 已含换行类但在单行 record 内无换行，故 `[^[:space:]]` 与 `[^\s\n]` 在单行匹配语境下等价。
- awk 分组捕获 `(...)` 在此仅作量词 `?` 的作用域，不取捕获值——与 .NET 非捕获组 `(?:...)` 行为等价（awk 不暴露捕获组，无副作用）。
- 结论：两实现容忍集合相同——裸标题 + 单 token 前缀标题命中，双 token 前缀及非 Insight 标题不命中。**对账通过**。

### 6.2 为何不用更宽的"任意前缀"正则

考虑过 `([^[:space:]].*[[:space:]])?Insights?` 类贪婪写法以容忍多 token 前缀，但**故意不采用**：
- `.ps1:48` 基准只容忍单 token；对齐债的定义就是"做到与 .ps1 同款"，过度放宽会制造新的不对称（反向）。
- 单 token 已覆盖历史全部踩坑形态（`## §N Insight` / `## N. Insight`），无证据需要更宽。
- 贪婪 `.*` 可能引入 `## Foo Insight Bar` 类误判风险，违背 BC-6 防假阳性。

## 7. Reuse audit

| Need | Existing code | File path | Decision |
|---|---|---|---|
| 前缀容错正则的权威写法 | `.ps1:48` `^##\s+(?:[^\s\n]+\s+)?Insights?\s*$` | `scripts/archive-task.ps1` | 1:1 翻译为 awk POSIX ERE，不发明新写法 |
| 收割 bullet 子正则 | `/^[[:space:]]*-[[:space:]]/` | `scripts/archive-task.sh:50`（同行 awk 后半） | 原样保留，不动 |
| flag 终止逻辑 | `/^##[[:space:]]/{flag=0}` | `scripts/archive-task.sh:50` | 原样保留，不动 |

复用审计非空：本任务核心就是"复用 .ps1 已验证的容错正则语义"，零发明。

## 8. Risk analysis（≥3）

- **R1：awk 方言差异让分组量词不被支持。** 缓解：`(group)?` 是 POSIX ERE 标准（非 GERE 扩展），gawk / mawk / BSD awk 均支持；06 的反向证伪用本机实际 awk 跑，证据落盘。
- **R2：改动误伤同行的 flag 终止 / bullet 子正则，引入回归。** 缓解：用 Edit 精确替换**仅标题正则子串**，保留 awk 行其余部分；06 用 AC-4（裸标题不回归）+ AC-5（非 Insight 不误命中）双侧守门。
- **R3：在真实仓库跑验证时污染 `.harness/insight-index.md`。** 缓解：BC-8 强制用临时 fixture 目录 + 直接对 fixture 文件跑独立 `awk`（不调 `archive-task.sh` 主流程的 move/append），或 `--dry-run`；命令与输出落进 06，绝不 append 真实 index。
- **R4：行号漂移让"只改 50 行"描述失真。** 缓解：以 Edit 的 old_string 唯一匹配为准（按内容定位非按行号），04 记录实际行号；改动面以 git diff stat 验证 = 1 文件、净 +1 注释行 + 1 行内容替换。

## 9. Migration / rollout plan

- 无数据迁移。纯脚本逻辑放宽。
- 向后兼容：新模式是旧模式的**超集**（旧命中集合 ⊂ 新命中集合），任何历史上能 harvest 的 07 仍能 harvest，不破坏既有行为。
- 回滚：单行 revert 即可（git）。

## 10. Out-of-scope clarifications

- 本设计不覆盖 N=0 显式 warning 对齐（`.sh` 无、`.ps1` 有）——见 01 OOS-4。
- 不覆盖把该 awk 抽成共享函数 / 跨脚本去重——非本债范围。
- 不覆盖 `verify_all` 新增静态闸门守门"双实现正则对齐"——可作未来独立任务，本次仅偿还内容债。

## 11. Partition assignment

项目存在 `dev-db` / `dev-backend` / `dev-frontend` 分区 agent，故本段为 REQUIRED。

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `scripts/archive-task.sh` | （无分区匹配 → 单 Developer / PM-role） | edit | — |

### Dispatch order

1. 单一改动，无分区依赖。

### Parallelism

None（单文件单行改动）。

**分区说明**：`scripts/archive-task.sh` 是 Harness 工具链 shell 脚本，既非 DB（migrations / SQLite）、非 backend（Go）、亦非 frontend（Vue/TS）——三个 `dev-*` 分区均不拥有 harness 工具脚本域。按 `.harness/rules/50-fullstack.md` 分区边界，此类改动落到**通用 Developer**（在本次 tool-clipped 派发上下文中由 PM 角色化承担，与历史 trivial 脚本任务 T-028 / T-044 同款路径）。架构上明确：**不**误派给任何 `dev-*` 分区。

## 12. Verdict

`READY`
