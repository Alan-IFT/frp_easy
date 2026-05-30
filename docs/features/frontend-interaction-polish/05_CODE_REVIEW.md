# 05 Code Review — T-058 frontend-interaction-polish

- **评审角色**: code-reviewer
- **日期**: 2026-05-30
- **输入**: 02 设计 + 03 conditions + 04 开发 + 实际 diff（5 SFC + 5 spec + baseline + dev-map）

## 1. 范围合规

| 检查 | 结论 |
|---|---|
| 改动文件白名单（AC-X1） | PASS — FirewallHint.vue / PublicIpDetector.vue / Server.vue / Client.vue / Wizard.vue + 5 spec + baseline.json + dev-map.md（dev-map 为 docs 同步，允许）。无后端/store/路由/util/AppLayout 触碰 |
| 未创建 utils/clipboard.ts（D1） | PASS — 内联实现，未扩散 |
| 红线 .claude/CLAUDE.md/.github 未碰 | PASS |

## 2. 逐处实现审查

### (A) 剪贴板失败反馈
- FirewallHint.vue `copyText`：与 LogViewer:147-171 范式一致（try writeText→success；catch→textarea+execCommand→ok?success:error）。`finally { removeChild }` 保证临时节点清理，execCommand 抛异常也兜（内层 try/catch）。✅
- `copyCmd`/`copyAll`：`if (await copyText(...))` 仅成功置短暂态——保留"已复制 ✓"且失败不假反馈。✅
- PublicIpDetector.vue：同款，`copyIp` 先 `if (!result.value?.ip) return` 守门后调 copyText。✅
- 空 `catch {}` 已全部消除。✅

### (B) 重置防误丢
- Server.vue `isDirty()`：6 字段逐一 `!==` 比较，`snap == null` 返回 false（首挂载前安全）。✅
- `handleReloadClick()`：dirty → 只置 `reloadConfirmShow=true`（**绝不在此直接 loadConfig**，反向证伪测试守门）；不 dirty → `void loadConfig()`。✅
- `confirmReload()`：仅 `void loadConfig()`，弹窗关闭由 ConfirmDialog 自身 `emit('update:show', false)` 负责（与 T-056 一致）。✅
- `loadedSnapshot.value = { ...form.value }` 放在 6 字段赋值之后，快照准确。✅
- Client.vue：同款 3 字段。原 `@click="loadConfig()"` 已改 `@click="handleReloadClick"`。✅
- ConfirmDialog 复用正确（v-model:show + @confirm）。✅

### (C) 死分支清理
- Wizard.vue：两个相同 `<n-text>` 合并为单个无条件，外层 `v-if` 仍控可见性。零行为变化，模板渲染结果不变。✅

## 3. 测试质量审查（重点核 insight L45 / T-057 教训）

- **零 naive-ui 组件名查询**：grep 5 个 spec 文件，无 `findComponent({name:` / `findAllComponents({name:` 模式。按钮定位全用 `findAll('button').find(b => b.text().includes(...))`，文案用 `wrapper.text()`，DOM 用 `querySelectorAll('textarea[aria-hidden]')`。✅
- **message 单例 spy**：FirewallHint.spec / PublicIpDetector.spec 用 `vi.mock('naive-ui')` 工厂内单例 + 导出 `__messageSpies`，断言可靠。✅
- **clipboard 失败模拟真实**：`navigator.clipboard.writeText` mock reject + `document.execCommand` mock（happy-dom 默认无 execCommand，beforeEach 显式装上）。✅
- **dirty 测试用 apiGet 调用计数**：`getMock.mock.calls.length` before/after 断言"确认前不重载 / 取消不重载 / 确认后重载"——直接验证不静默丢弃语义。✅
- **反向证伪**：Server/Client 各一条 Adversarial"dirty 时 handleReloadClick 不静默重载"——若实现退回直接 loadConfig 则 FAIL。FirewallHint/PublicIpDetector 各一条"双重失败仍 message.error 不抛"。✅

## 4. 潜在缺陷扫描

- `as unknown as { __messageSpies: ... }`：受控类型断言取回 mock 工厂导出，仅测试代码，可接受。
- happy-dom `document.execCommand`：测试 beforeEach 显式赋值 mock，生产代码 `document.execCommand('copy')` 在真实浏览器内网 http 下可用（fallback 主场景）。无缺陷。
- dirty 不含 AllowPortsEditor：已知局限（C-5），非缺陷，已文档化。

## 5. Verdict

**APPROVED**

实现与设计一致，范围受控，测试质量高（零组件名查询、单例 spy、真实失败模拟、反向证伪齐全），insight L45 教训已规避。无需 fix 循环。放行 stage 6。
