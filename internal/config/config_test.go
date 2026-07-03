package config

import (
	"path/filepath"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SB_FOX_ADDR", "")
	t.Setenv("SB_FOX_DATA_DIR", "")
	t.Setenv("SB_FOX_KERNEL", "")
	t.Setenv("SB_FOX_DAEMON", "")
	t.Setenv("SB_FOX_REG", "")
	t.Setenv("SB_FOX_LOG", "")
}

func setEUID(t *testing.T, id int) {
	t.Helper()
	old := currentEUID
	currentEUID = func() int { return id }
	t.Cleanup(func() { currentEUID = old })
}

func TestParseServeDefaults(t *testing.T) {
	clearEnv(t)
	setEUID(t, 1000)
	t.Setenv("HOME", "/home/tester")

	cfg, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Mode != ModeServe {
		t.Fatalf("mode = %q, want %q", cfg.Mode, ModeServe)
	}
	if cfg.Action != ActionServe {
		t.Fatalf("action = %q, want serve", cfg.Action)
	}
	if cfg.Addr != defaultAddr {
		t.Fatalf("addr = %q, want %q", cfg.Addr, defaultAddr)
	}
	wantDataDir := filepath.Join("/home/tester", defaultUserDataSubpath)
	if cfg.DataDir != wantDataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, wantDataDir)
	}
	if cfg.DBPath != filepath.Join(wantDataDir, "sb-fox.db") {
		t.Fatalf("db path = %q", cfg.DBPath)
	}
	if cfg.AddrExplicit || cfg.DataDirExplicit {
		t.Fatalf("explicit flags = addr:%v data:%v", cfg.AddrExplicit, cfg.DataDirExplicit)
	}
	if cfg.SocketPath != "" {
		t.Fatalf("socket path = %q, want empty", cfg.SocketPath)
	}
	if cfg.LogLevel != defaultLogLevel {
		t.Fatalf("log level = %q, want %q", cfg.LogLevel, defaultLogLevel)
	}
}

func TestParseShortOptions(t *testing.T) {
	clearEnv(t)

	cfg, err := Parse([]string{"-a", "localhost:18080", "-D", "/tmp/sb-data", "-k", "/bin/sing-box", "-v"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Addr != "localhost:18080" {
		t.Fatalf("addr = %q", cfg.Addr)
	}
	if cfg.DataDir != "/tmp/sb-data" {
		t.Fatalf("data dir = %q", cfg.DataDir)
	}
	if cfg.KernelPath != "/bin/sing-box" {
		t.Fatalf("kernel path = %q", cfg.KernelPath)
	}
	if !cfg.ShowVersion {
		t.Fatal("version flag not set")
	}
	if !cfg.AddrExplicit || !cfg.DataDirExplicit {
		t.Fatalf("explicit flags = addr:%v data:%v", cfg.AddrExplicit, cfg.DataDirExplicit)
	}
}

func TestParseDaemonInstallDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Parse([]string{"--daemon"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Mode != ModeServe {
		t.Fatalf("mode = %q, want serve management mode", cfg.Mode)
	}
	if cfg.Action != ActionInstallDaemon {
		t.Fatalf("action = %q, want %q", cfg.Action, ActionInstallDaemon)
	}
	if cfg.DaemonCommand != DaemonEnable {
		t.Fatalf("daemon command = %q, want %q", cfg.DaemonCommand, DaemonEnable)
	}
	if cfg.DataDir != defaultDaemonDataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, defaultDaemonDataDir)
	}
	if cfg.DBPath != "/var/lib/sb-fox/sb-fox.db" {
		t.Fatalf("db path = %q", cfg.DBPath)
	}
	if cfg.SocketPath != "" {
		t.Fatalf("socket path = %q, want empty", cfg.SocketPath)
	}

	cfg, err = Parse([]string{"--daemon=true"})
	if err != nil {
		t.Fatalf("Parse daemon bool form: %v", err)
	}
	if cfg.DaemonCommand != DaemonEnable || cfg.DataDir != defaultDaemonDataDir {
		t.Fatalf("daemon bool form command=%q data dir=%q", cfg.DaemonCommand, cfg.DataDir)
	}
}

func TestParseDaemonCommands(t *testing.T) {
	clearEnv(t)

	for _, tc := range []struct {
		args []string
		want DaemonCommand
	}{
		{[]string{"--daemon", "enable"}, DaemonEnable},
		{[]string{"--daemon", "start"}, DaemonStart},
		{[]string{"--daemon", "stop"}, DaemonStop},
		{[]string{"--daemon", "restart"}, DaemonRestart},
		{[]string{"--daemon", "disable"}, DaemonDisable},
		{[]string{"-d", "stop"}, DaemonStop},
	} {
		cfg, err := Parse(tc.args)
		if err != nil {
			t.Fatalf("Parse(%v): %v", tc.args, err)
		}
		if cfg.Action != ActionInstallDaemon || cfg.DaemonCommand != tc.want {
			t.Fatalf("Parse(%v) action=%q command=%q, want %q/%q", tc.args, cfg.Action, cfg.DaemonCommand, ActionInstallDaemon, tc.want)
		}
	}
}

func TestParseDaemonRejectsInvalidCommand(t *testing.T) {
	clearEnv(t)

	if _, err := Parse([]string{"--daemon", "reload"}); err == nil {
		t.Fatal("expected invalid daemon command error")
	}
}

func TestParseDaemonRuntime(t *testing.T) {
	clearEnv(t)
	t.Setenv("SB_FOX_DAEMON", "1")

	cfg, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Mode != ModeDaemon {
		t.Fatalf("mode = %q, want %q", cfg.Mode, ModeDaemon)
	}
	if cfg.Action != ActionServe {
		t.Fatalf("action = %q, want serve", cfg.Action)
	}
	if cfg.DataDir != defaultDaemonDataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, defaultDaemonDataDir)
	}
	if cfg.SocketPath != defaultDaemonSocket {
		t.Fatalf("socket path = %q, want %q", cfg.SocketPath, defaultDaemonSocket)
	}
}

