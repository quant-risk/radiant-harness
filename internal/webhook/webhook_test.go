package webhook_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/webhook"
)

func TestSend_PostsJSON(t *testing.T) {
	var received webhook.Payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := webhook.Payload{
		Event: webhook.EventLoopDone,
		RunID: "run-abc",
		Data:  map[string]any{"cost_usd": 0.0042},
	}
	if err := webhook.Send(context.Background(), srv.URL, p); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received.Event != webhook.EventLoopDone {
		t.Errorf("expected event %q, got %q", webhook.EventLoopDone, received.Event)
	}
	if received.RunID != "run-abc" {
		t.Errorf("expected run_id 'run-abc', got %q", received.RunID)
	}
}

func TestSend_EmptyURL_NoOp(t *testing.T) {
	// Empty URL must return nil without panicking.
	if err := webhook.Send(context.Background(), "", webhook.Payload{Event: webhook.EventLoopDone}); err != nil {
		t.Fatalf("Send with empty URL should be a no-op, got: %v", err)
	}
}

func TestSend_ServerError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := webhook.Send(context.Background(), srv.URL, webhook.Payload{Event: webhook.EventFleetDone})
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestSend_Timeout_ReturnsError(t *testing.T) {
	// Server that hangs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(15 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := webhook.Send(ctx, srv.URL, webhook.Payload{Event: webhook.EventLoopFailed})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSend_TimestampAutoSet(t *testing.T) {
	var received webhook.Payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	before := time.Now().UTC().Add(-time.Second)
	_ = webhook.Send(context.Background(), srv.URL, webhook.Payload{Event: webhook.EventTaskDone, RunID: "r"})
	if received.Timestamp.Before(before) {
		t.Errorf("expected timestamp to be set automatically, got %v", received.Timestamp)
	}
}

func TestSend_TaskEvent(t *testing.T) {
	var received webhook.Payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := webhook.Payload{
		Event:  webhook.EventTaskDone,
		RunID:  "fleet-123",
		TaskID: "task-02",
		Data:   map[string]any{"evidence": "all tests pass"},
	}
	if err := webhook.Send(context.Background(), srv.URL, p); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received.TaskID != "task-02" {
		t.Errorf("expected task_id 'task-02', got %q", received.TaskID)
	}
}
