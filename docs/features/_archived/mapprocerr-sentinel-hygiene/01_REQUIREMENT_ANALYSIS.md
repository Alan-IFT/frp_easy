# 01 需求分析 — T-065 mapprocerr-sentinel-hygiene

> 阶段 1 / Requirement Analyst。模式：**full**。批次 `ux-ui-uplift-2026-05` 第 4 个。语言：中文。

## 1. 目标（一句话）

把 `internal/httpapi/handlers_proc.go:mapProcErr` 对 procmgr 内部错误文本的脆弱 `strings.Contains`
匹配收口为 procmgr sentinel（`procmgr.ErrBusy`），并让其 500 兜底路径走 `h.writeInternalError`
（固定中文文案 + 原始 error 进 logger，不向前端泄露内部文本）。

## 2. 范围内行为（可测）

- **FR-1**：procmgr 包导出哨兵错误 `ErrBusy`（`errors.New` 风格的包级 `var`）。
- **FR-2**：procmgr 中所有"因进程处于过渡/活动态而拒绝操作"且**当前会返回 error** 的分支，改为返回
  **包装了 `ErrBusy` 的错误**：`fmt.Errorf("<可读 cause>: %w", ErrBusy)`，保留原可读 cause 文本（进日志可诊断），同时 `errors.Is(err, ErrBusy)` 为真。
  - **盘点结论（权威，SA 须复核）**：经全量通读 `manager.go`，procmgr 当前**唯一**会因"忙/过渡态"而返回 error 的分支是
    `Start:171` 的 `StateStopping` 分支（`fmt.Errorf("procmgr.Start(%s): currently stopping", kind)`）。
  - `Start` 的 `StateStarting` / `StateRunning` 分支是 **idempotent 早返回**（返回当前 `ProcessInfo` + `nil`，**不报错**，:164-168）。
    因此 handler 现行 `strings.Contains(low,"starting")` / `strings.Contains(low,"running")` 在当前代码下**永不命中**任何
    procmgr 返回的错误文本（这些状态不报错）——属现行匹配的"误纳/空匹配"成分。
  - `Stop` 全程 idempotent，不返"忙"错误；`Restart` = `Stop` + `Start`，仅透传 `Start` 的错误。
  - `Start` 的其余 error 分支（`no config path`、`mkdir log` 失败、`cmd.Start()` 失败、`process exited/disappeared`）
    **不是"忙"语义**，不得包 `ErrBusy`（保持非 sentinel，由 handler 走 500）。
- **FR-3**：`mapProcErr` 改为按以下顺序分类（删除原 `strings.ToLower` + `strings.Contains` 整块）：
  1. `errors.Is(err, binloc.ErrBinMissing)` → 422 `CodeBinMissing`（**保持现状不变**，binloc sentinel 已是好范式）。
  2. `errors.Is(err, procmgr.ErrBusy)` → 409 `CodeProcBusy`，面向前端**固定中文文案**（不透传 procmgr 内部英文 cause）。
  3. 其余 → `h.writeInternalError(w, "<固定中文>", err)`（500 `CodeInternal` 固定中文 + 原始 error 进 logger）。
- **FR-4**：因 FR-3 第 3 分支需 `h.writeInternalError`（是 `*handlers` 方法），`mapProcErr` 须从包级函数改为
  `*handlers` 方法（`func (h *handlers) mapProcErr(...)`），调用点 `procStart:25` / `procRestart:67` 同步改 `h.mapProcErr(...)`。
- **FR-5**：409 PROC_BUSY 面向前端固定文案语义为"进程正忙（启动/停止进行中），请稍后重试"（精确文案由 SA 定，须为中文、不含 procmgr 内部英文 cause）。
- **FR-6**：500 INTERNAL 面向前端固定文案语义为"操作进程失败"类（精确文案由 SA 定，须为中文、不含原始 error 子串），原始 error 进 `logger`（复用 `writeInternalError` 既有 `Logger.Error("internal error",...)` 路径）。
- **FR-7**：`CodeProcBusy` / `CodeBinMissing` / `CodeInternal` 错误码常量值与语义保持不变（仅复核，不改 `errors.go`）。
- **FR-8**：补测锁死新意图（详见验收）：sentinel 直测、handler 分类测、500 不泄露测、对抗测（procmgr 文本变化不影响分类）。

## 3. 范围外（本次明确不做）

