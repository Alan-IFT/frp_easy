package frpconf

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

func TestRenderFrpc_RoundTrip(t *testing.T) {
	rp := 6000
	in := FrpcRenderInput{
		ServerAddr: "fr.example.com",
		ServerPort: 7000,
		AuthMethod: "token",
		AuthToken:  "s3cr3t-token",
		AdminAddr:  "127.0.0.1",
		AdminPort:  7400,
		AdminUser:  "admin",
		AdminPass:  "pw",
		LogPath:    "/tmp/frpc.log",
		LogLevel:   "info",
		LogMaxDays: 7,
		Proxies: []ProxyInput{
			{Name: "ssh", Type: "tcp", LocalIP: "127.0.0.1", LocalPort: 22, RemotePort: &rp},
			{Name: "web", Type: "http", LocalPort: 80, CustomDomains: []string{"www.example.com"}},
		},
	}
	data, err := RenderFrpc(in)
	if err != nil {
		t.Fatalf("RenderFrpc: %v", err)
	}
	// 反序列化验证关键字段
	var got map[string]any
	if err := toml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v\n--- data:\n%s", err, data)
	}
	if got["serverAddr"] != "fr.example.com" {
		t.Errorf("serverAddr: %v", got["serverAddr"])
	}
	if int(got["serverPort"].(int64)) != 7000 {
		t.Errorf("serverPort: %v", got["serverPort"])
	}
	auth := got["auth"].(map[string]any)
	if auth["method"] != "token" || auth["token"] != "s3cr3t-token" {
		t.Errorf("auth: %v", auth)
	}
	ws := got["webServer"].(map[string]any)
	if ws["addr"] != "127.0.0.1" || int(ws["port"].(int64)) != 7400 {
		t.Errorf("webServer: %v", ws)
	}
	// proxies
	proxies, ok := got["proxies"].([]any)
	if !ok || len(proxies) != 2 {
		t.Fatalf("proxies: %T %v", got["proxies"], got["proxies"])
	}
	p0 := proxies[0].(map[string]any)
	if p0["name"] != "ssh" || p0["type"] != "tcp" {
		t.Errorf("proxy[0]: %v", p0)
	}
	if int(p0["remotePort"].(int64)) != 6000 {
		t.Errorf("remotePort: %v", p0["remotePort"])
	}
	if _, has := p0["customDomains"]; has {
		t.Errorf("tcp proxy must not have customDomains")
	}
	p1 := proxies[1].(map[string]any)
	if cd, ok := p1["customDomains"].([]any); !ok || len(cd) != 1 || cd[0] != "www.example.com" {
		t.Errorf("customDomains: %v", p1["customDomains"])
	}
}

func TestRenderFrpc_OmitAuthWhenTokenEmpty(t *testing.T) {
	in := FrpcRenderInput{
		ServerAddr: "x.example.com",
		ServerPort: 7000,
		// AuthToken 留空
	}
	data, err := RenderFrpc(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "[auth]") || strings.Contains(s, "auth.method") || strings.Contains(s, "auth.token") {
		t.Errorf("auth section should be omitted, got:\n%s", s)
	}
}

func TestRenderFrpc_RejectInvalidType(t *testing.T) {
	in := FrpcRenderInput{
		ServerAddr: "x", ServerPort: 7000,
		Proxies: []ProxyInput{
			{Name: "bad", Type: "xtcp", LocalPort: 22},
		},
	}
	if _, err := RenderFrpc(in); err == nil {
		t.Fatal("expected error for unsupported proxy type")
	}
}

func TestRenderFrpc_RejectTCPWithoutRemotePort(t *testing.T) {
	in := FrpcRenderInput{
		ServerAddr: "x", ServerPort: 7000,
		Proxies: []ProxyInput{
			{Name: "ssh", Type: "tcp", LocalPort: 22}, // 缺 RemotePort
		},
	}
	if _, err := RenderFrpc(in); err == nil {
		t.Fatal("expected error for tcp without remotePort")
	}
}

