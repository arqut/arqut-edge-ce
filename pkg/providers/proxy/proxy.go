package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/arqut/arqut-edge-ce/pkg/config"
	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/arqut/arqut-edge-ce/pkg/models"
	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/arqut/arqut-edge-ce/pkg/signaling"
	"github.com/arqut/arqut-edge-ce/pkg/storage/repositories"
	"github.com/arqut/arqut-edge-ce/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

// Message type constants for proxy service sync
const (
	MessageTypeServiceSync      = "service-sync"
	MessageTypeServiceSyncBatch = "service-sync-batch"
	MessageTypeServiceSyncAck   = "service-sync-ack"
)

// SyncCallback tracks a pending sync operation
type SyncCallback struct {
	operation  string
	serviceID  string
	timestamp  time.Time
	retryCount int
}

// ProxyProvider implements Provider using HTTP reverse proxy
type ProxyProvider struct {
	cfg        *config.Config
	repo       *repositories.ServiceRepository
	logger     *logger.Logger
	interfaces map[string]string // interface name -> IP
	servers    map[string]*http.Server
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
	portRange  struct {
		start int
		end   int
	}
	pingServer      *http.Server
	shutdownTimeout time.Duration
	started         bool
	syncChan        chan<- *signaling.OutboundMessage
	syncCallbacks   map[string]SyncCallback // Track pending syncs by message ID
	callbackMu      sync.Mutex
}

// NewProxyProvider creates a new proxy provider
func NewProxyProvider() *ProxyProvider {
	proxy := &ProxyProvider{
		interfaces:      make(map[string]string),
		servers:         make(map[string]*http.Server),
		shutdownTimeout: 30 * time.Second,
		started:         false,
		syncCallbacks:   make(map[string]SyncCallback),
	}

	// Default port range for tunnel ports
	proxy.portRange.start = 8000
	proxy.portRange.end = 9000

	return proxy
}

// Name returns the service name
func (p *ProxyProvider) Name() string {
	return "proxy"
}

// Initialize sets up the proxy service with dependencies
func (p *ProxyProvider) Initialize(ctx context.Context, registry *providers.Registry) error {
	registry.Logger().Println("Initializing proxy service")

	cfg, ok := registry.Config().(*config.Config)
	if !ok {
		return fmt.Errorf("invalid config type")
	}
	p.cfg = cfg

	p.repo = registry.DB().ServiceRepo()
	p.logger = registry.Logger()

	// Expose UI as service if no services exist
	if err := p.ExposeUIAsService(); err != nil {
		return fmt.Errorf("failed to expose UI as service: %w", err)
	}

	return nil
}

// IsRunnable returns true as proxy service runs in background
func (p *ProxyProvider) IsRunnable() bool {
	return true
}

// Stop gracefully shuts down the proxy service
func (p *ProxyProvider) Stop(ctx context.Context) error {
	p.stopInternal()
	return nil
}

// RegisterAPIRoutes registers proxy-related routes
func (p *ProxyProvider) RegisterAPIRoutes(router fiber.Router, middlewares ...fiber.Handler) {
	p.RegisterRoutes(router, middlewares...)
}

// SetSyncChannel sets the channel for sending sync messages to signaling
func (p *ProxyProvider) SetSyncChannel(ch chan<- *signaling.OutboundMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.syncChan = ch
}

// syncAllServices sends all services to the cloud via signaling channel
func (p *ProxyProvider) syncAllServices(remove bool) {
	p.mu.RLock()
	syncChan := p.syncChan
	p.mu.RUnlock()

	if syncChan == nil {
		// No sync channel configured, skip sync
		return
	}

	services, err := p.repo.GetServices()
	if err != nil {
		if p.logger != nil {
			p.logger.Printf("[Proxy] Failed to get services for sync: %v", err)
		}
		return
	}

	if len(services) == 0 {
		if p.logger != nil {
			p.logger.Println("[Proxy] No services to sync")
		}
		return
	}

	// Generate unique message ID for tracking
	messageID, _ := utils.GenerateID()
	operation := "sync"
	if remove {
		operation = "remove"
	}

	// Register callback before sending (batch operation)
	p.callbackMu.Lock()
	p.syncCallbacks[messageID] = SyncCallback{
		operation:  "batch-" + operation,
		serviceID:  fmt.Sprintf("%d services", len(services)),
		timestamp:  time.Now(),
		retryCount: 0,
	}
	p.callbackMu.Unlock()

	// Prepare sync message data
	data := map[string]any{
		"message_id": messageID,
		"services":   services,
		"operation":  operation,
	}

	// Send to outbound channel (non-blocking)
	select {
	case syncChan <- &signaling.OutboundMessage{
		Type: MessageTypeServiceSyncBatch,
		Data: data,
	}:
		if p.logger != nil {
			p.logger.Printf("[Proxy] Queued %s for %d services (msg_id: %s)", operation, len(services), messageID)
		}
	default:
		// Remove callback if we can't send
		p.callbackMu.Lock()
		delete(p.syncCallbacks, messageID)
		p.callbackMu.Unlock()

		if p.logger != nil {
			p.logger.Println("[Proxy] Warning: sync channel full, skipping sync")
		}
	}
}

