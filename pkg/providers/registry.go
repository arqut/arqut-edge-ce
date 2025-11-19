package providers

import (
	"context"
	"fmt"

	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/arqut/arqut-edge-ce/pkg/signaling"
	"github.com/arqut/arqut-edge-ce/pkg/storage"
	"github.com/gofiber/fiber/v2"
)

// Service is the base interface that all providers must implement
type Service interface {
	// Name returns unique service identifier (constant)
	Name() string

	// Initialize sets up the service with dependencies from registry
	Initialize(ctx context.Context, registry *Registry) error

	// IsRunnable indicates if service needs to run in background
	IsRunnable() bool

	// Start starts the service (only called if IsRunnable returns true)
	Start(ctx context.Context) error

	// Stop gracefully shuts down the service
	Stop(ctx context.Context) error

	// RegisterAPIRoutes registers HTTP routes for this service
	// The app parameter is typically *fiber.App but uses interface{} to avoid circular imports
	RegisterAPIRoutes(app interface{}) error
}

// Registry manages service lifecycle and dependencies
type Registry struct {
	services  map[string]Service
	runnable  []Service
	db        storage.Storage
	logger    *logger.Logger
	config    interface{}
	sigClient *signaling.Client // Signaling client for cloud connectivity (can be nil if not configured)
}

// NewRegistry creates a new service registry
func NewRegistry(db storage.Storage, log *logger.Logger, cfg interface{}, sigClient *signaling.Client) *Registry {
	return &Registry{
		services:  make(map[string]Service),
		runnable:  make([]Service, 0),
		db:        db,
		logger:    log,
		config:    cfg,
		sigClient: sigClient,
	}
}

// MustRegister registers a service and panics on error (for convenience in main)
func (r *Registry) MustRegister(service Service) {
	if err := r.Register(service); err != nil {
		panic(fmt.Sprintf("Failed to register service %s: %v", service.Name(), err))
	}
}

// DB returns the database storage
func (r *Registry) DB() storage.Storage {
	return r.db
}

// Logger returns the logger
func (r *Registry) Logger() *logger.Logger {
	return r.logger
}


func (r *Registry) Config() interface{} {
	return r.config
}

// SignalingClient returns the signaling client (can be nil if not configured)
func (r *Registry) SignalingClient() *signaling.Client {
	return r.sigClient
}

// Register adds a service to the registry (before initialization)
func (r *Registry) Register(service Service) error {
	name := service.Name()
	if _, exists := r.services[name]; exists {
		return fmt.Errorf("service %s already registered", name)
	}

	r.services[name] = service

	if service.IsRunnable() {
		r.runnable = append(r.runnable, service)
	}

	return nil
}

// InitializeAll initializes all services
func (r *Registry) InitializeAll(ctx context.Context) error {
	r.logger.Info("Initializing services...")

	for name, service := range r.services {
		r.logger.Info("Initializing service: %s", name)
		if err := service.Initialize(ctx, r); err != nil {
			return fmt.Errorf("failed to initialize service %s: %w", name, err)
		}
	}

	r.logger.Info("All %d services initialized successfully", len(r.services))
	return nil
}

// StartRunnable starts all background services
func (r *Registry) StartRunnable(ctx context.Context) error {
	if len(r.runnable) == 0 {
		r.logger.Info("No runnable services to start")
		return nil
	}

	r.logger.Info("Starting %d runnable services...", len(r.runnable))

	for _, service := range r.runnable {
		r.logger.Info("Starting service: %s", service.Name())

		// Start each service in its own goroutine
		go func(s Service) {
			if err := s.Start(ctx); err != nil {
				r.logger.Error("Service %s stopped with error: %v", s.Name(), err)
			}
		}(service)
	}

	r.logger.Info("All runnable services started")
	return nil
}

// Shutdown gracefully stops all services in reverse order
func (r *Registry) Shutdown(ctx context.Context) error {
	r.logger.Info("Shutting down services...")

	// Stop runnable services first
	for i := len(r.runnable) - 1; i >= 0; i-- {
		service := r.runnable[i]
		r.logger.Info("Stopping service: %s", service.Name())
		if err := service.Stop(ctx); err != nil {
			r.logger.Error("Error stopping service %s: %v", service.Name(), err)
		}
	}

	// Stop all other services
	for name, service := range r.services {
		if !service.IsRunnable() {
			r.logger.Info("Stopping service: %s", name)
			if err := service.Stop(ctx); err != nil {
				r.logger.Error("Error stopping service %s: %v", name, err)
			}
		}
	}

	r.logger.Info("All services stopped")
	return nil
}

// Get retrieves an initialized service by name
func (r *Registry) Get(name string) (Service, error) {
	service, exists := r.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}
	return service, nil
}

// RegisterAllRoutes registers API routes for all services
func (r *Registry) RegisterAllRoutes(app *fiber.App) error {
	r.logger.Info("Registering API routes for all services...")

	for name, service := range r.services {
		r.logger.Info("Registering routes for service: %s", name)
		if err := service.RegisterAPIRoutes(app); err != nil {
			return fmt.Errorf("failed to register routes for service %s: %w", name, err)
		}
	}

	r.logger.Info("Routes registered for %d services", len(r.services))
	return nil
}

// GetAuth returns the auth service with type assertion
func (r *Registry) GetAuth() (AuthProvider, error) {
	service, err := r.Get("auth")
	if err != nil {
		return nil, err
	}
	authProvider, ok := service.(AuthProvider)
	if !ok {
		return nil, fmt.Errorf("service is not an AuthProvider")
	}
	return authProvider, nil
}

// GetACL returns the ACL service with type assertion
func (r *Registry) GetACL() (ACLProvider, error) {
	service, err := r.Get("acl")
	if err != nil {
		return nil, err
	}
	aclProvider, ok := service.(ACLProvider)
	if !ok {
		return nil, fmt.Errorf("service is not an ACLProvider")
	}
	return aclProvider, nil
}

// GetAnalytics returns the analytics service with type assertion
func (r *Registry) GetAnalytics() (AnalyticsProvider, error) {
	service, err := r.Get("analytics")
	if err != nil {
		return nil, err
	}
	analyticsProvider, ok := service.(AnalyticsProvider)
	if !ok {
		return nil, fmt.Errorf("service is not an AnalyticsProvider")
	}
	return analyticsProvider, nil
}

// GetIntegration returns the integration service with type assertion
func (r *Registry) GetIntegration() (IntegrationProvider, error) {
	service, err := r.Get("integration")
	if err != nil {
		return nil, err
	}
	integrationProvider, ok := service.(IntegrationProvider)
	if !ok {
		return nil, fmt.Errorf("service is not an IntegrationProvider")
	}
	return integrationProvider, nil
}

// GetProxy returns the proxy service with type assertion
func (r *Registry) GetProxy() (ProxyProvider, error) {
	service, err := r.Get("proxy")
	if err != nil {
		return nil, err
	}
	proxyProvider, ok := service.(ProxyProvider)
	if !ok {
		return nil, fmt.Errorf("service is not a ProxyProvider")
	}
	return proxyProvider, nil
}

// GetWireGuard returns the wireguard service
func (r *Registry) GetWireGuard() (Service, error) {
	service, err := r.Get("wireguard")
	if err != nil {
		return nil, err
	}
	return service, nil
}
