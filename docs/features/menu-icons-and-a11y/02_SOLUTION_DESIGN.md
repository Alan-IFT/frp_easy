# 02 方案设计 — T-064 · menu-icons-and-a11y

> Harness stage 2 · Solution Architect · mode=**full** · 输出中文。
> 上游 01_REQUIREMENT_ANALYSIS.md verdict=READY，已确认无歧义。

## 1. Architecture summary（架构概述）

纯前端、纯展示层 + 可访问性属性的最小改动：在三个既有 Vue SFC 上 (1) 替换侧边栏菜单中重复的齿轮字形使 7 个顶层项图标两两互不相同，并在每个图标渲染节点上挂 ARIA 无障碍名；(2) 给日志滚动容器加 `tabindex` + ARIA role + aria-label 使纯键盘可聚焦滚动；(3) 给两处复制反馈承载元素加 `aria-live` 使屏幕阅读器播报复制结果。**零新依赖、零行为逻辑改动、零结构/路由改动**。系统层面无新模块、无数据/API 变化。

## 2. Affected modules（受影响模块，含文件路径）

| 文件 | 改动性质 |
|---|---|
| `web/src/components/AppLayout.vue` | edit — `menuOptions`（:130-171）图标 render 函数：换 `settings` 字形 + 每项 icon span 挂 `aria-label`/`title`/`role="img"` |
| `web/src/components/log/LogList.vue` | edit — 滚动容器 `div.log-list-scroll`（:18-24）加 `tabindex="0"` + `role` + `aria-label` |
| `web/src/components/FirewallHint.vue` | edit — 单条复制按钮（:15-22）+ 复制全部按钮（:29-31）承载切换文案的元素加 `aria-live="polite"` |
| `web/src/components/PublicIpDetector.vue` | edit — 复制按钮（:20-22）承载切换文案的元素加 `aria-live="polite"` |
| `web/src/components/__tests__/AppLayout.spec.ts` | **new** — 当前无 AppLayout spec |
| `web/src/components/log/__tests__/LogList.spec.ts` | **new** — 当前无 LogList spec（注意目录：`web/src/components/log/__tests__/`） |
| `web/src/components/__tests__/FirewallHint.spec.ts` | edit — 既有，追加 a11y 断言 |
| `web/src/components/__tests__/PublicIpDetector.spec.ts` | edit — 既有，追加 a11y 断言 |
| `scripts/baseline.json` | edit — bump frontend_tests / test_count / version |
| `docs/dev-map.md` | edit — AppLayout / LogList 相关行（若有结构性说明变化） |

> LogList spec 目录核实：源文件在 `web/src/components/log/LogList.vue`，但既有日志相关 spec（useLogBuffer/useLogSearch 等）落在 `web/src/components/__tests__/`。dev-frontend 实测既有约定后择一放置，**推荐与组件同级建 `web/src/components/log/__tests__/`** 或沿用 `web/src/components/__tests__/`，只要 vitest glob 能扫到即可（vitest 默认扫 `**/*.spec.ts`）。不构成设计阻塞。

## 3. Module decomposition（模块分解）

无新模块。三处均为既有 SFC 内的属性级编辑。

### 3.1 AppLayout.vue menuOptions（IS-1 / IS-2）

设计采用**方案 (a)：互不相同 Unicode 字形 + 在 icon span 上挂 ARIA 名**（理由见 §7 Reuse audit + §8 Risk）。

当前每项图标 render 形如：
```ts
icon: () => h('span', { class: 'n-icon' }, '⚙')
```

设计后形如（以 `server` / `settings` 为例，二者字形不同 + 各挂 aria-label/title/role）：
```ts
// 服务端配置：保留齿轮 ⚙
icon: () => h('span', { class: 'n-icon', role: 'img', 'aria-label': '服务端配置', title: '服务端配置' }, '⚙'),
...
// 设置：改用不同字形（消除与服务端配置的撞车）
icon: () => h('span', { class: 'n-icon', role: 'img', 'aria-label': '设置', title: '设置' }, '⚒'),
```

**字形分配表（dev-frontend 须保证全表两两互不相同，且与各项 label 语义不悖）：**

