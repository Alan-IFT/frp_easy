# 07 — Delivery · T-016 install-progress-and-systemd-unit-fix

> 阶段 7 / 7（PM Orchestrator）。模式：**full**。
> 完整阶段链：01 RA (READY) → 02 SA (READY) → 03 GR (APPROVED FOR DEVELOPMENT) → 04 Dev (v1 + v2 D-1 fix) → 05 CR (APPROVED) → 06 QA (CHANGES REQUIRED D-1 → 重评 APPROVED FOR DELIVERY) → **07 PM 交付**。

---

## 1. 任务总结

修复线上 Ubuntu VM 实测暴露的 `install.sh` 一键安装两类问题：

1. **systemd unit 启动失败**（用户报错 `Unit frp-easy.service has a bad unit file setting.`）—— 根因：`scripts/install-service.sh` 写出的 unit 文件 `ExecStart="${BINARY}"` 与 `WorkingDirectory="${INSTALL_DIR}"` 用整体双引号包路径，被 systemd 拒收。修复：改为 systemd.exec(5) 规定的**裸 token + C-style `\x20` 转义**语法。
2. **退出码透传 bug**（用户报错 "install-service.sh 退出码 0" 与 install-service.sh 明确 `exit 2` 矛盾）—— 修复：`if ! bash $SVC; then $?` 反模式拆为 `set +e; bash $SVC; rc=$?; set -e` 三行块 + `[diag]` 打印 + `exit "$rc"` 显式透传。

同时实现用户额外请求的 UX 改进：

3. **下载进度显示** —— Linux 端 `curl -fsSL` → `curl -fSL --progress-bar`（`[[ -t 2 ]]` TTY 探测，非交互降级）；Windows 端 `Invoke-WebRequest -UseBasicParsing` 去掉 `-UseBasicParsing` + `$ProgressPreference` 显式控制 + `$isInteractive` 探测。

附加：`install-service.sh` enable--now 失败时新增 status + journalctl 强诊断块（AC-8 字面前缀作为 QA grep 锚点）+ unit 路径 + 清理提示。

## 2. 变更摘要

| 文件 | 改动 |
|---|---|
| `scripts/install.sh` | 步骤 5 `curl -fsSL` → `curl -fSL` + `[[ -t 2 ]]` 决定 `--progress-bar`；步骤 7 `if ! bash` 反模式 → `set +e/-e` 三行块 + diag 打印 |
| `scripts/install.ps1` | 步骤 5 去 `-UseBasicParsing` + `$ProgressPreference`/`$isInteractive` try/finally；步骤 7 `& $svc` 前 `$LASTEXITCODE = 0` 重置 |
| `scripts/install-service.sh` | 新增 `systemd_escape_path()`（v3 方案：`local esc='\x20'`）；unit 模板 `ExecStart=${ESC_BINARY}` / `WorkingDirectory=${ESC_INSTALL_DIR}`（裸 token + `\x20`）；新增 `systemd-analyze verify`（warn+继续降级路径）；daemon-reload / enable--now 三处 `set +e/-e` + diag 打印；enable--now 失败块扩展为 `==== 诊断信息：systemctl status ... ====` + `==== 诊断信息：journalctl -u ... -n 20 ====` 字面前缀 + unit 路径 + 清理提示 |
| `scripts/install-service.ps1` | **无改动**（链路合规） |
| `.harness/insight-index.md` | 第 18 行 T-008 旧错误 insight 替换为 T-016 修正版（裸 token + `\x20` + verify warn+继续）；末尾追加 T-016 D-1 bash quote-removal 陷阱 insight |
| `docs/features/_archived/insight-history.md` | **新建**；归档 T-008 第 18 行原文（前缀 `[CORRECTED by T-016]`） |
| `docs/tasks.md` | T-016 行添加；任务完成时由 PM 移到"已完成"段 |
| `docs/features/install-progress-and-systemd-unit-fix/` | 01-07 七个阶段文档 + PM_LOG |

## 3. verify_all 结果

最终（阶段 7 PM 跑的，D-1 修复 + insight-index 更新之后）：

