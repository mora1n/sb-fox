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
	"asia":     {"HK", "CN", "JP", "KR", "SG", "TW", "IN", "TH", "MY", "PH", "VN", "ID", "IL", "AE", "SA", "KW", "PK", "BD", "KZ", "UZ", "TR", "RU"},
	"americas": {"US", "CA", "BR", "MX", "AR", "CL", "CO", "PE", "VE"},
	"europe":   {"GB", "FR", "DE", "NL", "CH", "IT", "ES", "SE", "NO", "FI", "PL", "AT", "BE", "DK", "PT", "GR", "IE", "CZ", "RO", "UA", "LT", "LV", "EE", "BG", "HR", "SK", "SI", "HU"},
	"africa":   {"ZA", "EG", "NG"},
	"oceania":  {"AU", "NZ", "FJ"},
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
