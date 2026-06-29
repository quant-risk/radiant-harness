package mode

import "testing"

func TestMode_String(t *testing.T) {
	cases := []struct {
		m    Mode
		want string
	}{
		{Light, "light"},
		{Full, "full"},
		{"weird", "weird"},
	}
	for _, c := range cases {
		if got := c.m.String(); got != c.want {
			t.Errorf("%q.String() = %q, want %q", c.m, got, c.want)
		}
	}
}

func TestMode_Description(t *testing.T) {
	if Light.Description() == "" {
		t.Error("Light should have a description")
	}
	if Full.Description() == "" {
		t.Error("Full should have a description")
	}
	if Mode("nonsense").Description() == "" {
		t.Error("unknown mode should still return a description (even if generic)")
	}
}

func TestMode_IsValid(t *testing.T) {
	if !Light.IsValid() {
		t.Error("Light should be valid")
	}
	if !Full.IsValid() {
		t.Error("Full should be valid")
	}
	if Mode("auto").IsValid() {
		t.Error("auto should not be valid (removed in v2.42.0)")
	}
	if Mode("").IsValid() {
		t.Error("empty should not be valid")
	}
}