# 03 — 闸门评审：T-010 deploy-polish-and-ci

> Stage 3 of 7 · PM-authored（清理性任务，PM 自审；用户授权自主决策）

---

## 评审结论

**APPROVED FOR DEVELOPMENT**

理由：4 条工作线全部为加法（新文件 + 文本替换），与现有架构不冲突；依赖增量小且业界标准；测试覆盖明确；回退路径清晰。

---

## 1. 需求 ↔ 设计可追溯

| AC | 设计章节 | 覆盖 |
|---|---|---|
| AC-1 占位符消除 | §2 L1 | ✅ 命中位置全列 |
| AC-2 自动开浏览器 | §4 L3 | ✅ 含 TTY 检测 + flag + env 三层 opt-out |
| AC-3 日志轮转 | §3 L2 | ✅ 含权限保持 + 子日志暂不轮转的说明 |
| AC-4 CI 发布 | §5 L4 | ✅ 复用现有 build/package 脚本 |
| AC-5 verify_all 不退化 | §8 实施顺序 + §9 回退 | ✅ 每线独立可回退 |
| AC-6 单测 | §3.5 / §4.5 | ✅ 三类测试明确 |
| AC-7 文档同步 | §6 | ✅ dev-map 增量段已写 |

无未覆盖 AC。

## 2. 风险评审

| 风险 | 设计缓解 | 评 |
|---|---|---|
| R-1 systemd 误触自动开浏览器 | TTY 检测 + env var 双保险 | OK |
| R-2 新依赖 lumberjack | MIT + k8s 在用 + 纯 Go | OK |
| R-3 平台 open 命令差异 | 三 case 分支 + WARN fallback | OK |
| R-4 CI workflow 失败 | 不在 verify_all 闸门内；GitHub UI 反馈 | OK（接受）|
| R-5 误中归档文档 | 显式排除 `_archived/` | OK |
| R-6 Windows 文件锁 | lumberjack 已处理 | OK |
| R-7 0.0.0.0 + 自动开浏览器 | rewrite URL 为 127.0.0.1 | OK |

无 MAJOR/CRITICAL。

## 3. 边界检查

- ✅ 未触 db migration（无 schema 变更）
- ✅ 未触 REST API（无 handler 变更）
- ✅ 未触前端代码（web/ 不动）
- ✅ 未触 archived 历史文档
- ✅ 不需要 frp 上游协议确认（无 frpc/frps 字段变更）

## 4. MINOR 建议（Developer 吸收即可，不阻塞）

- **M-1**：`browseropen.Open` 在 Linux 上若 `xdg-open` 不存在（headless server），错误信息会是 `executable file not found`；可在 ShouldOpen 之外多一层 fallback——`exec.LookPath("xdg-open") == nil` 时也跳过。优化项，不阻塞。
- **M-2**：`logrotate.New` 中 `Compress: false` 决策已注释（"简化运维：gzip 历史不易 grep"），保留这条注释即可。
- **M-3**：`.github/workflows/release.yml` 用 `actions/setup-node@v4` 缓存依赖；如果 `web/package-lock.json` 未提交（grep 验证）会 fail——验证一下。Developer 实施时确认。

## 5. 派发指令

PM 决定本期 dev 由 PM 直接做（机械性 + 清理性 + 无设计争议），完成后派 `code-reviewer` 与 `qa-tester` 子 agent 走 05/06 阶段。
