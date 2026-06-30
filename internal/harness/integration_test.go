package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	radiant "github.com/quant-risk/radiant-harness/v3/internal"
	"github.com/quant-risk/radiant-harness/v3/internal/quality"
)

// ── Integration Tests (simulate real-world usage) ──

// TestIntegrationFullPipeline tests the complete SDD pipeline end-to-end.
func TestIntegrationFullPipeline(t *testing.T) {
	dir := t.TempDir()

	// Create a realistic feature
	specDir := filepath.Join(dir, "specs", "0001-collect-feedback")
	os.MkdirAll(specDir, 0o755)

	specContent := `---
name: collect-feedback
description: Collect user feedback widget
alwaysApply: true
---

# Spec — Collect Feedback

## Summary
The widget sends feedback (text + context) and the system stores it.

## Acceptance criteria

### AC-1: valid feedback is accepted
- **Given** non-empty text and a context
- **When** the widget sends
- **Then** the feedback is stored and returns an id

### AC-2: empty feedback is rejected
- **Given** empty text (or only spaces)
- **When** sending
- **Then** returns validation error and does NOT store

### AC-3: oversized feedback is rejected
- **Given** text with more than 1000 characters
- **When** sending
- **Then** returns validation error

## Out of scope
- Moderation / anti-spam.
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	taskContent := `| # | Task | Covers AC | Depends on | Gate | Status |
|---|------|-----------|------------|------|--------|
| 1 | Validate input | AC-2, AC-3 | — | echo "gate-pass" | todo |
| 2 | Store and return id | AC-1 | 1 | echo "gate-pass" | todo |
`
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	// Run the orchestrator in dry-run mode
	orch := NewWithNoDetect(dir, 3)
	result, err := orch.Run(context.Background(), specDir)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// Verify results
	if !result.Succeeded {
		t.Error("pipeline should succeed in dry-run mode")
	}

	// Verify state was saved
	state := NewState(dir)
	if state.CurrentState() != radiant.StateDone {
		t.Errorf("expected done state, got %s", state.CurrentState())
	}

	// Verify progress file exists
	progressFile := filepath.Join(dir, ".radiant-harness", "progress.json")
	if _, err := os.Stat(progressFile); os.IsNotExist(err) {
		t.Error("progress.json should exist")
	}
}

// TestIntegrationMultiTaskParallel tests parallel task execution.
func TestIntegrationMultiTaskParallel(t *testing.T) {
	dir := t.TempDir()

	specDir := filepath.Join(dir, "specs", "0002-parallel")
	os.MkdirAll(specDir, 0o755)

	specContent := `---
name: parallel-tasks
alwaysApply: true
---

### AC-1: task A
- **Given** X
- **When** Y
- **Then** Z

### AC-2: task B
- **Given** A
- **When** B
- **Then** C

### AC-3: task C
- **Given** M
- **When** N
- **Then** O
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	// Tasks with parallelism marker [P]
	taskContent := `| # | Task | AC | Dep | Gate | Status |
|---|------|----|-----|------|--------|
| 1 | Setup | AC-1 | — | echo ok | todo |
| 2 | Part A [P] | AC-2 | 1 | echo ok | todo |
| 3 | Part B [P] | AC-3 | 1 | echo ok | todo |
`
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	orch := NewWithNoDetect(dir, 0)
	result, err := orch.Run(context.Background(), specDir)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	if !result.Succeeded {
		t.Error("parallel pipeline should succeed")
	}

	if result.Attempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", result.Attempts)
	}
}

