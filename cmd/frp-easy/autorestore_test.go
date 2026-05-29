package main

// T-050 A-3：autoRestoreProcs + retryRestoreLoop 的自动化测试（T-038 "开机即用"保证，
// 此前仅人肉真机验证）。
//
// 覆盖目标（注入短 backoff，绝不用固定 time.Sleep 当同步点，统一 poll-until-condition + deadline）：
//   - first-fail → 启动 retry goroutine（first attempt 同步失败被记录在 attempts[0]）。
//   - retry 全失败 → outcome=="exhausted"，attempts 数 == 1(first) + len(retryBackoff)。
//   - ctx 中途取消 → outcome=="canceled"，goroutine 及时退出（无泄漏）。
//   - binary missing → outcome=="binary-missing"，不进 retry（永久失败）。
//   - config missing → outcome=="config-missing"，不进 retry。
//   - 每轮 outcome 都写入 kv system.autorestore.<kind>（可被 GET service-status 读到）。
//
// 反向证伪点：
//   - 若 first-fail 不触发 retry，exhausted 用例的 attempts 数会停在 1 而非 1+N。
//   - 若 ctx 取消不被 retry 循环感知，canceled 用例会超时（视为 goroutine 泄漏）。
//   - 若 binary/config missing 误进 retry，对应用例 outcome 会是 exhausted 而非 *-missing。

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/procmgr"
	"github.com/frp-easy/frp-easy/internal/storage"
)

// arFakeLocator：可控的 binloc.Locator 替身。
//   - missing 非空 → Missing() 返回该列表（触发 autoRestoreProcs 的 binary-missing 分支）。
//   - 否则 FRPCPath/FRPSPath 返回一个"存在但不可执行"的路径，让 procmgr.Start 的
//     cmd.Start() 失败（exec 报错），从而稳定触发 retry，而无需真 spawn frpc/frps。
type arFakeLocator struct {
	frpcPath string
	frpsPath string
	missing  []string
}

func (f *arFakeLocator) FRPCPath() (string, error) {
	if f.frpcPath == "" {
		return "", errors.New("no frpc")
	}
	return f.frpcPath, nil
}
func (f *arFakeLocator) FRPSPath() (string, error) {
	if f.frpsPath == "" {
		return "", errors.New("no frps")
	}
	return f.frpsPath, nil
}
func (f *arFakeLocator) Missing() []string { return f.missing }
func (f *arFakeLocator) Root() string       { return "" }

var _ binloc.Locator = (*arFakeLocator)(nil)
var _ procmgr.Locator = (*arFakeLocator)(nil)

// arSetBackoff 临时把包级 retryBackoff 换成短序列，测试结束自动还原。
func arSetBackoff(t *testing.T, seq []time.Duration) {
	t.Helper()
	orig := retryBackoff
	retryBackoff = seq
	t.Cleanup(func() { retryBackoff = orig })
}

// arOpenStore 建一个隔离的 store（t.TempDir），并设 mode.<kind>.enabled=true。
func arOpenStore(t *testing.T, enabledKinds ...string) *storage.Store {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	ctx := context.Background()
	for _, k := range enabledKinds {
		if err := store.KVSet(ctx, "mode."+k+".enabled", "true"); err != nil {
			t.Fatalf("set mode.%s.enabled: %v", k, err)
		}
	}
	return store
}

// arNonExecBinary 在 tempdir 写一个普通文本文件冒充二进制；cmd.Start 执行它会失败。
func arNonExecBinary(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-frpc")
	if err := os.WriteFile(p, []byte("not an executable\n"), 0o644); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return p
}

// arTouchConfig 在 dir 下写一个空 toml，让 autoRestoreProcs 的 config-missing 预检通过。
func arTouchConfig(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// arPollLastRun 轮询 kv system.autorestore.<kind> 直到出现且 outcome ∈ want，或 deadline。
// poll-until-condition：绝不用固定 sleep 当同步点。
func arPollLastRun(t *testing.T, store *storage.Store, kind string, deadline time.Duration, want ...string) AutoRestoreLastRun {
	t.Helper()
	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
	}
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		v, ok, err := store.KVGet(ctx, "system.autorestore."+kind)
		cancel()
		if err == nil && ok && v != "" {
			var run AutoRestoreLastRun
			if json.Unmarshal([]byte(v), &run) == nil {
				if len(wantSet) == 0 || wantSet[run.Outcome] {
					return run
				}
			}
		}
		time.Sleep(2 * time.Millisecond) // 仅轮询间隔，非同步点：条件满足即提前返回。
	}
	t.Fatalf("autorestore.%s 未在 %v 内出现期望 outcome %v", kind, deadline, want)
	return AutoRestoreLastRun{}
}

