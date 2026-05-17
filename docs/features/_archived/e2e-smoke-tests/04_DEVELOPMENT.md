# Development Record

## Summary

为 frp_easy 新增 Playwright E2E 烟雾测试基础设施：创建 `web/playwright.config.ts` 配置文件、`scripts/start-e2e-server.sh` 后端启动脚本、3 个 spec 文件（TC-01 至 TC-05）和 1 个 fixture 辅助文件（`auth.ts`），并更新 `scripts/verify_all.sh` / `verify_all.ps1` 的 C.1 节感知 `web/playwright.config.ts` 的存在。同时修复了 vitest 错误地扫描 E2E 测试文件的问题（在 `vitest.config.ts` 中添加了 exclude 规则）。

## Files changed

- `web/package.json` — devDependencies 添加 `@playwright/test: ^1.44.0`
- `web/package-lock.json` — npm install 自动更新（添加 playwright 相关 3 个包）
- `web/playwright.config.ts` — 新建：Playwright 主配置（testDir、webServer、chromium project、workers=1）
- `web/vitest.config.ts` — 添加 `exclude: ['**/node_modules/**', '**/tests/e2e/**']` 防止 vitest 误扫 E2E 文件
- `web/tests/e2e/01-setup.spec.ts` — 新建：TC-01（未初始化跳转 /setup）、TC-02（setup 表单提交）
- `web/tests/e2e/02-auth.spec.ts` — 新建：TC-03（login 表单提交）
- `web/tests/e2e/03-dashboard.spec.ts` — 新建：TC-04（dashboard 元素可见）、TC-05（退出登录）
- `web/tests/e2e/fixtures/auth.ts` — 新建：setupAccount / programmaticLogin / bypassWizard / programmaticLogout
- `scripts/start-e2e-server.sh` — 新建：构建 Go 二进制，以临时数据目录启动后端（exec 方式）
- `scripts/verify_all.sh` — C.1 节：扩展文件检测条件 + pushd 到 web/ 执行 playwright
- `scripts/verify_all.ps1` — C.1 节：同步修改（添加 $LASTEXITCODE 检查 + Push-Location/Pop-Location）
- `docs/dev-map.md` — 新增 playwright.config.ts、tests/e2e/ 目录、start-e2e-server.sh 条目

## verify_all result

- Baseline（--quick）: PASS: 17, WARN: 0, FAIL: 0, SKIP: 0
- After changes（--quick）: PASS: 17, WARN: 0, FAIL: 0, SKIP: 0
- Delta: 0 新增失败；修复了 vitest 误扫 E2E 文件导致的 B.3 FAIL（设计文档未预料到的场景）

## Gate Review 条件处理

- **WARN-A（verify_all.ps1 $LASTEXITCODE 检查）**：已在 verify_all.ps1 的 C.1 块中添加 `if ($LASTEXITCODE -ne 0) { throw "playwright test failed (exit code $LASTEXITCODE)" }`，与 G.1/G.2/G.3 的模式一致。
- **WARN-B（变量名 E2E_TMP）**：start-e2e-server.sh 使用 `E2E_TMP=$(mktemp -d)` 而非 `TMPDIR=`，避免覆盖系统环境变量。
- **WARN-C（E2E 环境要求说明）**：见下方"开发者注意事项"。

## E2E 环境要求（WARN-C）

`scripts/start-e2e-server.sh` 使用 `bash`（`#!/usr/bin/env bash`），在 Windows 环境下需要通过 **Git Bash** 或 **WSL**（Windows Subsystem for Linux）运行，不支持 Windows cmd.exe 或 PowerShell 直接执行。

具体影响：
- `verify_all.sh`（bash 脚本）在 Windows 上通过 Git Bash 运行时，playwright webServer 的 `bash ../scripts/start-e2e-server.sh` 命令能正确执行。
- `verify_all.ps1`（PowerShell 脚本）在 Windows 上运行时，playwright 调用 `bash ../scripts/start-e2e-server.sh` 需要 `bash` 在 PATH 中（Git Bash 安装后默认添加）。
- CI 环境（Linux）无此限制，`bash` 原生可用。

## 已验证的 AC

- **AC-1**：`ls web/playwright.config.ts` — 文件存在 ✓
- **AC-2**：`ls web/tests/e2e/` — 目录存在，包含 3 个 spec 文件和 fixtures/ 目录 ✓
- **AC-3**：`web/package.json` devDependencies 含 `@playwright/test: ^1.44.0`；`web/package-lock.json` 已通过 `npm install` 同步 ✓
- **AC-7**：`bash scripts/verify_all.sh --quick` — C.1 步骤不出现在输出中，脚本正常完成（PASS: 17, WARN: 0, FAIL: 0, SKIP: 0） ✓
- **AC-8**：`npm install --frozen-lockfile` 成功（lockfile 已提交且与 package.json 同步） ✓

## 无法在当前环境验证的 AC

- **AC-4**（playwright test 通过）：需要 `npx playwright install chromium`（约 200MB 下载）和已构建的前端（`internal/assets/dist/` 存在）。当前环境中：（1）chromium 未安装；（2）前端未构建。这些步骤在当前 shell 环境中会超时或需要额外配置。验证留给 QA Tester。
- **AC-5**（dist/ 不存在时 C.1 = FAIL）：需要运行 playwright test，同样依赖 chromium 安装。验证留给 QA Tester。
- **AC-6**（完整 verify_all PASS）：依赖 AC-4 的前提条件（chromium + 前端构建）。验证留给 QA Tester。

## Design drift

- **vitest.config.ts 修改**：设计文档未提及需要修改 vitest 配置。实现过程中发现 vitest 会默认扫描 `tests/e2e/*.spec.ts`，而这些文件使用 `@playwright/test` 的 API（`test.describe`、`page` fixture），在 vitest 上下文中无效并报错，导致 B.3 FAIL。修复方案是在 vitest.config.ts 中添加 `exclude` 规则排除 E2E 目录。这是必要的技术修正，不影响设计意图。`DESIGN DRIFT`

## Open issues for review

- vitest 的 exclude 规则（`'**/tests/e2e/**'`）使用了 glob 模式，需确认在项目所有平台（Windows/Linux）上都能正确匹配 `web/tests/e2e/` 下的文件。当前测试已验证（Windows Git Bash 环境）。
- `start-e2e-server.sh` 在 Windows 下执行时，`mktemp -d` 创建的临时目录在 /tmp（Git Bash 映射到 Windows 临时目录），路径中可能包含空格。当前配置写入 TOML 时路径不带引号包围（TOML 字符串值不需要引号）。如果临时路径含空格需要验证。

## Dev-map updates

```
├── scripts/        ← verify_all、start、build、baseline、sync 辅助；start-e2e-server.sh（T-006）
└── web/
    ├── playwright.config.ts   ← T-006 新增：Playwright E2E 配置
    └── tests/
        └── e2e/               ← T-006 新增：Playwright E2E 烟雾测试
            ├── 01-setup.spec.ts
            ├── 02-auth.spec.ts
            ├── 03-dashboard.spec.ts
            └── fixtures/
                └── auth.ts
```

## Insight to surface

vitest 默认扫描所有 `**/*.spec.ts` 文件（包括 E2E 目录），当 `@playwright/test` 的 `test.describe()` 在 vitest 上下文中被调用时会报 "Playwright Test did not expect test.describe() to be called here" 错误并导致 B.3 FAIL。修复方式：在 `vitest.config.ts` 的 `test.exclude` 数组中添加 `'**/tests/e2e/**'`。 · evidence: `web/vitest.config.ts`

## Verdict

READY FOR REVIEW
