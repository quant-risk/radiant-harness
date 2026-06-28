// Package webhook provides a fire-and-forget HTTP POST notification for
// loop and fleet events. The payload is a JSON object with a fixed envelope.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Event types posted to the webhook URL.
const (
	EventLoopDone    = "loop.done"
	EventLoopFailed  = "loop.failed"
	EventTaskDone    = "fleet.task.done"
	EventTaskFailed  = "fleet.task.failed"
	EventFleetDone   = "fleet.done"
)

// Payload is the JSON body sent to the webhook URL.
type Payload struct {
	Event     string         `json:"event"`
	RunID     string         `json:"run_id"`
	TaskID    string         `json:"task_id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// Send posts payload to url as JSON. It respects ctx for cancellation and
// applies a 10-second timeout on top of any deadline already in ctx.
// Errors are returned but callers may choose to ignore them (fire-and-forget).
func Send(ctx context.Context, url string, p Payload) error {
	if url == "" {
		return nil
	}
	if p.Timestamp.IsZero() {
		p.Timestamp = time.Now().UTC()
	}
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("webhook marshal: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "radiant-harness/webhook")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: server returned %d", resp.StatusCode)
	}
	return nil
}
