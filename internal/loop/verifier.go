package loop

import (
	"fmt"
	"strings"
)

// VerifyResult is the outcome of an adversarial verification pass.
type VerifyResult struct {
	Approved  bool     // true if the verifier approved the executor's work
	Score     float64  // 0.0–1.0 confidence of the verdict
	Issues    []string // specific issues found (empty if approved)
	Evidence  string   // summary evidence for the verdict
	TokensIn  int      // tokens used for verification prompt
	TokensOut int      // tokens used for verification response
	// Escalate signals that this result requires human review and the loop
	// should stop with ExitNeedsHuman instead of retrying. The verifier
	// sets this when the situation is genuinely ambiguous or risky — not
	// a simple implementation bug the executor can fix on retry.
	Escalate bool
}

// VerifierConfig controls adversarial verification behavior.
type VerifierConfig struct {
	// MinScore is the minimum score required for approval (default 0.7).
	MinScore float64
	// RequireEvidence forces the verifier to cite specific evidence.
	RequireEvidence bool
	// StrictMode makes the verifier default to "rejected" on ambiguity.
	StrictMode bool
}

// DefaultVerifierConfig returns sensible defaults.
func DefaultVerifierConfig() VerifierConfig {
	return VerifierConfig{
		MinScore:        0.70,
		RequireEvidence: true,
		StrictMode:      true,
	}
}

// BuildVerifierPrompt produces the adversarial verification prompt.
//
// Key design: the verifier is instructed to assume the work is broken
// until proven otherwise. This prevents the common failure mode where
// an LLM agent confirms its own output without genuine scrutiny.
func BuildVerifierPrompt(goal, executorOutput string, cfg VerifierConfig) string {
	strictClause := ""
	if cfg.StrictMode {
		strictClause = `
IMPORTANT: Default to REJECTED if you are uncertain. The burden of proof
is on the executor's output to demonstrate correctness, not on you to
find a reason to reject it.`
	}

	evidenceClause := ""
	if cfg.RequireEvidence {
		evidenceClause = `
You MUST cite specific evidence for your verdict:
- For APPROVED: quote the specific output line or test result that confirms success
- For REJECTED: quote the specific gap, error, or missing element`
	}

	return fmt.Sprintf(`You are an adversarial code reviewer. Your job is to verify whether
the following work actually achieves the stated goal.

GOAL:
%s

EXECUTOR OUTPUT:
%s

Your task:
1. Assume the work is BROKEN until you find concrete evidence otherwise
2. Check: does the output demonstrably achieve the goal?
3. Check: are there missing pieces, partial implementations, or incorrect assumptions?
4. Check: do tests exist and do they actually verify the goal (not just pass trivially)?
%s%s

ANTI-CHEAT CHECKS (verify all before approving):
- No test was deleted, commented out, or had its assertion weakened
- No function was left as a stub or placeholder
- Scope is unchanged from the original goal (no unrelated changes snuck in)
- No gate or threshold was widened just to make a check pass
If any of these are violated, you MUST set ESCALATE: true.

ESCALATE (set to true if the situation requires human review):
- The problem is genuinely ambiguous and retrying will not resolve it
- A security or data-integrity concern requires human judgment
- An anti-cheat violation was detected (see above)

Respond with EXACTLY this format:
VERDICT: [APPROVED|REJECTED]
SCORE: [0.0-1.0]
EVIDENCE: [one sentence citing specific proof]
ESCALATE: [true|false]
ISSUES:
- [issue 1, if any]
- [issue 2, if any]`,
		goal,
		truncateOutput(executorOutput, 4000),
		strictClause,
		evidenceClause,
	)
}

// ParseVerifyResponse parses the structured verifier response.
func ParseVerifyResponse(response string, cfg VerifierConfig) VerifyResult {
	result := VerifyResult{}
	lines := strings.Split(response, "\n")

	inIssues := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "VERDICT:"):
			verdict := strings.TrimSpace(strings.TrimPrefix(line, "VERDICT:"))
			result.Approved = strings.EqualFold(verdict, "approved")

		case strings.HasPrefix(line, "SCORE:"):
			scoreStr := strings.TrimSpace(strings.TrimPrefix(line, "SCORE:"))
			var score float64
			fmt.Sscanf(scoreStr, "%f", &score)
			result.Score = clampScore(score)

		case strings.HasPrefix(line, "EVIDENCE:"):
			result.Evidence = strings.TrimSpace(strings.TrimPrefix(line, "EVIDENCE:"))
			inIssues = false

		case strings.HasPrefix(line, "ESCALATE:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "ESCALATE:"))
			result.Escalate = strings.EqualFold(val, "true")
			inIssues = false

		case strings.HasPrefix(line, "ISSUES:"):
			inIssues = true

		case inIssues && strings.HasPrefix(line, "-"):
			issue := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if issue != "" {
				result.Issues = append(result.Issues, issue)
			}
		}
	}

	// Enforce minimum score — if score below threshold, force rejection
	if result.Approved && result.Score < cfg.MinScore {
		result.Approved = false
		result.Issues = append(result.Issues,
			fmt.Sprintf("score %.2f below minimum threshold %.2f", result.Score, cfg.MinScore))
	}

	// Strict mode: if no verdict was parsed, default to rejected
	if !result.Approved && result.Evidence == "" && cfg.StrictMode {
		result.Evidence = "no clear verdict found in response"
		result.Issues = append(result.Issues, "verifier response did not follow expected format")
	}

	return result
}

// ShouldRetry returns true if the result warrants retrying execute.
// We retry on rejection but NOT on critical parse failures.
func ShouldRetry(result VerifyResult) bool {
	return !result.Approved && len(result.Issues) > 0
}

// FormatVerifyResult renders a VerifyResult for display.
func FormatVerifyResult(result VerifyResult) string {
	var sb strings.Builder
	if result.Approved {
		sb.WriteString(fmt.Sprintf("✓ APPROVED (score: %.2f)\n", result.Score))
	} else {
		sb.WriteString(fmt.Sprintf("✗ REJECTED (score: %.2f)\n", result.Score))
	}
	if result.Evidence != "" {
		sb.WriteString(fmt.Sprintf("  Evidence: %s\n", result.Evidence))
	}
	for _, issue := range result.Issues {
		sb.WriteString(fmt.Sprintf("  Issue: %s\n", issue))
	}
	return sb.String()
}

// truncateOutput clips executor output to maxChars for the verification prompt,
// preserving the beginning and end (most relevant parts).
func truncateOutput(output string, maxChars int) string {
	if len(output) <= maxChars {
		return output
	}
	half := maxChars / 2
	return output[:half] + "\n\n[... truncated ...]\n\n" + output[len(output)-half:]
}

func clampScore(s float64) float64 {
	if s < 0 {
		return 0
	}
	if s > 1 {
		return 1
	}
	return s
}