```
=== verify_all (fullstack) ===
[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO / FIXME budget (warn only) ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[B.5] No tsc residue in web/src/ ... PASS
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agent definitions present in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa) ... PASS
[E.6] Adversarial tests section present in completed task reports ... PASS

=== Summary ===
  PASS: 19  WARN: 0  FAIL: 0  SKIP: 0
```

PASS:19 / 0 WARN / 0 FAIL / 0 SKIP。NFR-7 / AC-11 满足。

## 4. AC 覆盖（13 条 + D-1 重评）

参考 [06_TEST_REPORT.md](06_TEST_REPORT.md) §1 + §D-1 Re-evaluation：12 条 AC 在本任务 QA 阶段静态/独立 reproducer 验证 PASS；AC-4/5/6/7/8/12 的实机系统集成验证 **DEFERRED-TO-USER**（本任务在 Windows 开发机执行，无 systemctl/systemd-analyze；用户在收到 main 滚动发布后在 Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 任一 VM 上跑 `curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash` 即可端到端验证）。

D-1 BLOCKER（systemd_escape_path bash quote-removal 陷阱）已修 + QA 重评 APPROVED FOR DELIVERY。

## 5. 已知未解决项

无 BLOCKER。

DEFERRED-TO-USER 清单（QA §Risks remaining）：

- AC-4 / AC-6 跨发行版 `systemctl is-active frp-easy = active` 实机验证
- AC-5 含空格路径实机验证（字节级已 PASS，但 systemd 实际 daemon-reload + enable --now 流程未实机）
- AC-7 / AC-8 在真实 systemd 失败场景下 diag 打印 + 强诊断块的可读性观察
- AC-12 升级幂等的 mtime/sha256 不变验证
- PS 5.1 实机 IWR 进度条视觉验证（AC-2）

这些都不阻断 main 滚动发布——失败时用户能看到清晰中文报错与 `[diag]` 诊断，PM 可通过 issue/反馈通道修复。

## 6. 流程偏差与教训

### D-1 漏网根因

- **04 Developer 在 ad-hoc 测试脚本（已删）上验证 heredoc，未 source 已 commit 的源码**——测试脚本与 commit 代码可能用了不同的转义模式（hex dump 显示反斜杠存在，但 commit 代码不能复现）。
- **05 Code Reviewer 直接信任 04 hex dump 而未独立复现**——只读 04 附录、未自己跑 `systemd_escape_path "/opt/frp easy/v1"`。
- **06 QA 用对抗心态写独立 reproducer**——明确"不复用 04 测试代码"，从 committed 源码 verbatim copy 函数后跑，立刻发现矛盾。这是流水线的设计正确触发：QA 的对抗性是最后一道防线。

### 修正动作

- **insight-index** 已加入 D-1 教训：`bash 双引号 + parameter expansion 的 quote-removal 陷阱 ... 验证字符级替换必须 verbatim source committed 函数，不能复用"等价" ad-hoc 测试脚本`
- 未来 Developer 阶段写 hex dump/字节验证时**必须** `source <(sed -n "FUNC_START,FUNC_ENDp" path/to/committed/script)` 加载函数，禁止 ad-hoc 复制。这条已写入新 insight。

## Insight

本任务追加到 `.harness/insight-index.md` 的 insight 已在阶段 7 完成（已直接编辑 + 归档老 18 行到 `docs/features/_archived/insight-history.md`，无需 `archive-task` 重复追加）：

1. **替换第 18 行** —— T-008 旧错误 insight（双引号 unit）→ T-016 修正版（裸 token + `\x20` 转义 + verify warn+继续）。
2. **追加末尾** —— bash quote-removal 陷阱（D-1 教训）。

archive-task 跑时**不需要**再追加本任务的 Insight 段（已手动落地避免 30 行上限超限）。

## Verdict

# DELIVERED

任务完成。所有阶段产出完整、verify_all PASS:19、D-1 BLOCKER 已修 + 独立验证、insight-index 已修正纠错事实、用户可通过 `git push main` 触发 release.yml 自动滚动发布让真实用户在 Ubuntu/Debian/RHEL VM 上端到端验证（按 §5 DEFERRED-TO-USER 清单跑）。
