package manage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestInstallDaemonWritesServiceConfig(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "templates")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "fakeip.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var commands []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/app" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":{},"error":null}`))
	}))
	defer server.Close()

	err := InstallDaemon(Options{
		Root:              root,
		BinaryPath:        "/usr/local/bin/sb-fox",
		Addr:              server.URL,
		DataDir:           DefaultDataDir,
		KernelPath:        "sing-box",
		RegMode:           "on",
		LogLevel:          "debug",
		TemplateSourceDir: source,
		HTTPClient:        server.Client(),
		Runner: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return nil, nil
		},
		Stdout: io.Discard,
	})
	if err != nil {
		t.Fatalf("InstallDaemon: %v", err)
	}
	service, err := os.ReadFile(filepath.Join(root, "etc/systemd/system/sb-fox.service"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(service)
	if !strings.Contains(content, `Environment="SB_FOX_DAEMON=1"`) {
		t.Fatalf("service missing daemon env:\n%s", content)
	}
	for _, want := range []string{
		`Environment="SB_FOX_ADDR=` + server.URL + `"`,
		`Environment="SB_FOX_DATA_DIR=/var/lib/sb-fox"`,
		`Environment="SB_FOX_KERNEL=sing-box"`,
		`Environment="SB_FOX_REG=on"`,
		`Environment="SB_FOX_LOG=debug"`,
		"StandardOutput=journal",
		"StandardError=journal",
		"SyslogIdentifier=sb-fox",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("service missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "EnvironmentFile=") {
		t.Fatalf("service should not reference env file:\n%s", content)
	}
	if strings.Contains(content, "--socket") {
		t.Fatalf("service exposes socket:\n%s", content)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/sb-fox/sb-fox.env")); !os.IsNotExist(err) {
		t.Fatalf("env file should not be generated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/sb-fox/templates/fakeip.json")); err != nil {
		t.Fatalf("seed template not copied: %v", err)
	}
	got := strings.Join(commands, "\n")
	for _, want := range []string{
		"systemctl daemon-reload",
		"systemctl enable sb-fox",
		"systemctl restart sb-fox",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("unexpected systemctl calls, missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "systemctl enable --now sb-fox") {
		t.Fatalf("unexpected systemctl calls:\n%s", got)
	}
}

func TestControlDaemonStartCommands(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	for _, tc := range []struct {
		command string
		want    []string
		forbid  string
	}{
		{"", []string{"systemctl enable sb-fox", "systemctl restart sb-fox"}, "systemctl enable --now sb-fox"},
		{"enable", []string{"systemctl enable sb-fox", "systemctl restart sb-fox"}, "systemctl enable --now sb-fox"},
		{"start", []string{"systemctl start sb-fox"}, ""},
		{"restart", []string{"systemctl restart sb-fox"}, ""},
	} {
		name := tc.command
		if name == "" {
			name = "default"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(t.TempDir(), "templates")
			if err := os.MkdirAll(source, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "fakeip.json"), []byte(`{"ok":true}`), 0o644); err != nil {
				t.Fatal(err)
			}
			healthChecks := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/app" {
					http.NotFound(w, r)
					return
				}
				healthChecks++
				_, _ = w.Write([]byte(`{"data":{},"error":null}`))
			}))
			defer server.Close()

			var commands []string
			err := ControlDaemon(Options{
				Root:              root,
				BinaryPath:        "/usr/local/bin/sb-fox",
				Addr:              server.URL,
				DataDir:           DefaultDataDir,
				TemplateSourceDir: source,
				HTTPClient:        server.Client(),
				Stdout:            io.Discard,
				Runner: func(name string, args ...string) ([]byte, error) {
					commands = append(commands, name+" "+strings.Join(args, " "))
					return nil, nil
				},
			}, tc.command)
			if err != nil {
				t.Fatalf("ControlDaemon(%q): %v", tc.command, err)
			}
			if _, err := os.Stat(filepath.Join(root, "etc/systemd/system/sb-fox.service")); err != nil {
				t.Fatalf("service file not written: %v", err)
			}
			got := strings.Join(commands, "\n")
			for _, want := range append([]string{"systemctl daemon-reload"}, tc.want...) {
				if !strings.Contains(got, want) {
					t.Fatalf("unexpected systemctl calls for %s, missing %q:\n%s", tc.command, want, got)
				}
			}
			if tc.forbid != "" && strings.Contains(got, tc.forbid) {
				t.Fatalf("unexpected forbidden systemctl call for %s:\n%s", tc.command, got)
			}
			if healthChecks == 0 {
				t.Fatal("expected health-check after daemon start command")
			}
		})
	}
}

