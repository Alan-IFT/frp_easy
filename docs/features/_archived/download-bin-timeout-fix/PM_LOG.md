# PM_LOG — T-025 download-bin-timeout-fix

mode: full
stage: requirements
created: 2026-05-23

## 起源

用户报告：webUI 一键下载 frps 报错，frpc 同款问题。生产 systemd journal 显示：
- 21:26:32 POST /api/v1/system/download-bin (202)
- 21:26:33 download started, kind=frps, url=https://github.com/.../frp_0.69.0_linux_amd64.tar.gz
- 21:27:33 ERROR "下载写入失败: context deadline exceeded (Client.Timeout or context cancellation while reading body)"

精确 60 秒后失败 → 根因：`internal/downloader/downloader.go:71` `&http.Client{Timeout: 60 * time.Second}` 是整请求总超时（含 body 读取）；frp archive ~14MB 在国内 GitHub CDN 速率下几乎必崩。

## 关联

- T-014 frp-binary-auto-download：首次引入 downloader.Manager + 60s Timeout
- T-018 upload-bin-multiport-ip-probe：扩展 Install 函数共享、未碰 Timeout

## insight-index 相关条目

- 无直接对应（GitHub API timeout 类问题首次出现）
- L29/L45：spec mock 测试无法捕获契约漂移 → adversarial tests 必须用真实网络仿真（httptest 慢响应）

## 阶段路由

待派发：Requirement Analyst
