package frpcadmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReload_Success(t *testing.T) {
	var seenStrict string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/reload" {
			t.Errorf("path = %s", r.URL.Path)
		}
		seenStrict = r.URL.Query().Get("strictConfig")
		u, p, ok := r.BasicAuth()
		if !ok || u != "admin" || p != "pw" {
			t.Errorf("auth: %s/%s ok=%v", u, p, ok)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewWithBaseURL(srv.URL, "admin", "pw", 2*time.Second)
	if err := c.Reload(context.Background(), true); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if seenStrict != "true" {
		t.Errorf("strictConfig param missing: %s", seenStrict)
	}
}

func TestReload_NonStrictOmitsParam(t *testing.T) {
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	if err := c.Reload(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if rawQuery != "" {
		t.Errorf("expected no query, got %q", rawQuery)
	}
}

func TestReload_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("strict config failed"))
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	err := c.Reload(context.Background(), true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "strict") {
		t.Errorf("err lacks detail: %v", err)
	}
}

func TestStatus_GroupedByType(t *testing.T) {
	payload := map[string][]ProxyStatus{
		"tcp": {
			{Name: "ssh", Type: "tcp", Status: "running", LocalAddr: "127.0.0.1:22", RemoteAddr: ":6000"},
		},
		"http": {
			{Name: "web", Type: "http", Status: "running"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			t.Errorf("path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	got, err := c.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got["tcp"]) != 1 || got["tcp"][0].Name != "ssh" {
		t.Errorf("tcp group: %+v", got["tcp"])
	}
	if len(got["http"]) != 1 {
		t.Errorf("http group: %+v", got["http"])
	}
}

func TestStatus_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	c := NewWithBaseURL(srv.URL, "u", "p", time.Second)
	if _, err := c.Status(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_BuildsBaseURL(t *testing.T) {
	c := New("127.0.0.1", 7400, "admin", "pw")
	if c.baseURL != "http://127.0.0.1:7400" {
		t.Errorf("baseURL: %s", c.baseURL)
	}
}
