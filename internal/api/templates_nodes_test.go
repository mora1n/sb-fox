package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestTemplateLookupByNameForOverwriteFlow(t *testing.T) {
	_, ts := testServer(t)
	admin := newClient(t, ts.URL)
	admin.http.Jar = login(t, ts.URL)

	var created struct {
		Template struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Content string `json:"content"`
		} `json:"template"`
	}
	decodeData(t, admin.do(http.MethodPost, "/api/templates", map[string]string{
		"name": "dup", "content": `{"a":1}`, "description": "old",
	}), &created)
	if created.Template.ID == 0 {
		t.Fatalf("created template = %+v", created.Template)
	}

	var found struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Content string `json:"content"`
	}
	decodeData(t, admin.do(http.MethodGet, "/api/templates/by-name?name=dup", nil), &found)
	if found.ID != created.Template.ID || found.Name != "dup" || found.Kind != "user" || found.Content != `{"a":1}` {
		t.Fatalf("found template = %+v, created = %+v", found, created.Template)
	}

	status, _, msg := decodeError(t, admin.do(http.MethodGet, "/api/templates/by-name", nil))
	if status != http.StatusBadRequest || !strings.Contains(msg, "name is required") {
		t.Fatalf("missing name status=%d msg=%q", status, msg)
	}
	status, _, msg = decodeError(t, admin.do(http.MethodGet, "/api/templates/by-name?name=missing", nil))
	if status != http.StatusNotFound || !strings.Contains(msg, "template not found") {
		t.Fatalf("missing template status=%d msg=%q", status, msg)
	}

	decodeData(t, admin.do(http.MethodPut, "/api/templates/"+itoa(found.ID), map[string]string{
		"content": `{"a":2}`, "description": "new",
	}), nil)
	var updated struct {
		ID          int64  `json:"id"`
		Content     string `json:"content"`
		Description string `json:"description"`
	}
	decodeData(t, admin.do(http.MethodGet, "/api/templates/"+itoa(found.ID), nil), &updated)
	if updated.ID != found.ID || updated.Content != `{"a":2}` || updated.Description != "new" {
		t.Fatalf("updated template = %+v", updated)
	}

	var user struct {
		ID int64 `json:"id"`
	}
	decodeData(t, admin.do(http.MethodPost, "/api/users", map[string]any{
		"username": "dave", "password": "password123", "template_limit": 1,
	}), &user)
	userClient := newClient(t, ts.URL)
	userClient.http.Jar = loginAs(t, ts.URL, "dave", "password123")
	status, _, _ = decodeError(t, userClient.do(http.MethodGet, "/api/templates/by-name?name=dup", nil))
	if status != http.StatusNotFound {
		t.Fatalf("other user lookup status=%d", status)
	}
}

