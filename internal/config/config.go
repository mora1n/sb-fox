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

// Action is a one-shot management operation requested by a CLI command or a
// legacy flag.
type Action string

const (
	ActionServe         Action = ""
	ActionInstallDaemon Action = "install-daemon"
	ActionUpdate        Action = "update"
	ActionUninstall     Action = "uninstall"
	ActionResetAdmin    Action = "reset-admin"
)

// DaemonCommand is the systemd operation requested through the daemon command.
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
	var command, daemonCommandArg string
	var err error
	args, command, daemonCommandArg, err = normalizeCommand(args)
	if err != nil {
		return nil, err
	}
	mode := ModeServe
	if os.Getenv("SB_FOX_DAEMON") == "1" {
		mode = ModeDaemon
	}

	name := "sb-fox"
	dataDirEnv, hasDataDirEnv := os.LookupEnv("SB_FOX_DATA_DIR")
	hasDataDirFlag := flagPresent(args, "--data-dir", "-D")
	addrExplicit := flagPresent(args, "--addr", "-a", "--address")
	daemonRequested := command == "daemon" || flagPresent(args, "--daemon", "-d")

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
		"--addr": addr, "-a": addr, "--address": addr,
		"--data-dir": dataDir, "-D": dataDir,
		"--kernel": kernel, "-k": kernel,
		"--reg": reg, "-r": reg, "--registration": reg,
		"--log": logLevel, "-l": logLevel, "--log-level": logLevel,
	})
	regExplicit := flagPresent(args, "--reg", "-r", "--registration")
	var installDaemon, update, uninstall, resetAdmin, purge, dev, showVersion bool
	if command == "daemon" && daemonCommandArg != "" {
		args = append(args, daemonCommandArg)
	}
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintln(out, "用法:")
		fmt.Fprintln(out, "  sb-fox [run] [选项]")
		fmt.Fprintln(out, "  sb-fox daemon [enable|start|stop|restart|disable] [选项]")
		fmt.Fprintln(out, "  sb-fox update [选项]")
		fmt.Fprintln(out, "  sb-fox uninstall [--purge] [选项]")
		fmt.Fprintln(out, "  sb-fox reset-admin [选项]")
		fmt.Fprintln(out, "\n选项:")
		fmt.Fprintln(out, "  --addr string")
		fmt.Fprintf(out, "\t监听地址（默认 %q）\n", addr)
		fmt.Fprintln(out, "  --data-dir string")
		fmt.Fprintf(out, "\t数据目录（SQLite 和临时文件，默认 %q）\n", dataDir)
		fmt.Fprintln(out, "  --kernel string")
		fmt.Fprintf(out, "\t用于配置校验的 sing-box 路径（默认 %q）\n", kernel)
		fmt.Fprintln(out, "  --registration on|off")
		fmt.Fprintf(out, "\t公开注册开关（默认 %q）\n", reg)
		fmt.Fprintln(out, "  --log-level error|warn|info|debug")
		fmt.Fprintf(out, "\t日志级别（默认 %q）\n", logLevel)
		fmt.Fprintln(out, "  --purge")
		fmt.Fprintln(out, "\t卸载时同时删除配置和数据")
		fmt.Fprintln(out, "  --dev")
		fmt.Fprintln(out, "\t开发模式（仅提供 API）")
		fmt.Fprintln(out, "  --version")
		fmt.Fprintln(out, "\t显示版本并退出")
	}
	fs.StringVar(&addr, "addr", addr, "listen address")
	fs.StringVar(&addr, "a", addr, "listen address")
	fs.StringVar(&addr, "address", addr, "listen address")
	fs.StringVar(&dataDir, "data-dir", dataDir, "data directory (sqlite + temp)")
	fs.StringVar(&dataDir, "D", dataDir, "data directory (sqlite + temp)")
	fs.StringVar(&kernel, "kernel", kernel, "sing-box binary path for config validation")
	fs.StringVar(&kernel, "k", kernel, "sing-box binary path for config validation")
	fs.BoolVar(&installDaemon, "daemon", false, "manage the system daemon")
	fs.BoolVar(&installDaemon, "d", false, "manage the system daemon")
	fs.BoolVar(&update, "update", false, "update installed binary")
	fs.BoolVar(&update, "u", false, "update installed binary")
	fs.BoolVar(&uninstall, "uninstall", false, "uninstall service and binary")
	fs.BoolVar(&uninstall, "U", false, "uninstall service and binary")
	fs.BoolVar(&purge, "purge", false, "remove config and data during uninstall")
	fs.BoolVar(&purge, "p", false, "remove config and data during uninstall")
	fs.StringVar(&reg, "reg", reg, "public registration switch (on|off)")
	fs.StringVar(&reg, "r", reg, "public registration switch (on|off)")
	fs.StringVar(&reg, "registration", reg, "public registration switch (on|off)")
	fs.StringVar(&logLevel, "log", logLevel, "log level (error|warn|info|debug)")
	fs.StringVar(&logLevel, "l", logLevel, "log level (error|warn|info|debug)")
	fs.StringVar(&logLevel, "log-level", logLevel, "log level (error|warn|info|debug)")
	fs.BoolVar(&resetAdmin, "reset-admin", false, "reset admin password and print a new random password")
	fs.BoolVar(&resetAdmin, "P", false, "reset admin password and print a new random password")
	fs.BoolVar(&dev, "dev", false, "dev mode (serve API only)")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.BoolVar(&showVersion, "v", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	switch command {
	case "daemon":
		installDaemon = true
	case "update":
		update = true
	case "uninstall":
		uninstall = true
	case "reset-admin":
		resetAdmin = true
	case "version":
		showVersion = true
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

// normalizeCommand converts the user-facing subcommand syntax into the
// existing flag-based representation. Legacy flags remain accepted so older
// service scripts and operators can upgrade without a breaking transition.
func normalizeCommand(args []string) ([]string, string, string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return args, "", "", nil
	}
	command := args[0]
	args = args[1:]
	switch command {
	case "run", "serve":
		return args, "", "", nil
	case "update", "uninstall", "reset-admin", "version":
		return args, command, "", nil
	case "daemon":
		var daemonCommand string
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			daemonCommand = args[0]
			args = args[1:]
		}
		return args, command, daemonCommand, nil
	default:
		return nil, "", "", fmt.Errorf("未知命令 %q；使用 sb-fox --help 查看帮助", command)
	}
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
		return "", fmt.Errorf("--registration must be on or off")
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
		return "", fmt.Errorf("--log-level must be one of error, warn, info or debug")
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
