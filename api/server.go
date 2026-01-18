package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/arqut/arqut-edge-ce/pkg/api"
	"github.com/arqut/arqut-edge-ce/pkg/core"
	"github.com/arqut/arqut-edge-ce/pkg/haaddon"
	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/arqut/arqut-edge-ce/pkg/providers/proxy"
	"github.com/arqut/arqut-edge-ce/ui"
)

// ApiServer is the HTTP server using Fiber
type ApiServer struct {
	app       *fiber.App
	api       fiber.Router
	coreApp   core.App
	providers *providers.Registry
}

// New creates a new HTTP server with the given service registry
func New(p *providers.Registry) *ApiServer {
	app := fiber.New(fiber.Config{
		ErrorHandler:          customErrorHandler,
		DisableStartupMessage: true,
	})

	s := &ApiServer{
		app:       app,
		coreApp:   core.NewMainApp(p),
		providers: p,
	}

	s.setupMiddleware()
	s.setupRoutes()
	s.setupUI()

	return s
}

func (s *ApiServer) setupMiddleware() {
	s.app.Use(recover.New())
	s.app.Use(logger.New())
}

func (s *ApiServer) setupRoutes() {
	// API routes
	s.api = s.app.Group("/api")

	s.api.Post("/login", s.handleLogin)
	s.api.Get("/check-access", s.authMiddleware, s.handleCheckAccess)
	s.api.Post("/send-data", s.authMiddleware, s.handleSendData)
	s.api.Post("/metrics", s.authMiddleware, s.handleGetMetrics)

	// Integration routes (for HA addon)
	s.api.Get("/integrations/network", s.handleGetNetworkSettings)
	s.api.Post("/config/haaddon/expose", s.handleExposeHAAddon)

	s.app.Get("/health", s.handleHealth)
}

func (s *ApiServer) setupUI() {
	// Serve the embedded UI using Fiber's static middleware
	s.app.Use("/", filesystem.New(filesystem.Config{
		Root:   http.FS(ui.FS),
		Browse: false,
	}))
}

// App returns the underlying Fiber app for route registration
func (s *ApiServer) App() *fiber.App {
	return s.app
}

func (s *ApiServer) ApiRouter() fiber.Router {
	return s.api
}

// Start starts the HTTP server
func (s *ApiServer) Start(addr string) error {
	s.providers.Logger().Printf("Starting server on %s", addr)
	return s.app.Listen(addr)
}

// Shutdown gracefully shuts down the server
func (s *ApiServer) Shutdown(ctx context.Context) error {
	s.providers.Logger().Println("Server shutdown requested")
	return s.app.ShutdownWithContext(ctx)
}

// authMiddleware extracts and validates the bearer token
func (s *ApiServer) authMiddleware(c *fiber.Ctx) error {
	token := extractToken(c)
	if token == "" {
		return api.ErrorUnauthorizedResp(c, "Missing authorization token")
	}

	// Store token in context for handlers
	c.Locals("token", token)
	return c.Next()
}

// handleLogin handles user login
func (s *ApiServer) handleLogin(c *fiber.Ctx) error {
	var req core.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return api.ErrorBadRequestResp(c, "Invalid request body")
	}

	resp, err := s.coreApp.Login(c.Context(), req)
	if err != nil {
		return api.ErrorUnauthorizedResp(c, err.Error())
	}

	return api.SuccessResp(c, resp)
}

// handleCheckAccess handles access verification
func (s *ApiServer) handleCheckAccess(c *fiber.Ctx) error {
	token := c.Locals("token").(string)
	resource := c.Query("resource")
	action := c.Query("action")

	if resource == "" || action == "" {
		return api.ErrorBadRequestResp(c, "Missing resource or action parameter")
	}

	hasAccess, err := s.coreApp.CheckAccess(c.Context(), token, resource, action)
	if err != nil {
		return api.ErrorUnauthorizedResp(c, err.Error())
	}

	return api.SuccessResp(c, fiber.Map{
		"has_access": hasAccess,
	})
}

// handleSendData handles sending data to integrations
func (s *ApiServer) handleSendData(c *fiber.Ctx) error {
	token := c.Locals("token").(string)

	var reqData struct {
		Destination string `json:"destination"`
		Data        any    `json:"data"`
	}

	if err := c.BodyParser(&reqData); err != nil {
		return api.ErrorBadRequestResp(c, "Invalid request body")
	}

	if reqData.Destination == "" {
		return api.ErrorBadRequestResp(c, "Missing destination")
	}

	err := s.coreApp.SendData(c.Context(), token, reqData.Destination, reqData.Data)
	if err != nil {
		return api.ErrorCodeResp(c, fiber.StatusForbidden, err.Error())
	}

	return api.SuccessResp(c, fiber.Map{
		"status": "success",
	})
}

// handleGetMetrics handles metrics retrieval
func (s *ApiServer) handleGetMetrics(c *fiber.Ctx) error {
	token := c.Locals("token").(string)

	var query providers.MetricsQuery
	if err := c.BodyParser(&query); err != nil {
		return api.ErrorBadRequestResp(c, "Invalid request body")
	}

	result, err := s.coreApp.GetMetrics(c.Context(), token, query)
	if err != nil {
		return api.ErrorCodeResp(c, fiber.StatusForbidden, err.Error())
	}

	return api.SuccessResp(c, result)
}

// handleHealth handles health checks
func (s *ApiServer) handleHealth(c *fiber.Ctx) error {
	return api.SuccessResp(c, fiber.Map{
		"status": "healthy",
	})
}

// handleGetNetworkSettings returns network subnets for HA addon configuration
func (s *ApiServer) handleGetNetworkSettings(c *fiber.Ctx) error {
	subnets, err := haaddon.GetNetworkSubnets()
	if err != nil {
		return api.ErrorInternalServerErrorResp(c, "Failed to get network settings")
	}

	return api.SuccessResp(c, fiber.Map{
		"subnets": subnets,
	})
}

// handleExposeHAAddon exposes Home Assistant as a proxy service
func (s *ApiServer) handleExposeHAAddon(c *fiber.Ctx) error {
	// Get the proxy provider
	svc, err := s.providers.Get("proxy")
	if err != nil {
		return api.ErrorInternalServerErrorResp(c, "Proxy service not available")
	}

	proxyImpl, ok := svc.(*proxy.ProxyProvider)
	if !ok {
		return api.ErrorInternalServerErrorResp(c, "Invalid proxy service")
	}

	// Create the HA Addon service
	service, err := proxyImpl.CreateHAAddonService()
	if err != nil {
		// Check if service already exists
		if strings.Contains(err.Error(), "already set up") {
			return api.ErrorCodeResp(c, fiber.StatusConflict, "Home Assistant service is already configured")
		}
		return api.ErrorInternalServerErrorResp(c, err.Error())
	}

	// Update HA config with trusted proxy subnets
	haaddon.UpdateHAConfig()

	return api.SuccessResp(c, service)
}

// extractToken extracts the bearer token from the Authorization header
func extractToken(c *fiber.Ctx) string {
	auth := c.Get("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// customErrorHandler handles errors
func customErrorHandler(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		status = e.Code
	}

	return c.Status(status).JSON(api.ApiResponse{
		Success: false,
		Error: &api.ApiError{
			Status:  status,
			Message: err.Error(),
		},
	})
}
