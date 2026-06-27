package loop

import (
	"fmt"
	"math"
	"strings"
)

// ReviewPanel is the post-convergence verification layer.
//
// The loop runs two verification layers:
//  1. The per-iteration verifier (verifier.go) — checks each executor output.
//  2. ReviewPanel — runs ONLY after the verifier passes (convergence). A fail
//     re-opens the loop body with findings threaded to the next iteration.
//
// This is the "review slot" pattern from jonny981/loops:loop.ts:config.review().
// MaxRestarts caps the worker↔reviewer standoff independently of MaxIter.
type ReviewPanel struct {
	MaxRestarts int // default 3; 0 → uses default
}

// ReviewResult is the outcome of the post-convergence review slot.
type ReviewResult struct {
	Pass     bool
	Score    float64  // 0..1
	Findings []string // fed to next iteration as context if !Pass
	Evidence string
}

// DefaultReviewPanel returns a ReviewPanel with default settings.
func DefaultReviewPanel() ReviewPanel {
	return ReviewPanel{MaxRestarts: 3}
}

// maxRestarts returns the effective restart cap.
func (rp ReviewPanel) maxRestarts() int {
	if rp.MaxRestarts <= 0 {
		return 3
	}
	return rp.MaxRestarts
}

// BuildReviewPrompt produces a prompt for the post-convergence reviewer.
// lastFindings are the findings from the previous review round (empty on first run).
func BuildReviewPrompt(goal, executorOutput string, lastFindings []string) string {
	priorContext := ""
	if len(lastFindings) > 0 {
		priorContext = fmt.Sprintf(`
PRIOR REVIEW FINDINGS (the executor was asked to fix these):
%s

Check whether each prior finding was addressed. If any remain unresolved, REJECT.
`, "- "+strings.Join(lastFindings, "\n- "))
	}

	return fmt.Sprintf(`You are a senior reviewer performing a final quality gate.
The loop has converged — the per-iteration verifier approved the output.
Your job is a DEEPER review: correctness, completeness, and no regressions.

GOAL:
%s

FINAL OUTPUT:
%s
%s
Score each dimension 0..1. The final score is the GEOMETRIC MEAN of all dimensions.
Any dimension at 0 drives the total to 0.

DIMENSIONS:
- correctness: does the implementation actually satisfy the goal?
- completeness: are there missing pieces, stubs, or TODOs left behind?
- test_quality: do the tests meaningfully verify behavior (not just pass trivially)?
- regression_risk: could this change break existing functionality?

Respond with EXACTLY this format:
REVIEW: [PASS|FAIL]
SCORE: [0.0-1.0]
EVIDENCE: [one sentence]
FINDINGS:
- [finding 1, if any]
- [finding 2, if any]`,
		goal,
		truncateOutput(executorOutput, 3000),
		priorContext,
	)
}

// ParseReviewResponse parses the structured reviewer response.
func ParseReviewResponse(response string) ReviewResult {
	result := ReviewResult{}
	lines := strings.Split(response, "\n")
	inFindings := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "REVIEW:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "REVIEW:"))
			result.Pass = strings.EqualFold(val, "pass")
		case strings.HasPrefix(line, "SCORE:"):
			var s float64
			fmt.Sscanf(strings.TrimPrefix(line, "SCORE:"), "%f", &s)
			result.Score = clampScore(s)
		case strings.HasPrefix(line, "EVIDENCE:"):
			result.Evidence = strings.TrimSpace(strings.TrimPrefix(line, "EVIDENCE:"))
			inFindings = false
		case strings.HasPrefix(line, "FINDINGS:"):
			inFindings = true
		case inFindings && strings.HasPrefix(line, "-"):
			f := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if f != "" {
				result.Findings = append(result.Findings, f)
			}
		}
	}
	return result
}

// ---- Quorum verifier ----

// QuorumConfig controls the k-of-n parallel judge pool.
type QuorumConfig struct {
	// K is the minimum number of judges that must pass (APPROVED).
	// 0 disables quorum (falls back to single verifier).
	K int
	// N is the total number of parallel judge calls. Default = K+1 if 0.
	N int
}

// QuorumResult is the aggregated outcome of N parallel judge calls.
type QuorumResult struct {
	Passed     int     // number of judges that approved
	Total      int     // number of judges that returned (excluding panics)
	Met        bool    // Passed >= K
	Confidence float64 // mean score of passing judges
	Reason     string
}

// RunQuorum runs N verifier calls concurrently (simulated via goroutines) and
// requires K to pass. A judge that panics counts as a "no" vote.
//
// buildPrompt is called for each judge index i (0..N-1), allowing different
// prompts per judge (e.g., different lenses or temperatures).
// parseResult maps the raw response string to a VerifyResult.
func RunQuorum(cfg QuorumConfig, judges []VerifyResult) QuorumResult {
	k := cfg.K
	n := cfg.N
	if n <= 0 {
		n = k + 1
	}

	type vote struct {
		approved bool
		score    float64
	}

	votes := make([]vote, len(judges))
	for i, j := range judges {
		votes[i] = vote{approved: j.Approved, score: j.Score}
	}

	var passing []float64
	for _, v := range votes {
		if v.approved {
			passing = append(passing, v.score)
		}
	}

	met := len(passing) >= k
	confidence := 0.0
	if len(passing) > 0 {
		sum := 0.0
		for _, s := range passing {
			sum += s
		}
		confidence = sum / float64(len(passing))
	}

	return QuorumResult{
		Passed:     len(passing),
		Total:      len(votes),
		Met:        met,
		Confidence: confidence,
		Reason:     fmt.Sprintf("quorum %d/%d held (need %d)", len(passing), len(votes), k),
	}
}

// ---- Geometric mean per dimension ----

// VerifyDimension is a named scoring axis (0..1).
type VerifyDimension struct {
	Name  string
	Score float64 // 0..1; a zero drives the geometric mean to 0
}

// GeometricMean returns the geometric mean of dimension scores.
// Any zero or negative value drives the result to 0 (no hiding a bad dimension).
func GeometricMean(dims []VerifyDimension) float64 {
	if len(dims) == 0 {
		return 0
	}
	scores := make([]float64, len(dims))
	for i, d := range dims {
		scores[i] = d.Score
	}
	return geometricMeanOf(scores)
}

func geometricMeanOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	for _, v := range values {
		if v <= 0 {
			return 0
		}
	}
	sum := 0.0
	for _, v := range values {
		sum += math.Log(v)
	}
	return math.Exp(sum / float64(len(values)))
}
