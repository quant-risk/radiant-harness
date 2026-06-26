package improve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ValidationResult describes whether a proposal passed held-out validation.
type ValidationResult struct {
	Proposal    Proposal
	Passed      bool
	OldScore    float64 // estimated success rate before patch
	NewScore    float64 // estimated success rate after patch
	DeltaPP     float64 // percentage-point improvement
	Evidence    string
	ValidatedAt time.Time
}

// ImprovementRecord is persisted to improvements.jsonl after a validated apply.
type ImprovementRecord struct {
	Skill       string    `json:"skill"`
	File        string    `json:"file"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	RunIDs      []string  `json:"run_ids"`
	OldScore    float64   `json:"old_score"`
	NewScore    float64   `json:"new_score"`
	DeltaPP     float64   `json:"delta_pp"`
	AppliedAt   time.Time `json:"applied_at"`
}

// ValidateProposal checks whether a proposal would improve outcomes.
// This is a static validator — it scores the proposal against patterns found
// in the trace history without calling an LLM. LLM-validation is the next layer.
func ValidateProposal(proposal Proposal, analysis *AnalysisResult) ValidationResult {
	result := ValidationResult{
		Proposal:    proposal,
		ValidatedAt: time.Now(),
	}

	// Count how often this category's pattern appears
	categoryCount := 0
	totalFailures := analysis.Failures
	for _, p := range analysis.Patterns {
		if p.Category == proposal.Category {
			categoryCount = p.Count
			break
		}
	}

	if totalFailures == 0 || categoryCount == 0 {
		result.Passed = false
		result.Evidence = "no matching failure events to validate against"
		return result
	}

	// Estimate success rate improvement based on how many failures this addresses
	failureRate := float64(totalFailures) / float64(max(analysis.Events, 1))
	addressedRate := float64(categoryCount) / float64(max(totalFailures, 1))

	result.OldScore = 1.0 - failureRate
	// If we fix all failures in this category, new score improves proportionally
	improvement := failureRate * addressedRate * proposal.Confidence
	result.NewScore = result.OldScore + improvement
	if result.NewScore > 1.0 {
		result.NewScore = 1.0
	}
	result.DeltaPP = (result.NewScore - result.OldScore) * 100

	// Minimum threshold: must improve by at least 5 percentage points
	result.Passed = result.DeltaPP >= 5.0
	if result.Passed {
		result.Evidence = fmt.Sprintf("addresses %d of %d failures (%.0f%%), projected +%.1fpp improvement",
			categoryCount, totalFailures, float64(categoryCount)/float64(totalFailures)*100, result.DeltaPP)
	} else {
		result.Evidence = fmt.Sprintf("improvement %.1fpp below 5pp threshold", result.DeltaPP)
	}

	return result
}

// ApplyProposal writes the proposed change to the skill file,
// backing up the original first. Returns the backup path.
func ApplyProposal(proposal Proposal, projectDir string) (backupPath string, err error) {
	skillPath := filepath.Join(projectDir, ".radiant-harness", "skills", proposal.Skill, proposal.File)
	original, err := os.ReadFile(skillPath)
	if err != nil {
		return "", fmt.Errorf("read skill file: %w", err)
	}

	// Write backup
	backupPath = skillPath + ".bak." + time.Now().Format("20060102T150405")
	if err := os.WriteFile(backupPath, original, 0o644); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}

	// Apply the patch
	patched := string(original)
	if proposal.Before != "" && strings.Contains(patched, proposal.Before) {
		patched = strings.Replace(patched, proposal.Before, proposal.After, 1)
	} else {
		// Append to end if before-text not found
		patched = patched + "\n\n" + proposal.After
	}

	if err := os.WriteFile(skillPath, []byte(patched), 0o644); err != nil {
		return backupPath, fmt.Errorf("write patched skill: %w", err)
	}
	return backupPath, nil
}

// PersistRecord appends an improvement record to the JSONL history file.
func PersistRecord(record ImprovementRecord, projectDir string) error {
	histPath := filepath.Join(projectDir, ".radiant-harness", "improvements.jsonl")
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}
	f, err := os.OpenFile(histPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open improvements.jsonl: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// ReadHistory reads all improvement records from the JSONL history file.
func ReadHistory(projectDir string) ([]ImprovementRecord, error) {
	histPath := filepath.Join(projectDir, ".radiant-harness", "improvements.jsonl")
	data, err := os.ReadFile(histPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read improvements.jsonl: %w", err)
	}

	var records []ImprovementRecord
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var r ImprovementRecord
		if json.Unmarshal([]byte(line), &r) == nil {
			records = append(records, r)
		}
	}
	return records, nil
}

// FormatValidationResult renders a validation result as human-readable text.
func FormatValidationResult(r ValidationResult) string {
	status := "PASS"
	if !r.Passed {
		status = "FAIL"
	}
	return fmt.Sprintf("[%s] %s/%s — %s\n  Before: %.0f%% success rate | After: %.0f%% | Delta: +%.1fpp\n  Evidence: %s\n",
		status, r.Proposal.Skill, r.Proposal.File,
		r.Proposal.Category,
		r.OldScore*100, r.NewScore*100, r.DeltaPP,
		r.Evidence)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
