# T-009 · polish-pass · 用户原始输入

> 收到日期：2026-05-19 · 触发模式：`/harness`（全 7 阶段流水线）

## 用户原话

> 解决所有遗留问题，检查是否有影响用户体验的，若有则你来决策处理；以用户体验好，符合软件工程标准，长期易使用易维护为原则来决策；你来决策就可以了，我只看结果是否符合需求；所以需要你根据我要求的原则来决策，然后执行，最后返回给我改动详情，当前情况等我需要关注的信息；所有 commit 都由你来操作

## PM 解读

用户授权 PM 自主决策：
- **范围**：扫描整个项目，识别一切影响 UX / 工程标准 / 长期维护的遗留点；
- **决策准则**：UX 优先 + 软件工程标准 + 长期易使用易维护；
- **交付物**：改动详情 + 当前状态 + 需关注信息（口语化总结）。
- **commit 全部由 PM 执行**。

不是开放式探索（不要用 `/harness-explore`）；目标是把现状从 "deploy-kit 完成时 18 PASS" 拉回到稳定 18 PASS，同时清理已经出现裂纹的过程性遗留（未归档任务文档、混杂语言注释、跨 shell verify_all 不一致）。

## 处理优先级（按 PM 初步评估）

| 优先级 | 项 | 影响 | 决策 |
|---|---|---|---|
| P1 | PowerShell 下 verify_all 1 FAIL（Playwright C.1）| 用户在 Windows PowerShell 跑 verify_all 永远不绿，违反"声明完成前必须 PASS"红线 | 修：playwright.config.ts 跨平台 |
| P2 | T-001 web-ui-mvp 文档未归档 | 违反"DELIVERED 即归档"约定，tasks.md 与磁盘不一致 | 修：跑 archive-task |
| P3 | dev-map.md 含日文注释（早期遗留） | CLAUDE.md 规定中文，多语言混杂影响维护 | 修：翻译为中文 |
| P4 | deploy-kit 中"release-smoke 验证项"（5 AC 未在主机实跑）| 仅发布前关注，非日常开发瓶颈 | 不处理，docs/DEPLOYMENT.md 已记录入口 |
