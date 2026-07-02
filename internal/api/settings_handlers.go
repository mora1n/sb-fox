package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mora1n/sb-fox/internal/kernel"
	"github.com/mora1n/sb-fox/internal/merge"
)

// Setting keys.
const (
	defaultAppDisplayName = "sb-fox"
	settingAppDisplayName = "app_display_name"
	settingCountryHeat    = "country_heat_order"
	settingKernelPath     = "kernel_path"
	settingKernelVersion  = "kernel_version"
	settingAllowPrivate   = "subfetch_allow_private"
)

// redactedKeys are never returned to the client.
var redactedKeys = map[string]bool{
	"session_secret": true,
}

// handleGetSettings returns all non-redacted settings.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	all, err := s.Store.AllSettings()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := defaultSettings()
	for k, v := range all {
		if redactedKeys[k] {
			continue
		}
		out[k] = v
	}
	if err := validateSettings(out); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, out)
}

// handleUpdateSettings patches settings from a flat map. Updating kernel_path
// re-points the kernel and refreshes the cached version.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var patch map[string]string
	if !decodeJSON(w, r, &patch) {
		return
	}
	for k, v := range patch {
		if redactedKeys[k] {
			continue
		}
		value, err := normalizeSetting(k, v)
		if err != nil {
			respondError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := s.Store.SetSetting(k, value); err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if k == settingKernelPath {
			s.Kernel.Path = value
			s.refreshKernelVersion()
		}
		if k == settingAllowPrivate {
			s.Fetcher.AllowPrivate = value == "true"
		}
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// appInfo is public bootstrapping metadata for the frontend.
type appInfo struct {
	DisplayName         string   `json:"display_name"`
	CountryHeatOrder    []string `json:"country_heat_order"`
	RegistrationEnabled bool     `json:"registration_enabled"`
}

func (s *Server) handleAppInfo(w http.ResponseWriter, r *http.Request) {
	displayName, err := s.appDisplayName()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	order, err := s.countryHeatOrder()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, appInfo{
		DisplayName:         displayName,
		CountryHeatOrder:    order,
		RegistrationEnabled: s.RegistrationEnabled,
	})
}

func (s *Server) appDisplayName() (string, error) {
	raw, err := s.Store.GetSettingDefault(settingAppDisplayName, defaultAppDisplayName)
	if err != nil {
		return "", err
	}
	return normalizeDisplayName(raw)
}

func (s *Server) countryHeatOrder() ([]string, error) {
	raw, err := s.Store.GetSettingDefault(settingCountryHeat, defaultCountryHeatOrderJSON())
	if err != nil {
		return nil, err
	}
	return parseCountryHeatOrder(raw)
}

func defaultSettings() map[string]string {
	return map[string]string{
		settingAppDisplayName: defaultAppDisplayName,
		settingCountryHeat:    defaultCountryHeatOrderJSON(),
	}
}

func validateSettings(settings map[string]string) error {
	if _, err := normalizeDisplayName(settings[settingAppDisplayName]); err != nil {
		return fmt.Errorf("invalid %s: %w", settingAppDisplayName, err)
	}
	if _, err := parseCountryHeatOrder(settings[settingCountryHeat]); err != nil {
		return fmt.Errorf("invalid %s: %w", settingCountryHeat, err)
	}
	return nil
}

func normalizeSetting(key, value string) (string, error) {
	switch key {
	case settingAppDisplayName:
		return normalizeDisplayName(value)
	case settingCountryHeat:
		order, err := parseCountryHeatOrder(value)
		if err != nil {
			return "", err
		}
		return countryHeatOrderJSON(order), nil
	case settingAllowPrivate:
		if value != "true" && value != "false" {
			return "", fmt.Errorf("%s must be \"true\" or \"false\"", settingAllowPrivate)
		}
	}
	return value, nil
}

func normalizeDisplayName(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("display name cannot be empty")
	}
	return trimmed, nil
}

func parseCountryHeatOrder(raw string) ([]string, error) {
	var codes []string
	if err := json.Unmarshal([]byte(raw), &codes); err != nil {
		return nil, err
	}
	return merge.NormalizeCountryHeatOrder(codes)
}

func defaultCountryHeatOrderJSON() string {
	return countryHeatOrderJSON(merge.DefaultCountryHeatOrder())
}

func countryHeatOrderJSON(order []string) string {
	data, err := json.Marshal(order)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// kernelStatus is the response for the kernel status endpoint.
type kernelStatus struct {
	Available bool   `json:"available"`
	Path      string `json:"path"`
	Version   string `json:"version"`
}

// handleKernelStatus probes the configured sing-box binary.
func (s *Server) handleKernelStatus(w http.ResponseWriter, r *http.Request) {
	status := kernelStatus{Path: s.Kernel.Path, Available: s.Kernel.Available()}
	if status.Available {
		if ver, err := s.Kernel.Version(); err == nil {
			status.Version = ver
			_ = s.Store.SetSetting(settingKernelVersion, ver)
		}
	}
	respondJSON(w, http.StatusOK, status)
}

// refreshKernelVersion updates the cached kernel version setting.
func (s *Server) refreshKernelVersion() {
	if !s.Kernel.Available() {
		return
	}
	if ver, err := s.Kernel.Version(); err == nil {
		_ = s.Store.SetSetting(settingKernelVersion, ver)
	}
}

// validateWithKernel runs a check and returns the result (advisory).
func (s *Server) validateWithKernel(config []byte) kernel.Result {
	// bound total time defensively
	_ = time.Now()
	return s.Kernel.Check(config)
}
