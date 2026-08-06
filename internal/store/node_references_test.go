package store

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mora1n/sb-fox/internal/models"
)

func TestRemoveNodeIDsFromProfileOptions(t *testing.T) {
	input := `{
		"autoCountryGroups":true,
		"chainProxy":true,
		"chainProxyNodeId":2,
		"chainProxyNodeIds":[1,2,3],
		"groupSelections":{"Proxy":{"nodeIds":[3,2,1],"nodeGroupIds":[9],"outboundRefs":["Direct"],"skipCountryGroups":true,"custom":"keep"}},
		"autoCountrySelection":{"nodeIds":[2,4],"nodeGroupIds":[10]},
		"chainProxySelection":{"nodeIds":[3],"nodeGroupIds":[11]},
		"extra":null
	}`
	got, changed, err := removeNodeIDsFromProfileOptions(input, map[int64]bool{2: true, 3: true})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected options to change")
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got), &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["chainProxyNodeId"]; ok {
		t.Fatal("legacy chainProxyNodeId was not removed")
	}
	assertRawNodeIDs(t, root["chainProxyNodeIds"], []int64{1})
	if string(root["extra"]) != "null" {
		t.Fatalf("extra field = %s, want null", root["extra"])
	}

	var groups map[string]json.RawMessage
	if err := json.Unmarshal(root["groupSelections"], &groups); err != nil {
		t.Fatal(err)
	}
	var proxy map[string]json.RawMessage
	if err := json.Unmarshal(groups["Proxy"], &proxy); err != nil {
		t.Fatal(err)
	}
	assertRawNodeIDs(t, proxy["nodeIds"], []int64{1})
	if string(proxy["custom"]) != `"keep"` || string(proxy["nodeGroupIds"]) != "[9]" || string(proxy["outboundRefs"]) != `["Direct"]` {
		t.Fatalf("unrelated selection fields changed: %s", groups["Proxy"])
	}
	assertSelectionNodeIDs(t, root["autoCountrySelection"], []int64{4})
	assertSelectionNodeIDs(t, root["chainProxySelection"], []int64{})

	unchanged, changed, err := removeNodeIDsFromProfileOptions(input, map[int64]bool{99: true})
	if err != nil || changed || unchanged != input {
		t.Fatalf("unchanged options = %q changed=%v err=%v", unchanged, changed, err)
	}
	if _, _, err := removeNodeIDsFromProfileOptions(`{"groupSelections":{"Proxy":{"nodeIds":"bad"}}}`, map[int64]bool{1: true}); err == nil {
		t.Fatal("invalid nodeIds should return an error")
	}
}

