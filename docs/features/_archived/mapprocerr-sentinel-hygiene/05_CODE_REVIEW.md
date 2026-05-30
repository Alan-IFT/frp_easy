# Code Review — T-065 mapprocerr-sentinel-hygiene

> 阶段 5 / Code Reviewer（独立视角）。输入：01 / 02 / 04。语言：中文。

## Files reviewed

- `internal/procmgr/manager.go`（ErrBusy 定义 + StateStopping wrap）
- `internal/httpapi/handlers_proc.go`（mapProcErr 方法化 + 收口 + 调用点）
- `internal/procmgr/manager_test.go`（+3 测试 + import）
- `internal/httpapi/handlers_hygiene_test.go`（+3 测试 + import）
- `scripts/baseline.json`（bump）
- `docs/dev-map.md`（procmgr 行 + 错误映射行）
- 交叉核对：`internal/binloc/binloc.go:44`（ErrBinMissing 范式）、`internal/httpapi/handlers_proxies.go:240-264`（mapProxyWriteError 范式）、`internal/httpapi/handlers_cancel_then_upload_test.go:347`（确认 OOS-1）

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### NIT
- [STYLE] `internal/procmgr/manager.go:38` — `ErrBusy` 的 message `"procmgr: process busy"` 与 cause 文本 `"currently stopping"` 在 `errors.Is` 路径下都不会出现在前端响应（handler 用固定中文），仅进日志。当前措辞清晰，无需改。纯记录。
- [STYLE] `internal/httpapi/handlers_hygiene_test.go:200` — `procBusyErr` 用包级 `var` 而非测试内局部构造，与同文件 `secretCause` 包级常量风格一致，可接受。纯偏好。

## Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 build 通过 + mapProcErr 方法化 + 调用点改 | `handlers_proc.go:100` `func (h *handlers) mapProcErr`；`:24`/`:66` `h.mapProcErr(w, err)` | ✅ |
| AC-2 procmgr 导出 ErrBusy + Start StateStopping 返回 errors.Is==ErrBusy + cause 可读 | `manager.go:38` var ErrBusy；`:184` `fmt.Errorf("...: currently stopping: %w", kind, ErrBusy)`；测 `TestStart_StoppingReturnsErrBusy` 白盒真覆盖 | ✅ |
| AC-3 ErrBusy 错误 → 409 PROC_BUSY 固定中文 + 不泄露内部英文 | `handlers_proc.go:105-107`；测 `TestMapProcErr_Busy_409_FixedMessage_NoLeak`（断言 message 含'进程正忙' + leak 集 procmgr/currently stopping/Start(frpc)/process busy 均不出现） | ✅ |
| AC-4 非 sentinel → 500 INTERNAL 固定中文 + 不泄露 + cause 进 logger | `handlers_proc.go:109` `h.writeInternalError(w,"操作进程失败",err)`；测 `TestMapProcErr_Internal_500_NoLeak`（断言固定中文 + leak 不出现 + cause 进 logBuf） | ✅ |
| AC-5 binloc.ErrBinMissing → 422 BIN_MISSING（回归护栏） | `handlers_proc.go:101-103`（保持现状）；测 `TestMapProcErr_BinMissing_422_Preserved` | ✅ |
| AC-6 handlers_proc.go mapProcErr 体无 strings.Contains/ToLower + 移除 strings import | import 块（:3-12）无 `"strings"`；函数体（:100-110）纯 errors.Is；CR grep 确认 `strings.` 仅注释 :89 残留（非代码） | ✅ |
| AC-7 procmgr 既有测试零回归 | 既有测试均不断言 Start 忙错误文本（仅 err==nil/!=nil / 终态集合）；wrap 后 err!=nil 仍成立、文本仍含 stopping；CR 复读 manager_test/lifecycle_helper/qa_t007 确认 | ✅ |
| AC-8 对抗：procmgr 文本变化不影响分类 | handler 仅 errors.Is（AC-6 已无 strings.Contains）→ 退化为静态事实（insight L34）；QA 06 将补对抗用例 | ✅（待 06 QA 补对抗证伪用例落实裸标题） |
| AC-9 baseline bump | `baseline.json` version 30→31 / go_tests 333→339 / test_count 885→891 / passing 891 | ✅（注：06 QA 增量后再 bump，最终值见 07） |
| AC-10 dev-map 更新 | procmgr 行加 ErrBusy 说明；新增「HTTP 错误映射（sentinel 收口）」行 | ✅ |
| AC-11 verify_all PASS | 标 PENDING + 执行规格交 batch orchestrator 真跑（insight L31） | ⏳ PENDING（非 CR 可裁，交硬闸门） |

