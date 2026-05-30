package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/frp-easy/frp-easy/internal/storage"
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
func (f *fakeKV) KVListByPrefix(_ context.Context, prefix string) ([]storage.KVEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []storage.KVEntry
	for k, v := range f.m {
		if strings.HasPrefix(k, prefix) {
			out = append(out, storage.KVEntry{Key: k, Value: v})
		}
	}
	// 按 key 升序，与真 Store.KVListByPrefix 的 ORDER BY key 对齐（确定性断言）。
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
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

// --- PurgeExpired (T-063) ---

// seedFail 直接往 fakeKV 写一条 loginfail.<ip> 记录（count + firstAt），绕过 RecordFailure
// 以便精确控制 firstAt，构造过期/活/边界场景。
func seedFail(t *testing.T, kv *fakeKV, ip string, count int, firstAt time.Time) {
	t.Helper()
	b, err := json.Marshal(failRecord{Count: count, FirstAt: firstAt})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := kv.KVSet(context.Background(), LoginFailKeyPrefix+ip, string(b)); err != nil {
		t.Fatalf("seed KVSet: %v", err)
	}
}

func hasKey(kv *fakeKV, k string) bool {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	_, ok := kv.m[k]
	return ok
}

// TestPurgeExpired_RemovesExpiredKeepsLive 核心正确性（T-063 AC-3 / BC-2 / BC-3 / BC-4 / NF-S）：
// 过期行被删、窗口内活行保留、窗口边界 now==firstAt+window 不删（与 Allow 的严格 After 一致）。
func TestPurgeExpired_RemovesExpiredKeepsLive(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return now }

	// 过期：firstAt 在 window 之前更久（now - 120s > now - 60s）。
	seedFail(t, kv, "9.9.9.9", failMax, now.Add(-120*time.Second))
	// 活：firstAt 在窗口内（now - 30s）。
	seedFail(t, kv, "1.1.1.1", failMax, now.Add(-30*time.Second))
	// 边界：firstAt 恰好 now - 60s → expires == now，After(now) 为 false → 不删。
	seedFail(t, kv, "2.2.2.2", failMax, now.Add(-failWindow))

	purged, err := rl.PurgeExpired(context.Background())
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if purged != 1 {
		t.Fatalf("purged got %d want 1 (only the 120s-old expired row)", purged)
	}
	if hasKey(kv, LoginFailKeyPrefix+"9.9.9.9") {
		t.Fatal("expired row 9.9.9.9 should be purged")
	}
	if !hasKey(kv, LoginFailKeyPrefix+"1.1.1.1") {
		t.Fatal("live row 1.1.1.1 must survive purge (rate limit must not be weakened)")
	}
	if !hasKey(kv, LoginFailKeyPrefix+"2.2.2.2") {
		t.Fatal("boundary row (expires==now) must survive (After is strict >, matches Allow)")
	}

	// 活行仍触发限流（NF-S：清理后限流不失效）。
	if allowed, _ := rl.Allow("1.1.1.1"); allowed {
		t.Fatal("live blocked IP must still be blocked after purge")
	}
	// 边界行也仍被视为 blocked（now 未越过 expires）。
	if allowed, _ := rl.Allow("2.2.2.2"); allowed {
		t.Fatal("boundary IP at now==expires must still be blocked (consistent with Allow)")
	}
}

// TestPurgeExpired_CorruptValueRemoved 损坏 JSON 值视为过期删除（T-063 BC-5 / R-2）。
func TestPurgeExpired_CorruptValueRemoved(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return now }

	// 损坏值（非合法 failRecord JSON）。
	if err := kv.KVSet(context.Background(), LoginFailKeyPrefix+"5.5.5.5", "{not-json"); err != nil {
		t.Fatalf("seed corrupt: %v", err)
	}
	// 一条活行确保不被连累。
	seedFail(t, kv, "1.1.1.1", failMax, now.Add(-10*time.Second))

	purged, err := rl.PurgeExpired(context.Background())
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if purged != 1 {
		t.Fatalf("purged got %d want 1 (corrupt row)", purged)
	}
	if hasKey(kv, LoginFailKeyPrefix+"5.5.5.5") {
		t.Fatal("corrupt loginfail row should be purged as garbage")
	}
	if !hasKey(kv, LoginFailKeyPrefix+"1.1.1.1") {
		t.Fatal("live row must survive")
	}
}

