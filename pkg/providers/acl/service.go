package acl

import (
	"context"
	"sync"

	"github.com/arqut/arqut-edge-ce/pkg/providers"
)

// Service implements access control service
type Service struct {
	permissions map[string][]providers.Permission // username -> permissions
	mu          sync.RWMutex
}

// NewService creates a new ACL service
func NewService() *Service {
	return &Service{
		permissions: make(map[string][]providers.Permission),
	}
}

// Name returns the service name
func (s *Service) Name() string {
	return "acl"
}

// Initialize sets up the service with default permissions
func (s *Service) Initialize(ctx context.Context, registry *providers.Registry) error {
	registry.Logger().Println("Initializing ACL service with default permissions")

	s.mu.Lock()
	defer s.mu.Unlock()

	// Setup default permissions
	s.permissions["admin"] = []providers.Permission{
		{Resource: "*", Action: "*"},
	}
	s.permissions["user"] = []providers.Permission{
		{Resource: "data", Action: "read"},
		{Resource: "profile", Action: "read"},
		{Resource: "profile", Action: "write"},
	}

	return nil
}

// IsRunnable returns false as ACL service doesn't need background processing
func (s *Service) IsRunnable() bool {
	return false
}

// Run is not used for ACL service
func (s *Service) Start(ctx context.Context) error {
	return nil
}

// Stop gracefully shuts down the service
func (s *Service) Stop(ctx context.Context) error {
	// No cleanup needed for ACL service
	return nil
}

// RegisterAPIRoutes registers ACL-related routes
func (s *Service) RegisterAPIRoutes(app interface{}) error {
	// ACL routes are handled by apiserver for now
	// This can be moved here in the future
	return nil
}

// CheckPermission checks if a user has permission for a resource/action
func (s *Service) CheckPermission(ctx context.Context, username, resource, action string) (bool, error) {
	s.mu.RLock()
	userPerms, exists := s.permissions[username]
	s.mu.RUnlock()

	if !exists {
		return false, nil
	}

	for _, perm := range userPerms {
		if (perm.Resource == "*" || perm.Resource == resource) &&
			(perm.Action == "*" || perm.Action == action) {
			return true, nil
		}
	}

	return false, nil
}

// ListPermissions returns all permissions for a user
func (s *Service) ListPermissions(ctx context.Context, username string) ([]providers.Permission, error) {
	s.mu.RLock()
	perms, exists := s.permissions[username]
	s.mu.RUnlock()

	if !exists {
		return []providers.Permission{}, nil
	}

	result := make([]providers.Permission, len(perms))
	copy(result, perms)
	return result, nil
}

// Verify that Service implements both Service and ACLProvider interfaces
var _ providers.Service = (*Service)(nil)
var _ providers.ACLProvider = (*Service)(nil)
