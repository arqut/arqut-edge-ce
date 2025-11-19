package main

import (
	"context"
	"testing"

	"github.com/arqut/arqut-edge-ce/pkg/config"
	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/arqut/arqut-edge-ce/pkg/providers/acl"
	"github.com/arqut/arqut-edge-ce/pkg/providers/analytics"
	"github.com/arqut/arqut-edge-ce/pkg/providers/auth"
	"github.com/arqut/arqut-edge-ce/pkg/providers/integration"
	"github.com/arqut/arqut-edge-ce/pkg/providers/proxy"
	"github.com/arqut/arqut-edge-ce/pkg/storage"
)

func TestServiceRegistryIntegration(t *testing.T) {
	// Setup test database
	store, err := storage.NewSQLiteStorage(":memory:", nil)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer store.Close()

	testLogger := logger.NewDefault("TEST")
	cfg := &config.Config{}

	// Create service registry (no signaling client needed for these tests)
	registry := providers.NewRegistry(store, testLogger, cfg, nil)

	// Register all services
	if err := registry.Register(auth.NewService()); err != nil {
		t.Fatalf("Failed to register auth service: %v", err)
	}
	if err := registry.Register(acl.NewService()); err != nil {
		t.Fatalf("Failed to register ACL service: %v", err)
	}
	if err := registry.Register(analytics.NewService()); err != nil {
		t.Fatalf("Failed to register analytics service: %v", err)
	}
	if err := registry.Register(integration.NewService()); err != nil {
		t.Fatalf("Failed to register integration service: %v", err)
	}
	if err := registry.Register(proxy.NewProxyProvider()); err != nil {
		t.Fatalf("Failed to register proxy service: %v", err)
	}

	// Initialize all services
	ctx := context.Background()
	if err := registry.InitializeAll(ctx); err != nil {
		t.Fatalf("Failed to initialize services: %v", err)
	}

	// Test typed getters
	authProvider, err := registry.GetAuth()
	if err != nil {
		t.Errorf("Failed to get auth provider: %v", err)
	}
	if authProvider == nil {
		t.Error("Expected auth provider, got nil")
	}

	aclProvider, err := registry.GetACL()
	if err != nil {
		t.Errorf("Failed to get ACL provider: %v", err)
	}
	if aclProvider == nil {
		t.Error("Expected ACL provider, got nil")
	}

	analyticsProvider, err := registry.GetAnalytics()
	if err != nil {
		t.Errorf("Failed to get analytics provider: %v", err)
	}
	if analyticsProvider == nil {
		t.Error("Expected analytics provider, got nil")
	}

	integrationProvider, err := registry.GetIntegration()
	if err != nil {
		t.Errorf("Failed to get integration provider: %v", err)
	}
	if integrationProvider == nil {
		t.Error("Expected integration provider, got nil")
	}

	proxyProvider, err := registry.GetProxy()
	if err != nil {
		t.Errorf("Failed to get proxy provider: %v", err)
	}
	if proxyProvider == nil {
		t.Error("Expected proxy provider, got nil")
	}

	// Test authentication flow
	token, err := authProvider.Authenticate(ctx, "admin", "admin123")
	if err != nil {
		t.Errorf("Authentication failed: %v", err)
	}
	if token == "" {
		t.Error("Expected token, got empty string")
	}

	username, err := authProvider.ValidateToken(ctx, token)
	if err != nil {
		t.Errorf("Token validation failed: %v", err)
	}
	if username != "admin" {
		t.Errorf("Expected username 'admin', got %s", username)
	}

	// Test ACL
	hasAccess, err := aclProvider.CheckPermission(ctx, "admin", "any-resource", "any-action")
	if err != nil {
		t.Errorf("Permission check failed: %v", err)
	}
	if !hasAccess {
		t.Error("Expected admin to have access to any resource")
	}

	// Test analytics
	err = analyticsProvider.Track(ctx, providers.Event{
		Type:   "test",
		UserID: "admin",
		Data:   map[string]interface{}{"test": true},
	})
	if err != nil {
		t.Errorf("Failed to track event: %v", err)
	}

	// Test proxy
	service, err := proxyProvider.AddService("test-service", "localhost", 3000, "http")
	if err != nil {
		t.Errorf("Failed to add proxy service: %v", err)
	}
	if service == nil {
		t.Error("Expected service, got nil")
	}

	// Test shutdown
	if err := registry.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}
