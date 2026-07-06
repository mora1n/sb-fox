package subfetch

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestValidateURL(t *testing.T) {
	bad := []string{"file:///etc/passwd", "ftp://x/y", "gopher://x", "not a url", "http://"}
	for _, u := range bad {
		if err := validateURL(u); err == nil {
			t.Errorf("validateURL(%q) = nil, want error", u)
		}
	}
	good := []string{"http://example.com/sub", "https://example.com:8443/x"}
	for _, u := range good {
		if err := validateURL(u); err != nil {
			t.Errorf("validateURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestBlockedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "::1", "10.0.0.1", "192.168.1.1", "172.16.0.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "fe80::1"}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("isBlockedIP(%s) = false, want true", s)
		}
	}
	allowed := []string{"1.1.1.1", "8.8.8.8", "93.184.216.34"}
	for _, s := range allowed {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("isBlockedIP(%s) = true, want false", s)
		}
	}
}

func TestDecodeBody(t *testing.T) {
	raw := "ss://abc@1.2.3.4:8388#node\nvmess://xyz"
	if got := DecodeBody(raw); got != raw {
		t.Errorf("raw body altered: %q", got)
	}
	// base64 of raw links should decode
	encoded := "c3M6Ly9hYmNAMS4yLjMuNDo4Mzg4I25vZGUK" // base64("ss://abc@1.2.3.4:8388#node\n")
	got := DecodeBody(encoded)
	if got == encoded {
		t.Errorf("expected base64 to decode, got original")
	}
}

// TestFetchGuardsLoopback verifies the SSRF guard refuses a localhost target.
func TestFetchGuardsLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ss://secret"))
	}))
	defer srv.Close()

	f := New()
	_, err := f.Fetch(context.Background(), srv.URL) // srv.URL is 127.0.0.1
	if err == nil {
		t.Fatal("expected SSRF guard to block loopback fetch")
	}
}

// TestFetchAllowPrivate confirms the guard can be bypassed intentionally.
func TestFetchAllowPrivate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ss://secret@1.2.3.4:8388#n"))
	}))
	defer srv.Close()

	f := New()
	f.AllowPrivate = true
	body, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("AllowPrivate fetch failed: %v", err)
	}
	if body == "" {
		t.Error("empty body")
	}
}

func TestFetchManyUsesOptionsAndShortTTLCache(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if got := r.UserAgent(); got != "CustomUA" {
			t.Errorf("User-Agent = %q, want CustomUA", got)
		}
		if got := r.Header.Get("X-Test"); got != "yes" {
			t.Errorf("X-Test = %q, want yes", got)
		}
		w.Write([]byte("ss://secret@1.2.3.4:8388#n"))
	}))
	defer srv.Close()

	f := New()
	f.AllowPrivate = true
	headers := url.QueryEscape(`{"X-Test":"yes"}`)
	rawURL := srv.URL + "#ua=CustomUA&headers=" + headers + "&cacheTtl=60"
	first, err := f.FetchMany(context.Background(), rawURL, Options{})
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	second, err := f.FetchMany(context.Background(), rawURL, Options{})
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("calls = %d, want cached second request", calls)
	}
	if !second.Items[0].FromCache {
		t.Fatalf("second item did not report cache: %+v", second.Items)
	}
	if first.Bodies[0] != second.Bodies[0] {
		t.Fatalf("cached body mismatch")
	}

	_, err = f.FetchMany(context.Background(), srv.URL+"#ua=CustomUA&headers="+headers+"&noCache", Options{})
	if err != nil {
		t.Fatalf("noCache fetch: %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("calls = %d, want noCache to bypass cache", calls)
	}
}

func TestFetchManyPartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "bad") {
			http.Error(w, "bad", http.StatusBadGateway)
			return
		}
		w.Write([]byte("ss://secret@1.2.3.4:8388#ok"))
	}))
	defer srv.Close()

	f := New()
	f.AllowPrivate = true
	result, err := f.FetchMany(context.Background(), srv.URL+"/bad\n"+srv.URL+"/ok", Options{})
	if err != nil {
		t.Fatalf("partial fetch should succeed: %v", err)
	}
	if len(result.Bodies) != 1 || len(result.Items) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if result.Items[0].OK || result.Items[0].Error == "" || !result.Items[1].OK {
		t.Fatalf("items = %+v", result.Items)
	}
}

func TestSafeURLMasksSensitiveParts(t *testing.T) {
	got := SafeURL("https://user:pass@example.com/sub?token=secret#headers=%7B%7D")
	if strings.Contains(got, "user") || strings.Contains(got, "pass") ||
		strings.Contains(got, "secret") || strings.Contains(got, "headers") {
		t.Fatalf("SafeURL leaked sensitive data: %s", got)
	}
	if got != "https://example.com/sub?..." {
		t.Fatalf("SafeURL = %q", got)
	}
}
