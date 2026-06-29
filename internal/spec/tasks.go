package spec

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	radiant "github.com/quant-risk/radiant-harness/internal"
)

// headerRowRe matches the header / separator rows so we can skip them.
var headerSeparatorRe = regexp.MustCompile(`^\s*\|?\s*[-:]+`)

// ParseTasks reads and parses a tasks.md file. Returns an error only on
// I/O failure or unparseable structure; individual bad rows are skipped
// (with the caller's quality pipeline flagging missing coverage later).
func ParseTasks(path string) (*radiant.TaskPlan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open tasks: %w", err)
	}
	defer f.Close()

	plan := &radiant.TaskPlan{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), scannerBufferSize)

	var skippedHeader bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip non-table lines and table separators (|---|---|---|).
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if headerSeparatorRe.MatchString(line) {
			skippedHeader = true
			continue
		}
		// Skip the first "| # | Task | …" header row.
		if !skippedHeader {
			skippedHeader = true
			continue
		}

		cols := parseTableRow(line)
		if len(cols) < 5 {
			// Malformed row — skip rather than fail. The validator will
			// surface missing tasks via AC coverage gaps.
			continue
		}

		id, err := strconv.Atoi(strings.TrimSpace(cols[0]))
		if err != nil {
			continue
		}

		// Some specs add a "Phase" column or split Depends/Gate into more
		// columns. Map by position where possible, fall back to "—" sentinel
		// when the row is short.
		gate := ""
		status := ""
		switch {
		case len(cols) >= 6:
			gate = strings.TrimSpace(cols[4])
			status = strings.TrimSpace(cols[5])
		case len(cols) == 5:
			// Assume the last column is status; gate empty.
			status = strings.TrimSpace(cols[4])
		}

		task := radiant.Task{
			ID:        id,
			Name:      strings.TrimSpace(cols[1]),
			CoversACs: splitACs(cols[2]),
			DependsOn: parseDeps(cols[3]),
			Gate:      gate,
			Status:    status,
			Parallel:  strings.Contains(cols[1], "[P]"),
		}
		plan.Tasks = append(plan.Tasks, task)
	}

	// Group sequential tasks and parallel-marked bursts into phases so the
	// orchestrator can stream them sequentially while parallelizing within
	// each phase.
	plan.Phases = groupPhases(plan.Tasks)

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan tasks: %w", err)
	}
	return plan, nil
}

// parseTableRow splits a single markdown table row into its cell contents,
// trimming surrounding whitespace and backtick-fenced commands.
func parseTableRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	result := make([]string, len(parts))
	for i, p := range parts {
		cell := strings.TrimSpace(p)
		// Strip inline code fences so the cell holds the command verbatim.
		cell = strings.TrimPrefix(cell, "`")
		cell = strings.TrimSuffix(cell, "`")
		result[i] = cell
	}
	return result
}

// splitACs splits the "Covers AC" cell on commas, normalizing each token
// through NormalizeACID so AC1, AC-1, AC_1 all map to AC-1.
func splitACs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" || s == "-" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Some authors write "AC-1 · AC-2" (middle dot separator).
		for _, sub := range strings.FieldsFunc(p, func(r rune) bool {
			return r == '·' || r == ';' || r == '|'
		}) {
			sub = strings.TrimSpace(sub)
			if sub == "" {
				continue
			}
			result = append(result, NormalizeACID(sub))
		}
	}
	return result
}

// parseDeps parses the "Depends on" cell. Accepts comma- or whitespace-
// separated integers; non-numeric tokens are silently dropped (the
// validator will surface dependency failures at runtime).
func parseDeps(s string) []int {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" || s == "-" {
		return nil
	}
	var result []int
	for _, p := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '·'
	}) {
		if n, err := strconv.Atoi(p); err == nil {
			result = append(result, n)
		}
	}
	return result
}

// groupPhases converts a flat task list into phases. Each phase groups
// tasks of the same kind (sequential or parallel-marked) that appear
// consecutively in the file. A phase boundary is set whenever the kind
// changes — so `[P]` tasks next to each other form one parallel phase, and
// a non-parallel task following a parallel burst starts a new sequential
// phase. This matches the natural reading of tasks.md: a parallel burst
// is a single batch, not a series of single-task phases.
func groupPhases(tasks []radiant.Task) []radiant.Phase {
	if len(tasks) == 0 {
		return nil
	}

	var phases []radiant.Phase
	current := radiant.Phase{Name: "Phase 1"}

	for _, t := range tasks {
		if len(current.Tasks) > 0 && current.Tasks[0].Parallel != t.Parallel {
			phases = append(phases, current)
			current = radiant.Phase{
				Name:  fmt.Sprintf("Phase %d", len(phases)+1),
				Tasks: []radiant.Task{},
			}
		}
		current.Tasks = append(current.Tasks, t)
	}
	if len(current.Tasks) > 0 {
		phases = append(phases, current)
	}
	return phases
}
