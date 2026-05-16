package assets

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandler_ServesIndexHTML は embed.FS から dist/index.html が返ること、
// および未知パスが SPA fallback で index.html を返すことを確認する。
func TestHandler_ServesIndexHTML(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	for _, path := range []string{"/", "/dashboard", "/login", "/nonexistent"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d", path, resp.StatusCode)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "<div") && !strings.Contains(string(body), "<!DOCTYPE") {
			t.Errorf("GET %s: unexpected body: %.100s", path, body)
		}
	}
}

// TestHandler_ContentType は index.html が text/html を返すことを確認する。
func TestHandler_ContentType(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html, got %q", ct)
	}
}
