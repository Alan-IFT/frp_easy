# T-027 PM_LOG — download-cancel-and-upload-decouple

## 任务起源（用户 2026-05-23 报告）

用户场景：
1. 在 Web UI 点"一键下载 frpc/frps" → 网络慢 → 下载 goroutine 卡 archive body 读取，**无法取消**。
2. 用户索性手动从 GitHub releases 下载 binary → 想用"上传 frpc/frps"按钮提交 → 后端检测到 `Status==downloading` → **409 PROC_BUSY**：`下载进行中，请稍后再上传或取消下载`。
3. 但**前端没有"取消下载"按钮**，提示语指向一个不存在的能力，用户陷死局。

## PM 决策依据（用户提出的三条原则）

1. **用户体验好**：长耗时操作必须给用户主动止损的通道（业界 mainstream，参考 npm/curl/git clone 的 Ctrl-C 语义）。
2. **符合软件工程标准**：异步任务的"启动 / 状态查询 / 取消"三件套是网络长操作的标准 ABI。
3. **长期易使用易维护**：状态机必须确定；不允许"上传静默打断下载"或"上传/下载并发覆盖目标 binary"这类隐性 race。

## PM 决策结论（PM-DECIDED）

**采用候选 A：补齐"取消下载"能力，下载/上传互斥保留但变成"可操作互斥"。**

候选评估：
- **A. 加 Cancel API + UI 取消按钮**：补齐异步任务三件套；保留下载/上传互斥（避免双写 race）；上传被阻断时前端给清晰的"先取消下载"操作路径。✅
- **B. 上传静默覆盖下载**：行为不可预测；用户上传时不知道有下载在跑会无声打断；状态机变复杂。✗
- **C. 移除互斥（上传/下载并发）**：两个写路径并发到同一 binary 路径，原子 rename 的"先到先输"是 race，状态混乱。✗

具体改动方向（细节给 Solution Architect 拍）：
- **后端**：`downloader.Manager` 新增 `Cancel(kind) error`；`doDownload` 改用 `context.Context`，HTTP 请求 `req.WithContext`，body 拷贝随 ctx 取消自然解除；新增 `StatusCanceled` 状态（与 `StatusFailed` 区分，"用户主动取消"≠"网络/解压失败"）；新增 HTTP endpoint。
- **前端**：下载按钮在 `downloading` 状态下变成"取消下载"按钮（或加副按钮）；`canceled` 状态下回到可重试入口；上传按钮在下载中时 hover 提示"先取消下载"。
- **测试**：单测 Cancel 触发后 ctx 真正解除 io.Copy；适配 HTTP 路由 + 锁互斥；前端 store 状态机回归。

## 相关 insight 索引（已读 .harness/insight-index.md）

- **L38（T-025）**：`http.Client.Timeout=0` 已配置 archive 下载 client；本任务的取消信号要走 ctx，不依赖 client.Timeout 也不要回退到 Timeout 切断 —— 那是"上限保护"，不是"用户取消"。
- **L29 / L40（T-018 / T-023）**：前端 TS 接口与后端 Go struct 字段名漂移在双方 mock 都 PASS 时无法捕获 —— 新增 `StatusCanceled` / `Cancel` endpoint 时 OpenAPI 一份 + 前后端字段一锤定音。
- **L42（T-025）**：注释中引用归档后路径 `_archived/` 在归档前 commit 会 404。本任务 doDownload / Cancel 注释中若引用 02 设计文档路径，写"docs/features/download-cancel-and-upload-decouple/ 或归档后 docs/features/_archived/download-cancel-and-upload-decouple/"双路径。
- **L21（T-018）**：QA `06_TEST_REPORT.md` 必须含**裸**`## Adversarial tests` 段（无数字前缀），verify_all E.6 闸门。
- **L43（T-019）**：07_DELIVERY.md 的 insight 段必须**裸** `## Insight` 或 `## Insights`，archive-task.ps1 收割 regex 不允许数字前缀。
- **L41 / L44（T-018 / T-019）**：reviewer 子 agent 倾向不落盘，派发 prompt 显式"必须直接写到 0X_*.md 文件"。

## 派发计划

完整 7 阶段（feature 类、跨前后端、引入新 ABI + 新状态，trivial 路径远远不够）。

## 阶段日志

### 2026-05-23 12:00 任务创建

- 创建 `docs/features/download-cancel-and-upload-decouple/`
- 准备在 `docs/tasks.md` 进行中表追加 T-027 stage:req
- 派发 Requirement Analyst → 01_REQUIREMENT_ANALYSIS.md
