//go:build with_full

package llm

import (
	"testing"
)

// Compile-time interface conformance checks — ensures both backends satisfy
// the Backend interface at build time, catching signature drift early.

func TestHTTPBackend_ImplementsBackend(t *testing.T) {
	var _ Backend = (*HTTPBackend)(nil)
}

func TestSamplingBackend_ImplementsBackend(t *testing.T) {
	var _ Backend = (*SamplingBackend)(nil)
}

func TestNewHTTPBackend_PreservesModel(t *testing.T) {
	m := Model{Model: "test-model-42", Provider: ProviderOpenAI}
	b := NewHTTPBackend(m)
	if b.ModelID() != "test-model-42" {
		t.Errorf("ModelID() = %q, want %q", b.ModelID(), "test-model-42")
	}
}

func TestSamplingBackend_ModelID_Default(t *testing.T) {
	sb := NewSamplingBackend(SamplingOptions{})
	if sb.ModelID() != "mcp-sampling" {
		t.Errorf("default ModelID = %q, want %q", sb.ModelID(), "mcp-sampling")
	}
}

func TestSamplingBackend_ModelID_Hint(t *testing.T) {
	sb := NewSamplingBackend(SamplingOptions{ModelHint: "claude-sonnet-4-6"})
	if sb.ModelID() != "claude-sonnet-4-6" {
		t.Errorf("hinted ModelID = %q, want %q", sb.ModelID(), "claude-sonnet-4-6")
	}
}
