package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// Session 表示一条登录会话。CSRFToken 与 Token 一一对应（02 §5.1 NF-S3 双保险）。
type Session struct {
	Token     string
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateSession 生成新会话（Token 与 CSRFToken 均 crypto/rand 32B base64url）
// 并写入 sessions 表，TTL = ttl（建议 12h）。
func (s *Store) CreateSession(ctx context.Context, ttl time.Duration) (*Session, error) {
	if ttl <= 0 {
		return nil, errors.New("storage.CreateSession: ttl must be > 0")
	}
	token, err := randToken()
	if err != nil {
		return nil, fmt.Errorf("storage.CreateSession token: %w", err)
	}
	csrf, err := randToken()
	if err != nil {
		return nil, fmt.Errorf("storage.CreateSession csrf: %w", err)
	}
	now := time.Now().UTC()
	expires := now.Add(ttl)

	s.mu.Lock()
	defer s.mu.Unlock()
	const q = `
		INSERT INTO sessions (token, csrf_token, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`
	if _, err := s.db.ExecContext(ctx, q, token, csrf,
		now.Format(time.RFC3339Nano), expires.Format(time.RFC3339Nano)); err != nil {
		return nil, fmt.Errorf("storage.CreateSession insert: %w", err)
	}
	return &Session{
		Token:     token,
		CSRFToken: csrf,
		CreatedAt: now,
		ExpiresAt: expires,
	}, nil
}

// GetSession 按 token 查询会话。
// 找不到返回 (nil, ErrNotFound)；找到但已过期返回 (nil, ErrNotFound) 且不自动删除
// （删除由 PurgeExpiredSessions 周期任务负责，避免每次读引入写）。
func (s *Store) GetSession(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	const q = `SELECT token, csrf_token, created_at, expires_at FROM sessions WHERE token = ?`
	var (
		sess                 Session
		createdStr, expStr   string
	)
	err := s.db.QueryRowContext(ctx, q, token).Scan(&sess.Token, &sess.CSRFToken, &createdStr, &expStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("storage.GetSession: %w", err)
	}
	created, err := parseSQLiteTime(createdStr)
	if err != nil {
		return nil, fmt.Errorf("storage.GetSession parse created: %w", err)
	}
	expires, err := parseSQLiteTime(expStr)
	if err != nil {
		return nil, fmt.Errorf("storage.GetSession parse expires: %w", err)
	}
	sess.CreatedAt = created
	sess.ExpiresAt = expires
	if time.Now().UTC().After(sess.ExpiresAt) {
		return nil, ErrNotFound
	}
	return &sess, nil
}

// DeleteSession 删除指定 token；不存在时返回 nil（idempotent）。
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("storage.DeleteSession: %w", err)
	}
	return nil
}

// PurgeExpiredSessions 删除所有 expires_at < now 的会话；返回删除条数。
func (s *Store) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("storage.PurgeExpiredSessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// randToken 返回 32 字节 base64url-encoded（无填充）随机串。
func randToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
