package api

import (
	"net/http"
	"strings"
	"testing"
)

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
