package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileEmptyNodeIDsAreJSONArrays(t *testing.T) {
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

	var created struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "empty-nodes", "template_id": templateID, "options": map[string]bool{"autoCountryGroups": true},
	}), &created)

	var got struct {
		NodeIDs      []int64 `json:"node_ids"`
		NodeGroupIDs []int64 `json:"node_group_ids"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/profiles/"+itoa(created.ID), nil), &got)
	if got.NodeIDs == nil {
		t.Fatal("node_ids decoded as nil; want empty JSON array")
	}
	if got.NodeGroupIDs == nil {
		t.Fatal("node_group_ids decoded as nil; want empty JSON array")
	}
}

func TestPreviewSavedProfileRendersConfigAndChecksOwnership(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	profileID := createSavedPreviewProfile(t, c)
	decodeData(t, c.do(http.MethodPut, "/api/profiles/"+itoa(profileID)+"/subscription-enabled", map[string]bool{
		"subscription_enabled": false,
	}), nil)

	var preview struct {
		Config string `json:"config"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/generate/preview", map[string]any{"profile_id": profileID}), &preview)
	assertPreviewConfigHasSavedNode(t, preview.Config)

	decodeData(t, c.do(http.MethodPost, "/api/users", map[string]any{
		"username": "other-preview", "password": "password123",
	}), nil)
	other := newClient(t, ts.URL)
	other.http.Jar = loginAs(t, ts.URL, "other-preview", "password123")
	status, code, _ := decodeError(t, other.do(http.MethodPost, "/api/generate/preview", map[string]any{
		"profile_id": profileID,
	}))
	if status != http.StatusUnprocessableEntity || code != "generate_error" {
		t.Fatalf("other user preview status=%d code=%q", status, code)
	}
}

