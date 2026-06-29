package harness

import (
	"math"
	"strings"
	"unicode/utf8"
)

// TokenEstimator provides improved token estimation.
// Uses multiple heuristics for better accuracy than simple char/4.
type TokenEstimator struct {
	CodeMultiplier   float64
	ProseMultiplier  float64
	MarkupMultiplier float64
}

// NewTokenEstimator creates an estimator with calibrated defaults.
func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		CodeMultiplier:   0.28,
		ProseMultiplier:  0.25,
		MarkupMultiplier: 0.30,
	}
}

// EstimateTokens estimates token count from text using improved heuristics.
func (te *TokenEstimator) EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	words := splitTokens(text)
	total := 0.0

	for _, w := range words {
		runeCount := utf8.RuneCountInString(w)

		if runeCount <= 3 {
			total += 1.0
			continue
		}

		if runeCount <= 8 {
			total += 1.5
			continue
		}

		if strings.Contains(w, "_") || strings.Contains(w, ".") {
			total += math.Ceil(float64(runeCount) * te.CodeMultiplier)
		} else {
			total += math.Ceil(float64(runeCount) * te.ProseMultiplier)
		}
	}

	specialCount := 0
	for _, r := range text {
		if r > 127 || (r >= 33 && r <= 47) || (r >= 58 && r <= 64) || (r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
			specialCount++
		}
	}
	total += float64(specialCount) * 0.3

	newlines := strings.Count(text, "\n")
	total += float64(newlines) * 0.2

	return int(math.Ceil(total))
}

// EstimateFromChars is the simple fallback (1 token = 4 chars).
func EstimateFromChars(text string) int {
	return int(math.Ceil(float64(len(text)) / 4.0))
}

func splitTokens(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if isWordChar(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			if !isWhitespace(r) {
				tokens = append(tokens, string(r))
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-'
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
