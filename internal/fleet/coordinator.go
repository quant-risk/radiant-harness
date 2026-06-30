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
				//
				// v3.7.13 — also include great-grandchildren
				// counts for the rare but possible case where
				// the task spawned a helper that spawned a
				// sub-helper that spawned a subprocess.
				ev := fmt.Sprintf("agent pid %d not alive (liveness probe)", pid)
				if len(tree.ChildrenPids) > 0 {
					ev += fmt.Sprintf("; %d children recorded, %d still alive",
						len(tree.ChildrenPids), tree.ChildCount)
				}
				if len(tree.GrandchildrenPids) > 0 {
					ev += fmt.Sprintf("; %d grandchildren recorded, %d still alive",
						len(tree.GrandchildrenPids), tree.GrandchildrenCount)
				}
				if len(tree.GreatGrandchildrenPids) > 0 {
					ev += fmt.Sprintf("; %d great-grandchildren recorded, %d still alive",
						len(tree.GreatGrandchildrenPids), tree.GreatGrandchildrenCount)
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

// FormatStatusHTML renders the fleet status as a self-contained HTML
// report. v3.7.13+ — surfaces the nested pid tree (parent → child →
// grandchild → great-grandchild) visually so the operator can scan
// orphaned helpers at a glance without parsing JSON. No external
// dependencies — pure HTML + CSS, no JavaScript, no remote fonts, no
// CDN. Designed to render offline and survive the same "save as
// email attachment" workflow as the text report.
//
// Layout:
//   - Header: run id, goal, agent count, task counts by status
//   - Per-task section with status pill, agent id, worktree, evidence
//   - Per-task pid tree (when liveness dir is set): tree drawn with
//     ASCII glyphs wrapped in <pre> — survives mail clients and
//     terminal-style viewers better than SVG
//   - Footer: run start time + radiant version
func FormatStatusHTML(status FleetStatus) string {
	var sb strings.Builder

	// Counts
	counts := map[TaskStatus]int{}
	for _, t := range status.Tasks {
		counts[t.Status]++
	}
	total := len(status.Tasks)

	// HTML escape helper
	esc := func(s string) string {
		r := strings.NewReplacer(
			"&", "&amp;",
			"<", "&lt;",
			">", "&gt;",
			"\"", "&quot;",
			"'", "&#39;",
		)
		return r.Replace(s)
	}

	// Status → CSS class mapping
	statusClass := func(s TaskStatus) string {
		switch s {
		case TaskDone:
			return "status-done"
		case TaskFailed, TaskCrashed:
			return "status-failed"
		case TaskAssigned:
			return "status-assigned"
		case TaskPending:
			return "status-pending"
		default:
			return "status-other"
		}
	}

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Fleet `)
	sb.WriteString(esc(status.RunID))
	sb.WriteString(`</title>
<style>
  :root {
    --bg: #0e1116;
    --fg: #e6edf3;
    --muted: #7d8590;
    --border: #21262d;
    --accent: #58a6ff;
    --done: #3fb950;
    --failed: #f85149;
    --assigned: #d29922;
    --pending: #6e7681;
    --other: #a371f7;
    --tree-ok: #3fb950;
    --tree-dead: #f85149;
    --code-bg: #161b22;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    padding: 24px;
    background: var(--bg);
    color: var(--fg);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    font-size: 14px;
    line-height: 1.5;
  }
  .container { max-width: 960px; margin: 0 auto; }
  h1 { font-size: 24px; margin: 0 0 8px 0; color: var(--fg); }
  h2 { font-size: 16px; margin: 32px 0 12px 0; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
  h3 { font-size: 14px; margin: 16px 0 8px 0; color: var(--fg); }
  .meta { color: var(--muted); margin-bottom: 24px; }
  .meta span { display: inline-block; margin-right: 16px; }
  .meta strong { color: var(--fg); font-weight: 600; }
  .counts { display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 24px; }
  .count {
    padding: 6px 12px;
    background: var(--code-bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 13px;
  }
  .count strong { color: var(--accent); }
  .task {
    border: 1px solid var(--border);
    border-radius: 8px;
    margin-bottom: 12px;
    overflow: hidden;
  }
  .task-header {
    padding: 12px 16px;
    background: var(--code-bg);
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }
  .task-id {
    font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
    font-size: 12px;
    color: var(--muted);
  }
  .task-title { flex: 1; min-width: 200px; }
  .pill {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .pill.status-done { background: rgba(63,185,80,0.15); color: var(--done); }
  .pill.status-failed { background: rgba(248,81,73,0.15); color: var(--failed); }
  .pill.status-assigned { background: rgba(210,153,34,0.15); color: var(--assigned); }
  .pill.status-pending { background: rgba(110,118,129,0.15); color: var(--pending); }
  .pill.status-other { background: rgba(163,113,247,0.15); color: var(--other); }
  .task-body { padding: 12px 16px; }
  .task-body dl { margin: 0; display: grid; grid-template-columns: 120px 1fr; gap: 6px 12px; font-size: 13px; }
  .task-body dt { color: var(--muted); }
  .task-body dd { margin: 0; font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace; }
  .evidence { margin-top: 8px; font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace; font-size: 12px; color: var(--muted); }
  pre.pid-tree {
    margin: 8px 0 0 0;
    padding: 10px 12px;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
    font-size: 12px;
    overflow-x: auto;
    white-space: pre;
    line-height: 1.4;
  }
  .pid-alive { color: var(--tree-ok); }
  .pid-dead { color: var(--tree-dead); }
  .empty { color: var(--muted); padding: 32px; text-align: center; border: 1px dashed var(--border); border-radius: 8px; }
  .footer { margin-top: 32px; padding-top: 16px; border-top: 1px solid var(--border); color: var(--muted); font-size: 12px; }
  .footer code { color: var(--fg); }
</style>
</head>
<body>
<div class="container">
`)

	sb.WriteString("  <h1>Fleet ")
	sb.WriteString(esc(status.RunID))
	sb.WriteString("</h1>\n")
	sb.WriteString("  <div class=\"meta\">\n")
	sb.WriteString("    <span>Goal: <strong>")
	sb.WriteString(esc(status.Goal))
	sb.WriteString("</strong></span>\n")
	sb.WriteString("    <span>Agents: <strong>")
	sb.WriteString(fmt.Sprintf("%d", status.AgentCount))
	sb.WriteString("</strong></span>\n")
	sb.WriteString("  </div>\n")

	// Counts
	sb.WriteString("  <div class=\"counts\">\n")
	sb.WriteString(fmt.Sprintf("    <div class=\"count\">Total: <strong>%d</strong></div>\n", total))
	if total > 0 {
		sb.WriteString(fmt.Sprintf("    <div class=\"count\">Pending: <strong>%d</strong></div>\n", counts[TaskPending]))
		sb.WriteString(fmt.Sprintf("    <div class=\"count\">Assigned: <strong>%d</strong></div>\n", counts[TaskAssigned]))
		sb.WriteString(fmt.Sprintf("    <div class=\"count\">Done: <strong>%d</strong></div>\n", counts[TaskDone]))
		sb.WriteString(fmt.Sprintf("    <div class=\"count\">Failed: <strong>%d</strong></div>\n", counts[TaskFailed]))
	}
	if status.DispatcherAlive {
		sb.WriteString("    <div class=\"count\">Dispatcher: <strong style=\"color:var(--done)\">alive (pid ")
		sb.WriteString(fmt.Sprintf("%d", status.DispatcherPid))
		sb.WriteString(")</strong></div>\n")
	} else {
		sb.WriteString("    <div class=\"count\">Dispatcher: <strong style=\"color:var(--failed)\">not alive</strong></div>\n")
	}
	sb.WriteString("  </div>\n")

	// Tasks
	if total == 0 {
		sb.WriteString("  <div class=\"empty\">(no tasks — run <code>radiant fleet plan &lt;run-id&gt;</code> first)</div>\n")
	} else {
		sb.WriteString("  <h2>Tasks</h2>\n")
		for _, t := range status.Tasks {
			cls := statusClass(t.Status)
			sb.WriteString("  <div class=\"task\">\n")
			sb.WriteString("    <div class=\"task-header\">\n")
			sb.WriteString("      <span class=\"pill ")
			sb.WriteString(cls)
			sb.WriteString("\">")
			sb.WriteString(esc(string(t.Status)))
			sb.WriteString("</span>\n")
			sb.WriteString("      <span class=\"task-title\">")
			sb.WriteString(esc(t.Title))
			sb.WriteString("</span>\n")
			sb.WriteString("      <span class=\"task-id\">")
			sb.WriteString(esc(t.ID))
			sb.WriteString("</span>\n")
			sb.WriteString("    </div>\n")
			sb.WriteString("    <div class=\"task-body\">\n")
			sb.WriteString("      <dl>\n")
			if t.AgentID != "" {
				sb.WriteString("        <dt>Agent</dt><dd>")
				sb.WriteString(esc(t.AgentID))
				sb.WriteString("</dd>\n")
			}
			if t.WorktreeDir != "" {
				sb.WriteString("        <dt>Worktree</dt><dd>")
				sb.WriteString(esc(t.WorktreeDir))
				sb.WriteString("</dd>\n")
			}
			if t.Status == TaskDone && t.Evidence != "" {
				sb.WriteString("        <dt>Evidence</dt><dd>")
				sb.WriteString(esc(t.Evidence))
				sb.WriteString("</dd>\n")
			} else if t.Evidence != "" {
				sb.WriteString("        <dt>Evidence</dt><dd class=\"evidence\">")
				sb.WriteString(esc(t.Evidence))
				sb.WriteString("</dd>\n")
			}
			// Pid tree (resolved from TaskLiveness map if present)
			if live, ok := status.TaskLiveness[t.ID]; ok {
				if live.Tree.ParentPid != 0 || len(live.Tree.ChildrenPids) > 0 || len(live.Tree.GrandchildrenPids) > 0 || len(live.Tree.GreatGrandchildrenPids) > 0 {
					sb.WriteString("        <dt>PID tree</dt><dd>")
					sb.WriteString(renderPidTreeHTML(live.Tree))
					sb.WriteString("</dd>\n")
				}
			}
			sb.WriteString("      </dl>\n")
			sb.WriteString("    </div>\n")
			sb.WriteString("  </div>\n")
		}
	}

	if len(status.Conflicts) > 0 {
		sb.WriteString("  <h2>Conflicts</h2>\n")
		sb.WriteString("  <pre>")
		sb.WriteString(esc(FormatConflicts(status.Conflicts, status.Resolutions)))
		sb.WriteString("</pre>\n")
	}

	// Footer
	sb.WriteString("  <div class=\"footer\">\n")
	sb.WriteString("    Generated by <code>radiant fleet status --html</code>\n")
	sb.WriteString("  </div>\n")
	sb.WriteString("</div>\n")
	sb.WriteString("</body>\n</html>\n")

	return sb.String()
}

// renderPidTreeHTML renders the nested PidTree as a visual ASCII tree
// in a <pre> block. Each node is colored by liveness — alive = green,
// dead = red, vacuous (empty list) = muted. Uses Unicode box-drawing
// glyphs (└─ ├─) for cleaner rendering than ASCII pipes.
func renderPidTreeHTML(t PidTree) string {
	var sb strings.Builder

	pidSpan := func(pid, fallback int) string {
		if pid <= 0 {
			pid = fallback
		}
		return fmt.Sprintf("%d", pid)
	}

	// Parent line
	parentLabel := "parent " + pidSpan(t.ParentPid, t.ParentPid)
	if t.ParentAlive {
		sb.WriteString("<span class=\"pid-alive\">")
	} else {
		sb.WriteString("<span class=\"pid-dead\">")
	}
	sb.WriteString("● ")
	sb.WriteString(parentLabel)
	sb.WriteString("</span>\n")

	// Children
	if len(t.ChildrenPids) == 0 {
		sb.WriteString("<span class=\"pid-dead\">└─ (no children recorded)</span>\n")
	} else {
		for i, c := range t.ChildrenPids {
			connector := "├─"
			if i == len(t.ChildrenPids)-1 {
				connector = "└─"
			}
			sb.WriteString("<span class=\"pid-")
			if t.ChildrenAlive {
				sb.WriteString("alive\">")
			} else {
				sb.WriteString("dead\">")
			}
			sb.WriteString(connector)
			sb.WriteString(" child ")
			sb.WriteString(fmt.Sprintf("%d", c))
			sb.WriteString("</span>")
			if t.ChildCount > 0 {
				sb.WriteString(fmt.Sprintf(" (%d live)", t.ChildCount))
			}
			sb.WriteString("\n")
		}
	}

	// Grandchildren
	if len(t.GrandchildrenPids) == 0 {
		if len(t.ChildrenPids) > 0 {
			sb.WriteString("<span class=\"pid-dead\">   └─ (no grandchildren recorded)</span>\n")
		}
	} else {
		for i, gc := range t.GrandchildrenPids {
			connector := "   ├─"
			if i == len(t.GrandchildrenPids)-1 {
				connector = "   └─"
			}
			sb.WriteString("<span class=\"pid-")
			if t.GrandchildrenAlive {
				sb.WriteString("alive\">")
			} else {
				sb.WriteString("dead\">")
			}
			sb.WriteString(connector)
			sb.WriteString(" grandchild ")
			sb.WriteString(fmt.Sprintf("%d", gc))
			sb.WriteString("</span>")
			if t.GrandchildrenCount > 0 {
				sb.WriteString(fmt.Sprintf(" (%d live)", t.GrandchildrenCount))
			}
			sb.WriteString("\n")
		}
	}

	// Great-grandchildren
	if len(t.GreatGrandchildrenPids) == 0 {
		if len(t.GrandchildrenPids) > 0 {
			sb.WriteString("<span class=\"pid-dead\">      └─ (no great-grandchildren recorded)</span>\n")
		}
	} else {
		for i, ggc := range t.GreatGrandchildrenPids {
			connector := "      ├─"
			if i == len(t.GreatGrandchildrenPids)-1 {
				connector = "      └─"
			}
			sb.WriteString("<span class=\"pid-")
			if t.GreatGrandchildrenAlive {
				sb.WriteString("alive\">")
			} else {
				sb.WriteString("dead\">")
			}
			sb.WriteString(connector)
			sb.WriteString(" great-grandchild ")
			sb.WriteString(fmt.Sprintf("%d", ggc))
			sb.WriteString("</span>")
			if t.GreatGrandchildrenCount > 0 {
				sb.WriteString(fmt.Sprintf(" (%d live)", t.GreatGrandchildrenCount))
			}
			sb.WriteString("\n")
		}
	}

	return "<pre class=\"pid-tree\">" + sb.String() + "</pre>"
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
