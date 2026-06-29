package possess

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quant-risk/radiant-harness/internal/llm"
	"github.com/quant-risk/radiant-harness/internal/tools"
)

// scriptedBackend is a hand-rolled ChatWithTools implementation that
// walks through a list of scripted responses. Each response is
// returned once per ChatWithTools call; the last item is repeated
// for any further call. Lets us assert that Drive consumes the
// scripted turns in the correct order.
//
// Every successful turn can be either pure text (model answered
// without tool_use) or a tool_use block plus optional preceding
// text. The driver MUST execute the tool_use via the Registry and
// feed tool_result echoes back to the next turn.
type scriptedBackend struct {
	turns []scriptedTurn
	i     atomic.Int32

	calls atomic.Int32
}

type scriptedTurn struct {
	// Text precedes the tool_use blocks. Empty for tool-only turns.
	Text string
	// ToolUses is the model's request — each one drives one
	// Registry.Call per the driver contract.
	ToolUses []scriptedToolUse
	// DoneAfter marks this turn as "VERDICT surfaced; driver
	// should treat this as a successful end-of-run instead of
	// looping more turns". Only meaningful on text-only turns.
	DoneAfter bool
}

type scriptedToolUse struct {
	ID    string
	Name  string
	Input json.RawMessage
}

func (b *scriptedBackend) Chat(ctx context.Context, messages []llm.Message) (*llm.ChatResponse, error) {
	// Required by llm.Backend; identical semantics to ChatWithTools
	// in the scripted harness.
	return b.ChatWithTools(ctx, messages, nil, nil)
}

func (b *scriptedBackend) ChatStream(ctx context.Context, messages []llm.Message, cb llm.StreamCallback) (*llm.ChatResponse, error) {
	return b.ChatWithTools(ctx, messages, nil, nil)
}

func (b *scriptedBackend) ChatWithTools(ctx context.Context, messages []llm.Message, _ []llm.Tool, _ *llm.ToolChoice) (*llm.ChatResponse, error) {
	b.calls.Add(1)
	idx := int(b.i.Add(1) - 1)
	if idx >= len(b.turns) {
		idx = len(b.turns) - 1
	}
	turn := b.turns[idx]
	resp := &llm.ChatResponse{}
	resp.Choices = append(resp.Choices, llm.ChatResponseChoice{})
	c := &resp.Choices[0]
	c.Message.Role = "assistant"
	c.Message.Content = turn.Text
	for _, tu := range turn.ToolUses {
		c.Message.ToolCalls = append(c.Message.ToolCalls, llm.ToolCall{
			ID:    tu.ID,
			Name:  tu.Name,
			Input: tu.Input,
		})
	}
	if len(turn.ToolUses) > 0 {
		c.FinishReason = "tool_use"
	} else {
		c.FinishReason = "stop"
	}
	return resp, nil
}

func (b *scriptedBackend) ModelID() string { return "scripted-test" }

// Compile-time assertions.
var _ llm.Backend = (*scriptedBackend)(nil)
var _ llm.ToolCapable = (*scriptedBackend)(nil)

