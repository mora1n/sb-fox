package store

import (
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
	n := &models.Node{
		Tag: "🇭🇰 HK-1", Type: "shadowsocks", Server: "hk.example.com", ServerPort: 8388,
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
	list, err := s.ListNodes(NodeFilter{CountryCode: "HK"})
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
	// need a template (FK) and nodes
	tid, err := s.CreateTemplate(&models.Template{Name: "t1", Kind: "user", Content: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	var nodeIDs []int64
	for i := 0; i < 3; i++ {
		id, err := s.CreateNode(&models.Node{Tag: "n", Type: "vmess", Source: "manual", Raw: "{}"})
		if err != nil {
			t.Fatal(err)
		}
		nodeIDs = append(nodeIDs, id)
	}
	p := &models.Profile{Name: "prof1", TemplateID: tid, Options: "{}", Token: "tok123", NodeIDs: []int64{nodeIDs[2], nodeIDs[0], nodeIDs[1]}}
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
	// lookup by token
	byTok, err := s.GetProfileByToken("tok123")
	if err != nil || byTok.ID != pid {
		t.Fatalf("by token: %v", err)
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

func TestSeedUserTemplate(t *testing.T) {
	s := openTest(t)
	inserted, err := s.SeedUserTemplate("fakeip", `{"a":1}`, "desc")
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("first seed did not insert")
	}
	// Re-seeding never overwrites a normal template.
	inserted, err = s.SeedUserTemplate("fakeip", `{"a":2}`, "desc2")
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
