package sblink

import (
	"github.com/mora1n/sb-fox/internal/merge"
)

// buildTransport builds a sing-box transport block for the given network type
// and common link parameters. Returns nil when no transport is needed (tcp).
// Field order follows sing-box docs: type, then type-specific fields.
func buildTransport(network, path, host, serviceName string) *merge.OrderedMap {
	switch network {
	case "ws":
		t := merge.NewOrderedMap()
		t.Set("type", "ws")
		if path != "" {
			t.Set("path", path)
		}
		if host != "" {
			headers := merge.NewOrderedMap()
			headers.Set("Host", host)
			t.Set("headers", headers)
		}
		return t
	case "grpc":
		t := merge.NewOrderedMap()
		t.Set("type", "grpc")
		if serviceName != "" {
			t.Set("service_name", serviceName)
		}
		return t
	case "h2", "http":
		t := merge.NewOrderedMap()
		t.Set("type", "http")
		if host != "" {
			t.Set("host", []any{host})
		}
		if path != "" {
			t.Set("path", path)
		}
		return t
	case "httpupgrade":
		t := merge.NewOrderedMap()
		t.Set("type", "httpupgrade")
		if host != "" {
			t.Set("host", host)
		}
		if path != "" {
			t.Set("path", path)
		}
		return t
	default:
		// tcp / udp → no transport block
		return nil
	}
}
