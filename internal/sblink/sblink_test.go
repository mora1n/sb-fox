package sblink

import (
	"encoding/base64"
	"encoding/json"
	"os/exec"
	"os"
	"path/filepath"
	"testing"

	"github.com/mora1n/sb-fox/internal/merge"
)

// field returns a string field from an outbound, failing the test if absent.
func field(t *testing.T, m *merge.OrderedMap, key string) string {
	t.Helper()
	v, ok := m.Get(key)
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	s, _ := v.(string)
	return s
}

func nested(t *testing.T, m *merge.OrderedMap, key string) *merge.OrderedMap {
	t.Helper()
	v, ok := m.Get(key)
	if !ok {
		t.Fatalf("missing nested %q", key)
	}
	om, ok := v.(*merge.OrderedMap)
	if !ok {
		t.Fatalf("field %q is not an object", key)
	}
	return om
}

func TestParseShadowsocksLegacy(t *testing.T) {
	// legacy: ss://base64(method:password)@host:port#tag
	userinfo := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:secretpass"))
	uri := "ss://" + userinfo + "@1.2.3.4:8388#HK-Node"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "type") != "shadowsocks" {
		t.Errorf("type = %s", field(t, out, "type"))
	}
	if field(t, out, "method") != "aes-256-gcm" {
		t.Errorf("method = %s", field(t, out, "method"))
	}
	if field(t, out, "password") != "secretpass" {
		t.Errorf("password = %s", field(t, out, "password"))
	}
	if field(t, out, "server") != "1.2.3.4" {
		t.Errorf("server = %s", field(t, out, "server"))
	}
	if field(t, out, "tag") != "HK-Node" {
		t.Errorf("tag = %s", field(t, out, "tag"))
	}
}

func TestParseShadowsocksSIP002(t *testing.T) {
	// SIP002: userinfo is base64url of method:password, plugin in query
	userinfo := base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:pw123"))
	uri := "ss://" + userinfo + "@example.com:443?plugin=obfs-local%3Bobfs%3Dhttp#SG"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "method") != "chacha20-ietf-poly1305" {
		t.Errorf("method = %s", field(t, out, "method"))
	}
	if got := field(t, out, "server"); got != "example.com" {
		t.Errorf("server = %s", got)
	}
}

func TestParseVMessWSTLS(t *testing.T) {
	vmessJSON := map[string]any{
		"v": "2", "ps": "JP-WS", "add": "jp.example.com", "port": "443",
		"id": "b831381d-6324-4d53-ad4f-8cda48b30811", "aid": "0", "scy": "auto",
		"net": "ws", "type": "none", "host": "jp.example.com", "path": "/path", "tls": "tls",
	}
	raw, _ := json.Marshal(vmessJSON)
	uri := "vmess://" + base64.StdEncoding.EncodeToString(raw)
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "type") != "vmess" {
		t.Errorf("type = %s", field(t, out, "type"))
	}
	if field(t, out, "uuid") != "b831381d-6324-4d53-ad4f-8cda48b30811" {
		t.Errorf("uuid = %s", field(t, out, "uuid"))
	}
	if field(t, out, "server") != "jp.example.com" {
		t.Errorf("server = %s", field(t, out, "server"))
	}
	tls := nested(t, out, "tls")
	if field(t, tls, "server_name") != "jp.example.com" {
		t.Errorf("tls server_name = %s", field(t, tls, "server_name"))
	}
	tr := nested(t, out, "transport")
	if field(t, tr, "type") != "ws" || field(t, tr, "path") != "/path" {
		t.Errorf("transport = %+v", tr)
	}
}

func TestParseVLESSReality(t *testing.T) {
	uri := "vless://uuid-1234@vless.example.com:443?encryption=none&security=reality&sni=www.microsoft.com&fp=chrome&pbk=publickeyxyz&sid=abcd&flow=xtls-rprx-vision&type=tcp#Reality"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "flow") != "xtls-rprx-vision" {
		t.Errorf("flow = %s", field(t, out, "flow"))
	}
	tls := nested(t, out, "tls")
	if field(t, tls, "server_name") != "www.microsoft.com" {
		t.Errorf("sni = %s", field(t, tls, "server_name"))
	}
	reality := nested(t, tls, "reality")
	if field(t, reality, "public_key") != "publickeyxyz" {
		t.Errorf("pbk = %s", field(t, reality, "public_key"))
	}
}