- **OOS-1**：`handlers_system.go` 中独立的 `CodeProcBusy` 用法（下载已在进行中 :134、上传进行中 :555/561）
  与 `mapProcErr` **完全独立**（uploadBin/download-bin 路径，不经 procmgr），**不动**。
  特别地 `handlers_cancel_then_upload_test.go:347`（`TestUploadBin_409Message_RuntimeAssert`）断言的是 uploadBin 路径，**与本任务无关，不改**。
- **OOS-2**：uploadBin errno 透传（`handlers_system.go:587` 落盘失败 `"落盘失败: "+err.Error()`，B-A.12 有意决策），**不动**。
- **OOS-3**：不改 procmgr 进程生命周期行为（Start/Stop/Restart/ApplyConfigChange 的**状态机转移、idempotent 语义、3s 稳定等待、5s kill 超时、emit 次数/顺序**全部不变）。本任务**仅**改"忙"分支的错误**返回值构造方式**（裸 `fmt.Errorf` → wrap `ErrBusy`），不改任何 `ps.info.State` 赋值。
- **OOS-4**：不碰前端（Vue）、不碰 `internal/storage`、不碰 DB/migration、不碰 `internal/binloc`（仅引用其 sentinel）。
- **OOS-5**：不为 `StateStarting` / `StateRunning` **新增**拒绝-报错语义（保持 idempotent 不报错）。即不"补"starting/running 的忙错误——这会改变 Start 的可观察行为（OOS-3 红线）。handler 删除对 starting/running 文本的匹配是**消除空匹配**，不是把语义搬到别处。

## 4. 边界条件

- **BC-1**：`mapProcErr` 收到 `nil` error——调用点已先判 `if err != nil` 才调用（:24/:66），故 `mapProcErr` 不会收到 nil；无需额外 nil 守卫（与现行一致）。
- **BC-2**：`ErrBusy` 被多层 `%w` 包裹（如未来 Restart 包一层）——`errors.Is` 沿 wrap 链匹配，仍为真。本任务 Restart 透传不额外包裹，单层即可。
- **BC-3**：同时是 `ErrBinMissing` 与（理论上）`ErrBusy`——实际不可能（二进制缺失发生在 `binPathFor`，早于 StateStopping 检查；且二者来自不同分支）。分类顺序 binMissing 在前，确定性。
- **BC-4**：`h.deps.Logger == nil`——`writeInternalError` 已有 nil 守卫（:108），500 仍正常返回固定文案，不 panic。
- **BC-5**：procmgr cause 文本含 `%`、引号等——固定中文文案不引用 cause，无格式注入风险；cause 仅进结构化 logger 字段，安全。
- **BC-6**：`mapProcErr` 改为方法后，包内是否有其它调用点——grep 确认仅 procStart/procRestart 两处（PM 已核），无遗漏。

## 5. 验收标准（可验证）

- **AC-1**：`go build ./...` 通过；`mapProcErr` 为 `*handlers` 方法，procStart/procRestart 调用 `h.mapProcErr`。
- **AC-2**：`internal/procmgr` 导出 `ErrBusy`；`Start` 的 StateStopping 分支返回 `errors.Is == ErrBusy` 为真的错误，且 `err.Error()` 仍含可读 cause（如含 `stopping`）。
- **AC-3**：`mapProcErr` 收到包 `ErrBusy` 的错误 → 响应 409 + code `PROC_BUSY` + message 为固定中文 + **不含** procmgr 内部英文子串（如不含 `procmgr.Start`、不含 `currently stopping`）。
- **AC-4**：`mapProcErr` 收到非 sentinel、非 binMissing 的错误（如 `no config path configured`）→ 响应 500 + code `INTERNAL` + message 为固定中文 + **不含**原始 error 子串；原始 error 进 logger（捕获型 logger 断言 cause 在日志）。
- **AC-5**：`mapProcErr` 收到 `binloc.ErrBinMissing` 包裹错误 → 仍 422 + `BIN_MISSING`（行为不变，回归护栏）。
- **AC-6**：`handlers_proc.go` 中 `mapProcErr` 函数体**不再出现** `strings.Contains` / `strings.ToLower`（grep 静态护栏；`strings` import 若无其它使用则移除）。
- **AC-7**：procmgr 既有测试（manager_test / lifecycle_helper_test / qa_t007_adversarial_test）**全部不回归**——它们均不断言 Start 的"忙"错误文本，仅断言 err==nil / err!=nil（已核），wrap ErrBusy 后 `err != nil` 仍成立。
- **AC-8**：对抗测试（06 §Adversarial）反向证伪：把 procmgr 的 cause 文本改成任意字（"currently stopping"→"is stopping"/"busy now"）handler 分类仍为 409——因 handler 仅 `errors.Is`，已无 strings.Contains（insight L34 退化为静态事实）。
- **AC-9**：`scripts/baseline.json` 的 `go_tests` / `test_count` 同步 bump（+净新增用例数）+ `version` +1；`go_tests` ≤ 实际 `go test -list` 顶层计数。
- **AC-10**：`docs/dev-map.md` 加 procmgr sentinel 行（ErrBusy）+ handler 错误映射行（mapProcErr）。
- **AC-11**：`scripts/verify_all` 预期 PASS（PM/dev 上下文无 Bash，标 PENDING + 执行规格交 batch orchestrator 真跑）。

