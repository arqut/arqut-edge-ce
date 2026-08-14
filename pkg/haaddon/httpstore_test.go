package haaddon

import (
	"encoding/json"
	"testing"
)

// config builds a stored config from a JSON literal, the shape Core sends.
func config(t *testing.T, raw string) haConfig {
	t.Helper()
	var out haConfig
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("parse config %s: %v", raw, err)
	}
	return out
}

// proxies reads the trusted proxy list back out of a config.
func proxies(t *testing.T, c haConfig) []string {
	t.Helper()
	raw, ok := c[keyTrustedProxies]
	if !ok {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse %s: %v", keyTrustedProxies, err)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWithTrustedProxiesAddsSubnetsAndEnablesForwarding(t *testing.T) {
	base := config(t, `{"server_port": 8123, "trusted_proxies": []}`)

	out, changed, err := withTrustedProxies(base, []string{"172.30.32.0/24"})
	if err != nil {
		t.Fatalf("withTrustedProxies: %v", err)
	}
	if !changed {
		t.Fatal("expected a change when the subnet is missing")
	}
	if got := proxies(t, out); !equalStrings(got, []string{"172.30.32.0/24"}) {
		t.Errorf("trusted proxies = %v, want [172.30.32.0/24]", got)
	}
	if string(out[keyUseXForwardedFor]) != "true" {
		t.Errorf("%s = %s, want true", keyUseXForwardedFor, out[keyUseXForwardedFor])
	}
	// Settings we have no opinion on have to survive the round trip, since the
	// configure command replaces the whole config rather than patching it.
	if string(out["server_port"]) != "8123" {
		t.Errorf("server_port = %s, want 8123", out["server_port"])
	}
}

// A run that changes nothing must report exactly that: `changed` is what stops
// every add-on start from restarting Home Assistant.
func TestWithTrustedProxiesIsIdempotent(t *testing.T) {
	base := config(t, `{
		"server_port": 8123,
		"use_x_forwarded_for": true,
		"trusted_proxies": ["172.30.32.0/24", "192.168.1.0/24"]
	}`)

	out, changed, err := withTrustedProxies(base, []string{"172.30.32.0/24"})
	if err != nil {
		t.Fatalf("withTrustedProxies: %v", err)
	}
	if changed {
		t.Error("expected no change when the subnet is already trusted")
	}
	if got := proxies(t, out); !equalStrings(got, []string{"172.30.32.0/24", "192.168.1.0/24"}) {
		t.Errorf("trusted proxies = %v, want them left alone", got)
	}
}

// Trusted proxies without use_x_forwarded_for do nothing, and Core's schema
// rejects one without the other, so a stored config missing the flag is still
// a config that needs changing.
func TestWithTrustedProxiesEnablesForwardingOnItsOwn(t *testing.T) {
	base := config(t, `{"server_port": 8123, "trusted_proxies": ["172.30.32.0/24"]}`)

	out, changed, err := withTrustedProxies(base, []string{"172.30.32.0/24"})
	if err != nil {
		t.Fatalf("withTrustedProxies: %v", err)
	}
	if !changed {
		t.Fatal("expected a change when use_x_forwarded_for is missing")
	}
	if string(out[keyUseXForwardedFor]) != "true" {
		t.Errorf("%s = %s, want true", keyUseXForwardedFor, out[keyUseXForwardedFor])
	}
}

// Core's configure schema has no room for the fields it maintains itself, so a
// config read from the store cannot be sent back as-is.
func TestWithTrustedProxiesDropsMetadata(t *testing.T) {
	base := config(t, `{
		"server_port": 8123,
		"created_at": "2026-08-14T00:00:00+00:00",
		"error": null,
		"error_message": null
	}`)

	out, _, err := withTrustedProxies(base, []string{"172.30.32.0/24"})
	if err != nil {
		t.Fatalf("withTrustedProxies: %v", err)
	}
	for _, key := range metaKeys {
		if _, ok := out[key]; ok {
			t.Errorf("%s survived into the config to send", key)
		}
	}
}

// Rewriting an entry into our own formatting would read as a config change to
// Core and cost a restart for nothing, so a subnet already trusted under a
// different spelling counts as present.
func TestWithTrustedProxiesMatchesEquivalentSpellings(t *testing.T) {
	base := config(t, `{"use_x_forwarded_for": true, "trusted_proxies": ["172.30.32.1/32"]}`)

	out, changed, err := withTrustedProxies(base, []string{"172.30.32.1"})
	if err != nil {
		t.Fatalf("withTrustedProxies: %v", err)
	}
	if changed {
		t.Error("expected a bare address to match the single-host network it means")
	}
	if got := proxies(t, out); !equalStrings(got, []string{"172.30.32.1/32"}) {
		t.Errorf("trusted proxies = %v, want the stored spelling kept", got)
	}
}

// Core validated whatever is in the store, so an entry this code cannot read is
// still the user's and must not take the whole run down with it.
func TestWithTrustedProxiesKeepsUnreadableEntries(t *testing.T) {
	base := config(t, `{"use_x_forwarded_for": true, "trusted_proxies": ["not-a-network"]}`)

	out, changed, err := withTrustedProxies(base, []string{"172.30.32.0/24"})
	if err != nil {
		t.Fatalf("withTrustedProxies: %v", err)
	}
	if !changed {
		t.Fatal("expected the new subnet to be added")
	}
	if got := proxies(t, out); !equalStrings(got, []string{"not-a-network", "172.30.32.0/24"}) {
		t.Errorf("trusted proxies = %v, want the unreadable entry kept", got)
	}
}

func TestNormalizeNetwork(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "10.0.0.0/24", want: "10.0.0.0/24"},
		{in: "10.0.0.1", want: "10.0.0.1/32"},
		{in: "172.30.32.2", want: "172.30.32.2/32"},
		{in: "fd00::", want: "fd00::/128"},
		{in: "fd00::/64", want: "fd00::/64"},
		// Core runs entries through ip_network(), which rejects host bits set
		// past the prefix rather than masking them away.
		{in: "10.0.0.5/24", wantErr: true},
		{in: "not-a-network", wantErr: true},
		{in: "", wantErr: true},
	}

	for _, tc := range tests {
		got, err := normalizeNetwork(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeNetwork(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeNetwork(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeNetwork(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A pending config that failed its trial keeps an error and is never applied
// again, so building on it would stage a config Core has already rejected.
func TestConfigOnTrial(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "no error recorded", raw: `{"error": null}`, want: true},
		{name: "no error field", raw: `{"server_port": 8123}`, want: true},
		{name: "failed to apply", raw: `{"error": "apply_failed"}`, want: false},
		{name: "never confirmed", raw: `{"error": "not_promoted"}`, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := config(t, tc.raw).onTrial(); got != tc.want {
				t.Errorf("onTrial() = %v, want %v", got, tc.want)
			}
		})
	}

	// No pending config at all is not a trial either.
	if haConfig(nil).onTrial() {
		t.Error("onTrial() = true for a missing pending config, want false")
	}
}
