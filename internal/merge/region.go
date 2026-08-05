package merge

import (
	"fmt"
	"strings"
)

// Region priority configuration ported from the original country grouping
// logic. The hot country list is now an explicit Generate option.

var defaultCountryHeatOrder = []string{"JP", "CN", "HK", "US", "TW", "SG"}
var europePriority = []string{"GB", "NL", "FR", "DE", "CH"}

var regionFallbackOrder = map[string]int{
	"europe":   3,
	"asia":     4,
	"americas": 5,
	"africa":   6,
	"oceania":  7,
}

var regionSets = map[string][]string{
	"asia":     strings.Fields("AE AF AM AZ BD BH BN BT CN CY GE HK ID IL IN IQ IR JO JP KG KH KP KR KW KZ LA LB LK MM MN MO MV MY NP OM PH PK PS QA SA SG SY TH TJ TL TM TR TW UZ VN YE"),
	"americas": strings.Fields("AG AI AR AW BB BL BM BO BQ BR BS BV BZ CA CL CO CR CU CW DM DO EC FK GD GF GL GP GS GT GY HN HT JM KN KY LC MF MQ MS MX NI PA PE PM PR PY SR SV SX TC TT US UY VC VE VG VI"),
	"europe":   strings.Fields("AD AL AT AX BA BE BG BY CH CQ CZ DE DK EE ES FI FO FR GB GG GI GR HR HU IE IM IS IT JE LI LT LU LV MC MD ME MK MT NL NO PL PT RO RS RU SE SI SJ SK SM UA VA XK"),
	"africa":   strings.Fields("AO BF BI BJ BW CD CF CG CI CM CV DJ DZ EA EG EH ER ET GA GH GM GN GQ GW IC IO KE KM LR LS LY MA MG ML MR MU MW MZ NA NE NG RE RW SC SD SH SL SN SO SS ST SZ TD TF TG TN TZ UG YT ZA ZM ZW"),
	"oceania":  strings.Fields("AC AQ AS AU CC CK CP CX DG FJ FM GU HM KI MH MP NC NF NR NU NZ PF PG PN PW SB TA TK TO TV UM VU WF WS"),
}

const (
	sortBucketHot            = 1
	sortBucketEuropePriority = 2
)

type sortInfo struct {
	bucket   int
	priority int
}

var regionMap = buildRegionMap()

func buildRegionMap() map[string]string {
	m := make(map[string]string)
	for region, codes := range regionSets {
		for _, code := range codes {
			m[code] = region
		}
	}
	return m
}

func inferRegion(code string) string {
	if r, ok := regionMap[code]; ok {
		return r
	}
	return "unknown"
}

// DefaultCountryHeatOrder returns the default hot-country order.
func DefaultCountryHeatOrder() []string {
	out := make([]string, len(defaultCountryHeatOrder))
	copy(out, defaultCountryHeatOrder)
	return out
}

// NormalizeCountryHeatOrder trims, uppercases, validates and de-duplicates a
// user-provided hot-country order.
func NormalizeCountryHeatOrder(codes []string) ([]string, error) {
	seen := make(map[string]struct{}, len(codes))
	out := make([]string, 0, len(codes))
	for _, raw := range codes {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if code == "" {
			continue
		}
		if _, ok := countryMap[code]; !ok {
			return nil, fmt.Errorf("unknown country code %q", raw)
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out, nil
}

// buildCountrySortCache ports buildCountrySortCache: hot countries first, then
// europe-priority, then region-fallback buckets. First assignment wins.
func buildCountrySortCache(hotCountryOrder []string) map[string]sortInfo {
	cache := make(map[string]sortInfo)
	if len(hotCountryOrder) == 0 {
		hotCountryOrder = defaultCountryHeatOrder
	}

	assign := func(codes []string, bucket int) {
		for i, code := range codes {
			if _, ok := cache[code]; !ok {
				cache[code] = sortInfo{bucket: bucket, priority: i + 1}
			}
		}
	}
	assign(hotCountryOrder, sortBucketHot)
	assign(europePriority, sortBucketEuropePriority)

	for code := range countryMap {
		if _, ok := cache[code]; !ok {
			bucket := regionFallbackOrder[inferRegion(code)]
			if bucket == 0 {
				bucket = 999
			}
			cache[code] = sortInfo{bucket: bucket, priority: 999}
		}
	}
	return cache
}

// getCountrySortInfo ports getCountrySortInfo with the same fallback behaviour.
func getCountrySortInfo(cache map[string]sortInfo, code string) sortInfo {
	if info, ok := cache[code]; ok {
		return info
	}
	bucket := regionFallbackOrder[inferRegion(code)]
	if bucket == 0 {
		bucket = 999
	}
	return sortInfo{bucket: bucket, priority: 999}
}
