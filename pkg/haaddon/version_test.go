package haaddon

import "testing"

func TestParseHAVersion(t *testing.T) {
	tests := []struct {
		in      string
		want    haVersion
		wantErr bool
	}{
		{in: "2026.8.0", want: haVersion{2026, 8}},
		{in: "2025.12.4", want: haVersion{2025, 12}},
		// The marker file Core writes ends in a newline.
		{in: "2026.8.1\n", want: haVersion{2026, 8}},
		// Pre-release and dev builds are still that release.
		{in: "2026.8.0b3", want: haVersion{2026, 8}},
		{in: "2026.9.0.dev0", want: haVersion{2026, 9}},
		{in: "2026.8", want: haVersion{2026, 8}},
		{in: "2026", wantErr: true},
		{in: "", wantErr: true},
		{in: "unknown.version", wantErr: true},
	}

	for _, tc := range tests {
		got, err := parseHAVersion(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseHAVersion(%q) = %v, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseHAVersion(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseHAVersion(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// Picking the wrong side of this line fails silently in both directions, so the
// boundary is worth pinning down.
func TestVersionPicksTheConfigurationMechanism(t *testing.T) {
	tests := []struct {
		version   string
		wantStore bool
	}{
		{version: "2025.12.0", wantStore: false},
		{version: "2026.7.5", wantStore: false},
		{version: "2026.8.0b0", wantStore: true},
		{version: "2026.8.0", wantStore: true},
		{version: "2026.12.1", wantStore: true},
		{version: "2027.2.0", wantStore: true},
	}

	for _, tc := range tests {
		version, err := parseHAVersion(tc.version)
		if err != nil {
			t.Fatalf("parseHAVersion(%q): %v", tc.version, err)
		}
		if got := version.atLeast(storeConfigVersion); got != tc.wantStore {
			t.Errorf("%s uses the store = %v, want %v", tc.version, got, tc.wantStore)
		}
	}
}
