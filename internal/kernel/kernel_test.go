package kernel

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestKernelUnavailable verifies graceful degradation with no binary.
func TestKernelUnavailable(t *testing.T) {
	k := New("/nonexistent/sing-box-xyz", t.TempDir(), 5*time.Second)
	if k.Available() {
		t.Fatal("expected unavailable")
	}
	res := k.Check([]byte(`{}`))
	if res.Status != StatusUnavailable {
		t.Errorf("got %s, want unavailable", res.Status)
	}
	if _, err := k.Version(); err == nil {
		t.Error("expected version error when unavailable")
	}
}

// TestKernelEmptyPath verifies an empty path disables validation.
func TestKernelEmptyPath(t *testing.T) {
	k := New("", t.TempDir(), 5*time.Second)
	if k.Available() {
		t.Fatal("empty path should be unavailable")
	}
}

func TestKernelProbeRejectsNonSingBoxVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake-version")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nif [ \"$1\" = version ]; then echo 'other-tool version 1.0'; exit 0; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	k := New(path, t.TempDir(), 5*time.Second)
	probe := k.Probe()
	if !probe.Available {
		t.Fatalf("probe should be available: %+v", probe)
	}
	if probe.Valid {
		t.Fatalf("probe should reject non-sing-box output: %+v", probe)
	}
}

// TestKernelReal exercises the real sing-box binary when present in PATH.
func TestKernelReal(t *testing.T) {
	path, err := exec.LookPath("sing-box")
	if err != nil {
		t.Skip("sing-box not in PATH; skipping real kernel test")
	}
	k := New(path, t.TempDir(), 15*time.Second)
	if !k.Available() {
		t.Fatal("expected available")
	}
	ver, err := k.Version()
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if ver == "" {
		t.Error("empty version")
	}
	t.Logf("kernel version: %s", ver)
	if probe := k.Probe(); !probe.Valid {
		t.Fatalf("kernel probe invalid: %+v", probe)
	}

	// A minimal valid config: one direct outbound.
	valid := []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}]}`)
	if res := k.Check(valid); res.Status != StatusOK {
		t.Errorf("valid config check = %s: %s", res.Status, res.Messages)
	}

	// Invalid: unknown outbound type.
	invalid := []byte(`{"outbounds":[{"type":"nonsense-proto","tag":"x"}]}`)
	if res := k.Check(invalid); res.Status != StatusInvalid {
		t.Errorf("invalid config check = %s, want invalid", res.Status)
	}
}
