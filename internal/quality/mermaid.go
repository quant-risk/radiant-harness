package quality

import (
	"os"
	"path/filepath"
	"strings"

	radiant "github.com/quant-risk/radiant-harness/internal"
)

var mermaidTypes = map[string]bool{
	"flowchart": true, "graph": true, "sequenceDiagram": true,
	"classDiagram": true, "stateDiagram": true, "erDiagram": true,
	"journey": true, "gantt": true, "pie": true, "mindmap": true,
	"timeline": true, "gitGraph": true, "quadrantChart": true,
	"requirementDiagram": true, "C4Context": true, "C4Container": true,
	"C4Component": true, "kanban": true, "radar": true, "zenuml": true,
}

// ValidateMermaid validates Mermaid diagram blocks in markdown files.
func ValidateMermaid(root string) radiant.ScriptResult {
	var errors []string
	var warnings []string

	files := walkMarkdown(root)

	for _, f := range files {
		text, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, f)
		lines := strings.Split(string(text), "\n")

		inBlock := false
		blockStart := 0
		blockNum := 0
		var blockLines []string

		for i, line := range lines {
			trimmed := strings.TrimSpace(line)

			if !inBlock {
				if strings.HasPrefix(trimmed, "```mermaid") || strings.HasPrefix(trimmed, "``` mermaid") {
					inBlock = true
					blockStart = i + 1
					blockNum++
					blockLines = nil
				}
			} else {
				if trimmed == "```" {
					where := rel + " (mermaid #" + itoa(blockNum) + ", line " + itoa(blockStart) + ")"

					if len(blockLines) == 0 {
						errors = append(errors, where+": empty mermaid block")
					} else {
						first := ""
						for _, bl := range blockLines {
							s := strings.TrimSpace(bl)
							if s != "" && !strings.HasPrefix(s, "%%") && s != "---" {
								first = s
								break
							}
						}
						if first == "" {
							errors = append(errors, where+": empty mermaid block")
						} else {
							parts := strings.Fields(first)
							if len(parts) > 0 && !mermaidTypes[parts[0]] {
								errors = append(errors, where+": unknown diagram type \""+parts[0]+"\"")
							}
						}

						allText := strings.Join(blockLines, "\n")
						quoteCount := strings.Count(allText, "\"")
						if quoteCount%2 != 0 {
							errors = append(errors, where+": unbalanced double quotes")
						}
					}

					inBlock = false
				} else {
					blockLines = append(blockLines, line)
				}
			}
		}
	}

	return radiant.ScriptResult{
		OK:       len(errors) == 0,
		Errors:   errors,
		Warnings: warnings,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
