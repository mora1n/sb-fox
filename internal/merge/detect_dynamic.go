package merge

import (
	"regexp"
	"sort"
	"strings"
)

// buildFlagEmoji ports buildFlagEmoji: two uppercase letters -> regional
// indicator symbols. Invalid input returns the white flag.
func buildFlagEmoji(code string) string {
	if !regexp.MustCompile(`^[A-Z]{2}$`).MatchString(code) {
		return "🏳️"
	}
	var b strings.Builder
	for _, char := range code {
		b.WriteRune(127397 + char)
	}
	return b.String()
}

var reTwoLetterCode = regexp.MustCompile(`^[A-Z]{2}$`)

// dynamicCountryCache mirrors DYNAMIC_COUNTRY_INFO_CACHE.
var dynamicCountryCache = map[string]*CountryInfo{}

// getCountryInfoByCode ports getCountryInfoByCode. Unknown-but-valid ISO codes
// get a synthesized entry (flag from code math, name = code). Note: merge.js
// uses Intl.DisplayNames for the name; Go has no stdlib equivalent, so we fall
// back to the code itself. This path is not exercised by the regression
// fixtures (which only use codes present in countryMap).
func getCountryInfoByCode(code string) *CountryInfo {
	if code == "" {
		return nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if !reTwoLetterCode.MatchString(normalized) {
		return nil
	}
	if info, ok := countryMap[normalized]; ok {
		c := info
		return &c
	}
	if cached, ok := dynamicCountryCache[normalized]; ok {
		return cached
	}
	info := &CountryInfo{
		Code:  normalized,
		Name:  normalized,
		Emoji: buildFlagEmoji(normalized),
	}
	dynamicCountryCache[normalized] = info
	return info
}

// matchIndexes holds the precompiled structures used by extractCountry.
type matchIndexes struct {
	emojiByCode     []CountryInfo // in countryOrder
	exactAliasMap   map[string]CountryInfo
	compiledAliases []compiledAlias
}

var countryMatchIndexes = buildCountryMatchIndexes()

// buildCountryMatchIndexes ports buildCountryMatchIndexes.
func buildCountryMatchIndexes() matchIndexes {
	idx := matchIndexes{
		exactAliasMap: make(map[string]CountryInfo),
	}

	for _, code := range countryOrder {
		info := countryMap[code]
		idx.emojiByCode = append(idx.emojiByCode, info)

		for _, alias := range info.Aliases {
			aliasType := getAliasType(code, alias, info.Emoji)
			if aliasType == "emoji" {
				continue
			}
			idx.exactAliasMap[strings.ToLower(alias)] = info

			matcher := buildAliasMatcher(alias, aliasType)
			needle := alias
			if matcher.target == "lower" {
				needle = strings.ToLower(alias)
			}
			aliasTypePriority := 1
			if aliasType == "name" {
				aliasTypePriority = 2
			}
			idx.compiledAliases = append(idx.compiledAliases, compiledAlias{
				code:              code,
				name:              info.Name,
				emoji:             info.Emoji,
				text:              alias,
				needle:            needle,
				regex:             matcher.regex,
				target:            matcher.target,
				boundaryPriority:  matcher.boundaryPriority,
				aliasTypePriority: aliasTypePriority,
				aliasLength:       jsLen(alias),
			})
		}
	}

	sort.SliceStable(idx.compiledAliases, func(i, j int) bool {
		return compareCountryAliasCandidate(idx.compiledAliases[i], idx.compiledAliases[j]) < 0
	})
	return idx
}
