// Package config holds runtime configuration parsed from CLI flags and env.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
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

// Action is a one-shot management operation requested by a CLI command.
type Action string

const (
	ActionServe         Action = ""
	ActionInstallDaemon Action = "install-daemon"
	ActionUpdate        Action = "update"
	ActionUninstall     Action = "uninstall"
	ActionResetAdmin    Action = "reset-admin"
	ActionStatus        Action = "status"
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
	RegExplicit         bool   // --registration was provided
	AddrExplicit        bool   // --addr/--address was provided
	DataDirExplicit     bool   // --data-dir was provided
	LogLevel            string // error, warn, info or debug
	RegistrationEnabled bool
	Dev                 bool // dev mode: serve API only, skip embedded frontend requirement
	ShowVersion         bool // print version and exit
	ShowHelp            bool // print command help and exit
	Command             string
}

// Parse reads flags (with env fallbacks) and returns the config.
func Parse(args []string) (*Config, error) {
	showDefaultHelp := len(args) == 0 && os.Getenv("SB_FOX_DAEMON") != "1"
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
	hasDataDirFlag := flagPresent(args, "--data-dir")
	addrExplicit := flagPresent(args, "--addr", "--address")
	daemonRequested := command == "daemon"

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
		"--addr": addr, "--address": addr,
		"--data-dir":     dataDir,
		"--kernel":       kernel,
		"--registration": reg,
		"--log-level":    logLevel,
	})
	regExplicit := flagPresent(args, "--registration")
	var installDaemon, update, uninstall, resetAdmin, purge, dev, showVersion bool
	if command == "daemon" && daemonCommandArg != "" {
		args = append(args, daemonCommandArg)
	}
	fs.Usage = func() {
		printHelp(fs.Output(), command, addr, dataDir, kernel, reg, logLevel)
	}
	fs.StringVar(&addr, "addr", addr, "listen address")
	fs.StringVar(&addr, "address", addr, "listen address")
	fs.StringVar(&dataDir, "data-dir", dataDir, "data directory (sqlite + temp)")
	fs.StringVar(&kernel, "kernel", kernel, "sing-box binary path for config validation")
	fs.BoolVar(&purge, "purge", false, "remove config and data during uninstall")
	fs.StringVar(&reg, "registration", reg, "public registration switch (on|off)")
	fs.StringVar(&logLevel, "log-level", logLevel, "log level (error|warn|info|debug)")
	fs.BoolVar(&dev, "dev", false, "dev mode (serve API only)")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")

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
	if err := validateCommandOptions(command, fs); err != nil {
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
	if command == "status" {
		if action != ActionServe {
			return nil, errors.New("status cannot be combined with another management command")
		}
		action = ActionStatus
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
		ShowHelp:            showDefaultHelp,
		Command:             command,
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

// normalizeCommand extracts the user-facing subcommand before flag parsing.
func normalizeCommand(args []string) ([]string, string, string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return args, "", "", nil
	}
	command := args[0]
	args = args[1:]
	switch command {
	case "run", "serve":
		return args, "run", "", nil
	case "update", "uninstall", "reset-admin", "status", "version":
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

func validateCommandOptions(command string, fs *flag.FlagSet) error {
	if command == "" {
		return nil
	}
	allowed := map[string]bool{}
	switch command {
	case "run":
		allowed = map[string]bool{"addr": true, "address": true, "data-dir": true, "kernel": true, "registration": true, "log-level": true, "dev": true}
	case "daemon":
		allowed = map[string]bool{"addr": true, "address": true, "data-dir": true, "kernel": true, "registration": true, "log-level": true}
	case "uninstall":
		allowed = map[string]bool{"purge": true, "data-dir": true}
	case "reset-admin":
		allowed = map[string]bool{"data-dir": true}
	case "update", "status", "version":
		allowed = map[string]bool{}
	default:
		return nil
	}
	var invalid string
	fs.Visit(func(f *flag.Flag) {
		if invalid == "" && !allowed[f.Name] {
			invalid = f.Name
		}
	})
	if invalid != "" {
		return fmt.Errorf("命令 %s 不支持参数 --%s", command, invalid)
	}
	return nil
}

// PrintHelp prints the help for the command selected in cfg.
func PrintHelp(w io.Writer, cfg *Config) {
	if cfg == nil {
		printHelp(w, "", defaultAddr, defaultDaemonDataDir, "sing-box", "off", defaultLogLevel)
		return
	}
	printHelp(w, cfg.Command, cfg.Addr, cfg.DataDir, cfg.KernelPath, cfg.RegMode, cfg.LogLevel)
}

func printHelp(w io.Writer, command, addr, dataDir, kernel, reg, logLevel string) {
	switch command {
	case "update":
		fmt.Fprintln(w, "用法:\n  sb-fox update\n\n说明:\n  更新已安装的 sb-fox。更新版本和 GitHub 凭据通过环境变量或当前安装环境确定。")
		return
	case "status":
		fmt.Fprintln(w, "用法:\n  sb-fox status\n\n说明:\n  显示正在运行的 sb-fox daemon 状态。")
		return
	case "version":
		fmt.Fprintln(w, "用法:\n  sb-fox version\n\n说明:\n  显示当前 sb-fox 版本。")
		return
	case "uninstall":
		fmt.Fprintln(w, "用法:\n  sb-fox uninstall [--purge]\n\n选项:\n  --purge\n\t同时删除配置和数据。")
		return
	case "reset-admin":
		fmt.Fprintf(w, "用法:\n  sb-fox reset-admin [--data-dir <目录>]\n\n选项:\n  --data-dir string\n\t数据库和配置目录（默认 %q）。\n", dataDir)
		return
	case "run", "serve":
		fmt.Fprintln(w, "用法:")
		fmt.Fprintln(w, "  sb-fox run [选项]")
		fmt.Fprintln(w, "\n选项:")
		printRunOptions(w, addr, dataDir, kernel, reg, logLevel, true)
		return
	case "daemon":
		fmt.Fprintln(w, "用法:")
		fmt.Fprintln(w, "  sb-fox daemon [enable|start|stop|restart|disable] [选项]")
		fmt.Fprintln(w, "\n选项:")
		printRunOptions(w, addr, dataDir, kernel, reg, logLevel, false)
		return
	}
	fmt.Fprintln(w, "用法:")
	fmt.Fprintln(w, "  sb-fox run [选项]")
	fmt.Fprintln(w, "  sb-fox daemon [enable|start|stop|restart|disable] [选项]")
	fmt.Fprintln(w, "  sb-fox update")
	fmt.Fprintln(w, "  sb-fox status")
	fmt.Fprintln(w, "  sb-fox uninstall [--purge]")
	fmt.Fprintln(w, "  sb-fox reset-admin [--data-dir <目录>]")
	fmt.Fprintln(w, "\n运行 sb-fox --help 查看此帮助，运行各子命令的 -h/--help 查看命令帮助。")
}

func printRunOptions(w io.Writer, addr, dataDir, kernel, reg, logLevel string, includeDev bool) {
	fmt.Fprintln(w, "  --addr string")
	fmt.Fprintf(w, "\t监听地址（默认 %q）\n", addr)
	fmt.Fprintln(w, "  --data-dir string")
	fmt.Fprintf(w, "\t数据目录（默认 %q）\n", dataDir)
	fmt.Fprintln(w, "  --kernel string")
	fmt.Fprintf(w, "\t用于配置校验的 sing-box 路径（默认 %q）\n", kernel)
	fmt.Fprintln(w, "  --registration on|off")
	fmt.Fprintf(w, "\t公开注册开关（默认 %q）\n", reg)
	fmt.Fprintln(w, "  --log-level error|warn|info|debug")
	fmt.Fprintf(w, "\t日志级别（默认 %q）\n", logLevel)
	if includeDev {
		fmt.Fprintln(w, "  --dev\n\t开发模式（仅提供 API）")
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
		return "", fmt.Errorf("unknown argument %q", args[1])
	}
	switch DaemonCommand(args[0]) {
	case DaemonEnable, DaemonStart, DaemonStop, DaemonRestart, DaemonDisable:
		return DaemonCommand(args[0]), nil
	default:
		return "", fmt.Errorf("daemon command must be one of enable, start, stop, restart or disable")
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
		return "", errors.New("--purge can only be used with uninstall")
	}
	return action, nil
}
