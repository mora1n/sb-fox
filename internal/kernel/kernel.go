// Package kernel shells out to a sing-box binary to validate and format
// configs. When the binary is absent or non-executable, operations report an
// "unavailable" status rather than failing — validation is advisory, never a
// hard gate on config generation.
package kernel

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Status is the outcome of a validation attempt.
type Status string

const (
	StatusOK          Status = "ok"          // config valid
	StatusInvalid     Status = "invalid"     // kernel rejected the config
	StatusUnavailable Status = "unavailable" // kernel binary missing/unusable
)

// Result is a validation or format outcome.
type Result struct {
	Status   Status `json:"status"`
	Messages string `json:"messages,omitempty"`  // kernel stderr/stdout on invalid
	Formatted string `json:"formatted,omitempty"` // pretty config on successful format
}

// Kernel invokes a sing-box binary at Path. An empty Path disables validation.
type Kernel struct {
	Path    string
	DataDir string        // where temp config files are written
	Timeout time.Duration // per-invocation timeout
}

// New returns a Kernel. A zero timeout defaults to 15s.
func New(path, dataDir string, timeout time.Duration) *Kernel {
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &Kernel{Path: path, DataDir: dataDir, Timeout: timeout}
}

// Available reports whether the configured binary looks usable.
func (k *Kernel) Available() bool {
	if strings.TrimSpace(k.Path) == "" {
		return false
	}
	if _, err := exec.LookPath(k.Path); err == nil {
		return true
	}
	info, err := os.Stat(k.Path)
	return err == nil && !info.IsDir()
}

// Version returns the sing-box version string, or an error if unavailable.
func (k *Kernel) Version() (string, error) {
	if !k.Available() {
		return "", errors.New("kernel: binary unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), k.Timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, k.Path, "version").CombinedOutput()
	if err != nil {
		return "", err
	}
	// first line, e.g. "sing-box version 1.14.0-alpha.33"
	line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	return strings.TrimSpace(line), nil
}

// Check runs `sing-box check -c <tmp>` on the given config bytes.
func (k *Kernel) Check(config []byte) Result {
	return k.run(config, false)
}

// Format runs `sing-box format -c <tmp>` and returns the pretty output.
func (k *Kernel) Format(config []byte) Result {
	return k.run(config, true)
}

func (k *Kernel) run(config []byte, format bool) Result {
	if !k.Available() {
		return Result{Status: StatusUnavailable}
	}
	tmp, cleanup, err := k.writeTemp(config)
	if err != nil {
		return Result{Status: StatusUnavailable, Messages: err.Error()}
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), k.Timeout)
	defer cancel()

	sub := "check"
	if format {
		sub = "format"
	}
	cmd := exec.CommandContext(ctx, k.Path, sub, "-c", tmp)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return Result{Status: StatusInvalid, Messages: msg}
	}
	res := Result{Status: StatusOK}
	if format {
		res.Formatted = stdout.String()
	}
	return res
}

// writeTemp writes config to a 0600 temp file in DataDir and returns a cleanup.
func (k *Kernel) writeTemp(config []byte) (string, func(), error) {
	dir := k.DataDir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp(dir, "sb-fox-check-*.json")
	if err != nil {
		return "", nil, err
	}
	name := f.Name()
	cleanup := func() { os.Remove(name) }
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if _, err := f.Write(config); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	_ = filepath.Base(name) // name is absolute already
	return name, cleanup, nil
}
