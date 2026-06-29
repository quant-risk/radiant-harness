package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name: "test_tool",
		Params: []Param{{Name: "x", Type: "string"}},
		Invoke: func(ctx context.Context, args json.RawMessage) (any, error) {
			return "ok", nil
		},
	})
	got := r.Get("test_tool")
	if got == nil {
		t.Fatal("Get returned nil after Register")
	}
	if got.Name != "test_tool" {
		t.Errorf("name: got %q want test_tool", got.Name)
	}
	if r.Get("nonexistent") != nil {
		t.Errorf("Get should return nil for unknown tool")
	}
}

func TestRegistry_Names(t *testing.T) {
	r := Default()
	names := r.Names()
	want := map[string]bool{
		"read_file":   false,
		"write_file":  false,
		"search_code": false,
		"run_gate":    false,
	}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, found := range want {
		if !found {
			t.Errorf("expected tool %q in default registry", n)
		}
	}
}

func TestRegistry_Call_Unknown(t *testing.T) {
	r := Default()
	_, err := r.Call(context.Background(), "nope", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error should mention unknown tool: %v", err)
	}
}

func TestRegistry_Call_Stub(t *testing.T) {
	r := Default()
	for _, name := range r.Names() {
		_, err := r.Call(context.Background(), name, json.RawMessage(`{}`))
		if err == nil {
			t.Errorf("expected stub to error for %q", name)
		}
		if !strings.Contains(err.Error(), "not yet wired") {
			t.Errorf("error for %q should mention 'not yet wired', got: %v", name, err)
		}
	}
}

func TestRegistry_RegisterPanicsOnDuplicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on duplicate Register")
		}
	}()
	r := NewRegistry()
	tool := &Tool{Name: "dup"}
	r.Register(tool)
	r.Register(tool) // should panic
}

func TestRegistry_Concurrent(t *testing.T) {
	r := Default()
	// Run many concurrent reads to verify the RWMutex works.
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			_ = r.Names()
			_ = r.Get("read_file")
			done <- true
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}