// TestDriverRunsToolsAndStopsOnVerdict walks a 4-step happy path:
//
//	turn 1: list_dir (project fingerprint) → tool_use
//	turn 2: read_file (CONTEXT.md)         → tool_use
//	turn 3: write_file (spec.md)           → tool_use
//	turn 4: text + VERDICT: APPROVED       → done
//
// The driver should execute every tool_use through the registry,
// emit one user message per tool_result, and stop at the VERDICT.
func TestDriverRunsToolsAndStopsOnVerdict(t *testing.T) {
	dir := t.TempDir()

	reg := tools.NewRegistry()
	// Just two tools for the test: a fake list_dir + read_file +
	// write_file. Using the tools.Tool signature so the registry is
	// usable.
	reg.Register(&tools.Tool{
		Name:        "list_dir",
		Description: "List entries at a project-relative path.",
		Params:      []tools.Param{{Name: "path", Type: "string", Required: false}},
		Invoke: func(ctx context.Context, args json.RawMessage) (any, error) {
			return []string{"README.md", "scripts/"}, nil
		},
	})
	reg.Register(&tools.Tool{
		Name:        "read_file",
		Description: "Read a project-relative file.",
		Params:      []tools.Param{{Name: "path", Type: "string", Required: true}},
		Invoke: func(ctx context.Context, args json.RawMessage) (any, error) {
			return "test content", nil
		},
	})
	reg.Register(&tools.Tool{
		Name:        "write_file",
		Description: "Write content to a file.",
		Params: []tools.Param{
			{Name: "path", Type: "string", Required: true},
			{Name: "content", Type: "string", Required: true},
		},
		Invoke: func(ctx context.Context, args json.RawMessage) (any, error) {
			var a struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			_ = json.Unmarshal(args, &a)
			if a.Path == "" {
				return nil, errors.New("path required")
			}
			return "ok", nil
		},
	})

	backend := &scriptedBackend{
		turns: []scriptedTurn{
			{
				Text: "Let me inspect the project first.",
				ToolUses: []scriptedToolUse{
					{ID: "toolu_1", Name: "list_dir", Input: json.RawMessage(`{"path":"."}`)},
				},
			},
			{
				Text: "Now I will read the context.",
				ToolUses: []scriptedToolUse{
					{ID: "toolu_2", Name: "read_file", Input: json.RawMessage(`{"path":"README.md"}`)},
				},
			},
			{
				Text: "Writing the spec.",
				ToolUses: []scriptedToolUse{
					{ID: "toolu_3", Name: "write_file", Input: json.RawMessage(`{"path":"specs/0001-foo/spec.md","content":"# spec\nAC1 stub"}`)},
				},
			},
			{
				Text: "All done.\n\nVERDICT: APPROVED\nSCORE: 0.95\nEVIDENCE: spec written and gates not requested\nESCALATE: false\nISSUES:\n-",
			},
		},
	}

	d, err := NewDriver(DriverConfig{
		Backend:     backend,
		ProjectRoot: dir,
		Registry:    reg,
		Out:         nil,
		MaxIter:     8,
		MaxWall:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewDriver: %v", err)
	}

	tr, err := d.Drive(context.Background(),
		"system prompt here",
		"do a thing")
	if err != nil {
		t.Fatalf("Drive returned err: %v", err)
	}
	if tr.Verdict != "VERDICT: APPROVED" {
		t.Fatalf("Verdict = %q, want %q", tr.Verdict, "VERDICT: APPROVED")
	}
	if tr.Iterations != 4 {
		t.Fatalf("Iterations = %d, want 4 (one per scripted turn)", tr.Iterations)
	}
	if got := len(tr.ToolInvocations); got != 3 {
		t.Fatalf("ToolInvocations len = %d, want 3 (one tool per non-terminal turn)", got)
	}
	if backend.calls.Load() != 4 {
		t.Fatalf("backend.calls = %d, want 4 (driver must re-sample after every tool_result)",
			backend.calls.Load())
	}
	// Recorded tools preserve names + IDs from the wire.
	wantNames := []string{"list_dir", "read_file", "write_file"}
	for i, name := range wantNames {
		if tr.ToolInvocations[i].Name != name {
			t.Fatalf("ToolInvocations[%d].Name = %q, want %q",
				i, tr.ToolInvocations[i].Name, name)
		}
		if tr.ToolInvocations[i].ID != "toolu_"+string(rune('1'+i)) {
			t.Fatalf("ToolInvocations[%d].ID = %q, want toolu_%c",
				i, tr.ToolInvocations[i].ID, '1'+i)
		}
	}
}

// TestDriverFallsBackWhenModelNeverCallsTools covers the
// ErrBackendToolsUnsupported path: two text-only turns in a row
// from a host that claimed ToolCapable — the driver bails and lets
// the caller downgrade.
func TestDriverFallsBackWhenModelNeverCallsTools(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&tools.Tool{
		Name: "noop",
		Params: []tools.Param{{Name: "x", Type: "string"}},
		Invoke: func(ctx context.Context, args json.RawMessage) (any, error) {
			return nil, nil
		},
	})
	backend := &scriptedBackend{
		turns: []scriptedTurn{
			{Text: "thinking out loud…"},
			{Text: "still thinking…"},
		},
	}

	d, err := NewDriver(DriverConfig{
		Backend:     backend,
		ProjectRoot: t.TempDir(),
		Registry:    reg,
		MaxIter:     5,
		MaxWall:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewDriver: %v", err)
	}

	_, err = d.Drive(context.Background(), "system", "task")
	if !errors.Is(err, ErrBackendToolsUnsupported) {
		t.Fatalf("Drive err = %v, want %v", err, ErrBackendToolsUnsupported)
	}
}

// TestNewDriverRejectsNonToolableBackend enforces the loud-failure
// contract: a Backend that doesn't implement ToolCapable must NOT
// fall back silently to text-only. Better to fail at construction
// than to lose the capability and have the user find out later.
func TestNewDriverRejectsNonToolableBackend(t *testing.T) {
	nonToolable := &nonToolableBackend{}
	_, err := NewDriver(DriverConfig{
		Backend:     nonToolable,
		ProjectRoot: t.TempDir(),
		Registry:    tools.NewRegistry(),
	})
	if err == nil {
		t.Fatal("NewDriver accepted a non-ToolCapable backend; expected loud failure")
	}
	if !strings.Contains(err.Error(), "ToolCapable") {
		t.Fatalf("error must mention ToolCapable to aid debugging, got: %v", err)
	}
}

// nonToolableBackend implements llm.Backend only — no ChatWithTools.
// NewDriver must reject it.
type nonToolableBackend struct{}

func (nonToolableBackend) Chat(ctx context.Context, m []llm.Message) (*llm.ChatResponse, error) {
	return nil, errors.New("nope")
}
func (nonToolableBackend) ChatStream(ctx context.Context, m []llm.Message, cb llm.StreamCallback) (*llm.ChatResponse, error) {
	return nil, errors.New("nope")
}
func (nonToolableBackend) ModelID() string { return "non-toolable" }

// Compile-time assertion: only llm.Backend, not ToolCapable.
var _ llm.Backend = nonToolableBackend{}
