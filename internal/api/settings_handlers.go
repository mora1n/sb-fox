package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

// Setting keys.
const (
	defaultAppDisplayName      = "sb-fox"
	settingAppDisplayName      = "app_display_name"
	settingCountryHeat         = "country_heat_order"
	settingKernelPath          = "kernel_path"
	settingKernelProfiles      = "kernel_profiles"
	settingKernelVersion       = "kernel_version"
	settingAllowPrivate        = "subfetch_allow_private"
	settingSubHostPrefix       = "subscription_host_prefix"
	SettingRegistrationEnabled = "registration_enabled"
)

// redactedKeys are never returned to the client.
var redactedKeys = map[string]bool{
	"session_secret": true,
}

// handleGetSettings returns all non-redacted settings.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
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
	if !u.IsAdmin() {
		for k := range out {
			if adminOnlySetting(k) {
				delete(out, k)
			}
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// handleUpdateSettings patches settings from a flat map. Updating kernel_path
// re-points the kernel and refreshes the cached version.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var patch map[string]string
	if !decodeJSON(w, r, &patch) {
		return
	}
	for k, v := range patch {
		if redactedKeys[k] {
			continue
		}
		if !u.IsAdmin() && adminOnlySetting(k) {
			respondError(w, http.StatusForbidden, "forbidden", "admin only")
			return
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
		if k == SettingRegistrationEnabled {
			s.SetRegistrationEnabled(value == "true")
		}
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// appInfo is public bootstrapping metadata for the frontend.
type appInfo struct {
	DisplayName         string   `json:"display_name"`
	CountryHeatOrder    []string `json:"country_heat_order"`
	RegistrationEnabled bool     `json:"registration_enabled"`
	SubscriptionHost    string   `json:"subscription_host_prefix"`
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
	subHost, err := s.Store.GetSettingDefault(settingSubHostPrefix, "")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, appInfo{
		DisplayName:         displayName,
		CountryHeatOrder:    order,
		RegistrationEnabled: s.IsRegistrationEnabled(),
		SubscriptionHost:    subHost,
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
		settingAppDisplayName:      defaultAppDisplayName,
		settingCountryHeat:         defaultCountryHeatOrderJSON(),
		settingSubHostPrefix:       "",
		SettingRegistrationEnabled: "false",
	}
}

func adminOnlySetting(key string) bool {
	switch key {
	case settingAppDisplayName, settingKernelPath, settingKernelProfiles, settingKernelVersion, settingAllowPrivate, settingSubHostPrefix, SettingRegistrationEnabled:
		return true
	default:
		return false
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
	case SettingRegistrationEnabled:
		if value != "true" && value != "false" {
			return "", fmt.Errorf("%s must be \"true\" or \"false\"", SettingRegistrationEnabled)
		}
	case settingSubHostPrefix:
		return normalizeSubscriptionHostPrefix(value)
	case settingKernelProfiles:
		profiles, err := parseKernelProfiles(value)
		if err != nil {
			return "", err
		}
		normalized, err := normalizeKernelProfiles(profiles)
		if err != nil {
			return "", err
		}
		data, err := json.Marshal(normalized)
		if err != nil {
			return "", err
		}
		return string(data), nil
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

func normalizeSubscriptionHostPrefix(value string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return "", errors.New("subscription host prefix must start with http:// or https://")
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
