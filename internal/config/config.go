// Package config holds runtime configuration parsed from CLI flags and env.
package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Mode is the top-level runtime mode.
type Mode string

const (
	ModeServe  Mode = "serve"
	ModeDaemon Mode = "daemon"

	defaultAddr            = "127.0.0.1:7878"
	defaultUserDataSubpath = ".local/share/sb-fox"
	defaultDaemonDataDir   = "/var/lib/sb-fox"
	defaultDaemonSocket    = "/var/run/sb-fox.sock"
	defaultLogLevel        = "info"
)

var currentEUID = os.Geteuid

// Action is a one-shot management operation requested by CLI flags.
type Action string

const (
	ActionServe         Action = ""
	ActionInstallDaemon Action = "install-daemon"
	ActionUpdate        Action = "update"
	ActionUninstall     Action = "uninstall"
	ActionResetAdmin    Action = "reset-admin"
)

// DaemonCommand is the systemd operation requested through --daemon.
type DaemonCommand string

const (
	DaemonEnable  DaemonCommand = "enable"
	DaemonStart   DaemonCommand = "start"
	DaemonStop    DaemonCommand = "stop"
	DaemonRestart DaemonCommand = "restart"
	DaemonDisable DaemonCommand = "disable"
)

// Config is the resolved runtime configuration.
type Config struct {
	Addr                string // listen address, e.g. "127.0.0.1:7878"
	DataDir             string // directory for the SQLite db and temp files
	DBPath              string // resolved sqlite path (DataDir/sb-fox.db)
	KernelPath          string // initial sing-box binary path (overridable in settings)
	SocketPath          string // daemon singleton socket path
	Mode                Mode   // serve or daemon
	Action              Action // management operation, if any
	DaemonCommand       DaemonCommand
	Purge               bool   // uninstall removes config/data without prompting
	RegMode             string // on or off
	RegExplicit         bool   // --reg/-r was provided
	AddrExplicit        bool   // --addr/-a was provided
	DataDirExplicit     bool   // --data-dir/-D was provided
	LogLevel            string // error, warn, info or debug
	RegistrationEnabled bool
	Dev                 bool // dev mode: serve API only, skip embedded frontend requirement
	ShowVersion         bool // print version and exit
}

