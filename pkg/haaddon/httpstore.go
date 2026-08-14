package haaddon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/gofiber/fiber/v2/log"
)

const (
	cmdGetConfig = "http/config"
	cmdConfigure = "http/config/configure"
	cmdPromote   = "http/config/promote"

	keyUseXForwardedFor = "use_x_forwarded_for"
	keyTrustedProxies   = "trusted_proxies"
	keyError            = "error"

	// codeNotRunning is what Core answers while it is still starting. Applying
	// a config means restarting, and restarting a start that has not finished
	// leaves half-configured integrations behind, so it refuses until it is up.
	codeNotRunning = "not_running"
)

// errCoreNotRunning marks a Core that is up but not finished starting.
var errCoreNotRunning = errors.New("Home Assistant has not finished starting")

// metaKeys are the bookkeeping fields Core attaches to a stored config. Its
// configure schema does not accept them, so a config read from the store has to
// be stripped of them before it can be sent back.
var metaKeys = []string{"created_at", "error", "error_message"}

// haConfig is one stored http configuration. Values stay as raw JSON so that
// the settings we have no opinion on survive the round trip exactly as Core
// wrote them.
type haConfig map[string]json.RawMessage

// storeState is what Core reports about its http configuration: the last
// config confirmed to work, the one currently on trial, and which of them the
// running server was started with.
type storeState struct {
	Stable           haConfig `json:"stable"`
	Pending          haConfig `json:"pending"`
	RevertAt         *string  `json:"revert_at"`
	ActiveConfigType string   `json:"active_config_type"`
}

// onTrial reports whether the pending config is one Core will still apply. A
// pending config that already failed its trial keeps an error and is never
// applied again, which makes it useless to build on.
func (c haConfig) onTrial() bool {
	if c == nil {
		return false
	}
	raw, ok := c[keyError]
	if !ok {
		return true
	}
	var code *string
	if err := json.Unmarshal(raw, &code); err != nil {
		return false
	}
	return code == nil
}

// withoutMeta copies the config without the fields Core maintains itself.
func (c haConfig) withoutMeta() haConfig {
	out := make(haConfig, len(c))
	for key, value := range c {
		out[key] = value
	}
	for _, key := range metaKeys {
		delete(out, key)
	}
	return out
}

// reconcileTrustedProxies makes Core trust the given proxy subnets, waiting out
// a Core that is not ready to be configured yet.
func reconcileTrustedProxies(ctx context.Context, subnets []string) error {
	return whenReady(ctx, func(ctx context.Context) error {
		return configureTrustedProxies(ctx, subnets)
	})
}

// configureTrustedProxies makes Core trust the given proxy subnets, through the
// store the http integration uses from 2026.8 on.
//
// Applying a config is a trial: Core stages it as pending, restarts into it,
// and reverts to the last confirmed config a few minutes later unless it is
// promoted. The round trip is therefore configure -> wait out the restart ->
// reconnect -> promote. It is resumable by design, so a run that finds a
// matching pending config left over from an interrupted attempt only has to
// confirm it.
func configureTrustedProxies(ctx context.Context, subnets []string) error {
	token, err := supervisorToken()
	if err != nil {
		return err
	}

	client, err := dialHA(ctx, token)
	if err != nil {
		return err
	}
	defer func() { client.close() }()

	state, err := client.httpConfig()
	if err != nil {
		return fmt.Errorf("read the HTTP configuration: %w", err)
	}

	// A pending config that has not failed its trial is what Core is running,
	// or is about to, so extend that one. Anything else means starting from the
	// last config known to work.
	base := state.Stable
	onTrial := state.Pending.onTrial()
	if onTrial {
		base = state.Pending
	}

	desired, changed, err := withTrustedProxies(base, subnets)
	if err != nil {
		return err
	}

	if !changed {
		if !onTrial {
			log.Info("Home Assistant already trusts the Arqut Edge proxy")
			return nil
		}
		// Core is on trial with a config that already trusts us, so confirming
		// it is all that is left. Skipping this would let it revert.
		log.Info("Confirming the pending Home Assistant HTTP configuration")
		return client.promote()
	}

	restart, err := client.configure(desired)
	if err != nil {
		return fmt.Errorf("apply the HTTP configuration: %w", err)
	}

	if restart {
		log.Info("Home Assistant is restarting to trust the Arqut Edge proxy")
		client.close()
		if client, err = redialHA(ctx, token); err != nil {
			return fmt.Errorf("reconnect after the restart: %w", err)
		}
	}

	return client.promote()
}

