package manage

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mora1n/sb-fox/internal/bootstrap"
	"github.com/mora1n/sb-fox/internal/store"
)

// InstallDaemon writes the systemd service and enables it immediately.
func InstallDaemon(opts Options) error {
	return ControlDaemon(opts, "enable")
}

// ControlDaemon manages the systemd service. The empty command defaults to
// enable for compatibility with the original --daemon behavior.
func ControlDaemon(opts Options, command string) error {
	opts = opts.withDefaults()
	if err := requireLinuxRoot(opts.Root); err != nil {
		return err
	}
	if command == "" {
		command = "enable"
	}
	switch command {
	case "enable":
		if err := prepareDaemonService(opts); err != nil {
			return err
		}
		if err := ensureDaemonAdmin(opts, true); err != nil {
			return err
		}
		if err := runSystemctl(opts, "enable", ServiceName); err != nil {
			return err
		}
		if err := runSystemctl(opts, "restart", ServiceName); err != nil {
			return err
		}
		return finishDaemonStart(opts, "sb-fox service enabled and restarted")
	case "start":
		if err := prepareDaemonService(opts); err != nil {
			return err
		}
		if err := ensureDaemonAdmin(opts, false); err != nil {
			return err
		}
		if err := runSystemctl(opts, "start", ServiceName); err != nil {
			return err
		}
		return finishDaemonStart(opts, "sb-fox service started")
	case "restart":
		if err := prepareDaemonService(opts); err != nil {
			return err
		}
		if err := ensureDaemonAdmin(opts, false); err != nil {
			return err
		}
		if err := runSystemctl(opts, "restart", ServiceName); err != nil {
			return err
		}
		return finishDaemonStart(opts, "sb-fox service restarted")
	case "stop":
		if err := runSystemctl(opts, "stop", ServiceName); err != nil {
			return err
		}
		fmt.Fprintln(opts.Stdout, "sb-fox service stopped")
		return nil
	case "disable":
		if err := runSystemctl(opts, "disable", "--now", ServiceName); err != nil {
			return err
		}
		fmt.Fprintln(opts.Stdout, "sb-fox service disabled and stopped")
		return nil
	default:
		return fmt.Errorf("unsupported daemon command %q", command)
	}
}