func TestProfileValidationMarksMissingNodeAndBlocksGeneration(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	profileID, nodeID, profileName := createSavedPreviewProfileWithNode(t, c)
	decodeData(t, c.do(http.MethodDelete, "/api/nodes/"+itoa(nodeID), nil), nil)

	var list []struct {
		ID         int64 `json:"id"`
		Validation struct {
			Valid               bool    `json:"valid"`
			MissingTemplate     bool    `json:"missing_template"`
			MissingNodeIDs      []int64 `json:"missing_node_ids"`
			MissingNodeGroupIDs []int64 `json:"missing_node_group_ids"`
		} `json:"validation"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/profiles", nil), &list)
	var listed *struct {
		ID         int64 `json:"id"`
		Validation struct {
			Valid               bool    `json:"valid"`
			MissingTemplate     bool    `json:"missing_template"`
			MissingNodeIDs      []int64 `json:"missing_node_ids"`
			MissingNodeGroupIDs []int64 `json:"missing_node_group_ids"`
		} `json:"validation"`
	}
	for i := range list {
		if list[i].ID == profileID {
			listed = &list[i]
			break
		}
	}
	if listed == nil {
		t.Fatalf("profile %d missing from list", profileID)
	}
	if listed.Validation.Valid || listed.Validation.MissingTemplate || !sameInt64s(listed.Validation.MissingNodeIDs, []int64{nodeID}) || len(listed.Validation.MissingNodeGroupIDs) != 0 {
		t.Fatalf("list validation = %+v", listed.Validation)
	}

	var profile struct {
		ID         int64 `json:"id"`
		Validation struct {
			Valid               bool    `json:"valid"`
			MissingTemplate     bool    `json:"missing_template"`
			MissingNodeIDs      []int64 `json:"missing_node_ids"`
			MissingNodeGroupIDs []int64 `json:"missing_node_group_ids"`
		} `json:"validation"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/profiles/"+itoa(profileID), nil), &profile)
	if profile.Validation.Valid || profile.Validation.MissingTemplate || !sameInt64s(profile.Validation.MissingNodeIDs, []int64{nodeID}) || len(profile.Validation.MissingNodeGroupIDs) != 0 {
		t.Fatalf("get validation = %+v", profile.Validation)
	}

	status, code, msg := decodeError(t, c.do(http.MethodPost, "/api/generate/preview", map[string]any{
		"profile_id": profileID,
	}))
	if status != http.StatusUnprocessableEntity || code != "generate_error" || !strings.Contains(msg, "missing node") {
		t.Fatalf("preview status=%d code=%q msg=%q", status, code, msg)
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/auth/subscription-token", nil), &tokenResp)
	resp, err := http.Get(ts.URL + "/sub/" + tokenResp.Token + "/" + profileName)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(body), "missing node") {
		t.Fatalf("public sub status=%d body=%q", resp.StatusCode, string(body))
	}
}

func TestProfileValidationMarksMissingNodeInsideNodeGroup(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	templateID := createPreviewOrderTemplate(t, c)
	nodeID := createPreviewOrderNodes(t, c, []string{"dangling-node"})[0]
	groupID := createPreviewOrderGroup(t, c, "dangling-group", []int64{nodeID})
	var profile struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name":        "group-invalid",
		"template_id": templateID,
		"options": map[string]any{
			"autoCountryGroups": false,
			"groupSelections": map[string]any{
				"Proxy": map[string]any{
					"nodeGroupIds": []int64{groupID},
				},
			},
		},
	}), &profile)

	decodeData(t, c.do(http.MethodDelete, "/api/nodes/"+itoa(nodeID), nil), nil)

	var got struct {
		Validation struct {
			Valid               bool    `json:"valid"`
			MissingTemplate     bool    `json:"missing_template"`
			MissingNodeIDs      []int64 `json:"missing_node_ids"`
			MissingNodeGroupIDs []int64 `json:"missing_node_group_ids"`
		} `json:"validation"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/profiles/"+itoa(profile.ID), nil), &got)
	if got.Validation.Valid || got.Validation.MissingTemplate || len(got.Validation.MissingNodeIDs) != 0 || !sameInt64s(got.Validation.MissingNodeGroupIDs, []int64{groupID}) {
		t.Fatalf("validation = %+v", got.Validation)
	}

	status, code, msg := decodeError(t, c.do(http.MethodPost, "/api/generate/preview", map[string]any{
		"profile_id": profile.ID,
	}))
	if status != http.StatusUnprocessableEntity || code != "generate_error" || !strings.Contains(msg, "missing node") {
		t.Fatalf("preview status=%d code=%q msg=%q", status, code, msg)
	}
}

func TestPreviewPreservesSelectionAndNodeGroupOrder(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	templateID := createPreviewOrderTemplate(t, c)
	nodeIDs := createPreviewOrderNodes(t, c, []string{"n1", "n2", "n3", "n4", "n5"})
	groupA := createPreviewOrderGroup(t, c, "ga", []int64{nodeIDs[4], nodeIDs[0]})
	groupB := createPreviewOrderGroup(t, c, "gb", []int64{nodeIDs[3], nodeIDs[1]})

	var preview struct {
		Config string `json:"config"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/generate/preview", map[string]any{
		"template_id": templateID,
		"options": map[string]any{
			"autoCountryGroups": false,
			"groupSelections": map[string]any{
				"Proxy": map[string]any{
					"nodeIds":      []int64{nodeIDs[2], nodeIDs[0]},
					"nodeGroupIds": []int64{groupB, groupA},
				},
			},
		},
	}), &preview)

	outbounds := generatedOutboundMap(t, []byte(preview.Config))
	got := stringSliceValue(t, outbounds["Proxy"]["outbounds"])
	want := []string{"n3", "n1", "n4", "n2", "n5"}
	if !sameStrings(got, want) {
		t.Fatalf("Proxy outbounds = %v, want %v", got, want)
	}
}