// TestIntegrationSpecParsing tests spec parsing with real-world content.
func TestIntegrationSpecParsing(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-test")
	os.MkdirAll(specDir, 0o755)

	specContent := `---
name: stripe-integration
description: Payment gateway integration with Stripe
alwaysApply: true
---

# Spec — Stripe Integration

## Summary
Integrate Stripe as payment gateway for subscription management.

## Acceptance criteria

### AC-1: create Stripe customer
- **Given** a new user signs up
- **When** they start a free trial
- **Then** a Stripe customer is created with metadata

### AC-2: handle webhook events
- **Given** Stripe sends a webhook
- **When** the event type is invoice.payment_succeeded
- **Then** the subscription status is updated to active

### AC-3: failed payment retry
- **Given** a payment fails
- **When** Stripe retries after 3 days
- **Then** the user is notified and subscription stays in grace period

## Out of scope
- Custom payment UI (use Stripe Checkout)
- Multi-currency support
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	taskContent := `| # | Task | AC | Dep | Gate | Status |
|---|------|----|-----|------|--------|
| 1 | Stripe SDK setup | — | — | echo "build-ok" | todo |
| 2 | Customer creation | AC-1 | 1 | echo "test-ok" | todo |
| 3 | Webhook handler | AC-2 | 1 | echo "test-ok" | todo |
| 4 | Payment retry logic | AC-3 | 2,3 | echo "test-ok" | todo |
`
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	orch := NewWithNoDetect(dir, 0)
	result, err := orch.Run(context.Background(), specDir)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	if !result.Succeeded {
		t.Error("Stripe integration pipeline should succeed")
	}
}

// TestIntegrationValidationWithRealSpec tests validation against a real spec.
func TestIntegrationValidationWithRealSpec(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-real")
	os.MkdirAll(specDir, 0o755)

	specContent := `---
name: real-feature
alwaysApply: true
---

### AC-1: user login
- **Given** valid credentials
- **When** user submits login form
- **Then** JWT token is returned

### AC-2: invalid credentials
- **Given** wrong password
- **When** user submits login form
- **Then** 401 error is returned
`
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	taskContent := `| # | Task | AC | Dep | Gate | Status |
|---|------|----|-----|------|--------|
| 1 | Auth endpoint | AC-1, AC-2 | — | echo ok | todo |
`
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	// Use quality.quality.ValidateFeature
	result := quality.ValidateFeature(specDir)

	if !result.Passed {
		t.Errorf("validation should pass: %v", result.Errors)
	}

	if result.Score != 1.0 {
		t.Errorf("expected score 1.0, got %f", result.Score)
	}
}

// TestIntegrationStatePersistence tests that state survives across runs.
func TestIntegrationStatePersistence(t *testing.T) {
	dir := t.TempDir()

	// First run
	specDir := filepath.Join(dir, "specs", "0001-persist")
	os.MkdirAll(specDir, 0o755)

	specContent := "---\nname: test\nalwaysApply: true\n---\n\n### AC-1: test\n- **Given** X\n- **When** Y\n- **Then** Z\n"
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	taskContent := "| # | Task | AC | Dep | Gate | Status |\n|---|------|----|-----|------|--------|\n| 1 | Test | AC-1 | — | echo ok | todo |\n"
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	orch1 := NewWithNoDetect(dir, 0)
	result1, _ := orch1.Run(context.Background(), specDir)
	if !result1.Succeeded {
		t.Fatal("first run should succeed")
	}

	// Second run — state should persist
	orch2 := NewWithNoDetect(dir, 0)
	if orch2.State.CurrentState() != radiant.StateDone {
		t.Errorf("state should be done from first run, got %s", orch2.State.CurrentState())
	}
}

// TestIntegrationQualityScripts tests all quality scripts together.
func TestIntegrationQualityScripts(t *testing.T) {
	dir := t.TempDir()

	// Create a valid project structure
	os.MkdirAll(filepath.Join(dir, "specs", "0001-test"), 0o755)

	// Valid doc with frontmatter
	validDoc := "---\nname: test\ndescription: Test\nalwaysApply: false\n---\nBody"
	os.WriteFile(filepath.Join(dir, "test.md"), []byte(validDoc), 0o644)

	// Spec with ACs
	specContent := "---\nname: spec\ndescription: test\nalwaysApply: true\n---\n\n### AC-1: test\nGiven X\nWhen Y\nThen Z"
	os.WriteFile(filepath.Join(dir, "specs", "0001-test", "spec.md"), []byte(specContent), 0o644)

	taskContent := "---\nname: tasks\ndescription: test\nalwaysApply: false\n---\n\n| 1 | Task | AC-1 | — | echo ok | todo |"
	os.WriteFile(filepath.Join(dir, "specs", "0001-test", "tasks.md"), []byte(taskContent), 0o644)

	// Run all quality scripts
	audit := quality.AuditPipeline(dir)
	if !audit.OK {
		t.Errorf("audit failed: %v", audit.Errors)
	}

	fidelity := quality.EvalSpecFidelity(dir)
	if !fidelity.OK {
		t.Errorf("fidelity failed: %v", fidelity.Errors)
	}

	mermaid := quality.ValidateMermaid(dir)
	if !mermaid.OK {
		t.Errorf("mermaid failed: %v", mermaid.Errors)
	}
}

// TestIntegrationProtocolRegistry tests the protocol registry.
func TestIntegrationProtocolRegistry(t *testing.T) {
	// List all protocols
	protocols := ListProtocols()
	if len(protocols) < 6 {
		t.Errorf("expected at least 6 protocols, got %d", len(protocols))
	}

	// Get specific protocols
	claude := GetProtocol("claude")
	if claude.Name() != "claude" {
		t.Errorf("expected claude, got %s", claude.Name())
	}

	copilot := GetProtocol("copilot")
	if copilot.Name() != "copilot" {
		t.Errorf("expected copilot, got %s", copilot.Name())
	}

	gemini := GetProtocol("gemini")
	if gemini.Name() != "gemini" {
		t.Errorf("expected gemini, got %s", gemini.Name())
	}

	// Unknown protocol should return generic
	unknown := GetProtocol("unknown-agent")
	if unknown.Name() != "unknown-agent" {
		t.Errorf("expected unknown-agent, got %s", unknown.Name())
	}
}

// TestIntegrationCapabilities tests capability detection.
func TestIntegrationCapabilities(t *testing.T) {
	claude := DetectCapabilities("claude")
	if !claude.SupportsStreaming {
		t.Error("claude should support streaming")
	}
	if !claude.SupportsMCP {
		t.Error("claude should support MCP")
	}
	if claude.MaxContextTokens < 100000 {
		t.Error("claude should have large context window")
	}

	cursor := DetectCapabilities("cursor")
	if !cursor.SupportsStreaming {
		t.Error("cursor should support streaming")
	}

	unknown := DetectCapabilities("unknown")
	if unknown.MaxContextTokens > 50000 {
		t.Error("unknown agent should have small context window")
	}
}

// TestIntegrationLoggerWithHooks tests logger with multiple hooks.
func TestIntegrationLoggerWithHooks(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, metrics, err := NewMultiLogger(LevelInfo, logFile, "")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Log some events
	logger.Info("test event", "key", "value")
	logger.Warn("warning event", "count", 42)
	logger.Error("error event", "err", "something broke")

	// Check metrics
	if metrics.Counts["info"] != 1 {
		t.Errorf("expected 1 info, got %d", metrics.Counts["info"])
	}
	if metrics.Counts["warn"] != 1 {
		t.Errorf("expected 1 warn, got %d", metrics.Counts["warn"])
	}
	if metrics.Counts["error"] != 1 {
		t.Errorf("expected 1 error, got %d", metrics.Counts["error"])
	}

	// Check file was written
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file should exist")
	}
}

// TestIntegrationTokenEstimationRealWorld tests token estimation on real content.
func TestIntegrationTokenEstimationRealWorld(t *testing.T) {
	te := NewTokenEstimator()

	// Real spec content
	specContent := `---
name: stripe-integration
description: Payment gateway integration
alwaysApply: true
---

# Spec — Stripe Integration

## Summary
Integrate Stripe as payment gateway for subscription management.

### AC-1: create Stripe customer
- **Given** a new user signs up
- **When** they start a free trial
- **Then** a Stripe customer is created with metadata
`

	tokens := te.EstimateTokens(specContent)
	if tokens < 50 || tokens > 200 {
		t.Errorf("spec content tokens should be 50-200, got %d", tokens)
	}

	// Code content
	codeContent := `func (s *StripeService) CreateCustomer(ctx context.Context, user *User) (*Customer, error) {
	customer, err := s.client.Customers.New(&stripe.CustomerParams{
		Email: stripe.String(user.Email),
		Metadata: map[string]string{
			"user_id": user.ID.String(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create stripe customer: %w", err)
	}
	return &Customer{ID: customer.ID, Email: customer.Email}, nil
}`

	codeTokens := te.EstimateTokens(codeContent)
	if codeTokens < 80 || codeTokens > 300 {
		t.Errorf("code content tokens should be 80-300, got %d", codeTokens)
	}
}
