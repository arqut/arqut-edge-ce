package core

import (
	"context"

	"github.com/arqut/arqut-edge-ce/pkg/providers"
)

// App defines the core application business logic interface
type App interface {
	// Login authenticates a user and returns their token and permissions
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)

	// CheckAccess verifies if a user has access to a resource
	CheckAccess(ctx context.Context, token, resource, action string) (bool, error)

	// SendData sends data to external integrations
	SendData(ctx context.Context, token, destination string, data interface{}) error

	// GetMetrics retrieves analytics metrics
	GetMetrics(ctx context.Context, token string, query providers.MetricsQuery) (*providers.MetricsResult, error)
}
