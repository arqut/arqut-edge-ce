package haaddon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"github.com/gorilla/websocket"
)

const (
	// supervisorHost is the name Supervisor gives itself on the add-on network.
	// It does not resolve when the add-on runs with host_network, so
	// supervisorFallbackHost - Supervisor's fixed address on the hassio bridge,
	// which stays reachable from the host namespace - is tried next.
	supervisorHost         = "supervisor"
	supervisorFallbackHost = "172.30.32.2"

	handshakeTimeout = 10 * time.Second
	callTimeout      = 30 * time.Second
)

// errAuth marks a rejected token, as opposed to an unreachable Core. Retrying
// or falling back to another address cannot fix it.
var errAuth = errors.New("authentication rejected")

// errUnreachable marks a Core that could not be reached at all. The add-on is
// started before Core, so early on this says "not yet" rather than "broken".
var errUnreachable = errors.New("Home Assistant is unreachable")

// supervisorToken returns the token Supervisor injects into add-ons.
func supervisorToken() (string, error) {
	token := os.Getenv("SUPERVISOR_TOKEN")
	if token == "" {
		return "", errors.New("SUPERVISOR_TOKEN is not set; the add-on needs `homeassistant_api: true` in its config")
	}
	return token, nil
}

// haClient is a Core websocket connection, authenticated and ready for
// commands. It is deliberately single-use and synchronous: everything here is
// a short request/response exchange around a restart.
type haClient struct {
	conn *websocket.Conn
	// version is what Core reports during the handshake.
	version haVersion
	nextID  int
}

// dialHA connects to Core through the Supervisor proxy, trying each address
// Supervisor may be reachable at.
func dialHA(ctx context.Context, token string) (*haClient, error) {
	var lastErr error
	for _, host := range []string{supervisorHost, supervisorFallbackHost} {
		client, err := dialHAHost(ctx, host, token)
		if err == nil {
			return client, nil
		}
		if errors.Is(err, errAuth) {
			return nil, err
		}
		log.Debugf("Could not reach Home Assistant via %s: %v", host, err)
		lastErr = err
	}
	return nil, lastErr
}

func dialHAHost(ctx context.Context, host, token string) (*haClient, error) {
	url := fmt.Sprintf("ws://%s/core/websocket", host)

	dialer := websocket.Dialer{HandshakeTimeout: handshakeTimeout}
	conn, resp, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("dial %s: %w: %w (HTTP %s)", url, errUnreachable, err, resp.Status)
		}
		return nil, fmt.Errorf("dial %s: %w: %w", url, errUnreachable, err)
	}

	client := &haClient{conn: conn, nextID: 1}
	if err := client.authenticate(token); err != nil {
		client.close()
		return nil, fmt.Errorf("authenticate against %s: %w", url, err)
	}
	return client, nil
}

// redialHA waits for Core to accept connections again after the restart that
// applies a pending config. Core starts its revert timer as soon as it loads
// that config, so this gives up well inside the revert window instead of
// reconnecting into a config that is already gone.
func redialHA(ctx context.Context, token string) (*haClient, error) {
	const (
		pollInterval = 3 * time.Second
		pollTimeout  = 3 * time.Minute
	)

	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()

	lastErr := errors.New("no connection attempt completed")
	for {
		// Wait first: Core takes a moment to drop its listener, and connecting
		// to the server that is still shutting down looks like a success right
		// up until the first command fails.
		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			return nil, fmt.Errorf("Home Assistant did not come back within %s: %w", pollTimeout, lastErr)
		}

		// Every error is worth retrying here, including a rejected token: while
		// Core is down the Supervisor proxy has no session to authenticate
		// against and reports that as a refusal rather than a dial failure.
		client, err := dialHA(ctx, token)
		if err == nil {
			return client, nil
		}
		lastErr = err
	}
}

