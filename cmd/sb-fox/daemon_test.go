package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