// syncServiceOperation sends an individual service operation to the cloud
func (p *ProxyProvider) syncServiceOperation(operation string, service *models.ProxyService) {
	p.mu.RLock()
	syncChan := p.syncChan
	p.mu.RUnlock()

	if syncChan == nil {
		// No sync channel configured, skip sync
		return
	}

	// Generate unique message ID for tracking
	messageID, _ := utils.GenerateID()

	// Register callback before sending
	p.callbackMu.Lock()
	p.syncCallbacks[messageID] = SyncCallback{
		operation:  operation,
		serviceID:  service.ID,
		timestamp:  time.Now(),
		retryCount: 0,
	}
	p.callbackMu.Unlock()

	// Prepare sync message data
	data := map[string]any{
		"message_id": messageID,
		"operation":  operation,
		"service":    service,
	}

	// Send to outbound channel (non-blocking)
	select {
	case syncChan <- &signaling.OutboundMessage{
		Type: MessageTypeServiceSync,
		Data: data,
	}:
		if p.logger != nil {
			p.logger.Printf("[Proxy] Queued %s operation for service %s (msg_id: %s)", operation, service.ID, messageID)
		}
	default:
		// Remove callback if we can't send
		p.callbackMu.Lock()
		delete(p.syncCallbacks, messageID)
		p.callbackMu.Unlock()

		if p.logger != nil {
			p.logger.Printf("[Proxy] Warning: sync channel full, skipping %s for service %s", operation, service.ID)
		}
	}
}

// OnReconnect is called when signaling reconnects, triggers full service sync
func (p *ProxyProvider) OnReconnect(ctx context.Context) error {
	p.logger.Println("[Proxy] Signaling reconnected, syncing all services")
	p.syncAllServices(false)
	return nil
}

// HandleServiceSyncAck processes acknowledgment from cloud server
func (p *ProxyProvider) HandleServiceSyncAck(ctx context.Context, msg *signaling.SignallingMessage) error {
	var ack map[string]any
	if err := json.Unmarshal(msg.Data, &ack); err != nil {
		return fmt.Errorf("failed to unmarshal ack: %w", err)
	}

	status, _ := ack["status"].(string)
	message, _ := ack["message"].(string)
	messageID, _ := ack["message_id"].(string)

	// Look up the callback for this message
	p.callbackMu.Lock()
	callback, exists := p.syncCallbacks[messageID]
	if exists {
		delete(p.syncCallbacks, messageID)
	}
	p.callbackMu.Unlock()

	if status == "success" {
		if exists {
			p.logger.Printf("[Proxy] Service sync acknowledged - %s (operation: %s, service: %s)",
				message, callback.operation, callback.serviceID)
		} else {
			p.logger.Printf("[Proxy] Service sync acknowledged - %s", message)
		}
		// Future: Track success metrics here
	} else {
		if exists {
			p.logger.Printf("[Proxy] Service sync failed - %s (operation: %s, service: %s)",
				message, callback.operation, callback.serviceID)
			// Future: Implement retry logic here
		} else {
			p.logger.Printf("[Proxy] Service sync failed - %s", message)
		}
	}

	return nil
}

// SetPortRange sets the port allocation range
func (p *ProxyProvider) SetPortRange(start, end int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.portRange.start = start
	p.portRange.end = end
}

// allocatePort finds an available port in the configured range
func (p *ProxyProvider) allocatePort() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	usedPorts, err := p.repo.GetUsedPorts()
	if err != nil {
		return 0, fmt.Errorf("failed to get used ports: %w", err)
	}

	usedPortMap := make(map[int]bool)
	for _, port := range usedPorts {
		usedPortMap[port] = true
	}

	for port := p.portRange.start; port <= p.portRange.end; port++ {
		if !usedPortMap[port] {
			// Verify port is actually available on the system
			if ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port)); err == nil {
				ln.Close()
				return port, nil
			}
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", p.portRange.start, p.portRange.end)
}

