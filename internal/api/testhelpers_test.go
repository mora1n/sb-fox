package api

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"testing"
)

func newCookieJar(t *testing.T) http.CookieJar {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return jar
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func writeFileImpl(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
