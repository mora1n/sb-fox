package kernel

import (
	"os/exec"
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
