package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	hash, _ := auth.HashPassword("password123")
	if err := db.SetAdmin("admin", hash); err != nil {
		t.Fatal(err)
	}
	admin, err := db.FirstAdmin()
	if err != nil {
		t.Fatal(err)
	}

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
		if _, err := db.SeedUserTemplate(admin.ID, name, string(content), "test template"); err != nil {
			t.Fatal(err)
		}
	}

	kernelPath, _ := exec.LookPath("sing-box")
	srv := &Server{
		Store:       db,
		Auth:        auth.New([]byte("test-secret")),
		Kernel:      kernel.New(kernelPath, t.TempDir(), 15*time.Second),
		Fetcher:     subfetch.New(),
		TemplateDir: filepath.Join("..", "..", "data", "templates"),
		DevMode:     true,
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

func decodeError(t *testing.T, resp *http.Response) (int, string, string) {
	t.Helper()
	defer resp.Body.Close()
	var env struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error == nil {
		t.Fatalf("expected api error, got status %d", resp.StatusCode)
	}
	return resp.StatusCode, env.Error.Code, env.Error.Message
}

func decodeErrorDetails(t *testing.T, resp *http.Response) (int, string, string, generationErrorDetails) {
	t.Helper()
	defer resp.Body.Close()
	var env struct {
		Error *struct {
			Code    string                 `json:"code"`
			Message string                 `json:"message"`
			Details generationErrorDetails `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error == nil {
		t.Fatalf("expected api error, got status %d", resp.StatusCode)
	}
	return resp.StatusCode, env.Error.Code, env.Error.Message, env.Error.Details
}

// --- helpers ---

func login(t *testing.T, base string) http.CookieJar {
	return loginAs(t, base, "admin", "password123")
}

func loginAs(t *testing.T, base, username, password string) http.CookieJar {
	t.Helper()
	client := &http.Client{}
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := client.Post(base+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login %s status %d", username, resp.StatusCode)
	}
	resp.Body.Close()
	return mustJar(t, base, resp.Cookies())
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
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
