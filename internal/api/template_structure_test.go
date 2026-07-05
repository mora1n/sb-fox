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
	if err == nil || !strings.Contains(err.Error(), "cannot be deleted") {
		t.Fatalf("expected deleted route group error, got %v", err)
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

	_, err = writeTemplateStructure(content, templateStructure{
		Final: "Missing",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Direct"}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "final outbound") {
		t.Fatalf("expected unknown final error, got %v", err)
	}
}

func TestReadTemplateStructureIncludesRouteReachableURLTest(t *testing.T) {
	content := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Auto","Direct"]},
    {"type":"urltest","tag":"Auto","outbounds":[]},
    {"type":"urltest","tag":"Unused","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`

	st, err := readTemplateStructure(content)
	if err != nil {
		t.Fatal(err)
	}
	if st.Final != "Proxy" || len(st.Groups) != 2 {
		t.Fatalf("structure = %+v", st)
	}
	if st.Groups[0].Tag != "Proxy" || st.Groups[1].Tag != "Auto" {
		t.Fatalf("groups = %+v", st.Groups)
	}
	auto := st.Groups[1]
	if auto.Type != "urltest" || len(auto.Outbounds) != 0 {
		t.Fatalf("Auto group = %+v", auto)
	}
	if !containsString(auto.ReferencedBy, "Proxy") {
		t.Fatalf("Auto referenced_by = %+v", auto.ReferencedBy)
	}
	for _, g := range st.Groups {
		if g.Tag == "Unused" {
			t.Fatalf("unused group should not be route-reachable: %+v", st.Groups)
		}
	}
}

func TestWriteTemplateStructureAllowsReachableURLTestAndRejectsDeletingIt(t *testing.T) {
	content := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Auto","Direct"]},
    {"type":"urltest","tag":"Auto","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`

	updated, err := writeTemplateStructure(content, templateStructure{
		Final: "Proxy",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Auto", "Direct"}},
			{Tag: "Auto", Type: "urltest", Outbounds: []string{"Direct"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := readTemplateStructure(updated)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Groups) != 2 || st.Groups[1].Tag != "Auto" {
		t.Fatalf("structure = %+v", st)
	}
	if !sameStrings(st.Groups[1].Outbounds, []string{"Direct"}) {
		t.Fatalf("Auto outbounds = %+v", st.Groups[1].Outbounds)
	}

	_, err = writeTemplateStructure(content, templateStructure{
		Final: "Proxy",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Auto", "Direct"}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be deleted") {
		t.Fatalf("expected deleting reachable urltest rejection, got %v", err)
	}
}

func TestWriteTemplateStructureAllowsStaticAndEmptyFinal(t *testing.T) {
	content := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Proxy"}],
    "final":"Proxy"
  }
}`

	updated, err := writeTemplateStructure(content, templateStructure{
		Final: "Direct",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Direct"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := readTemplateStructure(updated)
	if err != nil {
		t.Fatal(err)
	}
	if st.Final != "Direct" || len(st.Groups) != 1 || st.Groups[0].Tag != "Proxy" {
		t.Fatalf("structure = %+v", st)
	}

	updated, err = writeTemplateStructure(content, templateStructure{
		Final: "",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Direct"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated, `"final"`) {
		t.Fatalf("empty final should delete route.final:\n%s", updated)
	}
	st, err = readTemplateStructure(updated)
	if err != nil {
		t.Fatal(err)
	}
	if st.Final != "" || len(st.Groups) != 1 || st.Groups[0].Tag != "Proxy" {
		t.Fatalf("structure = %+v", st)
	}
}

func TestWriteTemplateStructureRejectsGroupCycles(t *testing.T) {
	content := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Fallback","outbounds":["Direct"]},
    {"type":"selector","tag":"Child","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [
      {"domain":["fallback.example"],"outbound":"Fallback"},
      {"domain":["child.example"],"outbound":"Child"}
    ],
    "final":"Proxy"
  }
}`

	for name, groups := range map[string][]templateStructureGroup{
		"direct": {
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Fallback"}},
			{Tag: "Fallback", Type: "selector", Outbounds: []string{"Proxy"}},
			{Tag: "Child", Type: "selector", Outbounds: []string{"Direct"}},
		},
		"indirect": {
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Fallback"}},
			{Tag: "Fallback", Type: "selector", Outbounds: []string{"Child"}},
			{Tag: "Child", Type: "selector", Outbounds: []string{"Proxy"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := writeTemplateStructure(content, templateStructure{
				Final:  "Proxy",
				Groups: groups,
			})
			if err == nil || !strings.Contains(err.Error(), "group reference cycle") {
				t.Fatalf("expected cycle error, got %v", err)
			}
		})
	}
}

func TestWriteTemplateStructureRejectsGroupAddDeleteAndPreservesReferences(t *testing.T) {
	deleteContent := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"RuleGroup","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"RuleGroup"}],
    "final":"Proxy"
  }
}`
	_, err := writeTemplateStructure(deleteContent, templateStructure{
		Final: "Proxy",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Direct"}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be deleted") {
		t.Fatalf("expected delete rejection, got %v", err)
	}

	content := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Child","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`

	_, err = writeTemplateStructure(content, templateStructure{
		Final: "Proxy",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Direct"}},
			{Tag: "New", Type: "selector", Outbounds: []string{"Direct"}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("expected add rejection, got %v", err)
	}

	updated, err := writeTemplateStructure(content, templateStructure{
		Final: "Proxy",
		Groups: []templateStructureGroup{
			{Tag: "Proxy", Type: "selector", Outbounds: []string{"Child", "Direct"}, Default: "Child"},
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
		t.Fatalf("child group should become route-reachable: %+v", st.Groups)
	}
	if !containsString(st.AvailableOutbounds, "Child") {
		t.Fatalf("child selector should remain available: %+v", st.AvailableOutbounds)
	}
	if !strings.Contains(updated, `"tag": "Child"`) {
		t.Fatalf("unmanaged child selector was not preserved:\n%s", updated)
	}
}
