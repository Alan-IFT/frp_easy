# Development Record — Frontend partition

## Partition

dev-frontend — owns: `web/**`

## Files changed (this partition only)

### 設定ファイル
- `web/package.json` — 依存関係定義（Vue3/Pinia/NaiveUI/Axios + dev: Vite/vue-tsc/ESLint/Vitest/@vue/test-utils/happy-dom）
- `web/vite.config.ts` — Vite 設定（ビルド先 `../internal/assets/dist`、dev proxy `/api → :8080`）
- `web/tsconfig.json` — TypeScript strict モード（skipLibCheck 追加）
- `web/vitest.config.ts` — Vitest 設定（environment: happy-dom）
- `web/.eslintrc.cjs` — ESLint 設定（eslint-plugin-vue vue3-recommended + @typescript-eslint/parser）
- `web/index.html` — SPA エントリ HTML

### エントリポイント
- `web/src/main.ts` — createApp + Pinia + Router + CSRF トークンゲッター登録
- `web/src/App.vue` — ルートコンポーネント（router-view のみ）
- `web/src/router.ts` — Vue Router 4 (history mode)；ナビゲーションガード（initialized チェック / 認証チェック）
- `web/src/types.ts` — バックエンド API 契約と一致する型定義（Proxy, ProcessInfo, FrpsConfig 等）

### API クライアント層（`web/src/api/`）
- `web/src/api/client.ts` — axios インスタンス（CSRF インターセプター、401 → /login リダイレクト）
- `web/src/api/auth.ts` — `/api/v1/auth/*` / `/api/v1/setup` ラッパー
- `web/src/api/system.ts` — `/api/v1/system/ready`
- `web/src/api/proxies.ts` — `/api/v1/proxies` CRUD
- `web/src/api/server.ts` — `/api/v1/server`（reveal クエリ対応）
- `web/src/api/frpclient.ts` — `/api/v1/client`（名前衝突回避のため frpclient.ts）
- `web/src/api/proc.ts` — `/api/v1/proc/{kind}/start|stop|restart` + `/proc/status`
- `web/src/api/logs.ts` — `/api/v1/logs/{kind}` tail / incremental 両モード
- `web/src/api/mode.ts` — `/api/v1/mode`

### Pinia ストア（`web/src/stores/`）
- `web/src/stores/auth.ts` — user/csrfToken；login/logout/checkMe/fetchCsrf/setup
- `web/src/stores/proc.ts` — frpc/frps ProcessInfo；2s ポーリング startPolling/stopPolling
- `web/src/stores/proxies.ts` — Proxy[] CRUD
- `web/src/stores/app.ts` — initialized/binMissing/version；fetchReady()

### Composables（`web/src/composables/`）
- `web/src/composables/statusUtils.ts` — getTagType/getStateLabel（ProcessState → Naive UI タグ色/ラベル）
- `web/src/composables/useProxyForm.ts` — ProxyForm フォームロジック（isTcpUdp/isHttpHttps/toProxyInput/syncFromInput）

### コンポーネント（`web/src/components/`）
- `web/src/components/AppLayout.vue` — サイドナビ + ヘッダ（binMissing バナー、ログアウト）+ コンテンツ
- `web/src/components/StatusBadge.vue` — ProcessState → 色付き NTag
- `web/src/components/ProxyForm.vue` — Proxy 新規/編集フォーム（type 連動フィールド切り替え、楽観的ロック version）
- `web/src/components/ConfirmDialog.vue` — 削除確認二次確認モーダル
- `web/src/components/LogViewer.vue` — ログビューア（TailLines 初期表示 + 2s 増分ポーリング）

### ページ（`web/src/pages/`）
- `web/src/pages/Setup.vue` — 初回セットアップ（username + password 検証 ≥12字・英数字混在）
- `web/src/pages/Login.vue` — ログイン（429 → Retry-After カウントダウン表示）
- `web/src/pages/Dashboard.vue` — frpc/frps 状態バッジ + 起動/停止/再起動ボタン + error 時ログ摘要
- `web/src/pages/Proxies.vue` — Proxy 一覧テーブル + 新規/編集/削除（確認ダイアログ経由）
- `web/src/pages/Server.vue` — frps 設定フォーム（bindPort/authToken/dashboard 設定）
- `web/src/pages/Client.vue` — frpc 接続設定フォーム（serverAddr/serverPort/authToken）
- `web/src/pages/Logs.vue` — ログビューア（LogViewer コンポーネント利用）
- `web/src/pages/Settings.vue` — パスワード変更フォーム（旧パスワード検証 + 新パスワード強度チェック）

### テスト（`web/src/**/__tests__/`）
- `web/src/stores/__tests__/auth.spec.ts` — AuthStore login/logout/checkMe/fetchCsrf（axios モック）
- `web/src/stores/__tests__/proc.spec.ts` — ProcStore pollStatus/startPolling/stopPolling（fake timers）
- `web/src/components/__tests__/StatusBadge.spec.ts` — getTagType/getStateLabel 関数ユニットテスト
- `web/src/components/__tests__/ProxyForm.spec.ts` — useProxyForm composable テスト（type 切り替えフィールド制御）

## Out-of-partition coordination

フロントエンド実装において `internal/assets/dist/` へのビルド産物書き込みが発生したが、02 §F-2 / Gate Review C-1 の決定「`internal/assets/dist/` は構建产物、dev-frontend の `npm run build` 書き込みは越界でない」に従い合法として処理。

Go コード（`internal/assets/embed.go` の `//go:embed all:dist` 有効化）は dev-backend Round 2 が担当。本パーティションでは触れない。

## 設計上の注意点

### `<script setup>` での export 制限
`StatusBadge.vue` 当初は `<script setup>` 内に `export function` を記述したが、`vue/no-export-in-script-setup` ルールでエラー。composable として `src/composables/statusUtils.ts` に分離した。テストは `StatusBadge.vue` 直接でなく composable をインポートする形に変更。

### ProxyForm テスト戦略
Naive UI コンポーネントのスタブ経由 DOM テストは attribute binding が期待通り動作しなかったため、`useProxyForm` composable を抽出してロジックをピュアに単体テストする方針に変更。これにより DOM 依存なしで type 切り替えロジックを完全テストできる。

### CSRF トークンの循環依存回避
`api/client.ts` が Pinia ストアを直接 import すると循環依存が発生するため、`setCsrfTokenGetter()` 関数でゲッターを後から注入する方式を採用。`main.ts` で Pinia 初期化後に登録する。

## verify_all result

verify_all の B 系ステップは `web/` サブディレクトリ内の `npm` で個別実行：

```
npm run lint   → PASS（ESLint エラー 0）
vue-tsc --noEmit → PASS（型エラー 0）
npm run build  → PASS（internal/assets/dist/index.html 生成）
npm run test   → PASS（32 tests passed）
```

`scripts/verify_all` は `web/package.json` を自動検出しないため、SKIP 扱いとなる（02 §10.3 設計通り）。Go 系の G.* ステップは dev-backend 管轄につき本パーティションでは不変。

## Verdict

READY FOR REVIEW (frontend partition complete)
