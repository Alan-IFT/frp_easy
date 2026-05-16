# 需求分析 — T-004 tech-debt-cleanup

**任务 ID**：T-004  
**日期**：2026-05-16  
**输入**：T-003 分析的 TD-1～TD-8 + OPT-1～OPT-9

---

## 功能需求

### F-1：修复 verify_all 前端质量门禁（OPT-1 / TD-3）

`verify_all.sh` 和 `verify_all.ps1` 的 B.1-B.4 检查在根目录查找 `package.json`，而项目的 `package.json` 在 `web/` 子目录，导致永久 SKIP。45 个前端测试从未进入质量门禁。

**AC-F1-1**：`scripts/verify_all.sh` 执行后 B.1（typecheck）、B.3（unit tests）不再 SKIP，实际运行前端检查。  
**AC-F1-2**：`scripts/verify_all.ps1` 同上。  
**AC-F1-3**：前端测试全部通过时 B.3 = PASS；B.4 检查前端测试数 ≥ baseline。

### F-2：补全向导路由守卫（OPT-2 / TD-1）

`router.ts` 的 beforeEach 守卫仅在导航到 `/dashboard` 时触发向导检查，直接访问 `/wizard`（当 wizard 已完成后）不会被重定向。

**AC-F2-1**：向导已完成（`wizard.handled === true`）时，导航到 `/wizard` 被重定向到 `/dashboard`。  
**AC-F2-2**：向导未完成时，直接访问 `/wizard` 正常显示向导页。  
**AC-F2-3**：已有的向导触发逻辑（从 /dashboard 导航）不受影响。

### F-3：slog 双写 stderr+文件（OPT-4 / TD-5）

`cmd/frp-easy/main.go` 的 `newLogger` 函数在 logFile 非 nil 时只写文件，stderr 不收 slog 输出，开发调试不便。

**AC-F3-1**：logFile 非 nil 时，slog 输出同时写入 logFile 和 stderr（`io.MultiWriter`）。  
**AC-F3-2**：logFile 为 nil 时行为不变（写 stderr）。

### F-4：版本号从 git describe 注入（OPT-5 / TD-4）

`scripts/build.sh` 和 `build.ps1` 中 `VERSION="0.1.0"` 硬编码，缺乏构建可追溯性。

**AC-F4-1**：`build.sh` 使用 `git describe --tags --always --dirty` 作为版本，git 不可用时 fallback 到 `"dev"`。  
**AC-F4-2**：`build.ps1` 同上。  
**AC-F4-3**：版本字符串通过 `-ldflags "-X main.Version=..."` 注入二进制，`/api/v1/system/ready` 返回正确版本。

### F-5：统一 ParseIPFromJSON（OPT-6 / TD-2）

`internal/httpapi/handlers_system.go` 的 `fetchIPFromURL` 内联解析 `{"ip":"..."}` JSON，与 `internal/downloader` 包导出的 `ParseIPFromJSON` 重复。

**AC-F5-1**：`fetchIPFromURL` 改为读取全部 body 后调用 `downloader.ParseIPFromJSON`，删除内联解析结构体。  
**AC-F5-2**：现有公网 IP 检测功能测试全部通过。

### F-6：添加 /api/v1/health 端点（OPT-7）

缺少标准健康检查端点，外部监控工具（Uptime Kuma 等）无法探测服务存活。

**AC-F6-1**：`GET /api/v1/health` 返回 HTTP 200 + `{"status":"ok","version":"..."}` JSON。  
**AC-F6-2**：此端点不需要认证（公开）。  
**AC-F6-3**：此端点不经过 ReadyGate（服务启动中也可返回 200）。  
**AC-F6-4**：添加对应集成测试。

### F-7：auto-restore TOML 预检（OPT-8 / TD-7）

`cmd/frp-easy/main.go` 的 `autoRestoreProcs` 在 TOML 配置文件不存在时直接调用 `pm.Start(kind)`，导致子进程立即失败且错误信息模糊。

**AC-F7-1**：`autoRestoreProcs` 在 Start 前检查 `ConfigPaths[kind]` 文件是否存在。  
**AC-F7-2**：TOML 不存在时记录 warn 日志并跳过，不调用 Start。  
**AC-F7-3**：TOML 存在时行为不变。

---

## 验收标准汇总（测试可验证）

| ID | 条件 | 验证方法 |
|---|---|---|
| AC-F1-1 | verify_all.sh B.1 不再 SKIP | `bash scripts/verify_all.sh` B.1=PASS 或 FAIL（不是 SKIP） |
| AC-F1-3 | B.3 前端测试 PASS | verify_all.sh B.3=PASS |
| AC-F2-1 | 向导完成后访问 /wizard → 重定向 /dashboard | 前端 store wizard test |
| AC-F3-1 | slog 写 MultiWriter | main.go 代码审查 |
| AC-F4-1 | build.sh 读 git describe | build.sh 代码审查 |
| AC-F5-1 | fetchIPFromURL 调用 downloader.ParseIPFromJSON | 代码审查 + go test |
| AC-F6-1 | GET /health 返回 200 + JSON | 集成测试 |
| AC-F6-3 | /health 绕过 ReadyGate | 代码审查 |
| AC-F7-1 | autoRestoreProcs 预检 TOML | 代码审查 + 现有测试 |
| AC-VERIFY | verify_all FAIL: 0 | `bash scripts/verify_all.sh` |

---

## 范围外

- TD-8（SQLite 连接数）：单连接是 SQLite 的正确并发设计
- OPT-9（OpenAPI schema）：大范围专项，不含在本次
- OPT-3（dist/ git 追踪）：README 已文档化 Option B
