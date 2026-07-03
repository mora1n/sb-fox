// Command sb-fox is a single-binary sing-box config/subscription management
// panel: it serves an embedded web UI and an API for editing nodes, templates
// and profiles, and renders full config.json documents via tokenized links.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mora1n/sb-fox/internal/api"
	"github.com/mora1n/sb-fox/internal/assets"
	"github.com/mora1n/sb-fox/internal/auth"
	"github.com/mora1n/sb-fox/internal/bootstrap"
	"github.com/mora1n/sb-fox/internal/config"
	"github.com/mora1n/sb-fox/internal/kernel"
	"github.com/mora1n/sb-fox/internal/manage"
	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
	"github.com/mora1n/sb-fox/internal/subfetch"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

type runtimeLogLevel int

const (
	levelError runtimeLogLevel = iota
	levelWarn
	levelInfo
	levelDebug
)

var activeLogLevel = levelInfo

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatalf("sb-fox: %v", err)
	}
}

func run(args []string) error {
	cfg, err := config.Parse(args)
	if err != nil {
		return err
	}
	setLogLevel(cfg.LogLevel)
	if cfg.ShowVersion {
		fmt.Printf("sb-fox %s\n", version)
		return nil
	}
	switch cfg.Action {
	case config.ActionInstallDaemon:
		opts := manage.Options{
			Addr:       cfg.Addr,
			DataDir:    cfg.DataDir,
			KernelPath: cfg.KernelPath,
			RegMode:    cfg.RegMode,
			LogLevel:   cfg.LogLevel,
		}
		return manage.ControlDaemon(opts, string(cfg.DaemonCommand))
	case config.ActionUpdate:
		return manage.Update(manage.Options{Addr: cfg.Addr, DataDir: cfg.DataDir, KernelPath: cfg.KernelPath, Version: version})
	case config.ActionUninstall:
		return manage.Uninstall(manage.Options{Addr: cfg.Addr, DataDir: cfg.DataDir, KernelPath: cfg.KernelPath, Purge: cfg.Purge})
	case config.ActionResetAdmin:
		return resetAdminPassword(cfg)
	}
	if cfg.Mode == config.ModeServe {
		handled, err := maybeUseDaemonControl(cfg)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
	}
	if err := cfg.EnsureDataDir(); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := bootstrapAdmin(db); err != nil {
		return err
	}
	if err := seedTemplates(db, cfg.DataDir); err != nil {
		return err
	}
	registrationEnabled, err := registrationEnabledFromSettings(db, cfg.RegistrationEnabled)
	if err != nil {
		return err
	}

	secret, err := sessionSecret(db)
	if err != nil {
		return err
	}
	kernelPath, err := db.GetSettingDefault("kernel_path", cfg.KernelPath)
	if err != nil {
		return err
	}
	allowPrivate, _ := db.GetSettingDefault("subfetch_allow_private", "false")

	fetcher := subfetch.New()
	fetcher.AllowPrivate = allowPrivate == "true"

	srv := &api.Server{
		Store:               db,
		Auth:                auth.New(secret),
		Kernel:              kernel.New(kernelPath, cfg.DataDir, 15*time.Second),
		Fetcher:             fetcher,
		TemplateDir:         filepath.Join(cfg.DataDir, "templates"),
		Secure:              false, // set true behind TLS; cookies still HttpOnly+SameSite
		RegistrationEnabled: registrationEnabled,
		DevMode:             cfg.Dev,
	}
	if cfg.Mode == config.ModeDaemon {
		release, err := acquireDaemonSocket(cfg.SocketPath, daemonRuntime{cfg: cfg, srv: srv}.handle)
		if err != nil {
			return err
		}
		defer release()
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	uiState := "embedded UI"
	if cfg.Dev || !assets.HasDist() {
		uiState = "API only (no embedded UI)"
	}
	logInfo("sb-fox version=%s listening on %s (%s), data-dir=%s, kernel=%q, registration=%s",
		version, cfg.Addr, uiState, cfg.DataDir, kernelPath, regStatus(registrationEnabled))
	logDebug("runtime mode=%s db=%s daemon_socket=%s", cfg.Mode, cfg.DBPath, cfg.SocketPath)
	return serveHTTP(httpSrv)
}

func serveHTTP(srv *http.Server) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case sig := <-sigCh:
		logInfo("received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

func acquireDaemonSocket(path string, handle daemonControlHandler) (func(), error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("daemon socket path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create daemon socket directory: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		conn, dialErr := net.DialTimeout("unix", path, 500*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("sb-fox daemon already running at %s", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale daemon socket %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat daemon socket %s: %w", path, err)
	}

	addr := &net.UnixAddr{Name: path, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("listen daemon socket %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("chmod daemon socket %s: %w", path, err)
	}

	done := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					logWarn("daemon socket accept error: %v", err)
					return
				}
			}
			go serveDaemonControl(conn, handle)
		}
	}()

	return func() {
		close(done)
		_ = ln.Close()
		_ = os.Remove(path)
	}, nil
}

