# 03 闸门评审 — T-065 mapprocerr-sentinel-hygiene

> 阶段 3 / Gate Reviewer。模式：**full**。输入：01 READY + 02 READY。语言：中文。
> GR 独立验证：所有设计引用的代码已 Read 确认存在。

## 1. 审计清单（8 维）

| # | 维度 | 结论 | 一句话理由 |
|---|---|---|---|
| 1 | 需求完整性 | **PASS** | FR-1~FR-8 全可测；忙返回点全集（Start:171）经 RA+SA 双重通读确认；固定文案语义与精确措辞均已定。 |
| 2 | 设计完整性 | **PASS** | 02 §5 契约表覆盖 422/409/500 三分支 + §6 流程图；mapProcErr 方法化 + 调用点同步改 + strings import 移除全部点名。 |
| 3 | 复用正确性 | **PASS** | GR 已 Read 验证：`handlers_proxies.go:240-264` mapProxyWriteError 确为 `*handlers` 方法 + errors.Is 分类 + writeInternalError 兜底范式（T-059）；`binloc.go:44` ErrBinMissing 存在；`handlers_hygiene_test.go:23-106` newCapturingHandlers + mapProxyWriteError 直测范式可直接复刻；`writeInternalError` 在 handlers_proc.go:107 含 nil 守卫。复用审计准确无遗漏。 |
| 4 | 风险覆盖 | **PASS** | R-1（漏纳忙点，缓解=双通读+dev grep）、R-2（StateStopping 难注入，缓解=直测 wrap 不需真触发，关键正确裁定）、R-3（方法化调用点，缓解=grep 仅 2 处）、R-4（删 strings import，缓解=dev 核对 import）、R-5（500 文案中性）覆盖真实风险，缓解可执行。 |
| 5 | 迁移安全 | **PASS** | 无 DB/schema/migration；状态码语义不变，仅 message 文本英→中；纯代码回滚（git revert）。 |
| 6 | 边界处理 | **PASS** | BC-1（nil err 调用点已挡）、BC-2（多层 wrap errors.Is 仍真）、BC-3（binMissing 优先序确定）、BC-4（nil logger 已有守卫，GR 已读 handlers_proc.go:108）、BC-5（cause 不进文案无注入）、BC-6（调用点 grep 仅 2 处）全设计到位。 |
| 7 | 测试可行性 | **PASS** | 每条 AC 可测：AC-2/3/4/5 由 newCapturingHandlers + httptest.NewRecorder + 捕获 slog 直测（既有范式，GR 已读 handlers_hygiene_test.go:23-106）；AC-6 grep 静态护栏；AC-7 既有 procmgr 测试不断言文本（GR 已读 grep 确认）；AC-8 对抗测=构造任意 cause wrap ErrBusy 仍 409。R-2 裁定 sentinel 测不需真 spawn，避开 insight L9。 |
| 8 | 范围外清晰 | **PASS** | OOS-1（handlers_system PROC_BUSY 独立路径 + handlers_cancel_then_upload_test.go:347 uploadBin）、OOS-2（uploadBin errno）、OOS-3（生命周期状态机）、OOS-4（前端/storage/DB）、OOS-5（不补 starting/running 忙语义）边界明确，dev 不会过建。 |

**8/8 PASS。**

## 2. Findings（WARN/FAIL）

无 WARN，无 FAIL。

补充确认（非缺陷，正面陈述）：
- **F-1（正面）**：insight L35 的"开工前 grep 出断言旧透传行为的测试"经 RA（§8 交接点 4）+ SA（§盘点确认末段）双重 grep 确认为**空集**——
  procmgr 测试（manager_test/lifecycle_helper_test/qa_t007_adversarial_test）均不断言 Start 忙错误文本（仅 err==nil/!=nil）；
  httpapi 无任何直测 mapProcErr 409/500 message 透传；`handlers_cancel_then_upload_test.go:347` 经 GR 复读确认是 uploadBin（OOS-1）。
  GR 独立 grep 复核一致。→ 本任务为纯补测，**当前无需 PM 批准的过时断言更新**（与 T-059 不同，T-059 有 Validation_Preserved 需改名改断言；本任务无对应物）。
