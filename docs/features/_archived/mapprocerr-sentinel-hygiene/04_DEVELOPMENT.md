# 04 开发 — T-065 mapprocerr-sentinel-hygiene

> 阶段 4 / dev-backend（单分区）。输入：03 APPROVED FOR DEVELOPMENT。语言：中文。
> verify_all 全量真跑因 role-collapsed 上下文无 Bash/PS（insight L31）标 **PENDING**，附执行规格交 batch orchestrator 硬闸门。

## 1. 改动清单（全在 dev-backend owned paths）

| 文件 | 改动 |
|---|---|
| `internal/procmgr/manager.go` | 加 `errors` import；新增包级 `var ErrBusy = errors.New("procmgr: process busy")`（带 T-065 注释，紧邻 State 常量后）；`Start` StateStopping 分支裸 `fmt.Errorf` → `fmt.Errorf("procmgr.Start(%s): currently stopping: %w", kind, ErrBusy)` |
| `internal/httpapi/handlers_proc.go` | `mapProcErr` 包级函数 → `func (h *handlers) mapProcErr(...)`；删 `strings.ToLower`+`strings.Contains` 整块改 `errors.Is(binloc.ErrBinMissing)`→422 / `errors.Is(procmgr.ErrBusy)`→409 固定中文 / else→`h.writeInternalError(w,"操作进程失败",err)`；删 `"strings"` import；调用点 procStart:24 / procRestart:66 改 `h.mapProcErr` |
| `internal/procmgr/manager_test.go` | 加 `fmt`/`strings` import；+3 测试 |
| `internal/httpapi/handlers_hygiene_test.go` | 加 `fmt`/`binloc`/`procmgr` import；+3 测试 |
| `scripts/baseline.json` | version 30→31；go_tests 333→339；test_count 885→891；passing_count 885→891；notes 更新 |
| `docs/dev-map.md` | procmgr 行加 ErrBusy 哨兵说明 + 新增「HTTP 错误映射（sentinel 收口）」行 |

## 2. 实现要点与设计符合性

- **FR-1/AC-2**：`manager.go:38` `var ErrBusy`（同 binloc.go:44 范式）。`Start:184` StateStopping 分支 wrap `%w ErrBusy`，保留 `currently stopping` 可读 cause（C-4）。
- **FR-3/FR-4/AC-1**：`mapProcErr` 升级 `*handlers` 方法（同包 mapProxyWriteError 既有方法范式），调用点同步改（C-3）。
- **FR-5/FR-6/C-1**：409 文案逐字 `进程正忙（启动或停止进行中），请稍后重试`；500 userMsg 逐字 `操作进程失败`。
- **AC-6/C-2**：删 strings.Contains 整块 + 删 `"strings"` import；grep 确认 handlers_proc.go 函数体无 `strings.`（唯一残留在注释 :89，非代码）。
- **OOS-5**：未为 StateStarting/StateRunning 新增忙错误（保持 idempotent 不报错）；删除对 "starting"/"running" 文本匹配同时消除既有空匹配。
- **C-6 对账**：grep `ErrBusy` 确认生产代码仅 manager.go:38（定义）+ :184（StateStopping 唯一使用）；grep `mapProcErr` 确认调用点仅 procStart:24 / procRestart:66。忙返回点全集 = Start StateStopping 一处，无遗漏无误纳。
- **不改生命周期（OOS-3）**：仅改"忙"分支错误返回值构造方式，`ps.info.State` 赋值零改动，锁/状态机不变。

## 3. 新增测试（dev 6 个，QA 对抗测试在 06 阶段另增量）

### procmgr/manager_test.go（+3，package procmgr 白盒）

- `TestErrBusy_IsSentinel`（FR-1/BC-2）：单层/多层 `%w` 包裹 `errors.Is(ErrBusy)`==true；保留可读 cause `currently stopping`；无关错误不命中。
- `TestStart_StoppingReturnsErrBusy`（AC-2，R-2）：白盒注入 `m.processes["frpc"].info.State = StateStopping`（持 m.mu）后调 `Start`，断言 `errors.Is(err, ErrBusy)`==true + cause 含 `stopping`。**真覆盖 manager.go:184 分支，无需真起子进程**。
- `TestStart_NonBusyErrorsNotErrBusy`（FR-2 不误纳）：no-config-path 与 bin-missing 错误均**不**命中 ErrBusy（防 handler 误给 409）。