// TestAutoRestore_BinaryMissing：二进制缺失 → outcome=binary-missing，不 retry。
func TestAutoRestore_BinaryMissing(t *testing.T) {
	arSetBackoff(t, []time.Duration{time.Millisecond})
	store := arOpenStore(t, "frpc")
	loc := &arFakeLocator{missing: []string{"frpc"}}
	pm := procmgr.New(procmgr.Config{
		Locator:     loc,
		ConfigPaths: map[string]string{"frpc": filepath.Join(t.TempDir(), "frpc.toml")},
	})
	ctx := context.Background()
	autoRestoreProcs(ctx, store, pm, loc, discardLogger(), map[string]string{"frpc": filepath.Join(t.TempDir(), "frpc.toml")})

	run := arPollLastRun(t, store, "frpc", 2*time.Second, "binary-missing")
	if run.Kind != "frpc" {
		t.Errorf("kind=%q want frpc", run.Kind)
	}
	if len(run.Attempts) != 1 || run.Attempts[0].OK {
		t.Errorf("binary-missing 应记 1 条失败 attempt，got %+v", run.Attempts)
	}
}

// TestAutoRestore_ConfigMissing：配置文件缺失 → outcome=config-missing，不 retry。
func TestAutoRestore_ConfigMissing(t *testing.T) {
	arSetBackoff(t, []time.Duration{time.Millisecond})
	store := arOpenStore(t, "frpc")
	bin := arNonExecBinary(t)
	loc := &arFakeLocator{frpcPath: bin} // Missing() 空 → 不走 binary-missing 分支
	missingCfg := filepath.Join(t.TempDir(), "does-not-exist", "frpc.toml")
	cfgPaths := map[string]string{"frpc": missingCfg}
	pm := procmgr.New(procmgr.Config{Locator: loc, ConfigPaths: cfgPaths})
	ctx := context.Background()
	autoRestoreProcs(ctx, store, pm, loc, discardLogger(), cfgPaths)

	run := arPollLastRun(t, store, "frpc", 2*time.Second, "config-missing")
	if len(run.Attempts) != 1 || run.Attempts[0].OK {
		t.Errorf("config-missing 应记 1 条失败 attempt，got %+v", run.Attempts)
	}
}

// TestAutoRestore_FirstFailThenExhausted：first attempt 失败（cmd.Start 失败）→ 进 retry，
// 注入 2 步短 backoff，每步仍失败 → outcome=exhausted，attempts 数 == 1 + 2。
func TestAutoRestore_FirstFailThenExhausted(t *testing.T) {
	arSetBackoff(t, []time.Duration{time.Millisecond, 2 * time.Millisecond})
	store := arOpenStore(t, "frpc")
	bin := arNonExecBinary(t)
	loc := &arFakeLocator{frpcPath: bin}
	dir := t.TempDir()
	cfg := filepath.Join(dir, "frpc.toml")
	arTouchConfig(t, cfg)
	cfgPaths := map[string]string{"frpc": cfg}
	pm := procmgr.New(procmgr.Config{
		Locator:     loc,
		ConfigPaths: cfgPaths,
		LogFiles:    map[string]string{"frpc": filepath.Join(dir, "frpc.log")},
	})
	ctx := context.Background()
	autoRestoreProcs(ctx, store, pm, loc, discardLogger(), cfgPaths)

	run := arPollLastRun(t, store, "frpc", 5*time.Second, "exhausted")
	wantAttempts := 1 + len(retryBackoff) // first(index0) + 每步 retry
	if len(run.Attempts) != wantAttempts {
		t.Errorf("exhausted attempts=%d want %d: %+v", len(run.Attempts), wantAttempts, run.Attempts)
	}
	for i, a := range run.Attempts {
		if a.OK {
			t.Errorf("attempt[%d] 应全失败，got OK=true", i)
		}
		if a.Index != i {
			t.Errorf("attempt[%d].Index=%d 应递增", i, a.Index)
		}
	}
}

