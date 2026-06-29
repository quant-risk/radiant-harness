// Package casetest drives a real `radiant mcp serve` subprocess against a
// synthetic LLM host and a real case (a .zip file or directory), capturing
// every JSON-RPC round-trip, simulating realistic sampling latency, and
// rendering a structured Markdown report.
//
// The package exists to reproduce the failure mode observed in the wild:
// a host agent (Hermes mimo / Codex GPT-5 / Claude Code) takes 20–40 s
// cold-start per sampling call, the harness loop must finish in fewer than
// the host's outer tool-call timeout (typically 300 s), and the per-phase
// results must be visible in a form a human can audit. Earlier in the
// project we validated the harness only against a synthetic responder with
// zero latency; that hid the failure modes.
//
// The driver is small and explicit: spawn `radiant mcp serve`, send the
// JSON-RPC discovery sequence, read sampling/createMessage requests, wait
// the configured cold-start delay, and reply with a phase-correct canned
// response. No third-party deps. No network egress. No LLMs involved.
package casetest

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Case describes a real-world test case for the harness. It can come
// from a .zip file (typical: a "case pack" someone hands you) or a
// directory (typical: an existing project you want to drive).
type Case struct {
	Path       string // filesystem path to the unpacked case
	Name       string // slug derived from the original .zip or directory
	UserPrompt string // the verbatim task the harness will receive
	TaskID     string // 16-char hash used to look up state in .radiant-harness/
}

// LoadFromDir treats the directory at `dir` as a case root. The user
// prompt is read from the first existing file in promptOrder.
func LoadFromDir(dir string) (*Case, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("case dir: %w", err)
	}
	name := filepath.Base(dir)
	prompt, err := readFirstPrompt(dir)
	if err != nil {
		return nil, err
	}
	c := &Case{
		Path:       dir,
		Name:       slugify(name),
		UserPrompt: prompt,
	}
	c.TaskID = computeTaskID(c.Path, c.UserPrompt)
	return c, nil
}

// computeTaskID returns the same hex prefix the harness uses to look up
// possession state on disk (see cmd/radiant/cmd_mcp_possess.go::taskID).
// We duplicate the hash here to keep `internal/casetest` independent of
// `cmd/radiant`; the two implementations are tested to stay aligned.
//
// Format:
//   SHA-256(workdir || 0x00 || task)   →  hex, first 16 chars
func computeTaskID(path, prompt string) string {
	h := sha256.New()
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte(prompt))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// LoadFromZip extracts `path` into a unique temp directory and treats it
// as a case root.
func LoadFromZip(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	out, err := os.MkdirTemp("", "radiant-case-*")
	if err != nil {
		return "", err
	}

	for _, f := range r.File {
		// Guard against zip-slip: refuse entries with absolute paths or
		// ".." components after joining onto `out`.
		dest := filepath.Join(out, f.Name)
		if !strings.HasPrefix(filepath.Clean(dest), out+string(os.PathSeparator)) && filepath.Clean(dest) != out {
			return "", fmt.Errorf("zip slip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(dest, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", err
		}
		w, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", err
		}
		rc, err := f.Open()
		if err != nil {
			w.Close()
			return "", err
		}
		if _, err := io.Copy(w, rc); err != nil {
			rc.Close()
			w.Close()
			return "", err
		}
		rc.Close()
		w.Close()
	}
	return out, nil
}

var promptOrder = []string{
	"CONTEXT.md",
	"context.md",
	"README.md",
	"case.md",
	"case_description.md",
	"enunciado.md",
	"CONTEXTO.md",
}

// readFirstPrompt finds the first available "user prompt" markdown file
// in `dir` and returns its contents. If nothing matches, returns a generic
// fallback so the harness always has something to chew on.
func readFirstPrompt(dir string) (string, error) {
	for _, name := range promptOrder {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			data, err := os.ReadFile(p)
			if err != nil {
				return "", fmt.Errorf("read %s: %w", name, err)
			}
			text := string(data)
			if text = strings.TrimSpace(text); text != "" {
				return text, nil
			}
		}
	}
	// If we walked the prompt list and found nothing, scan the top level for
	// any *.md file and pick the largest one.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("scan top level: %w", err)
	}
	var best string
	var bestSize int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.Size() > bestSize {
			bestSize = info.Size()
			best = p
		}
	}
	if best != "" {
		data, err := os.ReadFile(best)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", errors.New("no prompt file found in case dir; expected one of: " + strings.Join(promptOrder, ", "))
}

// slugify returns a filesystem-safe version of `s`.
func slugify(s string) string {
	out := strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range out {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '/':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// promptBytes is a tiny helper kept here so the script and the case
// loader agree on what counts as a "prompt" bytewise (used by tests).
func promptBytes(s string) []byte { return []byte(strings.TrimSpace(s)) }
