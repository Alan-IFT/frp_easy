# 交付报告 — T-004 tech-debt-cleanup

**交付日期**：2026-05-16  
**任务 ID**：T-004  
**Slug**：tech-debt-cleanup

---

## 功能摘要

T-004 清理了 T-003 识别的全部可操作技术债和优化建议，7 项改动全部交付：

1. **OPT-1 verify_all 前端质量门禁修复**：B.1-B.4 前端检查从永久 SKIP 变为真正执行（Go 测试 PASS:12→PASS:16，SKIP:6→SKIP:2）。意外发现并修复了 TypeScript Vue SFC 类型声明缺失（新增 `env.d.ts`）。

2. **OPT-2 向导路由守卫补全**：向导已完成后直接访问 `/wizard` 现在正确重定向到 `/dashboard`，消除 TD-1。

3. **OPT-4 slog 双写**：日志现在同时写文件和 stderr，开发期 `go run` 和生产期均可在 stderr 实时看到日志。

4. **OPT-5 版本号注入**：`build.sh` / `build.ps1` 从 `git describe --tags --always --dirty` 读取版本，不再写死 0.1.0，无 git tag 时 fallback "dev"。

5. **OPT-6 ParseIPFromJSON 统一**：`handlers_system.go` 的公网 IP 解析改用 `downloader.ParseIPFromJSON`，消除重复代码。

6. **OPT-7 /api/v1/health 端点**：新增公开健康检查端点，返回 `{"status":"ok","version":"..."}` 200，绕过 ReadyGate（启动中也可访问），适合 Uptime Kuma 等外部监控。

7. **OPT-8 auto-restore TOML 预检**：重启时如果 frpc.toml / frps.toml 不存在，记录 warn 并跳过，不再触发子进程立即失败。

---

## 文件改动

| 文件 | 改动类型 |
|---|---|
| `scripts/verify_all.sh` | 修改（前端路径修复） |
| `scripts/verify_all.ps1` | 修改（前端路径修复） |
| `scripts/build.sh` | 修改（版本注入） |
| `scripts/build.ps1` | 修改（版本注入） |
| `web/src/router.ts` | 修改（向导守卫） |
| `web/src/env.d.ts` | **新建**（Vue SFC 类型声明） |
| `cmd/frp-easy/main.go` | 修改（slog 双写 + TOML 预检） |
| `internal/httpapi/router.go` | 修改（health 端点 + Group 重构） |
| `internal/httpapi/handlers_system.go` | 修改（ParseIPFromJSON + health handler） |
| `internal/httpapi/qa_ac_test.go` | 修改（新增 2 个 health 测试） |
| `docs/dev-map.md` | 修改（补录 T-003 文件） |

---

## 测试基线变化

| 维度 | T-003 结束 | T-004 结束 | 增量 |
|---|---|---|---|
| Go tests | 117 | **119** | +2 |
| Frontend tests（现进入质量门禁） | 45 | 45 | 0 |
| verify_all PASS | 12 | **16** | +4 |
| verify_all SKIP | 6 | **2** | -4 |

---

## 技术债清偿状态

| ID | 描述 | 状态 |
|---|---|---|
| TD-1 | 向导路由守卫漏洞 | ✅ 已修复（OPT-2） |
| TD-2 | ParseIPFromJSON 重复 | ✅ 已修复（OPT-6） |
| TD-3 | verify_all 前端永久 SKIP | ✅ 已修复（OPT-1） |
| TD-4 | Version 写死 | ✅ 已修复（OPT-5） |
| TD-5 | slog 单写 | ✅ 已修复（OPT-4） |
| TD-6 | dist/ .gitignore 歧义 | ✅ 文档化（README，OPT-3 选 B） |
| TD-7 | TOML 预检缺失 | ✅ 已修复（OPT-8） |
| TD-8 | SQLite 单连接 | 🟡 保留（SQLite 正确设计） |

## Insight

**修复质量门禁往往会暴露隐藏问题**：OPT-1 让 tsc 首次真正运行，立即发现了已存在多任务的 Vue SFC 类型声明缺失（TD-9 级别，属于 P0 工程基础），这种"防腐层"问题在没有真正运行类型检查时永远不会被发现。建议每次新增前端功能后立即跑一次 tsc 而不等 verify_all。
