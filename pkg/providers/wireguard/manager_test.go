package wireguard

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"

	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/arqut/arqut-edge-ce/pkg/signaling"
)

// testManager builds a Manager with just the fields the turnCreds tests need.
// We bypass NewManager because it generates a real WireGuard keypair and
// spawns the periodic-update goroutine — neither is relevant to the getter
// contract.
func testManager(t *testing.T) *Manager {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &Manager{
		id:     "test-edge",
		ctx:    ctx,
		cancel: cancel,
		logger: logger.New(io.Discard, "test", logger.InfoLevel),
	}
}

// feedTurnResponse marshals creds into the wire format and calls
// handleTurnResponse the same way the signaling client would.
func feedTurnResponse(t *testing.T, m *Manager, creds TurnCredentials) {
	t.Helper()
	raw, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("marshal creds: %v", err)
	}
	msg := &signaling.SignallingMessage{Type: "turn-response", Data: raw}
	if err := m.handleTurnResponse(context.Background(), msg); err != nil {
		t.Fatalf("handleTurnResponse: %v", err)
	}
}

func TestManager_GetTurnCredentials_NilBeforeResponse(t *testing.T) {
	m := testManager(t)
	if got := m.GetTurnCredentials(); got != nil {
		t.Fatalf("expected nil before any handleTurnResponse, got %+v", got)
	}
}

func TestManager_GetTurnCredentials_ReturnsLatest(t *testing.T) {
	m := testManager(t)

	want := TurnCredentials{
		Username: "edge:test-edge:9999999999",
		Password: "hmac-secret",
		TTL:      86400,
		URLs:     []string{"stun:turn.example.com:3478", "turn:turn.example.com:3478?transport=udp"},
	}
	feedTurnResponse(t, m, want)

	got := m.GetTurnCredentials()
	if got == nil {
		t.Fatal("expected creds after handleTurnResponse, got nil")
	}
	if got.Username != want.Username {
		t.Errorf("Username: got %q want %q", got.Username, want.Username)
	}
	if got.Password != want.Password {
		t.Errorf("Password: got %q want %q", got.Password, want.Password)
	}
	if got.TTL != want.TTL {
		t.Errorf("TTL: got %d want %d", got.TTL, want.TTL)
	}
	if len(got.URLs) != len(want.URLs) {
		t.Fatalf("URLs length: got %d want %d", len(got.URLs), len(want.URLs))
	}
	for i := range got.URLs {
		if got.URLs[i] != want.URLs[i] {
			t.Errorf("URLs[%d]: got %q want %q", i, got.URLs[i], want.URLs[i])
		}
	}
}

func TestManager_GetTurnCredentials_ReplacesPrevious(t *testing.T) {
	m := testManager(t)
	feedTurnResponse(t, m, TurnCredentials{Username: "first", URLs: []string{"turn:a"}})
	feedTurnResponse(t, m, TurnCredentials{Username: "second", URLs: []string{"turn:b"}})

	got := m.GetTurnCredentials()
	if got == nil || got.Username != "second" {
		t.Fatalf("expected latest creds (second), got %+v", got)
	}
}

// TestManager_HandleTurnResponse_RaceSafeUnderConcurrentReads runs writes and
// reads in parallel — `go test -race` flags an unlocked write here. With the
// fix in place the run is clean.
func TestManager_HandleTurnResponse_RaceSafeUnderConcurrentReads(t *testing.T) {
	m := testManager(t)

	const writers = 4
	const readers = 8
	const iterations = 200

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				feedTurnResponse(t, m, TurnCredentials{
					Username: "writer", Password: "p", TTL: id*1000 + i,
					URLs: []string{"turn:host:3478"},
				})
			}
		}(w)
	}
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = m.GetTurnCredentials()
			}
		}()
	}
	wg.Wait()
}

// TestManager_GetTurnCredentials_CallerMutationDoesNotPoison verifies the
// getter returns a defensive copy. A caller scribbling on the result must
// not corrupt the manager's stored credentials.
func TestManager_GetTurnCredentials_CallerMutationDoesNotPoison(t *testing.T) {
	m := testManager(t)
	feedTurnResponse(t, m, TurnCredentials{
		Username: "u", Password: "p", TTL: 1,
		URLs: []string{"turn:original"},
	})

	first := m.GetTurnCredentials()
	if first == nil {
		t.Fatal("nil creds")
	}
	first.URLs[0] = "turn:tampered"

	second := m.GetTurnCredentials()
	if second == nil {
		t.Fatal("nil creds on second read")
	}
	if second.URLs[0] != "turn:original" {
		t.Fatalf("manager state was poisoned by caller mutation: %q", second.URLs[0])
	}
}
