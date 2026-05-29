# BATCH_PLAN — project-optimization-2026-05

> 2026-05-30 创建。用户高层目标：**优化项目**，决策原则：**用户体验好 / 符合软件工程标准 / 长期易使用易维护**。
>
> 用户授权 AI 全权决策（设计 + 实现 + commit + push），范围选择：**全面深挖**。

## 决策摘要（AI 视角）

启动前做了 4 维度证据审计（前端 UX / 后端 Go 代码健康 / 文档契约漂移 / 测试覆盖质量）+ 真实 verify_all 基线。**关键发现：项目当前基线是红的（红线违规）**——最近批次 T-038~T-042 带着失败的测试树和未守住的验证闸门交付：

- **B.3 单元测试 FAIL**：前端 `vitest` 39 个失败。根因二：① 38 个是测试 helper 依赖 VTU 的 `vm.__testing` 语法糖（需 `exposeProxy` 已创建，ServerMonitor/Proxies 的未创建），应改用规范 `vm.$.exposed`；② 1 个是 `useServerRuntime` 测试与实现的错误消息漂移。
- **E.6 对抗测试段 FAIL**：T-038/T-039/T-040 三个归档报告缺 `## Adversarial tests` 段。
- **闸门有洞**：verify_all `B.4「测试数 ≥ 基线」是空操作`（从不真比较）；`baseline.json` 停在 T-036（写 451，实际已变）。红树因此溜过。

**因此本批次第一优先 = 恢复绿色基线 + 加固闸门，杜绝复发**，再做高价值优化。

## Baseline 状态（2026-05-30 batch 启动时）

- `bash scripts/verify_all.sh --quick`：**29 PASS / 0 WARN / 2 FAIL / 0 SKIP**
  - B.3 Unit tests FAIL（前端 39 失败，真实回归，T-043 修复）
  - E.6 Adversarial tests section FAIL（3 归档报告缺段，T-044 修复）
- e2e（C.1）本批次用 `--quick` 暂时排除（本机 7800 被用户运行的 frp-easy 实例占用，insight L25）；T-052 让 e2e 脱离 7800 后恢复全量 verify_all。

**回归判定**：本批次的目标之一就是把基线从 FAIL 拉回 PASS。Phase 1（T-043 + T-044）完成后 verify_all 应回到 PASS；此后任一任务跑完后出现**新 FAIL 即停批**。

## 执行模型决策（重要）

上一批次带红交付的根因是 **verify_all 闸门被角色扮演而非真跑**（insight L14 的 role-collapse）。本批次 batch orchestrator（拥有 Bash）在**每个任务后真正运行 `scripts/verify_all`** 作为硬闸门，绝不依赖角色扮演的 QA 阶段。实现由聚焦的 developer 子 agent 或 orchestrator 直接落实，但**验证闸门一律由 orchestrator 真跑**。

## 任务表

