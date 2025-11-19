package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/arqut/arqut-edge-ce/pkg/api"
	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/arqut/arqut-edge-ce/pkg/storage"
	"github.com/gofiber/fiber/v2"
)

func setupTestProxy(t *testing.T) (*ProxyProvider, *fiber.App) {
	// Create in-memory storage
	store, err := storage.NewSQLiteStorage(":memory:", nil)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	testLogger := logger.NewDefault("TEST")

	// Create proxy provider
	proxy := NewProxyProvider()

	// Initialize with dependencies
	registry := providers.NewRegistry(store, testLogger, nil, nil)
	if err := proxy.Initialize(context.Background(), registry); err != nil {
		t.Fatalf("Failed to initialize proxy: %v", err)
	}

	// Create fiber app and register routes
	app := fiber.New()
	proxy.RegisterRoutes(app)

	return proxy, app
}

func TestGetServices_Empty(t *testing.T) {
	_, app := setupTestProxy(t)

	req := httptest.NewRequest("GET", "/api/services", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var response api.ApiResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success response")
	}
	if response.Error != nil {
		t.Errorf("Expected error to be nil, got %v", response.Error)
	}
}

func TestCreateService_Success(t *testing.T) {
	_, app := setupTestProxy(t)

	reqBody := ProxyServiceRequest{
		Name:      "Test Service",
		Protocol:  "http",
		LocalHost: "localhost",
		LocalPort: 8080,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/services", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	responseBody, _ := io.ReadAll(resp.Body)
	var response api.ApiResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success response")
	}
	if response.Error != nil {
		t.Errorf("Expected no error, got %v", response.Error)
	}
	if response.Data == nil {
		t.Error("Expected data to be present")
	}
}

func TestCreateService_MissingFields(t *testing.T) {
	_, app := setupTestProxy(t)

	reqBody := ProxyServiceRequest{
		Name:      "",
		LocalHost: "localhost",
		LocalPort: 8080,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/services", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	responseBody, _ := io.ReadAll(resp.Body)
	var response api.ApiResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Success {
		t.Error("Expected error response")
	}
	if response.Error == nil || response.Error.Message != "Missing required fields (name, local_host)" {
		t.Error("Expected validation error message")
	}
}

func TestUpdateService(t *testing.T) {
	proxy, app := setupTestProxy(t)

	// First create a service
	service, err := proxy.AddService("Test Service", "localhost", 8080, "http")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Update the service
	newName := "Updated Service"
	newPort := 9090
	reqBody := ProxyServiceUpdateRequest{
		Name:      &newName,
		LocalPort: &newPort,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/services/"+service.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	responseBody, _ := io.ReadAll(resp.Body)
	var response api.ApiResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success response")
	}

	// Verify the update
	updated, err := proxy.GetService(service.ID)
	if err != nil {
		t.Fatalf("Failed to get updated service: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("Expected name %s, got %s", newName, updated.Name)
	}
	if updated.LocalPort != newPort {
		t.Errorf("Expected port %d, got %d", newPort, updated.LocalPort)
	}
}

func TestAPIEnableDisableService(t *testing.T) {
	proxy, app := setupTestProxy(t)

	// Create a service
	service, err := proxy.AddService("Test Service", "localhost", 8080, "http")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Disable the service
	req := httptest.NewRequest("PATCH", "/api/services/"+service.ID+"/disable", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify it's disabled
	updated, _ := proxy.GetService(service.ID)
	if updated.Enabled {
		t.Error("Expected service to be disabled")
	}

	// Enable the service
	req = httptest.NewRequest("PATCH", "/api/services/"+service.ID+"/enable", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify it's enabled
	updated, _ = proxy.GetService(service.ID)
	if !updated.Enabled {
		t.Error("Expected service to be enabled")
	}
}

func TestAPIDeleteService(t *testing.T) {
	proxy, app := setupTestProxy(t)

	// Create a service
	service, err := proxy.AddService("Test Service", "localhost", 8080, "http")
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Delete the service
	req := httptest.NewRequest("DELETE", "/api/services/"+service.ID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	responseBody, _ := io.ReadAll(resp.Body)
	var response api.ApiResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success response")
	}

	// Verify it's deleted
	_, err = proxy.GetService(service.ID)
	if err == nil {
		t.Error("Expected service to be deleted")
	}
}

func TestGetServices_WithData(t *testing.T) {
	proxy, app := setupTestProxy(t)

	// Create multiple services
	proxy.AddService("Service 1", "localhost", 8080, "http")
	proxy.AddService("Service 2", "localhost", 8081, "http")

	req := httptest.NewRequest("GET", "/api/services", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var response api.ApiResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success response")
	}
	if response.Data == nil {
		t.Fatal("Expected data to be present")
	}

	// Type assert to slice
	services, ok := response.Data.([]interface{})
	if !ok {
		t.Fatal("Expected data to be a slice")
	}

	if len(services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(services))
	}
}
