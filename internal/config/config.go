// Package config holds runtime configuration parsed from CLI flags and env.
package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Mode is the top-level runtime mode.
type Mode string

const (
	ModeServe  Mode = "serve"
	ModeDaemon Mode = "daemon"

	defaultAddr          = "127.0.0.1:17890"
	defaultServeDataDir  = "./data"
	defaultDaemonDataDir = "/var/lib/sb-fox"
	defaultDaemonSocket  = "/var/run/sb-fox.sock"
)

// Action is a one-shot management operation requested by CLI flags.
type Action string

const (
	ActionServe         Action = ""
	ActionInstallDaemon Action = "install-daemon"
	ActionUpdate        Action = "update"
	ActionUninstall     Action = "uninstall"
)

// Config is the resolved runtime configuration.
type Config struct {
	Addr        string // listen address, e.g. "127.0.0.1:17890"
	DataDir     string // directory for the SQLite db and temp files
	DBPath      string // resolved sqlite path (DataDir/sb-fox.db)
	KernelPath  string // initial sing-box binary path (overridable in settings)
	SocketPath  string // daemon singleton socket path
	Mode        Mode   // serve or daemon
	Action      Action // management operation, if any
	Purge       bool   // uninstall removes config/data without prompting
	Dev         bool   // dev mode: serve API only, skip embedded frontend requirement
	ShowVersion bool   // print version and exit
}

// Parse reads flags (with env fallbacks) and returns the config.
func Parse(args []string) (*Config, error) {
	mode := ModeServe
	if os.Getenv("SB_FOX_DAEMON") == "1" {
		mode = ModeDaemon
	}

	name := "sb-fox"
	dataDirDefault := envOr("SB_FOX_DATA_DIR", defaultServeDataDir)
	if mode == ModeDaemon {
		name = "sb-fox daemon runtime"
		dataDirDefault = envOr("SB_FOX_DATA_DIR", defaultDaemonDataDir)
	}
	if hasFlag(args, "--daemon", "-d") {
		dataDirDefault = envOr("SB_FOX_DATA_DIR", defaultDaemonDataDir)
	}

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	addr := envOr("SB_FOX_ADDR", defaultAddr)
	dataDir := dataDirDefault
	kernel := envOr("SB_FOX_KERNEL", "sing-box")
	var installDaemon, update, uninstall, purge, dev, showVersion bool
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage of %s:\n", name)
		fmt.Fprintln(out, "  --addr, -a string")
		fmt.Fprintf(out, "\tlisten address (default %q)\n", addr)
		fmt.Fprintln(out, "  --data-dir, -D string")
		fmt.Fprintf(out, "\tdata directory (sqlite + temp) (default %q)\n", dataDir)
		fmt.Fprintln(out, "  --kernel, -k string")
		fmt.Fprintf(out, "\tsing-box binary path for config validation (default %q)\n", kernel)
		fmt.Fprintln(out, "  --daemon, -d")
		fmt.Fprintln(out, "\tinstall, enable and start the system daemon")
		fmt.Fprintln(out, "  --update, -u")
		fmt.Fprintln(out, "\tupdate installed binary")
		fmt.Fprintln(out, "  --uninstall, -U")
		fmt.Fprintln(out, "\tuninstall service and binary")
		fmt.Fprintln(out, "  --purge, -p")
		fmt.Fprintln(out, "\tremove config and data during uninstall")
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
	fs.BoolVar(&dev, "dev", false, "dev mode (serve API only)")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.BoolVar(&showVersion, "v", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unknown argument %q", fs.Arg(0))
	}
	action, err := resolveAction(installDaemon, update, uninstall, purge)
	if err != nil {
		return nil, err
	}
	if mode == ModeDaemon && action != ActionServe {
		return nil, errors.New("management flags cannot be used inside daemon runtime")
	}

	c := &Config{
		Addr:        addr,
		DataDir:     dataDir,
		KernelPath:  kernel,
		SocketPath:  "",
		Mode:        mode,
		Action:      action,
		Purge:       purge,
		Dev:         dev,
		ShowVersion: showVersion,
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func hasFlag(args []string, names ...string) bool {
	for _, arg := range args {
		for _, name := range names {
			if arg == name {
				return true
			}
		}
	}
	return false
}

func resolveAction(installDaemon, update, uninstall, purge bool) (Action, error) {
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
	if count > 1 {
		return "", errors.New("only one management flag can be used at a time")
	}
	if purge && !uninstall {
		return "", errors.New("--purge can only be used with --uninstall")
	}
	return action, nil
}
