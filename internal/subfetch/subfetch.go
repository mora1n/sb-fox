// Package subfetch fetches subscription content from user-supplied URLs with an
// SSRF guard: only http/https, and connections to private/loopback/link-local
// addresses are refused (re-checked after redirects). Content is size-capped
// and base64/plain decoded.
package subfetch

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// maxResponseBytes caps a subscription body (2 MiB is generous for node lists).
const maxResponseBytes = 2 << 20
const defaultUserAgent = "clash.meta/v1.19.23"
const defaultCacheTTL = 5 * time.Minute

// Fetcher retrieves subscription bodies safely.
type Fetcher struct {
	client         *http.Client
	insecureClient *http.Client
	mu             sync.Mutex
	cache          map[string]cacheEntry
	// AllowPrivate disables the SSRF guard (for trusted/self-hosted setups).
	AllowPrivate bool
}

// New returns a Fetcher with a guarded dialer and sane timeouts.
func New() *Fetcher {
	f := &Fetcher{}
	f.client = f.newHTTPClient(false)
	f.insecureClient = f.newHTTPClient(true)
	f.cache = map[string]cacheEntry{}
	return f
}

func (f *Fetcher) newHTTPClient(insecure bool) *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("subfetch: too many redirects")
			}
			return f.guardRequest(req)
		},
		Transport: &http.Transport{
			DialContext:         f.guardedDial,
			TLSClientConfig:     tlsConfig(insecure),
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        10,
		},
	}
}

func tlsConfig(insecure bool) *tls.Config {
	if !insecure {
		return nil
	}
	return &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit per-URL opt-in for subscription sources.
}

// Fetch retrieves url and returns the decoded text body. The body is
// base64-decoded when it appears to be a base64 blob (common for subscriptions).
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (string, error) {
	result, err := f.FetchWithOptions(ctx, rawURL, Options{})
	if err != nil {
		return "", err
	}
	return result.Body, nil
}

// FetchBytes retrieves an uncached raw HTTP body without text/base64
// transformations. It is used for source JSON and binary SRS rule-set inputs.
func (f *Fetcher) FetchBytes(ctx context.Context, rawURL string, opts ByteOptions) (ByteResult, error) {
	if opts.MaxBytes <= 0 {
		return ByteResult{}, errors.New("subfetch: max bytes must be positive")
	}
	opts.Request.NoCache = true
	spec, err := parseURLSpec(rawURL, opts.Request)
	if err != nil {
		return ByteResult{}, err
	}
	if err := validateURL(spec.URL); err != nil {
		return ByteResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.URL, nil)
	if err != nil {
		return ByteResult{}, err
	}
	for key, value := range spec.Headers {
		req.Header.Set(key, value)
	}
	client := f.client
	if spec.Insecure {
		client = f.insecureClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return ByteResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ByteResult{}, fmt.Errorf("subfetch: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBytes+1))
	if err != nil {
		return ByteResult{}, err
	}
	if int64(len(body)) > opts.MaxBytes {
		return ByteResult{}, fmt.Errorf("subfetch: response exceeds %d bytes", opts.MaxBytes)
	}
	return ByteResult{URL: spec.SafeURL, Body: body}, nil
}

// FetchWithOptions retrieves a single subscription URL with Sub-Store-style
// fragment options stripped from the actual request URL.
func (f *Fetcher) FetchWithOptions(ctx context.Context, rawURL string, opts Options) (Result, error) {
	spec, err := parseURLSpec(rawURL, opts)
	if err != nil {
		return Result{}, err
	}
	if err := validateURL(spec.URL); err != nil {
		return Result{}, err
	}
	if cached, ok := f.getCached(spec.CacheKey, spec.NoCache); ok {
		return Result{URL: spec.SafeURL, Body: DecodeBody(cached), FromCache: true}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.URL, nil)
	if err != nil {
		return Result{}, err
	}
	for key, value := range spec.Headers {
		req.Header.Set(key, value)
	}

	client := f.client
	if spec.Insecure {
		client = f.insecureClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("subfetch: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Result{}, err
	}
	raw := string(body)
	f.setCached(spec.CacheKey, raw, spec.CacheTTL, spec.NoCache)
	return Result{URL: spec.SafeURL, Body: DecodeBody(raw)}, nil
}

// FetchMany retrieves newline-separated subscription URLs. It keeps successful
// bodies in input order and returns per-URL failures for caller-level warnings.
func (f *Fetcher) FetchMany(ctx context.Context, rawURLs string, opts Options) (BatchResult, error) {
	urls := splitInputURLs(rawURLs)
	if len(urls) == 0 {
		return BatchResult{}, errors.New("subfetch: empty url")
	}
	result := BatchResult{Items: make([]BatchItem, 0, len(urls))}
	for _, rawURL := range urls {
		item := BatchItem{URL: SafeURL(rawURL)}
		fetched, err := f.FetchWithOptions(ctx, rawURL, opts)
		if err != nil {
			item.Error = err.Error()
			result.Items = append(result.Items, item)
			continue
		}
		item.URL = fetched.URL
		item.OK = true
		item.FromCache = fetched.FromCache
		item.Body = fetched.Body
		result.Items = append(result.Items, item)
		result.Bodies = append(result.Bodies, fetched.Body)
	}
	if len(result.Bodies) == 0 {
		return result, errors.New("subfetch: all urls failed")
	}
	return result, nil
}

func (f *Fetcher) getCached(key string, noCache bool) (string, bool) {
	if noCache || key == "" {
		return "", false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, ok := f.cache[key]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expiresAt) {
		delete(f.cache, key)
		return "", false
	}
	return entry.body, true
}

func (f *Fetcher) setCached(key, body string, ttl time.Duration, noCache bool) {
	if noCache || key == "" || body == "" {
		return
	}
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache[key] = cacheEntry{body: body, expiresAt: time.Now().Add(ttl)}
}

// DecodeBody returns the plausibly-base64-decoded body, else the original.
func DecodeBody(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return body
	}
	// A subscription body is typically either raw links (contains "://") or a
	// single base64 blob. Only attempt decode when it doesn't already look raw.
	if strings.Contains(trimmed, "://") {
		return body
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	} {
		if decoded, err := enc.DecodeString(trimmed); err == nil {
			if s := string(decoded); strings.Contains(s, "://") {
				return s
			}
		}
	}
	return body
}

// validateURL enforces the http/https scheme requirement.
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("subfetch: invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("subfetch: unsupported scheme %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return errors.New("subfetch: missing host")
	}
	return nil
}

// guardRequest re-validates a redirect target's host.
func (f *Fetcher) guardRequest(req *http.Request) error {
	if f.AllowPrivate {
		return nil
	}
	host := req.URL.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("subfetch: resolve %q: %w", host, err)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("subfetch: refusing connection to non-public address %s", ip)
		}
	}
	return nil
}

// guardedDial resolves and checks every candidate IP before dialing.
func (f *Fetcher) guardedDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	var dialErr error
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	for _, ip := range ips {
		if !f.AllowPrivate && isBlockedIP(ip) {
			return nil, fmt.Errorf("subfetch: refusing connection to non-public address %s", ip)
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		dialErr = err
	}
	if dialErr == nil {
		dialErr = fmt.Errorf("subfetch: no addresses for %q", host)
	}
	return nil, dialErr
}

// isBlockedIP reports whether ip is in a range we refuse to connect to.
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// cloud metadata endpoint
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	// IPv4 shared address space (CGNAT) 100.64.0.0/10
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1]&0xC0 == 64 {
		return true
	}
	return false
}