func serveDaemonControl(conn net.Conn, handle daemonControlHandler) {
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		logWarn("daemon socket deadline error: %v", err)
		return
	}
	var req daemonControlRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	if handle == nil {
		handle = func(daemonControlRequest) daemonControlResponse {
			return daemonControlResponse{OK: true}
		}
	}
	resp := handle(req)
	if resp.Error != "" {
		resp.OK = false
	}
	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		logWarn("daemon socket response error: %v", err)
	}
}

// seedTemplates idempotently loads file-backed templates from data/templates.
func seedTemplates(db *store.Store, dataDir string) error {
	templateDir := filepath.Join(dataDir, "templates")
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return seedTemplateDirError(templateDir, err)
	}
	users, err := db.ListUsers()
	if err != nil {
		return fmt.Errorf("list users for template seed: %w", err)
	}
	scanned := 0
	inserted := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		scanned++
		path := filepath.Join(templateDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", path, err)
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		for _, user := range users {
			ok, err := db.SeedUserTemplate(user.ID, name, string(content), "file: data/templates/"+entry.Name())
			if err != nil {
				return fmt.Errorf("seed template %s for user %d: %w", name, user.ID, err)
			}
			if ok {
				inserted++
			}
		}
	}
	logInfo("template seed scanned=%d users=%d inserted=%d from %s", scanned, len(users), inserted, templateDir)
	return nil
}

func seedTemplateDirError(templateDir string, err error) error {
	return fmt.Errorf("read seed template directory %s: %w; reinstall with scripts/install.sh or run with --data-dir pointing to a directory that contains templates/", templateDir, err)
}

// bootstrapAdmin creates the admin with a random printed password on first run.
// An env override SB_FOX_ADMIN_PASSWORD sets a known initial password.
func bootstrapAdmin(db *store.Store) error {
	result, err := bootstrap.EnsureAdmin(db)
	if err != nil {
		return err
	}
	if !result.Created {
		return nil
	}
	if result.Generated {
		log.Printf("┌──────────────────────────────────────────────────────────┐")
		log.Printf("│ initial admin created — username: admin                  │")
		log.Printf("│ password: %-46s │", result.Password)
		log.Printf("│ (shown once; change it in Settings after logging in)     │")
		log.Printf("└──────────────────────────────────────────────────────────┘")
	} else {
		log.Printf("initial admin created (username: admin) from SB_FOX_ADMIN_PASSWORD")
	}
	return nil
}

func resetAdminPassword(cfg *config.Config) error {
	if err := cfg.EnsureDataDir(); err != nil {
		return resetAdminDataDirError(cfg.DataDir, err)
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	password := randomHex(12)
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	admin, err := db.FirstAdmin()
	if err == store.ErrNotFound {
		if _, err := db.CreateUser(&models.User{Username: "admin", PasswordHash: hash, Role: models.RoleAdmin}); err != nil {
			return err
		}
		fmt.Printf("admin password created\nusername: admin\npassword: %s\n", password)
		return nil
	}
	if err != nil {
		return err
	}
	if err := db.SetUserPassword(admin.ID, hash); err != nil {
		return err
	}
	fmt.Printf("admin password reset\nusername: %s\npassword: %s\n", admin.Username, password)
	return nil
}

func resetAdminDataDirError(dataDir string, err error) error {
	return fmt.Errorf("create data dir %s: %w; local data: ./sb-fox -P -D ./data; daemon data: sudo sb-fox -P", dataDir, err)
}

func registrationEnabledFromSettings(db *store.Store, initial bool) (bool, error) {
	value, ok, err := db.GetSetting(api.SettingRegistrationEnabled)
	if err != nil {
		return false, err
	}
	if !ok {
		if err := db.SetSetting(api.SettingRegistrationEnabled, boolSetting(initial)); err != nil {
			return false, err
		}
		return initial, nil
	}
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be \"true\" or \"false\"", api.SettingRegistrationEnabled)
	}
}

func setLogLevel(value string) {
	switch value {
	case "error":
		activeLogLevel = levelError
	case "warn":
		activeLogLevel = levelWarn
	case "debug":
		activeLogLevel = levelDebug
	default:
		activeLogLevel = levelInfo
	}
}

func logWarn(format string, args ...any) {
	logAt(levelWarn, format, args...)
}

func logInfo(format string, args ...any) {
	logAt(levelInfo, format, args...)
}

func logDebug(format string, args ...any) {
	logAt(levelDebug, format, args...)
}

func logAt(level runtimeLogLevel, format string, args ...any) {
	if level <= activeLogLevel {
		log.Printf(format, args...)
	}
}

// sessionSecret returns the stored HMAC session secret, creating one on first
// run so sessions survive restarts.
func sessionSecret(db *store.Store) ([]byte, error) {
	existing, ok, err := db.GetSetting("session_secret")
	if err != nil {
		return nil, err
	}
	if ok && existing != "" {
		return []byte(existing), nil
	}
	secret := randomHex(32)
	if err := db.SetSetting("session_secret", secret); err != nil {
		return nil, err
	}
	return []byte(secret), nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(errors.New("sb-fox: cannot read random bytes"))
	}
	return hex.EncodeToString(b)
}
