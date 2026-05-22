# 05 Code Review — T-014 frp-binary-auto-download

> Harness 流水线 stage 5 产出。Code Reviewer 独立核对 git 工作区实际改动（含 git rm 删除；未 commit）。
> 上游：01/02/03（APPROVED FOR DEVELOPMENT）/04（无 DESIGN DRIFT，1 Open issue）。

## Verdict：APPROVED（BLOCKER 0 / MAJOR 0 / MINOR 2 / NIT 1）

13 条完成闸门 AC 全部有实现、无 CRITICAL 缺口；设计保真度高、无静默方案漂移；4 个新增测试 meaningful 且全程 httptest。

## 核查结论
- **git 移除内置二进制**：8 个 frp 文件已 `git rm`；`frp_linux/LICENSE`、`frp_win/LICENSE` 保留为占位锚点；`.gitignore` 前导 `/` 根锚定精确 4 文件名，不误伤 LICENSE。✅
- **downloader 改下载最新版**：`resolveLatestAsset` 先判状态码后解析 JSON、User-Agent 头、io.LimitReader 1MiB、后缀匹配、403/网络/非200/资产未匹配中文降级齐全；latest 解析插在 resolveParams 后、MkdirAll 前（失败不留空目录）。✅
- **OQ-4 升级路径修复**：install.sh/install.ps1 升级分支已不再 rm -rf/cp `frp_linux`/`frp_win`，用户运行时下载的 frpc/frps 升级时保留；发布包其它文件仍正确覆盖。✅
- **package.sh + package.ps1**：frp 二进制前置检查与打包逻辑均已移除（Gate F-1 的 package.ps1 已同步）。✅
- **NOTICE / 文档 / install 更新提示**：NOTICE 改写为"运行时下载"；README/dev-map 更新；install 脚本加"更新"段。✅
- **baseline.json**：go_tests 167→171、test_count 224→228，与新增 4 测试吻合（红线 3 满足）。✅
- **无红线文件被改**。✅

## Findings

### BLOCKER / MAJOR
无。

### MINOR
- **M-1 `[MAINT]` `internal/downloader/downloader.go:58` `Manager.baseURL` 死字段**：改造后 downloader 包内（含测试）零引用，下载 URL 全部来自 `resolveLatestAsset`。Developer"保留以最小化改动面"论据不成立——删除是 1 行字段+注释的净删除，不触碰任何签名/调用方，改动面比保留更小。红线"不留死代码"。→ 本期删除。
- **M-2 `[MAINT]` `docs/DEPLOYMENT.md` C.2.4 / C.3.4 手动升级指令过时**：两段仍引用已移除的发布包目录（C.2.4 `tar xzf ... frp_linux`、C.3.4 `Copy-Item ... frp_win`），照做会令 tar/Copy-Item 报错。设计 §3.8 只点了 F.5/A.0，遗漏此两节。AC-11 grep 不覆盖 → verify_all 仍 PASS 但文档对用户错误。→ 本期修正，与 A.0"升级保留已下载二进制"对齐。

→ PM 裁决：M-1/M-2 均路由回 Developer 本期修复（M-1 是红线死代码；M-2 是会让用户操作失败的过时文档，与用户一贯"文档须准确"诉求冲突）。

### NIT（无需改）
- N-1 `downloader.go:136` 日志文案 `"download started"` 与设计示例 `"resolved latest frp release"` 略有出入，信息无损。

## 给 PM / QA
- QA 须实跑 verify_all 复核 AC-13，特别留意 C.1 Playwright e2e（R-1）。
- R-2 长期风险（frp latest 版本不受控、TOML schema 兼容）须写入 07 Insight。
