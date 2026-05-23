# PM_LOG — T-018 upload-bin-multiport-ip-probe

> PM Orchestrator 的派发流水账。每次 stage 切换都追加一条。

## 任务来源

用户原话（2026-05-23）：

> 1.frpc或frps很容易下载失败，要不在webUI上写个上传入口吧；同时支持一键下载或人工直接上传上去；
> 2.ip的校验也可以用ip.cn等网站来校验大陆机器的ip；
> 3.根据context7的文档，frp是支持多个端口的转发，但当前UI设计好像不支持多端口的转发，也没有预设常用端口的转发，以及自动探测哪些端口可用等功能，需要设计并实现相关功能；
> 以用户体验好，符合软件工程标准，长期易使用易维护为原则来决策；
> 你来决策就可以了，我只看结果是否符合需求；
> 所以需要你根据我要求的原则来决策，然后执行；所有commit和push都由你来操作。

## PM 决策

- **任务粒度**：合并 3 项为单任务 T-018，原因：3 项互不冲突 / 都属 UX 增强 / 共用一次 verify_all + 一次发布；拆 3 任务会让用户多等 3 轮流水线开销。
- **强分区**：所有阶段文档使用 A/B/C 子模块前缀（A=上传二进制 / B=IP 校验源扩展 / C=多端口转发与端口探测），让 review/QA 可逐项核对。
- **风险隔离**：三模块在数据库 schema 与 API 路由互不重叠，单一 module 实现失败不阻其他模块发布。

## Stage 切换

| 时间 | From | To | 备注 |
|---|---|---|---|
| 2026-05-23 | — | req | T-018 创建，派 requirement-analyst |
| 2026-05-23 | req | design | 01 完成，Verdict=READY，含 10 个 [PM-DECIDED]；派 solution-architect |
| 2026-05-23 | design | gate-review | 02 完成，Verdict=READY，dev-backend → dev-frontend 派发顺序，dev-db 不参与；派 gate-reviewer |
| 2026-05-23 | gate-review | design | 03 首轮 Verdict=CHANGES REQUIRED（3 P0 + 3 P1 + 7 P2），路由回 Solution Architect 修订 02（不重走 Stage 1，仅 01 FR-A.1 文案同步） |
| 2026-05-23 | design | gate-review | 02 v2 修订完成（末尾有 Revision Log，01 FR-A.1 同步），所有 P0/P1/P2/Q1/Q2 吸收；派 gate-reviewer 二次评审 |
| 2026-05-23 | gate-review | development | 03 二次 Verdict=APPROVED FOR DEVELOPMENT；仅 1 个不阻塞小瑕疵（A.3 注释陈旧）；派 dev-backend 先行（dev-db 不参与），dev-frontend 待后端 API 就绪后接力 |
| 2026-05-23 | development | code-review | 04 合并完成（10 后端新文件 + 11 前端新文件，全部 14 后端包 PASS + 前端 99 vitest PASS + 自跑 verify_all PASS:19）；派 code-reviewer |
| 2026-05-23 | code-review | development | 05 首轮 Verdict=CHANGES REQUIRED（2 P0 契约漂移 + 1 P1 上限值偏离）；并行派 dev-frontend 修 P0-1/P0-2 + dev-backend 修 P1-1 |
| 2026-05-23 | development | qa | 修复完成（前端 96/96 PASS + 后端全包 PASS）；05 末尾"修复确认"= APPROVED FOR QA；派 qa-tester |
| 2026-05-23 | qa | delivery | 06 Verdict=PASS；首跑 verify_all E.6 FAIL（QA 写了"## 2. Adversarial tests"带数字编号），PM 修为裸"## Adversarial tests"后 verify_all PASS:19 |
| 2026-05-23 | delivery | done | 07 完成，baseline v6→v7（test_count 231→333），verify_all PASS:19 |
