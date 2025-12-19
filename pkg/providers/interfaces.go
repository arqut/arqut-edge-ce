package providers

import (
	"context"
	"time"

	"github.com/arqut/arqut-edge-ce/pkg/models"
)

// AuthProvider defines authentication operations
type AuthProvider interface {
	// Authenticate validates user credentials and returns a token
	Authenticate(ctx context.Context, username, password string) (string, error)
	// ValidateToken verifies a token and returns the username
	ValidateToken(ctx context.Context, token string) (string, error)
}

// ACLProvider defines access control operations
type ACLProvider interface {
	// CheckPermission verifies if a user has permission for a resource/action
	CheckPermission(ctx context.Context, username, resource, action string) (bool, error)
	// ListPermissions returns all permissions for a user
	ListPermissions(ctx context.Context, username string) ([]Permission, error)
}

// Permission represents a user permission
type Permission struct {
	Resource string
	Action   string
}

// AnalyticsProvider defines analytics operations
type AnalyticsProvider interface {
	// Track records an analytics event
	Track(ctx context.Context, event Event) error
	// GetMetrics retrieves metrics for a given query
	GetMetrics(ctx context.Context, query MetricsQuery) (*MetricsResult, error)
}

// Event represents an analytics event
type Event struct {
	Type      string
	Timestamp time.Time
	UserID    string
	Data      map[string]any
}

// MetricsQuery defines parameters for metrics retrieval
type MetricsQuery struct {
	StartTime  time.Time
	EndTime    time.Time
	EventTypes []string
}

// MetricsResult contains aggregated metrics
type MetricsResult struct {
	Data  map[string]any
	Count int64
}

// IntegrationProvider defines external integration operations
type IntegrationProvider interface {
	// Send sends data to external systems
	Send(ctx context.Context, destination string, payload any) error
	// Receive receives data from external systems
	Receive(ctx context.Context, source string) (any, error)
}

// ProxyProvider defines proxy service management operations
type ProxyProvider interface {
	// Service Management
	AddService(name, localHost string, localPort int, protocol string) (*models.ProxyService, error)
	ModifyService(id string, config models.ProxyServiceConfig, operations ...string) error
	DeleteService(id string) error
	// GetServices() ([]*storage.ProxyService, error)
	// GetService(id string) (*storage.ProxyService, error)
	// GetServiceByHostPort(host string, port int) (*storage.ProxyService, error)

	// Service Control
	EnableService(id string) error
	DisableService(id string) error

	// Interface Management (for multi-interface support)
	SetInterfaceIPs(ips map[string]string)
	AddInterface(name, ip string)
	RemoveInterface(name string)

	// Utility
	SetPortRange(start, end int)
	Clear() error
}
