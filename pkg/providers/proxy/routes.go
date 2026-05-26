package proxy

import (
	"math"

	"github.com/arqut/arqut-edge-ce/pkg/api"
	"github.com/arqut/arqut-edge-ce/pkg/models"
	"github.com/arqut/arqut-edge-ce/pkg/storage/repositories"
	"github.com/gofiber/fiber/v2"
)

// ProxyServiceRequest represents the request body for creating a service
type ProxyServiceRequest struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	LocalHost string `json:"local_host"`
	LocalPort int    `json:"local_port"`
	Path      string `json:"path,omitempty"`
}

// ProxyServiceUpdateRequest represents the request body for updating a service
type ProxyServiceUpdateRequest struct {
	Name      *string `json:"name"`
	Protocol  *string `json:"protocol"`
	LocalHost *string `json:"local_host"`
	LocalPort *int    `json:"local_port"`
	Path      *string `json:"path"`
	Enabled   *bool   `json:"enabled"`
}

// ProxyServiceResponse represents the response for a proxy service
type ProxyServiceResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	TunnelPort int     `json:"tunnel_port"`
	LocalHost  string  `json:"local_host"`
	LocalPort  int     `json:"local_port"`
	Path       *string `json:"path,omitempty"`
	Protocol   string  `json:"protocol"`
	Enabled    bool    `json:"enabled"`
	CreatedAt  string  `json:"created_at"`
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

// handleGetServices handles GET /api/services — returns a page of proxy
// services matching the query-string filter. Query params:
//
//	page      1-indexed page number (default 1)
//	page_size items per page (default 25, clamped to [1, 200])
//	q         case-insensitive substring on `name`
//	protocol  exact match: "http" or "websocket"
//	enabled   tri-state: "true" / "false" / omitted = any
//
// Response envelope: `{ success, data: [...], meta: { pagination: {...} } }`.
// Repo ordering: alphabetical by name (so a stable cursor across pages).
func (p *ProxyProvider) handleGetServices(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := c.QueryInt("page_size", 25)
	if pageSize < 1 || pageSize > 200 {
		pageSize = 25
	}

	f := repositories.ServiceFilter{
		Name:     c.Query("q"),
		Protocol: c.Query("protocol"),
	}
	switch c.Query("enabled") {
	case "true":
		tru := true
		f.Enabled = &tru
	case "false":
		fal := false
		f.Enabled = &fal
	}

	services, total, err := p.repo.ListServicesPaginated(page, pageSize, f)
	if err != nil {
		p.logger.Printf("Error listing services: %v", err)
		return api.ErrorInternalServerErrorResp(c, "Failed to get services")
	}

	serviceList := make([]ProxyServiceResponse, 0, len(services))
	for _, service := range services {
		serviceList = append(serviceList, ProxyServiceResponse{
			ID:         service.ID,
			Name:       service.Name,
			TunnelPort: service.TunnelPort,
			LocalHost:  service.LocalHost,
			LocalPort:  service.LocalPort,
			Path:       service.Path,
			Protocol:   service.Protocol,
			Enabled:    service.Enabled,
			CreatedAt:  service.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	meta := api.ApiResponseMeta{
		Pagination: &api.Pagination{
			Page:       page,
			PerPage:    pageSize,
			Total:      int(total),
			TotalPages: totalPages,
		},
	}
	return api.SuccessResp(c, serviceList, meta)
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

	// Convert path string to pointer (nil if empty)
	var path *string
	if req.Path != "" {
		path = &req.Path
	}

	service, err := p.AddService(req.Name, req.LocalHost, req.LocalPort, req.Protocol, path)
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
		Path:      req.Path,
		Protocol:  req.Protocol,
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
		return api.ErrorInternalServerErrorResp(c, err.Error())
	}

	return api.SuccessResp(c, nil)
}
