package store

import (
	"bytes"
	"testing"

	"github.com/mora1n/sb-fox/internal/models"
)

func TestRuleSetCRUDPublishesArtifactsAndKeepsSourceOrder(t *testing.T) {
	s := openTest(t)
	ownerID := createTestUser(t, s)
	user, err := s.GetUser(ownerID)
	if err != nil {
		t.Fatal(err)
	}
	item := &models.RuleSet{
		OwnerUserID:   ownerID,
		Name:          "telegram",
		Description:   "rules",
		PublishedJSON: []byte(`{"version":4,"rules":[]}`),
		PublishedSRS:  []byte{0, 1, 2, 3},
		RuleCount:     2,
		JSONSize:      24,
		SRSSize:       4,
		JSONSHA256:    "json-hash",
		SRSSHA256:     "srs-hash",
		KernelVersion: "sing-box version 1.13.14",
		Sources: []*models.RuleSetSource{
			{Kind: "manual", Format: "source", Content: `{"version":4,"rules":[]}`},
			{Kind: "remote", Format: "binary", URL: "https://example.com/rules.srs"},
		},
	}
	id, err := s.CreateRuleSet(item)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRuleSetForUser(id, ownerID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 2 || got.Sources[0].Kind != "manual" || got.Sources[1].Format != "binary" {
		t.Fatalf("sources = %+v", got.Sources)
	}
	if !bytes.Equal(got.PublishedSRS, item.PublishedSRS) || got.RuleCount != 2 {
		t.Fatalf("artifact = %+v", got)
	}
	public, err := s.GetRuleSetByNameAndToken(item.Name, user.SubscriptionToken)
	if err != nil || public.ID != id {
		t.Fatalf("public lookup = %+v err=%v", public, err)
	}

	list, err := s.ListRuleSets(ownerID, false)
	if err != nil || len(list) != 1 || list[0].SourceCount != 2 || list[0].PublishedJSON != nil {
		t.Fatalf("summary = %+v err=%v", list, err)
	}

	got.Name = "telegram-new"
	got.PublishedSRS = []byte{4, 5}
	got.SRSSize = 2
	got.Sources = got.Sources[:1]
	if err := s.UpdateRuleSet(got); err != nil {
		t.Fatal(err)
	}
	updated, err := s.GetRuleSetForUser(id, ownerID, false)
	if err != nil || updated.Name != "telegram-new" || len(updated.Sources) != 1 {
		t.Fatalf("updated = %+v err=%v", updated, err)
	}
	if err := s.RecordRuleSetFailure(id, ownerID, "upstream failed"); err != nil {
		t.Fatal(err)
	}
	failed, _ := s.GetRuleSetForUser(id, ownerID, false)
	if failed.LastError != "upstream failed" || !bytes.Equal(failed.PublishedSRS, []byte{4, 5}) {
		t.Fatalf("failed refresh state = %+v", failed)
	}
	if _, err := s.DeleteRuleSetsForUser([]int64{id}, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetRuleSetForUser(id, ownerID, false); err != ErrNotFound {
		t.Fatalf("get after delete = %v", err)
	}
}

func TestRuleSetOwnershipAndUniqueNames(t *testing.T) {
	s := openTest(t)
	first := createTestUser(t, s)
	second, err := s.CreateUser(&models.User{Username: "u2", PasswordHash: "hash", Role: models.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	makeItem := func(owner int64) *models.RuleSet {
		return &models.RuleSet{
			OwnerUserID: owner, Name: "same", PublishedJSON: []byte(`{}`), PublishedSRS: []byte{1},
			JSONSHA256: "j", SRSSHA256: "s", KernelVersion: "k",
			Sources: []*models.RuleSetSource{{Kind: "manual", Format: "source", Content: `{}`}},
		}
	}
	id, err := s.CreateRuleSet(makeItem(first))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRuleSet(makeItem(first)); err == nil {
		t.Fatal("duplicate owner/name should fail")
	}
	if _, err := s.CreateRuleSet(makeItem(second)); err != nil {
		t.Fatalf("same name for another owner: %v", err)
	}
	if _, err := s.GetRuleSetForUser(id, second, false); err != ErrNotFound {
		t.Fatalf("cross-owner get = %v", err)
	}
}
