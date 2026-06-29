// Package possess — agentic tool-calling driver (v3.7.0+).
//
// When the host agent's MCP server implements sampling with native
// tool-use (Anthropic-style content: [{type: "tool_use"}]), the
// harness can drive real file I/O and shell commands through the
// model. The Driver here is the controller:
//
//   1. Build a system prompt that establishes the role + constraints.
//   2. Send the user's task alongside the tool manifest.
//   3. Loop:
//      a. Call backend.ChatWithTools(messages, tools, choice)
//      b. If response text matches a VERDICT/REVIEW marker, we're done.
//      c. Otherwise dispatch each ToolCall via the tools.Registry,
//         capture each result.
//      d. Append a user message carrying [{type: tool_result, id,
//         content}] for each executed tool.
//      e. Re-loop until VERDICT, max_iter, wallclock, or cancellation.
//
// Failure modes:
//
//   - Backend implements llm.ToolCapable: ChatWithTools succeeds.
//   - Backend does NOT implement ToolCapable: NewDriver's caller is
//     expected to either downgrade to text-only Chat() (with the
//     engine's existing fenced-tool_call code-block path) or fall
//     back to the self-driven scaffold pipeline. Driver itself
//     panics if the type assertion fails — by design, because silent
//     text-only fallback hides the loss of tool capability.
//
//   - Backend advertises tools but the model never emits tool_use
//     (model doesn't actually support the feature on this host):
//     Driver detects this after the first non-tool iteration and
//     returns ErrBackendToolsUnsupported so the caller can switch
//     to the self-driven path. The text it produced is preserved
//     in Trace.TextSoFar so the caller can show it.
//
// Concurrency: not safe to share a Driver across goroutines. Each
// possession spawns its own.
package possess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/quant-risk/radiant-harness/internal/llm"
	"github.com/quant-risk/radiant-harness/internal/tools"
)

// ErrBackendToolsUnsupported is returned by Drive when the backend
// accepts tools in the wire request but the model never emits a
// tool_use block. The caller should downgrade to text-only or to
// the self-driven scaffold path.
var ErrBackendToolsUnsupported = errors.New("backend returned no tool_use despite tools offered")

// ErrHostSamplingUnsupported is returned by Drive when the first
// sampling call into the host agent comes back with JSON-RPC -32601
// — i.e. the host does NOT implement sampling/createMessage at all.
// This is a sentinel: callers (cmd_mcp_possess.go::runPossessWithBackend
// in particular) catch it via errors.Is and route to the
// self-driven scaffold path instead of returning a fatal error.
//
// v3.7.1: the previous version (v3.7.0) let this fall through as a
// generic error, leaving the workdir with empty docs/specs/scripts
// and a state.json marked "current_phase=verify, all phases
// pending" — exactly the hollow-stub failure mode v3.6.0 was meant
// to close. Sentinel + caller downgrade closes that gap for the
// agentic path.
//
// Detection: ErrSamplingUnsupported from internal/llm propagates
// through backend.ChatWithTools via wrap, so any host that fails
// the first sampling call surfaces the same way.
var ErrHostSamplingUnsupported = errors.New("host sampling/createMessage returned -32601 (use self-driven)")

// MaxIterDefault caps the model round-trips at a sane value. The
// task in 4-phase possession is bounded; 25 is enough for the
// discover→plan→execute→verify flow with breathing room.
const MaxIterDefault = 25

// MaxWallDefault is a hard cap on the whole run — sampling latency
// on Hermes mimo / xiaomi is 20-40 s per call. 10 minutes lets a
// full 4-phase possession go through even on slow hosts.
const MaxWallDefault = 10 * time.Minute

// Verdict patterns. Either:
//
//   VERDICT: APPROVED|REJECTED       (per-iteration check)
//   SCORE: <0.00-1.00>
//   EVIDENCE: <one sentence>
//   ESCALATE: true|false
//   ISSUES:
//   - <one bullet>
//
// or (post-convergence review panel):
//
//   REVIEW: PASS|FAIL
//   SCORE: ...
//   FINDINGS:
//
// We accept either as a stop signal — the driver doesn't care which
// surface produced it, only that *one of* them means "stop".
var (
	verdictRegex = regexp.MustCompile(`(?m)^VERDICT:\s*(APPROVED|REJECTED)\b`)
	reviewRegex  = regexp.MustCompile(`(?m)^REVIEW:\s*(PASS|FAIL)\b`)
)

