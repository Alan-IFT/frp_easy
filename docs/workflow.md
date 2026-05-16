# 工作流：7-Agent 流水线

本仓库的标准开发工作流。每一个非琐碎改动都流经下面 7 个阶段。

## 阶段

```
1. requirement-analyst   →  01_REQUIREMENT_ANALYSIS.md
2. solution-architect    →  02_SOLUTION_DESIGN.md
3. gate-reviewer         →  03_GATE_REVIEW.md
4. developer             →  04_DEVELOPMENT.md  (+ 代码改动)
5. code-reviewer         →  05_CODE_REVIEW.md
6. qa-tester             →  06_TEST_REPORT.md  (+ 测试代码)
7. pm-orchestrator       →  07_DELIVERY.md
```

每个任务的文档都放在 `docs/features/<task-slug>/` 下。

## 各角色职责（一句话）

| Agent | 一句话职责 |
|---|---|
| **PM Orchestrator** | 把任务在流水线里路由。**绝不**给专业意见。 |
| **Requirement Analyst** | 把模糊请求 → 结构化、可测试的需求。把歧义列给用户。 |
| **Solution Architect** | 把需求 → 基于实际代码的技术方案（**强制要求**复用审计）。 |
| **Gate Reviewer** | 开发前最后一道闸门。8 维度审计。**实际打开文件**验证设计引用的代码存在。 |
| **Developer** | 唯一写生产代码的 agent。声明完成前必须跑 verify_all。 |
| **Code Reviewer** | 对照需求 + 设计审查代码，不只是看 style。6 维度，分严重度。 |
| **QA Tester** | 验证用户可观察的行为。负责长期维护自动化测试套件。 |

## 回退路由

当某阶段发现上游缺陷，**发现者不能自己修**。PM 路由回去：

| 谁发现 | 缺陷在 | 路由回 |
|---|---|---|
| Gate Reviewer | 需求 | requirement-analyst |
| Gate Reviewer | 方案 | solution-architect |
| Code Reviewer | 代码 | developer |
| Code Reviewer | 方案漂移 | solution-architect |
| QA Tester | 代码 bug | developer |
| QA Tester | 漏测或漏需求 | requirement-analyst |

**同一阶段连续回退 3 次** → PM 停下来问用户。

## 任务怎么开始

用户对 Claude Code 描述任务：

```
Take this task: 在订单页加 CSV 导出功能。
```

PM Orchestrator：
1. 创建 `docs/features/orders-csv-export/`。
2. 写 `INPUT.md` 保留用户的原话。
3. 读 `docs/tasks.md` 找相关历史任务。
4. 通过 Task tool 派发 Stage 1。

每个阶段产出一份文档。PM 读完，决定（前进 / 回退 / 停止），把决定写到 `PM_LOG.md`，派发下一阶段。

## 阶段闸门（不能跳过的检查）

- **进入 Stage 4（开发）前**：闸门审查必须 `APPROVED`（或 `APPROVED WITH CONDITIONS` 且条件已记录）。
- **进入 Stage 5（评审）前**：开发文档必须显示 `verify_all PASS`。
- **进入 Stage 7（交付）前**：代码评审和测试报告都必须 `APPROVED`。

## 轻量变种

不是每个任务都需要完整流水线。

| 任务类型 | 推荐流程 |
|---|---|
| 重要功能 / 跨模块变更 | 完整 7 阶段 |
| 中等功能、单模块 | 如果需求 + 方案都很明确，可跳过 Gate (3) |
| Bug 修复 | 根因分析 → developer → reviewer → tester |
| 琐碎（typo、≤10 行） | 直接改 + verify_all |

**PM Orchestrator 根据用户描述的范围决定**。拿不准时，倾向完整流程。

## 流水线什么时候停

PM 停下来问用户的情况：

- 同一阶段回退 3 次。
- 需求互相冲突，analyst 无法调和。
- 缺外部能力（如需要某个 MCP 服务器）。
- 涉及生产环境的破坏性操作。

## 演化原则

如果 AI 反复犯同样的错，修复**不是**"再试一次"。修复是以下之一：

| 错误类型 | 加 / 改 |
|---|---|
| 违反编码规范 | CLAUDE.md 加一行 + verify_all 加一项检查 |
| 漏了某个步骤 | 加一个 `.claude/skills/<name>/SKILL.md` |
| 角色越界或不到位 | 编辑 agent 定义 |
| 缺外部能力 | 注册一个 MCP 服务器 |
| 整个流水线阶段缺失 | 加新 agent + 修改 workflow |

变更记录到 `CHANGELOG.md`。
