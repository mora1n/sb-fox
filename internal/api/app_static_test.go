package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestAppDisplayNameDrivesIndexTitleAndManifest(t *testing.T) {
	srv, ts := testServer(t)
	rrDefault := httptest.NewRecorder()
	srv.serveIndex(rrDefault, fstest.MapFS{
		"index.html": {Data: []byte(`<!doctype html><html><head><title>Loading...</title></head><body></body></html>`)},
	})
	defaultBody := rrDefault.Body.String()
	if !strings.Contains(defaultBody, "<title>App</title>") || strings.Contains(defaultBody, "sb-fox") {
		t.Fatalf("default index title exposed project name:\n%s", defaultBody)
	}

	if err := srv.Store.SetSetting(settingAppDisplayName, "Fox Panel"); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv.serveIndex(rr, fstest.MapFS{
		"index.html": {Data: []byte(`<!doctype html><html><head><title>Loading...</title></head><body></body></html>`)},
	})
	body := rr.Body.String()
	if !strings.Contains(body, "<title>Fox Panel</title>") || strings.Contains(body, "<title>Loading...</title>") {
		t.Fatalf("index title was not injected:\n%s", body)
	}
	if strings.Contains(body, "sb-fox") {
		t.Fatalf("index exposed default project name:\n%s", body)
	}

	c := newClient(t, ts.URL)
	resp := c.do(http.MethodGet, "/site.webmanifest", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manifest status = %d", resp.StatusCode)
	}
	var manifest struct {
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.Name != "Fox Panel" || manifest.ShortName != "Fox Panel" {
		t.Fatalf("manifest name = %+v", manifest)
	}
}

func TestSPAAssetCacheHeaders(t *testing.T) {
	srv, _ := testServer(t)
	handler := srv.spaHandler(fstest.MapFS{
		"index.html":        {Data: []byte(`<!doctype html><title>Loading...</title>`)},
		"assets/index.js":   {Data: []byte(`console.log("ok")`)},
		"favicon-32x32.png": {Data: []byte("png")},
	})

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/index.js", nil))
	if got := asset.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("asset cache-control = %q", got)
	}

	index := httptest.NewRecorder()
	handler.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := index.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("index cache-control = %q", got)
	}
}
