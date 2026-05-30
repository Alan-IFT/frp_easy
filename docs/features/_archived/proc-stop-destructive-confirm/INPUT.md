# Task Input — T-056

- **slug**: `proc-stop-destructive-confirm`
- **mode**: full
- **一句话目标**: 给 Dashboard 的进程**停止 / 重启**破坏性操作加二次确认，与站内"删除代理规则"的确认标准对齐，避免误点导致瞬间中断所有穿透连接。

## Orchestrator 已定决策（dev 照做，勿改范围）

- **停止 + 重启** 两个操作，对 **frpc 和 frps 都加确认**（两者都会中断活动连接）。**启动不加确认**（非破坏性）。
- 复用既有组件 `web/src/components/ConfirmDialog.vue`（props: `show/title/content`；events: `update:show/confirm/cancel`）——与 `Proxies.vue` 删除确认同款范式。
- 确认文案按进程 + 后果定制（用既有 `labelOf(kind)`：客户端 frpc / 服务端 frps）：
  - 停止：标题"停止{label}？"，内容点明后果——frps："将立即中断所有正在穿透的远程连接。" / frpc："将断开本机所有正在转发的连接。"
  - 重启：标题"重启{label}？"，内容"将短暂中断当前所有连接后重新建立。"

## 精确技术上下文（orchestrator 已核实）

- 文件：`web/src/pages/Dashboard.vue`。
  - frpc 停止按钮 L87-94 `@click="handleStop('frpc')"`；frpc 重启 L95-101 `@click="handleRestart('frpc')"`。
  - frps 停止按钮 L173-180 `@click="handleStop('frps')"`；frps 重启 L181-187 `@click="handleRestart('frps')"`。
  - 脚本段 `<script setup>` 从 L196 起；`handleStop`/`handleStart`/`handleRestart`、`loadingMap`、`labelOf`、`canStart/canStop` 已存在。
- import `ConfirmDialog` from `'../components/ConfirmDialog.vue'`。

## 要求

1. 完整产出 7 阶段文档（01-07 + PM_LOG），中文。
2. 改动**仅限** `Dashboard.vue` + 其测试 + 必要时 ConfirmDialog（尽量不改它）。
3. 补组件测试（测试数只升不降）；同步 bump `scripts/baseline.json`。
4. 06_TEST_REPORT.md 含裸 `## Adversarial tests` 段。
5. 不要 git commit/push、不跑 archive-task（orchestrator 负责）。
6. 07_DELIVERY.md 含裸 `## Insight` 段。
7. 红线：不编辑 `.claude/`/`CLAUDE.md`/`.github/`；遵守 eslint；Vue SFC 纯逻辑 < 200 行。