## 6. 非功能需求

- **NF-1（安全/信息泄露）**：500 与 409 响应体均不得包含 procmgr 内部 error 文本（路径、内部函数名、英文 cause）——与 T-055 `writeInternalError` 原则一致（insight L28）。
- **NF-2（可诊断性）**：500 路径的原始 error 必须进 logger（`writeInternalError` 既有路径），保留运维可定位性。
- **NF-3（并发）**：本任务不改 procmgr 锁/状态机，并发安全不变；`-race` 本机无 cgo 不跑，静态论证锁顺序不变（OOS-3）。

## 7. 相关历史任务

- **T-059** `proxy-remoteport-conflict-sentinel`（`docs/features/proxy-remoteport-conflict-sentinel/`）：**主范式**。
  storage 层 `ErrDuplicateRemotePort` sentinel + handler `errors.Is` 收口 + 固定中文 + 删 strings.Contains。本任务对称复刻，只是匹配的是 procmgr 文本而非 SQL 文本。
- **T-055** `backend-api-hygiene`：`writeInternalError` helper 来源（`handlers_proc.go:107`），500 固定文案 + 原始 error 进 logger。本任务复用它。
- **T-050** `backend-test-coverage`：procmgr 子进程生命周期测试范式（编译独立 helper 程序 `lifecycle_helper_test.go` + `testing.Short()` 门控，insight L9）。
- **T-045** `backend-deadcode-cleanup`：procmgr 删发布订阅死代码——确认 procmgr 当前无残留 emit 机制。
- insight L34（驱动文本→分类收敛回 sentinel 的反向证伪=grep 确认无 strings.Contains）、L35（PM 批准的过时断言更新红线 3 例外）、L9（procmgr 真 spawn 编译独立 helper）、L31（role-collapsed 无 Bash 写执行规格交真跑）。

## 8. 待用户澄清的问题

**无开放问题。** 设计方向、范围、固定文案语义、红线均由 PM INPUT 与用户任务描述明确给定；procmgr "忙"语义返回点全集经全量通读已确定（仅 Start:171 一处）。

关于"现行 handler 匹配 starting/running 但 procmgr 不报错"这一观察，已在 FR-2 / OOS-5 给出确定结论（消除空匹配，不补语义），无需用户裁定——理由：补 starting/running 的忙错误会改变 Start 的 idempotent 可观察行为（OOS-3 红线）。SA 若有不同设计裁量，在 02 记录并经 GR 闸门。

## 9. 裁决

**READY**（无开放问题，可进入 Stage 2 方案设计）。

### 给 SA 的关键交接点

1. procmgr "忙"返回点全集 = `Start:171` StateStopping 一处（已通读确认）。SA 须独立复核并在 02 §盘点确认。
2. `mapProcErr` 须升级为 `*handlers` 方法（FR-4）以调用 `h.writeInternalError`。
3. 固定中文文案精确措辞由 SA 定（FR-5/FR-6），须中文 + 不泄露内部文本。
4. **关键（insight L35）**：经 mapProcErr 的"断言旧行为=透传"的既有测试集 = **空集**（RA 已 grep 全量确认：procmgr 测试不断言 Start 忙错误文本；httpapi 测试无任何直测 mapProcErr 409/500 文本透传；`handlers_cancel_then_upload_test.go:347` 是 uploadBin 独立路径=OOS-1）。故本任务是"补测锁新意图"为主，**不存在需 PM 批准更新的过时透传断言**——除非 SA/dev 在分区内发现 RA 漏掉的断言（届时按红线 3 例外流程经 PM 批准）。SA 须在 02 复核此"空集"结论。
5. 分区：纯后端，dev-backend 单分区（procmgr + httpapi 同属后端，无 db/前端）。
