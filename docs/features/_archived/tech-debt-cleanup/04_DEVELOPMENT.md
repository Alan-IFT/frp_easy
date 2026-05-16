# 开发报告 — T-004 tech-debt-cleanup

**任务 ID**：T-004  
**日期**：2026-05-16  
**分区**：scripts + frontend + backend

---

## 完成状态

全部 7 项实施完毕，verify_all PASS:16 FAIL:0。

## 额外修复

在修复 verify_all 前端路径（OPT-1）后，B.1 typecheck 首次真正运行，暴露出**预存在的 TypeScript Vue SFC 类型声明缺失**问题（从未被发现是因为 tsc 之前从未执行）。新增 `web/src/env.d.ts`，添加 `/// <reference types="vite/client" />` 和 `declare module '*.vue'`，修复后 typecheck PASS。

## DESIGN DRIFT

**D-1（轻微）**：`scripts/verify_all.sh` 中 B.4 的 `scripts/baseline.json` 路径改为 `"$ROOT/scripts/baseline.json"`（pushd 后相对路径失效需用绝对路径）。不影响功能，已记录。

## 文件改动汇总

| 文件 | 改动 |
|---|---|
| `scripts/verify_all.sh` | B.1-B.4 改为检查 web/package.json，pushd/popd web/；ROOT 变量 |
| `scripts/verify_all.ps1` | B.1-B.4 同上（Push-Location/Pop-Location web/） |
| `scripts/build.sh` | VERSION 从 git describe 注入，fallback "dev" |
| `scripts/build.ps1` | 同上 |
| `web/src/router.ts` | 向导路由守卫补全（直访 /wizard 且已完成 → /dashboard） |
| `web/src/env.d.ts` | **新建**：Vue SFC 类型声明（`*.vue` module + vite/client ref） |
| `cmd/frp-easy/main.go` | slog 双写（io.MultiWriter）；autoRestoreProcs TOML 预检 |
| `internal/httpapi/router.go` | health 端点单独挂顶层（绕过 ReadyGate），其余路由入 Group |
| `internal/httpapi/handlers_system.go` | fetchIPFromURL 改用 downloader.ParseIPFromJSON；health handler |
| `internal/httpapi/qa_ac_test.go` | TestHealth_ReturnsOK + TestHealth_BypassesReadyGate |
| `docs/dev-map.md` | 补录 T-003 新增的 README.md 和 docs/project-status.html |

## verify_all 对比

| | T-003 结束 | T-004 结束 |
|---|---|---|
| PASS | 12 | **16** |
| WARN | 0 | 0 |
| FAIL | 0 | 0 |
| SKIP | 6 | **2** |

新增 PASS：B.1 typecheck、B.2 lint、B.3 frontend tests、B.4 test count（4 项前端检查从 SKIP → PASS）。