func TestUserResourceIsolationAcrossAccounts(t *testing.T) {
	srv, ts := testServer(t)
	admin := newClient(t, ts.URL)
	admin.http.Jar = login(t, ts.URL)

	createUser := func(username string) int64 {
		t.Helper()
		var user struct {
			ID int64 `json:"id"`
		}
		decodeData(t, admin.do(http.MethodPost, "/api/users", map[string]any{
			"username": username, "password": "password123",
		}), &user)
		if user.ID == 0 {
			t.Fatalf("created user %q = %+v", username, user)
		}
		return user.ID
	}
	aliceID := createUser("isolate-alice")
	bobID := createUser("isolate-bob")

	alice := newClient(t, ts.URL)
	alice.http.Jar = loginAs(t, ts.URL, "isolate-alice", "password123")
	bob := newClient(t, ts.URL)
	bob.http.Jar = loginAs(t, ts.URL, "isolate-bob", "password123")

	templateContent := `{"outbounds":[{"type":"selector","tag":"Proxy","outbounds":[]}],"route":{"final":"Proxy"}}`
	var bobTemplate struct {
		Template struct {
			ID int64 `json:"id"`
		} `json:"template"`
	}
	decodeData(t, bob.do(http.MethodPost, "/api/templates", map[string]string{
		"name": "shared", "content": templateContent,
	}), &bobTemplate)
	if bobTemplate.Template.ID == 0 {
		t.Fatalf("bob template = %+v", bobTemplate)
	}
	status, _, _ := decodeError(t, alice.do(http.MethodGet, "/api/templates/"+itoa(bobTemplate.Template.ID), nil))
	if status != http.StatusNotFound {
		t.Fatalf("alice get bob template status = %d", status)
	}
	status, _, _ = decodeError(t, alice.do(http.MethodDelete, "/api/templates/"+itoa(bobTemplate.Template.ID), nil))
	if status != http.StatusNotFound {
		t.Fatalf("alice delete bob template status = %d", status)
	}
	decodeData(t, bob.do(http.MethodGet, "/api/templates/"+itoa(bobTemplate.Template.ID), nil), nil)

	var bobNode struct {
		ID int64 `json:"id"`
	}
	decodeData(t, bob.do(http.MethodPost, "/api/nodes", map[string]string{
		"raw": `{"type":"shadowsocks","tag":"bob-node","server":"bob.example.com","server_port":443}`,
	}), &bobNode)
	status, _, _ = decodeError(t, alice.do(http.MethodDelete, "/api/nodes/"+itoa(bobNode.ID), nil))
	if status != http.StatusNotFound {
		t.Fatalf("alice delete bob node status = %d", status)
	}
	decodeData(t, bob.do(http.MethodGet, "/api/nodes/"+itoa(bobNode.ID), nil), nil)

	var bobGroup struct {
		ID int64 `json:"id"`
	}
	decodeData(t, bob.do(http.MethodPost, "/api/node-groups", map[string]any{
		"name": "bob-group", "node_ids": []int64{bobNode.ID},
	}), &bobGroup)
	status, _, _ = decodeError(t, alice.do(http.MethodDelete, "/api/node-groups/"+itoa(bobGroup.ID), nil))
	if status != http.StatusNotFound {
		t.Fatalf("alice delete bob group status = %d", status)
	}
	decodeData(t, bob.do(http.MethodGet, "/api/node-groups/"+itoa(bobGroup.ID), nil), nil)

	var bobProfile struct {
		ID int64 `json:"id"`
	}
	decodeData(t, bob.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "bob-profile", "template_id": bobTemplate.Template.ID, "node_ids": []int64{bobNode.ID},
	}), &bobProfile)
	status, _, _ = decodeError(t, alice.do(http.MethodPut, "/api/profiles/"+itoa(bobProfile.ID)+"/subscription-enabled", map[string]bool{
		"subscription_enabled": false,
	}))
	if status != http.StatusNotFound {
		t.Fatalf("alice toggle bob profile status = %d", status)
	}
	status, _, _ = decodeError(t, alice.do(http.MethodDelete, "/api/profiles/"+itoa(bobProfile.ID), nil))
	if status != http.StatusNotFound {
		t.Fatalf("alice delete bob profile status = %d", status)
	}
	decodeData(t, bob.do(http.MethodGet, "/api/profiles/"+itoa(bobProfile.ID), nil), nil)

	sourceID, err := srv.Store.CreateSource(bobID, "bob-source", "https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	status, _, _ = decodeError(t, alice.do(http.MethodDelete, "/api/sources/"+itoa(sourceID), nil))
	if status != http.StatusNotFound {
		t.Fatalf("alice delete bob source status = %d", status)
	}
	var bobSources []struct {
		ID int64 `json:"id"`
	}
	decodeData(t, bob.do(http.MethodGet, "/api/sources", nil), &bobSources)
	if len(bobSources) != 1 || bobSources[0].ID != sourceID {
		t.Fatalf("bob sources = %+v", bobSources)
	}

	var adminTemplates []struct {
		ID          int64 `json:"id"`
		OwnerUserID int64 `json:"owner_user_id"`
	}
	decodeData(t, admin.do(http.MethodGet, "/api/templates?summary=1", nil), &adminTemplates)
	for _, tmpl := range adminTemplates {
		if tmpl.OwnerUserID == bobID || tmpl.OwnerUserID == aliceID {
			t.Fatalf("admin ordinary template list leaked user resource: %+v", adminTemplates)
		}
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

func TestImportConfigDedupesExactNodesButKeepsDifferentTags(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	nodeA := `{"type":"shadowsocks","tag":"same","server":"dup.example.com","server_port":8388,"method":"aes-256-gcm","password":"pw"}`
	nodeAReordered := `{"password":"pw","method":"aes-256-gcm","server_port":8388,"server":"dup.example.com","tag":"same","type":"shadowsocks"}`
	nodeDifferentTag := `{"type":"shadowsocks","tag":"other","server":"dup.example.com","server_port":8388,"method":"aes-256-gcm","password":"pw"}`

	var first struct {
		Imported int `json:"imported"`
		Deduped  int `json:"deduped"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/config", map[string]string{
		"config": `{"outbounds":[` + nodeA + `,` + nodeA + `]}`,
	}), &first)
	if first.Imported != 1 || first.Deduped != 1 {
		t.Fatalf("first import imported=%d deduped=%d, want 1/1", first.Imported, first.Deduped)
	}

	var second struct {
		Imported int `json:"imported"`
		Deduped  int `json:"deduped"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/config", map[string]string{
		"config": `{"outbounds":[` + nodeAReordered + `]}`,
	}), &second)
	if second.Imported != 0 || second.Deduped != 1 {
		t.Fatalf("second import imported=%d deduped=%d, want 0/1", second.Imported, second.Deduped)
	}

	var third struct {
		Imported int `json:"imported"`
		Deduped  int `json:"deduped"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/config", map[string]string{
		"config": `{"outbounds":[` + nodeDifferentTag + `]}`,
	}), &third)
	if third.Imported != 1 || third.Deduped != 0 {
		t.Fatalf("third import imported=%d deduped=%d, want 1/0", third.Imported, third.Deduped)
	}

	var nodes []struct {
		Tag string `json:"tag"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/nodes", nil), &nodes)
	if len(nodes) != 2 {
		t.Fatalf("node count = %d, want 2", len(nodes))
	}
}
