package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/arqut/arqut-edge-ce/pkg/providers"
	"github.com/gofiber/fiber/v2"
)

// Service implements authentication service
type Service struct {
	users  map[string]string // username -> password hash
	tokens map[string]string // token -> username
	mu     sync.RWMutex
}

// NewService creates a new auth service
func NewService() *Service {
	return &Service{
		users:  make(map[string]string),
		tokens: make(map[string]string),
	}
}

// Name returns the service name
func (s *Service) Name() string {
	return "auth"
}

// Initialize sets up the service with demo users
func (s *Service) Initialize(ctx context.Context, registry *providers.Registry) error {
	registry.Logger().Println("Initializing auth service with demo users")

	s.mu.Lock()
	defer s.mu.Unlock()

	s.users["admin"] = hashPassword("admin123")
	s.users["user"] = hashPassword("user123")

	return nil
}

// IsRunnable returns false as auth service doesn't need background processing
func (s *Service) IsRunnable() bool {
	return false
}

// Run is not used for auth service
func (s *Service) Start(ctx context.Context) error {
	return nil
}

// Stop gracefully shuts down the service
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear tokens on shutdown
	s.tokens = make(map[string]string)
	return nil
}

// RegisterAPIRoutes registers auth-related routes
func (s *Service) RegisterAPIRoutes(router fiber.Router, middlewares ...fiber.Handler) {
	// Auth routes are handled by apiserver for now
	// This can be moved here in the future
}

// Authenticate validates credentials and returns a token
func (s *Service) Authenticate(ctx context.Context, username, password string) (string, error) {
	s.mu.RLock()
	expectedHash, exists := s.users[username]
	s.mu.RUnlock()

	if !exists || expectedHash != hashPassword(password) {
		return "", errors.New("invalid credentials")
	}

	token := generateToken(username)
	s.mu.Lock()
	s.tokens[token] = username
	s.mu.Unlock()

	return token, nil
}

// ValidateToken validates a token and returns the username
func (s *Service) ValidateToken(ctx context.Context, token string) (string, error) {
	s.mu.RLock()
	username, exists := s.tokens[token]
	s.mu.RUnlock()

	if !exists {
		return "", errors.New("invalid token")
	}

	return username, nil
}

// Helper functions

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func generateToken(username string) string {
	data := fmt.Sprintf("%s:%d", username, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Verify that Service implements both Service and AuthProvider interfaces
var (
	_ providers.Service      = (*Service)(nil)
	_ providers.AuthProvider = (*Service)(nil)
)
