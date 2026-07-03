// Package manage implements Linux service installation, update and uninstall
// operations for sb-fox.
package manage

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	ServiceName       = "sb-fox"
	DefaultAddr       = "127.0.0.1:7878"
	DefaultDataDir    = "/var/lib/sb-fox"
	DefaultSocketPath = "/var/run/sb-fox.sock"

	defaultLatestURL    = "https://api.github.com/repos/mora1n/sb-fox/releases/latest"
	defaultDownloadBase = "https://github.com/mora1n/sb-fox/releases/download"
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
	if o.HealthTimeout == 0 {
		o.HealthTimeout = 20 * time.Second
	}
	if o.HealthInterval == 0 {
		o.HealthInterval = 500 * time.Millisecond
	}
	return o
}

// InstallDaemon writes the systemd service and enables it immediately.
func InstallDaemon(opts Options) error {
	opts = opts.withDefaults()
	if err := requireLinuxRoot(opts.Root); err != nil {
		return err
	}
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
	if err := runSystemctl(opts, "enable", "--now", ServiceName); err != nil {
		return err
	}
	if err := HealthCheck(opts, envAddr(opts)); err != nil {
		return fmt.Errorf("health-check failed: %w", err)
	}
	fmt.Fprintln(opts.Stdout, "sb-fox service installed and started")
	return nil
}

// Update replaces the installed binary from the latest release and rolls back
// if restart or health-check fails.
func Update(opts Options) error {
	opts = opts.withDefaults()
	if err := requireLinuxRoot(opts.Root); err != nil {
		return err
	}
	if runtime.GOOS != "linux" {
		return errors.New("update is only supported on Linux")
	}
	target, err := resolveBinaryPath(opts.BinaryPath)
	if err != nil {
		return err
	}
	latest, err := fetchLatest(opts)
	if err != nil {
		return err
	}
	archiveName, err := releaseArchiveName(latest.TagName)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "sb-fox-update-*")
	if err != nil {
		return fmt.Errorf("create update workspace: %w", err)
	}
	defer os.RemoveAll(tmp)

	fmt.Fprintf(opts.Stdout, "latest version: %s\n", latest.TagName)
	archivePath := filepath.Join(tmp, archiveName)
	sumPath := filepath.Join(tmp, "SHA256SUMS")
	if err := download(opts, releaseURL(opts, latest.TagName, archiveName), archivePath, "download archive"); err != nil {
		return err
	}
	if err := download(opts, releaseURL(opts, latest.TagName, "SHA256SUMS"), sumPath, "download checksum"); err != nil {
		return err
	}
	if err := verifySHA256(archivePath, sumPath, archiveName); err != nil {
		return err
	}
	fmt.Fprintln(opts.Stdout, "checksum verified")
	if err := extractTarGz(archivePath, tmp); err != nil {
		return err
	}
	newBinary, err := findBinary(tmp)
	if err != nil {
		return err
	}
	backup := target + ".bak-" + time.Now().UTC().Format("20060102-150405")
	if err := copyFile(target, backup, 0o755); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}
	fmt.Fprintf(opts.Stdout, "backup created: %s\n", filepath.Base(backup))
	if err := replaceBinary(target, newBinary); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	fmt.Fprintln(opts.Stdout, "binary replaced")

	if err := restartAndCheck(opts); err != nil {
		fmt.Fprintln(opts.Stdout, "health-check failed, rolling back")
		if rbErr := replaceBinary(target, backup); rbErr != nil {
			return fmt.Errorf("rollback failed after update error: %v; rollback: %w", err, rbErr)
		}
		if checkErr := restartAndCheck(opts); checkErr != nil {
			return fmt.Errorf("rollback health-check failed after update error: %v; rollback check: %w", err, checkErr)
		}
		return fmt.Errorf("update rolled back: %w", err)
	}
	fmt.Fprintln(opts.Stdout, "update completed")
	return nil
}

