# 任务输入 — T-001 · web-ui-mvp

## 用户原话（2026-05-16）

> frp_linux 下放的 linux 版本，frp_win 下放的 win 版本，需要你帮我设计 UI，达到项目目的，人工主要配置登录的账号密码，端口开关这些就行了吧？可通过 context7 查看 frp 和 frp panel 官方文档来参考设计；

## 用户对决策权的授权（2026-05-16，第二条消息）

> 以用户体验好，符合软件工程标准，长期易使用易维护为原则来决策；你来决策就可以了，我只看结果是否符合需求；所以需要你根据我要求的原则来决策，然后执行，最后返回给我改动详情，当前情况等我需要关注的信息；所有 commit 都由你来操作

## 决策原则（PM 全程必须遵循，并要求每个 agent 遵循）

按下列优先级排序：

1. **用户体验**：默认安全可用、最少必填字段、错误可恢复、操作可观察。
2. **软件工程规范**：分层清晰、单一职责、可测、可观测、配置 / secret 与代码分离、依赖明确、跨平台一致。
3. **长期可维护性**：技术栈主流稳定、文档随代码、状态可迁移、升级路径可控。

## 项目语境

- 现状：仓库刚 bootstrap，零代码；`frp_linux/` 与 `frp_win/` 内仅含上游 FRP 二进制（`frpc` / `frps`）与默认 TOML 配置（见 `frp_linux/frpc.toml` 等）。
- 项目目的：把 FRP 包装成"开箱即用"的本地工具——非技术用户 git clone 后即可通过 Web UI 配置常用项（端口转发、登录凭据等），无需手写 TOML。
- 用户提示：可参考 FRP（[fatedier/frp](https://github.com/fatedier/frp)）与 FRP Panel（[VaalaCat/frp-panel](https://github.com/VaalaCat/frp-panel)）官方文档，通过 context7 拉取。

## 工作模式

- **PM 自治模式**：用户授权 PM 全权决策。开放问题原则上由 PM 自答，不向用户阻塞。
- **回报粒度**：PM 在 Stage 7 交付时汇总：改动详情 + 当前状态 + 用户需关注的关键决策与风险。
- **commit 归 PM**：所有 commit 由 PM 操作（含每阶段里程碑提交）。

## 给 agent 的统一约束

1. 不要再向用户提"待回答的开放问题"。每个开放问题必须由该 agent 在文档中**自行作答**并标 `[PM 决策]`，给出依据（哪条原则）。
2. 输出全部中文。
3. 引用 FRP / FRP Panel 文档时，必须通过 context7（`mcp__plugin_context7_context7__resolve-library-id` + `mcp__plugin_context7_context7__query-docs`）拉取，禁止凭印象写。
4. 安全相关默认值（凭据加密、绑定地址、TLS）必须高于"能跑就行"。
