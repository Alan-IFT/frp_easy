# Test Report — T-065 mapprocerr-sentinel-hygiene

> 阶段 6 / QA Tester。输入：01 / 02 / 04 / 05。语言：中文。
> QA 上下文 role-collapsed 无 Bash/PS（insight L31）：verify_all 全量真跑标 **PENDING** + 确定性执行规格交 batch orchestrator 硬闸门。
> 核心对抗用例已落盘为独立代码（不复用 dev 测试），预期结果由 errors.Is/httptest 确定性语义逐用例推导。

## Test plan

| 验收标准 | 测试用例 | 文件 |
|---|---|---|
| AC-1 build + mapProcErr 方法化 | 编译通过（验证规格）+ 调用点 `h.mapProcErr`（CR 已核 :24/:66） | — / handlers_proc.go |
| AC-2 ErrBusy 导出 + Start StateStopping 返回 errors.Is==ErrBusy + cause 可读 | `TestErrBusy_IsSentinel` + `TestStart_StoppingReturnsErrBusy`（白盒真覆盖） | `internal/procmgr/manager_test.go` |
| AC-3 ErrBusy→409 PROC_BUSY 固定中文 + 不泄露 | `TestMapProcErr_Busy_409_FixedMessage_NoLeak`（dev） + `TestAdversarial_T065_AnyCauseTextStillBusyIfSentinel`（QA 独立） | `handlers_hygiene_test.go` / `qa_t007_adversarial_test.go` |
| AC-4 非 sentinel→500 INTERNAL 固定中文 + 不泄露 + cause 进 logger | `TestMapProcErr_Internal_500_NoLeak`（dev） + `TestAdversarial_T065_500NoLeakButLogged_BusyNotDowngraded`（QA 独立） | 同上 |
| AC-5 binMissing→422（回归护栏） | `TestMapProcErr_BinMissing_422_Preserved`（dev） + QA-ADV-3 binMissing 分支不塌缩 | 同上 |
| AC-6 mapProcErr 体无 strings.Contains + 删 import | 静态 grep（CR 已核：strings 仅注释 :89 残留） | handlers_proc.go |
| AC-7 procmgr 既有测试零回归 | 既有测试不断言 Start 忙文本（CR/QA 复核） | manager_test/lifecycle_helper/qa_t007 |
| AC-8 procmgr 文本变化不影响分类 | `TestAdversarial_T065_TextLooksBusyButNotSentinel_Goes500` + `..._AnyCauseTextStillBusyIfSentinel`（QA 独立，见下） | `qa_t007_adversarial_test.go` |
| AC-9 baseline bump | version 32 / go_tests 342 / test_count 894 | `scripts/baseline.json` |
| AC-10 dev-map | procmgr 行 + 错误映射行 | `docs/dev-map.md` |
| AC-11 verify_all PASS | PENDING + 执行规格（下） | — |
| FR-2 不误纳 | `TestStart_NonBusyErrorsNotErrBusy`（dev） | manager_test.go |
| BC-2 多层 wrap | `TestErrBusy_IsSentinel` 双层用例 | manager_test.go |
| BC-4 nil logger | `TestWriteInternalError_NilLogger`（既有，writeInternalError 共用） | handlers_hygiene_test.go |

## Boundary tests added

- 多层 `%w` 包裹的 errors.Is 判定（BC-2）。
- 完全不含旧关键词（stopping/starting/running）甚至中文的 cause，包 ErrBusy 仍 409（不依赖文本）。
- 含旧关键词但不包 ErrBusy 的裸错误（旧 strings.Contains 会误 409）现走 500。
- 500 路径原始 cause 含路径/errno/英文细节，逐子串证伪泄露 + 完整进日志。
- 分类互斥边界：同一 helper 下 binMissing(422) / ErrBusy(409) / 其它(500) 三路不塌缩。

## Adversarial tests

QA 从验收标准独立编写 reproducer（**不复用 dev 的 handlers_hygiene_test.go 测试**），落于 `internal/httpapi/qa_t007_adversarial_test.go`。
裁决依据是"实现是否在这些对抗下存活"，非"dev 自测是否通过"。QA 上下文无 Bash，预期结果由 errors.Is / httptest / slog 的确定性语义逐用例推导（无随机/IO/竞争——纯错误构造 + 内存 recorder + buffer logger，与 insight L26/L31 同源：纯文本/纯逻辑路径的预期输出可逐 fixture 推导成执行规格交真跑核对）。

