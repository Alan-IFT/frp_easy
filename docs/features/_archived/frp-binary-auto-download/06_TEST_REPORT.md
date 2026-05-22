# 06 · 测试报告 — T-014 · frp-binary-auto-download

> Harness 流水线 stage 6 产出 · 编写：qa-tester · 日期：2026-05-22 · PM 自治模式
> 上游（只读）：01（READY）/02（READY）/03（APPROVED）/04（无 DESIGN DRIFT，含 M-1/M-2 修复）/05（APPROVED · MINOR 2 已修）
> 改动未 commit。本任务 13 条完成闸门 AC（AC-1~AC-13）+ 3 条交付后人工验证 MV（MV-1~MV-3，非闸门）。

## Verdict

**PASS** — 13 条完成闸门 AC 全部通过；verify_all PASS 19/0/0/0；对抗性测试实现全部挺住；无 BLOCKER/CRITICAL/MAJOR 缺陷。

---

## Test plan（AC → 测试 / 验证证据）

| AC | 验证方法 | 文件 / 证据 | 结果 |
|---|---|---|---|
| AC-1 git 不再跟踪 4 个 frp 可执行文件 | `git status --short` | `frp_linux/{frpc,frps}`、`frp_win/{frpc.exe,frps.exe}` 均显示 `D`（staged 删除），另含 4 个 toml | PASS |
| AC-2 `frp_linux/`、`frp_win/` 目录仍存在 | `git ls-files frp_linux/ frp_win/` | 输出恰为 `frp_linux/LICENSE`、`frp_win/LICENSE` 各 1 个被跟踪文件 | PASS |
| AC-3 package 无 frp 二进制打包逻辑 | grep `package.sh`/`package.ps1` | `frpc/frps/frp_linux/frp_win` 仅出现在注释中，无前置检查块、无整目录 `cp`/`Copy-Item` | PASS |
| AC-4 package 无 frp 二进制时打包成功 | 实跑 `bash scripts/package.sh --linux --skip-build` | exit 0；产物 tar 7 文件，`tar tzf ... \| grep -c frp_linux/frpc` = 0 | PASS |
| AC-5 downloader 不用写死版本号构造 URL | grep `internal/` | 无 `FRPVersion` 常量（仅 `downloader.go:128` 注释提及历史名）；URL 来自 `resolveLatestAsset` | PASS |
| AC-6 downloader 测试覆盖 latest 解析 4 分支 | `go test ./internal/downloader/...` | `TestResolveLatest_Success/RateLimited403/AssetNotMatched/NetworkFailure` 全绿 | PASS |
| AC-7 binloc 既有测试不失败 | `go test ./internal/binloc/...` | ok（binloc 未改，临时目录自造假二进制） | PASS |
| AC-8 NOTICE 去「随附」含「运行时下载」+「Apache-2.0」 | `grep -c 随附\|vendored NOTICE` | = 0；含 `fatedier/frp` + `Apache License 2.0` + 「运行时下载」 | PASS |
| AC-9 install.sh 成功输出含「更新」小节 | grep `scripts/install.sh` | L296 `更新：` + L299「升级会保留你的配置（frp_easy.toml）与数据（.frp_easy/）」 | PASS |
| AC-10 install.ps1 成功输出含「更新」小节 | grep `scripts/install.ps1` | L252 `更新：` + L255 等效内容（Windows 路径形态） | PASS |
| AC-11 README/DEPLOYMENT/dev-map 已更新 | grep 三文件 | README L69「如何更新」、L212-213 目录树注释；dev-map L31-32「运行时下载落地目录」；DEPLOYMENT「如何更新」段 | PASS |
| AC-12 install.sh/.ps1 语法正确 | `bash -n` + verify_all B.2 lint | install.sh syntax OK；install.ps1 经 verify_all PASS | PASS |
| AC-13 verify_all 全绿 pass_count ≥ 19 | `pwsh -File scripts/verify_all.ps1` | PASS 19 / WARN 0 / FAIL 0 / SKIP 0 | PASS |

**MV-1/MV-2/MV-3**（真实联网下载、版本一致性、离线启动）为交付后人工验证项，非完成闸门，不在本报告范围。

---

## Boundary tests added（QA 新增边界测试）

