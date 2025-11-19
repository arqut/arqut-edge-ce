package integration

import (
	"context"
	"sync"

	"github.com/arqut/arqut-edge-ce/pkg/providers"
)

// Service implements integration service
type Service struct {
	mu       sync.RWMutex
	registry *providers.Registry
}

// NewService creates a new integration service
func NewService() *Service {
	return &Service{}
}

// Name returns the service name
func (s *Service) Name() string {
	return "integration"
}

// Initialize sets up the service
func (s *Service) Initialize(ctx context.Context, registry *providers.Registry) error {
	s.registry = registry
	registry.Logger().Println("Initializing integration service")
	return nil
}

// IsRunnable returns false as integration service doesn't need background processing
func (s *Service) IsRunnable() bool {
	return false
}

// Run is not used for integration service
func (s *Service) Start(ctx context.Context) error {
	return nil
}

// Stop gracefully shuts down the service
func (s *Service) Stop(ctx context.Context) error {
	// No cleanup needed for integration service
	return nil
}

// RegisterAPIRoutes registers integration-related routes
func (s *Service) RegisterAPIRoutes(app interface{}) error {
	// Integration routes are handled by apiserver for now
	// This can be moved here in the future
	return nil
}

// Send sends data to an external destination
func (s *Service) Send(ctx context.Context, destination string, payload interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Basic implementation: just log the send
	if s.registry != nil {
		s.registry.Logger().Printf("Sending to %s: %v", destination, payload)
	}
	return nil
}

// Receive receives data from an external source
func (s *Service) Receive(ctx context.Context, source string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Basic implementation: return empty data
	return map[string]interface{}{
		"source": source,
		"data":   nil,
	}, nil
}

// Verify that Service implements both Service and IntegrationProvider interfaces
var _ providers.Service = (*Service)(nil)
var _ providers.IntegrationProvider = (*Service)(nil)
