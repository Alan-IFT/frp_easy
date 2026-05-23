# Insight History — frp_easy

> 主索引 `.harness/insight-index.md` 中被替换 / 轮转出去的历史 insight 归档。
> 永不删除（参考 `.harness/rules/05-insight-index.md` "归档"段）。
> 新条目追加到顶部。

---

## 2026-05-23 · [CORRECTED by T-016] T-008 systemd unit 双引号 insight 错误

原始文本（位于 `.harness/insight-index.md` 第 18 行，已被 T-016 替换）：

> - **2026-05-19** · systemd unit 中 `ExecStart=${PATH}` 与 `WorkingDirectory=${PATH}` 必须用双引号包路径（`ExecStart="${PATH}"`，systemd 5.0+ 语法），否则路径含空格时 systemd 按空格分参导致启动失败。Code Review MAJOR-1 直接证据 · evidence: T-008 deploy-kit

**纠错背景**：T-016 install-progress-and-systemd-unit-fix 任务中，线上 Ubuntu VM 实测显示 `WorkingDirectory="/opt/frp-easy"`（整体双引号）触发 `Failed to start frp-easy.service: Unit frp-easy.service has a bad unit file setting.`。systemd.exec(5) 实际语义：`WorkingDirectory=` 字段任何 systemd 版本都不接受整体双引号——引号字符进入字符串本身让目录路径变成 `"/opt/frp-easy"` 字面（含引号），路径不存在。正确做法是裸 token + C-style `\x20` 转义。

T-016 已用此真相替换主索引第 18 行。

## Rotated 2026-05-23

- YYYY-MM-DD · <一句话事实> · evidence: <任务 slug 或 commit sha>
- 2026-05-16 · Windows os.Rename 不能覆盖已存在文件，需先 Remove 再 Rename；但 Remove 成功后 Rename 失败会丢失原文件，正确模式是先试 Rename 失败再 Remove+Rename · evidence: zero-config-quickstart
