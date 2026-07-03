package repositories

import (
	"testing"

	"github.com/arqut/arqut-edge-ce/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// names extracts the Name field from a slice of services, for readable
// expected/actual diffs in assertion failures.
func names(svcs []*models.ProxyService) []string {
	out := make([]string, len(svcs))
	for i, s := range svcs {
		out[i] = s.Name
	}
	return out
}

// newTestRepo builds an in-memory SQLite-backed repo with a deterministic
// seed: 5 services across both protocols and both enabled states so the
// pagination/filter assertions below have something to bite into.
func newTestRepo(t *testing.T) *ServiceRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	r := NewServiceRepository(db)
	// Names chosen so alphabetical ordering puts them in this exact order:
	// "Alpha", "Bravo", "Charlie", "Delta", "Echo".
	seed := []struct {
		name      string
		localHost string
		localPort int
		tunnel    int
		protocol  string
		enabled   bool
	}{
		{"Alpha", "127.0.0.1", 8001, 9001, "http", true},
		{"Bravo", "127.0.0.1", 8002, 9002, "http", false},
		{"Charlie", "127.0.0.1", 8003, 9003, "websocket", true},
		{"Delta", "127.0.0.1", 8004, 9004, "websocket", true},
		{"Echo", "127.0.0.1", 8005, 9005, "http", true},
	}
	for _, s := range seed {
		svc, err := r.AddService(s.name, s.localHost, s.localPort, s.tunnel, s.protocol, false, nil, nil)
		if err != nil {
			t.Fatalf("seed %s: %v", s.name, err)
		}
		if !s.enabled {
			// AddService stores enabled=true by default; flip directly via GORM.
			if err := db.Model(svc).Update("enabled", false).Error; err != nil {
				t.Fatalf("disable %s: %v", s.name, err)
			}
		}
	}
	return r
}

func TestListServicesPaginated_NoFilter(t *testing.T) {
	r := newTestRepo(t)

	// Page 1, size 3 → first three alphabetically.
	got, total, err := r.ListServicesPaginated(1, 3, ServiceFilter{})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(got) != 3 {
		t.Fatalf("len(page1) = %d, want 3", len(got))
	}
	wantNames := []string{"Alpha", "Bravo", "Charlie"}
	for i, w := range wantNames {
		if got[i].Name != w {
			t.Errorf("page1[%d].Name = %q, want %q", i, got[i].Name, w)
		}
	}

	// Page 2, size 3 → last two.
	got, total, err = r.ListServicesPaginated(2, 3, ServiceFilter{})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(got) != 2 {
		t.Fatalf("len(page2) = %d, want 2", len(got))
	}
	if got[0].Name != "Delta" || got[1].Name != "Echo" {
		t.Errorf("page2 = [%s, %s], want [Delta, Echo]", got[0].Name, got[1].Name)
	}
}

func TestListServicesPaginated_NameFilter(t *testing.T) {
	r := newTestRepo(t)

	// Substring match, case-insensitive.
	got, total, err := r.ListServicesPaginated(1, 10, ServiceFilter{Name: "a"})
	if err != nil {
		t.Fatalf("name=a: %v", err)
	}
	// Alpha, Bravo, Charlie, Delta — all contain 'a'.
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
}

func TestListServicesPaginated_ProtocolFilter(t *testing.T) {
	r := newTestRepo(t)

	got, total, err := r.ListServicesPaginated(1, 10, ServiceFilter{Protocol: "websocket"})
	if err != nil {
		t.Fatalf("protocol=websocket: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(got) != 2 || got[0].Name != "Charlie" || got[1].Name != "Delta" {
		t.Errorf("got %v, want [Charlie Delta]", names(got))
	}
}

func TestListServicesPaginated_EnabledFilter(t *testing.T) {
	r := newTestRepo(t)

	tru := true
	got, total, err := r.ListServicesPaginated(1, 10, ServiceFilter{Enabled: &tru})
	if err != nil {
		t.Fatalf("enabled=true: %v", err)
	}
	// Alpha, Charlie, Delta, Echo (Bravo was disabled).
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}

	fal := false
	got, total, err = r.ListServicesPaginated(1, 10, ServiceFilter{Enabled: &fal})
	if err != nil {
		t.Fatalf("enabled=false: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].Name != "Bravo" {
		t.Errorf("got %v (total=%d), want [Bravo] (total=1)", names(got), total)
	}
}

func TestListServicesPaginated_CombinedFilters(t *testing.T) {
	r := newTestRepo(t)

	tru := true
	got, total, err := r.ListServicesPaginated(1, 10, ServiceFilter{
		Protocol: "http",
		Enabled:  &tru,
	})
	if err != nil {
		t.Fatalf("combined: %v", err)
	}
	// Of the http services (Alpha, Bravo, Echo), only Alpha + Echo are enabled.
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(got) != 2 || got[0].Name != "Alpha" || got[1].Name != "Echo" {
		t.Errorf("got %v, want [Alpha Echo]", names(got))
	}
}

func TestListServicesPaginated_BeyondLastPage(t *testing.T) {
	r := newTestRepo(t)

	got, total, err := r.ListServicesPaginated(99, 10, ServiceFilter{})
	if err != nil {
		t.Fatalf("page 99: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0 on out-of-range page", len(got))
	}
}
