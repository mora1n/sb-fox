// Package ruleset builds deterministic sing-box source JSON and binary SRS
// artifacts from ordered manual and remote inputs.
package ruleset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/subfetch"
)

const (
	MaxSourceBytes = int64(64 << 20)
	MaxTotalBytes  = int64(256 << 20)
)

type rawFetcher interface {
	FetchBytes(context.Context, string, subfetch.ByteOptions) (subfetch.ByteResult, error)
}

type ruleSetKernel interface {
	Version() (string, error)
	FormatRuleSet([]byte) ([]byte, error)
	DecompileRuleSet([]byte) ([]byte, error)
	CompileRuleSet([]byte) ([]byte, error)
}

type Publisher struct {
	Fetcher rawFetcher
}

type Artifact struct {
	JSON          []byte
	SRS           []byte
	RuleCount     int
	JSONSHA256    string
	SRSSHA256     string
	KernelVersion string
}

type Error struct {
	Stage        string
	SourceIndex  *int
	SourceKind   string
	SourceFormat string
	URL          string
	Err          error
}

func (e *Error) Error() string {
	prefix := "rule-set " + e.Stage
	if e.SourceIndex != nil {
		prefix += fmt.Sprintf(" source[%d]", *e.SourceIndex)
	}
	if e.URL != "" {
		prefix += " " + e.URL
	}
	return prefix + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

type LimitError struct{ Message string }

func (e *LimitError) Error() string { return e.Message }

type sourceDocument struct {
	Version int               `json:"version"`
	Rules   []json.RawMessage `json:"rules"`
}

func (p *Publisher) Publish(ctx context.Context, sources []*models.RuleSetSource, runtime ruleSetKernel) (Artifact, error) {
	if len(sources) == 0 {
		return Artifact{}, errors.New("rule-set requires at least one source")
	}
	if p == nil || p.Fetcher == nil {
		return Artifact{}, errors.New("rule-set fetcher is unavailable")
	}
	if runtime == nil {
		return Artifact{}, errors.New("rule-set kernel is unavailable")
	}
	documents := make([]sourceDocument, 0, len(sources))
	var total int64
	for index, source := range sources {
		document, inputSize, err := p.prepareSource(ctx, index, source, runtime)
		if err != nil {
			return Artifact{}, err
		}
		total += inputSize
		if total > MaxTotalBytes {
			return Artifact{}, sourceError(index, source, subfetch.SafeURL(source.URL), "limit", &LimitError{
				Message: fmt.Sprintf("rule-set inputs exceed %d bytes", MaxTotalBytes),
			})
		}
		documents = append(documents, document)
	}
	merged, ruleCount, err := mergeDocuments(documents)
	if err != nil {
		return Artifact{}, &Error{Stage: "merge", Err: err}
	}
	formatted, err := runtime.FormatRuleSet(merged)
	if err != nil {
		return Artifact{}, &Error{Stage: "format", Err: err}
	}
	binary, err := runtime.CompileRuleSet(formatted)
	if err != nil {
		return Artifact{}, &Error{Stage: "compile", Err: err}
	}
	version, err := runtime.Version()
	if err != nil {
		return Artifact{}, &Error{Stage: "version", Err: err}
	}
	return Artifact{
		JSON:          formatted,
		SRS:           binary,
		RuleCount:     ruleCount,
		JSONSHA256:    digest(formatted),
		SRSSHA256:     digest(binary),
		KernelVersion: version,
	}, nil
}

func (p *Publisher) prepareSource(ctx context.Context, index int, source *models.RuleSetSource, runtime ruleSetKernel) (sourceDocument, int64, error) {
	raw, safeURL, err := p.readSource(ctx, source)
	if err != nil {
		stage := "fetch"
		var limit *LimitError
		if errors.As(err, &limit) {
			stage = "limit"
		}
		return sourceDocument{}, 0, sourceError(index, source, safeURL, stage, err)
	}
	inputSize := int64(len(raw))
	if source.Format == "binary" {
		raw, err = runtime.DecompileRuleSet(raw)
		if err != nil {
			return sourceDocument{}, 0, sourceError(index, source, safeURL, "decompile", err)
		}
	}
	formatted, err := runtime.FormatRuleSet(raw)
	if err != nil {
		return sourceDocument{}, 0, sourceError(index, source, safeURL, "format", err)
	}
	var document sourceDocument
	if err := json.Unmarshal(formatted, &document); err != nil {
		return sourceDocument{}, 0, sourceError(index, source, safeURL, "decode", err)
	}
	if document.Version <= 0 {
		return sourceDocument{}, 0, sourceError(index, source, safeURL, "decode", errors.New("missing rule-set version"))
	}
	return document, inputSize, nil
}

func (p *Publisher) readSource(ctx context.Context, source *models.RuleSetSource) ([]byte, string, error) {
	if source == nil {
		return nil, "", errors.New("source is null")
	}
	if source.Kind != "manual" && source.Kind != "remote" {
		return nil, "", fmt.Errorf("unknown source kind %q", source.Kind)
	}
	if source.Format != "source" && source.Format != "binary" {
		return nil, "", fmt.Errorf("unknown source format %q", source.Format)
	}
	if source.Kind == "manual" {
		if source.Format != "source" {
			return nil, "", errors.New("manual source must use source format")
		}
		content := []byte(strings.TrimSpace(source.Content))
		if len(content) == 0 {
			return nil, "", errors.New("manual source content is required")
		}
		if int64(len(content)) > MaxSourceBytes {
			return nil, "", &LimitError{Message: fmt.Sprintf("source exceeds %d bytes", MaxSourceBytes)}
		}
		return content, "", nil
	}
	if strings.TrimSpace(source.URL) == "" {
		return nil, "", errors.New("remote source URL is required")
	}
	result, err := p.Fetcher.FetchBytes(ctx, source.URL, subfetch.ByteOptions{
		Request:  subfetch.Options{NoCache: true},
		MaxBytes: MaxSourceBytes,
	})
	if err != nil {
		return nil, subfetch.SafeURL(source.URL), err
	}
	return result.Body, result.URL, nil
}

func mergeDocuments(documents []sourceDocument) ([]byte, int, error) {
	merged := sourceDocument{Rules: []json.RawMessage{}}
	seen := make(map[string]struct{})
	for _, document := range documents {
		if document.Version > merged.Version {
			merged.Version = document.Version
		}
		for _, raw := range document.Rules {
			canonical, err := canonicalJSON(raw)
			if err != nil {
				return nil, 0, err
			}
			key := string(canonical)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			merged.Rules = append(merged.Rules, canonical)
		}
	}
	if merged.Version <= 0 {
		return nil, 0, errors.New("missing rule-set version")
	}
	output, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	return append(output, '\n'), len(merged.Rules), nil
}

func canonicalJSON(raw json.RawMessage) ([]byte, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func sourceError(index int, source *models.RuleSetSource, url, stage string, err error) *Error {
	result := &Error{Stage: stage, SourceIndex: &index, URL: url, Err: err}
	if source != nil {
		result.SourceKind = source.Kind
		result.SourceFormat = source.Format
	}
	return result
}

func digest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
