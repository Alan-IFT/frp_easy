# T-026 PM_LOG — install-ps1-iex-bom-and-host-exit-fix

## 任务起源（用户 2026-05-23 报告）

`irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex` 实测：

1. **首部两条 ParserError**（非终止性，但用户视觉污染）：
   - `﻿#: The term '﻿#' is not recognized as a name of a cmdlet, function, script file, or executable program.`
   - `param: The term 'param' is not recognized as a name of a cmdlet, function, script file, or executable program.`
2. 脚本继续执行到 `==> [5/8] 下载并校验发布包...`，然后用户报告"在 8 或 9 步把终端关闭，导致无法确定是否安装"。

## PM 根因初判（待 RA 正式分析覆盖）

- **E1 BOM 引发解析错误**：`scripts/install.ps1` 文件首 3 字节 = `EF BB BF`（UTF-8 BOM，T-021 加的）。`irm` 解码为字符串时 BOM 进入字符串作为 U+FEFF 字符。`iex` 解析器把 `<U+FEFF>#` 当 cmdlet 名解析，找不到 → 报错 1；接着 `param()` 不再处于"脚本第一句"位置 → 报错 2。两条错均非终止性，故脚本继续往下跑。
- **E2 `exit N` 在 iex 形态杀宿主**：`install.ps1` 多处 `exit 0` / `exit 1` / `exit 2`。`iex` 在父 runspace 执行，`exit N` 直接终止宿主 PowerShell 会话 → 用户看到的"终端关闭、无法验证"。
- **冲突约束**：T-021 给 .ps1 加 BOM 是为修 PS5.1+zh-CN 磁盘形态的 Chinese 解析。删 BOM 会回归到 T-021 之前的 GBK 误解码风险（install-service.ps1 / uninstall-service.ps1 仍是磁盘形态调用，BOM 必须保留；本任务仅处理 install.ps1 这一个 iex 入口）。

## 相关 insight 索引（已读 .harness/insight-index.md）

- **L36（T-024）**：iex 形态禁用 `[CmdletBinding()]` —— 已修复。但此次报告说明 BOM 同样是 iex 形态杀手，需补一条 insight。
- **L32-L33（T-021）**：BOM 加在 .ps1 是为 PS5.1+zh-CN 磁盘形态；L33 明确"PS5.1/7.x 加载磁盘 .ps1 时先剥 BOM 再 parse"。但 **L33 隐含范围仅限磁盘形态**——iex 形态下 BOM 经 irm 解码成 U+FEFF 字符进入字符串，不会被剥。
- **L25**：管道形态（curl|bash / irm|iex）禁用 `$0`/`$PSScriptRoot`。install.ps1 当前没用 `$PSScriptRoot`（OK），但 `& $svc` 调 install-service.ps1 用的是 `Join-Path $InstallDir "scripts\install-service.ps1"`（绝对路径锚定）—— 符合 L25。
- **L12**：固定安装目录 + mktemp 临时目录 —— install.ps1 现状符合。

## 派发计划

7 阶段全跑（feature/bug fix 含约束权衡 + 跨形态测试需求，trivial fix 路径不够稳）。

## 阶段日志

### 2026-05-23 11:00 任务创建

- 创建 `docs/features/install-ps1-iex-bom-and-host-exit-fix/`
- 在 `docs/tasks.md` 进行中表加 T-026 stage:req
- 派发 Requirement Analyst → 01_REQUIREMENT_ANALYSIS.md
