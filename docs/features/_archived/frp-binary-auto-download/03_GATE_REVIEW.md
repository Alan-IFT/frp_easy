# 03 Gate Review — T-014 frp-binary-auto-download

> Harness 流水线 stage 3 产出。Gate Reviewer 独立验证（所有"复用现有代码"声明已实读核对，引用行号已抽样验证）。
> 上游：01（READY）、02（READY）。

## 1. 八维审计

| # | 维度 | 结论 |
|---|---|---|
| 1 | 需求完整性 | PASS — 14 FR 可观察可测，13 闸门 AC + 3 MV 划分清晰 |
| 2 | 设计完整性 | WARN — package.ps1 改动只在 §9 一笔带过、无确切行号（F-1） |
| 3 | 复用正确性 | PASS — extractFromTarGz/Zip 用 filepath.Base 匹配、baseURL/goos seam 经实读核实 |
| 4 | 风险覆盖 | PASS — R-1~R-7 到位；verify_all/e2e 不依赖 frp 二进制经独立核实 |
| 5 | 迁移安全 | PASS — git rm 可逆；升级路径改隐式契约为显式契约 |
| 6 | 边界处理 | PASS — latest 解析 4 失败分支 + JSON 非法 + LimitReader 防御 |
| 7 | 测试可行性 | PASS — AC 全静态可验；AC-6 四分支测试用 httptest seam 注入 |
| 8 | 范围边界 | PASS — O-1~O-8 明确 |

## 2. Findings

- **F-1（WARN，必办）**：`package.ps1` 是 `package.sh` 的 Windows 对等脚本，含同款 frp 二进制前置检查与打包逻辑，但设计 §3.6/§9 只对 package.sh 给了精确行号。确切改动点（评审补出）：删 L80-89 + L90-98 两块 frp 子二进制前置检查；删 L204、L210 两行 frp 二进制 Copy-Item；同步 L11 头注释；staging 文件数断言阈值 6 不变。不改会导致 Windows 打包在仓库无 frp 二进制时 exit 1，违反 FR-4。
- **F-2（低风险提示）**：`.gitignore` 的 `.dl-*.tmp` glob 需同时覆盖 downloader 实际的两个临时前缀 `.dl-archive-*.tmp` 与 `.dl-bin-*.tmp`，设计写法正确，落地勿改窄。
- **F-3（观察项，建议 PM 知会用户）**：OQ-2 选候选 A（不自动触发下载、沿用 T-002 横幅点击），工程上合理（启动不阻塞是结构性保证、零 main.go 改动）。但用户原话"自动下载"被解读为"app 帮你下 vs 手动放文件"而非"启动即自动下"。RA 已用 O-6 把"首启自动触发"固化为 out-of-scope，决策合规。建议 PM 交付时知会用户实现为"打开 UI → 横幅一键下载"。

## 3. 关注点回应
- **移除内置二进制连锁影响**：覆盖完整。binloc 不改（纯 os.Stat 探测）；downloader_test.go 编译失败点实读确认为 L98/L139；`.gitignore` 根锚定精确 4 文件名、不误伤 frp_linux/LICENSE。唯一缺口是 package.ps1（F-1）。
- **OQ-4 升级路径修复**：正确且必要。显式删除 install.sh 升级路径的 `rm -rf frp_linux/`+cp 块、install.ps1 L183 的 `frp_win` 数组项；修复后用户运行时下载的 frpc/frps 升级时不被触碰，发布包该更新的文件仍全覆盖、无遗漏。
- **downloader 改下载最新版**：健全。API 端点 `/repos/fatedier/frp/releases/latest`、User-Agent 头、先判状态码后解析 JSON、资产后缀匹配、限流降级齐全；改动面确实小。
- **R-2 长期风险**：已记录，要求写入 07 Insight，本期不做版本适配可接受。

## 4. Verdict

**APPROVED FOR DEVELOPMENT**

进入 Developer 前 PM 须纳入两项工作项（均不阻塞）：
1. **F-1（必办）**：package.ps1 与 package.sh 同步改造，确切行号见上。
2. **F-3（PM 知会用户）**：OQ-2 实现为"横幅一键下载"，建议交付时向用户说明。
