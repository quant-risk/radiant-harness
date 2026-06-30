package loop

// TestProfileDefaultsMatrix is the closed-table test for the
// v3.7.x per-profile budget matrix. It is intentionally a
// build-tag-free test so it runs under both Light (`./...`) and
// Full (`-tags with_full`). The test guards:
//
//   1. Every documented profile (lean / standard / thorough) has
//      a row.
//   2. Unknown / typoed profile names fall back to Standard —
//      never to "unbounded".
//   3. Across the matrix, the Lean row is the cheapest and
//      Thorough is the most generous. Operators rely on this
//      monotonicity to pick a profile by name without inspecting
//      every field.
//   4. Token / iter / time / cost are all > 0 on every
//      documented profile.
import "testing"

func TestProfileDefaultsMatrix(t *testing.T) {
	t.Parallel()
	all := ProfileBudgets()
	if len(all) != 3 {
		t.Fatalf("ProfileBudgets() returned %d rows; want 3 (lean, standard, thorough)", len(all))
	}
	byName := map[BudgetProfile]ProfileBudget{}
	for _, p := range all {
		byName[p.Profile] = p
	}
	for _, want := range []BudgetProfile{ProfileLean, ProfileStandard, ProfileThorough} {
		got, ok := byName[want]
		if !ok {
			t.Errorf("missing profile row for %q", want)
			continue
		}
		if got.MaxTokens <= 0 || got.MaxIter <= 0 || got.MaxDuration <= 0 || got.MaxCostUSD <= 0 {
			t.Errorf("profile %q has a zero cap — should be bounded: tokens=%d iter=%d duration=%s cost=%g",
				want, got.MaxTokens, got.MaxIter, got.MaxDuration, got.MaxCostUSD)
		}
	}

	// Monotonicity: lean < standard < thorough on every axis.
	lean := byName[ProfileLean]
	std := byName[ProfileStandard]
	thr := byName[ProfileThorough]
	if !(lean.MaxTokens < std.MaxTokens && std.MaxTokens < thr.MaxTokens) {
		t.Errorf("tokens not monotonic: lean=%d std=%d thr=%d",
			lean.MaxTokens, std.MaxTokens, thr.MaxTokens)
	}
	if !(lean.MaxIter < std.MaxIter && std.MaxIter < thr.MaxIter) {
		t.Errorf("iter not monotonic: lean=%d std=%d thr=%d",
			lean.MaxIter, std.MaxIter, thr.MaxIter)
	}
	if !(lean.MaxDuration < std.MaxDuration && std.MaxDuration < thr.MaxDuration) {
		t.Errorf("duration not monotonic: lean=%s std=%s thr=%s",
			lean.MaxDuration, std.MaxDuration, thr.MaxDuration)
	}
	if !(lean.MaxCostUSD < std.MaxCostUSD && std.MaxCostUSD < thr.MaxCostUSD) {
		t.Errorf("cost not monotonic: lean=%g std=%g thr=%g",
			lean.MaxCostUSD, std.MaxCostUSD, thr.MaxCostUSD)
	}

	// Unknown profile falls back to Standard — NOT to "unbounded".
	if got := DefaultsForProfile("turbo"); got != DefaultsForProfile(ProfileStandard) {
		t.Errorf("unknown profile should fall back to Standard; got %+v", got)
	}
	if got := DefaultsForProfile(""); got != DefaultsForProfile(ProfileStandard) {
		t.Errorf("empty profile should fall back to Standard; got %+v", got)
	}
	if got := DefaultsForProfile("Lean "); got != DefaultsForProfile(ProfileStandard) {
		// Whitespace / case-mismatch not silently mapped; operator sees
		// Standard defaults and can audit from `radiant budget list`.
		t.Errorf("ambiguous profile should fall back to Standard; got %+v", got)
	}
}
