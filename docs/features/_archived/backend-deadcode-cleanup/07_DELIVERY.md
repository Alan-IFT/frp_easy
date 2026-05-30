# 07_DELIVERY — T-045 backend-deadcode-cleanup

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05

## 需求

清理后端死代码，降低关键文件（尤其 procmgr）的认知负担与"看似已接通实则没有"的误导。

## 改动（均经全仓 grep 确认无生产消费方）

- **F-4 procmgr 发布订阅整套删除**：`StatusEvent` 类型、`subMu`/`subscribers` 字段、`Subscribe()`、`emit()` + 5 处 hot-path `emit` 调用（Start 成功/失败、Stop ×2、supervise 退出、waitUntilStable）。生产零订阅者（前端走 `/proc/status` 轮询），`emit` 每次状态变更都锁 `subMu` 广播给空列表 —— 纯开销 + 误导"状态推送已接通"。连带删唯一测试 `TestSubscribe_NonBlockingDrop`（测的就是被删的机制），并清理 `shouldEmit` 变量 + 级联孤立的 `info` 快照局部变量 + 孤立 import（`runtime`/`errors`）。
- **F-7 死函数删除**：`config_helper.go::proxyToFrpconf`（零调用，renderAndApplyFrpc 内联了同款转换）+ `handlers_proxies.go::maybeApplyConfig`（零生产调用，注释自称"向后兼容保留"）。连带删孤立 import（`storage`）。
- **F-6 导入抑制 hack 删除**：`frpcadmin/client.go` 的 `var _ = strconv.Itoa` + `manager.go` 的 `var _ = runtime.GOOS` / `var _ = errors.New`，连同被它们强行保活的 `strconv` import。这类空白赋值是反模式——掩盖"这些 import 当前无用"的事实、让 goimports/linter 失效。

净删除约 90 行（含 ~50 行发布订阅）。

## 保留决策（F-5 不删）

`frpcadmin.Status`/`ProxyStatus`/`ErrUnauthorized` 当前仅测试使用，但**保留**：它是有完整单测、镜像真实 frpc admin `/api/status` 端点的客户端方法，与 frps 侧运行态监控（T-039~042）对称、具明确未来价值（frpc 监控页）；删除会无谓丢失覆盖（红线#3）。

## 验证

- `go build ./...` + `go vet ./...`：clean（修复了删除级联出的 2 个孤立 import：config_helper 的 storage、manager_test 的 time）。
- `go test ./...`：全 PASS。Go 顶层测试 285→284（删 TestSubscribe_NonBlockingDrop）。
- `bash scripts/verify_all.sh`（完整含 e2e）：**PASS 32 / WARN 0 / FAIL 0**。B.4 对新基线（go_tests=284）PASS。
- baseline.json 同步 v18：go_tests 284 / test_count 581（PM 批准删除死代码的死测试，红线#3 显式例外，已在 notes 记录）。

## Adversarial tests

- 删除前 grep 全仓确认 `Subscribe`/`StatusEvent`/`proxyToFrpconf`/`maybeApplyConfig` 在生产代码（排除 _test.go）零引用 —— 这是删除型任务的"反向证伪"：若有任何生产消费方，grep 会命中、go build 会断。
- 删除后 `go build` + `go vet` 全绿证明无悬挂引用；procmgr 的 Start/Stop/Restart/supervise 行为由既有 manager_test.go + qa_t007_adversarial + e2e（C.1）守门，全 PASS 证明删 emit 未改进程生命周期语义。

## Insight

- 关键文件里的死代码比普通死代码危害更大：procmgr 的发布订阅让维护者误以为"状态推送已接通"，实则 5 处 emit 广播给空列表。删除型清理必须配 grep 全仓确认零生产消费 + go build/vet 兜底悬挂引用。
- `var _ = pkg.Symbol` 形式的"导入保活 hack"是反模式：它假装某 import 有用，实际掩盖了"当前无用"，并让 goimports/linter 失效。需要时直接加回 import 即可，不该预先保活。
- 删除死代码的死测试导致 go_tests 计数下降，与 B.4 的"测试数只升不降"张力：正解是 PM 显式批准 + baseline.json notes 记录例外（区别于"为过测删活测试"的红线违规）。B.4 仍守住"意外/静默下降"。
