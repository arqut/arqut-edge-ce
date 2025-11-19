package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/arqut/arqut-edge-ce/pkg/providers"
)

// MainApp is the main application implementation
type MainApp struct {
	providers *providers.Registry
}

// NewMainApp creates a new main application instance
func NewMainApp(p *providers.Registry) *MainApp {
	return &MainApp{
		providers: p,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string
	Password string
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token       string
	Username    string
	Permissions []providers.Permission
}

// Login authenticates a user and returns their token and permissions
func (a *MainApp) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, errors.New("username and password are required")
	}

	// Get auth provider
	auth, err := a.providers.GetAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth provider: %w", err)
	}

	// Authenticate user
	token, err := auth.Authenticate(ctx, req.Username, req.Password)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Get ACL provider
	acl, err := a.providers.GetACL()
	if err != nil {
		return nil, fmt.Errorf("failed to get ACL provider: %w", err)
	}

	// Get user permissions
	permissions, err := acl.ListPermissions(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Get analytics provider
	analytics, _ := a.providers.GetAnalytics()

	// Track login event
	_ = analytics.Track(ctx, providers.Event{
		Type:   "login",
		UserID: req.Username,
		Data: map[string]interface{}{
			"success": true,
		},
	})

	return &LoginResponse{
		Token:       token,
		Username:    req.Username,
		Permissions: permissions,
	}, nil
}

// CheckAccess verifies if a user has access to a resource
func (a *MainApp) CheckAccess(ctx context.Context, token, resource, action string) (bool, error) {
	// Get auth provider
	auth, err := a.providers.GetAuth()
	if err != nil {
		return false, fmt.Errorf("failed to get auth provider: %w", err)
	}

	// Validate token
	username, err := auth.ValidateToken(ctx, token)
	if err != nil {
		return false, fmt.Errorf("invalid token: %w", err)
	}

	// Get ACL provider
	acl, err := a.providers.GetACL()
	if err != nil {
		return false, fmt.Errorf("failed to get ACL provider: %w", err)
	}

	// Check permission
	hasAccess, err := acl.CheckPermission(ctx, username, resource, action)
	if err != nil {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	// Get analytics provider
	analytics, _ := a.providers.GetAnalytics()

	// Track access check
	_ = analytics.Track(ctx, providers.Event{
		Type:   "access_check",
		UserID: username,
		Data: map[string]interface{}{
			"resource":   resource,
			"action":     action,
			"has_access": hasAccess,
		},
	})

	return hasAccess, nil
}

// SendData sends data to external integrations
func (a *MainApp) SendData(ctx context.Context, token, destination string, data interface{}) error {
	// Get auth provider
	auth, err := a.providers.GetAuth()
	if err != nil {
		return fmt.Errorf("failed to get auth provider: %w", err)
	}

	// Validate token
	username, err := auth.ValidateToken(ctx, token)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	// Get ACL provider
	acl, err := a.providers.GetACL()
	if err != nil {
		return fmt.Errorf("failed to get ACL provider: %w", err)
	}

	// Check permission
	hasAccess, err := acl.CheckPermission(ctx, username, "integrations", "write")
	if err != nil {
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		return errors.New("access denied")
	}

	// Get integration provider
	integration, err := a.providers.GetIntegration()
	if err != nil {
		return fmt.Errorf("failed to get integration provider: %w", err)
	}

	// Send data
	err = integration.Send(ctx, destination, data)
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}

	// Get analytics provider
	analytics, _ := a.providers.GetAnalytics()

	// Track send event
	_ = analytics.Track(ctx, providers.Event{
		Type:   "integration_send",
		UserID: username,
		Data: map[string]interface{}{
			"destination": destination,
		},
	})

	return nil
}

// GetMetrics retrieves analytics metrics
func (a *MainApp) GetMetrics(ctx context.Context, token string, query providers.MetricsQuery) (*providers.MetricsResult, error) {
	// Get auth provider
	auth, err := a.providers.GetAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth provider: %w", err)
	}

	// Validate token
	username, err := auth.ValidateToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Get ACL provider
	acl, err := a.providers.GetACL()
	if err != nil {
		return nil, fmt.Errorf("failed to get ACL provider: %w", err)
	}

	// Check permission
	hasAccess, err := acl.CheckPermission(ctx, username, "analytics", "read")
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	if !hasAccess {
		return nil, errors.New("access denied")
	}

	// Get analytics provider
	analytics, err := a.providers.GetAnalytics()
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics provider: %w", err)
	}

	// Get metrics
	result, err := analytics.GetMetrics(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	return result, nil
}

// Verify that MainApp implements App interface
var _ App = (*MainApp)(nil)
