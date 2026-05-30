# Delivery Summary — T-065 mapprocerr-sentinel-hygiene

- Task: `mapprocerr-sentinel-hygiene` — 把 handlers_proc.go:mapProcErr 对 procmgr 内部错误文本的脆弱 strings.Contains 匹配收口为 procmgr.ErrBusy sentinel + 500 路径走 writeInternalError 不泄露内部文本（对称复刻 T-059/T-055）。
- Mode: **full**（7-stage）。批次 `ux-ui-uplift-2026-05` 第 4 个。
- Stages traversed: 1 RA(READY) → 2 SA(READY) → 3 GR(APPROVED FOR DEVELOPMENT, 8/8 PASS) → 4 dev-backend(READY FOR REVIEW) → 5 CR(APPROVED, 0C/0M/0m/2NIT) → 6 QA(APPROVED FOR DELIVERY, 0 缺陷) → 7 PM(DELIVERED)。
- Rollbacks: **0**。
- Final verify_all result: **PENDING（预期 PASS）** — PM/dev/QA role-collapsed 上下文无 Bash/PS（insight L31），标 PENDING + 确定性执行规格，真跑由 batch orchestrator 作硬闸门。
- Baseline changes: go_tests 333→**342**（+9：dev 6 + QA 对抗 3）；test_count 885→**894**；frontend_tests **552** 不变；version 30→**32**；passing 894。

## Files changed（git diff stat 预期）

生产代码（2 文件）：
- `internal/procmgr/manager.go` — 加 `errors` import + 包级 `var ErrBusy = errors.New("procmgr: process busy")`（同 binloc 范式）；Start StateStopping 分支裸 `fmt.Errorf` → `fmt.Errorf("procmgr.Start(%s): currently stopping: %w", kind, ErrBusy)`。
- `internal/httpapi/handlers_proc.go` — `mapProcErr` 包级函数 → `*handlers` 方法；删 `strings.ToLower`+`strings.Contains` 整块改 `errors.Is(binloc.ErrBinMissing)`→422 / `errors.Is(procmgr.ErrBusy)`→409 固定中文 / else→`h.writeInternalError(w,"操作进程失败",err)`；删 `"strings"` import；调用点 procStart/procRestart 改 `h.mapProcErr`。

测试（3 文件，+9）：
- `internal/procmgr/manager_test.go` — +3 dev（ErrBusy sentinel / StateStopping 白盒覆盖 / 非忙错误不误纳）+ fmt/strings import。
- `internal/httpapi/handlers_hygiene_test.go` — +3 dev（409 不泄露 / 500 不泄露+进日志 / binMissing 回归）+ fmt/binloc/procmgr import。
- `internal/httpapi/qa_t007_adversarial_test.go` — +3 QA 独立对抗（文本似忙但非 sentinel 恒 500 / cause 任意变化仍 409 / 500 不泄露+进日志+不塌缩）+ encoding/json/errors/fmt/httptest/binloc/procmgr import。

文档/配置（2 文件）：
- `scripts/baseline.json` — bump。
- `docs/dev-map.md` — procmgr 行加 ErrBusy 哨兵说明 + 新增「HTTP 错误映射（sentinel 收口）」行。

未碰：前端（web/）、storage、DB/migration、binloc 源码（仅引用其 sentinel）、errors.go（三常量复核未改）、handlers_system.go（OOS-1 独立 PROC_BUSY 路径）。

## Outstanding risks

- verify_all 全量真跑未在本上下文执行（无 Bash）。所有论证为静态 + 确定性（新增 9 测试无随机/IO/竞争/真子进程）。**交付硬闸门 = batch orchestrator Bash 会话真跑**，特别核对：(1) `go build ./...` 绿（删 strings import 后无 unused）；(2) go_tests==342 / frontend_tests==552 / test_count==894；(3) procmgr 既有测试零回归（尤其 manager_test/lifecycle_helper/qa_t007，wrap ErrBusy 不改 err!=nil 语义）；(4) handlers_cancel_then_upload_test.go:347 TestUploadBin_409Message_RuntimeAssert 不受影响（uploadBin 独立路径）。
- `-race` 本机无 cgo 不跑（与 T-050/T-063 先例一致），静态论证并发安全不变（不改 procmgr 锁/状态机）。

## Next steps for user

- batch orchestrator 在 Bash 会话执行 `scripts/verify_all` 作硬闸门，确认 PASS（预期）后按批次约定统一 commit/archive（本任务按批次约定**未 commit / 未 archive**）。
- 后续候选（非本任务）：若 procmgr 未来扩展更多"忙/过渡态拒绝"语义（如显式拒绝 StateStarting 期间的 Start），届时新分支同样 `%w ErrBusy` 即可，handler 无需改（sentinel 收口的扩展友好性已就位）。

## Insight

- 2026-05-31 · 把 handler 层对下层内部错误文本的脆弱 `strings.Contains` 分类收敛为下层 sentinel（`procmgr.ErrBusy`，`%w` 包裹保留可读 cause 进日志）时，**最强的反向证伪不是"忙态仍 409"（正向），而是"含旧关键词但不包 sentinel 的裸错误现在走 500 而非旧 409"**——这一条同时在运行时和静态上证明旧脆弱文本匹配确已死（旧 strings.Contains 会对该错误误给 409，sentinel 化后只剩 errors.Is 故走 fallback 500）。正向用例（包 sentinel→409）只证明新路径通，负向"陷阱文本不再误判"才证明旧路径已切断，二者缺一不可。与 T-059 insight L34 同源但更进一步：L34 说"grep 无 strings.Contains 即退化为静态事实"，本任务补充"再加一个陷阱文本运行时用例可把静态事实坐实为可执行护栏，防未来有人在 handler 重新引入文本匹配" · evidence: T-065 qa_t007_adversarial_test.go::TestAdversarial_T065_TextLooksBusyButNotSentinel_Goes500（3 个含 stopping/running/starting 文本但无 ErrBusy 的裸 error 恒 500）+ AnyCauseTextStillBusyIfSentinel（cause 改中文仍 409）
- 2026-05-31 · "把脆弱文本匹配收口为 sentinel"任务**未必伴随既有断言更新**（红线 3 例外）：是否需要 PM 批准取决于"被收口的代码此前是否有断言锁定其旧透传/旧文本行为的护栏测试"。T-059 的 mapProxyWriteError 有 `Validation_Preserved` 显式断言透传英文→需批准改断言；而 T-065 的 mapProcErr 此前是**包级函数无直测** + procmgr 测试只断言 err==nil/!=nil（从不断言 "currently stopping" 文本）→ 五轮（RA/SA/GR/CR/QA）grep 一致确认旧透传断言集=**空集**→纯补测无需红线 3 批准。教训：insight L35 的"开工前 grep 出断言旧行为的测试"其产出可能是空集，此时 PM_LOG 仍须显式记录"为何无需批准"的判定（用例数不降 + 全为新增非改/删 + grep 证空集），而非省略该段——空集结论本身也是一次受审计的判断 · evidence: T-065 PM_LOG 断言更新批准段 + mapProcErr 此前为包级函数无直测 vs T-059 handlers_hygiene_test.go::TestMapProxyWriteError_Validation_Preserved→FixedMessage_NoLeak
