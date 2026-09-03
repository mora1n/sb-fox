package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mora1n/sb-fox/internal/models"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func createTestUser(t *testing.T, s *Store) int64 {
	t.Helper()
	id, err := s.CreateUser(&models.User{Username: "u", PasswordHash: "hash", Role: models.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestMigrateIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("open1: %v", err)
	}
	s1.Close()
	// second open must not re-apply migrations or error
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("open2: %v", err)
	}
	defer s2.Close()
	applied, err := s2.appliedVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != len(migrations)-1 {
		t.Errorf("applied %d migrations, want %d", len(applied), len(migrations)-1)
	}
}

func TestMigrateSingleAdminDataToUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(migrations[0]); err != nil {
		t.Fatal(err)
	}
	const multiUserMigrationIndex = 10
	for i := 1; i < multiUserMigrationIndex; i++ {
		if _, err := db.Exec(migrations[i]); err != nil {
			t.Fatalf("old migration %d: %v", i, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`, i, now()); err != nil {
			t.Fatal(err)
		}
	}
	ts := now()
	if _, err := db.Exec(`INSERT INTO admin (id, username, password_hash, created_at, updated_at) VALUES (1, 'admin', 'hash', ?, ?)`, ts, ts); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO templates (id, name, kind, content, description, created_at, updated_at) VALUES (1, 'fakeip', 'user', '{}', '', ?, ?)`, ts, ts); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO subscription_sources (id, name, url, created_at) VALUES (1, 'src', 'https://example.com/sub', ?)`, ts); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO nodes (id, tag, type, source, source_ref, raw, created_at, updated_at) VALUES (1, 'n', 'vmess', 'subscription', 1, '{}', ?, ?)`, ts, ts); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO profiles (id, name, template_id, token, created_at, updated_at) VALUES (1, 'p', 1, 'tok', ?, ?)`, ts, ts); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO profile_nodes (profile_id, node_id, position) VALUES (1, 1, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	admin, err := s.FirstAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if admin.ID != 1 || admin.Username != "admin" {
		t.Fatalf("admin = %+v", admin)
	}
	if admin.SubscriptionToken == "" {
		t.Fatal("admin subscription token was not backfilled")
	}
	profiles, err := s.ListProfiles(1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].OwnerUserID != 1 || len(profiles[0].NodeIDs) != 1 {
		t.Fatalf("profiles = %+v", profiles)
	}
	nodes, err := s.ListNodes(NodeFilter{OwnerUserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].OwnerUserID != 1 {
		t.Fatalf("nodes = %+v", nodes)
	}
}

func TestAdminRoundTrip(t *testing.T) {
	s := openTest(t)
	if ok, _ := s.AdminExists(); ok {
		t.Fatal("admin should not exist initially")
	}
	if err := s.SetAdmin("admin", "hash1"); err != nil {
		t.Fatal(err)
	}
	a, err := s.GetAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if a.Username != "admin" || a.PasswordHash != "hash1" {
		t.Errorf("got %+v", a)
	}
	// update
	if err := s.SetAdmin("root", "hash2"); err != nil {
		t.Fatal(err)
	}
	a, _ = s.GetAdmin()
	if a.Username != "root" || a.PasswordHash != "hash2" {
		t.Errorf("update failed: %+v", a)
	}
}

func TestNodeCRUD(t *testing.T) {
	s := openTest(t)
	ownerID := createTestUser(t, s)
	n := &models.Node{
		OwnerUserID: ownerID,
		Tag:         "🇭🇰 HK-1", Type: "shadowsocks", Server: "hk.example.com", ServerPort: 8388,
		CountryCode: "HK", CountrySource: "auto", Source: "manual",
		Raw: `{"type":"shadowsocks","tag":"🇭🇰 HK-1"}`,
	}
	id, err := s.CreateNode(n)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetNode(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != n.Tag || got.CountryCode != "HK" {
		t.Errorf("got %+v", got)
	}
	// filter by country
	list, err := s.ListNodes(NodeFilter{OwnerUserID: ownerID, CountryCode: "HK"})
	if err != nil || len(list) != 1 {
		t.Fatalf("filter: %v len=%d", err, len(list))
	}
	// update
	got.CountryCode = "JP"
	got.CountrySource = "manual"
	if err := s.UpdateNode(got); err != nil {
		t.Fatal(err)
	}
	after, _ := s.GetNode(id)
	if after.CountryCode != "JP" {
		t.Errorf("update failed: %+v", after)
	}
	// delete
	if err := s.DeleteNode(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetNode(id); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestProfileWithNodes(t *testing.T) {
	s := openTest(t)
	ownerID := createTestUser(t, s)
	// need a template (FK) and nodes
	tid, err := s.CreateTemplate(&models.Template{OwnerUserID: ownerID, Name: "t1", Kind: "user", Content: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	var nodeIDs []int64
	for i := 0; i < 3; i++ {
		id, err := s.CreateNode(&models.Node{OwnerUserID: ownerID, Tag: "n", Type: "vmess", Source: "manual", Raw: "{}"})
		if err != nil {
			t.Fatal(err)
		}
		nodeIDs = append(nodeIDs, id)
	}
	p := &models.Profile{OwnerUserID: ownerID, Name: "prof1", TemplateID: tid, Options: "{}", Token: "tok123", NodeIDs: []int64{nodeIDs[2], nodeIDs[0], nodeIDs[1]}}
	pid, err := s.CreateProfile(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetProfile(pid)
	if err != nil {
		t.Fatal(err)
	}
	// order must be preserved
	if len(got.NodeIDs) != 3 || got.NodeIDs[0] != nodeIDs[2] || got.NodeIDs[2] != nodeIDs[1] {
		t.Errorf("node order not preserved: %v", got.NodeIDs)
	}
	if !got.SubEnabled {
		t.Fatal("profile subscription should be enabled by default")
	}
	if err := s.SetProfileSubscriptionEnabled(pid, ownerID, false); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetProfile(pid)
	if err != nil {
		t.Fatal(err)
	}
	if got.SubEnabled {
		t.Fatal("profile subscription switch was not persisted")
	}
	byName, err := s.GetProfileByNameForUser("prof1", ownerID)
	if err != nil || byName.ID != pid {
		t.Fatalf("by name: %v", err)
	}
	if byName.SubEnabled {
		t.Fatal("profile by name did not return persisted subscription switch")
	}
	disabledID, err := s.CreateProfile(&models.Profile{
		OwnerUserID:   ownerID,
		Name:          "prof-disabled",
		TemplateID:    tid,
		Options:       "{}",
		Token:         "tok-disabled",
		SubEnabled:    false,
		SubEnabledSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := s.GetProfile(disabledID)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.SubEnabled {
		t.Fatal("explicitly disabled profile subscription should stay disabled")
	}
	u, err := s.GetUser(ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if u.SubscriptionToken == "" {
		t.Fatal("user subscription token is empty")
	}
	byToken, err := s.GetUserBySubscriptionToken(u.SubscriptionToken)
	if err != nil || byToken.ID != ownerID {
		t.Fatalf("user by subscription token: %v", err)
	}
	// deleting a node cascades from profile_nodes
	if err := s.DeleteNode(nodeIDs[0]); err != nil {
		t.Fatal(err)
	}
	after, _ := s.GetProfile(pid)
	if len(after.NodeIDs) != 2 {
		t.Errorf("cascade failed, node ids: %v", after.NodeIDs)
	}
}

func TestNodeGroupAndProfileMembership(t *testing.T) {
	s := openTest(t)
	ownerID := createTestUser(t, s)
	tid, err := s.CreateTemplate(&models.Template{OwnerUserID: ownerID, Name: "t1", Kind: "user", Content: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	var nodeIDs []int64
	for i := 0; i < 3; i++ {
		id, err := s.CreateNode(&models.Node{OwnerUserID: ownerID, Tag: "n", Type: "vmess", Source: "manual", Raw: "{}"})
		if err != nil {
			t.Fatal(err)
		}
		nodeIDs = append(nodeIDs, id)
	}
	gid, err := s.CreateNodeGroup(&models.NodeGroup{
		OwnerUserID: ownerID,
		Name:        "g1",
		NodeIDs:     []int64{nodeIDs[2], nodeIDs[0]},
	})
	if err != nil {
		t.Fatal(err)
	}
	group, err := s.GetNodeGroupForUser(gid, ownerID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(group.NodeIDs) != 2 || group.NodeIDs[0] != nodeIDs[2] || group.NodeIDs[1] != nodeIDs[0] {
		t.Fatalf("group node order = %v", group.NodeIDs)
	}

	pid, err := s.CreateProfile(&models.Profile{
		OwnerUserID:  ownerID,
		Name:         "p1",
		TemplateID:   tid,
		Options:      fmt.Sprintf(`{"groupSelections":{"Proxy":{"nodeGroupIds":[%d]}}}`, gid),
		Token:        "tok-groups",
		NodeIDs:      []int64{nodeIDs[1]},
		NodeGroupIDs: []int64{gid},
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := s.GetProfile(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.NodeGroupIDs) != 1 || profile.NodeGroupIDs[0] != gid {
		t.Fatalf("profile groups = %v", profile.NodeGroupIDs)
	}
	if err := s.DeleteNodeGroup(gid); err != nil {
		t.Fatal(err)
	}
	profile, err = s.GetProfile(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.NodeGroupIDs) != 0 {
		t.Fatalf("group cascade failed: %v", profile.NodeGroupIDs)
	}
	var options models.ProfileOptions
	if err := json.Unmarshal([]byte(profile.Options), &options); err != nil {
		t.Fatal(err)
	}
	if len(options.GroupSelections["Proxy"].NodeGroupIDs) != 0 {
		t.Fatalf("profile option group refs = %v", options.GroupSelections["Proxy"].NodeGroupIDs)
	}
}

func TestListNodeUsageDirectAndViaGroup(t *testing.T) {
	s := openTest(t)
	ownerID := createTestUser(t, s)
	otherOwnerID, err := s.CreateUser(&models.User{Username: "other", PasswordHash: "hash", Role: models.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	tid, err := s.CreateTemplate(&models.Template{OwnerUserID: ownerID, Name: "t1", Kind: "user", Content: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	otherTid, err := s.CreateTemplate(&models.Template{OwnerUserID: otherOwnerID, Name: "t2", Kind: "user", Content: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	nid, err := s.CreateNode(&models.Node{OwnerUserID: ownerID, Tag: "n1", Type: "vmess", Source: "manual", Raw: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	otherNID, err := s.CreateNode(&models.Node{OwnerUserID: otherOwnerID, Tag: "n2", Type: "vmess", Source: "manual", Raw: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	gid, err := s.CreateNodeGroup(&models.NodeGroup{
		OwnerUserID: ownerID,
		Name:        "g1",
		NodeIDs:     []int64{nid},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(&models.Profile{
		OwnerUserID: ownerID,
		Name:        "direct",
		TemplateID:  tid,
		Options:     "{}",
		Token:       "tok-direct",
		NodeIDs:     []int64{nid},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(&models.Profile{
		OwnerUserID:  ownerID,
		Name:         "grouped",
		TemplateID:   tid,
		Options:      "{}",
		Token:        "tok-grouped",
		NodeGroupIDs: []int64{gid},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(&models.Profile{
		OwnerUserID: otherOwnerID,
		Name:        "other",
		TemplateID:  otherTid,
		Options:     "{}",
		Token:       "tok-other",
		NodeIDs:     []int64{otherNID},
	}); err != nil {
		t.Fatal(err)
	}

	usage, err := s.ListNodeUsage(nid, ownerID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 2 {
		t.Fatalf("usage len = %d, usage = %+v", len(usage), usage)
	}
	byProfile := map[string]*models.NodeUsage{}
	for _, u := range usage {
		byProfile[u.ProfileName] = u
	}
	if byProfile["direct"] == nil || byProfile["direct"].ViaGroupID != 0 {
		t.Fatalf("direct usage = %+v", byProfile["direct"])
	}
	if byProfile["grouped"] == nil || byProfile["grouped"].ViaGroupName != "g1" {
		t.Fatalf("group usage = %+v", byProfile["grouped"])
	}

	all, err := s.ListNodeUsage(otherNID, ownerID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ProfileName != "other" {
		t.Fatalf("all owner usage = %+v", all)
	}
}

func TestListProfilesWithNodesDoesNotNestOpenRows(t *testing.T) {
	s := openTest(t)
	ownerID := createTestUser(t, s)
	tid, err := s.CreateTemplate(&models.Template{OwnerUserID: ownerID, Name: "t1", Kind: "user", Content: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	nid, err := s.CreateNode(&models.Node{OwnerUserID: ownerID, Tag: "n", Type: "vmess", Source: "manual", Raw: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(&models.Profile{
		OwnerUserID: ownerID,
		Name:        "prof1",
		TemplateID:  tid,
		Options:     "{}",
		Token:       "tok-list",
		NodeIDs:     []int64{nid},
	}); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListProfiles(ownerID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || len(list[0].NodeIDs) != 1 || list[0].NodeIDs[0] != nid {
		t.Fatalf("profiles = %+v", list)
	}
}

func TestSeedUserTemplate(t *testing.T) {
	s := openTest(t)
	ownerID := createTestUser(t, s)
	inserted, err := s.SeedUserTemplate(ownerID, "fakeip", `{"a":1}`, "desc")
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("first seed did not insert")
	}
	// Re-seeding never overwrites a normal template.
	inserted, err = s.SeedUserTemplate(ownerID, "fakeip", `{"a":2}`, "desc2")
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("second seed should not insert")
	}
	got, err := s.GetTemplateByName("fakeip")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != `{"a":1}` || got.Kind != "user" {
		t.Errorf("got %+v", got)
	}
}