// TestAutoRestore_CanceledMidway：retry goroutine 在 ctx 取消时短路退出，且把
// outcome=="canceled" 持久化（T-053 修复 canceled 分支用 detached ctx 后）。
//
// 用很长的 backoff（10s）确保 select 里 ctx.Done() 先于 time.After 命中 → 进 canceled 分支；
// 取消后断言：(1) 最终 KV 出现 outcome=="canceled"（T-053 修复前因用已取消 ctx 持久化而
// 永远落不进，是 F-canceled bug）；(2) 绝不出现 exhausted/ok（取消真的短路了循环）。
func TestAutoRestore_CanceledMidway(t *testing.T) {
	arSetBackoff(t, []time.Duration{10 * time.Second}) // 长 backoff：取消必先于 time.After 命中
	store := arOpenStore(t, "frpc")
	bin := arNonExecBinary(t)
	loc := &arFakeLocator{frpcPath: bin}
	dir := t.TempDir()
	cfg := filepath.Join(dir, "frpc.toml")
	arTouchConfig(t, cfg)
	cfgPaths := map[string]string{"frpc": cfg}
	pm := procmgr.New(procmgr.Config{
		Locator:     loc,
		ConfigPaths: cfgPaths,
		LogFiles:    map[string]string{"frpc": filepath.Join(dir, "frpc.log")},
	})

	ctx, cancel := context.WithCancel(context.Background())
	// first attempt 同步失败后启 retry goroutine（进 10s backoff 的 select）。
	autoRestoreProcs(ctx, store, pm, loc, discardLogger(), cfgPaths)
	cancel() // 取消让 retry goroutine 从 select 的 <-ctx.Done() 短路退出 + 持久化 canceled。

	// 正向断言：canceled outcome 必须落 KV（T-053 反向证伪 —— 修复前此处会超时）。
	run := arPollLastRun(t, store, "frpc", 2*time.Second, "canceled")
	if run.Outcome != "canceled" {
		t.Fatalf("outcome=%q want canceled", run.Outcome)
	}

	// 再守门：给足窗口确认绝不被升级成 exhausted/ok（取消真的短路了循环，没继续 retry）。
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		gctx, gcancel := context.WithTimeout(context.Background(), time.Second)
		v, ok, err := store.KVGet(gctx, "system.autorestore.frpc")
		gcancel()
		if err == nil && ok && v != "" {
			var r AutoRestoreLastRun
			if json.Unmarshal([]byte(v), &r) == nil {
				if r.Outcome == "exhausted" || r.Outcome == "ok" {
					t.Fatalf("取消应短路 retry 循环，却写出了 outcome=%q", r.Outcome)
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestAutoRestore_DisabledMode：mode.*.enabled 未设/为 false → 完全不动 kv（不写 last_run）。
func TestAutoRestore_DisabledMode(t *testing.T) {
	arSetBackoff(t, []time.Duration{time.Millisecond})
	store := arOpenStore(t) // 不 enable 任何 kind
	loc := &arFakeLocator{}
	cfgPaths := map[string]string{"frpc": filepath.Join(t.TempDir(), "frpc.toml")}
	pm := procmgr.New(procmgr.Config{Locator: loc, ConfigPaths: cfgPaths})
	ctx := context.Background()
	autoRestoreProcs(ctx, store, pm, loc, discardLogger(), cfgPaths)

	// 给可能的（不应存在的）异步写一点时间，再确认 kv 仍空。
	for _, kind := range []string{"frpc", "frps"} {
		gctx, gcancel := context.WithTimeout(context.Background(), time.Second)
		_, ok, err := store.KVGet(gctx, "system.autorestore."+kind)
		gcancel()
		if err != nil {
			t.Fatalf("kvget: %v", err)
		}
		if ok {
			t.Errorf("disabled mode 不应写 autorestore.%s", kind)
		}
	}
}
