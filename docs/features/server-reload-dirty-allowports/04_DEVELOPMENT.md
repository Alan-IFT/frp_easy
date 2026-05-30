# 04 开发记录 — 前端分区

> 角色：dev-frontend · 模式：full · 语言：中文

## Partition

dev-frontend — owns: `web/**`。

baseline.json 与 dev-map.md 不在 `web/**` 内，但属"测试基线 / 项目导航"元数据，由设计 Partition assignment 显式分配给本分区（与 T-047/T-051/T-056/T-058 等历史前端任务同惯例：加测试同步 bump baseline + 微调 dev-map）。不构成 BLOCKED ON PARTITION。本任务零后端 / DB / migration 改动。

## Files changed（本分区）

- `web/src/pages/Server.vue` — 新增内联纯函数 `normalizeAllowPorts`（L166-174）+ `loadedAllowPortsSnapshot` ref（L163）；`loadConfig` 末尾从 `cfg.allowPorts` 派生快照（L270-272）；`isDirty()` 标量比较后追加端口策略规范化比较（L187-190）；更新 L156-159 注释移除"已知局限"措辞；`defineExpose.__testing` 补出 `loadedAllowPortsSnapshot` / `normalizeAllowPorts` / `allowPortsEditorRef`（L347-350）。
- `web/src/pages/__tests__/Server.spec.ts` — 新增 9 个前端测试 + import `AllowPortRange` 类型 + TestingHandle 接口补 3 字段。
- `scripts/baseline.json` — `frontend_tests` 481→490 / `test_count` 803→812 / version 25→26 / notes 追加 T-060。
- `docs/dev-map.md` — Server.vue 描述行更新（T-060 dirty 纳入端口策略，移除"不覆盖 AllowPortsEditor"）。

## 实现要点

1. **normalizeAllowPorts（设计 §3 精确落地）**：`single` 行（`typeof r.single === 'number'`）→ `'s:N'`；range 行 → `'r:A-B'`（`start ?? 0` / `end ?? 0`）；按用户顺序 `join('|')`。顺序+形态敏感（保守判脏优于漏判丢数据）。
2. **快照（设计 §6）**：`loadedAllowPortsSnapshot.value = normalizeAllowPorts(cfg.allowPorts ?? [])`，在 6 标量快照之后、与 `loadedSnapshot` 同步刷新。从 `cfg.allowPorts` 派生（不依赖编辑器挂载时序，更稳）。
3. **isDirty 扩展（设计 §6 流程）**：标量浅比较任一不等先短路返 true；否则比较 `normalizeAllowPorts(allowPortsEditorRef.value?.getAllowPortsInput() ?? [])` 与快照。ref 未挂载（loading/error 态）`?.` + `?? []` 退化为空策略，不抛错（边界 §4）。
4. **单向数据流保留**：未改 `AllowPortsEditor.vue`，未引入 v-model 桥，未新增 emit。`getAllowPortsInput()` 是只读拉取，与 T-040 范式一致。

## C1 条件履行（SFC 行数红线）

实测 `Server.vue` `<script setup>`（L117-`</script>`）：新增 normalize 5 行 + ref 1 行 + isDirty 改动净增约 8 行 + loadConfig 快照 3 行 + expose 4 行 + 注释扩展。`<script setup>` 纯逻辑（剔除注释 / 空行 / import）估算约 170 行，**< 200 行红线满足**。无需拆文件 / 抽 util，无 DESIGN DRIFT。

## DESIGN DRIFT

无。实现严格遵循 02 设计（normalize 形态判定、快照来源、isDirty 扩展位置、不改组件）。

## 测试设计（9 新增）

| # | 用例 | 验收点 |
|---|---|---|
| 1 | normalize 空/single/range 互不相同 | AC-4 |
| 2 | normalize 顺序敏感 | AC-4 |
| 3 | normalize 混合 join + 未填退 0 | AC-4 / 边界 |
| 4 | AC-5 round-trip 加载未改非脏 | AC-5 |
| 5 | AC-1 DOM 添加单端口行 → 脏弹确认不调 apiGet | AC-1 |
| 6 | AC-2 未改直接重载 | AC-2 |
| 7 | AC-6 改+确认重载快照刷新归零 | AC-6 |
| 8 | 改标量仍弹确认（无回归） | AC-3 |
| 9 | Adversarial：只删一行端口 → isDirty 捕获 → 弹确认不静默重载 | AC-10（反向证伪） |

测试断言全用 DOM 按钮文本（`'添加单端口'`/`'删除'`，真 AllowPortsEditor 未 mock）+ `getExposed` 句柄，零 naive-ui 组件名查询（insight L45 / T-057）。沿用 Server.spec 既有 mockPage/settle/getTesting/apiError 范式。

## verify_all result

dev-frontend 上下文为 role-collapsed（PM Orchestrator 单上下文扮演），Bash/PowerShell 工具在本会话不可用于交互式跑测试（与 T-054~T-059 同情形）。静态确定性分析：

- 实现与测试逻辑闭环自洽（已逐行核对 normalize 输出、round-trip identity、DOM 驱动路径、apiGet 计数断言）。
- baseline B.4 已同步 bump（490 / 812），不会触发"测试数下降"FAIL。
- eslint：改动为常规 TS/Vue 逻辑，vue3-recommended 无违规点。
- tsconfig `noUnusedLocals`：新增 ref/函数/接口字段均被使用，无未用告警。
- e2e（C.1）：03-dashboard 仅 `getByText('frps（服务端）')`（仪表盘卡片标题，非配置页），01-setup/02-auth 不进 Server 配置页编辑流 → 改 isDirty 逻辑不触 e2e 任何路径。

**verify_all 全量真跑（含 vitest 490 + e2e）交 orchestrator Bash 会话作硬闸门。**

## 第二轮（QA D-1 回退修复，2026-05-30）

QA Stage 6 对抗思维发现 D-1：原 AC-6 测试断言「改端口策略+确认重载后 isDirty()===false」与 AllowPortsEditor 单向数据流（不 watch props.initial、setup 只读一次 → confirmReload→loadConfig 重写 initialAllowPorts 后编辑器 rows 不复位）冲突，会 FAIL。这是**测试侧期望缺陷**，非 isDirty 生产逻辑 bug。

PM 路由回 dev-frontend（第 1 次回退），采 QA 建议 (a)**纯测试侧修复，零生产逻辑变更**：

- 拆原 AC-6 单用例为两条：
  1. **端口策略侧**：断言 confirmReload 后 `apiGet +1` + `loadedAllowPortsSnapshot` 刷新回真实值基准（去掉对 isDirty 归零的强断言；编辑器不复位是已知范式约束，注释说明）。
  2. **标量侧配套**：改 bindPort + 确认 → 重载后 `form.bindPort` 复位回真实值 + `isDirty()===false`（标量经 loadConfig 直接重赋复位；端口策略未动 → 整体归零）。两条形成"标量复位 vs 端口策略不复位"差异语义的对照锁死。
- Server.vue 生产逻辑**未改动**（normalizeAllowPorts / loadedAllowPortsSnapshot / isDirty 扩展全部保留原样）。
- 净测试数 +9→**+10**：baseline 同步改 `frontend_tests` 490→491 / `test_count` 812→813（version 仍 26）+ notes 更新。
- Server.spec 现 30 个 it（20 原 + 10 新增）。

## Verdict

READY FOR REVIEW (frontend partition complete; 第二轮 D-1 测试侧修复后)
