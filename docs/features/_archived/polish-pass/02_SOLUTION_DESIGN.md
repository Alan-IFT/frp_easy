# 02 — 方案设计：T-009 polish-pass

> Stage 2 of 7-stage `/harness` 流水线 · 中文 · PM 亲撰

---

## 1. 设计概要

三条独立修复线，互不依赖，可并行实施：

| 线 | 触动文件 | 风险 | 验证手段 |
|---|---|---|---|
| L1 Playwright cross-shell parity | `web/playwright.config.ts` + 新 `scripts/start-e2e-server.ps1` | 低（脚本 + 一行 config） | PowerShell 中跑 verify_all 全 18 PASS |
| L2 T-001 归档 | 调 `scripts/archive-task.sh` | 极低（脚本本身已经过 T-002~T-008 验证 7 次） | `ls docs/features/_archived/web-ui-mvp/` |
| L3 dev-map 语言一致性 | `docs/dev-map.md` | 极低（纯文档） | 全文 grep 无日文假名 |

---

## 2. L1 详细设计：Playwright cross-shell parity

### 2.1 问题根因

`web/playwright.config.ts:19` 的 `webServer.command = 'bash ../scripts/start-e2e-server.sh'`：
- Git Bash / Linux / macOS 下，`bash` = MSYS bash 或系统 bash —— 工作。
- PowerShell 下，Windows PATH 优先解析到 `C:\Users\<user>\AppData\Local\Microsoft\WindowsApps\bash.exe`，这是 WSL 的 shim，调用 wsl.exe；用户未安装 WSL 发行版时返回乱码错误 "适用于 Linux 的 Windows 子系统没有已安装的发行版"。

### 2.2 设计决策

**方案 A（采用）**：在 playwright.config.ts 中按 `process.platform` 路由：
- `win32` → `pwsh -NoProfile -ExecutionPolicy Bypass -File ../scripts/start-e2e-server.ps1`
- 其他 → `bash ../scripts/start-e2e-server.sh`

**为何不选 B（在 bash 脚本里把执行委托给 Node.js 子进程）**：会引入 node 与 shell 的复杂调用栈，调试更难。

**为何不选 C（让 bash 脚本检测 WSL shim 并切换）**：把跨平台耦合塞进单脚本，违反"双脚本配对"项目惯例（package.sh/ps1, install-service.sh/ps1）。

### 2.3 start-e2e-server.ps1 接口契约

输入：无参数。
环境：必须能解析 `pwsh.exe`（PowerShell 7+）；`go` 在 PATH 中可访问。
输出：
- 启动 `bin/frp-easy.exe`（必要时先 `go build`）。
- 监听 `127.0.0.1:8080`。
- 临时目录 `$env:TEMP\frp-easy-e2e-<random>\` 存放 frp_easy.toml + data + logs。
- 通过环境变量 `FRP_EASY_CONFIG` 注入到子进程。
- 进程 SIGTERM/Ctrl+C 时返回，Playwright 通过 `taskkill /T` 关掉整棵进程树。
- stderr 打印 `[e2e-server] ...` 前缀的诊断行（与 .sh 版对齐）。

退出码：0 正常 / 非 0 任何前置失败（缺 go、build 失败、bin 没产物）。

### 2.4 与已有 .sh 版的行为等价对照

| 行为 | .sh 实现 | .ps1 设计 |
|---|---|---|
| 探测是否需要重建 | `find dist/ -newer $BIN -type f` | 比较 `dist/` 目录下文件 LastWriteTime vs `$bin.LastWriteTime`（PowerShell `Get-ChildItem` + `Where-Object`） |
| 二进制路径 | `bin/frp-easy` 或 `bin/frp-easy.exe` | `bin\frp-easy.exe`（Windows 总是 .exe） |
| 临时目录 | `mktemp -d` | `Join-Path $env:TEMP "frp-easy-e2e-$([Guid]::NewGuid().ToString('N'))"` |
| 写 frp_easy.toml | bash heredoc | `Set-Content -Path ... -Encoding UTF8 -Value @"..."@` |
| 启动二进制 | `exec "$BIN"` | `& $BinaryPath`（Playwright 通过 SIGTERM 终止） |

### 2.5 playwright.config.ts 改动（最小化）

```ts
// 前
webServer: {
  command: 'bash ../scripts/start-e2e-server.sh',
  ...
}

// 后
webServer: {
  command: process.platform === 'win32'
    ? 'pwsh -NoProfile -ExecutionPolicy Bypass -File ../scripts/start-e2e-server.ps1'
    : 'bash ../scripts/start-e2e-server.sh',
  ...
}
```

不动 `url` / `timeout` / `reuseExistingServer` / 标准输出配置。

---

## 3. L2 详细设计：T-001 归档

### 3.1 动作

```bash
bash scripts/archive-task.sh --task web-ui-mvp
```

期望效果：
- `docs/features/web-ui-mvp/` → `docs/features/_archived/web-ui-mvp/`。
- 若 `07_DELIVERY.md` 中有 `## Insight` 段，则 harvest 到 `.harness/insight-index.md`。

### 3.2 验证 07_DELIVERY.md 是否含 Insight 段

需先读 T-001 07_DELIVERY.md 确认。若**无 Insight 段**，archive-task 不会动 insight-index.md，符合期望。

