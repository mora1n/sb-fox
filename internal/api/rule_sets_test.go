package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"testing"
)

const validRuleSetJSON = `{"version":4,"rules":[{"domain_suffix":["example.com"]}]}`

func TestRuleSetAPIAndPublicArtifactsWithRealKernel(t *testing.T) {
	if _, err := exec.LookPath("sing-box"); err != nil {
		t.Skip("sing-box not in PATH")
	}
	_, ts := testServer(t)
	client := newClient(t, ts.URL)
	client.http.Jar = login(t, ts.URL)

	var created struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		RuleCount  int    `json:"rule_count"`
		JSONSHA256 string `json:"json_sha256"`
		SRSSize    int64  `json:"srs_size"`
	}
	decodeData(t, client.do(http.MethodPost, "/api/rule-sets", map[string]any{
		"name": "api 规则", "description": "test",
		"sources": []map[string]any{{"kind": "manual", "format": "source", "content": validRuleSetJSON}},
	}), &created)
	if created.ID == 0 || created.RuleCount != 1 || created.JSONSHA256 == "" || created.SRSSize == 0 {
		t.Fatalf("created = %+v", created)
	}

	var token struct {
		Token string `json:"token"`
	}
	decodeData(t, client.do(http.MethodGet, "/api/auth/subscription-token", nil), &token)
	publicJSON := ts.URL + "/rules/" + url.PathEscape(token.Token) + "/" + url.PathEscape(created.Name) + ".json"
	resp, err := http.Get(publicJSON)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "example.com") {
		t.Fatalf("public JSON status=%d body=%s", resp.StatusCode, body)
	}
	etag := resp.Header.Get("ETag")
	if etag == "" || resp.Header.Get("Content-Disposition") == "" {
		t.Fatalf("public headers = %+v", resp.Header)
	}
	revalidate, _ := http.NewRequest(http.MethodGet, publicJSON, nil)
	revalidate.Header.Set("If-None-Match", etag)
	revalidated, err := http.DefaultClient.Do(revalidate)
	if err != nil {
		t.Fatal(err)
	}
	revalidated.Body.Close()
	if revalidated.StatusCode != http.StatusNotModified {
		t.Fatalf("revalidate status = %d", revalidated.StatusCode)
	}

	srsResp, err := http.Get(strings.TrimSuffix(publicJSON, ".json") + ".srs")
	if err != nil {
		t.Fatal(err)
	}
	srsBody, _ := io.ReadAll(srsResp.Body)
	srsResp.Body.Close()
	if srsResp.StatusCode != http.StatusOK || len(srsBody) == 0 || srsResp.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("public SRS status=%d size=%d headers=%v", srsResp.StatusCode, len(srsBody), srsResp.Header)
	}

	status, code, _ := decodeError(t, client.do(http.MethodPut, "/api/rule-sets/"+itoa(created.ID), map[string]any{
		"name": "api 规则", "sources": []map[string]any{{"kind": "manual", "format": "source", "content": `{}`}},
	}))
	if status != http.StatusUnprocessableEntity || code != "ruleset_publish_failed" {
		t.Fatalf("invalid update status=%d code=%s", status, code)
	}
	var after struct {
		JSONSHA256 string `json:"json_sha256"`
	}
	decodeData(t, client.do(http.MethodGet, "/api/rule-sets/"+itoa(created.ID), nil), &after)
	if after.JSONSHA256 != created.JSONSHA256 {
		t.Fatalf("failed update replaced artifact: %s != %s", after.JSONSHA256, created.JSONSHA256)
	}

	decodeData(t, client.do(http.MethodPost, "/api/auth/subscription-token/rotate", nil), &token)
	oldResp, err := http.Get(publicJSON)
	if err != nil {
		t.Fatal(err)
	}
	oldResp.Body.Close()
	if oldResp.StatusCode != http.StatusNotFound {
		t.Fatalf("old token status = %d", oldResp.StatusCode)
	}
}

func TestRuleSetRefreshFailureKeepsPublishedSnapshot(t *testing.T) {
	if _, err := exec.LookPath("sing-box"); err != nil {
		t.Skip("sing-box not in PATH")
	}
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(validRuleSetJSON))
	}))
	server, ts := testServer(t)
	server.Fetcher.AllowPrivate = true
	client := newClient(t, ts.URL)
	client.http.Jar = login(t, ts.URL)

	var created struct {
		ID         int64  `json:"id"`
		JSONSHA256 string `json:"json_sha256"`
	}
	decodeData(t, client.do(http.MethodPost, "/api/rule-sets", map[string]any{
		"name":    "remote-rules",
		"sources": []map[string]any{{"kind": "remote", "format": "source", "url": sourceServer.URL}},
	}), &created)
	sourceServer.Close()

	status, code, _ := decodeError(t, client.do(http.MethodPost, "/api/rule-sets/"+itoa(created.ID)+"/refresh", nil))
	if status != http.StatusUnprocessableEntity || code != "ruleset_publish_failed" {
		t.Fatalf("refresh status=%d code=%s", status, code)
	}
	var current struct {
		JSONSHA256 string `json:"json_sha256"`
		LastError  string `json:"last_error"`
	}
	decodeData(t, client.do(http.MethodGet, "/api/rule-sets/"+itoa(created.ID), nil), &current)
	if current.JSONSHA256 != created.JSONSHA256 || current.LastError == "" {
		t.Fatalf("current = %+v", current)
	}
}

func TestRuleSetPublishErrorDetailsShape(t *testing.T) {
	var details ruleSetErrorDetails
	raw, err := json.Marshal(details)
	if err != nil || !strings.Contains(string(raw), `"kind"`) {
		t.Fatalf("details JSON = %s err=%v", raw, err)
	}
}