### httpapi/handlers_hygiene_test.go（+3，复用 newCapturingHandlers + 捕获 slog，insight L28）

- `TestMapProcErr_Busy_409_FixedMessage_NoLeak`（AC-3）：包 ErrBusy 错误 → 409 PROC_BUSY + message 含 `进程正忙` + 不泄露 `procmgr`/`currently stopping`/`Start(frpc)`/`process busy`。
- `TestMapProcErr_Internal_500_NoLeak`（AC-4）：非 sentinel 错误（`no config path configured`）→ 500 INTERNAL + 固定中文 `操作进程失败` + 不泄露内部 cause + cause 进 logger。
- `TestMapProcErr_BinMissing_422_Preserved`（AC-5 回归护栏）：`binloc.ErrBinMissing` 包裹 → 仍 422 BIN_MISSING。

## 4. 既有测试零回归论证

- procmgr：`TestStart_BinMissing` / `TestStart_NoConfigPath` / `TestInvalidKind` / `TestStop_Idempotent` / lifecycle helper 系列 / qa_t007_adversarial——均不断言 Start 的"忙"错误文本（仅 err==nil/!=nil 或终态集合），wrap ErrBusy 后 `err != nil` 仍成立、文本仍含 `currently stopping`，**零回归**。
- httpapi：`handlers_cancel_then_upload_test.go:347` `TestUploadBin_409Message_RuntimeAssert` 测的是 **uploadBin** 路径（handlers_system.go PROC_BUSY，OOS-1），**不经 mapProcErr**，零影响。mapProxyWriteError 系列测试不受 mapProcErr 改动影响（不同函数）。
- **insight L35 结论**：经 mapProcErr 的"断言旧透传行为"既有测试集 = **空集**，本任务为纯补测，无需 PM 批准的过时断言更新。

## 5. verify_all 执行规格（PENDING，交 batch orchestrator 真跑）

**预期：PASS。**

- `go build ./...`：预期成功。风险点已自查：handlers_proc.go 删 strings import 后 errors/binloc/procmgr 仍全部在用（errors.Is / binloc.ErrBinMissing / procmgr.ErrBusy + procStatus 的 procmgr.ProcessInfo）；manager.go 加 errors import（用于 errors.New）+ 既有 fmt 仍在用；test 文件 import 增量（manager_test +fmt+strings；handlers_hygiene_test +fmt+binloc+procmgr）全部在新测试中使用。
- `go test ./...`：预期全 PASS。新增 6 个顶层 Test 全部确定性（无随机/IO/竞争——白盒注入状态 + 纯错误构造 + httptest）。
- **go_tests 计数**：333 → **339**（+6 dev）。`go test -list '.*' ./...` 顶层 Test* 应 ≥ 339。
  - 新增顶层 Test：`TestErrBusy_IsSentinel`、`TestStart_StoppingReturnsErrBusy`、`TestStart_NonBusyErrorsNotErrBusy`（procmgr）；`TestMapProcErr_Busy_409_FixedMessage_NoLeak`、`TestMapProcErr_Internal_500_NoLeak`、`TestMapProcErr_BinMissing_422_Preserved`（httpapi）。
  - （注：06 QA 阶段将再增量对抗测试，baseline 届时再 bump，最终计数见 06/07。）
- **改了断言的测试 OLD→NEW**：**无**（本任务为纯补测，无既有断言更新——经 RA/SA/GR 三轮 grep 确认旧透传断言集为空集）。
- `frontend_tests`：552 不变（不碰前端）。
- `-race`：本机无 cgo 不跑（insight，与 T-050/T-063 先例一致）；静态论证：不改 procmgr 锁/状态机，仅改"忙"分支错误构造，并发安全不变。

## 6. 裁决

**READY FOR REVIEW**（dev-backend 单分区完成；verify_all 标 PENDING 附执行规格，预期 PASS / go_tests 333→339）。
