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
	release, err := acquireDaemonSocket(path, nil)
	if err != nil {
		t.Fatalf("acquireDaemonSocket: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		release()
		t.Fatalf("socket not created: %v", err)
	}

	_, err = acquireDaemonSocket(path, nil)
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

	release, err := acquireDaemonSocket(path, nil)
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

func TestDaemonSocketStatusCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb-fox.sock")
	release, err := acquireDaemonSocket(path, func(req daemonControlRequest) daemonControlResponse {
		if req.Command != "status" {
			t.Fatalf("command = %q, want status", req.Command)
		}
		return daemonControlResponse{
			OK: true,
			Status: daemonControlStatus{
				Addr:                "127.0.0.1:7878",
				DataDir:             "/var/lib/sb-fox",
				RegistrationEnabled: true,
			},
		}
	})
	if err != nil {
		t.Fatalf("acquireDaemonSocket: %v", err)
	}
	defer release()

	status, err := queryDaemonStatus(path)
	if err != nil {
		t.Fatalf("queryDaemonStatus: %v", err)
	}
	if status.Addr != "127.0.0.1:7878" || status.DataDir != "/var/lib/sb-fox" || !status.RegistrationEnabled {
		t.Fatalf("status = %+v", status)
	}
}

func TestDaemonSocketRejectsFailedResponse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb-fox.sock")
	release, err := acquireDaemonSocket(path, func(daemonControlRequest) daemonControlResponse {
		return daemonControlResponse{Error: "nope"}
	})
	if err != nil {
		t.Fatalf("acquireDaemonSocket: %v", err)
	}
	defer release()

	if _, err := queryDaemonStatus(path); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("query error = %v", err)
	}
}

func TestMaybeUseDaemonControlHandlesSameAddr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb-fox.sock")
	withDaemonControlSocket(t, path)
	release, err := acquireDaemonSocket(path, func(daemonControlRequest) daemonControlResponse {
		return daemonControlResponse{OK: true, Status: daemonControlStatus{
			Addr:    "127.0.0.1:7878",
			DataDir: "/var/lib/sb-fox",
		}}
	})
	if err != nil {
		t.Fatalf("acquireDaemonSocket: %v", err)
	}
	defer release()

	handled, err := maybeUseDaemonControl(&config.Config{Addr: "127.0.0.1:7878"})
	if err != nil {
		t.Fatalf("maybeUseDaemonControl: %v", err)
	}
	if !handled {
		t.Fatal("same address should be handled by existing daemon")
	}
}

func TestMaybeUseDaemonControlAllowsDifferentAddrAndReusesDataDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb-fox.sock")
	withDaemonControlSocket(t, path)
	release, err := acquireDaemonSocket(path, func(daemonControlRequest) daemonControlResponse {
		return daemonControlResponse{OK: true, Status: daemonControlStatus{
			Addr:    "127.0.0.1:7878",
			DataDir: "/var/lib/sb-fox",
		}}
	})
	if err != nil {
		t.Fatalf("acquireDaemonSocket: %v", err)
	}
	defer release()

	cfg := &config.Config{Addr: "127.0.0.1:7879", AddrExplicit: true}
	handled, err := maybeUseDaemonControl(cfg)
	if err != nil {
		t.Fatalf("maybeUseDaemonControl: %v", err)
	}
	if handled {
		t.Fatal("different explicit address should start foreground service")
	}
	if cfg.DataDir != "/var/lib/sb-fox" || cfg.DBPath != "/var/lib/sb-fox/sb-fox.db" {
		t.Fatalf("config data path = %q db=%q", cfg.DataDir, cfg.DBPath)
	}
}

func TestMaybeUseDaemonControlSetsRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb-fox.sock")
	withDaemonControlSocket(t, path)
	var got daemonControlRequest
	release, err := acquireDaemonSocket(path, func(req daemonControlRequest) daemonControlResponse {
		got = req
		return daemonControlResponse{OK: true, Status: daemonControlStatus{
			Addr:                "127.0.0.1:7878",
			DataDir:             "/var/lib/sb-fox",
			RegistrationEnabled: true,
		}}
	})
	if err != nil {
		t.Fatalf("acquireDaemonSocket: %v", err)
	}
	defer release()

	handled, err := maybeUseDaemonControl(&config.Config{Addr: "127.0.0.1:7878", RegExplicit: true, RegMode: "on"})
	if err != nil {
		t.Fatalf("maybeUseDaemonControl: %v", err)
	}
	if !handled {
		t.Fatal("registration control should be handled by daemon")
	}
	if got.Command != "set_registration" || got.RegMode != "on" {
		t.Fatalf("request = %+v", got)
	}
}

func withDaemonControlSocket(t *testing.T, path string) {
	t.Helper()
	old := daemonControlSocketPath
	daemonControlSocketPath = path
	t.Cleanup(func() { daemonControlSocketPath = old })
}

func TestRegistrationEnabledFromSettingsPersists(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "sb-fox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	enabled, err := registrationEnabledFromSettings(db, true)
	if err != nil {
		t.Fatalf("registrationEnabledFromSettings initial: %v", err)
	}
	if !enabled {
		t.Fatal("initial registration should use CLI value")
	}
	if err := db.SetSetting("registration_enabled", "false"); err != nil {
		t.Fatal(err)
	}
	enabled, err = registrationEnabledFromSettings(db, true)
	if err != nil {
		t.Fatalf("registrationEnabledFromSettings persisted: %v", err)
	}
	if enabled {
		t.Fatal("persisted registration setting should override CLI value")
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