QA 独立审查 `02_SOLUTION_DESIGN.md §3.4.5` 列出的全部降级分支后，发现开发者新增的 4 个测试覆盖了「成功 / 403 / 资产未匹配 / 网络失败」，但**遗漏了 §3.4.5 表格中的另外两类降级**：「HTTP 200 但响应体非法 JSON」「200 + 合法 JSON 但缺 tag_name」；另补「HTTP 500 等其它非 200 状态码」。QA 在 `internal/downloader/downloader_adversarial_test.go`（新建）补 3 个测试：

- `TestAdversarial_ResolveLatest_MalformedJSON` — HTTP 200 响应体非法 JSON → `failed` + 「解析」消息，不 panic。
- `TestAdversarial_ResolveLatest_MissingTagName` — 200 + 合法 JSON 但缺 `tag_name` → `failed` + 「版本号」消息。
- `TestAdversarial_ResolveLatest_HTTP500` — HTTP 500 → `failed` + 「HTTP 500」消息。

全程 `httptest`，无真实网络（沿用 C-4 约束）。Go 测试函数 171 → 174（红线 3 只升不降）。

---

## Adversarial tests（对抗性测试 · 每条 AC 一个独立反证假设）

> 三铁律：① 无工具证据不下结论；② 用独立 reproducer 而非开发者的测试；③ 每条 AC 先写下预测失败假设再跑。verdict 基于「实现是否挺过本测试」。

| AC | 失败假设（"我预期失败当…"） | 独立 reproducer（QA 编写） | 结果（含工具输出） |
|---|---|---|---|
| AC-1 | git rm 只是物理删文件、索引仍跟踪 | `git status --short` | 挺住 — `frp_linux/frpc` 等 8 文件首列为 `D`（staged 删除），非 `??`/未跟踪 |
| AC-2 | `.gitignore` 通配会误伤保留的 LICENSE，致 git 不跟踪 | `git check-ignore -v frp_linux/LICENSE frp_win/LICENSE` | 挺住 — 两者 `exit=1`（未被忽略）；而 `frp_linux/frpc` 命中 `.gitignore:61` 根锚定规则、`frp_win/frpc.exe` 命中 `:63`，精确锚定不误伤 |
| AC-3 | package.ps1（Gate F-1）漏改、仍含 frp 打包 | `grep -n 'frp_linux\|frp_win\|frpc\|frps' scripts/package.ps1` | 挺住 — 4 处命中全为注释（L13/83/92/187），无 `Copy-Item` frp 目录、无前置检查 `foreach` 块 |
| AC-4 | 仓库无 frp 二进制时 package.sh 仍 `exit 1`（前置检查未删干净） | 实跑 `bash scripts/package.sh --linux --skip-build`（当前工作区 frp_linux/ 仅 LICENSE） | 挺住 — `package.sh exit=0`，产物 `frp-easy-...-linux-amd64.tar.gz`；`tar tzf` 7 文件、`grep -c frp_linux/frpc\|frp_win\|frps$` = **0** |
| AC-5 | URL 仍由写死版本号拼接 | `grep -n 'FRPVersion\|resolveLatestAsset' downloader.go` | 挺住 — 无 `FRPVersion` 常量声明（L128 仅注释）；`doDownload` 调 `resolveLatestAsset(goos)` 取 `browser_download_url` |
| AC-6 | resolveLatestAsset 先解析 JSON 后判状态码（403 体也是合法 JSON，顺序错则限流误判为数据异常） | 实读 `downloader.go:336-352`：`switch resp.StatusCode` 在 L336、`json.Unmarshal` 在 L350 | 挺住 — 状态码 switch 严格在 JSON 解析之前；403 在 case 分支直接 return，根本不执行 Unmarshal |
| AC-6+ | §3.4.5 列的「非法 JSON / 缺 tag_name / HTTP 500」降级分支无测试覆盖、可能 panic 或误进资产未匹配分支 | **QA 新建** `go test ./internal/downloader -run TestAdversarial -v` | 挺住 — 3/3 PASS（见下方输出）；非法 JSON 进「解析失败」、缺 tag 进「版本号」、500 进「HTTP 500」，均 `failed` 不 panic |
| AC-7 | 移除内置二进制后 binloc 测试因找不到文件 FAIL | `go test ./internal/binloc/...` | 挺住 — `ok`（binloc 用 `t.TempDir()` 自造假二进制，不依赖仓库内置文件） |
| AC-8 | NOTICE 仍残留「随附」/「vendored」旧表述 | `grep -c '随附\|vendored' NOTICE` | 挺住 — 计数 = 0；标题与正文均「运行时下载」，含 fatedier/frp + Apache-2.0 |
| AC-9/10 | install 升级路径仍 `rm -rf`/删 `frp_linux/`/`frp_win/`，每次升级清掉用户下载的 frpc/frps（OQ-4 关键正确性） | 全文 grep `scripts/install.sh` 的 `rm -rf`/`cp`、`install.ps1` 的 `Remove-Item`/`Copy-Item` | 挺住 — install.sh 升级分支白名单只覆盖 `frp-easy`/`scripts/`/README/VERSION/LICENSE/toml.example，`rm -rf` 仅作用于 `$TMP_DIR`、`$INSTALL_DIR/scripts`；**无任何对 frp_linux/frp_win 的 rm/cp**。install.ps1 升级 `foreach` 仅含 `scripts`。证伪「升级会清掉已下载二进制」成功 |
| AC-11 | M-2 修复不彻底，DEPLOYMENT C.2.4/C.3.4 仍 `tar xzf ... frp_linux`/`Copy-Item ... frp_win`（对不存在成员报错） | `grep -n 'frp_linux\|frp_win' docs/DEPLOYMENT.md` | 挺住 — C.2.4(L346)/C.3.4(L422) 仅解压/拷贝主二进制；剩余引用为 A.0 升级保留说明与 F.5 离线手动放置说明（`<INSTALL_DIR>/frp_linux/`，运行时目录非发布包成员，按边界 4.4 保留，正确） |
| AC-12 | install 脚本改动引入语法错误 | `bash -n scripts/install.sh` + verify_all B.2 | 挺住 — `install.sh syntax OK`；install.ps1 经 verify_all PASS |
| AC-13 | 移除内置 frp 二进制致 C.1 Playwright e2e FAIL（R-1） | `pwsh -File scripts/verify_all.ps1` | 挺住 — `[C.1] E2E smoke (playwright) ... PASS`；总计 PASS 19 / FAIL 0 / WARN 0 |
| M-1 | `Manager.baseURL` 死字段未删 / 删后 go build 失败 | `go build ./...` + 实读 `downloader.go:50-60` | 挺住 — `BUILD OK exit=0`；Manager 结构体只剩 `apiBaseURL`、`goos`，无 `baseURL` |

