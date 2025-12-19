package proxy

import (
	"sort"

	"github.com/arqut/arqut-edge-ce/pkg/api"
	"github.com/arqut/arqut-edge-ce/pkg/models"
	"github.com/gofiber/fiber/v2"
)

// ProxyServiceRequest represents the request body for creating a service
type ProxyServiceRequest struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	LocalHost string `json:"local_host"`
	LocalPort int    `json:"local_port"`
}

// ProxyServiceUpdateRequest represents the request body for updating a service
type ProxyServiceUpdateRequest struct {
	Name      *string `json:"name"`
	LocalHost *string `json:"local_host"`
	LocalPort *int    `json:"local_port"`
	Enabled   *bool   `json:"enabled"`
}

// ProxyServiceResponse represents the response for a proxy service
type ProxyServiceResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TunnelPort int    `json:"tunnel_port"`
	LocalHost  string `json:"local_host"`
	LocalPort  int    `json:"local_port"`
	Protocol   string `json:"protocol"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at"`
}

// RegisterRoutes registers all proxy-related API routes
func (p *ProxyProvider) RegisterRoutes(router fiber.Router, middlewares ...fiber.Handler) {
	proxyAPI := router.Group("/services", middlewares...)

	proxyAPI.Get("/", p.handleGetServices)
	proxyAPI.Post("/", p.handleCreateService)
	proxyAPI.Put("/:id", p.handleUpdateService)
	proxyAPI.Patch("/:id/enable", p.handleEnableService)
	proxyAPI.Patch("/:id/disable", p.handleDisableService)
	proxyAPI.Delete("/:id", p.handleDeleteService)
}

// handleGetServices handles GET /api/services - returns all proxy services
func (p *ProxyProvider) handleGetServices(c *fiber.Ctx) error {
	services, err := p.repo.GetServices()
	if err != nil {
		p.logger.Printf("Error getting services: %v", err)
		return api.ErrorInternalServerErrorResp(c, "Failed to get services")
	}

	var serviceList []ProxyServiceResponse
	for _, service := range services {
		serviceList = append(serviceList, ProxyServiceResponse{
			ID:         service.ID,
			Name:       service.Name,
			TunnelPort: service.TunnelPort,
			LocalHost:  service.LocalHost,
			LocalPort:  service.LocalPort,
			Protocol:   service.Protocol,
			Enabled:    service.Enabled,
			CreatedAt:  service.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	// Sort by creation date
	sort.Slice(serviceList, func(i, j int) bool {
		return serviceList[i].CreatedAt < serviceList[j].CreatedAt
	})

	return api.SuccessResp(c, serviceList)
}

// handleCreateService handles POST /api/services - creates a new proxy service
func (p *ProxyProvider) handleCreateService(c *fiber.Ctx) error {
	var req ProxyServiceRequest
	if err := c.BodyParser(&req); err != nil {
		return api.ErrorBadRequestResp(c, "Invalid request body")
	}

	if req.Name == "" || req.LocalHost == "" {
		return api.ErrorBadRequestResp(c, "Missing required fields (name, local_host)")
	}

	service, err := p.AddService(req.Name, req.LocalHost, req.LocalPort, req.Protocol)
	if err != nil {
		p.logger.Printf("Error creating service: %v", err)
		return api.ErrorInternalServerErrorResp(c, "Failed to create service")
	}

	return api.SuccessResp(c, service)
}

// handleUpdateService handles PUT /api/services/:id - updates a proxy service
func (p *ProxyProvider) handleUpdateService(c *fiber.Ctx) error {
	serviceID := c.Params("id")
	if serviceID == "" {
		return api.ErrorBadRequestResp(c, "Service ID is required")
	}

	var req ProxyServiceUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return api.ErrorBadRequestResp(c, "Invalid request body")
	}

	// Validate non-empty values if provided
	if req.Name != nil && *req.Name == "" {
		return api.ErrorBadRequestResp(c, "Name cannot be empty")
	}
	if req.LocalHost != nil && *req.LocalHost == "" {
		return api.ErrorBadRequestResp(c, "Local host cannot be empty")
	}

	config := models.ProxyServiceConfig{
		Name:      req.Name,
		LocalHost: req.LocalHost,
		LocalPort: req.LocalPort,
		Enabled:   req.Enabled,
	}

	if err := p.ModifyService(serviceID, config); err != nil {
		p.logger.Printf("Error updating service: %v", err)
		return api.ErrorInternalServerErrorResp(c, "Failed to update service")
	}

	return api.SuccessResp(c, nil)
}

// handleEnableService handles PATCH /api/services/:id/enable - enables a proxy service
func (p *ProxyProvider) handleEnableService(c *fiber.Ctx) error {
	serviceID := c.Params("id")
	if serviceID == "" {
		return api.ErrorBadRequestResp(c, "Service ID is required")
	}

	if err := p.EnableService(serviceID); err != nil {
		p.logger.Printf("Error enabling service: %v", err)
		return api.ErrorInternalServerErrorResp(c, "Failed to enable service")
	}

	return api.SuccessResp(c, nil)
}

// handleDisableService handles PATCH /api/services/:id/disable - disables a proxy service
func (p *ProxyProvider) handleDisableService(c *fiber.Ctx) error {
	serviceID := c.Params("id")
	if serviceID == "" {
		return api.ErrorBadRequestResp(c, "Service ID is required")
	}

	if err := p.DisableService(serviceID); err != nil {
		p.logger.Printf("Error disabling service: %v", err)
		return api.ErrorInternalServerErrorResp(c, "Failed to disable service")
	}

	return api.SuccessResp(c, nil)
}

// handleDeleteService handles DELETE /api/services/:id - deletes a proxy service
func (p *ProxyProvider) handleDeleteService(c *fiber.Ctx) error {
	serviceID := c.Params("id")
	if serviceID == "" {
		return api.ErrorBadRequestResp(c, "Service ID is required")
	}

	if err := p.DeleteService(serviceID); err != nil {
		p.logger.Printf("Error deleting service: %v", err)
		return api.ErrorInternalServerErrorResp(c, "Failed to delete service")
	}

	return api.SuccessResp(c, nil)
}
