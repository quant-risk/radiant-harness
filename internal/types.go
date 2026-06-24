// Package radiant provides the core types for the Radiant Harness.
package radiant

import "time"

// AgentID identifies a supported AI agent client.
type AgentID string

const (
	AgentClaude   AgentID = "claude"
	AgentCodex    AgentID = "codex"
	AgentCursor   AgentID = "cursor"
	AgentCopilot  AgentID = "copilot"
	AgentGemini   AgentID = "gemini"
	AgentWindsurf AgentID = "windsurf"
)

// AllAgents returns all supported agent IDs in alphabetical order. Used by
// `radiant init --all` to generate configs for every supported vendor.
func AllAgents() []AgentID {
	return []AgentID{AgentClaude, AgentCodex, AgentCopilot, AgentCursor, AgentGemini, AgentWindsurf}
}

// IsValidAgent returns true if the ID is a supported agent.
func IsValidAgent(id string) bool {
	switch AgentID(id) {
	case AgentClaude, AgentCodex, AgentCursor, AgentCopilot, AgentGemini, AgentWindsurf:
		return true
	}
	return false
}

// AgentAdapter defines how to transform canonical content for a specific agent.
type AgentAdapter struct {
	ID           AgentID
	Label        string
	Canonical    bool
	InstTo       string // instruction file path
	InstFM       string // "keep" or "strip"
	SkillsDir    string
	SkillsLayout string // "skill-dir" or "flat"
	SkillsExt    string
}

// Spec represents a parsed spec.md file.
type Spec struct {
	Name        string
	Description string
	Summary     string
	ACs         []AcceptanceCriteria
	OutOfScope  []string
	EdgeCases   []string
}

// AcceptanceCriteria is a single AC from spec.md.
type AcceptanceCriteria struct {
	ID    string // "AC-1", "AC-2", ...
	Title string
	Given string
	When  string
	Then  string
}

// Task represents a single task from tasks.md.
type Task struct {
	ID        int
	Name      string
	CoversACs []string
	DependsOn []int
	Gate      string
	Status    string // "todo", "in_progress", "done"
	Parallel  bool   // marked with [P]
}

// TaskPlan is the full task breakdown.
type TaskPlan struct {
	Tasks   []Task
	Phases  []Phase // sequential groups
}

// Phase groups tasks that can run in parallel.
type Phase struct {
	Name  string
	Tasks []Task
}

// HarnessState represents the current state of the harness.
type HarnessState string

const (
	StateIdle       HarnessState = "idle"
	StateResearch   HarnessState = "research"
	StatePlan       HarnessState = "plan"
	StateImplement  HarnessState = "implement"
	StateValidate   HarnessState = "validate"
	StateCorrecting HarnessState = "correcting"
	StateDone       HarnessState = "done"
	StateFailed     HarnessState = "failed"
)

// Progress tracks the progress of a feature implementation.
type Progress struct {
	Feature     string
	State       HarnessState
	CurrentTask int
	TotalTasks  int
	StartedAt   time.Time
	UpdatedAt   time.Time
	Log         []ProgressEntry
}

// ProgressEntry is a single log entry.
type ProgressEntry struct {
	Timestamp time.Time
	TaskID    int
	Action    string // "started", "completed", "failed", "corrected"
	Detail    string
}

// ValidationResult is the output of a validation run.
type ValidationResult struct {
	Passed    bool
	Score     float64 // 0.0 to 1.0
	ACResults []ACResult
	Errors    []string
}

// ACResult is the validation result for a single AC.
type ACResult struct {
	AC       string
	TestFile string
	TestName string
	Passed   bool
	Output   string
}

// FeedbackResult is the output of the feedback loop.
type FeedbackResult struct {
	Succeeded   bool
	Attempts    int
	FinalResult ValidationResult
	Corrections []Correction
}

// Correction records a single correction attempt.
type Correction struct {
	Attempt int
	Errors  []string
	Fixed   bool
	Output  string
}

// Manifest is stored in .radiant-harness/manifest.json.
type Manifest struct {
	Version string   `json:"version"`
	Agents  []string `json:"agents"`
}

// ScriptResult is the output of a quality script.
type ScriptResult struct {
	OK       bool
	Errors   []string
	Warnings []string
}
