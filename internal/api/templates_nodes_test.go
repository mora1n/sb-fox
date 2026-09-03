package api

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mora1n/sb-fox/internal/models"
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

func TestTemplateImportPreservesRouteReferencedOutbounds(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)
	content := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["node-a","Reqable"]},
    {"type":"shadowsocks","tag":"node-a","server":"a.example.com","server_port":8388},
    {"type":"http","tag":"Reqable","server":"127.0.0.1","server_port":9000}
  ],
  "route": {"rules":[{"type":"logical","mode":"and","rules":[{"process_name":"Reqable.exe"},{"network":"tcp"}],"outbound":"Reqable"}],"final":"Proxy"}
}`

	var created struct {
		Template struct {
			ID      int64  `json:"id"`
			Content string `json:"content"`
		} `json:"template"`
		Imported int `json:"imported"`
		Nodes    []struct {
			Tag string `json:"tag"`
		} `json:"nodes"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/templates", map[string]string{
		"name": "route-ref", "content": content,
	}), &created)
	if created.Imported != 1 || len(created.Nodes) != 1 || created.Nodes[0].Tag != "node-a" {
		t.Fatalf("created import result = %+v", created)
	}
	if strings.Contains(created.Template.Content, `"tag": "node-a"`) || !strings.Contains(created.Template.Content, `"tag": "Reqable"`) {
		t.Fatalf("created template content = %s", created.Template.Content)
	}

	var nodes []struct {
		Tag string `json:"tag"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/nodes", nil), &nodes)
	for _, node := range nodes {
		if node.Tag == "Reqable" {
			t.Fatalf("route-referenced outbound was imported as a node: %+v", nodes)
		}
	}

	var updateResult struct {
		Imported int `json:"imported"`
	}
	decodeData(t, c.do(http.MethodPut, "/api/templates/"+itoa(created.Template.ID), map[string]string{
		"content": content,
	}), &updateResult)
	if updateResult.Imported != 0 {
		t.Fatalf("updated template imported %d nodes, want 0", updateResult.Imported)
	}
	var updated struct {
		Content string `json:"content"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/templates/"+itoa(created.Template.ID), nil), &updated)
	if strings.Contains(updated.Content, `"tag": "node-a"`) || !strings.Contains(updated.Content, `"tag": "Reqable"`) {
		t.Fatalf("updated template content = %s", updated.Content)
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

func TestNodeGroupEmptyNodeIDsAreJSONArrays(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	var created struct {
		ID      int64   `json:"id"`
		NodeIDs []int64 `json:"node_ids"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/node-groups", map[string]string{
		"name": "empty-group",
	}), &created)
	if created.NodeIDs == nil {
		t.Fatal("created node_ids decoded as nil; want empty JSON array")
	}

	var got struct {
		ID      int64   `json:"id"`
		NodeIDs []int64 `json:"node_ids"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/node-groups/"+itoa(created.ID), nil), &got)
	if got.NodeIDs == nil {
		t.Fatal("get node_ids decoded as nil; want empty JSON array")
	}

	var groups []struct {
		ID      int64   `json:"id"`
		NodeIDs []int64 `json:"node_ids"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/node-groups", nil), &groups)
	for _, group := range groups {
		if group.ID == created.ID {
			if group.NodeIDs == nil {
				t.Fatal("list node_ids decoded as nil; want empty JSON array")
			}
			return
		}
	}
	t.Fatalf("created group %d not found in list", created.ID)
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
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "legacy-chain", "template_id": templateID, "node_ids": []int64{n1, n2},
		"options": map[string]any{
			"autoCountryGroups": false,
			"chainProxy":        true,
			"chainProxyNodeIds": []int64{n1},
		},
	}), nil)

	var usage []struct {
		ProfileName  string `json:"profile_name"`
		ViaGroupName string `json:"via_group_name"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/nodes/"+itoa(n1)+"/usage", nil), &usage)
	if len(usage) != 3 {
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
	if _, ok := seen["legacy-chain"]; !ok {
		t.Fatalf("missing legacy chain usage: %+v", usage)
	}
}

func TestBulkDeleteNodesPreviewAndDelete(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	templateID := createTemplate(t, c, "bulk-node-template", `{"outbounds":[{"type":"selector","tag":"Proxy","outbounds":[]}],"route":{"final":"Proxy"}}`)
	nodeIDs := createPreviewOrderNodes(t, c, []string{"bulk-a", "bulk-b"})
	groupID := createPreviewOrderGroup(t, c, "bulk-group", []int64{nodeIDs[0]})

	var profile struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name":        "bulk-profile",
		"template_id": templateID,
		"node_ids":    []int64{nodeIDs[0]},
		"options": map[string]any{
			"autoCountryGroups": false,
			"groupSelections": map[string]any{
				"Proxy": map[string]any{
					"nodeIds":      []int64{nodeIDs[1]},
					"nodeGroupIds": []int64{groupID},
				},
			},
		},
	}), &profile)

	var preview struct {
		Usage []struct {
			NodeID       int64  `json:"node_id"`
			ProfileName  string `json:"profile_name"`
			ViaGroupName string `json:"via_group_name"`
		} `json:"usage"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/bulk-delete/preview", map[string]any{
		"ids": nodeIDs,
	}), &preview)
	seen := map[int64]bool{}
	for _, item := range preview.Usage {
		if item.ProfileName == "bulk-profile" {
			seen[item.NodeID] = true
		}
	}
	if !seen[nodeIDs[0]] || !seen[nodeIDs[1]] {
		t.Fatalf("bulk preview usage = %+v, want both nodes", preview.Usage)
	}

	var deleted struct {
		Deleted int `json:"deleted"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/bulk-delete", map[string]any{
		"ids": nodeIDs,
	}), &deleted)
	if deleted.Deleted != len(nodeIDs) {
		t.Fatalf("deleted nodes = %d, want %d", deleted.Deleted, len(nodeIDs))
	}
	for _, id := range nodeIDs {
		status, _, _ := decodeError(t, c.do(http.MethodGet, "/api/nodes/"+itoa(id), nil))
		if status != http.StatusNotFound {
			t.Fatalf("node %d still exists, get status=%d", id, status)
		}
	}
	status, _, _ := decodeError(t, c.do(http.MethodGet, "/api/node-groups/"+itoa(groupID), nil))
	if status != http.StatusNotFound {
		t.Fatalf("group after bulk delete status = %d, want 404", status)
	}
	var saved struct {
		NodeIDs    []int64 `json:"node_ids"`
		Options    string  `json:"options"`
		Validation struct {
			Valid               bool    `json:"valid"`
			MissingNodeIDs      []int64 `json:"missing_node_ids"`
			MissingNodeGroupIDs []int64 `json:"missing_node_group_ids"`
		} `json:"validation"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/profiles/"+itoa(profile.ID), nil), &saved)
	if len(saved.NodeIDs) != 0 || !saved.Validation.Valid || len(saved.Validation.MissingNodeIDs) != 0 || len(saved.Validation.MissingNodeGroupIDs) != 0 {
		t.Fatalf("profile after bulk delete = %+v", saved)
	}
	var options models.ProfileOptions
	if err := json.Unmarshal([]byte(saved.Options), &options); err != nil {
		t.Fatal(err)
	}
	selection := options.GroupSelections["Proxy"]
	if len(selection.NodeIDs) != 0 || len(selection.NodeGroupIDs) != 0 {
		t.Fatalf("profile selection after bulk delete = %+v", selection)
	}
}

func TestBulkDeleteRejectsInvalidBatchWithoutPartialDelete(t *testing.T) {
	srv, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	var userTemplate struct {
		Template struct {
			ID int64 `json:"id"`
		} `json:"template"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/templates", map[string]string{
		"name":    "bulk-user-template",
		"content": `{"outbounds":[]}`,
	}), &userTemplate)
	protectedID, err := srv.Store.CreateTemplate(&models.Template{
		OwnerUserID: 1,
		Name:        "bulk-protected",
		Kind:        "builtin",
		Content:     `{"outbounds":[]}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	status, code, _ := decodeError(t, c.do(http.MethodPost, "/api/templates/bulk-delete", map[string]any{
		"ids": []int64{userTemplate.Template.ID, protectedID},
	}))
	if status != http.StatusForbidden || code != "forbidden" {
		t.Fatalf("mixed template bulk delete status=%d code=%q", status, code)
	}
	decodeData(t, c.do(http.MethodGet, "/api/templates/"+itoa(userTemplate.Template.ID), nil), nil)

	nodeIDs := createPreviewOrderNodes(t, c, []string{"bulk-group-node"})
	groupIDs := []int64{
		createPreviewOrderGroup(t, c, "bulk-group-a", nodeIDs),
		createPreviewOrderGroup(t, c, "bulk-group-b", nodeIDs),
	}
	var groupDeleted struct {
		Deleted int `json:"deleted"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/node-groups/bulk-delete", map[string]any{
		"ids": groupIDs,
	}), &groupDeleted)
	if groupDeleted.Deleted != len(groupIDs) {
		t.Fatalf("deleted groups = %d, want %d", groupDeleted.Deleted, len(groupIDs))
	}

	templateID := createTemplate(t, c, "bulk-profile-template", `{"outbounds":[{"type":"selector","tag":"Proxy","outbounds":[]}],"route":{"final":"Proxy"}}`)
	profileIDs := make([]int64, 0, 2)
	for _, name := range []string{"bulk-profile-a", "bulk-profile-b"} {
		var profile struct {
			ID int64 `json:"id"`
		}
		decodeData(t, c.do(http.MethodPost, "/api/profiles", map[string]any{
			"name":        name,
			"template_id": templateID,
			"options": map[string]any{
				"autoCountryGroups": false,
				"groupSelections": map[string]any{
					"Proxy": map[string]any{"nodeIds": nodeIDs},
				},
			},
		}), &profile)
		profileIDs = append(profileIDs, profile.ID)
	}
	var profileDeleted struct {
		Deleted int `json:"deleted"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/profiles/bulk-delete", map[string]any{
		"ids": profileIDs,
	}), &profileDeleted)
	if profileDeleted.Deleted != len(profileIDs) {
		t.Fatalf("deleted profiles = %d, want %d", profileDeleted.Deleted, len(profileIDs))
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

func TestCreateAndUpdateVLESSNodeNormalizesPacketEncodingNone(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	createRaw := `{"type":"vless","tag":"VLESS","server":"vless.example.com","server_port":443,"uuid":"uuid-1234","packet_encoding":"none"}`
	var created struct {
		ID  int64  `json:"id"`
		Raw string `json:"raw"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes", map[string]string{"raw": createRaw}), &created)
	if strings.Contains(created.Raw, "packet_encoding") {
		t.Fatalf("created raw keeps packet_encoding=none: %s", created.Raw)
	}

	updateRaw := `{"type":"vless","tag":"VLESS","server":"vless.example.com","server_port":443,"uuid":"uuid-1234","packet_encoding":"xudp"}`
	var updated struct {
		Raw string `json:"raw"`
	}
	decodeData(t, c.do(http.MethodPut, "/api/nodes/"+itoa(created.ID), map[string]string{"raw": updateRaw}), &updated)
	if !strings.Contains(updated.Raw, `"packet_encoding":"xudp"`) {
		t.Fatalf("updated raw dropped valid packet_encoding: %s", updated.Raw)
	}
}

