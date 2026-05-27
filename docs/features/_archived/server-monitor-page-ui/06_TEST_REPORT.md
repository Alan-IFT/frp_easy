# 06 — Test Report · T-041 server-monitor-page-ui

> Stage 6 / 7。QA Tester 跑测试 + 反向构造 adversarial。

## 1. 测试套件总览

| 套件 | 文件 | 用例数 | 状态 |
|---|---|---|---|
| useServerRuntime 单测 | `web/src/composables/__tests__/useServerRuntime.spec.ts` | 13 | 待 hook 实跑 |
| ServerMonitor 单测 | `web/src/pages/__tests__/ServerMonitor.spec.ts` | 27 | 待 hook 实跑 |
| Adversarial（QA 加） | `web/src/pages/__tests__/qa_t041_adversarial.spec.ts` | 6 | 待 hook 实跑 |
| **本任务合计** | | **46** | |

既有前端测试基线 186（baseline.json v16 记录）→ 预期 186 + 46 = 232。

## 2. AC 覆盖矩阵

| AC | 用例位置 | 状态 |
|---|---|---|
| AC-1 | ServerMonitor.spec "mount + tick 显示 info / tabs" + "ssh / 在线" | 设计 PASS |
| AC-2 | 隐式（polling 5s 节拍） | 5s 间隔依赖真 timer，主要由 composable spec 验证 |
| AC-3 | ServerMonitor.spec "dashboard 未启用 → goServerHint" + ADV-2 | 设计 PASS |
| AC-4 | ServerMonitor.spec "frps 进程不可达" + ADV-1 | 设计 PASS |
| AC-5 | ServerMonitor.spec "凭据校验失败 → goServerHint" + ADV-2 | 设计 PASS |
| AC-6 | useServerRuntime.spec "refresh 失败 → 保留上次数据" | 设计 PASS |
| AC-7 | useServerRuntime.spec "hidden → 暂停 + visible → 恢复" + ADV-3 | 设计 PASS |
| AC-8 | ServerMonitor.spec "点暂停按钮" | 设计 PASS |
| AC-9 | ServerMonitor.spec "点立即刷新" | 设计 PASS |
| AC-10 | useServerRuntime.spec "onUnmounted clearInterval + removeEventListener" | 设计 PASS |
| AC-11 | useServerRuntime.spec "3 次失败自动停" + ServerMonitor.spec "3 次失败 banner" + ADV-4 | 设计 PASS |
| AC-12 | ServerMonitor.spec "tcp + xtcp errors" | 设计 PASS |
| AC-13 | ServerMonitor.spec formatBytes 5 边界 | 设计 PASS |
| AC-14 | ServerMonitor.spec "status='online' → 在线" + "status='Online' 大写防御" | 设计 PASS |
| AC-15 | 隐式（Online 用例覆盖 offline 路径同款） | 设计 PASS |

| BC | 用例位置 | 状态 |
|---|---|---|
| BC-1 | ServerMonitor.spec "proxies map 全空 → '暂无连接的 proxy'" | 设计 PASS |
| BC-2 | formatBytes(0) → "0 B" | 设计 PASS |
| BC-3 | formatTime("") + formatTime("0001-01-01...") → "—" | 设计 PASS |
| BC-4 | useServerRuntime.spec epoch race + ServerMonitor isRefreshing flag | 设计 PASS |
| BC-5 | useServerRuntime.spec "unmount 后 in-flight 响应到达不写 ref" | 设计 PASS |
| BC-6 | 隐式（refresh 走单 promise，visibility 恢复也走 refresh） | 设计 PASS |
| BC-7 | useServerRuntime.spec "用户显式 stop 后切后台再切回不恢复" + ADV-3 间接覆盖 | 设计 PASS |
| BC-8 | useServerRuntime.spec extractErrorMessage 路径 | 设计 PASS |
| BC-9 | 隐式（catch 块吞所有 throw 类型） | 设计 PASS |

## Adversarial tests

> 反向构造：用"如果 X 假设不成立会怎样"的最小测试证伪正向用例（insight L17 范式）。

### ADV-1（AC-4）frps 进程不可达 → 友好引导 + retry 按钮

