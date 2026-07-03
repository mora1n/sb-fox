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

func TestAuthErrorsPasswordMinimumAndSubscriptionHostPrefix(t *testing.T) {
	srv, ts := testServer(t)
	c := newClient(t, ts.URL)

	status, code, msg := decodeError(t, c.do(http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin", "password": "wrong",
	}))
	if status != http.StatusUnauthorized || code != "unauthorized" || msg != "invalid username or password" {
		t.Fatalf("bad login status=%d code=%q msg=%q", status, code, msg)
	}

	srv.RegistrationEnabled = true
	status, _, msg = decodeError(t, c.do(http.MethodPost, "/api/auth/register", map[string]string{
		"username": "short", "password": "123",
	}))
	if status != http.StatusBadRequest || !strings.Contains(msg, "at least 4") {
		t.Fatalf("short register status=%d msg=%q", status, msg)
	}
	decodeData(t, c.do(http.MethodPost, "/api/auth/register", map[string]string{
		"username": "ok4", "password": "1234",
	}), nil)

	admin := newClient(t, ts.URL)
	admin.http.Jar = login(t, ts.URL)
	status, _, msg = decodeError(t, admin.do(http.MethodPost, "/api/auth/password", map[string]string{
		"old_password": "password123", "new_password": "123",
	}))
	if status != http.StatusBadRequest || !strings.Contains(msg, "at least 4") {
		t.Fatalf("short password change status=%d msg=%q", status, msg)
	}
	status, _, msg = decodeError(t, admin.do(http.MethodPost, "/api/auth/password", map[string]string{
		"old_password": "bad", "new_password": "1234",
	}))
	if status != http.StatusUnauthorized || msg != "current password is incorrect" {
		t.Fatalf("wrong current password status=%d msg=%q", status, msg)
	}

	status, _, msg = decodeError(t, admin.do(http.MethodPut, "/api/settings", map[string]string{
		"subscription_host_prefix": "ftp://example.com",
	}))
	if status != http.StatusBadRequest || !strings.Contains(msg, "http:// or https://") {
		t.Fatalf("invalid host prefix status=%d msg=%q", status, msg)
	}
	decodeData(t, admin.do(http.MethodPut, "/api/settings", map[string]string{
		"subscription_host_prefix": "https://example.com/subs/",
	}), nil)
	var app struct {
		SubscriptionHost string `json:"subscription_host_prefix"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/app", nil), &app)
	if app.SubscriptionHost != "https://example.com/subs" {
		t.Fatalf("subscription host prefix = %q", app.SubscriptionHost)
	}
}

func TestRegistrationAdminResetAndQuota(t *testing.T) {
	srv, ts := testServer(t)
	c := newClient(t, ts.URL)

	resp := c.do(http.MethodPost, "/api/auth/register", map[string]string{"username": "alice", "password": "password123"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("register disabled status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	srv.RegistrationEnabled = true
	var registered struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/auth/register", map[string]string{"username": "alice", "password": "password123"}), &registered)
	if registered.ID == 0 || registered.Role != "user" {
		t.Fatalf("registered user = %+v", registered)
	}

	c.http.Jar = login(t, ts.URL)
	var reset struct {
		Password string `json:"password"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/users/"+itoa(registered.ID)+"/reset-password", nil), &reset)
	if reset.Password == "" {
		t.Fatal("reset password is empty")
	}
	decodeData(t, c.do(http.MethodPut, "/api/users/"+itoa(registered.ID), map[string]any{
		"username": "alice", "role": "user", "node_limit": 1, "profile_limit": 0, "template_limit": 0,
	}), nil)

	userClient := newClient(t, ts.URL)
	badLogin := userClient.do(http.MethodPost, "/api/auth/login", map[string]string{"username": "alice", "password": "password123"})
	if badLogin.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old password login status = %d", badLogin.StatusCode)
	}
	badLogin.Body.Close()
	userClient.http.Jar = loginAs(t, ts.URL, "alice", reset.Password)

	raw1 := `{"type":"shadowsocks","tag":"n1","server":"a.example.com","server_port":1}`
	raw2 := `{"type":"shadowsocks","tag":"n2","server":"b.example.com","server_port":2}`
	create1 := userClient.do(http.MethodPost, "/api/nodes", map[string]string{"raw": raw1})
	if create1.StatusCode != http.StatusCreated {
		t.Fatalf("first node status = %d", create1.StatusCode)
	}
	create1.Body.Close()
	create2 := userClient.do(http.MethodPost, "/api/nodes", map[string]string{"raw": raw2})
	if create2.StatusCode != http.StatusForbidden {
		t.Fatalf("quota status = %d", create2.StatusCode)
	}
	create2.Body.Close()
}

