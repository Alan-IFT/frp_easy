# 02 方案设计 — T-065 mapprocerr-sentinel-hygiene

> 阶段 2 / Solution Architect。模式：**full**。输入：01 裁决 READY。语言：中文。

## 1. 架构摘要

在 `internal/procmgr` 引入包级哨兵 `var ErrBusy = errors.New(...)`，把当前唯一"因过渡态拒绝操作"的
`Start` StateStopping 分支的裸 `fmt.Errorf` 改为 `fmt.Errorf("...: %w", ErrBusy)`（保留可读 cause、`errors.Is` 可判）。
`internal/httpapi/handlers_proc.go` 的 `mapProcErr` 由包级函数升级为 `*handlers` 方法，分类逻辑改为
`errors.Is(binloc.ErrBinMissing)` / `errors.Is(procmgr.ErrBusy)` / fallback `h.writeInternalError`，
删除对 procmgr 文本的 `strings.ToLower` + `strings.Contains` 整块。系统级行为变化：500/409 响应体不再泄露
procmgr 内部英文文本，分类不再依赖文本可变性。procmgr 生命周期状态机零改动。

## 2. 受影响模块

| 文件 | 改动 |
|---|---|
| `internal/procmgr/manager.go` | 新增包级 `ErrBusy`（紧邻 State 常量后）；`Start:171` StateStopping 分支裸 `fmt.Errorf` → wrap `%w ErrBusy` |
| `internal/httpapi/handlers_proc.go` | `mapProcErr` 包级函数 → `*handlers` 方法；删 strings.Contains 整块改 errors.Is + writeInternalError；调用点 procStart:25 / procRestart:67 改 `h.mapProcErr`；移除未用 `strings` import |
| `internal/procmgr/manager_test.go`（或新建 `errbusy_test.go`） | 新增 ErrBusy sentinel 直测（dev 决定文件落点，建议加到 manager_test.go） |
| `internal/httpapi/handlers_hygiene_test.go` | 新增 mapProcErr 分类测（409 sentinel / 500 fallback 不泄露 / 422 binMissing 回归 / strings.Contains 删除静态护栏） |
| `internal/httpapi/qa_t007_adversarial_test.go`（或 handlers_hygiene_test.go） | QA 独立对抗测：procmgr 文本变化不影响分类 |
| `scripts/baseline.json` | bump go_tests / test_count / version |
| `docs/dev-map.md` | procmgr sentinel 行 + handler 错误映射行 |

## 3. 模块分解（无新模块）

无新文件/包。`ErrBusy` 是 procmgr 既有包的新导出符号。`mapProcErr` 是既有函数的签名升级（包级 → 方法）。

## 4. 数据模型变更

无。不碰 DB / migration / schema。

## 5. API 契约

`mapProcErr` 的契约（被 `procStart` / `procRestart` 调用，error 非 nil）：

| 输入 error | HTTP | code | message（面向前端） | Field |
|---|---|---|---|---|
| `errors.Is(binloc.ErrBinMissing)` | 422 | `BIN_MISSING` | `err.Error()`（**保持现状不变**） | "" |
| `errors.Is(procmgr.ErrBusy)` | 409 | `PROC_BUSY` | 固定中文 `"进程正忙（启动或停止进行中），请稍后重试"` | "" |
| 其余 | 500 | `INTERNAL` | 固定中文 `"操作进程失败"`（经 `h.writeInternalError`，原始 error 进 logger） | "" |

### 文案精确措辞（dev 须逐字使用）

- 409 PROC_BUSY message：`进程正忙（启动或停止进行中），请稍后重试`
- 500 INTERNAL userMsg（传给 writeInternalError）：`操作进程失败`

理由：
- 409 文案概括 procmgr 当前忙语义（仅 stopping），但用"启动或停止进行中"覆盖用户心智模型中的"进程过渡态"，
  与现有 `handlers_system.go` PROC_BUSY 风格中文一致（如 "上传进行中，请稍后重试"）。**有意不区分 starting/stopping
  细分态**——因 procmgr 实际只有 stopping 一种忙错误（OOS-5），固定文案不绑定具体内部态，未来 procmgr 即便扩展忙语义文案也不需改。
- 500 文案 `操作进程失败` 覆盖 Start/Restart 两调用点的"启动/重启进程"通用语义（不写"启动失败"以兼容 Restart 调用点）。

### CodeProcBusy / CodeBinMissing / CodeInternal

值与语义不变（`errors.go:21-23` 已核：`BIN_MISSING` / `PROC_BUSY` / `INTERNAL`）。本任务不改 `errors.go`。

## 6. 序列 / 流程

