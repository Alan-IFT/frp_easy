# 07_DELIVERY — T-049 docs-contract-drift-fix

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05

## 需求

修复文档/契约与实现的漂移（文档审计 F-1/F-5/F-6/F-7/F-8），让契约文件与导航对新人/集成方诚实。

## 改动

- **F-1 + F-8**（openapi.yaml）：补 `GET /api/v1/system/service-status` 路径 + `ServiceStatus` schema（精确匹配 `SystemServiceStatusResponse` 的 wire 形状：supervised/supervisor(enum)/boot_autostart/run_as/probe_error?/auto_restore{enabled_kinds,last_runs}）。这是 T-038 上线且前端 ServiceStatusCard 在用、却漏在 openapi 的 live 路由 —— 补齐后 README:40"openapi 描述全部 REST 路由"重新为真（F-8 随之消解，无需改 README）。openapi 现 30 个 path，js-yaml 解析校验通过。
- **F-5**（dev-map 目录树）：树中补 `internal/svcprobe/`（T-038，此前仅在 prose 表出现）+ `web/src/utils/`（format.ts/proxyStatus.ts）+ `web/src/test-utils/`（T-043 getExposed/apiError helper），消除"树说不存在、prose 说存在"的自相矛盾。
- **F-6**（dev-map）：行 17 openapi "28 条路由" 改为"覆盖全部 REST 路由…30 个 path，T-049 与 router.go 对齐"；行 51 httpapi 树注解的"T-001:22 条；T-002:+5 条"硬编码改为指向下方 router.go 行的累计口径（消除与行 149 准确 prose 的自相矛盾）。
- **F-7**（project-status.html / architecture.html）：两份冻结在 T-010/T-011 的总览页顶部各加橙色"时效声明"横幅，明示未反映 T-012~T-053、并指向 docs/tasks.md + docs/features/_archived/ + dev-map.md。

## 验证

- openapi.yaml：`js-yaml` 解析 OK，30 paths，service-status 路径 + ServiceStatus schema 均存在。
- `bash scripts/verify_all.sh`（完整含 e2e）：**PASS 32 / WARN 0 / FAIL 0**（D.1 OpenAPI present PASS）。纯文档变更，测试数不变。

## Adversarial tests

- 契约对齐反向核对：service-status schema 字段名/类型逐一对照 `SystemServiceStatusResponse`/`SystemAutoRestoreSection` 的 json tag（snake_case：boot_autostart/run_as/auto_restore/enabled_kinds/last_runs），杜绝"补了但形状不符"。

## Insight

- "契约文件描述全部路由"的声明（README/openapi）必须有机制守门，否则新路由（service-status）漏登、声明变假却无人发现。理想是 verify_all 加一道"router.go 路由集 == openapi paths 集"的静态闸门（本任务未做，建议未来 T 候选）。
- 文档自相矛盾（dev-map 树 vs prose）的根因是"树是手画快照、prose 增量更新"两条更新路径不同步。修法是树只放结构、增删数字一律指向单一权威（router.go 行），避免在两处各维护一份会漂移的计数。
- 大型冻结文档（project-status/architecture.html）与其重生成，不如加显眼时效声明 + 指向持续更新的 dev-map/tasks —— 低成本止损"半年前快照被当现状"。
