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

// T-038: 渲染出的 frpc.toml 必须含 `loginFailExit = false` 字面，让 frpc 在登录
// 失败时不直接 exit 而是进入重连循环（与 autoRestoreProcs retry 形成双层防御）。
// 反向证伪：删掉 RenderFrpc 内 LoginFailExit 赋值 → 此测试失败。
func TestRenderFrpc_LoginFailExitFalse(t *testing.T) {
	in := FrpcRenderInput{
		ServerAddr: "x.example.com",
		ServerPort: 7000,
	}
	data, err := RenderFrpc(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "loginFailExit = false") {
		t.Errorf("expected 'loginFailExit = false' in output, got:\n%s", s)
	}
	// 反序列化也要验证字段语义
	var got map[string]any
	if err := toml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	v, ok := got["loginFailExit"]
	if !ok {
		t.Fatalf("loginFailExit not present in TOML")
	}
	if b, _ := v.(bool); b {
		t.Errorf("expected loginFailExit=false, got %v", v)
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

// ---------------------------------------------------------------------------
// T-040: allowPorts 端口策略渲染 + 校验测试集
// 覆盖 02 §7.1 的 11 个用例 + AC-1~6 + OQ-1/2/6
// ---------------------------------------------------------------------------

func TestRenderFrps_AllowPorts_Empty(t *testing.T) {
	// AC-2：nil 与 [] 都不渲染 [[allowPorts]] 段
	cases := []struct {
		name string
		in   FrpsRenderInput
	}{
		{"nil", FrpsRenderInput{BindPort: 7000}},
		{"empty", FrpsRenderInput{BindPort: 7000, AllowPorts: []FrpsAllowPortRange{}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := RenderFrps(c.in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			s := string(data)
			if strings.Contains(s, "[[allowPorts]]") || strings.Contains(s, "allowPorts") {
				t.Errorf("allowPorts 段不应出现，got:\n%s", s)
			}
		})
	}
}

func TestRenderFrps_AllowPorts_SingleRange(t *testing.T) {
	// AC-1：一个 range entry
	in := FrpsRenderInput{
		BindPort:   7000,
		AllowPorts: []FrpsAllowPortRange{{Start: 6000, End: 7000}},
	}
	data, err := RenderFrps(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "[[allowPorts]]") {
		t.Errorf("missing [[allowPorts]]: %s", s)
	}
	if !strings.Contains(s, "start = 6000") || !strings.Contains(s, "end = 7000") {
		t.Errorf("missing start/end: %s", s)
	}
	if strings.Contains(s, "single") {
		t.Errorf("range entry 不应含 single 字面: %s", s)
	}
}

func TestRenderFrps_AllowPorts_MultiRange(t *testing.T) {
	// OQ-6：顺序保留（两 range + 一 single）
	in := FrpsRenderInput{
		BindPort: 7000,
		AllowPorts: []FrpsAllowPortRange{
			{Start: 6000, End: 7000},
			{Single: 9000},
			{Start: 10000, End: 11000},
		},
	}
	data, err := RenderFrps(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(data)
	// 三段都应出现
	if strings.Count(s, "[[allowPorts]]") != 3 {
		t.Errorf("expected 3 [[allowPorts]] blocks, got:\n%s", s)
	}
	// 顺序检查：start=6000 应在 single=9000 之前
	idx6000 := strings.Index(s, "6000")
	idx9000 := strings.Index(s, "single = 9000")
	idx10000 := strings.Index(s, "10000")
	if idx6000 == -1 || idx9000 == -1 || idx10000 == -1 {
		t.Fatalf("missing markers in:\n%s", s)
	}
	if !(idx6000 < idx9000 && idx9000 < idx10000) {
		t.Errorf("顺序错乱：6000@%d, 9000@%d, 10000@%d", idx6000, idx9000, idx10000)
	}
}

func TestRenderFrps_AllowPorts_SingleOnly(t *testing.T) {
	// AC-1：单 single entry
	in := FrpsRenderInput{
		BindPort:   7000,
		AllowPorts: []FrpsAllowPortRange{{Single: 9000}},
	}
	data, err := RenderFrps(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "single = 9000") {
		t.Errorf("missing single=9000: %s", s)
	}
	if strings.Contains(s, "start = ") || strings.Contains(s, "end = ") {
		t.Errorf("single entry 不应含 start/end 字面: %s", s)
	}
}

func TestRenderFrps_AllowPorts_BoundaryMinMax(t *testing.T) {
	// AC-6：边界 1 / 65535 合法
	in := FrpsRenderInput{
		BindPort: 7000,
		AllowPorts: []FrpsAllowPortRange{
			{Single: 1},
			{Start: 65534, End: 65535},
		},
	}
	data, err := RenderFrps(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "single = 1") {
		t.Errorf("missing single=1: %s", s)
	}
	if !strings.Contains(s, "end = 65535") {
		t.Errorf("missing end=65535: %s", s)
	}
}

func TestRenderFrps_AllowPorts_StartGreaterThanEnd(t *testing.T) {
	// AC-3
	in := FrpsRenderInput{
		BindPort:   7000,
		AllowPorts: []FrpsAllowPortRange{{Start: 80, End: 70}},
	}
	_, err := RenderFrps(in)
	if err == nil {
		t.Fatal("expected error for start > end")
	}
	if !strings.Contains(err.Error(), "start=80") || !strings.Contains(err.Error(), "end=70") {
		t.Errorf("错误文案缺定位: %v", err)
	}
}

func TestRenderFrps_AllowPorts_Mutex(t *testing.T) {
	// AC-4：Single + Start/End 同设
	cases := []struct {
		name string
		in   FrpsAllowPortRange
	}{
		{"single_and_start", FrpsAllowPortRange{Single: 80, Start: 100}},
		{"single_and_end", FrpsAllowPortRange{Single: 80, End: 100}},
		{"single_and_both", FrpsAllowPortRange{Single: 80, Start: 100, End: 200}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := RenderFrps(FrpsRenderInput{BindPort: 7000, AllowPorts: []FrpsAllowPortRange{c.in}})
			if err == nil {
				t.Fatalf("expected error for %+v", c.in)
			}
			if !strings.Contains(err.Error(), "互斥") {
				t.Errorf("错误文案应含 '互斥': %v", err)
			}
		})
	}
}

func TestRenderFrps_AllowPorts_Overlap(t *testing.T) {
	// AC-5：两 range 重叠
	in := FrpsRenderInput{
		BindPort: 7000,
		AllowPorts: []FrpsAllowPortRange{
			{Start: 1000, End: 2000},
			{Start: 1500, End: 2500},
		},
	}
	_, err := RenderFrps(in)
	if err == nil {
		t.Fatal("expected error for overlapping ranges")
	}
	if !strings.Contains(err.Error(), "重叠") {
		t.Errorf("错误文案应含 '重叠': %v", err)
	}
}

func TestRenderFrps_AllowPorts_OverlapBoundary(t *testing.T) {
	// OQ-2：闭区间边界重叠（2000 同属两段）
	in := FrpsRenderInput{
		BindPort: 7000,
		AllowPorts: []FrpsAllowPortRange{
			{Start: 1000, End: 2000},
			{Start: 2000, End: 3000},
		},
	}
	_, err := RenderFrps(in)
	if err == nil {
		t.Fatal("expected error for boundary-touching ranges (闭区间语义)")
	}
}

func TestRenderFrps_AllowPorts_OverlapSingleVsRange(t *testing.T) {
	// 补充覆盖：单端口与范围重叠
	in := FrpsRenderInput{
		BindPort: 7000,
		AllowPorts: []FrpsAllowPortRange{
			{Start: 1000, End: 2000},
			{Single: 1500},
		},
	}
	_, err := RenderFrps(in)
	if err == nil {
		t.Fatal("expected error for single-in-range overlap")
	}
}

func TestRenderFrps_AllowPorts_OutOfRange(t *testing.T) {
	// AC-6 反向：0 / 65536 各种姿势
	cases := []struct {
		name string
		in   FrpsAllowPortRange
	}{
		{"single_zero", FrpsAllowPortRange{Single: 0, Start: 0, End: 0}}, // empty entry
		{"single_neg_via_overflow", FrpsAllowPortRange{Start: 0, End: 100}},
		{"end_too_big", FrpsAllowPortRange{Start: 100, End: 65536}},
		{"start_too_big", FrpsAllowPortRange{Start: 65536, End: 65537}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := RenderFrps(FrpsRenderInput{BindPort: 7000, AllowPorts: []FrpsAllowPortRange{c.in}})
			if err == nil {
				t.Fatalf("expected error for %+v", c.in)
			}
		})
	}
}

func TestRenderFrps_AllowPorts_TooMany(t *testing.T) {
	// OQ-1：上限 100
	list := make([]FrpsAllowPortRange, 101)
	for i := range list {
		// 用 single 且互不重叠
		list[i] = FrpsAllowPortRange{Single: 1000 + i}
	}
	_, err := RenderFrps(FrpsRenderInput{BindPort: 7000, AllowPorts: list})
	if err == nil {
		t.Fatal("expected error for >100 entries")
	}
	if !strings.Contains(err.Error(), "100") {
		t.Errorf("错误文案应含上限数字: %v", err)
	}

	// 边界：100 条正好通过
	list = list[:100]
	if _, err := RenderFrps(FrpsRenderInput{BindPort: 7000, AllowPorts: list}); err != nil {
		t.Fatalf("100 条应通过: %v", err)
	}
}

func TestRenderFrps_AllowPorts_TOMLRoundTrip(t *testing.T) {
	// 反序列化验证 frp 上游 schema 字面
	in := FrpsRenderInput{
		BindPort: 7000,
		AllowPorts: []FrpsAllowPortRange{
			{Start: 6000, End: 7000},
			{Single: 9000},
		},
	}
	data, err := RenderFrps(in)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := toml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, data)
	}
	arr, ok := got["allowPorts"].([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("allowPorts: %T len=%d", got["allowPorts"], len(arr))
	}
	first := arr[0].(map[string]any)
	if int(first["start"].(int64)) != 6000 || int(first["end"].(int64)) != 7000 {
		t.Errorf("first entry: %v", first)
	}
	if _, has := first["single"]; has {
		t.Errorf("range entry 不应含 single key: %v", first)
	}
	second := arr[1].(map[string]any)
	if int(second["single"].(int64)) != 9000 {
		t.Errorf("second entry: %v", second)
	}
	if _, has := second["start"]; has {
		t.Errorf("single entry 不应含 start key: %v", second)
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