func TestAdminUserRoleIsNotEditableFromPanel(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	status, _, msg := decodeError(t, c.do(http.MethodPost, "/api/users", map[string]any{
		"username": "root2", "password": "password123", "role": "admin",
	}))
	if status != http.StatusBadRequest || !strings.Contains(msg, "role user") {
		t.Fatalf("create admin status=%d msg=%q", status, msg)
	}

	var user struct {
		ID   int64  `json:"id"`
		Role string `json:"role"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/users", map[string]any{
		"username": "bob", "password": "password123",
	}), &user)
	if user.Role != "user" {
		t.Fatalf("created role = %q", user.Role)
	}

	status, _, msg = decodeError(t, c.do(http.MethodPut, "/api/users/"+itoa(user.ID), map[string]any{
		"username": "bob", "role": "admin",
	}))
	if status != http.StatusBadRequest || !strings.Contains(msg, "cannot be changed") {
		t.Fatalf("update role status=%d msg=%q", status, msg)
	}

	var updated struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	decodeData(t, c.do(http.MethodPut, "/api/users/"+itoa(user.ID), map[string]any{
		"username": "bob2",
	}), &updated)
	if updated.Username != "bob2" || updated.Role != "user" {
		t.Fatalf("updated user = %+v", updated)
	}
}

func TestUserSettingsPermissionsAndPublicKernelStatus(t *testing.T) {
	srv, ts := testServer(t)
	srv.Kernel.Path = ""
	admin := newClient(t, ts.URL)
	admin.http.Jar = login(t, ts.URL)

	var user struct {
		ID int64 `json:"id"`
	}
	decodeData(t, admin.do(http.MethodPost, "/api/users", map[string]any{
		"username": "carol", "password": "password123",
	}), &user)

	userClient := newClient(t, ts.URL)
	userClient.http.Jar = loginAs(t, ts.URL, "carol", "password123")

	var settings map[string]string
	decodeData(t, userClient.do(http.MethodGet, "/api/settings", nil), &settings)
	if _, ok := settings["app_display_name"]; ok {
		t.Fatalf("non-admin settings leaked app_display_name: %+v", settings)
	}
	if _, ok := settings["kernel_path"]; ok {
		t.Fatalf("non-admin settings leaked kernel_path: %+v", settings)
	}
	if _, ok := settings["subscription_host_prefix"]; ok {
		t.Fatalf("non-admin settings leaked subscription_host_prefix: %+v", settings)
	}
	if _, ok := settings["country_heat_order"]; !ok {
		t.Fatalf("non-admin settings missing country_heat_order: %+v", settings)
	}

	status, _, msg := decodeError(t, userClient.do(http.MethodPut, "/api/settings", map[string]string{"kernel_path": "sing-box"}))
	if status != http.StatusForbidden || !strings.Contains(msg, "admin only") {
		t.Fatalf("kernel setting status=%d msg=%q", status, msg)
	}
	status, _, msg = decodeError(t, userClient.do(http.MethodPut, "/api/settings", map[string]string{"subfetch_allow_private": "true"}))
	if status != http.StatusForbidden || !strings.Contains(msg, "admin only") {
		t.Fatalf("private fetch setting status=%d msg=%q", status, msg)
	}
	status, _, msg = decodeError(t, userClient.do(http.MethodPut, "/api/settings", map[string]string{"subscription_host_prefix": "https://example.com"}))
	if status != http.StatusForbidden || !strings.Contains(msg, "admin only") {
		t.Fatalf("host prefix setting status=%d msg=%q", status, msg)
	}
	status, _, _ = decodeError(t, userClient.do(http.MethodGet, "/api/settings/kernel", nil))
	if status != http.StatusForbidden {
		t.Fatalf("non-admin kernel settings status=%d", status)
	}

	var public map[string]any
	decodeData(t, userClient.do(http.MethodGet, "/api/kernel/status", nil), &public)
	if _, ok := public["path"]; ok {
		t.Fatalf("public kernel status leaked path: %+v", public)
	}
	if got, ok := public["available"].(bool); !ok || got {
		t.Fatalf("public kernel status = %+v", public)
	}

	var adminKernel map[string]any
	decodeData(t, admin.do(http.MethodGet, "/api/settings/kernel", nil), &adminKernel)
	if _, ok := adminKernel["path"]; !ok {
		t.Fatalf("admin kernel status missing path: %+v", adminKernel)
	}
}

func TestNodeUsageEndpoint(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	var templates []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/templates", nil), &templates)
	var templateID int64
	for _, tm := range templates {
		if tm.Name == "fakeip" {
			templateID = tm.ID
		}
	}
	if templateID == 0 {
		t.Fatal("fakeip template not seeded")
	}

	createNode := func(tag string) int64 {
		t.Helper()
		raw := `{"type":"shadowsocks","tag":"` + tag + `","server":"example.com","server_port":443}`
		var node struct {
			ID int64 `json:"id"`
		}
		decodeData(t, c.do(http.MethodPost, "/api/nodes", map[string]string{"raw": raw}), &node)
		return node.ID
	}
	n1 := createNode("n1")
	n2 := createNode("n2")

	var group struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/node-groups", map[string]any{
		"name": "g1", "node_ids": []int64{n1},
	}), &group)
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "direct", "template_id": templateID, "node_ids": []int64{n1, n2},
		"options": map[string]bool{"autoCountryGroups": true},
	}), nil)
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "grouped", "template_id": templateID, "node_group_ids": []int64{group.ID},
		"options": map[string]bool{"autoCountryGroups": true},
	}), nil)

	var usage []struct {
		ProfileName  string `json:"profile_name"`
		ViaGroupName string `json:"via_group_name"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/nodes/"+itoa(n1)+"/usage", nil), &usage)
	if len(usage) != 2 {
		t.Fatalf("usage = %+v", usage)
	}
	seen := map[string]string{}
	for _, u := range usage {
		seen[u.ProfileName] = u.ViaGroupName
	}
	if _, ok := seen["direct"]; !ok {
		t.Fatalf("missing direct usage: %+v", usage)
	}
	if seen["grouped"] != "g1" {
		t.Fatalf("missing group usage: %+v", usage)
	}
}

func TestExportLinksRejectsUnsupportedNode(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	var node struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes", map[string]string{
		"raw": `{"type":"direct","tag":"direct"}`,
	}), &node)

	status, code, msg := decodeError(t, c.do(http.MethodPost, "/api/nodes/export/links", map[string]any{
		"node_ids": []int64{node.ID},
	}))
	if status != http.StatusUnprocessableEntity || code != "unsupported_node" || !strings.Contains(msg, "unsupported outbound type") {
		t.Fatalf("export unsupported status=%d code=%q msg=%q", status, code, msg)
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

	exportResp := c.do(http.MethodPost, "/api/nodes/export/links", map[string]any{"node_ids": nodeIDs})
	if exportResp.StatusCode != http.StatusOK {
		status, code, msg := decodeError(t, exportResp)
		t.Fatalf("export links status=%d code=%q msg=%q", status, code, msg)
	}
	if got := exportResp.Header.Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("export links content-type = %q", got)
	}
	if got := exportResp.Header.Get("Content-Disposition"); !strings.Contains(got, "nodes-links.txt") {
		t.Fatalf("export links disposition = %q", got)
	}
	exportBody := new(bytes.Buffer)
	exportBody.ReadFrom(exportResp.Body)
	exportResp.Body.Close()
	exportText := exportBody.String()
	if !strings.Contains(exportText, "ss://") || !strings.Contains(exportText, "vmess://") {
		t.Fatalf("export links body = %q", exportText)
	}
	if strings.Contains(exportText, "%23HK") || strings.Contains(exportText, "%23JP") {
		t.Fatalf("export links leaked country server marker: %q", exportText)
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
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "myprofile", "template_id": fakeipID,
		"node_ids": nodeIDs, "options": map[string]bool{"autoCountryGroups": true},
	}), &profile)
	if profile.ID == 0 {
		t.Fatal("profile id is empty")
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/auth/subscription-token", nil), &tokenResp)
	if tokenResp.Token == "" {
		t.Fatal("no shared subscription token issued")
	}
	oldResp, err := http.Get(ts.URL + "/sub/" + tokenResp.Token)
	if err != nil {
		t.Fatal(err)
	}
	oldResp.Body.Close()
	if oldResp.StatusCode != http.StatusNotFound {
		t.Fatalf("old public sub status %d", oldResp.StatusCode)
	}

	// fetch the PUBLIC subscription (no auth) and validate with the kernel
	pubResp, err := http.Get(ts.URL + "/sub/" + tokenResp.Token + "/myprofile")
	if err != nil {
		t.Fatal(err)
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode != http.StatusOK {
		t.Fatalf("public sub status %d", pubResp.StatusCode)
	}
	if got := pubResp.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("public sub content-type = %q, want application/json", got)
	}
	if got := pubResp.Header.Get("Content-Disposition"); got != "" {
		t.Fatalf("public sub should render inline without content-disposition, got %q", got)
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
