package bootstrap

import (
	"path/filepath"
	"testing"

	"github.com/mora1n/sb-fox/internal/store"
)

func TestEnsureAdminCreatesOnce(t *testing.T) {
	t.Setenv("SB_FOX_ADMIN_PASSWORD", "")
	db, err := store.Open(filepath.Join(t.TempDir(), "sb-fox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	first, err := EnsureAdmin(db)
	if err != nil {
		t.Fatalf("EnsureAdmin first: %v", err)
	}
	if !first.Created || !first.Generated || first.Username != "admin" || first.Password == "" {
		t.Fatalf("first result = %+v", first)
	}

	second, err := EnsureAdmin(db)
	if err != nil {
		t.Fatalf("EnsureAdmin second: %v", err)
	}
	if second.Created {
		t.Fatalf("second result should not create admin: %+v", second)
	}
}

func TestEnsureAdminEnvPasswordIsNotReturned(t *testing.T) {
	t.Setenv("SB_FOX_ADMIN_PASSWORD", "known-password")
	db, err := store.Open(filepath.Join(t.TempDir(), "sb-fox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := EnsureAdmin(db)
	if err != nil {
		t.Fatalf("EnsureAdmin: %v", err)
	}
	if !result.Created || !result.FromEnv || result.Generated || result.Password != "" {
		t.Fatalf("env password result = %+v", result)
	}
}
