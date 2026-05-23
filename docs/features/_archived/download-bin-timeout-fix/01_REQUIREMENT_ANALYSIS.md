---
task_id: T-025
slug: download-bin-timeout-fix
stage: requirements
author: Requirement Analyst
date: 2026-05-23
mode: full
---

# 01 — 需求分析（T-025 download-bin-timeout-fix）

## 1. 现象与根因摘要

- 用户在 webUI 点「一键下载 frps」失败；用户同时声明「frpc 应也有」。
- 生产 systemd journal（Linux 服务模式）证据：
  - `21:26:32 POST /api/v1/system/download-bin → 202`
  - `21:26:33 download started kind=frps version=v0.69.0 url=…frp_0.69.0_linux_amd64.tar.gz`（对应 `internal/downloader/downloader.go:135` 的日志输出）
  - `21:27:33 ERROR "下载失败" err="下载写入失败: context deadline exceeded (Client.Timeout or context cancellation while reading body)"`（对应 `internal/downloader/downloader.go:185`）
- 精确 60 秒切断，根因定位：`internal/downloader/downloader.go:71` `client: &http.Client{Timeout: 60 * time.Second}`。
- Go `http.Client.Timeout` 语义为「整个请求生命周期上限（含 dial + TLS + 写请求 + 读 header + 读 body）」。frp archive ~14MB；在国内 GitHub release CDN（受 GFW 影响）实测速率常为 50–200 KB/s，完成 14MB 需 70–300s，60s 几乎必然提前触发 deadline。
- 同一 `m.client` 同时承担 `resolveLatestAsset`（GitHub API JSON 查询，`downloader.go:317`）与 archive 下载（`downloader.go:162`），两者的超时需求完全不同。

## 2. 范围

### 2.1 In scope

- `internal/downloader.Manager` 内部 HTTP 客户端的超时策略调整。
- archive 下载链路（`doDownload` 162 行 `m.client.Do(req)` 以及随后的 `io.Copy` 174–187 行）不再受单一 60s 总超时切断。
- `resolveLatestAsset`（GitHub API 查询）保留短超时（≤60s）以便快速失败。
- frpc / frps 两个 kind 同款修复（共享 client，自动覆盖）。
- 必要的 adversarial 测试（httptest 仿真慢响应 + 死连接）。

### 2.2 Out of scope（本任务不做）

- 前端「取消下载」按钮 / Abort 控制。
- 断点续传（Range 请求 / resume）。
- 失败后自动重试 / 退避策略。
- 镜像源切换（ghproxy / fastgit 等）。
- downloader 模块整体重写或重构 archive 解压流。
- API/响应体 schema 变更（`/api/v1/system/download-bin`、`/download-status/{kind}`）。

## 3. 功能性需求 FR

- **FR-1**：在慢源（持续低速但不断流）网络下，archive 下载链路总用时 ≥10 分钟也不应被 downloader 自身的超时配置切断。
- **FR-2**：连接建立、TLS 握手、响应头读取仍须有上限（防 hang on 死/半死服务器），上限在合理量级（≤60s）。
- **FR-3**：`resolveLatestAsset` 对 GitHub API 的查询保留短上限（≤60s），失败行为与既有一致（中文错误消息、HTTP 状态码分支）。
- **FR-4**：archive 下载失败时面向用户的错误消息保持/优化为中文，至少能区分「网络/连接类失败」与「写入/磁盘类失败」（保持既有 `setFailed` 调用语义）。
- **FR-5**：frpc 与 frps 两个 kind 行为完全一致；不引入 kind 特有分支。

## 4. 非功能性需求 NFR

- **NFR-1（稳定性）**：超慢但活的连接（≥10 分钟，14MB）能完成下载并 install 成功，不引入新的 hang 风险（必须有 idle / response-header 上限做兜底）。
- **NFR-2（可观测）**：现有 `m.logger.Info("download started", …)` 与 `m.logger.Error("download failed", …)` 调用点保留；不必新增字段（但允许 Architect 阶段考虑添加 elapsed/bytes 字段，非本 RA 强制）。
- **NFR-3（向后兼容）**：HTTP API contract（`POST /download-bin`、`GET /download-status/{kind}` 响应体 `{status, progress, error}`）零变更；前端 `web/src/stores/downloader.ts` 与 `web/src/api/downloader.ts` 零改动。
- **NFR-4（最小侵入）**：只改 `internal/downloader/`；不动 binloc、httpapi handler、service mode、前端。
- **NFR-5（测试可注入）**：保留 `apiBaseURL`、`goos` 既有测试 seam；如新增 timeout 相关字段，须支持包内测试覆盖。

## 5. 验收标准 AC

