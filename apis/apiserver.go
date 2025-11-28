package apis

import (
	"context"
	"io/fs"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/arqut/arqut-edge-ce/pkg/api"
	"github.com/arqut/arqut-edge-ce/pkg/core"
	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/arqut/arqut-edge-ce/ui"
)

// ApiServer is the HTTP server using Fiber
type ApiServer struct {
	app       *fiber.App
	coreApp   core.App
	providers *providers.Registry
}

// New creates a new HTTP server with the given service registry
func New(p *providers.Registry, withUI bool) *ApiServer {
	app := fiber.New(fiber.Config{
		ErrorHandler: customErrorHandler,
	})

	s := &ApiServer{
		app:       app,
		coreApp:   core.NewMainApp(p),
		providers: p,
	}

	s.setupMiddleware()
	s.setupRoutes(withUI)

	return s
}

func (s *ApiServer) setupMiddleware() {
	s.app.Use(recover.New())
	s.app.Use(logger.New())
}

func (s *ApiServer) setupRoutes(withUI bool) {
	// API routes
	apiGroup := s.app.Group("/api")

	apiGroup.Post("/login", s.handleLogin)
	apiGroup.Get("/check-access", s.authMiddleware, s.handleCheckAccess)
	apiGroup.Post("/send-data", s.authMiddleware, s.handleSendData)
	apiGroup.Post("/metrics", s.authMiddleware, s.handleGetMetrics)

	s.app.Get("/health", s.handleHealth)

	// Serve UI if enabled
	if withUI {
		s.setupUIRoutes()
	}
}

// setupUIRoutes configures routes to serve the embedded UI
func (s *ApiServer) setupUIRoutes() {
	// Get the embedded filesystem
	distFS, err := fs.Sub(ui.DistFS, "dist/spa")
	if err != nil {
		s.providers.Logger().Printf("Warning: Failed to setup UI filesystem: %v", err)
		return
	}

	// Serve all static files (JS, CSS, images, fonts, etc.)
	s.app.Use(func(c *fiber.Ctx) error {
		// Skip API routes
		if strings.HasPrefix(c.Path(), "/api/") || c.Path() == "/health" {
			return c.Next()
		}

		path := strings.TrimPrefix(c.Path(), "/")

		// Try to serve the file from embedded filesystem
		file, err := distFS.Open(path)
		if err == nil {
			defer file.Close()

			// Get file info to check if it's a directory
			stat, err := file.Stat()
			if err == nil && !stat.IsDir() {
				// Set correct Content-Type based on file extension
				setContentType(c, path)
				return c.SendStream(file)
			}
		}

		// If file not found or is a directory, serve index.html for SPA routing
		indexFile, err := distFS.Open("index.html")
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, "UI not found")
		}
		defer indexFile.Close()

		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendStream(indexFile)
	})
}

// setContentType sets the appropriate Content-Type header based on file extension
func setContentType(c *fiber.Ctx, path string) {
	ext := ""
	if idx := strings.LastIndex(path, "."); idx != -1 {
		ext = path[idx:]
	}

	switch ext {
	case ".js", ".mjs":
		c.Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		c.Set("Content-Type", "text/css; charset=utf-8")
	case ".json":
		c.Set("Content-Type", "application/json; charset=utf-8")
	case ".png":
		c.Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		c.Set("Content-Type", "image/jpeg")
	case ".svg":
		c.Set("Content-Type", "image/svg+xml")
	case ".ico":
		c.Set("Content-Type", "image/x-icon")
	case ".woff":
		c.Set("Content-Type", "font/woff")
	case ".woff2":
		c.Set("Content-Type", "font/woff2")
	case ".ttf":
		c.Set("Content-Type", "font/ttf")
	case ".eot":
		c.Set("Content-Type", "application/vnd.ms-fontobject")
	case ".html":
		c.Set("Content-Type", "text/html; charset=utf-8")
	default:
		c.Set("Content-Type", "application/octet-stream")
	}
}

// App returns the underlying Fiber app for route registration
func (s *ApiServer) App() *fiber.App {
	return s.app
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
		Destination string      `json:"destination"`
		Data        interface{} `json:"data"`
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
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(api.ApiResponse{
		Success: false,
		Error: &api.ApiError{
			Code:    code,
			Message: err.Error(),
		},
	})
}