- **F-2（正面，insight L34）**：AC-8 对抗论点"procmgr 文本变化不影响 handler 分类"在 sentinel 化后退化为**纯静态事实**（handler 仅 errors.Is，AC-6 grep 确认无 strings.Contains），无需运行时证据即可确定性论证。

## 3. 开发期高概率问题（预答）

- **Q1：ErrBusy 该用 `errors.New` 还是 `fmt.Errorf`？** 用 `var ErrBusy = errors.New("procmgr: process busy")`（与 binloc.go:44 ErrBinMissing 同范式：包级裸 sentinel）。
- **Q2：StateStopping 分支怎么 wrap？** 改 `manager.go:171` 为 `startErr = fmt.Errorf("procmgr.Start(%s): currently stopping: %w", kind, ErrBusy)`。保留 `currently stopping` 可读 cause（进日志），尾部 `%w ErrBusy` 让 errors.Is 真。
- **Q3：mapProcErr 测试需要 import procmgr 吗？** 需要。`handlers_hygiene_test.go` 当前 import storage，新增 `procmgr` import，测试用 `fmt.Errorf("procmgr.Start(frpc): currently stopping: %w", procmgr.ErrBusy)` 构造喂给 `h.mapProcErr`。也需 import `fmt`。
- **Q4：AC-2「Start StateStopping 返回 errors.Is==ErrBusy」要不要真起子进程触发 stopping？** 不需要（02 R-2 裁定）。procmgr 包内直测 `errors.Is(fmt.Errorf("x: %w", ErrBusy), ErrBusy)`==true 验证 sentinel 可用；Start 分支用了 ErrBusy 是源码静态可见 + handler 侧分类测覆盖。**不强求真触发 StateStopping**（瞬态、需 helper 复杂度）。
- **Q5：500 文案传给 writeInternalError 的 userMsg 用什么？** `操作进程失败`（02 §5 定，中性覆盖 Start/Restart 两调用点；不写"启动失败"以兼容 Restart）。
- **Q6：mapProcErr 还是包级函数会怎样？** 不行——fallback 分支需 `h.writeInternalError`（*handlers 方法）。必须升级为 `func (h *handlers) mapProcErr(w http.ResponseWriter, err error)`，调用点改 `h.mapProcErr(w, err)`。

## 4. 裁决

**APPROVED FOR DEVELOPMENT**

设计完整、可实现、可测，8 维全 PASS，零 WARN 零 FAIL。dev-backend 单分区可直接进入开发。

### 开发须遵守的条件（来自设计，非新增）

- **C-1**：固定文案逐字使用——409=`进程正忙（启动或停止进行中），请稍后重试`；500 userMsg=`操作进程失败`。
- **C-2**：删 `strings.Contains` 整块后**必须**删 `handlers_proc.go` 的 `"strings"` import（R-4，防 imported-not-used 编译错）。
- **C-3**：mapProcErr 升级 `*handlers` 方法，procStart:25 / procRestart:67 调用点同步改 `h.mapProcErr`（C-6 预答）。
- **C-4**：StateStopping 分支保留 `currently stopping` 可读 cause + 尾 `%w ErrBusy`（不删可读文本）。
- **C-5**：用例数**不降**；本任务为净新增补测，bump baseline go_tests/test_count（+净新增）+ version；go_tests ≤ 实际 `go test -list` 计数。
- **C-6**：dev 改动后再 grep `mapProcErr`（确认仅 2 调用点）+ grep procmgr `fmt.Errorf` 状态相关返回（对账忙返回点全集=Start:171 唯一）。
- **C-7**：06 含裸标题 `## Adversarial tests`（procmgr 文本变化不影响分类 + 500 不泄露 + 忙态仍 409 反向证伪）。
