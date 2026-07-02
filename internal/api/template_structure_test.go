package api

import (
	"strings"
	"testing"
)

func TestExtractTemplateProxyOutbounds(t *testing.T) {
	content := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["node-a","Direct"],"default":"node-a"},
    {"type":"shadowsocks","tag":"node-a","server":"a.example.com","server_port":8388},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`

	processed, proxies, err := extractTemplateProxyOutbounds(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 || proxies[0].GetString("tag") != "node-a" {
		t.Fatalf("proxies = %+v", proxies)
	}
	if strings.Contains(processed, `"tag": "node-a"`) {
		t.Fatalf("processed template still contains proxy node:\n%s", processed)
	}
	if strings.Contains(processed, `"default"`) {
		t.Fatalf("processed template kept default pointing at removed node:\n%s", processed)
	}
	st, err := readTemplateStructure(processed)
	if err != nil {
		t.Fatal(err)
	}
	if st.Final != "Proxy" || len(st.Groups) != 1 {
		t.Fatalf("structure = %+v", st)
	}
	if len(st.Groups[0].Outbounds) != 1 || st.Groups[0].Outbounds[0] != "Direct" {
		t.Fatalf("outbounds = %+v", st.Groups[0].Outbounds)
	}
	if len(st.AvailableOutbounds) != 2 || st.AvailableOutbounds[0] != "Direct" || st.AvailableOutbounds[1] != "Proxy" {
		t.Fatalf("available outbounds = %+v", st.AvailableOutbounds)
	}
}

func TestWriteTemplateStructureValidation(t *testing.T) {
	content := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`

	_, err := writeTemplateStructure(content, templateStructure{
		Final:  "Proxy",
		Groups: []templateStructureGroup{},
	})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected missing final error, got %v", err)
	}

	_, err = writeTemplateStructure(content, templateStructure{
		Final: "Proxy",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Direct", "Direct"}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate outbound") {
		t.Fatalf("expected duplicate outbound error, got %v", err)
	}
}

func TestWriteTemplateStructureAddsChildSelector(t *testing.T) {
	content := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`

	updated, err := writeTemplateStructure(content, templateStructure{
		Final: "Proxy",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Child", "Direct"}, Default: "Child"},
			{Tag: "Child", Type: "selector", Outbounds: []string{"Direct"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := readTemplateStructure(updated)
	if err != nil {
		t.Fatal(err)
	}
	if st.Final != "Proxy" || len(st.Groups) != 2 {
		t.Fatalf("structure = %+v", st)
	}
	if st.Groups[0].Tag != "Proxy" || st.Groups[0].Default != "Child" {
		t.Fatalf("proxy group = %+v", st.Groups[0])
	}
	if st.Groups[1].Tag != "Child" {
		t.Fatalf("child group order = %+v", st.Groups)
	}
}
