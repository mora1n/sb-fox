package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) seedTemplatesForUser(ownerUserID int64) error {
	if s.TemplateDir == "" {
		return nil
	}
	entries, err := os.ReadDir(s.TemplateDir)
	if err != nil {
		return fmt.Errorf("read template directory %s: %w", s.TemplateDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.TemplateDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", path, err)
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if _, err := s.Store.SeedUserTemplate(ownerUserID, name, string(content), "file: data/templates/"+entry.Name()); err != nil {
			return fmt.Errorf("seed template %s for user %d: %w", name, ownerUserID, err)
		}
	}
	return nil
}