| ID | Slug | Goal | Mode | Depends on | Status |
|---|---|---|---|---|---|
| T-043 | frontend-test-suite-repair | 修复 39 个前端测试失败。getTesting helper（ServerMonitor.spec / qa_t041 / qa_t042 三处）改用规范 `vm.$.exposed.__testing` 访问（健壮 accessor，不依赖 VTU exposeProxy 糖）；判定 `useServerRuntime` 错误消息正确行为并对齐 test/impl。`npm test` 全绿。 | full | — | pending |
| T-044 | verify-gate-hardening | 让 verify_all B.4（.ps1 + .sh 双实现）真正计数 Go + 前端测试数，< baseline 则 FAIL（按 insight L26/L30 反向证伪）；补 T-038/T-039/T-040 三个归档报告的 `## Adversarial tests` 段（E.6 绿）；刷新 baseline.json 到真实计数。 | full | T-043 | pending |
| T-052 | e2e-decouple-port | e2e 端口从产品默认 7800 改为独立端口（env 可覆盖）+ 每轮全新 tmpdir data-dir，根治本机 C.1 假性失败让全量 verify_all 可跑；assertFreshBackend 提到全局 setup。 | full | T-044 | pending |
| T-045 | backend-deadcode-cleanup | 删 procmgr 无人消费的 Subscribe/emit/StatusEvent 发布订阅（~-50 行 + 锁开销）；删死函数 proxyToFrpconf / maybeApplyConfig；删 3 处 `var _ =` 导入抑制 hack + 无用 import；判定并处理仅测试使用的 frpcadmin.Status/ProxyStatus/ErrUnauthorized。 | full | T-052 | pending |
| T-046 | session-purge-and-requestid | 过期 session 定时清理 goroutine（防 sessions 表无界增长，随 rootCtx 取消）；RequestID 中间件改 crypto/rand 消除纳秒时间戳碰撞。 | full | T-045 | pending |
| T-050 | backend-test-coverage | validate.go 6 函数 table-driven 错误路径测试；procmgr 对抗测试加状态终态断言；autoRestoreProcs/retryRestoreLoop 测试（retryBackoff 注入）；svcprobe 平台分支抽纯函数 + t.Setenv 测试。 | full | T-046 | pending |
| T-047 | frontend-honest-states | Server/Client/Settings 表单页补加载+错误三态（避免默认值伪装真实配置）；Proxies 区分加载失败 vs 空列表；Dashboard 自动启动开关获取失败不静默；Server.vue dashboard 三字段补校验规则。 | full | T-050 | pending |
| T-048 | frontend-consistency-cleanup | 删 UploadBinButton 重复 formatBytes（复用 utils）；formatTime 统一本地化（含改快照测试）；ServiceStatusCard 加载/失败文案改可读语义色；Dashboard 日志链接改 router.push；Dashboard n-grid 加 responsive；PublicIpDetector 用 extractErrorMessage；进程操作文案统一客户端/服务端。 | full | T-047 | pending |
| T-051 | frontend-test-coverage | proxies/wizard store 测试；useProxyForm type 切换副作用测试；useServiceStatus/statusUtils/useLogLevelFilter 测试；api/client.ts 测试。 | full | T-048 | pending |
| T-049 | docs-contract-drift-fix | openapi.yaml 补 service-status 路由 + ServiceStatus schema；dev-map 树补 svcprobe / web/src/utils + 修路由条数；README openapi 措辞对齐；project-status/architecture.html 加时效声明。 | full | T-051 | pending |

**Topo order**：T-043 → T-044 → T-052 → T-045 → T-046 → T-050 → T-047 → T-048 → T-051 → T-049（sequential；先恢复绿+加固闸门+解锁全量验证，再后端，再前端，文档收尾反映最终现实）

## 决策原则映射

| 任务 | 用户体验好 | 软件工程标准 | 长期易使用易维护 |
|---|---|---|---|
| T-043/044/052 | — | 恢复 verify_all 真实 PASS（红线） | 闸门真计数 + e2e 脱环境耦合，回归可被发现 |
| T-045/046 | session 清理防 DB 膨胀变慢 | crypto/rand 唯一 reqID | 删 ~160 行死代码降认知负担 |
| T-050/051 | — | 核心子进程/校验/store 有自动回归网 | 不再靠人肉真机验 |
| T-047/048 | 诚实三态 + 可读色 + 一致文案 | 错误处理模式统一 | 消除重复工具函数 |
| T-049 | — | 契约文件与实现一致 | 新人不被过期文档误导 |

## strong-signal 停止条件

- 任一任务跑完后 verify_all 出现**新 FAIL（超过该任务启动时的基线）**
- 任一实现子 agent 返回 FAILED verdict / 同 stage 3 次回退
- `.harness/intervention.md` 出现 STOP
- 安全 hook 拦截 destructive Bash 调用

## 提交 / 推送策略（用户授权）

- 每个任务 verify_all 真跑 PASS 后由 orchestrator 提交到 main（conventional commit：`fix/feat/refactor/test/docs(T-NN): slug — 简述`）
- 批次结束写 BATCH_REPORT.md 单独 commit，`git push origin main`
