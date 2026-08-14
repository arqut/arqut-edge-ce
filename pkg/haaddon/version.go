package haaddon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// storeConfigVersion is the first Home Assistant release that manages the http
// integration through its own store instead of configuration.yaml. From this
// release on a `http:` block is imported once and then ignored, so trusted
// proxies have to be set through the websocket API instead.
var storeConfigVersion = haVersion{year: 2026, month: 8}

// haVersion is a Home Assistant calendar version, truncated to the parts that
// decide which configuration mechanism applies.
type haVersion struct {
	year  int
	month int
}

func (v haVersion) String() string {
	return fmt.Sprintf("%d.%d", v.year, v.month)
}

// atLeast reports whether v is other or a later release.
func (v haVersion) atLeast(other haVersion) bool {
	if v.year != other.year {
		return v.year > other.year
	}
	return v.month >= other.month
}

// parseHAVersion reads the leading "YEAR.MONTH" of a Home Assistant version
// string. Patch and pre-release suffixes ("2026.8.0b3") are dropped: they never
// change which configuration mechanism applies.
func parseHAVersion(value string) (haVersion, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ".", 3)
	if len(parts) < 2 {
		return haVersion{}, fmt.Errorf("malformed Home Assistant version %q", value)
	}

	year, err := leadingInt(parts[0])
	if err != nil {
		return haVersion{}, fmt.Errorf("malformed Home Assistant version %q: %w", value, err)
	}
	month, err := leadingInt(parts[1])
	if err != nil {
		return haVersion{}, fmt.Errorf("malformed Home Assistant version %q: %w", value, err)
	}

	return haVersion{year: year, month: month}, nil
}

// readHAVersion reads the version of the Core instance this add-on runs
// alongside from the marker file Core keeps in its config directory.
func readHAVersion() (haVersion, error) {
	raw, err := os.ReadFile(filepath.Join(haConfigDir, ".HA_VERSION"))
	if err != nil {
		return haVersion{}, err
	}
	return parseHAVersion(string(raw))
}

// leadingInt parses the digits at the start of s, ignoring any suffix. Dev and
// pre-release builds can carry one ("2026.8.0.dev0"), and rejecting the whole
// version over it would leave us without a mechanism to pick.
func leadingInt(s string) (int, error) {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, fmt.Errorf("%q does not start with a number", s)
	}
	return strconv.Atoi(s[:end])
}
