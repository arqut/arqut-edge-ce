package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/arqut/arqut-edge-ce/pkg/utils"
	"github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v3"
)

var cfg *Config

// Config holds the application configuration
type Config struct {
	EdgeID     string `yaml:"edge_id"` // Unique edge identifier (auto-generated if not set)
	APIKey     string `yaml:"api_key"` // API key for authentication
	DBPath     string `yaml:"db_path"`
	CloudURL   string `yaml:"cloud_url"` // Cloud server URL for edge registry, WebRTC signaling, and API key management
	ServerAddr string `yaml:"server_addr"`
	LogLevel   string `yaml:"log_level"`

	Version string `yaml:"-"`

	mu   sync.Mutex `yaml:"-"`
	file string     `yaml:"-"`
}

func (c *Config) GetServerPort() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return strings.Split(c.ServerAddr, ":")[1]
}

// // GetConfigFile returns the path to the config file
// func (c *Config) GetConfigFile() string {
// 	return c.file
// }

// Save writes the current configuration back to the file
func (c *Config) Save() error {
	if c.file == "" {
		return fmt.Errorf("config file path is not set")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	err = os.WriteFile(c.file, data, 0o644)
	if err != nil {
		return err
	}

	return nil
}

// ensureDefaultConfig sets default values for missing config fields
func (c *Config) EnsureDefaultConfig(save bool) error {
	changed := false
	c.mu.Lock()

	// Env overrides
	if apiKey := utils.Env("ARQUT_API_KEY", ""); apiKey != "" {
		c.APIKey = apiKey
	}

	if cloudUrl := utils.Env("ARQUT_CLOUD_URL", ""); cloudUrl != "" {
		c.CloudURL = cloudUrl
	}

	if logLevel := utils.Env("ARQUT_LOG_LEVEL", ""); logLevel != "" {
		c.LogLevel = logLevel
	}

	// Create defaults
	if c.EdgeID == "" {
		edgeID, _ := utils.GenerateID()
		c.EdgeID = edgeID
		changed = true
	}

	if c.DBPath == "" {
		dir := filepath.Dir(c.file)
		c.DBPath = dir + "/arqut.db"
		changed = true
	}

	if c.ServerAddr == "" {
		c.ServerAddr = ":3030"
		changed = true
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
		changed = true
	}

	c.mu.Unlock()

	if changed && save {
		return c.Save()
	}
	return nil
}

// ConfigInstance returns the global config instance
func ConfigInstance() *Config {
	return cfg
}

// Load loads configuration from the specified file and environment variables
func Load(version, file, logLevel string) (*Config, error) {
	_ = godotenv.Load(".env")

	cfg = &Config{
		Version: version,
		file:    file,
	}

	yamlFeeder := feeder.Yaml{Path: file}
	_ = config.New().AddFeeder(yamlFeeder).AddStruct(cfg).Feed()

	if err := cfg.EnsureDefaultConfig(true); err != nil {
		return nil, err
	}

	// Override log level from command-line argument
	if logLevel != "" {
		cfg.LogLevel = logLevel
	}

	return cfg, nil
}