## Design fidelity check

| Design item（02） | Implementation | Status |
|---|---|---|
| ErrBusy = errors.New（同 binloc 范式） | `manager.go:38` | ✅ |
| StateStopping 唯一忙返回点 wrap %w，保留 cause（C-4） | `manager.go:184` 保留 `currently stopping` + 尾 `%w ErrBusy` | ✅ |
| 不为 StateStarting/StateRunning 补忙语义（OOS-5） | `manager.go:165-168` idempotent 早返回 nil 未改 | ✅ |
| mapProcErr 升级 *handlers 方法（FR-4） | `handlers_proc.go:100` | ✅ |
| 409 固定文案逐字（C-1） | `:106` `进程正忙（启动或停止进行中），请稍后重试` | ✅ |
| 500 userMsg 逐字（C-1） | `:109` `操作进程失败` | ✅ |
| 分类顺序 binMissing→ErrBusy→fallback（§6 流程） | `:101`→`:105`→`:109` | ✅ |
| 删 strings import（C-2/R-4） | import 块无 strings | ✅ |
| 不改生命周期状态机（OOS-3） | 仅改"忙"分支错误构造；`ps.info.State` 赋值零改动；锁/IIFE 结构不变 | ✅ |
| sentinel 测不需真 spawn（R-2） | `TestStart_StoppingReturnsErrBusy` 白盒注入 StateStopping，Start 在 IIFE 内 case StateStopping 早返回 startErr，**successPath=false 不 spawn、不起 goroutine**（CR 追踪 Start:159-234 控制流确认无副作用/无泄漏） | ✅ |

## 逻辑正确性专项（dimension 1）

- **错误链顺序**：binMissing 在 ErrBusy 前——二者来自不同分支不会同时命中；即便理论同时，binMissing 优先确定（BC-3）。✅
- **errors.Is 沿 wrap 链**：StateStopping 单层 wrap，Restart 透传不额外包裹，单层 errors.Is 必真；多层场景由 `TestErrBusy_IsSentinel` 双层用例护栏。✅
- **白盒测试无副作用**：`TestStart_StoppingReturnsErrBusy` 注入状态后 Start 走 StateStopping case 早返回，不触 cmd.Start/supervise/waitUntilStable；`TestStart_NonBusyErrorsNotErrBusy` 的 m1/m2 分别在 no-config-path / binPathFor 错误处早返回 IIFE，均不 spawn。无 goroutine/文件泄漏（无需 t.TempDir log 文件，因不到 mkdir 段）。✅
- **nil 守卫**：500 路径经 writeInternalError，`h.deps.Logger != nil && cause != nil` 双守卫（:118），BC-4 满足。✅

## 测试有意义性（dimension 6，hard rule 4）

- 测试断言**新意图**而非 shape-matching：409/500 测试既断言正向（code + 固定中文）又断言负向（leak 集不出现 + cause 进日志），是真护栏。
- `TestStart_NonBusyErrorsNotErrBusy` 是关键"不误纳"反向用例（防 handler 把内部失败错给 409），非冗余。
- `TestStart_StoppingReturnsErrBusy` 真覆盖生产分支（非纯包级符号 wrap），强于最低要求。
- 无删除既有测试；无既有断言更新（旧透传断言集=空集，CR 独立 grep 复核确认 `handlers_cancel_then_upload_test.go:347` 为 uploadBin OOS-1）。

## 安全/性能/迁移（dimensions 4/5/与迁移）

- 安全：500/409 不泄露内部文本（NoLeak 测试护栏）；cause 仅进结构化 logger 字段无注入。✅
- 性能：移除 strings.ToLower + 3 次 Contains，改 2 次 errors.Is，性能无回归（更优）。✅
- 迁移：无 schema/DB；状态码语义不变仅 message 英→中；纯代码回滚。✅

## Verdict

**APPROVED**（0 CRITICAL / 0 MAJOR / 0 MINOR / 2 NIT）。

设计保真度全 ✅，AC-1~AC-10 实现齐备，AC-11 verify_all 交硬闸门 PENDING（非 CR 裁量），AC-8 对抗用例待 06 QA 落实裸标题。代码可进入 Stage 6 QA。
