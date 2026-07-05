package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSummaryListEndpointsOmitHeavyFields(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)

	raw := `{"type":"shadowsocks","tag":"n1","server":"example.com","server_port":443}`
	decodeData(t, c.do(http.MethodPost, "/api/nodes", map[string]string{"raw": raw}), nil)

	var nodesSummary []map[string]json.RawMessage
	decodeData(t, c.do(http.MethodGet, "/api/nodes?summary=1", nil), &nodesSummary)
	if len(nodesSummary) == 0 {
		t.Fatal("summary nodes is empty")
	}
	if _, ok := nodesSummary[0]["raw"]; ok {
		t.Fatalf("node summary includes raw: %+v", nodesSummary[0])
	}
	if _, ok := nodesSummary[0]["tag"]; !ok {
		t.Fatalf("node summary missing tag: %+v", nodesSummary[0])
	}

	var nodesFull []map[string]json.RawMessage
	decodeData(t, c.do(http.MethodGet, "/api/nodes", nil), &nodesFull)
	if len(nodesFull) == 0 {
		t.Fatal("full nodes is empty")
	}
	if _, ok := nodesFull[0]["raw"]; !ok {
		t.Fatalf("full node list missing raw: %+v", nodesFull[0])
	}

	var templatesSummary []map[string]json.RawMessage
	decodeData(t, c.do(http.MethodGet, "/api/templates?summary=1", nil), &templatesSummary)
	if len(templatesSummary) == 0 {
		t.Fatal("summary templates is empty")
	}
	if _, ok := templatesSummary[0]["content"]; ok {
		t.Fatalf("template summary includes content: %+v", templatesSummary[0])
	}
	if _, ok := templatesSummary[0]["name"]; !ok {
		t.Fatalf("template summary missing name: %+v", templatesSummary[0])
	}

	var templatesFull []map[string]json.RawMessage
	decodeData(t, c.do(http.MethodGet, "/api/templates", nil), &templatesFull)
	if len(templatesFull) == 0 {
		t.Fatal("full templates is empty")
	}
	if _, ok := templatesFull[0]["content"]; !ok {
		t.Fatalf("full template list missing content: %+v", templatesFull[0])
	}
}
