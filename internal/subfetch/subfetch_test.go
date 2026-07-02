package subfetch

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
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
