package fleet

import (
	"fmt"
	"strings"
	"time"
)

// FleetStatus describes the overall state of a multi-agent run.
type FleetStatus struct {
	RunID       string
	Goal        string
	AgentCount  int
	Tasks       []Task
	Conflicts   []Conflict
	Resolutions []Resolution
	StartedAt   time.Time
	UpdatedAt   time.Time
}

// AgentRecord tracks a single agent in the fleet.
type AgentRecord struct {
	ID         string
	Role       AgentRole
	TaskID     string
	Phase      string
	TokensUsed int
	LastAction string
	StartedAt  time.Time
	UpdatedAt  time.Time
}

// FleetState is persisted per-run.
type FleetState struct {
	RunID     string        `json:"run_id"`
	Goal      string        `json:"goal"`
	Agents    []AgentRecord `json:"agents"`
	StartedAt time.Time     `json:"started_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// Coordinator orchestrates multi-agent execution state.
// Real process spawning is handled by Dispatcher (dispatch.go),
// which runs one OS process per task in an isolated git worktree.
type Coordinator struct {
	store *Store
	state FleetState
	roles map[AgentRole]RoleConfig
}

// NewCoordinator creates a Coordinator backed by a Store.
func NewCoordinator(store *Store, agentCount int) *Coordinator {
	state := FleetState{
		RunID:     store.ctx.RunID,
		Goal:      store.ctx.Goal,
		Agents:    make([]AgentRecord, 0, agentCount),
		StartedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	return &Coordinator{
		store: store,
		state: state,
		roles: DefaultRoleConfigs(),
	}
}

// RegisterAgent adds an agent to the fleet.
func (c *Coordinator) RegisterAgent(id string, role AgentRole) {
	c.state.Agents = append(c.state.Agents, AgentRecord{
		ID:        id,
		Role:      role,
		Phase:     "idle",
		StartedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	c.state.UpdatedAt = time.Now().UTC()
}

// UpdateAgent records a progress update from an agent.
func (c *Coordinator) UpdateAgent(agentID, phase, lastAction string, tokensUsed int) {
	for i := range c.state.Agents {
		if c.state.Agents[i].ID == agentID {
			c.state.Agents[i].Phase = phase
			c.state.Agents[i].LastAction = lastAction
			c.state.Agents[i].TokensUsed = tokensUsed
			c.state.Agents[i].UpdatedAt = time.Now().UTC()
			break
		}
	}
	c.state.UpdatedAt = time.Now().UTC()
}

// Status returns the current fleet status.
func (c *Coordinator) Status() FleetStatus {
	ctx := c.store.Snapshot()
	conflicts := DetectConflicts(ctx.Tasks)
	resolutions := make([]Resolution, 0, len(conflicts))
	for _, conflict := range conflicts {
		resolutions = append(resolutions, ResolveConflict(conflict, ctx.Tasks))
	}
	return FleetStatus{
		RunID:       ctx.RunID,
		Goal:        ctx.Goal,
		AgentCount:  len(c.state.Agents),
		Tasks:       ctx.Tasks,
		Conflicts:   conflicts,
		Resolutions: resolutions,
		StartedAt:   c.state.StartedAt,
		UpdatedAt:   ctx.UpdatedAt,
	}
}

// RolePrompt returns the system prompt for a given role,
// injecting the goal and task assignment context.
func (c *Coordinator) RolePrompt(role AgentRole, task *Task) string {
	cfg, ok := c.roles[role]
	if !ok {
		return ""
	}
	ctx := c.store.Snapshot()
	prompt := cfg.SystemPrompt
	prompt += fmt.Sprintf("\n\nOVERALL GOAL: %s\n", ctx.Goal)
	if task != nil {
		prompt += fmt.Sprintf("\nYOUR TASK:\n  Title: %s\n  Files: %s\n  Done when: %s\n",
			task.Title, strings.Join(task.Files, ", "), task.DoneWhen)
	}
	return prompt
}

// FormatStatus renders fleet status as a human-readable table.
func FormatStatus(status FleetStatus) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Fleet: %s\n", status.RunID))
	sb.WriteString(fmt.Sprintf("Goal:  %s\n", status.Goal))
	sb.WriteString(fmt.Sprintf("Agents: %d\n", status.AgentCount))

	// Count tasks by status.
	counts := map[TaskStatus]int{}
	for _, t := range status.Tasks {
		counts[t.Status]++
	}
	total := len(status.Tasks)
	sb.WriteString(fmt.Sprintf("Tasks:  %d total", total))
	if total > 0 {
		sb.WriteString(fmt.Sprintf(" — %d pending, %d assigned, %d done, %d failed",
			counts[TaskPending], counts[TaskAssigned], counts[TaskDone], counts[TaskFailed]))
	}
	sb.WriteString("\n\n")

	if total == 0 {
		sb.WriteString("  (no tasks — run `radiant fleet plan <run-id>` first)\n")
	} else {
		// Task table with worktree and evidence preview.
		sb.WriteString(fmt.Sprintf("%-8s %-28s %-10s %-14s %s\n", "ID", "Title", "Status", "Agent", "Worktree/Evidence"))
		sb.WriteString(strings.Repeat("-", 80) + "\n")
		for _, t := range status.Tasks {
			title := t.Title
			if len(title) > 28 {
				title = title[:25] + "..."
			}
			agent := t.AgentID
			if agent == "" {
				agent = "(unassigned)"
			}
			extra := t.WorktreeDir
			if t.Status == TaskDone && t.Evidence != "" {
				ev := t.Evidence
				if len(ev) > 40 {
					ev = ev[:37] + "..."
				}
				extra = ev
			}
			sb.WriteString(fmt.Sprintf("%-8s %-28s %-10s %-14s %s\n",
				t.ID, title, t.Status, agent, extra))
		}
	}

	if len(status.Conflicts) > 0 {
		sb.WriteString("\n")
		sb.WriteString(FormatConflicts(status.Conflicts, status.Resolutions))
	}

	return sb.String()
}

// FormatSummary renders a consolidated summary of completed task evidence.
func FormatSummary(status FleetStatus) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Summary — Fleet: %s\n", status.RunID))
	sb.WriteString(fmt.Sprintf("Goal: %s\n\n", status.Goal))

	var done []Task
	for _, t := range status.Tasks {
		if t.Status == TaskDone {
			done = append(done, t)
		}
	}

	if len(done) == 0 {
		sb.WriteString("No completed tasks yet.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("%d/%d tasks completed\n\n", len(done), len(status.Tasks)))
	for _, t := range done {
		sb.WriteString(fmt.Sprintf("── %s: %s\n", t.ID, t.Title))
		if t.Evidence != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", t.Evidence))
		} else {
			sb.WriteString("   (no evidence recorded)\n")
		}
		if t.WorktreeDir != "" {
			sb.WriteString(fmt.Sprintf("   worktree: %s\n", t.WorktreeDir))
		}
		sb.WriteString("\n")
	}

	var failed []Task
	for _, t := range status.Tasks {
		if t.Status == TaskFailed {
			failed = append(failed, t)
		}
	}
	if len(failed) > 0 {
		sb.WriteString(fmt.Sprintf("%d task(s) failed:\n", len(failed)))
		for _, t := range failed {
			sb.WriteString(fmt.Sprintf("  ✗ %s: %s\n", t.ID, t.Title))
		}
	}

	return sb.String()
}
