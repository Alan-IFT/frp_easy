package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Admin 表示系统唯一管理员（admin 表 CHECK(id = 1) 强制单行）。
type Admin struct {
	Username     string
	PasswordHash string // PHC 串，由 internal/auth 产出（02 §6.2 argon2id）
	UpdatedAt    time.Time
}

// GetAdmin 返回当前管理员记录。
// 若 admin 表为空（首次启动尚未 setup）返回 (nil, nil)。
func (s *Store) GetAdmin(ctx context.Context) (*Admin, error) {
	const q = `SELECT username, password_hash, updated_at FROM admin WHERE id = 1`
	var (
		a         Admin
		updatedAt string
	)
	err := s.db.QueryRowContext(ctx, q).Scan(&a.Username, &a.PasswordHash, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage.GetAdmin: %w", err)
	}
	ts, err := parseSQLiteTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("storage.GetAdmin parse time: %w", err)
	}
	a.UpdatedAt = ts
	return &a, nil
}

// SetAdmin 插入或更新唯一管理员记录（INSERT OR REPLACE）。
// passwordHash 调用方负责（不在本包做哈希）。
func (s *Store) SetAdmin(ctx context.Context, username, passwordHash string) error {
	if username == "" || passwordHash == "" {
		return errors.New("storage.SetAdmin: username and passwordHash required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	const q = `
		INSERT INTO admin (id, username, password_hash, updated_at)
		VALUES (1, ?, ?, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			username = excluded.username,
			password_hash = excluded.password_hash,
			updated_at = datetime('now')
	`
	if _, err := s.db.ExecContext(ctx, q, username, passwordHash); err != nil {
		return fmt.Errorf("storage.SetAdmin: %w", err)
	}
	return nil
}