// Parse reads flags (with env fallbacks) and returns the config.
func Parse(args []string) (*Config, error) {
	mode := ModeServe
	if os.Getenv("SB_FOX_DAEMON") == "1" {
		mode = ModeDaemon
	}

	name := "sb-fox"
	dataDirEnv, hasDataDirEnv := os.LookupEnv("SB_FOX_DATA_DIR")
	hasDataDirFlag := flagPresent(args, "--data-dir", "-D")
	addrExplicit := flagPresent(args, "--addr", "-a")
	daemonRequested := flagPresent(args, "--daemon", "-d")

	dataDirDefault, err := defaultServeDataDir()
	if err != nil {
		return nil, err
	}
	if hasDataDirEnv && dataDirEnv != "" {
		dataDirDefault = dataDirEnv
	}
	if mode == ModeDaemon {
		name = "sb-fox daemon runtime"
		dataDirDefault = defaultDaemonDataDir
		if hasDataDirEnv && dataDirEnv != "" {
			dataDirDefault = dataDirEnv
		}
	}
	if daemonRequested {
		dataDirDefault = defaultDaemonDataDir
		if hasDataDirEnv && dataDirEnv != "" {
			dataDirDefault = dataDirEnv
		}
	}

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	addr := envOr("SB_FOX_ADDR", defaultAddr)
	dataDir := dataDirDefault
	kernel := envOr("SB_FOX_KERNEL", "sing-box")
	reg := envOr("SB_FOX_REG", "off")
	logLevel := envOr("SB_FOX_LOG", defaultLogLevel)
	args = fillMissingStringFlagValues(args, map[string]string{
		"--addr":     addr,
		"-a":         addr,
		"--data-dir": dataDir,
		"-D":         dataDir,
		"--kernel":   kernel,
		"-k":         kernel,
		"--reg":      reg,
		"-r":         reg,
		"--log":      logLevel,
		"-l":         logLevel,
	})
	regExplicit := flagPresent(args, "--reg", "-r")
	var installDaemon, update, uninstall, resetAdmin, purge, dev, showVersion bool
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage of %s:\n", name)
		fmt.Fprintln(out, "  --addr, -a string")
		fmt.Fprintf(out, "\tlisten address (default %q)\n", addr)
		fmt.Fprintln(out, "  --data-dir, -D string")
		fmt.Fprintf(out, "\tdata directory (sqlite + temp) (default %q)\n", dataDir)
		fmt.Fprintln(out, "  --kernel, -k string")
		fmt.Fprintf(out, "\tsing-box binary path for config validation (default %q)\n", kernel)
		fmt.Fprintln(out, "  --daemon, -d [enable|start|stop|restart|disable]")
		fmt.Fprintln(out, "\tmanage the system daemon (default enable)")
		fmt.Fprintln(out, "  --update, -u")
		fmt.Fprintln(out, "\tupdate installed binary")
		fmt.Fprintln(out, "  --uninstall, -U")
		fmt.Fprintln(out, "\tuninstall service and binary")
		fmt.Fprintln(out, "  --purge, -p")
		fmt.Fprintln(out, "\tremove config and data during uninstall")
		fmt.Fprintln(out, "  --reg, -r on|off")
		fmt.Fprintf(out, "\tpublic registration switch (default %q)\n", reg)
		fmt.Fprintln(out, "  --log, -l error|warn|info|debug")
		fmt.Fprintf(out, "\tlog level (default %q)\n", logLevel)
		fmt.Fprintln(out, "  --reset-admin, -P")
		fmt.Fprintln(out, "\treset admin password and print a new random password")
		fmt.Fprintln(out, "  --dev")
		fmt.Fprintln(out, "\tdev mode (serve API only)")
		fmt.Fprintln(out, "  --version, -v")
		fmt.Fprintln(out, "\tprint version and exit")
	}
	fs.StringVar(&addr, "addr", addr, "listen address")
	fs.StringVar(&addr, "a", addr, "listen address")
	fs.StringVar(&dataDir, "data-dir", dataDir, "data directory (sqlite + temp)")
	fs.StringVar(&dataDir, "D", dataDir, "data directory (sqlite + temp)")
	fs.StringVar(&kernel, "kernel", kernel, "sing-box binary path for config validation")
	fs.StringVar(&kernel, "k", kernel, "sing-box binary path for config validation")
	fs.BoolVar(&installDaemon, "daemon", false, "install, enable and start the system daemon")
	fs.BoolVar(&installDaemon, "d", false, "install, enable and start the system daemon")
	fs.BoolVar(&update, "update", false, "update installed binary")
	fs.BoolVar(&update, "u", false, "update installed binary")
	fs.BoolVar(&uninstall, "uninstall", false, "uninstall service and binary")
	fs.BoolVar(&uninstall, "U", false, "uninstall service and binary")
	fs.BoolVar(&purge, "purge", false, "remove config and data during uninstall")
	fs.BoolVar(&purge, "p", false, "remove config and data during uninstall")
	fs.StringVar(&reg, "reg", reg, "public registration switch (on|off)")
	fs.StringVar(&reg, "r", reg, "public registration switch (on|off)")
	fs.StringVar(&logLevel, "log", logLevel, "log level (error|warn|info|debug)")
	fs.StringVar(&logLevel, "l", logLevel, "log level (error|warn|info|debug)")
	fs.BoolVar(&resetAdmin, "reset-admin", false, "reset admin password and print a new random password")
	fs.BoolVar(&resetAdmin, "P", false, "reset admin password and print a new random password")
	fs.BoolVar(&dev, "dev", false, "dev mode (serve API only)")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.BoolVar(&showVersion, "v", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	daemonCommand, err := resolveDaemonCommand(installDaemon, fs.Args())
	if err != nil {
		return nil, err
	}
	if !installDaemon && fs.NArg() > 0 {
		return nil, fmt.Errorf("unknown argument %q", fs.Arg(0))
	}
	action, err := resolveAction(installDaemon, update, uninstall, resetAdmin, purge)
	if err != nil {
		return nil, err
	}
	regMode, err := normalizeReg(reg)
	if err != nil {
		return nil, err
	}
	normalizedLogLevel, err := normalizeLogLevel(logLevel)
	if err != nil {
		return nil, err
	}
	if mode == ModeDaemon && action != ActionServe {
		return nil, errors.New("management flags cannot be used inside daemon runtime")
	}

	c := &Config{
		Addr:                addr,
		DataDir:             dataDir,
		KernelPath:          kernel,
		SocketPath:          "",
		Mode:                mode,
		Action:              action,
		DaemonCommand:       daemonCommand,
		Purge:               purge,
		RegMode:             regMode,
		RegExplicit:         regExplicit,
		AddrExplicit:        addrExplicit,
		DataDirExplicit:     hasDataDirFlag || (hasDataDirEnv && dataDirEnv != ""),
		LogLevel:            normalizedLogLevel,
		RegistrationEnabled: regMode == "on",
		Dev:                 dev,
		ShowVersion:         showVersion,
	}
	if c.Mode == ModeDaemon {
		c.SocketPath = defaultDaemonSocket
	}
	c.DBPath = filepath.Join(c.DataDir, "sb-fox.db")
	return c, nil
}

