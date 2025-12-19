package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/arqut/arqut-edge-ce/api"
	"github.com/arqut/arqut-edge-ce/pkg/config"
	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/arqut/arqut-edge-ce/pkg/providers/proxy"
	"github.com/arqut/arqut-edge-ce/pkg/providers/wireguard"
	"github.com/arqut/arqut-edge-ce/pkg/signaling"
	"github.com/arqut/arqut-edge-ce/pkg/storage"
)

var version = "0.1.0"

func main() {
	// Create structured logger
	appLogger := logger.NewDefault("ARQUT")

	var cfgFile, logLevel string
	flag.StringVar(&cfgFile, "config", "./arqut.yaml", "Path to configuration file")
	flag.StringVar(&logLevel, "loglevel", "", "Set the log level")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(version, cfgFile, logLevel)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	switch cfg.LogLevel {
	case "debug":
		appLogger.SetLevel(logger.DebugLevel)
	case "warn":
		appLogger.SetLevel(logger.WarnLevel)
	case "error":
		appLogger.SetLevel(logger.ErrorLevel)
	default:
		appLogger.SetLevel(logger.InfoLevel)
	}

	appLogger.Info("Starting Arqut Edge Community Edition...")
	appLogger.Info("API Key: %s...", maskAPIKey(cfg.APIKey))

	// Initialize storage
	store, err := storage.NewSQLiteStorage(cfg.DBPath, appLogger)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Create signaling client if CloudURL is configured
	var sigClient *signaling.Client
	if cfg.CloudURL != "" {
		client, err := signaling.NewClient(cfg.CloudURL, appLogger)
		if err != nil {
			log.Fatalf("Failed to create signaling client: %v", err)
		}
		sigClient = client
		defer sigClient.Close()
		appLogger.Info("Signaling client initialized with cloud URL: %s", cfg.CloudURL)
	} else {
		appLogger.Info("Cloud URL not configured, running without cloud connectivity")
	}

	// Create service registry and register all default services
	registry := createServiceRegistry(store, appLogger, cfg, sigClient)

	// Initialize all services
	ctx := context.Background()
	if err := registry.InitializeAll(ctx); err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

	// Wire up signaling channel with proxy provider and connect if signaling client exists
	if sigClient != nil {
		if svc, err := registry.Get("proxy"); err == nil {
			if proxyImpl, ok := svc.(*proxy.ProxyProvider); ok {
				// Set the outbound channel for proxy to send sync messages
				proxyImpl.SetSyncChannel(sigClient.OutboundChannel())
				appLogger.Info("Proxy sync channel configured")

				// Register proxy's sync ack handler with signaling client
				sigClient.SetMessageHandler(
					proxy.MessageTypeServiceSyncAck,
					proxyImpl.HandleServiceSyncAck,
				)
				appLogger.Info("Proxy sync ack handler registered")

				// Register reconnect handler for full service sync on reconnection
				sigClient.AddOnConnectHandler(proxyImpl.OnReconnect)
				appLogger.Info("Proxy reconnect handler registered")
			}
		}

		// Connect to signaling server
		if cfg.APIKey != "" && cfg.CloudURL != "" {
			sigClient.Connect(ctx, cfg.APIKey, cfg.EdgeID, cfg.ServerAddr)
			appLogger.Info("Connected to signaling server with edge ID: %s", cfg.EdgeID)
		} else {
			appLogger.Warn("-------------------------------------------------------------------------")
			appLogger.Warn("`api_key` or `cloud_url` is not configured, skipping signaling connection")
			appLogger.Warn("Please configure `api_key` and `cloud_url` to enable cloud connectivity")
			appLogger.Warn("Configuration file: %s ", cfgFile)
			appLogger.Warn("-------------------------------------------------------------------------")
		}
	}

	// Start runnable services
	if err := registry.StartRunnable(ctx); err != nil {
		log.Fatalf("Failed to start runnable services: %v", err)
	}

	// Create API server
	srv := api.New(registry)

	// Register service-specific routes
	registry.RegisterAllRoutes(srv.ApiRouter())

	// Start server in a goroutine
	go func() {
		if err := srv.Start(cfg.ServerAddr); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx := context.Background()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("Server shutdown error: %v", err)
	}

	// Shutdown all services
	if err := registry.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("Service shutdown error: %v", err)
	}

	appLogger.Info("Server exited")
}

// createServiceRegistry creates and populates the service registry with default services
func createServiceRegistry(store storage.Storage, log *logger.Logger, cfg *config.Config, sigClient *signaling.Client) *providers.Registry {
	registry := providers.NewRegistry(store, log, cfg, sigClient)

	// Register all default services
	// registry.MustRegister(auth.NewService())
	// registry.MustRegister(acl.NewService())
	// registry.MustRegister(analytics.NewService())
	// registry.MustRegister(integration.NewService())
	registry.MustRegister(proxy.NewProxyProvider())
	registry.MustRegister(wireguard.NewService())

	return registry
}

// maskAPIKey masks the API key for logging (shows first 8 chars)
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:8] + "***"
}
