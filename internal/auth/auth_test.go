package auth

import (
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- HashPassword / VerifyPassword ---

func TestHashAndVerify_RoundTrip(t *testing.T) {
	p := "correct horse battery staple 123!"
	enc, err := HashPassword(p)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$") {
		t.Fatalf("bad PHC prefix: %s", enc)
	}
	ok, err := VerifyPassword(p, enc)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatal("expected verify ok")
	}
}

func TestVerify_WrongPassword(t *testing.T) {
	enc, _ := HashPassword("right-password-123")
	ok, err := VerifyPassword("wrong-password-456", enc)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ok {
		t.Fatal("expected mismatch")
	}
}

func TestVerify_MalformedPHC(t *testing.T) {
	cases := []string{
		"",
		"not-a-phc",
		"$argon2id$v=19$m=65536,t=3,p=2$$",                     // 空 salt/hash
		"$bcrypt$v=19$m=65536,t=3,p=2$YWJj$ZGVm",                // 错误 algo
		"$argon2id$v=20$m=65536,t=3,p=2$YWJjZGVm$YWJjZGVmZw",   // 错误 version
	}
	for _, c := range cases {
		_, err := VerifyPassword("any", c)
		if err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestHashPassword_EmptyRejected(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Fatal("expected error")
	}
}

// --- token ---

func TestGenerateSessionToken_LengthAndCharset(t *testing.T) {
	tok, err := GenerateSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	// 32 字节 base64url 无填充 = 43 字符
	if len(tok) != 43 {
		t.Fatalf("len = %d, want 43; tok=%s", len(tok), tok)
	}
	if _, err := base64.RawURLEncoding.DecodeString(tok); err != nil {
		t.Fatalf("not base64url: %v", err)
	}
}

func TestGenerateCSRFToken_LengthAndCharset(t *testing.T) {
	tok, err := GenerateCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 43 {
		t.Fatalf("len = %d", len(tok))
	}
}

func TestTokensAreUnique(t *testing.T) {
	a, _ := GenerateSessionToken()
	b, _ := GenerateSessionToken()
	if a == b {
		t.Fatal("two consecutive tokens collided — entropy broken")
	}
}

// --- RateLimiter ---

// fakeKV 实现 kvStore 接口，纯 in-memory，不依赖 sqlite。
type fakeKV struct {
	mu sync.Mutex
	m  map[string]string
}

func newFakeKV() *fakeKV { return &fakeKV{m: map[string]string{}} }

func (f *fakeKV) KVGet(_ context.Context, k string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.m[k]
	return v, ok, nil
}
func (f *fakeKV) KVSet(_ context.Context, k, v string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[k] = v
	return nil
}
func (f *fakeKV) KVDelete(_ context.Context, k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, k)
	return nil
}

func TestRateLimiter_FirstFiveAllowedSixthBlocked(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return now }

	ip := "127.0.0.1"
	for i := 1; i <= 5; i++ {
		allowed, _ := rl.Allow(ip)
		if !allowed {
			t.Fatalf("attempt %d should be allowed before recording", i)
		}
		count, retry, err := rl.RecordFailure(ip)
		if err != nil {
			t.Fatal(err)
		}
		if count != i {
			t.Fatalf("count want %d got %d", i, count)
		}
		if i < 5 && retry != 0 {
			t.Fatalf("attempt %d should not retry-after yet", i)
		}
		if i == 5 && retry <= 0 {
			t.Fatalf("attempt 5 should set retry-after, got %v", retry)
		}
	}

	allowed, retry := rl.Allow(ip)
	if allowed {
		t.Fatal("6th attempt must be blocked")
	}
	if retry <= 0 || retry > 60*time.Second {
		t.Fatalf("retry-after out of range: %v", retry)
	}
}

func TestRateLimiter_WindowExpires(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	base := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return base }

	ip := "10.0.0.1"
	for i := 0; i < 5; i++ {
		_, _, _ = rl.RecordFailure(ip)
	}
	allowed, _ := rl.Allow(ip)
	if allowed {
		t.Fatal("must be blocked at t=0")
	}

	// 推进 70 秒（> 60s 窗口）。
	rl.now = func() time.Time { return base.Add(70 * time.Second) }
	allowed, retry := rl.Allow(ip)
	if !allowed || retry != 0 {
		t.Fatalf("after window: allowed=%v retry=%v", allowed, retry)
	}
}

func TestRateLimiter_ResetOnSuccess(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	ip := "1.2.3.4"
	for i := 0; i < 5; i++ {
		_, _, _ = rl.RecordFailure(ip)
	}
	if allowed, _ := rl.Allow(ip); allowed {
		t.Fatal("should be blocked")
	}
	if err := rl.Reset(ip); err != nil {
		t.Fatal(err)
	}
	if allowed, _ := rl.Allow(ip); !allowed {
		t.Fatal("should be allowed after reset")
	}
}

func TestRateLimiter_EmptyIPNoop(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	if allowed, _ := rl.Allow(""); !allowed {
		t.Fatal("empty ip should be allowed (no-op)")
	}
	if _, _, err := rl.RecordFailure(""); err == nil {
		t.Fatal("expected error on empty ip")
	}
	if err := rl.Reset(""); err != nil {
		t.Fatal(err)
	}
}
