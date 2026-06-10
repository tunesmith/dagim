// SPDX-License-Identifier: GPL-3.0-or-later

package graph

import "strings"

const (
	maxSlugWords    = 8
	maxSlugLen      = 60
	maxSlugTokenLen = 24
)

var slugStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "before": true, "by": true, "for": true, "from": true,
	"in": true, "into": true, "is": true, "it": true, "its": true,
	"make": true, "of": true, "on": true, "onto": true, "or": true,
	"over": true, "the": true, "then": true, "to": true, "under": true,
	"with": true, "without": true,

	"about": true, "frequently": true,
}

var measurementWords = map[string]bool{
	"cup": true, "cups": true, "dash": true, "dashes": true,
	"g": true, "gram": true, "grams": true, "kg": true,
	"l": true, "liter": true, "liters": true, "litre": true, "litres": true,
	"lb": true, "lbs": true, "ml": true,
	"min": true, "mins": true, "minute": true, "minutes": true,
	"hour": true, "hours": true,
	"oz": true, "ounce": true, "ounces": true,
	"pinch": true, "pinches": true,
	"pound": true, "pounds": true,
	"qt": true, "qts": true, "quart": true, "quarts": true,
	"tablespoon": true, "tablespoons": true, "tbsp": true,
	"teaspoon": true, "teaspoons": true, "tsp": true,
}

var numberWords = map[string]bool{
	"zero": true, "one": true, "two": true, "three": true, "four": true,
	"five": true, "six": true, "seven": true, "eight": true, "nine": true,
	"ten": true, "eleven": true, "twelve": true,
	"half": true, "quarter": true,
}

func Slugify(text string) string {
	raw := slugTokens(text)
	if len(raw) == 0 {
		return ""
	}

	tokens := removeMeasurementTokens(raw)
	tokens = uniqueTokens(tokens)
	if len(tokens) == 0 {
		tokens = uniqueTokens(raw)
	}

	if len(tokens) > 4 || joinedTokenLen(tokens) > 40 {
		filtered := make([]string, 0, len(tokens))
		for _, token := range tokens {
			if !slugStopWords[token] {
				filtered = append(filtered, token)
			}
		}
		if len(filtered) > 0 {
			tokens = filtered
		}
	}

	return compactSlug(tokens)
}

func slugTokens(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	var tokens []string
	var b strings.Builder
	for _, r := range text {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			tokens = append(tokens, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens
}

func removeMeasurementTokens(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for i, token := range tokens {
		if measurementWords[token] {
			continue
		}
		if (isNumberToken(token) || numberWords[token]) && nearMeasurement(tokens, i) {
			continue
		}
		out = append(out, token)
	}
	return out
}

func nearMeasurement(tokens []string, index int) bool {
	for _, offset := range []int{-2, -1, 1, 2} {
		i := index + offset
		if i >= 0 && i < len(tokens) && measurementWords[tokens[i]] {
			return true
		}
	}
	return false
}

func isNumberToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func uniqueTokens(tokens []string) []string {
	seen := make(map[string]bool, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, token)
	}
	return out
}

func compactSlug(tokens []string) string {
	out := make([]string, 0, len(tokens))
	total := 0
	for _, token := range tokens {
		if len(token) > maxSlugTokenLen {
			token = token[:maxSlugTokenLen]
		}
		addLen := len(token)
		if len(out) > 0 {
			addLen++
		}
		if len(out) >= maxSlugWords {
			break
		}
		if len(out) >= 3 && total+addLen > maxSlugLen {
			break
		}
		if total+addLen > maxSlugLen {
			remaining := maxSlugLen - total
			if len(out) > 0 {
				remaining--
			}
			if remaining < 3 {
				break
			}
			token = token[:remaining]
			addLen = remaining
			if len(out) > 0 {
				addLen++
			}
		}
		out = append(out, token)
		total += addLen
	}
	if len(out) == 0 && len(tokens) > 0 {
		token := tokens[0]
		if len(token) > maxSlugLen {
			token = token[:maxSlugLen]
		}
		out = append(out, token)
	}
	return strings.Join(out, "-")
}

func joinedTokenLen(tokens []string) int {
	if len(tokens) == 0 {
		return 0
	}
	length := len(tokens) - 1
	for _, token := range tokens {
		length += len(token)
	}
	return length
}
