package sblink

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

// uriParts holds the decomposed components of a userinfo-style share link.
type uriParts struct {
	user   *url.Userinfo
	server string // cleaned server (no #CC)
	port   string
	portN  json.Number
	query  url.Values
	tag    string
}

// parseURILink parses a "scheme://userinfo@host:port?query#fragment" URI and
// returns its components with server cleaned and port validated. The tag is
// derived from the fragment (URL-decoded) or synthesized from server:port.
func parseURILink(uri string) (*uriParts, error) {
	return parseURILinkWithDefault(uri, 0)
}

// parseURILinkWithDefault behaves like parseURILink, but fills defaultPort when
// the URI omits an explicit port. A zero defaultPort keeps the port required.
func parseURILinkWithDefault(uri string, defaultPort int) (*uriParts, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("sblink: bad URI %q: %w", uri, err)
	}
	host := cleanServer(u.Hostname())
	port := u.Port()
	if port == "" && defaultPort > 0 {
		port = strconv.Itoa(defaultPort)
	}
	pn, err := portNumber(port)
	if err != nil {
		return nil, err
	}
	return &uriParts{
		user:   u.User,
		server: host,
		port:   port,
		portN:  pn,
		query:  u.Query(),
		tag:    tagOrDefault(u.Fragment, host, port),
	}, nil
}

// parseVLESS maps a vless:// URI to a sing-box vless outbound.
func parseVLESS(uri string) (*merge.OrderedMap, error) {
	p, err := parseURILink(uri)
	if err != nil {
		return nil, err
	}
	uuid := ""
	if p.user != nil {
		uuid = p.user.Username()
	}

	out := merge.NewOrderedMap()
	out.Set("type", "vless")
	out.Set("tag", p.tag)
	out.Set("server", p.server)
	out.Set("server_port", p.portN)
	out.Set("uuid", uuid)
	if flow := p.query.Get("flow"); flow != "" {
		out.Set("flow", flow)
	}

	if tls := vlessTLS(p.query); tls != nil {
		out.Set("tls", tls)
	}
	if t := transportFromQuery(p.query); t != nil {
		out.Set("transport", t)
	}
	enc, err := vlessPacketEncoding(p.query)
	if err != nil {
		return nil, err
	}
	if enc != "" {
		out.Set("packet_encoding", enc)
	}
	return out, nil
}

func vlessPacketEncoding(q url.Values) (string, error) {
	enc := strings.ToLower(strings.TrimSpace(queryFirst(q, "packetEncoding", "packet_encoding", "packet-encoding")))
	switch enc {
	case "", "none":
		return "", nil
	case "packetaddr", "xudp":
		return enc, nil
	default:
		return "", fmt.Errorf("sblink: unsupported vless packetEncoding %q", enc)
	}
}

// vlessTLS builds the tls block from a vless query, handling both plain TLS and
// reality (security=reality with pbk/sid/fp).
func vlessTLS(q url.Values) *merge.OrderedMap {
	security := q.Get("security")
	switch security {
	case "tls":
		return buildTLS(tlsParams{
			enabled:     true,
			serverName:  queryFirst(q, "sni", "peer"),
			insecure:    boolQuery(q, "insecure", "allowInsecure", "allow-insecure", "skip-cert-verify"),
			alpn:        splitALPN(q.Get("alpn")),
			fingerprint: queryFirst(q, "fp", "fingerprint", "client-fingerprint"),
		})
	case "reality":
		return buildTLS(tlsParams{
			enabled:     true,
			serverName:  queryFirst(q, "sni", "peer"),
			alpn:        splitALPN(q.Get("alpn")),
			fingerprint: queryFirst(q, "fp", "fingerprint", "client-fingerprint"),
			realityPbk:  q.Get("pbk"),
			realitySid:  q.Get("sid"),
		})
	default:
		return nil
	}
}

// transportFromQuery builds a transport block from the standard vless/trojan
// query keys (type, path, host, serviceName).
func transportFromQuery(q url.Values) *merge.OrderedMap {
	network := q.Get("type")
	if network == "" || network == "tcp" {
		return nil
	}
	return buildTransport(network, q.Get("path"), q.Get("host"), q.Get("serviceName"))
}

// boolParam interprets common truthy query values ("1", "true").
func boolParam(v string) bool {
	return v == "1" || v == "true" || v == "True" || v == "TRUE"
}
