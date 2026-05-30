# 01 — 需求分析（Requirement Analysis）

- 任务：T-057 `binary-missing-onboarding-ux`
- 模式：full
- Stage：1 / 7（requirement-analyst）

## 1. Goal（一句话）

改善"二进制缺失"首次使用体验的两处信息架构缺陷：Dashboard 的缺失提示要导向已存在的可操作入口（顶部横幅一键下载/手动上传），Wizard 完成保存配置后要校验所选角色对应二进制是否就绪并在缺失时给出清晰的就地警告而非静默自动跳走。

## 2. In-scope behaviors（可测试，编号）

### A. Dashboard 缺失提示文案对齐信息架构

- **IS-1**：当 `appStore.binMissing.length > 0` 时，Dashboard 顶部的 `n-alert`（二进制文件缺失）文案必须引导用户使用**顶部横幅**提供的一键下载 / 手动上传入口，而非仅导向手动拷贝文件。文案须包含"顶部横幅 / 上传 / 下载"等引导关键字。
- **IS-2**：手动拷贝到 `frp_win/` / `frp_linux/` 目录后重启的说明**退为兜底**保留（措辞如"或手动放置…后重启"），不作为首选路径。
- **IS-3**：Dashboard 的该 alert **不**新增任何下载/上传按钮组件（避免与 AppLayout 顶栏横幅的入口重复）。它只是文字引导指向已有横幅。

### B. Wizard 完成前校验所选角色二进制就绪

- **IS-4**：Wizard step 2 完成配置时，在保存配置（`apiPutServer` / `apiPutClient`）与开启自动启动（`apiPutMode`）成功之后、跳转之前，依据 `appStore.binMissing` 判断**所选角色**对应二进制是否缺失：
  - `selectedRole === 'frpc'` → 检查 `'frpc'`
  - `selectedRole === 'frps'` → 检查 `'frps'`
  - `selectedRole === 'both'` → 检查 `'frpc'` 与 `'frps'` 两者
- **IS-5**：若所选角色对应二进制**缺失**：
  - 不阻断（配置保存与模式开启已成功生效）。
  - 不执行自动跳转 + 不发"配置已保存，正在跳转..."的 success toast。
  - 在向导内（step 3）展示一个清晰的 `n-alert`（type=warning），文案说明：配置已保存，但所选角色的二进制（具体列出缺失的 frpX）尚未就绪，进入仪表盘后请用顶部横幅一键下载或手动上传，二进制就绪后才能启动。
  - 提供一个"进入仪表盘"按钮，由用户主动点击跳转（`router.push('/dashboard')`），而非自动跳走错过提示。
- **IS-6**：若所选角色对应二进制**不缺失**：维持现有行为——`message.success('配置已保存，正在跳转...')` + 自动 `router.push('/dashboard')`。
- **IS-7**：完成校验前，Wizard 在校验逻辑前 `await appStore.fetchReady()` 一次，确保 `binMissing` 为最新（境内用户可能在向导停留期间通过其他途径变更二进制状态；且保证测试可独立控制 store 状态）。`fetchReady` 内部已 try/catch 吞错（见 `stores/app.ts:25-35`），失败不抛，不破坏完成流程。

## 3. Out-of-scope（本次明确不做）

- **OOS-1**：不在 Wizard 内新增一键下载 / 手动上传按钮组件（向导内只给警告 + 引导 + "进入仪表盘"按钮；真正的下载/上传在仪表盘顶栏横幅完成）。
- **OOS-2**：不改后端 `system/ready` / `binMissing` 计算逻辑。
- **OOS-3**：不改 AppLayout 顶栏横幅（其入口已完整，T-018 / T-027 已实现）。
- **OOS-4**：不改 wizard store / wizard API / 路由守卫的向导决策逻辑。
- **OOS-5**：不改 Wizard step 1/2 的表单校验、字段、API 调用 payload。
- **OOS-6**：不引入新的国际化机制（项目当前硬编码中文文案）。

## 4. Boundary conditions

