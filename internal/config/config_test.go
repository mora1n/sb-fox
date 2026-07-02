package config

import "testing"

func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SB_FOX_ADDR", "")
	t.Setenv("SB_FOX_DATA_DIR", "")
	t.Setenv("SB_FOX_KERNEL", "")
	t.Setenv("SB_FOX_DAEMON", "")
	t.Setenv("SB_FOX_REG", "")
}

func setEUID(t *testing.T, id int) {
	t.Helper()
	old := currentEUID
	currentEUID = func() int { return id }
	t.Cleanup(func() { currentEUID = old })
}

func TestParseServeDefaults(t *testing.T) {
	clearEnv(t)

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
	if cfg.DataDir != defaultServeDataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, defaultServeDataDir)
	}
	if cfg.DBPath != "data/sb-fox.db" {
		t.Fatalf("db path = %q", cfg.DBPath)
	}
	if cfg.SocketPath != "" {
		t.Fatalf("socket path = %q, want empty", cfg.SocketPath)
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
	if cfg.DataDir != defaultDaemonDataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, defaultDaemonDataDir)
	}
	if cfg.DBPath != "/var/lib/sb-fox/sb-fox.db" {
		t.Fatalf("db path = %q", cfg.DBPath)
	}
	if cfg.SocketPath != "" {
		t.Fatalf("socket path = %q, want empty", cfg.SocketPath)
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

func TestParseResetAdminDefaultsToServeDataDirWithoutRoot(t *testing.T) {
	clearEnv(t)
	setEUID(t, 1000)

	cfg, err := Parse([]string{"-P"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Action != ActionResetAdmin {
		t.Fatalf("action = %q, want %q", cfg.Action, ActionResetAdmin)
	}
	if cfg.DataDir != defaultServeDataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, defaultServeDataDir)
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
