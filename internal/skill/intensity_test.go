package skill

import (
	"strings"
	"testing"
)

const sampleBody = `# Lazy Executor

## Persistência

ATIVO EM TODA ITERAÇÃO.

## A ladder

1. Existe?
2. Reusa?
3. Stdlib?

## Intensidade

| Nível | O que muda |
|-------|-----------|
| **lite** | Constrói o pedido, cita alt. |
| **full** | A ladder aplicada. Default. |
| **ultra** | YAGNI extremista. |

Exemplo: "Adiciona cache."
- lite: "Feito, cite lru_cache."
- full: "lru_cache direto. Pulou cache class."
- ultra: "Sem cache até profiler. Quando: lru_cache."

## Boundaries

Lazy governa o que constrói.`

func TestFilterForIntensity_Lite(t *testing.T) {
	got := FilterForIntensity(sampleBody, IntensityLite)
	// lite keeps: table lite row, bullet lite example
	if !strings.Contains(got, "LITE") {
		t.Errorf("lite should keep lite table row")
	}
	if strings.Contains(got, "FULL") || strings.Contains(got, "ULTRA") {
		t.Errorf("lite should drop full/ultra rows")
	}
	if !strings.Contains(got, "- LITE:") {
		t.Errorf("lite should keep lite bullet")
	}
	if strings.Contains(got, "- FULL:") || strings.Contains(got, "- ULTRA:") {
		t.Errorf("lite should drop full/ultra bullets")
	}
}

func TestFilterForIntensity_Full(t *testing.T) {
	got := FilterForIntensity(sampleBody, IntensityFull)
	if !strings.Contains(got, "FULL") {
		t.Errorf("full should keep full table row")
	}
	if strings.Contains(got, "LITE") || strings.Contains(got, "ULTRA") {
		t.Errorf("full should drop lite/ultra rows")
	}
	if !strings.Contains(got, "- FULL:") {
		t.Errorf("full should keep full bullet")
	}
	if strings.Contains(got, "- LITE:") || strings.Contains(got, "- ULTRA:") {
		t.Errorf("full should drop lite/ultra bullets")
	}
}

func TestFilterForIntensity_Ultra(t *testing.T) {
	got := FilterForIntensity(sampleBody, IntensityUltra)
	if !strings.Contains(got, "ULTRA") {
		t.Errorf("ultra should keep ultra table row")
	}
	if strings.Contains(got, "LITE") || strings.Contains(got, "FULL") {
		t.Errorf("ultra should drop lite/full rows")
	}
	if !strings.Contains(got, "- ULTRA:") {
		t.Errorf("ultra should keep ultra bullet")
	}
	if strings.Contains(got, "- LITE:") || strings.Contains(got, "- FULL:") {
		t.Errorf("ultra should drop lite/full bullets")
	}
}

func TestFilterForIntensity_Off(t *testing.T) {
	if got := FilterForIntensity(sampleBody, IntensityOff); got != "" {
		t.Errorf("off should return empty string, got %q", got)
	}
}

func TestFilterForIntensity_PreservesNonLabeledLines(t *testing.T) {
	got := FilterForIntensity(sampleBody, IntensityUltra)
	if !strings.Contains(got, "ATIVO EM TODA ITERAÇÃO") {
		t.Errorf("non-labeled prose should be preserved")
	}
	if !strings.Contains(got, "## A ladder") {
		t.Errorf("section headers should be preserved")
	}
	if !strings.Contains(got, "1. Existe?") {
		t.Errorf("numbered list should be preserved")
	}
	if !strings.Contains(got, "## Boundaries") {
		t.Errorf("closing sections should be preserved")
	}
}

func TestParseIntensity(t *testing.T) {
	cases := map[string]Intensity{
		"":      IntensityFull, // default
		"full":  IntensityFull,
		"lite":  IntensityLite,
		"ultra": IntensityUltra,
		"off":   IntensityOff,
		"FULL":  IntensityFull,
		"  lite ": IntensityLite,
	}
	for in, want := range cases {
		got, err := ParseIntensity(in)
		if err != nil {
			t.Errorf("ParseIntensity(%q): unexpected err %v", in, err)
		}
		if got != want {
			t.Errorf("ParseIntensity(%q): got %q want %q", in, got, want)
		}
	}
	if _, err := ParseIntensity("nuclear"); err == nil {
		t.Errorf("expected error for invalid intensity")
	}
}

func TestLoadLazyExecutorSkill_Full(t *testing.T) {
	got, err := LoadLazyExecutorSkill(IntensityFull)
	if err != nil {
		t.Fatalf("LoadLazyExecutorSkill: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty body for full")
	}
	// Frontmatter should be stripped.
	if strings.HasPrefix(got, "---") {
		t.Errorf("frontmatter should be stripped, got: %q", got[:20])
	}
	// Body should contain the ladder.
	if !strings.Contains(got, "ladder") {
		t.Errorf("body should contain ladder content")
	}
}

func TestLoadLazyExecutorSkill_Off(t *testing.T) {
	got, err := LoadLazyExecutorSkill(IntensityOff)
	if err != nil {
		t.Fatalf("LoadLazyExecutorSkill off: %v", err)
	}
	if got != "" {
		t.Errorf("off should return empty, got %d chars", len(got))
	}
}

func TestTableRowMatch_NoLabel(t *testing.T) {
	// Header row shouldn't match.
	if got := tableRowMatch("| Nível | O que muda |"); got != "" {
		t.Errorf("header should not match, got %q", got)
	}
}

func TestBulletLabelMatch_NoLabel(t *testing.T) {
	if got := bulletLabelMatch("- Normal bullet without label"); got != "" {
		t.Errorf("normal bullet should not match, got %q", got)
	}
}