package casetest

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Report is the structured result of one test-case run.
type Report struct {
	Case       *Case
	Config     Config
	StartedAt  time.Time
	FinishedAt time.Time
	Events     []Event
	Outcome    string // "success" | "critical_failure" | "exit" | "timeout"
	Summary    string // assistant's final result text (verbatim from harness)
}

// renderReport builds the structured report from the events + final outcome.
func renderReport(c *Case, cfg Config, events []Event) *Report {
	start, end := time.Time{}, time.Now()
	for _, e := range events {
		if start.IsZero() || e.At.Before(start) {
			start = e.At
		}
	}
	summary := lastAssistantMessage(events)
	outcome := classifyOutcome(summary, events)
	return &Report{
		Case:       c,
		Config:     cfg,
		StartedAt:  start,
		FinishedAt: end,
		Events:     events,
		Outcome:    outcome,
		Summary:    summary,
	}
}

// classifyOutcome extracts the harness's exit reason from the final
// assistant message ("Exit: success", "Loop failed: …", etc.). When the
// report never completed a phase, returns "incomplete".
func classifyOutcome(summary string, events []Event) string {
	low := strings.ToLower(summary)
	switch {
	case low == "":
		return "incomplete"
	case strings.Contains(low, "exit: success"):
		return "success"
	case strings.Contains(low, "loop failed"):
		return "critical_failure"
	case strings.Contains(low, "critical_failure"):
		return "critical_failure"
	case strings.Contains(low, "exit:") :
		return strings.TrimSpace(strings.SplitN(strings.SplitN(low, "exit:", 2)[1], "\n", 2)[0])
	}
	// If we have phase-done for all 4 phases but no explicit exit summary,
	// assume success.
	phases := map[Phase]bool{}
	for _, e := range events {
		if e.Kind == "phase-done" {
			phases[e.Phase] = true
		}
	}
	if len(phases) == 4 {
		return "success"
	}
	return "incomplete"
}

// lastAssistantMessage returns the text of the final event emitted
// before the driver returned (the harness's final assistant message).
func lastAssistantMessage(events []Event) string {
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if e.Kind == "final" || (e.Kind == "phase-done" && e.Phase == PhaseVerify) {
			return e.Text
		}
	}
	return ""
}

// WriteMarkdown renders the structured report as Markdown and writes
// it to `w`. The format is designed to be readable in plaintext, on
// GitHub, and convertible to HTML without further work.
func (r *Report) WriteMarkdown(w io.Writer) error {
	out := &strings.Builder{}
	if _, err := fmt.Fprintln(out, "# radiant test-case report"); err != nil {
		return err
	}

	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "- **case name**:        %s\n", r.Case.Name)
	fmt.Fprintf(out, "- **task id**:          %s\n", r.Case.TaskID)
	fmt.Fprintf(out, "- **case path**:        %s\n", r.Case.Path)
	fmt.Fprintf(out, "- **cold start (ms)**:  %d ± %d\n", r.Config.ColdStartMs, r.Config.JitterMs)
	fmt.Fprintf(out, "- **sampling timeout**: %s\n", r.Config.SamplingTO)
	fmt.Fprintf(out, "- **profile**:          %s\n", r.Config.Profile)
	fmt.Fprintf(out, "- **started**:          %s\n", r.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(out, "- **finished**:         %s\n", r.FinishedAt.Format(time.RFC3339))
	fmt.Fprintf(out, "- **elapsed**:          %s\n", r.FinishedAt.Sub(r.StartedAt).Round(time.Millisecond))
	fmt.Fprintf(out, "- **outcome**:          %s\n", r.Outcome)
	fmt.Fprintf(out, "- **sampling calls**:   %d\n", r.SamplingCallCount())
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "## Per-phase timing")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "| phase | sampling calls | simulated latency |")
	fmt.Fprintln(out, "|-------|-----------------|--------------------|")
	for _, ph := range []Phase{PhaseDiscover, PhasePlan, PhaseExecute, PhaseVerify} {
		msgs := r.MessagesByPhase(ph)
		if len(msgs) == 0 {
			continue
		}
		fmt.Fprintf(out, "| %s | %d | %s |\n", ph, len(msgs), msgs[0].Latency.Round(time.Millisecond))
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "## Final assistant message")
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "```text\n%s\n```\n", r.Summary)

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "## Full event log")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "```json")
	for _, e := range r.Events {
		fmt.Fprintf(out,
			"%s kind=%-10s phase=%-9s text=%q\n",
			e.At.Format(time.RFC3339Nano), e.Kind, e.Phase, truncateForLog(e.Text, 80))
	}
	fmt.Fprintln(out, "```")

	_, err := io.WriteString(w, out.String())
	return err
}

// SamplingCallCount returns the number of sampling/createMessage
// exchanges the driver observed. This is the single best proxy for
// "how many round-trips would a real LLM-backed agent make?".
func (r *Report) SamplingCallCount() int {
	n := 0
	for _, e := range r.Events {
		if e.Kind == "sampling" {
			n++
		}
	}
	return n
}

// MessagesByPhase returns the sampling events filtered by phase.
func (r *Report) MessagesByPhase(p Phase) []Event {
	var out []Event
	for _, e := range r.Events {
		if e.Kind == "sampling" && e.Phase == p {
			out = append(out, e)
		}
	}
	return out
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