- **BC-1**：`binMissing` 为 `[]`（不缺失）→ 走 IS-6 自动跳转分支。
- **BC-2**：`selectedRole === 'both'` 且仅 `frps` 缺失（`frpc` 存在）→ 视为缺失（所选角色之一缺失即缺失）→ 走 IS-5 警告分支，警告文案列出 `frps`。
- **BC-3**：`selectedRole === 'frpc'` 但缺失的是 `'frps'`（与所选无关）→ 视为不缺失（只看所选角色对应项）→ 走 IS-6 自动跳转。
- **BC-4**：`selectedRole === 'both'` 且 `frpc` 与 `frps` 都缺失 → 警告文案列出两者。
- **BC-5**：`appStore.fetchReady()` 在校验前调用失败（fetchReady 内部吞错，binMissing 维持调用前值）→ 不抛错、不阻断；以现有 binMissing 值判断（降级但不崩）。
- **BC-6**：配置保存阶段（`apiPutServer`/`apiPutClient`/`apiPutMode`）失败 → 走现有 catch 分支（`configError`），不进入二进制校验逻辑（保存失败优先于二进制提示）。
- **BC-7**：Dashboard `binMissing` 为 `[]` → alert 不渲染（v-if 已有，不变）。

## 5. Acceptance criteria（可验证）

- **AC-1**（IS-1/IS-2）：Dashboard binMissing 非空时，alert 文案含"上传"或"下载"或"顶部"引导关键字；且不再只导向手动拷贝（文案出现引导横幅入口的表述）。手动放置说明作为兜底保留。
- **AC-2**（IS-3）：Dashboard alert 内不含 `<button>` / 下载 / 上传组件（纯文字引导）。
- **AC-3**（IS-5，缺失分支）：`selectedRole='both'` 且 `frps` 缺失，完成配置后：(a) 不调用自动 `router.push`（push spy 未被以 '/dashboard' 调用），(b) 不发"正在跳转"success toast，(c) 向导内出现 warning alert 含缺失的 'frps' 与引导文案，(d) 存在"进入仪表盘"按钮，点击后才 `router.push('/dashboard')`。
- **AC-4**（IS-6，不缺失分支）：`selectedRole='both'` 且 binMissing=[]，完成配置后：维持 success toast + 自动 `router.push('/dashboard')`。
- **AC-5**（BC-3）：`selectedRole='frpc'` 但仅 `frps` 缺失（与所选无关）→ 走自动跳转分支（不误报警告）。
- **AC-6**（IS-7）：完成流程在校验前调用了 `appStore.fetchReady`（或等效保证 binMissing 新鲜的机制）。
- **AC-7**（红线）：`scripts/verify_all` PASS（eslint、SFC 纯逻辑 < 200 行、测试数 ≥ 基线）。
- **AC-8**（baseline）：`scripts/baseline.json` 的 `frontend_tests` + `test_count` 按新增前端测试数同步 bump。
- **AC-9**（QA）：06_TEST_REPORT.md 含裸 `## Adversarial tests` 段，至少一条"所选 both 但 frps 缺失 → 警告出现、未静默跳"反向证伪。

## 6. Non-functional requirements

- **NFR-1**：改动仅限 `web/src/pages/Dashboard.vue`（文案）+ `web/src/pages/Wizard.vue`（校验+提示+其测试）+ 其 spec + `scripts/baseline.json` + 必要时 `web/src/stores/app.ts`（仅当确需补 fetch；本次复用既有 `fetchReady`，预计不改 store）+ `docs/dev-map.md`（Wizard 行为变更记一行）。禁扩散。
- **NFR-2**：Wizard.vue 若改动后纯逻辑行数逼近 200，按 insight L22 metric（script 段非 import 非 testing hook 纯逻辑行数）判定，而非 wc -l。
- **NFR-3**：测试模拟 API 失败用 `web/src/test-utils/apiError.ts`；读暴露句柄用 `web/src/test-utils/exposed.ts::getExposed`（insight L45）。
- **NFR-4**：测试数只升不降。

## 7. Related tasks

- T-002（`docs/features/_archived/zero-config-quickstart/`）：Wizard.vue 引入。
- T-014 / T-018 / T-027：二进制下载/上传/取消链路 + AppLayout 横幅入口（本次复用，不改）。
- T-047（`docs/features/frontend-honest-states/`）：Dashboard 不静默范式（本次延续"不静默跳走"精神）。
- T-051（`docs/features/frontend-test-coverage/`）：wizard store 测试。
- T-056（`docs/features/proc-stop-destructive-confirm/`）：Dashboard.spec.ts 当前测试范式（naive-ui mock、useMessage 单例 spy、getExposed、apiError、router push spy）。

## 8. Open questions for user

无。任务上下文（orchestrator 已核实）对两处修复方向、缺失判定语义、跳转行为、e2e 兼容策略均已明确给出取舍。`fetchReady` 复用与"缺失才改手动跳转"策略由本分析在 §2/§4 固化，无需用户裁决。

## 9. Verdict

**READY** — 无 open question，可进入 stage 2（solution-architect）。