// Start starts the proxy service
func (p *ProxyProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return fmt.Errorf("proxy already started")
	}

	childCtx, cancel := context.WithCancel(ctx)
	p.ctx = childCtx
	p.cancel = cancel
	p.started = true
	p.mu.Unlock()

	// Start ping service on port 3031 (non-critical, log error but don't fail)
	if err := p.startPingService(childCtx, 3031); err != nil {
		p.logger.Printf("Warning: Ping service on port 3031 failed to start: %v", err)
		p.logger.Printf("Continuing without ping service (this is non-critical)")
	}

	// Load and start all enabled services
	services, err := p.repo.GetServices()
	if err != nil {
		p.mu.Lock()
		p.started = false
		p.mu.Unlock()
		cancel()
		return fmt.Errorf("failed to load services: %w", err)
	}

	p.logger.Printf("Starting proxy with %d services", len(services))

	var startErrors []error
	for _, service := range services {
		if service.Enabled {
			if err := p.startService(childCtx, service); err != nil {
				startErrors = append(startErrors, fmt.Errorf("service %s: %w", service.Name, err))
				p.logger.Printf("Failed to start service %s: %v", service.Name, err)
			}
		}
	}

	if len(startErrors) > 0 {
		p.logger.Printf("Some services failed to start: %d errors", len(startErrors))
	}

	return nil
}

// stopInternal stops all proxy services (internal method)
func (p *ProxyProvider) stopInternal() {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return
	}
	p.started = false
	p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
	}

	// Wait for graceful shutdown with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if p.logger != nil {
			p.logger.Println("All proxy services stopped gracefully")
		} else {
			log.Println("All proxy services stopped gracefully")
		}
	case <-time.After(p.shutdownTimeout):
		if p.logger != nil {
			p.logger.Println("Proxy shutdown timeout reached")
		} else {
			log.Println("Proxy shutdown timeout reached")
		}
	}
}

