package fleet_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/quant-risk/radiant-harness/internal/fleet"
	"github.com/quant-risk/radiant-harness/internal/llm"
)

// ── planHeuristic (nil client) ─────────────────────────────────────────────

func TestPlan_NilClient_Returns3Tasks(t *testing.T) {
	tasks, err := fleet.Plan(context.Background(), "build a rate limiter", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestPlan_NilClient_AllTasksPending(t *testing.T) {
	tasks, _ := fleet.Plan(context.Background(), "add auth middleware", nil)
	for _, t2 := range tasks {
		if t2.Status != fleet.TaskPending {
			t.Errorf("task %q should be pending, got %q", t2.ID, t2.Status)
		}
	}
}

func TestPlan_NilClient_IDsSequential(t *testing.T) {
	tasks, _ := fleet.Plan(context.Background(), "refactor database layer", nil)
	ids := []string{"task-01", "task-02", "task-03"}
	for i, want := range ids {
		if tasks[i].ID != want {
			t.Errorf("task[%d].ID = %q, want %q", i, tasks[i].ID, want)
		}
	}
}

func TestPlan_NilClient_TitleContainsGoal(t *testing.T) {
	tasks, _ := fleet.Plan(context.Background(), "implement caching", nil)
	found := false
	for _, task := range tasks {
		if contains(task.Title, "implement caching") || contains(task.DoneWhen, "implement caching") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("goal not reflected in any task title or done_when")
	}
}

func TestPlan_NilClient_LongGoalTruncatedInTitle(t *testing.T) {
	longGoal := "build a distributed rate limiter with Redis backend that supports sliding window and fixed window algorithms"
	tasks, _ := fleet.Plan(context.Background(), longGoal, nil)
	for _, task := range tasks {
		if len(task.Title) > 120 {
			t.Errorf("task title too long (%d chars): %q", len(task.Title), task.Title)
		}
	}
}

func TestPlan_NilClient_DoneWhenNonEmpty(t *testing.T) {
	tasks, _ := fleet.Plan(context.Background(), "write API docs", nil)
	for _, task := range tasks {
		if task.DoneWhen == "" {
			t.Errorf("task %q has empty DoneWhen", task.ID)
		}
	}
}

// ── LLM client — fail-open behaviour ──────────────────────────────────────

// failingClient always returns an error to test the fallback path.
type failingClient struct{}

func (f *failingClient) Chat(_ context.Context, _ []llm.Message) (*llm.ChatResponse, error) {
	return nil, fmt.Errorf("no API key")
}

func TestPlan_LLMError_FallsBackToHeuristic(t *testing.T) {
	tasks, err := fleet.Plan(context.Background(), "deploy to production", &failingClient{})
	if err != nil {
		t.Fatalf("Plan should not surface LLM errors: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 heuristic tasks on LLM failure, got %d", len(tasks))
	}
}

// ── LLM client — success path ─────────────────────────────────────────────

// stubClient returns a fixed JSON payload.
type stubClient struct{ json string }

func (s *stubClient) Chat(_ context.Context, _ []llm.Message) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Choices: []llm.ChatResponseChoice{
			{Message: llm.ChatResponseMessage{Role: "assistant", Content: s.json}},
		},
	}, nil
}

func TestPlan_LLMSuccess_ParsesTasks(t *testing.T) {
	payload := `[
		{"id":"task-01","title":"Research options","done_when":"Research doc written"},
		{"id":"task-02","title":"Implement feature","done_when":"All tests pass"}
	]`
	tasks, err := fleet.Plan(context.Background(), "add search", &stubClient{json: payload})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Title != "Research options" {
		t.Errorf("task[0].Title = %q", tasks[0].Title)
	}
	if tasks[1].DoneWhen != "All tests pass" {
		t.Errorf("task[1].DoneWhen = %q", tasks[1].DoneWhen)
	}
}

func TestPlan_LLMSuccess_AllTasksPending(t *testing.T) {
	payload := `[{"id":"task-01","title":"Do X","done_when":"X done"}]`
	tasks, _ := fleet.Plan(context.Background(), "do X", &stubClient{json: payload})
	for _, task := range tasks {
		if task.Status != fleet.TaskPending {
			t.Errorf("LLM task should be pending, got %q", task.Status)
		}
	}
}

func TestPlan_LLMSuccess_MarkdownFencesStripped(t *testing.T) {
	payload := "```json\n[{\"id\":\"task-01\",\"title\":\"T\",\"done_when\":\"D\"}]\n```"
	tasks, err := fleet.Plan(context.Background(), "goal", &stubClient{json: payload})
	if err != nil {
		t.Fatalf("should strip markdown fences: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}

func TestPlan_LLMSuccess_SkipsIncompleteEntries(t *testing.T) {
	// Missing done_when → should be skipped.
	payload := `[
		{"id":"task-01","title":"Good task","done_when":"criteria"},
		{"id":"task-02","title":"Bad task"}
	]`
	tasks, err := fleet.Plan(context.Background(), "goal", &stubClient{json: payload})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 valid task, got %d", len(tasks))
	}
	if tasks[0].ID != "task-01" {
		t.Errorf("expected task-01, got %q", tasks[0].ID)
	}
}