func TestControlDaemonStopCommands(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	for _, tc := range []struct {
		command string
		want    string
	}{
		{"stop", "systemctl stop sb-fox"},
		{"disable", "systemctl disable --now sb-fox"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			root := t.TempDir()
			var commands []string
			err := ControlDaemon(Options{
				Root:   root,
				Stdout: io.Discard,
				Runner: func(name string, args ...string) ([]byte, error) {
					commands = append(commands, name+" "+strings.Join(args, " "))
					return nil, nil
				},
			}, tc.command)
			if err != nil {
				t.Fatalf("ControlDaemon(%q): %v", tc.command, err)
			}
			got := strings.Join(commands, "\n")
			if got != tc.want {
				t.Fatalf("systemctl calls = %q, want %q", got, tc.want)
			}
			if _, err := os.Stat(filepath.Join(root, "etc/systemd/system/sb-fox.service")); !os.IsNotExist(err) {
				t.Fatalf("stop-like command should not write service, stat err=%v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "var/lib/sb-fox/sb-fox.db")); !os.IsNotExist(err) {
				t.Fatalf("stop-like command should not create admin db, stat err=%v", err)
			}
		})
	}
}

func TestControlDaemonPrintsInitialAdminPassword(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	t.Setenv("SB_FOX_ADMIN_PASSWORD", "")
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "templates")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "fakeip.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/app" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":{},"error":null}`))
	}))
	defer server.Close()

	var output bytes.Buffer
	opts := Options{
		Root:              root,
		BinaryPath:        "/usr/local/bin/sb-fox",
		Addr:              server.URL,
		DataDir:           DefaultDataDir,
		TemplateSourceDir: source,
		HTTPClient:        server.Client(),
		Stdout:            &output,
		Runner: func(name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	}
	if err := ControlDaemon(opts, "enable"); err != nil {
		t.Fatalf("ControlDaemon first: %v", err)
	}
	first := output.String()
	if !strings.Contains(first, "initial admin created") ||
		!strings.Contains(first, "username: admin") ||
		!strings.Contains(first, "password:") {
		t.Fatalf("initial password output missing:\n%s", first)
	}

	output.Reset()
	if err := ControlDaemon(opts, "enable"); err != nil {
		t.Fatalf("ControlDaemon second: %v", err)
	}
	second := output.String()
	if strings.Contains(second, "password:") {
		t.Fatalf("second daemon command should not print password:\n%s", second)
	}
	if !strings.Contains(second, "admin already exists") || !strings.Contains(second, "sudo sb-fox -P") {
		t.Fatalf("existing admin reset hint missing:\n%s", second)
	}

	output.Reset()
	if err := ControlDaemon(opts, "restart"); err != nil {
		t.Fatalf("ControlDaemon restart: %v", err)
	}
	restart := output.String()
	if strings.Contains(restart, "password:") || strings.Contains(restart, "admin already exists") {
		t.Fatalf("restart should not print admin password or reset hint:\n%s", restart)
	}
}

func TestControlDaemonDoesNotPrintEnvPassword(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	t.Setenv("SB_FOX_ADMIN_PASSWORD", "known-password")
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "templates")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "fakeip.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/app" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":{},"error":null}`))
	}))
	defer server.Close()

	var output bytes.Buffer
	err := ControlDaemon(Options{
		Root:              root,
		BinaryPath:        "/usr/local/bin/sb-fox",
		Addr:              server.URL,
		DataDir:           DefaultDataDir,
		TemplateSourceDir: source,
		HTTPClient:        server.Client(),
		Stdout:            &output,
		Runner: func(name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	}, "enable")
	if err != nil {
		t.Fatalf("ControlDaemon: %v", err)
	}
	got := output.String()
	if !strings.Contains(got, "SB_FOX_ADMIN_PASSWORD") || strings.Contains(got, "known-password") {
		t.Fatalf("env password output = %q", got)
	}
}

