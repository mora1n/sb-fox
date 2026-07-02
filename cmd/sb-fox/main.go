// Command sb-fox is a single-binary sing-box config/subscription management
// panel: it serves an embedded web UI and an API for editing nodes, templates
// and profiles, and renders full config.json documents via tokenized links.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mora1n/sb-fox/internal/api"
	"github.com/mora1n/sb-fox/internal/assets"
	"github.com/mora1n/sb-fox/internal/auth"
	"github.com/mora1n/sb-fox/internal/config"
	"github.com/mora1n/sb-fox/internal/kernel"
	"github.com/mora1n/sb-fox/internal/store"
	"github.com/mora1n/sb-fox/internal/subfetch"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("sb-fox: %v", err)
	}
}

func run(args []string) error {
	cfg, err := config.Parse(args)
	if err != nil {
		return err
	}
	if cfg.ShowVersion {
		fmt.Printf("sb-fox %s\n", version)
		return nil
	}
	if err := cfg.EnsureDataDir(); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := seedTemplates(db, cfg.DataDir); err != nil {
		return err
	}
	if err := bootstrapAdmin(db); err != nil {
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
		Store:   db,
		Auth:    auth.New(secret),
		Kernel:  kernel.New(kernelPath, cfg.DataDir, 15*time.Second),
		Fetcher: fetcher,
		Secure:  false, // set true behind TLS; cookies still HttpOnly+SameSite
		DevMode: cfg.Dev,
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
	log.Printf("sb-fox %s listening on %s (%s), data-dir=%s, kernel=%q", version, cfg.Addr, uiState, cfg.DataDir, kernelPath)
	return httpSrv.ListenAndServe()
}

// seedTemplates idempotently loads file-backed templates from data/templates.
func seedTemplates(db *store.Store, dataDir string) error {
	templateDir := filepath.Join(dataDir, "templates")
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return fmt.Errorf("read template directory %s: %w", templateDir, err)
	}
	seeded := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(templateDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", path, err)
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		ok, err := db.SeedUserTemplate(name, string(content), "file: data/templates/"+entry.Name())
		if err != nil {
			return fmt.Errorf("seed template %s: %w", name, err)
		}
		if ok {
			seeded++
		}
	}
	log.Printf("seeded %d file-backed templates from %s", seeded, templateDir)
	return nil
}

// bootstrapAdmin creates the admin with a random printed password on first run.
// An env override SB_FOX_ADMIN_PASSWORD sets a known initial password.
func bootstrapAdmin(db *store.Store) error {
	exists, err := db.AdminExists()
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	password := os.Getenv("SB_FOX_ADMIN_PASSWORD")
	generated := false
	if password == "" {
		password = randomHex(12)
		generated = true
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := db.SetAdmin("admin", hash); err != nil {
		return err
	}
	if generated {
		log.Printf("┌──────────────────────────────────────────────────────────┐")
		log.Printf("│ initial admin created — username: admin                  │")
		log.Printf("│ password: %-46s │", password)
		log.Printf("│ (shown once; change it in Settings after logging in)     │")
		log.Printf("└──────────────────────────────────────────────────────────┘")
	} else {
		log.Printf("initial admin created (username: admin) from SB_FOX_ADMIN_PASSWORD")
	}
	return nil
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
