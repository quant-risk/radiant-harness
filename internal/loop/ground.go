package loop

import (
	"fmt"
	"os/exec"
	"strings"
)

// GroundingBlock returns a markdown block of the N most recent commits on the
// current branch, formatted for injection into a loop iteration prompt.
//
// Purpose: "fresh context kills rot; grounding kills amnesia."
// Each iteration starts with a clean context window but reads this block so
// the agent knows what prior iterations already tried — preventing dead-end loops.
//
// repoDir is the project root. maxCommits is capped at 20 (0 → default 10).
// Returns ("", nil) if the repo has no commits or git is unavailable.
func GroundingBlock(repoDir string, maxCommits int) (string, error) {
	if maxCommits <= 0 {
		maxCommits = 10
	}
	if maxCommits > 20 {
		maxCommits = 20
	}

	cmd := exec.Command("git", "log",
		fmt.Sprintf("--max-count=%d", maxCommits),
		"--format=----%ncommit %h%ndate %ci%n%n%s%n%n%b",
		"--no-merges",
	)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		// git not available or no commits — not an error, just no grounding
		return "", nil
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return "", nil
	}

	// Truncate long commit bodies to avoid re-introducing context rot.
	entries := splitAndTruncate(raw, 400)
	if len(entries) == 0 {
		return "", nil
	}

	branch := currentBranch(repoDir)
	header := "## Recent work"
	if branch != "" {
		header = fmt.Sprintf("## Recent work on `%s`", branch)
	}

	return fmt.Sprintf("%s\n\n%s\n\n"+
		"Read this before acting — do not repeat approaches already tried.\n",
		header, strings.Join(entries, "\n\n")), nil
}

// currentBranch returns the current git branch name, or "" on error.
func currentBranch(repoDir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// splitAndTruncate splits the raw git log output on "----" separators and
// truncates each entry body to maxBodyChars characters.
func splitAndTruncate(raw string, maxBodyChars int) []string {
	parts := strings.Split(raw, "----")
	var entries []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) > maxBodyChars {
			p = p[:maxBodyChars] + "…"
		}
		entries = append(entries, p)
	}
	return entries
}
