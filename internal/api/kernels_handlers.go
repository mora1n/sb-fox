package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mora1n/sb-fox/internal/kernel"
	"github.com/mora1n/sb-fox/internal/models"
)

type kernelProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type kernelProfilesRequest struct {
	Kernels []kernelProfile `json:"kernels"`
}

type setActiveKernelRequest struct {
	ID string `json:"id"`
}

type kernelProbeResponse struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Path      string `json:"path,omitempty"`
	Available bool   `json:"available"`
	Valid     bool   `json:"valid"`
	Active    bool   `json:"active,omitempty"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

type kernelStatusResponse struct {
	Available      bool                  `json:"available"`
	ActiveKernelID string                `json:"active_kernel_id"`
	Version        string                `json:"version,omitempty"`
	Path           string                `json:"path,omitempty"`
	Active         *kernelProbeResponse  `json:"active,omitempty"`
	Kernels        []kernelProbeResponse `json:"kernels"`
}

const kernelProbeCacheTTL = 10 * time.Second

type kernelProbeCacheEntry struct {
	probe     kernel.ProbeResult
	expiresAt time.Time
}

func parseKernelProfiles(raw string) ([]kernelProfile, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var profiles []kernelProfile
	if err := json.Unmarshal([]byte(raw), &profiles); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", settingKernelProfiles, err)
	}
	return profiles, nil
}

func normalizeKernelProfiles(profiles []kernelProfile) ([]kernelProfile, error) {
	out := make([]kernelProfile, 0, len(profiles))
	seenID := map[string]bool{}
	seenPath := map[string]bool{}
	for i, p := range profiles {
		p.ID = strings.TrimSpace(p.ID)
		p.Name = strings.TrimSpace(p.Name)
		p.Path = strings.TrimSpace(p.Path)
		if p.Name == "" {
			return nil, fmt.Errorf("kernel %d name is required", i+1)
		}
		if p.Path == "" {
			return nil, fmt.Errorf("kernel %q path is required", p.Name)
		}
		if p.ID == "" {
			id, err := newToken()
			if err != nil {
				return nil, err
			}
			p.ID = id
		}
		if seenID[p.ID] {
			return nil, fmt.Errorf("duplicate kernel id %q", p.ID)
		}
		if seenPath[p.Path] {
			return nil, fmt.Errorf("duplicate kernel path %q", p.Path)
		}
		seenID[p.ID] = true
		seenPath[p.Path] = true
		out = append(out, p)
	}
	return out, nil
}

func (s *Server) kernelProfiles() ([]kernelProfile, error) {
	raw, ok, err := s.Store.GetSetting(settingKernelProfiles)
	if err != nil {
		return nil, err
	}
	if ok {
		profiles, err := parseKernelProfiles(raw)
		if err != nil {
			return nil, err
		}
		return normalizeKernelProfiles(profiles)
	}
	path, err := s.Store.GetSettingDefault(settingKernelPath, s.defaultKernelPath())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	return []kernelProfile{{ID: "default", Name: "sing-box", Path: strings.TrimSpace(path)}}, nil
}

func (s *Server) defaultKernelPath() string {
	if s.Kernel == nil {
		return ""
	}
	return s.Kernel.Path
}

func (s *Server) kernelRuntime(path string) *kernel.Kernel {
	dataDir := ""
	timeout := 15 * time.Second
	if s.Kernel != nil {
		dataDir = s.Kernel.DataDir
		if s.Kernel.Timeout != 0 {
			timeout = s.Kernel.Timeout
		}
	}
	return kernel.New(path, dataDir, timeout)
}

func (s *Server) probeKernelPath(path string, force bool) kernel.ProbeResult {
	path = strings.TrimSpace(path)
	if !force {
		now := time.Now()
		s.kernelProbeMu.Lock()
		entry, ok := s.kernelProbeCache[path]
		if ok && now.Before(entry.expiresAt) {
			probe := entry.probe
			s.kernelProbeMu.Unlock()
			return probe
		}
		s.kernelProbeMu.Unlock()
	}

	probe := s.kernelRuntime(path).Probe()
	s.kernelProbeMu.Lock()
	if s.kernelProbeCache == nil {
		s.kernelProbeCache = make(map[string]kernelProbeCacheEntry)
	}
	s.kernelProbeCache[path] = kernelProbeCacheEntry{
		probe:     probe,
		expiresAt: time.Now().Add(kernelProbeCacheTTL),
	}
	s.kernelProbeMu.Unlock()
	return probe
}

func (s *Server) clearKernelProbeCache() {
	s.kernelProbeMu.Lock()
	defer s.kernelProbeMu.Unlock()
	s.kernelProbeCache = nil
}

func (s *Server) probeKernelProfile(profile kernelProfile, includePath bool, active bool) kernelProbeResponse {
	return s.probeKernelProfileWithForce(profile, includePath, active, false)
}

func (s *Server) probeKernelProfileWithForce(profile kernelProfile, includePath bool, active bool, force bool) kernelProbeResponse {
	probe := s.probeKernelPath(profile.Path, force)
	out := kernelProbeResponse{
		ID:        profile.ID,
		Name:      profile.Name,
		Available: probe.Available,
		Valid:     probe.Valid,
		Active:    active,
		Version:   probe.Version,
		Error:     probe.Error,
	}
	if includePath {
		out.Path = profile.Path
	}
	return out
}

func (s *Server) kernelStatusForUser(u *models.User, includePath bool, validOnly bool) (kernelStatusResponse, error) {
	profiles, err := s.kernelProfiles()
	if err != nil {
		return kernelStatusResponse{}, err
	}
	status := kernelStatusResponse{
		ActiveKernelID: strings.TrimSpace(u.ActiveKernelID),
		Kernels:        make([]kernelProbeResponse, 0, len(profiles)),
	}
	var firstValid *kernelProbeResponse
	for _, profile := range profiles {
		isActive := profile.ID == status.ActiveKernelID
		probe := s.probeKernelProfile(profile, includePath, isActive)
		if !validOnly || probe.Valid {
			status.Kernels = append(status.Kernels, probe)
		}
		if probe.Valid {
			if firstValid == nil {
				cp := probe
				firstValid = &cp
			}
		}
		if isActive {
			cp := probe
			status.Active = &cp
		}
	}
	if status.Active == nil && status.ActiveKernelID == "" && firstValid != nil {
		cp := *firstValid
		cp.Active = true
		status.Active = &cp
		status.ActiveKernelID = cp.ID
	}
	if status.Active != nil && status.Active.Valid {
		status.Available = true
		status.Version = status.Active.Version
		status.Path = status.Active.Path
	}
	return status, nil
}

func (s *Server) activeKernelForUser(u *models.User) (*kernel.Kernel, error) {
	profiles, err := s.kernelProfiles()
	if err != nil {
		return nil, err
	}
	activeID := strings.TrimSpace(u.ActiveKernelID)
	if activeID == "" {
		for _, profile := range profiles {
			runtime := s.kernelRuntime(profile.Path)
			if probe := runtime.Probe(); probe.Valid {
				return runtime, nil
			}
		}
		return nil, errors.New("no valid sing-box kernel configured")
	}
	for _, profile := range profiles {
		if profile.ID != activeID {
			continue
		}
		runtime := s.kernelRuntime(profile.Path)
		probe := runtime.Probe()
		if !probe.Valid {
			if probe.Error != "" {
				return nil, fmt.Errorf("selected kernel %q is invalid: %s", profile.Name, probe.Error)
			}
			return nil, fmt.Errorf("selected kernel %q is invalid", profile.Name)
		}
		return runtime, nil
	}
	return nil, fmt.Errorf("selected kernel %q no longer exists", activeID)
}

func (s *Server) handleListKernels(w http.ResponseWriter, r *http.Request) {
	u, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	status, err := s.kernelStatusForUser(u, true, false)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleSaveKernels(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	var req kernelProfilesRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	profiles, err := normalizeKernelProfiles(req.Kernels)
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	data, err := json.Marshal(profiles)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := s.Store.SetSetting(settingKernelProfiles, string(data)); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if len(profiles) > 0 {
		_ = s.Store.SetSetting(settingKernelPath, profiles[0].Path)
	}
	s.clearKernelProbeCache()
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTestKernel(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	var req kernelProfile
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Path = strings.TrimSpace(req.Path)
	if req.Name == "" {
		req.Name = "sing-box"
	}
	if req.Path == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "kernel path is required")
		return
	}
	respondJSON(w, http.StatusOK, s.probeKernelProfileWithForce(req, true, false, true))
}

// handleKernelStatus is the legacy admin endpoint. It now returns the active
// kernel plus the valid kernel list, with paths included for administrators.
func (s *Server) handleKernelStatus(w http.ResponseWriter, r *http.Request) {
	u, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	status, err := s.kernelStatusForUser(u, true, false)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if status.Available && status.Version != "" {
		_ = s.Store.SetSetting(settingKernelVersion, status.Version)
	}
	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handlePublicKernelStatus(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	status, err := s.kernelStatusForUser(u, false, true)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	status.Path = ""
	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleSetActiveKernel(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req setActiveKernelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "kernel id is required")
		return
	}
	profiles, err := s.kernelProfiles()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	for _, profile := range profiles {
		if profile.ID != id {
			continue
		}
		probe := s.probeKernelProfile(profile, false, false)
		if !probe.Valid {
			msg := probe.Error
			if msg == "" {
				msg = "kernel is invalid"
			}
			respondError(w, http.StatusBadRequest, "bad_request", msg)
			return
		}
		if err := s.Store.SetUserActiveKernel(u.ID, id); err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		u.ActiveKernelID = id
		status, err := s.kernelStatusForUser(u, false, true)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		respondJSON(w, http.StatusOK, status)
		return
	}
	respondError(w, http.StatusBadRequest, "bad_request", "kernel not found")
}

// refreshKernelVersion updates the cached kernel version setting for the legacy
// kernel_path setting.
func (s *Server) refreshKernelVersion() {
	if s.Kernel == nil {
		return
	}
	probe := s.probeKernelPath(s.Kernel.Path, true)
	if probe.Valid {
		_ = s.Store.SetSetting(settingKernelVersion, probe.Version)
	}
}

func (s *Server) validateWithKernelForUser(u *models.User, config []byte) kernel.Result {
	runtime, err := s.activeKernelForUser(u)
	if err != nil {
		return kernel.Result{Status: kernel.StatusUnavailable, Messages: err.Error()}
	}
	return runtime.Check(config)
}

func (s *Server) formatWithKernelForUser(u *models.User, config []byte) kernel.Result {
	runtime, err := s.activeKernelForUser(u)
	if err != nil {
		return kernel.Result{Status: kernel.StatusUnavailable, Messages: err.Error()}
	}
	return runtime.Format(config)
}