func TestPreviewReturnsStructuredGenerationErrors(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)
	nodeID := createPreviewOrderNodes(t, c, []string{"node-us"})[0]
	baseTemplateID := createTemplate(t, c, "structured-base", `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`)

	tests := []struct {
		name       string
		templateID int64
		content    string
		options    map[string]any
		status     int
		code       string
		want       generationErrorDetails
	}{
		{
			name: "empty group",
			content: `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Fallback","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Fallback"}],
    "final":"Proxy"
  }
}`,
			options: map[string]any{
				"autoCountryGroups": false,
				"groupSelections": map[string]any{
					"Proxy": map[string]any{"outboundRefs": []string{"Direct"}},
				},
			},
			status: http.StatusBadRequest,
			code:   "bad_request",
			want:   generationErrorDetails{Kind: generateErrEmptyGroup, Panel: "group", GroupTag: "Fallback"},
		},
		{
			name: "invalid final",
			content: `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`,
			options: map[string]any{
				"autoCountryGroups": false,
			},
			status: http.StatusUnprocessableEntity,
			code:   "generate_error",
			want:   generationErrorDetails{Kind: generateErrInvalidFinal, Panel: "group", GroupTag: "Proxy"},
		},
		{
			name: "unknown outbound ref",
			content: `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Fallback","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Fallback"}],
    "final":"Proxy"
  }
}`,
			options: map[string]any{
				"autoCountryGroups": false,
				"groupSelections": map[string]any{
					"Fallback": map[string]any{"outboundRefs": []string{"Proxy"}},
				},
			},
			status: http.StatusBadRequest,
			code:   "bad_request",
			want:   generationErrorDetails{Kind: generateErrUnknownOutboundRef, Panel: "group", GroupTag: "Fallback", OutboundTag: "Proxy"},
		},
		{
			name: "group cycle",
			content: `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Fallback","Direct"]},
    {"type":"selector","tag":"Fallback","outbounds":["Proxy","Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Fallback"}],
    "final":"Proxy"
  }
}`,
			options: map[string]any{
				"autoCountryGroups": false,
				"groupSelections": map[string]any{
					"Proxy":    map[string]any{"outboundRefs": []string{"Fallback"}},
					"Fallback": map[string]any{"outboundRefs": []string{"Proxy"}},
				},
			},
			status: http.StatusBadRequest,
			code:   "bad_request",
			want:   generationErrorDetails{Kind: generateErrGroupCycle, Panel: "group", GroupTag: "Proxy", Cycle: []string{"Proxy", "Fallback", "Proxy"}},
		},
		{
			name:       "auto country empty",
			templateID: baseTemplateID,
			options: map[string]any{
				"autoCountryGroups":    true,
				"autoCountrySelection": map[string]any{},
				"groupSelections": map[string]any{
					"Proxy": map[string]any{"nodeIds": []int64{nodeID}},
				},
			},
			status: http.StatusBadRequest,
			code:   "bad_request",
			want:   generationErrorDetails{Kind: generateErrAutoCountryEmpty, Panel: "country"},
		},
		{
			name:       "chain proxy empty",
			templateID: baseTemplateID,
			options: map[string]any{
				"autoCountryGroups": false,
				"chainProxy":        true,
				"groupSelections": map[string]any{
					"Proxy": map[string]any{"outboundRefs": []string{"Direct"}},
				},
			},
			status: http.StatusBadRequest,
			code:   "bad_request",
			want:   generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			templateID := tc.templateID
			if tc.content != "" {
				templateID = createTemplate(t, c, "structured-"+strings.ReplaceAll(tc.name, " ", "-"), tc.content)
			}
			status, code, _, details := decodeErrorDetails(t, c.do(http.MethodPost, "/api/generate/preview", map[string]any{
				"template_id": templateID,
				"options":     tc.options,
			}))
			if status != tc.status || code != tc.code {
				t.Fatalf("status=%d code=%q, want status=%d code=%q", status, code, tc.status, tc.code)
			}
			if details.Kind != tc.want.Kind || details.Panel != tc.want.Panel || details.GroupTag != tc.want.GroupTag || details.OutboundTag != tc.want.OutboundTag {
				t.Fatalf("details=%+v, want %+v", details, tc.want)
			}
			if len(tc.want.Cycle) > 0 && !sameStrings(details.Cycle, tc.want.Cycle) {
				t.Fatalf("cycle=%v, want %v", details.Cycle, tc.want.Cycle)
			}
		})
	}
}

