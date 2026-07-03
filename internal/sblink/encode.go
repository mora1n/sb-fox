package sblink

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/mora1n/sb-fox/internal/merge"
)

// Encode converts a sing-box proxy outbound back to a share-link URI.
func Encode(out *merge.OrderedMap) (string, error) {
	if out == nil {
		return "", fmt.Errorf("sblink: nil outbound")
	}
	switch out.GetString("type") {
	case "shadowsocks":
		return encodeShadowsocks(out)
	case "vmess":
		return encodeVMess(out)
	case "vless":
		return encodeVLESS(out)
	case "trojan":
		return encodeTrojan(out)
	case "hysteria2":
		return encodeHysteria2(out)
	case "tuic":
		return encodeTUIC(out)
	case "naive":
		return encodeNaive(out)
	default:
		return "", fmt.Errorf("sblink: unsupported outbound type %q", out.GetString("type"))
	}
}

func encodeShadowsocks(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	method, err := requiredString(out, "method")
	if err != nil {
		return "", err
	}
	password, err := requiredString(out, "password")
	if err != nil {
		return "", err
	}
	userinfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + password))
	q := url.Values{}
	if plugin := out.GetString("plugin"); plugin != "" {
		value := plugin
		if opts := out.GetString("plugin_opts"); opts != "" {
			value += ";" + opts
		}
		q.Set("plugin", value)
	}
	return linkURL("ss", url.User(userinfo), server, port, q, out.GetString("tag")), nil
}

func encodeVMess(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	uuid, err := requiredString(out, "uuid")
	if err != nil {
		return "", err
	}
	v := vmessLink{
		PS:   out.GetString("tag"),
		Add:  server,
		Port: port,
		ID:   uuid,
		Aid:  "0",
		Net:  "tcp",
		Type: "none",
	}
	if aid, ok := out.Get("alter_id"); ok {
		v.Aid = scalarString(aid)
	}
	if security := out.GetString("security"); security != "" {
		v.Scy = security
	}
	if tls, ok := orderedField(out, "tls"); ok && tlsEnabled(tls) {
		v.TLS = "tls"
		v.SNI = tls.GetString("server_name")
	}
	if transport, ok := orderedField(out, "transport"); ok {
		if err := applyVMessTransport(&v, transport); err != nil {
			return "", err
		}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("sblink: vmess JSON: %w", err)
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(raw), nil
}

func encodeVLESS(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	uuid, err := requiredString(out, "uuid")
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("encryption", "none")
	if flow := out.GetString("flow"); flow != "" {
		q.Set("flow", flow)
	}
	if enc := out.GetString("packet_encoding"); enc != "" {
		q.Set("packetEncoding", enc)
	}
	if err := addTLSQuery(q, out, true, true); err != nil {
		return "", err
	}
	if err := addTransportQuery(q, out); err != nil {
		return "", err
	}
	return linkURL("vless", url.User(uuid), server, port, q, out.GetString("tag")), nil
}

func encodeTrojan(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	password, err := requiredString(out, "password")
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("security", "tls")
	if err := addTLSQuery(q, out, false, true); err != nil {
		return "", err
	}
	if err := addTransportQuery(q, out); err != nil {
		return "", err
	}
	return linkURL("trojan", url.User(password), server, port, q, out.GetString("tag")), nil
}

func encodeHysteria2(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	password, err := requiredString(out, "password")
	if err != nil {
		return "", err
	}
	q := url.Values{}
	if obfs, ok := orderedField(out, "obfs"); ok {
		if typ := obfs.GetString("type"); typ != "" {
			q.Set("obfs", typ)
		}
		if password := obfs.GetString("password"); password != "" {
			q.Set("obfs-password", password)
		}
	}
	if err := addTLSQuery(q, out, false, false); err != nil {
		return "", err
	}
	return linkURL("hysteria2", url.User(password), server, port, q, out.GetString("tag")), nil
}

func encodeTUIC(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	uuid, err := requiredString(out, "uuid")
	if err != nil {
		return "", err
	}
	password, err := requiredString(out, "password")
	if err != nil {
		return "", err
	}
	q := url.Values{}
	if cc := out.GetString("congestion_control"); cc != "" {
		q.Set("congestion_control", cc)
	}
	if mode := out.GetString("udp_relay_mode"); mode != "" {
		q.Set("udp_relay_mode", mode)
	}
	if err := addTLSQuery(q, out, false, false); err != nil {
		return "", err
	}
	return linkURL("tuic", url.UserPassword(uuid, password), server, port, q, out.GetString("tag")), nil
}

func encodeNaive(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	if truthyField(out, "quic") {
		q.Set("quic", "true")
	}
	if cc := out.GetString("quic_congestion_control"); cc != "" {
		q.Set("quic_congestion_control", cc)
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
	return linkURL("naive+https", user, server, port, q, out.GetString("tag")), nil
}
