package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/frp-easy/frp-easy/internal/storage"
)

// 限流参数：5 次失败 / 60 秒滑窗（01 B-5 / AC-4 / NF-S 限流）。
const (
	failWindow = 60 * time.Second
	failMax    = 5
)

// LoginFailKeyPrefix 是 RateLimiter 持久化键的统一前缀（T-063 导出）。
// key(ip) == LoginFailKeyPrefix + ip，即 "loginfail.<ip>"。
// PurgeExpired 与单测引用此常量，避免 "loginfail." 字面量散落。
const LoginFailKeyPrefix = "loginfail."

// kvStore 是 RateLimiter 持久化所需的最小接口（取自 storage.Store 的
// KVGet/KVSet/KVDelete/KVListByPrefix）。用接口而非具体类型，便于在测试里用
// in-memory fake 注入。
type kvStore interface {
	KVGet(ctx context.Context, key string) (string, bool, error)
	KVSet(ctx context.Context, key, value string) error
	KVDelete(ctx context.Context, key string) error
	// KVListByPrefix 机械地按前缀列举 KV 行（T-063：过期判定留本包，见 PurgeExpired）。
	KVListByPrefix(ctx context.Context, prefix string) ([]storage.KVEntry, error)
}

// failRecord 是写入 KV 的 JSON 值。
type failRecord struct {
	Count   int       `json:"count"`
	FirstAt time.Time `json:"firstAt"`
}

// RateLimiter 基于 KV 表的滑窗限流器（per-IP）。
//
// 行为：
//   - 每次登录失败调用 RecordFailure(ip)，计数 +1。
//   - 计数达到 failMax (=5) 时，后续 Allow(ip) 在 failWindow (=60s) 内返回
//     (false, remaining)；remaining 由 firstAt + window - now 计算。
//   - 滑窗"满"=超出 firstAt + window 后下一次失败重置 firstAt + 计数 = 1。
//   - Reset(ip)：登录成功后清理。
//
// 持久化路径：KV key = "loginfail.<ip>"，value = JSON {count, firstAt}。
// 重启 UI 服务后窗口持续 —— 防止简单的"重启绕过"。
//
// 内部 sync.Mutex 仅保护"读 → 判断 → 写"复合操作的原子性，避免并发
// goroutine 互相覆盖；DB 写本身由 storage.Store 自己的 mu 兜底。
type RateLimiter struct {
	kv  kvStore
	now func() time.Time // 测试时可替换
	mu  sync.Mutex
}

// NewRateLimiter 建一个挂在 kv 上的限流器。
func NewRateLimiter(kv kvStore) *RateLimiter {
	return &RateLimiter{kv: kv, now: func() time.Time { return time.Now().UTC() }}
}

// Allow 询问 ip 当前能否再尝试一次登录。
// 返回 (allowed, retryAfter)：
//   - allowed=true → retryAfter=0，可继续走鉴权。
//   - allowed=false → retryAfter > 0，调用方应回 429 + Retry-After 头。
func (r *RateLimiter) Allow(ip string) (bool, time.Duration) {
	if ip == "" {
		return true, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.read(ip)
	if !ok {
		return true, 0
	}
	if rec.Count < failMax {
		return true, 0
	}
	expires := rec.FirstAt.Add(failWindow)
	now := r.now()
	if now.After(expires) {
		// 窗口已过期，惰性清理。
		_ = r.kv.KVDelete(context.Background(), key(ip))
		return true, 0
	}
	return false, expires.Sub(now)
}

// RecordFailure 在一次错误密码登录后调用。
// 返回新的计数与（若已达上限）剩余退避时间。
func (r *RateLimiter) RecordFailure(ip string) (count int, retryAfter time.Duration, err error) {
	if ip == "" {
		return 0, 0, errors.New("auth.RateLimiter: empty ip")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	rec, ok := r.read(ip)
	if !ok || now.After(rec.FirstAt.Add(failWindow)) {
		// 新窗口
		rec = failRecord{Count: 1, FirstAt: now}
	} else {
		rec.Count++
	}
	if err := r.write(ip, rec); err != nil {
		return rec.Count, 0, err
	}
	if rec.Count >= failMax {
		return rec.Count, rec.FirstAt.Add(failWindow).Sub(now), nil
	}
	return rec.Count, 0, nil
}

// Reset 清空 ip 的失败计数（登录成功时调用）。
func (r *RateLimiter) Reset(ip string) error {
	if ip == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.kv.KVDelete(context.Background(), key(ip))
}

// PurgeExpired 删除所有**已过期**的 loginfail.<ip> 计数行，返回删除条数（T-063）。
//
// 过期判定与 Allow 字节级同义：now.After(rec.FirstAt.Add(failWindow))。
// 这保证清理删除的行集 ⊆ Allow 在同一 now 下会惰性删除的行集——绝不删任何
// Allow 仍判 blocked 的活计数行（NF-S 限流不失效，02 §6 集合包含证明）。
//
// 损坏的 JSON 值（json.Unmarshal 失败）视为过期删除：Allow 的 read() 对损坏值
// 同样返回 ok=false → 该 IP 本就被放行，故删除损坏行对限流行为零影响，纯垃圾回收
// （02 §8 R-2）。
//
// best-effort：单条 KVDelete 失败不中止，继续清理其余，返回首个错误 + 已删数。
// 与 RecordFailure/Allow/Reset 共享 r.mu，保证"列举 → 逐条判定 → 删除"的原子性；
// 锁顺序与既有路径一致（先 r.mu，storage 内部各自取 s.mu）。
func (r *RateLimiter) PurgeExpired(ctx context.Context) (purged int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, lerr := r.kv.KVListByPrefix(ctx, LoginFailKeyPrefix)
	if lerr != nil {
		return 0, fmt.Errorf("auth.RateLimiter.PurgeExpired list: %w", lerr)
	}
	now := r.now()
	var firstErr error
	for _, e := range entries {
		var rec failRecord
		expired := false
		if uerr := json.Unmarshal([]byte(e.Value), &rec); uerr != nil {
			// 损坏值：当作过期垃圾删除（对限流零影响，见上）。
			expired = true
		} else if now.After(rec.FirstAt.Add(failWindow)) {
			expired = true
		}
		if !expired {
			continue
		}
		if derr := r.kv.KVDelete(ctx, e.Key); derr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("auth.RateLimiter.PurgeExpired delete(%q): %w", e.Key, derr)
			}
			continue
		}
		purged++
	}
	return purged, firstErr
}

func (r *RateLimiter) read(ip string) (failRecord, bool) {
	v, ok, err := r.kv.KVGet(context.Background(), key(ip))
	if err != nil || !ok {
		return failRecord{}, false
	}
	var rec failRecord
	if err := json.Unmarshal([]byte(v), &rec); err != nil {
		return failRecord{}, false
	}
	return rec, true
}

func (r *RateLimiter) write(ip string, rec failRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return r.kv.KVSet(context.Background(), key(ip), string(b))
}

func key(ip string) string {
	return LoginFailKeyPrefix + ip
}