### 3.3 tasks.md 同步

archive-task.sh 不会改 tasks.md。**PM 手工更新**：T-001 的"文档目录"列从 `docs/features/web-ui-mvp/` 改为 `docs/features/_archived/web-ui-mvp/`。

---

## 4. L3 详细设计：dev-map.md 语言清理

### 4.1 范围

`docs/dev-map.md` 第 49–96 行（web/src/ 子树注释）含日文假名。其他章节是中文。

### 4.2 翻译表（按原文逐句对齐）

| 原（日文） | 译（中文） |
|---|---|
| ルートコンポーネント | 根组件 |
| ラップ | 包裹 |
| 修正 | 修复 |
| app 入口；Pinia・Router 組み立て・CSRF トークンゲッター登録 | app 入口；组合 Pinia / Router；注册 CSRF token getter |
| ナビゲーションガード | 导航守卫 |
| バックエンド API 契約と一致する型定義 | 与后端 API 契约一致的类型定义 |
| エンドポイント別ラッパー | 按端点分组的封装 |
| インスタンス | 实例 |
| インターセプター | 拦截器 |
| リダイレクト | 重定向 |
| ストア | store |
| ポーリング | 轮询 |
| 再利用ロジック | 可复用逻辑 |
| 色付き NTag | 带颜色的 NTag |
| フォーム | 表单 |
| 連動フィールド切り替え | 联动字段切换 |
| 破壊的操作の二次確認モーダル | 破坏性操作的二次确认弹窗 |
| 表示 | 显示 |
| 増分ポーリング | 增量轮询 |
| 検出ボタン + 結果表示 | 检测按钮 + 结果显示 |
| サイドナビ + ヘッダ + コンテンツ共通レイアウト | 侧边导航 + 头部 + 内容公用布局 |
| 下載ボタン追加 | 新增下载按钮 |
| 初回セットアップ | 首次安装 |
| ログイン | 登录 |
| カウントダウン対応 | 倒计时支持 |
| 状態バッジ | 状态徽章 |
| 起動/停止/再起動ボタン | 启动/停止/重启按钮 |
| 一覧 | 列表 |
| 新規/編集/削除 | 新增/编辑/删除 |
| 追加 | 添加 |
| 設定フォーム | 配置表单 |
| 接続設定フォーム | 连接配置表单 |
| ログビューア | 日志查看器 |
| パスワード変更フォーム | 修改密码表单 |
| 部署向导 | 部署向导（中日同形，保持） |
| トップレベルルート | 顶级路由 |
| 3ステップ | 3 步 |

### 4.3 边界

- 不改 `web/src/**.vue/.ts` 内的注释（如果有）—— 仅 dev-map.md 这一份文档。
- 函数 / 变量 / 文件名 / 字段名一律不动（"main.ts" 等英文标识符保留）。

---

## 5. 实施顺序

| 步 | 动作 | 验证 |
|---|---|---|
| 1 | 写 `scripts/start-e2e-server.ps1` | `pwsh -File scripts/start-e2e-server.ps1` 单独跑能起服；浏览器访问 127.0.0.1:8080 OK；Ctrl+C 退出 |
| 2 | 改 `web/playwright.config.ts` 加平台分支 | PowerShell 跑 `npx playwright test --project=chromium` 测试通过 |
| 3 | 改 `docs/dev-map.md` 日文 → 中文 | grep `[ぁ-んァ-ンー一-龥]` 应仅留汉字（无假名） |
| 4 | `bash scripts/archive-task.sh --task web-ui-mvp` | `ls docs/features/_archived/web-ui-mvp/` 含 7 阶段文档 |
| 5 | 更新 tasks.md：T-001 文档目录列 → `_archived/web-ui-mvp/` | 文本一致 |
| 6 | PowerShell + Git Bash 各跑一次 verify_all | 两边都 PASS: 18 |

---

## 6. 风险点

| # | 描述 | 缓解 |
|---|---|---|
| R-1 | pwsh 是 PowerShell 7+ 的命令名，Windows PowerShell 5.x 是 `powershell.exe`；用户机若未装 pwsh 会失败 | 项目其他 .ps1 已假设 pwsh 7+；与现状一致；README 已隐含 |
| R-2 | `pwsh -ExecutionPolicy Bypass` 在某些企业 GPO 下可能被阻挡 | 不能避免；项目场景为开发者本机 |
| R-3 | Playwright Node child_process 在 Windows 上 SIGTERM 不能优雅停 PowerShell 子进程，可能产生 stale frp-easy.exe | 加 `$ErrorActionPreference = "Stop"` + 让进程退出时数据目录留在 $env:TEMP（不阻断后续运行） |
| R-4 | start-e2e-server.ps1 写 frp_easy.toml 用 UTF-8 BOM 会被 BurntSushi/toml 拒绝 | 用 `[System.IO.File]::WriteAllText($path, $content, [System.Text.UTF8Encoding]::new($false))` 强制无 BOM |
| R-5 | 端口 8080 被占用 | 与 .sh 版同问题；不在本任务修复范围；docs/DEPLOYMENT.md F.3 已记录 |

---

## 7. Verdict

**READY FOR GATE REVIEW** — 进入 Stage 3。
