// Package sblink parses proxy share-link URIs (ss, vmess, vless, trojan,
// hysteria2, tuic) into sing-box outbound JSON objects, represented as
// *merge.OrderedMap so that field ordering stays deterministic.
package sblink

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

// Parse dispatches on the URI scheme prefix and returns the sing-box outbound.
func Parse(uri string) (*merge.OrderedMap, error) {
	uri = strings.TrimSpace(uri)
	switch {
	case strings.HasPrefix(uri, "ss://"):
		return parseShadowsocks(uri)
	case strings.HasPrefix(uri, "vmess://"):
		return parseVMess(uri)
	case strings.HasPrefix(uri, "vless://"):
		return parseVLESS(uri)
	case strings.HasPrefix(uri, "trojan://"):
		return parseTrojan(uri)
	case strings.HasPrefix(uri, "hysteria2://"):
		return parseHysteria2(strings.TrimPrefix(uri, "hysteria2://"))
	case strings.HasPrefix(uri, "hy2://"):
		return parseHysteria2(strings.TrimPrefix(uri, "hy2://"))
	case strings.HasPrefix(uri, "tuic://"):
		return parseTUIC(uri)
	default:
		return nil, fmt.Errorf("sblink: unknown scheme in %q", uri)
	}
}

// ParseMany splits text into individual links and parses each. If the whole
// input does not look like links, it tries base64-decoding first. Blank lines
// are skipped. It errors only when every candidate line fails.
func ParseMany(text string) ([]*merge.OrderedMap, error) {
	lines := splitLinks(text)
	if !anyLooksLikeLink(lines) {
		if decoded, ok := tryBase64(text); ok {
			lines = splitLinks(decoded)
		}
	}

	var out []*merge.OrderedMap
	var lastErr error
	seen := false
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		seen = true
		m, err := Parse(ln)
		if err != nil {
			lastErr = err
			continue
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("sblink: no links parsed: %w", lastErr)
		}
		if seen {
			return nil, fmt.Errorf("sblink: no links parsed")
		}
		return nil, fmt.Errorf("sblink: empty input")
	}
	return out, nil
}

func splitLinks(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

func anyLooksLikeLink(lines []string) bool {
	for _, ln := range lines {
		if looksLikeLink(strings.TrimSpace(ln)) {
			return true
		}
	}
	return false
}

func looksLikeLink(ln string) bool {
	for _, p := range []string{"ss://", "vmess://", "vless://", "trojan://", "hysteria2://", "hy2://", "tuic://"} {
		if strings.HasPrefix(ln, p) {
			return true
		}
	}
	return false
}

// tryBase64 attempts to decode s as a base64 blob (tolerant of encoding
// variants) and reports whether the decoded text contains link-like content.
func tryBase64(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	dec, ok := decodeBase64(s)
	if !ok {
		return "", false
	}
	return string(dec), true
}

// decodeBase64 tries std, raw-std, url and raw-url base64 encodings, returning
// the first successful decode. Proxy links use all of these variants.
func decodeBase64(s string) ([]byte, bool) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		if b, err := enc.DecodeString(s); err == nil {
			return b, true
		}
	}
	return nil, false
}

// tagOrDefault URL-decodes the fragment as the outbound tag, falling back to
// "server:port" when no fragment is present.
func tagOrDefault(fragment, server, port string) string {
	if fragment != "" {
		if dec, err := url.QueryUnescape(fragment); err == nil {
			return dec
		}
		return fragment
	}
	if port != "" {
		return server + ":" + port
	}
	return server
}

// cleanServer strips a literal `#CC` country suffix from a server host using
// the same precedence as the merge package. The country marking itself is
// handled elsewhere; here we only return the clean host.
func cleanServer(server string) string {
	if ov := merge.ExtractServerCountryOverride(server); ov != nil {
		return ov.Server
	}
	return server
}

// portNumber converts a port string into a json.Number, erroring on invalid
// input so parsing fails loudly rather than emitting a bad outbound.
func portNumber(port string) (json.Number, error) {
	port = strings.TrimSpace(port)
	if port == "" {
		return "", fmt.Errorf("sblink: missing port")
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return "", fmt.Errorf("sblink: invalid port %q: %w", port, err)
	}
	return json.Number(strconv.Itoa(n)), nil
}

// intNumber wraps an int as a json.Number for integer JSON fields.
func intNumber(n int) json.Number {
	return json.Number(strconv.Itoa(n))
}

// anyToString renders a JSON-decoded scalar (string, json.Number, float64,
// bool, nil) as its string form. Used for vmess fields that may be either a
// string or a number depending on the producing client.
func anyToString(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(val)
	case json.Number:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// atoiDefault parses s as an int, returning def when it is empty or invalid.
func atoiDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	// Tolerate a float form like "0.0" that some clients emit.
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
