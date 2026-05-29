# BATCH_LOG — project-optimization-2026-05

2026-05-30 · batch 启动 · baseline=29 PASS / 2 FAIL (B.3 前端39失败 / E.6 缺3对抗段) · 范围=全面深挖 · 10 任务
2026-05-30 · T-043 · frontend-test-suite-repair · dispatching · mode=full
2026-05-30 · T-043 · DELIVERED · 39 前端失败→0（getExposed/apiError test-utils + 7 spec 健壮化）· verify_all --quick PASS 30 / FAIL 1（仅 E.6 pre-existing，待 T-044）
2026-05-30 · T-044 · DELIVERED · .ps1 B.3 真查退出码 + B.4 双实现真计数(go test -list + vitest)·baseline 刷新 285/297/582·E.6 三报告标题去前缀·B.4 反向证伪通过·verify_all.sh --quick PASS 31 / FAIL 0（基线恢复绿色）
2026-05-30 · T-052 · DELIVERED · e2e 改独立端口 17800（env 可覆盖）+ webServer.env 注入 + 双 start 脚本对称 + auth.ts 文案更新 · 用户 frp-easy 占 7800 时 e2e 5/5 过 · 完整 verify_all.sh PASS 32 / FAIL 0（含 C.1 e2e，首次本机全绿）
2026-05-30 · T-045 · DELIVERED · 删 procmgr 发布订阅(Subscribe/emit/StatusEvent+5调用点)+死函数(proxyToFrpconf/maybeApplyConfig)+3 var _ hack+孤立 import 净~90 行 · go_tests 285→284(删死测试,PM批准) · 完整 verify_all PASS 32/0/0
2026-05-30 · T-046 · DELIVERED · 过期 session 定时清理 loop(随 rootCtx 取消) + RequestID 改 crypto/rand · +3 Go 测试(go_tests 284→287) · 完整 verify_all PASS 32/0/0
2026-05-30 · T-047 · DELIVERED · Server/Client 加载+错误三态(失败态不渲染表单防误覆盖) + Proxies 失败/空区分 + Dashboard 开关不静默(disabled+tooltip+重试) + Server dashboard 三字段校验 · frontend_tests 297→327 · 完整 verify_all PASS 32/0/0 · dev-frontend 子 agent 实现
2026-05-30 · T-048 · DELIVERED · 删重复 formatBytes + formatTime 本地化统一(三份归一) + ServiceStatusCard 可读语义色 + Dashboard router.push/响应式/进程文案统一 + PublicIpDetector extractErrorMessage · frontend_tests 327→342 · 完整 verify_all PASS 32/0/0 · dev-frontend 子 agent 实现
2026-05-30 · T-050 · DELIVERED · +21 Go 测试(validate.go/procmgr 终态断言/autoRestore/svcprobe/procmgr 生命周期 helper) go_tests 287→308 · 发现 retryRestoreLoop canceled-persist bug(报告不修,→T-053) · 完整 verify_all PASS 32/0/0 · dev-backend 子 agent 实现
