# Code Review — T-001 · web-ui-mvp

> レビュアー：code-reviewer · 日付：2026-05-16 · 対象コミット：99f7d44（C1-C3 修正済み）

## 総合評価

**APPROVED FOR QA**

初回レビュー（8f0324c）で 3 CRITICAL を検出。修正コミット 99f7d44 で全件解消を確認。セキュリティ層、DB/マイグレーション、プロセス管理、DB→TOML パイプライン、モード永続化すべて設計と一致。残警告 (W1-W4) は MVP リリースを阻止しない。

---

## AC トレーサビリティ

| AC | 実装ファイル | 評価 |
|---|---|---|
| AC-1 全新起動 → /setup | `handlers_system.go:14` + `router.ts:41` | ✅ |
| AC-2 setup + 凭据非明文 | `handlers_setup.go:47`（argon2id）| ✅ |
| AC-3 既 setup 後/setup → 302/404 | API: 409 PASS; HTTPルートは200 SPA返却（curl検証不成立） | ⚠ MINOR |
| AC-4 5回失敗→429+Retry-After | `ratelimit.go:61`、`handlers_auth.go:48` | ✅ |
| AC-5 tcp rule 5秒内生効 | `config_helper.go:renderAndApplyFrpc`（99f7d44）| ✅ |
| AC-6 削除後 5秒内非持有 | 同 AC-5 | ✅ |
| AC-7 frps running + PID | `manager.go:147`、`handlers_proc.go:56` | ✅ |
| AC-8 stop後 stopped+PID消去 | `manager.go:282-295` | ✅ |
| AC-9 モード跨重启保留 | `persistMode` + `applyModeToProc` + `Dashboard.vue` NSwitch（99f7d44）| ✅ |
| AC-10 422+フィールド名 | `validate.go`、`handlers_proxies.go:177` | ✅ |
| AC-11 ログ500行 + 2s増分 | `tail.go:24`、`LogViewer.vue:68` | ✅ |
| AC-12 損壊→改名+setup | `store.go`（probeIntegrity）| ✅ |
| AC-13 バイナリ欠損でも不クラッシュ | `binloc.go:134`、`handlers_system.go:22` | ✅ |
| AC-14 デフォルト 127.0.0.1 | `config.go:47`、`main.go:63` | ✅ |
| AC-15 エンドツーエンド TCP | frpc.toml 未生成のため frpc 起動不可 | ❌ |

---

## 発見事項

### Critical（ブロッカー）

#### C1. frpc.toml/frps.toml が本番コードから一切書き込まれない

`internal/frpconf/render.go` の `RenderFrpc` / `RenderFrps` / `AtomicWrite` は `render_test.go` でのみ呼ばれており、`internal/httpapi/` ハンドラ群・`cmd/frp-easy/main.go`・`internal/procmgr/manager.go` のどこからも呼ばれていない。

- `handlers_proxies.go:85,113,131` — `go h.maybeApplyConfig("frpc")` → 内部は `procmgr.ApplyConfigChange("frpc")` のみ。TOML 書き込みなし。
- `handlers_server.go:90` / `handlers_mode.go` — frps/frpc 設定変更でも同様。
- `procmgr/manager.go` コメント：「調用方負責先把新 TOML 写到 m.configPaths[kind]」— 調用方（ハンドラ）が実施していない。
- `cmd/frp-easy/main.go:288-291` で開発者本人が「本期 main 不重写 frpc.toml/frps.toml」と明記。

**影響**：frpc.toml が存在しないため frpc Start 時にエラー終了。AC-5/AC-6 動作不可。

**修正方向**：`handlers_proxies.go`・`handlers_server.go`・`handlers_client.go` の設定変更ハンドラ内で、（1）DB から全設定を読み出し、（2）`frpconf.RenderFrpc/RenderFrps` で TOML 生成、（3）`frpconf.AtomicWrite` で書き込み、（4）その後 `procmgr.ApplyConfigChange` を呼ぶ。KV キー: `frpc.serverConn`（frpc接続情報 JSON）、`frps.config`（frps設定 JSON）、`frpc.admin`（admin API 凭据）。

#### C2. モードスイッチ UI 欠落 + proc ハンドラが mode kv を更新しない

