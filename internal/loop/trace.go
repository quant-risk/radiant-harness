package loop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TraceEvent is a single action recorded during a loop run.
// It forms the append-only JSONL reasoning trace.
type TraceEvent struct {
	Timestamp  time.Time         `json:"ts"`
	RunID      string            `json:"run"`
	Phase      Phase             `json:"phase"`
	Action     string            `json:"action"`
	Agent      string            `json:"agent,omitempty"`
	PromptHash string            `json:"prompt_hash,omitempty"` // sha256[:8] of prompt
	TokensIn   int               `json:"tokens_in,omitempty"`
	TokensOut  int               `json:"tokens_out,omitempty"`
	Result     string            `json:"result"` // "ok" | "failed" | "skipped"
	Evidence   string            `json:"evidence,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}

// Tracer records loop events to an append-only JSONL file.
// All methods are safe for concurrent use.
type Tracer struct {
	mu    sync.Mutex
	runID string
	path  string
	file  *os.File
}

// NewTracer opens (or creates) the JSONL trace file for runID.
// The file lives at <projectDir>/.radiant-harness/traces/<runID>.jsonl.
func NewTracer(projectDir, runID string) (*Tracer, error) {
	dir := filepath.Join(projectDir, ".radiant-harness", "traces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir traces: %w", err)
	}
	path := filepath.Join(dir, runID+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open trace file: %w", err)
	}
	return &Tracer{runID: runID, path: path, file: f}, nil
}

// Record appends an event to the trace file.
func (t *Tracer) Record(e TraceEvent) error {
	e.Timestamp = time.Now().UTC()
	e.RunID = t.runID

	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	_, err = fmt.Fprintf(t.file, "%s\n", b)
	return err
}

// Close flushes and closes the trace file.
func (t *Tracer) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.file == nil {
		return nil
	}
	if err := t.file.Sync(); err != nil {
		return err
	}
	err := t.file.Close()
	t.file = nil
	return err
}

// Path returns the absolute path of the trace file.
func (t *Tracer) Path() string {
	return t.path
}

// RunID returns the run identifier for this tracer.
func (t *Tracer) RunID() string {
	return t.runID
}

// ReadTrace reads all events from a JSONL trace file.
func ReadTrace(path string) ([]TraceEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read trace: %w", err)
	}
	var events []TraceEvent
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e TraceEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("parse trace line: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

// ListTraces returns all run IDs (trace file basenames) in projectDir.
func ListTraces(projectDir string) ([]string, error) {
	dir := filepath.Join(projectDir, ".radiant-harness", "traces")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".jsonl"))
		}
	}
	return ids, nil
}

// TraceInfo summarises a single trace file without loading all events.
type TraceInfo struct {
	RunID      string    `json:"run_id"`
	EventCount int       `json:"event_count"`
	LastPhase  Phase     `json:"last_phase,omitempty"`
	LastResult string    `json:"last_result,omitempty"`
	LastAction string    `json:"last_action,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
	TokensIn   int       `json:"tokens_in"`
	TokensOut  int       `json:"tokens_out"`
	CostUSD    float64   `json:"cost_usd"`
	ModelID    string    `json:"model_id,omitempty"`
}

// ListTraceInfos returns a summary row per trace, newest-first.
func ListTraceInfos(projectDir string) ([]TraceInfo, error) {
	ids, err := ListTraces(projectDir)
	if err != nil {
		return nil, err
	}
	infos := make([]TraceInfo, 0, len(ids))
	for _, id := range ids {
		path := TracePath(projectDir, id)
		events, err := ReadTrace(path)
		if err != nil || len(events) == 0 {
			infos = append(infos, TraceInfo{RunID: id})
			continue
		}
		last := events[len(events)-1]
		var tIn, tOut int
		var modelID string
		for _, e := range events {
			tIn += e.TokensIn
			tOut += e.TokensOut
			if e.Meta != nil && e.Meta["model"] != "" {
				modelID = e.Meta["model"]
			}
		}
		cost, _ := EstimateCost(modelID, tIn, tOut)
		infos = append(infos, TraceInfo{
			RunID:      id,
			EventCount: len(events),
			LastPhase:  last.Phase,
			LastResult: last.Result,
			LastAction: last.Action,
			UpdatedAt:  last.Timestamp,
			TokensIn:   tIn,
			TokensOut:  tOut,
			CostUSD:    cost,
			ModelID:    modelID,
		})
	}
	// Newest first (by UpdatedAt; zero times sort last).
	for i := 1; i < len(infos); i++ {
		for j := i; j > 0 && infos[j].UpdatedAt.After(infos[j-1].UpdatedAt); j-- {
			infos[j], infos[j-1] = infos[j-1], infos[j]
		}
	}
	return infos, nil
}

