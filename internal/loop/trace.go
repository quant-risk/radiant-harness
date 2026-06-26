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