// withTrustedProxies returns base extended to trust subnets, and reports
// whether anything had to change. Entries already there are kept verbatim:
// rewriting them into our own formatting would read as a config change to Core
// and cost a restart for nothing.
func withTrustedProxies(base haConfig, subnets []string) (haConfig, bool, error) {
	out := base.withoutMeta()
	changed := false

	var trusted []string
	if raw, ok := out[keyTrustedProxies]; ok {
		if err := json.Unmarshal(raw, &trusted); err != nil {
			return nil, false, fmt.Errorf("read %s: %w", keyTrustedProxies, err)
		}
	}

	known := make(map[string]struct{}, len(trusted))
	for _, proxy := range trusted {
		normalized, err := normalizeNetwork(proxy)
		if err != nil {
			// Core validated whatever is stored here, so an entry we cannot
			// read is ours to leave alone rather than to reject.
			log.Warnf("Leaving unrecognised trusted proxy %q in the Home Assistant configuration alone", proxy)
			continue
		}
		known[normalized] = struct{}{}
	}

	for _, subnet := range subnets {
		normalized, err := normalizeNetwork(subnet)
		if err != nil {
			return nil, false, fmt.Errorf("invalid subnet %q: %w", subnet, err)
		}
		if _, ok := known[normalized]; ok {
			continue
		}
		known[normalized] = struct{}{}
		trusted = append(trusted, normalized)
		changed = true
	}

	if changed {
		encoded, err := json.Marshal(trusted)
		if err != nil {
			return nil, false, fmt.Errorf("encode %s: %w", keyTrustedProxies, err)
		}
		out[keyTrustedProxies] = encoded
	}

	// Core's schema treats the two as inclusive: neither is accepted without
	// the other, and trusted proxies do nothing on their own.
	forwarded := false
	if raw, ok := out[keyUseXForwardedFor]; ok {
		if err := json.Unmarshal(raw, &forwarded); err != nil {
			return nil, false, fmt.Errorf("read %s: %w", keyUseXForwardedFor, err)
		}
	}
	if !forwarded {
		out[keyUseXForwardedFor] = json.RawMessage("true")
		changed = true
	}

	return out, changed, nil
}

// normalizeNetwork renders a trusted proxy the way Core stores it, so entries
// compare equal however they were written. Core runs each one through Python's
// ip_network(), which turns a bare address into a single-host network and
// rejects anything with bits set past the prefix.
func normalizeNetwork(value string) (string, error) {
	if ip := net.ParseIP(value); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return (&net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)}).String(), nil
		}
		return (&net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}).String(), nil
	}

	address, network, err := net.ParseCIDR(value)
	if err != nil {
		return "", err
	}
	if !address.Equal(network.IP) {
		return "", fmt.Errorf("%s sets bits beyond its prefix", value)
	}
	return network.String(), nil
}

func (c *haClient) httpConfig() (*storeState, error) {
	raw, err := c.call(map[string]any{"type": cmdGetConfig})
	if err != nil {
		return nil, err
	}

	var state storeState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("decode the HTTP configuration: %w", err)
	}
	return &state, nil
}

// configure stages config as the pending one and reports whether Core is
// restarting to apply it.
func (c *haClient) configure(config haConfig) (bool, error) {
	raw, err := c.call(map[string]any{"type": cmdConfigure, "config": config})
	if err != nil {
		var failure *wsError
		if errors.As(err, &failure) && failure.Code == codeNotRunning {
			return false, fmt.Errorf("%w: %s", errCoreNotRunning, failure.Message)
		}
		return false, err
	}

	var result struct {
		Restart bool `json:"restart"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return false, fmt.Errorf("decode the configure result: %w", err)
	}
	return result.Restart, nil
}

// promote confirms the pending config, which is what stops Core reverting it.
func (c *haClient) promote() error {
	if _, err := c.call(map[string]any{"type": cmdPromote}); err != nil {
		return fmt.Errorf("confirm the HTTP configuration: %w", err)
	}
	log.Info("Home Assistant now trusts the Arqut Edge proxy")
	return nil
}
