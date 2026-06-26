package context

import (
	"fmt"
	"strings"
)

// SummaryResult holds the output of a phase summary operation.
type SummaryResult struct {
	Phase      string
	Original   int // token estimate of input
	Summarized int // token estimate of output
	Ratio      float64
	Content    string
	KeyFacts   []string // extracted key facts preserved in the summary
}

// SummarizePhase compresses a completed phase's context to ≤20% of original tokens.
// It extracts critical information (decisions, outcomes, blockers) and discards
// intermediate reasoning that won't be needed again.
func SummarizePhase(phase, content string) SummaryResult {
	original := estimateTokens(content)
	targetTokens := max(original/5, 50) // ≤20% of original, minimum 50 tokens

	// Extract key facts before compressing
	facts := extractKeyFacts(content)

	// Build compact summary
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Phase summary: %s (completed)\n\n", phase))

	if len(facts) > 0 {
		sb.WriteString("**Key outcomes:**\n")
		for _, f := range facts {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}

	// Add condensed body (first paragraph + last paragraph strategy)
	body := condenseBody(content, targetTokens-estimateTokens(sb.String()))
	if body != "" {
		sb.WriteString(body)
	}

	result := sb.String()
	summarized := estimateTokens(result)

	return SummaryResult{
		Phase:      phase,
		Original:   original,
		Summarized: summarized,
		Ratio:      float64(summarized) / float64(max(original, 1)),
		Content:    result,
		KeyFacts:   facts,
	}
}

// SummarizeTrace compresses a list of trace events into a compact narrative.
// Used to compress loop trace context when budget is running low.
func SummarizeTrace(events []string) string {
	if len(events) == 0 {
		return "(no trace events)"
	}

	var sb strings.Builder
	sb.WriteString("**Loop trace summary:**\n")

	// Group by result
	var ok, failed []string
	for _, e := range events {
		if strings.Contains(e, "✓") || strings.Contains(e, `"result":"ok"`) {
			ok = append(ok, e)
		} else {
			failed = append(failed, e)
		}
	}

	sb.WriteString(fmt.Sprintf("- %d actions completed successfully\n", len(ok)))
	if len(failed) > 0 {
		sb.WriteString(fmt.Sprintf("- %d actions failed:\n", len(failed)))
		for _, f := range failed {
			// Extract action name from trace line
			action := extractAction(f)
			sb.WriteString(fmt.Sprintf("  - %s\n", action))
		}
	}

	return sb.String()
}

// extractKeyFacts pulls critical information from phase content.
// Looks for decisions, outcomes, blockers, and acceptance criteria.
func extractKeyFacts(content string) []string {
	var facts []string
	lines := strings.Split(content, "\n")

	keywords := []string{
		"decision:", "decided:", "✓", "✗", "APPROVED", "REJECTED",
		"blocker:", "blocked by:", "completed:", "done:", "outcome:",
		"result:", "error:", "fixed:", "implemented:",
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || len(line) < 10 {
			continue
		}
		lower := strings.ToLower(line)
		for _, kw := range keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				// Keep it short
				if len(line) > 120 {
					line = line[:117] + "..."
				}
				facts = append(facts, line)
				break
			}
		}
		if len(facts) >= 8 {
			break // cap at 8 key facts
		}
	}
	return facts
}

// condenseBody returns the most relevant portion of content within a token budget.
// Strategy: first 40% + last 20% of lines (intro context + conclusion).
func condenseBody(content string, budgetTokens int) string {
	if budgetTokens <= 0 {
		return ""
	}

	// Strip markdown headers and completed-phase blocks first
	content = stripCompletedPhases(content)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return ""
	}

	headCount := max(len(lines)*40/100, 3)
	tailCount := max(len(lines)*20/100, 2)

	var parts []string
	if headCount >= len(lines) {
		parts = lines
	} else {
		parts = append(lines[:headCount], "...(condensed)...")
		if tailStart := len(lines) - tailCount; tailStart > headCount {
			parts = append(parts, lines[tailStart:]...)
		}
	}

	result := strings.Join(parts, "\n")
	// Trim to budget
	if estimateTokens(result) > budgetTokens {
		cr := Compress(result, budgetTokens)
		return cr.Content
	}
	return result
}

func extractAction(traceLine string) string {
	// Try to extract action field from JSON or formatted trace line
	if idx := strings.Index(traceLine, `"action":"`); idx >= 0 {
		start := idx + len(`"action":"`)
		end := strings.Index(traceLine[start:], `"`)
		if end > 0 {
			return traceLine[start : start+end]
		}
	}
	// Fallback: return first 60 chars
	if len(traceLine) > 60 {
		return traceLine[:57] + "..."
	}
	return traceLine
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
