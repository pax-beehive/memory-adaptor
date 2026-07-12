package retrieval

import (
	"reflect"
	"strings"
	"unicode"
)

type analyzedTerm struct {
	canonical string
	patterns  []string
	expands   bool
}

type analysis []analyzedTerm

func analyze(text string) analysis {
	words := splitWords(text)
	result := make(analysis, 0, len(words))
	for _, word := range words {
		canonical := canonicalize(word)
		result = append(result, analyzedTerm{canonical: canonical, patterns: searchPatterns(canonical), expands: expandsWord(word)})
	}
	return result
}

func (a analysis) canonicalTerms() []string {
	terms := make([]string, 0, len(a))
	for _, term := range a {
		terms = append(terms, term.canonical)
	}
	return terms
}

func (a analysis) needsSupplement(text string) bool {
	if len(a) > 1 || !reflect.DeepEqual(a.canonicalTerms(), legacyTerms(text)) {
		return true
	}
	for _, r := range text {
		if unicode.In(r, unicode.Han) {
			return true
		}
	}
	for _, term := range a {
		if term.expands {
			return true
		}
	}
	return false
}

func legacyTerms(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return fields
}

func splitWords(text string) []string {
	runes := []rune(strings.TrimSpace(text))
	words := make([]string, 0, len(runes)/4+1)
	start := -1
	flush := func(end int) {
		if start >= 0 && start < end {
			words = append(words, strings.ToLower(string(runes[start:end])))
		}
		start = -1
	}
	for i, current := range runes {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			flush(i)
			continue
		}
		if start < 0 {
			start = i
			continue
		}
		previous := runes[i-1]
		nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		caseBoundary := unicode.IsLower(previous) && unicode.IsUpper(current)
		acronymBoundary := unicode.IsUpper(previous) && unicode.IsUpper(current) && nextIsLower
		if caseBoundary || acronymBoundary {
			flush(i)
			start = i
		}
	}
	flush(len(runes))
	return words
}

func canonicalize(word string) string {
	switch word {
	case "deployment", "deploy":
		return "deploy"
	case "configuration", "configure", "config":
		return "config"
	case "authentication", "authenticate":
		return "auth"
	case "retries", "retry":
		return "retry"
	case "repository", "repo":
		return "repo"
	default:
		return word
	}
}

func searchPatterns(canonical string) []string {
	switch canonical {
	case "auth":
		return []string{"authent"}
	case "retry":
		return []string{"retry", "retri"}
	default:
		return []string{canonical}
	}
}

func expandsWord(word string) bool {
	switch word {
	case "deploy", "configure", "config", "authenticate", "retry", "repo":
		return true
	default:
		return false
	}
}
