package sblink

import (
	"net/url"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

func parseHTTPProxy(uri string) (*merge.OrderedMap, error) {
	u, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return nil, err
	}
	server := cleanServer(u.Hostname())
	defaultPort := 80
	if u.Scheme == "https" {
		defaultPort = 443
	}
	port := u.Port()
	if port == "" {
		port = intNumber(defaultPort).String()
	}
	pn, err := portNumber(port)
	if err != nil {
		return nil, err
	}

	out := merge.NewOrderedMap()
	out.Set("type", "http")
	out.Set("tag", tagOrDefault(u.Fragment, server, port))
	out.Set("server", server)
	out.Set("server_port", pn)
	if u.User != nil {
		if username := u.User.Username(); username != "" {
			out.Set("username", username)
		}
		if password, ok := u.User.Password(); ok {
			out.Set("password", password)
		}
	}
	if u.Path != "" && u.Path != "/" {
		out.Set("path", u.Path)
	}
	q := u.Query()
	if headers, err := parseHeadersObject(q.Get("headers")); err != nil {
		return nil, err
	} else if headers != nil {
		out.Set("headers", headers)
	}
	tls, err := tlsFromQuery(q, u.Scheme == "https" || boolQuery(q, "tls") || queryFirst(q, "security") == "tls", false)
	if err != nil {
		return nil, err
	}
	if tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func encodeHTTPProxy(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	scheme := "http"
	if tls, ok := orderedField(out, "tls"); ok && tlsEnabled(tls) {
		scheme = "https"
	}
	q := url.Values{}
	if headers, ok := orderedField(out, "headers"); ok {
		raw, err := encodeHeadersObject(headers)
		if err != nil {
			return "", err
		}
		if raw != "" {
			q.Set("headers", raw)
		}
	}
	if err := addTLSQuery(q, out, false, false); err != nil {
		return "", err
	}
	var user *url.Userinfo
	username := out.GetString("username")
	password := out.GetString("password")
	if username != "" || password != "" {
		user = url.UserPassword(username, password)
	}
	return linkURLWithPath(scheme, user, server, port, out.GetString("path"), q, out.GetString("tag")), nil
}

func parseSOCKS(uri string) (*merge.OrderedMap, error) {
	u, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return nil, err
	}
	server := cleanServer(u.Hostname())
	port := u.Port()
	if port == "" {
		port = "1080"
	}
	pn, err := portNumber(port)
	if err != nil {
		return nil, err
	}
	version := "5"
	switch strings.ToLower(u.Scheme) {
	case "socks4":
		version = "4"
	case "socks4a":
		version = "4a"
	}

	out := merge.NewOrderedMap()
	out.Set("type", "socks")
	out.Set("tag", tagOrDefault(u.Fragment, server, port))
	out.Set("server", server)
	out.Set("server_port", pn)
	out.Set("version", version)
	if u.User != nil {
		if username := u.User.Username(); username != "" {
			out.Set("username", username)
		}
		if password, ok := u.User.Password(); ok {
			out.Set("password", password)
		}
	}
	q := u.Query()
	setStringIfPresent(out, "network", queryFirst(q, "network"))
	if value := queryFirst(q, "uot", "udp_over_tcp", "udp-over-tcp"); value != "" || boolQuery(q, "uot", "udp_over_tcp", "udp-over-tcp") {
		setUDPOverTCP(out, value)
	}
	return out, nil
}

func encodeSOCKS(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	version := out.GetString("version")
	scheme := "socks5"
	switch version {
	case "4":
		scheme = "socks4"
	case "4a":
		scheme = "socks4a"
	}
	q := url.Values{}
	if network := out.GetString("network"); network != "" {
		q.Set("network", network)
	}
	addUDPOverTCPQuery(q, out)
	var user *url.Userinfo
	username := out.GetString("username")
	password := out.GetString("password")
	if username != "" || password != "" {
		user = url.UserPassword(username, password)
	}
	return linkURL(scheme, user, server, port, q, out.GetString("tag")), nil
}

func setUDPOverTCP(out *merge.OrderedMap, raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" || boolParam(raw) {
		out.Set("udp_over_tcp", true)
		return
	}
	if version := atoiDefault(raw, 0); version > 0 {
		uot := merge.NewOrderedMap()
		uot.Set("enabled", true)
		uot.Set("version", intNumber(version))
		out.Set("udp_over_tcp", uot)
	}
}

func addUDPOverTCPQuery(q url.Values, out *merge.OrderedMap) {
	value, ok := out.Get("udp_over_tcp")
	if !ok {
		return
	}
	if truthy(value) {
		q.Set("uot", "1")
		return
	}
	uot, ok := value.(*merge.OrderedMap)
	if !ok || !truthyField(uot, "enabled") {
		return
	}
	version := ""
	if raw, ok := uot.Get("version"); ok {
		version = scalarString(raw)
	}
	if version == "" {
		version = "1"
	}
	q.Set("uot", version)
}
