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

// Coordinator orchestrates multi-agent execution.
// It does NOT spawn real processes — it manages state and provides
// the scaffolding for LLM-driven agents to coordinate.
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
	sb.WriteString(fmt.Sprintf("Agents: %d\n\n", status.AgentCount))

	// Task table
	sb.WriteString(fmt.Sprintf("%-6s %-30s %-10s %-12s\n", "ID", "Title", "Status", "Agent"))
	sb.WriteString(strings.Repeat("-", 62) + "\n")
	for _, t := range status.Tasks {
		title := t.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		agent := t.AgentID
		if agent == "" {
			agent = "(unassigned)"
		}
		sb.WriteString(fmt.Sprintf("%-6s %-30s %-10s %-12s\n",
			t.ID, title, t.Status, agent))
	}

	if len(status.Conflicts) > 0 {
		sb.WriteString("\n")
		sb.WriteString(FormatConflicts(status.Conflicts, status.Resolutions))
	}

	return sb.String()
}
