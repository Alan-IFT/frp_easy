package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
