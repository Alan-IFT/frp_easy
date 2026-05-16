---
applyTo: "**"
---
# frp_easy — bootstrap 规则

输出语言：**中文**。

完整项目规则在 `AI-GUIDE.md`（根目录）+ `.harness/rules/*.md`。**开始任何任务前先读 `AI-GUIDE.md`，根据它每条的"什么时候读"描述选择性加载相关片段，不要全读**。

红线（永不违反）：
- 禁编辑生成/静态文件：`.claude/`、`CLAUDE.md`、本文件
- 声明完成前必须跑 `scripts/verify_all` PASS
- 一次一个角色：读对应 `.harness/agents/<name>.md` 按契约工作，不要中途悄悄换角色

本文件是**静态**的 —— init 时写一次，刻意精简，永不膨胀持久 context 预算。其余全在 `AI-GUIDE.md` 或 `.harness/`。