func TestRenderFrpc_RejectHTTPWithoutDomains(t *testing.T) {
	in := FrpcRenderInput{
		ServerAddr: "x", ServerPort: 7000,
		Proxies: []ProxyInput{
			{Name: "web", Type: "http", LocalPort: 80},
		},
	}
	if _, err := RenderFrpc(in); err == nil {
		t.Fatal("expected error for http without customDomains")
	}
}

func TestRenderFrps_Minimal(t *testing.T) {
	in := FrpsRenderInput{BindPort: 7000}
	data, err := RenderFrps(in)
	if err != nil {
		t.Fatalf("RenderFrps: %v", err)
	}
	var got map[string]any
	if err := toml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if int(got["bindPort"].(int64)) != 7000 {
		t.Errorf("bindPort: %v", got["bindPort"])
	}
	if _, has := got["auth"]; has {
		t.Errorf("auth should be omitted")
	}
	if _, has := got["webServer"]; has {
		t.Errorf("webServer should be omitted when dashboard disabled")
	}
}

func TestRenderFrps_WithDashboard(t *testing.T) {
	in := FrpsRenderInput{
		BindPort:         7000,
		AuthMethod:       "token",
		AuthToken:        "tk",
		DashboardEnabled: true,
		DashboardPort:    7500,
		DashboardUser:    "admin",
		DashboardPass:    "pw",
	}
	data, err := RenderFrps(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "bindPort = 7000") {
		t.Errorf("missing bindPort: %s", s)
	}
	if !strings.Contains(s, "[auth]") {
		t.Errorf("missing auth: %s", s)
	}
	if !strings.Contains(s, "[webServer]") {
		t.Errorf("missing webServer: %s", s)
	}
}

func TestAtomicWrite_ReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frpc.toml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(path, []byte("new content here")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new content here" {
		t.Errorf("content: %s", got)
	}
	// 临时文件应被清理（同目录无 .frpconf-*.tmp）
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".frpconf-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestAtomicWrite_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime", "deep", "frpc.toml")
	if err := AtomicWrite(path, []byte("x")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file missing: %v", err)
	}
}

// TestAtomicWritePerm0600 验证 AC-1.1：AtomicWrite 后目标文件权限位 = 0o600。
// Windows 上 os.Chmod 仅控制 ReadOnly attr，权限位语义与 POSIX 不同，跳过断言。
func TestAtomicWritePerm0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("权限位语义在 Windows 不同；本测试仅覆盖 POSIX 平台")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "frpc.toml")
	if err := AtomicWrite(path, []byte("token=secret")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("perm = %o, want 0o600", perm)
	}
	// 同时确认 group/other 位完全为 0。
	if perm&0o077 != 0 {
		t.Errorf("group/other bits set: %o", perm&0o077)
	}
}

// TestAtomicWritePerm0600_OverwriteExisting 验证 AC-1.2：目标文件已存在且权限
// 宽松（如 0o644）时，AtomicWrite 后必须收紧到 0o600，不允许保留旧权限。
func TestAtomicWritePerm0600_OverwriteExisting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("权限位语义在 Windows 不同；本测试仅覆盖 POSIX 平台")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "frpc.toml")
	// 先写一个宽松权限的旧文件
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(path, []byte("new")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm after overwrite = %o, want 0o600", perm)
	}
}

// TestAtomicWrite_NoTempLeakOnSuccess 回归 AC-1.3：原有"不残留 .frpconf-*.tmp"
// 行为不退化（与 TestAtomicWrite_ReplacesExisting 中的同名断言冗余但显式，
// 保证 chmod 改动不引入新的 cleanup 漏洞）。
func TestAtomicWrite_NoTempLeakOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frpc.toml")
	if err := AtomicWrite(path, []byte("x")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".frpconf-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}
