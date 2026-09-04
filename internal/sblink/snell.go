package sblink

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

// parseSnell accepts the two common Snell URI layouts used by subscription
// tools. sing-box 1.14 exposes Snell v4 and v6 as outbounds; a v5 server uses
// the v4 client wire format unless it requires the unsupported QUIC mode.
func parseSnell(uri string) (*merge.OrderedMap, error) {
	p, err := parseURILink(uri)
	if err != nil {
		return nil, err
	}
	psk, userKey, err := snellCredentials(p)
	if err != nil {
		return nil, err
	}
	version, err := snellVersion(p.query.Get("version"))
	if err != nil {
		return nil, err
	}
	if boolQuery(p.query, "quic", "quic_proxy", "quic-proxy") {
		return nil, fmt.Errorf("sblink: snell QUIC mode is not supported by sing-box")
	}
	if version == 6 && (len([]byte(psk)) < 12 || len([]byte(psk)) > 255) {
		return nil, fmt.Errorf("sblink: snell v6 psk must be 12-255 bytes")
	}

	out := merge.NewOrderedMap()
	out.Set("type", "snell")
	out.Set("tag", p.tag)
	out.Set("server", p.server)
	out.Set("server_port", p.portN)
	out.Set("version", intNumber(version))
	out.Set("psk", psk)
	setStringIfPresent(out, "userkey", userKey)
	if boolQuery(p.query, "reuse") {
		out.Set("reuse", true)
	}
	if network := strings.TrimSpace(queryFirst(p.query, "network")); network != "" {
		if err := validateSnellNetwork(network); err != nil {
			return nil, err
		}
		out.Set("network", snellNetworkValue(network))
	}
	if version == 6 {
		if obfs := queryFirst(p.query, "obfs", "obfs_mode", "obfs-mode"); obfs != "" {
			return nil, fmt.Errorf("sblink: snell obfs is only valid for version 4")
		}
		if mode := strings.TrimSpace(p.query.Get("mode")); mode != "" {
			if mode != "default" && mode != "unshaped" && mode != "unsafe-raw" {
				return nil, fmt.Errorf("sblink: unsupported snell v6 mode %q", mode)
			}
			out.Set("mode", mode)
		}
		return out, nil
	}
	obfsMode := strings.ToLower(strings.TrimSpace(queryFirst(p.query, "obfs", "obfs_mode", "obfs-mode")))
	if obfsMode != "" && obfsMode != "none" && obfsMode != "http" && obfsMode != "tls" {
		return nil, fmt.Errorf("sblink: unsupported snell v4 obfs mode %q", obfsMode)
	}
	if obfsMode == "http" || obfsMode == "tls" {
		out.Set("obfs_mode", obfsMode)
		setStringIfPresent(out, "obfs_host", queryFirst(p.query, "obfs-host", "obfs_host"))
	}
	if p.query.Get("mode") != "" {
		return nil, fmt.Errorf("sblink: snell mode is only valid for version 6")
	}
	return out, nil
}

func snellCredentials(p *uriParts) (string, string, error) {
	userinfo := ""
	if p.user != nil {
		userinfo = p.user.Username()
	}
	queryPSK := queryFirst(p.query, "psk", "password")
	userKey := queryFirst(p.query, "userkey", "user-key")
	if queryPSK != "" {
		if userinfo != "" {
			if userKey != "" && userKey != userinfo {
				return "", "", fmt.Errorf("sblink: snell userkey specified twice with different values")
			}
			userKey = userinfo
		}
		return queryPSK, userKey, nil
	}
	if userinfo == "" {
		return "", userKey, fmt.Errorf("sblink: snell missing psk")
	}
	return userinfo, userKey, nil
}

func snellVersion(raw string) (int, error) {
	version, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		if strings.TrimSpace(raw) == "" {
			return 0, fmt.Errorf("sblink: snell missing version")
		}
		return 0, fmt.Errorf("sblink: invalid snell version %q", raw)
	}
	switch version {
	case 4, 6:
		return version, nil
	case 5:
		return 4, nil
	default:
		return 0, fmt.Errorf("sblink: unsupported snell version %d", version)
	}
}

func validateSnellNetwork(network string) error {
	for _, item := range strings.Split(network, ",") {
		item = strings.TrimSpace(item)
		if item != "tcp" && item != "udp" {
			return fmt.Errorf("sblink: unsupported snell network %q", network)
		}
	}
	return nil
}

func snellNetworkValue(network string) any {
	items := strings.Split(network, ",")
	if len(items) == 1 {
		return strings.TrimSpace(items[0])
	}
	values := make([]any, 0, len(items))
	for _, item := range items {
		values = append(values, strings.TrimSpace(item))
	}
	return values
}
