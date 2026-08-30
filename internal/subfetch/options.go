package subfetch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Options are server-side defaults for a subscription fetch. URL fragments can
// override these per request.
type Options struct {
	UserAgent string
	Headers   map[string]string
	NoCache   bool
	Insecure  bool
	CacheTTL  time.Duration
}

type Result struct {
	URL       string
	Body      string
	FromCache bool
}

// ByteOptions configures an uncached raw response fetch. MaxBytes must be
// positive; callers use it to make resource limits explicit.
type ByteOptions struct {
	Request  Options
	MaxBytes int64
}

type ByteResult struct {
	URL  string
	Body []byte
}

type BatchResult struct {
	Bodies []string
	Items  []BatchItem
}

type BatchItem struct {
	URL       string `json:"url"`
	OK        bool   `json:"ok"`
	Nodes     int    `json:"nodes,omitempty"`
	Error     string `json:"error,omitempty"`
	FromCache bool   `json:"from_cache,omitempty"`
	Body      string `json:"-"`
}

type cacheEntry struct {
	body      string
	expiresAt time.Time
}

type urlSpec struct {
	URL      string
	SafeURL  string
	Headers  map[string]string
	NoCache  bool
	Insecure bool
	CacheTTL time.Duration
	CacheKey string
}

func splitInputURLs(raw string) []string {
	lines := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func parseURLSpec(rawURL string, opts Options) (urlSpec, error) {
	cleanURL, args, err := splitURLArgs(rawURL)
	if err != nil {
		return urlSpec{}, err
	}
	headers := http.Header{}
	userAgent := strings.TrimSpace(opts.UserAgent)
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	for k, v := range opts.Headers {
		if strings.TrimSpace(k) != "" {
			headers.Set(k, v)
		}
	}
	headers.Set("User-Agent", userAgent)

	noCache := opts.NoCache
	insecure := opts.Insecure
	cacheTTL := opts.CacheTTL
	for key, value := range args {
		switch strings.ToLower(key) {
		case "nocache", "no-cache":
			b, err := parseBoolArg(value)
			if err != nil {
				return urlSpec{}, fmt.Errorf("subfetch: invalid noCache: %w", err)
			}
			noCache = b
		case "insecure":
			b, err := parseBoolArg(value)
			if err != nil {
				return urlSpec{}, fmt.Errorf("subfetch: invalid insecure: %w", err)
			}
			insecure = b
		case "ua", "useragent", "user-agent":
			if strings.TrimSpace(value) != "" {
				headers.Set("User-Agent", value)
			}
		case "headers":
			parsed, err := parseHeadersArg(value)
			if err != nil {
				return urlSpec{}, err
			}
			for k, values := range parsed {
				if len(values) > 0 {
					headers.Set(k, values[0])
				}
			}
		case "cachettl", "cache-ttl":
			ttl, err := parseTTLArg(value)
			if err != nil {
				return urlSpec{}, err
			}
			cacheTTL = ttl
		}
	}
	if cacheTTL <= 0 {
		cacheTTL = defaultCacheTTL
	}
	spec := urlSpec{
		URL:      cleanURL,
		SafeURL:  SafeURL(rawURL),
		Headers:  headerMap(headers),
		NoCache:  noCache,
		Insecure: insecure,
		CacheTTL: cacheTTL,
	}
	spec.CacheKey = cacheKey(spec)
	return spec, nil
}

func splitURLArgs(rawURL string) (string, map[string]string, error) {
	cleanURL, fragment, ok := strings.Cut(strings.TrimSpace(rawURL), "#")
	if cleanURL == "" {
		return "", nil, errors.New("subfetch: empty url")
	}
	args := map[string]string{}
	if !ok || strings.TrimSpace(fragment) == "" {
		return cleanURL, args, nil
	}
	decoded, err := url.QueryUnescape(fragment)
	if err != nil {
		return "", nil, fmt.Errorf("subfetch: invalid url options: %w", err)
	}
	if strings.HasPrefix(strings.TrimSpace(decoded), "{") {
		args, err := argsFromJSON(decoded)
		return cleanURL, args, err
	}
	for _, pair := range strings.Split(fragment, "&") {
		if strings.TrimSpace(pair) == "" {
			continue
		}
		key, value, found := strings.Cut(pair, "=")
		key, err = url.QueryUnescape(strings.TrimSpace(key))
		if err != nil {
			return "", nil, fmt.Errorf("subfetch: invalid url option key: %w", err)
		}
		if !found {
			args[key] = "true"
			continue
		}
		value, err = url.QueryUnescape(value)
		if err != nil {
			return "", nil, fmt.Errorf("subfetch: invalid url option value: %w", err)
		}
		args[key] = value
	}
	return cleanURL, args, nil
}

func argsFromJSON(raw string) (map[string]string, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("subfetch: invalid url options json: %w", err)
	}
	args := make(map[string]string, len(data))
	for key, value := range data {
		switch v := value.(type) {
		case string:
			args[key] = v
		case bool:
			args[key] = strconv.FormatBool(v)
		case float64:
			args[key] = strconv.FormatFloat(v, 'f', -1, 64)
		case map[string]any:
			rawValue, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			args[key] = string(rawValue)
		default:
			args[key] = fmt.Sprint(v)
		}
	}
	return args, nil
}

func parseBoolArg(value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return true, nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", value)
	}
}

func parseTTLArg(value string) (time.Duration, error) {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("subfetch: invalid cacheTtl %q", value)
	}
	if seconds == 0 {
		return defaultCacheTTL, nil
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseHeadersArg(value string) (http.Header, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return nil, fmt.Errorf("subfetch: invalid headers json: %w", err)
	}
	headers := http.Header{}
	for key, item := range raw {
		if strings.TrimSpace(key) == "" {
			continue
		}
		switch v := item.(type) {
		case string:
			headers.Set(key, v)
		case float64:
			headers.Set(key, strconv.FormatFloat(v, 'f', -1, 64))
		case bool:
			headers.Set(key, strconv.FormatBool(v))
		default:
			headers.Set(key, fmt.Sprint(v))
		}
	}
	return headers, nil
}

func headerMap(headers http.Header) map[string]string {
	out := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			out[strings.ToLower(key)] = values[0]
		}
	}
	return out
}

func cacheKey(spec urlSpec) string {
	keys := make([]string, 0, len(spec.Headers))
	for key := range spec.Headers {
		keys = append(keys, strings.ToLower(key))
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(spec.URL)
	b.WriteString("\n")
	b.WriteString(strconv.FormatBool(spec.Insecure))
	for _, key := range keys {
		b.WriteString("\n")
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(spec.Headers[key])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// SafeURL returns a URL suitable for logs or API warnings. It strips fragments,
// userinfo and query values.
func SafeURL(rawURL string) string {
	cleanURL, _, _ := strings.Cut(strings.TrimSpace(rawURL), "#")
	u, err := url.Parse(cleanURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return cleanURL
	}
	u.User = nil
	u.Fragment = ""
	if u.RawQuery != "" {
		u.RawQuery = "..."
	}
	return u.String()
}