func TestControlDaemonDoesNotIgnoreSystemctlErrors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}

	err := ControlDaemon(Options{
		Root:   t.TempDir(),
		Stdout: io.Discard,
		Runner: func(name string, args ...string) ([]byte, error) {
			return []byte("Access denied"), errors.New("exit status 1")
		},
	}, "stop")
	if err == nil || !strings.Contains(err.Error(), "Access denied") {
		t.Fatalf("ControlDaemon error = %v", err)
	}
}

func TestUpdateReplacesBinaryWithoutPrintingSourceURL(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	root := t.TempDir()
	current := filepath.Join(root, "usr/local/bin/sb-fox")
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	archiveName, err := releaseArchiveName("v9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	archiveBytes := makeReleaseArchive(t, archiveName, "new")
	sum := sha256.Sum256(archiveBytes)
	var output bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
		case "/v9.9.9/" + archiveName:
			_, _ = w.Write(archiveBytes)
		case "/v9.9.9/SHA256SUMS":
			_, _ = fmt.Fprintf(w, "%x  %s\n", sum, archiveName)
		case "/api/app":
			_, _ = w.Write([]byte(`{"data":{},"error":null}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	err = Update(Options{
		Root:           root,
		BinaryPath:     current,
		Addr:           server.URL,
		HTTPClient:     server.Client(),
		LatestURL:      server.URL + "/latest",
		DownloadBase:   server.URL,
		HealthTimeout:  100 * time.Millisecond,
		HealthInterval: 10 * time.Millisecond,
		Stdout:         &output,
		Runner: func(name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("binary = %q, want new", data)
	}
	backups, err := filepath.Glob(current + ".bak-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("backup should be removed after successful update: %v", backups)
	}
	if strings.Contains(output.String(), server.URL) || strings.Contains(output.String(), "mora1n") {
		t.Fatalf("update output exposed source info:\n%s", output.String())
	}
}

func TestUpdateRollsBackWhenHealthCheckFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	root := t.TempDir()
	current := filepath.Join(root, "usr/local/bin/sb-fox")
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	archiveName, err := releaseArchiveName("v9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	archiveBytes := makeReleaseArchive(t, archiveName, "new")
	sum := sha256.Sum256(archiveBytes)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
		case "/v9.9.9/" + archiveName:
			_, _ = w.Write(archiveBytes)
		case "/v9.9.9/SHA256SUMS":
			_, _ = fmt.Fprintf(w, "%x  %s\n", sum, archiveName)
		case "/api/app":
			http.Error(w, "down", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	err = Update(Options{
		Root:           root,
		BinaryPath:     current,
		Addr:           server.URL,
		HTTPClient:     server.Client(),
		LatestURL:      server.URL + "/latest",
		DownloadBase:   server.URL,
		HealthTimeout:  50 * time.Millisecond,
		HealthInterval: 10 * time.Millisecond,
		Stdout:         io.Discard,
		Runner: func(name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected update error")
	}
	data, readErr := os.ReadFile(current)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "old" {
		t.Fatalf("binary = %q, want rollback to old", data)
	}
}

func TestUninstallKeepsConfigByDefaultAnswer(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	root := t.TempDir()
	current := filepath.Join(root, "usr/local/bin/sb-fox")
	configDir := filepath.Join(root, "etc/sb-fox")
	dataDir := filepath.Join(root, "var/lib/sb-fox")
	for _, dir := range []string{filepath.Dir(current), configDir, dataDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(current, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "sb-fox.env"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "sb-fox.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Uninstall(Options{
		Root:       root,
		BinaryPath: current,
		Stdin:      strings.NewReader("\n"),
		Stdout:     io.Discard,
		Runner: func(name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(current); !os.IsNotExist(err) {
		t.Fatalf("binary still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(configDir); err != nil {
		t.Fatalf("config should be preserved: %v", err)
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Fatalf("data should be preserved: %v", err)
	}
}

func TestUninstallPurgeRemovesConfigAndData(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	root := t.TempDir()
	current := filepath.Join(root, "usr/local/bin/sb-fox")
	configDir := filepath.Join(root, "etc/sb-fox")
	dataDir := filepath.Join(root, "var/lib/sb-fox")
	socketPath := filepath.Join(root, "var/run/sb-fox.sock")
	for _, dir := range []string{filepath.Dir(current), configDir, dataDir, filepath.Dir(socketPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{current, filepath.Join(configDir, "sb-fox.env"), filepath.Join(dataDir, "sb-fox.db"), socketPath} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	err := Uninstall(Options{
		Root:       root,
		BinaryPath: current,
		Purge:      true,
		Stdout:     io.Discard,
		Runner: func(name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	for _, path := range []string{current, configDir, dataDir, socketPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed, stat err=%v", path, err)
		}
	}
}

func TestUninstallPurgeContinuesWhenServiceMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	root := t.TempDir()
	current := filepath.Join(root, "usr/local/bin/sb-fox")
	configDir := filepath.Join(root, "etc/sb-fox")
	dataDir := filepath.Join(root, "var/lib/sb-fox")
	socketPath := filepath.Join(root, "var/run/sb-fox.sock")
	for _, dir := range []string{filepath.Dir(current), configDir, dataDir, filepath.Dir(socketPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{current, filepath.Join(configDir, "sb-fox.env"), filepath.Join(dataDir, "sb-fox.db"), socketPath} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var commands []string
	err := Uninstall(Options{
		Root:       root,
		BinaryPath: current,
		Purge:      true,
		Stdout:     io.Discard,
		Runner: func(name string, args ...string) ([]byte, error) {
			cmd := name + " " + strings.Join(args, " ")
			commands = append(commands, cmd)
			switch strings.Join(args, " ") {
			case "disable --now sb-fox", "reset-failed sb-fox":
				return []byte("Unit sb-fox.service could not be found."), errors.New("exit status 1")
			default:
				return nil, nil
			}
		},
	})
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	for _, path := range []string{current, configDir, dataDir, socketPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed, stat err=%v", path, err)
		}
	}
	got := strings.Join(commands, "\n")
	if !strings.Contains(got, "systemctl daemon-reload") || !strings.Contains(got, "systemctl reset-failed sb-fox") {
		t.Fatalf("missing cleanup systemctl calls:\n%s", got)
	}
}

func TestUninstallDoesNotIgnoreSystemctlErrors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd management is Linux-only")
	}
	root := t.TempDir()
	current := filepath.Join(root, "usr/local/bin/sb-fox")
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := Uninstall(Options{
		Root:       root,
		BinaryPath: current,
		Purge:      true,
		Stdout:     io.Discard,
		Runner: func(name string, args ...string) ([]byte, error) {
			return []byte("Access denied"), errors.New("exit status 1")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "Access denied") {
		t.Fatalf("Uninstall error = %v", err)
	}
	if _, err := os.Stat(current); err != nil {
		t.Fatalf("binary should remain after failed uninstall: %v", err)
	}
}

func TestEnvAddrReadsServiceEnvironment(t *testing.T) {
	root := t.TempDir()
	servicePath := filepath.Join(root, "etc/systemd/system/sb-fox.service")
	if err := os.MkdirAll(filepath.Dir(servicePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(servicePath, []byte(serviceContent("/usr/local/bin/sb-fox", Options{
		Addr:       "localhost:19090",
		DataDir:    DefaultDataDir,
		KernelPath: "sing-box",
		RegMode:    "off",
		LogLevel:   "info",
	})), 0o644); err != nil {
		t.Fatal(err)
	}

	got := envAddr(Options{Root: root, Addr: DefaultAddr})
	if got != "localhost:19090" {
		t.Fatalf("envAddr = %q, want service address", got)
	}
}

func makeReleaseArchive(t *testing.T, archiveName, binaryContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	root := strings.TrimSuffix(archiveName, ".tar.gz")
	body := []byte(binaryContent)
	if err := tw.WriteHeader(&tar.Header{
		Name: root + "/sb-fox",
		Mode: 0o755,
		Size: int64(len(body)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
