package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/quant-risk/radiant-harness/v3/internal/llm"
)

// PlannerClient is the minimal interface Planner needs from an LLM client.
// *llm.Client satisfies it.
type PlannerClient interface {
	Chat(ctx context.Context, messages []llm.Message) (*llm.ChatResponse, error)
}

// Plan decomposes goal into a slice of Tasks. When client is nil (or when the
// LLM call fails), it falls back to a deterministic 3-task skeleton so the
// fleet pipeline never stalls waiting for model access.
func Plan(ctx context.Context, goal string, client PlannerClient) ([]Task, error) {
	if client != nil {
		tasks, err := planWithLLM(ctx, goal, client)
		if err == nil && len(tasks) > 0 {
			return tasks, nil
		}
		// Fall through to heuristic on any LLM error.
	}
	return planHeuristic(goal), nil
}

// planHeuristic returns a 3-phase skeleton: research → implement → verify.
func planHeuristic(goal string) []Task {
	short := goal
	if len(short) > 60 {
		short = short[:57] + "..."
	}
	return []Task{
		{
			ID:       "task-01",
			Title:    fmt.Sprintf("Research: %s", short),
			DoneWhen: fmt.Sprintf("A clear implementation plan exists for: %s", goal),
			Status:   TaskPending,
		},
		{
			ID:       "task-02",
			Title:    fmt.Sprintf("Implement: %s", short),
			DoneWhen: fmt.Sprintf("All code changes are complete and tests pass for: %s", goal),
			Status:   TaskPending,
		},
		{
			ID:       "task-03",
			Title:    fmt.Sprintf("Verify: %s", short),
			DoneWhen: fmt.Sprintf("All tests pass and the implementation is reviewed for: %s", goal),
			Status:   TaskPending,
		},
	}
}

const planSystemPrompt = `You are a software project planner. Decompose the given goal into 2–6 focused, independently-executable tasks. Each task must be completable by a single autonomous coding agent.

Respond with a JSON array only — no markdown, no explanation:
[
  {"id": "task-01", "title": "...", "done_when": "..."},
  ...
]

Rules:
- "title" is a short imperative phrase (≤ 10 words)
- "done_when" is a clear, testable completion criterion (≤ 30 words)
- Tasks must be ordered: research → implement → verify
- IDs must be "task-01", "task-02", etc.`

type planTaskJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	DoneWhen string `json:"done_when"`
}

func planWithLLM(ctx context.Context, goal string, client PlannerClient) ([]Task, error) {
	messages := []llm.Message{
		{Role: "system", Content: planSystemPrompt},
		{Role: "user", Content: fmt.Sprintf("Goal: %s", goal)},
	}

	resp, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	// Strip markdown code fences if present.
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var raw []planTaskJSON
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse planner response: %w", err)
	}

	tasks := make([]Task, 0, len(raw))
	for _, r := range raw {
		if r.ID == "" || r.Title == "" || r.DoneWhen == "" {
			continue
		}
		tasks = append(tasks, Task{
			ID:       r.ID,
			Title:    r.Title,
			DoneWhen: r.DoneWhen,
			Status:   TaskPending,
		})
	}
	return tasks, nil
}
