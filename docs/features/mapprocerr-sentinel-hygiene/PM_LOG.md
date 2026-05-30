# PM_LOG — T-065 mapprocerr-sentinel-hygiene

> PM Orchestrator 路由决策日志。模式：**full**（7-stage）。批次 `ux-ui-uplift-2026-05` 第 4 个。
> 当前 baseline（task 开始）：go_tests=333 / frontend_tests=552 / test_count=885 / version=30。

## 任务一句话目标

把 `internal/httpapi/handlers_proc.go:mapProcErr` 对 procmgr 内部错误文本的脆弱 `strings.Contains`
匹配收口为 procmgr sentinel（`procmgr.ErrBusy`），让 500 路径走 `writeInternalError` 不泄露内部文本——
对称复刻 T-059（mapProxyWriteError 收口）/ T-055（writeInternalError）。

## 适用 insight（dispatch 时透传给下游）

- L34：把驱动错误文本→分类的脆弱依赖收敛回源层 sentinel 时，正确反向证伪是 **grep 确认 handler 已无 strings.Contains**，
  使"文本变化是否影响分类"退化为纯静态事实，无需运行时证据即可确定性论证。
- L35（红线 3 例外核心）：把面向前端 msg 从透传内部英文改固定中文会确定性破坏断言透传原文的既有测试；
  当任务**有意改变该行为**时同步改断言是 PM 批准的过时断言更新而非删活测试。判别：用例数不降 + 改断言内容（验证新意图）非删用例 + PM_LOG 显式批准。开工前必须 grep 出所有断言旧行为的测试纳入同分区。
- L9：procmgr 真 spawn 测试用编译独立 helper 程序（被测代码硬编码自定义 flag）。
- L10：同步点禁固定 sleep，用 poll-until-condition + deadline。
- L23：错误/取消路径最终状态持久化用 detached context（本任务可能不涉及，备查）。
- L31：role-collapsed PM/dev 上下文无 Bash，纯静态/确定性断言可写"执行规格"交 batch orchestrator 真跑作硬闸门。

## 关键现状（PM 已读源码确认，供下游核对）

- `handlers_proc.go:88-100` mapProcErr：binloc.ErrBinMissing→422；`strings.Contains(low,"stopping"|"starting"|"running")`→409 CodeProcBusy（透传 msg）；
  末尾 `writeError(w,500,CodeInternal,msg,"")` **透传原始 error 文本**（含 `procmgr.Start(frpc): ...` 内部细节）。
- `handlers_proc.go:42` procStop 已用 `h.writeInternalError`——mapProcErr 是唯一漏网一致性缺口。
- `manager.go` 中"忙"语义返回点盘点（PM 初查，SA/RA 须复核全集）：
  - `Start:171` `StateStopping` 分支 → `fmt.Errorf("procmgr.Start(%s): currently stopping", kind)` —— 唯一显式"忙"返回。
  - `Start` 的 `StateStarting/StateRunning`：**idempotent 早返回 ProcessInfo + nil（不报错）**——handler 匹配 "starting"/"running" 实际上当前永不命中任何 procmgr 返回的错误文本。SA 须裁定是否仅 ErrBusy 包 stopping 一处，还是需要补 starting/running 的拒绝语义（不得改变 Start/Stop/Restart 生命周期状态机行为，红线）。
  - `Stop`：全程 idempotent 不返"忙"错误；`Restart`=Stop+Start，透传 Start 的错误。
- `mapProcErr` 唯一调用点：procStart:25 与 procRestart:67（procStop 已走 writeInternalError）。
- `CodeProcBusy` 常量须保留语义不变（核查在 handlers errcodes 文件）。

## 阶段流转

