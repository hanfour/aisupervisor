package config

import "testing"

// TestResolvePixelLabAPIKey covers the env-over-YAML precedence rule.
// Production code calls this once at supervisor startup; a regression
// (e.g. swapping the precedence) would silently leak the YAML-stored
// key into a system that should be using the operator's env-supplied
// override (CI, ephemeral sandboxes, key rotation).
func TestResolvePixelLabAPIKey(t *testing.T) {
	cases := []struct {
		name      string
		envValue  string
		yamlValue string
		want      string
	}{
		{"env wins over yaml", "from-env", "from-yaml", "from-env"},
		{"yaml used when env unset", "", "from-yaml", "from-yaml"},
		{"both unset returns empty", "", "", ""},
		{"empty env falls through to yaml", "", "yaml-only", "yaml-only"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PIXELLAB_API_KEY", tc.envValue)
			got := ResolvePixelLabAPIKey(PixelLabConfig{APIKey: tc.yamlValue})
			if got != tc.want {
				t.Errorf("ResolvePixelLabAPIKey(env=%q, yaml=%q) = %q, want %q",
					tc.envValue, tc.yamlValue, got, tc.want)
			}
		})
	}
}
