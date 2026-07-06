package sblink

import (
	"fmt"
	"net/url"

	"github.com/mora1n/sb-fox/internal/merge"
)

func parseShadowTLS(uri string) (*merge.OrderedMap, error) {
	p, err := parseURILinkWithDefault(uri, 443)
	if err != nil {
		return nil, err
	}
	password := ""
	if p.user != nil {
		password = p.user.Username()
	}
	version := atoiDefault(p.query.Get("version"), 0)
	if version > 1 && password == "" {
		return nil, fmt.Errorf("sblink: shadowtls version %d requires password", version)
	}

	out := merge.NewOrderedMap()
	out.Set("type", "shadowtls")
	out.Set("tag", p.tag)
	out.Set("server", p.server)
	out.Set("server_port", p.portN)
	if version > 0 {
		out.Set("version", intNumber(version))
	}
	if password != "" {
		out.Set("password", password)
	}
	tls, err := tlsFromQuery(p.query, true, false)
	if err != nil {
		return nil, err
	}
	out.Set("tls", tls)
	return out, nil
}

func encodeShadowTLS(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	if value, ok := out.Get("version"); ok {
		if version := scalarString(value); version != "" && version != "0" {
			q.Set("version", version)
		}
	}
	if err := addTLSQuery(q, out, false, true); err != nil {
		return "", err
	}
	password := out.GetString("password")
	var user *url.Userinfo
	if password != "" {
		user = url.User(password)
	}
	return linkURL("shadowtls", user, server, port, q, out.GetString("tag")), nil
}
