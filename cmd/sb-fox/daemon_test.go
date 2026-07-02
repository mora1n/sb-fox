package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mora1n/sb-fox/internal/config"
	"github.com/mora1n/sb-fox/internal/store"
)

func TestAcquireDaemonSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb-fox.sock")
	release, err := acquireDaemonSocket(path)
	if err != nil {
		t.Fatalf("acquireDaemonSocket: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		release()
		t.Fatalf("socket not created: %v", err)
	}

	_, err = acquireDaemonSocket(path)
	if err == nil {
		release()
		t.Fatal("expected duplicate daemon error")
	}
	if !strings.Contains(err.Error(), "already running") {
		release()
		t.Fatalf("duplicate error = %q", err)
	}

	release()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("socket not cleaned up: %v", err)
	}
}

func TestAcquireDaemonSocketReplacesStaleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb-fox.sock")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale socket placeholder: %v", err)
	}

	release, err := acquireDaemonSocket(path)
	if err != nil {
		t.Fatalf("acquireDaemonSocket: %v", err)
	}
	defer release()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("socket not created: %v", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("path is not a socket: mode=%s", info.Mode())
	}
}

func TestResetAdminPasswordWithExplicitDataDir(t *testing.T) {
	cfg, err := config.Parse([]string{"-P", "-D", t.TempDir()})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := resetAdminPassword(cfg); err != nil {
		t.Fatalf("resetAdminPassword create: %v", err)
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := db.FirstAdmin()
	if err != nil {
		t.Fatal(err)
	}
	firstHash := admin.PasswordHash
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := resetAdminPassword(cfg); err != nil {
		t.Fatalf("resetAdminPassword reset: %v", err)
	}
	db, err = store.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	admin, err = db.FirstAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if admin.Username != "admin" {
		t.Fatalf("admin username = %q", admin.Username)
	}
	if admin.PasswordHash == "" || admin.PasswordHash == firstHash {
		t.Fatalf("admin password hash was not reset")
	}
}

func TestResetAdminDataDirErrorHasActionableHints(t *testing.T) {
	err := resetAdminDataDirError("/var/lib/sb-fox", os.ErrPermission)
	msg := err.Error()
	if !strings.Contains(msg, "-D ./data") || !strings.Contains(msg, "sudo sb-fox -P") {
		t.Fatalf("error lacks reset hints: %s", msg)
	}
}
