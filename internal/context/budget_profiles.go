package context

import "fmt"

// BudgetProfile defines token limits per phase for a named budget tier.
type BudgetProfile struct {
	Name        string
	TotalTokens int
	PerPhase    map[string]int // tokens allocated per loop phase
	WarnRatio   float64        // fraction at which to warn (e.g. 0.70)
}

// Predefined profiles matching internal/loop budget levels.
var (
	ProfileLean = BudgetProfile{
		Name:        "lean",
		TotalTokens: 10_000,
		WarnRatio:   0.70,
		PerPhase: map[string]int{
			"discover": 1_000,
			"plan":     2_000,
			"execute":  5_000,
			"verify":   1_500,
			"persist":  500,
		},
	}

	ProfileStandard = BudgetProfile{
		Name:        "standard",
		TotalTokens: 50_000,
		WarnRatio:   0.70,
		PerPhase: map[string]int{
			"discover": 3_000,
			"plan":     8_000,
			"execute":  25_000,
			"verify":   10_000,
			"persist":  4_000,
		},
	}

	ProfileThorough = BudgetProfile{
		Name:        "thorough",
		TotalTokens: 200_000,
		WarnRatio:   0.70,
		PerPhase: map[string]int{
			"discover": 10_000,
			"plan":     30_000,
			"execute":  100_000,
			"verify":   40_000,
			"persist":  20_000,
		},
	}
)

// GetProfile returns the named budget profile, or ProfileStandard if not found.
func GetProfile(name string) BudgetProfile {
	switch name {
	case "lean":
		return ProfileLean
	case "thorough":
		return ProfileThorough
	default:
		return ProfileStandard
	}
}

// PhaseEstimate is a per-phase token estimate for a spec directory.
type PhaseEstimate struct {
	Phase   string
	Min     int
	Max     int
	Typical int
	Budget  int // from profile
	Fits    bool
}

// EstimateSpec produces a token estimate for each loop phase given a spec directory.
// It reads the spec content (if available) and multiplies by empirical per-phase ratios.
func EstimateSpec(specContent string, profile BudgetProfile) []PhaseEstimate {
	specTokens := estimateTokens(specContent)
	if specTokens < 100 {
		specTokens = 500 // fallback for empty/tiny specs
	}

	// Empirical multipliers: each phase uses N× the spec tokens as context
	phases := []struct {
		name   string
		minMul float64
		typMul float64
		maxMul float64
	}{
		{"discover", 0.5, 1.0, 2.0},
		{"plan", 1.5, 3.0, 5.0},
		{"execute", 3.0, 6.0, 12.0},
		{"verify", 1.0, 2.0, 4.0},
		{"persist", 0.3, 0.5, 1.0},
	}

	estimates := make([]PhaseEstimate, 0, len(phases))
	for _, p := range phases {
		budget := profile.PerPhase[p.name]
		typ := int(float64(specTokens) * p.typMul)
		est := PhaseEstimate{
			Phase:   p.name,
			Min:     int(float64(specTokens) * p.minMul),
			Max:     int(float64(specTokens) * p.maxMul),
			Typical: typ,
			Budget:  budget,
			Fits:    budget <= 0 || typ <= budget,
		}
		estimates = append(estimates, est)
	}
	return estimates
}

// FormatEstimate renders the estimate table as a human-readable string.
func FormatEstimate(estimates []PhaseEstimate, profile BudgetProfile) string {
	var sb fmt.Stringer
	_ = sb

	out := fmt.Sprintf("Budget estimate — profile: %s (%d tokens total)\n\n", profile.Name, profile.TotalTokens)
	out += fmt.Sprintf("%-12s %8s %8s %8s %8s %6s\n", "Phase", "Min", "Typical", "Max", "Budget", "Fits?")
	out += "------------ -------- -------- -------- -------- ------\n"

	totalTyp := 0
	for _, e := range estimates {
		fits := "yes"
		if !e.Fits && e.Budget > 0 {
			fits = "⚠ no"
		}
		out += fmt.Sprintf("%-12s %8d %8d %8d %8d %6s\n",
			e.Phase, e.Min, e.Typical, e.Max, e.Budget, fits)
		totalTyp += e.Typical
	}
	out += "------------ -------- -------- -------- -------- ------\n"
	out += fmt.Sprintf("%-12s %8s %8d %8s %8d %6s\n",
		"TOTAL", "", totalTyp, "", profile.TotalTokens,
		boolStr(totalTyp <= profile.TotalTokens, "yes", "⚠ no"))
	return out
}

func boolStr(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}