// DriverConfig captures the per-run configuration.
type DriverConfig struct {
	// Backend must implement llm.ToolCapable. NewDriver type-asserts
	// up front so a misconfigured caller fails loudly instead of
	// silently downgrading.
	Backend llm.Backend

	// ProjectRoot is the workdir every tool call resolves under.
	// PathIsSafe-equivalent checks happen inside each tool Invoke
	// (fsutil.PathIsSafe) — passing a misformed root lets the
	// individual tool surface the error.
	ProjectRoot string

	// Registry wires tool invocations. NewDriver parses the manifest
	// (tool.Name + Description + Params) and sends it on every
	// sampling call; tool execution routes back through
	// Registry.Call.
	Registry *tools.Registry

	// Profile picks profile-specific knobs (max_iter, max_wallclock).
	// Three values: "lean" | "standard" | "thorough". Empty is
	// treated as "standard".
	Profile string

	// MaxIter overrides MaxIterDefault when set.
	MaxIter int

	// MaxWall overrides MaxWallDefault when set.
	MaxWall time.Duration

	// Out streams human-readable progress. nil = io.Discard.
	Out io.Writer
}

// Driver is the per-run controller. NewDriver returns a configured
// instance; call Drive to run.
type Driver struct {
	cfg     DriverConfig
	wireTls []llm.Tool
	started time.Time
}

// NewDriver validates the config and pre-renders the wire tool
// manifest. The driver panics if cfg.Backend does not implement
// llm.ToolCapable — by design, because silent fallback hides the
// capability loss from the operator.
func NewDriver(cfg DriverConfig) (*Driver, error) {
	if cfg.Backend == nil {
		return nil, errors.New("possess: Backend is required")
	}
	if _, ok := cfg.Backend.(llm.ToolCapable); !ok {
		return nil, errors.New("possess: Backend does not implement llm.ToolCapable (refusing to fall back silently to text-only)")
	}
	if cfg.Registry == nil {
		return nil, errors.New("possess: Registry is required")
	}
	if cfg.ProjectRoot == "" {
		return nil, errors.New("possess: ProjectRoot is required")
	}
	if cfg.MaxIter <= 0 {
		cfg.MaxIter = MaxIterDefault
	}
	if cfg.MaxWall <= 0 {
		cfg.MaxWall = MaxWallDefault
	}
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.Profile == "" {
		cfg.Profile = "standard"
	}

	// Pre-render the wire manifest so ChatWithTools just hands the
	// same slice to every sampling call.
	names := cfg.Registry.Names()
	wire := make([]llm.Tool, 0, len(names))
	for _, name := range names {
		t := cfg.Registry.Get(name)
		if t == nil {
			continue
		}
		wire = append(wire, llm.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: jsonSchemaFromParams(t.Params),
		})
	}

	return &Driver{cfg: cfg, wireTls: wire, started: time.Now()}, nil
}