func TestCreateAndUpdateHTTPNodePreservesDialFields(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	createRaw := `{
  "type":"http",
  "tag":"⚪ Po0",
  "server":"console.po0.com",
  "server_port":443,
  "path":"/modules/servers/penguin/api/firewall.php?action=add",
  "username":"",
  "password":"",
  "detour":"Proxy",
  "bind_interface":"eth0",
  "inet4_bind_address":"192.0.2.10",
  "inet6_bind_address":"2001:db8::10",
  "bind_address_no_port":true,
  "protect_path":"/run/sing-box/protect.sock",
  "routing_mark":"0x1234",
  "reuse_addr":true,
  "netns":"blue",
  "connect_timeout":"5s",
  "tcp_fast_open":true,
  "tcp_multi_path":true,
  "disable_tcp_keep_alive":true,
  "tcp_keep_alive":"5m",
  "tcp_keep_alive_interval":"75s",
  "udp_fragment":true,
  "domain_resolver":{"server":"hosts","timeout":"1s","disable_cache":true},
  "network_strategy":"fallback",
  "network_type":["wifi"],
  "fallback_network_type":["cellular"],
  "fallback_delay":"1s"
}`

	var created struct {
		ID         int64  `json:"id"`
		Type       string `json:"type"`
		Tag        string `json:"tag"`
		Server     string `json:"server"`
		ServerPort int    `json:"server_port"`
		Detour     string `json:"detour"`
		Raw        string `json:"raw"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes", map[string]string{"raw": createRaw}), &created)
	if created.Type != "http" || created.Tag != "⚪ Po0" || created.Server != "console.po0.com" || created.ServerPort != 443 {
		t.Fatalf("created node = %+v", created)
	}
	if created.Detour != "Proxy" || !strings.Contains(created.Raw, `"domain_resolver":{"server":"hosts","timeout":"1s","disable_cache":true}`) {
		t.Fatalf("created raw = %s", created.Raw)
	}
	for _, want := range []string{
		`"inet4_bind_address":"192.0.2.10"`,
		`"inet6_bind_address":"2001:db8::10"`,
		`"bind_address_no_port":true`,
		`"protect_path":"/run/sing-box/protect.sock"`,
		`"reuse_addr":true`,
		`"netns":"blue"`,
		`"tcp_fast_open":true`,
		`"tcp_multi_path":true`,
		`"disable_tcp_keep_alive":true`,
		`"tcp_keep_alive":"5m"`,
		`"tcp_keep_alive_interval":"75s"`,
	} {
		if !strings.Contains(created.Raw, want) {
			t.Fatalf("created raw missing %s: %s", want, created.Raw)
		}
	}

	var fetched struct {
		ID         int64  `json:"id"`
		ServerPort int    `json:"server_port"`
		Detour     string `json:"detour"`
		Raw        string `json:"raw"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/nodes/"+itoa(created.ID), nil), &fetched)
	if fetched.ServerPort != 443 || fetched.Detour != "Proxy" || !strings.Contains(fetched.Raw, `"routing_mark":"0x1234"`) {
		t.Fatalf("fetched node = %+v", fetched)
	}

	updateRaw := `{
  "type":"http",
  "tag":"⚪ Po0",
  "server":"console.po0.com",
  "server_port":443,
  "path":"/modules/servers/penguin/api/firewall.php?action=add",
  "username":"",
  "password":"",
  "detour":"Proxy",
  "bind_interface":"eth1",
  "routing_mark":"0x5678",
  "connect_timeout":"10s",
  "udp_fragment":false,
  "domain_resolver":"hosts",
  "network_strategy":"hybrid",
  "network_type":["wifi","ethernet"]
}`

	var updated struct {
		ID         int64  `json:"id"`
		ServerPort int    `json:"server_port"`
		Detour     string `json:"detour"`
		Raw        string `json:"raw"`
	}
	decodeData(t, c.do(http.MethodPut, "/api/nodes/"+itoa(created.ID), map[string]string{"raw": updateRaw}), &updated)
	if updated.ServerPort != 443 || updated.Detour != "Proxy" {
		t.Fatalf("updated node = %+v", updated)
	}
	for _, want := range []string{
		`"bind_interface":"eth1"`,
		`"routing_mark":"0x5678"`,
		`"connect_timeout":"10s"`,
		`"domain_resolver":"hosts"`,
		`"network_strategy":"hybrid"`,
		`"network_type":["wifi","ethernet"]`,
	} {
		if !strings.Contains(updated.Raw, want) {
			t.Fatalf("updated raw missing %s: %s", want, updated.Raw)
		}
	}
}

func TestImportAndExportAnyTLSLink(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	var imported struct {
		Imported int `json:"imported"`
		Nodes    []struct {
			ID         int64  `json:"id"`
			Type       string `json:"type"`
			ServerPort int    `json:"server_port"`
			Raw        string `json:"raw"`
		} `json:"nodes"`
	}
	link := "anytls://secret@any.example.com?security=tls&sni=any.example.com&idle_session_check_interval=20s#Any"
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/links", map[string]string{"links": link}), &imported)
	if imported.Imported != 1 || len(imported.Nodes) != 1 {
		t.Fatalf("imported = %+v, want one anytls node", imported)
	}
	if imported.Nodes[0].Type != "anytls" || imported.Nodes[0].ServerPort != 443 || !strings.Contains(imported.Nodes[0].Raw, `"type":"anytls"`) {
		t.Fatalf("imported node = %+v", imported.Nodes[0])
	}

	resp := c.do(http.MethodPost, "/api/nodes/export/links", map[string]any{"node_ids": []int64{imported.Nodes[0].ID}})
	if resp.StatusCode != http.StatusOK {
		status, code, msg := decodeError(t, resp)
		t.Fatalf("export status=%d code=%q msg=%q", status, code, msg)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if text := string(body); !strings.Contains(text, "anytls://secret@any.example.com:443") || !strings.Contains(text, "idle_session_check_interval=20s") {
		t.Fatalf("export text = %q", text)
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

func TestImportPreviewConfigDedupesAndDoesNotWriteNodes(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	node := `{"type":"shadowsocks","tag":"preview","server":"preview.example.com","server_port":8388,"method":"aes-256-gcm","password":"pw"}`
	direct := `{"type":"direct","tag":"Direct"}`

	var preview struct {
		Parsed     int              `json:"parsed"`
		Importable int              `json:"importable"`
		Deduped    int              `json:"deduped"`
		Nodes      []map[string]any `json:"nodes"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/config/preview", map[string]string{
		"config": `{"outbounds":[` + node + `,` + node + `,` + direct + `]}`,
	}), &preview)
	if preview.Parsed != 2 || preview.Importable != 1 || preview.Deduped != 1 {
		t.Fatalf("preview parsed/importable/deduped = %d/%d/%d, want 2/1/1", preview.Parsed, preview.Importable, preview.Deduped)
	}
	if len(preview.Nodes) != 1 || preview.Nodes[0]["tag"] != "preview" {
		t.Fatalf("preview nodes = %+v", preview.Nodes)
	}
	if _, ok := preview.Nodes[0]["raw"]; ok {
		t.Fatalf("preview node leaked raw: %+v", preview.Nodes[0])
	}

	var nodes []struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/nodes", nil), &nodes)
	if len(nodes) != 0 {
		t.Fatalf("preview wrote nodes: %+v", nodes)
	}

	var imported struct {
		Imported int `json:"imported"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/config", map[string]string{
		"config": `{"outbounds":[` + node + `]}`,
	}), &imported)
	if imported.Imported != 1 {
		t.Fatalf("imported = %d, want 1", imported.Imported)
	}

	var duplicatePreview struct {
		Parsed     int `json:"parsed"`
		Importable int `json:"importable"`
		Deduped    int `json:"deduped"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/config/preview", map[string]string{
		"config": `{"outbounds":[` + node + `]}`,
	}), &duplicatePreview)
	if duplicatePreview.Parsed != 1 || duplicatePreview.Importable != 0 || duplicatePreview.Deduped != 1 {
		t.Fatalf("duplicate preview parsed/importable/deduped = %d/%d/%d, want 1/0/1", duplicatePreview.Parsed, duplicatePreview.Importable, duplicatePreview.Deduped)
	}
}

func TestImportPreviewSubscriptionDoesNotCreateSource(t *testing.T) {
	srv, ts := testServer(t)
	srv.Fetcher.AllowPrivate = true
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	link := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw")) + "@1.1.1.1:8388#PreviewSub"
	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(link))
	}))
	t.Cleanup(sub.Close)

	var preview struct {
		Parsed     int `json:"parsed"`
		Importable int `json:"importable"`
		Deduped    int `json:"deduped"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/subscription/preview", map[string]string{
		"url": sub.URL,
	}), &preview)
	if preview.Parsed != 1 || preview.Importable != 1 || preview.Deduped != 0 {
		t.Fatalf("subscription preview parsed/importable/deduped = %d/%d/%d, want 1/1/0", preview.Parsed, preview.Importable, preview.Deduped)
	}

	var sources []struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/sources", nil), &sources)
	if len(sources) != 0 {
		t.Fatalf("preview created sources: %+v", sources)
	}
	var nodes []struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/nodes", nil), &nodes)
	if len(nodes) != 0 {
		t.Fatalf("preview wrote nodes: %+v", nodes)
	}
}