// FormatTraceList renders a compact table of trace infos.
func FormatTraceList(infos []TraceInfo) string {
	if len(infos) == 0 {
		return "No traces found. Start a loop with: radiant loop start \"<goal>\"\n"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-36s  %6s  %-10s  %-8s  %9s  %s\n", "RUN-ID", "EVENTS", "PHASE", "RESULT", "COST", "UPDATED"))
	sb.WriteString(strings.Repeat("-", 88) + "\n")
	for _, info := range infos {
		ts := ""
		if !info.UpdatedAt.IsZero() {
			ts = info.UpdatedAt.Format("2006-01-02 15:04")
		}
		runID := info.RunID
		if len(runID) > 36 {
			runID = runID[:33] + "..."
		}
		cost := ""
		if info.CostUSD > 0 {
			cost = FormatCost(info.CostUSD)
		}
		sb.WriteString(fmt.Sprintf("%-36s  %6d  %-10s  %-8s  %9s  %s\n",
			runID, info.EventCount, info.LastPhase, info.LastResult, cost, ts))
	}
	return sb.String()
}

// TracePath returns the expected JSONL file path for a given runID.
func TracePath(projectDir, runID string) string {
	return filepath.Join(projectDir, ".radiant-harness", "traces", runID+".jsonl")
}

// FormatProgress renders a compact status summary of a running or completed loop.
// It derives iteration number, current phase, token totals, and last action from
// the event stream without needing the live Cycle object.
func FormatProgress(runID, modelID string, events []TraceEvent) string {
	if len(events) == 0 {
		return fmt.Sprintf("Run %s — no events recorded yet.\n", runID)
	}
	// modelID can be read from the first event's Meta if not passed.
	if modelID == "" {
		if events[0].Meta != nil {
			modelID = events[0].Meta["model"]
		}
	}

	var (
		tokensIn, tokensOut int
		iteration           int
		lastPhase           Phase
		lastAction          string
		lastResult          string
		lastEvidence        string
		firstTS             = events[0].Timestamp
		lastTS              = events[len(events)-1].Timestamp
	)

	for _, e := range events {
		tokensIn += e.TokensIn
		tokensOut += e.TokensOut
		if e.Phase == PhaseDiscover {
			iteration++
		}
		lastPhase = e.Phase
		lastAction = e.Action
		lastResult = e.Result
		lastEvidence = e.Evidence
	}
	if iteration == 0 {
		iteration = 1
	}

	elapsed := lastTS.Sub(firstTS).Round(time.Second)
	totalTokens := tokensIn + tokensOut

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Run:      %s\n", runID))
	sb.WriteString(fmt.Sprintf("Elapsed:  %s  (%s → %s)\n",
		elapsed, firstTS.Format("15:04:05"), lastTS.Format("15:04:05")))
	sb.WriteString(fmt.Sprintf("Iteration: %d\n", iteration))
	sb.WriteString(fmt.Sprintf("Phase:     %s\n", lastPhase))
	sb.WriteString(fmt.Sprintf("Tokens:    %d total (%d in / %d out)\n", totalTokens, tokensIn, tokensOut))
	if cost, ok := EstimateCost(modelID, tokensIn, tokensOut); ok {
		sb.WriteString(fmt.Sprintf("Cost:      %s\n", FormatCost(cost)))
	}
	sb.WriteString(fmt.Sprintf("Events:    %d\n", len(events)))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Last action: %s → %s %s\n", lastAction, resultIcon(lastResult), lastResult))
	if lastEvidence != "" {
		ev := lastEvidence
		if len(ev) > 80 {
			ev = ev[:77] + "..."
		}
		sb.WriteString(fmt.Sprintf("Evidence:    %s\n", ev))
	}
	return sb.String()
}

// FormatTrace renders a trace as human-readable text.
func FormatTrace(events []TraceEvent) string {
	if len(events) == 0 {
		return "(empty trace)\n"
	}
	var sb strings.Builder
	for _, e := range events {
		icon := resultIcon(e.Result)
		sb.WriteString(fmt.Sprintf("%s [%s] %s — %s %s",
			e.Timestamp.Format("15:04:05"),
			e.Phase,
			icon,
			e.Action,
			e.Result,
		))
		if e.Evidence != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", e.Evidence))
		}
		if e.TokensIn+e.TokensOut > 0 {
			sb.WriteString(fmt.Sprintf(" [%d+%d tokens]", e.TokensIn, e.TokensOut))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func resultIcon(result string) string {
	switch result {
	case "ok":
		return "✓"
	case "failed":
		return "✗"
	case "skipped":
		return "⊘"
	default:
		return "·"
	}
}
