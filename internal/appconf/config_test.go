package appconf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DefaultsWrittenOnMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frp_easy.toml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UIBindAddr != "127.0.0.1" || cfg.UIPort != 8080 {
		t.Fatalf("unexpected default: %+v", cfg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default file not written: %v", err)
	}
}

func TestLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frp_easy.toml")
	content := `UIBindAddr = "0.0.0.0"
UIPort = 9090
DataDir = "/tmp/data"
LogDir = "/tmp/logs"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UIBindAddr != "0.0.0.0" || cfg.UIPort != 9090 {
		t.Fatalf("got %+v", cfg)
	}
	if cfg.DataDir != "/tmp/data" || cfg.LogDir != "/tmp/logs" {
		t.Fatalf("got %+v", cfg)
	}
}

func TestLoad_InvalidTOMLDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frp_easy.toml")
	orig := "this is = = not toml ====\nbroken"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error on broken toml")
	}
	got, _ := os.ReadFile(path)
	if string(got) != orig {
		t.Fatalf("original file got overwritten: %s", got)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  AppConfig
		ok   bool
	}{
		{"default", AppConfig{UIBindAddr: "127.0.0.1", UIPort: 8080, DataDir: "x", LogDir: "y"}, true},
		{"port too low", AppConfig{UIBindAddr: "127.0.0.1", UIPort: 0, DataDir: "x", LogDir: "y"}, false},
		{"port too high", AppConfig{UIBindAddr: "127.0.0.1", UIPort: 70000, DataDir: "x", LogDir: "y"}, false},
		{"empty bind", AppConfig{UIBindAddr: "", UIPort: 8080, DataDir: "x", LogDir: "y"}, false},
		{"bind with port", AppConfig{UIBindAddr: "127.0.0.1:8080", UIPort: 8080, DataDir: "x", LogDir: "y"}, false},
		{"empty datadir", AppConfig{UIBindAddr: "127.0.0.1", UIPort: 8080, DataDir: "", LogDir: "y"}, false},
	}
	for _, c := range cases {
		err := c.cfg.Validate()
		if c.ok && err != nil {
			t.Errorf("%s: expected ok, got %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}
}

func TestListenAddr(t *testing.T) {
	c := &AppConfig{UIBindAddr: "127.0.0.1", UIPort: 8080}
	if a := c.ListenAddr(); a != "127.0.0.1:8080" {
		t.Fatalf("got %s", a)
	}
	c2 := &AppConfig{UIBindAddr: "::1", UIPort: 8080}
	a := c2.ListenAddr()
	if !strings.Contains(a, "8080") {
		t.Fatalf("got %s", a)
	}
}

func TestDoc_PortTablePresent(t *testing.T) {
	// 【I-2 守护】config.go 顶部 doc-comment 必须含"内部占用端口表"四个关键端口数字。
	data, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, must := range []string{"8080", "7400", "7500", "7000"} {
		if !strings.Contains(s, must) {
			t.Errorf("config.go doc-comment missing port %s in 内部占用端口表", must)
		}
	}
}
