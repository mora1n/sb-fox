package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestUserSettingsPermissionsAndPublicKernelStatus(t *testing.T) {
	srv, ts := testServer(t)
	srv.Kernel.Path = ""
	admin := newClient(t, ts.URL)
	admin.http.Jar = login(t, ts.URL)
	decodeData(t, admin.do(http.MethodPut, "/api/settings", map[string]string{
		"country_heat_order": `["US","JP"]`,
	}), nil)

	var user struct {
		ID int64 `json:"id"`
	}
	decodeData(t, admin.do(http.MethodPost, "/api/users", map[string]any{
		"username": "carol", "password": "password123",
	}), &user)

	userClient := newClient(t, ts.URL)
	userClient.http.Jar = loginAs(t, ts.URL, "carol", "password123")
	countryPrefix := func(raw string) []string {
		t.Helper()
		var order []string
		if err := json.Unmarshal([]byte(raw), &order); err != nil {
			t.Fatalf("decode country order: %v", err)
		}
		if len(order) < 2 {
			t.Fatalf("short country order: %v", order)
		}
		return order[:2]
	}

	var settings map[string]string
	decodeData(t, userClient.do(http.MethodGet, "/api/settings", nil), &settings)
	if len(settings) != 1 {
		t.Fatalf("non-admin settings should only include personal country order: %+v", settings)
	}
	if _, ok := settings["app_display_name"]; ok {
		t.Fatalf("non-admin settings leaked app_display_name: %+v", settings)
	}
	if _, ok := settings["kernel_path"]; ok {
		t.Fatalf("non-admin settings leaked kernel_path: %+v", settings)
	}
	if _, ok := settings["subscription_host_prefix"]; ok {
		t.Fatalf("non-admin settings leaked subscription_host_prefix: %+v", settings)
	}
	if _, ok := settings["registration_enabled"]; ok {
		t.Fatalf("non-admin settings leaked registration_enabled: %+v", settings)
	}
	if got := countryPrefix(settings["country_heat_order"]); got[0] != "US" || got[1] != "JP" {
		t.Fatalf("non-admin default country order = %v", got)
	}

	decodeData(t, userClient.do(http.MethodPut, "/api/settings", map[string]string{"country_heat_order": `["DE","FR"]`}), nil)
	decodeData(t, userClient.do(http.MethodGet, "/api/settings", nil), &settings)
	if got := countryPrefix(settings["country_heat_order"]); got[0] != "DE" || got[1] != "FR" {
		t.Fatalf("non-admin personal country order = %v", got)
	}
	var adminSettings map[string]string
	decodeData(t, admin.do(http.MethodGet, "/api/settings", nil), &adminSettings)
	if got := countryPrefix(adminSettings["country_heat_order"]); got[0] != "US" || got[1] != "JP" {
		t.Fatalf("admin country order changed by user preference: %v", got)
	}
	var app struct {
		CountryHeatOrder []string `json:"country_heat_order"`
	}
	decodeData(t, userClient.do(http.MethodGet, "/api/app", nil), &app)
	if len(app.CountryHeatOrder) < 2 || app.CountryHeatOrder[0] != "US" || app.CountryHeatOrder[1] != "JP" {
		t.Fatalf("public app country order should remain admin order: %+v", app.CountryHeatOrder)
	}

	if _, ok := settings["country_heat_order"]; !ok {
		t.Fatalf("non-admin settings missing country_heat_order: %+v", settings)
	}

	status, _, msg := decodeError(t, userClient.do(http.MethodPut, "/api/settings", map[string]string{"kernel_path": "sing-box"}))
	if status != http.StatusForbidden || !strings.Contains(msg, "admin only") {
		t.Fatalf("kernel setting status=%d msg=%q", status, msg)
	}
	status, _, msg = decodeError(t, userClient.do(http.MethodPut, "/api/settings", map[string]string{"subfetch_allow_private": "true"}))
	if status != http.StatusForbidden || !strings.Contains(msg, "admin only") {
		t.Fatalf("private fetch setting status=%d msg=%q", status, msg)
	}
	status, _, msg = decodeError(t, userClient.do(http.MethodPut, "/api/settings", map[string]string{"subscription_host_prefix": "https://example.com"}))
	if status != http.StatusForbidden || !strings.Contains(msg, "admin only") {
		t.Fatalf("host prefix setting status=%d msg=%q", status, msg)
	}
	status, _, msg = decodeError(t, userClient.do(http.MethodPut, "/api/settings", map[string]string{"unknown_setting": "value"}))
	if status != http.StatusForbidden || !strings.Contains(msg, "admin only") {
		t.Fatalf("unknown setting status=%d msg=%q", status, msg)
	}
	status, _, msg = decodeError(t, userClient.do(http.MethodPut, "/api/settings", map[string]string{
		"country_heat_order": `["CN","HK"]`,
		"app_display_name":   "bad",
	}))
	if status != http.StatusForbidden || !strings.Contains(msg, "admin only") {
		t.Fatalf("mixed setting status=%d msg=%q", status, msg)
	}
	decodeData(t, userClient.do(http.MethodGet, "/api/settings", nil), &settings)
	if got := countryPrefix(settings["country_heat_order"]); got[0] != "DE" || got[1] != "FR" {
		t.Fatalf("mixed forbidden patch changed personal country order: %v", got)
	}
	status, _, _ = decodeError(t, userClient.do(http.MethodGet, "/api/settings/kernel", nil))
	if status != http.StatusForbidden {
		t.Fatalf("non-admin kernel settings status=%d", status)
	}

	var public map[string]any
	decodeData(t, userClient.do(http.MethodGet, "/api/kernel/status", nil), &public)
	if _, ok := public["path"]; ok {
		t.Fatalf("public kernel status leaked path: %+v", public)
	}
	if got, ok := public["available"].(bool); !ok || got {
		t.Fatalf("public kernel status = %+v", public)
	}

	var adminKernel map[string]any
	decodeData(t, admin.do(http.MethodGet, "/api/settings/kernel", nil), &adminKernel)
	if got, ok := adminKernel["available"].(bool); !ok || got {
		t.Fatalf("empty admin kernel status = %+v", adminKernel)
	}

	validPath := fakeKernel(t, "sing-box version 1.13.14")
	invalidPath := fakeKernel(t, "other-tool version 1.0")
	var invalidProbe map[string]any
	decodeData(t, admin.do(http.MethodPost, "/api/settings/kernels/test", map[string]string{
		"name": "bad", "path": invalidPath,
	}), &invalidProbe)
	if got, ok := invalidProbe["valid"].(bool); !ok || got {
		t.Fatalf("invalid kernel probe = %+v", invalidProbe)
	}

	decodeData(t, admin.do(http.MethodPut, "/api/settings/kernels", map[string]any{
		"kernels": []map[string]string{
			{"name": "sing-box", "path": validPath},
			{"name": "sing-box", "path": invalidPath},
		},
	}), &struct{}{})

	var adminKernels kernelStatusResponse
	decodeData(t, admin.do(http.MethodGet, "/api/settings/kernels", nil), &adminKernels)
	if len(adminKernels.Kernels) != 2 {
		t.Fatalf("admin kernels = %+v", adminKernels)
	}
	if adminKernels.Kernels[0].Path == "" {
		t.Fatalf("admin kernel path not returned: %+v", adminKernels.Kernels[0])
	}
	var validID, invalidID string
	for _, item := range adminKernels.Kernels {
		if item.Valid {
			validID = item.ID
		} else {
			invalidID = item.ID
		}
	}
	if validID == "" || invalidID == "" {
		t.Fatalf("expected one valid and one invalid kernel: %+v", adminKernels.Kernels)
	}
	status, _, msg = decodeError(t, admin.do(http.MethodPut, "/api/settings/kernels", map[string]any{
		"kernels": []map[string]string{
			{"name": "sing-box stable", "path": validPath},
			{"name": "sing-box duplicate", "path": validPath},
		},
	}))
	if status != http.StatusBadRequest || !strings.Contains(msg, "duplicate kernel path") {
		t.Fatalf("duplicate kernel path status=%d msg=%q", status, msg)
	}

	var userStatus kernelStatusResponse
	decodeData(t, userClient.do(http.MethodGet, "/api/kernel/status", nil), &userStatus)
	if userStatus.Path != "" {
		t.Fatalf("user kernel status leaked path: %+v", userStatus)
	}
	if len(userStatus.Kernels) != 1 || userStatus.Kernels[0].ID != validID {
		t.Fatalf("user kernel status should expose only valid kernels: %+v", userStatus)
	}
	status, _, _ = decodeError(t, userClient.do(http.MethodPut, "/api/kernel/active", map[string]string{"id": invalidID}))
	if status != http.StatusBadRequest {
		t.Fatalf("invalid active kernel status=%d", status)
	}
	decodeData(t, userClient.do(http.MethodPut, "/api/kernel/active", map[string]string{"id": validID}), &userStatus)
	if !userStatus.Available || userStatus.ActiveKernelID != validID {
		t.Fatalf("active kernel not updated: %+v", userStatus)
	}
}

