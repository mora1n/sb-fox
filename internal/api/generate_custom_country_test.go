package api

import (
	"io"
	"net/http"
	"testing"

	"github.com/mora1n/sb-fox/internal/kernel"
	"github.com/mora1n/sb-fox/internal/models"
)

func TestGenerateConfigCustomCountryGroupSelection(t *testing.T) {
	const template = `{"outbounds":[
		{"type":"selector","tag":"Proxy","outbounds":[]},
		{"type":"selector","tag":"Rule","outbounds":[]},
		{"type":"selector","tag":"Skip","outbounds":[]},
		{"type":"selector","tag":"Outside","outbounds":[]},
		{"type":"direct","tag":"Direct"}],"route":{"final":"Proxy"}}`
	unknown := testNode(1, "mystery-one", "")
	known := testNode(2, "plain-node", "JP")
	outside := testNode(3, "mystery-outside", "")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy": {unknown, known}, "Skip": {unknown}, "Outside": {outside},
	}, []*models.Node{unknown, known}, nil, models.ProfileOptions{
		AutoCountryGroups: true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy": {NodeIDs: []int64{1, 2}}, "Skip": {NodeIDs: []int64{1}, SkipCountryGroups: true}, "Outside": {NodeIDs: []int64{3}},
		},
		AutoCountrySelected: &models.NodeSelection{NodeIDs: []int64{1, 2}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	outbounds := generatedOutboundMap(t, config)
	for tag, want := range map[string][]string{
		"🏳️‍🌈Custom": {"mystery-one"}, "🇯🇵Japan": {"plain-node"},
		"Proxy": {"🇯🇵Japan", "🏳️‍🌈Custom"}, "Rule": {"🇯🇵Japan", "🏳️‍🌈Custom"},
		"Skip": {"mystery-one"}, "Outside": {"mystery-outside"},
	} {
		if got := stringSliceValue(t, outbounds[tag]["outbounds"]); !sameStrings(got, want) {
			t.Fatalf("%s outbounds = %v, want %v", tag, got, want)
		}
	}
}

func TestCustomCountryGroupPreviewAndSubscription(t *testing.T) {
	srv, ts := testServer(t)
	c := newClient(t, ts.URL)
	c.http.Jar = login(t, ts.URL)
	profileID, nodeID, profileName := createSavedPreviewProfileWithNode(t, c)
	profile, err := srv.Store.GetProfile(profileID)
	if err != nil {
		t.Fatal(err)
	}
	decodeData(t, c.do(http.MethodPut, "/api/profiles/"+itoa(profileID), map[string]any{
		"name": profileName, "template_id": profile.TemplateID,
		"options": models.ProfileOptions{
			AutoCountryGroups:   true,
			GroupSelections:     map[string]models.NodeSelection{"Proxy": {NodeIDs: []int64{nodeID}}},
			AutoCountrySelected: &models.NodeSelection{NodeIDs: []int64{nodeID}},
		},
	}), nil)
	var preview struct {
		Config string `json:"config"`
	}
	decodeData(t, c.do(http.MethodPost, "/api/generate/preview", map[string]any{"profile_id": profileID}), &preview)
	var token struct {
		Token string `json:"token"`
	}
	decodeData(t, c.do(http.MethodGet, "/api/auth/subscription-token", nil), &token)
	resp := newClient(t, ts.URL).do(http.MethodGet, "/sub/"+token.Token+"/"+profileName, nil)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("public subscription status=%d err=%v", resp.StatusCode, err)
	}
	for name, config := range map[string][]byte{"preview": []byte(preview.Config), "subscription": data} {
		t.Run(name, func(t *testing.T) {
			outbounds := generatedOutboundMap(t, config)
			for tag, want := range map[string][]string{"🏳️‍🌈Custom": {"preview-node"}, "Proxy": {"🏳️‍🌈Custom"}} {
				if got := stringSliceValue(t, outbounds[tag]["outbounds"]); !sameStrings(got, want) {
					t.Fatalf("%s outbounds = %v, want %v", tag, got, want)
				}
			}
			if srv.Kernel.Available() {
				if result := srv.Kernel.Check(config); result.Status != kernel.StatusOK {
					t.Fatalf("kernel rejected %s: %s", name, result.Messages)
				}
			}
		})
	}
}
