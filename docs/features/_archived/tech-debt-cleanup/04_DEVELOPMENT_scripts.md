# Development Record — scripts 分区（T-004）

## Summary

修复了 verify_all.sh / verify_all.ps1 中前端质量门禁永久 SKIP 的问题（B.1-B.4 检测路径从根目录 package.json 改为 web/package.json，并用 pushd/Push-Location 将工作目录切换到 web/ 后再执行 npm 命令）。同时将 build.sh 和 build.ps1 的版本号从硬编码 "0.1.0" 改为通过 `git describe` 动态注入。

## Files changed

- `scripts/verify_all.sh`
  - 新增第 19 行：`ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"`（顶部 set 之后）
  - B 块守卫条件：`[[ ! -f package.json ]]` → `[[ ! -f web/package.json ]]`
  - else 块首行加 `pushd "$ROOT/web" >/dev/null`，末行加 `popd >/dev/null`
  - B.4 中 scripts/baseline.json 路径改为 `"$ROOT/scripts/baseline.json"`（pushd 后相对路径会失效，用绝对路径保证正确性）

- `scripts/verify_all.ps1`
  - B.1-B.4 守卫条件：`Test-Path "package.json"` → `Test-Path "web/package.json"`
  - B.1-B.3 scriptblock 内：`Push-Location (Join-Path $root "web")` + `try/finally { Pop-Location }` 包裹执行逻辑
  - B.4：`scripts/baseline.json` 路径改为 `Join-Path $root "scripts/baseline.json"` 以防止 Push-Location 后路径漂移
  - B.2 的 `$hasEslint` 检测现在在 web/ 目录下执行，自动找到 `web/.eslintrc.cjs`

- `scripts/build.sh`
  - 第 19 行：`VERSION="0.1.0"` → `VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")`

- `scripts/build.ps1`
  - 第 34-35 行：`$version = "0.1.0"` → 两行动态注入逻辑（git describe + 失败回退 "dev"）

## verify_all result

并发开发中，未在此分区运行 verify_all（依照任务说明：其他分区尚未完成）。

- Baseline：以最近 commit 1138694 结果为准
- After changes：脚本结构性变更，不影响 Go / Harness 检查，B 块从 SKIP 变为实际可执行

## Design drift (if any)

**DESIGN DRIFT（轻微）**：B.4 的 `scripts/baseline.json` 路径，设计文档未明确要求更新为绝对路径，但 pushd 后相对路径会指向 `web/scripts/baseline.json`（不存在），导致 B.4 结果错乱。实现中主动改用 `$ROOT/scripts/baseline.json` / `Join-Path $root "scripts/baseline.json"` 修正此问题，属于必要的防御性修复。

## Open issues for review

- `verify_all.ps1` 的 B.2 中 `return "SKIP"` 位于 `try/finally` 内部，PowerShell finally 块会在 return 前执行（保证 Pop-Location），行为正确，但审阅者可确认此语义。
- C.1 的 `pkgmgr` / `$pkgMgr` 仍在根目录下检测 lock 文件；playwright 目前不存在所以恒 SKIP，暂不处理。

## Dev-map updates

无新增文件，无结构变更。

## Verdict
READY FOR REVIEW