| key | label | 当前字形 | 设计后字形 | 备注 |
|---|---|---|---|---|
| `dashboard` | 仪表盘 | `⊙` | `⊙` 不变 | 唯一 |
| `proxies` | 代理规则 | `⇌` | `⇌` 不变 | 唯一 |
| `server` | 服务端配置 | `⚙` | `⚙` 保留 | 齿轮归"配置类"主项 |
| `server/monitor` | 服务端监控 | `◉` | `◉` 不变 | 唯一 |
| `client` | 客户端配置 | `↗` | `↗` 不变 | 唯一 |
| `logs` | 日志 | `≡` | `≡` 不变 | 唯一 |
| `settings` | 设置 | `⚙` ❌重复 | `⚒` | **唯一缺陷点：换字形消除与 server 撞车** |

> `⚒`（U+2692 HAMMER AND PICK）作"设置/工具"语义合理且与齿轮形态明显不同；dev-frontend 若实测某系统字体下 `⚒` 渲染为豆腐块，备选 `✎`(U+270E)/`⚑`(U+2691)/`⊛`，但必须 (1) 与其余 6 个不重复 (2) 语义不悖 (3) 常见字体可渲染（NFR-5）。**最终字形由 dev-frontend 实测确定，设计只约束"唯一 + 语义不悖 + 可渲染"三条**。

**ARIA 名挂载点决策（Q-1）**：挂在 icon render 返回的 `<span>` 上，用 **`aria-label` + `title` + `role="img"` 三者齐发**：
- `aria-label`：屏幕阅读器主路径，折叠态无文字标签时朗读该名；
- `title`：鼠标悬停 tooltip，折叠态视力用户也能确认；
- `role="img"`：让该 span 被 AT 当作有名图像而非装饰字符（否则裸文本字形可能被逐字朗读为"齿轮符号"等无意义内容）。
- 取值 = 该项 `label` 中文文案（`server`→"服务端配置"，`settings`→"设置"），保证 AC-3 两项不相同。

> 备选 Q-1 方案被否：在 `MenuOption` 对象上挂 `props: { 'aria-label': ... }` 透传到 `n-menu` 渲染的 `<a>`/`<div>` —— naive-ui MenuOption 透传 ARIA 的行为版本敏感、不如直接控制 icon span 确定。采用直接 render span 挂属性（dev 完全掌控 DOM、可被 `find('span.n-icon').attributes('aria-label')` 稳定断言）。

### 3.2 LogList.vue 滚动容器（IS-3）

`div.log-list-scroll`（:18-24）加三属性：
```html
<div
  v-else
  ref="scrollEl"
  class="log-list-scroll"
  tabindex="0"
  role="log"
  aria-label="日志输出"
  :style="..."
  @scroll="onScrollNative"
>
```

**role 决策（Q-2）**：采用 **`role="log"`**。理由：ARIA `log` role 语义最贴日志流，是 paused-banner 同文件已建立的"懂 ARIA"团队范式的自然延伸。

> **Risk-2 权衡（重要，见 §8）**：`role="log"` 隐含 `aria-live="polite"`，对高频追加日志可能产生屏幕阅读器播报噪音。但本任务核心诉求是 IS-3 的"**可聚焦滚动**"（tabindex），role 是附带语义。若 dev-frontend / Gate Reviewer 认为自动跟读噪音风险高于收益，**允许降级为 `role="region"`**（不隐含 live region，纯可聚焦地标）——两者都满足 AC-5（role ∈ {log, region} + 非空 aria-label）。设计**首选 `role="log"`**，备选 `region`，由 GR 在 §Risk 处确认。AC-5 已写成兼容两值。

tabindex="0" 使容器进入 Tab 序，键盘用户聚焦后浏览器原生支持方向键/PageUp/PageDown 滚动 `overflow-y:auto` 容器（无需额外 keydown 处理，区别于 paused-banner 的 button 语义需显式 enter/space）。

### 3.3 FirewallHint.vue + PublicIpDetector.vue 复制反馈（IS-4）

**aria-live 决策（Q-3）**：采用 **`aria-live="polite"`**（直接挂属性，不用 `role="status"`——避免改变 button 的既有 button role 语义）。

挂载点：直接挂在承载"复制"↔"已复制 ✓"切换文案的 `<n-button>` 上。naive-ui 组件根元素 fallthrough 透传未声明 attrs 到真实 DOM 节点（`<button>`），故 `aria-live` 会落到渲染出的 button 上。

FirewallHint.vue：
```html
<!-- 单条命令复制按钮（:15-22）-->
<n-button size="tiny" type="default" text aria-live="polite" @click="copyCmd(cmd)">
  {{ copiedCmd === cmd ? '已复制 ✓' : '复制' }}
</n-button>
...
<!-- 复制全部按钮（:29-31）-->
<n-button size="small" aria-live="polite" @click="copyAll">
  {{ copiedAll ? '已复制全部 ✓' : '复制全部' }}
</n-button>
```

