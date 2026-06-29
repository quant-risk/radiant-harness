package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/quant-risk/radiant-harness/internal/tools"
)

// toolsCmd is the parent for `radiant tools ...`. Subcommands list
// the registered tools and (in future) describe their schemas.
var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Inspect the structured tool-use registry",
	Long: `Inspect the tool-use registry exposed to the executor.

In v2.37.0 the registry shipped as a scaffold; v2.38.0 (Sprint 69)
wires the first concrete tool (write_file) through the engine.
Tools are invoked by LLMs via fenced tool_call blocks in their
output, dispatched through internal/tools/Registry. The dispatcher
falls back to the legacy code-block emission path when no tool calls
are present in the LLM output.`,
}

// toolsListCmd shows all registered tools with their param counts.
// Honours --json for machine-readable output (CI / dashboards).
var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered tools (name, description, param count)",
	Long: `List every tool currently registered in the active registry.

Two registries exist:
- Default: the four stub tools advertised in v2.37.0 (read_file,
  write_file, search_code, run_gate). Calling them errors with a
  "not yet wired" message — they exist to make the surface area
  visible before wiring.
- Real: the registry produced by ` + "`tools.RealRegistry(projectDir)`" + `.
  In v2.38.0 this contains a concrete write_file backed by
  internal/tools/fs.WriteFileTool. Sprint 70-71 add read_file,
  search_code, and run_gate.

Use --real to show the real (concrete) registry. The default
shows the v2.37.0 stub registry for back-compat with operators
inspecting the surface area.`,
	RunE: runToolsList,
}

var (
	toolsListJSON bool
	toolsListReal bool
)

func init() {
	toolsCmd.AddCommand(toolsListCmd)
	toolsListCmd.Flags().BoolVar(&toolsListJSON, "json", false, "emit machine-readable JSON")
	toolsListCmd.Flags().BoolVar(&toolsListReal, "real", false, "show the real (concrete) registry instead of the v2.37.0 default stubs")
}

// registerToolsCmd wires the tools command into the root CLI tree.
func registerToolsCmd(root *cobra.Command) {
	root.AddCommand(toolsCmd)
}

func runToolsList(cmd *cobra.Command, args []string) error {
	var reg *tools.Registry
	if toolsListReal {
		// The real registry captures projectDir; we pass the cwd as
		// a sensible default. CLI callers can re-implement this with
		// the actual project root in their own wrappers if needed.
		reg = tools.RealRegistry(".")
		if reg == nil {
			return fmt.Errorf("tools: real registry builder not initialised; import internal/loop")
		}
	} else {
		reg = tools.Default()
	}

	if toolsListJSON {
		return printToolsJSON(cmd.OutOrStdout(), reg)
	}
	return printToolsTable(cmd.OutOrStdout(), reg)
}

func printToolsJSON(w io.Writer, reg *tools.Registry) error {
	names := reg.Names()
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		t := reg.Get(n)
		params := make([]map[string]any, 0, len(t.Params))
		for _, p := range t.Params {
			params = append(params, map[string]any{
				"name":        p.Name,
				"type":        p.Type,
				"required":    p.Required,
				"description": p.Description,
			})
		}
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"params":      params,
		})
	}
	_, err := fmt.Fprintf(w, "[\n")
	if err != nil {
		return err
	}
	for i, t := range out {
		_, err = fmt.Fprintf(w, "  %s\n", jSONString(t))
		if err != nil {
			return err
		}
		if i < len(out)-1 {
			if _, err := fmt.Fprint(w, ",\n"); err != nil {
				return err
			}
		}
	}
	_, err = fmt.Fprintf(w, "]\n")
	return err
}

// jSONString is a tiny indented JSON serializer to avoid pulling in
// encoding/json just for this CLI output. Skips quotes safely because
// the only string fields here come from controlled sources (skill
// descriptions, param names) — none contain user-supplied data.
func jSONString(m map[string]any) string {
	out := "{\n"
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Stable order: name first, description second, params last.
	order := []string{"name", "description", "params"}
	for _, k := range order {
		if _, ok := m[k]; !ok {
			continue
		}
		out += fmt.Sprintf("    %q: ", k)
		switch v := m[k].(type) {
		case string:
			out += fmt.Sprintf("%q", v)
		case bool:
			if v {
				out += "true"
			} else {
				out += "false"
			}
		default:
			out += fmt.Sprintf("%v", v)
		}
		out += ",\n"
	}
	out += "  }"
	return out
}

func printToolsTable(w io.Writer, reg *tools.Registry) error {
	names := reg.Names()
	if len(names) == 0 {
		_, err := fmt.Fprintln(w, "(no tools registered)")
		return err
	}
	_, err := fmt.Fprintf(w, "%-15s %-60s %s\n", "NAME", "DESCRIPTION", "PARAMS")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%-15s %-60s %s\n", "----", "-----------", "------")
	if err != nil {
		return err
	}
	for _, n := range names {
		t := reg.Get(n)
		desc := t.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		_, err = fmt.Fprintf(w, "%-15s %-60s %d\n", t.Name, desc, len(t.Params))
		if err != nil {
			return err
		}
	}
	return nil
}