package manage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
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

func TestInstallDaemonWritesServiceAndEnv(t *testing.T) {
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
		RegExplicit:       true,
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
	if !strings.Contains(content, "Environment=SB_FOX_DAEMON=1") {
		t.Fatalf("service missing daemon env:\n%s", content)
	}
	if strings.Contains(content, "--socket") {
		t.Fatalf("service exposes socket:\n%s", content)
	}
	env, err := os.ReadFile(filepath.Join(root, "etc/sb-fox/sb-fox.env"))
	if err != nil {
		t.Fatal(err)
	}
	envText := string(env)
	if !strings.Contains(envText, "SB_FOX_DATA_DIR=/var/lib/sb-fox") {
		t.Fatalf("env missing data dir:\n%s", env)
	}
	if !strings.Contains(envText, "SB_FOX_REG=on") {
		t.Fatalf("env missing registration switch:\n%s", env)
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/sb-fox/templates/fakeip.json")); err != nil {
		t.Fatalf("seed template not copied: %v", err)
	}
	got := strings.Join(commands, "\n")
	if !strings.Contains(got, "systemctl daemon-reload") || !strings.Contains(got, "systemctl enable --now sb-fox") {
		t.Fatalf("unexpected systemctl calls:\n%s", got)
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
