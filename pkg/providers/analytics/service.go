package analytics

import (
	"context"
	"sync"

	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/gofiber/fiber/v2"
)

// Service implements analytics service
type Service struct {
	events []providers.Event
	mu     sync.RWMutex
}

// NewService creates a new analytics service
func NewService() *Service {
	return &Service{
		events: make([]providers.Event, 0),
	}
}

// Name returns the service name
func (s *Service) Name() string {
	return "analytics"
}

// Initialize sets up the service
func (s *Service) Initialize(ctx context.Context, registry *providers.Registry) error {
	registry.Logger().Println("Initializing analytics service")
	return nil
}

// IsRunnable returns false for now (could be true if we add event batching/flushing)
func (s *Service) IsRunnable() bool {
	return false
}

// Run is not used for analytics service currently
func (s *Service) Start(ctx context.Context) error {
	return nil
}

// Stop gracefully shuts down the service
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear events on shutdown
	s.events = nil
	return nil
}

// RegisterAPIRoutes registers analytics-related routes
func (s *Service) RegisterAPIRoutes(router fiber.Router, middlewares ...fiber.Handler) {
	// Analytics routes are handled by apiserver for now
	// This can be moved here in the future
}

// Track records an analytics event
func (s *Service) Track(ctx context.Context, event providers.Event) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

// GetMetrics retrieves analytics metrics based on query
func (s *Service) GetMetrics(ctx context.Context, query providers.MetricsQuery) (*providers.MetricsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := int64(0)
	typeFilter := make(map[string]bool)
	for _, t := range query.EventTypes {
		typeFilter[t] = true
	}

	for _, event := range s.events {
		if len(typeFilter) > 0 && !typeFilter[event.Type] {
			continue
		}
		count++
	}

	return &providers.MetricsResult{
		Data: map[string]any{
			"total_events": count,
		},
		Count: count,
	}, nil
}

// Verify that Service implements both Service and AnalyticsProvider interfaces
var (
	_ providers.Service           = (*Service)(nil)
	_ providers.AnalyticsProvider = (*Service)(nil)
)
