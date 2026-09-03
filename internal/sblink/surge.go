package sblink

import (
	"fmt"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

// looksLikeSurgeProfile identifies the INI-style profile format used by Surge.
// Surge profiles are not YAML: proxy declarations live in a [Proxy] section.
func looksLikeSurgeProfile(text string) bool {
	for _, rawLine := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimPrefix(strings.TrimSpace(rawLine), "\ufeff")
		if len(line) >= 2 && line[0] == '[' && line[len(line)-1] == ']' &&
			strings.EqualFold(strings.TrimSpace(line[1:len(line)-1]), "proxy") {
			return true
		}
	}
	return false
}

func parseSurgeProfile(text string) ([]*merge.OrderedMap, []string, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	inProxy := false
	var out []*merge.OrderedMap
	var warnings []string
	for _, rawLine := range lines {
		line := strings.TrimPrefix(strings.TrimSpace(rawLine), "\ufeff")
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if len(line) >= 2 && line[0] == '[' && line[len(line)-1] == ']' {
			section := strings.TrimSpace(line[1 : len(line)-1])
			inProxy = strings.EqualFold(section, "proxy")
			continue
		}
		if !inProxy || strings.HasPrefix(line, "#!") {
			continue
		}
		name, fields, err := parseSurgeProxyLine(line)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("surge: %v", err))
			continue
		}
		if len(fields) == 0 || surgeIgnoredProxyType(fields[0]) {
			continue
		}
		proxy, err := surgeProxyOutbound(name, fields)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		out = append(out, proxy)
	}
	if len(out) == 0 {
		return nil, warnings, fmt.Errorf("sblink: no supported surge proxies parsed")
	}
	return out, warnings, nil
}

func parseSurgeProxyLine(line string) (string, []string, error) {
	eq := indexOutsideQuotes(line, '=')
	if eq <= 0 {
		return "", nil, fmt.Errorf("invalid proxy declaration")
	}
	nameParts, err := splitSurgeFields(line[:eq])
	if err != nil {
		return "", nil, err
	}
	if len(nameParts) != 1 || strings.TrimSpace(nameParts[0]) == "" {
		return "", nil, fmt.Errorf("proxy declaration has an empty name")
	}
	fields, err := splitSurgeFields(line[eq+1:])
	if err != nil {
		return "", nil, err
	}
	return strings.TrimSpace(nameParts[0]), fields, nil
}

func splitSurgeFields(raw string) ([]string, error) {
	var fields []string
	var field strings.Builder
	var quote rune
	escaped := false
	for _, ch := range raw {
		if escaped {
			field.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			} else {
				field.WriteRune(ch)
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case ',':
			fields = append(fields, strings.TrimSpace(field.String()))
			field.Reset()
		default:
			field.WriteRune(ch)
		}
	}
	if escaped {
		field.WriteByte('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in proxy declaration")
	}
	fields = append(fields, strings.TrimSpace(field.String()))
	return fields, nil
}

func indexOutsideQuotes(value string, target rune) int {
	var quote rune
	escaped := false
	for index, ch := range value {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == target {
			return index
		}
	}
	return -1
}

func surgeIgnoredProxyType(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "direct", "reject", "reject-tinygif", "pass", "compatible":
		return true
	default:
		return false
	}
}

func surgeProxyOutbound(name string, fields []string) (*merge.OrderedMap, error) {
	if len(fields) < 3 {
		return nil, fmt.Errorf("proxy declaration requires type, server and port")
	}
	typ := strings.ToLower(strings.TrimSpace(fields[0]))
	p := map[string]any{
		"name":   name,
		"type":   typ,
		"server": strings.TrimSpace(fields[1]),
		"port":   strings.TrimSpace(fields[2]),
	}
	options, err := surgeOptions(fields[3:])
	if err != nil {
		return nil, err
	}
	for key, value := range options {
		p[key] = value
	}
	normalizeSurgeProxyOptions(p, typ, options)
	if typ == "socks5-tls" {
		p["type"] = "socks5"
		p["tls"] = "true"
	}
	if typ == "http-tls" {
		p["type"] = "https"
	}
	return clashProxyOutbound(p)
}

func surgeOptions(fields []string) (map[string]string, error) {
	options := make(map[string]string, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		eq := indexOutsideQuotes(field, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("invalid option %q", field)
		}
		key := normalizeSurgeKey(field[:eq])
		if key == "" {
			return nil, fmt.Errorf("option has an empty key")
		}
		options[key] = strings.TrimSpace(field[eq+1:])
	}
	return options, nil
}

func normalizeSurgeKey(key string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", "-"))
}

func normalizeSurgeProxyOptions(p map[string]any, typ string, options map[string]string) {
	if value := surgeOption(options, "encrypt-method", "method", "cipher"); value != "" {
		p["cipher"] = value
	}
	if value := surgeOption(options, "username", "uuid"); value != "" {
		if typ == "vmess" || typ == "vless" || typ == "tuic" {
			p["uuid"] = value
		} else {
			p["username"] = value
		}
	}
	if value := surgeOption(options, "password", "psk"); value != "" && typ == "snell" {
		p["psk"] = value
	}
	if value := surgeOption(options, "auth", "password"); value != "" && typ == "hysteria2" {
		p["password"] = value
	}
	if typ == "snell" {
		if value := surgeOption(options, "obfs", "obfs-mode"); value != "" {
			p["obfs"] = value
		}
		if value := surgeOption(options, "obfs-host", "obfs-hostname"); value != "" {
			p["obfs-host"] = value
		}
	}
	if value := surgeOption(options, "ws-path", "path"); value != "" {
		if typ == "vmess" || typ == "vless" || typ == "trojan" {
			p["network"] = "ws"
			p["ws-opts"] = map[string]any{"path": value}
		}
	}
	if value := surgeOption(options, "ws-host", "host"); value != "" {
		if opts, ok := p["ws-opts"].(map[string]any); ok {
			opts["host"] = value
		} else if typ == "vmess" || typ == "vless" || typ == "trojan" {
			p["network"] = "ws"
			p["ws-opts"] = map[string]any{"headers": map[string]any{"Host": value}}
		}
	}
	if value := surgeOption(options, "sni", "servername"); value != "" {
		p["sni"] = value
	}
}

func surgeOption(options map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := options[normalizeSurgeKey(key)]; value != "" {
			return value
		}
	}
	return ""
}