| 时间 | 阶段 | 决策 | 备注 |
|---|---|---|---|
| T0 | 启动 | 创建任务目录、读 insight-index/tasks/baseline、读 handlers_proc.go + manager.go 源码 | mode=full |
| T1 | Stage 1 RA | **ADVANCE → Stage 2**。01 裁决 READY，无开放问题。 | 关键结论：procmgr "忙"返回点全集=Start:171 StateStopping 一处（StateStarting/StateRunning 是 idempotent 不报错→handler 现行匹配 starting/running 为空匹配，OOS-5 消除空匹配不补语义）；mapProcErr 须升级为 *handlers 方法（FR-4 调 writeInternalError）；**insight L35 关键复核：经 mapProcErr 的"断言旧透传行为"既有测试集=空集**（procmgr 测试不断言 Start 忙错误文本仅 err==nil/!=nil；httpapi 无直测 mapProcErr 文本透传；handlers_cancel_then_upload_test.go:347 是 uploadBin 独立路径=OOS-1 不动）→ 本任务"补测锁新意图"为主，无需 PM 批准的过时断言更新（除非分区内发现 RA 漏掉的断言）。errors.go 三常量值已核（PROC_BUSY/BIN_MISSING/INTERNAL）。dev partition 检测：dev-db/dev-backend/dev-frontend 存在→分区模式，本任务纯后端=dev-backend 单分区。 |
| T2 | Stage 2 SA | **ADVANCE → Stage 3**。02 裁决 READY。 | SA 独立全量通读 manager.go 复核 §盘点确认：唯一忙返回点=Start:171 StateStopping（与 RA 一致）；mapProcErr 升级 *handlers 方法（同包 mapProxyWriteError 已是方法范式，无障碍）；固定文案 409=`进程正忙（启动或停止进行中），请稍后重试` / 500=`操作进程失败`（中性覆盖 Start+Restart 两调用点）；R-2 关键裁定：sentinel 测**不需真触发 StateStopping**，直测 errors.Is wrap + handler 侧构造 fmt.Errorf("...: %w", ErrBusy) 喂 mapProcErr 即可（避开 insight L9 helper 复杂度）；R-4 删 strings.Contains 须连带删 "strings" import（防 imported-not-used）。Partition assignment：全 dev-backend 单分区，顺序 manager.go→handlers_proc.go→测试→baseline→dev-map。SA 复核确认旧透传断言测试集=空集。 |
| T3 | Stage 3 GR | **ADVANCE → Stage 4**。03 裁决 APPROVED FOR DEVELOPMENT，8/8 PASS，0 WARN/0 FAIL。 | GR 独立 Read 验证复用引用全部存在：handlers_proxies.go:240-264 mapProxyWriteError（*handlers 方法 + errors.Is + writeInternalError 兜底）、binloc.go:44 ErrBinMissing、handlers_hygiene_test.go:23-106 newCapturingHandlers+捕获 slog+mapProxyWriteError 直测范式、writeInternalError:107-108 nil 守卫。F-1 正面确认旧透传断言测试集=空集（GR 独立 grep 复核一致），本任务为纯补测无需 PM 批准过时断言更新（与 T-059 有 Validation_Preserved 不同）。预答 Q1-Q6（ErrBusy=errors.New 同 binloc 范式 / StateStopping wrap %w / 测试需 import procmgr+fmt / sentinel 测不需真 spawn R-2 / 500 userMsg=操作进程失败 / mapProcErr 必须方法化）。条件 C-1~C-7。**Stage 4 闸门满足（Stage 3 显式 PASS）。** dev partition：所有改动文件在 dev-backend owned paths（procmgr/httpapi/dev-map）；baseline.json PM 授权 dev-backend 编辑（测试计数账本归测试改动分区，T-059/T-063 既定实践）。 |
| T4 | Stage 4 dev-backend | **ADVANCE → Stage 5**。04 裁决 READY FOR REVIEW。 | 实现：manager.go 加 ErrBusy + errors import + StateStopping wrap %w；handlers_proc.go mapProcErr 方法化 + errors.Is 分类 + writeInternalError 兜底 + 删 strings.Contains 整块 + 删 strings import + 调用点改 h.mapProcErr；+6 dev 测试（procmgr 3 + httpapi 3）；baseline bump 30→31 / go_tests 333→339 / test_count 885→891；dev-map 两处。dev 自查 C-2（无 strings. 代码残留）/C-6（ErrBusy 仅 manager.go:38+184；mapProcErr 仅 2 调用点）全过。**Stage 5 闸门：verify_all 标 PENDING + 执行规格（预期 PASS / go_tests 333→339 / 无既有断言更新）——role-collapsed 无 Bash（insight L31），与 T-063/T-064 先例一致，PM 接受 PENDING 推进，真跑由 batch orchestrator 作硬闸门。** 无既有断言更新（旧透传断言集=空集，已三轮 grep）。 |
| T5 | Stage 5 CR | **ADVANCE → Stage 6**。05 裁决 APPROVED（0 CRITICAL/MAJOR/MINOR，2 NIT）。 | CR 独立读全部改动文件 + 交叉核对 binloc.go:44/handlers_proxies.go:240-264/handlers_cancel_then_upload_test.go:347（确认 OOS-1）；walk through AC-1~AC-11 全 ✅（AC-11 verify_all PENDING 交硬闸门、AC-8 待 06 QA 落实裸标题）；设计保真 12 项全 ✅；专项追踪 Start:159-234 控制流确认白盒测试 successPath=false 不 spawn 无 goroutine/文件泄漏；测试有意义性确认非 shape-matching（正向+负向 leak 双断言）。 |
| T6 | Stage 6 QA | **ADVANCE → Stage 7**。06 裁决 APPROVED FOR DELIVERY，0 缺陷。 | QA 独立从 AC 写 3 个对抗 reproducer（不复用 dev 测试，落 qa_t007_adversarial_test.go）：TextLooksBusyButNotSentinel_Goes500（含忙态文本但无 ErrBusy 恒 500——旧脆弱匹配已死的运行时+静态双证 AC-8 核心）/ AnyCauseTextStillBusyIfSentinel（cause 任意变化甚至中文只要包 ErrBusy 恒 409）/ 500NoLeakButLogged_BusyNotDowngraded（逐子串不泄露+完整进 logger+分类不塌缩 AC-4/NF-1）。baseline 再 bump 31→32 / go_tests 339→342 / test_count 891→894（QA +3）。06 含裸 ## Adversarial tests。**Stage 7 闸门满足（CR APPROVED + QA APPROVED FOR DELIVERY 双 PASS）。** verify_all 仍 PENDING（QA 同 role-collapsed 无 Bash），确定性执行规格交 batch orchestrator 真跑作硬闸门。 |
| T7 | Stage 7 PM | **DELIVERED**。07 产出，含裸 ## Insight。更新 tasks 看板。 | 0 rollback。verify_all=PENDING（预期 PASS / go_tests 342 / frontend 552 / test_count 894）。按批次约定不 commit/不 archive，交 batch orchestrator。 |