func prepareDaemonService(opts Options) error {
	binaryPath, err := resolveBinaryPath(opts.BinaryPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(opts.rooted(filepath.Join(opts.DataDir, "templates")), 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	if err := seedTemplates(opts); err != nil {
		return err
	}
	servicePath := opts.rooted("/etc/systemd/system/sb-fox.service")
	if err := os.MkdirAll(filepath.Dir(servicePath), 0o755); err != nil {
		return fmt.Errorf("create service directory: %w", err)
	}
	if err := os.WriteFile(servicePath, []byte(serviceContent(binaryPath, opts)), 0o644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}
	if err := runSystemctl(opts, "daemon-reload"); err != nil {
		return err
	}
	return nil
}

func ensureDaemonAdmin(opts Options, printExistingHint bool) error {
	db, err := store.Open(filepath.Join(opts.rooted(opts.DataDir), "sb-fox.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := bootstrap.EnsureAdmin(db)
	if err != nil {
		return err
	}
	printAdminInit(opts.Stdout, result, printExistingHint)
	return nil
}

func printAdminInit(w io.Writer, result *bootstrap.AdminInit, printExistingHint bool) {
	if result == nil {
		return
	}
	if !result.Created {
		if printExistingHint {
			fmt.Fprintln(w, "admin already exists; existing password cannot be shown")
			fmt.Fprintln(w, "reset admin: sudo sb-fox -P")
		}
		return
	}
	if result.Generated {
		fmt.Fprintf(w, "initial admin created\nusername: %s\npassword: %s\n", result.Username, result.Password)
		return
	}
	fmt.Fprintf(w, "initial admin created from SB_FOX_ADMIN_PASSWORD\nusername: %s\n", result.Username)
}

func finishDaemonStart(opts Options, message string) error {
	if err := HealthCheck(opts, envAddr(opts)); err != nil {
		return fmt.Errorf("health-check failed: %w", err)
	}
	fmt.Fprintln(opts.Stdout, message)
	return nil
}

// HealthCheck waits until /api/app responds successfully.
func HealthCheck(opts Options, addr string) error {
	opts = opts.withDefaults()
	url, err := healthURL(addr)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(opts.HealthTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := opts.HTTPClient.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(opts.HealthInterval)
	}
	return lastErr
}

func serviceContent(binaryPath string, opts Options) string {
	return fmt.Sprintf(`[Unit]
Description=sb-fox
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
%s%s%s%s%s%s
WorkingDirectory=%s
ExecStart=%s
Restart=on-failure
RestartSec=3
StandardOutput=journal
StandardError=journal
SyslogIdentifier=sb-fox

[Install]
WantedBy=multi-user.target
`,
		systemdEnv("SB_FOX_DAEMON", "1"),
		systemdEnv("SB_FOX_ADDR", opts.Addr),
		systemdEnv("SB_FOX_DATA_DIR", opts.DataDir),
		systemdEnv("SB_FOX_KERNEL", opts.KernelPath),
		systemdEnv("SB_FOX_REG", opts.RegMode),
		systemdEnv("SB_FOX_LOG", opts.LogLevel),
		opts.DataDir,
		binaryPath)
}

func systemdEnv(key, value string) string {
	return "Environment=" + strconv.Quote(key+"="+value) + "\n"
}

func seedTemplates(opts Options) error {
	target := opts.rooted(filepath.Join(opts.DataDir, "templates"))
	if _, err := os.Stat(filepath.Join(target, "fakeip.json")); err == nil {
		return nil
	}
	source := opts.TemplateSourceDir
	if source == "" {
		source = filepath.Join("data", "templates")
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("read seed template source %s: %w", source, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		src := filepath.Join(source, entry.Name())
		dst := filepath.Join(target, entry.Name())
		if err := copyFile(src, dst, 0o644); err != nil {
			return fmt.Errorf("copy seed template: %w", err)
		}
	}
	return nil
}

func runSystemctl(opts Options, args ...string) error {
	out, err := opts.Runner("systemctl", args...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("systemctl %s failed: %s", strings.Join(args, " "), msg)
	}
	return nil
}

func runSystemctlAllowMissingUnit(opts Options, args ...string) error {
	out, err := opts.Runner("systemctl", args...)
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	if systemctlUnitMissing(msg) {
		return nil
	}
	return fmt.Errorf("systemctl %s failed: %s", strings.Join(args, " "), msg)
}

func systemctlUnitMissing(msg string) bool {
	msg = strings.ToLower(msg)
	if !strings.Contains(msg, strings.ToLower(ServiceName)) {
		return false
	}
	for _, marker := range []string{
		"not loaded",
		"not found",
		"could not be found",
		"does not exist",
		"no such file",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func restartAndCheck(opts Options) error {
	if err := runSystemctl(opts, "restart", ServiceName); err != nil {
		return err
	}
	return HealthCheck(opts, envAddr(opts))
}

func envAddr(opts Options) string {
	if addr, ok := serviceEnvValue(opts, "SB_FOX_ADDR"); ok {
		return addr
	}
	if addr, ok := legacyEnvValue(opts, "SB_FOX_ADDR"); ok {
		return addr
	}
	return opts.Addr
}

func serviceEnvValue(opts Options, key string) (string, bool) {
	file, err := os.Open(opts.rooted("/etc/systemd/system/sb-fox.service"))
	if err != nil {
		return "", false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "Environment=") {
			continue
		}
		value := strings.TrimPrefix(line, "Environment=")
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		if envKey, envValue, ok := strings.Cut(value, "="); ok && envKey == key {
			return envValue, true
		}
	}
	return "", false
}

func legacyEnvValue(opts Options, key string) (string, bool) {
	file, err := os.Open(opts.rooted("/etc/sb-fox/sb-fox.env"))
	if err != nil {
		return "", false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	prefix := key + "="
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix), true
		}
	}
	return "", false
}

func healthURL(addr string) (string, error) {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/") + "/api/app", nil
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid listen address %q", addr)
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/api/app", nil
}
