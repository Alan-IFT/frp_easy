package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// AC-6 对抗：构造 sqlite 文本未来变成 "UNIQUE constraint failed:proxies.name" (少空格)
// 或者 "unique constraint failed: proxies.name" (小写) 时，isDuplicateNameError 是否漏识别？
// 当前实现：strings.Contains(s, "UNIQUE constraint failed") && strings.Contains(s, "proxies.name")
// 区分大小写敏感 + 严格空格。下列文本变体应能识别 / 不应识别：
func TestAdversarial_AC6_ErrorTextVariants(t *testing.T) {
	cases := []struct {
		name    string
		errText string
		want    bool
		note    string
	}{
		{"normal", "UNIQUE constraint failed: proxies.name", true, "驱动当前输出格式"},
		{"with code", "constraint failed: UNIQUE constraint failed: proxies.name (2067)", true, "wrapped"},
		{"lowercase", "unique constraint failed: proxies.name", false, "大小写敏感 → 未来驱动变小写会漏！"},
		{"no space after colon", "UNIQUE constraint failed:proxies.name", true, "因为只检查包含两个 substring，无空格也命中"},
		{"different column", "UNIQUE constraint failed: proxies.type, proxies.remote_port", false, "组合 UNIQUE 不能误识别为 name"},
		{"only column", "constraint failed: proxies.name", false, "无 UNIQUE 关键字 → 不识别"},
	}
	for _, c := range cases {
		got := isDuplicateNameError(errors.New(c.errText))
		if got != c.want {
			t.Errorf("ADVERSARIAL: case %q (%s): isDuplicateNameError(%q) = %v, want %v",
				c.name, c.note, c.errText, got, c.want)
		}
	}
	// 关键发现：lowercase 不识别。如果未来 modernc.org/sqlite 改文本就漏 → 已记入 report
}

// AC-6 对抗：用真实 sqlite 实例触发 UNIQUE 冲突，确认 sentinel 真的从底层错误冒泡。
func TestAdversarial_AC6_RealDBDuplicateNameSentinel(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	rp := 8080
	rp2 := 8081
	p1 := &Proxy{Name: "alpha", Type: "tcp", LocalIP: "127.0.0.1", LocalPort: 80,
		RemotePort: &rp, Enabled: true}
	if err := st.UpsertProxy(context.Background(), p1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// 同名（不同 remotePort，绕开 (type,remote_port) UNIQUE） → 应返回 ErrDuplicateName
	p2 := &Proxy{Name: "alpha", Type: "tcp", LocalIP: "127.0.0.1", LocalPort: 80,
		RemotePort: &rp2, Enabled: true}
	err = st.UpsertProxy(context.Background(), p2)
	if !errors.Is(err, ErrDuplicateName) {
		t.Errorf("ADVERSARIAL FAIL: same name should return ErrDuplicateName, got: %v", err)
	}

	// 同 (type, remotePort)，name 不同 → 不应返回 ErrDuplicateName
	p3 := &Proxy{Name: "beta", Type: "tcp", LocalIP: "127.0.0.1", LocalPort: 81,
		RemotePort: &rp, Enabled: true}
	err = st.UpsertProxy(context.Background(), p3)
	if errors.Is(err, ErrDuplicateName) {
		t.Errorf("ADVERSARIAL FAIL: (type,remotePort) conflict should NOT be ErrDuplicateName, got: %v", err)
	}
	// 但 err 应该是非 nil
	if err == nil {
		t.Errorf("ADVERSARIAL FAIL: (type,remotePort) conflict should still error, got nil")
	}
}