### 工具输出证据

**AC-1 / AC-2（git 移除 + gitignore 不误伤）**：
```
git status --short → D  frp_linux/frpc / frp_linux/frps / frp_win/frpc.exe / frp_win/frps.exe (+4 toml)
git ls-files frp_linux/ frp_win/ → frp_linux/LICENSE  frp_win/LICENSE
git check-ignore frp_linux/LICENSE → exit=1（未被忽略）；frp_win/LICENSE → exit=1
git check-ignore frp_linux/frpc → .gitignore:61:/frp_linux/frpc（命中，精确根锚定）
```

**AC-4（无 frp 二进制打包成功）**：
```
package.sh exit=0
tar tzf frp-easy-...-linux-amd64.tar.gz → 7 文件（frp-easy / frp_easy.toml.example / LICENSE / README.txt / scripts/{install,uninstall}-service.sh / VERSION）
tar tzf ... | grep -c 'frp_linux/frpc|frp_win|frps$' → 0
```

**AC-6+（QA 对抗性边界测试）**：
```
=== RUN   TestAdversarial_ResolveLatest_MalformedJSON
--- PASS: TestAdversarial_ResolveLatest_MalformedJSON (0.01s)
=== RUN   TestAdversarial_ResolveLatest_MissingTagName
--- PASS: TestAdversarial_ResolveLatest_MissingTagName (0.01s)
=== RUN   TestAdversarial_ResolveLatest_HTTP500
--- PASS: TestAdversarial_ResolveLatest_HTTP500 (0.01s)
PASS  ok  github.com/frp-easy/frp-easy/internal/downloader  0.823s
```

**AC-9/10（install 升级路径不删 frp 目录）**：
```
install.sh rm -rf 全文 → 仅 L190 "$TMP_DIR"、L231 "$INSTALL_DIR/scripts"
install.ps1 Remove-Item 全文 → 仅 L189 升级分支内 scripts $dst、L220 $tmpDir
install.sh/.ps1 中对 frp_linux/frp_win 的引用全部为注释或 banner 文案，无 rm/cp/Remove-Item/Copy-Item
```

**M-1（baseURL 死字段已删 + build 通过）**：
```
go build ./... → BUILD OK exit=0
downloader.go Manager 结构体字段：apiBaseURL string / goos string（无 baseURL）
```