```
procStart(kind) / procRestart(kind)
  └─ ProcMgr.Start(kind) / .Restart(kind)
       └─ [StateStopping] → return fmt.Errorf("procmgr.Start(%s): currently stopping: %w", kind, ErrBusy)
       └─ [其它 error 分支] → return 裸 fmt.Errorf(...)（非 sentinel）
  └─ if err != nil → h.mapProcErr(w, err)
       ├─ errors.Is(err, binloc.ErrBinMissing) → 422 BIN_MISSING（err.Error()）
       ├─ errors.Is(err, procmgr.ErrBusy)       → 409 PROC_BUSY（固定中文，不读 err.Error()）
       └─ else                                   → h.writeInternalError(w,"操作进程失败",err)
                                                     ├─ Logger.Error("internal error","userMsg","操作进程失败","cause",err)（nil 守卫）
                                                     └─ 500 INTERNAL（固定中文，不含 err 文本）
```

关键不变量：`StateStopping` 分支的错误**文本仍含 `currently stopping`**（cause 保留进日志可诊断），
只是额外 wrap `ErrBusy` 让 `errors.Is` 可判。`Start` 其它分支错误**不变**（仍裸 `fmt.Errorf`，走 500）。

## 7. Reuse audit

| 需要 | 既有代码 | 文件路径 | 决策 |
|---|---|---|---|
| sentinel + errors.Is 收口范式 | `ErrDuplicateRemotePort` + `errors.Is` in mapProxyWriteError | `internal/storage/store.go` + `internal/httpapi/handlers_proxies.go:~250` | 复刻范式（T-059 主参照） |
| 500 固定文案 + 原始 error 进 logger | `(*handlers).writeInternalError` | `internal/httpapi/handlers_proc.go:107` | 直接复用，零改动 |
| binloc sentinel + errors.Is | `binloc.ErrBinMissing` | `internal/binloc`（已被 mapProcErr:89 引用） | 保持现状 |
| sentinel 直测范式 | `TestIsDuplicateRemotePortError_DirectChecks` | `internal/storage/proxies_test.go` | 复刻为 ErrBusy 直测 |
| handler 500 不泄露直测 | `TestWriteInternalError_FixedMessage_NoLeak` / `TestMapProxyWriteError_Fallback_NoLeak` | `internal/httpapi/handlers_hygiene_test.go` | 复刻为 mapProcErr 版（捕获型 slog + httptest.NewRecorder，insight L28） |
| procmgr 测试 manager 构造 | `mkManager` / `mkAdvManager` helper | `internal/procmgr/manager_test.go` / `qa_t007_adversarial_test.go` | 复用构造 manager 测 sentinel（StateStopping 注入见 §8 R-2） |

## 8. 风险分析

- **R-1（漏纳/误纳"忙"返回点）**：若 procmgr 还有别处返回忙错误而 SA 漏盘点，handler 该分支退化 500。
  **缓解**：SA 已独立全量通读 `manager.go`（见 §盘点确认），确认唯一忙返回点 = `Start:171`。dev 在改动时再 grep 一次
  `fmt.Errorf` + 状态相关返回，对账 01 §FR-2 盘点。`Stop`/`Restart`/`ApplyConfigChange` 经核均无独立忙返回。
- **R-2（测 ErrBusy sentinel 需触发 StateStopping，而 StateStopping 是瞬态难注入）**：
  **缓解**：sentinel 直测**不需要**真触发 StateStopping。两条路径任选：
  (a) **直测错误构造**——dev 测 `errors.Is(fmt.Errorf("x: %w", procmgr.ErrBusy), procmgr.ErrBusy)` 为真（验证 wrap 正确，纯包级符号测，零进程）；
  (b) **handler 侧注入**——mapProcErr 测直接构造 `fmt.Errorf("...: %w", procmgr.ErrBusy)` 喂给 mapProcErr 验 409（不经真 Start）。
  二者均无需真起 StateStopping 子进程。AC-2「Start StateStopping 返回 errors.Is==ErrBusy」由 (a) 间接保证 + 源码静态可见（dev 可加注释指明），无须真 spawn（避免 insight L9 的 helper 复杂度）。**首选 (a)+(b)，不强求真触发 StateStopping**。
- **R-3（mapProcErr 改方法破坏包内其它调用点）**：grep 确认仅 procStart:25 / procRestart:67 两调用点（PM/RA 已核）。
  **缓解**：dev 改动后再 grep `mapProcErr` 确认零遗漏；procStop 不调 mapProcErr（已走 writeInternalError）。
