package sblink

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

// vmessLink mirrors the v2rayN vmess:// base64-JSON schema. Numeric fields use
// json.RawMessage-friendly types via a custom decode below because port/aid may
// appear as either strings or numbers in the wild.
type vmessLink struct {
	PS                  string `json:"ps"`
	Add                 string `json:"add"`
	Port                any    `json:"port"`
	ID                  string `json:"id"`
	Aid                 any    `json:"aid"`
	Scy                 string `json:"scy"`
	Net                 string `json:"net"`
	Type                string `json:"type"`
	Host                string `json:"host"`
	Path                string `json:"path"`
	TLS                 string `json:"tls"`
	SNI                 string `json:"sni"`
	GlobalPadding       bool   `json:"global_padding"`
	AuthenticatedLength bool   `json:"authenticated_length"`
	PacketEncoding      string `json:"packet_encoding"`
	Network             string `json:"network"`
}

// parseVMess decodes the base64 JSON payload and maps it to a sing-box vmess
// outbound.
func parseVMess(uri string) (*merge.OrderedMap, error) {
	body := strings.TrimPrefix(uri, "vmess://")
	dec, ok := decodeBase64(body)
	if !ok {
		return nil, fmt.Errorf("sblink: vmess payload is not valid base64")
	}
	var v vmessLink
	if err := json.Unmarshal(dec, &v); err != nil {
		return nil, fmt.Errorf("sblink: vmess JSON: %w", err)
	}

	server := cleanServer(v.Add)
	port := anyToString(v.Port)
	pn, err := portNumber(port)
	if err != nil {
		return nil, err
	}

	out := merge.NewOrderedMap()
	out.Set("type", "vmess")
	out.Set("tag", tagOrDefault(v.PS, server, port))
	out.Set("server", server)
	out.Set("server_port", pn)
	out.Set("uuid", v.ID)
	out.Set("alter_id", intNumber(atoiDefault(anyToString(v.Aid), 0)))
	if v.Scy != "" {
		out.Set("security", v.Scy)
	}
	if v.GlobalPadding {
		out.Set("global_padding", true)
	}
	if v.AuthenticatedLength {
		out.Set("authenticated_length", true)
	}
	if v.PacketEncoding != "" && v.PacketEncoding != "none" {
		out.Set("packet_encoding", v.PacketEncoding)
	}
	if v.Network != "" {
		out.Set("network", v.Network)
	}
	// Note: vmess `net` (ws/grpc/h2/tcp) selects the sing-box TRANSPORT, not the
	// outbound `network` field (which only accepts tcp/udp/tcpudp). The transport
	// is emitted below; do not set `network` here.

	if strings.EqualFold(v.TLS, "tls") {
		sni := v.SNI
		if sni == "" {
			sni = v.Host
		}
		if tls := buildTLS(tlsParams{enabled: true, serverName: sni}); tls != nil {
			out.Set("tls", tls)
		}
	}
	if t := vmessTransport(v); t != nil {
		out.Set("transport", t)
	}
	return out, nil
}

// vmessTransport builds the transport block from vmess net/host/path fields.
func vmessTransport(v vmessLink) *merge.OrderedMap {
	switch v.Net {
	case "ws":
		return buildTransport("ws", v.Path, v.Host, "")
	case "grpc":
		// v2rayN carries the gRPC service name in `path`.
		return buildTransport("grpc", "", "", v.Path)
	case "h2":
		return buildTransport("h2", v.Path, v.Host, "")
	case "quic":
		return buildTransport("quic", "", "", "")
	default:
		return nil
	}
}
