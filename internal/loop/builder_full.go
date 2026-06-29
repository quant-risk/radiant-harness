//go:build !light_only

package loop

import "github.com/quant-risk/radiant-harness/internal/llm"

// init registers the HTTP backend builder for the Full build. The
// Light build (//go:build light_only) does NOT have this file —
// internal/loop's httpBackendBuilder stays nil, and the loop package
// requires callers to supply cfg.Backend.
//
// This is the same wiring that cmd/radiant/helpers.go does via its
// own init() — but doing it here means internal/loop's tests don't
// need that init to have run. The cmd/radiant init() is still
// needed (and harmless: it's the same factory pointer).
func init() {
	SetHTTPBackendBuilder(func(m llm.Model) llm.Backend {
		return llm.NewHTTPBackend(m)
	})
}