func TestParseTrojanWS(t *testing.T) {
	uri := "trojan://mypassword@trojan.example.com:443?security=tls&sni=trojan.example.com&type=ws&path=%2Fws#Trojan"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "type") != "trojan" || field(t, out, "password") != "mypassword" {
		t.Errorf("trojan fields wrong: %+v", out)
	}
	tr := nested(t, out, "transport")
	if field(t, tr, "type") != "ws" || field(t, tr, "path") != "/ws" {
		t.Errorf("transport = %+v", tr)
	}
}

func TestParseHysteria2(t *testing.T) {
	uri := "hysteria2://pass123@hy2.example.com:8443?sni=hy2.example.com&obfs=salamander&obfs-password=obfspw#HY2"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "type") != "hysteria2" || field(t, out, "password") != "pass123" {
		t.Errorf("hy2 fields wrong: %+v", out)
	}
	obfs := nested(t, out, "obfs")
	if field(t, obfs, "type") != "salamander" || field(t, obfs, "password") != "obfspw" {
		t.Errorf("obfs = %+v", obfs)
	}
}

func TestParseTUIC(t *testing.T) {
	uri := "tuic://uuid-9:password9@tuic.example.com:443?congestion_control=bbr&sni=tuic.example.com#TUIC"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "uuid") != "uuid-9" || field(t, out, "password") != "password9" {
		t.Errorf("tuic fields wrong: %+v", out)
	}
	if field(t, out, "congestion_control") != "bbr" {
		t.Errorf("cc = %s", field(t, out, "congestion_control"))
	}
}

func TestParseManyBase64Blob(t *testing.T) {
	links := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw")) + "@1.1.1.1:8388#A\n" +
		"trojan://pw@2.2.2.2:443?security=tls#B"
	blob := base64.StdEncoding.EncodeToString([]byte(links))
	out, err := ParseMany(blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("parsed %d links, want 2", len(out))
	}
}

func TestServerHashCCStripped(t *testing.T) {
	// server carrying a #CC suffix (requirement h) is cleaned
	userinfo := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw"))
	uri := "ss://" + userinfo + "@relay.example.com%23CN:8388#Relay"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if got := field(t, out, "server"); got != "relay.example.com" {
		t.Errorf("server not cleaned of #CN: %q", got)
	}
}

// TestParsedOutboundsValidateWithKernel builds a full config from parsed
// outbounds and runs the real sing-box `check`, proving the emitted JSON is
// schema-valid (not just field-correct).
func TestParsedOutboundsValidateWithKernel(t *testing.T) {
	path, err := exec.LookPath("sing-box")
	if err != nil {
		t.Skip("sing-box not in PATH")
	}

	ssUserinfo := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw"))
	vmessJSON, _ := json.Marshal(map[string]any{
		"ps": "v", "add": "v.example.com", "port": "443", "id": "b831381d-6324-4d53-ad4f-8cda48b30811",
		"aid": "0", "net": "ws", "path": "/p", "host": "v.example.com", "tls": "tls",
	})
	uris := []string{
		"ss://" + ssUserinfo + "@1.2.3.4:8388#SS",
		"vmess://" + base64.StdEncoding.EncodeToString(vmessJSON),
		"vless://b831381d-6324-4d53-ad4f-8cda48b30811@vl.example.com:443?security=tls&sni=vl.example.com&type=ws&path=/w#VL",
		"trojan://pw@tr.example.com:443?security=tls&sni=tr.example.com#TR",
		"hysteria2://pw@hy.example.com:8443?sni=hy.example.com#HY",
		"tuic://b831381d-6324-4d53-ad4f-8cda48b30811:p@tu.example.com:443?sni=tu.example.com&congestion_control=bbr#TU",
	}

	outbounds := []any{}
	for _, u := range uris {
		m, err := Parse(u)
		if err != nil {
			t.Fatalf("parse %q: %v", u, err)
		}
		outbounds = append(outbounds, m)
	}
	// add a direct so route.final can resolve, plus the parsed nodes
	direct := merge.NewOrderedMap()
	direct.Set("type", "direct")
	direct.Set("tag", "direct")
	outbounds = append(outbounds, direct)

	cfg := merge.NewOrderedMap()
	logBlock := merge.NewOrderedMap()
	logBlock.Set("level", "error")
	cfg.Set("log", logBlock)
	cfg.Set("outbounds", outbounds)

	raw, err := cfg.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	pretty, _ := merge.Indent(raw)

	tmp := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmp, pretty, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, "check", "-c", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("sing-box check failed on parsed outbounds:\n%s\n--- config ---\n%s", out, pretty)
	}
}
