//go:build light_only

package loop

// Light build stub: no HTTP backend registered. Callers MUST supply
// cfg.Backend explicitly (SamplingBackend or another). The HTTP
// fallback path in resolveBackends returns nil backends if
// httpBackendBuilder is nil, which surfaces as a panic at the first
// LLM call rather than silently using HTTP.

// (no init needed)
