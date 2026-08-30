package ruleset

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mora1n/sb-fox/internal/kernel"
	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/subfetch"
)

type testFetcher struct {
	bodies map[string][]byte
	err    error
}

func (f testFetcher) FetchBytes(_ context.Context, rawURL string, _ subfetch.ByteOptions) (subfetch.ByteResult, error) {
	if f.err != nil {
		return subfetch.ByteResult{}, f.err
	}
	return subfetch.ByteResult{URL: rawURL, Body: f.bodies[rawURL]}, nil
}

type testKernel struct {
	decompiled []byte
}

func (k testKernel) Version() (string, error) { return "sing-box version test", nil }
func (k testKernel) FormatRuleSet(input []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(input, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
func (k testKernel) DecompileRuleSet([]byte) ([]byte, error) { return k.decompiled, nil }
func (k testKernel) CompileRuleSet(input []byte) ([]byte, error) {
	return append([]byte("srs:"), input...), nil
}

func TestPublishMergesVersionsAndDeduplicatesStructuralRules(t *testing.T) {
	first := `{"version":2,"rules":[{"domain_suffix":["example.com"],"invert":false}]}`
	second := []byte(`{"version":4,"rules":[{"invert":false,"domain_suffix":["example.com"]},{"ip_cidr":["1.1.1.0/24"]}]}`)
	publisher := &Publisher{Fetcher: testFetcher{bodies: map[string][]byte{"https://example.com/b.srs": []byte("binary")}}}
	artifact, err := publisher.Publish(context.Background(), []*models.RuleSetSource{
		{Kind: "manual", Format: "source", Content: first},
		{Kind: "remote", Format: "binary", URL: "https://example.com/b.srs"},
	}, testKernel{decompiled: second})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.RuleCount != 2 || artifact.KernelVersion != "sing-box version test" {
		t.Fatalf("artifact = %+v", artifact)
	}
	var output sourceDocument
	if err := json.Unmarshal(artifact.JSON, &output); err != nil {
		t.Fatal(err)
	}
	if output.Version != 4 || len(output.Rules) != 2 {
		t.Fatalf("output = %+v", output)
	}
	if len(artifact.SRS) == 0 || artifact.JSONSHA256 == "" || artifact.SRSSHA256 == "" {
		t.Fatalf("missing artifact output: %+v", artifact)
	}
}

func TestPublishFailsOnAnySourceError(t *testing.T) {
	publisher := &Publisher{Fetcher: testFetcher{err: errors.New("upstream failed")}}
	_, err := publisher.Publish(context.Background(), []*models.RuleSetSource{
		{Kind: "remote", Format: "source", URL: "https://example.com/rules.json"},
	}, testKernel{})
	var publishErr *Error
	if !errors.As(err, &publishErr) || publishErr.Stage != "fetch" || publishErr.SourceIndex == nil || *publishErr.SourceIndex != 0 {
		t.Fatalf("error = %#v", err)
	}
}

func TestManualBinarySourceIsRejected(t *testing.T) {
	publisher := &Publisher{Fetcher: testFetcher{}}
	_, err := publisher.Publish(context.Background(), []*models.RuleSetSource{
		{Kind: "manual", Format: "binary", Content: "x"},
	}, testKernel{})
	if err == nil {
		t.Fatal("expected manual binary rejection")
	}
}

func TestPublishWithRealKernelAndRemoteSRS(t *testing.T) {
	path, err := exec.LookPath("sing-box")
	if err != nil {
		t.Skip("sing-box not in PATH")
	}
	runtime := kernel.New(path, filepath.Join(t.TempDir(), "kernel"), 30*time.Second)
	second := []byte(`{"version":4,"rules":[{"ip_cidr":["1.1.1.0/24"]}]}`)
	binary, err := runtime.CompileRuleSet(second)
	if err != nil {
		t.Fatal(err)
	}
	publisher := &Publisher{Fetcher: testFetcher{bodies: map[string][]byte{"https://example.com/ip.srs": binary}}}
	artifact, err := publisher.Publish(context.Background(), []*models.RuleSetSource{
		{Kind: "manual", Format: "source", Content: `{"version":4,"rules":[{"domain_suffix":["example.com"]}]}`},
		{Kind: "remote", Format: "binary", URL: "https://example.com/ip.srs"},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.RuleCount != 2 || len(artifact.SRS) == 0 {
		t.Fatalf("artifact = %+v", artifact)
	}
}