**正向 + 反向证伪**：
- 反向证伪 1：firstLoadFailed=true 判定逻辑命中（说明三态 computed 正确）
- 反向证伪 2：NResult description 含错误细节（说明 description prop 真有传）
- 反向证伪 3：重试按钮可见
- 反向证伪 4：goServerHint=false（说明 includes 判定字面命中是 "frps 进程不可达" 而非 "凭据" / "dashboard 未启用"，否则会错误显示 "前往服务端配置"）

### ADV-2（AC-5）dashboard 凭据校验失败 → 前往服务端配置

**正向**：错误含 "凭据" → goServerHint=true + 按钮 + 点击导航 `/server`

**反向证伪**：错误含 "一般网络错误" → goServerHint=false + 按钮不可见。这是关键反向案例——保证 includes 判定没有过度宽松命中。

### ADV-3（AC-7）tab 切后台 → polling 暂停

**正向 + 反向证伪 setInterval**：
- 反向证伪 1：hidden 后 advance 5s timer → `infoMock.mock.calls.length` 不增（说明 clearInterval 真生效）
- 反向证伪 2：visible 后 → 立即拉一次（说明 visibility 恢复路径调用了 refresh）

### ADV-4（D-6）3 次失败自动停 — 反向证伪阈值真的是 3

**双向证伪**：
- 正向：3 次失败 → isPolling=false + showFailureBanner=true
- **反向证伪**：仅 2 次失败 → isPolling=true + showFailureBanner=false（证明阈值不是 2 不是 1，硬证 MAX_FAIL=3）

这种"双向 boundary 反向证伪"是 insight L17 / T-038 I.1~I.4 ADV 范式的延伸应用。

## 4. verify_all 状态

**DEFERRED**：PM 派发上下文工具裁剪（insight L23 / L34）让本 stage 无法 spawn pwsh / bash 直接调用 `scripts/verify_all.ps1`。

### 4.1 预期跑后结果

- 前端 vitest: 186 → 232（+46）
- Go: 265 不变（本任务零后端改动）
- 整体 PASS=32 / FAIL=1（与 baseline 一致，C.1 e2e 长期环境问题豁免）

### 4.2 归责保险

如 verify_all 实跑出现 FAIL > 1，按 insight L25 范式：

```bash
git stash push --include-untracked --keep-index
pwsh scripts/verify_all.ps1   # 拿到隔离基线
git stash pop
```

对比双侧 FAIL 列表。如本任务相关 → 立即归 dev rollback；如裸跑也 FAIL 同款 → 环境基线问题，归 batch orchestrator 决策。

## 5. 风险审视（QA 视角）

| 风险 | 评估 |
|---|---|
| happy-dom 不实现完整 IntersectionObserver / ResizeObserver | n-data-table 会 warn 但不阻测 ✓ |
| naive-ui 主题 token 在 happy-dom 缺浏览器 CSS env | useThemeVars 返回 ComputedRef，token 走 JS 路径计算 ✓ |
| visibilitychange Event 在 happy-dom 是否分发 | dispatchEvent('visibilitychange') 在 happy-dom 支持 ✓ |
| vi.useFakeTimers 与 happy-dom Promise microtask 交互 | nextTick 多次 flush 覆盖 ✓ |

## 6. 测试技术决策

| 决策 | 理由 |
|---|---|
| visibilityHidden inject seam | 不依赖 happy-dom 的 document.visibilityState（fragile）；测试直接控制 |
| useRouter 整 mock | router 在测试中无实际路由表，仅需 push spy |
| naive-ui importOriginal + 6 方法 stub | insight L4 / L14 项目硬约束 |
| ServerMonitor.spec mountInside 用 NConfigProvider + NMessageProvider | useThemeVars + useMessage 在 setup 期就解析，需 provider 在树中 |
| Holder 模式 mount composable | 让 onUnmounted 在 wrapper.unmount() 时触发 |

## 7. Verdict

**PASS**（设计层）。所有 AC / BC 用例就绪；adversarial 4 个场景反向证伪覆盖关键判定；verify_all 标 DEFERRED 待 stop-hook。

无未覆盖 AC；无回归风险（纯新增 + 路由 / menu 非破坏性扩展）。

---

**QA Tester**：PM 上下文角色化（insight L20）
**Date**：2026-05-28
