# 06 测试报告 — T-058 frontend-interaction-polish

- **测试角色**: qa-tester
- **日期**: 2026-05-30
- **输入**: 01 AC + 04 开发 + 05 评审

## 1. 测试矩阵（+27 前端测试，frontend_tests 454→481 / test_count 772→799）

### (A) 剪贴板失败反馈

| 用例 | AC | 文件 |
|---|---|---|
| FirewallHint copyCmd writeText 成功 → message.success + "已复制 ✓" | A1 | FirewallHint.spec |
| FirewallHint copyCmd reject + execCommand true → message.success | A2 | FirewallHint.spec |
| FirewallHint copyCmd reject + execCommand false → message.error | A2/A4 | FirewallHint.spec |
| FirewallHint copyAll 成功 → message.success + "已复制全部 ✓" | A1 | FirewallHint.spec |
| FirewallHint copyAll reject + execCommand false → message.error | A2 | FirewallHint.spec |
| PublicIpDetector copyIp writeText 成功 → message.success + "已复制 ✓" | A3 | PublicIpDetector.spec |
| PublicIpDetector copyIp reject + execCommand true → message.success | A3 | PublicIpDetector.spec |
| PublicIpDetector copyIp reject + execCommand false → message.error | A3/A4 | PublicIpDetector.spec |

### (B) 重置防误丢

| 用例 | AC | 文件 |
|---|---|---|
| 文案"重新加载"而非"重置" | B1 | Server/Client.spec |
| 不 dirty → 直接重载（apiGet 再调 +1，不弹确认） | B3 | Server/Client.spec |
| dirty → handleReloadClick 弹确认且 apiGet 未调 | B2 | Server/Client.spec |
| dirty + confirmReload → apiGet 再调并覆盖回真实值，isDirty 归零 | B2 | Server/Client.spec |
| dirty + 取消（不调 confirmReload）→ apiGet 不调，编辑保留 | B2 | Server/Client.spec |
| loadedSnapshot 每次成功加载后刷新（仅 Server） | B4 | Server.spec |

### (C) 死分支清理

| 用例 | AC | 文件 |
|---|---|---|
| selectedRole='frpc' → 恰一次"frpc 客户端配置"标题，无 frps 段 | C1/C2 | Wizard.spec |
| selectedRole='both' → 恰一次"frpc 客户端配置"标题 + frps 段 | C1/C2 | Wizard.spec |
| selectedRole='frps' → 不渲染"frpc 客户端配置"标题 | C2 | Wizard.spec |

## 2. 断言方式合规（insight L45 / T-057 教训）

- 全部 27 个新断言**零** naive-ui 组件名查询（无 `findComponent({name:...})` / `findAllComponents({name:...})`）。
- 按钮按可见文本定位（`findAll('button').find(b => b.text().includes(...))`）；文案断言用 `wrapper.text()`；DOM 节点用 `document.querySelectorAll('textarea[aria-hidden]')`；句柄用 `getExposed`；apiGet 调用用 `mock.calls.length`。
- message 用 `vi.mock('naive-ui')` 单例 spy。clipboard 用 `navigator.clipboard.writeText` mock + `document.execCommand` mock（happy-dom 默认无 execCommand，beforeEach 显式装上）。

## 3. e2e 影响评估（C-3 闭环）

- grep `web/tests/e2e/{01-setup,02-auth,03-dashboard}.spec.ts` + fixtures：无任何对"重置"/"重新加载"/"frpc 客户端配置"/"复制"的断言。
- 03-dashboard 用 `bypassWizard(page)` 调 wizard/complete API 绕过向导，不渲染 Wizard step2 标题。
- 本改动（纯前端 SFC 交互文案 + clipboard fallback + dirty 确认）不触 login/dashboard/proxies 核心 e2e 路径。**预判：e2e 不受影响**，由 orchestrator 全量真跑确认。

## 4. 已知局限

- **dirty 检测不覆盖 AllowPortsEditor 子组件内部行编辑状态**（Server.vue，决策 D2）：若用户**仅**改了端口策略行（增删/编辑）而未动 6 个标量字段，`isDirty()` 返回 false → "重新加载"直接重载、不弹确认 → 该 allowPorts 编辑可能被静默覆盖。
  - 缓解：allowPorts 增删行是显式操作用户感知强，误丢风险远低于输入框；纳入 dirty 需 AllowPortsEditor 暴露 dirty 句柄会扩散范围外文件。记为可接受局限 + backlog。

## 5. verify_all（orchestrator Bash 会话全量真跑作硬闸门）

- 本任务加测试已 bump baseline（frontend_tests 481 / test_count 799），B.4 计数闸门匹配。
- 全量 `bash scripts/verify_all.sh`（含 e2e）结果回填本节由 orchestrator 执行。

## Adversarial tests

反向证伪用例（裸标题，无前缀 —— insight L40）：

- **ADV-A1（A，关键）**：FirewallHint copyCmd —— `navigator.clipboard.writeText` reject + `document.execCommand` **抛异常**（双重失败）→ 断言 `message.error('复制失败：请手动选择文本复制')` 被调、`message.success` 未被调、不抛未捕获错误、不显示"已复制 ✓"假反馈。守门"clipboard reject → fallback 也失败 → message.error 出现"这条核心证伪路径。
- **ADV-A2（A）**：PublicIpDetector copyIp —— 同款双重失败 → message.error，不抛。
- **ADV-A3（A）**：FirewallHint fallback 失败后临时 textarea 必须从 DOM 移除（`querySelectorAll('textarea[aria-hidden="true"]').length === 0`），守门 finally removeChild 不残留隐藏节点。
- **ADV-B1（B）**：Server.vue dirty（改 bindPort + authToken）时 `handleReloadClick` 绝不静默重载——断言 `apiGetServer` 调用次数不变、authToken 编辑保留、仅 `reloadConfirmShow=true`。若实现退回"直接 loadConfig"则此用例 FAIL。
- **ADV-B2（B）**：Client.vue dirty（改 authToken）同款不静默重载证伪。

全部 Adversarial 用例预期 PASS（实现满足）；其设计意图是**当未来回归引入静默丢弃 / 静默吞错时立即变红**。
