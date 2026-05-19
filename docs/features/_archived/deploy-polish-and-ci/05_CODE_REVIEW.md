# 05 — 代码评审：T-010 deploy-polish-and-ci

> Stage 5 of 7 · 派发独立 code-reviewer 子 agent · 中文

---

## 1. Verdict

**CHANGES REQUIRED** → 修后 **APPROVED FOR QA**（PM 接手在 04 §6 完成 rework，详见下）。

子 agent 给出：1 MAJOR，0 CRITICAL，多 MINOR / NIT。

## 2. 子 agent 原始报告（精简转载）

### MAJOR-1 — L5 .js 清理没真生效

`04_DEVELOPMENT.md §L5` 与 `scripts/baseline.json: frontend_tests: 57` 声称清掉了 29 个残留 `.js`，但子 agent 在 review 时 Glob `web/src/**/*.js` 看到全 29 个文件仍在。`verify_all PASS:18` 不抓这个 —— B.4 是占位的硬编码 PASS（`scripts/verify_all.sh:162-166`）。`.gitignore:24` 的 `web/src/**/*.js` 让 git status 看不到这些文件，掩盖了问题。

根因猜测（**已证实**）：(a) `find -delete` 真执行了；(b) 之后 `npm run build` 或 `vue-tsc` 把它们再 emit 回来。检查 `tsconfig.json` 没有 `"noEmit": true`。

要求三选一：
- 实删 + 加 verify_all 闸门防再生
- 改 04 文档撤回 L5 claim + 改 baseline.json 为 ~114（与 .js 共存）
- 任一 + 显式说明

### MINOR / NIT 汇总

| ID | 类 | 位置 | 内容 |
|---|---|---|---|
| M-2 | MINOR | `browseropen_test.go:93-118` | TestOpen_CommandSelection 只测当前主机 GOOS 分支 |
| M-3 | MINOR | `logrotate_test.go:34-63` | TestNew_RotatesOnSize 没断言轮转产物 perm 0o600（AC-3 显式要求） |
| M-4 | MINOR | `logrotate_test.go:78-79` | t.Setenv("", "") 不等于真正 unset；当前 != "" 检查偶然容忍 |
| M-5 | MINOR | `browseropen_test.go:64,72,86` | 同上 env-clearing 问题 |
| M-6 | MINOR | `logrotate.go:55-66` | New 自做 OpenFile+Close+Chmod 链路解释（已注释清楚，确认无 bug） |
| M-7 | MINOR | `main.go:268-270` | URL rewrite 不覆盖 `[::]` 等 IPv6 unspecified 写法 |
| M-8 | MINOR | `main.go:266-276` | browser open 在主 goroutine 同步跑，xdg-open shell fork 可能延迟（实际无感） |
| M-9 | MINOR | `logrotate.go:86,91` | MaxBackups=0 / MaxAgeDays=0 = lumberjack "无限保留"，未文档化 |
| N-1 | NIT | `browseropen.go:31-40` | 4 个 package-level mock var 不是 idiomatic Go |
| N-2 | NIT | `browseropen_test.go:41` | stub command 用不存在的 executable 名作占位 |
| N-3 | NIT | `release.yml:35` | `go-version: '1.22'` 与 `go.mod: go 1.25.0` 不对齐 |
| N-4 | NIT | `main.go:73` | `FRP_EASY_NO_BROWSER=0` 文案语义反直觉 |
| N-5 | NIT | `DEPLOYMENT.md:18` | "除上述两个占位符外" 略生硬 |

### 子 agent 的 latent risk 提示

1. lumberjack 升级可能误改 0o644 → 加 perm 断言守
2. flatpak/snap-confined Linux 上 xdg-open 可能存在但 broken（仅 UX 漏）
3. `FRP_EASY_LOG_MAX_BACKUPS=0` = unlimited 是脚陷
4. CI workflow 冷缓存 + package-lock.json drift 时 build.sh `--frozen-lockfile` 会失败
5. `Mode == 0` fallback 0o600 的零值语义
6. **L5 .js cleanup 没有 enforcement** → 需要 B.5 闸门或 pre-commit hook

## 3. PM Rework 响应

PM 接手做 rework（一 session 完成）：

| 反馈 | 响应 | 落点 |
|---|---|---|
| MAJOR-1 | **三层防御**：tsconfig.json 加 `"noEmit": true` + verify_all `npm exec -- tsc --noEmit` 修参数透传 + 新增 B.5 哨兵 + 实删 29 个 .js | 04 §6.1 |
| M-2 | 在 browseropen.go 加 `goosFunc` seam，重写 TestOpen_CommandSelection 为 table-driven 三平台 | 04 §6.3 |
| M-3 | TestNew_RotatesOnSize 加轮转产物 perm 0o600 断言 | 04 §6.2 |
| N-3 | workflow `go-version: '1.22'` → `'1.25'` | 04 §6.4 |
| M-4 / M-5 | 不动（当前 `!= ""` 实现已覆盖空串 & unset 等价；改 LookupEnv 是反向语义变更） | 04 §6.5 |
| M-6 / M-7 / M-8 / M-9 / N-1 / N-2 / N-4 / N-5 | 不动（已注释清楚 / 已知边界 / 风格选择 / 文档措辞） | — |

子 agent 的 latent risk #6 (L5 enforcement) 用 B.5 哨兵机器化执行，未来再犯当场 FAIL。

## 4. Rework 后再评（PM 自审）

- L5 复测：`find web/src -name '*.js' -not -name 'env.d.ts' | wc -l == 0`，verify_all 后立查仍 0（noEmit + B.5 双保险）
- MINOR perm 断言通过：`go test ./internal/logrotate -v` 4/4 PASS
- MINOR cross-platform mock：`go test ./internal/browseropen -v` 5/5 PASS（含 3 个 table-driven 子用例）+ 1 Linux-only SKIP
- 双 shell verify_all 19/19 PASS

## 5. 最终 verdict

**APPROVED FOR QA**。Rework 解决了所有阻塞项；剩余 MINOR/NIT 经 PM 评估均不阻塞（含理由）。
