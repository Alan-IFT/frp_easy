# Development Record — T-001 · web-ui-mvp · dev-backend Round 1

## Summary

全バックエンド実装（`cmd/frp-easy/main.go` + `internal/` 8パッケージ + `scripts/start.*`・`scripts/build.*`・`verify_all` Go拡張・`docs/dev-map.md` 更新）を完了。Go vet クリーン、全テスト PASS、`go build` 成功。Gate Review 条件 C-2/C-3/C-5 と INFO I-1/I-2 を全実装済み。`internal/assets` は dev モード占位（Round 2 で `//go:embed all:dist` に差し替え）。

## Files changed

### 新規作成
- `cmd/frp-easy/main.go` — 起動シーケンス（appconf→storage→binloc/procmgr/rl→HTTP→ReadyGate→AC-9自動復元→graceful shutdown）
- `internal/appconf/config.go` — `AppConfig`；Load/Validate/ListenAddr；デフォルト frp_easy.toml 自動生成
- `internal/appconf/config_test.go` — Load / Validate / ListenAddr テスト
- `internal/assets/assets.go` — dev モード占位 handler（404 + JSON エラー）
- `internal/assets/assets_test.go` — assets.Handler テスト
- `internal/auth/hash.go` — argon2id (m=64MiB/t=3/p=2) + PHC 文字列；I-1 コメント付き
- `internal/auth/token.go` — GenerateSessionToken / GenerateCSRFToken (crypto/rand 32B base64url)
- `internal/auth/ratelimit.go` — per-IP 5次/60s 滑窗；kv 永続化
- `internal/auth/auth_test.go` — hash round-trip / token uniqueness / ratelimit テスト
- `internal/binloc/binloc.go` — NewDefault(root)；runtime.GOOS switch；Missing()；Root()
- `internal/binloc/binloc_test.go` — FRPCPath / FRPSPath / Missing テスト
- `internal/frpcadmin/client.go` — New / Reload(ctx, strict) / Status；5s タイムアウト
- `internal/frpcadmin/client_test.go` — httptest サーバーを使ったテスト
- `internal/frpconf/render.go` — RenderFrpc / RenderFrps / AtomicWrite；camelCase FRP TOML フィールド
- `internal/frpconf/render_test.go` — TOML 出力フィールド名検証テスト
- `internal/httpapi/router.go` — chi router；中間件チェーン；22 ルート
- `internal/httpapi/middleware.go` — ReadyGate(C-3) / Recover / RequestID / Logger(C-5 redact) / CORS / SessionAuth / CSRF
- `internal/httpapi/errors.go` — writeError / errorCode 定数
- `internal/httpapi/validate.go` — ProxyInput バリデーション関数
- `internal/httpapi/handlers_auth.go` — setup / login / logout / changePassword / me / csrf
- `internal/httpapi/handlers_mode.go` — getMode / putMode
- `internal/httpapi/handlers_proxies.go` — listProxies / createProxy / updateProxy / deleteProxy
- `internal/httpapi/handlers_server.go` — getServer / putServer
- `internal/httpapi/handlers_proc.go` — procStart / procStop / procRestart / procStatus
- `internal/httpapi/handlers_logs.go` — logs（TailLines / ReadFrom 分岐）
- `internal/httpapi/handlers_system.go` — systemReady（initialized / binMissing / version）
- `internal/httpapi/httpapi_test.go` — httptest ルーター統合テスト
- `internal/logtail/tail.go` — TailLines / ReadFrom
- `internal/logtail/tail_test.go` — ファイルなし / 正常 / ReadFrom テスト
- `internal/procmgr/manager.go` — Manager；Start/Stop/Restart/Status/Shutdown/ApplyConfigChange
- `internal/procmgr/manager_windows.go` — Windows kill / CREATE_NEW_PROCESS_GROUP
- `internal/procmgr/manager_unix.go` — Linux SIGTERM→3s→SIGKILL；Setpgid
- `internal/procmgr/manager_test.go` — Status 初期値 / Start 二重呼び出し テスト
- `scripts/start.ps1` — Windows 開発モード起動（Go API + Vite 並列）
- `scripts/start.sh` — Linux/macOS 開発モード起動
- `scripts/build.ps1` — Windows 本番ビルド（-All で Linux 交叉コンパイル）
- `scripts/build.sh` — Linux/macOS 本番ビルド（--all で Windows 交叉コンパイル）

