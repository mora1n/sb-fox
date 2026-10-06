package api

import (
	"io"
	"net/http"
	"testing"

	"github.com/mora1n/sb-fox/internal/models"
)

func TestManualCustomCountryPersistsAndCanBeCleared(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)
	const raw = `{"type":"socks","tag":"🇯🇵 Japan node","server":"example.com","server_port":1080}`
	var node models.Node
	decodeData(t, c.do(http.MethodPost, "/api/nodes", map[string]string{
		"raw": raw, "country_code": "CUSTOM", "country_source": "manual",
	}), &node)
	path := "/api/nodes/" + itoa(node.ID)
	decodeData(t, c.do(http.MethodPost, "/api/nodes/refresh-country", map[string]any{"node_ids": []int64{node.ID}}), nil)
	decodeData(t, c.do(http.MethodGet, path, nil), &node)
	if node.CountryCode != "CUSTOM" || node.CountrySource != "manual" {
		t.Fatalf("custom selection lost after refresh: %+v", node)
	}
	var filtered []models.Node
	decodeData(t, c.do(http.MethodGet, "/api/nodes?country=CUSTOM", nil), &filtered)
	if len(filtered) != 1 || filtered[0].ID != node.ID {
		t.Fatalf("custom country filter = %+v", filtered)
	}
	decodeData(t, c.do(http.MethodPut, path, map[string]string{"raw": raw}), nil)
	decodeData(t, c.do(http.MethodGet, path, nil), &node)
	if node.CountryCode != "JP" || node.CountrySource != "auto" {
		t.Fatalf("automatic country not restored: %+v", node)
	}
	decodeData(t, c.do(http.MethodPut, path, map[string]string{
		"raw": raw, "country_code": "CUSTOM", "country_source": "manual",
	}), nil)
	decodeData(t, c.do(http.MethodGet, path, nil), &node)
	if node.CountryCode != "CUSTOM" || node.CountrySource != "manual" {
		t.Fatalf("editing existing node did not persist custom: %+v", node)
	}
}

func TestManualCustomCountryTemplateExportAndImport(t *testing.T) {
	_, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)
	var node models.Node
	decodeData(t, c.do(http.MethodPost, "/api/nodes", map[string]string{
		"raw":          `{"type":"socks","tag":"🇯🇵 Japan node","server":"example.com","server_port":1080}`,
		"country_code": "CUSTOM", "country_source": "manual",
	}), &node)
	resp := c.do(http.MethodPost, "/api/nodes/export/template", map[string]any{"node_ids": []int64{node.ID}})
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("template export status=%d err=%v", resp.StatusCode, err)
	}
	decodeData(t, c.do(http.MethodDelete, "/api/nodes/"+itoa(node.ID), nil), nil)
	decodeData(t, c.do(http.MethodPost, "/api/nodes/import/config", map[string]string{"config": string(data)}), nil)
	var imported []models.Node
	decodeData(t, c.do(http.MethodGet, "/api/nodes", nil), &imported)
	if len(imported) != 1 || imported[0].CountryCode != "CUSTOM" || imported[0].Server != "example.com" {
		t.Fatalf("custom category not preserved through template export/import: %+v", imported)
	}
}
