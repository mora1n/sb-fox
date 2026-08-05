package merge

import "strings"

// isEmojiRune reports whether r has (approximately) the Unicode Emoji property,
// as used by merge.js's stripEmoji regex `[\p{Emoji}...]`. RE2 has no
// \p{Emoji} class, so we classify by range. This deliberately includes the
// ASCII members of \p{Emoji} (digits, '#', '*') to match JS behaviour exactly.
// Coverage spans the emoji blocks plus variation selectors, ZWJ and keycap.
func isEmojiRune(r rune) bool {
	switch {
	case r == '#' || r == '*' || (r >= '0' && r <= '9'):
		return true
	case r == 0x00A9 || r == 0x00AE:
		return true
	case r == 0x203C || r == 0x2049 || r == 0x2122 || r == 0x2139:
		return true
	case r >= 0x2194 && r <= 0x2199, r >= 0x21A9 && r <= 0x21AA:
		return true
	case r >= 0x231A && r <= 0x231B, r == 0x2328, r == 0x23CF:
		return true
	case r >= 0x23E9 && r <= 0x23F3, r >= 0x23F8 && r <= 0x23FA:
		return true
	case r == 0x24C2:
		return true
	case r >= 0x25AA && r <= 0x25AB, r == 0x25B6, r == 0x25C0, r >= 0x25FB && r <= 0x25FE:
		return true
	case r >= 0x2600 && r <= 0x27BF: // misc symbols + dingbats
		return true
	case r >= 0x2934 && r <= 0x2935:
		return true
	case r >= 0x2B00 && r <= 0x2BFF: // arrows, stars
		return true
	case r == 0x3030 || r == 0x303D || r == 0x3297 || r == 0x3299:
		return true
	case r == 0x200D || r == 0x20E3: // ZWJ, combining enclosing keycap
		return true
	case r >= 0xFE00 && r <= 0xFE0F: // variation selectors
		return true
	case r >= 0x1F000 && r <= 0x1FAFF: // emoji planes (incl. regional indicators)
		return true
	default:
		return false
	}
}

// stripEmoji ports stripEmoji: remove a leading run of emoji runes plus any
// following whitespace, then trim. Non-emoji-leading tags are only trimmed.
func stripEmoji(tag string) string {
	runes := []rune(tag)
	i := 0
	for i < len(runes) && isEmojiRune(runes[i]) {
		i++
	}
	if i == 0 {
		return strings.TrimSpace(tag)
	}
	// consume trailing whitespace of the matched \s* group
	for i < len(runes) && isSpace(runes[i]) {
		i++
	}
	return strings.TrimSpace(string(runes[i:]))
}

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x85, 0xA0:
		return true
	}
	return false
}

// matchTag ports matchTag: emoji-strip + lowercase equality.
func matchTag(tag, target string) bool {
	return strings.ToLower(stripEmoji(tag)) == strings.ToLower(stripEmoji(target))
}

// extractCache mirrors COUNTRY_EXTRACTION_CACHE.
var extractCache = map[string]*CountryInfo{}

// DetectCountry returns the country detected from a node tag, or nil if none.
// Exported wrapper over the internal detection used during merge, so import
// paths can classify nodes with identical semantics.
func DetectCountry(tag string) *CountryInfo {
	return extractCountry(tag)
}

// CountryByCode returns the country info for an ISO code (known or synthesized
// via flag-emoji math), or nil for an invalid code.
func CountryByCode(code string) *CountryInfo {
	return getCountryInfoByCode(code)
}

// extractCountry ports extractCountry: exact-alias, then emoji-scan, then
// ranked compiled-alias regex matching. Returns nil when no country matches.
func extractCountry(tag string) *CountryInfo {
	if tag == "" {
		return nil
	}
	if cached, ok := extractCache[tag]; ok {
		return cached
	}

	strippedTag := stripEmoji(tag)
	lowerTag := strings.ToLower(tag)
	lowerStrippedTag := strings.ToLower(strippedTag)
	var result *CountryInfo

	if strippedTag != "" {
		if info, ok := countryMatchIndexes.exactAliasMap[lowerStrippedTag]; ok {
			c := info
			result = &c
		}
	}

	if result == nil {
		for _, info := range countryMatchIndexes.emojiByCode {
			if strings.Contains(tag, info.Emoji) {
				c := info
				result = &c
				break
			}
		}
	}

	if result == nil {
		var best *compiledAlias
		for i := range countryMatchIndexes.compiledAliases {
			alias := countryMatchIndexes.compiledAliases[i]
			targetText := tag
			if alias.target == "lower" {
				targetText = lowerTag
			}
			if !strings.Contains(targetText, alias.needle) {
				continue
			}
			if alias.regex.MatchString(targetText) {
				if isBetterCountryAliasCandidate(alias, best) {
					a := alias
					best = &a
				}
			}
		}
		if best != nil {
			result = &CountryInfo{Code: best.code, Name: best.name, Emoji: best.emoji}
		}
	}

	if result == nil {
		result = extractUpperCountryCode(tag)
	}

	extractCache[tag] = result
	return result
}
