# Delivery — T-001 · web-ui-mvp

> 日期：2026-05-16 · 最终 commit：689958d · verify_all：12 PASS / 0 FAIL / 6 SKIP（预期）

## 交付摘要

T-001 web-ui-mvp 已完成全部 7 阶段 Harness 流程，APPROVED FOR DELIVERY。

| 阶段 | 文档 | 结果 |
|---|---|---|
| S1 需求分析 | 01_REQUIREMENTS.md | DONE |
| S2 方案设计 | 02_SOLUTION_DESIGN.md | DONE |
| S3 闸门审查 | 03_GATE_REVIEW.md | APPROVED |
| S4 开发（后端+前端+embed） | 04_DEVELOPMENT_*.md | DONE（commits b7531be, 8f0324c） |
| S5 代码评审 | 05_CODE_REVIEW.md | APPROVED FOR QA（commit 99f7d44 修复 3 CRITICAL） |
| S6 QA 测试 | 06_TEST_REPORT.md | APPROVED FOR DELIVERY（commit 689958d） |
| S7 交付 | 本文档 | ✅ |

## 功能清单（已实现）

### 核心功能
- **setup 向导**：首次启动 → `/setup`，argon2id 密码哈希，session 自动登录
- **frpc/frps 进程控制**：启动 / 停止 / 重启，状态轮询（2s），PID 显示
- **模式自动启动**：NSwitch 切换，`mode.{kind}.enabled` KV 持久化，重启后自动恢复（AC-9）
- **frpc 客户端配置**：serverAddr / serverPort / authMethod / authToken
- **frps 服务端配置**：bindPort / authToken / dashboard，token 脱敏
- **代理规则管理**：tcp/udp/http/https，增删改查，200 条上限，乐观锁
- **DB→TOML 管道**：配置变更即时写 frpc.toml / frps.toml，atomic write，ApplyConfigChange reload
- **日志查看**：tail 500 行 + 2s 增量轮询，frpc / frps 独立页
- **认证与安全**：session cookie，CSRF token（X-CSRF-Token），5 次失败 429 + Retry-After，argon2id

### 技术架构
- 单 Go 二进制（`CGO_ENABLED=0`）+ `embed.FS` 内嵌 Vue 3 SPA
- SQLite（modernc.org/sqlite，纯 Go）
- go-chi/chi v5 路由 + ReadyGate 中间件（503 + Retry-After: 2）
- Vue 3 + Vite + TypeScript + Pinia + Naive UI
- SPA history mode fallback（未知路径 → dist/index.html）

## 测试覆盖

| 类型 | 数量 |
|---|---|
| Go httpapi integration | 32 |
| Go 全包 | 101 |
| Vitest 前端 | 45 |
| **总计** | **146** |

**对抗测试覆盖**：SQL 注入（6 pattern）、并发写入竞争、超长输入（65 字符名称）、不合法 JSON。

## 已知限制（非 BLOCKER）

| 编号 | 描述 | 影响 |
|---|---|---|
| W1 | AC-7/AC-8 需真实 frp 二进制，CI 无法自动验证 | 手动测试覆盖 |
| W2 | B.1-B.4、C.1 verify_all 项 SKIP（package.json 在 web/ 子目录）| verify_all 配置问题，不影响实际测试 |
| W3 | frpc admin reload（热重载）当 frpc 未运行时静默忽略 | 符合设计 |
| W4 | `/setup` → SPA 返回 200，curl 无法验证 302 重定向 | SPA 架构决策 |

## 产物

```
cmd/frp-easy/             — 主程序入口
internal/
  appconf/                — 配置文件加载
  assets/                 — embed.FS + SPA fallback
  auth/                   — argon2id + session + CSRF + ratelimit
  binloc/                 — FRP 二进制定位
  frpcadmin/              — frpc admin API 客户端（热重载）
  frpconf/                — TOML 渲染 + AtomicWrite
  httpapi/                — 所有 API handler + router + config_helper
  logtail/                — 日志读取
  procmgr/                — 进程生命周期管理
  storage/                — SQLite store（KV + proxies + users）
migrations/               — SQL 迁移文件
web/src/                  — Vue 3 SPA（pages / stores / api / components）
internal/assets/dist/     — 构建产物（embed 进二进制）
```

## 交付确认

- [x] verify_all: 12 PASS / 0 FAIL
- [x] go test ./...: 全部通过
- [x] npm run build: 成功（dist/ 已嵌入）
- [x] 所有 14 个 AC 有测试用例（AC-7/AC-8 需真实进程，已标注）
- [x] 代码已提交至 main 分支