func TestDeleteNodeCleansRelationalAndOptionReferences(t *testing.T) {
	s := openTest(t)
	ownerID := createTestUser(t, s)
	templateID, err := s.CreateTemplate(&models.Template{OwnerUserID: ownerID, Name: "t", Kind: "user", Content: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	targetID := createReferenceTestNode(t, s, ownerID, "target")
	keepID := createReferenceTestNode(t, s, ownerID, "keep")
	groupID, err := s.CreateNodeGroup(&models.NodeGroup{OwnerUserID: ownerID, Name: "g", NodeIDs: []int64{targetID, keepID}})
	if err != nil {
		t.Fatal(err)
	}
	options := fmt.Sprintf(`{"autoCountryGroups":false,"chainProxy":true,"chainProxyNodeId":%d,"chainProxyNodeIds":[%d,%d],"groupSelections":{"Proxy":{"nodeIds":[%d,%d],"nodeGroupIds":[%d]}},"autoCountrySelection":{"nodeIds":[%d,%d]},"chainProxySelection":{"nodeIds":[%d,%d]}}`,
		targetID, targetID, keepID, targetID, keepID, groupID, targetID, keepID, targetID, keepID)
	profileID, err := s.CreateProfile(&models.Profile{
		OwnerUserID: ownerID,
		Name:        "p",
		TemplateID:  templateID,
		Options:     options,
		Token:       "tok-clean",
		NodeIDs:     []int64{targetID, keepID},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteNodeForUser(targetID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetNode(targetID); err != ErrNotFound {
		t.Fatalf("deleted node error = %v, want ErrNotFound", err)
	}
	group, err := s.GetNodeGroupForUser(groupID, ownerID, false)
	if err != nil || len(group.NodeIDs) != 1 || group.NodeIDs[0] != keepID {
		t.Fatalf("group after delete = %+v err=%v", group, err)
	}
	profile, err := s.GetProfile(profileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.NodeIDs) != 1 || profile.NodeIDs[0] != keepID {
		t.Fatalf("profile relational nodes = %v", profile.NodeIDs)
	}
	assertProfileOptionNodeIDs(t, profile.Options, keepID)
}

func TestBulkDeleteNodesCleansProfilesForEachOwner(t *testing.T) {
	s := openTest(t)
	ownerIDs := []int64{createTestUser(t, s)}
	secondOwnerID, err := s.CreateUser(&models.User{Username: "u2", PasswordHash: "hash", Role: models.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	ownerIDs = append(ownerIDs, secondOwnerID)

	var nodeIDs []int64
	var profileIDs []int64
	for i, ownerID := range ownerIDs {
		templateID, err := s.CreateTemplate(&models.Template{OwnerUserID: ownerID, Name: "t", Kind: "user", Content: "{}"})
		if err != nil {
			t.Fatal(err)
		}
		nodeID := createReferenceTestNode(t, s, ownerID, fmt.Sprintf("n%d", i))
		nodeIDs = append(nodeIDs, nodeID)
		profileID, err := s.CreateProfile(&models.Profile{
			OwnerUserID: ownerID,
			Name:        "p",
			TemplateID:  templateID,
			Options:     fmt.Sprintf(`{"autoCountryGroups":false,"groupSelections":{"Proxy":{"nodeIds":[%d]}}}`, nodeID),
			Token:       fmt.Sprintf("tok-%d", i),
		})
		if err != nil {
			t.Fatal(err)
		}
		profileIDs = append(profileIDs, profileID)
	}

	deleted, err := s.DeleteNodesByIDs(nodeIDs)
	if err != nil || deleted != len(nodeIDs) {
		t.Fatalf("bulk delete = %d err=%v", deleted, err)
	}
	for _, profileID := range profileIDs {
		profile, err := s.GetProfile(profileID)
		if err != nil {
			t.Fatal(err)
		}
		var options models.ProfileOptions
		if err := json.Unmarshal([]byte(profile.Options), &options); err != nil {
			t.Fatal(err)
		}
		if got := options.GroupSelections["Proxy"].NodeIDs; len(got) != 0 {
			t.Fatalf("profile %d node ids = %v", profileID, got)
		}
	}
}

func TestDeleteNodeRollsBackOnInvalidProfileOptions(t *testing.T) {
	s := openTest(t)
	ownerID := createTestUser(t, s)
	templateID, err := s.CreateTemplate(&models.Template{OwnerUserID: ownerID, Name: "t", Kind: "user", Content: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	nodeID := createReferenceTestNode(t, s, ownerID, "target")
	groupID, err := s.CreateNodeGroup(&models.NodeGroup{OwnerUserID: ownerID, Name: "g", NodeIDs: []int64{nodeID}})
	if err != nil {
		t.Fatal(err)
	}
	profileID, err := s.CreateProfile(&models.Profile{
		OwnerUserID: ownerID,
		Name:        "broken",
		TemplateID:  templateID,
		Options:     `{"groupSelections":{"Proxy":{"nodeIds":"bad"}}}`,
		Token:       "tok-broken",
		NodeIDs:     []int64{nodeID},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteNodeForUser(nodeID, ownerID); err == nil {
		t.Fatal("delete should fail for invalid profile options")
	}
	if _, err := s.GetNode(nodeID); err != nil {
		t.Fatalf("node should remain after rollback: %v", err)
	}
	group, err := s.GetNodeGroupForUser(groupID, ownerID, false)
	if err != nil || len(group.NodeIDs) != 1 || group.NodeIDs[0] != nodeID {
		t.Fatalf("group after rollback = %+v err=%v", group, err)
	}
	profile, err := s.GetProfile(profileID)
	if err != nil || len(profile.NodeIDs) != 1 || profile.NodeIDs[0] != nodeID {
		t.Fatalf("profile after rollback = %+v err=%v", profile, err)
	}
}

func createReferenceTestNode(t *testing.T, s *Store, ownerID int64, tag string) int64 {
	t.Helper()
	id, err := s.CreateNode(&models.Node{OwnerUserID: ownerID, Tag: tag, Type: "shadowsocks", Source: "manual", Raw: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func assertProfileOptionNodeIDs(t *testing.T, blob string, want int64) {
	t.Helper()
	var options models.ProfileOptions
	if err := json.Unmarshal([]byte(blob), &options); err != nil {
		t.Fatal(err)
	}
	if options.ChainProxyNodeID != 0 || len(options.ChainProxyNodeIDs) != 1 || options.ChainProxyNodeIDs[0] != want {
		t.Fatalf("chain options = %+v", options)
	}
	for name, ids := range map[string][]int64{
		"group":   options.GroupSelections["Proxy"].NodeIDs,
		"country": options.AutoCountrySelected.NodeIDs,
		"chain":   options.ChainProxySelected.NodeIDs,
	} {
		if len(ids) != 1 || ids[0] != want {
			t.Fatalf("%s node ids = %v, want [%d]", name, ids, want)
		}
	}
}

func assertSelectionNodeIDs(t *testing.T, raw json.RawMessage, want []int64) {
	t.Helper()
	var selection map[string]json.RawMessage
	if err := json.Unmarshal(raw, &selection); err != nil {
		t.Fatal(err)
	}
	assertRawNodeIDs(t, selection["nodeIds"], want)
}

func assertRawNodeIDs(t *testing.T, raw json.RawMessage, want []int64) {
	t.Helper()
	var got []int64
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("node ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("node ids = %v, want %v", got, want)
		}
	}
}