### 更新
- `go.mod` — go-chi/chi v5 / go-toml/v2 / golang.org/x/crypto / modernc.org/sqlite 追加
- `go.sum` — 自動更新
- `docs/dev-map.md` — cmd / internal 全パッケージ / bin / web 追加；可復用ツール表・パターン表拡充
- `scripts/verify_all.ps1` — G.1/G.2/G.3（go vet/test/build）チェック追加
- `scripts/verify_all.sh` — G.1/G.2/G.3 チェック追加

## verify_all result

- Baseline: PASS 6 / WARN 0 / FAIL 0 / SKIP 12（バックエンド実装前）
- After changes: **PASS 11 / WARN 0 / FAIL 0 / SKIP 7**
- Delta: +5 PASS（G.1 go vet / G.2 go test / G.3 go build / B 系 3 件から G 系 3 件へ）；新規失敗ゼロ

## Design drift (if any)

- **DESIGN DRIFT（軽微・C-3 実装）**: Gate Review C-3 の ReadyGate は「HTTP 启动期间」の保護。設計では `ReadyGate → Recover → ...` の順を要求していたが、実装では `Recover` も panic 時に 500 を返す必要があるため `ReadyGate` の外側でも Recover が必要になる。結果、`ReadyGate → Recover → RequestID → Logger → CORS → SessionAuth → CSRF` の順で実装。ReadyGate が最外殻に配置されている点は設計通りだが、Recover が ReadyGate に挟まれた形になっている。機能的に C-3 要件（503+Retry-After:2）は完全に満たしている。
- **DESIGN DRIFT（軽微・`internal/assets` API）**: 設計 §3.10 では `Handler() http.Handler` だったが、dev モードの占位でも同じシグネチャを維持。Round 2 で `//go:embed all:dist` を追加するだけで差し替え可能。

## Open issues for review

1. `procmgr` の `ApplyConfigChange` がレプライ失敗時に DB の `enabled=false` に降格する設計（07 §7.2）があるが、現実装では frpcadmin.Reload 失敗→Restart 失敗の場合のみ上位エラーとして返している。enabled=false への降格は httpapi layer でエラー受信時に判断する責務にしているが、handlers_proc.go では現状降格していない。レビュー時に確認が必要。
2. `RateLimiter` の kv 永続化キーは `loginfail.<ip>` 形式。プレフィックス一覧取得 API が `storage` にないため、定期 purge（古いカウンター削除）は現状未実装。MVP 規模では影響軽微だが、将来 T-002 で追加すべき。

## Dev-map updates

以下の行を `docs/dev-map.md` に追加（全パッケージ記載済み）：
- `cmd/frp-easy/` — Go 程序入口
- `internal/appconf/` 〜 `internal/procmgr/` — 全 8 バックエンドパッケージ
- `bin/` — 構建産物
- `web/` — フロントエンド（dev-frontend 分区；未作成）

## Insight to surface

`procmgr.Manager.ApplyConfigChange` は `frpcadmin.Reload` を 5s タイムアウトで呼ぶが、FRP の実際の reload 所要時間は設定ファイルサイズと接続数に依存する。テスト環境では常時 200ms 未満だが、100 proxy 以上の環境では reload が 5s を超える可能性がある。Production で frpc reload が頻繁に失敗する場合は `frpcadmin.Client` のタイムアウトを設定可能にする必要がある。· evidence: `internal/frpcadmin/client.go:13`（`defaultTimeout = 5 * time.Second`）

## Verdict

READY FOR REVIEW