- **R-4（移除 strings import 破坏编译）**：`handlers_proc.go` 的 `strings` import 仅 mapProcErr 用（PM 读全文件确认无其它 strings 调用）。
  **缓解**：dev 删 strings.Contains 后须删 `"strings"` import 行（否则 `imported and not used` 编译错）。dev 改动后核对 import 块。
- **R-5（500 文案绑定 Start 语义但 Restart 也调用）**：500 文案 `操作进程失败` 已选用中性措辞覆盖两调用点（§5 理由）。

## 9. 迁移 / 回滚计划

- 后向兼容：HTTP 状态码语义不变（409 仍 PROC_BUSY，500 仍 INTERNAL，422 仍 BIN_MISSING）。仅 **message 文本** 从透传英文改固定中文。
  前端若有对 message 文本的硬编码断言需复核——但前端 OOS（不碰），且前端通常按 code 分支不按 message 文本（按现有前端 extractErrorMessage 范式展示 message 即可，中文反而更友好）。
- 无数据迁移、无 feature flag。
- 回滚：纯代码回滚（git revert），无状态/schema 影响。

## 10. 范围外澄清（设计边界）

- 不为 StateStarting/StateRunning 新增忙错误（OOS-5，保持 idempotent 不报错，不改 Start 可观察行为）。
- 不改 `handlers_system.go` 的独立 PROC_BUSY 路径（OOS-1）、不改 uploadBin errno 透传（OOS-2）、不改 procmgr 生命周期状态机（OOS-3）、不碰前端/storage/DB（OOS-4）。
- `Start` 的 `no config path` / `mkdir log` / `cmd.Start` / `process exited` 等非忙错误**不**包 ErrBusy（走 500，符合"内部失败"语义）。

## SA 独立盘点确认（manager.go 全量通读）

SA 复核结论与 01 §FR-2 一致：procmgr 中**唯一**因"过渡/活动态拒绝操作而返回 error"的分支是 `Start:169-172` 的 `StateStopping` case。
- `Start:164-168` StateStarting/StateRunning → idempotent 返回 `ProcessInfo + nil`（不报错）。
- `Stop:253-257` StateStopped/cmd==nil → idempotent 返回 `info + nil`（不报错）；其余 Stop 路径不返"忙"错误。
- `Restart:298-302` = Stop 后 Start，仅透传 Start 错误（含包了 ErrBusy 的 StateStopping 错误，沿 wrap 链 errors.Is 仍真）。
- `ApplyConfigChange` 的错误是 reload/restart 失败的复合错误，**非"忙"语义**，不包 ErrBusy（走 500，符合内部失败语义）。
- `validateKind` 返回的 `invalid kind` 是参数错误，handler 已有 `validProcKind` 前置校验（:17/:35/:61）拦截，理论上 mapProcErr 不会收到；即便收到也属 500（非忙），符合预期。

**确认 01 §交接点 4「经 mapProcErr 的旧透传断言测试集=空集」**：SA grep 复核 `internal/procmgr/*_test.go` 与 `internal/httpapi/*_test.go`，
无任何测试断言 Start 的 "currently stopping" 文本或 mapProcErr 的 409/500 message 透传内容。`handlers_cancel_then_upload_test.go:347` 确为 uploadBin（OOS-1）。
→ 无需 PM 批准的过时断言更新；本任务为纯补测。

## 11. Partition assignment（分区模式存在 dev-*.md）

| 文件 | 分区 | 新建/编辑 | 依赖 |
|---|---|---|---|
| `internal/procmgr/manager.go` | dev-backend | 编辑（加 ErrBusy + wrap StateStopping 分支） | — |
| `internal/httpapi/handlers_proc.go` | dev-backend | 编辑（mapProcErr 方法化 + errors.Is + 删 strings） | 依赖 procmgr.ErrBusy |
| `internal/procmgr/manager_test.go` | dev-backend | 编辑（ErrBusy 直测） | — |
| `internal/httpapi/handlers_hygiene_test.go` | dev-backend | 编辑（mapProcErr 分类 + 不泄露测 + QA 对抗） | — |
| `scripts/baseline.json` | dev-backend | 编辑（bump） | 所有测试改动后 |
| `docs/dev-map.md` | dev-backend | 编辑（两行） | — |

### Dispatch order

1. dev-backend（单分区，覆盖全任务）

### Parallelism

无。单分区。procmgr 改动须先于 handler 改动（handler 依赖 procmgr.ErrBusy 符号），dev-backend 内部按
manager.go → handlers_proc.go → 测试 → baseline → dev-map 顺序实现即可。

## 12. 裁决

**READY**（设计完整，dev 可无歧义实现；纯后端 dev-backend 单分区）。