// startPingService starts the health check endpoint
func (p *ProxyProvider) startPingService(ctx context.Context, port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"pong"}`))
	})

	p.pingServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	p.wg.Add(2)

	go func() {
		defer p.wg.Done()
		p.logger.Printf("Starting ping service on :%d", port)
		if err := p.pingServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.logger.Printf("Ping service error: %v", err)
		}
	}()

	go func() {
		defer p.wg.Done()
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := p.pingServer.Shutdown(shutdownCtx); err != nil {
			p.logger.Printf("Force closing ping server: %v", err)
			p.pingServer.Close()
		}
		p.logger.Println("Ping service stopped")
	}()

	return nil
}

// SetInterfaceIPs sets all network interfaces
func (p *ProxyProvider) SetInterfaceIPs(ips map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.interfaces = ips
}

// AddInterface adds a network interface
func (p *ProxyProvider) AddInterface(name, ip string) {
	p.mu.Lock()
	p.interfaces[name] = ip
	p.mu.Unlock()

	p.startServicesOnInterface(ip)
}

// RemoveInterface removes a network interface
func (p *ProxyProvider) RemoveInterface(name string) {
	p.mu.Lock()
	ip, exists := p.interfaces[name]
	if exists {
		delete(p.interfaces, name)
	}
	p.mu.Unlock()

	if exists {
		p.stopServicesOnInterface(ip)
	}
}

func (p *ProxyProvider) ExposeUIAsService() error {
	// Check if any services exist, if not create a default one
	count, err := p.repo.Count()
	if err != nil {
		return fmt.Errorf("failed to count proxy services: %w", err)
	}

	if count > 0 {
		return nil
	}
	p.logger.Println("No proxy services found, creating default service")

	host, portStr, err := net.SplitHostPort(p.cfg.ServerAddr)
	if err != nil {
		return fmt.Errorf("failed to parse server address: %w", err)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return fmt.Errorf("failed to lookup server port: %w", err)
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
	}

	tunnelPort, err := p.allocatePort()
	if err != nil {
		return fmt.Errorf("failed to allocate tunnel port for default service: %w", err)
	}

	_, err = p.repo.AddService("Edge UI", host, port, tunnelPort, "http")
	if err != nil {
		return fmt.Errorf("failed to create default proxy service: %w", err)
	}
	return nil
}

// AddService creates a new proxy service
func (p *ProxyProvider) AddService(name, localHost string, localPort int, protocol string) (*models.ProxyService, error) {
	// Allocate tunnel port
	tunnelPort, err := p.allocatePort()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate port: %w", err)
	}

	service, err := p.repo.AddService(name, localHost, localPort, tunnelPort, protocol)
	if err != nil {
		return nil, fmt.Errorf("failed to add service: %w", err)
	}

	// Start service if proxy is running
	p.mu.RLock()
	started := p.started
	ctx := p.ctx
	p.mu.RUnlock()

	if started && ctx != nil {
		if err := p.startService(ctx, service); err != nil {
			return service, err
		}
	}

	// Trigger sync after successful add
	p.syncServiceOperation("created", service)

	return service, nil
}

// ModifyService updates a proxy service
func (p *ProxyProvider) ModifyService(id string, config models.ProxyServiceConfig, operations ...string) error {
	err := p.repo.UpdateService(id, config)
	if err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}

	p.restartService(id)

	// Get updated service for sync
	service, err := p.repo.GetService(id)
	if err != nil {
		p.logger.Printf("[Proxy] Failed to get service for sync after modify: %v", err)
		return nil // Don't fail the modify operation
	}

	// Trigger sync after successful modify
	operation := "updated"
	if len(operations) > 0 {
		operation = operations[0]
	}
	p.syncServiceOperation(operation, service)

	return nil
}

// EnableService enables a proxy service
func (p *ProxyProvider) EnableService(id string) error {
	enabled := true
	return p.ModifyService(id, models.ProxyServiceConfig{Enabled: &enabled}, "enabled")
}

// DisableService disables a proxy service
func (p *ProxyProvider) DisableService(id string) error {
	enabled := false
	return p.ModifyService(id, models.ProxyServiceConfig{Enabled: &enabled}, "disabled")
}

// DeleteService deletes a proxy service
func (p *ProxyProvider) DeleteService(id string) error {
	// Get service before deleting for sync
	service, err := p.repo.GetService(id)
	if err != nil {
		return fmt.Errorf("failed to get service: %w", err)
	}

	p.stopService(id)

	if err := p.repo.DeleteService(id); err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	// Trigger sync after successful delete
	p.syncServiceOperation("deleted", service)

	return nil
}

// Clear removes all proxy services
func (p *ProxyProvider) Clear() error {
	p.syncAllServices(true)
	return p.repo.Clear()
}

// CreateHAAddonService creates a proxy service for Home Assistant when running as HA Add-on
func (p *ProxyProvider) CreateHAAddonService() (*models.ProxyService, error) {
	if !p.cfg.IsHAAddon {
		return nil, fmt.Errorf("not running in HA Addon mode")
	}

	// Check if service already exists
	_, err := p.repo.GetServiceByHostPort("homeassistant.local", 8123)
	if err == nil {
		return nil, fmt.Errorf("the service is already set up and running")
	}

	p.logger.Info("Trying to expose HA Addon as a service")

	service, err := p.AddService("Home Assistant Dashboard", "homeassistant.local", 8123, "http")
	if err != nil {
		return nil, fmt.Errorf("could not create the service to expose the Home Assistant Add-on: %w", err)
	}

	return service, nil
}

// startService starts a proxy service on all interfaces
func (p *ProxyProvider) startService(ctx context.Context, service *models.ProxyService) error {
	p.mu.RLock()
	interfaces := make(map[string]string)
	maps.Copy(interfaces, p.interfaces)
	p.mu.RUnlock()

	var startErrors []error
	for _, ip := range interfaces {
		addr := fmt.Sprintf("%s:%d", ip, service.TunnelPort)

		if err := p.startReverseProxyService(ctx, service, addr); err != nil {
			startErrors = append(startErrors, fmt.Errorf("failed to start %s service %s on %s: %w",
				strings.ToUpper(service.Protocol), service.Name, addr, err))
		}
	}

	if len(startErrors) > 0 {
		for _, err := range startErrors {
			p.logger.Println(err.Error())
		}
		return startErrors[0]
	}

	return nil
}

// startReverseProxyService starts a reverse proxy on a specific address
func (p *ProxyProvider) startReverseProxyService(ctx context.Context, service *models.ProxyService, addr string) error {
	scheme := "http"
	if strings.ToLower(service.Protocol) == "websocket" {
		scheme = "http" // WebSocket upgrades start as HTTP
	}

	target, err := url.Parse(fmt.Sprintf("%s://%s:%d", scheme, service.LocalHost, service.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to parse target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// Log incoming request
		p.logger.Printf("[Proxy] %s -> %s %s%s", service.Name, req.Method, req.Host, req.URL.RequestURI())

		originalDirector(req)

		// Set the Host header to the target host (required for HA and other apps that check Host)
		req.Host = target.Host

		// Add forwarded headers
		if req.Header.Get("X-Forwarded-Proto") == "" {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
		if req.Header.Get("X-Forwarded-For") == "" {
			if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
				req.Header.Set("X-Forwarded-For", clientIP)
			}
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		p.logger.Printf("Proxy error for service %s: %v", service.Name, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      proxy,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	key := fmt.Sprintf("%s-%s", service.ID, addr)
	p.mu.Lock()
	p.servers[key] = server
	p.mu.Unlock()

	p.wg.Add(2)

	go func() {
		defer p.wg.Done()
		p.logger.Printf("Starting %s proxy service %s on %s -> %s:%d",
			strings.ToUpper(service.Protocol), service.Name, addr, service.LocalHost, service.LocalPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.logger.Printf("Proxy server error for %s: %v", service.Name, err)
		}
	}()

	go func() {
		defer p.wg.Done()
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			p.logger.Printf("Force closing server for %s: %v", service.Name, err)
			server.Close()
		}

		p.logger.Printf("Stopped %s proxy service %s on %s",
			strings.ToUpper(service.Protocol), service.Name, addr)
	}()

	return nil
}

// restartService restarts a proxy service
func (p *ProxyProvider) restartService(id string) {
	p.stopService(id)

	service, err := p.repo.GetService(id)
	if err != nil {
		p.logger.Printf("Failed to get service %s for restart: %v", id, err)
		return
	}

	p.mu.RLock()
	started := p.started
	ctx := p.ctx
	p.mu.RUnlock()

	if service.Enabled && started && ctx != nil {
		if err := p.startService(ctx, service); err != nil {
			p.logger.Printf("Failed to restart service %s: %v", id, err)
		}
	}
}

// stopService stops a proxy service
func (p *ProxyProvider) stopService(id string) {
	p.mu.Lock()
	var serversToShutdown []*http.Server
	keysToDelete := []string{}

	for key, server := range p.servers {
		if strings.HasPrefix(key, id+"-") {
			serversToShutdown = append(serversToShutdown, server)
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(p.servers, key)
	}
	p.mu.Unlock()

	for _, server := range serversToShutdown {
		p.logger.Printf("Stopping server for service %s on %s", id, server.Addr)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := server.Shutdown(shutdownCtx); err != nil {
			p.logger.Printf("Graceful shutdown failed for %s, forcing close: %v", server.Addr, err)
			server.Close()
		}
		cancel()
	}
}

// startServicesOnInterface starts all services on a new interface
func (p *ProxyProvider) startServicesOnInterface(ip string) {
	p.mu.RLock()
	started := p.started
	ctx := p.ctx
	p.mu.RUnlock()

	if !started || ctx == nil {
		return
	}

	services, err := p.repo.GetServices()
	if err != nil {
		p.logger.Printf("Failed to get services for interface %s: %v", ip, err)
		return
	}

	for _, service := range services {
		if service.Enabled {
			addr := fmt.Sprintf("%s:%d", ip, service.TunnelPort)
			if err := p.startReverseProxyService(ctx, service, addr); err != nil {
				p.logger.Printf("Failed to start service %s on new interface %s: %v", service.Name, ip, err)
			}
		}
	}
}

// stopServicesOnInterface stops all services on a removed interface
func (p *ProxyProvider) stopServicesOnInterface(ip string) {
	p.mu.Lock()
	var serversToShutdown []*http.Server
	keysToDelete := []string{}

	for key, server := range p.servers {
		host, _, err := net.SplitHostPort(server.Addr)
		if err == nil && host == ip {
			serversToShutdown = append(serversToShutdown, server)
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(p.servers, key)
	}
	p.mu.Unlock()

	for _, server := range serversToShutdown {
		p.logger.Printf("Stopping server on removed interface %s: %s", ip, server.Addr)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := server.Shutdown(shutdownCtx); err != nil {
			p.logger.Printf("Graceful shutdown failed for %s, forcing close: %v", server.Addr, err)
			server.Close()
		}
		cancel()
	}
}

// Verify that ProxyProvider implements both Service and Provider interfaces
var (
	_ providers.ProxyProvider = (*ProxyProvider)(nil)
	_ providers.Service       = (*ProxyProvider)(nil)
)
