package sblink

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

func queryFirst(q url.Values, keys ...string) string {
	for _, key := range keys {
		if value := q.Get(key); value != "" {
			return value
		}
	}
	return ""
}

func boolQuery(q url.Values, keys ...string) bool {
	for _, key := range keys {
		if _, ok := q[key]; ok {
			return boolParam(q.Get(key))
		}
	}
	return false
}

func setStringIfPresent(out *merge.OrderedMap, key string, value string) {
	if strings.TrimSpace(value) != "" {
		out.Set(key, value)
	}
}

func setIntIfPresent(out *merge.OrderedMap, key string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("sblink: invalid %s %q: %w", key, value, err)
	}
	out.Set(key, intNumber(n))
	return nil
}

func setStringListIfPresent(out *merge.OrderedMap, key string, values []string) {
	if len(values) == 0 {
		return
	}
	arr := make([]any, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			arr = append(arr, value)
		}
	}
	if len(arr) > 0 {
		out.Set(key, arr)
	}
}

func parseHeadersObject(raw string) (*merge.OrderedMap, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	headers, err := merge.ParseOrdered([]byte(raw))
	if err != nil {
		return nil, fmt.Errorf("sblink: headers must be a JSON object: %w", err)
	}
	return headers, nil
}

func encodeHeadersObject(headers *merge.OrderedMap) (string, error) {
	if headers == nil || len(headers.Keys()) == 0 {
		return "", nil
	}
	raw, err := headers.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("sblink: headers JSON: %w", err)
	}
	return string(raw), nil
}

func orderedFromMap(m map[string]any) *merge.OrderedMap {
	if len(m) == 0 {
		return nil
	}
	out := merge.NewOrderedMap()
	for key, value := range m {
		out.Set(key, orderedAny(value))
	}
	return out
}

func orderedAny(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return orderedFromMap(value)
	case map[any]any:
		converted := make(map[string]any, len(value))
		for key, item := range value {
			converted[anyToString(key)] = item
		}
		return orderedFromMap(converted)
	case []any:
		items := make([]any, len(value))
		for i, item := range value {
			items[i] = orderedAny(item)
		}
		return items
	default:
		return value
	}
}

func tlsFromQuery(q url.Values, enabled bool, allowReality bool) (*merge.OrderedMap, error) {
	params := tlsParams{
		enabled:     enabled,
		serverName:  queryFirst(q, "sni", "peer", "servername"),
		insecure:    boolQuery(q, "insecure", "allowInsecure", "allow-insecure", "skip-cert-verify"),
		alpn:        splitALPN(queryFirst(q, "alpn")),
		fingerprint: queryFirst(q, "fp", "fingerprint", "client-fingerprint"),
	}
	if allowReality && queryFirst(q, "security") == "reality" {
		params.realityPbk = q.Get("pbk")
		params.realitySid = q.Get("sid")
	}
	return buildTLS(params), nil
}

func looksLikeHTTPProxyLink(ln string) bool {
	u, err := url.Parse(ln)
	if err != nil || u.Scheme == "" || u.Hostname() == "" {
		return false
	}
	if u.User != nil || u.Fragment != "" {
		return true
	}
	q := u.Query()
	for _, key := range []string{"tls", "headers", "sni", "insecure", "skip-cert-verify", "fingerprint"} {
		if _, ok := q[key]; ok {
			return true
		}
	}
	return false
}