- **AC-1（慢传输不超时）**：httptest 仿真服务器以 50 KB/s 速率回写 14 MB 响应体（≈ 280s），downloader 调用 `Start` 并轮询 `Status`，最终 `status == "success"`；中途无 `context deadline exceeded` 失败。
- **AC-2（死连接快速失败）**：httptest 仿真服务器 accept 后不发任何 response header（持续 hang），downloader 在 ResponseHeader 上限（≤60s）内以 `StatusFailed` 收场，错误消息含「下载超时」或同等中文。
- **AC-3（GitHub API 短超时保留）**：httptest 仿真 `/repos/fatedier/frp/releases/latest` 持续 hang，downloader 在 ≤60s 内以 `StatusFailed` 收场，错误消息为既有的「无法访问 GitHub（请检查网络或代理）: …」前缀。
- **AC-4（双 kind 一致）**：上述 AC-1/AC-2/AC-3 三类用例对 `kind=frpc` 与 `kind=frps` 均通过（参数化或重复用例皆可）。
- **AC-5（无回归）**：`scripts/verify_all` PASS；既有 downloader 单测（含 T-014 引入的 spec 测试）全 PASS；adversarial 测试用 `httptest.Server` 真实回写，而非 mock interface（遵循 insight L29/L45）。
- **AC-6（前端体验保持）**：在 AC-1 慢源场景下，前端轮询 `/download-status/{kind}` 期间能看到 `progress` 字段持续上升（`progressWriter` 在 `downloader.go:381–411` 的既有行为）；本任务不改动 progress 计算，仅验证不被超时打断后 progress 持续推进。

## 6. 自决断的歧义项（无澄清，按用户指示直接定）

> 用户指令：「不停下来问澄清问题，做合理判断继续」。以下每条均为 RA 当场决策，Architect 阶段可在 SD 中调整但需说明理由。

- **D-1：archive 下载总超时**：决策为「不设总 `Client.Timeout`，但设 idle / response-header 兜底」。
  - 理由：14MB 在国内 release CDN 极端情形可能要 5–10 分钟；用户主动操作（webUI 点击），长时间运行可接受；前端 polling 显示 progress，用户随时可关页面（即便不能真正 cancel goroutine，UI 不会卡死）。
  - 兜底由 D-2 / D-3 提供，防止真死连接无限挂起。
- **D-2：ResponseHeaderTimeout**：决策为 60s。
  - 理由：与既有 60s 行为一致，覆盖国内 GFW 偶发的「能建连但 server 不发 header」情况。
- **D-3：连接 / TLS 阶段上限**：决策为 dial 30s、TLS handshake 30s。
  - 理由：与 Go 标准库 DefaultTransport 不同（DefaultTransport dial 30s 但无 TLS handshake 显式限制）；本场景显式给出可观测上限。
- **D-4：拆分 API client 与 download client**：决策为「拆」。
  - 理由：两者超时需求完全不同（API 快、download 长）；继续共享一个 client 会被未来人误改回单值。拆分增加 ~10 行代码、零额外维护负担。
  - 命名建议（仅 hint，Architect 决定）：`apiClient`（短超时）+ `downloadClient`（仅有 header/dial 上限、无总超时）。
- **D-5：前端取消按钮**：out of scope（保留给未来任务，例如 T-026 「downloader-cancel-button」若被提出）。
- **D-6：失败错误消息**：保持既有中文文案（`下载超时:`、`下载写入失败:`、`HTTP %d: 下载失败`）；不强制改写。Architect 可在 SD 中按需调整。
- **D-7：context 注入 / Manager 级 cancel**：out of scope（同 D-5，需要前端配套）。

## 7. 风险 R

- **R-1**：移除总超时后，若 server 在 body 阶段「逐字节 trickle」（每 30s 发 1 字节，连接看似 alive 但实际 stall），下载可能挂数小时。
  - 缓解：Architect 在 SD 中考虑 idle read timeout（如 `net.Conn` SetReadDeadline 周期性刷新，或 `http.Transport.IdleConnTimeout`）；如不引入，应在 SD 文档中显式记录该已知风险。
- **R-2**：goroutine 长生命周期（10 分钟级）下，若用户重启服务（systemd restart）或进程退出，下载会丢失。属于既有行为，不在本任务变更。
- **R-3**：拆分两个 client 后，单测注入 seam 数量增加；测试代码改动量略大于「单字段替换」。属于可接受成本。
- **R-4**：前端 polling 当 `progress` 长时间不变（例如 ResponseHeader 阶段或慢 trickle）时，用户体感「卡住」。属于既有 UX，不在本任务变更，但 Architect 可考虑在 SD 中评估是否在 progressWriter 增加「已接收字节」字段（非强制）。
- **R-5**：adversarial 测试用 httptest 慢响应（`time.Sleep` + 分块 `Write`）会让 CI 单次跑慢。Architect/QA 阶段应让慢响应测试用「短 deadline + 小总量」校准（例如 5 KB/s × 50 KB = 10s 即可验证「不在 N 秒处崩」），而不是真跑 280s。

## 8. 关联历史任务

- **T-014 frp-binary-auto-download**（`docs/features/_archived/frp-binary-auto-download/`）：首次引入 `downloader.Manager` 及 60s `Client.Timeout`。本任务直接修正该决策。Architect 阶段须读 T-014 的 `02_SOLUTION_DESIGN.md` 以避免与该 SD 的其他约束冲突。
- **T-018 upload-bin-multiport-ip-probe**（`docs/features/_archived/upload-bin-multiport-ip-probe/`）：扩展了 `Manager.Install` 共享路径（参见 `downloader.go:226–238` 的复用调用），未触及 client/timeout。
- **insight L29/L45**：spec mock 测试无法捕获契约漂移 — 本任务 adversarial 测试必须用 `httptest.Server` 真实 HTTP 行为，禁止用 interface mock 替代。

## 9. 判定

**READY**（无未决问题；所有歧义项已按 §6 自决断并记入文档，Architect 可直接进入方案设计）。
