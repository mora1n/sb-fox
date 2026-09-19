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
	if !cfg.ShowHelp {
		t.Fatal("no-argument invocation should show help")
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

func TestParseLongOptions(t *testing.T) {
	clearEnv(t)

	cfg, err := Parse([]string{"run", "--addr", "localhost:18080", "--data-dir", "/tmp/sb-data", "--kernel", "/bin/sing-box"})
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
	if !cfg.AddrExplicit || !cfg.DataDirExplicit {
		t.Fatalf("explicit flags = addr:%v data:%v", cfg.AddrExplicit, cfg.DataDirExplicit)
	}
	versionCfg, err := Parse([]string{"version"})
	if err != nil || !versionCfg.ShowVersion {
		t.Fatalf("version command config = %+v, err=%v", versionCfg, err)
	}
}

func TestParseDaemonInstallDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Parse([]string{"daemon"})
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

	if _, err := Parse([]string{"--daemon"}); err == nil {
		t.Fatal("legacy --daemon flag should be rejected")
	}
}

func TestParseDaemonCommands(t *testing.T) {
	clearEnv(t)

	for _, tc := range []struct {
		args []string
		want DaemonCommand
	}{
		{[]string{"daemon", "enable"}, DaemonEnable},
		{[]string{"daemon", "start"}, DaemonStart},
		{[]string{"daemon", "stop"}, DaemonStop},
		{[]string{"daemon", "restart"}, DaemonRestart},
		{[]string{"daemon", "disable"}, DaemonDisable},
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

func TestParseSubcommandsAndLongOptions(t *testing.T) {
	clearEnv(t)
	setEUID(t, 1000)
	t.Setenv("HOME", "/home/tester")
	cases := []struct {
		args []string
		want Action
	}{
		{[]string{"update"}, ActionUpdate},
		{[]string{"uninstall", "--purge"}, ActionUninstall},
		{[]string{"reset-admin"}, ActionResetAdmin},
	}
	for _, tc := range cases {
		cfg, err := Parse(tc.args)
		if err != nil {
			t.Fatalf("Parse(%v): %v", tc.args, err)
		}
		if cfg.Action != tc.want {
			t.Fatalf("Parse(%v) action=%q, want %q", tc.args, cfg.Action, tc.want)
		}
	}
	cfg, err := Parse([]string{"daemon", "restart", "--address", "127.0.0.1:9999", "--registration", "on", "--log-level", "debug"})
	if err != nil {
		t.Fatalf("Parse daemon subcommand: %v", err)
	}
	if cfg.Action != ActionInstallDaemon || cfg.DaemonCommand != DaemonRestart || cfg.Addr != "127.0.0.1:9999" || !cfg.RegistrationEnabled || cfg.LogLevel != "debug" {
		t.Fatalf("daemon config = %+v", cfg)
	}
}

func TestParseStatusAndCommandOptionValidation(t *testing.T) {
	clearEnv(t)
	cfg, err := Parse([]string{"status"})
	if err != nil {
		t.Fatalf("Parse status: %v", err)
	}
	if cfg.Action != ActionStatus {
		t.Fatalf("status action = %q", cfg.Action)
	}
	if _, err := Parse([]string{"update", "--data-dir", "/tmp/sb-fox"}); err == nil {
		t.Fatal("update should reject runtime options")
	}
	if _, err := Parse([]string{"status", "--addr", "127.0.0.1:9999"}); err == nil {
		t.Fatal("status should reject runtime options")
	}
}

func TestParseRejectsLegacyManagementFlags(t *testing.T) {
	clearEnv(t)
	for _, args := range [][]string{
		{"--update"}, {"-u"}, {"--daemon"}, {"-d"},
		{"--uninstall"}, {"-U"}, {"--reset-admin"}, {"-P"},
		{"--reg", "on"}, {"-r", "on"}, {"--log", "debug"}, {"-l", "debug"},
		{"-a", "127.0.0.1:9999"}, {"-D", "/tmp/sb-fox"}, {"-k", "sing-box"},
	} {
		if _, err := Parse(args); err == nil {
			t.Fatalf("legacy flag %v should be rejected", args)
		}
	}
}

func TestParseDaemonRejectsInvalidCommand(t *testing.T) {
	clearEnv(t)

	if _, err := Parse([]string{"daemon", "reload"}); err == nil {
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

	if _, err := Parse([]string{"daemon", "restart", "update"}); err == nil {
		t.Fatal("expected invalid extra daemon command")
	}
	if _, err := Parse([]string{"daemon", "stop", "--purge"}); err == nil {
		t.Fatal("expected purge conflict after daemon command")
	}
	if _, err := Parse([]string{"--purge"}); err == nil {
		t.Fatal("expected purge without uninstall error")
	}
}

func TestParseRegistrationSwitch(t *testing.T) {
	clearEnv(t)

	cfg, err := Parse([]string{"--registration", "on"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !cfg.RegistrationEnabled || cfg.RegMode != "on" || !cfg.RegExplicit {
		t.Fatalf("registration config = mode:%q enabled:%v explicit:%v", cfg.RegMode, cfg.RegistrationEnabled, cfg.RegExplicit)
	}

	cfg, err = Parse([]string{"--registration", "off"})
	if err != nil {
		t.Fatalf("Parse off: %v", err)
	}
	if cfg.RegistrationEnabled || cfg.RegMode != "off" || !cfg.RegExplicit {
		t.Fatalf("registration config = mode:%q enabled:%v explicit:%v", cfg.RegMode, cfg.RegistrationEnabled, cfg.RegExplicit)
	}

	if _, err := Parse([]string{"--registration", "maybe"}); err == nil {
		t.Fatal("expected invalid --registration value")
	}
}

func TestParseLogLevel(t *testing.T) {
	clearEnv(t)

	cfg, err := Parse([]string{"run", "--log-level", "debug"})
	if err != nil {
		t.Fatalf("Parse long: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("log level = %q, want debug", cfg.LogLevel)
	}

	cfg, err = Parse([]string{"run", "--log-level", "warn"})
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

	if _, err := Parse([]string{"run", "--log-level", "noisy"}); err == nil {
		t.Fatal("expected invalid --log value")
	}
}

func TestParseLongOptionsUseDefaultsWithoutValues(t *testing.T) {
	clearEnv(t)
	setEUID(t, 1000)
	t.Setenv("HOME", "/home/tester")

	cfg, err := Parse([]string{"run", "--log-level"})
	if err != nil {
		t.Fatalf("Parse log default: %v", err)
	}
	if cfg.LogLevel != defaultLogLevel {
		t.Fatalf("log level = %q, want %q", cfg.LogLevel, defaultLogLevel)
	}

	clearEnv(t)
	setEUID(t, 1000)
	t.Setenv("HOME", "/home/tester")
	t.Setenv("SB_FOX_LOG", "debug")
	cfg, err = Parse([]string{"run", "--log-level"})
	if err != nil {
		t.Fatalf("Parse log env default: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("env log level = %q, want debug", cfg.LogLevel)
	}

	clearEnv(t)
	setEUID(t, 1000)
	t.Setenv("HOME", "/home/tester")
	cfg, err = Parse([]string{"run", "--addr", "--data-dir", "--kernel", "--registration"})
	if err != nil {
		t.Fatalf("Parse string defaults: %v", err)
	}
	wantDataDir := filepath.Join("/home/tester", defaultUserDataSubpath)
	if cfg.Addr != defaultAddr || cfg.DataDir != wantDataDir || cfg.KernelPath != "sing-box" || cfg.RegMode != "off" {
		t.Fatalf("defaults addr=%q data=%q kernel=%q reg=%q", cfg.Addr, cfg.DataDir, cfg.KernelPath, cfg.RegMode)
	}
	if !cfg.AddrExplicit || !cfg.DataDirExplicit || !cfg.RegExplicit {
		t.Fatalf("explicit flags = addr:%v data:%v reg:%v", cfg.AddrExplicit, cfg.DataDirExplicit, cfg.RegExplicit)
	}
}

func TestParseResetAdminDefaultsToServeDataDirWithoutRoot(t *testing.T) {
	clearEnv(t)
	setEUID(t, 1000)
	t.Setenv("HOME", "/home/tester")

	cfg, err := Parse([]string{"reset-admin"})
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

	cfg, err := Parse([]string{"reset-admin"})
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

	cfg, err := Parse([]string{"reset-admin", "--data-dir", "/tmp/sb-fox"})
	if err != nil {
		t.Fatalf("Parse explicit: %v", err)
	}
	if cfg.DataDir != "/tmp/sb-fox" {
		t.Fatalf("explicit data dir = %q", cfg.DataDir)
	}

	clearEnv(t)
	setEUID(t, 0)
	t.Setenv("SB_FOX_DATA_DIR", "/tmp/sb-fox-env")
	cfg, err = Parse([]string{"reset-admin"})
	if err != nil {
		t.Fatalf("Parse env: %v", err)
	}
	if cfg.DataDir != "/tmp/sb-fox-env" {
		t.Fatalf("env data dir = %q", cfg.DataDir)
	}
}
