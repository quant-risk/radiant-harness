package context

import (
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf8"
)

// DefaultWarnThreshold is the fraction of the budget at which Compress
// starts emitting warnings. At 100% the content is hard-trimmed.
const DefaultWarnThreshold = 0.70

// CompressResult holds the outcome of a compression pass.
type CompressResult struct {
	Original   int     // original token estimate
	Compressed int     // post-compression token estimate
	Ratio      float64 // Compressed / Original
	Content    string  // compressed content
	Truncated  bool    // true if hard-truncated (couldn't fit in budget)
}

// Compress reduces content to fit within budgetTokens by:
//  1. Removing completed-phase blocks (marked with <!-- phase:done -->)
//  2. Summarizing long skill descriptions to first sentence only
//  3. Hard-trimming at section boundaries if still over budget
//
// If budgetTokens <= 0 the content is returned unchanged.
func Compress(content string, budgetTokens int) CompressResult {
	original := estimateTokens(content)

	// Pass 1: always strip completed-phase blocks (free optimization)
	compressed := stripCompletedPhases(content)

	if budgetTokens <= 0 || estimateTokens(compressed) <= budgetTokens {
		final := estimateTokens(compressed)
		ratio := 1.0
		if original > 0 {
			ratio = float64(final) / float64(original)
		}
		return CompressResult{
			Original:   original,
			Compressed: final,
			Ratio:      ratio,
			Content:    compressed,
		}
	}

	// Pass 2: trim skill descriptions to first sentence
	if estimateTokens(compressed) > budgetTokens {
		compressed = trimSkillDescriptions(compressed)
	}

	// Pass 3: drop the Loop Instructions footer if still needed
	if est := estimateTokens(compressed); est > budgetTokens {
		compressed = dropSection(compressed, "## Loop Instructions")
	}

	// Pass 4: hard-trim at last section boundary that fits
	truncated := false
	if est := estimateTokens(compressed); est > budgetTokens {
		compressed, truncated = hardTrim(compressed, budgetTokens)
	}

	final := estimateTokens(compressed)
	ratio := 1.0
	if original > 0 {
		ratio = float64(final) / float64(original)
	}

	return CompressResult{
		Original:   original,
		Compressed: final,
		Ratio:      ratio,
		Content:    compressed,
		Truncated:  truncated,
	}
}

// CompressFile reads a CONTEXT.md file, compresses it in-place, and
// writes the result back atomically.
func CompressFile(path string, budgetTokens int) (CompressResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CompressResult{}, fmt.Errorf("read %s: %w", path, err)
	}

	result := Compress(string(data), budgetTokens)
	if result.Original == result.Compressed {
		return result, nil // nothing to do
	}

	if err := atomicWrite(path, []byte(result.Content)); err != nil {
		return result, fmt.Errorf("write compressed: %w", err)
	}
	return result, nil
}

// EstimateFile returns the estimated token count of a file on disk.
func EstimateFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	return estimateTokens(string(data)), nil
}

// BudgetStatus describes where a token count falls relative to a budget.
type BudgetStatus int

const (
	BudgetOK       BudgetStatus = iota // below warn threshold
	BudgetWarning                      // above warn threshold, below limit
	BudgetExceeded                     // at or above limit
)

// CheckBudget returns the budget status for a given token count.
func CheckBudget(tokens, budgetTokens int, warnThreshold float64) BudgetStatus {
	if budgetTokens <= 0 {
		return BudgetOK
	}
	if warnThreshold <= 0 {
		warnThreshold = DefaultWarnThreshold
	}
	ratio := float64(tokens) / float64(budgetTokens)
	switch {
	case ratio >= 1.0:
		return BudgetExceeded
	case ratio >= warnThreshold:
		return BudgetWarning
	default:
		return BudgetOK
	}
}

// trimToTokenBudget is the inline version used by Assemble — no file I/O.
func trimToTokenBudget(content string, budget int) string {
	result := Compress(content, budget)
	return result.Content
}

// estimateTokens returns a conservative word-aware token estimate.
// Calibrated to match the harness.TokenEstimator within ±10%.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	total := 0.0
	for _, w := range words {
		rc := utf8.RuneCountInString(w)
		switch {
		case rc <= 3:
			total += 1.0
		case rc <= 8:
			total += 1.5
		default:
			if strings.ContainsAny(w, "_./") {
				total += math.Ceil(float64(rc) * 0.28)
			} else {
				total += math.Ceil(float64(rc) * 0.25)
			}
		}
	}
	// Markup penalty
	total += float64(strings.Count(text, "\n")) * 0.2
	return int(math.Ceil(total))
}

// stripCompletedPhases removes blocks between <!-- phase:done --> markers.
func stripCompletedPhases(content string) string {
	const open = "<!-- phase:done -->"
	const close = "<!-- /phase:done -->"
	for strings.Contains(content, open) {
		start := strings.Index(content, open)
		end := strings.Index(content, close)
		if end < 0 {
			break
		}
		content = content[:start] + content[end+len(close):]
	}
	return content
}

// trimSkillDescriptions reduces each skill section to its first line only.
func trimSkillDescriptions(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inSkill := false
	descLineCount := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			inSkill = true
			descLineCount = 0
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
			inSkill = false
			out = append(out, line)
			continue
		}
		if inSkill {
			if strings.HasPrefix(line, "**") || strings.HasPrefix(line, "`") || line == "" {
				out = append(out, line)
				continue
			}
			// Regular description line — keep only the first non-empty one
			if descLineCount == 0 && strings.TrimSpace(line) != "" {
				out = append(out, firstSentence(line, 100))
				descLineCount++
			} else if descLineCount > 0 {
				// Skip additional description lines
				continue
			} else {
				out = append(out, line)
			}
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// dropSection removes a markdown section (from its header to the next
// same-level header).
func dropSection(content, header string) string {
	start := strings.Index(content, header)
	if start < 0 {
		return content
	}
	// Find next ## or # at the same level
	after := content[start+len(header):]
	level := strings.TrimRight(strings.Split(header, " ")[0], " ")
	next := strings.Index(after, "\n"+level+" ")
	if next < 0 {
		return content[:start]
	}
	return content[:start] + after[next+1:]
}

// hardTrim cuts content at the last section boundary that fits within budget.
func hardTrim(content string, budget int) (string, bool) {
	lines := strings.Split(content, "\n")
	var kept []string
	for _, line := range lines {
		candidate := strings.Join(append(kept, line), "\n")
		if estimateTokens(candidate) > budget {
			break
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return content[:min(len(content), 200)], true
	}
	return strings.Join(kept, "\n"), true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