// Uninstall stops the service and removes installed files. Config/data are
// preserved unless Purge is true or the user answers no to the keep prompt.
func Uninstall(opts Options) error {
	opts = opts.withDefaults()
	if err := requireLinuxRoot(opts.Root); err != nil {
		return err
	}
	if err := runSystemctl(opts, "disable", "--now", ServiceName); err != nil {
		return err
	}
	if err := removeIfExists(opts.rooted("/etc/systemd/system/sb-fox.service")); err != nil {
		return err
	}
	if err := runSystemctl(opts, "daemon-reload"); err != nil {
		return err
	}
	binaryPath, err := resolveBinaryPath(opts.BinaryPath)
	if err != nil {
		return err
	}
	if err := removeIfExists(binaryPath); err != nil {
		return err
	}
	purge := opts.Purge
	if !purge {
		keep, err := askKeepData(opts)
		if err != nil {
			return err
		}
		purge = !keep
	}
	if purge {
		if err := os.RemoveAll(opts.rooted("/etc/sb-fox")); err != nil {
			return fmt.Errorf("remove config directory: %w", err)
		}
		if err := os.RemoveAll(opts.rooted(opts.DataDir)); err != nil {
			return fmt.Errorf("remove data directory: %w", err)
		}
		if err := removeIfExists(opts.rooted(DefaultSocketPath)); err != nil {
			return err
		}
		fmt.Fprintln(opts.Stdout, "sb-fox uninstalled and data removed")
		return nil
	}
	fmt.Fprintln(opts.Stdout, "sb-fox uninstalled; config and data preserved")
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

func defaultRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func fetchLatest(opts Options) (struct {
	TagName string `json:"tag_name"`
}, error) {
	var latest struct {
		TagName string `json:"tag_name"`
	}
	resp, err := opts.HTTPClient.Get(opts.LatestURL)
	if err != nil {
		return latest, errors.New("release metadata failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return latest, errors.New("release metadata failed")
	}
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return latest, errors.New("release metadata failed")
	}
	if latest.TagName == "" {
		return latest, errors.New("release metadata missing version")
	}
	return latest, nil
}

func releaseArchiveName(tag string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", errors.New("update is only supported on Linux")
	}
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return fmt.Sprintf("sb-fox-linux-%s-%s.tar.gz", runtime.GOARCH, tag), nil
	default:
		return "", fmt.Errorf("unsupported architecture %s", runtime.GOARCH)
	}
}

func releaseURL(opts Options, tag, name string) string {
	return strings.TrimRight(opts.DownloadBase, "/") + "/" + tag + "/" + name
}

func download(opts Options, url, path, label string) error {
	resp, err := opts.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("%s failed", label)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s failed", label)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%s failed", label)
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("%s failed", label)
	}
	return nil
}

func verifySHA256(archivePath, sumPath, archiveName string) error {
	want, err := checksumFor(sumPath, archiveName)
	if err != nil {
		return err
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("read archive for checksum: %w", err)
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if got != want {
		return errors.New("checksum verification failed")
	}
	return nil
}

func checksumFor(path, archiveName string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open checksum file: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == archiveName {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read checksum file: %w", err)
	}
	return "", errors.New("checksum entry not found")
}

func extractTarGz(path, dir string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("extract archive: %w", err)
		}
		clean := filepath.Clean(header.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return errors.New("archive contains unsafe path")
		}
		target := filepath.Join(dir, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create archive directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create archive parent: %w", err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create archive file: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return fmt.Errorf("write archive file: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close archive file: %w", err)
			}
		}
	}
}

func findBinary(dir string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "sb-fox" {
			return nil
		}
		found = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", fmt.Errorf("find release binary: %w", err)
	}
	if found == "" {
		return "", errors.New("release binary not found")
	}
	return found, nil
}

func replaceBinary(target, source string) error {
	tmp := target + ".new"
	if err := copyFile(source, tmp, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, target)
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

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func askKeepData(opts Options) (bool, error) {
	fmt.Fprint(opts.Stdout, "保留配置和数据? [Y/n]: ")
	reader := bufio.NewReader(opts.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "" || answer == "y" || answer == "yes", nil
}