func createTemplate(t *testing.T, c *apiClient, name, content string) int64 {
	t.Helper()
	var created struct {
		Template struct {
			ID int64 `json:"id"`
		} `json:"template"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/templates", map[string]string{
		"name":    name,
		"content": content,
	}), &created)
	return created.Template.ID
}

func createPreviewOrderTemplate(t *testing.T, c *apiClient) int64 {
	t.Helper()
	return createTemplate(t, c, "preview-order", `{"outbounds":[{"type":"selector","tag":"Proxy","outbounds":[]},{"type":"direct","tag":"Direct"}],"route":{"final":"Proxy"}}`)
}

func createPreviewOrderNodes(t *testing.T, c *apiClient, tags []string) []int64 {
	t.Helper()
	ids := make([]int64, 0, len(tags))
	for _, tag := range tags {
		var node struct {
			ID int64 `json:"id"`
		}
		raw := `{"type":"shadowsocks","tag":"` + tag + `","server":"example.com","server_port":443}`
		decodeData(t, c.do(http.MethodPost, "/api/nodes", map[string]string{"raw": raw}), &node)
		ids = append(ids, node.ID)
	}
	return ids
}

func createPreviewOrderGroup(t *testing.T, c *apiClient, name string, nodeIDs []int64) int64 {
	t.Helper()
	var group struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/node-groups", map[string]any{
		"name": name, "node_ids": nodeIDs,
	}), &group)
	return group.ID
}

func createSavedPreviewProfile(t *testing.T, c *apiClient) int64 {
	t.Helper()
	profileID, _, _ := createSavedPreviewProfileWithNode(t, c)
	return profileID
}

func createSavedPreviewProfileWithNode(t *testing.T, c *apiClient) (int64, int64, string) {
	t.Helper()
	template := `{"outbounds":[{"type":"selector","tag":"Proxy","outbounds":[]},{"type":"direct","tag":"Direct"}],"route":{"final":"Proxy"}}`
	var createdTemplate struct {
		Template struct {
			ID int64 `json:"id"`
		} `json:"template"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/templates", map[string]string{
		"name":    "preview-profile",
		"content": template,
	}), &createdTemplate)

	var node struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes", map[string]string{
		"raw": `{"type":"shadowsocks","tag":"preview-node","server":"example.com","server_port":443,"method":"aes-128-gcm","password":"pw"}`,
	}), &node)

	var profile struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name":        "saved-preview",
		"template_id": createdTemplate.Template.ID,
		"options": map[string]any{
			"autoCountryGroups": false,
			"groupSelections": map[string]any{
				"Proxy": map[string]any{"nodeIds": []int64{node.ID}},
			},
		},
	}), &profile)
	return profile.ID, node.ID, "saved-preview"
}

func sameInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func assertPreviewConfigHasSavedNode(t *testing.T, config string) {
	t.Helper()
	var rendered struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(config), &rendered); err != nil {
		t.Fatalf("unmarshal preview config: %v\n%s", err, config)
	}
	var hasNode, proxyReferencesNode bool
	for _, outbound := range rendered.Outbounds {
		switch outbound["tag"] {
		case "preview-node":
			hasNode = true
		case "Proxy":
			refs, _ := outbound["outbounds"].([]any)
			for _, ref := range refs {
				if ref == "preview-node" {
					proxyReferencesNode = true
				}
			}
		}
	}
	if !hasNode {
		t.Fatalf("preview config missing saved node: %s", config)
	}
	if !proxyReferencesNode {
		t.Fatalf("preview config missing selector membership: %s", config)
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
		ID                  int64 `json:"id"`
		SubscriptionEnabled bool  `json:"subscription_enabled"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "myprofile", "template_id": fakeipID,
		"node_ids": nodeIDs, "options": map[string]bool{"autoCountryGroups": true},
	}), &profile)
	if profile.ID == 0 {
		t.Fatal("profile id is empty")
	}
	if !profile.SubscriptionEnabled {
		t.Fatal("profile subscription should be enabled by default")
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

	decodeData(t, c.do(http.MethodPost, "/api/users", map[string]any{
		"username": "other", "password": "password123",
	}), nil)
	otherClient := newClient(t, ts.URL)
	otherClient.http.Jar = loginAs(t, ts.URL, "other", "password123")
	status, _, _ := decodeError(t, otherClient.do(http.MethodPut, "/api/profiles/"+itoa(profile.ID)+"/subscription-enabled", map[string]bool{
		"subscription_enabled": false,
	}))
	if status != http.StatusNotFound {
		t.Fatalf("non-owner subscription switch status=%d", status)
	}

	var profileSwitch struct {
		SubscriptionEnabled bool `json:"subscription_enabled"`
	}
	decodeData(t, c.do(http.MethodPut, "/api/profiles/"+itoa(profile.ID)+"/subscription-enabled", map[string]bool{
		"subscription_enabled": false,
	}), &profileSwitch)
	if profileSwitch.SubscriptionEnabled {
		t.Fatal("profile subscription should be disabled")
	}
	disabledResp, err := http.Get(ts.URL + "/sub/" + tokenResp.Token + "/myprofile")
	if err != nil {
		t.Fatal(err)
	}
	disabledResp.Body.Close()
	if disabledResp.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled public sub status %d", disabledResp.StatusCode)
	}
	decodeData(t, c.do(http.MethodPut, "/api/profiles/"+itoa(profile.ID)+"/subscription-enabled", map[string]bool{
		"subscription_enabled": true,
	}), &profileSwitch)
	if !profileSwitch.SubscriptionEnabled {
		t.Fatal("profile subscription should be enabled")
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
