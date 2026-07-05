// Package manage implements Linux service installation, update and uninstall
// operations for sb-fox.
package manage

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	ServiceName       = "sb-fox"
	DefaultAddr       = "127.0.0.1:7878"
	DefaultDataDir    = "/var/lib/sb-fox"
	DefaultSocketPath = "/var/run/sb-fox.sock"
)

// Runner executes an external command and returns its combined output.
type Runner func(name string, args ...string) ([]byte, error)

// Options controls management operations. Root is only for tests; generated
// service content still uses the real Linux paths.
type Options struct {
	Addr              string
	DataDir           string
	KernelPath        string
	BinaryPath        string
	TemplateSourceDir string
	Root              string
	Version           string
	Purge             bool
	RegMode           string
	LogLevel          string
	Stdin             io.Reader
	Stdout            io.Writer
	HTTPClient        *http.Client
	Runner            Runner
	LatestURL         string
	DownloadBase      string
	GitHubToken       string
	HealthTimeout     time.Duration
	HealthInterval    time.Duration
}

func (o Options) withDefaults() Options {
	if o.Addr == "" {
		o.Addr = DefaultAddr
	}
	if o.DataDir == "" {
		o.DataDir = DefaultDataDir
	}
	if o.KernelPath == "" {
		o.KernelPath = "sing-box"
	}
	if o.RegMode == "" {
		o.RegMode = "off"
	}
	if o.LogLevel == "" {
		o.LogLevel = "info"
	}
	if o.Stdin == nil {
		o.Stdin = os.Stdin
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if o.Runner == nil {
		o.Runner = defaultRunner
	}
	if o.LatestURL == "" {
		o.LatestURL = defaultLatestURL
	}
	if o.DownloadBase == "" {
		o.DownloadBase = defaultDownloadBase
	}
	if o.GitHubToken == "" {
		o.GitHubToken = releaseTokenFromEnv()
	}
	if o.HealthTimeout == 0 {
		o.HealthTimeout = 20 * time.Second
	}
	if o.HealthInterval == 0 {
		o.HealthInterval = 500 * time.Millisecond
	}
	return o
}

func (o Options) rooted(path string) string {
	if o.Root == "" {
		return path
	}
	return filepath.Join(o.Root, strings.TrimPrefix(path, string(os.PathSeparator)))
}

func requireLinuxRoot(root string) error {
	if runtime.GOOS != "linux" {
		return errors.New("operation is only supported on Linux")
	}
	if root == "" && os.Geteuid() != 0 {
		return errors.New("operation requires root")
	}
	return nil
}

func resolveBinaryPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.EvalSymlinks(exe)
}

func defaultRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
