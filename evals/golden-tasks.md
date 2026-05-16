# Golden Tasks — 轻量回归任务集

> 一小组**已知能正确流过 7-agent 流水线的代表性任务**。
> **改动 CLAUDE.md、agents、workflow、skills 之后手动重跑这些**。
>
> 个人项目规模：2-5 个就够了。**不要**建 CI eval pipeline，除非到了团队规模。

## 怎么用

改完 Harness 资产后，在 Claude Code 里：

```
Re-run golden task #1 and confirm the pipeline produces the same delivery shape.
```

然后人工扫一眼：

- 每个 stage 都产出文档了吗？
- 回退路由在预期情况下生效了吗？
- 最后 `verify_all` PASS 了吗？
- 任务用了大致相同的 stage 数量吗？

如果有"否" → 诊断是哪个 Harness 资产改动引入了回归。

## 任务

### Golden #1 — 琐碎新增

**描述**：往 `src/config.ts` 加一个常量 `MAX_RETRIES = 3`。

**预期形态**：
- 轻量流程（PM 可能跳过 Gate）。
- Developer 改完，verify_all 通过，reviewer 标"琐碎无忧"，QA 标"行为无变化"，交付。

### Golden #2 — Bug 修复 + 回归测试

**描述**：给某个已有函数加单测，复现（必要时修复）空数组输入的处理。

**预期形态**：
- 完整流水线。
- Requirement Analyst 标出边界条件。
- Developer 加一个在当前代码上失败的测试，然后修复。
- QA 确认测试数增加。

### Golden #3 — （边做项目边补）

（空）

### Golden #4 — （边做项目边补）

（空）

---

每次改动 Harness 资产后，在这里记录：

| 日期 | 改了什么 | 重跑哪些 Golden | 结果 |
|---|---|---|---|
| 2026-05-16 | 初始化 | — | — |
