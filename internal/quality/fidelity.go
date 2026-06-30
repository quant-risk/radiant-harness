package quality

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	radiant "github.com/quant-risk/radiant-harness/v3/internal"
)

var codeExts = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true,
	".jsx": true, ".py": true, ".go": true, ".java": true, ".rb": true,
	".php": true, ".cs": true, ".rs": true, ".kt": true, ".swift": true,
	".sql": true, ".feature": true,
}

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, ".claude": true, ".agents": true,
	".cursor": true, ".gemini": true, ".windsurf": true,
	"specs": true, "docs": true, "scripts": true,
}

// EvalSpecFidelity evaluates spec-to-code fidelity.
func EvalSpecFidelity(root string) radiant.ScriptResult {
	specsDir := filepath.Join(root, "specs")
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		return radiant.ScriptResult{OK: true, Warnings: []string{"No specs/ - nothing to evaluate"}}
	}

	codeBlob := walkCode(root)
	codeACSet := make(map[string]bool)
	for _, ac := range extractACs(codeBlob) {
		codeACSet[ac] = true
	}

	var errors []string
	var warnings []string

	entries, _ := os.ReadDir(specsDir)
	for _, e := range entries {
		if !e.IsDir() || len(e.Name()) < 4 {
			continue
		}

		specFile := filepath.Join(specsDir, e.Name(), "spec.md")
		specText, err := os.ReadFile(specFile)
		if err != nil {
			continue
		}

		acs := extractACs(string(specText))
		if len(acs) == 0 {
			continue
		}

		taskFile := filepath.Join(specsDir, e.Name(), "tasks.md")
		taskACs := make(map[string]bool)
		if taskText, err := os.ReadFile(taskFile); err == nil {
			for _, ac := range extractACs(string(taskText)) {
				taskACs[ac] = true
			}
		}

		for _, ac := range acs {
			if !taskACs[ac] {
				errors = append(errors, e.Name()+": "+ac+" without task coverage")
			}
			if !codeACSet[ac] {
				warnings = append(warnings, e.Name()+": "+ac+" without test reference")
			}
		}
	}

	return radiant.ScriptResult{OK: len(errors) == 0, Errors: errors, Warnings: warnings}
}

func walkCode(root string) string {
	var blob strings.Builder
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] || strings.HasPrefix(info.Name(), ".tmp") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(info.Name())
		if codeExts[ext] {
			if data, err := os.ReadFile(path); err == nil {
				blob.Write(data)
				blob.WriteByte('\n')
			}
		}
		return nil
	})
	return blob.String()
}

func extractACs(text string) []string {
	re := regexp.MustCompile(`(?i)AC[-_]?\d+`)
	matches := re.FindAllString(text, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		upper := strings.ToUpper(m)
		normalized := strings.Replace(upper, "_", "-", -1)
		if !strings.Contains(normalized, "-") {
			normalized = "AC-" + strings.TrimPrefix(normalized, "AC")
		}
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result
}