// jsonSchemaFromParams renders the tool's []tools.Param into a JSON
// schema fragment: {"type":"object","required":[...],"properties":{...}}.
// Anthropic / OpenAI both accept this shape; HTTP backends that want
// a flatter format translate at the boundary.
func jsonSchemaFromParams(params []tools.Param) json.RawMessage {
	required := make([]string, 0)
	properties := make(map[string]any, len(params))
	for _, p := range params {
		properties[p.Name] = map[string]any{
			"type":        p.Type,
			"description": p.Description,
		}
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	b, _ := json.Marshal(schema)
	return b
}

// Trace accumulates a per-run summary. Returned by Drive so the
// caller can emit it on state.json or surface it in the user-facing
// trace.
type Trace struct {
	Iterations  int             // model round-trips
	ToolInvocations []ToolRecord
	TextSoFar   string          // concatenated text (assistant text blocks)
	Verdict     string          // final VERDICT/APPROVED/... line if seen
	VerdictLine int             // 1-based line index of Verdict in TextSoFar
	StartAt     time.Time
	EndAt       time.Time
	Wall        time.Duration
}

// ToolRecord is one tool_use execution. Carries enough for the
// verifier to audit the action without re-running it.
type ToolRecord struct {
	Name      string          `json:"name"`
	ID        string          `json:"id,omitempty"`
	Input     json.RawMessage `json:"input"`
	Output    json.RawMessage `json:"output,omitempty"`
	Err       string          `json:"err,omitempty"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   time.Time       `json:"ended_at"`
}

// Drive runs the agentic loop. system + task are the conversation
// seeds. Returns Trace (always, even on error) + error (nil on
// VERDICT success, non-nil otherwise).
//
// Cancellation: ctx.Err() is checked before each tool invocation
// and before each sampling call. A cancelled run returns ctx.Err()
// wrapped, with whatever progress it managed in Trace.Iterations.
func (d *Driver) Drive(ctx context.Context, system, task string) (*Trace, error) {
	tc, ok := d.cfg.Backend.(llm.ToolCapable)
	if !ok {
		// unreachable — NewDriver checked — but defensive.
		return nil, fmt.Errorf("possess: backend lost ToolCapable mid-run")
	}
	tr := &Trace{StartAt: time.Now(), TextSoFar: ""}

	// Initial message pair: system + user task.
	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: task},
	}

	choice := &llm.ToolChoice{Type: "any"} // encourage tool use each turn

	iter := 0
	sawAnyToolUse := false
	for iter < d.cfg.MaxIter {
		if err := ctx.Err(); err != nil {
			tr.EndAt = time.Now()
			tr.Wall = tr.EndAt.Sub(tr.StartAt)
			return tr, fmt.Errorf("context cancelled at iter %d: %w", iter, err)
		}
		if elapsed := time.Since(tr.StartAt); elapsed > d.cfg.MaxWall {
			tr.EndAt = time.Now()
			tr.Wall = tr.EndAt.Sub(tr.StartAt)
			return tr, fmt.Errorf("wallclock budget exceeded at iter %d (%s > %s)", iter, elapsed, d.cfg.MaxWall)
		}
		iter++

		// Sampling call with tools.
		resp, err := tc.ChatWithTools(ctx, messages, d.wireTls, choice)
		if err != nil {
			tr.EndAt = time.Now()
			tr.Wall = tr.EndAt.Sub(tr.StartAt)
			tr.Iterations = iter
			// v3.7.1: if the first sampling call comes back with
			// -32601, surface as a SENTINEL so the MCP caller can
			// downgrade to the self-driven scaffold path. Returning
			// a generic error here is what left Codex
			// (~/Downloads/gpt-5-codex/state.json) with empty
			// docs/specs/scripts in the 2026-06-29 run — the
			// probe cache had "codex:false" but Detect() returned
			// AgentUnknown inside the Codex MCP subprocess because
			// CODEX_HOME wasn't propagated.
			if llm.IsSamplingUnsupported(err) {
				return tr, fmt.Errorf("%w (mid-run at iter %d): %v", ErrHostSamplingUnsupported, iter, err)
			}
			return tr, fmt.Errorf("sampling at iter %d: %w", iter, err)
		}

		if len(resp.Choices) == 0 {
			tr.EndAt = time.Now()
			tr.Wall = tr.EndAt.Sub(tr.StartAt)
			tr.Iterations = iter
			return tr, fmt.Errorf("empty chat response at iter %d", iter)
		}
		choice0 := resp.Choices[0]
		text := choice0.Message.Content
		toolCalls := choice0.Message.ToolCalls

		// Append the assistant text (if any).
		if text != "" {
			tr.TextSoFar += text
			if _, err := io.WriteString(d.cfg.Out, text); err != nil {
				// Out failing is non-fatal; we keep going.
			}
		}
		// VERDICT/REVIEW short-circuit.
		if len(toolCalls) == 0 {
			if v := verdictRegex.FindString(text); v != "" {
				tr.Verdict = strings.TrimSpace(v)
				tr.VerdictLine = strings.Count(text[:strings.Index(text, v)], "\n") + 1
				tr.Iterations = iter
				tr.EndAt = time.Now()
				tr.Wall = tr.EndAt.Sub(tr.StartAt)
				return tr, nil
			}
			if r := reviewRegex.FindString(text); r != "" {
				tr.Verdict = strings.TrimSpace(r)
				tr.VerdictLine = strings.Count(text[:strings.Index(text, r)], "\n") + 1
				tr.Iterations = iter
				tr.EndAt = time.Now()
				tr.Wall = tr.EndAt.Sub(tr.StartAt)
				return tr, nil
			}
			// No tools AND no verdict — model is just chatting.
			if !sawAnyToolUse && iter >= 2 {
				// Two iterations of pure text from a host that we
				// were told implements tools is a sign the model
				// actually doesn't. Bail to caller for downgrading.
				tr.Iterations = iter
				tr.EndAt = time.Now()
				tr.Wall = tr.EndAt.Sub(tr.StartAt)
				return tr, fmt.Errorf("%w (after %d text-only iterations; text-so-far length=%d)",
					ErrBackendToolsUnsupported, iter, len(tr.TextSoFar))
			}
			// Otherwise prompt the model to act: append a short nudge.
			messages = append(messages, llm.Message{Role: "assistant", Content: text})
			messages = append(messages, llm.Message{Role: "user", Content: "Continue. When you have completed the work, end with VERDICT: APPROVED|REJECTED on its own line."})
			continue
		}

		// Tool execution phase.
		messages = append(messages, llm.Message{Role: "assistant", Content: text})

		// Build tool_result echoes. Anthropic convention: a single
		// user message with content = array of tool_result blocks.
		// OpenAI convention: one tool message per call with
		// tool_call_id. Both shapes are accepted via the
		// Anthropic-style payload here; the SamplingBackend renders
		// to whichever its host expects (we currently always render
		// Anthropic-style).
		results := make([]samplingToolResultShim, 0, len(toolCalls))
		for _, tc := range toolCalls {
			sawAnyToolUse = true
			rec := ToolRecord{
				Name:      tc.Name,
				ID:        tc.ID,
				Input:     tc.Input,
				StartedAt: time.Now(),
			}
			out, err := d.cfg.Registry.Call(ctx, tc.Name, tc.Input)
			rec.EndedAt = time.Now()
			if err != nil {
				rec.Err = err.Error()
				results = append(results, samplingToolResultShim{
					ToolUseID: tc.ID,
					Err:       err.Error(),
				})
				if _, werr := fmt.Fprintf(d.cfg.Out, "  • tool %s → error: %s\n", tc.Name, err); werr != nil {
					// ignore
				}
			} else {
				// Render the tool result for the model.
				jb, _ := json.Marshal(out)
				rec.Output = jb
				results = append(results, samplingToolResultShim{
					ToolUseID: tc.ID,
					Output:    jb,
				})
				if _, werr := fmt.Fprintf(d.cfg.Out, "  • tool %s → ok (%d bytes)\n", tc.Name, len(jb)); werr != nil {
					// ignore
				}
			}
			tr.ToolInvocations = append(tr.ToolInvocations, rec)
		}

		// Append a single user message carrying the tool_results.
		messages = appendToolResults(messages, results)
		tr.Iterations = iter
	}

	// Ran out of iterations without a VERDICT. Return what we have.
	tr.EndAt = time.Now()
	tr.Wall = tr.EndAt.Sub(tr.StartAt)
	return tr, fmt.Errorf("max_iter reached (%d) without VERDICT", d.cfg.MaxIter)
}

// samplingToolResultShim is the in-memory shape we use to carry
// tool_result echoes between the registry callback and the next
// sampling request. Rendered into a user message body by
// appendToolResults.
type samplingToolResultShim struct {
	ToolUseID string
	Output    json.RawMessage
	Err       string
}

// appendToolResults builds a single user message whose content
// carries an Anthropic-style tool_result block for each executed
// tool. The next ChatWithTools round-trip includes this message so
// the model sees the results before it decides the next step.
func appendToolResults(messages []llm.Message, results []samplingToolResultShim) []llm.Message {
	blocks := make([]map[string]any, 0, len(results))
	for _, r := range results {
		block := map[string]any{
			"type":        "tool_result",
			"tool_use_id": r.ToolUseID,
		}
		if r.Err != "" {
			block["content"] = r.Err
			block["is_error"] = true
		} else {
			block["content"] = string(r.Output)
		}
		blocks = append(blocks, block)
	}
	body, _ := json.Marshal(map[string]any{"blocks": blocks})

	// The driver runs in-process; the rpc hop to the host agent is
	// invisible to it. For now we pack the blocks into a single
	// human-readable string the model can parse. A future v3.7.x
	// can promote this to a real tool_result user-role message
	// once MCP spec finalises the host-side renderer.
	return append(messages, llm.Message{
		Role: "user",
		Content: "TOOL RESULTS:\n" + string(body),
	})
}
