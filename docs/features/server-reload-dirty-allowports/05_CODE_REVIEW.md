# 05 代码评审 — T-060 server-reload-dirty-allowports

> 角色：Code Reviewer · 模式：full · 语言：中文 · 视角：外部独立审计

## Files reviewed

- `web/src/pages/Server.vue`（normalizeAllowPorts L166-174 / loadedAllowPortsSnapshot L163 / isDirty L176-191 / loadConfig 快照 L270-272 / 注释 L156-159 / expose L347-350）
- `web/src/pages/__tests__/Server.spec.ts`（+9 测试 + import + TestingHandle 扩展）
- `scripts/baseline.json`（490/812）
- `docs/dev-map.md`（Server.vue 描述行）
- 参照：`web/src/components/AllowPortsEditor.vue`（getAllowPortsInput / seed 规则，未改）、`web/src/types.ts`（AllowPortRange）

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### NIT
- [STYLE] `Server.vue:172` — `normalizeAllowPorts` 单行三元嵌 join 可读性中规中矩；当前写法简洁且有充分注释（L166-169），不建议拆。保留。

## 逐维度审计

### 1. 逻辑正确性 — PASS
- **round-trip identity**：normalize 双侧用同一函数；`typeof r.single === 'number'` 对 single 行（编辑器输出 `{single:N}`、后端加载 `{single:N}`）与 range 行（无 single 键）一致分流。合法加载值 `[{single:8080},{start:1000,end:2000}]` → seed→output identity → 双侧 `'s:8080|r:1000-2000'` → 非脏。正确（AC-5 锁死）。
- **边界 ref 未挂载**：`allowPortsEditorRef.value?.getAllowPortsInput() ?? []` 双重兜底；loading/error 态 ref 为 null 退化为空策略 `''`，但此时 `loadedSnapshot` 也为 null（isDirty 首行 `if (snap == null) return false` 已短路），不会误判。正确。
- **短路顺序**：标量 dirty 先判（`if (scalarDirty) return true`），端口策略比较仅在标量未变时执行 —— 与"任一脏即脏"语义等价，无遗漏。正确。
- **未填行语义**：编辑器加空行 → `{single:0}` → `'s:0'` 追加 → ≠ 快照 → 脏。符合"用户改了端口策略"语义（边界 §4）。正确。

### 2. 需求保真 — PASS（见下方 AC 覆盖表，全 ✅）

### 3. 设计保真 — PASS（见下方设计保真表，全 ✅）
- 无静默漂移。normalize 形态判定、快照来源（cfg.allowPorts 派生）、isDirty 扩展位置、不改 AllowPortsEditor —— 与 02 §3/§6 逐项一致。

### 4. 性能 — PASS
- normalize 是 O(n) map+join，n ≤ 100（后端上限）；isDirty 仅在用户点"重新加载"时调用（非热路径），无 watch / 无每帧执行。无性能问题。

### 5. 安全 — PASS
- 无新增数据出口；端口策略值不经新序列化/网络路径。无 secret、无注入面。normalize 输出仅用于内部字符串相等比较，不渲染到 DOM、不入 API。

### 6. 可维护性 — PASS
- normalize 内联 Server.vue（设计 OOS-4 决策，唯一消费者，不抽 util 合理）；注释解释 WHY（顺序+形态敏感的保守判脏理由 / round-trip identity / 单向数据流保留），无冗余。无死代码、无过早抽象。命名清晰（loadedAllowPortsSnapshot 与既有 loadedSnapshot 对称）。

## Requirement coverage check

| 验收点 | 实现 | 状态 |
|---|---|---|
| AC-1 仅改端口策略 → isDirty=true + 弹确认 + 不调 apiGet | `Server.vue:187-190` + Server.spec AC-1 测试（DOM 添加单端口） | ✅ |
| AC-2 未改 → 不弹确认 + 直接重载 | `Server.vue:193-200`（未改 isDirty 逻辑）+ Server.spec AC-2 | ✅ |
| AC-3 改标量仍弹确认（无回归） | `Server.vue:180-187` 标量短路保留 + Server.spec「改标量仍弹确认」 | ✅ |
| AC-4 normalize 稳定性单测 | Server.spec「normalizeAllowPorts 稳定性」3 it | ✅ |
| AC-5 round-trip 未改非脏 | `Server.vue:272`+`189` + Server.spec AC-5 | ✅ |
| AC-6 脏+确认 → 重载覆盖+isDirty 归零+快照刷新 | `Server.vue:202-205`+`270-272` + Server.spec AC-6 | ✅ |
| AC-7 verify_all PASS | 静态闸门全绿；全量真跑交 orchestrator | ⏳（交付硬闸门） |
| AC-8 baseline bump | `baseline.json` 490/812 + version 26 | ✅ |
| AC-9 SFC script < 200 行 | 实测约 170 行纯逻辑（C1 履行，04 记录） | ✅ |
| AC-10 06 含裸 ## Adversarial + 删行反向证伪 | Server.spec Adversarial「只删一行端口」已落地；06 段由 QA 写 | ✅（代码侧已就位） |