func TestParseSocketFlagRemoved(t *testing.T) {
	clearEnv(t)

	if _, err := Parse([]string{"--socket", "/tmp/sb.sock"}); err == nil {
		t.Fatal("expected unknown socket flag error")
	}
}

func TestParseManagementConflicts(t *testing.T) {
	clearEnv(t)

	if _, err := Parse([]string{"--daemon", "--update"}); err == nil {
		t.Fatal("expected management conflict")
	}
	if _, err := Parse([]string{"--daemon", "restart", "--update"}); err == nil {
		t.Fatal("expected management conflict after daemon command")
	}
	if _, err := Parse([]string{"--daemon", "stop", "--purge"}); err == nil {
		t.Fatal("expected purge conflict after daemon command")
	}
	if _, err := Parse([]string{"--purge"}); err == nil {
		t.Fatal("expected purge without uninstall error")
	}
}

func TestParseRegistrationSwitch(t *testing.T) {
	clearEnv(t)

	cfg, err := Parse([]string{"--reg", "on"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !cfg.RegistrationEnabled || cfg.RegMode != "on" || !cfg.RegExplicit {
		t.Fatalf("registration config = mode:%q enabled:%v explicit:%v", cfg.RegMode, cfg.RegistrationEnabled, cfg.RegExplicit)
	}

	cfg, err = Parse([]string{"-r", "off"})
	if err != nil {
		t.Fatalf("Parse short: %v", err)
	}
	if cfg.RegistrationEnabled || cfg.RegMode != "off" || !cfg.RegExplicit {
		t.Fatalf("registration config = mode:%q enabled:%v explicit:%v", cfg.RegMode, cfg.RegistrationEnabled, cfg.RegExplicit)
	}

	if _, err := Parse([]string{"--reg", "maybe"}); err == nil {
		t.Fatal("expected invalid --reg value")
	}
}

func TestParseLogLevel(t *testing.T) {
	clearEnv(t)

	cfg, err := Parse([]string{"--log", "debug"})
	if err != nil {
		t.Fatalf("Parse long: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("log level = %q, want debug", cfg.LogLevel)
	}

	cfg, err = Parse([]string{"-l", "warn"})
	if err != nil {
		t.Fatalf("Parse short: %v", err)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("log level = %q, want warn", cfg.LogLevel)
	}

	clearEnv(t)
	t.Setenv("SB_FOX_LOG", "error")
	cfg, err = Parse(nil)
	if err != nil {
		t.Fatalf("Parse env: %v", err)
	}
	if cfg.LogLevel != "error" {
		t.Fatalf("env log level = %q, want error", cfg.LogLevel)
	}

	if _, err := Parse([]string{"--log", "noisy"}); err == nil {
		t.Fatal("expected invalid --log value")
	}
}

func TestParseResetAdminDefaultsToServeDataDirWithoutRoot(t *testing.T) {
	clearEnv(t)
	setEUID(t, 1000)
	t.Setenv("HOME", "/home/tester")

	cfg, err := Parse([]string{"-P"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Action != ActionResetAdmin {
		t.Fatalf("action = %q, want %q", cfg.Action, ActionResetAdmin)
	}
	wantDataDir := filepath.Join("/home/tester", defaultUserDataSubpath)
	if cfg.DataDir != wantDataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, wantDataDir)
	}
}

func TestParseResetAdminDefaultsToDaemonDataDirWithRoot(t *testing.T) {
	clearEnv(t)
	setEUID(t, 0)

	cfg, err := Parse([]string{"-P"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.DataDir != defaultDaemonDataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, defaultDaemonDataDir)
	}
}

func TestParseResetAdminDataDirOverrides(t *testing.T) {
	clearEnv(t)
	setEUID(t, 0)

	cfg, err := Parse([]string{"-P", "-D", "/tmp/sb-fox"})
	if err != nil {
		t.Fatalf("Parse explicit: %v", err)
	}
	if cfg.DataDir != "/tmp/sb-fox" {
		t.Fatalf("explicit data dir = %q", cfg.DataDir)
	}

	clearEnv(t)
	setEUID(t, 0)
	t.Setenv("SB_FOX_DATA_DIR", "/tmp/sb-fox-env")
	cfg, err = Parse([]string{"-P"})
	if err != nil {
		t.Fatalf("Parse env: %v", err)
	}
	if cfg.DataDir != "/tmp/sb-fox-env" {
		t.Fatalf("env data dir = %q", cfg.DataDir)
	}
}
