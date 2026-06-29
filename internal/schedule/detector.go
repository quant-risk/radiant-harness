package schedule

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DetectSignals gathers work signals from a repository:
//   - new-commits: commits on HEAD since state.LastCommit
//   - pending-work: count of TODO/FIXME markers in tracked source files
//   - interval: always emitted (the elapsed-time trigger)
//
// failing-gate is not auto-detected here (it requires running gates); callers
// that know a gate is red can append a Signal{Kind: TriggerFailingGate}.
func DetectSignals(repoDir string, state State) []Signal {
	var signals []Signal

	// interval is always available; Evaluate gates it by policy + rate limit.
	signals = append(signals, Signal{Kind: TriggerInterval, Detail: "time elapsed", Value: 1})

	// new commits since last recorded commit
	if head, err := gitHead(repoDir); err == nil {
		if state.LastCommit != "" && head != state.LastCommit {
			n := gitCommitsSince(repoDir, state.LastCommit)
			if n > 0 {
				signals = append(signals, Signal{
					Kind:   TriggerNewCommits,
					Detail: fmt.Sprintf("%s..%s", short(state.LastCommit), short(head)),
					Value:  n,
				})
			}
		}
	}

	// pending work: TODO/FIXME markers
	if n := countTodos(repoDir); n > 0 {
		signals = append(signals, Signal{
			Kind:   TriggerPendingWork,
			Detail: "TODO/FIXME markers",
			Value:  n,
		})
	}

	return signals
}

// CurrentCommit returns HEAD for recording into state after a run.
func CurrentCommit(repoDir string) string {
	h, _ := gitHead(repoDir)
	return h
}

func gitHead(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func gitCommitsSince(repoDir, sinceRef string) int {
	cmd := exec.Command("git", "rev-list", "--count", sinceRef+"..HEAD")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

// countTodos scans tracked files (via git ls-files) for TODO/FIXME markers.
// Falls back to 0 if git is unavailable.
func countTodos(repoDir string) int {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}
	count := 0
	for _, f := range strings.Split(string(out), "\n") {
		if f == "" {
			continue
		}
		count += todosInFile(filepath.Join(repoDir, f))
	}
	return count
}

func todosInFile(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "TODO") || strings.Contains(line, "FIXME") {
			n++
		}
	}
	return n
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// ── Persistence ─────────────────────────────────────────────────────────────

// statePath is where scheduler state lives within a project.
func statePath(projectDir string) string {
	return filepath.Join(projectDir, ".radiant-harness", "schedule.json")
}

// LoadState reads scheduler state, returning a zero State if none exists.
func LoadState(projectDir string) (State, error) {
	data, err := os.ReadFile(statePath(projectDir))
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parse schedule state: %w", err)
	}
	return s, nil
}

// SaveState writes scheduler state atomically (temp + rename).
func SaveState(projectDir string, s State) error {
	dir := filepath.Join(projectDir, ".radiant-harness")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".schedule-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	tmp.Close()
	return os.Rename(name, statePath(projectDir))
}

// FormatDecision renders a decision for the CLI.
func FormatDecision(d Decision, now time.Time) string {
	var sb strings.Builder
	if d.ShouldRun {
		sb.WriteString("● RUN — " + d.Reason + "\n")
	} else {
		sb.WriteString("○ SKIP — " + d.Reason + "\n")
	}
	if len(d.Signals) > 0 {
		sb.WriteString("  signals:\n")
		for _, s := range d.Signals {
			fmt.Fprintf(&sb, "    - %-13s %s (%d)\n", s.Kind, s.Detail, s.Value)
		}
	}
	return sb.String()
}
