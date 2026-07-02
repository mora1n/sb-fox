package api

import (
	"strconv"

	"github.com/mora1n/sb-fox/internal/merge"
	"github.com/mora1n/sb-fox/internal/models"
)

// nodeFromOutbound builds a models.Node from a parsed sing-box outbound object,
// extracting the metadata columns. Country detection uses the same logic as the
// merge engine (tag-based), honoring a `server#CC` override with precedence.
func nodeFromOutbound(raw *merge.OrderedMap, source string, sourceRef *int64) (*models.Node, error) {
	compact, err := raw.MarshalJSON()
	if err != nil {
		return nil, err
	}

	n := &models.Node{
		Tag:           raw.GetString("tag"),
		Type:          raw.GetString("type"),
		Server:        raw.GetString("server"),
		Source:        source,
		SourceRef:     sourceRef,
		CountrySource: "auto",
		Raw:           string(compact),
	}

	if port, ok := raw.Get("server_port"); ok {
		n.ServerPort = numberToInt(port)
	}
	if detour := raw.GetString("detour"); detour != "" {
		n.HasDetour = true
		n.Detour = detour
	}

	// Country: `server#CC` override wins, else detect from tag.
	if ov := merge.ExtractServerCountryOverride(n.Server); ov != nil {
		n.Server = ov.Server
		n.CountryCode = ov.CountryCode
		n.CountrySource = "override"
		// persist the cleaned server back into the raw blob
		raw.Set("server", ov.Server)
		if cleaned, err := raw.MarshalJSON(); err == nil {
			n.Raw = string(cleaned)
		}
	} else if info := merge.DetectCountry(n.Tag); info != nil {
		n.CountryCode = info.Code
	}

	return n, nil
}

// numberToInt converts a JSON-decoded numeric value to int.
func numberToInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		// json.Number (string form)
		if s, ok := v.(interface{ String() string }); ok {
			if i, err := strconv.Atoi(s.String()); err == nil {
				return i
			}
		}
	}
	return 0
}