PublicIpDetector.vue：
```html
<!-- :20-22 -->
<n-button size="tiny" type="default" text aria-live="polite" @click="copyIp">
  {{ copied ? '已复制 ✓' : '复制' }}
</n-button>
```

> **验证点（dev-frontend 实测）**：确认 naive-ui `n-button` 把 `aria-live` 透传到渲染出的 `<button>` DOM 上（`find('button').attributes('aria-live')==='polite'`）。若实测发现 n-button **不**透传该属性（极少见，naive-ui 默认 `inheritAttrs` 透传），降级方案：在 button 外包一个 `<span aria-live="polite">` 承载文案，button 内仅放图标/触发——但这会改 DOM 结构，**首选透传方案**。dev 实测后在 04 记录实际落点。`aria-live="polite"` 语义保证首次渲染（"复制"初始态）不播报、仅后续文案变化（→"已复制 ✓"）播报，满足 BC-4。

## 4. Data model changes

无。

## 5. API contracts

无。本任务不涉及任何前后端接口。

## 6. Sequence / flow（流程）

无新数据流。三处改动均为静态渲染属性 + 既有事件处理器不变：
- 菜单：`menuOptions` icon render 返回带 ARIA 的 span → naive-ui n-menu 渲染；点击仍走既有 `handleMenuSelect`（OOS-2，不改）。
- 日志容器：`@scroll`/`onScrollNative`/`scrollElReady` emit 链不变（OOS-4）；新增 tabindex 让容器进 Tab 序，方向键滚动由浏览器原生处理。
- 复制：`@click` 仍调既有 `copyCmd`/`copyAll`/`copyIp` → `copyText` → `copyToClipboard`（OOS-3，不改）；`aria-live` 仅让既有文案切换被播报。

## 7. Reuse audit（复用审计）

| Need | Existing code | File path | Decision |
|---|---|---|---|
| 键盘可聚焦 + ARIA 范式 | paused-banner `role`/`tabindex`/keydown | `web/src/components/log/LogList.vue:30-41` | 复用范式（滚动容器用 tabindex+role，方向键滚动靠原生 overflow 故不需显式 keydown，比 banner 更简） |
| 图标 render 结构 | `h('span',{class:'n-icon'}, glyph)` | `web/src/components/AppLayout.vue:130-171` | 复用结构，仅在 props 对象追加 ARIA 键 + 换 1 个字形 |
| 复制逻辑 | `copyToClipboard` | `web/src/utils/clipboard.ts` | 复用不动（OOS-3）；仅在调用方按钮加 aria-live |
| 复制组件测试范式 | DOM 文本 + clipboard mock + naive-ui message 单例 spy | `web/src/components/__tests__/{FirewallHint,PublicIpDetector}.spec.ts`（T-058/T-061） | 复用范式追加 a11y 属性断言 |
| 图标库 | （不引入） | — | **零新依赖**：现有裸字形方案 + ARIA 名即可满足需求，引图标库违反 NFR-1 且增 bundle / 需过 build |

## 8. Risk analysis（风险 + 缓解，≥3）