## 断言更新显式批准段（红线 3 例外 — PM 必填，insight L35）

**结论：本任务无既有断言更新，故无需红线 3 例外批准。**

**判定依据（insight L35 的三项判别标准全程把关）**：
1. **用例数不降**：净 +9 Go 顶层测试（333→342），无任何删除。✅
2. **改的是断言内容还是删用例**：本任务**两者都不是**——所有 9 个测试均为**新增**，无任何既有测试的断言被修改或删除。
3. **PM 显式批准**：因不存在断言更新，无需批准；本段记录"为何无需批准"的判断。

**为何与 T-059 不同（T-059 有 Validation_Preserved→FixedMessage_NoLeak 的断言更新需 PM 批准 C-1）**：
T-059 收口的 mapProxyWriteError 此前有 `TestMapProxyWriteError_Validation_Preserved` 显式断言"透传 storage 英文原文"（锁定旧透传行为的护栏），收口改固定中文必然破坏它→需红线 3 例外批准改断言。
而本任务收口的 mapProcErr，经 **RA → SA → GR → CR → QA 五轮独立 grep** 一致确认：**不存在任何断言"mapProcErr 透传 procmgr 内部英文文本/500 含内部细节/409 文案是旧英文"的既有测试**——
- procmgr 测试（manager_test / lifecycle_helper_test / qa_t007_adversarial_test）只断言 `err==nil` / `err!=nil` / 终态集合，**从不断言 Start 的 "currently stopping" 文本**；wrap ErrBusy 后这些断言全部仍成立（err!=nil 仍真）。
- httpapi 测试无任何直测 mapProcErr 的 409/500 message 透传内容（mapProcErr 此前是包级函数，无直测；procStart/procRestart 的 HTTP e2e 测试不构造 StateStopping 忙态、不断言 500 文本）。
- `handlers_cancel_then_upload_test.go:347` `TestUploadBin_409Message_RuntimeAssert` 断言的是 **uploadBin** 路径的 PROC_BUSY（handlers_system.go，OOS-1），**完全不经 mapProcErr**——本任务不动该文件，该测试零影响。

故本任务的行为变更（mapProcErr 透传英文 500 → 固定中文 + 不泄露 + sentinel 判定）落在**无既有护栏断言锁定**的代码上，是纯补测锁新意图，**不触发红线 3 的"删活测试/改活断言"情形**。PM 据五轮 grep 一致结论确认此判定成立。