func TestKernelStatusCachesProbeAndExplicitActionsRefresh(t *testing.T) {
	srv, ts := testServer(t)
	srv.Kernel.Path = ""
	admin := newClient(t, ts.URL)
	admin.http.Jar = login(t, ts.URL)

	kernelPath, counterPath := fakeCountingKernel(t)
	decodeData(t, admin.do(http.MethodPut, "/api/settings/kernels", map[string]any{
		"kernels": []map[string]string{
			{"name": "sing-box", "path": kernelPath},
		},
	}), nil)

	var first kernelStatusResponse
	decodeData(t, admin.do(http.MethodGet, "/api/settings/kernels", nil), &first)
	if got := readProbeCount(t, counterPath); got != 1 {
		t.Fatalf("first status probe count = %d", got)
	}

	var second kernelStatusResponse
	decodeData(t, admin.do(http.MethodGet, "/api/settings/kernels", nil), &second)
	if got := readProbeCount(t, counterPath); got != 1 {
		t.Fatalf("cached status probe count = %d", got)
	}

	var explicit kernelProbeResponse
	decodeData(t, admin.do(http.MethodPost, "/api/settings/kernels/test", map[string]string{
		"name": "sing-box", "path": kernelPath,
	}), &explicit)
	if got := readProbeCount(t, counterPath); got != 2 {
		t.Fatalf("explicit test probe count = %d", got)
	}

	decodeData(t, admin.do(http.MethodPut, "/api/settings/kernels", map[string]any{
		"kernels": []map[string]string{
			{"name": "sing-box", "path": kernelPath},
		},
	}), nil)
	var afterSave kernelStatusResponse
	decodeData(t, admin.do(http.MethodGet, "/api/settings/kernels", nil), &afterSave)
	if got := readProbeCount(t, counterPath); got != 3 {
		t.Fatalf("status after save probe count = %d", got)
	}
}

func fakeKernel(t *testing.T, versionLine string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-sing-box")
	script := "#!/bin/sh\ncase \"$1\" in\nversion) echo '" + versionLine + "' ;;\ncheck) exit 0 ;;\nformat) cat \"$3\" ;;\n*) exit 1 ;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeCountingKernel(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-sing-box")
	counterPath := filepath.Join(dir, "version-count")
	script := "#!/bin/sh\ncase \"$1\" in\nversion) n=0; if [ -f '" + counterPath + "' ]; then n=$(cat '" + counterPath + "'); fi; n=$((n + 1)); printf '%s\\n' \"$n\" > '" + counterPath + "'; echo 'sing-box version 1.13.14' ;;\ncheck) exit 0 ;;\nformat) cat \"$3\" ;;\n*) exit 1 ;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, counterPath
}

func readProbeCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return n
}
