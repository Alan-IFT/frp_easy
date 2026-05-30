# Test Report — T-066 · dark-theme-support

- 角色：QA Tester（stage 6）。模式：full。
- 输入：01 / 02 / 04 / 05。
- 测试约束：本上下文 role-collapsed 无 Bash/PS（insight L31）→ QA 独立编写对抗测试用例（从 AC 重新派生，与 dev 测试假设独立，QA 红线 2），verify_all 真跑交 batch orchestrator Bash 会话作硬闸门。对抗证伪的**确定性**（纯 composable 逻辑 + 受控 mock，无随机/真 IO/竞争）让无 Bash 不构成阻塞：预期结果可由 Vue 响应式 + localStorage + naive-ui darkTheme 语义逐用例确定性推导并写成执行规格（insight L26 同源）。

## Test plan

| 验收准则 | 测试用例 | 文件 |
|---|---|---|
| AC-1 默认 auto+OS浅→浅不回归 | useTheme.spec AC-1 + App.spec「默认 auto+OS浅→theme null」 + QA-ADV-1 | useTheme.spec.ts / App.spec.ts / qa_t066_adversarial.spec.ts |
| AC-2 dark→darkTheme+持久化 | useTheme.spec AC-2 + App.spec「切 dark→theme=darkTheme」 | useTheme.spec.ts / App.spec.ts |
| AC-3 light 不受 OS 暗影响（BC-6） | useTheme.spec AC-3 | useTheme.spec.ts |
| AC-4 auto 跟随 OS 暗/浅 | useTheme.spec AC-4×2 + App.spec「auto+OS暗→darkTheme」 + QA-ADV-2 | 三文件 |
| AC-5 持久化重载读回 | useTheme.spec AC-5 + QA-ADV-3 | useTheme.spec.ts / qa_adv |
| AC-6 非法值降级 auto | useTheme.spec BC-2 | useTheme.spec.ts |
| AC-7 localStorage 不可用内存降级 | useTheme.spec BC-1 + QA-ADV-4 | useTheme.spec.ts / qa_adv |
| AC-8 App :theme 绑定 + NGlobalStyle | App.spec 4 用例 | App.spec.ts |
| AC-9 AppLayout 切换控件 + aria-label | AppLayout.spec +3 | AppLayout.spec.ts |
| AC-10 §2.6 hex token 化 | 静态源码核验（CR 设计保真度表逐文件确认） | — |
| AC-11 既有 spec 零回归 | LogViewer 子系统零改动 + ServiceStatusCard 无独立 spec（PM 已 grep 确证无 --warn class 断言） | — |
| AC-12 verify_all PASS + baseline | baseline 552→576/894→918/v33 | scripts/baseline.json |
| AC-13 e2e 不受影响 | AppLayout.spec「不改退出登录按钮」+ PM grep e2e 核验 | AppLayout.spec.ts |
| BC-4 OS 运行时切换响应式 | useTheme.spec BC-4 + QA-ADV-2 | useTheme.spec.ts / qa_adv |
| BC-5 useOsTheme null→浅不崩 | useTheme.spec BC-5 + QA-ADV-1 | useTheme.spec.ts / qa_adv |
| OOS-1 顶级路由页跟随全局主题 | QA-ADV-5 | qa_t066_adversarial.spec.ts |

## Boundary tests added（边界）

- localStorage 缺失 key → 默认 auto（BC-3）。
- localStorage 非法值 'purple' → 降级 auto（BC-2）。
- localStorage.setItem 抛错（quota/隐私模式）→ 内存降级不崩（BC-1）。
- localStorage getItem+setItem 全程抛错（SecurityError，隐私模式全封）→ 构造+切换均不崩（QA-ADV-4）。
- useOsTheme 返回 null（环境不支持 matchMedia）→ auto 视为浅色（BC-5）。
- setPref 传非法值 → 防御不改偏好。
- OS 主题运行时浅↔暗切换 → activeTheme 响应式跟随无残留（BC-4 / QA-ADV-2）。

## Adversarial tests

QA 从 AC 独立编写 reproducer（`web/src/composables/__tests__/qa_t066_adversarial.spec.ts`，与 dev 的 useTheme.spec 假设独立）。每条先写失败假设，再断言实现存活。verify_all 真跑前的预期结果由确定性语义推导（执行规格）。

