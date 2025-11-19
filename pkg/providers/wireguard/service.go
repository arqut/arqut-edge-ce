package wireguard

import (
	"context"
	"fmt"

	"github.com/arqut/arqut-edge-ce/pkg/api"
	"github.com/arqut/arqut-edge-ce/pkg/config"
	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/gofiber/fiber/v2"
)

// Service implements the providers.Service interface for WireGuard
type Service struct {
	manager  *Manager
	registry *providers.Registry
}

// NewService creates a new WireGuard service instance
func NewService() *Service {
	return &Service{}
}

// Name returns the service name
func (s *Service) Name() string {
	return "wireguard"
}

// Initialize sets up the WireGuard manager with the signaling client from registry
func (s *Service) Initialize(ctx context.Context, registry *providers.Registry) error {
	s.registry = registry

	sigClient := registry.SignalingClient()
	if sigClient == nil {
		registry.Logger().Printf("[WireGuard] Signaling client not configured, WireGuard will not be available")
		return nil
	}

	cfg, ok := registry.Config().(*config.Config)
	if !ok {
		return fmt.Errorf("invalid config type")
	}

	manager, err := NewManager(
		cfg.EdgeID,
		sigClient.SendMessage,
		registry.Logger(),
	)
	if err != nil {
		return fmt.Errorf("failed to create WireGuard manager: %w", err)
	}
	s.manager = manager

	s.manager.RegisterHandlers(func(msgType string, handler MessageHandler) {
		sigClient.SetMessageHandler(msgType, handler)
	})
	s.manager.RegisterOnConnectHandler(func(handler OnConnectHandler) {
		sigClient.AddOnConnectHandler(handler)
	})

	registry.Logger().Printf("[WireGuard] Initialized successfully")
	return nil
}

func (s *Service) IsRunnable() bool {
	return true
}

// Start begins the WireGuard service operation
func (s *Service) Start(ctx context.Context) error {
	// Get proxy provider from registry to set network service for interface management
	if svc, err := s.registry.Get("proxy"); err == nil {
		if networkService, ok := svc.(NetworkService); ok {
			s.manager.SetNetworkService(networkService)
			s.registry.Logger().Println("[WireGuard] Network service configured")
		}
	}

	s.registry.Logger().Printf("[WireGuard] Started successfully")
	return nil
}

// Stop shuts down the WireGuard service
func (s *Service) Stop(ctx context.Context) error {
	if s.manager != nil {
		s.manager.Stop()
	}
	// Note: We don't close the signaling client here as it's managed by the Registry
	// and may be used by other services
	s.registry.Logger().Printf("[WireGuard] Stopped")
	return nil
}

// RegisterAPIRoutes adds WireGuard API endpoints
func (s *Service) RegisterAPIRoutes(app interface{}) error {
	fiberApp, ok := app.(*fiber.App)
	if !ok {
		return fmt.Errorf("expected *fiber.App, got %T", app)
	}

	wgAPI := fiberApp.Group("/api/wireguard")

	// GET /api/wireguard/peers - List connected peers
	wgAPI.Get("/peers", func(c *fiber.Ctx) error {
		if s.manager == nil {
			return api.ErrorCodeResp(c, fiber.StatusServiceUnavailable, "WireGuard service not available")
		}

		peers := s.manager.GetConnectedPeers()
		return api.SuccessResp(c, fiber.Map{
			"peers": peers,
		})
	})

	// GET /api/wireguard/peers/:id - Get peer info
	wgAPI.Get("/peers/:id", func(c *fiber.Ctx) error {
		if s.manager == nil {
			return api.ErrorCodeResp(c, fiber.StatusServiceUnavailable, "WireGuard service not available")
		}

		peerID := c.Params("id")
		peerInfo, err := s.manager.GetPeerInfo(peerID)
		if err != nil {
			return api.ErrorNotFoundResp(c, err.Error())
		}

		return api.SuccessResp(c, peerInfo)
	})

	// DELETE /api/wireguard/peers/:id - Disconnect peer
	wgAPI.Delete("/peers/:id", func(c *fiber.Ctx) error {
		if s.manager == nil {
			return api.ErrorCodeResp(c, fiber.StatusServiceUnavailable, "WireGuard service not available")
		}

		peerID := c.Params("id")
		if err := s.manager.DisconnectPeer(peerID); err != nil {
			return api.ErrorNotFoundResp(c, err.Error())
		}

		return api.SuccessResp(c, fiber.Map{
			"message": "Peer disconnected",
		})
	})

	// GET /api/wireguard/interfaces - List interface IPs
	wgAPI.Get("/interfaces", func(c *fiber.Ctx) error {
		if s.manager == nil {
			return api.ErrorCodeResp(c, fiber.StatusServiceUnavailable, "WireGuard service not available")
		}

		interfaces := s.manager.GetInterfaceIPs()
		return api.SuccessResp(c, fiber.Map{
			"interfaces": interfaces,
		})
	})

	s.registry.Logger().Printf("[WireGuard] API routes registered")
	return nil
}

// GetManager returns the underlying WireGuard manager (for integration with other services)
func (s *Service) GetManager() *Manager {
	return s.manager
}
