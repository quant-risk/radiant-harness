package fleet

import (
	"fmt"
	"strings"
	"time"
)

// FleetStatus describes the overall state of a multi-agent run.
type FleetStatus struct {
	RunID       string       `json:"run_id"`
	Goal        string       `json:"goal"`
	AgentCount  int          `json:"agent_count"`
	Tasks       []Task       `json:"tasks"`
	Conflicts   []Conflict   `json:"conflicts,omitempty"`
	Resolutions []Resolution `json:"resolutions,omitempty"`
	StartedAt   time.Time    `json:"started_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	// Liveness (v3.7.9+) — populated from pid files under
	// `<workdir>/.radiant-harness/fleet/pids/`. DispatcherAlive
	// reflects the optional async-subprocess dispatcher
	// (`radiant fleet-async-runner <run-id>`); TaskLiveness
	// is keyed by task ID and tracks each spawned agent. Both
	// are populated only when the Coordinator was built with a
	// non-empty LivenessDir; otherwise they're omitted.
	DispatcherAlive bool                `json:"dispatcher_alive,omitempty"`
	DispatcherPid   int                 `json:"dispatcher_pid,omitempty"`
	TaskLiveness    map[string]TaskLive `json:"task_liveness,omitempty"`
}

// TaskLive describes the liveness of a single fleet task agent.
// Alive=false with Pid=0 means "no pid file on disk" (the agent
// hasn't started, or finished and the file was cleaned up).
// Alive=false with Pid>0 means "the pid file points at a dead
// process" — the agent crashed without writing terminal status.
//
// v3.7.10: gains a `tree` field with the nested child pid
// liveness. The flat `alive` + `pid` are kept for backwards
// compat — old callers still see "the agent is alive/dead".
type TaskLive struct {
	Alive bool    `json:"alive"`
	Pid   int     `json:"pid,omitempty"`
	Tree  PidTree `json:"tree,omitempty"`
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
	store       *Store
	state       FleetState
	roles       map[AgentRole]RoleConfig
	livenessDir string // optional; populated for v3.7.9 liveness probes
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

// WithLivenessDir enables liveness probing on Status(). The
// dir is the project workdir (the dispatcher resolves the pid
// directory under it). An empty string disables probing.
func (c *Coordinator) WithLivenessDir(workdir string) *Coordinator {
	c.livenessDir = workdir
	return c
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
	out := FleetStatus{
		RunID:       ctx.RunID,
		Goal:        ctx.Goal,
		AgentCount:  len(c.state.Agents),
		Tasks:       ctx.Tasks,
		Conflicts:   conflicts,
		Resolutions: resolutions,
		StartedAt:   c.state.StartedAt,
		UpdatedAt:   ctx.UpdatedAt,
	}

	// v3.7.9 — populate liveness from pid files when a
	// livenessDir was configured. Inline dispatchers have no
	// dispatcher pid file (they ARE the process) — that's fine,
	// `dispatcherLiveness` returns (false, 0) for missing
	// files and we omit the fields via omitempty.
	//
	// The crashed-status escalation mirrors the loop's v3.7.8
	// `phaseStatusSummary` logic: a task that the store still
	// considers `assigned` but whose pid file points at a dead
	// process is reported as `crashed` (the pid file is on disk,
	// the process is gone, the dispatcher never got to call
	// CompleteTask). This distinguishes "agent is still running"
	// from "agent died mid-execution" without requiring the host
	// to re-read every pid file.
	//
	// v3.7.10 — also populate the nested PidTree for each task
	// so the host can see "parent alive, child died" vs "parent
	// alive, all children alive". Tree lookup uses the sidecar
	// .children file (kept fresh by spawnAgent) so we don't
	// have to fork pgrep on every Status() call.
	if c.livenessDir != "" {
		if alive, pid := dispatcherLiveness(c.livenessDir, ctx.RunID); pid > 0 || alive {
			out.DispatcherAlive = alive
			out.DispatcherPid = pid
		}
		out.TaskLiveness = make(map[string]TaskLive, len(ctx.Tasks))
		for i := range ctx.Tasks {
			t := &ctx.Tasks[i]
			alive, pid := taskLiveness(c.livenessDir, ctx.RunID, t.ID)
			tree := TaskPidTree(c.livenessDir, ctx.RunID, t.ID)
			out.TaskLiveness[t.ID] = TaskLive{Alive: alive, Pid: pid, Tree: tree}
			if t.Status == TaskAssigned && pid > 0 && !alive {
				// v3.7.10 — enrich the evidence with child
				// status. If the parent died but children are
				// still alive, we know the agent exited without
				// reaping its helpers (orphaned). If children
				// are also dead, it's a clean crash.
				//
				// v3.7.12 — extend with grandchildren counts
				// so the operator can tell "parent died; 2
				// helpers orphaned; 1 grandchild orphaned" from
				// "parent died; everything cleaned up".
				ev := fmt.Sprintf("agent pid %d not alive (liveness probe)", pid)
				if len(tree.ChildrenPids) > 0 {
					ev += fmt.Sprintf("; %d children recorded, %d still alive",
						len(tree.ChildrenPids), tree.ChildCount)
				}
				if len(tree.GrandchildrenPids) > 0 {
					ev += fmt.Sprintf("; %d grandchildren recorded, %d still alive",
						len(tree.GrandchildrenPids), tree.GrandchildrenCount)
				}
				t.Evidence = ev
				t.Status = TaskCrashed
			}
		}
	}

	return out
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
