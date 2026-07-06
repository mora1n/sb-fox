package sblink

import (
	"fmt"
	"net/url"

	"github.com/mora1n/sb-fox/internal/merge"
)

func parseAnyTLS(uri string) (*merge.OrderedMap, error) {
	p, err := parseURILinkWithDefault(uri, 443)
	if err != nil {
		return nil, err
	}
	password := ""
	if p.user != nil {
		password = p.user.Username()
		if pw, ok := p.user.Password(); ok && pw != "" {
			password = pw
		}
	}
	if password == "" {
		return nil, fmt.Errorf("sblink: anytls missing password")
	}

	out := merge.NewOrderedMap()
	out.Set("type", "anytls")
	out.Set("tag", p.tag)
	out.Set("server", p.server)
	out.Set("server_port", p.portN)
	out.Set("password", password)
	setStringIfPresent(out, "idle_session_check_interval", queryFirst(p.query, "idle_session_check_interval", "idle-session-check-interval"))
	setStringIfPresent(out, "idle_session_timeout", queryFirst(p.query, "idle_session_timeout", "idle-session-timeout"))
	if err := setIntIfPresent(out, "min_idle_session", queryFirst(p.query, "min_idle_session", "min-idle-session")); err != nil {
		return nil, err
	}
	tls, err := tlsFromQuery(p.query, true, true)
	if err != nil {
		return nil, err
	}
	out.Set("tls", tls)
	return out, nil
}

func encodeAnyTLS(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	password, err := requiredString(out, "password")
	if err != nil {
		return "", err
	}
	q := url.Values{}
	for _, key := range []string{"idle_session_check_interval", "idle_session_timeout", "min_idle_session"} {
		if value, ok := out.Get(key); ok {
			q.Set(key, scalarString(value))
		}
	}
	if err := addTLSQuery(q, out, true, true); err != nil {
		return "", err
	}
	return linkURL("anytls", url.User(password), server, port, q, out.GetString("tag")), nil
}
