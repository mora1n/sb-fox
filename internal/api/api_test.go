package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mora1n/sb-fox/internal/auth"
	"github.com/mora1n/sb-fox/internal/kernel"
	"github.com/mora1n/sb-fox/internal/store"
	"github.com/mora1n/sb-fox/internal/subfetch"
)

// testServer builds a Server backed by a fresh temp DB with builtin templates
// seeded and a known admin, returning the server and an httptest.Server.
func testServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	entries, err := os.ReadDir(filepath.Join("..", "..", "data", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		content, err := os.ReadFile(filepath.Join("..", "..", "data", "templates", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if _, err := db.SeedUserTemplate(name, string(content), "test template"); err != nil {
			t.Fatal(err)
		}
	}
	hash, _ := auth.HashPassword("password123")
	if err := db.SetAdmin("admin", hash); err != nil {
		t.Fatal(err)
	}

	kernelPath, _ := exec.LookPath("sing-box")
	srv := &Server{
		Store:   db,
		Auth:    auth.New([]byte("test-secret")),
		Kernel:  kernel.New(kernelPath, t.TempDir(), 15*time.Second),
		Fetcher: subfetch.New(),
		DevMode: true,
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return srv, ts
}

// apiClient is a cookie-jar HTTP client for the test server.
type apiClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, base string) *apiClient {
	return &apiClient{t: t, base: base, http: &http.Client{}}
}

func (c *apiClient) do(method, path string, body any) *http.Response {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, c.base+path, &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	return resp
}

// decodeData decodes the envelope's data field into dst.
func decodeData(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	var env struct {
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Error != nil {
		t.Fatalf("api error %s: %s", env.Error.Code, env.Error.Message)
	}
	if dst != nil {
		if err := json.Unmarshal(env.Data, dst); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
	}
}

func TestAuthGuard(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	// unauthenticated request to a guarded endpoint → 401
	resp := c.do(http.MethodGet, "/api/nodes", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestFullFlow exercises login → import links → create profile → public sub →
// kernel validate, the primary end-to-end path.
func TestFullFlow(t *testing.T) {
	kernelPath, err := exec.LookPath("sing-box")
	if err != nil {
		t.Skip("sing-box not in PATH")
	}
	_, ts := testServer(t)
	c := newClient(t, ts.URL)

	// login (cookie jar retains the session)
	jar := &http.Client{}
	c.http = jar
	loginResp := c.do(http.MethodPost, "/api/auth/login", map[string]string{"username": "admin", "password": "password123"})
	decodeData(t, loginResp, nil)
	// capture the session cookie for subsequent requests
	setCookies := loginResp.Cookies()
	if len(setCookies) == 0 {
		t.Fatal("no session cookie set")
	}
	c.http = &http.Client{}
	c.http.Jar = mustJar(t, ts.URL, setCookies)

	// import share links (HK ss + JP vmess + a #CN override)
	ssHK := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw")) + "@hk.example.com:8388#🇭🇰 HK-1"
	vmessJSON, _ := json.Marshal(map[string]any{
		"ps": "🇯🇵 JP-1", "add": "jp.example.com", "port": "443", "id": "b831381d-6324-4d53-ad4f-8cda48b30811",
		"aid": "0", "net": "ws", "path": "/p", "host": "jp.example.com", "tls": "tls",
	})
	vmessJP := "vmess://" + base64.StdEncoding.EncodeToString(vmessJSON)
	links := ssHK + "\n" + vmessJP
	impResp := c.do(http.MethodPost, "/api/nodes/import/links", map[string]string{"links": links})
	var imp struct {
		Imported int `json:"imported"`
		Nodes    []struct {
			ID          int64  `json:"id"`
			Tag         string `json:"tag"`
			CountryCode string `json:"country_code"`
		} `json:"nodes"`
	}
	decodeData(t, impResp, &imp)
	if imp.Imported != 2 {
		t.Fatalf("imported %d nodes, want 2", imp.Imported)
	}
	// country detection: HK and JP from emoji flags
	countries := map[string]string{}
	var nodeIDs []int64
	for _, n := range imp.Nodes {
		countries[n.Tag] = n.CountryCode
		nodeIDs = append(nodeIDs, n.ID)
	}
	if countries["🇭🇰 HK-1"] != "HK" || countries["🇯🇵 JP-1"] != "JP" {
		t.Errorf("country detection wrong: %+v", countries)
	}

	// find the fakeip template id
	var templates []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/templates", nil), &templates)
	var fakeipID int64
	for _, tm := range templates {
		if tm.Name == "fakeip" {
			fakeipID = tm.ID
		}
	}
	if fakeipID == 0 {
		t.Fatal("fakeip template not seeded")
	}

	// create a profile with these nodes
	var profile struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "myprofile", "template_id": fakeipID,
		"node_ids": nodeIDs, "options": map[string]bool{"autoCountryGroups": true},
	}), &profile)
	if profile.Token == "" {
		t.Fatal("no token issued")
	}

	// fetch the PUBLIC subscription (no auth) and validate with the kernel
	pubResp, err := http.Get(ts.URL + "/sub/" + profile.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode != http.StatusOK {
		t.Fatalf("public sub status %d", pubResp.StatusCode)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(pubResp.Body)
	config := buf.Bytes()

	// the generated config must contain the country groups
	if !bytes.Contains(config, []byte("🇭🇰 Hong Kong")) || !bytes.Contains(config, []byte("🇯🇵 Japan")) {
		t.Errorf("generated config missing country groups")
	}

	// validate against the real kernel
	tmp := filepath.Join(t.TempDir(), "sub.json")
	writeFile(t, tmp, config)
	if out, err := exec.Command(kernelPath, "check", "-c", tmp).CombinedOutput(); err != nil {
		t.Errorf("kernel rejected generated config:\n%s", out)
	}
}

// --- helpers ---

func login(t *testing.T, base string) http.CookieJar {
	t.Helper()
	client := &http.Client{}
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "password123"})
	resp, err := client.Post(base+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return mustJar(t, base, resp.Cookies())
}

func mustJar(t *testing.T, base string, cookies []*http.Cookie) http.CookieJar {
	t.Helper()
	jar := newCookieJar(t)
	u := mustURL(t, base)
	jar.SetCookies(u, cookies)
	return jar
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := writeFileImpl(path, data); err != nil {
		t.Fatal(err)
	}
}