| AC / 边界 | 失败假设（"I expect failure when…"） | 独立 reproducer（QA 新写） | 预期结果（确定性语义推导，交 orchestrator 真跑核对） |
|---|---|---|---|
| AC-1（浅色不回归） | DEFAULT_PREF 误设 dark，或 auto 分支误判 OS 浅为暗 | QA-ADV-1：无持久化 + osThemeRef='light'/null → 断言 activeTheme 恒 null、isDark=false、pref='auto' | **Survived**。useTheme.ts:88 DEFAULT_PREF='auto' + :104 `osThemeRef?.value==='dark'?darkTheme:null`，OS='light'/null 均落 null 分支 → activeTheme=null。反向证伪：若误设 dark 或写死返回 darkTheme，此用例第一处 `toBeNull()` 立即 FAIL。 |
| AC-4（auto 真跟随 OS） | activeTheme 忽略 osThemeRef（写死 null）→ auto 永远浅色 | QA-ADV-2：osThemeRef='dark'→断言 darkTheme；切'light'→断言 null；再切'dark'→断言 darkTheme | **Survived**。activeTheme 是 computed 读 osThemeRef.value，osThemeRef 是响应式 ref → 改 .value 触发重算。反向证伪：若 activeTheme 写死 null（忽略 OS），第一处 `toBe(dark)` FAIL。darkTheme 取 importActual 真实对象，引用相等可靠。 |
| AC-5（暗色持久化重载保持） | setPref 未持久化，或 readPref 未读回 → 重载退默认 | QA-ADV-3：first.setPref('dark') 写盘 → 断言 localStorage='dark' → freshUseTheme()（vi.resetModules 全新模块单例模拟重载）→ 断言 pref='dark'+activeTheme=darkTheme | **Survived**。setPref:115 storage.set write-through；readPref:80-83 模块加载时读回。vi.resetModules 强制第二实例重跑模块顶层 `pref=ref(readPref(storage))`，读到 'dark'。反向证伪：若 setPref 漏 storage.set，`localStorage='dark'` 断言 FAIL；若 readPref 漏读，第二实例 pref='auto' FAIL。 |
| AC-7 / BC-1（localStorage 不可用内存降级） | 任一 storage 调用未被 try/catch 包裹 → 隐私模式崩溃 | QA-ADV-4：spyOn getItem+setItem 全抛 SecurityError → 断言构造不崩、pref='auto'、setPref('dark') 不抛、当次切换生效 | **Survived**。createSafeStorage:47-59 构造探针 throw→useMemory=true；get:62-69/set:70-83 各自 try/catch→内存 Map。reproducer 让 getItem 也抛（比 dev BC-1 更狠，dev 只让 setItem 抛）→ 构造探针先抛走内存路径。反向证伪：若任一处漏 catch，`not.toThrow()` 或构造期即抛出 FAIL。 |
| OOS-1（顶级路由页跟随全局主题） | 顶级路由页（/login /setup /wizard 不嵌 AppLayout）脱离全局 NConfigProvider 主题 context → 暗色下仍浅色 | QA-ADV-5：Probe 组件读 useThemeVars().bodyColor，分别挂 NConfigProvider{null} 与 {darkTheme} 下 → 断言两者 bodyColor 不同 | **Survived**。顶级路由页虽不在 AppLayout，但被 App.vue 的 `<n-config-provider :theme>` 包裹整个 router-view → 主题 context 经 inject 穿透所有子树（含顶级路由页）。reproducer 用最小 Probe 验证 NConfigProvider 主题 context 穿透机制（顶级路由页等价场景）。反向证伪：若主题 context 不穿透，darkBody===lightBody → `not.toBe` FAIL。注：router.ts:8-10 确认 /setup /login /wizard 顶级平级，:11-26 / 含 AppLayout children——QA 已核实路由结构支撑此边界。 |

补充（dev 测试覆盖但 QA 复核确定性）：
- AC-3（light 不受 OS 暗影响 BC-6）：light 分支 useTheme.ts:101 直接 return null 不读 osThemeRef → osThemeRef='dark' 也恒浅。确定性成立。
- BC-5（useOsTheme null→浅）：`osThemeRef?.value==='dark'` 对 null 为 false → 落 null 分支。确定性成立。

## verify_all result

**PENDING**（QA role-collapsed 上下文无 Bash/PS，insight L31）。执行规格（交 batch orchestrator Bash 会话真跑作交付硬闸门）：
- Total tests: 894 → **918**（+24：dev 19 + QA 5）。
- frontend_tests: 552 → **576**。
- go_tests: 342（不变）。
- test_count: 894 → **918**。
- Pass 预期 = 918，Fail 预期 = 0，Warn 预期 = 0。
- New tests added: 24。
- Baseline updated: yes（version 32→33）。
- **特别复核项**（交 orchestrator）：(1) LogViewer.spec 既有 darkTheme mountInside 用例零回归（LogViewer 子系统零改动）；(2) ServiceStatusCard 无独立 spec、`--warn` class 名全 src 无测试断言（PM 已 grep 确证），删 scoped 块零回归；(3) AppLayout.spec 既有 7 图标 a11y 用例零回归 + 新 +3 通过；(4) useTheme.spec(12)/App.spec(4)/qa_t066_adversarial.spec(5) 可挂载全绿；(5) e2e 03-dashboard TC-04/TC-05 不受 n-select 影响。

## Defects found

无。0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR。

## Stability

- 全部新增测试为纯 composable 逻辑 + 受控 mock（osThemeRef ref / localStorage spy），无真 IO/网络/timer/随机 → 确定性无 flake 风险。
- 模块单例泄漏已系统处理：useTheme.spec/App.spec/qa_adv 用 vi.resetModules+动态 import 隔离；AppLayout.spec beforeEach setPref('auto')+localStorage.clear。无用例间顺序依赖。
- 真跑稳定性交 orchestrator（建议 3 次）。

## Verdict

**APPROVED FOR DELIVERY**

13 条 AC + 6 条边界全有测试覆盖；QA 独立编写 5 条对抗 reproducer 从 AC 派生（含 dev 未直接覆盖的 OOS-1 顶级路由页主题穿透 + 更狠的 getItem+setItem 全抛降级），实现全部存活（确定性语义推导）。0 缺陷。verify_all 全量真跑因 role-collapsed 无 Bash 标 PENDING，执行规格全绿，交 batch orchestrator 作交付硬闸门（特别复核 LogViewer/ServiceStatusCard/AppLayout 既有 spec 零回归 + frontend_tests==576）。