## Design fidelity check

| 设计项 | 实现 | 状态 |
|---|---|---|
| normalizeAllowPorts（single→'s:N'/range→'r:A-B'/join('|')，typeof r.single 判定） | `Server.vue:170-174` | ✅ |
| loadedAllowPortsSnapshot 从 cfg.allowPorts 派生（§6） | `Server.vue:272` | ✅ |
| isDirty 末尾追加 normalize 比较（§6 流程） | `Server.vue:187-190` | ✅ |
| 不改 AllowPortsEditor / 不引 v-model 桥（OOS-1 / insight L13） | AllowPortsEditor.vue 未改动 | ✅ |
| 不改 Client.vue（OOS-2） | 未触 | ✅ |
| normalize 内联不抽 util（OOS-4） | 内联 Server.vue | ✅ |
| 注释移除"已知局限"措辞（范围内行为 8） | `Server.vue:156-159` 已重写 | ✅ |
| 测试禁 naive-ui 组件名查询（insight L45） | DOM 按钮文本 + getExposed，零 findComponent(NXxx) | ✅ |

## 测试质量评估（规则 4：测试是否有意义而非形状匹配）

- AC-1/AC-6/Adversarial 用**真 AllowPortsEditor** + DOM 按钮文本驱动真实 rows 变化（非 mock 返回值硬塞），测的是端到端"用户改端口策略 → isDirty 捕获"真实路径 —— 有意义，非形状匹配。
- Adversarial「只删一行端口」是真正的反向证伪：若 isDirty 退回 T-058 局限（漏 allowPorts 比较），`reloadConfirmShow=true` 与 `apiGet 计数不变` 两断言会 FAIL —— 能抓住缺陷回归。
- normalize 单测覆盖空/single/range/顺序/混合/未填，断言确定字符串值（非"truthy"），可证伪。
- DOM 驱动范式与已通过的 AllowPortsEditor.spec L66-90/L168-184 同款，可靠性有先例。

## Verdict

`APPROVED`（0 CRITICAL / 0 MAJOR / 0 MINOR / 1 NIT 不阻塞）

实现与需求/设计逐条一致，无静默漂移，测试有意义且含反向证伪，红线（单向数据流 / 不编辑生成文件 / 测试只升不降 / SFC<200 行）全部尊重。AC-7（verify_all 全量真跑）交 orchestrator Bash 会话作交付硬闸门。推进 Stage 6（QA）。

---

## 复审（第二轮，QA D-1 回退后，2026-05-30）

QA 发现 AC-6 测试断言与 AllowPortsEditor 单向数据流不复位范式冲突（D-1）。dev-frontend 第二轮做了**纯测试侧修复**。CR 复审：

- **生产逻辑核对**：`Server.vue` 的 normalizeAllowPorts / loadedAllowPortsSnapshot / isDirty 扩展 / loadConfig 快照 / expose **逐行未变**（diff 仅在 Server.spec.ts + baseline.json + 04 文档）。零生产逻辑变更，原 APPROVED 的 6 维度结论不受影响。
- **拆分后测试质量**：
  - 端口策略侧 AC-6：断言 apiGet+1 + 快照刷新回真实值——这是 confirmReload 触发重载的真实可观测行为，去掉了与单向数据流冲突的 isDirty 归零强断言（D-1 修复正确）。注释清晰说明编辑器不复位的范式根因。
  - 标量侧 AC-6 配套：断言标量复位（bindPort 回 7100）+ isDirty 归零——锁死"标量经 form 重赋复位 vs 端口策略经独立组件不复位"的差异语义。这是有意义的对照，非形状匹配。两条用例共同覆盖 AC-6 的真实行为边界。
- **测试只升不降**：+9→+10（拆出一条），baseline 491/813 同步，B.4 不降。✅
- **D-1 处理恰当性**：D-1 是测试期望缺陷而非核心逻辑 bug；核心缺陷修复（端口策略纳入 dirty）由 AC-1（加行弹确认）+ AC-10（删行 Adversarial）双向证伪锁死，逻辑正确性高置信。测试侧修复未削弱对核心缺陷的证伪能力。

复审裁决：`APPROVED`（第二轮）。生产逻辑零变更，测试侧修复正确且增强了差异语义覆盖。推进 Stage 6 复验。