---

## verify_all result

- Total tests（go func Test）：171 → **174**（QA 新增 3 个对抗性边界测试）
- Pass: **19**
- Fail: **0**
- Warn: **0**（无新增 WARN，AC-13 满足）
- Skip: 0
- New tests added: 3（`downloader_adversarial_test.go`）
- Baseline updated: **yes** — `go_tests` 171→174、`test_count` 228→231、`passing_count` 223→226（红线 3 只升不降）
- 特别确认：`[C.1] E2E smoke (playwright) ... PASS` —— 移除内置 frp 二进制不影响 e2e（R-1 验证通过）

---

## baseline.json 核对

| 字段 | dev 阶段（04） | QA 实测 | QA 更新后 | 一致性 |
|---|---|---|---|---|
| go_tests | 171 | `grep -rhn '^func Test'` = 174（含 QA +3） | 174 | ✅ 只升 |
| test_count | 228 | 174 go + 57 frontend = 231 | 231 | ✅ 只升 |
| passing_count | 223 | 226（全绿） | 226 | ✅ 只升 |
| frontend_tests | 57 | 57（未改前端） | 57 | ✅ 不变 |

> dev 阶段 04 文档声明的 `go_tests=171` 经 QA 实跑 `grep '^func Test'` 核对一致（QA 介入前为 171）。QA 新增 3 个对抗性测试后升至 174，baseline 同步上调。

---

## Defects found

无。BLOCKER 0 / CRITICAL 0 / MAJOR 0 / MINOR 0。

> 说明：QA 发现开发者新增测试遗漏了 `02 §3.4.5` 的 3 类降级分支（非法 JSON / 缺 tag / HTTP 500）。这不构成缺陷——实现本身（`downloader.go:350-355` + `:341-342`）已正确处理这些分支，仅是测试覆盖不足。QA 已按角色契约「补测试，不退回」直接补齐 3 个测试，实现全部挺住，无需路由回 Developer。

---

## Stability

- `go test ./internal/downloader/... -count=10`（成功 / 403 / 资产未匹配 / 网络失败 / 3 个 QA 对抗性 / 既有下载测试）：10 次连跑全绿，`ok ... 2.783s`，无 flake。✅
- `verify_all` 跑 2 次（baseline 更新前后）均 PASS 19/0/0/0，结果一致。✅

---

## 对抗性验证发现汇总

1. **OQ-4 升级路径**（关键正确性）：证伪「升级会清掉用户下载的 frpc/frps」成功 —— install.sh/install.ps1 升级分支白名单已彻底不含 frp_linux/frp_win，无任何 rm/cp 作用其上。
2. **gitignore 不误伤 LICENSE**：根锚定 `/frp_linux/frpc` 等 4 条精确规则，`git check-ignore` 推演证实 `LICENSE` 不被忽略、AC-2 不会因此失败。
3. **resolveLatestAsset 状态码先于 JSON**：实读确认 `switch resp.StatusCode`(L336) 严在 `json.Unmarshal`(L350) 之前，403 限流不会被误判。
4. **测试覆盖缺口已补齐**：开发者 4 测试漏掉 §3.4.5 的 3 类降级分支，QA 补 3 个独立测试，实现全部挺住。
5. **package.ps1（Gate F-1）已同步**：grep 确认 Windows 打包脚本无 frp 二进制残留逻辑。
6. **C.1 e2e 不受影响**（R-1）：移除内置 frp 二进制后 Playwright e2e 仍 PASS。
7. **M-1/M-2 修复有效**：`Manager.baseURL` 死字段已删且 `go build` 通过；DEPLOYMENT C.2.4/C.3.4 不再引用发布包内已移除目录。

---

## Verdict

**PASS** — APPROVED FOR DELIVERY（0 defects）。

13 条完成闸门 AC 全部通过；verify_all PASS 19/0/0/0、无新增 WARN/FAIL；C.1 Playwright e2e 不因移除内置 frp 二进制 FAIL（R-1 确认）；baseline 由 171→174 只升；OQ-4 升级保留语义经对抗性证伪验证成立。MV-1/MV-2/MV-3（真实联网下载、版本一致性、离线启动）为交付后人工验证项，需在 `07_DELIVERY.md` 交付时由人工执行。R-2（frp latest 版本不受控、TOML schema 兼容性）须按 04/05 要求写入 `07_DELIVERY.md` 的 `## Insight` 段。
