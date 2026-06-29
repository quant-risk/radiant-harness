package main

import "github.com/spf13/cobra"

func registerTelemetryCmds(root *cobra.Command) {
	// ── telemetry (Sprint 18 — privacy-first local usage stats) ──
	// `radiant telemetry {status|enable|disable|show}` — opt-in local
	// usage tracking. PRIVACY-FIRST: nothing is collected by
	// default. The user must explicitly run `radiant telemetry
	// enable` to start logging. Even when enabled, only the
	// command name + timestamp + a content hash are recorded
	// locally (no args, no paths, no project metadata).
	telemetryCmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Privacy-first local usage stats (opt-in)",
	}
	telemetryStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show whether telemetry is enabled and what is recorded",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetryStatus()
		},
	}
	telemetryEnableCmd := &cobra.Command{
		Use:   "enable",
		Short: "Opt in to local telemetry (writes to .radiant-harness/telemetry.jsonl)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetryEnable()
		},
	}
	telemetryDisableCmd := &cobra.Command{
		Use:   "disable",
		Short: "Opt out of telemetry (removes the log file)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetryDisable()
		},
	}
	telemetryShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show the local telemetry log (last 50 events)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetryShow()
		},
	}
	telemetryCmd.AddCommand(telemetryStatusCmd, telemetryEnableCmd, telemetryDisableCmd, telemetryShowCmd)
	root.AddCommand(telemetryCmd)

	// ── stats (Sprint 29b — stats test plan scaffold) ──
	// `radiant stats <test>` produces a structured hypothesis-test
	// plan scaffold (docs/stats/<test>-plan.md) following the
	// radiant-stats skill. The CLI scaffolds the input contract
	// (H0/H1, alpha, power, effect size, multiple-testing);
	// the user fills the actual values and runs the test.
	statsCmd := &cobra.Command{
		Use:   "stats <test>",
		Short: "Scaffold a hypothesis-test plan (alpha, power, effect size, multiple-testing)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			test := args[0]
			out, _ := cmd.Flags().GetString("output")
			alpha, _ := cmd.Flags().GetFloat64("alpha")
			power, _ := cmd.Flags().GetFloat64("power")
			return runStatsScaffold(test, alpha, power, out)
		},
	}
	statsCmd.Flags().StringP("output", "o", "", "output path (default: docs/stats/<test>-plan.md)")
	statsCmd.Flags().Float64("alpha", 0.05, "significance level (α)")
	statsCmd.Flags().Float64("power", 0.80, "statistical power (1-β)")
	root.AddCommand(statsCmd)

	// ── causal-estimate (Sprint 29b — causal analysis scaffold) ──
	// `radiant causal-estimate <design>` produces a structured
	// causal analysis plan (docs/causal/<design>-plan.md) with a
	// DAG template, identification assumption, sensitivity
	// checklist, and CATE exploration template. Follows the
	// radiant-causal + radiant-causal-ml skills.
	causalEstCmd := &cobra.Command{
		Use:   "causal-estimate <design>",
		Short: "Scaffold a causal analysis (DAG, identification, sensitivity, CATE)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			design := args[0]
			out, _ := cmd.Flags().GetString("output")
			return runCausalEstimateScaffold(design, out)
		},
	}
	causalEstCmd.Flags().StringP("output", "o", "", "output path (default: docs/causal/<design>-plan.md)")
	root.AddCommand(causalEstCmd)

	// ── model (Sprint 30c — model spec scaffold) ──
	// `radiant model <type>` produces a structured model spec
	// scaffold (docs/model/<type>-spec.md) following the
	// radiant-ml skill. Captures: target, features, train/eval
	// split, baseline, primary metric, ethics + monitoring plan.
	modelCmd := &cobra.Command{
		Use:   "model <type>",
		Short: "Scaffold a model spec (target, features, baseline, metric, monitoring)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			typ := args[0]
			out, _ := cmd.Flags().GetString("output")
			successMetric, _ := cmd.Flags().GetString("success-metric")
			return runModelScaffold(typ, successMetric, out)
		},
	}
	modelCmd.Flags().StringP("output", "o", "", "output path (default: docs/model/<type>-spec.md)")
	modelCmd.Flags().String("success-metric", "", "business-aligned success metric (e.g. '5% reduction in churn false negatives')")
	root.AddCommand(modelCmd)

	// ── predict (Sprint 30c — prediction request scaffold) ──
	// `radiant predict <model-id>` produces a structured prediction
	// request scaffold (docs/predict/<model-id>-request.md):
	// input contract, latency SLO, error semantics, monitoring
	// hook, fall-back policy. Wires the radiant-ml skill to a
	// concrete serving spec.
	predictCmd := &cobra.Command{
		Use:   "predict <model-id>",
		Short: "Scaffold a model serving request (inputs, latency, errors, monitoring)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelID := args[0]
			out, _ := cmd.Flags().GetString("output")
			latencyMs, _ := cmd.Flags().GetInt("latency-ms")
			return runPredictScaffold(modelID, latencyMs, out)
		},
	}
	predictCmd.Flags().StringP("output", "o", "", "output path (default: docs/predict/<model-id>-request.md)")
	predictCmd.Flags().Int("latency-ms", 200, "p99 latency budget in milliseconds")
	root.AddCommand(predictCmd)

	// ── train (Sprint 31b — training plan scaffold) ──
	// `radiant train <model-id>` produces a structured training
	// plan scaffold (docs/train/<model-id>-plan.md): data split,
	// training recipe (optimizer / LR / epochs / regularisation),
	// compute budget, checkpointing, reproducibility recipe.
	// Wires the radiant-ml + radiant-deep-learning skills.
	trainCmd := &cobra.Command{
		Use:   "train <model-id>",
		Short: "Scaffold a training plan (data, recipe, compute, checkpoints, reproducibility)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelID := args[0]
			out, _ := cmd.Flags().GetString("output")
			return runTrainScaffold(modelID, out)
		},
	}
	trainCmd.Flags().StringP("output", "o", "", "output path (default: docs/train/<model-id>-plan.md)")
	root.AddCommand(trainCmd)

	// ── evaluate (Sprint 31b — evaluation plan scaffold) ──
	// `radiant evaluate <model-id>` produces an evaluation plan
	// scaffold (docs/eval/<model-id>-eval.md): metrics, held-out
	// test, statistical significance, robustness, fairness slices.
	// Wires the radiant-ml + radiant-stats skills.
	evaluateCmd := &cobra.Command{
		Use:   "evaluate <model-id>",
		Short: "Scaffold an evaluation plan (metrics, held-out test, stats, robustness, fairness)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelID := args[0]
			out, _ := cmd.Flags().GetString("output")
			return runEvaluateScaffold(modelID, out)
		},
	}
	evaluateCmd.Flags().StringP("output", "o", "", "output path (default: docs/eval/<model-id>-eval.md)")
	root.AddCommand(evaluateCmd)

	// ── drift (Sprint 31b — drift monitoring scaffold) ──
	// `radiant drift <model-id>` produces a drift monitoring plan
	// scaffold (docs/drift/<model-id>-monitor.md): PSI / CSI
	// thresholds, alert escalation, retraining trigger, rollback
	// plan. Wires the radiant-ml + radiant-ml-ops skills.
	driftCmd := &cobra.Command{
		Use:   "drift <model-id>",
		Short: "Scaffold drift monitoring (PSI/CSI thresholds, alerts, retraining, rollback)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelID := args[0]
			out, _ := cmd.Flags().GetString("output")
			return runDriftScaffold(modelID, out)
		},
	}
	driftCmd.Flags().StringP("output", "o", "", "output path (default: docs/drift/<model-id>-monitor.md)")
	root.AddCommand(driftCmd)

	// ── autodata (Sprint 31c — LLM-driven skill authoring) ──
	// `radiant autodata <skill-name>` uses an LLM (when configured)
	// to draft a new skill from a domain prompt. The user reviews
	// the generated skill before promotion. Implementation of
	// Kulikov et al. (2026) "Autodata" pattern: agentic data
	// scientist for skill authoring.
	autodataCmd := &cobra.Command{
		Use:   "autodata <skill-name>",
		Short: "Auto-author a skill from a domain prompt via LLM (review before install)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName := args[0]
			domain, _ := cmd.Flags().GetString("domain")
			out, _ := cmd.Flags().GetString("output")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			return runAutodata(skillName, domain, out, dryRun)
		},
	}
	autodataCmd.Flags().String("domain", "", "domain prompt describing what the skill should cover")
	autodataCmd.Flags().StringP("output", "o", "", "output dir (default: internal/skill/skills/<skill-name>/)")
	autodataCmd.Flags().Bool("dry-run", false, "print generated skill to stdout instead of writing files")
	root.AddCommand(autodataCmd)

	// ── validate (Sprint 32c — validate a scaffolded plan / spec) ──
	// `radiant validate <path>` runs basic sanity checks on a
	// scaffolded artifact (stats-plan, causal-plan, model-spec,
	// train-plan, etc.): file exists, has expected sections, no
	// obvious placeholders left. Returns non-zero on failure.
	validateFileCmd := &cobra.Command{
		Use:   "validate-file <path>",
		Short: "Validate a scaffolded plan or spec (sections, placeholders, syntax)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			return runValidate(path)
		},
	}
	root.AddCommand(validateFileCmd)

	// ── profile (Sprint 32c — data / model profile scaffold) ──
	// `radiant profile <dataset>` produces a data-profile scaffold
	// (docs/profile/<dataset>-profile.md): schema, row count,
	// null rates, distribution checks, drift metrics, monitoring
	// plan. Wires the radiant-data + radiant-drift skills.
	profileCmd := &cobra.Command{
		Use:   "profile <dataset>",
		Short: "Scaffold a data profile (schema, distributions, drift, monitoring)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dataset := args[0]
			out, _ := cmd.Flags().GetString("output")
			return runProfileScaffold(dataset, out)
		},
	}
	profileCmd.Flags().StringP("output", "o", "", "output path (default: docs/profile/<dataset>-profile.md)")
	root.AddCommand(profileCmd)

	// ── incident (Sprint 19 — incident response scaffold) ──
	// `radiant incident <severity> <summary>` wires the `incident`
	// skill to a CLI. Generates docs/incidents/<NNNN>-<slug>.md
	// with the post-mortem template pre-filled; the on-call
	// engineer fills in the timeline + RCA + action items.
	incidentCmd := &cobra.Command{
		Use:   "incident <severity> <summary>",
		Short: "Start an incident: scaffold docs/incidents/<NNNN>-<slug>.md",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			severity := args[0]
			summary := args[1]
			out, _ := cmd.Flags().GetString("output")
			return runIncident(severity, summary, out)
		},
	}
	incidentCmd.Flags().StringP("output", "o", "", "output path (default: docs/incidents/<NNNN>-<slug>.md)")
	root.AddCommand(incidentCmd)

	// ── telemetry summary (Sprint 21 — aggregate counts) ──
	// `radiant telemetry summary` reads the local log and prints
	// aggregate stats: total events, top commands, daily counts.
	// Same privacy guarantees as `show` — only local file access.
	telemetrySummaryCmd := &cobra.Command{
		Use:   "summary",
		Short: "Show aggregate counts from the local telemetry log",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetrySummary()
		},
	}
	telemetryCmd.AddCommand(telemetrySummaryCmd)
	telemetryRotateCmd := &cobra.Command{
		Use:   "rotate",
		Short: "Archive old events when log exceeds --max-entries (default 1000)",
		RunE: func(cmd *cobra.Command, args []string) error {
			max, _ := cmd.Flags().GetInt("max-entries")
			return runTelemetryRotate(max)
		},
	}
	telemetryRotateCmd.Flags().Int("max-entries", 1000, "max events to keep in the active log; older events archived to telemetry-YYYY-MM-DD.jsonl")
	telemetryCmd.AddCommand(telemetryRotateCmd)
	telemetryExportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export telemetry log as JSON or CSV (default: JSON to stdout)",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			output, _ := cmd.Flags().GetString("output")
			since, _ := cmd.Flags().GetString("since")
			return runTelemetryExport(format, output, since)
		},
	}
	telemetryExportCmd.Flags().String("format", "json", "export format: json or csv")
	telemetryExportCmd.Flags().String("output", "", "output file path (default: stdout)")
	telemetryExportCmd.Flags().String("since", "", "filter events to >= YYYY-MM-DD (inclusive); empty = no filter")
	telemetryCmd.AddCommand(telemetryExportCmd)
}