func TestImportPreviewSubscriptionParsesMihomoAndSurge(t *testing.T) {
	srv, ts := testServer(t)
	srv.Fetcher.AllowPrivate = true
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)
	psk := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mihomo":
			_, _ = w.Write([]byte("proxies:\n  - name: Mihomo Snell\n    type: snell\n    server: mihomo.example.com\n    port: 17851\n    psk: " + psk + "\n    version: 6\n"))
		case "/surge":
			_, _ = w.Write([]byte("[Proxy]\nSurge Snell = snell, surge.example.com, 17851, psk=" + psk + ", version=6\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(sub.Close)
	for _, tc := range []struct {
		path string
		tag  string
	}{
		{path: "/mihomo", tag: "Mihomo Snell"},
		{path: "/surge", tag: "Surge Snell"},
	} {
		var preview struct {
			Parsed     int `json:"parsed"`
			Importable int `json:"importable"`
			Nodes      []struct {
				Type string `json:"type"`
				Tag  string `json:"tag"`
			} `json:"nodes"`
		}
		decodeData(t, c.do(http.MethodPost, "/api/nodes/import/subscription/preview", map[string]string{
			"url": sub.URL + tc.path,
		}), &preview)
		if preview.Parsed != 1 || preview.Importable != 1 || len(preview.Nodes) != 1 || preview.Nodes[0].Type != "snell" || preview.Nodes[0].Tag != tc.tag {
			t.Fatalf("%s preview = %+v", tc.path, preview)
		}
	}
}

func TestImportPreviewSubscriptionMultiURLPartialSuccess(t *testing.T) {
	srv, ts := testServer(t)
	srv.Fetcher.AllowPrivate = true
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad":
			http.Error(w, "bad", http.StatusBadGateway)
		case "/ok":
			_, _ = w.Write([]byte(testSSLink("partial-ok", "1.1.1.1")))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(sub.Close)

	var preview struct {
		Parsed     int `json:"parsed"`
		Importable int `json:"importable"`
		Deduped    int `json:"deduped"`
		Fetches    []struct {
			URL   string `json:"url"`
			OK    bool   `json:"ok"`
			Nodes int    `json:"nodes"`
			Error string `json:"error"`
		} `json:"fetches"`
		Warnings []string `json:"warnings"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/subscription/preview", map[string]string{
		"url": sub.URL + "/bad\n" + sub.URL + "/ok#noCache",
	}), &preview)
	if preview.Parsed != 1 || preview.Importable != 1 || preview.Deduped != 0 {
		t.Fatalf("preview parsed/importable/deduped = %d/%d/%d, want 1/1/0", preview.Parsed, preview.Importable, preview.Deduped)
	}
	if len(preview.Fetches) != 2 || preview.Fetches[0].OK || !preview.Fetches[1].OK || preview.Fetches[1].Nodes != 1 {
		t.Fatalf("fetches = %+v", preview.Fetches)
	}
	if len(preview.Warnings) == 0 || !strings.Contains(preview.Warnings[0], "status 502") {
		t.Fatalf("warnings = %+v", preview.Warnings)
	}
	if strings.Contains(preview.Fetches[1].URL, "noCache") {
		t.Fatalf("fetch URL leaked fragment options: %+v", preview.Fetches)
	}

	var sources []struct {
		ID int64 `json:"id"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/sources", nil), &sources)
	if len(sources) != 0 {
		t.Fatalf("preview created sources: %+v", sources)
	}
}

func TestImportSubscriptionMultiURLStoresSourceAndWarnings(t *testing.T) {
	srv, ts := testServer(t)
	srv.Fetcher.AllowPrivate = true
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad":
			http.Error(w, "bad", http.StatusBadGateway)
		case "/ok":
			_, _ = w.Write([]byte(testSSLink("stored-ok", "2.2.2.2")))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(sub.Close)
	rawURL := sub.URL + "/bad\n" + sub.URL + "/ok"

	var imported struct {
		Imported int   `json:"imported"`
		SourceID int64 `json:"source_id"`
		Fetches  []struct {
			OK    bool `json:"ok"`
			Nodes int  `json:"nodes"`
		} `json:"fetches"`
		Warnings []string `json:"warnings"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/subscription", map[string]string{
		"name": "multi", "url": rawURL,
	}), &imported)
	if imported.Imported != 1 || imported.SourceID == 0 {
		t.Fatalf("imported = %+v, want one node and source id", imported)
	}
	if len(imported.Fetches) != 2 || imported.Fetches[0].OK || !imported.Fetches[1].OK || imported.Fetches[1].Nodes != 1 {
		t.Fatalf("fetches = %+v", imported.Fetches)
	}
	if len(imported.Warnings) == 0 {
		t.Fatalf("warnings = %+v, want partial failure warning", imported.Warnings)
	}

	var sources []struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		URL        string `json:"url"`
		LastStatus string `json:"last_status"`
		NodeCount  int    `json:"node_count"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/sources", nil), &sources)
	if len(sources) != 1 || sources[0].ID != imported.SourceID || sources[0].URL != rawURL {
		t.Fatalf("sources = %+v, want stored multi-line URL", sources)
	}
	if sources[0].LastStatus != "ok with warnings" || sources[0].NodeCount != 1 {
		t.Fatalf("source status/count = %q/%d", sources[0].LastStatus, sources[0].NodeCount)
	}

	var nodes []struct {
		Tag       string `json:"tag"`
		SourceRef *int64 `json:"source_ref"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/nodes", nil), &nodes)
	if len(nodes) != 1 || nodes[0].Tag != "stored-ok" || nodes[0].SourceRef == nil || *nodes[0].SourceRef != imported.SourceID {
		t.Fatalf("nodes = %+v", nodes)
	}
}

func TestRefreshSubscriptionMultiURLReplacesNodesWithPartialSuccess(t *testing.T) {
	srv, ts := testServer(t)
	srv.Fetcher.AllowPrivate = true
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	var phase int32
	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&phase) == 0 {
			if r.URL.Path == "/a" {
				_, _ = w.Write([]byte(testSSLink("initial-a", "3.3.3.3")))
				return
			}
			_, _ = w.Write([]byte(testSSLink("initial-b", "4.4.4.4")))
			return
		}
		if r.URL.Path == "/a" {
			http.Error(w, "bad", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(testSSLink("refreshed-b", "5.5.5.5")))
	}))
	t.Cleanup(sub.Close)
	rawURL := sub.URL + "/a\n" + sub.URL + "/b"

	var imported struct {
		Imported int   `json:"imported"`
		SourceID int64 `json:"source_id"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/subscription", map[string]string{
		"name": "refresh", "url": rawURL,
	}), &imported)
	if imported.Imported != 2 || imported.SourceID == 0 {
		t.Fatalf("initial import = %+v", imported)
	}

	atomic.StoreInt32(&phase, 1)
	var refreshed struct {
		Imported int `json:"imported"`
		Fetches  []struct {
			OK    bool `json:"ok"`
			Nodes int  `json:"nodes"`
		} `json:"fetches"`
		Warnings []string `json:"warnings"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/sources/"+itoa(imported.SourceID)+"/refresh", nil), &refreshed)
	if refreshed.Imported != 1 || len(refreshed.Warnings) == 0 {
		t.Fatalf("refresh result = %+v", refreshed)
	}
	if len(refreshed.Fetches) != 2 || refreshed.Fetches[0].OK || !refreshed.Fetches[1].OK || refreshed.Fetches[1].Nodes != 1 {
		t.Fatalf("refresh fetches = %+v", refreshed.Fetches)
	}

	var nodes []struct {
		Tag string `json:"tag"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/nodes", nil), &nodes)
	if len(nodes) != 1 || nodes[0].Tag != "refreshed-b" {
		t.Fatalf("nodes after refresh = %+v", nodes)
	}

	var sources []struct {
		ID         int64  `json:"id"`
		LastStatus string `json:"last_status"`
		NodeCount  int    `json:"node_count"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/sources", nil), &sources)
	if len(sources) != 1 || sources[0].ID != imported.SourceID || sources[0].LastStatus != "ok with warnings" || sources[0].NodeCount != 1 {
		t.Fatalf("sources after refresh = %+v", sources)
	}
}

func TestImportPreviewSubscriptionAllURLsFail(t *testing.T) {
	srv, ts := testServer(t)
	srv.Fetcher.AllowPrivate = true
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/html" {
			_, _ = w.Write([]byte("<!DOCTYPE html><html><body>login</body></html>"))
			return
		}
		http.Error(w, "bad", http.StatusBadGateway)
	}))
	t.Cleanup(sub.Close)

	status, code, msg := decodeError(t, c.do(http.MethodPost, "/api/nodes/import/subscription/preview", map[string]string{
		"url": sub.URL + "/bad\n" + sub.URL + "/html",
	}))
	if status != http.StatusBadGateway || code != "fetch_error" {
		t.Fatalf("status=%d code=%q msg=%q, want 502 fetch_error", status, code, msg)
	}
	if !strings.Contains(msg, "status 502") || !strings.Contains(msg, "html") {
		t.Fatalf("all-fail message = %q", msg)
	}
}

func TestImportPreviewRespectsNodeQuota(t *testing.T) {
	_, ts := testServer(t)
	admin := newClient(t, ts.URL)
	admin.http.Jar = login(t, ts.URL)

	decodeData(t, admin.do(http.MethodPost, "/api/users", map[string]any{
		"username": "preview-quota", "password": "password123", "node_limit": 1,
	}), nil)

	user := newClient(t, ts.URL)
	user.http.Jar = loginAs(t, ts.URL, "preview-quota", "password123")
	decodeData(t, user.do(http.MethodPost, "/api/nodes", map[string]string{
		"raw": `{"type":"shadowsocks","tag":"existing","server":"one.example.com","server_port":8388,"method":"aes-256-gcm","password":"pw"}`,
	}), nil)

	status, code, _ := decodeError(t, user.do(http.MethodPost, "/api/nodes/import/config/preview", map[string]string{
		"config": `{"outbounds":[{"type":"shadowsocks","tag":"new","server":"two.example.com","server_port":8388,"method":"aes-256-gcm","password":"pw"}]}`,
	}))
	if status != http.StatusForbidden || code != "quota_exceeded" {
		t.Fatalf("quota preview status=%d code=%q, want 403 quota_exceeded", status, code)
	}
}

func testSSLink(tag, host string) string {
	return "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw")) + "@" + host + ":8388#" + tag
}
