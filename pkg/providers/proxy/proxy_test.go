package proxy

import (
	"context"
	"testing"

	"github.com/arqut/arqut-edge-ce/pkg/config"
	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/arqut/arqut-edge-ce/pkg/storage"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) storage.Storage {
	store, err := storage.NewSQLiteStorage(":memory:", nil)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return store
}

// setupTestProvider creates a fully initialized proxy provider for testing
func setupTestProvider(t *testing.T) (*ProxyProvider, storage.Storage) {
	store := setupTestDB(t)

	proxy := NewProxyProvider()

	// Initialize with dependencies
	registry := providers.NewRegistry(
		store,
		logger.NewDefault("TEST"),
		&config.Config{},
		nil, // No signaling client needed for proxy tests
	)

	ctx := context.Background()
	if err := proxy.Initialize(ctx, registry); err != nil {
		t.Fatalf("Failed to initialize proxy provider: %v", err)
	}

	return proxy, store
}

func TestNewProxyProvider(t *testing.T) {
	proxy, store := setupTestProvider(t)
	defer store.Close()

	if proxy == nil {
		t.Fatal("Expected proxy provider, got nil")
	}

	// Check default port range
	if proxy.portRange.start != 8000 || proxy.portRange.end != 9000 {
		t.Errorf("Expected port range 8000-9000, got %d-%d", proxy.portRange.start, proxy.portRange.end)
	}

	if proxy.started {
		t.Error("Expected proxy to not be started initially")
	}
}

func TestAddService(t *testing.T) {
	proxy, store := setupTestProvider(t)
	defer store.Close()

	// Add a service
	service, err := proxy.AddService("test-service", "localhost", 3000, "http")
	if err != nil {
		t.Fatalf("Failed to add service: %v", err)
	}

	if service == nil {
		t.Fatal("Expected service, got nil")
	}

	if service.Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got %s", service.Name)
	}

	if service.LocalHost != "localhost" {
		t.Errorf("Expected local host 'localhost', got %s", service.LocalHost)
	}

	if service.LocalPort != 3000 {
		t.Errorf("Expected local port 3000, got %d", service.LocalPort)
	}

	if service.Protocol != "http" {
		t.Errorf("Expected protocol 'http', got %s", service.Protocol)
	}

	if service.TunnelPort == 0 {
		t.Error("Expected tunnel port to be allocated")
	}

	if !service.Enabled {
		t.Error("Expected service to be enabled by default")
	}
}

func TestGetServices(t *testing.T) {
	proxy, store := setupTestProvider(t)
	defer store.Close()

	// Initially empty
	services, err := proxy.GetServices()
	if err != nil {
		t.Fatalf("Failed to get services: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected 0 services, got %d", len(services))
	}

	// Add services
	_, err = proxy.AddService("service1", "localhost", 3000, "http")
	if err != nil {
		t.Fatalf("Failed to add service1: %v", err)
	}

	_, err = proxy.AddService("service2", "localhost", 3001, "http")
	if err != nil {
		t.Fatalf("Failed to add service2: %v", err)
	}

	// Get all services
	services, err = proxy.GetServices()
	if err != nil {
		t.Fatalf("Failed to get services: %v", err)
	}

	if len(services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(services))
	}
}

func TestModifyService(t *testing.T) {
	proxy, store := setupTestProvider(t)
	defer store.Close()

	// Add a service
	service, err := proxy.AddService("test-service", "localhost", 3000, "http")
	if err != nil {
		t.Fatalf("Failed to add service: %v", err)
	}

	// Modify the service
	newName := "updated-service"
	err = proxy.ModifyService(service.ID, storage.ProxyServiceConfig{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("Failed to modify service: %v", err)
	}

	// Verify modification
	updated, err := proxy.GetService(service.ID)
	if err != nil {
		t.Fatalf("Failed to get updated service: %v", err)
	}

	if updated.Name != "updated-service" {
		t.Errorf("Expected service name 'updated-service', got %s", updated.Name)
	}
}

func TestDeleteService(t *testing.T) {
	proxy, store := setupTestProvider(t)
	defer store.Close()

	// Add a service
	service, err := proxy.AddService("test-service", "localhost", 3000, "http")
	if err != nil {
		t.Fatalf("Failed to add service: %v", err)
	}

	// Delete the service
	err = proxy.DeleteService(service.ID)
	if err != nil {
		t.Fatalf("Failed to delete service: %v", err)
	}

	// Verify deletion
	_, err = proxy.GetService(service.ID)
	if err == nil {
		t.Error("Expected error when getting deleted service")
	}
}

func TestEnableDisableService(t *testing.T) {
	proxy, store := setupTestProvider(t)
	defer store.Close()

	// Add a service (disabled by default)
	service, err := proxy.AddService("test-service", "localhost", 3000, "http")
	if err != nil {
		t.Fatalf("Failed to add service: %v", err)
	}

	if !service.Enabled {
		t.Error("Expected service to be enabled by default")
	}

	// Enable the service
	err = proxy.EnableService(service.ID)
	if err != nil {
		t.Fatalf("Failed to enable service: %v", err)
	}

	// Verify enabled
	enabled, err := proxy.GetService(service.ID)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if !enabled.Enabled {
		t.Error("Expected service to be enabled")
	}

	// Disable the service
	err = proxy.DisableService(service.ID)
	if err != nil {
		t.Fatalf("Failed to disable service: %v", err)
	}

	// Verify disabled
	disabled, err := proxy.GetService(service.ID)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if disabled.Enabled {
		t.Error("Expected service to be disabled")
	}
}

func TestSetPortRange(t *testing.T) {
	proxy, store := setupTestProvider(t)
	defer store.Close()

	// Change port range
	proxy.SetPortRange(9000, 10000)

	// Add a service and check it uses the new range
	service, err := proxy.AddService("test-service", "localhost", 3000, "http")
	if err != nil {
		t.Fatalf("Failed to add service: %v", err)
	}

	if service.TunnelPort < 9000 || service.TunnelPort > 10000 {
		t.Errorf("Expected tunnel port in range 9000-10000, got %d", service.TunnelPort)
	}
}

func TestClear(t *testing.T) {
	proxy, store := setupTestProvider(t)
	defer store.Close()

	// Add services
	_, err := proxy.AddService("service1", "localhost", 3000, "http")
	if err != nil {
		t.Fatalf("Failed to add service1: %v", err)
	}

	_, err = proxy.AddService("service2", "localhost", 3001, "http")
	if err != nil {
		t.Fatalf("Failed to add service2: %v", err)
	}

	// Clear all services
	err = proxy.Clear()
	if err != nil {
		t.Fatalf("Failed to clear services: %v", err)
	}

	// Verify all services deleted
	services, err := proxy.GetServices()
	if err != nil {
		t.Fatalf("Failed to get services: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected 0 services after clear, got %d", len(services))
	}
}

func TestGetServiceByHostPort(t *testing.T) {
	proxy, store := setupTestProvider(t)
	defer store.Close()

	// Add a service
	_, err := proxy.AddService("test-service", "localhost", 3000, "http")
	if err != nil {
		t.Fatalf("Failed to add service: %v", err)
	}

	// Get by host and port
	service, err := proxy.GetServiceByHostPort("localhost", 3000)
	if err != nil {
		t.Fatalf("Failed to get service by host/port: %v", err)
	}

	if service.Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got %s", service.Name)
	}

	// Try non-existent host/port
	_, err = proxy.GetServiceByHostPort("localhost", 9999)
	if err == nil {
		t.Error("Expected error when getting non-existent service by host/port")
	}
}