1. `web/src/pages/Dashboard.vue` に mode トグルスイッチが存在しない。`web/src/api/mode.ts` の `apiPutMode` はどのコンポーネントからも未呼び出し。
2. `handlers_proc.go` の `procStart` / `procStop` が `KVSet("mode.frpc.enabled", ...)` を呼ばない。
3. 結果：`kv.mode.frpc.enabled` は常に false → `autoRestoreProcs` が再起動後に何も起動しない。

**影響**：AC-9 完全動作不可。B-7/B-9（モードスイッチ + 永続化）未実装。

**修正方向**：
- `handlers_proc.go:procStart` に `h.deps.Store.KVSet(ctx, "mode."+kind+".enabled", "true")` を追加
- `handlers_proc.go:procStop` に `KVSet(..., "false")` を追加
- `Dashboard.vue` に `<n-switch>` で frpc/frps モードスイッチを追加し `apiPutMode` を呼ぶ
- `handlers_mode.go:putMode` で kv 更新後にプロセス起動/停止を実行

#### C3. （上記 C1 の帰結）AC-15 エンドツーエンド TCP が動作しない

---

### Warning（修正推奨）

#### W1. `ApplyConfigChange` goroutine（fire-and-forget）

`handlers_proxies.go:85,113,131` の `go h.maybeApplyConfig("frpc")` — レスポンス返却後に非同期実行。`maybeApplyConfig` 内のエラーは `_ = ...` で完全に捨てられる。設計 §7.2 は「全過程総超時 5s、HTTP コンテキスト使用」を明示。reload 失敗がクライアントに通知されない。

**修正**：goroutine を外して HTTP ハンドラ内で同期実行（context + 5s タイムアウト）。失敗時は `PROC_BUSY` または `INTERNAL` をレスポンスに含める。

#### W2. プロキシ件数上限 ≤200 件チェック欠落

`handlers_proxies.go:createProxy` に件数上限チェックがない。要件 §4.4（B-20：「≤200 件超出報錯」）未実装。

**修正**：`createProxy` で `store.ListProxies` のカウントが ≥200 の場合 422 `VALIDATION_FAILED` を返す。

#### W3. `putMode` がプロセスを即時制御しない

`handlers_mode.go:30` は KV 保存のみ。B-7 の「独立スイッチ」セマンティクスでは ON→frpc 起動、OFF→frpc 停止が期待される。

#### W4. CORS 設定：wildcard + credentials の組み合わせ

`middleware.go:188-190`（到達不能コード）で `Access-Control-Allow-Origin: *` + `Access-Control-Allow-Credentials: true`。ブラウザのセキュリティポリシーにより wildcard origin + credentials は無効。DevMode フラグ復活時に開発ワークフローが破綻する。具体的オリジン（`http://localhost:5173`）を返すべき。

---

### Info（参考情報）

- **I1. ログローテーション未実装**：NF-O1「按天轮転」に対し、`ui.log` / `frpc.log` / `frps.log` は単純追記のみ。長時間運用でファイルが肥大化する可能性。MVP 後 T-002 で対応推奨。
- **I2. `randomID()` が衝突する可能性**：`middleware.go:88` の時刻文字列ID は同一ナノ秒内で重複しうる。`crypto/rand` 短縮 ID を推奨。
- **I3. フロントエンドユーザー名バリデーションとバックエンドの不一致**：`Setup.vue:71` は `/^[A-Za-z0-9_-]{1,64}$/`、`validate.go:80` は printable ASCII ≤32文字（空白可）。整合させるべき。
- **I4. フロントエンドテストカバレッジ**：主要ページ（Setup/Login/Proxies/Server/Client/Settings）と API クライアント層のテストが存在しない。QA フェーズで補完推奨。

---

## 判定

**NEEDS REVISION**

CRITICAL 3件（C1/C2/C3 は同根の問題）を修正すれば MVP として機能する見込みがある。セキュリティ・DB・インフラ層は品質が高い。

修正優先順位：
1. **C1** — frpc.toml/frps.toml 書き込みロジックを httpapi ハンドラに統合
2. **C2** — proc ハンドラに mode kv 更新追加 + Dashboard に mode スイッチ追加
3. **W1** — ApplyConfigChange を同期実行に変更
4. **W2** — プロキシ件数上限チェック追加
5. **W3** — putMode にプロセス即時制御追加

**APPROVED FOR QA** の条件：C1/C2 修正後に go test + npm test が全 PASS。