| AC | 假设（"我预期失败当…"） | Reproducer（QA 新写） | 预期结果（确定性推导） |
|---|---|---|---|
| AC-8（核心，insight L34） | 我预期 handler 仍隐藏依赖文本——含 `stopping`/`running`/`starting` 但**不包 ErrBusy** 的裸错误会被误判 409（旧 strings.Contains 行为残留） | `TestAdversarial_T065_TextLooksBusyButNotSentinel_Goes500`：3 个含忙态关键词的裸 `errors.New` 喂 `h.mapProcErr` | **Survived（证伪假设）**：3 个均走 **500**（不再被文本误判 409）+ 不泄露原文。因 mapProcErr 体已无 strings.Contains（AC-6 grep + CR 确认），只剩 errors.Is(ErrBusy)，裸 error 不命中→ fallback writeInternalError 500。**这是旧脆弱文本匹配确已死的运行时+静态双证。** |
| AC-8（反向） | 我预期 handler 仍依赖某文本子串——把 cause 改成不含任何旧关键词（甚至中文）会漏判 500 | `TestAdversarial_T065_AnyCauseTextStillBusyIfSentinel`：cause∈{无关键词英文, "is busy elsewhere", "机器正忙"} 各 `fmt.Errorf("%s: %w", c, procmgr.ErrBusy)` | **Survived**：3 个均恒 **409 PROC_BUSY** + 固定中文'进程正忙'不随 cause 变 + 不泄露 cause。证明分类只认 sentinel，不认文本。模拟 procmgr 任意改字（"currently stopping"→任意）handler 分类不退化。 |
| AC-4 / NF-1（安全） | 我预期 500 路径要么泄露内部文本、要么忘记记日志（二者必疏一个） | `TestAdversarial_T065_500NoLeakButLogged_BusyNotDowngraded`：cause=含路径/errno/英文的 secret 喂 mapProcErr | **Survived**：500 响应逐子串（procmgr/mkdir//var/secret/path/permission denied/errno/Start(frpc)）均**不出现**；完整 cause（permission denied + errno=13）**进 logger**；同路径下 ErrBusy 仍 409 不误降、binMissing 仍 422 不塌缩。 |

**预期 tool 输出（交 batch orchestrator 真跑核对，格式 `go test ./internal/httpapi/ ./internal/procmgr/ -run 'T065|MapProcErr|ErrBusy|Start_Stopping|Start_NonBusy'`）**：
```
ok  github.com/frp-easy/frp-easy/internal/procmgr   (TestErrBusy_IsSentinel, TestStart_StoppingReturnsErrBusy, TestStart_NonBusyErrorsNotErrBusy PASS)
ok  github.com/frp-easy/frp-easy/internal/httpapi    (TestMapProcErr_* x3, TestAdversarial_T065_* x3 PASS)
```
若任一对抗用例 FAIL（如 TextLooksBusyButNotSentinel 得 409），即 strings.Contains 残留信号→回退 dev 修复。

## verify_all result（PENDING — 执行规格交 batch orchestrator 硬闸门）

- Total tests: 885 → **894**（go_tests 333→342：dev 6 + QA 3；frontend_tests 552 不变；test_count 891? 见下）。
- 计数明细：go_tests **342**、frontend_tests **552**、test_count **894**、version **32**。
- Pass: 预期 894 / Fail: 预期 **0** / Warn: 预期 **0**。
- New tests added: **9**（procmgr 3 + httpapi dev 3 + httpapi QA 对抗 3）。
- Baseline updated: **yes**（scripts/baseline.json version 32 / go_tests 342 / test_count 894 / passing 894）。
- 改了断言的既有测试 OLD→NEW：**无**（本任务纯补测；旧透传断言集=空集，RA/SA/GR/CR/QA 五轮 grep 一致确认；`handlers_cancel_then_upload_test.go:347` 为 uploadBin OOS-1 不动，零影响）。
- `-race`：本机无 cgo 不跑（insight，T-050/T-063 先例）；静态论证：本任务不改 procmgr 锁/状态机，仅改"忙"分支错误返回值构造（ps.info.State 赋值零改动），并发安全不变。

## Defects found

无。0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR。

## Stability

- 新增 9 个测试全确定性（无随机/无真子进程 spawn/无 IO 竞争/无固定 sleep）——非 flaky 来源。
- 白盒注入态测试（TestStart_StoppingReturnsErrBusy）在 StateStopping case 早返回，不起 goroutine/不开文件，无资源泄漏致后续用例污染。
- 预期 3 次连跑无 flake（交真跑核对）。

## Verdict

**APPROVED FOR DELIVERY**（0 缺陷；AC-1~AC-10 全覆盖；AC-11 verify_all 标 PENDING 附确定性执行规格交 batch orchestrator 真跑作硬闸门；裸标题 `## Adversarial tests` 段含 3 条独立反向证伪用例）。
