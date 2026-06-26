package fleet

import (
	"fmt"
	"strings"
	"time"
)

// Conflict describes two agents that have edited the same file.
type Conflict struct {
	File       string
	TaskA      string // task ID from first agent
	TaskB      string // task ID from second agent
	AgentA     string
	AgentB     string
	DetectedAt time.Time
}

// Resolution describes how a conflict was resolved.
type Resolution struct {
	Conflict
	Winner     string // agent ID whose version was kept
	Reason     string
	ResolvedAt time.Time
}

// DetectConflicts finds tasks that touch the same files.
// Called after all Implementer agents complete, before merging.
func DetectConflicts(tasks []Task) []Conflict {
	// Map file → list of (taskID, agentID) that claimed it
	fileOwners := map[string][]struct{ taskID, agentID string }{}

	for _, t := range tasks {
		if t.Status != TaskDone {
			continue
		}
		for _, f := range t.Files {
			fileOwners[f] = append(fileOwners[f], struct{ taskID, agentID string }{t.ID, t.AgentID})
		}
	}

	var conflicts []Conflict
	for file, owners := range fileOwners {
		if len(owners) < 2 {
			continue
		}
		// Report all pairs
		for i := 0; i < len(owners); i++ {
			for j := i + 1; j < len(owners); j++ {
				conflicts = append(conflicts, Conflict{
					File:       file,
					TaskA:      owners[i].taskID,
					TaskB:      owners[j].taskID,
					AgentA:     owners[i].agentID,
					AgentB:     owners[j].agentID,
					DetectedAt: time.Now().UTC(),
				})
			}
		}
	}
	return conflicts
}

// ResolveConflict picks which agent's version to keep.
// Strategy: prefer the task with more evidence (longer evidence string = more verification done).
// The Verifier role takes precedence over Implementer when both touched the same file.
func ResolveConflict(c Conflict, tasks []Task) Resolution {
	taskMap := map[string]Task{}
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	tA := taskMap[c.TaskA]
	tB := taskMap[c.TaskB]

	winner := c.AgentA
	reason := "agent A had more verification evidence"

	// Prefer verified task (longer evidence = more thorough verification)
	if len(tB.Evidence) > len(tA.Evidence) {
		winner = c.AgentB
		reason = "agent B had more verification evidence"
	}

	// Failed task loses to successful task
	if tA.Status == TaskFailed && tB.Status == TaskDone {
		winner = c.AgentB
		reason = "agent A's task failed; agent B's passed"
	} else if tB.Status == TaskFailed && tA.Status == TaskDone {
		winner = c.AgentA
		reason = "agent B's task failed; agent A's passed"
	}

	return Resolution{
		Conflict:   c,
		Winner:     winner,
		Reason:     reason,
		ResolvedAt: time.Now().UTC(),
	}
}

// FormatConflicts renders a conflict report for display.
func FormatConflicts(conflicts []Conflict, resolutions []Resolution) string {
	if len(conflicts) == 0 {
		return "No conflicts detected.\n"
	}

	resMap := map[string]Resolution{}
	for _, r := range resolutions {
		resMap[r.File] = r
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d conflict(s) detected:\n\n", len(conflicts)))
	for _, c := range conflicts {
		sb.WriteString(fmt.Sprintf("  File: %s\n", c.File))
		sb.WriteString(fmt.Sprintf("  Agent A (%s, task %s) vs Agent B (%s, task %s)\n",
			c.AgentA, c.TaskA, c.AgentB, c.TaskB))
		if r, ok := resMap[c.File]; ok {
			sb.WriteString(fmt.Sprintf("  → Resolved: kept agent %s (%s)\n", r.Winner, r.Reason))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
