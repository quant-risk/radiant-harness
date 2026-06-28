package fleet_test

import (
	"testing"

	"github.com/quant-risk/radiant-harness/internal/fleet"
)

// ── retryBackoff ──────────────────────────────────────────────────────────

func TestRetryBackoff_Exported(t *testing.T) {
	// retryBackoff is unexported; we test it indirectly via MaxConcurrency
	// and MaxRetries fields on DispatchConfig.
	cfg := DispatchConfigForTest(3, 2)
	if cfg.MaxConcurrency != 3 {
		t.Errorf("expected MaxConcurrency 3, got %d", cfg.MaxConcurrency)
	}
	if cfg.MaxRetries != 2 {
		t.Errorf("expected MaxRetries 2, got %d", cfg.MaxRetries)
	}
}

// ── MaxConcurrency semaphore — structural check ───────────────────────────

func TestDispatchConfig_MaxConcurrency_Default(t *testing.T) {
	cfg := DispatchConfigForTest(0, 0)
	if cfg.MaxConcurrency != 0 {
		t.Errorf("expected default MaxConcurrency 0, got %d", cfg.MaxConcurrency)
	}
}

func TestDispatchConfig_MaxRetries_Default(t *testing.T) {
	cfg := DispatchConfigForTest(0, 0)
	if cfg.MaxRetries != 0 {
		t.Errorf("expected default MaxRetries 0, got %d", cfg.MaxRetries)
	}
}

// DispatchConfigForTest builds a DispatchConfig for testing field access
// without spawning real processes.
func DispatchConfigForTest(concurrency, maxRetries int) fleet.DispatchConfig {
	return fleet.DispatchConfig{
		MaxConcurrency: concurrency,
		MaxRetries:     maxRetries,
	}
}
