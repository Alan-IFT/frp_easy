package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// KVGet 读取 key 对应的 value。
// found=false 表示 key 不存在（非错误）；err 仅在 IO / SQL 真错时返回。
func (s *Store) KVGet(ctx context.Context, key string) (value string, found bool, err error) {
	row := s.db.QueryRowContext(ctx, `SELECT value FROM kv WHERE key = ?`, key)
	var v string
	err = row.Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("storage.KVGet(%q): %w", key, err)
	}
	return v, true, nil
}

// KVSet 写入 key/value（upsert）。
func (s *Store) KVSet(ctx context.Context, key, value string) error {
	if key == "" {
		return errors.New("storage.KVSet: empty key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("storage.KVSet(%q): %w", key, err)
	}
	return nil
}

// KVDelete 删除 key；不存在时返回 nil（idempotent）。
func (s *Store) KVDelete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM kv WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("storage.KVDelete(%q): %w", key, err)
	}
	return nil
}

// KVEntry 是一条 KV 行（KVListByPrefix 返回用）。
type KVEntry struct {
	Key   string
	Value string
}

// KVListByPrefix 返回所有 key 以 prefix 开头的 KV 行，按 key 升序。
// prefix 为空串时返回全部行（调用方自担风险，本项目不会这么用）。
// 无匹配返回空 slice + nil；仅 IO/SQL 真错时返回 err。
//
// 这是**机械的**前缀列举——不懂任何业务/过期语义。调用方（如 auth.RateLimiter
// 的 loginfail 计数清理）拿到行后自行判定哪些该删（T-063：过期语义留 ratelimit 层，
// 与限流窗口常量单点一致）。
//
// 实现用 `LIKE escapeLike(prefix)+'%' ESCAPE '\'`：SQLite LIKE 中 `%`/`_` 是通配符，
// escapeLike 把 prefix 中的 `\`/`%`/`_` 转义成字面字符，防止调用方传入含元字符的
// 前缀时误匹配（防御性——当前唯一调用点 "loginfail." 不含这些字符，转义后行为等价）。
func (s *Store) KVListByPrefix(ctx context.Context, prefix string) ([]KVEntry, error) {
	pattern := escapeLike(prefix) + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value FROM kv WHERE key LIKE ? ESCAPE '\' ORDER BY key`, pattern)
	if err != nil {
		return nil, fmt.Errorf("storage.KVListByPrefix(%q): %w", prefix, err)
	}
	defer rows.Close()

	var out []KVEntry
	for rows.Next() {
		var e KVEntry
		if err := rows.Scan(&e.Key, &e.Value); err != nil {
			return nil, fmt.Errorf("storage.KVListByPrefix(%q) scan: %w", prefix, err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.KVListByPrefix(%q) rows: %w", prefix, err)
	}
	return out, nil
}

// escapeLike 把 LIKE 模式里的元字符（`\` `%` `_`）转义成字面字符，
// 用反斜杠作转义符（与 KVListByPrefix 的 `ESCAPE '\'` 配套）。
// 注意 `\` 必须先转义，否则会把后续插入的转义符再转义。
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