// TestPurgeExpired_OnlyTouchesLoginfailPrefix 反向证伪（T-063 AC-7(c) / R-3）：
// PurgeExpired 绝不碰非 loginfail 命名空间的 KV（mode.* / 配置 / 近似键）。
func TestPurgeExpired_OnlyTouchesLoginfailPrefix(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return now }

	ctx := context.Background()
	// 全部过期的 loginfail 行（都该被删）。
	seedFail(t, kv, "9.9.9.9", failMax, now.Add(-300*time.Second))
	seedFail(t, kv, "8.8.8.8", failMax, now.Add(-300*time.Second))
	// 不可被触碰的其它命名空间 + 近似键。
	untouchable := map[string]string{
		"mode.frpc.enabled":       "true",
		"mode.frps.enabled":       "false",
		"system.autorestore.last": `{"kind":"frpc"}`,
		"loginfailure.x":          "approx-no-dot",
		"loginfail":               "no-dot",
		"xloginfail.y":            "prefixed",
	}
	for k, v := range untouchable {
		if err := kv.KVSet(ctx, k, v); err != nil {
			t.Fatalf("seed %q: %v", k, err)
		}
	}

	purged, err := rl.PurgeExpired(ctx)
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if purged != 2 {
		t.Fatalf("purged got %d want 2 (only the two loginfail rows)", purged)
	}
	for k, v := range untouchable {
		got, found, _ := kv.KVGet(ctx, k)
		if !found {
			t.Fatalf("PurgeExpired deleted non-loginfail key %q", k)
		}
		if got != v {
			t.Fatalf("PurgeExpired mutated %q: got %q want %q", k, got, v)
		}
	}
}

// TestPurgeExpired_EmptyNoop 无任何 loginfail 行时返回 0、不报错（T-063 BC-1）。
func TestPurgeExpired_EmptyNoop(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	_ = kv.KVSet(context.Background(), "mode.frpc.enabled", "true")
	purged, err := rl.PurgeExpired(context.Background())
	if err != nil {
		t.Fatalf("PurgeExpired empty: %v", err)
	}
	if purged != 0 {
		t.Fatalf("purged got %d want 0 on no loginfail rows", purged)
	}
}

// --- QA 独立对抗（T-063；与 dev 测试不共享 fixture，从 AC 出发反向证伪）---

// TestQA_PurgeExpired_SubThresholdExpiredAlsoPurged 反向证伪假设："过期判定只看时间窗口，
// 与 count 是否达上限无关"。dev 测试只 seed count=failMax(5) 的行；若 PurgeExpired 误把
// 过期判定与 count>=failMax 耦合（如照搬 Allow 里 `rec.Count < failMax` 早返的结构），
// 则 count<failMax 的过期行会被漏清 → 永久滞留（正是本任务要消除的泄漏）。
// 我预期此测试通过（过期与 count 解耦），若失败则暴露漏清缺陷。
func TestQA_PurgeExpired_SubThresholdExpiredAlsoPurged(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	now := time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return now }

	// count=1（远低于 failMax=5）但窗口已过期（firstAt = now - 5min）。
	seedFail(t, kv, "3.3.3.3", 1, now.Add(-5*time.Minute))
	// count=2 也过期。
	seedFail(t, kv, "4.4.4.4", 2, now.Add(-90*time.Second))

	purged, err := rl.PurgeExpired(context.Background())
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if purged != 2 {
		t.Fatalf("sub-threshold expired rows must be purged regardless of count; purged=%d want 2", purged)
	}
	if hasKey(kv, LoginFailKeyPrefix+"3.3.3.3") || hasKey(kv, LoginFailKeyPrefix+"4.4.4.4") {
		t.Fatal("expired rows with count<failMax leaked (purge wrongly gated on count)")
	}
}

// TestQA_PurgeExpired_DoesNotCorruptSubsequentRateLimit 反向证伪假设："清理一个 IP 的过期
// 计数行后，该 IP 的后续 RecordFailure 从全新窗口 count=1 开始，限流语义未被破坏"。
// 这验证 purge 与活限流路径的交互——清理不该让某 IP 永久免疫或永久封禁。
// 我预期通过；若失败则说明 purge 留下了脏状态污染后续计数。
func TestQA_PurgeExpired_DoesNotCorruptSubsequentRateLimit(t *testing.T) {
	kv := newFakeKV()
	rl := NewRateLimiter(kv)
	base := time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return base }

	ip := "7.7.7.7"
	// 旧的过期满计数行。
	seedFail(t, kv, ip, failMax, base.Add(-10*time.Minute))

	purged, err := rl.PurgeExpired(context.Background())
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if purged != 1 {
		t.Fatalf("expired full-count row should be purged; purged=%d want 1", purged)
	}

	// 清理后该 IP 立刻可再尝试（不应残留封禁）。
	if allowed, _ := rl.Allow(ip); !allowed {
		t.Fatal("after purging expired row, IP must be allowed (no stale ban)")
	}
	// 重新累计：第 1 次失败应为全新窗口 count=1，不带旧计数。
	count, retry, err := rl.RecordFailure(ip)
	if err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	if count != 1 {
		t.Fatalf("post-purge RecordFailure must start fresh window count=1, got %d (stale state leaked)", count)
	}
	if retry != 0 {
		t.Fatalf("count=1 must not set retry-after, got %v", retry)
	}
}
