# INPUT · T-021 encoding-ps51-bom

> 来源：T-019 windows-service-scm-1053-fix `07_DELIVERY.md` §7 follow-up backlog 第 1 项；T-018 同款历史遗留首次记录。

## 用户原始诉求（PM 转述）

继续审视项目，找未完成工作 / 优化 / UX 不佳处。在 T-019 交付时被识别为 **MAJOR 历史遗留 backlog**：

> Windows PowerShell 5.1（zh-CN 主机）加载磁盘上的 `scripts/*.ps1` 文件时遇 UTF-8 无 BOM 解析失败 —— 中文字符被错解为 ANSI/GBK 编码，语法错误，脚本无法直接运行。
>
> 影响：所有 `scripts/*.ps1` 在 PS5.1 + zh-CN 主机执行非 `irm | iex` 管道形态会失败（管道形态由 PS 自身解码 stdin、不走磁盘 BOM 探测，故 `irm | iex` 路径已被 T-013 验证 OK）；T-018 首次遇到此 baseline，T-019 仍未阻塞。

## 当前状态（PM 调查）

```
noBOM  ascii     5044 bytes  scripts/archive-task.ps1
noBOM  zh        2176 bytes  scripts/build.ps1
noBOM  ascii     4685 bytes  scripts/harness-sync.ps1
noBOM  ascii     2783 bytes  scripts/install-hooks.ps1
noBOM  zh        9705 bytes  scripts/install-service.ps1
noBOM  zh       15596 bytes  scripts/install.ps1
noBOM  zh        9708 bytes  scripts/package.ps1
noBOM  zh        2923 bytes  scripts/start-e2e-server.ps1
noBOM  zh        2312 bytes  scripts/start.ps1
noBOM  zh        3990 bytes  scripts/uninstall-service.ps1
noBOM  zh       12459 bytes  scripts/verify_all.ps1
```

**11/11 .ps1 noBOM；其中 8 个含中文字符**（在 PS5.1 + zh-CN host 上磁盘加载报 syntax error）。3 个纯 ASCII 脚本严格而言不需要 BOM，但为一致性应统一加。

## 目标

让 PS5.1 + zh-CN 用户能直接 `powershell.exe -File scripts/install.ps1` / `.\scripts\verify_all.ps1` 等运行（**不依赖** `irm | iex` 管道形态、不依赖 PS7.x），中文字符不再被 GBK 错解。

## 范围

- 所有 `scripts/*.ps1` 改为 **UTF-8 with BOM**（字节级 `EF BB BF` 前缀）。
- 新增 `verify_all` 检查项，确保 `.ps1` 文件 BOM 持久（防回归：未来 PR 改 .ps1 时若工具误改回 noBOM 立即被拦）。

## 非范围 / 边界

- 不改 `.harness/skills/*/SKILL.md` 等非 .ps1 文件。
- 不动 `.sh` / `.go` / `.ts` 等其他源文件编码。
- 不改 install.sh 等 Linux/macOS 入口（无此问题）。
- 不改 PowerShell 行为本身（用户终端 `$OutputEncoding` / chcp 等不在范围）。

## 约束 / 依赖

- T-020-followup 已确认：根级 `_comment` 顶层字段允许，但子对象 `additionalProperties: false` 可能拒绝下划线字段；新加 verify check 不需触及 settings.json。
- insight L24 + L35：QA 报告 `## Adversarial tests` 必须裸英文标题、无数字编号前缀。
- insight L43：07 `## Insight` 必须裸标题、无编号，否则 archive-task 0 收割。
- insight L41 + L42：reviewer 的派发 prompt 必须显式 "必须直接写到文件" 否则 PM 接管落盘。

## 已知风险

1. PowerShell 5.1 + zh-CN 验证条件本地缺失（QA 主机 yangx 是 W11 Home 26200，PS7 默认；PS5.1 + zh-CN 真机断言由用户在 §6 真机验证清单复现），与 T-019 真机 AC 同款降级模式。
2. 部分编辑器（VS Code 默认 / Vim）可能在保存时去掉 BOM；需要 `.editorconfig` 或类似机制锁定。
3. archive-task.ps1 自身被加 BOM 后，能否正常解释执行需要 dogfood 验证。