// whenReady runs work until it stops reporting that Home Assistant is not
// there yet.
//
// Supervisor starts this add-on in the services phase, ahead of Core, so on a
// cold boot Core is first unreachable and then busy starting. Neither is a
// failure worth giving up over: they are the normal order of events, and Core
// can only finish starting once the add-ons before it are up.
func whenReady(ctx context.Context, work func(context.Context) error) error {
	const (
		retryInterval = 10 * time.Second
		retryTimeout  = 10 * time.Minute
	)

	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()

	for {
		err := work(ctx)
		if !errors.Is(err, errUnreachable) && !errors.Is(err, errCoreNotRunning) {
			return err
		}

		log.Debugf("Home Assistant is not ready yet (%v); retrying in %s", err, retryInterval)
		select {
		case <-time.After(retryInterval):
		case <-ctx.Done():
			return fmt.Errorf("Home Assistant was not ready within %s: %w", retryTimeout, err)
		}
	}
}

func (c *haClient) close() {
	if c == nil || c.conn == nil {
		return
	}
	c.conn.Close()
	c.conn = nil
}

// authMessage covers every frame exchanged before the connection is usable.
type authMessage struct {
	Type        string `json:"type"`
	HAVersion   string `json:"ha_version,omitempty"`
	Message     string `json:"message,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
}

func (c *haClient) authenticate(token string) error {
	var greeting authMessage
	if err := c.read(&greeting); err != nil {
		return fmt.Errorf("read greeting: %w", err)
	}
	if greeting.Type != "auth_required" {
		return fmt.Errorf("unexpected greeting %q", greeting.Type)
	}

	if err := c.write(authMessage{Type: "auth", AccessToken: token}); err != nil {
		return fmt.Errorf("send credentials: %w", err)
	}

	var reply authMessage
	if err := c.read(&reply); err != nil {
		return fmt.Errorf("read authentication reply: %w", err)
	}
	switch reply.Type {
	case "auth_ok":
	case "auth_invalid":
		return fmt.Errorf("%w: %s", errAuth, reply.Message)
	default:
		return fmt.Errorf("unexpected authentication reply %q", reply.Type)
	}

	// Core reports its version here, which saves reading it off disk when the
	// config directory is not mapped into the add-on.
	if version, err := parseHAVersion(reply.HAVersion); err == nil {
		c.version = version
	} else {
		log.Debugf("Could not parse the version Home Assistant reported: %v", err)
	}
	return nil
}

// wsError is the failure detail Core attaches to an unsuccessful command.
type wsError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *wsError) Error() string {
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

type wsResult struct {
	ID      int             `json:"id"`
	Type    string          `json:"type"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result"`
	Error   *wsError        `json:"error"`
}

// call runs a command and returns its result payload.
func (c *haClient) call(command map[string]any) (json.RawMessage, error) {
	id := c.nextID
	c.nextID++

	command["id"] = id
	if err := c.write(command); err != nil {
		return nil, fmt.Errorf("send %s: %w", command["type"], err)
	}

	for {
		var message wsResult
		if err := c.read(&message); err != nil {
			return nil, fmt.Errorf("read reply to %s: %w", command["type"], err)
		}
		// Core shares the connection with anything else it wants to push; only
		// the reply to this command settles it.
		if message.ID != id || message.Type != "result" {
			continue
		}
		if !message.Success {
			if message.Error != nil {
				return nil, message.Error
			}
			return nil, fmt.Errorf("%s failed without a reason", command["type"])
		}
		return message.Result, nil
	}
}

func (c *haClient) read(value any) error {
	if err := c.conn.SetReadDeadline(time.Now().Add(callTimeout)); err != nil {
		return err
	}
	return c.conn.ReadJSON(value)
}

func (c *haClient) write(value any) error {
	if err := c.conn.SetWriteDeadline(time.Now().Add(callTimeout)); err != nil {
		return err
	}
	return c.conn.WriteJSON(value)
}
