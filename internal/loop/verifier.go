package loop

import (
	"fmt"
	"strings"
)

// VerifyResult is the outcome of an adversarial verification pass.
type VerifyResult struct {
	Approved   bool              // true if the verifier approved the executor's work
	Score      float64           // geometric mean of Dimensions if set, else raw 0..1
	Dimensions []VerifyDimension // per-axis breakdown (optional; Sprint 45)
	Issues     []string          // specific issues found (empty if approved)
	Evidence   string            // summary evidence for the verdict
	TokensIn   int               // tokens used for verification prompt
	TokensOut  int               // tokens used for verification response
	// Escalate signals that this result requires human review and the loop
	// should stop with ExitNeedsHuman instead of retrying. The verifier
	// sets this when the situation is genuinely ambiguous or risky — not
	// a simple implementation bug the executor can fix on retry.
	Escalate bool
}

// ToolCallRecord mirrors engine.ToolCallRecord but is duplicated here
// to avoid a loop → engine import (loop is the lower layer). Sprint 70
// will consolidate by moving ToolCallRecord into a shared package
// (likely internal/toolrecord). For now, the verifier only reads
// the fields it needs: Name, Written, Bytes, Created, Err.
type ToolCallRecord struct {
	Name    string
	Written string
	Bytes   int
	Created bool
	Err     string
}

// VerifierConfig controls adversarial verification behavior.
type VerifierConfig struct {
	// MinScore is the minimum score required for approval (default 0.7).
	MinScore float64
	// RequireEvidence forces the verifier to cite specific evidence.
	RequireEvidence bool
	// StrictMode makes the verifier default to "rejected" on ambiguity.
	StrictMode bool
	// Quorum configures k-of-n parallel judging. Zero K disables quorum.
	Quorum QuorumConfig
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
//
// Sprint 69: toolTrace is the structured record of tool calls the
// executor dispatched in this iteration. When non-empty, the prompt
// gains a "TOOL CALLS OBSERVED" section so the verifier can audit
// each invocation (paths written, byte counts, errors). When empty
// (legacy code-block path), the prompt is unchanged from v2.37.0.
func BuildVerifierPrompt(goal, executorOutput string, cfg VerifierConfig, toolTrace []ToolCallRecord) string {
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

	toolClause := buildToolClause(toolTrace)

	return fmt.Sprintf(`You are an adversarial code reviewer. Your job is to verify whether
the following work actually achieves the stated goal.

GOAL:
%s

EXECUTOR OUTPUT:
%s
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
- (Tool-call iterations only) No tool call wrote outside the project boundary
- (Tool-call iterations only) A tool call erroring is NOT grounds for rejection if the executor correctly surfaced the error and adjusted
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
		toolClause,
		strictClause,
		evidenceClause,
	)
}

// buildToolClause renders the "TOOL CALLS OBSERVED" section. Returns
// the empty string when the trace is empty (so the prompt stays
// byte-identical to the v2.37.0 prompt for legacy callers).
func buildToolClause(toolTrace []ToolCallRecord) string {
	if len(toolTrace) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\nTOOL CALLS OBSERVED (in execution order):\n")
	for i, t := range toolTrace {
		sb.WriteString(fmt.Sprintf("%d. %s", i+1, t.Name))
		if t.Written != "" {
			sb.WriteString(fmt.Sprintf(" — %s", t.Written))
		}
		if t.Bytes > 0 {
			sb.WriteString(fmt.Sprintf(" (%d bytes", t.Bytes))
			if t.Created {
				sb.WriteString(", created")
			}
			sb.WriteString(")")
		}
		if t.Err != "" {
			sb.WriteString(fmt.Sprintf(" [ERROR: %s]", t.Err))
		}
		sb.WriteString("\n")
	}
	return sb.String()
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
