package config

import (
	"crypto/rand"
	"errors"
	"math/big"
	"os"

	"github.com/joho/godotenv"
)

const (
	alphabets    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	edgeIDLength = 16
)

// Config holds the application configuration
type Config struct {
	EdgeID     string // Unique edge identifier (auto-generated if not set)
	APIKey     string // API key for authentication
	ServerAddr string
	DBPath     string
	CloudURL   string // Cloud server URL for edge registry, WebRTC signaling, and API key management
}

// Load loads configuration from environment variables and optional .env file
func Load() (*Config, error) {
	// Try to load .env.local file (optional, for local development)
	_ = godotenv.Load(".env.local")

	// Also try .env file as fallback
	_ = godotenv.Load(".env")

	return loadFromEnv()
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv() (*Config, error) {
	apiKey := os.Getenv("ARQUT_API_KEY")
	if apiKey == "" {
		return nil, errors.New("ARQUT_API_KEY environment variable is required")
	}

	// Get or generate EdgeID
	edgeID := os.Getenv("EDGE_ID")
	if edgeID == "" {
		// Generate random 8-character ID if not provided
		generatedID, err := generateEdgeID()
		if err != nil {
			return nil, errors.New("failed to generate Edge ID: " + err.Error())
		}
		edgeID = generatedID
	}

	cfg := &Config{
		EdgeID:     edgeID,
		APIKey:     apiKey,
		ServerAddr: getEnv("SERVER_ADDR", ":3030"),
		DBPath:     getEnv("DB_PATH", "./data/edge.db"),
		CloudURL:   getEnv("CLOUD_URL", ""),
	}

	return cfg, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// generateEdgeID generates a random 8-character edge ID
func generateEdgeID() (string, error) {
	id := make([]byte, edgeIDLength)

	for i := range edgeIDLength {
		char, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabets))))
		if err != nil {
			return "", err
		}
		id[i] = alphabets[char.Int64()]
	}

	return string(id), nil
}