// EnsureDataDir creates the data directory if it does not exist.
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0o755)
}

// SetDataDir updates DataDir and the derived DBPath together.
func (c *Config) SetDataDir(dataDir string) {
	c.DataDir = dataDir
	c.DBPath = filepath.Join(c.DataDir, "sb-fox.db")
}

func defaultServeDataDir() (string, error) {
	if currentEUID() == 0 {
		return defaultDaemonDataDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user data dir: %w", err)
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return "", errors.New("resolve user data dir: HOME is empty")
	}
	return filepath.Join(home, defaultUserDataSubpath), nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func flagPresent(args []string, names ...string) bool {
	for _, arg := range args {
		for _, name := range names {
			if arg == name || strings.HasPrefix(arg, name+"=") {
				return true
			}
		}
	}
	return false
}

func fillMissingStringFlagValues(args []string, defaults map[string]string) []string {
	out := make([]string, 0, len(args))
	for i, arg := range args {
		out = append(out, arg)
		value, ok := defaults[arg]
		if !ok {
			continue
		}
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
			out = append(out, value)
		}
	}
	return out
}

func normalizeReg(value string) (string, error) {
	switch value {
	case "on", "off":
		return value, nil
	default:
		return "", fmt.Errorf("--reg must be on or off")
	}
}

func normalizeLogLevel(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error":
		return "error", nil
	case "warn":
		return "warn", nil
	case "info":
		return "info", nil
	case "debug":
		return "debug", nil
	default:
		return "", fmt.Errorf("--log must be one of error, warn, info or debug")
	}
}

func resolveDaemonCommand(enabled bool, args []string) (DaemonCommand, error) {
	if !enabled {
		return "", nil
	}
	if len(args) == 0 {
		return DaemonEnable, nil
	}
	if len(args) > 1 {
		for _, arg := range args[1:] {
			if isManagementArg(arg) {
				return "", errors.New("only one management flag can be used at a time")
			}
		}
		return "", fmt.Errorf("unknown argument %q", args[1])
	}
	switch DaemonCommand(args[0]) {
	case DaemonEnable, DaemonStart, DaemonStop, DaemonRestart, DaemonDisable:
		return DaemonCommand(args[0]), nil
	default:
		return "", fmt.Errorf("--daemon command must be one of enable, start, stop, restart or disable")
	}
}

func isManagementArg(arg string) bool {
	switch arg {
	case "--daemon", "-d", "--update", "-u", "--uninstall", "-U", "--purge", "-p", "--reset-admin", "-P":
		return true
	default:
		return false
	}
}

func resolveAction(installDaemon, update, uninstall, resetAdmin, purge bool) (Action, error) {
	count := 0
	var action Action
	if installDaemon {
		count++
		action = ActionInstallDaemon
	}
	if update {
		count++
		action = ActionUpdate
	}
	if uninstall {
		count++
		action = ActionUninstall
	}
	if resetAdmin {
		count++
		action = ActionResetAdmin
	}
	if count > 1 {
		return "", errors.New("only one management flag can be used at a time")
	}
	if purge && !uninstall {
		return "", errors.New("--purge can only be used with --uninstall")
	}
	return action, nil
}
