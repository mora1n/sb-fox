// Package config holds runtime configuration parsed from CLI flags and env.
package config

import (
	"flag"
	"os"
	"path/filepath"
)

// Config is the resolved runtime configuration.
type Config struct {
	Addr        string // listen address, e.g. ":8080"
	DataDir     string // directory for the SQLite db and temp files
	DBPath      string // resolved sqlite path (DataDir/sb-fox.db)
	KernelPath  string // initial sing-box binary path (overridable in settings)
	Dev         bool   // dev mode: serve API only, skip embedded frontend requirement
	ShowVersion bool   // print version and exit
}

// Parse reads flags (with env fallbacks) and returns the config.
func Parse(args []string) (*Config, error) {
	fs := flag.NewFlagSet("sb-fox", flag.ContinueOnError)
	addr := fs.String("addr", envOr("SB_FOX_ADDR", ":8080"), "listen address")
	dataDir := fs.String("data-dir", envOr("SB_FOX_DATA_DIR", "./data"), "data directory (sqlite + temp)")
	kernel := fs.String("kernel", envOr("SB_FOX_KERNEL", "sing-box"), "sing-box binary path for config validation")
	dev := fs.Bool("dev", false, "dev mode (serve API only)")
	showVersion := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	c := &Config{
		Addr:        *addr,
		DataDir:     *dataDir,
		KernelPath:  *kernel,
		Dev:         *dev,
		ShowVersion: *showVersion,
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