- **Risk-1（naive-ui MenuOption ARIA 透传不确定）**：若把 ARIA 挂在 option 对象上可能因版本不透传。**缓解**：本设计**不依赖 option 透传**，直接在 icon render 的 span 上挂属性（dev 完全掌控该 DOM 节点），`find('span.n-icon[aria-label]')` 可稳定断言。
- **Risk-2（`role="log"` 自动跟读噪音）**：`role="log"` 隐含 `aria-live=polite`，高频日志追加可能让屏幕阅读器持续播报。**缓解**：本任务核心是 tabindex 可聚焦（IS-3 主诉求），role 为附带语义；AC-5 已兼容 `region` 备选；GR 在评审时确认首选 `log` 还是降级 `region`。无论哪个，可聚焦诉求（AC-4 tabindex）都满足。**首选 log，GR 有否决权降 region**。
- **Risk-3（n-button aria-live 不透传到 DOM）**：naive-ui 组件默认 `inheritAttrs:true` 透传，但极端情况可能不落到 `<button>`。**缓解**：dev-frontend 实测 `find('button').attributes('aria-live')`，若不透传则降级为外包 `<span aria-live="polite">`（04 记录实际落点）；首选透传方案零结构改动。
- **Risk-4（字形跨字体渲染为豆腐块，NFR-5）**：`⚒` 等字形在某些系统字体可能缺字。**缓解**：dev-frontend 实测；字形分配表给了备选集；测试断言查 ARIA 名（语义稳定）而非依赖字形像素渲染。
- **Risk-5（e2e 回归）**：03-dashboard.spec.ts:11 断言 `getByText('仪表盘')`。**缓解**：本任务**不改任何菜单 label 文案**（仅换图标字形 + 加 ARIA），PM 已 grep 核实仅此一处按菜单文本断言且文案不变 → e2e 零影响（insight L34，AC-11）。
- **Risk-6（无既有 AppLayout/LogList spec，新建测试可能挂载失败）**：AppLayout 依赖多个 store（auth/app/downloader）+ vue-router + naive-ui provider。**缓解**：dev-frontend 复用项目既有 mount 范式（Pinia createTestingPinia + router mock + naive-ui，参照 Wizard.spec/Server.spec 的挂载样板）；若 AppLayout 整体挂载成本过高，可只断言 `menuOptions` 导出的图标 render 结果（但 menuOptions 当前是 module 内 const 非导出——dev 可在测试中 mount 组件后查 `span.n-icon` DOM，或评估最小可测路径）。这是 dev 实现细节，不阻塞设计。

## 9. Migration / rollout plan（迁移/上线）

- 纯前端属性级改动，向后兼容，无数据迁移、无 feature flag。
- 回滚：git revert 即可，无副作用。
- 构建：`web` 改动经 `verify_all` 的前端 build + vitest 闸门；Go 侧 embed 静态产物随构建重生成（无需手动）。

## 10. Out-of-scope clarifications（设计边界）

- 不引任何新依赖（NFR-1）。
- 不改菜单结构/路由 key/activeKey 计算（OOS-2，`AppLayout.vue:122-128` 特判不变）。
- 不改 copyToClipboard / 复制交互 / 复制文案（OOS-3）。
- 不改 LogViewer 滚动状态机/followTail/search/状态分支逻辑（OOS-4，仅加滚动容器 a11y 属性）。
- 不动 FullscreenLogModal 焦点陷阱（OOS-5，naive-ui n-modal 自带）。
- 不引颜色/主题改动（OOS-6，留 T-066）。
- 不改 paused-banner（OOS-7，仅作范式参考）。
- 子菜单项（frpc/frps 日志）默认有文字 label，IS-2 仅约束顶层项无障碍名；子项不强制加 aria-label（BC-2）。

## 11. Partition assignment（分区分配，必填）

`.harness/agents/dev-*.md` 存在（dev-db / dev-backend / dev-frontend）→ partitioned 模式。本任务**全部前端**。

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `web/src/components/AppLayout.vue` | dev-frontend | edit | — |
| `web/src/components/log/LogList.vue` | dev-frontend | edit | — |
| `web/src/components/FirewallHint.vue` | dev-frontend | edit | — |
| `web/src/components/PublicIpDetector.vue` | dev-frontend | edit | — |
| `web/src/components/__tests__/AppLayout.spec.ts` | dev-frontend | new | — |
| `web/src/components/log/__tests__/LogList.spec.ts`（或同级 __tests__） | dev-frontend | new | — |
| `web/src/components/__tests__/FirewallHint.spec.ts` | dev-frontend | edit | — |
| `web/src/components/__tests__/PublicIpDetector.spec.ts` | dev-frontend | edit | — |
| `scripts/baseline.json` | dev-frontend | edit | 所有测试完成后 |
| `docs/dev-map.md` | dev-frontend | edit | — |

### Dispatch order

1. dev-frontend（单分区，无跨分区依赖）

### Parallelism

无——单分区，全部改动在 `web/**`（+ baseline/dev-map 文档）owned by dev-frontend。无后端/DB 改动。

## 12. Verdict（结论）

**READY**

设计完整可直接实现：三处属性级改动 + 1 个字形替换，零新依赖、零行为/结构/路由改动。所有验收点可由 DOM 属性查询测试（insight L45）。开放设计点已收敛（Q-1 icon span 挂 aria-label+title+role；Q-2 首选 role=log 备选 region 由 GR 确认；Q-3 aria-live=polite）。三处实测点（n-menu icon span / role=log 噪音 / n-button aria-live 透传）由 dev-frontend 验证并在 04 记录，均有降级方案，不阻塞。
