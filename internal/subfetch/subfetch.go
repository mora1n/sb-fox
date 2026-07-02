// Package subfetch fetches subscription content from user-supplied URLs with an
// SSRF guard: only http/https, and connections to private/loopback/link-local
// addresses are refused (re-checked after redirects). Content is size-capped
// and base64/plain decoded.
package subfetch

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxResponseBytes caps a subscription body (2 MiB is generous for node lists).
const maxResponseBytes = 2 << 20

// Fetcher retrieves subscription bodies safely.
type Fetcher struct {
	client *http.Client
	// AllowPrivate disables the SSRF guard (for trusted/self-hosted setups).
	AllowPrivate bool
}

// New returns a Fetcher with a guarded dialer and sane timeouts.
func New() *Fetcher {
	f := &Fetcher{}
	f.client = &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("subfetch: too many redirects")
			}
			return f.guardRequest(req)
		},
		Transport: &http.Transport{
			DialContext:         f.guardedDial,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        10,
		},
	}
	return f
}

// Fetch retrieves url and returns the decoded text body. The body is
// base64-decoded when it appears to be a base64 blob (common for subscriptions).
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (string, error) {
	if err := validateURL(rawURL); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "sb-fox/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("subfetch: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", err
	}
	return DecodeBody(string(body)), nil
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
