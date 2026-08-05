package merge

import (
	"regexp"
	"strings"
	"unicode/utf16"
)

// jsLen returns the number of UTF-16 code units in s, matching JavaScript's
// String.prototype.length. Used for alias-length ranking so ordering matches
// merge.js exactly (ASCII is identical; CJK/emoji differ from byte length).
func jsLen(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// compiledAlias is a country alias compiled into a matcher, mirroring the
// objects pushed into COMPILED_COUNTRY_ALIASES in merge.js.
type compiledAlias struct {
	code              string
	name              string
	emoji             string
	text              string
	needle            string // substring pre-check (lowered or raw per target)
	regex             *regexp.Regexp
	target            string // "lower" or "raw"
	boundaryPriority  int
	aliasTypePriority int
	aliasLength       int
}

func isASCII(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r > 0x7f {
			return false
		}
	}
	return true
}

func isASCIIAlphaNumericByte(char byte) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9')
}

var reTwoAsciiLetters = regexp.MustCompile(`^[a-zA-Z]{2}$`)

// getAliasType classifies an alias as "emoji", "code" or "name" (merge.js).
func getAliasType(code, alias, emoji string) string {
	if alias == emoji {
		return "emoji"
	}
	if reTwoAsciiLetters.MatchString(alias) && strings.ToUpper(alias) == code {
		return "code"
	}
	return "name"
}

type aliasMatcher struct {
	regex            *regexp.Regexp
	target           string
	boundaryPriority int
}

// buildAliasMatcher ports buildAliasMatcher: Chinese/non-ASCII aliases match
// raw substrings; short/code aliases use non-alphanumeric boundaries; longer
// name aliases use word boundaries.
func buildAliasMatcher(alias, aliasType string) aliasMatcher {
	lowerAlias := strings.ToLower(alias)

	if !isASCII(lowerAlias) {
		return aliasMatcher{
			regex:            regexp.MustCompile(regexp.QuoteMeta(alias)),
			target:           "raw",
			boundaryPriority: 1,
		}
	}

	if aliasType == "code" || len(lowerAlias) < 4 {
		return aliasMatcher{
			regex:            regexp.MustCompile(`(?i)(^|[^a-z0-9])` + regexp.QuoteMeta(lowerAlias) + `($|[^a-z0-9])`),
			target:           "lower",
			boundaryPriority: 3,
		}
	}

	return aliasMatcher{
		regex:            regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(lowerAlias) + `\b`),
		target:           "lower",
		boundaryPriority: 2,
	}
}

// extractUpperCountryCode finds an uppercase two-letter catalog code with
// non-alphanumeric boundaries. It runs after full-name matching so a tag such
// as "Germany-US" keeps resolving to Germany, while "BO-01" resolves to
// Bolivia without making ordinary lowercase words such as "to" country codes.
func extractUpperCountryCode(tag string) *CountryInfo {
	for i := 0; i+1 < len(tag); i++ {
		first, second := tag[i], tag[i+1]
		if first < 'A' || first > 'Z' || second < 'A' || second > 'Z' {
			continue
		}
		if i > 0 && isASCIIAlphaNumericByte(tag[i-1]) {
			continue
		}
		if i+2 < len(tag) && isASCIIAlphaNumericByte(tag[i+2]) {
			continue
		}
		if info, ok := countryMap[tag[i:i+2]]; ok {
			country := info
			return &country
		}
		i++
	}
	return nil
}

// compareCountryAliasCandidate returns <0 if a should rank before b, matching
// the JS comparator: longer alias, then higher boundary/type priority, then
// longer text, then code order.
func compareCountryAliasCandidate(a, b compiledAlias) int {
	if a.aliasLength != b.aliasLength {
		return b.aliasLength - a.aliasLength
	}
	if a.boundaryPriority != b.boundaryPriority {
		return b.boundaryPriority - a.boundaryPriority
	}
	if a.aliasTypePriority != b.aliasTypePriority {
		return b.aliasTypePriority - a.aliasTypePriority
	}
	if jsLen(a.text) != jsLen(b.text) {
		return jsLen(b.text) - jsLen(a.text)
	}
	return strings.Compare(a.code, b.code)
}

func isBetterCountryAliasCandidate(candidate compiledAlias, best *compiledAlias) bool {
	return best == nil || compareCountryAliasCandidate(candidate, *best) < 0
}
