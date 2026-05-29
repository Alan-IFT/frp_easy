# 07_DELIVERY — T-050 backend-test-coverage

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05 · dev-backend 子 agent 实现，orchestrator 真跑 verify_all 闸门

## 需求

给项目最核心却几乎无自动化测试、靠人肉真机验证的后端代码补回归网（测试审计 A-1~A-4 + C-3）。

## 改动（+21 Go 顶层测试，287→308）

- **A-4** `internal/httpapi/validate_test.go`（新，6 Test）：6 个校验函数 table-driven 错误路径（空/超长/非法字符/边界端口 0·65536·1·65535/非法域名/枚举外类型/弱密码）。
- **C-3** `internal/procmgr/qa_t007_adversarial_test.go`（改，强化）：3 个并发用例补"收敛后 `Status().State` 落合法终态集 + `StatusAll` 长度恒=2"断言，把"不死锁"升级为"状态正确"。未删/弱化现有断言。
- **A-3** `cmd/frp-easy/autorestore_test.go`（新，5 Test）：binary-missing / config-missing 不进 retry、first-fail→exhausted（attempts==1+len(retryBackoff)、index 递增）、canceled 短路、disabled 不写 kv。注入短 backoff，poll-until + deadline 同步（无固定 sleep）。
- **A-2** svcprobe 可测化：`probe.go` 抽纯函数 `parseIsEnabled`、`probe_linux.go` 抽 `supervisedFromEnv`/`runAsFrom`（均字节级行为等价，调用点等价替换）；`parse_test.go`（平台无关，1 Test）+ `probe_linux_test.go`（`//go:build linux`，3 Test，`t.Setenv` 测分支）。
- **A-1** `internal/procmgr/lifecycle_helper_test.go`（新，9 Test）：因 procmgr 硬编码 `exec.Command(binPath,"-c",cfgPath)` 致标准 `TestHelperProcess+os.Args[0]` 模式不可用（testing flag 解析器拒 `-c`），改为测试时 `go build` 一个独立 helper 程序走完整 spawn 路径。覆盖 Start→Running、崩溃→Error+LastErr、Stop→stopped/PID=0、Restart PID 变化 + ringBuffer/waitUntilStable 纯单元。4 个慢 spawn 测试 `testing.Short()` 门控（Windows 真机全 PASS）。

## 验证

- `go build ./...` / `go vet ./...`：净。`go test ./...`：全 PASS。Go 顶层 287→308。
- orchestrator 真跑 `bash scripts/verify_all.sh`（完整含 e2e）：**PASS 32 / WARN 0 / FAIL 0**。baseline.json v22（go_tests 308 / test_count 650）。
- `-race`：本环境无 C 编译器（cgo 不可用）未跑；新增并发断言在 `wg.Wait()` 后调走锁的 Status，构造上 race-safe。**待带 C 编译器环境补跑 `CGO_ENABLED=1 go test -race ./internal/procmgr/...`**。

## Adversarial tests

- validate：每函数 valid + 多类 invalid 双侧；端口 0/65536 拒、1/65535 过的边界。
- procmgr 并发：收敛后终态集断言（而非仅不死锁）。
- autoRestore：exhausted 断言 attempts 精确计数 + index 递增；canceled 短路。
- A-1：崩溃进程 → Error + LastErr 非空（反向证伪"成功转换"逻辑）。

## 发现的疑似 bug（已报告，由 T-053 修复）

`cmd/frp-easy/main.go::retryRestoreLoop` 的 canceled 分支用**已取消的 ctx** 调 `persistAutoRestoreLast` → `context.WithTimeout(已取消ctx,5s)` 立即失效 → `KVSet` 因 `context.Canceled` 必失败 → UI 的 `GET /system/service-status` 永远看不到 `canceled` 这条 last_run。本任务 A-3 的 canceled 用例已规避依赖此 buggy 行为；修复在 T-053。

## Insight

- 有副作用的代码（子进程 spawn、平台探测、boot 自恢复）也能测：(1) 平台分支抽纯函数 + `t.Setenv` 注入；(2) 真 spawn 用编译独立 helper 程序（当被测代码硬编码自定义 flag、标准 `TestHelperProcess` 不可用时）；(3) 慢 spawn 测试 `testing.Short()` 门控。
- 同步点禁用固定 `time.Sleep`（脆弱），用 poll-until-condition + deadline。
- 加测试常顺带暴露 bug：A-3 写 canceled 用例时发现 retryRestoreLoop 的 canceled-persist 用错 ctx —— "为可测性细看代码"本身就是发现缺陷的高效路径。
